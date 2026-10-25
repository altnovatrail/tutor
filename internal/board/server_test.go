package board

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/altnovatrail/tutor/provider"
)

// fakeModel answers every request the same way and remembers what it was asked.
type fakeModel struct {
	name    string
	answer  string
	err     error
	billed  int64 // tokens a failed call still bills
	partial bool  // a failed call broke off after its answer began
	mu      sync.Mutex
	asked   []provider.Request
}

func (f *fakeModel) Name() string                   { return f.name }
func (f *fakeModel) Price() (input, output float64) { return 4, 20 }
func (f *fakeModel) Ask(_ context.Context, r provider.Request) (provider.Response, error) {
	f.mu.Lock()
	f.asked = append(f.asked, r)
	f.mu.Unlock()
	if f.err != nil {
		return provider.Response{InputTokens: f.billed, OutputTokens: f.billed, Partial: f.partial}, f.err
	}
	return provider.Response{JSON: f.answer, Model: f.name + "-id", InputTokens: 1000, OutputTokens: 500, ThinkingTokens: 300,
		FirstToken: 1500 * time.Millisecond, Retries: 1}, nil
}

func (f *fakeModel) requests() []provider.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]provider.Request(nil), f.asked...)
}

const goodAnswer = `{"verdict":"correct","mark":null}`

