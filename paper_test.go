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

// haikuOf is the library with Haiku answering every call with answer, and Sonnet and Opus, the later
// attempts of a cascade, another.
func haikuOf(t *testing.T, haiku, opus string) (*Tutor, *fake, *fake) {
	t.Helper()
	lib, m := cascadeOf(t, haiku, opus, opus)
	return lib, m["claude-haiku-5-5"], m["claude-opus-5-5"]
}

// cascadeOf is the library with each model of the cascade of a task from paper answering as named,
// by its ID.
func cascadeOf(t *testing.T, haiku, sonnet, opus string) (*Tutor, map[string]*fake) {
	t.Helper()
	fs := &fakes{models: map[string]*fake{
		"claude-haiku-5-5":  {resp: provider.Response{JSON: haiku, Model: "claude-haiku-5-5", InputTokens: 1200, OutputTokens: 200}},
		"claude-sonnet-5-5": {resp: provider.Response{JSON: sonnet, Model: "claude-sonnet-5-5", InputTokens: 1250, OutputTokens: 180}},
		"claude-opus-5-5":   {resp: provider.Response{JSON: opus, Model: "claude-opus-5-5", InputTokens: 1300, OutputTokens: 150}},
	}}
	return library(t, adultFacts, fs), fs.models
}

// The statement is read by Haiku at high first, from the crop sent as a photo, into its tasks as
// given; a statement that is no formula is its answer, and no later model is asked.
func TestReadStatement(t *testing.T) {
	lib, haiku, opus := haikuOf(t, `{"tasks": [{"statement": " 37 + 48 = ", "formula": " 37+48 "}]}`, `{}`)
	got := lib.ReadStatement(context.Background(), PaperTask{Route: platform, Statement: photoJPEG(t, 300, 80)})
	if got.Outcome.Kind != Answered || len(got.Tasks) != 1 || got.Tasks[0] != (StatementTask{Statement: "37 + 48 =", Formula: "37+48"}) ||
		len(got.Outcome.Usage) != 1 || len(opus.asked) != 0 {
		t.Fatalf("read %+v", got)
	}
	r := haiku.asked[0]
	if r.Effort != "high" || r.MaxOutputTokens != statement.MaxOutputTokens || len(r.Images) != 1 || len(r.Images[0].JPEG) == 0 ||
		r.Instructions != statement.Instructions() || !strings.Contains(r.Text, "300 pixels wide and 80 pixels high") {
		t.Errorf("the request is %+v", r)
	}
	// A statement that is no formula is an answer.
	haiku.resp.JSON = `{"tasks": [{"statement": "Ann has 3 apples.", "formula": ""}]}`
	if got := lib.ReadStatement(context.Background(), PaperTask{Route: platform, Statement: photoJPEG(t, 30, 30)}); got.Outcome.Kind != Answered || len(got.Tasks) != 1 || got.Tasks[0].Formula != "" || len(opus.asked) != 0 {
		t.Errorf("no formula: %+v", got)
	}
	// A crop with no task is an answer too.
	haiku.resp.JSON = `{"tasks": []}`
	if got := lib.ReadStatement(context.Background(), PaperTask{Route: platform, Statement: photoJPEG(t, 80, 300)}); got.Outcome.Kind != Answered || len(got.Tasks) != 0 || len(opus.asked) != 0 {
		t.Errorf("no task: %+v", got)
	}
}

// The solution is Haiku's when it gives an answer a key may hold, read into an Answer; with none,
// the later models are asked, and an answer none of them gives is nil.
func TestSolveTask(t *testing.T) {
	one := []TaskToSolve{{Number: 1, Statement: "A rope of 20 cm is cut in half. How long is each part?"}}
	lib, haiku, opus := haikuOf(t, `{"answers": [{"task": 1, "kind": "number", "value": "12", "units": ["cm"], "unknown": "", "options": [], "key": ""}]}`, `{}`)
	got := lib.SolveTask(context.Background(), PaperTask{Route: platform, Statement: cropPNG(t, 200, 100)}, one)
	if got.Outcome.Kind != Answered || got.Answers[1].GetNumber().GetValue() != "12" || got.Answers[1].GetNumber().GetUnits()[0] != "cm" || len(opus.asked) != 0 {
		t.Fatalf("solved %+v", got)
	}
	if r := haiku.asked[0]; r.MaxOutputTokens != solving.MaxOutputTokens || r.Instructions != solving.Instructions() || r.SchemaName != "task_answer" ||
		!strings.Contains(r.Text, "<tasks>\n1. A rope of 20 cm is cut in half. How long is each part?\n</tasks>") {
		t.Errorf("the request is %+v", r)
	}
	lib, m := cascadeOf(t, noAnswer, noAnswer, noAnswer)
	if got := lib.SolveTask(context.Background(), PaperTask{Route: platform, Statement: cropPNG(t, 20, 20)}, one); got.Outcome.Kind != Answered || got.Answers[1] != nil ||
		len(got.Outcome.Usage) != 3 || len(m["claude-opus-5-5"].asked) != 1 {
		t.Errorf("no answer: %+v", got)
	}
}

