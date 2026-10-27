package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/altnovatrail/tutor/internal/newfile"
	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/sheets"
	"github.com/altnovatrail/tutor/internal/synth"
)

// runDraw draws, with internal/synth, every sheet of the plan that the directory of sheets does
// not hold yet, and saves each as the record mode would, with the writer "synthetic" (R26). The
// sheets the author wrote stay as they are, and so does every file already there. The same seed
// draws the same sheets. With -png it also saves the pictures of each sheet it draws, in a
// directory of their own, so that the author and the executor can look at them. Of a plan of parts
// it draws the sheets code can draw (synth.Drawable): a sheet it is to draw and cannot, it names,
// and makes nothing.
func runDraw(args []string, stdout, stderr io.Writer, now func() time.Time) int {
	flags := flag.NewFlagSet("tutor-eval draw", flag.ContinueOnError)
	var f sheetFlags
	f.add(flags)
	seed := flags.Uint64("seed", 1, "the number the hands of the drawn sheets come from")
	pngDir := flags.String("png", "", "also save the final picture of each sheet drawn and its frame before each erasure, as PNG, in this directory, out of git and not among the sheets")
	if code, ok := parse(flags, args, stderr); !ok {
		return code
	}
	if !f.usable("tutor-eval draw", stderr) {
		return exitUsage
	}
	pictured := given(flags)["png"]
	if pictured {
		if err := picturesDir(*pngDir, f.dir); err != nil {
			_, _ = fmt.Fprintf(stderr, "tutor-eval draw: %v\n", err)
			return exitUsage
		}
	}
	bank, plan, err := readPlan(f.bank, f.plan)
	if err == nil {
		err = drawable(f.plan, f.dir, plan, bank)
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval draw: %v\n", err)
		return exitFail
	}
	for _, dir := range []string{f.dir, *pngDir} {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			_, _ = fmt.Fprintf(stderr, "tutor-eval draw: %v\n", err)
			return exitFail
		}
	}
	r, err := sheets.Read(f.dir, plan, bank)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval draw: %v\n", err)
		return exitFail
	}

	var drawn, kept []int
	saved := 0 // the pictures saved
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
		at = at.Add(time.Millisecond)
		name := sample.SheetFile(sh.Plan, at)
		data, err := sample.Marshal(s)
		var pics []picture
		if err == nil && pictured {
			pics, err = pictures(s, name)
		}
		// A sheet is saved with its pictures or not at all, so that the next run draws again what
		// this one did not save. The sheet goes last: a run cut short leaves no sheet without them.
		type file struct {
			path string
			data []byte
		}
		files := make([]file, 0, len(pics)+1)
		for _, p := range pics {
			files = append(files, file{filepath.Join(*pngDir, p.name), p.data})
		}
		files = append(files, file{filepath.Join(f.dir, name), data})
		var written []string
		for i := 0; err == nil && i < len(files); i++ {
			if err = newfile.Write(files[i].path, files[i].data, 0o600); err == nil {
				written = append(written, files[i].path)
			}
		}
		if err != nil {
			for _, path := range written {
				_ = os.Remove(path)
			}
			_, _ = fmt.Fprintf(stderr, "tutor-eval draw: sheet %d is not saved: %v\n", sh.Plan.Number, err)
			return exitFail
		}
		saved += len(pics)
		drawn = append(drawn, sh.Plan.Number)
	}
	out := fmt.Sprintf("Drawn by code: %s, into %s.\nKept as they were: %s.\n", sheetList(drawn), f.dir, sheetList(kept))
	if pictured {
		out += fmt.Sprintf("Pictures of the sheets drawn: %s, into %s.\n", count(saved, "file"), *pngDir)
	}
	if _, err := io.WriteString(stdout, out); err != nil {
		return exitFail
	}
	return exitOK
}

// drawable refuses a plan with a sheet code cannot draw yet among those it is to draw, the sheets
// the directory does not hold, and names the first such sheet and why. It reads the directory only
// when there is one, so that a refusal makes nothing.
func drawable(planPath, dir string, plan *sample.Plan, bank *sample.Bank) error {
	held := map[int]bool{}
	if _, err := os.Stat(dir); err == nil {
		r, err := sheets.Read(dir, plan, bank)
		if err != nil {
			return err
		}
		for _, sh := range r.Sheets {
			held[sh.Plan.Number] = sh.File != ""
		}
	}
	for i := range plan.Sheets {
		if sh := &plan.Sheets[i]; !held[sh.Number] {
			if err := synth.Drawable(sh); err != nil {
				return fmt.Errorf("%s: sheet %d: %w", planPath, sh.Number, err)
			}
		}
	}
	return nil
}

// picturesDir refuses a directory for the pictures that is not named, that is the directory of the
// sheets or lies in it, or that lies in the work tree outside samples and synthetic (checkIgnored).
func picturesDir(dir, sheetDir string) error {
	if dir == "" {
		return errors.New("-png is empty: name the directory of the pictures")
	}
	pics, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	of, err := filepath.Abs(sheetDir)
	if err != nil {
		return err
	}
	if within(of, pics) {
		return fmt.Errorf("-png %s is among the sheets of %s: give the pictures a directory of their own", dir, sheetDir)
	}
	return checkIgnored(dir, samplesFolder, syntheticFolder)
}

// picture is a PNG file of a drawn sheet: its name and its bytes.
type picture struct {
	name string
	data []byte
}

// pictures draws a sheet as the measurement shows it (internal/render): the final picture, under
// the name of the sheet's file, and the frame just before each of its erasures, numbered from 1.
func pictures(s *sample.Sample, file string) ([]picture, error) {
	base := strings.TrimSuffix(file, filepath.Ext(file))
	img, err := render.Final(s)
	if err != nil {
		return nil, err
	}
	data, err := render.PNG(img)
	if err != nil {
		return nil, err
	}
	pics := []picture{{base + ".png", data}}
	for i := range s.Erasures {
		img, err := render.Before(s, i)
		if err != nil {
			return nil, err
		}
		if data, err = render.PNG(img); err != nil {
			return nil, err
		}
		pics = append(pics, picture{fmt.Sprintf("%s-frame-%d.png", base, i+1), data})
	}
	return pics, nil
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