// sheet is a small white PNG with a black line, as the page sends it.
func sheet(t *testing.T, w, h int) string {
	t.Helper()
	img := image.NewGray(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	for x := 0; x < w; x++ {
		img.SetGray(x, h/2, color.Gray{Y: 0})
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

// offer offers every model at low and high effort, starting on high and ticked, and without its
// thinking at low.
func offer(models ...provider.Model) []Choice {
	choices := make([]Choice, len(models))
	for i, m := range models {
		choices[i] = Choice{Model: m, Efforts: []string{"low", "high"}, Effort: "high", Ticked: true, NoThinking: []string{"low"}}
	}
	return choices
}

// pick asks every named model at high effort.
func pick(names ...string) []Pick {
	picks := make([]Pick, len(names))
	for i, name := range names {
		picks[i] = Pick{Model: name, Effort: "high"}
	}
	return picks
}

func newBoard(t *testing.T, dir string, maxUSD float64, models ...provider.Model) http.Handler {
	t.Helper()
	b, err := New(offer(models...), dir, maxUSD)
	if err != nil {
		t.Fatal(err)
	}
	return b.Handler(boardAddr)
}

const boardAddr = "127.0.0.1:8080"

func call(t *testing.T, h http.Handler, method, path string, body any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(data)
	} else {
		reader = bytes.NewReader(nil)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, reader)
	req.Host = boardAddr
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	h.ServeHTTP(rec, req)
	var out map[string]any
	if strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
	}
	return rec, out
}

// line is the summary's line of a model at an effort, with its thinking, of short answers, or nil.
func line(t *testing.T, summary map[string]any, model, effort string) map[string]any {
	t.Helper()
	return lineOf(t, summary, model, effort, false, formShort)
}

// lineOf is the summary's line of a model at an effort, with its thinking or without, of answers
// of the form, or nil.
func lineOf(t *testing.T, summary map[string]any, model, effort string, noThinking bool, form string) map[string]any {
	t.Helper()
	for _, p := range summary["pairs"].([]any) {
		if l := p.(map[string]any); l["model"] == model && l["effort"] == effort && l["no_thinking"] == noThinking && l["form"] == form {
			return l
		}
	}
	return nil
}

func TestPageTasksAndModels(t *testing.T) {
	h := newBoard(t, t.TempDir(), 5, &fakeModel{name: "A", answer: goodAnswer})
	rec, _ := call(t, h, "GET", "/", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Check board") {
		t.Fatalf("GET / = %d, want the page", rec.Code)
	}
	// The summary's headers are in the markup, which a static reading of the page sees, and the
	// marks have a layer of their own.
	for _, want := range []string{`<th scope="col">Mean first token</th>`, `<th scope="col">Mean tokens out</th>`,
		`<th scope="col">Mean thinking tokens</th>`, `<canvas id="marks"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("the page does not hold %s", want)
		}
	}
	rec, _ = call(t, h, "GET", "/api/tasks", nil)
	var tasks []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &tasks); err != nil || len(tasks) != 5 {
		t.Fatalf("GET /api/tasks = %s, want five tasks", rec.Body)
	}
	for _, task := range tasks {
		if _, leaked := task["key"]; leaked || strings.Contains(rec.Body.String(), "x = 2") {
			t.Fatalf("the tasks sent to the page hold their answers: %s", rec.Body)
		}
	}
	rec, _ = call(t, h, "GET", "/api/models", nil)
	var offers []Offer
	if err := json.Unmarshal(rec.Body.Bytes(), &offers); err != nil || len(offers) != 1 {
		t.Fatalf("GET /api/models = %s, want one model", rec.Body)
	}
	if o := offers[0]; o.Model != "A" || o.Input != 4 || o.Output != 20 || strings.Join(o.Efforts, ",") != "low,high" || o.Effort != "high" || !o.Ticked ||
		strings.Join(o.NoThinking, ",") != "low" {
		t.Errorf("the offered model = %+v", o)
	}

	// A model whose thinking does not turn off is offered with an empty list, which the page reads.
	b, err := New([]Choice{{Model: &fakeModel{name: "B"}, Efforts: []string{"high"}, Effort: "high"}}, t.TempDir(), 5)
	if err != nil {
		t.Fatal(err)
	}
	rec, _ = call(t, b.Handler(boardAddr), "GET", "/api/models", nil)
	if !strings.Contains(rec.Body.String(), `"no_thinking_efforts":[]`) {
		t.Errorf("GET /api/models = %s, want an empty list of levels without thinking", rec.Body)
	}
}

func TestNewRefusesBadChoices(t *testing.T) {
	a := &fakeModel{name: "A"}
	tests := map[string][]Choice{
		"no model":                                 nil,
		"a model offered twice":                    append(offer(a), offer(a)...),
		"a level it is not offered":                {{Model: a, Efforts: []string{"low"}, Effort: "high"}},
		"no thinking at a level it is not offered": {{Model: a, Efforts: []string{"low"}, Effort: "low", NoThinking: []string{"high"}}},
	}
	for name, choices := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := New(choices, t.TempDir(), 5); err == nil {
				t.Error("taken")
			}
		})
	}
}

func TestCheckAndMark(t *testing.T) {
	dir := t.TempDir()
	good := &fakeModel{name: "Claude", answer: goodAnswer}
	broken := &fakeModel{name: "GPT", err: fmt.Errorf("%w: the model refused", provider.ErrNoAnswer), billed: 100}
	h := newBoard(t, dir, 5, good, broken)

	rec, out := call(t, h, "POST", "/api/check", CheckRequest{Task: "equation-1", Image: sheet(t, 100, 60), Models: pick("GPT", "Claude")})
	if rec.Code != http.StatusOK {
		t.Fatalf("check = %d %s", rec.Code, rec.Body)
	}
	results := out["results"].([]any)
	first, second := results[0].(map[string]any), results[1].(map[string]any)
	if first["model"] != "Claude" || first["verdict"].(map[string]any)["verdict"] != "correct" || first["cost_usd"].(float64) != 0.014 ||
		first["effort"] != "high" || first["input_tokens"].(float64) != 1000 || first["output_tokens"].(float64) != 500 {
		t.Errorf("the good model's result = %v, want it first, as the page offers the models", first)
	}
	if !strings.Contains(second["error"].(string), "gave no answer") || second["cost_usd"].(float64) != 0.0024 || second["output_tokens"].(float64) != 100 {
		t.Errorf("the broken model's result = %v, want its error in words and its billed tokens", second)
	}
	asked := good.requests()[0]
	if !strings.HasPrefix(asked.Text, "The picture is 100 pixels wide and 60 pixels high.\n") || !strings.Contains(asked.Text, "Correct answer: x = 2") ||
		len(asked.PNG) == 0 || asked.Schema == nil || asked.Effort != "high" || asked.NoThinking {
		t.Errorf("the model was asked %+v, want the picture's size, the task's answer, the picture, the schema and the effort", asked)
	}

	attempt := out["attempt"].(string)
	if _, err := os.Stat(filepath.Join(dir, attempt+".png")); err != nil {
		t.Errorf("the picture of the try is not kept: %v", err)
	}
	rec, out = call(t, h, "POST", "/api/mark", MarkRequest{Attempt: attempt, Model: "Claude", Right: true})
	if rec.Code != http.StatusOK {
		t.Fatalf("mark = %d %s", rec.Code, rec.Body)
	}
	if l := line(t, out, "Claude", "high"); l == nil || l["marked"].(float64) != 1 || l["right"].(float64) != 1 ||
		l["answers"].(float64) != 1 || l["mean_cost_usd"].(float64) != 0.014 {
		t.Errorf("summary after one mark = %v", out)
	}
	if l := line(t, out, "GPT", "high"); l == nil || l["answers"].(float64) != 1 || l["no_answer"].(float64) != 1 || l["marked"].(float64) != 0 {
		t.Errorf("the broken model's line = %v", out)
	}
	// Marking again changes the mark rather than adding one.
	_, out = call(t, h, "POST", "/api/mark", MarkRequest{Attempt: attempt, Model: "Claude", Right: false})
	if l := line(t, out, "Claude", "high"); l["marked"].(float64) != 1 || l["right"].(float64) != 0 {
		t.Errorf("summary after a changed mark = %v", out)
	}

	// A new run reads the answers and marks back from the log.
	h = newBoard(t, dir, 5, good, broken)
	_, out = call(t, h, "GET", "/api/summary", nil)
	if l := line(t, out, "Claude", "high"); l == nil || l["marked"].(float64) != 1 || l["right"].(float64) != 0 || l["answers"].(float64) != 1 {
		t.Errorf("summary after a restart = %v", out)
	}
	rec, _ = call(t, h, "POST", "/api/mark", MarkRequest{Attempt: attempt, Model: "Claude", Right: true})
	if rec.Code != http.StatusOK {
		t.Errorf("marking a try of an earlier run = %d %s", rec.Code, rec.Body)
	}
	// An answer without a verdict has nothing to mark, as the page shows no marks under it.
	rec, out = call(t, h, "POST", "/api/mark", MarkRequest{Attempt: attempt, Model: "GPT", Right: false})
	if rec.Code != http.StatusBadRequest || !strings.Contains(fmt.Sprint(out["error"]), "nothing to mark") {
		t.Errorf("a mark of an answer without a verdict = %d %s, want 400", rec.Code, rec.Body)
	}
}

// When the log cannot take the answers, they are still billed and shown, but they cannot be
// marked: the mark would be lost at the next start.
func TestLogFails(t *testing.T) {
	dir := t.TempDir()
	h := newBoard(t, dir, 5, &fakeModel{name: "A", answer: goodAnswer})
	if err := os.Mkdir(filepath.Join(dir, logName), 0o700); err != nil {
		t.Fatal(err)
	}
	rec, out := call(t, h, "POST", "/api/check", CheckRequest{Task: "posts", Image: sheet(t, 10, 10), Models: pick("A")})
	if rec.Code != http.StatusOK || !strings.Contains(fmt.Sprint(out["warning"]), "cannot be marked") {
		t.Fatalf("check = %d %s, want the answers with a warning", rec.Code, rec.Body)
	}
	if spent := out["summary"].(map[string]any)["spent_usd"].(float64); spent != 0.014 {
		t.Errorf("spent = %v, want the billed call counted", spent)
	}
	rec, _ = call(t, h, "POST", "/api/mark", MarkRequest{Attempt: out["attempt"].(string), Model: "A", Right: true})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("a mark of an answer not in the log = %d %s, want 400", rec.Code, rec.Body)
	}
}

// Only the ticked models are asked, each at its picked effort, and a pick the board does not offer
// is refused before any call.
func TestPicks(t *testing.T) {
	a := &fakeModel{name: "A", answer: goodAnswer}
	b := &fakeModel{name: "B", answer: goodAnswer}
	h := newBoard(t, t.TempDir(), 5, a, b)
	img := sheet(t, 10, 10)

	rec, out := call(t, h, "POST", "/api/check", CheckRequest{Task: "posts", Image: img, Models: []Pick{{Model: "A", Effort: "low"}}})
	if rec.Code != http.StatusOK {
		t.Fatalf("check = %d %s", rec.Code, rec.Body)
	}
	if got := a.requests(); len(got) != 1 || got[0].Effort != "low" {
		t.Errorf("A was asked %+v, want once at low", got)
	}
	if got := b.requests(); len(got) != 0 {
		t.Errorf("B was asked %d times, but it was not ticked", len(got))
	}
	results := out["results"].([]any)
	if len(results) != 1 || results[0].(map[string]any)["effort"] != "low" {
		t.Errorf("results = %v, want A's alone, at low", results)
	}
	attempt := out["attempt"].(string)
	rec, out = call(t, h, "POST", "/api/mark", MarkRequest{Attempt: attempt, Model: "B", Right: true})
	if rec.Code != http.StatusBadRequest || !strings.Contains(fmt.Sprint(out["error"]), "did not answer") {
		t.Errorf("a mark of a model that did not answer the try = %d %s", rec.Code, rec.Body)
	}

	// Without thinking, at a level where the model is offered so.
	rec, out = call(t, h, "POST", "/api/check", CheckRequest{Task: "posts", Image: img, Models: []Pick{{Model: "A", Effort: "low", NoThinking: true}}})
	if rec.Code != http.StatusOK {
		t.Fatalf("check without thinking = %d %s", rec.Code, rec.Body)
	}
	if got := a.requests(); len(got) != 2 || !got[1].NoThinking || got[1].Effort != "low" {
		t.Errorf("A was asked %+v, want without thinking at low", got)
	}
	if r := out["results"].([]any)[0].(map[string]any); r["no_thinking"] != true {
		t.Errorf("result = %v, want it marked as without thinking", r)
	}

	tests := []struct {
		name  string
		picks []Pick
		want  string
	}{
		{"no model ticked", nil, "tick a model"},
		{"a model not offered", []Pick{{Model: "Z", Effort: "high"}}, "no such model: Z"},
		{"a level not offered", []Pick{{Model: "A", Effort: "max"}}, `not offered at the effort "max"`},
		{"a model ticked twice", []Pick{{Model: "A", Effort: "low"}, {Model: "A", Effort: "high"}}, "ticked twice"},
		{"no thinking at a level not offered so", []Pick{{Model: "A", Effort: "high", NoThinking: true}}, `not offered without thinking at the effort "high"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, out := call(t, h, "POST", "/api/check", CheckRequest{Task: "posts", Image: img, Models: tt.picks})
			if rec.Code != http.StatusBadRequest || !strings.Contains(fmt.Sprint(out["error"]), tt.want) {
				t.Errorf("= %d %s, want 400 holding %q", rec.Code, rec.Body, tt.want)
			}
		})
	}
	if len(a.requests()) != 2 || len(b.requests()) != 0 {
		t.Error("a refused pick called a model")
	}

	// A model whose thinking does not turn off is not asked without it at any level.
	c := &fakeModel{name: "C", answer: goodAnswer}
	board, err := New([]Choice{{Model: c, Efforts: []string{"low", "high"}, Effort: "high"}}, t.TempDir(), 5)
	if err != nil {
		t.Fatal(err)
	}
	rec, out = call(t, board.Handler(boardAddr), "POST", "/api/check", CheckRequest{Task: "posts", Image: img, Models: []Pick{{Model: "C", Effort: "low", NoThinking: true}}})
	if rec.Code != http.StatusBadRequest || !strings.Contains(fmt.Sprint(out["error"]), "not offered without thinking") || len(c.requests()) != 0 {
		t.Errorf("no thinking on a model without it = %d %s", rec.Code, rec.Body)
	}
}

