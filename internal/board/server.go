// Package board is the check board: a local page where a person solves fixed tasks by hand, the
// models read the work and judge it, and the person marks whether each model was right. Nothing
// leaves the machine but the calls to the models; the pictures and the marks stay in a directory
// outside git (R19). Its record mode asks no model: the author writes the sheets of the sheet plan,
// and each is saved as a sample file in a directory outside git (R25).
package board

import (
	"bytes"
	"cmp"
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
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/altnovatrail/tutor/provider"
)

//go:embed page.html
var page embed.FS

// Limits of a try: the size of the request and of the picture, and how long the models may take —
// long enough that at the top effort levels the output limit, not the clock, ends a long answer.
const (
	maxBody    = 8 << 20
	maxSide    = 4000
	askTimeout = 10 * time.Minute
	logName    = "board.jsonl"
)

// effortBefore is the effort of the answers logged before the page offered a choice of efforts:
// the board asked every model at high (R19).
const effortBefore = "high"

// Choice is a model the board offers: the effort levels the page offers it at, least first, the
// level the page starts it on, whether the page starts with it ticked, and the levels at which it
// may be asked to answer without thinking — none if its thinking cannot be turned off.
type Choice struct {
	Model      provider.Model
	Efforts    []string
	Effort     string
	Ticked     bool
	NoThinking []string
}

// Server serves the board.
type Server struct {
	choices []Choice
	index   map[string]int // a model's name: its place among the choices
	dir     string
	maxUSD  float64

	checking sync.Mutex // one try at a time: the spending cap holds, and tries do not mix
	logMu    sync.Mutex // one writer of the log at a time

	mu      sync.Mutex
	spent   float64
	answers map[string]map[string]answer // try, model: the answers the log knows, so that a mark names a real one
	marks   map[string]map[string]bool   // model, try: was the model right
}

// answer is what the summary needs of a model's answer to a try: its line, its price and time,
// whether it gave no verdict, and the tokens and the time of its first text, with whether they are
// known. They are for an answer with a verdict alone, since the count of a failed call may be cut
// short: the log has kept the tokens since the page offered a choice of efforts, those spent
// thinking and the time of the first text since the short answer.
type answer struct {
	pair           pair
	costUSD        float64
	latencyMs      int64
	failed         bool
	outputTokens   int64
	knowsOutput    bool
	thinkingTokens int64
	knowsThinking  bool
	firstTokenMs   int64
	knowsFirst     bool
}

// pair is a line of the summary: a model at one effort, with its thinking or without, and the form
// of its answers.
type pair struct {
	model, effort string
	noThinking    bool
	form          string
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

// New returns a board that offers the given models, keeps its log and pictures in dir, and does
// not start a try that could take the calls of this run past maxUSD. The answers and marks of
// earlier runs are read back from the log, so the summary goes on where it stopped; a last line
// cut short by a crash is skipped.
func New(choices []Choice, dir string, maxUSD float64) (*Server, error) {
	if err := checkChoices(choices); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	s := &Server{choices: choices, index: map[string]int{}, dir: dir, maxUSD: maxUSD, answers: map[string]map[string]answer{}, marks: map[string]map[string]bool{}}
	for i, c := range choices {
		s.index[c.Model.Name()] = i
	}
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
		case r.Kind == "check" && r.Result != nil:
			s.remember(r.Attempt, r.Model, *r.Result)
		case r.Kind == "mark" && r.Right != nil:
			s.mark(r.Model, r.Attempt, *r.Right)
		}
	}
	return s, nil
}

// checkChoices makes sure that every offered model has a name of its own, starts on a level it is
// offered at, and may be asked not to think only at levels it is offered at.
func checkChoices(choices []Choice) error {
	if len(choices) == 0 {
		return errors.New("no model to offer")
	}
	seen := map[string]bool{}
	for _, c := range choices {
		name := c.Model.Name()
		if seen[name] {
			return fmt.Errorf("%s is offered twice", name)
		}
		seen[name] = true
		if !slices.Contains(c.Efforts, c.Effort) {
			return fmt.Errorf("%s starts on the effort %q, which it is not offered at", name, c.Effort)
		}
		for _, level := range c.NoThinking {
			if !slices.Contains(c.Efforts, level) {
				return fmt.Errorf("%s may be asked not to think at the effort %q, which it is not offered at", name, level)
			}
		}
	}
	return nil
}

