package tutor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/altnovatrail/spec/types/admissionv0"

	"github.com/altnovatrail/tutor/admission"
	"github.com/altnovatrail/tutor/provider"
)

// fake is a model that answers as it is told, after its delay, and records what it was asked.
type fake struct {
	mu     sync.Mutex
	asked  []provider.Request
	resp   provider.Response
	err    error
	delay  time.Duration
	prices provider.Prices
	// checked counts the checks of the key, and checkErr is what they answer.
	checked  int
	checkErr error
}

func (f *fake) Name() string            { return "fake" }
func (f *fake) Prices() provider.Prices { return f.prices }
func (f *fake) Check(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.checked++
	return f.checkErr
}
func (f *fake) Ask(ctx context.Context, r provider.Request) (provider.Response, error) {
	f.mu.Lock()
	f.asked = append(f.asked, r)
	resp, err, delay := f.resp, f.err, f.delay
	f.mu.Unlock()
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return provider.Response{}, ctx.Err()
		}
	}
	return resp, err
}

// fakes dials a fake for each model by its ID, and records the keys it was given.
type fakes struct {
	models map[string]*fake
	keys   []string
}

func (fs *fakes) dial(id, key string) (provider.Model, error) {
	fs.keys = append(fs.keys, key)
	f, ok := fs.models[id]
	if !ok {
		return nil, fmt.Errorf("no fake of %s", id)
	}
	return f, nil
}

const testKey = "the learner own key in a test"

// library is a library of the platform's table and the given facts, at the stage author, whose
// models are fakes.
func library(t *testing.T, facts string, fs *fakes) *Tutor {
	t.Helper()
	table, err := admission.Platform()
	if err != nil {
		t.Fatal(err)
	}
	f, err := admission.ReadFacts(table, []byte(facts))
	if err != nil {
		t.Fatal(err)
	}
	lib, err := New(Config{Table: table, Facts: f, Least: admission.StageAuthor, Keys: map[admissionv0.Provider]string{"anthropic": "platform-key"},
		Adapters: map[admissionv0.Provider]provider.Dial{"anthropic": fs.dial}})
	if err != nil {
		t.Fatal(err)
	}
	return lib
}

const adultFacts = "anthropic:\n  data_processing_agreement: [adult]\n  minimal_retention: [adult]\n"

var platform = Route{AI: admissionv0.AiRoutePlatformAi, Category: admissionv0.AgeCategoryAdult}

func TestNew(t *testing.T) {
	if _, err := New(Config{}); err == nil {
		t.Error("a config with no table makes a library")
	}
	table, err := admission.Platform()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(Config{Table: table, Least: "trial"}); err == nil {
		t.Error("a least stage of trial makes a library")
	}
	lib, err := New(Config{Table: table})
	if err != nil {
		t.Fatal(err)
	}
	if lib.least != admission.StageFull {
		t.Errorf("an empty least stage is %q; want full", lib.least)
	}
	// With the stage full, the author's pair serves no one.
	if _, a, _ := lib.Choose(admissionv0.CapabilityHandwritingReading, platform); a.Admitted {
		t.Errorf("the stage full chose %+v", a)
	}
}

func TestPrepareAndAsk(t *testing.T) {
	opus := &fake{resp: provider.Response{JSON: `{}`, Model: "claude-opus-5-5", InputTokens: 900, OutputTokens: 120, ThinkingTokens: 80}}
	haiku := &fake{resp: provider.Response{JSON: `{}`, Model: "claude-haiku-5-5", InputTokens: 900, OutputTokens: 60}}
	fs := &fakes{models: map[string]*fake{"claude-opus-5-5": opus, "claude-haiku-5-5": haiku}}
	lib := library(t, adultFacts, fs)

	calls, _, refused := lib.prepare(CallSheet, platform)
	if refused != nil || len(calls) != 2 || calls[0].model.ID != "claude-opus-5-5" || fs.keys[0] != "platform-key" {
		t.Fatalf("prepared %v, refused %+v, keys %v", calls, refused, fs.keys)
	}
	_, out := ask(context.Background(), admission.Answer{}, calls, []provider.Request{{Text: "a"}, {Text: "b"}}, 0)
	if out.Kind != Answered || out.Err != nil || len(out.Usage) != 2 || out.Usage[0].Model != "claude-opus-5-5" || out.Usage[0].ThinkingTokens != 80 || !out.Usage[1].Billed {
		t.Errorf("the outcome is %+v", out)
	}
	// Each model is asked at the effort the list admitted it at, whatever the request said.
	if opus.asked[0].Effort != "high" || haiku.asked[0].Effort != "medium" || opus.asked[0].Text != "a" || haiku.asked[0].Text != "b" {
		t.Errorf("asked %+v and %+v", opus.asked, haiku.asked)
	}

	// A failed call fails the outcome; the model that answered is billed all the same, and so is an
	// answer without JSON.
	answered := haiku.resp
	haiku.resp, haiku.err = provider.Response{}, errors.New("overloaded")
	_, out = ask(context.Background(), admission.Answer{}, calls, []provider.Request{{}, {}}, 0)
	if out.Kind != Failed || len(out.Usage) != 1 || !strings.Contains(out.Err.Error(), "claude-haiku-5-5 medium of anthropic: overloaded") {
		t.Errorf("one failed: %+v", out)
	}
	haiku.resp, haiku.err = answered, provider.ErrNoAnswer
	if _, out = ask(context.Background(), admission.Answer{}, calls, []provider.Request{{}, {}}, 0); out.Kind != Failed || len(out.Usage) != 2 {
		t.Errorf("no answer: %+v", out)
	}
}

