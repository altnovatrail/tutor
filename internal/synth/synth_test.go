package synth

import (
	"slices"
	"testing"

	"github.com/altnovatrail/tutor/internal/sample"
)

func TestMirroredDigitsAreMirrored(t *testing.T) {
	plain := Sample(sample.ScenarioSimilarDigits)
	mirrored := Sample(sample.ScenarioMirroredDigits)
	if mirrored.Writer != sample.WriterAuthorAsChild {
		t.Errorf("the mirrored sheet is written by %q, want %q", mirrored.Writer, sample.WriterAuthorAsChild)
	}
	if len(plain.Lines) != len(mirrored.Lines) || plain.Lines[0].Text != mirrored.Lines[0].Text {
		t.Fatal("the two sheets should write the same lines")
	}
	// The 7 of "47": its top stroke runs left to right when plain and right to left mirrored.
	seven := func(s *sample.Sample) []sample.Point {
		return strokeByID(s, s.Lines[0].Strokes[1]).Points
	}
	p, m := seven(plain), seven(mirrored)
	if !(p[5].X > p[0].X && m[5].X < m[0].X) {
		t.Errorf("the 7 is not mirrored: plain goes from x=%v to x=%v, mirrored from x=%v to x=%v", p[0].X, p[5].X, m[0].X, m[5].X)
	}
}

func TestInjectionIsInTheCorner(t *testing.T) {
	s := Sample(sample.ScenarioInjection)
	for _, id := range s.Injection.Strokes {
		for _, pt := range strokeByID(s, id).Points {
			if pt.Y < sheetHeight/2 {
				t.Fatalf("an injection stroke reaches y=%v, above the lower half of the sheet", pt.Y)
			}
		}
	}
}

// The writing fits the sheet: the pen's line, 1.5 units either side of a point, stays on it, and no
// point is pressed to an edge, where the writer would put a point that runs off the sheet.
func TestWritingFitsTheSheet(t *testing.T) {
	for _, s := range Samples() {
		for _, st := range s.Strokes {
			for _, pt := range st.Points {
				if pt.X < 1.5 || pt.X > sheetWidth-1.5 || pt.Y < 1.5 || pt.Y > sheetHeight-1.5 {
					t.Errorf("%s: stroke %s has a point at (%v, %v), at the edge of the %d × %d sheet", s.Scenario, st.ID, pt.X, pt.Y, sheetWidth, sheetHeight)
				}
			}
		}
	}
}

func TestStrokesFollowTheClock(t *testing.T) {
	for _, s := range Samples() {
		var last int64
		for _, st := range s.Strokes {
			if st.Points[0].TMs <= last {
				t.Fatalf("%s: stroke %s starts at %d ms, not after %d ms", s.Scenario, st.ID, st.Points[0].TMs, last)
			}
			last = st.Points[len(st.Points)-1].TMs
		}
	}
}

func TestUnknownScenarioPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Sample of an unknown scenario did not panic")
		}
	}()
	Sample("no-such-scenario")
}

func strokeByID(s *sample.Sample, id string) sample.Stroke {
	i := slices.IndexFunc(s.Strokes, func(st sample.Stroke) bool { return st.ID == id })
	return s.Strokes[i]
}

// The plan of the synthetic samples passes the plan's check with the bank, one sheet to each
// scenario a plan can hold, and each sample is the markup its plan sheet plans.
func TestPlan(t *testing.T) {
	plan, samples := Plan()
	if err := plan.Check(); err != nil {
		t.Fatal(err)
	}
	if err := plan.CheckWithBank(Bank()); err != nil {
		t.Fatal(err)
	}
	var scenarios []sample.Scenario
	for i, sh := range plan.Sheets {
		scenarios = append(scenarios, sh.Scenario)
		if s := samples[i]; s.Scenario != sh.Scenario || s.Task != sh.Task || len(s.Lines) != len(sh.Lines) {
			t.Errorf("sheet %d: the sample of %s, task %s, %d lines; want its plan sheet's", sh.Number, s.Scenario, s.Task, len(s.Lines))
		}
	}
	want := []sample.Scenario{sample.ScenarioCorrect, sample.ScenarioError, sample.ScenarioSimilarDigits, sample.ScenarioAnswerNotLast,
		sample.ScenarioMirroredDigits, sample.ScenarioErasureFixed, sample.ScenarioErasureBroken}
	if !slices.Equal(scenarios, want) {
		t.Errorf("scenarios = %v, want %v", scenarios, want)
	}
}
