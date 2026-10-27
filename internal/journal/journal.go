// Package journal tells how a sheet was written, for the forms of the measurement that show the
// model the writing process (R11, R24): a journal of numbered steps in English, and up to two
// frames of the sheet just before erasures. It builds both from the strokes and the erasures
// alone, as a device would, without the markup.
//
// A journal comes in two versions, each with its own words. A journal of lines (Of), version 1,
// is that of the measurement of E2: its lines are those lines.Group finds among all the strokes,
// the erased ones too, numbered from the top of the sheet down. A journal of parts (OfParts),
// version 2, is that of free writing (T63): its parts are the stroke groups lines.Groups finds,
// numbered in the order they were first written, each named by where it stands on the sheet.
//
// The journal tells in which order the lines or the parts were written, the long pauses, how many
// strokes each erasure took and from which, and the returns to those written before. It holds no
// time, no pressure, no coordinates and nothing of what was erased (R11).
package journal

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/altnovatrail/tutor/internal/lines"
	"github.com/altnovatrail/tutor/internal/sample"
)

const (
	// Pause is the shortest pause the journal tells, in milliseconds (R11; R16 canvas-api). A wait
	// before the first stroke is the reading of the task, and the journal does not tell it.
	Pause = 6000
	// MaxFrames is how many frames before erasures a sheet gives at most (R11).
	MaxFrames = 2
	// MaxSteps is how many steps the text of a journal tells at most, so that it stays near 300
	// tokens (R11); it counts the steps after them and leaves them out.
	MaxSteps = 30
)

// Kind is what a step of the journal is.
type Kind int

const (
	Wrote  Kind = iota // writing on a line or a part not written on before
	Again              // writing on the one written on last, after an erasure or a pause
	Back               // writing on one written on before, coming from another
	Erased             // an erasure, or erasures one after another from the same one
	Paused             // a pause of Pause or longer
)

// Step is one step of the journal.
type Step struct {
	Kind Kind
	// Line is the number of the line written on, in a step of writing; in a journal of parts, the
	// place of the part in Parts, from 1.
	Line int
	// Taken is how many strokes the erasures of the step took, by the number of their line, or by
	// the place of their part in Parts.
	Taken map[int]int
	// Erasures are the indexes, in the sample, of the erasures the step tells.
	Erasures []int
	// Frame is the number of the frame of the sheet just before the step, from 1, or 0.
	Frame int
}

// Frame is a frame of the sheet just before an erasure (R11): the first erasure of a line or a
// part, of those most strokes were erased from.
type Frame struct {
	Erasure int // the erasure's index in the sample: render.Before draws the frame
	Step    int // the index of the step of the journal the frame stands just before
	Line    int // the line the frame is for, or the place of its part in Parts
	Strokes int // how many strokes the erasures took from the line or the part, in all
}

// Journal is how a sheet was written.
type Journal struct {
	// Lines are the lines of all the strokes, the erased ones too, from the top of the sheet down,
	// in a journal of lines.
	Lines []lines.Line
	// Parts are the parts of a journal of parts, in the order they were first written.
	Parts []Part
	// Steps are all the steps of the writing; the text tells the first MaxSteps of them.
	Steps []Step
	// Frames are the frames before erasures, in the order of the writing.
	Frames []Frame
	// words are the words of its text, those of its version.
	words *words
}

// Of builds the journal of lines of a sample that passes its check (sample.Check).
func Of(s *sample.Sample) *Journal {
	j := &Journal{Lines: lines.Group(s.Strokes, lines.Gap), words: &linesWords}
	lineOf := map[string]int{}
	for i, l := range j.Lines {
		for _, id := range l.Strokes {
			lineOf[id] = i + 1
		}
	}
	j.walk(s, lineOf)
	return j
}

// walk tells the steps and the frames of the writing, with the number of the line or the part of
// each stroke.
func (j *Journal) walk(s *sample.Sample, lineOf map[string]int) {
	b := &builder{lineOf: lineOf, written: map[int]bool{}}
	var strokes []sample.Stroke
	for _, st := range s.Strokes {
		if len(st.Points) > 0 {
			strokes = append(strokes, st)
		}
	}
	// Strokes and erasures each go in the order of the clock; merged, they are the writing. An
	// erasure goes before a stroke begun at its very moment, unless it takes that stroke away: the
	// stroke is then the next one, as on the frames of internal/render.
	k, e := 0, 0
	for k < len(strokes) || e < len(s.Erasures) {
		if e < len(s.Erasures) && (k == len(strokes) || first(s.Erasures[e], strokes[k])) {
			b.erasure(e, s.Erasures[e])
			e++
		} else {
			b.stroke(strokes[k])
			k++
		}
	}
	j.Steps = b.steps
	j.Frames = frames(j.Steps)
	for n, f := range j.Frames {
		j.Steps[f.Step].Frame = n + 1
	}
}

