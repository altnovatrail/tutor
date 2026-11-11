package tutor

import (
	"context"
	"strings"
	"testing"

	"github.com/altnovatrail/spec/types/formatv0"
	"github.com/altnovatrail/spec/types/formatv0/open"

	observationv1 "github.com/altnovatrail/spec/contracts/novatrail/observation/v1"

	"github.com/altnovatrail/tutor/admission"
	"github.com/altnovatrail/tutor/internal/essay"
)

// essayOf is the open view of a task with an essay: one item, its answer slot bound to the sheet.
func essayOf() *open.Task {
	return &open.Task{
		ID:        "essay",
		Statement: []open.StatementPart{{Text: text("Write about your pet.")}},
		Canvas:    open.Canvas{Kind: formatv0.CanvasKindHandwritingSheet},
		Items:     []open.Item{{AnswerSlots: []open.AnswerSlot{{Binding: formatv0.AnswerSlotBindingSheet}}}},
	}
}

// admitEssay adds to the list an entry of one reader that measured the prompt of the reading of an
// essay, for the test alone.
func admitEssay(t *testing.T) {
	t.Helper()
	saved := admission.Admitted
	t.Cleanup(func() { admission.Admitted = saved })
	e := saved[0]
	e.Prompt, e.Models = essay.ReadFingerprint(), e.Models[:1]
	admission.Admitted = append([]admission.Entry{e}, saved...)
}

// The reading of a sheet admits nothing of an essay.
func TestReadEssayNotAdmitted(t *testing.T) {
	lib, opus, _ := readers(t)
	if got := lib.ReadEssay(context.Background(), EssayReading{Route: platform, Task: essayOf(), Crop: cropPNG(t, 60, 40)}); got.Outcome.Kind != NotAdmitted || len(opus.asked) != 0 {
		t.Errorf("an essay without its entry: %+v", got.Outcome)
	}
}

// One reader reads the crop, without the topic, into the observation of the sheet's answer slot: the
// text as it is written, or unreadable when the essay cannot be read, or is blank.
func TestReadEssay(t *testing.T) {
	admitEssay(t)
	lib, opus, haiku := readers(t)
	for answer, want := range map[string]func(*observationv1.AnswerSlotObservation) bool{
		`{"text": "My Cat\nmy cat is big", "legible": true}`: func(s *observationv1.AnswerSlotObservation) bool { return s.GetValue() == "My Cat\nmy cat is big" },
		`{"text": "", "legible": false}`: func(s *observationv1.AnswerSlotObservation) bool {
			return s.GetUnreadable() == observationv1.Unreadable_UNREADABLE_ILLEGIBLE
		},
		`{"text": "  ", "legible": true}`: func(s *observationv1.AnswerSlotObservation) bool {
			return s.GetUnreadable() == observationv1.Unreadable_UNREADABLE_BLANK
		},
	} {
		opus.resp.JSON = answer
		got := lib.ReadEssay(context.Background(), EssayReading{Route: platform, Task: essayOf(), Crop: cropPNG(t, 60, 40)})
		if got.Outcome.Kind != Answered || len(got.Observation.GetAnswerSlots()) != 1 || !want(got.Observation.GetAnswerSlots()[0]) || got.Observation.GetAnswerSlots()[0].GetAnswerSlot().GetItem() != 1 {
			t.Errorf("%s: %+v", answer, got)
		}
	}
	req := opus.asked[0]
	if len(haiku.asked) != 0 || len(req.Images) != 1 || strings.Contains(req.Text, "pet") || !strings.Contains(req.Instructions, "The learner may be a child.") || req.MaxOutputTokens != 16000 {
		t.Errorf("the request is %+v; Haiku asked %d", req, len(haiku.asked))
	}
	opus.resp.JSON = `{"text": "x"}`
	if got := lib.ReadEssay(context.Background(), EssayReading{Route: platform, Task: essayOf(), Crop: cropPNG(t, 60, 40)}); got.Outcome.Kind != Failed || got.Observation != nil {
		t.Errorf("an answer off the schema: %+v", got)
	}
}

// What is no essay is refused before any call.
func TestReadEssayRefused(t *testing.T) {
	admitEssay(t)
	lib, opus, _ := readers(t)
	two := essayOf()
	two.Items = append(two.Items, two.Items[0])
	for name, r := range map[string]EssayReading{
		"no task":    {Route: platform, Crop: cropPNG(t, 10, 10)},
		"a workbook": {Route: platform, Task: pageOf(), Crop: cropPNG(t, 10, 10)},
		"no essay":   {Route: platform, Task: sheetOf(), Crop: cropPNG(t, 10, 10)},
		"two essays": {Route: platform, Task: two, Crop: cropPNG(t, 10, 10)},
		"no PNG":     {Route: platform, Task: essayOf(), Crop: []byte("no")},
	} {
		if got := lib.ReadEssay(context.Background(), r); got.Outcome.Kind != Failed {
			t.Errorf("%s: %+v", name, got.Outcome)
		}
	}
	if len(opus.asked) != 0 {
		t.Errorf("asked %d times", len(opus.asked))
	}
}

// The essay is a call of its own: ChooseCall and Price take the one model it asks, and the check of
// a key reaches a model admitted for the essay alone.
func TestEssayCall(t *testing.T) {
	saved := admission.Admitted
	t.Cleanup(func() { admission.Admitted = saved })
	e := saved[0]
	e.Prompt = essay.ReadFingerprint()
	e.Models = []admission.Model{{Provider: "anthropic", ID: "claude-sonnet-5-5", Effort: "low"}, saved[0].Models[0]}
	admission.Admitted = append([]admission.Entry{e}, saved...)
	sonnet := &fake{prices: haikuPrices}
	fs := &fakes{models: map[string]*fake{"claude-opus-5-5": {prices: opusPrices}, "claude-haiku-5-5": {prices: haikuPrices}, "claude-sonnet-5-5": sonnet}}
	lib := library(t, adultFacts, fs)
	if e, a, err := lib.ChooseCall(CallEssay, platform); err != nil || !a.Admitted || len(e.Models) != 1 || e.Models[0].ID != "claude-sonnet-5-5" {
		t.Errorf("the essay chose %+v, %+v, %v", e, a, err)
	}
	if p, _, err := lib.Price(CallEssay, platform); err != nil || !p.Worst || !near(p.WorstUSD, (float64(24000+147*147)*0.1+16000*0.5)/1e6) {
		t.Errorf("the price of the essay is %+v, %v", p, err)
	}
	if got, err := lib.CheckKey(context.Background(), ownRoute); got != KeyAccepted || err != nil || sonnet.checked != 1 {
		t.Errorf("the check of the key: %v, %v; Sonnet checked %d", got, err, sonnet.checked)
	}
	mixed := essayOf()
	mixed.Items = append(mixed.Items, open.Item{AnswerSlots: []open.AnswerSlot{{Binding: formatv0.AnswerSlotBindingAnywhere}}})
	if got := lib.ReadEssay(context.Background(), EssayReading{Route: platform, Task: mixed, Crop: cropPNG(t, 10, 10)}); got.Outcome.Kind != Failed || !strings.Contains(got.Outcome.Err.Error(), "anywhere") {
		t.Errorf("an essay with a slot bound anywhere: %+v", got.Outcome)
	}
}
