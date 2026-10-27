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

// ErasurePart is the place of an erasure on the plane, for a sheet whose markup is made of parts,
// as a plan of version 1 makes it (T56): the number of the part most of the erasure's strokes stood
// on, the earlier in the markup of two that hold as many, or 0 when none of them stood on a part.
// An erased stroke stood on the part that holds the stroke on the sheet nearest to it, when their
// boxes come within the rule of groups' place, GroupAcross across and GroupDown down. Near is the
// larger of the two gaps between the boxes, and of two parts as near, the earlier in the markup
// takes it: an axis, an arc, a bar or a brace touches more than one part. The rule's time does not
// count, as a part written again after a pause has no neighbours in time, and strokes of no part
// do not count either.
func ErasurePart(s *sample.Sample, e sample.Erasure) int {
	pieces := make(map[string]piece, len(s.Strokes))
	for i, st := range s.Strokes {
		if p, ok := pieceOf(st, i); ok {
			pieces[st.ID] = p
		}
	}
	stood := make([]int, len(s.Lines)+1) // by the number of the part, from 1
	for _, id := range e.Strokes {
		erased, ok := pieces[id]
		if !ok {
			continue
		}
		part, nearest := 0, int64(0)
		for n, l := range s.Lines {
			for _, sid := range l.Strokes {
				p, ok := pieces[sid]
				if !ok {
					continue
				}
				across, down := gap(erased.minX, erased.maxX, p.minX, p.maxX), gap(erased.minY, erased.maxY, p.minY, p.maxY)
				if across > 10*GroupAcross || down > 10*GroupDown {
					continue
				}
				if d := max(across, down); part == 0 || d < nearest {
					part, nearest = n+1, d
				}
			}
		}
		if part > 0 {
			stood[part]++
		}
	}
	most := 0
	for n := 1; n < len(stood); n++ {
		if stood[n] > 0 && (most == 0 || stood[n] > stood[most]) {
			most = n
		}
	}
	return most
}
