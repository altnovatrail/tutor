package lines_test

import (
	"reflect"
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
		{"strokes less than the gap apart", []sample.Stroke{stroke("a", 100, 130), stroke("b", 141.9, 170)}, [][]string{{"a", "b"}}},
		{"strokes the gap apart", []sample.Stroke{stroke("a", 100, 130), stroke("b", 142, 170)}, [][]string{{"a"}, {"b"}}},
		{"overlapping strokes", []sample.Stroke{stroke("a", 100, 130), stroke("b", 120, 150)}, [][]string{{"a", "b"}}},
		{"strokes joined through another", []sample.Stroke{stroke("a", 100, 130), stroke("c", 200, 230), stroke("b", 125, 205)}, [][]string{{"a", "c", "b"}}},
		{"lines top to bottom, written bottom first", []sample.Stroke{stroke("low", 300, 330), stroke("high", 100, 130)}, [][]string{{"high"}, {"low"}}},
		{"a line in the order of writing", []sample.Stroke{stroke("x", 100, 130), stroke("power", 90, 105)}, [][]string{{"x", "power"}}},
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
