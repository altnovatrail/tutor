package report

import (
	"fmt"

	"github.com/altnovatrail/module-kit/verdict"
	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"

	"github.com/altnovatrail/tutor/admission"
)

// The rule of admission (R80; R07 spec; spec's contracts text, sections 13 and 14): which stage a
// runner's report admits its readers to. Only a report of reading handwriting is ruled on: the
// assessment by a rubric has no target until the essays of adult volunteers (R71 spec; П-16).
const (
	// FullTarget is the target of the stage full: the upper bound of the share of false alarms, and
	// of false "correct", no more than 1 % (product 10 spec).
	FullTarget = 0.01
	// AuthorSlots is how many answer slots of the stage author at least have a truth that is not
	// "not yet", so that a false alarm could happen on them: 30, whose bound with no event is near
	// 10 %, which the decision says beside it. It was 50 until the author stopped at 60 sheets, 35 of
	// them correct (amendment of 2026-11-08 to R80).
	AuthorSlots = 30
	// AuthorWrongSlots is how many at least have a truth that is not "correct", so that a false
	// "correct" could happen on them: without them the stage would pass that share on nothing. It was
	// 25 until the same amendment.
	AuthorWrongSlots = 20
)

// SlotShare is a share counted over answer slots rather than readings: a slot is an event when any
// of its readings that is not unchecked is one, and it is counted when one of them at least is.
// The readings of one slot are repeats of one work, not draws of their own, so the bound counts
// each slot once, conservatively: with no event at all report.Upper could not widen it for them.
type SlotShare struct {
	Events, Of int
	Upper      float64
}

// Decision is what the rule makes of a report: the stage it admits its readers to, none when it
// admits them to nothing, and why not; the readers and the set, as the report names them; the
// shares of false alarms and of false "correct" over the answer slots, with their bounds; and the
// measures of section 14 over the readings, needless "unreadable" and unchecked among them, which
// are shown beside and have no target.
type Decision struct {
	Stage    admission.Stage
	Why      string
	Readers  []string
	Set      string
	Repeats  int
	Alarm    SlotShare
	Correct  SlotShare
	Measures *ReadingMeasures
}

// Admit applies the rule of admission to a report. It admits nothing on a report whose verdicts
// module-kit did not give (amendment of 2026-11-03 to R07 spec), on one of another capability than
// reading handwriting, on one that refused a sample — a run that refuses a sample does not pass
// (spec's contracts text, section 13) — and on one that is not of a pair. Then the stage full is both
// bounds within FullTarget, and the stage author no false alarm and no false "correct" on
// AuthorSlots and AuthorWrongSlots slots at least. The measures of the first error and the marks of an essay, which a
// synthetic sample's code chose (amendment of 2026-11-08 to R95 spec), a run of reading does not
// hold, and the rule counts none.
func Admit(r *evalv1.Report) Decision {
	d := Decision{Set: setOf(r), Repeats: int(r.GetRepeats())}
	for _, m := range r.GetReaders() {
		d.Readers = append(d.Readers, m.GetProvider()+" "+m.GetModel())
	}
	switch {
	case r.GetVerdicts().GetCode() != evalv1.VerdictCode_VERDICT_CODE_MODULE_KIT:
		d.Why = "its verdicts are not module-kit's"
		return d
	case r.GetCapability() != evalv1.Capability_CAPABILITY_HANDWRITING_READING:
		d.Why = "it is not a run of reading handwriting"
		return d
	case len(d.Readers) != 2:
		// A sheet's answer slots are read by a pair, and a reading of one reader is no reading of the
		// platform's (R68 spec).
		d.Why = fmt.Sprintf("it names %d readers, not the pair of a reading", len(d.Readers))
		return d
	}
	refused := 0
	alarm, correct := slotCounts{}, slotCounts{}
	for _, s := range r.GetSamples() {
		if _, ok := s.GetResult().(*evalv1.SampleResult_Refused); ok {
			refused++
			continue
		}
		for _, slot := range s.GetMeasured().GetAnswerSlots() {
			t := verdict.Verdict(slot.GetTruth().GetVerdict())
			var read, isAlarm, isCorrect bool
			for _, rd := range slot.GetReadings() {
				v := verdict.Verdict(rd.GetVerdict().GetVerdict())
				if v == verdict.Unchecked {
					continue
				}
				read = true
				isAlarm = isAlarm || v == verdict.NotYet
				isCorrect = isCorrect || v == verdict.Correct
			}
			if !read {
				continue
			}
			if t != verdict.NotYet {
				alarm.add(isAlarm)
			}
			if t != verdict.Correct {
				correct.add(isCorrect)
			}
		}
	}
	d.Alarm, d.Correct = alarm.share(), correct.share()
	d.Measures = MeasuresOf(r).Reading
	switch {
	case refused > 0:
		d.Why = fmt.Sprintf("it refused %d samples", refused)
	case d.Alarm.Upper <= FullTarget && d.Correct.Upper <= FullTarget:
		d.Stage = admission.StageFull
	case d.Alarm.Events == 0 && d.Correct.Events == 0 && d.Alarm.Of >= AuthorSlots && d.Correct.Of >= AuthorWrongSlots:
		d.Stage = admission.StageAuthor
	case d.Alarm.Events > 0 || d.Correct.Events > 0:
		d.Why = fmt.Sprintf("%d false alarms and %d false correct", d.Alarm.Events, d.Correct.Events)
	case d.Alarm.Of < AuthorSlots:
		d.Why = fmt.Sprintf("%d slots a false alarm could happen on, fewer than %d", d.Alarm.Of, AuthorSlots)
	default:
		d.Why = fmt.Sprintf("%d slots a false correct could happen on, fewer than %d", d.Correct.Of, AuthorWrongSlots)
	}
	return d
}

// slotCounts are the events and the totals of a share by answer slot, one each.
type slotCounts struct{ events, totals []int }

func (c *slotCounts) add(event bool) {
	e := 0
	if event {
		e = 1
	}
	c.events, c.totals = append(c.events, e), append(c.totals, 1)
}

func (c slotCounts) share() SlotShare {
	s := SlotShare{Of: len(c.totals), Upper: Upper(c.events, c.totals)}
	for _, e := range c.events {
		s.Events += e
	}
	return s
}

// setOf is the name of a report's eval set: a capability's set by its name, or a module's by its ID
// and commit.
func setOf(r *evalv1.Report) string {
	if m := r.GetModule(); m != nil {
		return m.GetModule() + "@" + m.GetCommit()
	}
	return r.GetCapabilitySet().GetName()
}
