// Package measure runs the measurement of the writing process (R24): every sheet of a part in every
// form it has, by every judge, repeated. Before the run it counts the calls and estimates what
// they cost; during it, it starts no call whose worst case could take the run past its cap; and it
// logs every answer, a JSON object a line, outside git (R05).
package measure

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/altnovatrail/tutor/internal/journal"
	"github.com/altnovatrail/tutor/internal/judge"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/provider"
	"github.com/altnovatrail/tutor/provider/claude"
)

const (
	// MaxOutputTokens is the most a call may write, thinking included: the same for every judge,
	// as on the board, so that they are compared under one limit (R19).
	MaxOutputTokens = 16000
	// inputBudget is a generous count of a call's input tokens, for its worst case: a request with
	// three pictures of the sheet, the instructions and the journal comes to about 4 000.
	inputBudget = 8000
	// callTimeout is how long one call may take, as on the board.
	callTimeout = 10 * time.Minute
)

// The input tokens a call is expected to take, for the estimate: about 1 850 for form A, about 400
// more with the journal and about 820 more for each frame (docs/architecture.md, 3.1, «Смета»).
const (
	inputA       = 1850
	inputJournal = 400
	inputFrame   = 820
)

// Judge is a configuration of the judge (R24): a Claude model at an effort level, thinking or not,
// and the output tokens a call is expected to take, for the estimate (architecture 3.1).
type Judge struct {
	Name       string
	Model      claude.Model
	Effort     string
	NoThinking bool
	Output     int
}

// Judges are the three judges of the measurement: Opus 5.5 at high, Haiku 5.5 at low, and Haiku
// 5.5 at low without thinking (R24). A call is expected to write about 50 tokens, and Haiku with
// thinking about 300 (architecture 3.1).
func Judges() []Judge {
	model := func(id string) claude.Model {
		i := slices.IndexFunc(claude.Models, func(m claude.Model) bool { return m.ID == id })
		return claude.Models[i]
	}
	return []Judge{
		{Name: "opus-high", Model: model("claude-opus-5-5"), Effort: "high", Output: 50},
		{Name: "haiku-low", Model: model("claude-haiku-5-5"), Effort: "low", Output: 300},
		{Name: "haiku-low-no-thinking", Model: model("claude-haiku-5-5"), Effort: "low", NoThinking: true, Output: 50},
	}
}

// cost is what a call of the judge costs for its tokens, in US dollars.
func (j Judge) cost(in, out int64) float64 {
	return (float64(in)*j.Model.Input + float64(out)*j.Model.Output) / 1e6
}

// worst is the most a call of the judge can cost.
func (j Judge) worst() float64 { return j.cost(inputBudget, MaxOutputTokens) }

// expected is the input tokens a call of a form is expected to take, with the frames it shows.
func expected(f judge.Form, frames int) int64 {
	n := inputA
	if f.Journal() {
		n += inputJournal
	}
	if f.Frames() {
		n += inputFrame * frames
	}
	return int64(n)
}

// Sheet is a sheet of the part: its number in the plan and its file, its sample, which passes its
// check, and its task.
type Sheet struct {
	Number int
	File   string
	Sample *sample.Sample
	Task   *sample.Task
}

// Call is one call of a run: a sheet by its number, a form, a judge by its name and the repeat,
// from 1.
type Call struct {
	Sheet  int        `json:"sheet"`
	Form   judge.Form `json:"form"`
	Judge  string     `json:"judge"`
	Repeat int        `json:"repeat"`
}

// Question is what a call asks: the judge, the form, the sheet's sample and the request.
type Question struct {
	Judge   Judge
	Form    judge.Form
	Sample  *sample.Sample
	Request provider.Request
}

// Asker asks a question: of the judge's model, or of Stub, which answers by the markup.
type Asker func(ctx context.Context, q Question) (provider.Response, error)

// Models asks the judges' models with the Anthropic key.
func Models(key string) Asker {
	clients := map[string]*claude.Client{}
	return func(ctx context.Context, q Question) (provider.Response, error) {
		c, ok := clients[q.Judge.Model.ID]
		if !ok {
			c = claude.New(key, q.Judge.Model)
			clients[q.Judge.Model.ID] = c
		}
		return c.Ask(ctx, q.Request)
	}
}