// remember keeps what the summary needs of an answer. An answer logged before the page offered a
// choice of efforts has none, and it was asked at high; nor does it have its tokens. An answer
// logged before the short one has no form: it is of the earlier form, and the tokens it spent
// thinking are not known.
func (s *Server) remember(attempt, model string, res Result) {
	if s.answers[attempt] == nil {
		s.answers[attempt] = map[string]answer{}
	}
	form := cmp.Or(res.Form, formEarlier)
	answered := res.Verdict != nil
	s.answers[attempt][model] = answer{
		pair:           pair{model: model, effort: cmp.Or(res.Effort, effortBefore), noThinking: res.NoThinking, form: form},
		costUSD:        res.CostUSD,
		latencyMs:      res.LatencyMs,
		failed:         !answered,
		outputTokens:   res.OutputTokens,
		knowsOutput:    answered && res.Effort != "",
		thinkingTokens: res.ThinkingTokens,
		knowsThinking:  answered && form == formShort,
		firstTokenMs:   res.FirstTokenMs,
		knowsFirst:     answered && res.FirstTokenMs > 0,
	}
}

func (s *Server) mark(model, attempt string, right bool) {
	if s.marks[model] == nil {
		s.marks[model] = map[string]bool{}
	}
	s.marks[model][attempt] = right
}

// Handler routes the board's requests, behind guard. addr is the address the board listens on,
// for the message that turns a request away.
func (s *Server) Handler(addr string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		data, _ := page.ReadFile("page.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
	})
	mux.HandleFunc("GET /api/tasks", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, http.StatusOK, Tasks) })
	mux.HandleFunc("GET /api/models", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, http.StatusOK, s.offers()) })
	mux.HandleFunc("GET /api/summary", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, http.StatusOK, s.summary()) })
	mux.HandleFunc("POST /api/check", s.check)
	mux.HandleFunc("POST /api/mark", s.markHandler)
	return guard(addr, mux)
}

