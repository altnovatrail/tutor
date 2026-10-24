package gpt

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/openai/openai-go/v3/option"

	"github.com/altnovatrail/tutor/provider"
)

// fakeAPI answers /responses with the given body and keeps the request it got.
func fakeAPI(t *testing.T, answer string, got *map[string]any) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/responses") || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("request to %s with %q", r.URL.Path, r.Header.Get("Authorization"))
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, got); err != nil {
			t.Errorf("request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, answer)
	}))
	t.Cleanup(srv.Close)
	return New("test-key", option.WithBaseURL(srv.URL+"/v1/"), option.WithMaxRetries(0))
}

var request = provider.Request{
	Instructions: "Read and judge.",
	Text:         "Task: 3x + 1 = 7",
	PNG:          []byte("png"),
	SchemaName:   "verdict",
	Schema:       map[string]any{"type": "object"},
}

func TestAsk(t *testing.T) {
	var got map[string]any
	c := fakeAPI(t, `{"id":"resp_1","object":"response","status":"completed","model":"gpt-6-astra",
		"output":[{"type":"message","id":"m1","status":"completed","role":"assistant",
			"content":[{"type":"output_text","text":"{\"verdict\":\"error\"}","annotations":[]}]}],
		"usage":{"input_tokens":300,"output_tokens":80,"total_tokens":380,
			"input_tokens_details":{"cached_tokens":0},"output_tokens_details":{"reasoning_tokens":40}}}`, &got)
	resp, err := c.Ask(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if resp.JSON != `{"verdict":"error"}` || resp.Model != Model || resp.InputTokens != 300 || resp.OutputTokens != 80 {
		t.Errorf("response = %+v", resp)
	}
	sent, _ := json.Marshal(got)
	for _, want := range []string{`"model":"gpt-6-astra"`, `"type":"input_image"`, `"detail":"high"`, `data:image/png;base64,`,
		`"strict":true`, `"type":"json_schema"`, `"effort":"high"`, `"store":false`, `"instructions":"Read and judge."`} {
		if !strings.Contains(string(sent), want) {
			t.Errorf("the request does not hold %s:\n%s", want, sent)
		}
	}
}

// The adapter takes high effort alone and refuses another level before it calls the API.
func TestOtherEffort(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls++ }))
	t.Cleanup(srv.Close)
	c := New("test-key", option.WithBaseURL(srv.URL+"/v1/"), option.WithMaxRetries(0))
	r := request
	r.Effort = "low"
	if _, err := c.Ask(context.Background(), r); err == nil || !strings.Contains(err.Error(), `no effort level "low"`) {
		t.Errorf("error = %v, want the level refused", err)
	}
	if calls != 0 {
		t.Errorf("the API was called %d times for a level the adapter does not take", calls)
	}
}

func TestNoAnswer(t *testing.T) {
	tests := map[string]string{
		"incomplete": `{"id":"resp_1","object":"response","status":"incomplete","model":"gpt-6-astra",
			"incomplete_details":{"reason":"max_output_tokens"},"output":[],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`,
		"a refusal": `{"id":"resp_1","object":"response","status":"completed","model":"gpt-6-astra",
			"output":[{"type":"message","id":"m1","status":"completed","role":"assistant",
				"content":[{"type":"refusal","refusal":"I can't help with that."}]}],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`,
	}
	for name, answer := range tests {
		t.Run(name, func(t *testing.T) {
			var got map[string]any
			if _, err := fakeAPI(t, answer, &got).Ask(context.Background(), request); !errors.Is(err, provider.ErrNoAnswer) {
				t.Errorf("error = %v, want ErrNoAnswer", err)
			}
		})
	}
}
