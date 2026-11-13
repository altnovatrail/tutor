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

// The hint without the work is a kind of call of its own (R106): ChooseCall takes the entry of the
// hint that measured the words' prompt, not an entry of the hint on another prompt that stands
// before it, as Choose by capability does, and the words without the work ask that entry's model.
func TestChooseCallHint(t *testing.T) {
	withWords(t, admissionv0.CapabilitySolutionAnalysis, admissionv0.CapabilityHint)
	other := admission.Entry{Capability: admissionv0.CapabilityHint, Stage: admission.StageAuthor, Prompt: "another prompt",
		Models: []admission.Model{{Provider: "anthropic", ID: "claude-haiku-5-5", Effort: "medium"}}}
	admission.Admitted = append([]admission.Entry{other}, admission.Admitted...)
	opus := &fake{resp: provider.Response{JSON: answered, Model: "claude-opus-5-5"}}
	haiku := &fake{resp: provider.Response{JSON: answered, Model: "claude-haiku-5-5"}}
	lib := library(t, adultFacts, &fakes{models: map[string]*fake{"claude-opus-5-5": opus, "claude-haiku-5-5": haiku}})
	if e, a, err := lib.Choose(admissionv0.CapabilityHint, platform); err != nil || !a.Admitted || e.Models[0].ID != "claude-haiku-5-5" {
		t.Errorf("Choose(hint) = %+v, %+v, %v; want the first entry of the hint", e, a, err)
	}
	if e, a, err := lib.ChooseCall(CallHint, platform); err != nil || !a.Admitted || e.Prompt != hint.Fingerprint() || len(e.Models) != 1 || e.Models[0].ID != "claude-opus-5-5" {
		t.Errorf("ChooseCall(CallHint) = %+v, %+v, %v; want the entry of the words' prompt", e, a, err)
	}
	if got := lib.Words(context.Background(), wordsOf(nil)); got.Outcome.Kind != Answered || len(opus.asked) != 1 || len(haiku.asked) != 0 {
		t.Errorf("the hint without the work: %+v, asked %d and %d", got.Outcome, len(opus.asked), len(haiku.asked))
	}
	// An entry of the hint on another prompt alone admits neither.
	admission.Admitted = []admission.Entry{other}
	if _, a, err := lib.ChooseCall(CallHint, platform); err != nil || a.Admitted {
		t.Errorf("ChooseCall(CallHint) of another prompt: %+v, %v", a, err)
	}
	if got := lib.Words(context.Background(), wordsOf(nil)); got.Outcome.Kind != NotAdmitted || len(haiku.asked) != 0 {
		t.Errorf("the hint of another prompt: %+v, asked %d", got.Outcome, len(haiku.asked))
	}
}

// The words with the work are admitted only with the hint on the analysis's model (R84), and every
// place that decides on them holds it by one choice (Д13): with the hint of another model, of
// another effort or on another prompt, neither ChooseCall nor Price nor the call admits the words,
// and the check of a key asks only the models the calls would ask — the hint's own, not the
// analysis's. With the hint on the same model, all of them admit it, as before.
func TestWordsHintSameModel(t *testing.T) {
	opus := admission.Model{Provider: "anthropic", ID: "claude-opus-5-5", Effort: "high"}
	for _, tt := range []struct {
		name        string
		hint        admission.Entry
		admitted    bool
		opus, haiku int
		key         KeyCheck
	}{
		{"the same model", admission.Entry{Prompt: hint.Fingerprint(), Models: []admission.Model{opus}}, true, 1, 0, KeyAccepted},
		{"another model", admission.Entry{Prompt: hint.Fingerprint(), Models: []admission.Model{{Provider: "anthropic", ID: "claude-haiku-5-5", Effort: "medium"}}}, false, 0, 1, KeyAccepted},
		{"another effort", admission.Entry{Prompt: hint.Fingerprint(), Models: []admission.Model{{Provider: "anthropic", ID: "claude-opus-5-5", Effort: "medium"}}}, false, 1, 0, KeyAccepted},
		{"another prompt", admission.Entry{Prompt: "another prompt", Models: []admission.Model{opus}}, false, 0, 0, KeyProviderClosed},
	} {
		withWords(t, admissionv0.CapabilitySolutionAnalysis)
		h := tt.hint
		h.Capability, h.Stage = admissionv0.CapabilityHint, admission.StageAuthor
		// The analysis and the hint alone, so that the check of a key asks no reader of a sheet.
		admission.Admitted = append(slices.DeleteFunc(slices.Clone(admission.Admitted), func(e admission.Entry) bool {
			return e.Capability != admissionv0.CapabilitySolutionAnalysis
		}), h)
		lib, asked := tutorOf(t, answered)
		haiku := &fake{}
		lib.adapters["anthropic"] = (&fakes{models: map[string]*fake{"claude-opus-5-5": asked, "claude-haiku-5-5": haiku}}).dial

		e, a, err := lib.ChooseCall(CallWords, platform)
		if err != nil || a.Admitted != tt.admitted || (tt.admitted && (len(e.Models) != 1 || e.Models[0] != opus)) || (!tt.admitted && (e.Models != nil || a.Reason == nil)) {
			t.Errorf("%s: ChooseCall(CallWords) = %+v, %+v, %v", tt.name, e, a, err)
		}
		if p, a, err := lib.Price(CallWords, platform); err != nil || a.Admitted != tt.admitted || (!tt.admitted && p != (CallPrice{})) {
			t.Errorf("%s: Price(CallWords) = %+v, %+v, %v", tt.name, p, a, err)
		}
		got := lib.Words(context.Background(), wordsOf(cropPNG(t, 300, 120)))
		if want := map[bool]Kind{true: Answered, false: NotAdmitted}[tt.admitted]; got.Outcome.Kind != want || got.Outcome.Admission.Admitted != tt.admitted || len(asked.asked) != map[bool]int{true: 1, false: 0}[tt.admitted] {
			t.Errorf("%s: the words are %+v, asked %d", tt.name, got.Outcome, len(asked.asked))
		}
		if k, err := lib.CheckKey(context.Background(), ownRoute); k != tt.key || err != nil || asked.checked != tt.opus || haiku.checked != tt.haiku {
			t.Errorf("%s: CheckKey = %v, %v; checked Opus %d and Haiku %d times", tt.name, k, err, asked.checked, haiku.checked)
		}
	}
}