func TestBadRequests(t *testing.T) {
	h := newBoard(t, t.TempDir(), 5, &fakeModel{name: "A", answer: goodAnswer})
	tests := []struct {
		name string
		path string
		body any
		want string
	}{
		{"an unknown task", "/api/check", CheckRequest{Task: "nope", Image: sheet(t, 10, 10), Models: pick("A")}, "no such task"},
		{"not base64", "/api/check", CheckRequest{Task: "posts", Image: "%%%", Models: pick("A")}, "not base64"},
		{"not a PNG", "/api/check", CheckRequest{Task: "posts", Image: base64.StdEncoding.EncodeToString([]byte("GIF89a")), Models: pick("A")}, "not a PNG"},
		{"a picture too big", "/api/check", CheckRequest{Task: "posts", Image: sheet(t, 4001, 2), Models: pick("A")}, "larger than"},
		{"an unknown field", "/api/check", map[string]any{"task": "posts", "picture": "x"}, "unknown field"},
		{"a mark of no try", "/api/mark", MarkRequest{Attempt: "nope", Model: "A", Right: true}, "no such try"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, out := call(t, h, "POST", tt.path, tt.body)
			if rec.Code != http.StatusBadRequest || !strings.Contains(fmt.Sprint(out["error"]), tt.want) {
				t.Errorf("= %d %s, want 400 holding %q", rec.Code, rec.Body, tt.want)
			}
		})
	}
}