// Stub answers as a judge that is always right would, by the markup of the sheet: "correct" on a
// correct sheet, "unreadable" on an unreadable one, and on a sheet with a mistake "error" with the
// box round the points of the line of the first error. A sheet of another outcome, such as one
// without an answer, has no verdict of the judge's, and the stub fails on it. Its tokens are those
// the estimate expects, so that a run on it costs what the estimate says. It asks no one: a run on
// it needs no key and no network.
func Stub(_ context.Context, q Question) (provider.Response, error) {
	var answer judge.Answer
	switch e := q.Sample.Expected; e.Outcome {
	case sample.OutcomeCorrect:
		answer.Verdict = judge.Correct
	case sample.OutcomeUnreadable:
		answer.Verdict = judge.Unreadable
	case sample.OutcomeError:
		answer.Verdict = judge.Error
		if e.FirstError != nil {
			answer.Box = judge.Around(q.Sample, *e.FirstError)
		}
	default:
		return provider.Response{}, fmt.Errorf("measure: the stub has no verdict for a sheet of the outcome %q", e.Outcome)
	}
	data, err := json.Marshal(answer)
	if err != nil {
		return provider.Response{}, err
	}
	frames := len(q.Request.Images) - 1
	return provider.Response{
		JSON:         string(data),
		Model:        q.Judge.Model.ID,
		InputTokens:  expected(q.Form, frames),
		OutputTokens: int64(q.Judge.Output),
	}, nil
}

// Estimate is what a run takes before it starts: the calls, what they are expected to cost, and
// the most they could cost, in US dollars.
type Estimate struct {
	Calls    int     `json:"calls"`
	USD      float64 `json:"usd"`
	WorstUSD float64 `json:"worst_usd"`
}

// step is a call of the plan, or a form D that its form B stands for.
type step struct {
	Call
	sheet  *prepared
	judge  Judge
	sameAs judge.Form
}

// prepared is a sheet of the run with what the forms show of it.
type prepared struct {
	Sheet
	shown *judge.Sheet
}

// Run is a run of the measurement, planned: its sheets drawn, its calls in order, all in one version
// of the judge's prompt.
type Run struct {
	version judge.Version
	part    string
	judges  []Judge
	repeats int
	sheets  []prepared
	steps   []step
}

// New plans a run of the part's sheets by the judges, repeated: it draws what the forms show of
// every sheet and puts the calls in order, repeat by repeat, so that a run cut short has the same
// repeats of every sheet. Of each sheet, by each judge, it asks forms A and B and, with frames, C
// and D. A sheet without frames has no C, and its D is its B (architecture 3.1): the run does not
// ask it again, and the log says so.
func New(part string, sheets []Sheet, judges []Judge, repeats int) (*Run, error) {
	if repeats < 1 {
		return nil, fmt.Errorf("measure: %d repeats", repeats)
	}
	// The prompt of E2: which version a plan of free writing takes is T65's.
	r := &Run{version: judge.V1, part: part, judges: judges, repeats: repeats}
	for _, sh := range sheets {
		shown, err := judge.Prepare(sh.Sample, r.version)
		if err != nil {
			return nil, fmt.Errorf("measure: sheet %d: %w", sh.Number, err)
		}
		r.sheets = append(r.sheets, prepared{Sheet: sh, shown: shown})
	}
	for rep := 1; rep <= repeats; rep++ {
		for i := range r.sheets {
			sh := &r.sheets[i]
			for _, j := range judges {
				for _, f := range judge.Forms {
					st := step{Call: Call{Sheet: sh.Number, Form: f, Judge: j.Name, Repeat: rep}, sheet: sh, judge: j}
					switch {
					case len(sh.shown.Frames) > 0 || !f.Frames():
					case f == judge.FormD:
						st.sameAs = judge.FormB
					default:
						continue
					}
					r.steps = append(r.steps, st)
				}
			}
		}
	}
	return r, nil
}

// Framed counts the sheets of the run that have frames.
func (r *Run) Framed() int {
	n := 0
	for _, sh := range r.sheets {
		if len(sh.shown.Frames) > 0 {
			n++
		}
	}
	return n
}

// Estimate is what the run is expected to take.
func (r *Run) Estimate() Estimate {
	var e Estimate
	for _, st := range r.steps {
		if st.sameAs != "" {
			continue
		}
		e.Calls++
		e.USD += st.judge.cost(expected(st.Form, len(st.sheet.shown.Frames)), int64(st.judge.Output))
		e.WorstUSD += st.judge.worst()
	}
	return e
}

// Result is how a run ended: the calls it made and what they cost, and why it stopped short, if it
// did: the cap, or an interruption.
type Result struct {
	Calls    int     `json:"calls"`
	SpentUSD float64 `json:"spent_usd"`
	Stopped  string  `json:"stopped,omitempty"`
}

