package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/altnovatrail/tutor/internal/samplecheck"
)

// The tests run in the folder of this package, without spec's submodule at its root, and in CI
// without the submodule at all: sample-check, which holds the samples of the protocol to the rules
// of sections 11 and 12, is spec's to run, and the recipe sample-check runs it on the drawn files of
// internal/evalsample's testdata. Here every file keeps the rules unless a test says otherwise.
func init() {
	checkSamples = func([]string) (map[string][]int, error) { return map[string][]int{}, nil }
}

// checking runs the test with a check of its own in place of the one that passes every file.
func checking(t *testing.T, check samplecheck.Check) {
	t.Helper()
	was := checkSamples
	checkSamples = check
	t.Cleanup(func() { checkSamples = was })
}

// A sheet that sample-check finds breaking a rule is refused: the summary names it with its lines
// and no message, a run asks nothing of it and says so before the question, and the others run. A
// check that cannot run stops both.
func TestSampleCheckRefuses(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "lines")
	args := []string{"-dir", dir, "-bank", measurementBank, "-plan", measurementPlan}
	if code, _, errs := runIt(append([]string{"draw"}, args...)...); code != exitOK {
		t.Fatalf("draw = %d %s", code, errs)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "01-*.json"))
	if len(files) != 1 {
		t.Fatalf("sheet 1 is %v", files)
	}
	checking(t, func(paths []string) (map[string][]int, error) {
		if len(paths) != 40 {
			t.Errorf("the check is given %d files, want the 40 sheets", len(paths))
		}
		return map[string][]int{files[0]: {7, 12}}, nil
	})
	code, out, errs := runIt(append([]string{"sheets"}, args...)...)
	if code != exitOK || !strings.Contains(out, "Sheet 1 is refused, "+filepath.Base(files[0])+", as invalid: sample-check finds it breaks a rule of sections 11 and 12, at lines 7, 12.") ||
		!strings.Contains(out, "40 of the plan's 40 recorded, 39 of them passing the check.") {
		t.Errorf("sheets = %d %s%s", code, out, errs)
	}
	code, out, errs = runMeasure("yes\n", append([]string{"-kind", "reading", "-part", "all", "-stub", "-repeats", "1", "-log", filepath.Join(filepath.Dir(dir), "runs", "r.jsonl")}, args...)...)
	if code != exitOK || !strings.Contains(out, "Refused, and nothing of them goes to AI: lines sheet 1 (invalid: sample-check finds it breaks a rule of sections 11 and 12, at lines 7, 12).") ||
		!strings.Contains(out, "All the sheets: 39 sheets of 1 set") {
		t.Errorf("run = %d %s%s", code, out, errs)
	}

	for _, kind := range []string{"hint", "judge"} {
		code, out, errs = runMeasure("yes\n", append([]string{"-kind", kind, "-part", "all", "-stub", "-repeats", "1", "-log", filepath.Join(filepath.Dir(dir), "runs", kind+".jsonl")}, args...)...)
		if code != exitOK || !strings.Contains(out, "Refused, and nothing of them goes to AI: lines sheet 1 (invalid:") {
			t.Errorf("run -kind %s = %d %s%s", kind, code, out, errs)
		}
	}

	checking(t, func([]string) (map[string][]int, error) { return nil, errors.New("buf fails") })
	if code, _, errs := runIt(append([]string{"sheets"}, args...)...); code != exitFail || !strings.Contains(errs, "buf fails") {
		t.Errorf("sheets with a check that cannot run = %d %s", code, errs)
	}
}

// draw does not ask the check: it reads the directory only to see which sheets it holds.
func TestDrawAsksNoCheck(t *testing.T) {
	checking(t, func([]string) (map[string][]int, error) {
		t.Error("draw asked the check")
		return nil, nil
	})
	dir := filepath.Join(t.TempDir(), "lines")
	for range 2 {
		if code := runDraw([]string{"-dir", dir, "-bank", measurementBank, "-plan", measurementPlan}, &strings.Builder{}, &strings.Builder{}, time.Now); code != exitOK {
			t.Fatalf("draw = %d", code)
		}
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatal(err)
	}
}
