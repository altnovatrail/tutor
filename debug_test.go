package tutor

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/altnovatrail/spec/types/admissionv0"

	"github.com/altnovatrail/tutor/admission"
	"github.com/altnovatrail/tutor/provider"
)

var (
	sonnetLow = admission.Model{Provider: "anthropic", ID: "claude-sonnet-5-5", Effort: "low", NoThinking: true}
)

// debugLibrary is the library of haikuOf with the debug mode on, and Sonnet beside Haiku and Opus.
func debugLibrary(t *testing.T, answer string) (*Tutor, map[string]*fake) {
	t.Helper()
	lib, haiku, opus := haikuOf(t, answer, answer)
	sonnet := &fake{resp: provider.Response{JSON: answer, Model: "claude-sonnet-5-5", InputTokens: 900, OutputTokens: 90}, prices: provider.Prices{Input: 2, Output: 10, Read: "2026-11-11"}}
	fs := &fakes{models: map[string]*fake{"claude-haiku-5-5": haiku, "claude-opus-5-5": opus, "claude-sonnet-5-5": sonnet}}
	on, err := New(Config{Table: lib.table, Facts: lib.facts, Least: lib.least, Keys: lib.keys, Adapters: map[admissionv0.Provider]provider.Dial{"anthropic": fs.dial}, Debug: true})
	if err != nil {
		t.Fatal(err)
	}
	return on, fs.models
}

// A route with the debug mode fails on a library whose config has it off, before any model.
func TestDebugOff(t *testing.T) {
	lib, haiku, _ := haikuOf(t, `{"tasks": [{"statement": "1 + 1 =", "formula": "1+1"}]}`, `{}`)
	r := platform
	r.Debug = &Debug{First: &sonnetLow}
	got := lib.ReadStatement(context.Background(), PaperTask{Route: r, Statement: photoJPEG(t, 30, 30)})
	if got.Outcome.Kind != Failed || !errors.Is(got.Outcome.Err, errDebugOff) || len(haiku.asked) != 0 {
		t.Errorf("off: %+v", got)
	}
}

// The first model serves a call of a task from paper with its effort and thinking, one attempt and
// no cascade, and each call is recorded with its answer as the model returned it and its usage.
func TestDebugFirst(t *testing.T) {
	lib, m := debugLibrary(t, `{"tasks": [{"statement": "37 + 48 =", "formula": "37+48"}]}`)
	var rec []DebugCall
	r := platform
	r.Debug = &Debug{First: &sonnetLow, Record: func(c DebugCall) { rec = append(rec, c) }}
	got := lib.ReadStatement(context.Background(), PaperTask{Route: r, Statement: photoJPEG(t, 300, 80)})
	if got.Outcome.Kind != Answered || len(got.Tasks) != 1 || got.Tasks[0].Formula != "37+48" || len(m["claude-haiku-5-5"].asked) != 0 {
		t.Fatalf("read %+v", got)
	}
	if q := m["claude-sonnet-5-5"].asked; len(q) != 1 || q[0].Effort != "low" || !q[0].NoThinking {
		t.Errorf("Sonnet was asked %+v", q)
	}
	if len(rec) != 1 || rec[0].Call != CallStatement || rec[0].Attempt != 1 || rec[0].Model != sonnetLow || rec[0].Answer != `{"tasks": [{"statement": "37 + 48 =", "formula": "37+48"}]}` ||
		rec[0].Usage == nil || rec[0].Usage.InputTokens != 900 || rec[0].Err != nil {
		t.Errorf("recorded %+v", rec)
	}
	// A chosen model that does not do is the call's answer: no later attempt.
	m["claude-sonnet-5-5"].resp.JSON = checkAnswer("", twoHints, checkTask(1, "incorrect", "low", "13", "12"), checkTask(2, "unreadable", "high", "", "7"))
	if got := lib.CheckPaper(context.Background(), PaperCheck{Route: r, Statement: photoJPEG(t, 30, 30), Work: cropPNG(t, 400, 200), Tasks: 2, Language: "en"}); got.Outcome.Kind != Answered ||
		got.Slots[0].Confidence != "low" || len(m["claude-opus-5-5"].asked) != 0 || len(m["claude-sonnet-5-5"].asked) != 2 || rec[1].Call != CallChecking || rec[1].Attempt != 1 {
		t.Errorf("checked %+v, recorded %+v", got, rec)
	}
	// The price follows the models of the mode too: Sonnet alone has prices.
	p, a, err := lib.Price(CallStatement, r)
	if err != nil || !a.Admitted || !p.Worst || p.WorstUSD <= 0 {
		t.Errorf("price %+v %+v %v", p, a, err)
	}
}

