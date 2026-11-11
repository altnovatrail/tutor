package provider_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/altnovatrail/tutor/provider"
)

func TestBilled(t *testing.T) {
	cut := fmt.Errorf("the call: %w", context.DeadlineExceeded)
	for _, c := range []struct {
		name string
		resp provider.Response
		err  error
		want bool
	}{
		{"an answer", provider.Response{InputTokens: 1000, OutputTokens: 50}, nil, true},
		{"a refusal, with its tokens", provider.Response{InputTokens: 1000, OutputTokens: 50}, provider.ErrNoAnswer, true},
		{"an answer that broke off before the count", provider.Response{InputTokens: 1000, Partial: true}, errors.New("the stream broke"), false},
		{"a call cut off before any bill", provider.Response{}, cut, false},
		{"a call cut off after its bill", provider.Response{InputTokens: 1000, OutputTokens: 20}, cut, true},
		{"a call that failed before any bill", provider.Response{}, errors.New("400"), true},
	} {
		if got := provider.Billed(c.resp, c.err); got != c.want {
			t.Errorf("%s: billed %v, want %v", c.name, got, c.want)
		}
	}
}

// A cost holds every token at its price, and none when the prices do not hold them all: a write
// kept for a time they name no price of, or a long prompt whose prices were not read.
func TestCost(t *testing.T) {
	p := provider.Prices{Input: 4, Output: 20, CacheRead: 0.2, CacheWrite: map[time.Duration]float64{time.Hour: 8}, LongAbove: 1000, Read: "2026-11-11"}
	r := provider.Response{InputTokens: 500, CacheReadTokens: 100, CacheWrites: []provider.CacheWrite{{TTL: time.Hour, Tokens: 50}}, OutputTokens: 10}
	if usd, ok := p.Cost(r); !ok || math.Abs(usd-0.00262) > 1e-12 {
		t.Errorf("cost = %v, %v", usd, ok)
	}
	r.CacheWrites = append(r.CacheWrites, provider.CacheWrite{TTL: time.Minute, Tokens: 1})
	if _, ok := p.Cost(r); ok {
		t.Error("a write for a time the prices do not name has a cost")
	}
	if _, ok := p.Cost(provider.Response{InputTokens: 1001}); ok {
		t.Error("a long prompt without its prices has a cost")
	}
	p.Long = &provider.Prices{Input: 8}
	if usd, ok := p.Cost(provider.Response{InputTokens: 900, CacheReadTokens: 101}); !ok || math.Abs(usd-0.0072) > 1e-12 {
		t.Errorf("a long prompt, the cache counted in, costs %v, %v", usd, ok)
	}
}

// A response counts its tokens with the cache: a prompt read whole from the cache leaves the input
// at none.
func TestCounted(t *testing.T) {
	for _, r := range []provider.Response{{CacheReadTokens: 1}, {CacheWrites: []provider.CacheWrite{{TTL: time.Hour, Tokens: 1}}}, {OutputTokens: 1}, {InputTokens: 1}} {
		if !r.Counted() {
			t.Errorf("%+v counts no token", r)
		}
	}
	if (provider.Response{CacheWrites: []provider.CacheWrite{{TTL: time.Hour}}}).Counted() {
		t.Error("an empty write counts a token")
	}
	cut := fmt.Errorf("the call: %w", context.Canceled)
	if !provider.Billed(provider.Response{CacheReadTokens: 900}, cut) {
		t.Error("a call cut off after its prompt was read from the cache is not billed")
	}
}

// The worst price of an input token is the higher of its own and a write to the cache.
func TestWorstInput(t *testing.T) {
	p := provider.Prices{Input: 4, CacheWrite: map[time.Duration]float64{5 * time.Minute: 5, time.Hour: 8}}
	if p.WorstInput() != 8 || (provider.Prices{Input: 4}).WorstInput() != 4 {
		t.Errorf("worst input = %v", p.WorstInput())
	}
}
