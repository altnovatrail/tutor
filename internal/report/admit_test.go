package report_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"

	"github.com/altnovatrail/tutor/admission"
	"github.com/altnovatrail/tutor/internal/protocol"
	"github.com/altnovatrail/tutor/internal/protocol/protocoltest"
	"github.com/altnovatrail/tutor/internal/report"
)

// slots is a run of reading of n sheets of one answer slot each, whose truth is truth, each read
// in every repeat as reads says: reads[i] is the verdict of repeat i.
type slots struct {
	n     int
	truth string
	reads []string
}

// admissionReport is a report of reading by a pair, with module-kit's verdicts, of the slots.
func admissionReport(sets ...slots) *evalv1.Report {
	r := &evalv1.Report{
		Protocol:   protocol.Protocol,
		Runner:     "v0.1.13",
		Capability: evalv1.Capability_CAPABILITY_HANDWRITING_READING,
		Readers:    []*evalv1.Model{{Provider: "anthropic", Model: "claude-opus-5-5 high"}, {Provider: "anthropic", Model: "claude-haiku-5-5 medium"}},
		Verdicts:   &evalv1.VerdictSource{Code: evalv1.VerdictCode_VERDICT_CODE_MODULE_KIT, Version: "v0.2.3"},
		EvalSet:    &evalv1.Report_CapabilitySet{CapabilitySet: &evalv1.CapabilityEvalSet{Name: "author-all"}},
		Repeats:    3,
	}
	id := 0
	for _, s := range sets {
		for range s.n {
			id++
			slot := &evalv1.AnswerSlotResult{Truth: &evalv1.Verdict{Verdict: s.truth}}
			for _, v := range s.reads {
				rd := &evalv1.Reading{Verdict: &evalv1.Verdict{Verdict: v}}
				if v == "unchecked" {
					rd.Unchecked = evalv1.UncheckedCause_UNCHECKED_CAUSE_FAILURE
				}
				slot.Readings = append(slot.Readings, rd)
			}
			r.Samples = append(r.Samples, &evalv1.SampleResult{Sample: fmt.Sprintf("s-%03d", id),
				Result: &evalv1.SampleResult_Measured{Measured: &evalv1.Measurement{AnswerSlots: []*evalv1.AnswerSlotResult{slot}}}})
		}
	}
	return r
}

var right3 = []string{"correct", "correct", "correct"}

