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

func newBoard(t *testing.T, dir string, maxUSD float64, models ...provider.Model) http.Handler {
	t.Helper()
	b, err := New(models, dir, maxUSD)
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

func TestPageAndTasks(t *testing.T) {
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
}

func TestCheckAndMark(t *testing.T) {
	dir := t.TempDir()
	good := &fakeModel{name: "Claude", answer: goodAnswer}
	broken := &fakeModel{name: "GPT", err: fmt.Errorf("%w: the model refused", provider.ErrNoAnswer), billed: 100}
	h := newBoard(t, dir, 5, good, broken)

	rec, out := call(t, h, "POST", "/api/check", CheckRequest{Task: "equation-1", Image: sheet(t, 100, 60)})
	if rec.Code != http.StatusOK {
		t.Fatalf("check = %d %s", rec.Code, rec.Body)
	}
	results := out["results"].([]any)
	first, second := results[0].(map[string]any), results[1].(map[string]any)
	if first["verdict"].(map[string]any)["verdict"] != "correct" || first["cost_usd"].(float64) != 0.014 {
		t.Errorf("the good model's result = %v", first)
	}
	if !strings.Contains(second["error"].(string), "gave no answer") || second["cost_usd"].(float64) != 0.0024 {
		t.Errorf("the broken model's result = %v, want its error in words", second)
	}
	asked := good.asked[0]
	if !strings.Contains(asked.Text, "Correct answer: x = 2") || len(asked.PNG) == 0 || asked.Schema == nil {
		t.Errorf("the model was asked %+v, want the task's answer, the picture and the schema", asked)
	}

	attempt := out["attempt"].(string)
	if _, err := os.Stat(filepath.Join(dir, attempt+".png")); err != nil {
		t.Errorf("the picture of the try is not kept: %v", err)
	}
	rec, out = call(t, h, "POST", "/api/mark", MarkRequest{Attempt: attempt, Model: "Claude", Right: true})
	if rec.Code != http.StatusOK {
		t.Fatalf("mark = %d %s", rec.Code, rec.Body)
	}
	claudeLine := out["models"].([]any)[0].(map[string]any)
	if claudeLine["marked"].(float64) != 1 || claudeLine["right"].(float64) != 1 {
		t.Errorf("summary after one mark = %v", out)
	}
	// Marking again changes the mark rather than adding one.
	_, out = call(t, h, "POST", "/api/mark", MarkRequest{Attempt: attempt, Model: "Claude", Right: false})
	claudeLine = out["models"].([]any)[0].(map[string]any)
	if claudeLine["marked"].(float64) != 1 || claudeLine["right"].(float64) != 0 {
		t.Errorf("summary after a changed mark = %v", out)
	}

	// A new run reads the marks back from the log.
	h = newBoard(t, dir, 5, good, broken)
	_, out = call(t, h, "GET", "/api/summary", nil)
	claudeLine = out["models"].([]any)[0].(map[string]any)
	if claudeLine["marked"].(float64) != 1 || claudeLine["right"].(float64) != 0 {
		t.Errorf("summary after a restart = %v", out)
	}
	rec, _ = call(t, h, "POST", "/api/mark", MarkRequest{Attempt: attempt, Model: "GPT", Right: false})
	if rec.Code != http.StatusOK {
		t.Errorf("marking a try of an earlier run = %d %s", rec.Code, rec.Body)
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
		{"an unknown task", "/api/check", CheckRequest{Task: "nope", Image: sheet(t, 10, 10)}, "no such task"},
		{"not base64", "/api/check", CheckRequest{Task: "posts", Image: "%%%"}, "not base64"},
		{"not a PNG", "/api/check", CheckRequest{Task: "posts", Image: base64.StdEncoding.EncodeToString([]byte("GIF89a"))}, "not a PNG"},
		{"a picture too big", "/api/check", CheckRequest{Task: "posts", Image: sheet(t, 4001, 2)}, "larger than"},
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
	// One call of the fake model costs $0.014 and at worst $0.336, so the first check fits under
	// $0.34 and the second does not.
	h := newBoard(t, t.TempDir(), 0.34, &fakeModel{name: "A", answer: goodAnswer})
	req := CheckRequest{Task: "chain-1", Image: sheet(t, 10, 10)}
	if rec, _ := call(t, h, "POST", "/api/check", req); rec.Code != http.StatusOK {
		t.Fatalf("the first check = %d, want it to go through", rec.Code)
	}
	rec, out := call(t, h, "POST", "/api/check", req)
	if rec.Code != http.StatusTooManyRequests || !strings.Contains(fmt.Sprint(out["error"]), "can cost up to $0.34") {
		t.Errorf("a check over the cap = %d %s, want 429 naming -max-usd", rec.Code, rec.Body)
	}
}

func TestAnswerOffTheSchema(t *testing.T) {
	m := &fakeModel{name: "A", answer: "not json"}
	res := ask(context.Background(), m, &Tasks[0], []byte("png"))
	if res.Verdict != nil || !strings.Contains(res.Error, "off the schema") {
		t.Errorf("an answer off the schema = %+v", res)
	}
	m = &fakeModel{name: "A", err: context.DeadlineExceeded}
	if res := ask(context.Background(), m, &Tasks[0], nil); res.Error != "the model did not answer in time" {
		t.Errorf("a timeout = %+v", res)
	}
	m = &fakeModel{name: "A", err: errors.New("boom")}
	if res := ask(context.Background(), m, &Tasks[0], nil); !strings.Contains(res.Error, "boom") {
		t.Errorf("a failed call = %+v", res)
	}
}

func TestGuards(t *testing.T) {
	dir := t.TempDir()
	b, err := New([]provider.Model{&fakeModel{name: "A", answer: goodAnswer}}, dir, 5)
	if err != nil {
		t.Fatal(err)
	}
	h := b.Handler(boardAddr)
	body := func() *bytes.Reader {
		data, _ := json.Marshal(CheckRequest{Task: "posts", Image: sheet(t, 10, 10)})
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
		{"a rebound domain", "evil.example:8080", "", "application/json", http.StatusForbidden},
		{"another site's page", boardAddr, "https://evil.example", "application/json", http.StatusForbidden},
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
	good := `{"kind":"mark","time":"2026-10-22T00:00:00Z","attempt":"a1","model":"A","right":true}`
	write := func(content string) string {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, logName), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	m := &fakeModel{name: "A"}
	s, err := New([]provider.Model{m}, write(good+"\n"+`{"kind":"che`), 5)
	if err != nil {
		t.Fatalf("a last line cut short: %v, want it skipped", err)
	}
	if sum := s.summary(); sum.Models[0].Right != 1 {
		t.Errorf("summary = %+v, want the mark before the cut line", sum)
	}
	if _, err := New([]provider.Model{m}, write(`{"kind":"che`+"\n"+good+"\n"), 5); err == nil {
		t.Error("a broken line in the middle of the log was taken")
	}
}
