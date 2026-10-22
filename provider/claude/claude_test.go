package claude

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/altnovatrail/tutor/provider"
)

// fakeAPI answers /v1/messages with the given body and keeps the request it got.
func fakeAPI(t *testing.T, answer string, got *map[string]any) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" || r.Header.Get("X-Api-Key") != "test-key" {
			t.Errorf("request to %s with key %q", r.URL.Path, r.Header.Get("X-Api-Key"))
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, got); err != nil {
			t.Errorf("request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, answer)
	}))
	t.Cleanup(srv.Close)
	return New("test-key", option.WithBaseURL(srv.URL), option.WithMaxRetries(0))
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
	c := fakeAPI(t, `{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-5",
		"content":[{"type":"text","text":"{\"verdict\":\"correct\"}"}],"stop_reason":"end_turn","stop_details":null,
		"usage":{"input_tokens":120,"output_tokens":45}}`, &got)
	resp, err := c.Ask(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if resp.JSON != `{"verdict":"correct"}` || resp.Model != Model || resp.InputTokens != 120 || resp.OutputTokens != 45 {
		t.Errorf("response = %+v", resp)
	}
	sent, _ := json.Marshal(got)
	for _, want := range []string{`"model":"claude-opus-5-5"`, `"type":"image"`, `"media_type":"image/png"`, `"effort":"high"`, `"type":"json_schema"`, `"system":[{"text":"Read and judge."`} {
		if !strings.Contains(string(sent), want) {
			t.Errorf("the request does not hold %s:\n%s", want, sent)
		}
	}
}

func TestNoAnswer(t *testing.T) {
	tests := map[string]string{
		"a refusal": `{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-5","content":[],
			"stop_reason":"refusal","stop_details":{"type":"refusal","category":"cyber","explanation":"x"},"usage":{"input_tokens":1,"output_tokens":1}}`,
		"out of tokens": `{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-5","content":[{"type":"text","text":"{"}],
			"stop_reason":"max_tokens","stop_details":null,"usage":{"input_tokens":1,"output_tokens":1}}`,
		"no text": `{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-5","content":[],
			"stop_reason":"end_turn","stop_details":null,"usage":{"input_tokens":1,"output_tokens":1}}`,
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
