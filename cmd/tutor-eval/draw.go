package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/altnovatrail/tutor/internal/newfile"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/sheets"
	"github.com/altnovatrail/tutor/internal/synth"
)

// runDraw draws, with internal/synth, every sheet of the plan that the directory of sheets does
// not hold yet, and saves each as the record mode would, with the writer "synthetic" (R26). The
// sheets the author wrote stay as they are, and so does every file already there. The same seed
// draws the same sheets.
func runDraw(args []string, stdout, stderr io.Writer, now func() time.Time) int {
	flags := flag.NewFlagSet("tutor-eval draw", flag.ContinueOnError)
	var f sheetFlags
	f.add(flags)
	seed := flags.Uint64("seed", 1, "the number the hands of the drawn sheets come from")
	if code, ok := parse(flags, args, stderr); !ok {
		return code
	}
	if !f.usable("tutor-eval draw", stderr) {
		return exitUsage
	}
	if err := checkIgnored(f.dir); err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval draw: %v\n", err)
		return exitUsage
	}
	bank, plan, err := readPlan(f.bank, f.plan)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval draw: %v\n", err)
		return exitFail
	}
	if err := os.MkdirAll(f.dir, 0o700); err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval draw: %v\n", err)
		return exitFail
	}
	r, err := sheets.Read(f.dir, plan, bank)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval draw: %v\n", err)
		return exitFail
	}

	var drawn, kept []int
	at := now().UTC().Truncate(time.Millisecond)
	for _, sh := range r.Sheets {
		if sh.File != "" {
			kept = append(kept, sh.Plan.Number)
			continue
		}
		s := synth.Sheet(sh.Plan, *seed)
		if err := drawnRight(s, sh.Plan, bank); err != nil {
			_, _ = fmt.Fprintf(stderr, "tutor-eval draw: sheet %d is drawn wrong, and nothing more is saved: %v\n", sh.Plan.Number, err)
			return exitFail
		}
		data, err := sample.Marshal(s)
		if err == nil {
			at = at.Add(time.Millisecond)
			err = newfile.Write(filepath.Join(f.dir, sample.SheetFile(sh.Plan, at)), data, 0o600)
		}
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "tutor-eval draw: sheet %d: %v\n", sh.Plan.Number, err)
			return exitFail
		}
		drawn = append(drawn, sh.Plan.Number)
	}
	_, err = fmt.Fprintf(stdout, "Drawn by code: %s, into %s.\nKept as they were: %s.\n", sheetList(drawn), f.dir, sheetList(kept))
	if err != nil {
		return exitFail
	}
	return exitOK
}

// drawnRight checks a drawn sheet as the summary will: the format, the bank and the plan sheet's
// markup. A sheet that fails is a fault of the drawing, never saved.
func drawnRight(s *sample.Sample, sh *sample.PlanSheet, bank *sample.Bank) error {
	if err := s.Check(); err != nil {
		return err
	}
	if err := s.CheckWithBank(bank); err != nil {
		return err
	}
	if diffs := sheets.Differences(sh, s); len(diffs) > 0 {
		return fmt.Errorf("its markup is off the plan at %s: %s", diffs[0].Path, diffs[0].Message)
	}
	return nil
}

// sheetList names sheets by number, or says there are none.
func sheetList(numbers []int) string {
	if len(numbers) == 0 {
		return "none"
	}
	return fmt.Sprintf("%s, sheets %s", count(len(numbers), "sheet"), sheets.Numbers(numbers))
}