func TestSpendingCap(t *testing.T) {
	// One call of a fake model costs $0.014 and at worst $0.336, so the first check of one model
	// fits under $0.34 and the second does not.
	h := newBoard(t, t.TempDir(), 0.34, &fakeModel{name: "A", answer: goodAnswer})
	req := CheckRequest{Task: "chain-1", Image: sheet(t, 10, 10), Models: pick("A")}
	if rec, _ := call(t, h, "POST", "/api/check", req); rec.Code != http.StatusOK {
		t.Fatalf("the first check = %d, want it to go through", rec.Code)
	}
	rec, out := call(t, h, "POST", "/api/check", req)
	if rec.Code != http.StatusTooManyRequests || !strings.Contains(fmt.Sprint(out["error"]), "can cost up to $0.34") {
		t.Errorf("a check over the cap = %d %s, want 429 naming -max-usd", rec.Code, rec.Body)
	}

	// The cap counts the ticked models alone: two at once could cost up to $0.67, over $0.5, and
	// one goes through.
	h = newBoard(t, t.TempDir(), 0.5, &fakeModel{name: "A", answer: goodAnswer}, &fakeModel{name: "B", answer: goodAnswer})
	req.Models = pick("A", "B")
	if rec, out := call(t, h, "POST", "/api/check", req); rec.Code != http.StatusTooManyRequests || !strings.Contains(fmt.Sprint(out["error"]), "can cost up to $0.67") {
		t.Errorf("two models over the cap = %d %s, want 429", rec.Code, rec.Body)
	}
	req.Models = pick("B")
	if rec, _ := call(t, h, "POST", "/api/check", req); rec.Code != http.StatusOK {
		t.Errorf("one model under the cap = %d %s, want it to go through", rec.Code, rec.Body)
	}
}