// Each model's record of usage carries its own time and retries, so that the log shows which reader
// of a pair held the call, and when its first text came, where its adapter measures that: GPT's does
// not, and the Haiku here stands for it (R107). A billed answer without the JSON is timed too.
func TestAskTimes(t *testing.T) {
	opus := &fake{resp: provider.Response{JSON: `{}`, Model: "claude-opus-5-5", InputTokens: 900, OutputTokens: 120, FirstToken: 7 * time.Millisecond, Retries: 1}}
	haiku := &fake{resp: provider.Response{JSON: `{}`, Model: "claude-haiku-5-5", InputTokens: 900, OutputTokens: 60}, delay: 200 * time.Millisecond}
	lib := library(t, adultFacts, &fakes{models: map[string]*fake{"claude-opus-5-5": opus, "claude-haiku-5-5": haiku}})
	calls, a, refused := lib.prepare(CallSheet, platform)
	if refused != nil {
		t.Fatal(refused)
	}
	start := time.Now()
	_, out := ask(context.Background(), a, calls, []provider.Request{{}, {}}, 0)
	took := time.Since(start)
	if out.Kind != Answered || len(out.Usage) != 2 {
		t.Fatalf("the outcome is %+v", out)
	}
	o, h := out.Usage[0], out.Usage[1]
	// Opus answers at once and Haiku after its delay: each record carries its own time.
	if o.Latency <= 0 || h.Latency < 200*time.Millisecond || o.Latency >= h.Latency || h.Latency > took {
		t.Errorf("Opus took %v and Haiku %v of %v; want each its own, Haiku the longer", o.Latency, h.Latency, took)
	}
	if o.FirstToken != 7*time.Millisecond || o.Retries != 1 || h.FirstToken != 0 || h.Retries != 0 {
		t.Errorf("Opus %v after %d retries and Haiku %v after %d; want 7ms after 1, and none", o.FirstToken, o.Retries, h.FirstToken, h.Retries)
	}
	haiku.err = provider.ErrNoAnswer
	if _, out = ask(context.Background(), a, calls, []provider.Request{{}, {}}, 0); len(out.Usage) != 2 || out.Usage[1].Latency < 200*time.Millisecond {
		t.Errorf("a billed answer without the JSON: %+v", out.Usage)
	}
}

func TestNotAdmitted(t *testing.T) {
	fs := &fakes{models: map[string]*fake{}}
	// No facts: platform AI is closed, and no client is made, no model asked.
	lib := library(t, "", fs)
	calls, _, out := lib.prepare(CallSheet, platform)
	if calls != nil || out == nil || out.Kind != NotAdmitted || out.Admission.Admitted || *out.Admission.Reason != admissionv0.AnswerReasonFactMissing || len(out.Usage) != 0 || len(fs.keys) != 0 {
		t.Errorf("no facts: %v, %+v, keys %v", calls, out, fs.keys)
	}
	// A capability with no model admitted: the hint, its entries left out.
	withWords(t)
	lib = library(t, adultFacts, fs)
	if _, _, out := lib.prepare(CallHint, platform); out == nil || out.Kind != NotAdmitted || *out.Admission.Reason != admission.ReasonModelNotAdmitted {
		t.Errorf("no model of the hint: %+v", out)
	}
	// A child: platform AI is closed without the safeguards.
	child := Route{AI: admissionv0.AiRoutePlatformAi, Category: admissionv0.AgeCategoryUnder13}
	if _, _, out := lib.prepare(CallSheet, child); out == nil || out.Kind != NotAdmitted {
		t.Errorf("a child: %+v", out)
	}
	if len(fs.keys) != 0 {
		t.Errorf("clients made for refused calls: %v", fs.keys)
	}
}

