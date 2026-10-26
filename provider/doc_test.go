package provider_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

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
