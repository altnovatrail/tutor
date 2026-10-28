// Package states chooses the states of a sheet for the reading of unfinished work (О-8, О-21; T69):
// the moments of the writing at which a device that read on its own, without the learner asking,
// would read, and what the sheet shows at each. It finds them from the strokes and the erasures, as
// a device would, in the order of the writing the journal takes (journal.Writing):
//   - after a line: the writer finishes writing on a line or a part of the journal of the sheet and
//     goes on to write on another, erasing in between or not;
//   - at a pause: the longest pause of 0.4 s or more between two strokes one after the other within
//     the answer line of the markup, and within one other line of it. Only the markup knows the
//     answer line: which pauses are read the measurement chooses, the moments are the strokes';
//   - before an erasure: just before each run of erasures with no stroke between them, of the
//     scenario or a slip alike, for a device knows no scenario.
//
// Each state stands after an event of the writing, so no stroke is half drawn on it, and none is
// the finished sheet: an event follows each. What the writer had written of each part of the markup
// by then is the truth the report measures the reading of a state against (T72). It counts, too,
// how many readings a sheet would take beyond its sending by each policy of reading on its own
// (R10): after each line, and at each pause of 0.4, 2 and 6 s or more.
package states

import (
	"maps"
	"slices"

	"github.com/altnovatrail/tutor/internal/journal"
	"github.com/altnovatrail/tutor/internal/sample"
)

// MinPause is the shortest pause within a line that a state is read at, in milliseconds: the mean
// pause of a child's writing (docs/architecture.md, 3.1).
const MinPause = 400

// Progress is how far a part of the markup was written by a moment.
type Progress string

// How far a part was written: all its strokes, some of them, or none.
const (
	Done  Progress = "done"
	Begun Progress = "begun"
	None  Progress = "none"
)

// State is a state of a sheet: the sheet as it was once an event of the writing was done, and why
// it was chosen. A state chosen for two reasons is one state with both.
type State struct {
	// Event is the number of the event of the writing it stands after, from 1, and TMs when that
	// event ended.
	Event int   `json:"event"`
	TMs   int64 `json:"t_ms"`
	// Line is the line or part of the journal the writer finished there before writing on another,
	// from 1, or 0.
	Line int `json:"line,omitempty"`
	// Pause is the line of the markup, from 1, within which the writer paused there, or 0, and
	// PauseMs how long the pause was.
	Pause   int   `json:"pause,omitempty"`
	PauseMs int64 `json:"pause_ms,omitempty"`
	// Erasure is the erasure the state stands just before, by its index in the sample from 1, or 0:
	// the first of a run of erasures with no stroke between them.
	Erasure int `json:"erasure,omitempty"`
	// Parts are how far each part of the markup, in its order, was written by then.
	Parts []Progress `json:"parts"`
	// Erased is how many of the strokes the sheet shows then an erasure takes away later: they are
	// of no part of the markup, which names only the strokes left on the sheet.
	Erased int `json:"erased,omitempty"`
	// Shown are the strokes the sheet shows then, in the sample's order.
	Shown []sample.Stroke `json:"-"`
}

// Of finds the states of a sample that passes its check, in the order of the writing, with the
// journal of parts when its sheet is of a plan of parts, and the journal of lines otherwise.
func Of(s *sample.Sample, parts bool) []State {
	var j *journal.Journal
	if parts {
		j = journal.OfParts(s)
	} else {
		j = journal.Of(s)
	}
	events := journal.Writing(s)
	chosen := map[int]*State{} // by the index of the event they stand after
	at := func(k int) *State {
		if st, ok := chosen[k]; ok {
			return st
		}
		st := &State{Event: k + 1, TMs: events[k].End(s)}
		chosen[k] = st
		return st
	}
	for k := 0; k+1 < len(events); k++ {
		this, next := events[k], events[k+1]
		if this.Stroke == nil {
			continue
		}
		if next.Stroke == nil {
			at(k).Erasure = next.Erasure + 1
		}
		// The next stroke, past any erasures, is on another line or part.
		for _, later := range events[k+1:] {
			if later.Stroke != nil {
				if j.LineOf(later.Stroke.ID) != j.LineOf(this.Stroke.ID) {
					at(k).Line = j.LineOf(this.Stroke.ID)
				}
				break
			}
		}
	}
	for _, p := range pauses(s, events) {
		st := at(p.at)
		st.Pause, st.PauseMs = p.line, p.ms
	}
	var out []State
	for _, k := range slices.Sorted(maps.Keys(chosen)) {
		st := chosen[k]
		st.Shown, st.Parts, st.Erased = shownAfter(s, events, k)
		out = append(out, *st)
	}
	return out
}

