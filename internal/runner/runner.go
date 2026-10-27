// Package runner runs the calls of a measurement, whatever their kind (T67): the judge of the
// writing process, and in epic E4 also reading, the hint, the cells of the workbook and the rubric
// (docs/architecture.md, 3.4). A kind plans the calls of its run and says how each is asked and
// logged. The runner estimates the run before it starts, starts no call whose worst case could
// take the run past its cap, asks the calls in order, bills them, and logs every answer, a JSON
// object a line, outside git (R05).
package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/altnovatrail/tutor/provider"
	"github.com/altnovatrail/tutor/provider/claude"
)

const (
	// MaxOutputTokens is the most a call may write, thinking included: the same for every
	// configuration, as on the board, so that they are compared under one limit (R19).
	MaxOutputTokens = 16000
	// callTimeout is how long one call may take, as on the board.
	callTimeout = 10 * time.Minute
)

// Config is a configuration of a model that a run asks (R24): a Claude model at an effort level,
// thinking or not.
type Config struct {
	Name       string
	Model      claude.Model
	Effort     string
	NoThinking bool
}

// Configs are the three configurations of the measurements: Opus 5.5 at high, Haiku 5.5 at low,
// and Haiku 5.5 at low without thinking (R24; architecture 3.4).
func Configs() []Config {
	model := func(id string) claude.Model {
		i := slices.IndexFunc(claude.Models, func(m claude.Model) bool { return m.ID == id })
		return claude.Models[i]
	}
	return []Config{
		{Name: "opus-high", Model: model("claude-opus-5-5"), Effort: "high"},
		{Name: "haiku-low", Model: model("claude-haiku-5-5"), Effort: "low"},
		{Name: "haiku-low-no-thinking", Model: model("claude-haiku-5-5"), Effort: "low", NoThinking: true},
	}
}

// Cost is what a call of the configuration costs for its tokens, in US dollars.
func (c Config) Cost(in, out int64) float64 {
	return (float64(in)*c.Model.Input + float64(out)*c.Model.Output) / 1e6
}

// Step is a call of a run, in the order the run makes it: the configuration it asks, the input
// and output tokens the estimate expects it to take, the most input tokens it can take, for its
// worst case, and the call. How big a request can grow is the kind's to say: a request of the
// judge comes to about 4 000 tokens at most, and another kind's, with more pictures or a longer
// text, to more. A step that another call stands for, as form D of the judge stands on form B on a
// sheet without frames, has no call: the run writes its Note in its place in the log, and asks
// nothing.
type Step struct {
	Config           Config
	In, Out, WorstIn int64
	Call             Call
	Note             any
}

// Worst is the most the step's call can cost: its most input tokens, or the expected ones if the
// kind gave fewer, and the most a call may write.
func (st Step) Worst() float64 { return st.Config.Cost(max(st.WorstIn, st.In), MaxOutputTokens) }

// Call is what a kind says of a call of its run.
type Call interface {
	// Request is the call's request, built when the run comes to the call. A call whose request
	// cannot be built is not asked, and its record says so.
	Request() (provider.Request, error)
	// Right is the answer, in JSON, that a model which is always right would give, by the markup:
	// what Stub answers. A call that has no such answer fails on the stub.
	Right() (string, error)
	// Record is the call's record in the log, from what the call came to.
	Record(o Outcome) any
}

// Outcome is what a call came to: the answer as the model gave it, the failure if it failed, and
// what the call took. A call whose request could not be built was not asked, and says so.
type Outcome struct {
	NotAsked bool
	Answer   string
	Error    string
	Usage
}

// Usage is what a call took, under the names a kind's record of it gives them in the log: the
// model that answered, the tokens and the thinking, the time to the first token, the retries of
// the SDK, the time of the call and its cost.
type Usage struct {
	ModelID        string  `json:"model_id,omitempty"`
	InputTokens    int64   `json:"input_tokens,omitempty"`
	OutputTokens   int64   `json:"output_tokens,omitempty"`
	ThinkingTokens int64   `json:"thinking_tokens,omitempty"`
	FirstTokenMs   int64   `json:"first_token_ms,omitempty"`
	Retries        int     `json:"retries,omitempty"`
	LatencyMs      int64   `json:"latency_ms,omitempty"`
	CostUSD        float64 `json:"cost_usd,omitempty"`
}

