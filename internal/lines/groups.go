package lines

import (
	"cmp"
	"math"
	"slices"

	"github.com/altnovatrail/tutor/internal/sample"
)

// The rule of stroke groups of the eval protocol, R24 canvas as it is (Д36 in spec's RUN.md; R30
// in docs/decisions.md): two strokes are neighbours when no more than GroupPause milliseconds lie
// between them, from the end of the earlier to the start of the later, and the boxes of their
// points come within GroupAcross units of each other across the sheet and within GroupDown units
// down it. Each bound is included. A group is the strokes a chain of neighbours joins.
const (
	GroupPause  = 2000 // milliseconds
	GroupAcross = 50   // sheet units
	GroupDown   = 50   // sheet units
)

// StrokeGroup is one group of the strokes a sheet shows, as a device groups them (concepts 15 spec).
// A part of a sheet's plan is one group or several: a drawing redrawn after a pause is two.
type StrokeGroup struct {
	// Label is the group's label by place, as a device gives it: A to Z, then AA, AB and on, by
	// the top of its rectangle, then its left, its bottom and its right, then its earliest start.
	Label string
	// Strokes are its strokes, in the order they were written.
	Strokes []string
	// Erased are the erased strokes that would be neighbours of its strokes, in the order they
	// were written. An erased stroke is of every group that holds one: of two groups, or of none.
	Erased []string
	// Top, Left, Bottom and Right are its rectangle around the points of its strokes, in sheet
	// units. Erased strokes change neither the rectangle nor the label.
	Top, Left, Bottom, Right float64
}

// piece is a stroke as the rule sees it: its place in the strokes given, the box of its points in
// whole tenths of a unit and its span from its earliest point to its latest, in milliseconds.
type piece struct {
	at                     int
	minX, minY, maxX, maxY int64
	start, end             int64
}

// Groups finds the groups of the strokes a sheet shows: the strokes no erasure took, in the order
// their first strokes were written. It counts as canvas counts, so that a device and the runner
// find the same groups to the bit: places in whole tenths of a unit, rounded half up, and times in
// whole milliseconds, and then it only subtracts and compares. A stroke without points is of no
// group.
func Groups(strokes []sample.Stroke, erasures []sample.Erasure) []StrokeGroup {
	erased := map[string]bool{}
	for _, e := range erasures {
		for _, id := range e.Strokes {
			erased[id] = true
		}
	}
	var shown, gone []piece
	for i, st := range strokes {
		if p, ok := pieceOf(st, i); ok && erased[st.ID] {
			gone = append(gone, p)
		} else if ok {
			shown = append(shown, p)
		}
	}

	// A pair's pause only grows as the later stroke starts later, so each stroke looks forward in
	// the order of the starts until the pause is past the rule's.
	byStart := make([]int, len(shown))
	for i := range byStart {
		byStart[i] = i
	}
	slices.SortStableFunc(byStart, func(a, b int) int { return cmp.Compare(shown[a].start, shown[b].start) })
	root := make([]int, len(shown))
	for i := range root {
		root[i] = i
	}
	find := func(i int) int {
		for root[i] != i {
			root[i] = root[root[i]]
			i = root[i]
		}
		return i
	}
	for k, i := range byStart {
		for _, j := range byStart[k+1:] {
			if shown[j].start-shown[i].end > GroupPause {
				break
			}
			if neighbours(shown[i], shown[j]) {
				root[find(i)] = find(j)
			}
		}
	}

	// The members of each group, in the order of writing: shown is in that order already.
	var members [][]piece
	group := map[int]int{}
	for i, p := range shown {
		r := find(i)
		n, ok := group[r]
		if !ok {
			n = len(members)
			group[r] = n
			members = append(members, nil)
		}
		members[n] = append(members[n], p)
	}

	out := make([]StrokeGroup, len(members))
	places := make([]place, len(members))
	for n, m := range members {
		g := StrokeGroup{Strokes: make([]string, len(m))}
		for k, p := range m {
			g.Strokes[k] = strokes[p.at].ID
		}
		g.Top, g.Left, g.Bottom, g.Right = rectOf(strokes, m)
		for _, e := range gone {
			if slices.ContainsFunc(m, func(p piece) bool { return neighbours(e, p) }) {
				g.Erased = append(g.Erased, strokes[e.at].ID)
			}
		}
		out[n] = g
		places[n] = placeOf(m)
	}
	order := make([]int, len(members))
	for n := range order {
		order[n] = n
	}
	slices.SortFunc(order, func(a, b int) int { return places[a].compare(places[b]) })
	for k, n := range order {
		out[n].Label = labelOf(k)
	}
	return out
}

