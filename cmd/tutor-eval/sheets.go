package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/altnovatrail/tutor/internal/newfile"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/sheets"
)

// sheetFlags are the flags that the commands of the sheets — draw, sheets, split and run — share:
// the directory of the sheets, the bank and the plan, and once usable has passed the directory, the
// ignored folder it is in, or "" outside any work tree.
type sheetFlags struct {
	dir, bank, plan string
	folder          string
}

func (f *sheetFlags) add(flags *flag.FlagSet) {
	flags.StringVar(&f.dir, "dir", sheetsDir, "the directory of the sheets, out of git: in samples, or in synthetic for sheets drawn by code")
	flags.StringVar(&f.bank, "bank", "eval/bank.json", "the bank of tasks")
	flags.StringVar(&f.plan, "plan", "eval/sheets.json", "the sheet plan")
}

// usable refuses an empty -dir, since a command of the sheets reads them from a directory it is
// given, and a directory it may not take: one in the work tree outside samples and synthetic
// (checkIgnored), or either folder itself, since a run logs beside the directory of its sheets.
func (f *sheetFlags) usable(name string, stderr io.Writer) bool {
	if f.dir == "" {
		_, _ = fmt.Fprintf(stderr, "%s: -dir is empty: name the directory of the sheets, or leave the flag out for %s\n", name, sheetsDir)
		return false
	}
	err := checkIgnored(f.dir, samplesFolder, syntheticFolder)
	if err == nil {
		var abs, root string
		abs, root, f.folder, err = placeOf(f.dir)
		if err == nil && f.folder != "" && abs == filepath.Join(root, f.folder) {
			err = fmt.Errorf("%s is the %s folder itself: give -dir a folder in it, such as %s/lines, since a run logs beside the directory of its sheets", abs, f.folder, f.folder)
		}
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return false
	}
	return true
}

// given is the set of the flags the command line set.
func given(flags *flag.FlagSet) map[string]bool {
	set := map[string]bool{}
	flags.Visit(func(fl *flag.Flag) { set[fl.Name] = true })
	return set
}

// parse reads a command's flags and refuses arguments besides them. It returns false with the exit
// code when the command is not to run: after its help, or on a usage error.
func parse(flags *flag.FlagSet, args []string, stderr io.Writer) (int, bool) {
	flags.SetOutput(stderr)
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK, false
		}
		return exitUsage, false
	}
	if flags.NArg() > 0 {
		_, _ = fmt.Fprintf(stderr, "%s: unexpected arguments: %q\n", flags.Name(), flags.Args())
		return exitUsage, false
	}
	return exitOK, true
}

// readPlan reads the bank and the sheet plan and checks the plan against the bank. Its error says
// which file fails.
func readPlan(bankPath, planPath string) (*sample.Bank, *sample.Plan, error) {
	bank, err := readFile(bankPath, sample.ReadBank)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", bankPath, err)
	}
	plan, err := readFile(planPath, sample.ReadPlan)
	if err == nil {
		err = plan.CheckWithBank(bank)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", planPath, err)
	}
	return bank, plan, nil
}

// linesOnly refuses a plan of parts for a command that does not know parts yet: before plans of
// version 1 came, such a command took none, as their version was unknown to the reader. what says
// what the command does with sheets.
func linesOnly(planPath string, plan *sample.Plan, what string) error {
	if plan.HasParts() {
		return fmt.Errorf("%s is a plan of version %d, with parts, and %s sheets of plans of version %d alone", planPath, sample.PlanParts, what, sample.Version)
	}
	return nil
}

// runSheets prints the summary of the sheets the record mode saved, against the plan: what is
// recorded, what passes, by task, scenario and outcome, and nothing of the handwriting (R24). With
// the split made, it says so.
func runSheets(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("tutor-eval sheets", flag.ContinueOnError)
	var f sheetFlags
	f.add(flags)
	if code, ok := parse(flags, args, stderr); !ok {
		return code
	}
	if !f.usable("tutor-eval sheets", stderr) {
		return exitUsage
	}
	bank, plan, err := readPlan(f.bank, f.plan)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval sheets: %v\n", err)
		return exitFail
	}
	r, err := sheets.Read(f.dir, plan, bank)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval sheets: %v\n", err)
		return exitFail
	}
	if _, err := io.WriteString(stdout, sheets.Summary(r)+splitLine(f.dir, plan, r)); err != nil {
		return exitFail
	}
	return exitOK
}

