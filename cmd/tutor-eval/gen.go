package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/altnovatrail/tutor/internal/gen"
	"github.com/altnovatrail/tutor/internal/hint"
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

// runGen makes, with their truth by construction, from the seed, a bank of generated tasks and a
// plan of their sheets, or with workbook and essays one file of a set (runGenFile), and writes them
// as new files: it never writes over a file, so the sets of E2 and E4 stay as they are. The same
// seed makes the same files again. draw then draws a plan, as it draws one written by hand. A plan
// holds sample.MaxSheets sheets at most, so more sheets go into several plans, each made with its
// own seed and prefix. Of a bank and a plan it writes both files or neither.
func runGen(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "tutor-eval gen: give sheets, near, workbook, essays, words, words-cases or correct")
		return exitUsage
	}
	switch args[0] {
	case "words":
		return runGenSet("words", args[1:], stdout, stderr, func(from *sample.Plan, bank *sample.Bank, planFile string, seed uint64) (*sample.Plan, any, int, error) {
			plan, cases, err := gen.Words(from, bank, planFile, seed)
			if err != nil {
				return nil, nil, 0, err
			}
			return plan, cases, len(cases.Cases), nil
		})
	case "correct":
		return runGenSet("correct", args[1:], stdout, stderr, func(from *sample.Plan, bank *sample.Bank, planFile string, seed uint64) (*sample.Plan, any, int, error) {
			plan, cases, err := gen.Correct(from, bank, planFile, seed)
			if err != nil {
				return nil, nil, 0, err
			}
			return plan, cases, len(cases.Cases), nil
		})
	case "words-cases":
		return runGenWordsCases(args[1:], stdout, stderr)
	}
	if f, ok := fileGenerators[args[0]]; ok {
		return runGenFile(args[0], f, args[1:], stdout, stderr)
	}
	g, ok := generators[args[0]]
	if !ok {
		_, _ = fmt.Fprintf(stderr, "tutor-eval gen: %q is nothing gen makes: give sheets, near, workbook, essays, words, words-cases or correct\n", args[0])
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
	if err := writeBoth(*bankPath, bank, *planPath, plan); err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval gen: %v\n", err)
		return exitFail
	}
	_, _ = fmt.Fprintf(stdout, "%s and %s %s, of the seed %d: the bank %s and the plan %s.\n", count(len(bank.Tasks), "task"), count(len(plan.Sheets), "sheet"), g.what, *seed, *bankPath, *planPath)
	return exitOK
}

// fileGenerator makes a set as one file from a count and a seed, and says how many it made of what.
type fileGenerator struct {
	make func(n int, seed uint64) (any, int, error)
	what string
}

// fileGenerators are what gen makes as one file, by its first operand: the pages of the workbook and
// the essays, each with what it counts (T94; R50).
var fileGenerators = map[string]fileGenerator{
	"workbook": {func(n int, seed uint64) (any, int, error) {
		wb, err := gen.Workbook(n, seed)
		if err != nil {
			return nil, 0, err
		}
		return wb, len(wb.Pages), nil
	}, "page"},
	"essays": {func(n int, seed uint64) (any, int, error) {
		es, err := gen.Essays(n, seed)
		if err != nil {
			return nil, 0, err
		}
		return es, len(es.Essays), nil
	}, "essay"},
}

// runGenFile makes the pages of the workbook or the essays, with their truth by construction, from
// the seed, and writes them as a new file, as gen writes a bank and a plan.
func runGenFile(kind string, g fileGenerator, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("tutor-eval gen "+kind, flag.ContinueOnError)
	n := flags.Int("n", 300, "how many "+g.what+"s to make")
	seed := flags.Uint64("seed", 1, "the number the "+g.what+"s come from")
	path := flags.String("file", "", "the new file to write")
	if code, ok := parse(flags, args, stderr); !ok {
		return code
	}
	if *path == "" {
		_, _ = fmt.Fprintln(stderr, "tutor-eval gen: give -file, the new file to write")
		return exitUsage
	}
	if *n < 1 {
		_, _ = fmt.Fprintf(stderr, "tutor-eval gen: -n is %d: make at least one %s\n", *n, g.what)
		return exitUsage
	}
	v, made, err := g.make(*n, *seed)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval gen: %v\n", err)
		return exitFail
	}
	data, err := sample.Marshal(v)
	if err == nil {
		err = newfile.Write(*path, data, 0o644)
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval gen: %v\n", err)
		return exitFail
	}
	_, _ = fmt.Fprintf(stdout, "%s of the seed %d: %s.\n", count(made, g.what), *seed, *path)
	return exitOK
}

