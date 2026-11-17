package tutor

import (
	"context"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/admission"
)

// single is the library of readers in the mode of one reader, with its two fakes.
func single(t *testing.T) (*Tutor, *fake, *fake) {
	t.Helper()
	lib, opus, haiku := readers(t)
	lib.reading = ReadingSingle
	return lib, opus, haiku
}

// An empty mode of reading is the pair, and a mode that is neither of the two makes no library.
func TestNewReading(t *testing.T) {
	table, err := admission.Platform()
	if err != nil {
		t.Fatal(err)
	}
	for mode, want := range map[Reading]Reading{"": ReadingPair, ReadingPair: ReadingPair, ReadingSingle: ReadingSingle} {
		lib, err := New(Config{Table: table, Reading: mode})
		if err != nil || lib.reading != want {
			t.Errorf("the mode %q: %v, %+v; want %q", mode, err, lib, want)
		}
	}
	if _, err := New(Config{Table: table, Reading: "trio"}); err == nil || !strings.Contains(err.Error(), `"trio"`) {
		t.Errorf("a mode of trio: %v", err)
	}
}

// In the mode of one reader a sheet goes to Haiku at medium alone, and its observation is the
// reading; Opus is not asked and has no client.
func TestReadSheetSingle(t *testing.T) {
	lib, opus, haiku := single(t)
	got := lib.ReadSheet(context.Background(), SheetReading{Route: platform, Task: sheetOf(), Crop: cropPNG(t, 300, 120)})
	if got.Outcome.Kind != Answered || len(got.Observations) != 1 || len(got.Outcome.Usage) != 1 || got.Outcome.Usage[0].Model != "claude-haiku-5-5" {
		t.Fatalf("the reading is %+v", got)
	}
	if s := got.Observations[0].GetAnswerSlots(); len(s) != 1 || s[0].GetValue() != "15" || s[0].GetAnswerSlot().GetItem() != 2 {
		t.Errorf("the observation is %v", got.Observations[0])
	}
	if len(opus.asked) != 0 || len(haiku.asked) != 1 || haiku.asked[0].Effort != "medium" {
		t.Errorf("asked Opus %d times and Haiku %d", len(opus.asked), len(haiku.asked))
	}
	if _, ok := lib.clients[admission.Model{Provider: "anthropic", ID: "claude-opus-5-5", Effort: "high"}]; ok {
		t.Error("the mode of one reader made a client of Opus")
	}
	e, a, err := lib.ChooseCall(CallSheet, platform)
	if err != nil || !a.Admitted || len(e.Models) != 1 || e.Models[0].Name() != "claude-haiku-5-5 medium" {
		t.Errorf("the choice of a sheet: %+v, %+v, %v", e, a, err)
	}
	// The words and the reading of an essay ask what they asked: Opus.
	for _, c := range []Call{CallWords, CallEssay} {
		if c == CallEssay {
			admitEssay(t)
		}
		if e, a, err := lib.ChooseCall(c, platform); err != nil || !a.Admitted || len(e.Models) != 1 || e.Models[0].ID != "claude-opus-5-5" {
			t.Errorf("the choice of %s: %+v, %+v, %v", c, e, a, err)
		}
	}
}

// In the mode of one reader a workbook page goes to Haiku at medium alone, and a page with no filled
// box gives the one observation of the blanks.
func TestReadWorkbookSingle(t *testing.T) {
	admitWorkbook(t)
	lib, opus, haiku := single(t)
	haiku.resp.JSON = cells
	got := lib.ReadWorkbook(context.Background(), WorkbookReading{Route: platform, Task: pageOf(),
		Boxes: []BoxCrop{{Item: 1, Slot: 1, Crop: cropPNG(t, 40, 30)}, {Item: 2, Slot: 2, Crop: cropPNG(t, 30, 30)}}})
	if got.Outcome.Kind != Answered || len(got.Observations) != 1 || len(got.Outcome.Usage) != 1 || len(opus.asked) != 0 || len(haiku.asked) != 1 {
		t.Fatalf("the reading: %+v, asked Opus %d times", got, len(opus.asked))
	}
	if s := got.Observations[0].GetAnswerSlots(); len(s) != 3 || s[0].GetValue() != "8" || s[2].GetValue() != "4" {
		t.Errorf("the observation is %v", s)
	}
	blank := lib.ReadWorkbook(context.Background(), WorkbookReading{Route: platform, Task: pageOf()})
	if blank.Outcome.Kind != Answered || len(blank.Observations) != 1 || len(haiku.asked) != 1 {
		t.Errorf("a blank page: %+v", blank)
	}
}

