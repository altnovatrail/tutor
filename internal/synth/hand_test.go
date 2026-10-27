package synth_test

import (
	"os"
	"reflect"
	"testing"

	"github.com/altnovatrail/tutor/internal/lines"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/sheets"
	"github.com/altnovatrail/tutor/internal/synth"
)

// measurement reads the measurement's bank and sheet plan.
func measurement(t *testing.T) (*sample.Bank, *sample.Plan) {
	t.Helper()
	bf, err := os.Open("../../eval/bank.json")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = bf.Close() }()
	bank, err := sample.ReadBank(bf)
	if err != nil {
		t.Fatal(err)
	}
	pf, err := os.Open("../../eval/sheets.json")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pf.Close() }()
	plan, err := sample.ReadPlan(pf)
	if err != nil {
		t.Fatal(err)
	}
	return bank, plan
}

// Every sheet of the measurement's plan, drawn, passes the check with the bank, has the plan's
// markup and is drawn by code; and the lines the board would find on it are the plan's lines.
func TestSheetDrawsThePlan(t *testing.T) {
	bank, plan := measurement(t)
	for i := range plan.Sheets {
		sh := &plan.Sheets[i]
		for _, seed := range []uint64{1, 2, 77} {
			s := synth.Sheet(sh, seed)
			if err := s.Check(); err != nil {
				t.Fatalf("sheet %d, seed %d: %v", sh.Number, seed, err)
			}
			if err := s.CheckWithBank(bank); err != nil {
				t.Fatalf("sheet %d, seed %d: %v", sh.Number, seed, err)
			}
			if diffs := sheets.Differences(sh, s); len(diffs) > 0 {
				t.Errorf("sheet %d, seed %d: off the plan: %+v", sh.Number, seed, diffs)
			}
			if s.Writer != sample.WriterSynthetic {
				t.Errorf("sheet %d: the writer is %q", sh.Number, s.Writer)
			}
			erased := map[string]bool{}
			for _, e := range s.Erasures {
				for _, id := range e.Strokes {
					erased[id] = true
				}
			}
			var onSheet []sample.Stroke
			for _, st := range s.Strokes {
				if !erased[st.ID] {
					onSheet = append(onSheet, st)
				}
			}
			var found, want [][]string
			for _, l := range lines.Group(onSheet, lines.Gap) {
				found = append(found, l.Strokes)
			}
			for _, l := range s.Lines {
				want = append(want, l.Strokes)
			}
			if !reflect.DeepEqual(found, want) {
				t.Errorf("sheet %d, seed %d: the board finds %d lines, want the plan's %d", sh.Number, seed, len(found), len(want))
			}
			groupsHoldLines(t, sh, s, seed)
		}
	}
}

// groupsHoldLines checks a drawn sheet against the yardstick canvas names for its groups (R24
// canvas): no group of strokes holds strokes of two lines of the markup, and a line is one group
// unless the planned erasure took a part of it, as the fix written after a pause is a group of
// its own. This holds on the seeds of the test, not by construction: on other seeds a colon before
// a space in a large hand parts a line, and a short line written within 2 s of the next joins it
// (R30 in docs/decisions.md).
func groupsHoldLines(t *testing.T, sh *sample.PlanSheet, s *sample.Sample, seed uint64) {
	t.Helper()
	lineOf := map[string]int{}
	for i, l := range s.Lines {
		for _, id := range l.Strokes {
			lineOf[id] = i + 1
		}
	}
	groupsOf := make([]int, len(s.Lines)+1) // by the number of the line, from 1
	for _, g := range lines.Groups(s.Strokes, s.Erasures) {
		held := map[int]bool{}
		for _, id := range g.Strokes {
			if n := lineOf[id]; n > 0 { // a stroke of no line is of none to count
				held[n] = true
			}
		}
		if len(held) > 1 {
			t.Errorf("sheet %d, seed %d: group %s holds strokes of %d lines", sh.Number, seed, g.Label, len(held))
		}
		for n := range held {
			groupsOf[n]++
		}
	}
	for n := 1; n < len(groupsOf); n++ {
		want, most := 1, 1
		if sh.Erasure != nil && sh.Erasure.PartOfLine && sh.Erasure.Line == n {
			most = 2 // the fix, written after a pause, may be a group of its own
		}
		if groupsOf[n] < want || groupsOf[n] > most {
			t.Errorf("sheet %d, seed %d: line %d is %d groups, want %d to %d", sh.Number, seed, n, groupsOf[n], want, most)
		}
	}
}

// A seed draws the same sheet each time, and another seed another sheet.
func TestSheetIsTheSameEachTime(t *testing.T) {
	_, plan := measurement(t)
	sh := &plan.Sheets[1]
	a, b := synth.Sheet(sh, 5), synth.Sheet(sh, 5)
	if !reflect.DeepEqual(a, b) {
		t.Error("the same seed draws two different sheets")
	}
	if reflect.DeepEqual(a.Strokes, synth.Sheet(sh, 6).Strokes) {
		t.Error("two seeds draw the same strokes")
	}
}
