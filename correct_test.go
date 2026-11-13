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
	"github.com/altnovatrail/tutor/internal/correct"
	"github.com/altnovatrail/tutor/provider"
)

// withCorrect admits Opus 5.5 at high for the words after a correct answer alone, with the prompt
// named, for a test: the platform's list has no entry of them until their admission (T144).
func withCorrect(t *testing.T, prompt string) {
	t.Helper()
	old := admission.Admitted
	t.Cleanup(func() { admission.Admitted = old })
	list := slices.DeleteFunc(slices.Clone(old), func(e admission.Entry) bool {
		return e.Capability == admissionv0.CapabilityCorrectAnswerWords
	})
	if prompt != "" {
		list = append(list, admission.Entry{Capability: admissionv0.CapabilityCorrectAnswerWords, Stage: admission.StageAuthor, Prompt: prompt,
			Models: []admission.Model{{Provider: "anthropic", ID: "claude-opus-5-5", Effort: "high"}}})
	}
	admission.Admitted = list
}

func correctOf(crop []byte) CorrectRequest {
	return CorrectRequest{Route: platform, Task: sheetOf(), Crop: crop, Key: "15", Solution: []string{"12 + 3 = 15"},
		Techniques: []string{"a scheme of segments"}, Language: "en", Style: "an adult"}
}

const praised = `{"words": "The first line finds Ben's age before Ann's. Which picture helped you see it?"}`

func TestCorrectAnswerWords(t *testing.T) {
	withCorrect(t, correct.Fingerprint())
	lib, opus := tutorOf(t, praised)
	got := lib.CorrectAnswerWords(context.Background(), correctOf(cropPNG(t, 300, 120)))
	if got.Outcome.Kind != Answered || !strings.HasPrefix(got.Words, "The first line") || len(got.Outcome.Usage) != 1 {
		t.Fatalf("the words are %+v", got)
	}
	r := opus.asked[0]
	if r.NoPicture || len(r.Images) != 1 || r.Effort != "high" || r.MaxOutputTokens != wordsMaxOutput || r.SchemaName != "correct_answer_words" ||
		!strings.Contains(r.Text, "How old is Ann now? Write the answer.\nLanguage: English.\nWords: no more than 35.\nLearner: an adult.") ||
		!strings.Contains(r.Text, "- a scheme of segments") {
		t.Errorf("the request is %+v", r)
	}

	// A choice: no work, no picture, and the item of the choice is what the task asks.
	choice := correctOf(nil)
	choice.Task.Items = []open.Item{{Text: text("Pick Ann's age."), AnswerSlots: []open.AnswerSlot{{Binding: formatv0.AnswerSlotBindingChoice}}}}
	choice.Techniques = nil
	got = lib.CorrectAnswerWords(context.Background(), choice)
	if r := opus.asked[1]; got.Outcome.Kind != Answered || !r.NoPicture || len(r.Images) != 0 || !strings.Contains(r.Text, "How old is Ann now?\nPick Ann's age.\n") {
		t.Errorf("without the work: %+v, %+v", got, r)
	}
}

// An answer off the schema fails the call, with what it took, and gives no words.
func TestCorrectAnswerWordsFails(t *testing.T) {
	withCorrect(t, correct.Fingerprint())
	for name, answer := range map[string]string{
		"off the schema": `{"hint": "What next?"}`,
		"no words":       `{}`,
	} {
		lib, _ := tutorOf(t, answer)
		if got := lib.CorrectAnswerWords(context.Background(), correctOf(nil)); got.Outcome.Kind != Failed || len(got.Outcome.Usage) != 1 || got.Words != "" {
			t.Errorf("%s: %+v", name, got)
		}
	}
}