// The records of a run's log, a JSON object a line, each with its kind. The first tells the run,
// then one tells each sheet, one each call, and the last how the run ended. A call of form D that
// form B stands for has same_as and nothing else of a call's own (New).
type (
	// RunRecord tells the run: its part, judges and repeats, its cap and estimate, and the
	// fingerprint of its prompt (judge.Fingerprint).
	RunRecord struct {
		Kind     string        `json:"kind"`
		Time     time.Time     `json:"time"`
		Part     string        `json:"part"`
		Judges   []JudgeRecord `json:"judges"`
		Repeats  int           `json:"repeats"`
		MaxUSD   float64       `json:"max_usd"`
		Estimate Estimate      `json:"estimate"`
		Prompt   string        `json:"prompt"`
	}
	// JudgeRecord is a judge as the run's record tells it: its model and prices.
	JudgeRecord struct {
		Name       string  `json:"name"`
		Model      string  `json:"model"`
		Effort     string  `json:"effort"`
		NoThinking bool    `json:"no_thinking,omitempty"`
		InputUSD   float64 `json:"input_usd_per_mtok"`
		OutputUSD  float64 `json:"output_usd_per_mtok"`
	}
	// SheetRecord tells a sheet: its number and file, its task and scenario, the outcome and the
	// first error its markup says, its frames and the agreement of the journal's lines with its
	// markup.
	SheetRecord struct {
		Kind       string            `json:"kind"`
		Sheet      int               `json:"sheet"`
		File       string            `json:"file"`
		Task       string            `json:"task"`
		Scenario   sample.Scenario   `json:"scenario"`
		Outcome    sample.Outcome    `json:"outcome"`
		FirstError int               `json:"first_error,omitempty"`
		Frames     int               `json:"frames"`
		Lines      journal.Agreement `json:"lines"`
	}
	// CallRecord tells a call: the answer as the model gave it, its verdict and box, the line of
	// the markup the box hits, the failure if it failed, and what it took. A call whose request
	// could not be built was not asked, and says so.
	CallRecord struct {
		Kind string `json:"kind"`
		Call
		SameAs         judge.Form `json:"same_as,omitempty"`
		NotAsked       bool       `json:"not_asked,omitempty"`
		Answer         string     `json:"answer,omitempty"`
		Verdict        string     `json:"verdict,omitempty"`
		Box            *judge.Box `json:"box,omitempty"`
		Hit            int        `json:"hit,omitempty"`
		Error          string     `json:"error,omitempty"`
		ModelID        string     `json:"model_id,omitempty"`
		InputTokens    int64      `json:"input_tokens,omitempty"`
		OutputTokens   int64      `json:"output_tokens,omitempty"`
		ThinkingTokens int64      `json:"thinking_tokens,omitempty"`
		FirstTokenMs   int64      `json:"first_token_ms,omitempty"`
		Retries        int        `json:"retries,omitempty"`
		LatencyMs      int64      `json:"latency_ms,omitempty"`
		CostUSD        float64    `json:"cost_usd,omitempty"`
	}
	// EndRecord tells how the run ended.
	EndRecord struct {
		Kind string    `json:"kind"`
		Time time.Time `json:"time"`
		Result
	}
)

// Log is a run's log read back: the run, its sheets, its calls in the order they were made, and how
// the run ended, or nil when the log stops before the end: a run that broke off.
type Log struct {
	Run    RunRecord
	Sheets []SheetRecord
	Calls  []CallRecord
	End    *EndRecord
}

