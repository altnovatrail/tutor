package claude

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/altnovatrail/tutor/provider"
)

// The events of a streamed answer, as the API sends them: the start with the input tokens, a
// thinking block, a ping, the text of the answer in two parts, and the end with the final count
// of the output tokens and of those spent thinking.
const (
	evStart     = `{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-5","content":[],"stop_reason":null,"stop_sequence":null,"stop_details":null,"usage":{"input_tokens":1300,"output_tokens":1}}}`
	evThinking  = `{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":"","signature":""}}`
	evSignature = `{"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"c2ln"}}`
	evStop0     = `{"type":"content_block_stop","index":0}`
	evText      = `{"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}`
	evPing      = `{"type":"ping"}`
	evDelta1    = `{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"{\"verdict\":\"error\","}}`
	evDelta2    = `{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"\"mark\":null}"}}`
	evStop1     = `{"type":"content_block_stop","index":1}`
	evEnd       = `{"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null,"stop_details":null},"usage":{"output_tokens":45,"output_tokens_details":{"thinking_tokens":30}}}`
	evMsgStop   = `{"type":"message_stop"}`
	answer      = `{"verdict":"error","mark":null}`
)

// sse is a stream of the events, each named by its own type, as the SDK reads them.
func sse(events ...string) string {
	var b strings.Builder
	for _, data := range events {
		var e struct {
			Type string `json:"type"`
		}
		_ = json.Unmarshal([]byte(data), &e)
		fmt.Fprintf(&b, "event: %s\ndata: %s\n\n", e.Type, data)
	}
	return b.String()
}

var goodStream = sse(evStart, evThinking, evSignature, evStop0, evText, evPing, evDelta1, evDelta2, evStop1, evEnd, evMsgStop)

// reply is one answer of the fake API: an event stream, or, with a status, an error in JSON.
type reply struct {
	status int
	header http.Header
	body   string
}

// fake keeps the raw body of every request the fake API got.
type fake struct {
	mu     sync.Mutex
	bodies []string
}

func (f *fake) requests() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.bodies)
}

// serve starts a fake API with the handler and returns a client of the model that calls it and
// sends a failed request again up to retries times.
func serve(t *testing.T, m Model, retries int, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return New("test-key", m, option.WithBaseURL(srv.URL), option.WithMaxRetries(retries))
}

// fakeAPI answers /v1/messages with the replies in turn, and with the last one once they run out.
// The client sends a failed request again as many times as there are replies after the first.
func fakeAPI(t *testing.T, m Model, replies ...reply) (*Client, *fake) {
	t.Helper()
	f := &fake{}
	c := serve(t, m, len(replies)-1, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" || r.Header.Get("X-Api-Key") != "test-key" {
			t.Errorf("request to %s with key %q", r.URL.Path, r.Header.Get("X-Api-Key"))
		}
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.bodies = append(f.bodies, string(body))
		rep := replies[min(len(f.bodies), len(replies))-1]
		f.mu.Unlock()
		maps.Copy(w.Header(), rep.header)
		if rep.status == 0 {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(rep.status)
		}
		_, _ = io.WriteString(w, rep.body)
	})
	return c, f
}

var request = provider.Request{
	Instructions: "Read and judge.",
	Text:         "Task: 3x + 1 = 7",
	Images:       []provider.Image{{PNG: []byte("png")}},
	SchemaName:   "verdict",
	Schema:       map[string]any{"type": "object"},
	Effort:       "high",
}

func holds(t *testing.T, body string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("the request does not hold %s:\n%s", want, body)
		}
	}
}

