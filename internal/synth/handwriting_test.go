package synth

import (
	"math"
	"slices"
	"testing"
)

// A capital stands on the baseline and is size high, a tail goes below it, the line runs to the
// right, and a dot is a stroke of two points.
func TestHandwriting(t *testing.T) {
	strokes, missing := Handwriting("H", 100, 200, 40)
	if len(missing) != 0 || len(strokes) != 3 {
		t.Fatalf("H is %v, missing %q", strokes, missing)
	}
	top, foot := math.Inf(1), math.Inf(-1)
	for _, s := range strokes {
		for _, p := range s {
			top, foot = min(top, p.Y), max(foot, p.Y)
			if p.X < 100 || p.X > 100+40 {
				t.Errorf("a point of H at %v is off its place", p)
			}
		}
	}
	if top != 160 || foot != 200 {
		t.Errorf("H runs from %v down to %v; want 160 to 200", top, foot)
	}
	if s, _ := Handwriting("у", 0, 100, 40); lowest(s) <= 100 {
		t.Errorf("the tail of у ends at %v, not below the line", lowest(s))
	}
	line, _ := Handwriting("x = 2.5", 0, 100, 30)
	for _, s := range line {
		if len(s) < 2 {
			t.Errorf("a stroke of %d points: %v", len(s), s)
		}
	}
	one, _ := Handwriting("1", 0, 100, 30)
	two, _ := Handwriting("1 1", 0, 100, 30)
	if two[len(two)-1][0].X <= one[0][0].X+20 {
		t.Errorf("the second 1 starts at %v, not to the right of the first", two[len(two)-1][0])
	}
}

// A character the hand does not write is named once, and its room is left; one that looks like a
// character it writes is written as that one.
func TestHandwritingMissing(t *testing.T) {
	strokes, missing := Handwriting("a☃b☃€", 0, 100, 30)
	if !slices.Equal(missing, []rune{'☃', '€'}) || len(strokes) != len(glyphOf('a'))+len(glyphOf('b')) {
		t.Errorf("missing %q, %d strokes", missing, len(strokes))
	}
	if _, missing := Handwriting("x⋅y‐1", 0, 100, 30); len(missing) != 0 || Writes('☃') || !Writes(' ') || !Writes('′') {
		t.Errorf("missing %q", missing)
	}
}

// What school mathematics writes, the Cyrillic and Latin letters and the superscripts and
// subscripts are all written, each within the room above and below a line.
func TestHandwritingCharacters(t *testing.T) {
	const all = "0123456789+-−=×·÷:/()[]|<>≤≥≠≈±%°∞√∫π^_*∠αΔ△□.,;!?'\"–—²³⁰¹⁴⁵⁶⁷⁸⁹⁺⁻ⁿ₀₁₂₃₄₅₆₇₈₉" +
		"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz" +
		"АБВГДЕЁЖЗИЙКЛМНОПРСТУФХЦЧШЩЪЫЬЭЮЯабвгдеёжзийклмнопрстуфхцчшщъыьэюя"
	for _, r := range all {
		strokes, missing := Handwriting(string(r), 0, 100, 40)
		if len(missing) != 0 || len(strokes) == 0 {
			t.Errorf("%q is not written", r)
			continue
		}
		for _, s := range strokes {
			for _, p := range s {
				if p.Y < 100-40*1.25 || p.Y > 100+40*0.4 || p.X < 0 || p.X > 40 {
					t.Errorf("%q has a point at %v, off its room", r, p)
				}
			}
		}
	}
}

func lowest(strokes [][]Point) float64 {
	y := math.Inf(-1)
	for _, s := range strokes {
		for _, p := range s {
			y = max(y, p.Y)
		}
	}
	return y
}
