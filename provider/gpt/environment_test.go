package gpt

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/openai/openai-go/v3/option"

	"github.com/altnovatrail/tutor/internal/providertest"
)

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
		s := &providertest.Sent{Answer: func(r *http.Request) (string, string) {
			if strings.Contains(r.URL.Path, "/models/") {
				return `{"id":"gpt-6-astra","object":"model","created":1,"owned_by":"openai"}`, "application/json"
			}
			return goodAnswer, "application/json"
		}}
		c := New("test-key", option.WithHTTPClient(&http.Client{Transport: s}))
		if _, err := c.Ask(context.Background(), request); err != nil {
			t.Fatalf("Ask: %v", err)
		}
		if err := c.Check(context.Background()); err != nil {
			t.Fatalf("Check: %v", err)
		}
		s.Check(t, providertest.Want{
			URLs:      []string{"https://api.openai.com/v1/responses", "https://api.openai.com/v1/models/" + Model},
			KeyHeader: "Authorization", Key: "Bearer test-key",
			Absent: []string{"OpenAI-Organization", "OpenAI-Project", "X-From-Env", "OpenAI-Beta"},
		})
	}
	for k, v := range vars {
		t.Run(k, func(t *testing.T) { check(t, map[string]string{k: v}) })
	}
	t.Run("all", func(t *testing.T) { check(t, vars) })
}