// splitLine says whether the split is made, and with it which sheets of the split no longer have
// its file as the one that counts: saved again since the split, or gone.
func splitLine(dir string, plan *sample.Plan, r *sheets.Recorded) string {
	sp, err := readSplit(dir, plan)
	var fe *sample.FormatError
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "Split: not made yet.\n"
	case errors.As(err, &fe):
		return fmt.Sprintf("Split: %s does not pass the check: %v\n", sheets.SplitFile, err)
	case err != nil:
		return fmt.Sprintf("Split: %s cannot be read: %v\n", sheets.SplitFile, err)
	}
	line := fmt.Sprintf("Split: made with the seed %d, %s for development and %d for the test.\n", sp.Seed, count(len(sp.Dev), "sheet"), len(sp.Test))
	if stale := sheets.Stale(r, sp); len(stale) > 0 {
		line += fmt.Sprintf("The split's file no longer counts for sheets %s: saved again since the split, or gone. The split stays as it was made.\n", sheets.Numbers(stale))
	}
	return line
}

// readSplit reads the split in the directory of sheets and checks it against the plan.
func readSplit(dir string, plan *sample.Plan) (*sample.Split, error) {
	sp, err := readFile(filepath.Join(dir, sheets.SplitFile), sample.ReadSplit)
	if err == nil {
		err = sp.CheckWithPlan(plan)
	}
	return sp, err
}

// runSplit divides the recorded sheets into a development and a test part, by scenario, with the
// seed given, and writes the split next to the sheets. A split is made once, before the first
// change of the prompt, and is never overwritten (R24).
func runSplit(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("tutor-eval split", flag.ContinueOnError)
	var f sheetFlags
	f.add(flags)
	seed := flags.Int64("seed", 0, "the number that decides which sheets of each scenario go to development; give it")
	dev := flags.Int("dev", 10, "how many sheets go to development; the rest go to the test")
	if code, ok := parse(flags, args, stderr); !ok {
		return code
	}
	if !f.usable("tutor-eval split", stderr) {
		return exitUsage
	}
	if !given(flags)["seed"] {
		_, _ = fmt.Fprintln(stderr, "tutor-eval split: -seed is missing: the split is made once, with a seed you choose")
		return exitUsage
	}
	path := filepath.Join(f.dir, sheets.SplitFile)
	if _, err := os.Lstat(path); err == nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval split: %s is there already: a split is made once, before the first change of the prompt, and is not made again\n", path)
		return exitFail
	}
	bank, plan, err := readPlan(f.bank, f.plan)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval split: %v\n", err)
		return exitFail
	}
	r, err := sheets.Read(f.dir, plan, bank)
	if err == nil {
		var sp *sample.Split
		if sp, err = sheets.Split(r, *seed, *dev); err == nil {
			var data []byte
			if data, err = sample.Marshal(sp); err == nil {
				if err = newfile.Write(path, data, 0o600); err == nil {
					return saySplit(stdout, plan, sp, path)
				}
			}
		}
	}
	_, _ = fmt.Fprintf(stderr, "tutor-eval split: %v\n", err)
	return exitFail
}

// saySplit tells the parts of a split made, and the scenarios it leaves out of a part: the prompt is
// tuned without a scenario that has no development sheet.
func saySplit(stdout io.Writer, plan *sample.Plan, sp *sample.Split, path string) int {
	part := func(list []sample.SplitSheet) []int {
		n := make([]int, len(list))
		for i, s := range list {
			n[i] = s.Number
		}
		return n
	}
	out := fmt.Sprintf("Split with the seed %d, written to %s.\nDevelopment, %s: %s.\nTest, %s: %s.\n",
		sp.Seed, path, count(len(sp.Dev), "sheet"), sheets.Numbers(part(sp.Dev)), count(len(sp.Test), "sheet"), sheets.Numbers(part(sp.Test)))
	noDev, noTest := sheets.Lopsided(plan, sp)
	if len(noDev) > 0 {
		out += fmt.Sprintf("No sheet in development of %s.\n", scenarios(noDev))
	}
	if len(noTest) > 0 {
		out += fmt.Sprintf("No sheet in the test of %s.\n", scenarios(noTest))
	}
	if _, err := io.WriteString(stdout, out); err != nil {
		return exitFail
	}
	return exitOK
}

func scenarios(list []sample.Scenario) string {
	names := make([]string, len(list))
	for i, sc := range list {
		names[i] = string(sc)
	}
	return strings.Join(names, ", ")
}
