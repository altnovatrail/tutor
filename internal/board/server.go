// Package board is the check board: a local page where a person solves fixed tasks by hand, the
// models read the work and judge it, and the person marks whether each model was right. Nothing
// leaves the machine but the calls to the models; the pictures and the marks stay in a directory
// outside git (R19).
package board

import (
	"bytes"
	"context"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/altnovatrail/tutor/provider"
)

//go:embed page.html
var page embed.FS

// Limits of a try: the size of the request and of the picture, and how long the models may take.
const (
	maxBody    = 8 << 20
	maxSide    = 4000
	askTimeout = 4 * time.Minute
	logName    = "board.jsonl"
)

// Server serves the board.
type Server struct {
	models []provider.Model
	dir    string
	maxUSD float64

	checking sync.Mutex // one try at a time: the spending cap holds, and tries do not mix
	logMu    sync.Mutex // one writer of the log at a time

	mu       sync.Mutex
	spent    float64
	attempts map[string]bool            // the tries the log knows, so that a mark names a real one
	marks    map[string]map[string]bool // model, try: was the model right
}

// record is one line of the log: a model's answer to a try, or the person's mark of it.
type record struct {
	Kind    string    `json:"kind"` // "check" or "mark"
	Time    time.Time `json:"time"`
	Attempt string    `json:"attempt"`
	Task    string    `json:"task,omitempty"`
	Model   string    `json:"model"`
	Result  *Result   `json:"result,omitempty"`
	Right   *bool     `json:"right,omitempty"`
}

// New returns a board that asks the given models, keeps its log and pictures in dir, and does not
// start a try that could take the calls of this run past maxUSD. The tries and marks of earlier
// runs are read back from the log, so the summary goes on where it stopped; a last line cut short
// by a crash is skipped.
func New(models []provider.Model, dir string, maxUSD float64) (*Server, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	s := &Server{models: models, dir: dir, maxUSD: maxUSD, attempts: map[string]bool{}, marks: map[string]map[string]bool{}}
	data, err := os.ReadFile(filepath.Join(dir, logName))
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	lines := bytes.Split(bytes.TrimRight(data, "\n"), []byte("\n"))
	for i, line := range lines {
		var r record
		if err := json.Unmarshal(line, &r); err != nil {
			if i == len(lines)-1 {
				break // the write a crash cut short
			}
			return nil, fmt.Errorf("%s, line %d: %w", logName, i+1, err)
		}
		switch {
		case r.Kind == "check":
			s.attempts[r.Attempt] = true
		case r.Kind == "mark" && r.Right != nil:
			s.mark(r.Model, r.Attempt, *r.Right)
		}
	}
	return s, nil
}

func (s *Server) mark(model, attempt string, right bool) {
	if s.marks[model] == nil {
		s.marks[model] = map[string]bool{}
	}
	s.marks[model][attempt] = right
}

// Handler routes the board's requests. addr is the address the board listens on: a request must
// name it, or the same port on another loopback name, as its host — and, if it has an origin, come
// from it. A page on another site open in the same browser can neither spend the keys nor, by
// rebinding its domain to 127.0.0.1, read the answers.
func (s *Server) Handler(addr string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		data, _ := page.ReadFile("page.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
	})
	mux.HandleFunc("GET /api/tasks", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, http.StatusOK, Tasks) })
	mux.HandleFunc("GET /api/summary", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, http.StatusOK, s.summary()) })
	mux.HandleFunc("POST /api/check", s.check)
	mux.HandleFunc("POST /api/mark", s.markHandler)
	hosts := allowedHosts(addr)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !hosts[r.Host] {
			writeError(w, http.StatusForbidden, "the board answers only at its own address, http://"+addr)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			if u, err := url.Parse(origin); err != nil || !hosts[u.Host] {
				writeError(w, http.StatusForbidden, "a request from another site")
				return
			}
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		mux.ServeHTTP(w, r)
	})
}

// allowedHosts is the host part a request to the board may carry: its address, and the same port
// on every loopback name when it listens on one.
func allowedHosts(addr string) map[string]bool {
	hosts := map[string]bool{addr: true}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return hosts
	}
	if ip := net.ParseIP(host); host == "localhost" || (ip != nil && ip.IsLoopback()) {
		for _, name := range []string{"localhost", "127.0.0.1", "[::1]"} {
			hosts[name+":"+port] = true
		}
	}
	return hosts
}

// Summary is how often each model was right, by the person's marks, and what the calls of this
// run cost.
type Summary struct {
	Models   []ModelSummary `json:"models"`
	SpentUSD float64        `json:"spent_usd"`
	MaxUSD   float64        `json:"max_usd"`
}

// ModelSummary is one model's line of the summary.
type ModelSummary struct {
	Model  string `json:"model"`
	Marked int    `json:"marked"`
	Right  int    `json:"right"`
}

func (s *Server) summary() Summary {
	s.mu.Lock()
	defer s.mu.Unlock()
	sum := Summary{SpentUSD: s.spent, MaxUSD: s.maxUSD}
	for _, m := range s.models {
		line := ModelSummary{Model: m.Name()}
		for _, right := range s.marks[m.Name()] {
			line.Marked++
			if right {
				line.Right++
			}
		}
		sum.Models = append(sum.Models, line)
	}
	return sum
}