// guard passes a request on to next, in either mode of the board, only if it names a loopback
// host, on any port — VS Code forwards the board's port to the computer the browser runs on, and
// picks another number when that one is taken there — and, if it has an origin, comes from the
// address it names: the board's own page. A page on another site open in the same browser can
// neither spend the keys or write a sheet nor, by rebinding its domain to 127.0.0.1, read what the
// board answers: its requests name its own domain or come from it. addr is the address the board
// listens on, for the message that turns a request away.
func guard(addr string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !loopback(r.Host) {
			writeError(w, http.StatusForbidden, "the board answers only at a local address, such as http://"+addr)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			if u, err := url.Parse(origin); err != nil || u.Host != r.Host {
				writeError(w, http.StatusForbidden, "a request from another site")
				return
			}
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

// loopback reports whether a request's host, with or without its port, names this machine:
// localhost or a loopback address.
func loopback(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return host == "localhost" || (ip != nil && ip.IsLoopback())
}

// Offer is a model as the page offers it: its name, its price per million input and output tokens
// in US dollars, the effort levels it may be asked at, least first, the level the page starts it
// on, whether the page starts with it ticked, and the levels at which it may be asked not to think.
type Offer struct {
	Model      string   `json:"model"`
	Input      float64  `json:"input_usd_per_mtok"`
	Output     float64  `json:"output_usd_per_mtok"`
	Efforts    []string `json:"efforts"`
	Effort     string   `json:"effort"`
	Ticked     bool     `json:"ticked"`
	NoThinking []string `json:"no_thinking_efforts"`
}

func (s *Server) offers() []Offer {
	offers := make([]Offer, len(s.choices))
	for i, c := range s.choices {
		in, out := c.Model.Price()
		offers[i] = Offer{Model: c.Model.Name(), Input: in, Output: out, Efforts: c.Efforts, Effort: c.Effort, Ticked: c.Ticked,
			NoThinking: append([]string{}, c.NoThinking...)}
	}
	return offers
}

// Summary is, for each model at each effort it was asked at, how its answers went, and what the
// calls of this run cost.
type Summary struct {
	Pairs    []PairSummary `json:"pairs"`
	SpentUSD float64       `json:"spent_usd"`
	MaxUSD   float64       `json:"max_usd"`
}

// PairSummary is one line of the summary: a model at one effort, with its thinking or without, and
// the form of its answers. It counts the model's answers and those that gave no verdict, how many
// the person marked and how many of those were right, and the mean price and time of an answer —
// failed ones included, since they are billed and waited for too — and the means of when the
// first text came and of the tokens out and of those spent thinking, over the answers that gave a
// verdict and whose log line knows them; a mean is null where no line does.
type PairSummary struct {
	Model              string  `json:"model"`
	Effort             string  `json:"effort"`
	NoThinking         bool    `json:"no_thinking"`
	Form               string  `json:"form"`
	Answers            int     `json:"answers"`
	NoAnswer           int     `json:"no_answer"`
	Marked             int     `json:"marked"`
	Right              int     `json:"right"`
	MeanCostUSD        float64 `json:"mean_cost_usd"`
	MeanLatencyMs      int64   `json:"mean_latency_ms"`
	MeanFirstTokenMs   *int64  `json:"mean_first_token_ms"`
	MeanOutputTokens   *int64  `json:"mean_output_tokens"`
	MeanThinkingTokens *int64  `json:"mean_thinking_tokens"`
}

// totals are the sums behind a line of the summary, each with the number of answers it adds up.
type totals struct {
	cost                                float64
	latency                             int64
	first, output, thinking             int64
	withFirst, withOutput, withThinking int64
}

func (s *Server) summary() Summary {
	s.mu.Lock()
	defer s.mu.Unlock()
	lines := map[pair]*PairSummary{}
	sums := map[pair]*totals{}
	for _, byModel := range s.answers {
		for _, a := range byModel {
			p := a.pair
			if lines[p] == nil {
				lines[p] = &PairSummary{Model: p.model, Effort: p.effort, NoThinking: p.noThinking, Form: p.form}
				sums[p] = &totals{}
			}
			line, t := lines[p], sums[p]
			line.Answers++
			if a.failed {
				line.NoAnswer++
			}
			t.cost += a.costUSD
			t.latency += a.latencyMs
			if a.knowsFirst {
				t.first += a.firstTokenMs
				t.withFirst++
			}
			if a.knowsOutput {
				t.output += a.outputTokens
				t.withOutput++
			}
			if a.knowsThinking {
				t.thinking += a.thinkingTokens
				t.withThinking++
			}
		}
	}
	for model, byTry := range s.marks {
		for attempt, right := range byTry {
			a, ok := s.answers[attempt][model]
			if !ok {
				continue
			}
			line := lines[a.pair]
			line.Marked++
			if right {
				line.Right++
			}
		}
	}
	sum := Summary{Pairs: []PairSummary{}, SpentUSD: s.spent, MaxUSD: s.maxUSD}
	for p, line := range lines {
		t := sums[p]
		line.MeanCostUSD = t.cost / float64(line.Answers)
		line.MeanLatencyMs = t.latency / int64(line.Answers)
		line.MeanFirstTokenMs = mean(t.first, t.withFirst)
		line.MeanOutputTokens = mean(t.output, t.withOutput)
		line.MeanThinkingTokens = mean(t.thinking, t.withThinking)
		sum.Pairs = append(sum.Pairs, *line)
	}
	slices.SortFunc(sum.Pairs, func(a, b PairSummary) int {
		return cmp.Or(
			cmp.Compare(s.place(a.Model), s.place(b.Model)),
			strings.Compare(a.Model, b.Model),
			cmp.Compare(s.level(a.Model, a.Effort), s.level(b.Model, b.Effort)),
			strings.Compare(a.Effort, b.Effort),
			later(a.NoThinking, b.NoThinking),
			later(a.Form == formEarlier, b.Form == formEarlier))
	})
	return sum
}

// mean is the mean of n values that add up to sum, or nil if there are none.
func mean(sum, n int64) *int64 {
	if n == 0 {
		return nil
	}
	m := sum / n
	return &m
}

// later orders a line that is so after one that is not: a model without its thinking after the
// same model with it, and the earlier form of answer after the short one.
func later(a, b bool) int {
	switch {
	case a == b:
		return 0
	case a:
		return 1
	default:
		return -1
	}
}

// place is where the page offers a model; a model it does not offer this run — one whose key is
// not set, say — comes after every offered one.
func (s *Server) place(model string) int {
	if i, ok := s.index[model]; ok {
		return i
	}
	return len(s.choices)
}

// level is where an effort stands among the levels the page offers a model at, least first; a
// level it is not offered at comes after them.
func (s *Server) level(model, effort string) int {
	i := s.place(model)
	if i == len(s.choices) {
		return 0
	}
	if j := slices.Index(s.choices[i].Efforts, effort); j >= 0 {
		return j
	}
	return len(s.choices[i].Efforts)
}

// CheckRequest is a try: the task, the picture of the sheet — a PNG as a data URL — and the models
// to ask, each at an effort.
type CheckRequest struct {
	Task   string `json:"task"`
	Image  string `json:"image"`
	Models []Pick `json:"models"`
}

// Pick is a model ticked on the page, the effort to ask it at, and whether to ask it not to think.
type Pick struct {
	Model      string `json:"model"`
	Effort     string `json:"effort"`
	NoThinking bool   `json:"no_thinking,omitempty"`
}

// CheckResponse is every asked model's answer to the try.
type CheckResponse struct {
	Attempt string   `json:"attempt"`
	Results []Result `json:"results"`
	Summary Summary  `json:"summary"`
	Warning string   `json:"warning,omitempty"`
}

// picked turns the page's picks into the questions to ask, each model at its picked effort, in the
// order the page offers them. A pick must name an offered model, once, at a level it is offered
// at, and without thinking only at a level it is offered so at: the page cannot make the board
// call a model or a level the board does not offer.
func (s *Server) picked(picks []Pick) ([]question, error) {
	if len(picks) == 0 {
		return nil, errors.New("tick a model to ask")
	}
	chosen := make([]question, 0, len(picks))
	ticked := map[string]bool{}
	for _, p := range picks {
		i, ok := s.index[p.Model]
		if !ok {
			return nil, fmt.Errorf("no such model: %s", p.Model)
		}
		if ticked[p.Model] {
			return nil, fmt.Errorf("%s is ticked twice", p.Model)
		}
		ticked[p.Model] = true
		c := s.choices[i]
		if !slices.Contains(c.Efforts, p.Effort) {
			return nil, fmt.Errorf("%s is not offered at the effort %q", p.Model, p.Effort)
		}
		if p.NoThinking && !slices.Contains(c.NoThinking, p.Effort) {
			return nil, fmt.Errorf("%s is not offered without thinking at the effort %q", p.Model, p.Effort)
		}
		chosen = append(chosen, question{model: c.Model, effort: p.Effort, noThinking: p.NoThinking})
	}
	slices.SortFunc(chosen, func(a, b question) int { return cmp.Compare(s.place(a.model.Name()), s.place(b.model.Name())) })
	return chosen, nil
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
	chosen, err := s.picked(req.Models)
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
	for _, c := range chosen {
		worst += worstCost(c.model)
	}
	s.mu.Lock()
	spent := s.spent
	s.mu.Unlock()
	if spent+worst > s.maxUSD {
		writeError(w, http.StatusTooManyRequests, fmt.Sprintf(
			"spent $%.2f of the $%.2f this run may spend, and this check can cost up to $%.2f: tick fewer or cheaper models, or restart the board with a higher -max-usd",
			spent, s.maxUSD, worst))
		return
	}

	attempt, err := s.savePicture(pic.png)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), askTimeout)
	defer cancel()
	results := make([]Result, len(chosen))
	var wg sync.WaitGroup
	for i, c := range chosen {
		wg.Go(func() { results[i] = ask(ctx, c, task, pic) })
	}
	wg.Wait()

	now := time.Now().UTC()
	records := make([]record, len(results))
	for i := range results {
		records[i] = record{Kind: "check", Time: now, Attempt: attempt, Task: task.ID, Model: results[i].Model, Result: &results[i]}
	}
	resp := CheckResponse{Attempt: attempt, Results: results}
	logErr := s.appendLog(records...)
	if logErr != nil {
		resp.Warning = "the answers are not in the log, so they cannot be marked: " + logErr.Error()
	}
	// The calls are billed whether or not the log took them; only answers in the log can be marked,
	// since a mark of any other would be lost at the next start.
	s.mu.Lock()
	for _, res := range results {
		s.spent += res.CostUSD
		if logErr == nil {
			s.remember(attempt, res.Model, res)
		}
	}
	s.mu.Unlock()
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
	return attempt, createNew(filepath.Join(s.dir, attempt+".png"), pic)
}

// createNew writes a file that must not exist yet, readable by its owner alone: what the board
// writes is the author's handwriting. A file it could not finish, it removes.
func createNew(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(path)
	}
	return err
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
	a, answered := s.answers[req.Attempt][req.Model]
	s.mu.Unlock()
	if !answered {
		writeError(w, http.StatusBadRequest, "no such try or model: the model did not answer this try")
		return
	}
	if a.failed {
		writeError(w, http.StatusBadRequest, "the model gave no verdict on this try, so there is nothing to mark")
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

// decodePNG takes a data URL or bare base64, checks that it is a PNG of a sensible size, and
// returns it with its size.
func decodePNG(image string) (picture, error) {
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(image, "data:image/png;base64,"))
	if err != nil {
		return picture{}, errors.New("the picture is not base64")
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return picture{}, errors.New("the picture is not a PNG")
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > maxSide || cfg.Height > maxSide {
		return picture{}, fmt.Errorf("the picture is %d×%d, larger than %d×%d", cfg.Width, cfg.Height, maxSide, maxSide)
	}
	return picture{png: data, width: cfg.Width, height: cfg.Height}, nil
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