// The failures of the calls of a task from paper: an answer off the schema of every attempt, a crop
// that is no picture, before any call, a limit and an admission without their entries.
func TestPaperTaskFails(t *testing.T) {
	lib, haiku, _ := haikuOf(t, `{"tasks": [{"statement": "1 + 1 =", "formula": 1}]}`, `{}`)
	p := PaperTask{Route: platform, Statement: photoJPEG(t, 30, 30)}
	one := []TaskToSolve{{Number: 1, Statement: "1 + 1 ="}}
	if got := lib.ReadStatement(context.Background(), p); got.Outcome.Kind != Failed || len(got.Outcome.Usage) != 3 {
		t.Errorf("off the schema: %+v", got)
	}
	if got := lib.SolveTask(context.Background(), p, one); got.Outcome.Kind != Failed {
		t.Errorf("off the schema of the solution: %+v", got)
	}
	asked := len(haiku.asked)
	if got := lib.ReadStatement(context.Background(), PaperTask{Route: platform, Statement: []byte("no crop")}); got.Outcome.Kind != Failed || len(haiku.asked) != asked {
		t.Errorf("no crop: %+v", got)
	}
	haiku.err = &provider.LimitError{Limit: provider.LimitOwner}
	if got := lib.SolveTask(context.Background(), p, one); got.Outcome.Kind != LimitReached {
		t.Errorf("a limit: %+v", got)
	}
	haiku.err = errors.New("down")
	if got := lib.ReadStatement(context.Background(), p); got.Outcome.Kind != Failed {
		t.Errorf("a failure: %+v", got)
	}
	withoutWord(t)
	lib, haiku, _ = haikuOf(t, `{"tasks": []}`, `{}`)
	if lib.ReadStatement(context.Background(), p).Outcome.Kind != NotAdmitted || lib.SolveTask(context.Background(), p, one).Outcome.Kind != NotAdmitted || len(haiku.asked) != 0 {
		t.Error("a call of a task from paper was asked without its entry")
	}
}

const paperObservation = `{"parts": [{"kind": "drawing", "text": "circled: B", "legible": true, "other_readings": []}], "results": [{"task": 1, "part": 1, "box": [], "value": "B", "other_values": []}]}`

// The work is read by Haiku at high first, with thinking, into the one answer slot at 1, 1, by the
// prompt of paper, in either reading mode; a reading it does asks no later model.
func TestReadPaper(t *testing.T) {
	circle := []string{"Circle the larger number."}
	lib, haiku, opus := haikuOf(t, paperObservation, paperObservation)
	got := lib.ReadPaper(context.Background(), PaperReading{Route: platform, Crop: cropPNG(t, 400, 300), Tasks: circle})
	if got.Outcome.Kind != Answered || len(got.Observations) != 1 || len(got.Outcome.Usage) != 1 {
		t.Fatalf("read %+v", got)
	}
	if s := got.Observations[0].GetAnswerSlots()[0]; s.GetValue() != "B" || s.GetAnswerSlot().GetItem() != 1 || s.GetAnswerSlot().GetSlot() != 1 {
		t.Errorf("the observation is %v", got.Observations[0])
	}
	if r := haiku.asked[0]; r.Effort != "high" || r.NoThinking || r.Instructions != reading.PaperInstructions() || r.Text != "Image 1 is 400 pixels wide and 300 pixels high. The tasks:\n<tasks>\n1. Circle the larger number.\n</tasks>" || len(opus.asked) != 0 {
		t.Errorf("the request is %+v", r)
	}
	single, err := New(Config{Table: lib.table, Facts: lib.facts, Least: lib.least, Reading: ReadingSingle, Keys: lib.keys, Adapters: lib.adapters})
	if err != nil {
		t.Fatal(err)
	}
	if got := single.ReadPaper(context.Background(), PaperReading{Route: platform, Crop: cropPNG(t, 40, 30), Tasks: circle}); got.Outcome.Kind != Answered || len(got.Observations) != 1 || len(haiku.asked) != 2 || len(opus.asked) != 0 {
		t.Errorf("single: %+v", got)
	}
	// An answer off the schema of every model fails the reading, and a crop that is no PNG fails
	// before any call.
	lib, haiku, opus = haikuOf(t, `{}`, `{}`)
	if got := lib.ReadPaper(context.Background(), PaperReading{Route: platform, Crop: cropPNG(t, 40, 30), Tasks: circle}); got.Outcome.Kind != Failed || len(got.Observations) != 0 || len(opus.asked) != 1 {
		t.Errorf("off the schema: %+v", got)
	}
	if got := lib.ReadPaper(context.Background(), PaperReading{Route: platform, Crop: photoJPEG(t, 40, 30), Tasks: circle}); got.Outcome.Kind != Failed || len(haiku.asked) != 1 {
		t.Errorf("no PNG: %+v", got)
	}
	haiku.err = errors.New("down")
	if got := lib.ReadPaper(context.Background(), PaperReading{Route: platform, Crop: cropPNG(t, 40, 30), Tasks: circle}); got.Outcome.Kind != Failed {
		t.Errorf("a failure: %+v", got)
	}
	// A reading with no task fails before any call.
	asked := len(haiku.asked)
	if got := lib.ReadPaper(context.Background(), PaperReading{Route: platform, Crop: cropPNG(t, 40, 30)}); got.Outcome.Kind != Failed || len(haiku.asked) != asked {
		t.Errorf("no task: %+v", got)
	}
	withoutWord(t)
	lib, haiku, _ = haikuOf(t, paperObservation, paperObservation)
	if got := lib.ReadPaper(context.Background(), PaperReading{Route: platform, Crop: cropPNG(t, 40, 30), Tasks: circle}); got.Outcome.Kind != NotAdmitted || len(haiku.asked) != 0 {
		t.Errorf("not admitted: %+v", got)
	}
}