// Without a first model each call keeps its models of the list, the attempts of its cascade, and
// the mode records each with its place; a failure is recorded with what failed.
func TestDebugAsAdmitted(t *testing.T) {
	lib, m := debugLibrary(t, `{"tasks": [{"statement": "Ann has 3 apples.", "formula": ""}]}`)
	var rec []DebugCall
	r := platform
	r.Debug = &Debug{Record: func(c DebugCall) { rec = append(rec, c) }}
	if got := lib.ReadStatement(context.Background(), PaperTask{Route: r, Statement: photoJPEG(t, 30, 30)}); got.Outcome.Kind != Answered || len(m["claude-haiku-5-5"].asked) != 1 {
		t.Fatalf("read %+v", got)
	}
	m["claude-haiku-5-5"].err = errors.New("down")
	lib.CheckPaper(context.Background(), PaperCheck{Route: r, Statement: photoJPEG(t, 30, 30), Work: cropPNG(t, 40, 20), Tasks: 1, Language: "en"})
	if len(rec) != 4 || rec[0].Model.ID != "claude-haiku-5-5" || rec[0].Attempt != 1 || rec[1].Call != CallChecking || rec[1].Err == nil {
		t.Fatalf("recorded %+v", rec)
	}
	for i, id := range []string{"claude-haiku-5-5", "claude-sonnet-5-5", "claude-opus-5-5"} {
		if c := rec[i+1]; c.Call != CallChecking || c.Attempt != i+1 || c.Model.ID != id {
			t.Errorf("attempt %d: %+v", i+1, c)
		}
	}
}

// The mode passes a refusal of the admission by its models, and chooses no model outside a task
// from paper.
func TestDebugBypassAndScope(t *testing.T) {
	lib, _ := debugLibrary(t, `{"tasks": [{"statement": "1", "formula": "1"}]}`)
	child := Route{AI: admissionv0.AiRoutePlatformAi, Category: admissionv0.AgeCategoryUnder13}
	if _, a, err := lib.ChooseCall(CallStatement, child); err != nil || a.Admitted {
		t.Fatalf("a child is admitted without the mode: %+v %v", a, err)
	}
	child.Debug = &Debug{First: &sonnetLow}
	e, a, err := lib.ChooseCall(CallStatement, child)
	if err != nil || !a.Admitted || a.Reason != nil || len(e.Models) != 1 || e.Models[0] != sonnetLow {
		t.Errorf("bypass: %+v %+v %v", e, a, err)
	}
	child.Debug = &Debug{}
	if _, a, _ := lib.ChooseCall(CallStatement, child); a.Admitted {
		t.Error("the mode with no model admitted a refused call")
	}
	r := platform
	plain, _, _ := lib.ChooseCall(CallSheet, r)
	r.Debug = &Debug{First: &sonnetLow}
	if e, _, _ := lib.ChooseCall(CallSheet, r); len(e.Models) != len(plain.Models) || e.Models[0] != plain.Models[0] {
		t.Errorf("a sheet changed: %+v", e)
	}
}

