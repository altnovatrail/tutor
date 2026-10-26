package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/sheets"
	"github.com/altnovatrail/tutor/internal/synth"
)

// recordedSheets writes the synthetic bank and the plan of the synthetic sheets, and saves every
// sheet in a directory as the record mode saves it. It returns the flags that name the three.
func recordedSheets(t *testing.T) []string {
	t.Helper()
	root := t.TempDir()
	plan, samples := synth.Plan()
	dir := filepath.Join(root, "sheets")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC)
	for i, s := range samples {
		data, err := sample.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, sample.SheetFile(&plan.Sheets[i], at)), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]any{"bank.json": synth.Bank(), "plan.json": plan}
	for name, v := range files {
		data, err := sample.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return []string{"-dir", dir, "-bank", filepath.Join(root, "bank.json"), "-plan", filepath.Join(root, "plan.json")}
}

func runIt(args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestSheetsAndSplit(t *testing.T) {
	where := recordedSheets(t)
	dir := where[1]

	code, out, errs := runIt(append([]string{"sheets"}, where...)...)
	if code != exitOK || !strings.HasPrefix(out, "Sheets: 7 of the plan's 7 recorded, 7 of them passing the check.\n") || !strings.HasSuffix(out, "Split: not made yet.\n") {
		t.Fatalf("sheets = %d %q %q", code, out, errs)
	}

	code, out, errs = runIt(append([]string{"split", "-seed", "7", "-dev", "3"}, where...)...)
	path := filepath.Join(dir, sheets.SplitFile)
	if code != exitOK || !strings.HasPrefix(out, "Split with the seed 7, written to "+path+".\nDevelopment, 3 sheets: ") || !strings.Contains(out, "\nTest, 4 sheets: ") {
		t.Fatalf("split = %d %q %q", code, out, errs)
	}
	// Seven scenarios of one sheet each: four have no development sheet, and three no test sheet.
	if !strings.Contains(out, "\nNo sheet in development of ") || !strings.Contains(out, "\nNo sheet in the test of ") {
		t.Errorf("split = %q, want the scenarios it leaves out of a part named", out)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("the split's file: %v, %v; want it readable by its owner alone", info, err)
	}
	sp, err := readFile(path, sample.ReadSplit)
	if err != nil || sp.Seed != 7 || len(sp.Dev) != 3 || len(sp.Test) != 4 {
		t.Fatalf("the split read back = %+v, %v", sp, err)
	}

	// The split is made once.
	code, _, errs = runIt(append([]string{"split", "-seed", "8"}, where...)...)
	if code != exitFail || !strings.Contains(errs, "is there already: a split is made once") {
		t.Errorf("a second split = %d %q, want it refused", code, errs)
	}
	if again, _ := readFile(path, sample.ReadSplit); again.Seed != 7 {
		t.Errorf("the second split overwrote the first: seed %d", again.Seed)
	}
	code, out, _ = runIt(append([]string{"sheets"}, where...)...)
	if code != exitOK || !strings.HasSuffix(out, "Split: made with the seed 7, 3 sheets for development and 4 for the test.\n") {
		t.Errorf("sheets after the split = %d %q", code, out)
	}

	// A sheet of the split saved again: its file no longer counts, and the summary says so.
	plan, samples := synth.Plan()
	data, err := sample.Marshal(samples[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, sample.SheetFile(&plan.Sheets[0], time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC))), data, 0o600); err != nil {
		t.Fatal(err)
	}
	_, out, _ = runIt(append([]string{"sheets"}, where...)...)
	if !strings.HasSuffix(out, "The split's file no longer counts for sheets 1: saved again since the split, or gone. The split stays as it was made.\n") {
		t.Errorf("sheets with a sheet saved again = %q", out)
	}
}

// A split that cannot be read is told from one that does not pass the check.
func TestSheetsSplitUnread(t *testing.T) {
	where := recordedSheets(t)
	if err := os.Mkdir(filepath.Join(where[1], sheets.SplitFile), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, out, _ := runIt(append([]string{"sheets"}, where...)...); !strings.Contains(out, "Split: split.json cannot be read: ") {
		t.Errorf("sheets with a directory for the split = %q", out)
	}
	if err := os.Remove(filepath.Join(where[1], sheets.SplitFile)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(where[1], sheets.SplitFile), []byte(`{"format": "tutor-split", "version": 0, "seed": 1, "dev": [], "test": []}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, out, _ := runIt(append([]string{"sheets"}, where...)...); !strings.Contains(out, "Split: split.json does not pass the check: ") {
		t.Errorf("sheets with a broken split = %q", out)
	}
}

func TestSplitRefuses(t *testing.T) {
	where := recordedSheets(t)
	tests := []struct {
		name string
		args []string
		code int
		want string
	}{
		{"no seed", append([]string{"split"}, where...), exitUsage, "-seed is missing"},
		{"an empty directory", []string{"split", "-seed", "1", "-dir", "", "-bank", where[3], "-plan", where[5]}, exitUsage, "-dir is empty"},
		{"the summary of an empty directory", []string{"sheets", "-dir", "", "-bank", where[3], "-plan", where[5]}, exitUsage, "-dir is empty"},
		{"a development part of every sheet", append([]string{"split", "-seed", "1", "-dev", "7"}, where...), exitFail, "it takes 1 to 6 of the plan's 7 sheets"},
		{"a stray argument", append([]string{"split", "-seed", "1", "extra"}, where...), exitUsage, "unexpected arguments"},
		{"a broken plan", []string{"split", "-seed", "1", "-dir", where[1], "-bank", where[3], "-plan", where[3]}, exitFail, where[3] + ": "},
		// The tests run in cmd/tutor-eval, where samples/sheets is not the samples folder at the root.
		{"the default directory", []string{"split", "-seed", "1", "-bank", where[3], "-plan", where[5]}, exitUsage, "cmd/tutor-eval/samples/sheets is in the git work tree"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, _, errs := runIt(tt.args...)
			if code != tt.code || !strings.Contains(errs, tt.want) {
				t.Errorf("code %d, stderr %q; want %d and %q", code, errs, tt.code, tt.want)
			}
		})
	}
	// A sheet not recorded keeps the split from being made, and no file is written.
	entries, err := os.ReadDir(where[1])
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(where[1], entries[0].Name())); err != nil {
		t.Fatal(err)
	}
	code, _, errs := runIt(append([]string{"split", "-seed", "1"}, where...)...)
	if code != exitFail || !strings.Contains(errs, "this one is not: 1") {
		t.Errorf("a split with sheet 1 missing = %d %q", code, errs)
	}
	if _, err := os.Stat(filepath.Join(where[1], sheets.SplitFile)); !os.IsNotExist(err) {
		t.Errorf("a refused split left its file: %v", err)
	}
	if _, err := os.Stat("samples"); !os.IsNotExist(err) {
		t.Errorf("a samples directory was made in cmd/tutor-eval: %v", err)
	}
}
