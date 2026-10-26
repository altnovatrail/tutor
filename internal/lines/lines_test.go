package lines_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/altnovatrail/tutor/internal/lines"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/synth"
)

// visible returns the strokes still on a sample's sheet, in the order they were written.
func visible(s *sample.Sample) []sample.Stroke {
	erased := map[string]bool{}
	for _, e := range s.Erasures {
		for _, id := range e.Strokes {
			erased[id] = true
		}
	}
	var out []sample.Stroke
	for _, st := range s.Strokes {
		if !erased[st.ID] {
			out = append(out, st)
		}
	}
	return out
}

// strokesOf returns the strokes of each line that Group found.
func strokesOf(found []lines.Line) [][]string {
	var out [][]string
	for _, l := range found {
		out = append(out, l.Strokes)
	}
	return out
}

// On every synthetic sample, the lines found among the strokes still on the sheet are the lines
// of its markup, and the inscription of an injection is a line of its own.
func TestGroupFindsTheMarkedLines(t *testing.T) {
	for _, s := range synth.Samples() {
		t.Run(string(s.Scenario), func(t *testing.T) {
			var want [][]string
			for _, l := range s.Lines {
				want = append(want, l.Strokes)
			}
			if s.Injection != nil {
				want = append(want, s.Injection.Strokes)
			}
			if got := strokesOf(lines.Group(visible(s), lines.Gap)); !reflect.DeepEqual(got, want) {
				t.Errorf("lines = %q, want those of the markup, %q", got, want)
			}
		})
	}
}

// stroke is a stroke of two points, from the height top down to bottom.
func stroke(id string, top, bottom float64) sample.Stroke {
	return sample.Stroke{ID: id, Points: []sample.Point{{X: 10, Y: top}, {X: 20, Y: bottom}}}
}

func TestGroup(t *testing.T) {
	tests := []struct {
		name    string
		strokes []sample.Stroke
		want    [][]string
	}{
		// Strokes 30 high: a new line starts at a step of 15 between middles.
		{"middles less than the step apart", []sample.Stroke{stroke("a", 100, 130), stroke("b", 114.9, 144.9)}, [][]string{{"a", "b"}}},
		{"middles the step apart", []sample.Stroke{stroke("a", 100, 130), stroke("b", 115, 145)}, [][]string{{"a"}, {"b"}}},
		{"the step follows the handwriting, never below the gap", []sample.Stroke{stroke("a", 100, 104), stroke("b", 111.9, 115.9), stroke("c", 124, 128)}, [][]string{{"a", "b"}, {"c"}}},
		{"tall digits reaching each other's line", []sample.Stroke{
			stroke("a1", 100, 130), stroke("a4", 95, 140), stroke("b1", 150, 180), stroke("b4", 138, 182),
		}, [][]string{{"a1", "a4"}, {"b1", "b4"}}},
		{"lines top to bottom, written bottom first", []sample.Stroke{stroke("low", 300, 330), stroke("high", 100, 130)}, [][]string{{"high"}, {"low"}}},
		{"an exponent on its line, in the order of writing", []sample.Stroke{stroke("x", 100, 130), stroke("power", 90, 105)}, [][]string{{"x", "power"}}},
		{"a slip between two lines goes to the nearer", []sample.Stroke{stroke("a", 100, 130), stroke("b", 200, 230), stroke("slip", 140, 146)}, [][]string{{"a", "slip"}, {"b"}}},
		{"a stray stroke far from the lines stays a line", []sample.Stroke{stroke("a", 100, 130), stroke("b", 200, 230), stroke("stray", 400, 404)}, [][]string{{"a"}, {"b"}, {"stray"}}},
		{"a stroke of one point", []sample.Stroke{{ID: "dot", Points: []sample.Point{{X: 5, Y: 115}}}, stroke("a", 100, 130)}, [][]string{{"dot", "a"}}},
		{"a stroke without points", []sample.Stroke{{ID: "none"}, stroke("a", 100, 130)}, [][]string{{"a"}}},
		{"no strokes", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := strokesOf(lines.Group(tt.strokes, lines.Gap)); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("lines = %q, want %q", got, tt.want)
			}
		})
	}
}

// A line's band runs from its highest point down to its lowest, over all of its strokes.
func TestGroupBands(t *testing.T) {
	found := lines.Group([]sample.Stroke{
		{ID: "a", Points: []sample.Point{{Y: 120}, {Y: 104}, {Y: 131}}},
		stroke("b", 110, 150),
		stroke("c", 300, 320),
	}, lines.Gap)
	want := []lines.Line{{Strokes: []string{"a", "b"}, Top: 104, Bottom: 150}, {Strokes: []string{"c"}, Top: 300, Bottom: 320}}
	if !reflect.DeepEqual(found, want) {
		t.Errorf("lines = %+v, want %+v", found, want)
	}
}

