// Command tutor-eval is tutor's eval runner: it runs the AI capabilities on eval samples, on the
// same code that api calls, and reports how they did. Its runs come with the measurement's
// runner. Until then it checks the files of samples and banks, and writes synthetic ones: run it
// without arguments for its commands.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/synth"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// The exit codes.
const (
	exitOK    = 0
	exitFail  = 1 // a file is invalid or cannot be read or written, or the output cannot be written
	exitUsage = 2
)

const usage = `usage:
  tutor-eval board [-addr A] [-dir D] [-max-usd N]   the check board: solve tasks by hand, the models
                                                      judge, you mark whether they were right
  tutor-eval check FILE...   check samples and banks; samples against a bank given with them
  tutor-eval synth DIR       write the synthetic bank and one synthetic sample per scenario
  tutor-eval -version        print the version
`

// run is the command without the process around it, so that a test can drive it. It returns
// the exit code.
func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("tutor-eval", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() {
		_, _ = fmt.Fprint(stderr, usage)
		flags.PrintDefaults()
	}
	showVersion := flags.Bool("version", false, "print the version and exit")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage
	}
	rest := flags.Args()
	if *showVersion {
		if len(rest) > 0 {
			_, _ = fmt.Fprintf(stderr, "tutor-eval: unexpected arguments: %q\n", rest)
			return exitUsage
		}
		if _, err := fmt.Fprintf(stdout, "tutor-eval %s\n", version()); err != nil {
			return exitFail
		}
		return exitOK
	}
	if len(rest) == 0 {
		// A message that stderr does not take has nowhere else to go: the exit code still says it.
		_, _ = fmt.Fprint(stderr, usage)
		return exitUsage
	}
	switch cmd, operands := rest[0], rest[1:]; cmd {
	case "board":
		return runBoard(operands, stdout, stderr, os.Getenv, serveHTTP)
	case "check":
		if len(operands) == 0 {
			_, _ = fmt.Fprintln(stderr, "tutor-eval check: no files given")
			return exitUsage
		}
		return check(operands, stdout, stderr)
	case "synth":
		if len(operands) != 1 {
			_, _ = fmt.Fprintln(stderr, "tutor-eval synth: give one directory")
			return exitUsage
		}
		return writeSynth(operands[0], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "tutor-eval: unknown command %q\n%s", cmd, usage)
		return exitUsage
	}
}

// check reads every file, tells a sample from a bank by its "format" field, and checks each. A
// sample is also checked against a bank given in the same call; with no bank, its task is not.
func check(paths []string, stdout, stderr io.Writer) int {
	type read struct {
		path   string
		sample *sample.Sample
	}
	var samples []read
	var bank *sample.Bank
	bankPath, bankBroken := "", false
	failed := false
	fail := func(path string, err error) {
		failed = true
		_, _ = fmt.Fprintf(stderr, "%s: %v\n", path, err)
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			fail(path, err)
			continue
		}
		var head struct {
			Format string `json:"format"`
		}
		if err := json.Unmarshal(data, &head); err != nil {
			fail(path, fmt.Errorf("not a JSON object: %w", err))
			continue
		}
		switch head.Format {
		case sample.SampleFormat:
			s, err := sample.ReadSample(bytes.NewReader(data))
			if err != nil {
				fail(path, err)
				continue
			}
			samples = append(samples, read{path, s})
		case sample.BankFormat:
			b, err := sample.ReadBank(bytes.NewReader(data))
			switch {
			case err != nil:
				fail(path, err)
				bankBroken = true
			case bank != nil:
				fail(path, fmt.Errorf("a second bank: %s is the bank of this check", bankPath))
			default:
				bank, bankPath = b, path
				if _, err := fmt.Fprintf(stdout, "%s: ok, bank of %d tasks\n", path, len(b.Tasks)); err != nil {
					return exitFail
				}
			}
		default:
			fail(path, fmt.Errorf("format %q is neither %q nor %q", head.Format, sample.SampleFormat, sample.BankFormat))
		}
	}
	for _, r := range samples {
		note := "its task is not checked: no bank was given"
		if bankBroken {
			note = "its task is not checked: the bank is invalid"
		}
		if bank != nil {
			if err := r.sample.CheckWithBank(bank); err != nil {
				fail(r.path, err)
				continue
			}
			note = "task " + r.sample.Task
		}
		if _, err := fmt.Fprintf(stdout, "%s: ok, sample of the scenario %s, %s\n", r.path, r.sample.Scenario, note); err != nil {
			return exitFail
		}
	}
	if failed {
		return exitFail
	}
	return exitOK
}

// writeSynth writes the synthetic bank and samples into dir, creating it if need be. It never
// overwrites a file, and when it fails it removes the files it has written, so that the next run
// starts clean.
func writeSynth(dir string, stdout, stderr io.Writer) int {
	type file struct {
		name    string
		content any
	}
	files := []file{{"bank.json", synth.Bank()}}
	for _, s := range synth.Samples() {
		files = append(files, file{"sample-" + string(s.Scenario) + ".json", s})
	}
	var written []string
	err := os.MkdirAll(dir, 0o755)
	for _, f := range files {
		if err != nil {
			break
		}
		var data []byte
		if data, err = sample.Marshal(f.content); err == nil {
			path := filepath.Join(dir, f.name)
			if err = writeNew(path, data); err == nil {
				written = append(written, path)
			}
		}
	}
	if err != nil {
		for _, path := range written {
			_ = os.Remove(path)
		}
		_, _ = fmt.Fprintf(stderr, "tutor-eval synth: %v\n", err)
		return exitFail
	}
	if _, err := fmt.Fprintf(stdout, "wrote %d files to %s\n", len(files), dir); err != nil {
		return exitFail
	}
	return exitOK
}

// writeNew writes a file that must not exist yet. A file it could not finish, it removes.
func writeNew(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(path)
	}
	return err
}

// version is the version the go command stamped into the binary: the tag of a release it was
// installed from, or a pseudo-version of the commit a working copy was built at, with +dirty when
// that copy had changes. "(devel)" stands for no version at all, as for a test binary.
func version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" {
		return "(devel)"
	}
	return info.Main.Version
}