// pause is a pause within a line of the markup: after which event, in which line, how long.
type pause struct {
	at, line int
	ms       int64
}

// pauses are the pauses the states are read at: the longest pause of MinPause or more between two
// strokes one after the other within the answer line, and the longest within one other line, the
// one whose longest is longest. A line crossed out or written illegibly on purpose is no other
// line. Of two pauses as long, the earlier is taken.
func pauses(s *sample.Sample, events []journal.Event) []pause {
	lineOf := map[string]int{}
	for i, l := range s.Lines {
		for _, id := range l.Strokes {
			lineOf[id] = i + 1
		}
	}
	longest := map[int]pause{} // by the line
	for k := 0; k+1 < len(events); k++ {
		this, next := events[k], events[k+1]
		if this.Stroke == nil || next.Stroke == nil {
			continue
		}
		line := lineOf[this.Stroke.ID]
		if line == 0 || lineOf[next.Stroke.ID] != line {
			continue
		}
		ms := next.Start(s) - this.End(s)
		if p, ok := longest[line]; ms >= MinPause && (!ok || ms > p.ms) {
			longest[line] = pause{at: k, line: line, ms: ms}
		}
	}
	var out []pause
	var other *pause
	for line := 1; line <= len(s.Lines); line++ {
		p, ok := longest[line]
		l := s.Lines[line-1]
		switch {
		case !ok:
		case l.Answer:
			out = append(out, p)
		case !l.CrossedOut && !l.Illegible && (other == nil || p.ms > other.ms || p.ms == other.ms && p.at < other.at):
			other = &p
		}
	}
	if other != nil {
		out = append(out, *other)
	}
	return out
}

// shownAfter is what the sheet shows once the event k is done, how far each part of the markup was
// written by then, and how many of the strokes shown an erasure takes away later.
func shownAfter(s *sample.Sample, events []journal.Event, k int) ([]sample.Stroke, []Progress, int) {
	written, gone := map[string]bool{}, map[string]bool{}
	for _, ev := range events[:k+1] {
		if ev.Stroke != nil {
			written[ev.Stroke.ID] = true
			continue
		}
		for _, id := range s.Erasures[ev.Erasure].Strokes {
			gone[id] = true
		}
	}
	later := map[string]bool{}
	for _, ev := range events[k+1:] {
		if ev.Stroke == nil {
			for _, id := range s.Erasures[ev.Erasure].Strokes {
				later[id] = true
			}
		}
	}
	var shown []sample.Stroke
	erased := 0
	for _, st := range s.Strokes {
		if written[st.ID] && !gone[st.ID] {
			shown = append(shown, st)
			if later[st.ID] {
				erased++
			}
		}
	}
	parts := make([]Progress, len(s.Lines))
	for i, l := range s.Lines {
		n := 0
		for _, id := range l.Strokes {
			if written[id] {
				n++
			}
		}
		switch {
		case n == 0:
			parts[i] = None
		case n == len(l.Strokes):
			parts[i] = Done
		default:
			parts[i] = Begun
		}
	}
	return shown, parts, erased
}

// Policies are how many readings a sheet would take beyond its sending by each policy of reading
// on its own (R10): after each line, and at each pause of 0.4, 2 and 6 s or more between one event
// of the writing and the next. The wait before the first stroke is the reading of the task, and no
// policy reads at it.
type Policies struct {
	AfterLine int `json:"after_line"`
	Pause400  int `json:"pause_400"`
	Pause2000 int `json:"pause_2000"`
	Pause6000 int `json:"pause_6000"`
}

// PoliciesOf counts the readings of each policy on a sample that passes its check, with its states.
func PoliciesOf(s *sample.Sample, states []State) Policies {
	var p Policies
	for _, st := range states {
		if st.Line > 0 {
			p.AfterLine++
		}
	}
	events := journal.Writing(s)
	for k := 1; k < len(events); k++ {
		ms := events[k].Start(s) - events[k-1].End(s)
		for _, c := range []struct {
			n         *int
			threshold int64
		}{{&p.Pause400, 400}, {&p.Pause2000, 2000}, {&p.Pause6000, journal.Pause}} {
			if ms >= c.threshold {
				(*c.n)++
			}
		}
	}
	return p
}
