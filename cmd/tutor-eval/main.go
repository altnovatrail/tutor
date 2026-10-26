// Command tutor-eval is tutor's eval runner: it runs the AI capabilities on eval samples, on the
// same code that api calls, and reports how they did. Its run command runs the measurement of the
// writing process (R24), and its report command reports on a run; its other commands serve the
// board, check the files of samples, banks and sheet plans, and write synthetic ones: run it
// without arguments for its commands.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/altnovatrail/tutor/internal/newfile"
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
  tutor-eval board -record [-addr A] [-dir D] [-bank F] [-plan F]
                                                      the record mode: write the sheets of the plan,
                                                      and each is saved as a sample
  tutor-eval check FILE...   check samples, banks and sheet plans; samples and plans against
                             a bank given with them
  tutor-eval draw [-seed N] [-dir D] [-bank F] [-plan F]
                             draw by code every sheet of the plan not recorded yet, and save it
                             with the writer "synthetic"
  tutor-eval sheets [-dir D] [-bank F] [-plan F]
                             the summary of the recorded sheets against the plan, without the
                             handwriting
  tutor-eval split -seed N [-dev K] [-dir D] [-bank F] [-plan F]
                             divide the recorded sheets into development and test, once
  tutor-eval run -part dev|test [-repeats N] [-max-usd N] [-log F] [-stub] [-dir D] [-bank F] [-plan F]
                             run the measurement on a part: the estimate first, then calls
                             to Claude once you type yes, and a log of every answer
  tutor-eval report LOG      the report of a run from its log, asking no model: the criterion
                             and the numbers in Markdown and CSV, beside the log
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
	case "draw":
		return runDraw(operands, stdout, stderr, time.Now)
	case "sheets":
		return runSheets(operands, stdout, stderr)
	case "split":
		return runSplit(operands, stdout, stderr)
	case "run":
		return runRun(context.Background(), operands, os.Stdin, stdout, stderr, os.Getenv, time.Now)
	case "report":
		return runReport(operands, stdout, stderr)
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

// check reads every file, tells a sample, a bank and a plan apart by the "format" field, and
// checks each. A sample or a plan is also checked against a bank given in the same call; with no
// bank, its tasks are not.
func check(paths []string, stdout, stderr io.Writer) int {
	// A read is a sample or a plan that passed its own check. What in it depends on the bank is
	// checked once every file is read, wherever the bank stands among them.
	type read struct {
		path      string
		what      string                   // what it is, for the line that says it is ok
		withBank  func(*sample.Bank) error // its check against the bank
		inBank    string                   // what that line says once the bank has passed it
		unchecked string                   // what it says when there is no bank to check against
	}
	var reads []read
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
			reads = append(reads, read{
				path:      path,
				what:      "sample of the scenario " + string(s.Scenario),
				withBank:  s.CheckWithBank,
				inBank:    "task " + s.Task,
				unchecked: "its task is not checked",
			})
		case sample.PlanFormat:
			pl, err := sample.ReadPlan(bytes.NewReader(data))
			if err != nil {
				fail(path, err)
				continue
			}
			reads = append(reads, read{
				path:      path,
				what:      "plan of " + count(len(pl.Sheets), "sheet"),
				withBank:  pl.CheckWithBank,
				inBank:    "its tasks and traps are in the bank",
				unchecked: "its tasks are not checked",
			})
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
				if _, err := fmt.Fprintf(stdout, "%s: ok, bank of %s\n", path, count(len(b.Tasks), "task")); err != nil {
					return exitFail
				}
			}
		default:
			fail(path, fmt.Errorf("format %q is not %q, %q or %q", head.Format, sample.SampleFormat, sample.BankFormat, sample.PlanFormat))
		}
	}
	for _, r := range reads {
		note := r.unchecked + ": no bank was given"
		if bankBroken {
			note = r.unchecked + ": the bank is invalid"
		}
		if bank != nil {
			if err := r.withBank(bank); err != nil {
				fail(r.path, err)
				continue
			}
			note = r.inBank
		}
		if _, err := fmt.Fprintf(stdout, "%s: ok, %s, %s\n", r.path, r.what, note); err != nil {
			return exitFail
		}
	}
	if failed {
		return exitFail
	}
	return exitOK
}

// count writes a number with a noun, in the singular for one: "1 sheet", "40 sheets".
func count(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
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
			if err = newfile.Write(path, data, 0o644); err == nil {
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