// Estimate is what a run takes before it starts: the calls, what they are expected to cost, and
// the most they could cost, in US dollars.
type Estimate struct {
	Calls    int     `json:"calls"`
	USD      float64 `json:"usd"`
	WorstUSD float64 `json:"worst_usd"`
}

// Head is what the runner knows of a run as it starts it, for the run's record: the time, the cap
// and the estimate.
type Head struct {
	Time     time.Time
	MaxUSD   float64
	Estimate Estimate
}

// Run is a run as its kind planned it: the record that tells the run, the first line of its log,
// from what the runner knows as it starts; the records of its items, its sheets, which follow it;
// and its steps, in the order it makes them. The kind plans them repeat by repeat, so that a run
// cut short has the same repeats of every item. The record of a run of any kind but the judge
// tells its kind in a field call_kind; a log without one is the judge's, as those of E2 and T66
// are (CallKind).
type Run struct {
	Record func(h Head) any
	Items  []any
	Steps  []Step
}

// Estimate is what the run is expected to take.
func (r *Run) Estimate() Estimate {
	var e Estimate
	for _, st := range r.Steps {
		if st.Call == nil {
			continue
		}
		e.Calls++
		e.USD += st.Config.Cost(st.In, st.Out)
		e.WorstUSD += st.Worst()
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

// End is the last record of a run's log: how the run ended.
type End struct {
	Kind string    `json:"kind"`
	Time time.Time `json:"time"`
	Result
}

// Do runs the steps in order, asking each call of ask, and logs every answer to log as it comes:
// the run's record first, then those of its items, one each step, and the end. Before each call it
// reserves the call's worst case: a call that could take what the run has spent past maxUSD is not
// started, and the run stops there. A context that ends stops the run too. A failed call is logged
// with its error and counts as an answer, and its tokens are billed; a call whose bill is not known
// counts at its worst. The error is a log that cannot be written: the run stops then, since an
// answer it cannot keep is lost. The time of the run and of its calls is that of now: time.Now in
// a run, whose monotonic reading times the calls, and a fixed clock in a test.
func (r *Run) Do(ctx context.Context, ask Asker, maxUSD float64, log io.Writer, now func() time.Time) (Result, error) {
	w := &writer{log: log}
	w.put(r.Record(Head{Time: now().UTC(), MaxUSD: maxUSD, Estimate: r.Estimate()}))
	for _, it := range r.Items {
		w.put(it)
	}
	var res Result
	for _, st := range r.Steps {
		if w.err != nil {
			break
		}
		if st.Call == nil {
			w.put(st.Note)
			continue
		}
		if ctx.Err() != nil {
			res.Stopped = "interrupted"
			break
		}
		if worst := st.Worst(); res.SpentUSD+worst > maxUSD {
			res.Stopped = fmt.Sprintf("the cap: $%.4f spent of $%.2f, and the next call can cost up to $%.4f", res.SpentUSD, maxUSD, worst)
			break
		}
		o := call(ctx, ask, st, now)
		if !o.NotAsked {
			res.Calls++
		}
		res.SpentUSD += o.CostUSD
		w.put(st.Call.Record(o))
	}
	w.put(End{Kind: "end", Time: now().UTC(), Result: res})
	return res, w.err
}

// call asks one step's call and tells what it came to.
func call(ctx context.Context, ask Asker, st Step, now func() time.Time) Outcome {
	req, err := st.Call.Request()
	if err != nil {
		return Outcome{NotAsked: true, Error: err.Error()}
	}
	req.MaxOutputTokens, req.Effort, req.NoThinking = MaxOutputTokens, st.Config.Effort, st.Config.NoThinking
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	start := now()
	resp, err := ask(ctx, st, req)
	var o Outcome
	o.LatencyMs = now().Sub(start).Milliseconds()
	o.ModelID, o.InputTokens, o.OutputTokens, o.ThinkingTokens = resp.Model, resp.InputTokens, resp.OutputTokens, resp.ThinkingTokens
	o.FirstTokenMs, o.Retries = resp.FirstToken.Milliseconds(), resp.Retries
	o.CostUSD = st.Config.Cost(resp.InputTokens, resp.OutputTokens)
	if err != nil {
		o.Error = err.Error()
		if !provider.Billed(resp, err) {
			// The bill of such a call is not known, though the model may have spent tokens, as on
			// the board: it counts at the most it could cost, so that the cap still holds.
			o.CostUSD = st.Worst()
			o.Error += "; the call is counted at the most it could cost"
		}
		return o
	}
	o.Answer = resp.JSON
	return o
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
