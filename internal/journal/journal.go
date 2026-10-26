// Package journal tells how a sheet was written, for the forms of the measurement that show the
// model the writing process (R11, R24): a journal of numbered steps in English, and up to two
// frames of the sheet just before erasures. It builds both from the strokes and the erasures
// alone, as a device would, without the markup. The lines are those lines.Group finds among all
// the strokes, the erased ones too, numbered from the top of the sheet down.
//
// The journal tells in which order the lines were written, the long pauses, how many strokes each
// erasure took and from which line, and the returns to lines written before. It holds no time, no
// pressure, no coordinates and nothing of what was erased (R11).
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
	Wrote  Kind = iota // writing on a line not written on before
	Again              // writing on the line written on last, after an erasure or a pause
	Back               // writing on a line written on before, coming from another line
	Erased             // an erasure, or erasures one after another from the same one line
	Paused             // a pause of Pause or longer
)

// Step is one step of the journal.
type Step struct {
	Kind Kind
	// Line is the number of the line written on, in a step of writing.
	Line int
	// Taken is how many strokes the erasures of the step took, by the number of their line.
	Taken map[int]int
	// Erasures are the indexes, in the sample, of the erasures the step tells.
	Erasures []int
	// Frame is the number of the frame of the sheet just before the step, from 1, or 0.
	Frame int
}

// Frame is a frame of the sheet just before an erasure (R11): the first erasure of a line, of the
// lines most strokes were erased from.
type Frame struct {
	Erasure int // the erasure's index in the sample: render.Before draws the frame
	Step    int // the index of the step of the journal the frame stands just before
	Line    int // the line the frame is for
	Strokes int // how many strokes the erasures took from the line, in all
}

// Journal is how a sheet was written.
type Journal struct {
	// Lines are the lines of all the strokes, the erased ones too, from the top of the sheet down.
	Lines []lines.Line
	// Steps are all the steps of the writing; the text tells the first MaxSteps of them.
	Steps []Step
	// Frames are the frames before erasures, in the order of the writing.
	Frames []Frame
}

// Of builds the journal of a sample that passes its check (sample.Check).
func Of(s *sample.Sample) *Journal {
	j := &Journal{Lines: lines.Group(s.Strokes, lines.Gap)}
	b := &builder{lineOf: map[string]int{}, written: map[int]bool{}}
	for i, l := range j.Lines {
		for _, id := range l.Strokes {
			b.lineOf[id] = i + 1
		}
	}
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
	return j
}

// first reports whether an erasure comes before a stroke.
func first(e sample.Erasure, st sample.Stroke) bool {
	start := st.Points[0].TMs
	return e.TMs < start || e.TMs == start && !slices.Contains(e.Strokes, st.ID)
}

// builder turns the strokes and erasures, in the order of the writing, into steps.
type builder struct {
	steps   []Step
	lineOf  map[string]int // the number of the line of each stroke
	written map[int]bool   // the lines written on so far
	last    int            // the line written on last, 0 before any
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

// Text is the journal in English: a sentence on how to read it, then one numbered step a line, the
// first MaxSteps steps, and how many it leaves out. With marks, a step that a frame stands just
// before says so, by the frame's number, and so does the end for a frame among the steps left out.
func (j *Journal) Text(marks bool) string {
	var b strings.Builder
	b.WriteString("How the sheet was written, step by step. Lines are numbered from the top of the sheet down, erased writing included.\n")
	told := j.Steps[:min(len(j.Steps), MaxSteps)]
	for i, st := range told {
		fmt.Fprintf(&b, "%d. %s", i+1, st.text())
		if marks && st.Frame > 0 {
			fmt.Fprintf(&b, " Frame %d shows the sheet just before this.", st.Frame)
		}
		b.WriteString("\n")
	}
	left, which := j.Steps[len(told):], "one of them"
	switch len(left) {
	case 0:
		return b.String()
	case 1:
		b.WriteString("1 more step is left out.")
		which = "it"
	default:
		fmt.Fprintf(&b, "%d more steps are left out.", len(left))
	}
	for _, st := range left {
		if marks && st.Frame > 0 {
			fmt.Fprintf(&b, " Frame %d shows the sheet just before %s.", st.Frame, which)
		}
	}
	b.WriteString("\n")
	return b.String()
}

func (st Step) text() string {
	switch st.Kind {
	case Wrote:
		return fmt.Sprintf("Wrote line %d.", st.Line)
	case Again:
		return fmt.Sprintf("Wrote on line %d again.", st.Line)
	case Back:
		return fmt.Sprintf("Went back to line %d and wrote on it.", st.Line)
	case Paused:
		return "Long pause."
	}
	var parts []string
	for i, line := range slices.Sorted(maps.Keys(st.Taken)) {
		k := st.Taken[line]
		switch {
		case i > 0:
			parts = append(parts, fmt.Sprintf("%d on line %d", k, line))
		case k == 1:
			parts = append(parts, fmt.Sprintf("1 stroke on line %d", line))
		default:
			parts = append(parts, fmt.Sprintf("%d strokes on line %d", k, line))
		}
	}
	if n := len(parts); n > 1 {
		parts = []string{strings.Join(parts[:n-1], ", ") + " and " + parts[n-1]}
	}
	return "Erased " + parts[0] + "."
}

// Agreement is how the lines of a journal agree with the lines of a sample's markup: of the
// markup's lines, how many the journal found with just their strokes. It compares the strokes the
// markup's lines hold, which are all on the sheet: an erased stroke, an inscription or a stray mark
// the journal puts on a line does not count against it. The markup of a sheet starts from the lines
// the board proposes with lines.Group (R25), so the agreement is higher than it would be with a
// markup made apart, the more so on sheets saved before the board could edit the lines (T51).
type Agreement struct {
	Marked, Same int
}

// Agree finds how the lines of the journal of a sample agree with its markup.
func Agree(s *sample.Sample, j *Journal) Agreement {
	held := map[string]bool{} // the strokes the markup's lines hold
	for _, l := range s.Lines {
		for _, id := range l.Strokes {
			held[id] = true
		}
	}
	found := map[string]bool{}
	for _, l := range j.Lines {
		found[key(l.Strokes, held)] = true
	}
	a := Agreement{Marked: len(s.Lines)}
	for _, l := range s.Lines {
		if found[key(l.Strokes, held)] {
			a.Same++
		}
	}
	return a
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
