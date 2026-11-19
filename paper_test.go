package tutor

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/internal/reading"
	"github.com/altnovatrail/tutor/internal/solving"
	"github.com/altnovatrail/tutor/internal/statement"
	"github.com/altnovatrail/tutor/provider"
)

// haikuOf is the library with Haiku answering every call with answer, and Opus another.
func haikuOf(t *testing.T, haiku, opus string) (*Tutor, *fake, *fake) {
	t.Helper()
	h := &fake{resp: provider.Response{JSON: haiku, Model: "claude-haiku-5-5", InputTokens: 1200, OutputTokens: 200}}
	o := &fake{resp: provider.Response{JSON: opus, Model: "claude-opus-5-5", InputTokens: 1300, OutputTokens: 150}}
	return library(t, adultFacts, &fakes{models: map[string]*fake{"claude-haiku-5-5": h, "claude-opus-5-5": o}}), h, o
}

// The statement is read by Haiku alone, from the crop sent as a photo, into the formula as given.
func TestReadStatement(t *testing.T) {
	lib, haiku, opus := haikuOf(t, `{"formula": " 37+48 "}`, `{}`)
	got := lib.ReadStatement(context.Background(), PaperTask{Route: platform, Statement: photoJPEG(t, 300, 80)})
	if got.Outcome.Kind != Answered || got.Formula != "37+48" || len(got.Outcome.Usage) != 1 || len(opus.asked) != 0 {
		t.Fatalf("read %+v", got)
	}
	r := haiku.asked[0]
	if r.Effort != "medium" || r.MaxOutputTokens != statement.MaxOutputTokens || len(r.Images) != 1 || len(r.Images[0].JPEG) == 0 ||
		r.Instructions != statement.Instructions() || !strings.Contains(r.Text, "300 pixels wide and 80 pixels high") {
		t.Errorf("the request is %+v", r)
	}
	// A statement that is no formula is an answer.
	haiku.resp.JSON = `{"formula": ""}`
	if got := lib.ReadStatement(context.Background(), PaperTask{Route: platform, Statement: photoJPEG(t, 30, 30)}); got.Outcome.Kind != Answered || got.Formula != "" {
		t.Errorf("no formula: %+v", got)
	}
}

// The solution is Haiku's alone, its answer read into an Answer, nil for none.
func TestSolveTask(t *testing.T) {
	lib, haiku, opus := haikuOf(t, `{"kind": "number", "value": "12", "units": ["cm"], "unknown": "", "options": [], "key": ""}`, `{}`)
	got := lib.SolveTask(context.Background(), PaperTask{Route: platform, Statement: cropPNG(t, 200, 100)})
	if got.Outcome.Kind != Answered || got.Answer.GetNumber().GetValue() != "12" || got.Answer.GetNumber().GetUnits()[0] != "cm" || len(opus.asked) != 0 {
		t.Fatalf("solved %+v", got)
	}
	if r := haiku.asked[0]; r.MaxOutputTokens != solving.MaxOutputTokens || r.Instructions != solving.Instructions() || r.SchemaName != "task_answer" {
		t.Errorf("the request is %+v", r)
	}
	haiku.resp.JSON = `{"kind": "none", "value": "", "units": [], "unknown": "", "options": [], "key": ""}`
	if got := lib.SolveTask(context.Background(), PaperTask{Route: platform, Statement: cropPNG(t, 20, 20)}); got.Outcome.Kind != Answered || got.Answer != nil {
		t.Errorf("no answer: %+v", got)
	}
}

