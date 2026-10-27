// Package lines finds the lines of a sheet: the strokes that stand at one height, told apart by
// the gaps between them down the sheet. The record mode of the board proposes a sheet's lines with
// it and finds on which line an erasure took its strokes (R25), and the journal of the writing
// process groups with it all the strokes of a sheet, the erased ones too (internal/journal).
//
// It also finds the groups of strokes a device finds, by the eval protocol's rule of groups
// (R24 canvas; R30 in docs/decisions.md), and where an erasure stood on the plane on a sheet
// whose markup is made of parts. A free-form solution — a number line, a drawing, a note at the
// side — has no lines down the sheet, so that is what its sheets go by.
package lines

import (
	"cmp"
	"slices"

	"github.com/altnovatrail/tutor/internal/sample"
)

// Gap is the least step, in the sheet's logical units, from the middle of one stroke to the
// middle of the next one down the sheet that starts a new line. Group raises it to half the
// height of a typical stroke of the sheet, so that it follows the size of the handwriting. It is
// a guess made before any sheet was written, and the journal of the writing process may tune it
// (R25); the markup of a sheet already saved does not change.
const Gap = 12

// Line is one line that Group finds: its strokes, in the order they were written, and the band of
// the sheet it takes, from its highest point down to its lowest.
type Line struct {
	Strokes     []string
	Top, Bottom float64
}

// Group finds the lines of a sheet's strokes, top to bottom. A stroke stands at the middle of its
// band, from its highest point down to its lowest. Down the sheet, a step between the middles of
// two strokes of at least gap, or of half the median height of the sheet's strokes if that is
// more, starts a new line. The middles, not the ends, decide: a tall digit or a long tail of a
// letter reaches towards the next line, but its middle stays on its own. A line of small strokes
// near a line of the writing joins it, as below. A stroke without points
// is on no line. The strokes come in the order they were written, as a sample keeps them, and each
// line keeps that order; a line's band runs over all of its strokes, so two bands may overlap.
func Group(strokes []sample.Stroke, gap float64) []Line {
	type band struct {
		order               int
		id                  string
		top, bottom, middle float64
	}
	bands := make([]band, 0, len(strokes))
	var heights []float64
	for i, s := range strokes {
		if top, bottom, ok := Band(s); ok {
			bands = append(bands, band{order: i, id: s.ID, top: top, bottom: bottom, middle: (top + bottom) / 2})
			if bottom > top {
				heights = append(heights, bottom-top)
			}
		}
	}
	if len(heights) > 0 {
		slices.Sort(heights)
		gap = max(gap, heights[len(heights)/2]/2)
	}
	slices.SortStableFunc(bands, func(a, b band) int { return cmp.Compare(a.middle, b.middle) })

	var lines []Line
	var members [][]band
	for i, b := range bands {
		if n := len(lines); n > 0 && b.middle-bands[i-1].middle < gap {
			lines[n-1].Top = min(lines[n-1].Top, b.top)
			lines[n-1].Bottom = max(lines[n-1].Bottom, b.bottom)
			members[n-1] = append(members[n-1], b)
			continue
		}
		lines = append(lines, Line{Top: b.top, Bottom: b.bottom})
		members = append(members, []band{b})
	}
	// A line of small strokes only, such as a slip of the pen, a dot or an exponent, is no line of
	// its own when a line of the writing stands near it: it goes to the nearest such line, the
	// higher of two as near. Small is lower than three quarters of a typical stroke, near is less
	// than a typical stroke away; a stray stroke farther off stays a line of its own.
	small := func(l Line) bool { return l.Bottom-l.Top < 1.5*gap }
	for i := 0; i < len(lines); {
		to := -1
		if small(lines[i]) {
			for j := range lines {
				if j != i && !small(lines[j]) && apart(lines[i], lines[j]) < 2*gap && (to < 0 || apart(lines[i], lines[j]) < apart(lines[i], lines[to])) {
					to = j
				}
			}
		}
		if to < 0 {
			i++
			continue
		}
		lines[to].Top = min(lines[to].Top, lines[i].Top)
		lines[to].Bottom = max(lines[to].Bottom, lines[i].Bottom)
		members[to] = append(members[to], members[i]...)
		lines = slices.Delete(lines, i, i+1)
		members = slices.Delete(members, i, i+1)
	}
	for i, m := range members {
		slices.SortFunc(m, func(a, b band) int { return cmp.Compare(a.order, b.order) })
		for _, b := range m {
			lines[i].Strokes = append(lines[i].Strokes, b.id)
		}
	}
	return lines
}

// apart is how far down the sheet two lines stand from each other, 0 when their bands overlap.
func apart(a, b Line) float64 { return max(0, a.Top-b.Bottom, b.Top-a.Bottom) }

// Band returns the band of the sheet strokes take together, from their highest point down to their
// lowest; ok is false when they have no points.
func Band(strokes ...sample.Stroke) (top, bottom float64, ok bool) {
	for _, s := range strokes {
		for _, p := range s.Points {
			if !ok {
				top, bottom, ok = p.Y, p.Y, true
				continue
			}
			top, bottom = min(top, p.Y), max(bottom, p.Y)
		}
	}
	return top, bottom, ok
}

// On returns the index of the line a band of the sheet stands on: the line it overlaps the most, or
// else the nearest one, less than gap away; -1 when it stands on none. Of two lines it overlaps as
// much, the higher one is taken.
func On(top, bottom float64, ls []Line, gap float64) int {
	best, most := -1, -gap
	for i, l := range ls {
		// overlap is how much of the band the line shares, or, below 0, how far apart the two stand.
		if overlap := min(bottom, l.Bottom) - max(top, l.Top); overlap > most {
			best, most = i, overlap
		}
	}
	return best
}
