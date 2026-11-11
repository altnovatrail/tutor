package tutor

import (
	"context"
	"strings"
	"testing"

	"github.com/altnovatrail/spec/types/formatv0"
	"github.com/altnovatrail/spec/types/formatv0/open"

	observationv1 "github.com/altnovatrail/spec/contracts/novatrail/observation/v1"

	"github.com/altnovatrail/tutor/admission"
	"github.com/altnovatrail/tutor/internal/workbook"
	"github.com/altnovatrail/tutor/provider"
)

// pageOf is the open view of a workbook page of two items: 7 × [ ] = 56, and [ ] + [ ] = 10.
func pageOf() *open.Task {
	box := formatv0.LinePartBindingBox
	return &open.Task{
		ID:     "page",
		Canvas: open.Canvas{Kind: formatv0.CanvasKindWorkbook},
		Items: []open.Item{
			{Line: []open.LinePart{{Text: text("7 × ")}, {Binding: &box}, {Text: text(" = 56")}}},
			{Line: []open.LinePart{{Binding: &box}, {Text: text(" + ")}, {Binding: &box}, {Text: text(" = 10")}}},
		},
	}
}

// admitWorkbook adds to the list an entry of the pair that measured the prompt of the workbook, for
// the test alone.
func admitWorkbook(t *testing.T) {
	t.Helper()
	saved := admission.Admitted
	t.Cleanup(func() { admission.Admitted = saved })
	e := saved[0]
	e.Prompt = workbook.Fingerprint()
	admission.Admitted = append([]admission.Entry{e}, saved...)
}

const cells = `{"cells": [{"item": 1, "slot": 1, "text": "8", "legible": true, "other_readings": [], "mirrored": false, "blank": false}, {"item": 2, "slot": 2, "text": "4", "legible": true, "other_readings": [], "mirrored": false, "blank": false}]}`

// The reading of a sheet admits nothing of the workbook: another prompt is another reading.
func TestReadWorkbookNotAdmitted(t *testing.T) {
	lib, opus, _ := readers(t)
	got := lib.ReadWorkbook(context.Background(), WorkbookReading{Route: platform, Task: pageOf(), Boxes: []BoxCrop{{Item: 1, Slot: 1, Crop: cropPNG(t, 40, 30)}}})
	if got.Outcome.Kind != NotAdmitted || len(opus.asked) != 0 {
		t.Errorf("the workbook without its entry: %+v, asked %d", got.Outcome, len(opus.asked))
	}
}

// Both readers read the crops of the filled boxes, bare, and each gives the observation of every box
// of the page: the blank one unreadable, blank.
func TestReadWorkbook(t *testing.T) {
	admitWorkbook(t)
	lib, opus, haiku := readers(t)
	opus.resp.JSON, haiku.resp.JSON = cells, cells
	got := lib.ReadWorkbook(context.Background(), WorkbookReading{Route: platform, Task: pageOf(),
		Boxes: []BoxCrop{{Item: 1, Slot: 1, Crop: cropPNG(t, 40, 30)}, {Item: 2, Slot: 2, Crop: cropPNG(t, 30, 30)}}})
	if got.Outcome.Kind != Answered || len(got.Observations) != 2 || len(got.Outcome.Usage) != 2 {
		t.Fatalf("the reading: %+v", got.Outcome)
	}
	for _, obs := range got.Observations {
		s := obs.GetAnswerSlots()
		if len(s) != 3 || s[0].GetValue() != "8" || s[1].GetUnreadable() != observationv1.Unreadable_UNREADABLE_BLANK || s[2].GetValue() != "4" {
			t.Errorf("the observation is %v", s)
		}
	}
	req := opus.asked[0]
	if len(req.Images) != 2 || req.Images[0].Label != "Image 1: item 1, slot 1." || req.Images[1].Label != "Image 2: item 2, slot 2." ||
		strings.Contains(req.Instructions, "line of its item") || !strings.Contains(req.Instructions, "The learner may be a child.") || req.MaxOutputTokens != 16000 {
		t.Errorf("the request is %+v", req)
	}
	// An answer without the cell of a filled box is no observation, and fails the reading.
	haiku.resp.JSON = `{"cells": [{"item": 1, "slot": 1, "text": "8", "legible": true, "other_readings": [], "mirrored": false, "blank": false}]}`
	if got := lib.ReadWorkbook(context.Background(), WorkbookReading{Route: platform, Task: pageOf(),
		Boxes: []BoxCrop{{Item: 1, Slot: 1, Crop: cropPNG(t, 40, 30)}, {Item: 2, Slot: 2, Crop: cropPNG(t, 30, 30)}}}); got.Outcome.Kind != Failed || len(got.Observations) != 0 {
		t.Errorf("a missing cell: %+v", got)
	}
	// A page with no filled box goes to no model.
	asked := len(opus.asked)
	if got := lib.ReadWorkbook(context.Background(), WorkbookReading{Route: platform, Task: pageOf()}); got.Outcome.Kind != Answered || len(got.Observations) != 2 ||
		len(opus.asked) != asked || got.Observations[0].GetAnswerSlots()[2].GetUnreadable() != observationv1.Unreadable_UNREADABLE_BLANK {
		t.Errorf("a blank page: %+v", got)
	}
}

