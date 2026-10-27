package main

import (
	"bytes"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/synth"
)

// draw draws the measurement's plan into dir and returns what the command printed.
func draw(t *testing.T, dir string, at time.Time) string {
	t.Helper()
	var out, errs bytes.Buffer
	code := runDraw([]string{"-dir", dir, "-bank", "../../eval/bank.json", "-plan", "../../eval/sheets.json"}, &out, &errs, func() time.Time { return at })
	if code != exitOK {
		t.Fatalf("draw = %d, %s", code, errs.String())
	}
	return out.String()
}

// The command draws every sheet of the plan without a file, keeps the file already there, draws
// nothing on a second run, and the summary then finds all 40 sheets passing, 39 drawn by code.
func TestDraw(t *testing.T) {
	dir := t.TempDir()
	at := time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC)
	_, plan, err := readPlan("../../eval/bank.json", "../../eval/sheets.json")
	if err != nil {
		t.Fatal(err)
	}
	// The author's sheet 1: a file of its own, which the command must leave alone.
	mine := filepath.Join(dir, sample.SheetFile(&plan.Sheets[0], at.Add(-time.Hour)))
	if err := os.WriteFile(mine, []byte("the author's sheet"), 0o600); err != nil {
		t.Fatal(err)
	}

	out := draw(t, dir, at)
	if !strings.Contains(out, "Drawn by code: 39 sheets, sheets 2, 3,") || !strings.Contains(out, "Kept as they were: 1 sheet, sheets 1.") {
		t.Errorf("draw printed\n%s", out)
	}
	if data, err := os.ReadFile(mine); err != nil || string(data) != "the author's sheet" {
		t.Errorf("the author's sheet is now %q, %v", data, err)
	}
	if out := draw(t, dir, at.Add(time.Hour)); !strings.Contains(out, "Drawn by code: none") {
		t.Errorf("a second run printed\n%s", out)
	}

	// The author's sheet stands in for a real one: draw it too, in a directory of its own.
	if err := os.Remove(mine); err != nil {
		t.Fatal(err)
	}
	draw(t, dir, at.Add(2*time.Hour))
	var summary, errs bytes.Buffer
	if code := runSheets([]string{"-dir", dir, "-bank", "../../eval/bank.json", "-plan", "../../eval/sheets.json"}, &summary, &errs); code != exitOK {
		t.Fatalf("sheets = %d, %s", code, errs.String())
	}
	for _, want := range []string{
		"Sheets: 40 of the plan's 40 recorded, 40 of them passing the check.",
		"Planned erasures: 14. Slips: 0 erasures on 0 sheets.",
		"Written with a mouse, pressure 0.5 at every point: none.",
		"Marked off the plan: none.",
	} {
		if !strings.Contains(summary.String(), want) {
			t.Errorf("the summary\n%s\nwant it to hold %q", summary.String(), want)
		}
	}
}

// planFiles writes the synthetic bank and the plan of the synthetic sheets into a directory, and
// returns the flags that name them.
func planFiles(t *testing.T) []string {
	t.Helper()
	root := t.TempDir()
	plan, _ := synth.Plan()
	var flags []string
	for _, f := range []struct {
		flag, name string
		v          any
	}{{"-bank", "bank.json", synth.Bank()}, {"-plan", "plan.json", plan}} {
		data, err := sample.Marshal(f.v)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, f.name)
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		flags = append(flags, f.flag, path)
	}
	return flags
}

