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