// A crop of several tasks goes through every call: the statement lists them, the solution answers
// each asked in one call, and the reading gives a slot of each, in the order of the statements.
func TestSeveralTasksOfACrop(t *testing.T) {
	lib, haiku, opus := haikuOf(t, `{"tasks": [{"statement": "□ + 5 = 12", "formula": "x+5=12"}, {"statement": "Tom has 4 pens and gets 3 more. How many has he now?", "formula": ""}]}`, `{}`)
	p := PaperTask{Route: platform, Statement: photoJPEG(t, 80, 300)}
	read := lib.ReadStatement(context.Background(), p)
	if read.Outcome.Kind != Answered || len(read.Tasks) != 2 || read.Tasks[0].Formula != "x+5=12" || read.Tasks[1].Formula != "" {
		t.Fatalf("read %+v", read)
	}
	haiku.resp.JSON = `{"answers": [{"task": 2, "kind": "number", "value": "7", "units": ["pens"], "unknown": "", "options": [], "key": ""}, {"task": 1, "kind": "number", "value": "7", "units": [], "unknown": "x", "options": [], "key": ""}]}`
	asked := []TaskToSolve{{Number: 1, Statement: read.Tasks[0].Statement}, {Number: 2, Statement: read.Tasks[1].Statement}}
	solved := lib.SolveTask(context.Background(), p, asked)
	if solved.Outcome.Kind != Answered || len(solved.Answers) != 2 || solved.Answers[1].GetNumber().GetUnknown() != "x" || solved.Answers[2].GetNumber().GetUnits()[0] != "pens" || len(opus.asked) != 0 {
		t.Fatalf("solved %+v", solved)
	}
	if r := haiku.asked[1]; !strings.Contains(r.Text, "<tasks>\n1. □ + 5 = 12\n2. Tom has 4 pens") {
		t.Errorf("the solution's request is %q", r.Text)
	}
	haiku.resp.JSON = `{"parts": [{"kind": "text", "text": "7", "legible": true, "other_readings": []}, {"kind": "text", "text": "7 pens", "legible": true, "other_readings": []}], "results": [{"task": 1, "part": 1, "box": [100, 200, 300, 400], "value": "7", "other_values": []}, {"task": 2, "part": 2, "box": [], "value": "7", "other_values": []}]}`
	work := lib.ReadPaper(context.Background(), PaperReading{Route: platform, Crop: cropPNG(t, 80, 300), Tasks: []string{read.Tasks[0].Statement, read.Tasks[1].Statement}})
	if work.Outcome.Kind != Answered || len(work.Observations) == 0 {
		t.Fatalf("the work read %+v", work)
	}
	slots := work.Observations[0].GetAnswerSlots()
	if len(slots) != 2 {
		t.Fatalf("the observation is %v", work.Observations[0])
	}
	for i, s := range slots {
		if s.GetValue() != "7" || s.GetAnswerSlot().GetItem() != int32(i+1) || s.GetAnswerSlot().GetSlot() != 1 {
			t.Errorf("slot %d is %v", i+1, s)
		}
	}
	if b := slots[0].GetCropBox(); b.GetYmin() != 100 || b.GetXmin() != 200 || b.GetYmax() != 300 || b.GetXmax() != 400 || slots[1].GetCropBox() != nil {
		t.Errorf("the boxes are %v and %v", slots[0].GetCropBox(), slots[1].GetCropBox())
	}
}