// With -png the command saves the final picture of each sheet it draws and the frame before each
// erasure, under the sheet's name, in a directory of their own; the sheets it keeps get none.
func TestDrawPictures(t *testing.T) {
	dir, pics := t.TempDir(), filepath.Join(t.TempDir(), "png")
	at := time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)
	plan, samples := synth.Plan()
	// Sheet 1 is there already, and the command keeps it.
	data, err := sample.Marshal(samples[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, sample.SheetFile(&plan.Sheets[0], at.Add(-time.Hour))), data, 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errs bytes.Buffer
	if code := runDraw(append([]string{"-dir", dir, "-png", pics}, planFiles(t)...), &out, &errs, func() time.Time { return at }); code != exitOK {
		t.Fatalf("draw -png = %d, %s", code, errs.String())
	}

	// Every sheet but the first, and a frame for each of the two sheets with a planned erasure.
	var want []string
	erasures := 0
	for i, sh := range plan.Sheets[1:] {
		base := strings.TrimSuffix(sample.SheetFile(&plan.Sheets[i+1], at.Add(time.Duration(i+1)*time.Millisecond)), ".json")
		want = append(want, base+".png")
		if sh.Erasure != nil {
			want = append(want, base+"-frame-1.png")
			erasures++
		}
	}
	slices.Sort(want)
	entries, err := os.ReadDir(pics)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range entries {
		got = append(got, e.Name())
	}
	if erasures != 2 || !slices.Equal(got, want) {
		t.Fatalf("the pictures are\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if !strings.HasSuffix(out.String(), "Pictures of the sheets drawn: 8 files, into "+pics+".\n") {
		t.Errorf("draw -png printed\n%s", out.String())
	}
	f, err := os.Open(filepath.Join(pics, want[0]))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	cfg, err := png.DecodeConfig(f)
	if err != nil || cfg.Width != 1000 || cfg.Height != 600 {
		t.Errorf("a picture is %d × %d, %v; want the sheet, 1000 × 600", cfg.Width, cfg.Height, err)
	}
}

// A sheet is saved with its pictures or not at all: a picture that cannot be written leaves neither
// the sheet nor its other pictures, and the next run draws the sheet again.
func TestDrawSavesWhole(t *testing.T) {
	dir, pics := t.TempDir(), t.TempDir()
	files := planFiles(t)
	plan, _ := synth.Plan()
	at := time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)
	// Sheet 6, the first with an erasure, is drawn sixth: its frame's name is taken already.
	base := func(n int, at time.Time) string {
		return strings.TrimSuffix(sample.SheetFile(&plan.Sheets[n-1], at.Add(time.Duration(n)*time.Millisecond)), ".json")
	}
	if plan.Sheets[5].Erasure == nil {
		t.Fatal("sheet 6 of the synthetic plan has no erasure")
	}
	blocker := filepath.Join(pics, base(6, at)+"-frame-1.png")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errs bytes.Buffer
	if code := runDraw(append([]string{"-dir", dir, "-png", pics}, files...), &out, &errs, func() time.Time { return at }); code != exitFail || !strings.Contains(errs.String(), "sheet 6 is not saved") {
		t.Fatalf("draw with a frame's name taken = %d, %q", code, errs.String())
	}
	if _, err := os.Stat(filepath.Join(pics, base(6, at)+".png")); !os.IsNotExist(err) {
		t.Errorf("the final picture of the sheet not saved is left: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, base(6, at)+".json")); !os.IsNotExist(err) {
		t.Errorf("the sheet whose frame failed is saved: %v", err)
	}

	later := at.Add(time.Hour)
	out.Reset()
	if code := runDraw(append([]string{"-dir", dir, "-png", pics}, files...), &out, &errs, func() time.Time { return later }); code != exitOK ||
		!strings.Contains(out.String(), "Drawn by code: 2 sheets, sheets 6, 7, ") || !strings.Contains(out.String(), "Kept as they were: 5 sheets, sheets 1, 2, 3, 4, 5.") {
		t.Fatalf("the next draw = %d\n%s%s", code, out.String(), errs.String())
	}
	// The next run draws sheet 6 first, a millisecond after its start.
	for _, name := range []string{"", "-frame-1"} {
		if _, err := os.Stat(filepath.Join(pics, strings.TrimSuffix(sample.SheetFile(&plan.Sheets[5], later.Add(time.Millisecond)), ".json")+name+".png")); err != nil {
			t.Errorf("the next run did not save the picture%s of sheet 6: %v", name, err)
		}
	}
}

func TestDrawRefuses(t *testing.T) {
	files := planFiles(t)
	dir := t.TempDir()
	for _, c := range []struct {
		name string
		args []string
		says string
	}{
		{"no directory for the pictures", []string{"-dir", dir, "-png", ""}, "-png is empty"},
		{"the pictures among the sheets", []string{"-dir", dir, "-png", dir}, "is among the sheets of"},
		{"the pictures in a folder of the sheets", []string{"-dir", dir, "-png", filepath.Join(dir, "png")}, "is among the sheets of"},
		// The tests run in cmd/tutor-eval, where png and samples/sheets are folders git does not ignore.
		{"the pictures in the work tree", []string{"-dir", dir, "-png", "png"}, "cmd/tutor-eval/png is in the git work tree"},
		{"the default directory", nil, "cmd/tutor-eval/samples/sheets is in the git work tree"},
	} {
		t.Run(c.name, func(t *testing.T) {
			var out, errs bytes.Buffer
			if code := runDraw(append(c.args, files...), &out, &errs, time.Now); code != exitUsage || !strings.Contains(errs.String(), c.says) {
				t.Errorf("draw = %d, %q; want %d and %q", code, errs.String(), exitUsage, c.says)
			}
		})
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) > 0 {
		t.Errorf("a refused draw left %v in the directory of the sheets, %v", entries, err)
	}
	for _, made := range []string{"png", "samples"} {
		if _, err := os.Stat(made); !os.IsNotExist(err) {
			t.Errorf("a refused draw made %s in cmd/tutor-eval: %v", made, err)
		}
	}
}

// The command draws the free-form plan, 40 sheets of parts, and the author's three: they pass the
// summary with the plan's markup, and each has its picture, and a frame before its planned erasure.
func TestDrawSheetsOfParts(t *testing.T) {
	root := t.TempDir()
	for _, c := range []struct {
		plan                   string
		sheets, erasures, pics int
	}{{"../../eval/free-sheets.json", 40, 16, 56}, {"../../eval/free-author.json", 3, 2, 5}} {
		name := strings.TrimSuffix(filepath.Base(c.plan), ".json")
		dir, pics := filepath.Join(root, name), filepath.Join(root, name+"-png")
		files := []string{"-dir", dir, "-bank", freeBank, "-plan", c.plan}
		var out, errs bytes.Buffer
		if code := runDraw(append(files, "-png", pics), &out, &errs, time.Now); code != exitOK {
			t.Fatalf("draw %s = %d, %s", c.plan, code, errs.String())
		}
		if want := fmt.Sprintf("Pictures of the sheets drawn: %d files", c.pics); !strings.Contains(out.String(), want) {
			t.Errorf("draw printed\n%s\nwant %q", out.String(), want)
		}
		var summary bytes.Buffer
		errs.Reset()
		if code := runSheets(files, &summary, &errs); code != exitOK {
			t.Fatalf("sheets %s = %d, %s", c.plan, code, errs.String())
		}
		for _, want := range []string{
			fmt.Sprintf("Sheets: %d of the plan's %d recorded, %d of them passing the check.", c.sheets, c.sheets, c.sheets),
			fmt.Sprintf("Planned erasures: %d. Slips: 0 erasures on 0 sheets.", c.erasures),
			"Marked off the plan: none.",
		} {
			if !strings.Contains(summary.String(), want) {
				t.Errorf("the summary of %s\n%s\nwant it to hold %q", c.plan, summary.String(), want)
			}
		}
	}
}