// The summary has a line for each model at each effort, with its thinking and without, and for
// each form of answer, with the means of its answers. An answer logged before the page offered
// efforts counts as high and has no tokens; one logged before the short answer is of the earlier
// form, and the tokens it spent thinking are not known. The means of tokens and of the first token
// leave out the answers without a verdict, whose count may be cut short. A model this run does not
// offer comes last.
func TestSummaryByPairs(t *testing.T) {
	dir := t.TempDir()
	log := strings.Join([]string{
		`{"kind":"check","time":"2026-10-22T10:00:00Z","attempt":"t1","task":"posts","model":"A","result":{"model":"A","verdict":{"verdict":"correct"},"latency_ms":4000,"cost_usd":0.01}}`,
		`{"kind":"check","time":"2026-10-22T10:00:00Z","attempt":"t1","task":"posts","model":"GPT","result":{"model":"GPT","verdict":{"verdict":"error"},"latency_ms":9000,"cost_usd":0.05}}`,
		`{"kind":"check","time":"2026-10-23T10:00:00Z","attempt":"t2","task":"posts","model":"A","result":{"model":"A","effort":"high","verdict":{"lines":["x"],"answer":"x","verdict":"error","first_error_line":1,"comment":"c"},"input_tokens":1000,"output_tokens":500,"latency_ms":6000,"cost_usd":0.03}}`,
		`{"kind":"check","time":"2026-10-23T10:01:00Z","attempt":"t3","task":"posts","model":"A","result":{"model":"A","effort":"low","error":"the model gave no answer","input_tokens":1000,"output_tokens":100,"latency_ms":1000,"cost_usd":0.002}}`,
		`{"kind":"check","time":"2026-10-25T10:00:00Z","attempt":"t4","task":"posts","model":"A","result":{"model":"A","effort":"high","form":"short","verdict":{"verdict":"so_far_correct","mark":null},"input_tokens":1300,"output_tokens":200,"thinking_tokens":150,"first_token_ms":2000,"latency_ms":2500,"cost_usd":0.009}}`,
		`{"kind":"check","time":"2026-10-25T10:01:00Z","attempt":"t5","task":"posts","model":"A","result":{"model":"A","effort":"high","form":"short","error":"the call failed: boom","input_tokens":0,"output_tokens":0,"thinking_tokens":0,"latency_ms":500,"cost_usd":0}}`,
		`{"kind":"check","time":"2026-10-25T10:02:00Z","attempt":"t6","task":"posts","model":"A","result":{"model":"A","effort":"low","no_thinking":true,"form":"short","verdict":{"verdict":"error","mark":{"shape":"box","x1":1,"y1":2,"x2":3,"y2":4}},"place":{"shape":"box","x1":1,"y1":2,"x2":3,"y2":4},"input_tokens":1300,"output_tokens":40,"thinking_tokens":0,"first_token_ms":900,"latency_ms":1200,"cost_usd":0.0002}}`,
		`{"kind":"mark","time":"2026-10-22T10:01:00Z","attempt":"t1","model":"A","right":true}`,
		`{"kind":"mark","time":"2026-10-23T10:02:00Z","attempt":"t2","model":"A","right":false}`,
		`{"kind":"mark","time":"2026-10-23T10:02:00Z","attempt":"t3","model":"A","right":false}`,
		`{"kind":"mark","time":"2026-10-25T10:03:00Z","attempt":"t4","model":"A","right":true}`,
		`{"kind":"mark","time":"2026-10-25T10:03:00Z","attempt":"t6","model":"A","right":false}`,
		`{"kind":"mark","time":"2026-10-23T10:03:00Z","attempt":"t9","model":"A","right":true}`,
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, logName), []byte(log), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := New(offer(&fakeModel{name: "A"}), dir, 5)
	if err != nil {
		t.Fatal(err)
	}
	// model · effort · without thinking · form: answers, no answer, marked, right, mean price, time,
	// first token, tokens out and thinking tokens.
	want := []string{
		"A low false earlier: 1 1 1 0 $0.002000 1000 ms, first — out — thinking —",
		"A low true short: 1 0 1 0 $0.000200 1200 ms, first 900 out 40 thinking 0",
		"A high false short: 2 1 1 1 $0.004500 1500 ms, first 2000 out 200 thinking 150",
		"A high false earlier: 2 0 2 1 $0.020000 5000 ms, first — out 500 thinking —",
		"GPT high false earlier: 1 0 0 0 $0.050000 9000 ms, first — out — thinking —",
	}
	var got []string
	for _, l := range s.summary().Pairs {
		got = append(got, fmt.Sprintf("%s %s %v %s: %d %d %d %d $%.6f %d ms, first %s out %s thinking %s", l.Model, l.Effort, l.NoThinking, l.Form,
			l.Answers, l.NoAnswer, l.Marked, l.Right, l.MeanCostUSD, l.MeanLatencyMs, shown(l.MeanFirstTokenMs), shown(l.MeanOutputTokens), shown(l.MeanThinkingTokens)))
	}
	if !slices.Equal(got, want) {
		t.Errorf("summary:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// shown is a mean as the test names it: a dash where the log knows none.
func shown(mean *int64) string {
	if mean == nil {
		return "—"
	}
	return fmt.Sprint(*mean)
}

func TestAnswerOffTheSchema(t *testing.T) {
	pic := picture{png: []byte("png"), width: 1000, height: 600}
	for _, answer := range []string{"not json", `{"verdict":"maybe","mark":null}`} {
		m := &fakeModel{name: "A", answer: answer}
		if res := ask(context.Background(), question{model: m, effort: "high"}, &Tasks[0], pic); res.Verdict != nil || !strings.Contains(res.Error, "off the schema") {
			t.Errorf("the answer %s = %+v, want it off the schema", answer, res)
		}
	}
	// A call cut off by the clock or by a closed page before any bill came, and an answer that broke
	// off before its tokens were counted, are counted at the most they could cost.
	for name, m := range map[string]*fakeModel{
		"cut off by the clock":           {name: "A", err: context.DeadlineExceeded},
		"cut off by a closed page":       {name: "A", err: context.Canceled},
		"broken off before the count":    {name: "A", err: errors.New("stream broke"), billed: 1, partial: true},
		"cut off after the stream began": {name: "A", err: context.DeadlineExceeded, billed: 1300, partial: true},
	} {
		res := ask(context.Background(), question{model: m, effort: "high"}, &Tasks[0], pic)
		if !strings.Contains(res.Error, "counted at the most it could cost") || res.CostUSD != worstCost(m) {
			t.Errorf("%s = %+v, want it counted at $%.3f", name, res, worstCost(m))
		}
	}
	m := &fakeModel{name: "A", err: context.DeadlineExceeded}
	if res := ask(context.Background(), question{model: m, effort: "high"}, &Tasks[0], pic); !strings.HasPrefix(res.Error, "the model did not answer in time") {
		t.Errorf("a timeout = %+v", res)
	}
	// A call that failed before the stream began bills nothing.
	m = &fakeModel{name: "A", err: errors.New("boom")}
	if res := ask(context.Background(), question{model: m, effort: "high"}, &Tasks[0], pic); !strings.Contains(res.Error, "boom") || res.CostUSD != 0 {
		t.Errorf("a failed call = %+v, want its error and no cost", res)
	}
}

// ask carries the answer and what the call took into the result, with the place the board draws.
func TestAskCarriesTheAnswer(t *testing.T) {
	m := &fakeModel{name: "A", answer: `{"verdict":"error","mark":{"shape":"circle","x1":-5,"y1":20,"x2":40,"y2":50}}`}
	res := ask(context.Background(), question{model: m, effort: "low", noThinking: true}, &Tasks[0], picture{png: []byte("png"), width: 100, height: 60})
	if res.Verdict == nil || res.Verdict.Verdict != "error" || res.Verdict.Mark == nil || res.Verdict.Mark.X1 != -5 {
		t.Errorf("verdict = %+v, want the model's own", res.Verdict)
	}
	if res.Place == nil || *res.Place != (Mark{Shape: "circle", X1: 0, Y1: 20, X2: 40, Y2: 50}) {
		t.Errorf("place = %+v, want the model's box held inside the picture", res.Place)
	}
	if res.Form != formShort || !res.NoThinking || res.Effort != "low" || res.ModelID != "A-id" || res.InputTokens != 1000 || res.OutputTokens != 500 ||
		res.ThinkingTokens != 300 || res.FirstTokenMs != 1500 || res.Retries != 1 || res.CostUSD != 0.014 {
		t.Errorf("result = %+v", res)
	}
	asked := m.requests()[0]
	if asked.Instructions != instructions || !asked.NoThinking || asked.Effort != "low" || asked.MaxOutputTokens != maxOutputTokens ||
		!strings.HasPrefix(asked.Text, "The picture is 100 pixels wide and 60 pixels high.\nTask: ") {
		t.Errorf("the model was asked %+v", asked)
	}
}

// The schema is valid JSON with the board's verdicts and shapes, in the order the model writes
// them: the verdict, then the mark, and the box from its top-left corner.
func TestSchema(t *testing.T) {
	raw, ok := schema["properties"].(json.RawMessage)
	if !ok || !json.Valid(raw) {
		t.Fatalf("the schema's properties are not valid JSON:\n%s", raw)
	}
	var props struct {
		Verdict struct {
			Enum []string `json:"enum"`
		} `json:"verdict"`
		Mark struct {
			AnyOf []struct {
				Properties struct {
					Shape struct {
						Enum []string `json:"enum"`
					} `json:"shape"`
				} `json:"properties"`
			} `json:"anyOf"`
		} `json:"mark"`
	}
	if err := json.Unmarshal(raw, &props); err != nil || len(props.Mark.AnyOf) != 2 {
		t.Fatalf("the schema's properties = %v, %s", err, raw)
	}
	if !slices.Equal(props.Verdict.Enum, verdicts) || !slices.Equal(props.Mark.AnyOf[1].Properties.Shape.Enum, shapes) {
		t.Errorf("the schema's enums = %v and %v, want %v and %v", props.Verdict.Enum, props.Mark.AnyOf[1].Properties.Shape.Enum, verdicts, shapes)
	}
	compact := strings.Join(strings.Fields(string(raw)), "")
	order := []string{`"verdict":`, `"mark":`, `"shape":`, `"x1":`, `"y1":`, `"x2":`, `"y2":`}
	for i := 1; i < len(order); i++ {
		if strings.Index(compact, order[i-1]) > strings.Index(compact, order[i]) {
			t.Errorf("%s comes after %s in the schema", order[i-1], order[i])
		}
	}
}

// read takes the answer apart: the verdict stands, and the place is the model's box, its corners
// in either order, held inside the picture, for a mistake alone. A box wholly outside the picture,
// or with no width, or with no height unless underlined, is no place.
func TestRead(t *testing.T) {
	box := func(shape string, x1, y1, x2, y2 int) string {
		return fmt.Sprintf(`{"verdict":"error","mark":{"shape":%q,"x1":%d,"y1":%d,"x2":%d,"y2":%d}}`, shape, x1, y1, x2, y2)
	}
	tests := []struct {
		name    string
		answer  string
		verdict string
		place   *Mark
	}{
		{"inside the picture", box("underline", 10, 20, 90, 40), "error", &Mark{"underline", 10, 20, 90, 40}},
		{"slightly outside", box("box", -4, 50, 104, 66), "error", &Mark{"box", 0, 50, 100, 60}},
		{"wholly outside", box("circle", 120, 10, 150, 20), "error", nil},
		{"an underline below the picture", box("underline", 10, 80, 90, 80), "error", nil},
		{"the corners the other way round", box("box", 90, 40, 10, 20), "error", &Mark{"box", 10, 20, 90, 40}},
		{"a flat underline", box("underline", 10, 30, 90, 30), "error", &Mark{"underline", 10, 30, 90, 30}},
		{"a flat box", box("box", 10, 30, 90, 30), "error", nil},
		{"no width", box("circle", 50, 10, 50, 40), "error", nil},
		{"a shape of its own", box("arrow", 10, 20, 90, 40), "error", nil},
		{"an error without a place", `{"verdict":"error","mark":null}`, "error", nil},
		{"a place without an error", `{"verdict":"so_far_correct","mark":{"shape":"box","x1":1,"y1":1,"x2":5,"y2":5}}`, "so_far_correct", nil},
		{"correct", `{"verdict":"correct","mark":null}`, "correct", nil},
		{"unreadable", `{"verdict":"unreadable","mark":null}`, "unreadable", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, place, err := read(tt.answer, 100, 60)
			if err != nil {
				t.Fatal(err)
			}
			if v.Verdict != tt.verdict || !reflect.DeepEqual(place, tt.place) {
				t.Errorf("= %q, %+v; want %q, %+v", v.Verdict, place, tt.verdict, tt.place)
			}
		})
	}
}

func TestGuards(t *testing.T) {
	dir := t.TempDir()
	b, err := New(offer(&fakeModel{name: "A", answer: goodAnswer}), dir, 5)
	if err != nil {
		t.Fatal(err)
	}
	h := b.Handler(boardAddr)
	body := func() *bytes.Reader {
		data, _ := json.Marshal(CheckRequest{Task: "posts", Image: sheet(t, 10, 10), Models: pick("A")})
		return bytes.NewReader(data)
	}
	send := func(host, origin, contentType string, r *bytes.Reader) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/api/check", r)
		req.Host = host
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		req.Header.Set("Content-Type", contentType)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	tests := []struct {
		name        string
		host        string
		origin      string
		contentType string
		want        int
	}{
		{"the board's own page", boardAddr, "http://" + boardAddr, "application/json", http.StatusOK},
		{"another loopback name", "localhost:8080", "http://localhost:8080", "application/json", http.StatusOK},
		{"a port VS Code forwarded", "localhost:36615", "http://localhost:36615", "application/json", http.StatusOK},
		{"the IPv6 loopback", "[::1]:8080", "http://[::1]:8080", "application/json", http.StatusOK},
		{"a rebound domain", "evil.example:8080", "", "application/json", http.StatusForbidden},
		{"another site's page", boardAddr, "https://evil.example", "application/json", http.StatusForbidden},
		{"a page on another local port", "localhost:36615", "http://localhost:3000", "application/json", http.StatusForbidden},
		{"a form without a preflight", boardAddr, "", "text/plain", http.StatusUnsupportedMediaType},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if rec := send(tt.host, tt.origin, tt.contentType, body()); rec.Code != tt.want {
				t.Errorf("= %d %s, want %d", rec.Code, rec.Body, tt.want)
			}
		})
	}
	t.Run("a request too large", func(t *testing.T) {
		huge := bytes.NewReader(append([]byte(`{"task":"posts","image":"`), bytes.Repeat([]byte("A"), maxBody)...))
		if rec := send(boardAddr, "", "application/json", huge); rec.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("= %d %s, want 413", rec.Code, rec.Body)
		}
	})
	t.Run("a second check while one runs", func(t *testing.T) {
		b.checking.Lock()
		defer b.checking.Unlock()
		if rec := send(boardAddr, "", "application/json", body()); rec.Code != http.StatusConflict {
			t.Errorf("= %d %s, want 409", rec.Code, rec.Body)
		}
	})
}

func TestLogCutShort(t *testing.T) {
	good := `{"kind":"check","time":"2026-10-22T00:00:00Z","attempt":"a1","model":"A","result":{"model":"A","verdict":{"verdict":"correct"},"latency_ms":1,"cost_usd":0.01}}` + "\n" +
		`{"kind":"mark","time":"2026-10-22T00:00:00Z","attempt":"a1","model":"A","right":true}`
	write := func(content string) string {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, logName), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	choices := offer(&fakeModel{name: "A"})
	s, err := New(choices, write(good+"\n"+`{"kind":"che`), 5)
	if err != nil {
		t.Fatalf("a last line cut short: %v, want it skipped", err)
	}
	if sum := s.summary(); len(sum.Pairs) != 1 || sum.Pairs[0].Right != 1 {
		t.Errorf("summary = %+v, want the mark before the cut line", sum)
	}
	if _, err := New(choices, write(`{"kind":"che`+"\n"+good+"\n"), 5); err == nil {
		t.Error("a broken line in the middle of the log was taken")
	}
}
