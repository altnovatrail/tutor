package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/altnovatrail/tutor/internal/hint"
)

// A hint goes over three sets in one run, on the stub, and asks only the sheets with a mistake: the
// estimate names them by set, with the rungs, the languages and the tutors; the log beside the sets
// is named by the kind and the sets; and every call boxes the first error of its sheet, with a hint
// the learner reads. The report writes its three files beside the log, once, and a hint takes no
// -reads.
func TestHintOverSets(t *testing.T) {
	parent, sets := drawnSets(t, drawnSet{"lines", measurementBank, measurementPlan}, drawnSet{"free", freeBank, freePlan}, drawnSet{"near", nearBank, nearPlan})
	code, out, errs := runMeasure("yes\n", "-kind", "hint", "-part", "dev", "-stub", "-repeats", "1", "-set", sets[0], "-set", sets[1], "-set", sets[2])
	if code != exitOK {
		t.Fatalf("the run ends %d: %s%s", code, out, errs)
	}
	want := "The dev part: %d sheets with a mistake of 30 in 3 sets (lines %d of 10, free %d of 10, near %d of 10), each at 3 rungs in 2 languages, by opus-high, haiku-low, haiku-low-no-thinking in 1 repeat, in version 1 of the hint's prompt.\n"
	log := filepath.Join(parent, "runs", "20261026-123000-000-hint-lines+free+near-dev.jsonl")
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	l, err := hint.ReadLog(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]int{}
	first := map[string]int{}
	for _, sh := range l.Sheets {
		by[sh.Set]++
		first[fmt.Sprint(sh.Set, "/", sh.Sheet)] = sh.FirstError
	}
	if !strings.HasPrefix(out, fmt.Sprintf(want, len(l.Sheets), by["lines"], by["free"], by["near"])) || len(l.Sheets) == 0 {
		t.Errorf("the run says\n%s", out)
	}
	if strings.Join(l.Run.Sets, "+") != "lines+free+near" || len(l.Calls) != len(l.Sheets)*3*2*3 || l.End == nil || l.End.Stopped != "" {
		t.Errorf("the log tells %+v, %d sheets and %d calls", l.Run, len(l.Sheets), len(l.Calls))
	}
	for _, c := range l.Calls {
		if c.Hit != first[fmt.Sprint(c.Set, "/", c.Sheet)] || c.Error != "" || c.Fallback != "" || c.Shown != c.Hint {
			t.Errorf("the call %+v hits %d: %s", c.Call, c.Hit, c.Error)
		}
	}
	base := strings.TrimSuffix(log, ".jsonl")
	code, out, errs = runIt("report", log)
	if code != exitOK || !strings.Contains(out, "opus-high: leaked before the check 0 of") || !strings.HasSuffix(out, "The hints to read: "+base+".hints.md\n") {
		t.Errorf("the report of a hint: %d %s%s", code, out, errs)
	}
	for _, ext := range []string{".md", ".csv", ".hints.md"} {
		if _, err := os.Stat(base + ext); err != nil {
			t.Errorf("the report wrote no %s: %v", ext, err)
		}
	}
	if err := os.Remove(base + ".md"); err != nil {
		t.Fatal(err)
	}
	if code, _, errs := runIt("report", log); code != exitFail || !strings.Contains(errs, ".csv exists") {
		t.Errorf("a report over one that is there: %d %s", code, errs)
	}
	if code, _, errs := runMeasure("yes\n", "-kind", "hint", "-part", "dev", "-stub", "-reads", "keys", "-set", sets[0]); code != exitUsage || !strings.Contains(errs, "-reads is for -kind reading") {
		t.Errorf("a hint with -reads: %d %s", code, errs)
	}
}

// The author's three sheets are all correct, so a hint has nothing to ask of them: the run is
// refused before the question.
func TestHintWithoutMistakes(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "free")
	args := []string{"-dir", dir, "-bank", freeBank, "-plan", authorPlan}
	var out, errs bytes.Buffer
	if code := runDraw(append([]string{"-seed", "1"}, args...), &out, &errs, time.Now); code != exitOK {
		t.Fatalf("draw = %d %s", code, errs.String())
	}
	code, stdout, stderr := runMeasure("yes\n", append([]string{"-kind", "hint", "-part", "all", "-stub"}, args...)...)
	if code != exitFail || stdout != "" || !strings.Contains(stderr, "All the sheets: no sheet with a mistake (free 0 of 3)") {
		t.Errorf("a hint on correct sheets: %d %q %q", code, stdout, stderr)
	}
}