// first reports whether an erasure comes before a stroke.
func first(e sample.Erasure, st sample.Stroke) bool {
	start := st.Points[0].TMs
	return e.TMs < start || e.TMs == start && !slices.Contains(e.Strokes, st.ID)
}

// builder turns the strokes and erasures, in the order of the writing, into steps.
type builder struct {
	steps   []Step
	lineOf  map[string]int // the number of the line or the part of each stroke
	written map[int]bool   // the lines or the parts written on so far
	last    int            // the one written on last, 0 before any
	end     int64          // when the latest stroke or erasure ended
	begun   bool           // whether any came yet
}

// pause tells a long pause before what starts at the moment start.
func (b *builder) pause(start int64) {
	if b.begun && start-b.end >= Pause {
		b.steps = append(b.steps, Step{Kind: Paused})
	}
}

func (b *builder) stroke(st sample.Stroke) {
	b.pause(st.Points[0].TMs)
	line := b.lineOf[st.ID]
	if n := len(b.steps); n == 0 || !writing(b.steps[n-1].Kind) || b.steps[n-1].Line != line {
		kind := Wrote
		switch {
		case b.written[line] && line == b.last:
			kind = Again
		case b.written[line]:
			kind = Back
		}
		b.steps = append(b.steps, Step{Kind: kind, Line: line})
	}
	b.written[line], b.last = true, line
	b.end, b.begun = st.Points[len(st.Points)-1].TMs, true
}

func (b *builder) erasure(i int, e sample.Erasure) {
	taken := map[int]int{}
	for _, id := range e.Strokes {
		if n := b.lineOf[id]; n > 0 {
			taken[n]++
		}
	}
	if len(taken) == 0 {
		return // it took no stroke that stands on a line, as in no sample that passes its check
	}
	b.pause(e.TMs)
	b.end, b.begun = e.TMs, true
	if n := len(b.steps); n > 0 {
		if prev := &b.steps[n-1]; prev.Kind == Erased && oneLine(prev.Taken) > 0 && oneLine(prev.Taken) == oneLine(taken) {
			for line, k := range taken {
				prev.Taken[line] += k
			}
			prev.Erasures = append(prev.Erasures, i)
			return
		}
	}
	b.steps = append(b.steps, Step{Kind: Erased, Taken: taken, Erasures: []int{i}})
}

func writing(k Kind) bool { return k == Wrote || k == Again || k == Back }

// oneLine is the line strokes were taken from when they all were from one, or else 0.
func oneLine(taken map[int]int) int {
	if len(taken) != 1 {
		return 0
	}
	for line := range taken {
		return line
	}
	return 0
}

// frames chooses the frames of a sheet: for each line strokes were erased from, the frame just
// before its first erasure, and of these the frames of the lines most strokes were erased from,
// of two lines with as many the one erased first. Erasures of one line give one frame. A step
// of erasures from one line starts with the first of them that takes from the line, so a frame
// always stands just before a step.
func frames(steps []Step) []Frame {
	var all []Frame
	at := map[int]int{} // the index in all of each line's frame
	for i, st := range steps {
		if st.Kind != Erased {
			continue
		}
		for _, line := range slices.Sorted(maps.Keys(st.Taken)) {
			if i, ok := at[line]; ok {
				all[i].Strokes += st.Taken[line]
				continue
			}
			at[line] = len(all)
			all = append(all, Frame{Erasure: st.Erasures[0], Step: i, Line: line, Strokes: st.Taken[line]})
		}
	}
	slices.SortStableFunc(all, func(a, b Frame) int {
		return cmp.Or(cmp.Compare(b.Strokes, a.Strokes), cmp.Compare(a.Erasure, b.Erasure))
	})
	var chosen []Frame
	for _, f := range all {
		if len(chosen) < MaxFrames && !slices.ContainsFunc(chosen, func(c Frame) bool { return c.Erasure == f.Erasure }) {
			chosen = append(chosen, f)
		}
	}
	slices.SortFunc(chosen, func(a, b Frame) int { return cmp.Compare(a.Erasure, b.Erasure) })
	return chosen
}

// The words of a journal of lines' text: its first line, the phrase of each kind of step with the
// numbers it takes, the marks of the frames and the end of a journal cut short. A journal of parts
// shares the pause, the erasure, the marks, the end and the joins of a list.
const (
	header    = "How the sheet was written, step by step. Lines are numbered from the top of the sheet down, erased writing included."
	wrote     = "Wrote line %d."
	again     = "Wrote on line %d again."
	back      = "Went back to line %d and wrote on it."
	paused    = "Long pause."
	erased    = "Erased %s."
	oneStroke = "1 stroke on line %d"
	strokesOn = "%d strokes on line %d"
	moreOn    = "%d on line %d"
	frame     = " Frame %d shows the sheet just before this."
	leftOne   = "1 more step is left out."
	left      = "%d more steps are left out."
	frameLeft = " Frame %d shows the sheet just before %s."
	leftIt    = "it"
	leftOf    = "one of them"
	comma     = ", "
	and       = " and "
)

