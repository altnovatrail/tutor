package tutor

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/altnovatrail/spec/types/admissionv0"
	"github.com/altnovatrail/spec/types/formatv0"
	"github.com/altnovatrail/spec/types/formatv0/open"

	"github.com/altnovatrail/tutor/admission"
	"github.com/altnovatrail/tutor/internal/hint"
	"github.com/altnovatrail/tutor/provider"
)

// withWords admits Opus 5.5 at high for the capabilities named alone, for a test: the entries of
// the words of the platform's list (T127) are left out, so that a test says which it admits.
func withWords(t *testing.T, capabilities ...admissionv0.Capability) {
	t.Helper()
	old := admission.Admitted
	t.Cleanup(func() { admission.Admitted = old })
	list := slices.DeleteFunc(slices.Clone(old), func(e admission.Entry) bool {
		return e.Capability == admissionv0.CapabilitySolutionAnalysis || e.Capability == admissionv0.CapabilityHint
	})
	for _, c := range capabilities {
		list = append(list, admission.Entry{Capability: c, Stage: admission.StageAuthor, Prompt: hint.Fingerprint(),
			Models: []admission.Model{{Provider: "anthropic", ID: "claude-opus-5-5", Effort: "high"}}})
	}
	admission.Admitted = list
}

// tutorOf is a library at the stage author whose Opus answers as told.
func tutorOf(t *testing.T, answer string) (*Tutor, *fake) {
	t.Helper()
	opus := &fake{resp: provider.Response{JSON: answer, Model: "claude-opus-5-5", InputTokens: 2400, OutputTokens: 300}}
	return library(t, adultFacts, &fakes{models: map[string]*fake{"claude-opus-5-5": opus}}), opus
}

func wordsOf(crop []byte) WordsRequest {
	return WordsRequest{Route: platform, Task: sheetOf(), Crop: crop, Key: "15", Solution: []string{"12 + 3 = 15"},
		Traps: []Trap{{ID: "number-from-text", Description: "took a number from the text"}, {ID: "stopped-early", Description: "stopped a step early"}}, Found: "stopped-early",
		Techniques: []string{"a scheme of segments"}, Rung: 2, Language: "en", Style: "an adult", Asked: []string{"What did you find first?"}}
}

const answered = `{"box": {"x1": 30, "y1": 12, "x2": 150, "y2": 60}, "trap": "stopped-early", "hint": "Your first line is right. Whose age does the question ask for?"}`

func TestWords(t *testing.T) {
	withWords(t, admissionv0.CapabilitySolutionAnalysis, admissionv0.CapabilityHint)
	lib, opus := tutorOf(t, answered)
	crop := cropPNG(t, 300, 120)
	got := lib.Words(context.Background(), wordsOf(crop))
	if got.Outcome.Kind != Answered || got.Trap != "stopped-early" || !strings.HasPrefix(got.Question, "Your first line is right.") || len(got.Outcome.Usage) != 1 {
		t.Fatalf("the words are %+v", got)
	}
	// The box in pixels of the crop is its place in thousandths of it.
	if b := got.Box; b == nil || b.GetYmin() != 100 || b.GetXmin() != 100 || b.GetYmax() != 500 || b.GetXmax() != 500 {
		t.Errorf("the box is %v", got.Box)
	}
	r := opus.asked[0]
	if r.NoPicture || len(r.Images) != 1 || r.Effort != "high" || r.MaxOutputTokens != wordsMaxOutput ||
		!strings.Contains(r.Text, "Rung: 2, a question on the rule or the trap.\nLanguage: English.\nWords: no more than 35.\nLearner: an adult.") ||
		!strings.Contains(r.Text, "Trap the program found: stopped-early") || !strings.Contains(r.Text, "- What did you find first?") {
		t.Errorf("the request is %+v", r)
	}

	// «Explain it another way»: no work, no picture, and no box, whatever the model says.
	got = lib.Words(context.Background(), wordsOf(nil))
	if got.Outcome.Kind != Answered || got.Box != nil || !opus.asked[1].NoPicture || len(opus.asked[1].Images) != 0 {
		t.Errorf("without the work: %+v, %+v", got, opus.asked[1])
	}
}

// An answer off the schema, or naming a trap the request does not hold, fails the call, with what
// it took.
func TestWordsFails(t *testing.T) {
	withWords(t, admissionv0.CapabilitySolutionAnalysis, admissionv0.CapabilityHint)
	crop := cropPNG(t, 300, 120)
	for name, answer := range map[string]string{
		"off the schema": `{"hint": "What next?"}`,
		"another trap":   `{"box": null, "trap": "made-up", "hint": "What next?"}`,
	} {
		lib, _ := tutorOf(t, answer)
		if got := lib.Words(context.Background(), wordsOf(crop)); got.Outcome.Kind != Failed || len(got.Outcome.Usage) != 1 || got.Question != "" {
			t.Errorf("%s: %+v", name, got)
		}
	}
}