func TestAsk(t *testing.T) {
	c, f := fakeAPI(t, Models[2], reply{body: goodStream})
	resp, err := c.Ask(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if resp.JSON != answer || resp.Model != "claude-opus-5-5" || resp.InputTokens != 1300 || resp.OutputTokens != 45 ||
		resp.ThinkingTokens != 30 || resp.Retries != 0 || resp.Partial || resp.FirstToken <= 0 {
		t.Errorf("response = %+v", resp)
	}
	sent := f.requests()[0]
	holds(t, sent, `"model":"claude-opus-5-5"`, `"stream":true`, `"type":"image"`, `"media_type":"image/png"`,
		`"transformations":{"oversized_image":"error"}`, `"effort":"high"`, `"type":"json_schema"`, `"system":[{"text":"Read and judge."`)
	if strings.Contains(sent, `"thinking"`) {
		t.Errorf("a request with its thinking on sent a thinking setting:\n%s", sent)
	}
	if c.Name() != "Claude Opus 5.5" {
		t.Errorf("name = %q", c.Name())
	}
	if in, out := c.Price(); in != 4 || out != 20 {
		t.Errorf("price = %v, %v", in, out)
	}
}

// A value of the schema written as JSON goes out as it is written, in its own order.
func TestSchemaOrder(t *testing.T) {
	c, f := fakeAPI(t, Models[2], reply{body: goodStream})
	r := request
	r.Schema = map[string]any{"type": "object", "properties": json.RawMessage(`{"verdict": {}, "mark": {}}`)}
	if _, err := c.Ask(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	holds(t, f.requests()[0], `"properties":{"verdict":{},"mark":{}}`)
}

// The model and the effort a request names are the ones that reach the API, and a request without
// an effort leaves the model its own default.
func TestModelAndEffort(t *testing.T) {
	c, f := fakeAPI(t, Models[0], reply{body: goodStream})
	r := request
	r.Effort = "low"
	if _, err := c.Ask(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	holds(t, f.requests()[0], `"model":"claude-haiku-5-5"`, `"effort":"low"`)
	r.Effort = ""
	if _, err := c.Ask(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	if sent := f.requests()[1]; strings.Contains(sent, `"effort"`) {
		t.Errorf("a request without an effort sent one:\n%s", sent)
	}
}

// Haiku 5.5 turns its thinking off with "disabled", Sonnet 5.5 with "between_tools".
func TestNoThinking(t *testing.T) {
	for _, tt := range []struct {
		model Model
		want  string
	}{{Models[0], `"thinking":{"type":"disabled"}`}, {Models[1], `"thinking":{"type":"between_tools"}`}} {
		t.Run(tt.model.Name, func(t *testing.T) {
			c, f := fakeAPI(t, tt.model, reply{body: goodStream})
			r := request
			r.Effort, r.NoThinking = "low", true
			if _, err := c.Ask(context.Background(), r); err != nil {
				t.Fatal(err)
			}
			holds(t, f.requests()[0], tt.want, `"effort":"low"`)
		})
	}
}

// What a model cannot do is refused before any call: an unknown effort, and an answer without
// thinking on a model whose thinking does not turn off, at xhigh or max, or with no effort named.
func TestRefusedBeforeCall(t *testing.T) {
	tests := []struct {
		name       string
		model      Model
		effort     string
		noThinking bool
		want       string
	}{
		{"an unknown effort", Models[2], "extreme", false, `no effort level "extreme"`},
		{"Opus without thinking", Models[2], "low", true, "cannot answer without thinking"},
		{"Fable without thinking", Models[3], "low", true, "cannot answer without thinking"},
		{"Haiku without thinking at xhigh", Models[0], "xhigh", true, `at the effort "xhigh"`},
		{"Haiku without thinking at max", Models[0], "max", true, `at the effort "max"`},
		{"Sonnet without thinking at max", Models[1], "max", true, `at the effort "max"`},
		{"Haiku without thinking and no effort", Models[0], "", true, `at the effort ""`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, f := fakeAPI(t, tt.model, reply{body: goodStream})
			r := request
			r.Effort, r.NoThinking = tt.effort, tt.noThinking
			if _, err := c.Ask(context.Background(), r); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v, want it to hold %q", err, tt.want)
			}
			if n := len(f.requests()); n != 0 {
				t.Errorf("the API was called %d times", n)
			}
		})
	}
}

// The first token is when the answer's text begins, not when the stream does.
func TestFirstToken(t *testing.T) {
	head := sse(evStart, evThinking, evSignature, evStop0, evText)
	text := sse(evDelta1, evDelta2)
	tail := sse(evStop1, evEnd, evMsgStop)
	c := serve(t, Models[2], 0, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		for _, part := range []struct {
			body  string
			pause time.Duration
		}{{head, 50 * time.Millisecond}, {text, 150 * time.Millisecond}, {tail, 0}} {
			_, _ = io.WriteString(w, part.body)
			flusher.Flush()
			time.Sleep(part.pause)
		}
	})
	start := time.Now()
	resp, err := c.Ask(context.Background(), request)
	took := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if resp.FirstToken < 50*time.Millisecond || resp.FirstToken > took-100*time.Millisecond {
		t.Errorf("first token at %v of %v, want it after the thinking and well before the end", resp.FirstToken, took)
	}
}

// A request the SDK sends again after a failure counts as a retry.
func TestRetry(t *testing.T) {
	overloaded := reply{status: 529, header: http.Header{"Retry-After-Ms": {"0"}}, body: `{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`}
	c, f := fakeAPI(t, Models[2], overloaded, reply{body: goodStream})
	resp, err := c.Ask(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Retries != 1 || len(f.requests()) != 2 || resp.JSON != answer {
		t.Errorf("response = %+v after %d requests, want one retry", resp, len(f.requests()))
	}
}

// A call that fails before the stream begins has no tokens; one that breaks off after it began is
// partial, with the tokens counted so far.
func TestFailures(t *testing.T) {
	t.Run("a refused request", func(t *testing.T) {
		c, f := fakeAPI(t, Models[2], reply{status: 400, body: `{"type":"error","error":{"type":"invalid_request_error","message":"bad"}}`})
		resp, err := c.Ask(context.Background(), request)
		var apiErr *anthropic.Error
		if !errors.As(err, &apiErr) || apiErr.StatusCode != 400 || resp.InputTokens != 0 || resp.Partial || len(f.requests()) != 1 {
			t.Errorf("= %+v, %v, want a 400 without tokens, asked once", resp, err)
		}
	})
	broken := map[string]string{
		"an error in the stream":  sse(evStart, `{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`),
		"a stream that just ends": sse(evStart, evText, evDelta1),
	}
	for name, body := range broken {
		t.Run(name, func(t *testing.T) {
			c, _ := fakeAPI(t, Models[2], reply{body: body})
			resp, err := c.Ask(context.Background(), request)
			if err == nil || !resp.Partial || resp.InputTokens != 1300 {
				t.Errorf("= %+v, %v, want a partial answer with its input tokens", resp, err)
			}
		})
	}
	t.Run("a stream the clock cuts off", func(t *testing.T) {
		c := serve(t, Models[2], 0, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, sse(evStart))
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		})
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		resp, err := c.Ask(ctx, request)
		if !errors.Is(err, context.DeadlineExceeded) || !resp.Partial || resp.InputTokens != 1300 {
			t.Errorf("= %+v, %v, want a partial answer cut off by the clock", resp, err)
		}
	})
}