// Band spans every point of the strokes it is given, and none of no points.
func TestBand(t *testing.T) {
	if top, bottom, ok := lines.Band(stroke("a", 120, 140), sample.Stroke{ID: "none"}, stroke("b", 100, 130)); !ok || top != 100 || bottom != 140 {
		t.Errorf("Band = %v, %v, %v; want 100, 140, true", top, bottom, ok)
	}
	if _, _, ok := lines.Band(sample.Stroke{ID: "none"}); ok {
		t.Error("strokes without points have a band")
	}
}

func TestOn(t *testing.T) {
	found := []lines.Line{{Top: 100, Bottom: 130}, {Top: 200, Bottom: 230}}
	tests := []struct {
		name        string
		top, bottom float64
		want        int
	}{
		{"inside a line", 105, 120, 0},
		{"over two lines, more of the second", 125, 228, 1},
		{"as much of two lines", 115, 215, 0},
		{"just above a line, less than the gap", 80, 88.1, 0},
		{"the gap above a line", 80, 88, -1},
		{"between the lines, nearer the second", 160, 195, 1},
		{"far from every line", 400, 420, -1},
	}
	for _, tt := range tests {
		if got := lines.On(tt.top, tt.bottom, found, lines.Gap); got != tt.want {
			t.Errorf("%s: On(%v, %v) = %d, want %d", tt.name, tt.top, tt.bottom, got, tt.want)
		}
	}
	if got := lines.On(100, 120, nil, lines.Gap); got != -1 {
		t.Errorf("On with no lines = %d, want -1", got)
	}
}

// On every synthetic sample of an erasure scenario, the strokes of the planned erasure stand on the
// line written in their place, among the lines of the strokes still on the sheet.
func TestOnFindsThePlannedLine(t *testing.T) {
	checked := 0
	for _, s := range synth.Samples() {
		for _, e := range s.Erasures {
			if e.Planned == nil {
				continue
			}
			var erased []sample.Stroke
			for _, st := range s.Strokes {
				if slices.Contains(e.Strokes, st.ID) {
					erased = append(erased, st)
				}
			}
			top, bottom, _ := lines.Band(erased...)
			if got := lines.On(top, bottom, lines.Group(visible(s), lines.Gap), lines.Gap) + 1; got != e.Planned.Line {
				t.Errorf("%s: the planned erasure stands on line %d, want %d", s.Scenario, got, e.Planned.Line)
			}
			checked++
		}
	}
	if checked != 2 {
		t.Errorf("%d planned erasures checked, want those of erasure-fixed and erasure-broken", checked)
	}
}

// An erasure stands on the line most of its strokes stood on, stroke by stroke, the higher of two
// that hold as many.
func TestTakenFrom(t *testing.T) {
	found := []lines.Line{{Top: 40, Bottom: 70}, {Top: 130, Bottom: 160}}
	byID := map[string]sample.Stroke{}
	for _, st := range []sample.Stroke{stroke("a1", 50, 55), stroke("a2", 52, 58), stroke("b1", 140, 145), stroke("b2", 142, 150), stroke("off", 400, 410)} {
		byID[st.ID] = st
	}
	taken := lines.TakenFrom(byID, []sample.Erasure{
		{Strokes: []string{"a1", "b1", "b2"}},
		{Strokes: []string{"a1", "a2", "b1", "b2"}},
		{Strokes: []string{"off", "missing"}},
	}, found)
	if taken[0].Line != 2 || taken[0].From[1] != 1 || taken[0].From[2] != 2 {
		t.Errorf("taken[0] = %+v, want line 2, one stroke from line 1 and two from line 2", taken[0])
	}
	if taken[1].Line != 1 {
		t.Errorf("taken[1] = %+v, want line 1, the higher of two that hold as many", taken[1])
	}
	if taken[2].Line != 0 || len(taken[2].From) != 0 {
		t.Errorf("taken[2] = %+v, want no line", taken[2])
	}
}

// The line of an erasure in a sample's markup goes by the bands of its lines, whatever their order
// in the markup.
func TestErasureLine(t *testing.T) {
	s := &sample.Sample{
		Strokes: []sample.Stroke{stroke("low", 130, 160), stroke("high", 40, 70), stroke("between", 70, 130)},
		Lines:   []sample.Line{{Strokes: []string{"low"}}, {Strokes: []string{"high"}}},
	}
	if got := lines.ErasureLine(s, sample.Erasure{Strokes: []string{"between"}}); got != 2 {
		t.Errorf("ErasureLine = %d, want 2: the line higher on the sheet, second in the markup", got)
	}
	if got := lines.ErasureLine(s, sample.Erasure{Strokes: []string{"nowhere"}}); got != 0 {
		t.Errorf("ErasureLine of a stroke the sample does not hold = %d, want 0", got)
	}
}
