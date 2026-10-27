package sample

import (
	"fmt"
	"math"
	"slices"
	"strings"
)

// Check checks a sample on its own and returns a *FormatError listing every problem, or nil.
func (s *Sample) Check() error {
	p := &problems{what: "sample"}
	checkHeader(p, s.Format, SampleFormat, s.Version)
	if !(s.Sheet.Width > 0) || !(s.Sheet.Height > 0) {
		p.add("sheet", "width and height must be above 0, not %v × %v", s.Sheet.Width, s.Sheet.Height)
	}
	if s.Task == "" {
		p.add("task", "is missing: the ID of the task in the bank")
	}
	if !slices.Contains([]Writer{WriterAuthor, WriterAuthorAsChild, WriterSynthetic}, s.Writer) {
		p.add("writer", "%q is not %q, %q or %q", s.Writer, WriterAuthor, WriterAuthorAsChild, WriterSynthetic)
	}
	if !slices.Contains(Scenarios, s.Scenario) {
		p.add("scenario", "%q is not one of %q", s.Scenario, Scenarios)
	}

	strokes := checkStrokes(p, s)
	erased := checkErasures(p, s, strokes)
	used := checkLines(p, s, strokes, erased)
	checkInjection(p, s, strokes, erased, used)
	checkExpected(p, s)
	return p.err()
}

// CheckWithBank checks what in a sample depends on its task: that the bank has it, and that the
// trap the sample names is one of the task's. It returns a *FormatError, or nil.
func (s *Sample) CheckWithBank(b *Bank) error {
	p := &problems{what: "sample"}
	checkInBank(p, "", s.Task, s.Expected.Trap, b)
	return p.err()
}

// checkInBank checks that the bank has a task and, if a trap is named, that it is a trap of the
// task. The fields are task and expected.trap under path, in a sample and in a plan sheet alike.
func checkInBank(p *problems, path, task, trap string, b *Bank) {
	t := b.Task(task)
	switch {
	case t == nil:
		p.add(join(path, "task"), "%q is not in the bank", task)
	case trap != "" && !slices.ContainsFunc(t.Traps, func(tr Trap) bool { return tr.ID == trap }):
		p.add(join(path, "expected.trap"), "%q is not a trap of the task %q", trap, task)
	}
}

// checkHeader checks the format a file declares and its version, one of the versions known, or
// Version when none is given.
func checkHeader(p *problems, format, want string, version *int, known ...int) {
	if format != want {
		p.add("format", "is %q, not %q", format, want)
	}
	if len(known) == 0 {
		known = []int{Version}
	}
	switch {
	case version == nil:
		p.add("version", "is missing; this reader knows %s", versions(known))
	case !slices.Contains(known, *version):
		p.add("version", "%d is not supported; this reader knows %s", *version, versions(known))
	}
}

// versions names the versions a reader knows: "version 0", or "versions 0 and 1".
func versions(known []int) string {
	if len(known) == 1 {
		return fmt.Sprintf("version %d", known[0])
	}
	names := make([]string, len(known))
	for i, v := range known {
		names[i] = fmt.Sprint(v)
	}
	return fmt.Sprintf("versions %s and %s", strings.Join(names[:len(names)-1], ", "), names[len(names)-1])
}

// strokeSpan is when a stroke was written, from its first point to its last. A stroke without
// points has no time: empty marks it, so that it still counts as a stroke of the sample.
type strokeSpan struct {
	start, end int64
	empty      bool
}

// checkStrokes checks the strokes and returns when each one was written, by ID.
func checkStrokes(p *problems, s *Sample) map[string]strokeSpan {
	spans := make(map[string]strokeSpan, len(s.Strokes))
	if len(s.Strokes) == 0 {
		p.add("strokes", "are missing: a sheet without strokes is not a sample")
	}
	var previousEnd int64 = -1
	for i, st := range s.Strokes {
		path := fmt.Sprintf("strokes[%d]", i)
		switch _, seen := spans[st.ID]; {
		case st.ID == "":
			p.add(path+".id", "is missing")
		case seen:
			p.add(path+".id", "%q is used by an earlier stroke", st.ID)
		}
		if len(st.Points) == 0 {
			p.add(path+".points", "are missing: a stroke has at least one point")
			if _, seen := spans[st.ID]; st.ID != "" && !seen {
				spans[st.ID] = strokeSpan{empty: true}
			}
			continue
		}
		for j, pt := range st.Points {
			checkPoint(p, fmt.Sprintf("%s.points[%d]", path, j), pt, s.Sheet)
			if j > 0 && pt.TMs < st.Points[j-1].TMs {
				p.add(fmt.Sprintf("%s.points[%d].t_ms", path, j), "goes back in time, %d after %d", pt.TMs, st.Points[j-1].TMs)
			}
		}
		span := strokeSpan{start: st.Points[0].TMs, end: st.Points[len(st.Points)-1].TMs}
		if span.start < previousEnd {
			p.add(path, "starts at %d ms, before the stroke before it ends at %d ms: strokes go in the order they were written", span.start, previousEnd)
		}
		previousEnd = max(previousEnd, span.end)
		if st.ID != "" {
			if _, seen := spans[st.ID]; !seen {
				spans[st.ID] = span
			}
		}
	}
	return spans
}

