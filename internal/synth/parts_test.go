package synth_test

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/internal/lines"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/sheets"
	"github.com/altnovatrail/tutor/internal/synth"
)

// freeMeasurement reads the free-form bank and both of its plans: the 40 sheets code draws and the
// three the author writes, copies of three of them.
func freeMeasurement(t *testing.T) (*sample.Bank, []*sample.Plan) {
	t.Helper()
	read := func(path string, f func(*os.File) error) {
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = file.Close() }()
		if err := f(file); err != nil {
			t.Fatal(err)
		}
	}
	var bank *sample.Bank
	read("../../eval/free-bank.json", func(f *os.File) (err error) { bank, err = sample.ReadBank(f); return err })
	var plans []*sample.Plan
	for _, path := range []string{"../../eval/free-sheets.json", "../../eval/free-author.json"} {
		read(path, func(f *os.File) error {
			pl, err := sample.ReadPlan(f)
			plans = append(plans, pl)
			return err
		})
	}
	return bank, plans
}

// drawable returns the sheets of the plans that code draws already: those without drawings.
func drawable(plans []*sample.Plan) []*sample.PlanSheet {
	var out []*sample.PlanSheet
	for _, pl := range plans {
		for i := range pl.Sheets {
			if synth.Drawable(&pl.Sheets[i]) == nil {
				out = append(out, &pl.Sheets[i])
			}
		}
	}
	return out
}

// Every sheet of the free-form plans without a drawing — 13 of the code's and one of the author's —
// drawn with each seed, passes the check with the bank, has the plan's markup and is drawn by code.
func TestSheetOfPartsDrawsThePlan(t *testing.T) {
	bank, plans := freeMeasurement(t)
	todo := drawable(plans)
	if len(todo) != 14 {
		t.Fatalf("%d sheets without drawings, want 13 of the code's plan and one of the author's", len(todo))
	}
	for _, sh := range todo {
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
			notesAtTheSide(t, sh, s, seed)
		}
	}
}

// The groups a device would find hold the parts of the plan, by construction (R30 in
// docs/decisions.md): no group holds strokes of two parts, a part without its planned erasure is
// one group, and a part with it one or two, as the fix after a pause may be a group of its own. So
// this holds on a hundred seeds, not only on those of the test above.
func TestGroupsHoldTheParts(t *testing.T) {
	_, plans := freeMeasurement(t)
	for _, sh := range drawable(plans) {
		for seed := uint64(1); seed <= 100; seed++ {
			s := synth.Sheet(sh, seed)
			partOf := map[string]int{}
			for i, l := range s.Lines {
				for _, id := range l.Strokes {
					partOf[id] = i + 1
				}
			}
			groupsOf := make([]int, len(s.Lines)+1)
			for _, g := range lines.Groups(s.Strokes, s.Erasures) {
				held := map[int]bool{}
				for _, id := range g.Strokes {
					held[partOf[id]] = true
				}
				if len(held) != 1 || held[0] {
					t.Errorf("sheet %d, seed %d: group %s holds strokes of parts %v", sh.Number, seed, g.Label, held)
				}
				for n := range held {
					groupsOf[n]++
				}
			}
			for n := 1; n < len(groupsOf); n++ {
				most := 1
				if e := sh.Erasure; e != nil && e.Line == n {
					most = 2
				}
				if groupsOf[n] < 1 || groupsOf[n] > most {
					t.Errorf("sheet %d, seed %d: part %d is %d groups, want 1 to %d", sh.Number, seed, n, groupsOf[n], most)
				}
			}
		}
	}
}

// notesAtTheSide checks the layout: every stroke of a note at the side stands right of every stroke
// of the main column, and the parts of the main column stand one under another.
func notesAtTheSide(t *testing.T, sh *sample.PlanSheet, s *sample.Sample, seed uint64) {
	t.Helper()
	byID := map[string]sample.Stroke{}
	for _, st := range s.Strokes {
		byID[st.ID] = st
	}
	mainRight, sideLeft := 0.0, float64(s.Sheet.Width)
	lastBottom := -1.0
	for i, l := range s.Lines {
		for _, id := range l.Strokes {
			for _, p := range byID[id].Points {
				if sh.Parts[i].Place == sample.PlaceRight {
					sideLeft = min(sideLeft, p.X)
				} else {
					mainRight = max(mainRight, p.X)
				}
			}
		}
		if sh.Parts[i].Place == sample.PlaceRight {
			continue
		}
		var strokes []sample.Stroke
		for _, id := range l.Strokes {
			strokes = append(strokes, byID[id])
		}
		top, bottom, _ := lines.Band(strokes...)
		if top <= lastBottom-12 {
			t.Errorf("sheet %d, seed %d: part %d starts at %.1f, over the part above, which ends at %.1f", sh.Number, seed, i+1, top, lastBottom)
		}
		lastBottom = bottom
	}
	if mainRight >= sideLeft {
		t.Errorf("sheet %d, seed %d: the main column reaches %.1f, and the notes at the side begin at %.1f", sh.Number, seed, mainRight, sideLeft)
	}
}

// A sheet with a drawing is one code cannot draw yet, and the reason names the drawing.
func TestDrawableNamesTheDrawing(t *testing.T) {
	_, plans := freeMeasurement(t)
	sh := &plans[0].Sheets[0] // the grasshopper on a number line
	err := synth.Drawable(sh)
	if err == nil || !strings.Contains(err.Error(), "part 1 is a drawing, a number_line") {
		t.Errorf("Drawable = %v, want the number line of part 1 named", err)
	}
	defer func() {
		if r := recover(); r == nil {
			t.Error("Sheet drew a sheet with a drawing")
		}
	}()
	synth.Sheet(sh, 1)
}

// A seed draws the same sheet of parts each time, and another seed another sheet.
func TestSheetOfPartsIsTheSameEachTime(t *testing.T) {
	_, plans := freeMeasurement(t)
	sh := &plans[0].Sheets[1] // the album, with columns at the side
	if err := synth.Drawable(sh); err != nil {
		t.Fatalf("sheet 2 of the code's plan: %v", err)
	}
	a, b := synth.Sheet(sh, 5), synth.Sheet(sh, 5)
	if !reflect.DeepEqual(a, b) {
		t.Error("the same seed draws two different sheets")
	}
	if reflect.DeepEqual(a.Strokes, synth.Sheet(sh, 6).Strokes) {
		t.Error("two seeds draw the same strokes")
	}
}
