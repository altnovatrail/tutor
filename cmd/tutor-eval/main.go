// Command tutor-eval is tutor's eval runner: it runs the AI capabilities on eval samples, on the
// same code that api calls, and reports how they did. Its runs come with the measurement's
// runner; until then it only names its version.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is the command without the process around it, so that a test can drive it. It returns
// the exit code: 0 on success, 1 when the output cannot be written, 2 on a usage error.
func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("tutor-eval", flag.ContinueOnError)
	flags.SetOutput(stderr)
	showVersion := flags.Bool("version", false, "print the version and exit")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() > 0 {
		_, _ = fmt.Fprintf(stderr, "tutor-eval: unexpected arguments: %q\n", flags.Args())
		return 2
	}
	if *showVersion {
		if _, err := fmt.Fprintf(stdout, "tutor-eval %s\n", version()); err != nil {
			return 1
		}
		return 0
	}
	// A message that stderr does not take has nowhere else to go: the exit code still says it.
	_, _ = fmt.Fprintln(stderr, "tutor-eval: no runs are defined yet")
	return 2
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
