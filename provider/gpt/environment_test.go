package gpt

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/openai/openai-go/v3/option"
)

// sent is what reached the network: each request's address and headers. The transport answers
// itself, so a request to whatever address is seen, and none leaves the test.
type sent struct {
	mu       sync.Mutex
	requests []*http.Request
}

func (s *sent) RoundTrip(r *http.Request) (*http.Response, error) {
	s.mu.Lock()
	s.requests = append(s.requests, r)
	s.mu.Unlock()
	body := goodAnswer
	if strings.Contains(r.URL.Path, "/models/") {
		body = `{"id":"gpt-6-astra","object":"model","created":1,"owned_by":"openai"}`
	}
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}},
		Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
}

// TestNoEnvironment sets each variable the SDK reads (docs/sources.md, 1.17), and all of them
// together, and holds that the call goes to OpenAI's address, with the key it was given and with
// no header of the environment.
func TestNoEnvironment(t *testing.T) {
	vars := map[string]string{
		"OPENAI_BASE_URL":       "https://env.example/v1/",
		"OPENAI_API_KEY":        "the key of the environment",
		"OPENAI_ADMIN_KEY":      "the admin key of the environment",
		"OPENAI_ORG_ID":         "org-env",
		"OPENAI_PROJECT_ID":     "proj_env",
		"OPENAI_WEBHOOK_SECRET": "whsec_env",
		"OPENAI_CUSTOM_HEADERS": "X-From-Env: 1\nOpenAI-Beta: env",
	}
	check := func(t *testing.T, set map[string]string) {
		for k, v := range set {
			t.Setenv(k, v)
		}
		s := &sent{}
		c := New("test-key", option.WithHTTPClient(&http.Client{Transport: s}))
		if _, err := c.Ask(context.Background(), request); err != nil {
			t.Fatalf("Ask: %v", err)
		}
		if err := c.Check(context.Background()); err != nil {
			t.Fatalf("Check: %v", err)
		}
		want := []string{"https://api.openai.com/v1/responses", "https://api.openai.com/v1/models/" + Model}
		if len(s.requests) != len(want) {
			t.Fatalf("%d requests, want %d", len(s.requests), len(want))
		}
		for i, r := range s.requests {
			if got := r.URL.Scheme + "://" + r.URL.Host + r.URL.Path; got != want[i] {
				t.Errorf("the request went to %s, want %s", got, want[i])
			}
			if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
				t.Errorf("the key sent is %q", got)
			}
			for _, h := range []string{"OpenAI-Organization", "OpenAI-Project", "X-From-Env", "OpenAI-Beta"} {
				if v := r.Header.Get(h); v != "" {
					t.Errorf("the request has %s: %s", h, v)
				}
			}
		}
	}
	for k, v := range vars {
		t.Run(k, func(t *testing.T) { check(t, map[string]string{k: v}) })
	}
	t.Run("all", func(t *testing.T) { check(t, vars) })
}
