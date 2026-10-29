package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/altnovatrail/tutor/internal/gen"
	"github.com/altnovatrail/tutor/internal/newfile"
	"github.com/altnovatrail/tutor/internal/sample"
)

// generators are what gen makes, by its first operand, each with what it says it made: the sheets
// of lines, six a task, and the near sheets, two a task (T93; R50).
var generators = map[string]struct {
	make func(tasks int, seed uint64, prefix string) (*sample.Bank, *sample.Plan, error)
	what string
}{
	"sheets": {gen.Sheets, "of lines"},
	"near":   {gen.NearSheets, "of the near key"},
}

// runGen makes a bank of generated tasks and a plan of their sheets, with their truth by
// construction, from the seed, and writes them as new files: it never writes over a file, so the
// banks and plans of E2 and E4 stay as they are. The same seed makes the same files again. draw
// then draws the plan, as it draws one written by hand. A plan holds sample.MaxSheets sheets at most,
// so more sheets go into several plans, each made with its own seed and prefix. It writes both files
// or neither.
func runGen(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "tutor-eval gen: give sheets or near")
		return exitUsage
	}
	g, ok := generators[args[0]]
	if !ok {
		_, _ = fmt.Fprintf(stderr, "tutor-eval gen: %q is nothing gen makes: give sheets or near\n", args[0])
		return exitUsage
	}
	flags := flag.NewFlagSet("tutor-eval gen "+args[0], flag.ContinueOnError)
	tasks := flags.Int("tasks", 10, "how many tasks to make: six sheets each of lines, two each of near sheets, and a plan holds 99 sheets at most")
	seed := flags.Uint64("seed", 1, "the number the tasks and their numbers come from")
	prefix := flags.String("prefix", "gen", "what the IDs of the tasks begin with")
	bankPath := flags.String("bank", "", "the new file of the bank")
	planPath := flags.String("plan", "", "the new file of the plan")
	if code, ok := parse(flags, args[1:], stderr); !ok {
		return code
	}
	if *bankPath == "" || *planPath == "" {
		_, _ = fmt.Fprintln(stderr, "tutor-eval gen: give -bank and -plan, the new files to write")
		return exitUsage
	}
	// Neither file may be there yet: gen writes both or neither.
	for _, path := range []string{*bankPath, *planPath} {
		if _, err := os.Lstat(path); err == nil {
			_, _ = fmt.Fprintf(stderr, "tutor-eval gen: %s: file exists, and gen writes only new files\n", path)
			return exitFail
		}
	}
	bank, plan, err := g.make(*tasks, *seed, *prefix)
	switch {
	case errors.Is(err, gen.ErrTasks):
		_, _ = fmt.Fprintf(stderr, "tutor-eval gen: %v\n", err)
		return exitUsage
	case err != nil:
		_, _ = fmt.Fprintf(stderr, "tutor-eval gen: %v\n", err)
		return exitFail
	}
	for _, f := range []struct {
		path string
		v    any
	}{{*bankPath, bank}, {*planPath, plan}} {
		data, err := sample.Marshal(f.v)
		if err == nil {
			err = newfile.Write(f.path, data, 0o644)
		}
		if err != nil {
			_ = os.Remove(*bankPath)
			_, _ = fmt.Fprintf(stderr, "tutor-eval gen: %v\n", err)
			return exitFail
		}
	}
	_, _ = fmt.Fprintf(stdout, "%s and %s %s, of the seed %d: the bank %s and the plan %s.\n", count(len(bank.Tasks), "task"), count(len(plan.Sheets), "sheet"), g.what, *seed, *bankPath, *planPath)
	return exitOK
}
