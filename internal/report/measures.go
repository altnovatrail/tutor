package report

import (
	"slices"

	"github.com/altnovatrail/module-kit/verdict"
	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"
)

// The measures of a runner's report, counted from its records, all repeats together, by the rules
// of section 14 of spec's contracts text (R74): a report holds no totals, and any reader can count
// them again. T is the verdict on the truth, and R the verdict on a reading.

// Count is a share as section 14 counts it: the events among the readings or marks it is counted
// among.
type Count struct {
	Events, Of int
}

// add counts one more among, and one more event when it is one.
func (c *Count) add(event bool) {
	c.Of++
	if event {
		c.Events++
	}
}

// SlotMeasures are the measures of a run of reading on a set of its answer slots, each in a repeat:
// a false alarm, the main measure, a false "correct", a needless "unreadable", an "unreadable" kept
// and another trap, and how many readings are unchecked, by cause, among all of them.
type SlotMeasures struct {
	FalseAlarm, FalseCorrect, Needless, Kept, OtherTrap Count
	Readings                                            int
	Unchecked                                           map[evalv1.UncheckedCause]int
}

// add counts a reading of an answer slot.
func (m *SlotMeasures) add(t, r *evalv1.Verdict, cause evalv1.UncheckedCause) {
	m.Readings++
	tv, rv := verdict.Verdict(t.GetVerdict()), verdict.Verdict(r.GetVerdict())
	if rv == verdict.Unchecked {
		if m.Unchecked == nil {
			m.Unchecked = map[evalv1.UncheckedCause]int{}
		}
		m.Unchecked[cause]++
	} else {
		if tv != verdict.NotYet {
			m.FalseAlarm.add(rv == verdict.NotYet)
		}
		if tv != verdict.Correct {
			m.FalseCorrect.add(rv == verdict.Correct)
		}
		if tv == verdict.Correct || tv == verdict.NotYet {
			m.Needless.add(rv == verdict.Unreadable)
		}
		if tv == verdict.Unreadable {
			m.Kept.add(rv == verdict.Unreadable)
		}
	}
	if tv == verdict.NotYet && rv == verdict.NotYet {
		m.OtherTrap.add(t.GetTrap() != r.GetTrap())
	}
}

// ReadingMeasures are the measures of a run of reading: on all its answer slots, and apart on
// those noted mirrored and on those noted crossed out (R56 spec).
type ReadingMeasures struct {
	All, Mirrored, CrossedOut SlotMeasures
}

// CriterionMeasures are the measures of a criterion of the rubric in a run of assessment: its false
// alarms among the people's marks met, AI's marks that are the people's, its marks of none, and
// Cohen's kappa, nil when there is none, p_e being 1 or no mark counted.
type CriterionMeasures struct {
	Criterion             int
	FalseAlarm, Agreement Count
	None                  int
	Kappa                 *float64
	// marks counts the units by the people's mark and AI's: [people met][AI met].
	marks [2][2]int
}

// EssayMeasures are the measures of a run of assessment: the needless "unreadable" among the
// readings of essays whose truth holds their text that are not unchecked; and, over the criteria the
// people marked, in the repeats whose essay was assessed and that AI marked met or not met, the
// false alarms and the agreement in all, and each criterion's.
type EssayMeasures struct {
	Needless, FalseAlarm, Agreement Count
	None                            int
	Criteria                        []CriterionMeasures
	// Readings are the readings of the essays, and Unchecked those that gave nothing, by cause.
	Readings  int
	Unchecked map[evalv1.UncheckedCause]int
}

// Measures are a report's measures: of reading or of assessment, by its capability.
type Measures struct {
	Reading *ReadingMeasures
	Essay   *EssayMeasures
}

