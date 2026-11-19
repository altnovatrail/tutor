package tutor

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/altnovatrail/tutor/provider"
)

const (
	noTasks  = `{"tasks": []}`
	twoTasks = `{"tasks": [{"x1": 2, "y1": 2, "x2": 18, "y2": 10}, {"x1": 2, "y1": 15, "x2": 18, "y2": 25}]}`
	noAnswer = `{"kind": "none", "value": "", "units": [], "unknown": "", "options": [], "key": ""}`
	// broken is an answer of the schema that breaks a rule of a key: no number's notation.
	broken    = `{"kind": "number", "value": "twelve", "units": [], "unknown": "", "options": [], "key": ""}`
	twelve    = `{"kind": "number", "value": "12", "units": [], "unknown": "", "options": [], "key": ""}`
	illegible = `{"parts": [{"kind": "text", "text": "?", "legible": false, "other_readings": []}], "results": [{"part": 1, "value": "35", "other_values": []}]}`
	several   = `{"parts": [{"kind": "text", "text": "85 86", "legible": true, "other_readings": []}], "results": [{"part": 1, "value": "85", "other_values": []}, {"part": 1, "value": "86", "other_values": []}]}`
	none      = `{"parts": [], "results": []}`
	read85    = `{"parts": [{"kind": "text", "text": "85", "legible": true, "other_readings": []}], "results": [{"part": 1, "value": "85", "other_values": []}]}`
)

// timesAsked are how many times each model of the cascade was asked, Haiku, Sonnet and Opus.
func timesAsked(m map[string]*fake) [3]int {
	return [3]int{len(m["claude-haiku-5-5"].asked), len(m["claude-sonnet-5-5"].asked), len(m["claude-opus-5-5"].asked)}
}

// The finding asks a later model when the one before found no task, each at the effort of its
// attempt, and the frames are the first that found any; the usage is every attempt's.
func TestCascadeOfTheFinding(t *testing.T) {
	lib, m := cascadeOf(t, noTasks, noTasks, twoTasks)
	got := lib.FindTasks(context.Background(), TaskFinding{Route: platform, Photo: photoJPEG(t, 20, 30)})
	if got.Outcome.Kind != Answered || len(got.Frames) != 2 || len(got.Outcome.Usage) != 3 || timesAsked(m) != [3]int{1, 1, 1} {
		t.Fatalf("found %+v, asked %v", got, timesAsked(m))
	}
	for id, effort := range map[string]string{"claude-haiku-5-5": "high", "claude-sonnet-5-5": "medium", "claude-opus-5-5": "medium"} {
		if r := m[id].asked[0]; r.Effort != effort || r.NoThinking {
			t.Errorf("%s was asked at %q, no thinking %v", id, r.Effort, r.NoThinking)
		}
	}
	if u := got.Outcome.Usage; u[0].Model != "claude-haiku-5-5" || u[1].Model != "claude-sonnet-5-5" || u[2].Model != "claude-opus-5-5" {
		t.Errorf("the usage is %+v", u)
	}
	// Sonnet finds them, and Opus is not asked.
	lib, m = cascadeOf(t, noTasks, twoTasks, twoTasks)
	if got := lib.FindTasks(context.Background(), TaskFinding{Route: platform, Photo: photoJPEG(t, 20, 30)}); len(got.Frames) != 2 || timesAsked(m) != [3]int{1, 1, 0} {
		t.Errorf("found %+v, asked %v", got, timesAsked(m))
	}
}

// The statement asks a later model only on a failure: one that is no formula is the answer.
func TestCascadeOfTheStatement(t *testing.T) {
	lib, m := cascadeOf(t, `{"formula": 1}`, `{"formula": "37+48"}`, `{"formula": "37+48"}`)
	p := PaperTask{Route: platform, Statement: photoJPEG(t, 30, 30)}
	if got := lib.ReadStatement(context.Background(), p); got.Outcome.Kind != Answered || got.Formula != "37+48" || len(got.Outcome.Usage) != 2 || timesAsked(m) != [3]int{1, 1, 0} {
		t.Errorf("off the schema first: %+v, asked %v", got, timesAsked(m))
	}
	lib, m = cascadeOf(t, `{"formula": ""}`, `{"formula": "37+48"}`, `{"formula": "37+48"}`)
	if got := lib.ReadStatement(context.Background(), p); got.Formula != "" || timesAsked(m) != [3]int{1, 0, 0} {
		t.Errorf("no formula: %+v, asked %v", got, timesAsked(m))
	}
}

// The solution asks a later model when the one before gave no answer, or one a key may not hold.
func TestCascadeOfTheSolution(t *testing.T) {
	lib, m := cascadeOf(t, noAnswer, broken, twelve)
	got := lib.SolveTask(context.Background(), PaperTask{Route: platform, Statement: cropPNG(t, 20, 20)})
	if got.Outcome.Kind != Answered || got.Answer.GetNumber().GetValue() != "12" || timesAsked(m) != [3]int{1, 1, 1} {
		t.Errorf("solved %+v, asked %v", got, timesAsked(m))
	}
	// When none gives one, the answer is the last that answered, whatever it is.
	lib, _ = cascadeOf(t, noAnswer, noAnswer, broken)
	if got := lib.SolveTask(context.Background(), PaperTask{Route: platform, Statement: cropPNG(t, 20, 20)}); got.Outcome.Kind != Answered || got.Answer.GetNumber().GetValue() != "twelve" {
		t.Errorf("none gave one: %+v", got)
	}
}

