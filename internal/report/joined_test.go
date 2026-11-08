package report_test

import (
	"strings"
	"testing"

	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"

	"github.com/altnovatrail/tutor/internal/report"
)

// part is a report of a part of a set: of one reader, its samples measured or refused.
func part(set string, samples ...string) *evalv1.Report {
	r := &evalv1.Report{Protocol: "novatrail.eval.v1", Runner: "v0.1.10", Capability: evalv1.Capability_CAPABILITY_HANDWRITING_READING,
		Readers:  []*evalv1.Model{{Provider: "anthropic", Model: "claude-haiku-5-5 low"}},
		Verdicts: &evalv1.VerdictSource{Code: evalv1.VerdictCode_VERDICT_CODE_MODULE_KIT, Version: "v0.2.3"},
		EvalSet:  &evalv1.Report_CapabilitySet{CapabilitySet: &evalv1.CapabilityEvalSet{Name: set}}, Repeats: 1}
	for _, s := range samples {
		r.Samples = append(r.Samples, &evalv1.SampleResult{Sample: s, Result: &evalv1.SampleResult_Measured{Measured: &evalv1.Measurement{}}})
	}
	return r
}

// The reports of the parts of a set join into one report of the set, its samples all, ascending by
// ID; reports of other heads, of a module's set, or that share a sample, do not join.
func TestJoined(t *testing.T) {
	sheets, pages := part("canvas-all", "s-3", "s-1"), part("canvas-workbook-all", "s-2")
	j, err := report.Joined("canvas", sheets, pages)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, s := range j.GetSamples() {
		ids = append(ids, s.GetSample())
	}
	if strings.Join(ids, " ") != "s-1 s-2 s-3" || j.GetCapabilitySet().GetName() != "canvas" || j.GetRunner() != "v0.1.10" || len(j.GetReaders()) != 1 {
		t.Errorf("joined: %v", j)
	}
	if sheets.GetCapabilitySet().GetName() != "canvas-all" || len(sheets.GetSamples()) != 2 {
		t.Error("a part changed")
	}
	if one, err := report.Joined("canvas", pages); err != nil || one.GetCapabilitySet().GetName() != "canvas" || len(one.GetSamples()) != 1 {
		t.Errorf("one part: %v %v", one, err)
	}
	other := part("x", "s-4")
	other.Repeats = 2
	module := part("x", "s-5")
	module.EvalSet = &evalv1.Report_Module{Module: &evalv1.ModuleEvalSet{}}
	for name, c := range map[string]struct {
		parts []*evalv1.Report
		want  string
	}{
		"none":            {nil, "no report"},
		"other repeats":   {[]*evalv1.Report{sheets, other}, "another runner"},
		"a module's set":  {[]*evalv1.Report{sheets, module}, "no eval set of a capability"},
		"a shared sample": {[]*evalv1.Report{sheets, part("x", "s-1")}, "the sample s-1 is in two"},
	} {
		if _, err := report.Joined("canvas", c.parts...); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want %q", name, err, c.want)
		}
	}
}