// Words are all the words a journal of lines' text is made of. A change to any of them changes
// what a judge reads, so the fingerprint of the measurement's prompt holds them (internal/judge).
func Words() []string {
	return []string{header, wrote, again, back, paused, erased, oneStroke, strokesOn, moreOn, frame, leftOne, left, frameLeft, leftIt, leftOf, comma, and}
}

// words are what the text of one version of a journal has of its own: its first line and how it
// tells a step. The marks of the frames and the end of a journal cut short both versions share.
type words struct {
	header string
	step   func(j *Journal, st Step) string
}

var linesWords = words{header: header, step: func(_ *Journal, st Step) string { return st.text() }}

// Text is the journal in English: a sentence on how to read it, then one numbered step a line, the
// first MaxSteps steps, and how many it leaves out. With marks, a step that a frame stands just
// before says so, by the frame's number, and so does the end for a frame among the steps left out.
// Each version tells it in its own words.
func (j *Journal) Text(marks bool) string {
	var b strings.Builder
	b.WriteString(j.words.header + "\n")
	told := j.Steps[:min(len(j.Steps), MaxSteps)]
	for i, st := range told {
		fmt.Fprintf(&b, "%d. %s", i+1, j.words.step(j, st))
		if marks && st.Frame > 0 {
			fmt.Fprintf(&b, frame, st.Frame)
		}
		b.WriteString("\n")
	}
	rest, which := j.Steps[len(told):], leftOf
	switch len(rest) {
	case 0:
		return b.String()
	case 1:
		b.WriteString(leftOne)
		which = leftIt
	default:
		fmt.Fprintf(&b, left, len(rest))
	}
	for _, st := range rest {
		if marks && st.Frame > 0 {
			fmt.Fprintf(&b, frameLeft, st.Frame, which)
		}
	}
	b.WriteString("\n")
	return b.String()
}

func (st Step) text() string {
	switch st.Kind {
	case Wrote:
		return fmt.Sprintf(wrote, st.Line)
	case Again:
		return fmt.Sprintf(again, st.Line)
	case Back:
		return fmt.Sprintf(back, st.Line)
	case Paused:
		return paused
	}
	var parts []string
	for i, line := range slices.Sorted(maps.Keys(st.Taken)) {
		k := st.Taken[line]
		switch {
		case i > 0:
			parts = append(parts, fmt.Sprintf(moreOn, k, line))
		case k == 1:
			parts = append(parts, fmt.Sprintf(oneStroke, line))
		default:
			parts = append(parts, fmt.Sprintf(strokesOn, k, line))
		}
	}
	return fmt.Sprintf(erased, list(parts))
}

// list joins the items of a list in words: "a", "a and b", "a, b and c".
func list(items []string) string {
	if n := len(items); n > 1 {
		return strings.Join(items[:n-1], comma) + and + items[n-1]
	}
	return items[0]
}

// Agreement is how the lines of a journal agree with the lines of a sample's markup: of the
// markup's lines, how many the journal found with just their strokes. It compares the strokes the
// markup's lines hold, which are all on the sheet: an erased stroke, an inscription or a stray mark
// the journal puts on a line does not count against it. The markup of a sheet starts from the lines
// the board proposes with lines.Group (R25), so the agreement is higher than it would be with a
// markup made apart, the more so on sheets saved before the board could edit the lines (T51).
// Of a journal of parts, it compares the parts with the parts of the markup, which the board
// proposes from lines.Groups (T61).
type Agreement struct {
	Marked int `json:"marked"`
	Same   int `json:"same"`
}

// Agree finds how the lines or the parts of the journal of a sample agree with its markup.
func Agree(s *sample.Sample, j *Journal) Agreement {
	held := map[string]bool{} // the strokes the markup's lines hold
	for _, l := range s.Lines {
		for _, id := range l.Strokes {
			held[id] = true
		}
	}
	found := map[string]bool{}
	for _, ids := range j.strokeSets() {
		found[key(ids, held)] = true
	}
	a := Agreement{Marked: len(s.Lines)}
	for _, l := range s.Lines {
		if found[key(l.Strokes, held)] {
			a.Same++
		}
	}
	return a
}

// strokeSets are the strokes of each line of a journal of lines, or of each part of a journal of
// parts.
func (j *Journal) strokeSets() [][]string {
	var sets [][]string
	if j.words == &partsWords {
		for _, p := range j.Parts {
			sets = append(sets, p.Group.Strokes)
		}
		return sets
	}
	for _, l := range j.Lines {
		sets = append(sets, l.Strokes)
	}
	return sets
}

// key names a set of strokes by those of them the markup's lines hold.
func key(ids []string, held map[string]bool) string {
	var kept []string
	for _, id := range ids {
		if held[id] {
			kept = append(kept, id)
		}
	}
	slices.Sort(kept)
	return strings.Join(kept, "\x00")
}
