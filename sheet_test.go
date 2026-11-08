package tutor

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"strings"
	"testing"

	"github.com/altnovatrail/spec/types/admissionv0"
	"github.com/altnovatrail/spec/types/formatv0"
	"github.com/altnovatrail/spec/types/formatv0/open"

	observationv1 "github.com/altnovatrail/spec/contracts/novatrail/observation/v1"

	"github.com/altnovatrail/tutor/internal/reading"
	"github.com/altnovatrail/tutor/provider"
)

// cropPNG is a blank crop of the size.
func cropPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewGray(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func text(s string) *formatv0.Text { v := formatv0.Text(s); return &v }

// sheetOf is the open view of a task on a handwriting sheet: a question by choice, then the answer
// written anywhere, as item 2.
func sheetOf() *open.Task {
	alt := "a number line from 0 to 10"
	return &open.Task{
		ID:        "ages",
		Statement: []open.StatementPart{{Text: text("Ann is 3 years older than Ben.")}, {Alt: &alt}, {Text: text("How old is Ann now?")}},
		Canvas:    open.Canvas{Kind: formatv0.CanvasKindHandwritingSheet},
		Items: []open.Item{
			{AnswerSlots: []open.AnswerSlot{{Binding: formatv0.AnswerSlotBindingChoice}}},
			{Text: text("How old is Ann now? Write the answer."), AnswerSlots: []open.AnswerSlot{{Binding: formatv0.AnswerSlotBindingAnywhere}}},
		},
	}
}

const observed = `{"parts": [{"kind": "text", "text": "Ann: 12 + 3 = 15", "legible": true, "other_readings": []}], "results": [{"part": 1, "value": "15", "other_values": []}]}`

func readers(t *testing.T) (*Tutor, *fake, *fake) {
	t.Helper()
	opus := &fake{resp: provider.Response{JSON: observed, Model: "claude-opus-5-5", InputTokens: 1500, OutputTokens: 200}}
	haiku := &fake{resp: provider.Response{JSON: observed, Model: "claude-haiku-5-5", InputTokens: 1500, OutputTokens: 600}}
	lib := library(t, adultFacts, &fakes{models: map[string]*fake{"claude-opus-5-5": opus, "claude-haiku-5-5": haiku}})
	return lib, opus, haiku
}

func TestReadSheet(t *testing.T) {
	lib, opus, haiku := readers(t)
	crop := cropPNG(t, 300, 120)
	got := lib.ReadSheet(context.Background(), SheetReading{Route: platform, Task: sheetOf(), Crop: crop})
	if got.Outcome.Kind != Answered || len(got.Observations) != 2 || len(got.Outcome.Usage) != 2 {
		t.Fatalf("the reading is %+v", got)
	}
	if got.Observations[0].GetAnswerSlots()[0].GetAnswerSlot() == got.Observations[1].GetAnswerSlots()[0].GetAnswerSlot() {
		t.Error("the two observations share their place")
	}
	for i, o := range got.Observations {
		s := o.GetAnswerSlots()
		if len(s) != 1 || s[0].GetAnswerSlot().GetItem() != 2 || s[0].GetAnswerSlot().GetSlot() != 1 || s[0].GetValue() != "15" {
			t.Errorf("reader %d observed %v", i+1, o)
		}
	}
	// Both readers see the crop and the statement, with the picture by its words, and no key; each at
	// its effort, writing no more than the reading's bound.
	for _, r := range []provider.Request{opus.asked[0], haiku.asked[0]} {
		if !bytes.Equal(r.Images[0].PNG, crop) || !strings.Contains(r.Text, "Image 1 is 300 pixels wide and 120 pixels high.") ||
			!strings.Contains(r.Text, "Task: Ann is 3 years older than Ben.\na number line from 0 to 10\nHow old is Ann now?\nHow old is Ann now? Write the answer.") ||
			strings.Contains(r.Text, "key_and_solution") || strings.Contains(r.Instructions, "key_and_solution") ||
			!strings.Contains(r.Instructions, "The learner may be a child.") || r.MaxOutputTokens != reading.MaxOutputTokens {
			t.Errorf("the request is %+v", r)
		}
	}
	if opus.asked[0].Effort != "high" || haiku.asked[0].Effort != "medium" {
		t.Errorf("the efforts are %q and %q", opus.asked[0].Effort, haiku.asked[0].Effort)
	}

	// A reader that sees no final answer gives the reason, not a value.
	haiku.resp.JSON = `{"parts": [{"kind": "text", "text": "12 + 3", "legible": true, "other_readings": []}], "results": []}`
	got = lib.ReadSheet(context.Background(), SheetReading{Route: platform, Task: sheetOf(), Crop: crop})
	if got.Outcome.Kind != Answered || got.Observations[1].GetAnswerSlots()[0].GetUnreadable() != observationv1.Unreadable_UNREADABLE_NO_FINAL_ANSWER {
		t.Errorf("no final answer: %+v", got)
	}
}

// A reader that answers no observation, or fails, leaves no observation of the pair: the outcome
// is Failed, with what the calls that came back took.
func TestReadSheetFails(t *testing.T) {
	lib, _, haiku := readers(t)
	crop := cropPNG(t, 40, 40)
	haiku.resp.JSON = `{"parts": []}`
	got := lib.ReadSheet(context.Background(), SheetReading{Route: platform, Task: sheetOf(), Crop: crop})
	if got.Outcome.Kind != Failed || got.Observations != nil || len(got.Outcome.Usage) != 2 || !strings.Contains(got.Outcome.Err.Error(), "claude-haiku-5-5 medium") {
		t.Errorf("an answer off the schema: %+v", got)
	}
	haiku.resp, haiku.err = provider.Response{}, errors.New("overloaded")
	got = lib.ReadSheet(context.Background(), SheetReading{Route: platform, Task: sheetOf(), Crop: crop})
	if got.Outcome.Kind != Failed || got.Observations != nil || len(got.Outcome.Usage) != 1 {
		t.Errorf("a failed reader: %+v", got)
	}
	// Not admitted: no reader is asked.
	child := Route{AI: admissionv0.AiRoutePlatformAi, Category: admissionv0.AgeCategoryUnder13}
	before := len(haiku.asked)
	got = lib.ReadSheet(context.Background(), SheetReading{Route: child, Task: sheetOf(), Crop: crop})
	if got.Outcome.Kind != NotAdmitted || got.Observations != nil || len(haiku.asked) != before {
		t.Errorf("a child: %+v", got)
	}
}

// A task this reading does not read, and a crop that is no PNG, fail before any call.
func TestReadSheetRefuses(t *testing.T) {
	lib, opus, _ := readers(t)
	crop := cropPNG(t, 40, 40)
	tasks := map[string]func(*open.Task){
		"a workbook":       func(t *open.Task) { t.Canvas.Kind = formatv0.CanvasKindWorkbook },
		"a zone":           func(t *open.Task) { t.Items[0].AnswerSlots[0].Binding = formatv0.AnswerSlotBindingZone },
		"the whole sheet":  func(t *open.Task) { t.Items[0].AnswerSlots[0].Binding = formatv0.AnswerSlotBindingSheet },
		"two anywhere":     func(t *open.Task) { t.Items[0].AnswerSlots[0].Binding = formatv0.AnswerSlotBindingAnywhere },
		"nothing anywhere": func(t *open.Task) { t.Items = t.Items[:1] },
		"no words": func(t *open.Task) {
			blank := " "
			t.Statement = []open.StatementPart{{Text: text("")}, {Alt: &blank}}
			t.Items[1].Text = nil
		},
	}
	for name, change := range tasks {
		task := sheetOf()
		change(task)
		if got := lib.ReadSheet(context.Background(), SheetReading{Route: platform, Task: task, Crop: crop}); got.Outcome.Kind != Failed || got.Outcome.Err == nil {
			t.Errorf("%s: %+v", name, got)
		}
	}
	if got := lib.ReadSheet(context.Background(), SheetReading{Route: platform, Crop: crop}); got.Outcome.Kind != Failed {
		t.Errorf("no task: %+v", got)
	}
	full := cropPNG(t, 30, 30)
	for name, c := range map[string][]byte{"not a PNG": []byte("not a png"), "a broken PNG": full[:len(full)-20], "too large": cropPNG(t, maxCropSide+1, 2)} {
		if got := lib.ReadSheet(context.Background(), SheetReading{Route: platform, Task: sheetOf(), Crop: c}); got.Outcome.Kind != Failed || !strings.Contains(got.Outcome.Err.Error(), "no PNG of a sheet") {
			t.Errorf("%s: %+v", name, got)
		}
	}
	if len(opus.asked) != 0 {
		t.Errorf("%d calls asked for tasks and crops refused", len(opus.asked))
	}
}
