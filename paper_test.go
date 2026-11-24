package tutor

import (
	"context"
	"errors"
	"strings"
	"testing"

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

// The failures of the reading of a statement: an answer off the schema of every attempt, a crop
// that is no picture, before any call, a limit and an admission without their entries.
func TestPaperTaskFails(t *testing.T) {
	lib, haiku, _ := haikuOf(t, `{"tasks": [{"statement": "1 + 1 =", "formula": 1}]}`, `{}`)
	p := PaperTask{Route: platform, Statement: photoJPEG(t, 30, 30)}
	if got := lib.ReadStatement(context.Background(), p); got.Outcome.Kind != Failed || len(got.Outcome.Usage) != 3 {
		t.Errorf("off the schema: %+v", got)
	}
	asked := len(haiku.asked)
	if got := lib.ReadStatement(context.Background(), PaperTask{Route: platform, Statement: []byte("no crop")}); got.Outcome.Kind != Failed || len(haiku.asked) != asked {
		t.Errorf("no crop: %+v", got)
	}
	haiku.err = &provider.LimitError{Limit: provider.LimitOwner}
	if got := lib.ReadStatement(context.Background(), p); got.Outcome.Kind != LimitReached {
		t.Errorf("a limit: %+v", got)
	}
	haiku.err = errors.New("down")
	if got := lib.ReadStatement(context.Background(), p); got.Outcome.Kind != Failed {
		t.Errorf("a failure: %+v", got)
	}
	withoutWord(t)
	lib, haiku, _ = haikuOf(t, `{"tasks": []}`, `{}`)
	if lib.ReadStatement(context.Background(), p).Outcome.Kind != NotAdmitted || len(haiku.asked) != 0 {
		t.Error("a call of a task from paper was asked without its entry")
	}
}