// With no entry of the words after a correct answer, or one of another prompt, nothing is asked:
// they are a kind of call of their own (R112), and neither the hint's entry nor the analysis's admits
// them.
func TestCorrectAnswerWordsNotAdmitted(t *testing.T) {
	lib, opus := tutorOf(t, praised)
	for name, prompt := range map[string]string{"no entry": "", "another prompt": "old"} {
		withCorrect(t, prompt)
		if got := lib.CorrectAnswerWords(context.Background(), correctOf(nil)); got.Outcome.Kind != NotAdmitted || *got.Outcome.Admission.Reason != admission.ReasonModelNotAdmitted {
			t.Errorf("%s: %+v", name, got)
		}
		if _, a, err := lib.ChooseCall(CallCorrect, platform); err != nil || a.Admitted {
			t.Errorf("%s: ChooseCall = %+v, %v", name, a, err)
		}
		if p, a, err := lib.Price(CallCorrect, platform); err != nil || a.Admitted || p != (CallPrice{}) {
			t.Errorf("%s: Price = %+v, %+v, %v", name, p, a, err)
		}
	}
	bad := correctOf(nil)
	bad.Route = Route{AI: admissionv0.AiRouteOwnKey, Category: admissionv0.AgeCategoryAdult}
	if got := lib.CorrectAnswerWords(context.Background(), bad); got.Outcome.Kind != Failed {
		t.Errorf("an own key with no key: %+v", got)
	}
	if len(opus.asked) != 0 {
		t.Errorf("%d calls asked though not admitted", len(opus.asked))
	}
}

// What the words cannot ask fails before any call.
func TestCorrectAnswerWordsRefuses(t *testing.T) {
	withCorrect(t, correct.Fingerprint())
	lib, opus := tutorOf(t, praised)
	blank := " "
	for name, change := range map[string]func(*CorrectRequest){
		"no task": func(c *CorrectRequest) { c.Task = nil },
		"no words": func(c *CorrectRequest) {
			c.Task.Statement = []open.StatementPart{{Alt: &blank}}
			c.Task.Items[1].Text = nil
		},
		"a bad crop":   func(c *CorrectRequest) { c.Crop = []byte("no png") },
		"no language":  func(c *CorrectRequest) { c.Language = "und" },
		"no style":     func(c *CorrectRequest) { c.Style = " " },
		"a long style": func(c *CorrectRequest) { c.Style = strings.Repeat("a", 2*maxTextBytes) },
	} {
		c := correctOf(cropPNG(t, 30, 30))
		change(&c)
		if got := lib.CorrectAnswerWords(context.Background(), c); got.Outcome.Kind != Failed || got.Outcome.Err == nil {
			t.Errorf("%s: %+v", name, got)
		}
	}
	if len(opus.asked) != 0 {
		t.Errorf("%d calls asked for requests refused", len(opus.asked))
	}
}

// The words after a correct answer are one model's: ChooseCall takes their entry, Price is one call
// of it with no expected tokens, at worst the bound of the text and the largest crop, and the check
// of a key asks the model.
func TestCorrectAnswerWordsCall(t *testing.T) {
	withCorrect(t, correct.Fingerprint())
	opus := &fake{resp: provider.Response{JSON: praised, Model: "claude-opus-5-5"}, prices: opusPrices}
	haiku := &fake{}
	lib := library(t, adultFacts, &fakes{models: map[string]*fake{"claude-opus-5-5": opus, "claude-haiku-5-5": haiku}})
	if e, a, err := lib.ChooseCall(CallCorrect, platform); err != nil || !a.Admitted || e.Prompt != correct.Fingerprint() || len(e.Models) != 1 {
		t.Errorf("ChooseCall(CallCorrect) = %+v, %+v, %v", e, a, err)
	}
	maxIn := float64(24000 + 147*147)
	if p, a, err := lib.Price(CallCorrect, platform); err != nil || !a.Admitted || p.Expected || !p.Worst || !near(p.WorstUSD, (maxIn*5+16000*20)/1e6) {
		t.Errorf("Price(CallCorrect) = %+v, %+v, %v", p, a, err)
	}
	admission.Admitted = slices.DeleteFunc(slices.Clone(admission.Admitted), func(e admission.Entry) bool {
		return e.Capability != admissionv0.CapabilityCorrectAnswerWords
	})
	if got, err := lib.CheckKey(context.Background(), ownRoute); got != KeyAccepted || err != nil || opus.checked != 1 || haiku.checked != 0 {
		t.Errorf("CheckKey: %v, %v; checked %d and %d", got, err, opus.checked, haiku.checked)
	}
}
