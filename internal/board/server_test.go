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
	"strings"
	"sync"
	"testing"

	"github.com/altnovatrail/tutor/provider"
)

// fakeModel answers every request the same way and remembers what it was asked.
type fakeModel struct {
	name   string
	answer string
	err    error
	billed int64 // tokens a failed call still bills
	mu     sync.Mutex
	asked  []provider.Request
}

func (f *fakeModel) Name() string                   { return f.name }
func (f *fakeModel) Price() (input, output float64) { return 4, 20 }
func (f *fakeModel) Ask(_ context.Context, r provider.Request) (provider.Response, error) {
	f.mu.Lock()
	f.asked = append(f.asked, r)
	f.mu.Unlock()
	if f.err != nil {
		return provider.Response{InputTokens: f.billed, OutputTokens: f.billed}, f.err
	}
	return provider.Response{JSON: f.answer, Model: f.name + "-id", InputTokens: 1000, OutputTokens: 500}, nil
}

func (f *fakeModel) requests() []provider.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]provider.Request(nil), f.asked...)
}

const goodAnswer = `{"lines":["3x + 1 = 7","3x = 6","x = 2"],"answer":"x = 2","verdict":"correct","first_error_line":null,"comment":"Прочитал три строки, ответ верный."}`

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

// offer offers every model at low and high effort, starting on high and ticked.
func offer(models ...provider.Model) []Choice {
	choices := make([]Choice, len(models))
	for i, m := range models {
		choices[i] = Choice{Model: m, Efforts: []string{"low", "high"}, Effort: "high", Ticked: true}
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

// line is the summary's line of a model at an effort, or nil.
func line(t *testing.T, summary map[string]any, model, effort string) map[string]any {
	t.Helper()
	for _, p := range summary["pairs"].([]any) {
		if l := p.(map[string]any); l["model"] == model && l["effort"] == effort {
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
	if o := offers[0]; o.Model != "A" || o.Input != 4 || o.Output != 20 || strings.Join(o.Efforts, ",") != "low,high" || o.Effort != "high" || !o.Ticked {
		t.Errorf("the offered model = %+v", o)
	}
}

func TestNewRefusesBadChoices(t *testing.T) {
	a := &fakeModel{name: "A"}
	tests := map[string][]Choice{
		"no model":                  nil,
		"a model offered twice":     append(offer(a), offer(a)...),
		"a level it is not offered": {{Model: a, Efforts: []string{"low"}, Effort: "high"}},
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
	if !strings.Contains(asked.Text, "Correct answer: x = 2") || len(asked.PNG) == 0 || asked.Schema == nil || asked.Effort != "high" {
		t.Errorf("the model was asked %+v, want the task's answer, the picture, the schema and the effort", asked)
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

	tests := []struct {
		name  string
		picks []Pick
		want  string
	}{
		{"no model ticked", nil, "tick a model"},
		{"a model not offered", []Pick{{Model: "Z", Effort: "high"}}, "no such model: Z"},
		{"a level not offered", []Pick{{Model: "A", Effort: "max"}}, `not offered at the effort "max"`},
		{"a model ticked twice", []Pick{{Model: "A", Effort: "low"}, {Model: "A", Effort: "high"}}, "ticked twice"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, out := call(t, h, "POST", "/api/check", CheckRequest{Task: "posts", Image: img, Models: tt.picks})
			if rec.Code != http.StatusBadRequest || !strings.Contains(fmt.Sprint(out["error"]), tt.want) {
				t.Errorf("= %d %s, want 400 holding %q", rec.Code, rec.Body, tt.want)
			}
		})
	}
	if len(a.requests()) != 1 || len(b.requests()) != 0 {
		t.Error("a refused pick called a model")
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

// The summary has a line for each model at each effort, with the means of its answers; an answer
// logged before the page offered efforts counts as high, and a model this run does not offer comes
// last.
func TestSummaryByPairs(t *testing.T) {
	dir := t.TempDir()
	log := strings.Join([]string{
		`{"kind":"check","time":"2026-10-22T10:00:00Z","attempt":"t1","task":"posts","model":"A","result":{"model":"A","verdict":{"verdict":"correct"},"latency_ms":4000,"cost_usd":0.01}}`,
		`{"kind":"check","time":"2026-10-22T10:00:00Z","attempt":"t1","task":"posts","model":"GPT","result":{"model":"GPT","verdict":{"verdict":"error"},"latency_ms":9000,"cost_usd":0.05}}`,
		`{"kind":"check","time":"2026-10-23T10:00:00Z","attempt":"t2","task":"posts","model":"A","result":{"model":"A","effort":"high","verdict":{"verdict":"error"},"input_tokens":1000,"output_tokens":500,"latency_ms":6000,"cost_usd":0.03}}`,
		`{"kind":"check","time":"2026-10-23T10:01:00Z","attempt":"t3","task":"posts","model":"A","result":{"model":"A","effort":"low","error":"the model gave no answer","input_tokens":1000,"output_tokens":100,"latency_ms":1000,"cost_usd":0.002}}`,
		`{"kind":"mark","time":"2026-10-22T10:01:00Z","attempt":"t1","model":"A","right":true}`,
		`{"kind":"mark","time":"2026-10-23T10:02:00Z","attempt":"t2","model":"A","right":false}`,
		`{"kind":"mark","time":"2026-10-23T10:02:00Z","attempt":"t3","model":"A","right":false}`,
		`{"kind":"mark","time":"2026-10-23T10:03:00Z","attempt":"t9","model":"A","right":true}`,
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, logName), []byte(log), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := New(offer(&fakeModel{name: "A"}), dir, 5)
	if err != nil {
		t.Fatal(err)
	}
	want := []PairSummary{
		{Model: "A", Effort: "low", Answers: 1, NoAnswer: 1, Marked: 1, Right: 0, MeanCostUSD: 0.002, MeanLatencyMs: 1000},
		{Model: "A", Effort: "high", Answers: 2, Marked: 2, Right: 1, MeanCostUSD: 0.02, MeanLatencyMs: 5000},
		{Model: "GPT", Effort: "high", Answers: 1, MeanCostUSD: 0.05, MeanLatencyMs: 9000},
	}
	got := s.summary().Pairs
	if len(got) != len(want) {
		t.Fatalf("summary = %+v, want %+v", got, want)
	}
	for i := range want {
		g := got[i]
		if g.Model != want[i].Model || g.Effort != want[i].Effort || g.Answers != want[i].Answers || g.NoAnswer != want[i].NoAnswer ||
			g.Marked != want[i].Marked || g.Right != want[i].Right || g.MeanLatencyMs != want[i].MeanLatencyMs ||
			fmt.Sprintf("%.6f", g.MeanCostUSD) != fmt.Sprintf("%.6f", want[i].MeanCostUSD) {
			t.Errorf("line %d = %+v, want %+v", i, g, want[i])
		}
	}
}

func TestAnswerOffTheSchema(t *testing.T) {
	m := &fakeModel{name: "A", answer: "not json"}
	res := ask(context.Background(), m, "high", &Tasks[0], []byte("png"))
	if res.Verdict != nil || !strings.Contains(res.Error, "off the schema") {
		t.Errorf("an answer off the schema = %+v", res)
	}
	// A call cut off by the clock or by a closed page brings back no bill, so it counts at the most
	// it could cost.
	for _, err := range []error{context.DeadlineExceeded, context.Canceled} {
		m = &fakeModel{name: "A", err: err}
		res := ask(context.Background(), m, "high", &Tasks[0], nil)
		if !strings.Contains(res.Error, "counted at the most it could cost") || res.CostUSD != worstCost(m) {
			t.Errorf("a call cut off by %v = %+v, want it counted at $%.3f", err, res, worstCost(m))
		}
	}
	m = &fakeModel{name: "A", err: context.DeadlineExceeded}
	if res := ask(context.Background(), m, "high", &Tasks[0], nil); !strings.HasPrefix(res.Error, "the model did not answer in time") {
		t.Errorf("a timeout = %+v", res)
	}
	m = &fakeModel{name: "A", err: errors.New("boom")}
	if res := ask(context.Background(), m, "high", &Tasks[0], nil); !strings.Contains(res.Error, "boom") {
		t.Errorf("a failed call = %+v", res)
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