// runGenSet makes a set of the tutor's words of a plan of lines and its bank, from the seed, as new
// files, both or neither: of the words of the hint, words (T118), a plan of parts of its sheets with
// a mistake, ten of them again with a note at the side, and the file of the cases on it (gen.Words),
// which run -kind hint -cases asks; of the words after a correct answer, correct (T143), the same of
// its sheets whose answer is right (gen.Correct), which run -kind correct -cases asks. draw draws the
// plan, with the same bank.
func runGenSet(name string, args []string, stdout, stderr io.Writer, build func(from *sample.Plan, bank *sample.Bank, planFile string, seed uint64) (*sample.Plan, any, int, error)) int {
	flags := flag.NewFlagSet("tutor-eval gen "+name, flag.ContinueOnError)
	seed := flags.Uint64("seed", 1, "the number the notes, the traps found, the techniques and the questions asked come from")
	bankPath := flags.String("bank", "", "the bank of the plan of lines")
	fromPath := flags.String("from", "", "the plan of lines whose sheets the set takes: with a mistake for words, whose answer is right for correct")
	planPath := flags.String("plan", "", "the new file of the plan of the set")
	casesPath := flags.String("cases", "", "the new file of the cases")
	if code, ok := parse(flags, args, stderr); !ok {
		return code
	}
	if *bankPath == "" || *fromPath == "" || *planPath == "" || *casesPath == "" {
		_, _ = fmt.Fprintf(stderr, "tutor-eval gen %s: give -bank and -from, the bank and the plan of lines, and -plan and -cases, the new files to write\n", name)
		return exitUsage
	}
	for _, path := range []string{*planPath, *casesPath} {
		if _, err := os.Lstat(path); err == nil {
			_, _ = fmt.Fprintf(stderr, "tutor-eval gen %s: %s: file exists, and gen writes only new files\n", name, path)
			return exitFail
		}
	}
	bank, from, err := readPlan(*bankPath, *fromPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval gen %s: %v\n", name, err)
		return exitFail
	}
	plan, cases, n, err := build(from, bank.Bank, filepath.Base(*planPath), *seed)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval gen %s: %v\n", name, err)
		return exitFail
	}
	if err := writeBoth(*planPath, plan, *casesPath, cases); err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval gen %s: %v\n", name, err)
		return exitFail
	}
	_, _ = fmt.Fprintf(stdout, "%s and %s of the seed %d: the plan %s and the cases %s.\n", count(len(plan.Sheets), "sheet"), count(n, "case"), *seed, *planPath, *casesPath)
	return exitOK
}

// writeBoth writes two values as new files, both or neither: when the second cannot be written, the
// first goes again, and a file that was there before is never removed.
func writeBoth(firstPath string, first any, secondPath string, second any) error {
	data, err := sample.Marshal(first)
	if err == nil {
		err = newfile.Write(firstPath, data, 0o644)
	}
	if err != nil {
		return err
	}
	data, err = sample.Marshal(second)
	if err == nil {
		err = newfile.Write(secondPath, data, 0o644)
	}
	if err != nil {
		_ = os.Remove(firstPath)
	}
	return err
}

// runGenWordsCases makes the cases of the words on the sheets with a mistake of a plan whose sheets
// are written already, such as the author's (T119; gen.WordsCases), as a new file: run -kind hint
// -cases asks them on the set of that plan.
func runGenWordsCases(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("tutor-eval gen words-cases", flag.ContinueOnError)
	seed := flags.Uint64("seed", 1, "the number the questions asked come from")
	bankPath := flags.String("bank", "", "the bank of the plan")
	planPath := flags.String("plan", "", "the plan whose sheets with a mistake the cases are of")
	casesPath := flags.String("cases", "", "the new file of the cases")
	if code, ok := parse(flags, args, stderr); !ok {
		return code
	}
	if *bankPath == "" || *planPath == "" || *casesPath == "" {
		_, _ = fmt.Fprintln(stderr, "tutor-eval gen words-cases: give -bank and -plan, the bank and the plan, and -cases, the new file to write")
		return exitUsage
	}
	if _, err := os.Lstat(*casesPath); err == nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval gen words-cases: %s: file exists, and gen writes only new files\n", *casesPath)
		return exitFail
	}
	bank, plan, err := readPlan(*bankPath, *planPath)
	var cases *hint.Cases
	if err == nil {
		cases, err = gen.WordsCases(plan, bank.Bank, filepath.Base(*planPath), *seed)
	}
	var data []byte
	if err == nil {
		data, err = sample.Marshal(cases)
	}
	if err == nil {
		err = newfile.Write(*casesPath, data, 0o644)
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval gen words-cases: %v\n", err)
		return exitFail
	}
	_, _ = fmt.Fprintf(stdout, "%s of the seed %d on the plan %s: %s.\n", count(len(cases.Cases), "case"), *seed, *planPath, *casesPath)
	return exitOK
}