func checkPoint(p *problems, path string, pt Point, sheet Sheet) {
	if math.IsNaN(pt.X) || pt.X < 0 || pt.X > sheet.Width || math.IsNaN(pt.Y) || pt.Y < 0 || pt.Y > sheet.Height {
		p.add(path, "(%v, %v) is off the sheet, 0…%v × 0…%v", pt.X, pt.Y, sheet.Width, sheet.Height)
	}
	if pt.TMs < 0 {
		p.add(path+".t_ms", "%d is below 0", pt.TMs)
	}
	if math.IsNaN(pt.Pressure) || pt.Pressure < 0 || pt.Pressure > 1 {
		p.add(path+".pressure", "%v is not from 0 to 1", pt.Pressure)
	}
}

// checkErasures checks the erasures and returns the set of erased strokes.
func checkErasures(p *problems, s *Sample, spans map[string]strokeSpan) map[string]bool {
	erased := make(map[string]bool)
	planned := 0
	var previous int64
	for i, e := range s.Erasures {
		path := fmt.Sprintf("erasures[%d]", i)
		if e.TMs < previous {
			p.add(path+".t_ms", "goes back in time, %d after %d: erasures go in the order they were made", e.TMs, previous)
		}
		previous = max(previous, e.TMs)
		if len(e.Strokes) == 0 {
			p.add(path+".strokes", "are missing: an erasure removes at least one stroke")
		}
		for j, id := range e.Strokes {
			spath := fmt.Sprintf("%s.strokes[%d]", path, j)
			span, ok := spans[id]
			switch {
			case !ok:
				p.add(spath, "%q is not a stroke of the sample", id)
				continue
			case erased[id]:
				p.add(spath, "%q is erased twice", id)
			case !span.empty && span.end > e.TMs:
				p.add(spath, "%q ends at %d ms, after the erasure at %d ms", id, span.end, e.TMs)
			}
			erased[id] = true
		}
		for _, st := range s.Strokes {
			if span, ok := spans[st.ID]; ok && !span.empty && span.start < e.TMs && e.TMs < span.end {
				p.add(path+".t_ms", "%d falls while the stroke %q is being written: the pen cannot write and erase at once", e.TMs, st.ID)
			}
		}
		if e.Planned != nil {
			planned++
			checkPlanned(p, path+".planned", *e.Planned, len(s.Lines))
			if want, ok := plannedErasure[s.Scenario]; ok && e.Planned.Erased != want {
				p.add(path+".planned.erased", "is %q, but the planned erasure of the scenario %q erases a %q", e.Planned.Erased, s.Scenario, want)
			}
		}
	}
	if _, plans := plannedErasure[s.Scenario]; plans {
		if planned != 1 {
			p.add("erasures", "the scenario %q has one planned erasure, not %d", s.Scenario, planned)
		}
	} else if planned != 0 {
		p.add("erasures", "only the scenarios %q and %q plan an erasure; other erasures are slips and have no markup", ScenarioErasureFixed, ScenarioErasureBroken)
	}
	return erased
}

func checkPlanned(p *problems, path string, pl PlannedErasure, lines int) {
	if pl.Line < 1 || pl.Line > lines {
		p.add(path+".line", "%d is not a line of the sheet, 1…%d", pl.Line, lines)
	}
	if pl.Erased != ErasedMistake && pl.Erased != ErasedCorrectStep {
		p.add(path+".erased", "%q is not %q or %q", pl.Erased, ErasedMistake, ErasedCorrectStep)
	}
	if pl.Text == "" {
		p.add(path+".text", "is missing: what the erased strokes said")
	}
}

// checkLines checks the lines and returns which line holds each stroke, by ID.
func checkLines(p *problems, s *Sample, spans map[string]strokeSpan, erased map[string]bool) map[string]string {
	used := make(map[string]string)
	answers := 0
	for i, l := range s.Lines {
		path := fmt.Sprintf("lines[%d]", i)
		if l.Text == "" {
			p.add(path+".text", "is missing: what the writer meant to write")
		}
		if len(l.Strokes) == 0 {
			p.add(path+".strokes", "are missing: a line is written with at least one stroke")
		}
		for j, id := range l.Strokes {
			checkVisible(p, fmt.Sprintf("%s.strokes[%d]", path, j), id, path, spans, erased, used)
		}
		if l.Answer {
			answers++
			if l.CrossedOut {
				p.add(path, "is both the answer and crossed out")
			}
		}
	}
	switch s.Expected.Outcome {
	case OutcomeNoAnswer:
		if answers != 0 {
			p.add("lines", "a sheet with no answer marks no line as the answer, not %d", answers)
		}
	default:
		if answers != 1 {
			p.add("lines", "one line is the answer, not %d", answers)
		}
	}
	if n := len(s.Lines); s.Scenario == ScenarioAnswerNotLast && n > 0 && s.Lines[n-1].Answer {
		p.add(fmt.Sprintf("lines[%d]", n-1), "is the answer and the last line, but on a sheet of the scenario %q a line follows the answer", ScenarioAnswerNotLast)
	}
	return used
}

