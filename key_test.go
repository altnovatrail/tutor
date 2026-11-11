package tutor

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/altnovatrail/spec/types/admissionv0"

	"github.com/altnovatrail/tutor/provider"
)

// ownRoute is the route of an own key of Anthropic at an adult profile.
var ownRoute = Route{AI: admissionv0.AiRouteOwnKey, Category: admissionv0.AgeCategoryAdult, Own: NewOwnKey("anthropic", testKey)}

// The check of a key asks every model the library would call with it — both readers of the pair —
// with the key itself, and takes the worst end: a wrong key, then a model without rights, then a
// provider that did not answer. No error holds the key.
func TestCheckKey(t *testing.T) {
	opus, haiku := &fake{}, &fake{}
	fs := &fakes{models: map[string]*fake{"claude-opus-5-5": opus, "claude-haiku-5-5": haiku}}
	lib := library(t, adultFacts, fs)
	limit := &provider.LimitError{Limit: provider.LimitBalance, Err: errors.New("400")}
	for _, tt := range []struct {
		opus, haiku error
		want        KeyCheck
	}{
		{nil, nil, KeyAccepted},
		{limit, nil, KeyAccepted},
		{nil, provider.ErrNoRights, KeyNoRights},
		{errors.New("overloaded"), provider.ErrNoRights, KeyNoRights},
		{provider.ErrNoRights, provider.ErrKeyRefused, KeyWrong},
		{errors.New("overloaded"), nil, KeyUnanswered},
	} {
		opus.checkErr, haiku.checkErr = tt.opus, tt.haiku
		got, err := lib.CheckKey(context.Background(), ownRoute)
		if got != tt.want || (tt.want == KeyAccepted) != (err == nil) {
			t.Errorf("%v and %v: check = %v, %v; want %v", tt.opus, tt.haiku, got, err, tt.want)
		}
		if err != nil && strings.Contains(err.Error(), testKey) {
			t.Errorf("the error holds the key: %v", err)
		}
	}
	if opus.checked != 6 || haiku.checked != 6 || fs.keys[len(fs.keys)-1] != testKey {
		t.Errorf("checked %d and %d times, last with the key %q", opus.checked, haiku.checked, fs.keys[len(fs.keys)-1])
	}
	for name, r := range map[string]Route{
		"platform AI": platform,
		"no key":      {AI: admissionv0.AiRouteOwnKey, Category: admissionv0.AgeCategoryAdult, Own: NewOwnKey("anthropic", "")},
	} {
		if got, err := lib.CheckKey(context.Background(), r); got != KeyUnanswered || err == nil {
			t.Errorf("%s: %v, %v", name, got, err)
		}
	}
	// A provider the admission does not open, or one with no adapter, is closed, and no call goes.
	before := opus.checked
	for name, r := range map[string]Route{
		"a child's profile": {AI: admissionv0.AiRouteOwnKey, Category: admissionv0.AgeCategoryUnder13, Own: NewOwnKey("anthropic", testKey)},
		"no adapter":        {AI: admissionv0.AiRouteOwnKey, Category: admissionv0.AgeCategoryAdult, Own: NewOwnKey("openai", testKey)},
	} {
		if got, err := lib.CheckKey(context.Background(), r); got != KeyProviderClosed || err != nil {
			t.Errorf("%s: %v, %v", name, got, err)
		}
	}
	if opus.checked != before {
		t.Error("a closed provider was asked")
	}
	for k, s := range map[KeyCheck]string{KeyAccepted: "accepted", KeyWrong: "wrong", KeyNoRights: "no rights", KeyProviderClosed: "provider closed", KeyUnanswered: "unanswered", 0: "key check 0"} {
		if k.String() != s {
			t.Errorf("%d is %q", int(k), k.String())
		}
	}
}

// panicCheck is a model whose check panics.
type panicCheck struct{ fake }

func (*panicCheck) Check(context.Context) error { panic("boom") }

// A panic of an adapter fails the check rather than the process.
func TestCheckKeyPanic(t *testing.T) {
	p := &panicCheck{}
	lib := library(t, adultFacts, &fakes{models: map[string]*fake{"claude-opus-5-5": {}, "claude-haiku-5-5": {}}})
	lib.adapters["anthropic"] = func(string, string) (provider.Model, error) { return p, nil }
	if got, err := lib.CheckKey(context.Background(), ownRoute); got != KeyUnanswered || err == nil || !strings.Contains(err.Error(), "panicked: boom") {
		t.Errorf("a panic: %v, %v", got, err)
	}
}