func TestOwnKey(t *testing.T) {
	fs := &fakes{models: map[string]*fake{"claude-opus-5-5": {}, "claude-haiku-5-5": {}}}
	lib := library(t, "", fs)
	own := Route{AI: admissionv0.AiRouteOwnKey, Category: admissionv0.AgeCategoryAdult, Own: NewOwnKey("anthropic", testKey)}
	calls, _, out := lib.prepare(CallSheet, own)
	if out != nil || len(calls) != 2 || fs.keys[0] != testKey || fs.keys[1] != testKey {
		t.Fatalf("the own key prepared %v, %+v, keys of %d", calls, out, len(fs.keys))
	}
	// An own key of another provider takes no model of Anthropic.
	other := Route{AI: admissionv0.AiRouteOwnKey, Category: admissionv0.AgeCategoryAdult, Own: NewOwnKey("openai", testKey)}
	if _, _, out := lib.prepare(CallSheet, other); out == nil || out.Kind != NotAdmitted {
		t.Errorf("an OpenAI key read by Anthropic: %+v", out)
	}
	// A route whose key does not fit it fails before anything.
	for name, r := range map[string]Route{
		"own key without a key":  {AI: admissionv0.AiRouteOwnKey, Category: admissionv0.AgeCategoryAdult},
		"own key of nothing":     {AI: admissionv0.AiRouteOwnKey, Category: admissionv0.AgeCategoryAdult, Own: NewOwnKey("anthropic", "")},
		"own key of no provider": {AI: admissionv0.AiRouteOwnKey, Category: admissionv0.AgeCategoryAdult, Own: NewOwnKey("", testKey)},
		"platform with a key":    {AI: admissionv0.AiRoutePlatformAi, Category: admissionv0.AgeCategoryAdult, Own: NewOwnKey("anthropic", testKey)},
	} {
		_, _, out := lib.prepare(CallSheet, r)
		if out == nil || out.Kind != Failed || strings.Contains(out.Err.Error(), testKey) {
			t.Errorf("%s: %+v", name, out)
		}
	}
	// The key prints as its provider, whatever the verb, and as JSON it is nothing.
	k := NewOwnKey("anthropic", testKey)
	// A struct of api that keeps the key in an unexported field prints it field by field.
	type attempt struct {
		key OwnKey
		ptr *OwnKey
	}
	held := attempt{key: *k, ptr: k}
	for _, s := range []string{fmt.Sprint(k), fmt.Sprintf("%v %+v %#v %s %d %x %q", *k, *k, *k, *k, *k, *k, *k), fmt.Sprintf("%+v", own), fmt.Sprintf("%#v", own), fmt.Sprintf("%+v %#v", held, held)} {
		if strings.Contains(s, testKey) {
			t.Errorf("the key prints in %q", s)
		}
	}
	if data, err := json.Marshal(own); err != nil || strings.Contains(string(data), testKey) {
		t.Errorf("the route as JSON is %s, %v", data, err)
	}
	if k.Provider() != "anthropic" || k.String() != "own key of anthropic" {
		t.Errorf("the key is %q of %q", k.String(), k.Provider())
	}
}

func TestPlatformKeyAndAdapters(t *testing.T) {
	fs := &fakes{models: map[string]*fake{}}
	lib := library(t, adultFacts, fs)
	lib.keys = nil
	// Without the platform's key of Anthropic its pair serves no one on platform AI.
	if _, _, out := lib.prepare(CallSheet, platform); out == nil || out.Kind != NotAdmitted || *out.Admission.Reason != admission.ReasonModelNotAdmitted || len(fs.keys) != 0 {
		t.Errorf("no platform key: %+v", out)
	}
	lib = library(t, adultFacts, fs)
	if _, _, out := lib.prepare(CallSheet, platform); out == nil || out.Kind != Failed || !strings.Contains(out.Err.Error(), "no fake") {
		t.Errorf("a client that cannot be made: %+v", out)
	}
	// A key of a provider with no adapter is a config New refuses, and an adapter given as nil is
	// none.
	base := library(t, adultFacts, fs)
	keys := map[admissionv0.Provider]string{"anthropic": "platform-key"}
	for name, adapters := range map[string]map[admissionv0.Provider]provider.Dial{"none": nil, "nil": {"anthropic": nil}, "another provider's": {"openai": fs.dial}} {
		if _, err := New(Config{Table: base.table, Facts: base.facts, Keys: keys, Adapters: adapters}); err == nil || !strings.Contains(err.Error(), "a key of anthropic and no adapter") {
			t.Errorf("a key with %s adapter: %v", name, err)
		}
	}
	// Without an adapter of Anthropic its pair serves no own key of Anthropic either.
	noAnthropic, err := New(Config{Table: base.table, Facts: base.facts, Least: admission.StageAuthor, Adapters: map[admissionv0.Provider]provider.Dial{"openai": fs.dial}})
	if err != nil {
		t.Fatal(err)
	}
	own := Route{AI: admissionv0.AiRouteOwnKey, Category: admissionv0.AgeCategoryAdult, Own: NewOwnKey("anthropic", testKey)}
	if _, _, out := noAnthropic.prepare(CallSheet, own); out == nil || out.Kind != NotAdmitted || *out.Admission.Reason != admission.ReasonModelNotAdmitted {
		t.Errorf("an own key of a provider with no adapter: %+v", out)
	}
	// A model chosen whose provider has no adapter fails, before any call: Choose never chooses one.
	lib = library(t, adultFacts, fs)
	if _, err := lib.dial(admission.Model{Provider: "openai", ID: "gpt-6-astra"}, "k"); err == nil || !strings.Contains(err.Error(), "no adapter of openai") {
		t.Errorf("a model of no adapter: %v", err)
	}
}