// The work asks a later model when the one before read the slot as illegible or ambiguous, not when
// it holds no answer or several.
func TestCascadeOfTheWork(t *testing.T) {
	lib, m := cascadeOf(t, illegible, read85, read85)
	got := lib.ReadPaper(context.Background(), PaperReading{Route: platform, Crop: cropPNG(t, 40, 30)})
	if got.Outcome.Kind != Answered || len(got.Observations) != 1 || got.Observations[0].GetAnswerSlots()[0].GetValue() != "85" || timesAsked(m) != [3]int{1, 1, 0} {
		t.Errorf("read %+v, asked %v", got, timesAsked(m))
	}
	for _, held := range []string{several, none} {
		lib, m = cascadeOf(t, held, read85, read85)
		got = lib.ReadPaper(context.Background(), PaperReading{Route: platform, Crop: cropPNG(t, 40, 30)})
		if got.Outcome.Kind != Answered || got.Observations[0].GetAnswerSlots()[0].GetUnreadable() == 0 || timesAsked(m) != [3]int{1, 0, 0} {
			t.Errorf("%s: read %+v, asked %v", held, got, timesAsked(m))
		}
	}
	// Unreadable by all three is the reading: unreadable.
	lib, _ = cascadeOf(t, illegible, illegible, illegible)
	got = lib.ReadPaper(context.Background(), PaperReading{Route: platform, Crop: cropPNG(t, 40, 30)})
	if got.Outcome.Kind != Answered || got.Observations[0].GetAnswerSlots()[0].GetUnreadable() == 0 || len(got.Outcome.Usage) != 3 {
		t.Errorf("unreadable: %+v", got)
	}
}

// A refusal for a limit or of the key ends the cascade as its outcome; a failure of a later model
// leaves the answer of an earlier one; and every model failing is the failure.
func TestCascadeEnds(t *testing.T) {
	lib, m := cascadeOf(t, noTasks, noTasks, twoTasks)
	m["claude-sonnet-5-5"].err = &provider.LimitError{Limit: provider.LimitOwner}
	if got := lib.FindTasks(context.Background(), TaskFinding{Route: platform, Photo: photoJPEG(t, 20, 30)}); got.Outcome.Kind != LimitReached || timesAsked(m) != [3]int{1, 1, 0} {
		t.Errorf("a limit: %+v, asked %v", got, timesAsked(m))
	}
	m["claude-sonnet-5-5"].err = provider.ErrKeyRefused
	if got := lib.FindTasks(context.Background(), TaskFinding{Route: platform, Photo: photoJPEG(t, 20, 30)}); got.Outcome.Kind != KeyRefused {
		t.Errorf("a refused key: %+v", got)
	}
	lib, m = cascadeOf(t, noTasks, twoTasks, twoTasks)
	m["claude-sonnet-5-5"].err, m["claude-opus-5-5"].err = errors.New("down"), errors.New("down")
	if got := lib.FindTasks(context.Background(), TaskFinding{Route: platform, Photo: photoJPEG(t, 20, 30)}); got.Outcome.Kind != Answered || len(got.Frames) != 0 || got.Outcome.Err != nil {
		t.Errorf("later ones down: %+v", got)
	}
	m["claude-haiku-5-5"].err = errors.New("down")
	if got := lib.FindTasks(context.Background(), TaskFinding{Route: platform, Photo: photoJPEG(t, 20, 30)}); got.Outcome.Kind != Failed || got.Outcome.Err == nil {
		t.Errorf("all down: %+v", got)
	}
}

// A later attempt is not begun with less than cascadeFloor left before the deadline.
func TestCascadeFloor(t *testing.T) {
	lib, m := cascadeOf(t, noTasks, twoTasks, twoTasks)
	ctx, cancel := context.WithTimeout(context.Background(), cascadeFloor-time.Second)
	defer cancel()
	if got := lib.FindTasks(ctx, TaskFinding{Route: platform, Photo: photoJPEG(t, 20, 30)}); got.Outcome.Kind != Answered || len(got.Frames) != 0 || timesAsked(m) != [3]int{1, 0, 0} {
		t.Errorf("found %+v, asked %v", got, timesAsked(m))
	}
	ctx, cancel = context.WithTimeout(context.Background(), cascadeFloor+time.Minute)
	defer cancel()
	if got := lib.FindTasks(ctx, TaskFinding{Route: platform, Photo: photoJPEG(t, 20, 30)}); len(got.Frames) != 2 {
		t.Errorf("with time: %+v", got)
	}
}

// The price of a call of a cascade is that of all three attempts, as api reserves it, and is
// expected: each model has its tokens.
func TestCascadePrice(t *testing.T) {
	_, m := cascadeOf(t, noTasks, noTasks, noTasks)
	prices := map[string]provider.Prices{
		"claude-haiku-5-5":  {Input: 0.1, Output: 0.5, Read: "2026-11-11"},
		"claude-sonnet-5-5": {Input: 2, Output: 10, Read: "2026-11-11"},
		"claude-opus-5-5":   {Input: 4, Output: 20, Read: "2026-11-11"},
	}
	for id, p := range prices {
		m[id].prices = p
	}
	lib := library(t, adultFacts, &fakes{models: m})
	for _, c := range []Call{CallFinding, CallStatement, CallSolving, CallPaper} {
		p, a, err := lib.Price(c, platform)
		if err != nil || !a.Admitted || !p.Expected || !p.Worst {
			t.Fatalf("%s: %+v %+v %v", c, p, a, err)
		}
		want := 0.0
		for id, tok := range kinds[c].expected {
			want += (float64(tok.in)*prices[id].Input + float64(tok.out)*prices[id].Output) / 1e6
		}
		if len(kinds[c].expected) != 3 || math.Abs(p.ExpectedUSD-want) > 1e-12 {
			t.Errorf("%s is expected at %v; want %v of three models", c, p.ExpectedUSD, want)
		}
	}
}
