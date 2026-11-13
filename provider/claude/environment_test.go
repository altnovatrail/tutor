package claude

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/anthropics/anthropic-sdk-go/option"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/embedded"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/altnovatrail/tutor/internal/providertest"
	"github.com/altnovatrail/tutor/provider/claude/catalog"
)

// spans counts the tracers the SDK asks the process's TracerProvider for: it asks for one before
// each span. Its spans record nothing, but carry the trace of the caller's context, which the SDK
// would send on.
type spans struct {
	embedded.TracerProvider
	tracers atomic.Int32
}

func (p *spans) Tracer(name string, opts ...trace.TracerOption) trace.Tracer {
	p.tracers.Add(1)
	return noop.NewTracerProvider().Tracer(name, opts...)
}

// TestNoEnvironment sets each variable the SDK reads (docs/sources.md, 1.17), and all of them
// together, and holds that the call goes to Anthropic's address, with the key it was given, with
// no header of the environment and no span.
func TestNoEnvironment(t *testing.T) {
	dir := t.TempDir()
	write := func(name, text string) string {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	// A profile that sends a workspace's header and names another address.
	write("configs/elsewhere.json", `{"authentication":{"type":"user_oauth","client_id":"c"},"base_url":"https://profile.example","workspace_id":"wrkspc_env"}`)
	write("credentials/elsewhere.json", `{"type":"user_oauth","access_token":"profile-token"}`)
	token := write("identity", "identity-token")
	vars := map[string]string{
		"ANTHROPIC_BASE_URL":                                "https://env.example",
		"ANTHROPIC_API_KEY":                                 "the key of the environment",
		"ANTHROPIC_AUTH_TOKEN":                              "the token of the environment",
		"ANTHROPIC_PROFILE":                                 "elsewhere",
		"ANTHROPIC_CONFIG_DIR":                              dir,
		"XDG_CONFIG_HOME":                                   dir,
		"ANTHROPIC_FEDERATION_RULE_ID":                      "fdrl_env",
		"ANTHROPIC_ORGANIZATION_ID":                         "org_env",
		"ANTHROPIC_IDENTITY_TOKEN_FILE":                     token,
		"ANTHROPIC_IDENTITY_TOKEN":                          "identity-token",
		"ANTHROPIC_SERVICE_ACCOUNT_ID":                      "svac_env",
		"ANTHROPIC_WORKSPACE_ID":                            "wrkspc_env",
		"ANTHROPIC_SCOPE":                                   "workspace:developer",
		"ANTHROPIC_WEBHOOK_SIGNING_KEY":                     "whsec_env",
		"ANTHROPIC_CUSTOM_HEADERS":                          "X-From-Env: 1\nAnthropic-Beta: env",
		"ANTHROPIC_OPEN_TELEMETRY":                          "true",
		"ANTHROPIC_OPEN_TELEMETRY_PROPAGATION":              "true",
		"ANTHROPIC_OPEN_TELEMETRY_TRACES_CONTENT_MODE":      "content",
		"ANTHROPIC_OPEN_TELEMETRY_TRACES_MAX_CONTENT_BYTES": "1000000",
	}
	// The process traces, and the caller's context is in a trace.
	p := &spans{}
	savedProvider, savedPropagator := otel.GetTracerProvider(), otel.GetTextMapPropagator()
	otel.SetTracerProvider(p)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() {
		otel.SetTracerProvider(savedProvider)
		otel.SetTextMapPropagator(savedPropagator)
	})
	ctx := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{1}, SpanID: trace.SpanID{1}, TraceFlags: trace.FlagsSampled}))

	check := func(t *testing.T, set map[string]string) {
		for k, v := range set {
			t.Setenv(k, v)
		}
		p.tracers.Store(0)
		s := &providertest.Sent{Answer: func(r *http.Request) (string, string) {
			if strings.HasSuffix(r.URL.Path, "/count_tokens") {
				return `{"input_tokens":1}`, "application/json"
			}
			return goodStream, "text/event-stream"
		}}
		c := New("test-key", catalog.Models[2], option.WithHTTPClient(&http.Client{Transport: s}))
		if _, err := c.Ask(ctx, request); err != nil {
			t.Fatalf("Ask: %v", err)
		}
		if err := c.Check(ctx); err != nil {
			t.Fatalf("Check: %v", err)
		}
		s.Check(t, providertest.Want{
			URLs:      []string{"https://api.anthropic.com/v1/messages", "https://api.anthropic.com/v1/messages/count_tokens"},
			KeyHeader: "X-Api-Key", Key: "test-key",
			Absent: []string{"Authorization", "X-From-Env", "Anthropic-Beta", "Anthropic-Workspace-Id", "Traceparent", "Tracestate"},
		})
		if n := p.tracers.Load(); n != 0 {
			t.Errorf("the SDK asked the process for %d tracers", n)
		}
	}
	for k, v := range vars {
		t.Run(k, func(t *testing.T) {
			set := map[string]string{k: v}
			// A profile is read from the folder of the configuration, which the test names too.
			if k == "ANTHROPIC_PROFILE" {
				set["ANTHROPIC_CONFIG_DIR"] = dir
			}
			check(t, set)
		})
	}
	// The federation takes its rule, organization and identity together.
	t.Run("federation", func(t *testing.T) {
		set := map[string]string{}
		for _, k := range []string{"ANTHROPIC_FEDERATION_RULE_ID", "ANTHROPIC_ORGANIZATION_ID", "ANTHROPIC_IDENTITY_TOKEN_FILE",
			"ANTHROPIC_SERVICE_ACCOUNT_ID", "ANTHROPIC_WORKSPACE_ID", "ANTHROPIC_SCOPE"} {
			set[k] = vars[k]
		}
		check(t, set)
	})
	t.Run("all", func(t *testing.T) { check(t, vars) })
}