// The root package builds no adapter in: a service that takes the library carries the SDK of a
// provider only when it takes the provider's adapter itself (Д185 RUN.md spec; R90).
func TestRootImportsNoProvider(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", ".").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	for _, dep := range strings.Fields(string(out)) {
		if dep == "github.com/altnovatrail/tutor/provider/claude" || dep == "github.com/altnovatrail/tutor/provider/gpt" || strings.HasPrefix(dep, "github.com/anthropics/") || strings.HasPrefix(dep, "github.com/openai/") {
			t.Errorf("the root package takes %s", dep)
		}
	}
}

// The clients of the platform's keys are made once for each model; those of an own key, for each
// request.
func TestClients(t *testing.T) {
	fs := &fakes{models: map[string]*fake{"claude-opus-5-5": {}, "claude-haiku-5-5": {}}}
	lib := library(t, adultFacts, fs)
	for range 3 {
		if _, _, out := lib.prepare(CallSheet, platform); out != nil {
			t.Fatal(out)
		}
	}
	if len(fs.keys) != 2 {
		t.Errorf("%d clients made of the platform's key, want 2", len(fs.keys))
	}
	own := Route{AI: admissionv0.AiRouteOwnKey, Category: admissionv0.AgeCategoryAdult, Own: NewOwnKey("anthropic", testKey)}
	for range 2 {
		if _, _, out := lib.prepare(CallSheet, own); out != nil {
			t.Fatal(out)
		}
	}
	if len(fs.keys) != 6 {
		t.Errorf("%d clients made in all, want 2 more for each own key's request", len(fs.keys))
	}
}

// A model whose answer broke off is billed by what it counted; a panic of an adapter fails the call,
// and a request missing for a model fails the outcome before any call.
func TestAskEdges(t *testing.T) {
	broken := &fake{resp: provider.Response{Model: "claude-opus-5-5", InputTokens: 900, OutputTokens: 40, Partial: true}, err: errors.New("stream broke")}
	calls := []call{{model: admission.Model{Provider: "anthropic", ID: "claude-opus-5-5", Effort: "high"}, client: broken}, {model: admission.Model{Provider: "anthropic", ID: "claude-haiku-5-5", Effort: "medium"}, client: panicky{}}}
	_, out := ask(context.Background(), admission.Answer{}, calls, []provider.Request{{}, {}}, 0)
	if out.Kind != Failed || len(out.Usage) != 1 || out.Usage[0].Billed || out.Usage[0].InputTokens != 900 {
		t.Errorf("a broken answer and a panic: %+v", out)
	}
	if _, out := ask(context.Background(), admission.Answer{}, calls[:1], []provider.Request{{}, {}}, 0); out.Kind != Failed || !strings.Contains(out.Err.Error(), "2 requests for 1 models") || len(broken.asked) != 1 {
		t.Errorf("requests that do not fit: %+v", out)
	}
	_, out = ask(context.Background(), admission.Answer{}, calls[1:], []provider.Request{{}}, 0)
	if out.Kind != Failed || !strings.Contains(out.Err.Error(), "the adapter panicked: boom") {
		t.Errorf("a panic: %+v", out)
	}
}

// panicky is a model whose adapter panics.
type panicky struct{}

func (panicky) Name() string                { return "panicky" }
func (panicky) Prices() provider.Prices     { return provider.Prices{} }
func (panicky) Check(context.Context) error { return nil }
func (panicky) Ask(context.Context, provider.Request) (provider.Response, error) {
	panic("boom")
}

func TestKindString(t *testing.T) {
	want := map[Kind]string{Answered: "answered", Failed: "failed", LimitReached: "limit reached", NotAdmitted: "not admitted", KeyRefused: "key refused", 0: "kind 0"}
	for k, s := range want {
		if k.String() != s {
			t.Errorf("%d is %q, want %q", int(k), k.String(), s)
		}
	}
}