func TestAdmitStages(t *testing.T) {
	cases := []struct {
		name  string
		r     *evalv1.Report
		stage admission.Stage
		why   string
	}{
		{"three hundred clean slots of each truth reach the target", admissionReport(slots{300, "correct", right3}, slots{300, "not_yet", []string{"not_yet", "not_yet", "not_yet"}}), admission.StageFull, ""},
		{"the bound of false correct holds it back", admissionReport(slots{300, "correct", right3}, slots{30, "not_yet", []string{"not_yet", "not_yet", "not_yet"}}), admission.StageAuthor, ""},
		{"sixty clean slots are the author's stage", admissionReport(slots{60, "correct", right3}, slots{40, "not_yet", []string{"not_yet", "unreadable", "not_yet"}}), admission.StageAuthor, ""},
		{"a false alarm in one repeat", admissionReport(slots{59, "correct", right3}, slots{1, "correct", []string{"correct", "not_yet", "correct"}}), "", "1 false alarms"},
		{"a false correct", admissionReport(slots{60, "correct", right3}, slots{1, "not_yet", []string{"not_yet", "correct", "not_yet"}}), "", "1 false correct"},
		{"too few slots", admissionReport(slots{29, "correct", right3}, slots{30, "not_yet", []string{"not_yet", "not_yet", "not_yet"}}), "", "29 slots"},
		{"no slot to give a false correct", admissionReport(slots{60, "correct", right3}, slots{19, "not_yet", []string{"not_yet", "not_yet", "not_yet"}}), "", "19 slots a false correct"},
		{"a slot never read is not counted", admissionReport(slots{29, "correct", right3}, slots{5, "correct", []string{"unchecked", "unchecked", "unchecked"}}, slots{30, "not_yet", []string{"not_yet", "not_yet", "not_yet"}}), "", "29 slots"},
		{"an unreadable truth counts among the alarms", admissionReport(slots{25, "correct", right3}, slots{5, "unreadable", []string{"unreadable", "unreadable", "unreadable"}}, slots{20, "not_yet", []string{"not_yet", "not_yet", "not_yet"}}), admission.StageAuthor, ""},
	}
	for _, c := range cases {
		d := report.Admit(c.r)
		if d.Stage != c.stage || !strings.Contains(d.Why, c.why) {
			t.Errorf("%s: stage %q, why %q; want %q, %q", c.name, d.Stage, d.Why, c.stage, c.why)
		}
	}
	d := report.Admit(admissionReport(slots{60, "correct", right3}, slots{40, "not_yet", []string{"not_yet", "unreadable", "unchecked"}}))
	// Sixty slots of no event: the bound is 1 − 0.05^(1/60), counted by slot, not by reading.
	if want := 1 - math.Pow(0.05, 1.0/60); d.Alarm.Of != 60 || math.Abs(d.Alarm.Upper-want) > 1e-6 || d.Correct.Of != 40 {
		t.Errorf("the shares are %+v and %+v, want 0 of 60 bound %.4f and 0 of 40", d.Alarm, d.Correct, want)
	}
	if d.Set != "author-all" || d.Repeats != 3 || strings.Join(d.Readers, " + ") != "anthropic claude-opus-5-5 high + anthropic claude-haiku-5-5 medium" {
		t.Errorf("the decision names %q, %d repeats, %v", d.Set, d.Repeats, d.Readers)
	}
	if m := d.Measures.All; m.Readings != 300 || m.Unchecked[evalv1.UncheckedCause_UNCHECKED_CAUSE_FAILURE] != 40 || m.Needless.Events != 40 {
		t.Errorf("the measures of section 14 are %+v", m)
	}
}

func TestAdmitRefuses(t *testing.T) {
	clean := func() *evalv1.Report {
		return admissionReport(slots{300, "correct", right3}, slots{300, "not_yet", []string{"not_yet", "not_yet", "not_yet"}})
	}
	cases := map[string]func(*evalv1.Report){
		"verdicts":        func(r *evalv1.Report) { r.Verdicts.Code = evalv1.VerdictCode_VERDICT_CODE_RUNNER },
		"handwriting":     func(r *evalv1.Report) { r.Capability = evalv1.Capability_CAPABILITY_RUBRIC_ASSESSMENT },
		"names 0 readers": func(r *evalv1.Report) { r.Readers = nil },
		"names 1 readers": func(r *evalv1.Report) { r.Readers = r.Readers[:1] },
		"refused 1": func(r *evalv1.Report) {
			r.Samples = append(r.Samples, &evalv1.SampleResult{Sample: "child", Result: &evalv1.SampleResult_Refused{Refused: evalv1.Refusal_REFUSAL_CHILD}})
		},
	}
	for want, change := range cases {
		r := clean()
		change(r)
		if d := report.Admit(r); d.Stage != "" || !strings.Contains(d.Why, want) {
			t.Errorf("%s: stage %q, why %q", want, d.Stage, d.Why)
		}
	}
	module := clean()
	module.EvalSet = &evalv1.Report_Module{Module: &evalv1.ModuleEvalSet{Module: "olympiad-math", Commit: "abc"}}
	if d := report.Admit(module); d.Set != "olympiad-math@abc" || d.Stage != admission.StageFull {
		t.Errorf("a module's set is named %q, stage %q", d.Set, d.Stage)
	}
}

// spec's example reports read as the rule reads any. Their verdicts are the runner's own, which
// admit nothing; with module-kit's, the reading still refuses a child's sample, and the assessment
// is no reading of handwriting.
func TestAdmitExamples(t *testing.T) {
	ex := protocoltest.Examples(t)
	for _, c := range []struct{ name, why string }{
		{"report-reading.json", "refused 1"},
		{"report-assessment.json", "not a run of reading"},
	} {
		f, err := ex.Open(c.name)
		if err != nil {
			t.Fatal(err)
		}
		r, err := protocol.ReadReport(f)
		_ = f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if d := report.Admit(r); d.Stage != "" || !strings.Contains(d.Why, "not module-kit") {
			t.Errorf("%s: stage %q, why %q", c.name, d.Stage, d.Why)
		}
		r.Verdicts.Code = evalv1.VerdictCode_VERDICT_CODE_MODULE_KIT
		if d := report.Admit(r); d.Stage != "" || !strings.Contains(d.Why, c.why) {
			t.Errorf("%s with module-kit verdicts: stage %q, why %q", c.name, d.Stage, d.Why)
		}
	}
}

