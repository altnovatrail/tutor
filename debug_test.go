package tutor

import (
	"context"
	"errors"
	"testing"

	"github.com/altnovatrail/spec/types/admissionv0"

	"github.com/altnovatrail/tutor/admission"
	"github.com/altnovatrail/tutor/provider"
)

var (
	sonnetLow = admission.Model{Provider: "anthropic", ID: "claude-sonnet-5-5", Effort: "low", NoThinking: true}
	opusHigh  = admission.Model{Provider: "anthropic", ID: "claude-opus-5-5", Effort: "high"}
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
	lib, haiku, _ := haikuOf(t, `{"formula": "1+1"}`, `{}`)
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
	lib, m := debugLibrary(t, `{"formula": "37+48"}`)
	var rec []DebugCall
	r := platform
	r.Debug = &Debug{First: &sonnetLow, Record: func(c DebugCall) { rec = append(rec, c) }}
	got := lib.ReadStatement(context.Background(), PaperTask{Route: r, Statement: photoJPEG(t, 300, 80)})
	if got.Outcome.Kind != Answered || got.Formula != "37+48" || len(m["claude-haiku-5-5"].asked) != 0 {
		t.Fatalf("read %+v", got)
	}
	if q := m["claude-sonnet-5-5"].asked; len(q) != 1 || q[0].Effort != "low" || !q[0].NoThinking {
		t.Errorf("Sonnet was asked %+v", q)
	}
	if len(rec) != 1 || rec[0].Call != CallStatement || rec[0].Reader != 0 || rec[0].Attempt != 1 || rec[0].Model != sonnetLow || rec[0].Answer != `{"formula": "37+48"}` ||
		rec[0].Usage == nil || rec[0].Usage.InputTokens != 900 || rec[0].Err != nil {
		t.Errorf("recorded %+v", rec)
	}
	// A chosen model that does not do is the call's answer: no later attempt.
	m["claude-sonnet-5-5"].resp.JSON = `{"kind": "none", "value": "", "units": [], "unknown": "", "options": [], "key": ""}`
	if got := lib.SolveTask(context.Background(), PaperTask{Route: r, Statement: photoJPEG(t, 30, 30)}); got.Outcome.Kind != Answered || got.Answer != nil ||
		len(m["claude-opus-5-5"].asked) != 0 || len(m["claude-sonnet-5-5"].asked) != 2 {
		t.Errorf("solved %+v", got)
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
	lib, m := debugLibrary(t, `{"formula": ""}`)
	var rec []DebugCall
	r := platform
	r.Debug = &Debug{Record: func(c DebugCall) { rec = append(rec, c) }}
	if got := lib.ReadStatement(context.Background(), PaperTask{Route: r, Statement: photoJPEG(t, 30, 30)}); got.Outcome.Kind != Answered || len(m["claude-haiku-5-5"].asked) != 1 {
		t.Fatalf("read %+v", got)
	}
	m["claude-haiku-5-5"].err = errors.New("down")
	lib.SolveTask(context.Background(), PaperTask{Route: r, Statement: photoJPEG(t, 30, 30)})
	if len(rec) != 4 || rec[0].Model.ID != "claude-haiku-5-5" || rec[0].Attempt != 1 || rec[1].Call != CallSolving || rec[1].Err == nil {
		t.Fatalf("recorded %+v", rec)
	}
	for i, id := range []string{"claude-haiku-5-5", "claude-sonnet-5-5", "claude-opus-5-5"} {
		if c := rec[i+1]; c.Call != CallSolving || c.Attempt != i+1 || c.Model.ID != id {
			t.Errorf("attempt %d: %+v", i+1, c)
		}
	}
}

// A second model reads the work beside the first, as a pair: two observations, two readers.
func TestDebugSecondReader(t *testing.T) {
	lib, m := debugLibrary(t, paperObservation)
	var rec []DebugCall
	r := platform
	r.Debug = &Debug{First: &opusHigh, Second: &sonnetLow, Record: func(c DebugCall) { rec = append(rec, c) }}
	got := lib.ReadPaper(context.Background(), PaperReading{Route: r, Crop: cropPNG(t, 400, 300)})
	if got.Outcome.Kind != Answered || len(got.Observations) != 2 || len(got.Outcome.Usage) != 2 {
		t.Fatalf("read %+v", got)
	}
	if q := m["claude-opus-5-5"].asked; len(q) != 1 || q[0].Effort != "high" || q[0].NoThinking {
		t.Errorf("Opus was asked %+v", q)
	}
	if len(rec) != 2 || rec[0].Reader != 1 || rec[1].Reader != 2 || rec[0].Attempt != 1 || rec[1].Attempt != 1 || rec[1].Model != sonnetLow {
		t.Errorf("recorded %+v", rec)
	}
	// The second reads nothing but the work: the statement keeps one model.
	rec = nil
	lib.ReadStatement(context.Background(), PaperTask{Route: r, Statement: photoJPEG(t, 30, 30)})
	if len(rec) != 1 || rec[0].Model != opusHigh {
		t.Errorf("the statement recorded %+v", rec)
	}
}

// The mode passes a refusal of the admission by its models, and changes no call outside a task
// from paper.
func TestDebugBypassAndScope(t *testing.T) {
	lib, _ := debugLibrary(t, `{"formula": "1"}`)
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