// The mode records a reading of a sheet of a module, each reader with its place and its usage, and
// its first model serves no such call: the readers stay those of the list.
func TestDebugRecordsModuleCalls(t *testing.T) {
	opus := &fake{resp: provider.Response{JSON: observed, Model: "claude-opus-5-5", InputTokens: 1500, OutputTokens: 200}}
	haiku := &fake{resp: provider.Response{JSON: observed, Model: "claude-haiku-5-5", InputTokens: 1500, OutputTokens: 600}}
	sonnet := &fake{resp: provider.Response{JSON: observed, Model: "claude-sonnet-5-5"}}
	fs := &fakes{models: map[string]*fake{"claude-opus-5-5": opus, "claude-haiku-5-5": haiku, "claude-sonnet-5-5": sonnet}}
	plain := library(t, adultFacts, fs)
	lib, err := New(Config{Table: plain.table, Facts: plain.facts, Least: plain.least, Keys: plain.keys, Adapters: map[admissionv0.Provider]provider.Dial{"anthropic": fs.dial}, Debug: true})
	if err != nil {
		t.Fatal(err)
	}
	var rec []DebugCall
	r := platform
	r.Debug = &Debug{First: &sonnetLow, Record: func(c DebugCall) { rec = append(rec, c) }}
	got := lib.ReadSheet(context.Background(), SheetReading{Route: r, Task: sheetOf(), Crop: cropPNG(t, 300, 120)})
	if got.Outcome.Kind != Answered || len(sonnet.asked) != 0 || len(opus.asked) != 1 || len(haiku.asked) != 1 {
		t.Fatalf("read %+v, Sonnet asked %d", got, len(sonnet.asked))
	}
	if len(rec) != 2 {
		t.Fatalf("recorded %+v", rec)
	}
	for i, id := range []string{"claude-opus-5-5", "claude-haiku-5-5"} {
		if c := rec[i]; c.Call != CallSheet || c.Reader != i+1 || c.Attempt != 1 || c.Model.ID != id || c.Answer != observed || c.Usage == nil || c.Usage.InputTokens != 1500 {
			t.Errorf("reader %d: %+v", i+1, c)
		}
	}
	// A refusal of the admission stands, past the mode, outside a task from paper.
	child := Route{AI: admissionv0.AiRoutePlatformAi, Category: admissionv0.AgeCategoryUnder13, Debug: r.Debug}
	if got := lib.ReadSheet(context.Background(), SheetReading{Route: child, Task: sheetOf(), Crop: cropPNG(t, 30, 30)}); got.Outcome.Kind != NotAdmitted || len(rec) != 2 {
		t.Errorf("a child's sheet: %+v, recorded %d", got, len(rec))
	}
}

// Every kind of call of the library carries the mode of its route to each of its models, so that
// no model is asked without its record; a reading's models carry their places, any other none.
func TestDebugRecordsEveryKind(t *testing.T) {
	lib, _ := debugLibrary(t, `{}`)
	d := &Debug{Record: func(DebugCall) {}}
	r := platform
	r.Debug = d
	for c, k := range kinds {
		calls, a, refused := lib.prepare(c, r)
		if refused != nil || !a.Admitted || len(calls) == 0 {
			t.Fatalf("%s: no model, %+v %+v", c, a, refused)
		}
		for i, cl := range calls {
			if cl.debug != d || cl.kind != c {
				t.Errorf("%s: model %d carries no record", c, i+1)
			}
			if want := map[bool]int{true: i + 1, false: 0}[k.pair]; cl.reader != want {
				t.Errorf("%s: model %d is reader %d", c, i+1, cl.reader)
			}
		}
	}
}

// Calls names every kind of call of the library once, in order.
func TestCalls(t *testing.T) {
	got := Calls()
	if len(got) != len(kinds) || !slices.IsSorted(got) {
		t.Fatalf("Calls = %v", got)
	}
	for _, c := range got {
		if _, ok := kinds[c]; !ok {
			t.Errorf("%s is no kind", c)
		}
	}
}
