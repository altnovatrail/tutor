package gpt

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/openai/openai-go/v3/option"

	"github.com/altnovatrail/tutor/provider"
)

// reply is one answer of the fake API: a status, 200 if none, and a JSON body.
type reply struct {
	status int
	header http.Header
	body   string
}

// fakeAPI answers /responses with the replies in turn, and with the last one once they run out,
// and returns the raw bodies of the requests it got. The client sends a failed request again as
// many times as there are replies after the first.
func fakeAPI(t *testing.T, replies ...reply) (*Client, func() []string) {
	t.Helper()
	var mu sync.Mutex
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/responses") || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("request to %s with %q", r.URL.Path, r.Header.Get("Authorization"))
		}
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(body))
		rep := replies[min(len(bodies), len(replies))-1]
		mu.Unlock()
		for k, v := range rep.header {
			w.Header()[k] = v
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(max(rep.status, http.StatusOK))
		_, _ = io.WriteString(w, rep.body)
	}))
	t.Cleanup(srv.Close)
	sent := func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), bodies...)
	}
	return New("test-key", option.WithBaseURL(srv.URL+"/v1/"), option.WithMaxRetries(len(replies)-1)), sent
}

const goodAnswer = `{"id":"resp_1","object":"response","status":"completed","model":"gpt-6-astra",
	"output":[{"type":"message","id":"m1","status":"completed","role":"assistant",
		"content":[{"type":"output_text","text":"{\"verdict\":\"error\"}","annotations":[]}]}],
	"usage":{"input_tokens":300,"output_tokens":80,"total_tokens":380,
		"input_tokens_details":{"cached_tokens":0},"output_tokens_details":{"reasoning_tokens":40}}}`

var request = provider.Request{
	Instructions: "Read and judge.",
	Text:         "Task: 3x + 1 = 7",
	Images:       []provider.Image{{PNG: []byte("png")}},
	SchemaName:   "verdict",
	Schema:       map[string]any{"type": "object"},
}

func TestAsk(t *testing.T) {
	c, sent := fakeAPI(t, reply{body: goodAnswer})
	resp, err := c.Ask(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if resp.JSON != `{"verdict":"error"}` || resp.Model != Model || resp.InputTokens != 300 || resp.OutputTokens != 80 ||
		resp.ThinkingTokens != 40 || resp.Retries != 0 || resp.FirstToken != 0 {
		t.Errorf("response = %+v", resp)
	}
	body := sent()[0]
	var got map[string]any
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("request body: %v", err)
	}
	for _, want := range []string{`"model":"gpt-6-astra"`, `"type":"input_image"`, `"detail":"high"`, `data:image/png;base64,`,
		`"strict":true`, `"type":"json_schema"`, `"effort":"high"`, `"store":false`, `"instructions":"Read and judge."`} {
		if !strings.Contains(body, want) {
			t.Errorf("the request does not hold %s:\n%s", want, body)
		}
	}
}

// A value of the schema written as JSON goes out as it is written, in its own order.
func TestSchemaOrder(t *testing.T) {
	c, sent := fakeAPI(t, reply{body: goodAnswer})
	r := request
	r.Schema = map[string]any{"type": "object", "properties": json.RawMessage(`{"verdict":{},"mark":{}}`)}
	if _, err := c.Ask(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	if body := sent()[0]; !strings.Contains(body, `"properties":{"verdict":{},"mark":{}}`) {
		t.Errorf("the schema lost its order:\n%s", body)
	}
}

// The adapter takes high effort alone, with reasoning, and refuses anything else before it calls
// the API.
func TestRefusedBeforeCall(t *testing.T) {
	tests := map[string]struct {
		effort     string
		noThinking bool
		want       string
	}{
		"another level":     {"low", false, `no effort level "low"`},
		"without reasoning": {"high", true, "cannot answer without reasoning"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			c, sent := fakeAPI(t, reply{body: goodAnswer})
			r := request
			r.Effort, r.NoThinking = tt.effort, tt.noThinking
			if _, err := c.Ask(context.Background(), r); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v, want it to hold %q", err, tt.want)
			}
			if n := len(sent()); n != 0 {
				t.Errorf("the API was called %d times", n)
			}
		})
	}
}

// A request the SDK sends again after a failure counts as a retry.
func TestRetry(t *testing.T) {
	failed := reply{status: http.StatusInternalServerError, header: http.Header{"Retry-After-Ms": {"0"}}, body: `{"error":{"message":"down","type":"server_error"}}`}
	c, sent := fakeAPI(t, failed, reply{body: goodAnswer})
	resp, err := c.Ask(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Retries != 1 || len(sent()) != 2 {
		t.Errorf("response = %+v after %d requests, want one retry", resp, len(sent()))
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
			c, _ := fakeAPI(t, reply{body: answer})
			if _, err := c.Ask(context.Background(), request); !errors.Is(err, provider.ErrNoAnswer) {
				t.Errorf("error = %v, want ErrNoAnswer", err)
			}
		})
	}
}

// The adapter sends one picture at most, after its label, and refuses more before it calls the API.
func TestImages(t *testing.T) {
	c, sent := fakeAPI(t, reply{body: goodAnswer})
	r := request
	r.Images = []provider.Image{{PNG: []byte("one")}, {PNG: []byte("two")}}
	if _, err := c.Ask(context.Background(), r); err == nil || !strings.Contains(err.Error(), "a request of 2 pictures, and the adapter sends 1 at most") {
		t.Errorf("two pictures: %v, want them refused", err)
	}
	if n := len(sent()); n != 0 {
		t.Fatalf("the refused request reached the API %d times", n)
	}
	r.Images = []provider.Image{{Label: "Image 1: the sheet.", PNG: []byte("one")}}
	if _, err := c.Ask(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	body := sent()[0]
	label, image, text := strings.Index(body, `"text":"Image 1: the sheet."`), strings.Index(body, `"type":"input_image"`), strings.Index(body, `"text":"Task: 3x + 1 = 7"`)
	if label < 0 || image < 0 || text < 0 || label > image || image > text {
		t.Errorf("the request holds the label at %d, the picture at %d and the text at %d; want them in that order:\n%s", label, image, text, body)
	}
}