// checkVisible checks that a stroke the markup names exists, is still on the sheet and belongs
// to nothing else, and records its owner.
func checkVisible(p *problems, path, id, owner string, spans map[string]strokeSpan, erased map[string]bool, used map[string]string) {
	_, exists := spans[id]
	other, taken := used[id]
	switch {
	case !exists:
		p.add(path, "%q is not a stroke of the sample", id)
	case erased[id]:
		p.add(path, "%q is erased: the markup of the sheet names only strokes still on it", id)
	case taken:
		p.add(path, "%q already belongs to %s", id, other)
	default:
		used[id] = owner
	}
}

func checkInjection(p *problems, s *Sample, spans map[string]strokeSpan, erased map[string]bool, used map[string]string) {
	in := s.Injection
	switch {
	case s.Scenario == ScenarioInjection && in == nil:
		p.add("injection", "is missing: the scenario %q has one", ScenarioInjection)
		return
	case s.Scenario != ScenarioInjection && in != nil:
		p.add("injection", "only the scenario %q has one", ScenarioInjection)
		return
	case in == nil:
		return
	}
	if in.Text == "" {
		p.add("injection.text", "is missing: what the inscription says")
	}
	if in.Lang != LangRussian && in.Lang != LangEnglish {
		p.add("injection.lang", "%q is not %q or %q", in.Lang, LangRussian, LangEnglish)
	}
	if len(in.Strokes) == 0 {
		p.add("injection.strokes", "are missing: the inscription is written with at least one stroke")
	}
	for j, id := range in.Strokes {
		checkVisible(p, fmt.Sprintf("injection.strokes[%d]", j), id, "injection", spans, erased, used)
	}
}

func checkExpected(p *problems, s *Sample) {
	e := s.Expected
	outcomes := []Outcome{OutcomeCorrect, OutcomeError, OutcomeUnreadable, OutcomeNoAnswer}
	if !slices.Contains(outcomes, e.Outcome) {
		p.add("expected.outcome", "%q is not one of %q", e.Outcome, outcomes)
	}
	switch {
	case e.Outcome == OutcomeError && e.FirstError == nil:
		p.add("expected.first_error", "is missing: a sheet with an error names its first wrong line")
	case e.Outcome != OutcomeError && e.FirstError != nil:
		p.add("expected.first_error", "is set, but only a sheet with the outcome %q has a first error", OutcomeError)
	case e.FirstError != nil:
		n := *e.FirstError
		if n < 1 || n > len(s.Lines) {
			p.add("expected.first_error", "%d is not a line of the sheet, 1…%d", n, len(s.Lines))
		} else if s.Lines[n-1].CrossedOut {
			p.add("expected.first_error", "line %d is crossed out: it is not part of the solution", n)
		}
	}
	// An illegible answer and the outcome "unreadable" go together: reading such an answer asks to
	// write it again, and no verdict comes.
	for i, l := range s.Lines {
		switch {
		case l.Answer && !l.Illegible && e.Outcome == OutcomeUnreadable:
			p.add(fmt.Sprintf("lines[%d]", i), "is the answer of a sheet whose outcome is %q, so it is marked illegible", OutcomeUnreadable)
		case l.Answer && l.Illegible && e.Outcome != OutcomeUnreadable:
			p.add(fmt.Sprintf("lines[%d]", i), "is an illegible answer, so the outcome is %q, not %q", OutcomeUnreadable, e.Outcome)
		}
	}
	// The outcome each scenario is written for; the rest are free.
	want := map[Scenario]Outcome{
		ScenarioCorrect:       OutcomeCorrect,
		ScenarioError:         OutcomeError,
		ScenarioNearlyEmpty:   OutcomeNoAnswer,
		ScenarioErasureFixed:  OutcomeCorrect,
		ScenarioErasureBroken: OutcomeError,
	}
	if o, ok := want[s.Scenario]; ok && e.Outcome != o && slices.Contains(outcomes, e.Outcome) {
		p.add("expected.outcome", "is %q, but a sheet of the scenario %q is %q", e.Outcome, s.Scenario, o)
	}
	if e.Trap != "" && e.Outcome != OutcomeError {
		p.add("expected.trap", "is set, but only a sheet with the outcome %q falls into a trap", OutcomeError)
	}
}

// plannedErasure is what the planned erasure of each erasure scenario removes (R11).
var plannedErasure = map[Scenario]Erased{
	ScenarioErasureFixed:  ErasedMistake,
	ScenarioErasureBroken: ErasedCorrectStep,
}