// pageReport is a report of reading the workbook by a pair, with module-kit's verdicts: one page of
// the boxes of every kind, each box read in every repeat as its kind says, the boxes noted mirrored
// or crossed out as their markup notes them.
func pageReport(boxes ...pageBoxes) *evalv1.Report {
	r := admissionReport()
	r.EvalSet = &evalv1.Report_CapabilitySet{CapabilitySet: &evalv1.CapabilityEvalSet{Name: report.WorkbookSet}}
	page := &evalv1.Measurement{}
	for _, b := range boxes {
		for range b.n {
			slot := &evalv1.AnswerSlotResult{Truth: &evalv1.Verdict{Verdict: b.truth}, Mirrored: b.mirrored, CrossedOut: b.crossed}
			for _, v := range b.reads {
				slot.Readings = append(slot.Readings, &evalv1.Reading{Verdict: &evalv1.Verdict{Verdict: v}})
			}
			page.AnswerSlots = append(page.AnswerSlots, slot)
		}
	}
	r.Samples = []*evalv1.SampleResult{{Sample: "page-021", Result: &evalv1.SampleResult_Measured{Measured: page}}}
	return r
}

// pageBoxes are n boxes of a page whose truth is truth, read as reads says, mirrored or crossed out.
type pageBoxes struct {
	n                 int
	truth             string
	reads             []string
	mirrored, crossed bool
}

var wrong3 = []string{"not_yet", "not_yet", "not_yet"}

// The rule of the workbook counts the boxes an adult writes, and the mirrored and the crossed-out
// boxes apart, with no target: an alarm on them takes no stage away. It gives the stage author alone,
// though the bounds come within the target of the stage full.
func TestAdmitWorkbook(t *testing.T) {
	adult := []pageBoxes{{30, "correct", right3, false, false}, {20, "not_yet", wrong3, false, false}}
	apart := []pageBoxes{
		{4, "unreadable", []string{"unreadable", "not_yet", "correct"}, true, false},
		{3, "correct", []string{"correct", "not_yet", "correct"}, false, true},
	}
	d := report.AdmitWorkbook(pageReport(append(adult, apart...)...))
	if d.Stage != admission.StageAuthor || d.Why != "" || d.Alarm != (report.SlotShare{Of: 30, Upper: d.Alarm.Upper}) || d.Correct.Of != 20 || d.Correct.Events != 0 {
		t.Errorf("the boxes an adult writes are %+v and %+v, stage %q, why %q", d.Alarm, d.Correct, d.Stage, d.Why)
	}
	if m := d.Mirrored; m.Alarm.Events != 4 || m.Alarm.Of != 4 || m.Correct.Events != 4 || m.Correct.Of != 4 {
		t.Errorf("the mirrored boxes are %+v", m)
	}
	if c := d.CrossedOut; c.Alarm.Events != 3 || c.Alarm.Of != 3 || c.Correct.Of != 0 {
		t.Errorf("the crossed-out boxes are %+v", c)
	}
	if m := d.Measures.All; m.Readings != 3*57 {
		t.Errorf("the measures of section 14 count %d readings, not every box's", m.Readings)
	}
	// Admit's rule rules on no set of the workbook, and over all the boxes it would take the alarms on
	// the mirrored and crossed-out boxes.
	page := pageReport(append(adult, apart...)...)
	if s := report.Admit(page); s.Stage != "" || s.Why != "it is a run of the workbook, whose rule is AdmitWorkbook (R124)" {
		t.Errorf("Admit on the page is %q, %q", s.Stage, s.Why)
	}
	page.EvalSet = &evalv1.Report_CapabilitySet{CapabilitySet: &evalv1.CapabilityEvalSet{Name: "author-all"}}
	if s := report.Admit(page); s.Stage != "" || !strings.Contains(s.Why, "7 false alarms and 4 false correct") {
		t.Errorf("Admit on the page of another set is %q, %q", s.Stage, s.Why)
	}
	full := report.AdmitWorkbook(pageReport(pageBoxes{n: 300, truth: "correct", reads: right3}, pageBoxes{n: 300, truth: "not_yet", reads: wrong3}))
	if full.Stage != admission.StageAuthor || full.Alarm.Upper > report.FullTarget || full.Correct.Upper > report.FullTarget {
		t.Errorf("the bounds %.4f and %.4f give the stage %q", full.Alarm.Upper, full.Correct.Upper, full.Stage)
	}
	for _, c := range []struct {
		name  string
		boxes []pageBoxes
		why   string
	}{
		{"an alarm on a box an adult writes", append([]pageBoxes{{1, "correct", []string{"correct", "correct", "not_yet"}, false, false}}, adult...), "1 false alarms and 0 false correct"},
		{"a false correct on a near key", append([]pageBoxes{{1, "not_yet", []string{"not_yet", "correct", "not_yet"}, false, false}}, adult...), "0 false alarms and 1 false correct"},
		{"too few boxes an adult writes", []pageBoxes{{29, "correct", right3, false, false}, {20, "not_yet", wrong3, false, false}, {40, "correct", right3, false, true}}, "29 slots a false alarm"},
		{"too few for a false correct", []pageBoxes{{30, "correct", right3, false, false}, {19, "not_yet", wrong3, false, false}, {40, "unreadable", []string{"unreadable", "unreadable", "unreadable"}, true, false}}, "19 slots a false correct"},
	} {
		if d := report.AdmitWorkbook(pageReport(c.boxes...)); d.Stage != "" || !strings.Contains(d.Why, c.why) {
			t.Errorf("%s: stage %q, why %q; want %q", c.name, d.Stage, d.Why, c.why)
		}
	}
}

