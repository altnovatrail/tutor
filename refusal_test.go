package tutor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/altnovatrail/tutor/provider"
)

// A limit of one reader is the outcome of the whole reading, with its reason and when access
// returns, where the provider says it, and the reader that answered keeps its usage, since it is
// billed; the refused call has none. A refused key comes next, and a limit outranks it.
func TestRefusalOutcomes(t *testing.T) {
	lib, opus, haiku := pricedPair(t)
	calls, a, refused := lib.prepare(CallSheet, platform)
	if refused != nil {
		t.Fatal(refused)
	}
	limit := &provider.LimitError{Limit: provider.LimitBalance, Err: errors.New("402")}
	haiku.resp, haiku.err = provider.Response{}, limit
	_, out := ask(context.Background(), a, calls, []provider.Request{{}, {}}, 0)
	if out.Kind != LimitReached || out.Limit != LimitBalance || !out.AccessReturns.IsZero() || len(out.Usage) != 1 || out.Usage[0].Model != "claude-opus-5-5" || !errors.Is(out.Err, limit) {
		t.Errorf("a limit of one reader: %+v", out)
	}
	opus.resp, opus.err = provider.Response{}, provider.ErrKeyRefused
	if _, out = ask(context.Background(), a, calls, []provider.Request{{}, {}}, 0); out.Kind != LimitReached || out.Limit != LimitBalance {
		t.Errorf("a limit and a refused key: %+v", out)
	}
	haiku.err = errors.New("overloaded")
	if _, out = ask(context.Background(), a, calls, []provider.Request{{}, {}}, 0); out.Kind != KeyRefused || out.Limit != "" || !out.AccessReturns.IsZero() || !errors.Is(out.Err, provider.ErrKeyRefused) {
		t.Errorf("a refused key and a failure: %+v", out)
	}
	returns := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	opus.err = errors.New("overloaded")
	haiku.err = &provider.LimitError{Limit: provider.LimitTier, Returns: returns, Err: errors.New("429")}
	if _, out = ask(context.Background(), a, calls, []provider.Request{{}, {}}, 0); out.Kind != LimitReached || out.Limit != LimitTier || !out.AccessReturns.Equal(returns) {
		t.Errorf("the cap of the tier: %+v", out)
	}
}