// The failures of the calls of a task from paper: an answer off the schema, a crop that is no
// picture, before any call, a limit and an admission without their entries.
func TestPaperTaskFails(t *testing.T) {
	lib, haiku, _ := haikuOf(t, `{"formula": 1}`, `{}`)
	p := PaperTask{Route: platform, Statement: photoJPEG(t, 30, 30)}
	if got := lib.ReadStatement(context.Background(), p); got.Outcome.Kind != Failed || len(got.Outcome.Usage) != 1 {
		t.Errorf("off the schema: %+v", got)
	}
	if got := lib.SolveTask(context.Background(), p); got.Outcome.Kind != Failed {
		t.Errorf("off the schema of the solution: %+v", got)
	}
	asked := len(haiku.asked)
	if got := lib.ReadStatement(context.Background(), PaperTask{Route: platform, Statement: []byte("no crop")}); got.Outcome.Kind != Failed || len(haiku.asked) != asked {
		t.Errorf("no crop: %+v", got)
	}
	haiku.err = &provider.LimitError{Limit: provider.LimitOwner}
	if got := lib.SolveTask(context.Background(), p); got.Outcome.Kind != LimitReached {
		t.Errorf("a limit: %+v", got)
	}
	haiku.err = errors.New("down")
	if got := lib.ReadStatement(context.Background(), p); got.Outcome.Kind != Failed {
		t.Errorf("a failure: %+v", got)
	}
	withoutWord(t)
	lib, haiku, _ = haikuOf(t, `{"formula": ""}`, `{}`)
	if lib.ReadStatement(context.Background(), p).Outcome.Kind != NotAdmitted || lib.SolveTask(context.Background(), p).Outcome.Kind != NotAdmitted || len(haiku.asked) != 0 {
		t.Error("a call of a task from paper was asked without its entry")
	}
}

const paperObservation = `{"parts": [{"kind": "drawing", "text": "circled: B", "legible": true, "other_readings": []}], "results": [{"part": 1, "value": "B", "other_values": []}]}`

// The work is read by the pair, the strong reader first, each into the one answer slot at 1, 1, by
// the prompt of paper; in ReadingSingle, by Haiku alone.
func TestReadPaper(t *testing.T) {
	lib, haiku, opus := haikuOf(t, paperObservation, paperObservation)
	got := lib.ReadPaper(context.Background(), PaperReading{Route: platform, Crop: cropPNG(t, 400, 300)})
	if got.Outcome.Kind != Answered || len(got.Observations) != 2 || len(got.Outcome.Usage) != 2 {
		t.Fatalf("read %+v", got)
	}
	for _, o := range got.Observations {
		s := o.GetAnswerSlots()[0]
		if s.GetValue() != "B" || s.GetAnswerSlot().GetItem() != 1 || s.GetAnswerSlot().GetSlot() != 1 {
			t.Errorf("the observation is %v", o)
		}
	}
	if r := opus.asked[0]; r.Effort != "high" || r.Instructions != reading.PaperInstructions() || r.Text != "Image 1 is 400 pixels wide and 300 pixels high." || len(haiku.asked) != 1 {
		t.Errorf("the request is %+v", r)
	}
	single, err := New(Config{Table: lib.table, Facts: lib.facts, Least: lib.least, Reading: ReadingSingle, Keys: lib.keys, Adapters: lib.adapters})
	if err != nil {
		t.Fatal(err)
	}
	if got := single.ReadPaper(context.Background(), PaperReading{Route: platform, Crop: cropPNG(t, 40, 30)}); got.Outcome.Kind != Answered || len(got.Observations) != 1 || len(opus.asked) != 1 {
		t.Errorf("single: %+v", got)
	}
	// An answer off the schema fails the reading, and a crop that is no PNG fails before any call.
	opus.resp.JSON = `{}`
	if got := lib.ReadPaper(context.Background(), PaperReading{Route: platform, Crop: cropPNG(t, 40, 30)}); got.Outcome.Kind != Failed || len(got.Observations) != 0 {
		t.Errorf("off the schema: %+v", got)
	}
	if got := lib.ReadPaper(context.Background(), PaperReading{Route: platform, Crop: photoJPEG(t, 40, 30)}); got.Outcome.Kind != Failed || len(opus.asked) != 2 {
		t.Errorf("no PNG: %+v", got)
	}
	opus.err = errors.New("down")
	if got := lib.ReadPaper(context.Background(), PaperReading{Route: platform, Crop: cropPNG(t, 40, 30)}); got.Outcome.Kind != Failed {
		t.Errorf("a failure: %+v", got)
	}
	withoutWord(t)
	lib, _, opus = haikuOf(t, paperObservation, paperObservation)
	if got := lib.ReadPaper(context.Background(), PaperReading{Route: platform, Crop: cropPNG(t, 40, 30)}); got.Outcome.Kind != NotAdmitted || len(opus.asked) != 0 {
		t.Errorf("not admitted: %+v", got)
	}
}