// CheckRequest is a try: the task and the picture of the sheet, a PNG as a data URL.
type CheckRequest struct {
	Task  string `json:"task"`
	Image string `json:"image"`
}

// CheckResponse is every model's answer to the try.
type CheckResponse struct {
	Attempt string   `json:"attempt"`
	Results []Result `json:"results"`
	Summary Summary  `json:"summary"`
	Warning string   `json:"warning,omitempty"`
}

func (s *Server) check(w http.ResponseWriter, r *http.Request) {
	var req CheckRequest
	if !readJSON(w, r, &req) {
		return
	}
	task := taskByID(req.Task)
	if task == nil {
		writeError(w, http.StatusBadRequest, "no such task: "+req.Task)
		return
	}
	pic, err := decodePNG(req.Image)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !s.checking.TryLock() {
		writeError(w, http.StatusConflict, "another check is running: wait for its answers")
		return
	}
	defer s.checking.Unlock()
	worst := 0.0
	for _, m := range s.models {
		worst += worstCost(m)
	}
	s.mu.Lock()
	spent := s.spent
	s.mu.Unlock()
	if spent+worst > s.maxUSD {
		writeError(w, http.StatusTooManyRequests, fmt.Sprintf(
			"spent $%.2f of the $%.2f this run may spend, and a check can cost up to $%.2f: restart the board with a higher -max-usd",
			spent, s.maxUSD, worst))
		return
	}

	attempt, err := s.savePicture(pic)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), askTimeout)
	defer cancel()
	results := make([]Result, len(s.models))
	var wg sync.WaitGroup
	for i, m := range s.models {
		wg.Go(func() { results[i] = ask(ctx, m, task, pic) })
	}
	wg.Wait()

	s.mu.Lock()
	s.attempts[attempt] = true
	for _, res := range results {
		s.spent += res.CostUSD
	}
	s.mu.Unlock()
	now := time.Now().UTC()
	records := make([]record, len(results))
	for i := range results {
		records[i] = record{Kind: "check", Time: now, Attempt: attempt, Task: task.ID, Model: results[i].Model, Result: &results[i]}
	}
	resp := CheckResponse{Attempt: attempt, Results: results}
	if err := s.appendLog(records...); err != nil {
		resp.Warning = "the answers are not in the log: " + err.Error()
	}
	resp.Summary = s.summary()
	writeJSON(w, http.StatusOK, resp)
}

// savePicture keeps the picture of a try under a new name and returns the try's ID: the time and
// a random tail, so that two tries never share one.
func (s *Server) savePicture(pic []byte) (string, error) {
	tail := make([]byte, 3)
	if _, err := rand.Read(tail); err != nil {
		return "", err
	}
	attempt := time.Now().UTC().Format("20060102-150405") + "-" + hex.EncodeToString(tail)
	f, err := os.OpenFile(filepath.Join(s.dir, attempt+".png"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", err
	}
	_, err = f.Write(pic)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return attempt, err
}

// MarkRequest is the person's mark: whether a model was right about a try.
type MarkRequest struct {
	Attempt string `json:"attempt"`
	Model   string `json:"model"`
	Right   bool   `json:"right"`
}

func (s *Server) markHandler(w http.ResponseWriter, r *http.Request) {
	var req MarkRequest
	if !readJSON(w, r, &req) {
		return
	}
	s.mu.Lock()
	known := s.attempts[req.Attempt]
	s.mu.Unlock()
	if !known || !s.hasModel(req.Model) {
		writeError(w, http.StatusBadRequest, "no such try or model")
		return
	}
	right := req.Right
	if err := s.appendLog(record{Kind: "mark", Time: time.Now().UTC(), Attempt: req.Attempt, Model: req.Model, Right: &right}); err != nil {
		writeError(w, http.StatusInternalServerError, "the mark is not in the log: "+err.Error())
		return
	}
	s.mu.Lock()
	s.mark(req.Model, req.Attempt, right)
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, s.summary())
}

func (s *Server) hasModel(name string) bool {
	for _, m := range s.models {
		if m.Name() == name {
			return true
		}
	}
	return false
}

// appendLog adds records to the log in one write.
func (s *Server) appendLog(records ...record) error {
	var buf bytes.Buffer
	for _, r := range records {
		data, err := json.Marshal(r)
		if err != nil {
			return err
		}
		buf.Write(append(data, '\n'))
	}
	s.logMu.Lock()
	defer s.logMu.Unlock()
	f, err := os.OpenFile(filepath.Join(s.dir, logName), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	_, err = f.Write(buf.Bytes())
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// decodePNG takes a data URL or bare base64 and checks that it is a PNG of a sensible size.
func decodePNG(image string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(image, "data:image/png;base64,"))
	if err != nil {
		return nil, errors.New("the picture is not base64")
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, errors.New("the picture is not a PNG")
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > maxSide || cfg.Height > maxSide {
		return nil, fmt.Errorf("the picture is %d×%d, larger than %d×%d", cfg.Width, cfg.Height, maxSide, maxSide)
	}
	return data, nil
}

// readJSON reads a request's JSON body. It takes only application/json, which a page on another
// site cannot send without asking first, and a body no larger than maxBody.
func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "the request must be application/json")
		return false
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("the request is larger than %d MiB", maxBody>>20))
			return false
		}
		writeError(w, http.StatusBadRequest, "the request cannot be read: "+err.Error())
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
