package report

import (
	"fmt"
	"strings"

	"github.com/altnovatrail/module-kit/verdict"
	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"

	"github.com/altnovatrail/tutor/admission"
)

// The rule of admission (R80; R07 spec; spec's contracts text, sections 13 and 14): which stage a
// runner's report admits its readers to. Only a report of reading handwriting is ruled on: a sheet's
// by Admit, a workbook's by AdmitWorkbook (R124).
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
	// WorkbookSet is the set the rule of the workbook admits on: the test part of
	// eval/workbook-gen.json, pages 21–300, as the runner's report names it, without the dev part,
	// whose pages tuned the prompt (R124).
	WorkbookSet = "workbook-gen-test"
	// workbookSets is how the runner's reports name every set of the workbook, by its files,
	// eval/workbook*.json: Admit rules on none of them.
	workbookSets = "workbook"
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
// (spec's contracts text, section 13) — on one that is not of a pair, and on one of a set of the
// workbook, which AdmitWorkbook rules on (R124). Then the stage full is both
// bounds within FullTarget, and the stage author no false alarm and no false "correct" on
// AuthorSlots and AuthorWrongSlots slots at least. The measures of the first error and the marks of an essay, which a
// synthetic sample's code chose (amendment of 2026-11-08 to R95 spec), a run of reading does not
// hold, and the rule counts none.
func Admit(r *evalv1.Report) Decision {
	d, ok := admissible(r)
	if !ok {
		return d
	}
	if strings.HasPrefix(d.Set, workbookSets) {
		d.Why = "it is a run of the workbook, whose rule is AdmitWorkbook (R124)"
		return d
	}
	var p slotPair
	all := []*slotPair{&p}
	refused, _ := admitSlots(r, func(*evalv1.AnswerSlotResult) []*slotPair { return all })
	d.Alarm, d.Correct = p.alarm.share(), p.correct.share()
	d.Measures = MeasuresOf(r).Reading
	d.Stage, d.Why = stageOf(refused, d.Alarm, d.Correct, true)
	return d
}

// WorkbookDecision is what the rule of the workbook makes of a report of the reading of its pages
// (T155; R124; the 2026-11-16 amendment to R80): the decision on the boxes an adult writes, and the
// boxes with a digit written mirrored and those crossed out and written again, each a row of its
// own, with no target.
type WorkbookDecision struct {
	Decision
	Mirrored, CrossedOut Apart
}

// Apart are the shares of a kind of box the rule counts beside the target and holds to none.
type Apart struct {
	Alarm, Correct SlotShare
}

// AdmitWorkbook applies the rule of the workbook to a report of a run of its pages: Admit's rule by
// box, the answer slot of a page, with the same refusals, on the boxes an adult writes — correct, an
// error, a near key, a similar digit, a fraction stacked or in a line. A box the markup notes
// mirrored or crossed out is no such box: an adult writes no digit mirrored, and code draws a
// crossed-out digit with one short stroke, so that part of its errors comes from the drawing
// (measurements 3.7, 3.12). It counts in its own row and takes no stage away. The rule gives the stage
// author alone: its set is synthetic, and the stage full waits for the samples of adult volunteers
// (the 2026-11-16 amendment to R50; П-16). It admits nothing on a set other than WorkbookSet, nor on
// a run its cap stopped short: an event is one of all the repeats of a box (R124), and a box the run
// did not reach has fewer.
func AdmitWorkbook(r *evalv1.Report) WorkbookDecision {
	d, ok := admissible(r)
	w := WorkbookDecision{Decision: d}
	if !ok {
		return w
	}
	var adult, mirrored, crossed slotPair
	route := map[[2]bool][]*slotPair{
		{false, false}: {&adult}, {true, false}: {&mirrored}, {false, true}: {&crossed}, {true, true}: {&mirrored, &crossed},
	}
	refused, limited := admitSlots(r, func(s *evalv1.AnswerSlotResult) []*slotPair {
		return route[[2]bool{s.GetMirrored(), s.GetCrossedOut()}]
	})
	w.Alarm, w.Correct = adult.alarm.share(), adult.correct.share()
	w.Mirrored = Apart{mirrored.alarm.share(), mirrored.correct.share()}
	w.CrossedOut = Apart{crossed.alarm.share(), crossed.correct.share()}
	w.Measures = MeasuresOf(r).Reading
	w.Stage, w.Why = stageOf(refused, w.Alarm, w.Correct, false)
	switch {
	case refused > 0:
	case limited > 0:
		w.Stage, w.Why = "", fmt.Sprintf("its run stopped short by its cap: %d readings not reached", limited)
	case w.Set != WorkbookSet:
		w.Stage, w.Why = "", fmt.Sprintf("its set is %s, not %s, the test part of eval/workbook-gen.json", w.Set, WorkbookSet)
	}
	return w
}