// Every model of the list has its own ID, name and price, and its thinking turns off at high and
// below with the setting Anthropic names for it, or not at all.
func TestModels(t *testing.T) {
	ids, names := map[string]bool{}, map[string]bool{}
	for _, m := range Models {
		if m.ID == "" || m.Name == "" || m.Input <= 0 || m.Output <= m.Input || ids[m.ID] || names[m.Name] {
			t.Errorf("model %+v", m)
		}
		ids[m.ID], names[m.Name] = true, true
		want := []string{"low", "medium", "high"}
		switch m.ThinkingOff {
		case "":
			want = nil
		case "disabled", "between_tools":
		default:
			t.Errorf("%s turns its thinking off with %q, which the adapter does not send", m.Name, m.ThinkingOff)
		}
		if got := m.NoThinkingEfforts(); !slices.Equal(got, want) {
			t.Errorf("%s answers without thinking at %v, want %v", m.Name, got, want)
		}
	}
}

func TestNoAnswer(t *testing.T) {
	tests := map[string]string{
		"a refusal": sse(evStart, `{"type":"message_delta","delta":{"stop_reason":"refusal","stop_sequence":null,"stop_details":{"type":"refusal","category":"cyber","explanation":"x"}},"usage":{"output_tokens":3}}`, evMsgStop),
		"out of tokens": sse(evStart, evThinking, evStop0, evText, evDelta1, evStop1,
			`{"type":"message_delta","delta":{"stop_reason":"max_tokens","stop_sequence":null,"stop_details":null},"usage":{"output_tokens":16000,"output_tokens_details":{"thinking_tokens":15990}}}`, evMsgStop),
		"no text": sse(evStart, `{"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null,"stop_details":null},"usage":{"output_tokens":2}}`, evMsgStop),
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			c, _ := fakeAPI(t, Models[2], reply{body: body})
			resp, err := c.Ask(context.Background(), request)
			if !errors.Is(err, provider.ErrNoAnswer) || resp.Partial || resp.InputTokens != 1300 || resp.OutputTokens == 0 {
				t.Errorf("= %+v, %v, want ErrNoAnswer with the billed tokens", resp, err)
			}
		})
	}
}

