package tutor

import (
	"context"
	"strings"
	"testing"

	"github.com/altnovatrail/spec/types/admissionv0"

	"github.com/altnovatrail/tutor/admission"
	"github.com/altnovatrail/tutor/internal/essay"
)

// admitRubric adds to the list an entry of one model that measured the prompt of the assessment of a
// text, for the test alone.
func admitRubric(t *testing.T) {
	t.Helper()
	saved := admission.Admitted
	t.Cleanup(func() { admission.Admitted = saved })
	e := saved[0]
	e.Capability, e.Prompt, e.Models = admissionv0.CapabilityRubricAssessment, essay.TextFingerprint(), e.Models[:1]
	admission.Admitted = append([]admission.Entry{e}, saved...)
}

var rubricOf = RubricAssessment{Route: platform, Text: "My Cat\nmy cat is big.", Topic: "Write about your pet.", Criteria: []string{"The essay has a title.", "Each sentence begins with a capital letter."}, Style: "a child", Language: "ru"}

// The table of the platform does not open the assessment by a rubric to the pair, nor does any
// entry measure its prompt.
func TestAssessEssayNotAdmitted(t *testing.T) {
	lib, opus, _ := readers(t)
	got := lib.AssessEssay(context.Background(), rubricOf)
	if got.Outcome.Kind != NotAdmitted || len(got.Criteria) != 2 || got.Criteria[0].Met != nil || len(opus.asked) != 0 {
		t.Errorf("without its entry: %+v", got)
	}
}

// The criteria go to the model by number, the text in its block and no picture; each criterion is
// marked by its one judgement, and one judged twice or not at all is without an assessment.
func TestAssessEssay(t *testing.T) {
	admitRubric(t)
	lib, opus, _ := readers(t)
	opus.resp.JSON = `{"criteria": [{"id": "1", "evidence": "My Cat", "met": true}, {"id": "2", "evidence": "my cat", "met": false}], "feedback": "В тексте есть заголовок."}`
	got := lib.AssessEssay(context.Background(), rubricOf)
	if got.Outcome.Kind != Answered || len(got.Criteria) != 2 || got.Criteria[0].Met == nil || !*got.Criteria[0].Met || got.Criteria[0].Evidence != "My Cat" ||
		got.Criteria[1].Met == nil || *got.Criteria[1].Met || got.Feedback != "В тексте есть заголовок." {
		t.Errorf("the assessment: %+v", got)
	}
	req := opus.asked[0]
	for _, want := range []string{"Language: Russian.", "- 1: The essay has a title.", "- 2: Each sentence begins with a capital letter.", "Topic: Write about your pet.", "my cat is big."} {
		if !strings.Contains(req.Text, want) {
			t.Errorf("the request lacks %q:\n%s", want, req.Text)
		}
	}
	if len(req.Images) != 0 || !req.NoPicture || !strings.Contains(req.Instructions, "does not tell whether the learner is a boy or a girl") {
		t.Errorf("the request is %+v", req)
	}
	opus.resp.JSON = `{"criteria": [{"id": "1", "evidence": "a", "met": true}, {"id": "1", "evidence": "b", "met": false}], "feedback": "x"}`
	if got := lib.AssessEssay(context.Background(), rubricOf); got.Outcome.Kind != Answered || got.Criteria[0].Met != nil || got.Criteria[1].Met != nil {
		t.Errorf("a criterion twice and one missing: %+v", got)
	}
	opus.resp.JSON = `{"criteria": []}`
	if got := lib.AssessEssay(context.Background(), rubricOf); got.Outcome.Kind != Failed || len(got.Criteria) != 2 || got.Criteria[0].Met != nil || got.Feedback != "" {
		t.Errorf("an answer off the schema: %+v", got)
	}
}

// What cannot be assessed is refused before any call.
func TestAssessEssayRefused(t *testing.T) {
	admitRubric(t)
	lib, opus, _ := readers(t)
	for name, change := range map[string]func(*RubricAssessment){
		"no text":                       func(r *RubricAssessment) { r.Text = " " },
		"no criterion":                  func(r *RubricAssessment) { r.Criteria = nil },
		"no language":                   func(r *RubricAssessment) { r.Language = "" },
		"a language words cannot check": func(r *RubricAssessment) { r.Language = "x-foo" },
		"no topic":                      func(r *RubricAssessment) { r.Topic = "" },
		"no style":                      func(r *RubricAssessment) { r.Style = " " },
		"a blank criterion":             func(r *RubricAssessment) { r.Criteria = []string{"The essay has a title.", ""} },
		"a long text":                   func(r *RubricAssessment) { r.Text = strings.Repeat("a", maxTextBytes) },
	} {
		r := rubricOf
		change(&r)
		if got := lib.AssessEssay(context.Background(), r); got.Outcome.Kind != Failed || len(got.Criteria) != len(r.Criteria) {
			t.Errorf("%s: %+v", name, got)
		}
	}
	if len(opus.asked) != 0 {
		t.Errorf("asked %d times", len(opus.asked))
	}
}

// The rubric is a call of its own: ChooseCall takes its one model, and its worst price is the bound
// of the text and the output, with no picture.
func TestRubricCall(t *testing.T) {
	admitRubric(t)
	lib, _, _ := pricedPair(t)
	if e, a, err := lib.ChooseCall(CallRubric, platform); err != nil || !a.Admitted || len(e.Models) != 1 || e.Models[0].ID != "claude-opus-5-5" {
		t.Errorf("the rubric chose %+v, %+v, %v", e, a, err)
	}
	if p, _, err := lib.Price(CallRubric, platform); err != nil || !p.Worst || p.Expected || !near(p.WorstUSD, (24000*5+16000*20)/1e6) {
		t.Errorf("the price of the rubric is %+v, %v", p, err)
	}
}