// admissible is the head of a decision on a report — its readers, set and repeats — and whether the
// report may be ruled on at all: not when its verdicts are not module-kit's, when it is of another
// capability than reading handwriting, or when it is not of a pair, and then why not.
func admissible(r *evalv1.Report) (Decision, bool) {
	d := Decision{Set: setOf(r), Repeats: int(r.GetRepeats())}
	for _, m := range r.GetReaders() {
		d.Readers = append(d.Readers, m.GetProvider()+" "+m.GetModel())
	}
	switch {
	case r.GetVerdicts().GetCode() != evalv1.VerdictCode_VERDICT_CODE_MODULE_KIT:
		d.Why = "its verdicts are not module-kit's"
	case r.GetCapability() != evalv1.Capability_CAPABILITY_HANDWRITING_READING:
		d.Why = "it is not a run of reading handwriting"
	case len(d.Readers) != 2:
		// A sheet's answer slots are read by a pair, and so are a page's boxes, and a reading of one
		// reader is no reading of the platform's (R68 spec).
		d.Why = fmt.Sprintf("it names %d readers, not the pair of a reading", len(d.Readers))
	default:
		return d, true
	}
	return d, false
}

// slotPair are the two shares of a set of answer slots: of false alarms and of false "correct".
type slotPair struct{ alarm, correct slotCounts }

// admitSlots counts each answer slot of the report's measured samples, once over its readings,
// into the shares into gives it, and returns how many samples the report refused and how many
// readings its run did not reach, stopped by its cap.
func admitSlots(r *evalv1.Report, into func(*evalv1.AnswerSlotResult) []*slotPair) (refused, limited int) {
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
					if rd.GetUnchecked() == evalv1.UncheckedCause_UNCHECKED_CAUSE_LIMIT {
						limited++
					}
					continue
				}
				read = true
				isAlarm = isAlarm || v == verdict.NotYet
				isCorrect = isCorrect || v == verdict.Correct
			}
			if !read {
				continue
			}
			for _, p := range into(slot) {
				if t != verdict.NotYet {
					p.alarm.add(isAlarm)
				}
				if t != verdict.Correct {
					p.correct.add(isCorrect)
				}
			}
		}
	}
	return refused, limited
}

// stageOf is the stage the shares of a report admit its readers to, with full the stage full
// allowed, or none and why not: none on a report that refused a sample.
func stageOf(refused int, alarm, correct SlotShare, full bool) (admission.Stage, string) {
	switch {
	case refused > 0:
		return "", fmt.Sprintf("it refused %d samples", refused)
	case full && alarm.Upper <= FullTarget && correct.Upper <= FullTarget:
		return admission.StageFull, ""
	case alarm.Events == 0 && correct.Events == 0 && alarm.Of >= AuthorSlots && correct.Of >= AuthorWrongSlots:
		return admission.StageAuthor, ""
	case alarm.Events > 0 || correct.Events > 0:
		return "", fmt.Sprintf("%d false alarms and %d false correct", alarm.Events, correct.Events)
	case alarm.Of < AuthorSlots:
		return "", fmt.Sprintf("%d slots a false alarm could happen on, fewer than %d", alarm.Of, AuthorSlots)
	}
	return "", fmt.Sprintf("%d slots a false correct could happen on, fewer than %d", correct.Of, AuthorWrongSlots)
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