// The pictures go in their order in one message, each a block of its own after a block of its
// label, and the text after them.
func TestImages(t *testing.T) {
	c, f := fakeAPI(t, Models[2], reply{body: goodStream})
	r := request
	r.Images = []provider.Image{{Label: "Image 1: the sheet as it was finished.", PNG: []byte("one")},
		{Label: "Image 2: the sheet before erasure 1.", PNG: []byte("two")}, {Label: "Image 3: the sheet before erasure 2.", PNG: []byte("three")}}
	if _, err := c.Ask(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	var sent struct {
		Messages []struct {
			Content []struct {
				Type   string `json:"type"`
				Text   string `json:"text"`
				Source struct {
					Data string `json:"data"`
				} `json:"source"`
				Transformations struct {
					OversizedImage string `json:"oversized_image"`
				} `json:"transformations"`
			} `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal([]byte(f.requests()[0]), &sent); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, b := range sent.Messages[0].Content {
		switch b.Type {
		case "text":
			got = append(got, "text "+b.Text)
		case "image":
			got = append(got, "image "+b.Source.Data+" "+b.Transformations.OversizedImage)
		}
	}
	want := []string{
		"text Image 1: the sheet as it was finished.", "image b25l error",
		"text Image 2: the sheet before erasure 1.", "image dHdv error",
		"text Image 3: the sheet before erasure 2.", "image dGhyZWU= error",
		"text Task: 3x + 1 = 7",
	}
	if len(sent.Messages) != 1 || !slices.Equal(got, want) {
		t.Errorf("the message = %q, want %q", got, want)
	}
}

// Pictures the adapter will not send are refused before any call: none, an empty one, several
// without their labels, more than MaxImages.
func TestImagesRefused(t *testing.T) {
	c, f := fakeAPI(t, Models[2], reply{body: goodStream})
	many := make([]provider.Image, MaxImages+1)
	for i := range many {
		many[i] = provider.Image{Label: fmt.Sprintf("Image %d:", i+1), PNG: []byte("png")}
	}
	for _, tt := range []struct {
		images []provider.Image
		want   string
	}{
		{nil, "a request without a picture"},
		{[]provider.Image{{PNG: nil}}, "picture 1 has no PNG"},
		{[]provider.Image{{Label: "Image 1:", PNG: []byte("a")}, {PNG: []byte("b")}}, "picture 2 of 2 has no label"},
		{many, "a request of 21 pictures, and the adapter sends 20 at most"},
	} {
		r := request
		r.Images = tt.images
		if _, err := c.Ask(context.Background(), r); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%d pictures: %v, want %q", len(tt.images), err, tt.want)
		}
	}
	if n := len(f.requests()); n != 0 {
		t.Errorf("refused requests reached the API %d times", n)
	}
	// One picture without a label goes as it is, as the board sends it.
	if _, err := c.Ask(context.Background(), request); err != nil {
		t.Error(err)
	}
	// A text alone, as the assessment of a text an earlier call read, goes without a picture (T92).
	textOnly := request
	textOnly.Images, textOnly.NoPicture = nil, true
	if _, err := c.Ask(context.Background(), textOnly); err != nil {
		t.Error(err)
	}
	// Marked a text alone, it is refused with a picture or a blank text, and reaches no API.
	for _, bad := range []provider.Request{{Text: "x", Images: request.Images, NoPicture: true}, {Text: " \n", NoPicture: true}} {
		if _, err := c.Ask(context.Background(), bad); err == nil || !strings.Contains(err.Error(), "a request of a text alone") {
			t.Errorf("%+v: %v", bad, err)
		}
	}
	if bodies := f.requests(); len(bodies) != 2 || strings.Contains(bodies[1], `"image"`) || !strings.Contains(bodies[1], request.Text) {
		t.Errorf("the request of a text alone is %v", bodies[len(bodies)-1:])
	}
}
