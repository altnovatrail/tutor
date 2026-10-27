package journal

import (
	"fmt"
	"maps"
	"slices"

	"github.com/altnovatrail/tutor/internal/lines"
	"github.com/altnovatrail/tutor/internal/sample"
)

// Part is one part of a journal of parts: a group of the strokes the sheet shows, as lines.Groups
// finds it, or erased writing. Erased writing is erased strokes that no group holds, grouped among
// themselves by the same rule. A line erased whole and written again after a pause is such: it has
// no neighbours on the sheet, so by the rule it is of no group (R30). An erased stroke that would
// be a neighbour of a stroke on the sheet is of that group, though, so a line erased whole just
// after a stroke beside it starts on that part and goes on as erased writing.
type Part struct {
	// Group is the group: its strokes, the erased strokes that would be their neighbours and its
	// rectangle. The strokes of erased writing are all erased.
	Group lines.StrokeGroup
	// Erased tells erased writing from a part the sheet shows.
	Erased bool
	// Number is its number in the text, from 1. Parts and erased writing are numbered apart, each
	// in the order it was first written.
	Number int
	// Place is where it stands on the sheet, in the journal's words: "top right".
	Place string
}

// OfParts builds the journal of parts of a sample that passes its check (sample.Check), for free
// writing (T63). Each stroke stands on one part:
//   - a stroke the sheet shows, on its group;
//   - an erased stroke, on the group it would be a neighbour of; of two, on the one written on
//     last if it is one of them, or else on the earlier in the order of lines.Groups;
//   - an erased stroke of no group, on its erased writing.
//
// The parts are numbered in the order they were first written, the strokes erased later counting
// as writing too.
func OfParts(s *sample.Sample) *Journal {
	groups := lines.Groups(s.Strokes, s.Erasures)
	on := map[string][]int{} // the groups each stroke may stand on, by their index
	for n, g := range groups {
		for _, id := range slices.Concat(g.Strokes, g.Erased) {
			on[id] = append(on[id], n)
		}
	}
	var loose []sample.Stroke // the erased strokes of no group
	for _, st := range s.Strokes {
		if len(st.Points) > 0 && len(on[st.ID]) == 0 {
			loose = append(loose, st)
		}
	}
	erased := lines.Groups(loose, nil)
	for n, g := range erased {
		for _, id := range g.Strokes {
			on[id] = []int{len(groups) + n}
		}
	}
	all := slices.Concat(groups, erased)

	j := &Journal{words: &partsWords}
	at := map[int]int{} // the place in Parts of each group taken
	lineOf := map[string]int{}
	parts, writing := 0, 0 // how many parts and how much erased writing are numbered
	last := -1
	for _, st := range s.Strokes {
		may := on[st.ID]
		if len(may) == 0 {
			continue // a stroke without points stands on nothing
		}
		n := may[0]
		if slices.Contains(may, last) {
			n = last
		}
		if _, ok := at[n]; !ok {
			p := Part{Group: all[n], Erased: n >= len(groups), Place: placeOf(all[n], s.Sheet)}
			if p.Erased {
				writing++
				p.Number = writing
			} else {
				parts++
				p.Number = parts
			}
			at[n] = len(j.Parts)
			j.Parts = append(j.Parts, p)
		}
		lineOf[st.ID] = at[n] + 1
		last = n
	}
	j.walk(s, lineOf)
	return j
}

// The places of a sheet, in its thirds from the top down and from the left.
var places = [3][3]string{
	{"top left", "top centre", "top right"},
	{"middle left", "centre", "middle right"},
	{"bottom left", "bottom centre", "bottom right"},
}

// placeOf is where a group stands on a sheet: the third of its height and the third of its width
// that the middle of its rectangle stands in. The place holds no coordinates, which the journal
// does not tell (R11).
func placeOf(g lines.StrokeGroup, sh sample.Sheet) string {
	return places[third(g.Top+g.Bottom, sh.Height)][third(g.Left+g.Right, sh.Width)]
}

// third is the third of a side of the sheet a middle stands in, from 0, given twice the middle, so
// that it divides nothing. A middle on a border between two thirds stands in the later.
func third(twice, side float64) int {
	switch {
	case 3*twice < 2*side:
		return 0
	case 3*twice < 4*side:
		return 1
	}
	return 2
}

// The words of a journal of parts that are not those of a journal of lines: its first line, the
// names of its parts, the phrases of the steps of writing and the strokes an erasure took. The
// rest it shares with a journal of lines. Each step of writing and each erasure names a part,
// partName or erasedName, and the step that first writes on one says its place.
const (
	partsHeader = "How the sheet was written, step by step. A part is a group of strokes written close together in place and time. Parts are numbered in the order they were first written; a part's first step says where on the sheet it stands. Erased writing, numbered apart, is erased strokes that no part holds."
	partName    = "part %d"
	erasedName  = "erased writing %d"
	wrotePart   = "Wrote %s (%s)."
	againOn     = "Wrote on %s again."
	backTo      = "Went back to %s and wrote on it."
	oneStrokeOf = "1 stroke of %s"
	strokesOf   = "%d strokes of %s"
	moreOf      = "%d of %s"
)

var partsWords = words{header: partsHeader, step: (*Journal).partStep}

// PartsWords are all the words a journal of parts' text is made of, the places among them. As
// Words for a journal of lines, a judge's prompt that shows it holds them in its fingerprint, so a
// phrase partStep tells must be among them: the tests check that each word of the texts is.
func PartsWords() []string {
	w := []string{partsHeader, partName, erasedName, wrotePart, againOn, backTo, paused, erased, oneStrokeOf, strokesOf, moreOf, frame, leftOne, left, frameLeft, leftIt, leftOf, comma, and}
	for _, row := range places {
		w = append(w, row[:]...)
	}
	return w
}

// name is how the text names a part.
func (p Part) name() string {
	if p.Erased {
		return fmt.Sprintf(erasedName, p.Number)
	}
	return fmt.Sprintf(partName, p.Number)
}

// partStep tells a step of a journal of parts.
func (j *Journal) partStep(st Step) string {
	switch st.Kind {
	case Wrote:
		p := j.Parts[st.Line-1]
		return fmt.Sprintf(wrotePart, p.name(), p.Place)
	case Again:
		return fmt.Sprintf(againOn, j.Parts[st.Line-1].name())
	case Back:
		return fmt.Sprintf(backTo, j.Parts[st.Line-1].name())
	case Paused:
		return paused
	}
	var items []string
	for i, at := range slices.Sorted(maps.Keys(st.Taken)) {
		k, name := st.Taken[at], j.Parts[at-1].name()
		switch {
		case i > 0:
			items = append(items, fmt.Sprintf(moreOf, k, name))
		case k == 1:
			items = append(items, fmt.Sprintf(oneStrokeOf, name))
		default:
			items = append(items, fmt.Sprintf(strokesOf, k, name))
		}
	}
	return fmt.Sprintf(erased, list(items))
}