// The analysis needs the hint admitted too; with no model of the words admitted, nothing is asked.
func TestWordsNotAdmitted(t *testing.T) {
	crop := cropPNG(t, 300, 120)
	lib, opus := tutorOf(t, answered)
	withWords(t)
	if got := lib.Words(context.Background(), wordsOf(crop)); got.Outcome.Kind != NotAdmitted || *got.Outcome.Admission.Reason != admission.ReasonModelNotAdmitted {
		t.Errorf("no model of the words: %+v", got)
	}
	withWords(t, admissionv0.CapabilitySolutionAnalysis)
	if got := lib.Words(context.Background(), wordsOf(crop)); got.Outcome.Kind != NotAdmitted {
		t.Errorf("the analysis without the hint: %+v", got)
	}
	if got := lib.Words(context.Background(), wordsOf(nil)); got.Outcome.Kind != NotAdmitted {
		t.Errorf("the hint alone, not admitted: %+v", got)
	}
	// The hint admitted, but of another model than the analysis: the analysis's model may not word it.
	admission.Admitted = append(admission.Admitted, admission.Entry{Capability: admissionv0.CapabilityHint, Stage: admission.StageAuthor, Prompt: hint.Fingerprint(),
		Models: []admission.Model{{Provider: "anthropic", ID: "claude-haiku-5-5", Effort: "medium"}}})
	if got := lib.Words(context.Background(), wordsOf(crop)); got.Outcome.Kind != NotAdmitted || *got.Outcome.Admission.Reason != admission.ReasonModelNotAdmitted {
		t.Errorf("the hint of another model: %+v", got)
	}
	bad := wordsOf(crop)
	bad.Route = Route{AI: admissionv0.AiRouteOwnKey, Category: admissionv0.AgeCategoryAdult}
	if got := lib.Words(context.Background(), bad); got.Outcome.Kind != Failed {
		t.Errorf("an own key with no key: %+v", got)
	}
	if len(opus.asked) != 0 {
		t.Errorf("%d calls asked though not admitted", len(opus.asked))
	}
}

// What the words cannot ask fails before any call.
func TestWordsRefuses(t *testing.T) {
	withWords(t, admissionv0.CapabilitySolutionAnalysis, admissionv0.CapabilityHint)
	lib, opus := tutorOf(t, answered)
	crop := cropPNG(t, 30, 30)
	blank := " "
	for name, change := range map[string]func(*WordsRequest){
		"no task": func(w *WordsRequest) { w.Task = nil },
		"no words": func(w *WordsRequest) {
			w.Task.Statement = []open.StatementPart{{Alt: &blank}}
			w.Task.Items[1].Text = nil
		},
		"a bad crop":              func(w *WordsRequest) { w.Crop = []byte("no png") },
		"rung 4":                  func(w *WordsRequest) { w.Rung = 4 },
		"no language":             func(w *WordsRequest) { w.Language = "!!" },
		"no style":                func(w *WordsRequest) { w.Style = " " },
		"a trap found of no list": func(w *WordsRequest) { w.Found = "stoped-early" },
	} {
		w := wordsOf(crop)
		change(&w)
		if got := lib.Words(context.Background(), w); got.Outcome.Kind != Failed || got.Outcome.Err == nil {
			t.Errorf("%s: %+v", name, got)
		}
	}
	if len(opus.asked) != 0 {
		t.Errorf("%d calls asked for requests refused", len(opus.asked))
	}
	// A task with a question in an item still words the statement alone.
	w := wordsOf(crop)
	w.Task.Items = append(w.Task.Items, open.Item{Text: text("Item text"), AnswerSlots: []open.AnswerSlot{{Binding: formatv0.AnswerSlotBindingChoice}}})
	if got := lib.Words(context.Background(), w); got.Outcome.Kind != Answered || strings.Contains(opus.asked[0].Text, "Item text") ||
		!strings.Contains(opus.asked[0].Text, "How old is Ann now?\nHow old is Ann now? Write the answer.\nRung:") {
		t.Errorf("a task with an item's text: %+v\n%s", got, opus.asked[0].Text)
	}
	// An empty crop is no work: the rung put another way.
	w = wordsOf([]byte{})
	if got := lib.Words(context.Background(), w); got.Outcome.Kind != Answered || !opus.asked[1].NoPicture {
		t.Errorf("an empty crop: %+v", got)
	}
}