// Neighbours reports whether two strokes are neighbours by the rule of groups. A stroke without
// points is no stroke's neighbour.
func Neighbours(a, b sample.Stroke) bool {
	pa, okA := pieceOf(a, 0)
	pb, okB := pieceOf(b, 0)
	return okA && okB && neighbours(pa, pb)
}

func neighbours(a, b piece) bool {
	return gap(a.minX, a.maxX, b.minX, b.maxX) <= 10*GroupAcross &&
		gap(a.minY, a.maxY, b.minY, b.maxY) <= 10*GroupDown &&
		gap(a.start, a.end, b.start, b.end) <= GroupPause
}

// gap is how far apart two spans are: 0 when they overlap or touch.
func gap(low, high, otherLow, otherHigh int64) int64 {
	return max(0, low-otherHigh, otherLow-high)
}

// pieceOf is a stroke as the rule sees it, from one pass over its points; ok is false when it has
// none. Rounding keeps the order of numbers, so the rounded least and greatest are the least and
// greatest rounded.
func pieceOf(st sample.Stroke, at int) (p piece, ok bool) {
	for i, pt := range st.Points {
		x, y := Tenths(pt.X), Tenths(pt.Y)
		if i == 0 {
			p = piece{at: at, minX: x, minY: y, maxX: x, maxY: y, start: pt.TMs, end: pt.TMs}
			continue
		}
		p.minX, p.minY, p.maxX, p.maxY = min(p.minX, x), min(p.minY, y), max(p.maxX, x), max(p.maxY, y)
		p.start, p.end = min(p.start, pt.TMs), max(p.end, pt.TMs)
	}
	return p, len(st.Points) > 0
}

// Tenths is a place in whole tenths of a unit, as a sample file keeps them, rounded half up: 100.05
// units are 1000.5 tenths, which make 1001. The product is rounded on its own, float64(…), so that
// it fuses with nothing (see internal/synth). The groups and the crop (internal/render) count in
// them, as canvas's do (R24 and R25 canvas).
func Tenths(v float64) int64 { return roundHalfUp(float64(v * 10)) }

// roundHalfUp rounds to the nearest whole number and a half up, exactly as JavaScript's Math.round,
// by which canvas counts, for every finite number. math.Floor(x + 0.5) would not: at
// 0.49999999999999994 the sum rounds up to 1.
func roundHalfUp(x float64) int64 {
	r := math.Floor(x)
	if x-r >= 0.5 {
		r++
	}
	return int64(r)
}

// rectOf is the rectangle around the points of a group's strokes, in sheet units.
func rectOf(strokes []sample.Stroke, m []piece) (top, left, bottom, right float64) {
	first := true
	for _, p := range m {
		for _, pt := range strokes[p.at].Points {
			if first {
				top, left, bottom, right, first = pt.Y, pt.X, pt.Y, pt.X, false
				continue
			}
			top, left, bottom, right = min(top, pt.Y), min(left, pt.X), max(bottom, pt.Y), max(right, pt.X)
		}
	}
	return top, left, bottom, right
}

// place is where a group stands, as its label takes it: its box in tenths, its earliest start and
// the place of its first stroke in the strokes given.
type place struct {
	top, left, bottom, right int64
	start                    int64
	first                    int
}

func placeOf(m []piece) place {
	pl := place{top: m[0].minY, left: m[0].minX, bottom: m[0].maxY, right: m[0].maxX, start: m[0].start, first: m[0].at}
	for _, p := range m[1:] {
		pl.top, pl.left, pl.bottom, pl.right = min(pl.top, p.minY), min(pl.left, p.minX), max(pl.bottom, p.maxY), max(pl.right, p.maxX)
		pl.start = min(pl.start, p.start)
	}
	return pl
}

// compare orders places from the top down, then from the left; boxes alike in both go by their
// bottom, their right and then by which was begun first. canvas settles the last tie by the
// smaller ID, and its IDs follow the order of writing, as the place of the first stroke does here:
// a sample's IDs are strings, and "s-10" may not sort after "s-9".
func (a place) compare(b place) int {
	return cmp.Or(
		cmp.Compare(a.top, b.top), cmp.Compare(a.left, b.left),
		cmp.Compare(a.bottom, b.bottom), cmp.Compare(a.right, b.right),
		cmp.Compare(a.start, b.start), cmp.Compare(a.first, b.first),
	)
}

// labelOf is the label of the group in the place n, from 0: A to Z, then AA to AZ, BA to ZZ, AAA
// and on — letters in base 26 without a zero, as the columns of a spreadsheet.
func labelOf(n int) string {
	var label []byte
	for rest := n + 1; rest > 0; rest = (rest - 1) / 26 {
		label = append(label, byte('A'+(rest-1)%26))
	}
	slices.Reverse(label)
	return string(label)
}
