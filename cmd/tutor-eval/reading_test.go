package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/altnovatrail/tutor/internal/reading"
)

// drawnSet is a set of sheets a test draws: the name of its directory, its bank and its plan.
type drawnSet struct{ name, bank, plan string }

// drawnSets draws by code the sheets of the plan of each set, each in a directory of its own beside
// the others, and splits each, as a run over several sets takes them: by default the plan of E2 and
// the plan of free writing, as a reading over two sets takes them. It gives the value of -set of
// each.
func drawnSets(t *testing.T, of ...drawnSet) (string, []string) {
	t.Helper()
	if len(of) == 0 {
		of = []drawnSet{{"lines", measurementBank, measurementPlan}, {"free", freeBank, freePlan}}
	}
	parent := t.TempDir()
	var sets []string
	for _, s := range of {
		dir := filepath.Join(parent, s.name)
		args := []string{"-dir", dir, "-bank", s.bank, "-plan", s.plan}
		var out, errs bytes.Buffer
		if code := runDraw(append([]string{"-seed", "1"}, args...), &out, &errs, time.Now); code != exitOK {
			t.Fatalf("draw %s = %d %s", s.name, code, errs.String())
		}
		if code, _, errs := runIt(append([]string{"split", "-seed", "7"}, args...)...); code != exitOK {
			t.Fatalf("split %s = %d %s", s.name, code, errs)
		}
		sets = append(sets, strings.Join([]string{dir, s.bank, s.plan}, ","))
	}
	return parent, sets
}

// A reading goes over two sets in one run, on the stub: the estimate names the sets and the
// conditions, the log beside the sets is named by the kind and both sets, and every call comes to
// the verdict of its sheet's outcome with nothing read under the key. The report of the judge does
// not read it.
func TestReadingOverSets(t *testing.T) {
	parent, sets := drawnSets(t)
	code, out, errs := runMeasure("yes\n", "-kind", "reading", "-part", "dev", "-stub", "-repeats", "1", "-set", sets[0], "-set", sets[1])
	if code != exitOK {
		t.Fatalf("the run ends %d: %s%s", code, out, errs)
	}
	if !strings.HasPrefix(out, "The dev part: 20 sheets of 2 sets (lines 10, free 10), by opus-high once, haiku-low and haiku-low-no-thinking in 1 repeat, without a key and with the right key, and with the false keys on ") {
		t.Errorf("the run says\n%s", out)
	}
	log := filepath.Join(parent, "runs", "20261026-123000-000-reading-lines+free-dev.jsonl")
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	l, err := reading.ReadLog(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(l.Run.Sets, "+") != "lines+free" || len(l.Sheets) != 20 || l.End == nil || l.End.Stopped != "" {
		t.Errorf("the log tells %+v and %d sheets", l.Run, len(l.Sheets))
	}
	outcome := map[string]string{}
	for _, sh := range l.Sheets {
		outcome[fmt.Sprint(sh.Set, "/", sh.Sheet)] = string(sh.Outcome)
	}
	verdict := map[string]string{"correct": "correct", "error": "not_yet"}
	for _, c := range l.Calls {
		if c.Verdict != verdict[outcome[fmt.Sprint(c.Set, "/", c.Sheet)]] || c.UnderKey.Result || len(c.UnderKey.Parts) > 0 {
			t.Errorf("the call %+v is %s (%s), under %+v", c.Call, c.Verdict, c.Reason, c.UnderKey)
		}
	}
	code, out, errs = runIt("report", log)
	if code != exitOK || strings.Count(out, "the threshold holds here; only the test part decides") != 3 {
		t.Errorf("the report of a reading: %d %s%s", code, out, errs)
	}
	for _, ext := range []string{".md", ".csv"} {
		if _, err := os.Stat(strings.TrimSuffix(log, ".jsonl") + ext); err != nil {
			t.Errorf("the report of a reading: %v", err)
		}
	}
}

// A reading of the other heights and of the states alone, on the stub: Haiku reads them once and
// without a key, and Opus not at all, and the estimate says so; a state whose answer is not written
// yet has no result.
func TestReadingHeightsAndStates(t *testing.T) {
	parent, sets := drawnSets(t)
	code, out, errs := runMeasure("yes\n", "-kind", "reading", "-part", "dev", "-stub", "-reads", "heights,states", "-set", sets[1])
	if code != exitOK {
		t.Fatalf("the run ends %d: %s%s", code, out, errs)
	}
	if !strings.HasPrefix(out, "The dev part: 10 sheets of 1 set (free 10), once by haiku-low and haiku-low-no-thinking without a key, ") || !strings.Contains(out, " at other heights of digits and ") {
		t.Errorf("the run says\n%s", out)
	}
	data, err := os.ReadFile(filepath.Join(parent, "runs", "20261026-123000-000-reading-free-dev.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	l, err := reading.ReadLog(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(l.Run.Reads, ",") != "heights,states" || len(l.Calls) == 0 {
		t.Fatalf("the log tells %+v and %d calls", l.Run, len(l.Calls))
	}
	sheet := map[int]reading.SheetRecord{}
	for _, sh := range l.Sheets {
		sheet[sh.Sheet] = sh
	}
	for _, c := range l.Calls {
		if c.Reader == "opus-high" || c.Condition != reading.NoKey || c.Digit == 32 && c.State == 0 {
			t.Errorf("the call %+v", c.Call)
		}
		if c.State > 0 {
			sh := sheet[c.Sheet]
			st := sh.States[c.State-1]
			for i, p := range sh.Markup.Parts {
				if p.Answer && st.Parts[i] != "done" && c.Verdict != "unreadable" {
					t.Errorf("the state %+v before its answer is written: %s", c.Call, c.Verdict)
				}
			}
		}
	}
}

// The sets of a run are refused when -set is not DIR,BANK,PLAN, when it comes with -dir, when two
// sets share a name or do not lie in one folder, and when the judge is given more than one.
func TestSetsRefused(t *testing.T) {
	parent, sets := drawnSets(t)
	elsewhere := filepath.Join(t.TempDir(), "lines") + "," + measurementBank + "," + measurementPlan
	for _, c := range []struct {
		args []string
		code int
		says string
	}{
		{[]string{"-set", "a,b"}, exitUsage, `-set is "a,b": give DIR,BANK,PLAN`},
		{[]string{"-set", sets[0], "-dir", filepath.Join(parent, "lines")}, exitUsage, "in place of -dir, -bank and -plan"},
		{[]string{"-set", sets[0], "-set", sets[0]}, exitUsage, "two sets are named lines"},
		{[]string{"-set", sets[1], "-set", elsewhere}, exitUsage, "lie in one folder"},
		{[]string{"-kind", "judge", "-set", sets[0], "-set", sets[1]}, exitUsage, "the judge runs on one set of sheets, and 2 are given"},
		{[]string{"-reads", "keys,frames", "-set", sets[0]}, exitUsage, `-reads is "keys,frames"`},
		{[]string{"-kind", "judge", "-reads", "keys", "-set", sets[0]}, exitUsage, "-reads is for -kind reading or workbook, and the judge reads nothing of it"},
	} {
		args := append([]string{"-kind", "reading", "-part", "dev", "-stub"}, c.args...)
		if code, _, errs := runMeasure("yes\n", args...); code != c.code || !strings.Contains(errs, c.says) {
			t.Errorf("%v: ends %d, %s; want %d and %q", c.args, code, errs, c.code, c.says)
		}
	}
}