// What is no workbook page, or no box of it, is refused before any call.
func TestReadWorkbookRefused(t *testing.T) {
	admitWorkbook(t)
	lib, opus, _ := readers(t)
	crop := cropPNG(t, 20, 20)
	for name, r := range map[string]WorkbookReading{
		"a sheet":          {Route: platform, Task: sheetOf(), Boxes: []BoxCrop{{Item: 1, Slot: 1, Crop: crop}}},
		"no task":          {Route: platform},
		"no such box":      {Route: platform, Task: pageOf(), Boxes: []BoxCrop{{Item: 1, Slot: 2, Crop: crop}}},
		"a box twice":      {Route: platform, Task: pageOf(), Boxes: []BoxCrop{{Item: 1, Slot: 1, Crop: crop}, {Item: 1, Slot: 1, Crop: crop}}},
		"no PNG":           {Route: platform, Task: pageOf(), Boxes: []BoxCrop{{Item: 1, Slot: 1, Crop: []byte("no")}}},
		"a page of no box": {Route: platform, Task: &open.Task{Canvas: open.Canvas{Kind: formatv0.CanvasKindWorkbook}, Items: []open.Item{{Line: []open.LinePart{{Text: text("x")}}}}}},
	} {
		if got := lib.ReadWorkbook(context.Background(), r); got.Outcome.Kind != Failed {
			t.Errorf("%s: %+v", name, got.Outcome)
		}
	}
	if len(opus.asked) != 0 {
		t.Errorf("asked %d times", len(opus.asked))
	}
}

// Each kind of call chooses by its own prompt: with an entry of the workbook ahead of the sheet's,
// a sheet is still read by the pair that measured the sheet, and the workbook by its own entry.
func TestCallsChooseByPrompt(t *testing.T) {
	saved := admission.Admitted
	t.Cleanup(func() { admission.Admitted = saved })
	wb := saved[0]
	wb.Prompt, wb.Models = workbook.Fingerprint(), wb.Models[1:]
	admission.Admitted = append([]admission.Entry{wb}, saved...)
	lib, opus, _ := readers(t)
	if got := lib.ReadSheet(context.Background(), SheetReading{Route: platform, Task: sheetOf(), Crop: cropPNG(t, 40, 30)}); got.Outcome.Kind != Answered || len(opus.asked) != 1 {
		t.Errorf("the sheet: %+v, Opus asked %d", got.Outcome, len(opus.asked))
	}
	if e, a, err := lib.ChooseCall(CallWorkbook, platform); err != nil || !a.Admitted || len(e.Models) != 1 || e.Models[0].ID != "claude-haiku-5-5" {
		t.Errorf("the workbook chose %+v, %+v, %v", e, a, err)
	}
	if _, _, err := lib.ChooseCall("voice", platform); err == nil {
		t.Error("a call the library has none of is chosen")
	}
	priced, _, _ := pricedPair(t)
	p, _, err := priced.Price(CallWorkbook, platform)
	if err != nil || !p.Worst || p.Expected || !near(p.WorstUSD, (float64(24000+20*19*19)*0.1+16000*0.5)/1e6) {
		t.Errorf("the price of the workbook is %+v, %v", p, err)
	}
}

// A crop larger than a box, or more boxes than a reading shows, is refused before any call; each
// reader of a blank page has an observation of its own.
func TestReadWorkbookBounds(t *testing.T) {
	admitWorkbook(t)
	lib, opus, _ := readers(t)
	box := formatv0.LinePartBindingBox
	wide := &open.Task{Canvas: open.Canvas{Kind: formatv0.CanvasKindWorkbook}}
	var boxes []BoxCrop
	for i := 1; i <= maxBoxes+1; i++ {
		wide.Items = append(wide.Items, open.Item{Line: []open.LinePart{{Binding: &box}}})
		boxes = append(boxes, BoxCrop{Item: i, Slot: 1, Crop: cropPNG(t, 20, 20)})
	}
	for name, r := range map[string]WorkbookReading{
		"too many boxes":  {Route: platform, Task: wide, Boxes: boxes},
		"a crop too wide": {Route: platform, Task: pageOf(), Boxes: []BoxCrop{{Item: 1, Slot: 1, Crop: cropPNG(t, maxBoxSide+1, 20)}}},
	} {
		if got := lib.ReadWorkbook(context.Background(), r); got.Outcome.Kind != Failed {
			t.Errorf("%s: %+v", name, got.Outcome)
		}
	}
	got := lib.ReadWorkbook(context.Background(), WorkbookReading{Route: platform, Task: pageOf()})
	if len(got.Observations) != 2 || got.Observations[0] == got.Observations[1] || len(opus.asked) != 0 {
		t.Errorf("a blank page: %+v, asked %d", got, len(opus.asked))
	}
}

// The labels of the pictures count in the bound of the text.
func TestCheckTextLabels(t *testing.T) {
	r := provider.Request{Text: strings.Repeat("a", maxTextBytes-10), Images: []provider.Image{{Label: strings.Repeat("b", 11)}}}
	if checkText(r) == nil {
		t.Error("labels past the bound are not counted")
	}
}
