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
