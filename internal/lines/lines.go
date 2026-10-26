// Package lines finds the lines of a sheet: the strokes that stand at one height, told apart by
// the gaps between them down the sheet. The record mode of the board proposes a sheet's lines with
// it (R25), and the journal of the writing process is to group the strokes with it too.
package lines

import (
	"cmp"
	"slices"

	"github.com/altnovatrail/tutor/internal/sample"
)

// Gap is how far apart down the sheet, in its logical units, two strokes stand at least when they
// are on different lines. It is a guess made before any sheet was written: the journal of the
// writing process may tune it (R25), and the markup of a sheet already saved does not change.
const Gap = 12

// Line is one line that Group finds: its strokes, in the order they were written, and the band of
// the sheet it takes, from its highest point down to its lowest.
type Line struct {
	Strokes     []string
	Top, Bottom float64
}

// Group finds the lines of a sheet's strokes, top to bottom. A stroke takes the band from its
// highest point down to its lowest. Strokes whose bands overlap or stand less than gap apart are
// on one line, and so are strokes joined through others: a long tail of a letter can join two
// lines. A stroke without points takes no band and is on no line. The strokes come in the order
// they were written, as a sample keeps them, and each line keeps that order.
func Group(strokes []sample.Stroke, gap float64) []Line {
	type band struct {
		order       int
		id          string
		top, bottom float64
	}
	bands := make([]band, 0, len(strokes))
	for i, s := range strokes {
		if len(s.Points) == 0 {
			continue
		}
		b := band{order: i, id: s.ID, top: s.Points[0].Y, bottom: s.Points[0].Y}
		for _, p := range s.Points[1:] {
			b.top = min(b.top, p.Y)
			b.bottom = max(b.bottom, p.Y)
		}
		bands = append(bands, b)
	}
	slices.SortStableFunc(bands, func(a, b band) int { return cmp.Compare(a.top, b.top) })

	var lines []Line
	var members [][]band
	for _, b := range bands {
		if n := len(lines); n > 0 && b.top-lines[n-1].Bottom < gap {
			lines[n-1].Bottom = max(lines[n-1].Bottom, b.bottom)
			members[n-1] = append(members[n-1], b)
			continue
		}
		lines = append(lines, Line{Top: b.top, Bottom: b.bottom})
		members = append(members, []band{b})
	}
	for i, m := range members {
		slices.SortFunc(m, func(a, b band) int { return cmp.Compare(a.order, b.order) })
		for _, b := range m {
			lines[i].Strokes = append(lines[i].Strokes, b.id)
		}
	}
	return lines
}
