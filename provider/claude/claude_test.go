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

// fakeAPI answers /v1/messages with the given body, keeps the request it got and counts the calls.
func fakeAPI(t *testing.T, m Model, answer string, got *map[string]any) (*Client, *int) {
	t.Helper()
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
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
	return New("test-key", m, option.WithBaseURL(srv.URL), option.WithMaxRetries(0)), &calls
}

var request = provider.Request{
	Instructions: "Read and judge.",
	Text:         "Task: 3x + 1 = 7",
	PNG:          []byte("png"),
	SchemaName:   "verdict",
	Schema:       map[string]any{"type": "object"},
	Effort:       "high",
}

const goodAnswer = `{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-5",
	"content":[{"type":"text","text":"{\"verdict\":\"correct\"}"}],"stop_reason":"end_turn","stop_details":null,
	"usage":{"input_tokens":120,"output_tokens":45}}`

func TestAsk(t *testing.T) {
	var got map[string]any
	c, _ := fakeAPI(t, Models[2], goodAnswer, &got)
	resp, err := c.Ask(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if resp.JSON != `{"verdict":"correct"}` || resp.Model != "claude-opus-5-5" || resp.InputTokens != 120 || resp.OutputTokens != 45 {
		t.Errorf("response = %+v", resp)
	}
	sent, _ := json.Marshal(got)
	for _, want := range []string{`"model":"claude-opus-5-5"`, `"type":"image"`, `"media_type":"image/png"`, `"effort":"high"`, `"type":"json_schema"`, `"system":[{"text":"Read and judge."`} {
		if !strings.Contains(string(sent), want) {
			t.Errorf("the request does not hold %s:\n%s", want, sent)
		}
	}
	if c.Name() != "Claude Opus 5.5" {
		t.Errorf("name = %q", c.Name())
	}
	if in, out := c.Price(); in != 4 || out != 20 {
		t.Errorf("price = %v, %v", in, out)
	}
}

// The model and the effort a request names are the ones that reach the API, and a request without
// an effort leaves the model its own default.
func TestModelAndEffort(t *testing.T) {
	var got map[string]any
	c, _ := fakeAPI(t, Models[0], goodAnswer, &got)
	r := request
	r.Effort = "low"
	if _, err := c.Ask(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	sent, _ := json.Marshal(got)
	for _, want := range []string{`"model":"claude-haiku-5-5"`, `"effort":"low"`} {
		if !strings.Contains(string(sent), want) {
			t.Errorf("the request does not hold %s:\n%s", want, sent)
		}
	}
	r.Effort = ""
	if _, err := c.Ask(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	if sent, _ := json.Marshal(got); strings.Contains(string(sent), `"effort"`) {
		t.Errorf("a request without an effort sent one:\n%s", sent)
	}
}

func TestUnknownEffort(t *testing.T) {
	var got map[string]any
	c, calls := fakeAPI(t, Models[2], goodAnswer, &got)
	r := request
	r.Effort = "extreme"
	if _, err := c.Ask(context.Background(), r); err == nil || !strings.Contains(err.Error(), `no effort level "extreme"`) {
		t.Errorf("error = %v, want the unknown level named", err)
	}
	if *calls != 0 {
		t.Errorf("the API was called %d times for an unknown effort", *calls)
	}
}

// Every model of the list has its own ID, name and price.
func TestModels(t *testing.T) {
	ids, names := map[string]bool{}, map[string]bool{}
	for _, m := range Models {
		if m.ID == "" || m.Name == "" || m.Input <= 0 || m.Output <= m.Input || ids[m.ID] || names[m.Name] {
			t.Errorf("model %+v", m)
		}
		ids[m.ID], names[m.Name] = true, true
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
			c, _ := fakeAPI(t, Models[2], answer, &got)
			if _, err := c.Ask(context.Background(), request); !errors.Is(err, provider.ErrNoAnswer) {
				t.Errorf("error = %v, want ErrNoAnswer", err)
			}
		})
	}
}