// MeasuresOf counts the measures of a report from its records.
func MeasuresOf(r *evalv1.Report) Measures {
	if r.GetCapability() == evalv1.Capability_CAPABILITY_RUBRIC_ASSESSMENT {
		return Measures{Essay: essayMeasures(r)}
	}
	m := &ReadingMeasures{}
	for _, s := range r.GetSamples() {
		for _, slot := range s.GetMeasured().GetAnswerSlots() {
			for _, rd := range slot.GetReadings() {
				m.All.add(slot.GetTruth(), rd.GetVerdict(), rd.GetUnchecked())
				if slot.GetMirrored() {
					m.Mirrored.add(slot.GetTruth(), rd.GetVerdict(), rd.GetUnchecked())
				}
				if slot.GetCrossedOut() {
					m.CrossedOut.add(slot.GetTruth(), rd.GetVerdict(), rd.GetUnchecked())
				}
			}
		}
	}
	return Measures{Reading: m}
}

// essayMeasures are the measures of a run of assessment.
func essayMeasures(r *evalv1.Report) *EssayMeasures {
	m := &EssayMeasures{}
	at := map[int32]int{}
	criterion := func(n int32) *CriterionMeasures {
		i, ok := at[n]
		if !ok {
			i = len(m.Criteria)
			at[n] = i
			m.Criteria = append(m.Criteria, CriterionMeasures{Criterion: int(n)})
		}
		return &m.Criteria[i]
	}
	for _, s := range r.GetSamples() {
		e := s.GetMeasured().GetEssay()
		if e == nil {
			continue
		}
		people := map[int32]bool{}
		for _, p := range e.GetPeople() {
			people[p.GetCriterion()] = p.GetMet()
		}
		for _, rd := range e.GetReadings() {
			m.Readings++
			if u, ok := rd.GetOutcome().(*evalv1.EssayReading_Unchecked); ok {
				if m.Unchecked == nil {
					m.Unchecked = map[evalv1.UncheckedCause]int{}
				}
				m.Unchecked[u.Unchecked]++
			}
			if e.Truth == nil && rd.GetOutcome() != nil {
				if _, isUnchecked := rd.GetOutcome().(*evalv1.EssayReading_Unchecked); !isUnchecked {
					_, unreadable := rd.GetOutcome().(*evalv1.EssayReading_Unreadable)
					m.Needless.add(unreadable)
				}
			}
			for _, a := range rd.GetAssessment().GetCriteria() {
				met, marked := people[a.GetCriterion()]
				if !marked {
					continue
				}
				c := criterion(a.GetCriterion())
				var ai bool
				switch a.GetAssessment() {
				case evalv1.CriterionAssessment_CRITERION_ASSESSMENT_MET:
					ai = true
				case evalv1.CriterionAssessment_CRITERION_ASSESSMENT_NOT_MET:
				default:
					c.None++
					m.None++
					continue
				}
				if met {
					c.FalseAlarm.add(!ai)
					m.FalseAlarm.add(!ai)
				}
				c.Agreement.add(ai == met)
				m.Agreement.add(ai == met)
				c.marks[boolIndex(met)][boolIndex(ai)]++
			}
		}
	}
	for i := range m.Criteria {
		m.Criteria[i].Kappa = kappaOf(m.Criteria[i].marks)
	}
	slices.SortFunc(m.Criteria, func(a, b CriterionMeasures) int { return a.Criterion - b.Criterion })
	return m
}

// kappaOf is Cohen's kappa of the marks of a criterion, [people met][AI met] (kappa): nil
// when p_e is 1 or there is no mark.
func kappaOf(marks [2][2]int) *float64 {
	n := marks[0][0] + marks[0][1] + marks[1][0] + marks[1][1]
	k, ok := kappa(n, marks[0][0]+marks[1][1], marks[1][0]+marks[1][1], marks[0][1]+marks[1][1])
	if !ok {
		return nil
	}
	return &k
}

// boolIndex is 1 for true and 0 for false.
func boolIndex(b bool) int {
	if b {
		return 1
	}
	return 0
}
