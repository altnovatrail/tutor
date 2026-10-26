package lines

import (
	"cmp"
	"slices"

	"github.com/altnovatrail/tutor/internal/sample"
)

// Taken is what an erasure took from the lines of a sheet: how many of its strokes stood on each
// line, by the line's number, and the line most of them stood on, the higher of two that hold as
// many, or 0 when none of them stood on a line.
type Taken struct {
	From map[int]int
	Line int
}

// TakenFrom finds what each erasure took from lines that stand top to bottom, as Group gives them.
// A stroke stood on the line its band stands on (On), and a stroke of an ID the strokes do not hold
// stood on none.
func TakenFrom(byID map[string]sample.Stroke, erasures []sample.Erasure, found []Line) []Taken {
	out := make([]Taken, len(erasures))
	for i, e := range erasures {
		t := Taken{From: map[int]int{}}
		for _, id := range e.Strokes {
			if top, bottom, ok := Band(byID[id]); ok {
				if n := On(top, bottom, found, Gap) + 1; n > 0 {
					t.From[n]++
				}
			}
		}
		for n, k := range t.From {
			if k > t.From[t.Line] || (k == t.From[t.Line] && n < t.Line) {
				t.Line = n
			}
		}
		out[i] = t
	}
	return out
}

// ErasureLine is the number of the line of a sample's markup that most of an erasure's strokes
// stood on, by the bands of the lines, the higher on the sheet of two that hold as many, or 0 when
// none of them stood on a line. The markup's lines need not stand in order down the sheet.
func ErasureLine(s *sample.Sample, e sample.Erasure) int {
	byID := make(map[string]sample.Stroke, len(s.Strokes))
	for _, st := range s.Strokes {
		byID[st.ID] = st
	}
	// found holds the lines with a band; number gives each its number in the markup.
	var found []Line
	var number []int
	for i, l := range s.Lines {
		var strokes []sample.Stroke
		for _, id := range l.Strokes {
			strokes = append(strokes, byID[id])
		}
		if top, bottom, ok := Band(strokes...); ok {
			found = append(found, Line{Top: top, Bottom: bottom})
			number = append(number, i+1)
		}
	}
	order := make([]int, len(found))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int { return cmp.Compare(found[a].Top, found[b].Top) })
	sorted := make([]Line, len(found))
	for i, at := range order {
		sorted[i] = found[at]
	}
	if n := TakenFrom(byID, []sample.Erasure{e}, sorted)[0].Line; n > 0 {
		return number[order[n-1]]
	}
	return 0
}