// The rule of the workbook refuses as Admit refuses: verdicts not module-kit's, another capability,
// no pair, a refused sample; and a set other than the test part of eval/workbook-gen.json, and a run
// its cap stopped short, whose boxes not reached would drop out of the count.
func TestAdmitWorkbookRefuses(t *testing.T) {
	cases := map[string]func(*evalv1.Report){
		"verdicts":        func(r *evalv1.Report) { r.Verdicts.Code = evalv1.VerdictCode_VERDICT_CODE_RUNNER },
		"handwriting":     func(r *evalv1.Report) { r.Capability = evalv1.Capability_CAPABILITY_RUBRIC_ASSESSMENT },
		"names 1 readers": func(r *evalv1.Report) { r.Readers = r.Readers[:1] },
		"refused 1": func(r *evalv1.Report) {
			r.Samples = append(r.Samples, &evalv1.SampleResult{Sample: "page-022", Result: &evalv1.SampleResult_Refused{Refused: evalv1.Refusal_REFUSAL_CHILD}})
		},
		"its set is workbook-gen-all, not workbook-gen-test": func(r *evalv1.Report) {
			r.EvalSet = &evalv1.Report_CapabilitySet{CapabilitySet: &evalv1.CapabilityEvalSet{Name: "workbook-gen-all"}}
		},
		"stopped short by its cap: 2 readings": func(r *evalv1.Report) {
			slot := r.Samples[0].GetMeasured().AnswerSlots[0]
			for _, rd := range slot.Readings[1:] {
				rd.Verdict.Verdict, rd.Unchecked = "unchecked", evalv1.UncheckedCause_UNCHECKED_CAUSE_LIMIT
			}
		},
	}
	for want, change := range cases {
		r := pageReport(pageBoxes{n: 30, truth: "correct", reads: right3}, pageBoxes{n: 20, truth: "not_yet", reads: wrong3})
		change(r)
		if d := report.AdmitWorkbook(r); d.Stage != "" || !strings.Contains(d.Why, want) {
			t.Errorf("%s: stage %q, why %q", want, d.Stage, d.Why)
		}
	}
}