// ReadLog reads a run's log back: every line a record of a kind it knows, the run's first and the
// end, if there is one, last. A last line cut off half written is a run that broke off as it wrote
// it, and the log is read without it.
func ReadLog(r io.Reader) (*Log, error) {
	var l Log
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	var lines [][]byte
	for sc.Scan() {
		lines = append(lines, slices.Clone(sc.Bytes()))
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if n := len(lines); n > 1 && !json.Valid(lines[n-1]) {
		lines = lines[:n-1]
	}
	for i, line := range lines {
		n := i + 1
		var kind struct{ Kind string }
		if err := json.Unmarshal(line, &kind); err != nil {
			return nil, fmt.Errorf("line %d of the log is not a record: %w", n, err)
		}
		if (n == 1) != (kind.Kind == "run") || l.End != nil {
			return nil, fmt.Errorf("line %d of the log is a %q record out of its place", n, kind.Kind)
		}
		var err error
		switch kind.Kind {
		case "run":
			err = json.Unmarshal(line, &l.Run)
		case "sheet":
			var rec SheetRecord
			err = json.Unmarshal(line, &rec)
			l.Sheets = append(l.Sheets, rec)
		case "call":
			var rec CallRecord
			err = json.Unmarshal(line, &rec)
			l.Calls = append(l.Calls, rec)
		case "end":
			l.End = &EndRecord{}
			err = json.Unmarshal(line, l.End)
		default:
			err = fmt.Errorf("no record of the kind %q", kind.Kind)
		}
		if err != nil {
			return nil, fmt.Errorf("line %d of the log: %w", n, err)
		}
	}
	if l.Run.Kind == "" {
		return nil, fmt.Errorf("the log is empty")
	}
	return &l, nil
}

// Do runs the calls in order, asking each question of ask, and logs every answer to log as it
// comes. Before each call it reserves the call's worst case: a call that could take what the run
// has spent past maxUSD is not started, and the run stops there. A context that ends stops the run
// too. A failed call is logged with its error and counts as an answer, and its tokens are billed;
// a call whose bill is not known counts at its worst. The error is a log that cannot be written:
// the run stops then, since an answer it cannot keep is lost.
func (r *Run) Do(ctx context.Context, ask Asker, maxUSD float64, log io.Writer, now func() time.Time) (Result, error) {
	w := &writer{log: log}
	var judges []JudgeRecord
	for _, j := range r.judges {
		judges = append(judges, JudgeRecord{Name: j.Name, Model: j.Model.ID, Effort: j.Effort, NoThinking: j.NoThinking, InputUSD: j.Model.Input, OutputUSD: j.Model.Output})
	}
	w.put(RunRecord{Kind: "run", Time: now().UTC(), Part: r.part, Judges: judges, Repeats: r.repeats, MaxUSD: maxUSD, Estimate: r.Estimate(), Prompt: judge.Fingerprint(r.version)})
	for _, sh := range r.sheets {
		rec := SheetRecord{Kind: "sheet", Sheet: sh.Number, File: sh.File, Task: sh.Sample.Task, Scenario: sh.Sample.Scenario,
			Outcome: sh.Sample.Expected.Outcome, Frames: len(sh.shown.Frames), Lines: journal.Agree(sh.Sample, sh.shown.Journal)}
		if fe := sh.Sample.Expected.FirstError; fe != nil {
			rec.FirstError = *fe
		}
		w.put(rec)
	}
	var res Result
	for _, st := range r.steps {
		if w.err != nil {
			break
		}
		if st.sameAs != "" {
			w.put(CallRecord{Kind: "call", Call: st.Call, SameAs: st.sameAs})
			continue
		}
		if ctx.Err() != nil {
			res.Stopped = "interrupted"
			break
		}
		if res.SpentUSD+st.judge.worst() > maxUSD {
			res.Stopped = fmt.Sprintf("the cap: $%.4f spent of $%.2f, and the next call can cost up to $%.4f", res.SpentUSD, maxUSD, st.judge.worst())
			break
		}
		rec, asked := r.call(ctx, ask, st)
		if asked {
			res.Calls++
		}
		res.SpentUSD += rec.CostUSD
		w.put(rec)
	}
	w.put(EndRecord{Kind: "end", Time: now().UTC(), Result: res})
	return res, w.err
}

// call asks one question and turns the answer into its record; asked is false when the request
// could not be built, and nothing was asked.
func (r *Run) call(ctx context.Context, ask Asker, st step) (rec CallRecord, asked bool) {
	req, err := st.Form.Request(st.sheet.Task, st.sheet.shown)
	rec = CallRecord{Kind: "call", Call: st.Call}
	if err != nil {
		rec.Error, rec.NotAsked = err.Error(), true
		return rec, false
	}
	req.MaxOutputTokens, req.Effort, req.NoThinking = MaxOutputTokens, st.judge.Effort, st.judge.NoThinking
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	start := time.Now()
	resp, err := ask(ctx, Question{Judge: st.judge, Form: st.Form, Sample: st.sheet.Sample, Request: req})
	rec.LatencyMs = time.Since(start).Milliseconds()
	rec.ModelID, rec.InputTokens, rec.OutputTokens, rec.ThinkingTokens = resp.Model, resp.InputTokens, resp.OutputTokens, resp.ThinkingTokens
	rec.FirstTokenMs, rec.Retries = resp.FirstToken.Milliseconds(), resp.Retries
	rec.CostUSD = st.judge.cost(resp.InputTokens, resp.OutputTokens)
	if err != nil {
		rec.Error = err.Error()
		if !provider.Billed(resp, err) {
			// The bill of such a call is not known, though the model may have spent tokens, as on
			// the board: it counts at the most it could cost, so that the cap still holds.
			rec.CostUSD = st.judge.worst()
			rec.Error += "; the call is counted at the most it could cost"
		}
		return rec, true
	}
	rec.Answer = resp.JSON
	a, err := judge.Read(resp.JSON)
	if err != nil {
		rec.Error = err.Error()
		return rec, true
	}
	rec.Verdict, rec.Box = a.Verdict, a.Box
	if a.Box != nil {
		rec.Hit = judge.Hit(st.sheet.Sample, *a.Box)
	}
	return rec, true
}

// writer writes the records of the log, a JSON object a line, and keeps the first error.
type writer struct {
	log io.Writer
	err error
}

func (w *writer) put(v any) {
	if w.err != nil {
		return
	}
	data, err := json.Marshal(v)
	if err == nil {
		_, err = w.log.Write(append(data, '\n'))
	}
	w.err = err
}