// The price of a reading in the mode of one reader is Haiku's alone: expected by its tokens of the
// run that admitted the pair, at worst by the most input and output of the kind.
func TestPriceSingle(t *testing.T) {
	lib, _, _ := pricedPair(t)
	lib.reading = ReadingSingle
	p, a, err := lib.Price(CallSheet, platform)
	if err != nil || !a.Admitted {
		t.Fatalf("the price of a reading: %v, %+v", err, a)
	}
	maxIn := float64(24000 + 147*147)
	expected, worst := (1354*0.1+502*0.5)/1e6, (maxIn*0.1+16000*0.5)/1e6
	if !p.Expected || !p.Worst || !near(p.ExpectedUSD, expected) || !near(p.WorstUSD, worst) {
		t.Errorf("price = %+v; want expected %v and worst %v", p, expected, worst)
	}
}

// The one reader is found as the list names it, not by its place in the pair: an entry whose pair
// has Haiku first still gives Haiku, and an entry without Haiku at medium is a failure of the call,
// before any model is asked, with an error that names the reader the mode wants.
func TestSingleReaderOfEntry(t *testing.T) {
	saved := admission.Admitted
	t.Cleanup(func() { admission.Admitted = saved })
	lib, opus, haiku := single(t)

	swapped := saved[0]
	swapped.Models = []admission.Model{saved[0].Models[1], saved[0].Models[0]}
	admission.Admitted = append([]admission.Entry{swapped}, saved...)
	if e, _, err := lib.ChooseCall(CallSheet, platform); err != nil || len(e.Models) != 1 || e.Models[0].ID != "claude-haiku-5-5" {
		t.Errorf("Haiku first: %+v, %v", e, err)
	}

	low := saved[0]
	low.Models = []admission.Model{saved[0].Models[0], {Provider: "anthropic", ID: "claude-haiku-5-5", Effort: "low"}}
	admission.Admitted = append([]admission.Entry{low}, saved...)
	got := lib.ReadSheet(context.Background(), SheetReading{Route: platform, Task: sheetOf(), Crop: cropPNG(t, 40, 40)})
	if got.Outcome.Kind != Failed || got.Observations != nil || len(got.Outcome.Usage) != 0 || len(opus.asked)+len(haiku.asked) != 0 ||
		got.Outcome.Err == nil || !strings.Contains(got.Outcome.Err.Error(), "has no claude-haiku-5-5 medium") {
		t.Errorf("an entry without Haiku at medium: %+v", got.Outcome)
	}
	if _, _, err := lib.ChooseCall(CallSheet, platform); err == nil {
		t.Error("the choice of an entry without Haiku at medium has no error")
	}
	if _, _, err := lib.Price(CallSheet, platform); err == nil {
		t.Error("the price of an entry without Haiku at medium has no error")
	}
	// The pair reads by the same entry as it is.
	lib.reading = ReadingPair
	if e, _, err := lib.ChooseCall(CallSheet, platform); err != nil || len(e.Models) != 2 {
		t.Errorf("the pair of the entry: %+v, %v", e, err)
	}
}

// In the mode of one reader the key is checked for Haiku and for the models of the other kinds, and
// a reading whose entry has no Haiku at medium is left out of the check, not its end.
func TestCheckKeySingle(t *testing.T) {
	saved := admission.Admitted
	t.Cleanup(func() { admission.Admitted = saved })
	lib, opus, haiku := single(t)
	if got, err := lib.CheckKey(context.Background(), ownRoute); got != KeyAccepted || err != nil || haiku.checked != 1 || opus.checked != 1 {
		t.Errorf("the check: %v, %v; checked Opus %d times and Haiku %d", got, err, opus.checked, haiku.checked)
	}
	// Opus serves the words alone: the reading without Haiku at medium checks no other reader.
	low := saved[0]
	low.Models = []admission.Model{saved[0].Models[0], {Provider: "anthropic", ID: "claude-haiku-5-5", Effort: "low"}}
	admission.Admitted = append([]admission.Entry{low}, saved...)
	if got, err := lib.CheckKey(context.Background(), ownRoute); got != KeyAccepted || err != nil || haiku.checked != 1 || opus.checked != 2 {
		t.Errorf("an entry without Haiku at medium: %v, %v; checked Opus %d times and Haiku %d", got, err, opus.checked, haiku.checked)
	}
}
