package runner_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/altnovatrail/tutor/internal/runner"
	"github.com/altnovatrail/tutor/provider"
)

// echo is a kind of call for the tests: call n asks for its number, and the right answer is that
// number in JSON, padded with pad letters if pad is set. A call can be set to fail to build its
// request, or to have no right answer.
type echo struct {
	n, pad   int
	noReq    bool
	noAnswer bool
}

func (e *echo) Request() (provider.Request, error) {
	if e.noReq {
		return provider.Request{}, errors.New("echo: no request")
	}
	return provider.Request{Text: strings.Repeat("x", e.n)}, nil
}

func (e *echo) Right() (string, error) {
	if e.noAnswer {
		return "", errors.New("echo: no right answer")
	}
	answer := map[string]any{"n": e.n}
	if e.pad > 0 {
		answer["pad"] = strings.Repeat("x", e.pad)
	}
	data, err := json.Marshal(answer)
	return string(data), err
}

func (e *echo) Record(o runner.Outcome) any {
	return record{Kind: "call", N: e.n, NotAsked: o.NotAsked, Answer: o.Answer, Error: o.Error, Usage: o.Usage}
}

type record struct {
	Kind     string `json:"kind"`
	N        int    `json:"n"`
	NotAsked bool   `json:"not_asked,omitempty"`
	Answer   string `json:"answer,omitempty"`
	Error    string `json:"error,omitempty"`
	runner.Usage
}

var (
	opus      = runner.Configs()[0]
	noThought = runner.Configs()[2]
)

// worst is the most a call of Opus 5.5 can cost: 8 000 tokens in at $4 and 16 000 out at $20.
const worst = (8000*4 + 16000*20) / 1e6

// plan is a run of the echo kind: its record, one item and the steps.
func plan(steps ...runner.Step) *runner.Run {
	return &runner.Run{
		Record: func(h runner.Head) any {
			return map[string]any{"kind": "run", "call_kind": "echo", "time": h.Time, "max_usd": h.MaxUSD, "estimate": h.Estimate}
		},
		Items: []any{map[string]any{"kind": "item", "n": 1}},
		Steps: steps,
	}
}

func asked(c runner.Config, n int) runner.Step {
	return runner.Step{Config: c, In: 1000, Out: 50, WorstIn: 8000, Call: &echo{n: n}}
}

// clock is a clock that moves 1.5 s at every reading, from a fixed moment.
func clock() func() time.Time {
	at := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	return func() time.Time {
		at = at.Add(1500 * time.Millisecond)
		return at
	}
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// records reads the log back.
func records(t *testing.T, log []byte) []map[string]any {
	t.Helper()
	var out []map[string]any
	sc := bufio.NewScanner(bytes.NewReader(log))
	for sc.Scan() {
		var rec map[string]any
		if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
			t.Fatalf("a line of the log is not JSON: %s", sc.Text())
		}
		out = append(out, rec)
	}
	return out
}

// The three configurations are those of R24. The worst case of a call is the most input tokens
// its kind gives and the output limit, at the model's prices, and at least the expected input.
func TestConfigs(t *testing.T) {
	var names []string
	for _, c := range runner.Configs() {
		names = append(names, c.Name+" "+c.Model.ID+" "+c.Effort)
		if c.NoThinking != (c.Name == "haiku-low-no-thinking") {
			t.Errorf("%s thinks: %v", c.Name, !c.NoThinking)
		}
	}
	if got := strings.Join(names, ", "); got != "opus-high claude-opus-5-5 high, haiku-low claude-haiku-5-5 low, haiku-low-no-thinking claude-haiku-5-5 low" {
		t.Errorf("the configurations are %s", got)
	}
	if w := asked(opus, 1).Worst(); !near(w, worst) || !near(opus.Cost(1000, 50), (1000*4+50*20)/1e6) {
		t.Errorf("Opus costs %v at worst and %v for 1 000 tokens in and 50 out", w, opus.Cost(1000, 50))
	}
	if w := (runner.Step{Config: opus, In: 9000}).Worst(); !near(w, opus.Cost(9000, runner.MaxOutputTokens)) {
		t.Errorf("a step that gives no most input costs %v at worst, want its expected input", w)
	}
}

// A run on the stub logs its record, its items, every call in order with the stub's right answer
// and a note in its place, and its end. Each request has the output limit, the effort and the
// thinking of its configuration and a deadline; the stub bills the tokens the estimate expects,
// and the time of a call is that of the run's clock.
func TestDo(t *testing.T) {
	r := plan(asked(opus, 1), runner.Step{Config: opus, Note: map[string]any{"kind": "call", "n": 2, "same_as": 1}}, asked(noThought, 3))
	if e := r.Estimate(); e.Calls != 2 || !near(e.USD, opus.Cost(1000, 50)+noThought.Cost(1000, 50)) || !near(e.WorstUSD, worst+noThought.Cost(8000, runner.MaxOutputTokens)) {
		t.Errorf("the estimate is %+v", e)
	}
	var reqs []provider.Request
	ask := func(ctx context.Context, st runner.Step, req provider.Request) (provider.Response, error) {
		if _, ok := ctx.Deadline(); !ok {
			t.Error("a call has no deadline")
		}
		reqs = append(reqs, req)
		return runner.Stub(ctx, st, req)
	}
	var log bytes.Buffer
	res, err := r.Do(context.Background(), ask, 10, &log, clock())
	if err != nil {
		t.Fatal(err)
	}
	if res.Calls != 2 || !near(res.SpentUSD, r.Estimate().USD) || res.Stopped != "" {
		t.Errorf("the run ended %+v", res)
	}
	if len(reqs) != 2 || reqs[0].MaxOutputTokens != runner.MaxOutputTokens || reqs[0].Effort != "high" || reqs[0].NoThinking || reqs[1].Effort != "low" || !reqs[1].NoThinking || reqs[1].Text != "xxx" {
		t.Errorf("the requests are %+v", reqs)
	}
	recs := records(t, log.Bytes())
	var kinds []string
	for _, rec := range recs {
		kinds = append(kinds, rec["kind"].(string))
	}
	if got := strings.Join(kinds, " "); got != "run item call call call end" {
		t.Fatalf("the log is of the records %s", got)
	}
	if run := recs[0]; run["max_usd"] != 10.0 || run["time"] != "2026-10-27T12:00:01.5Z" || run["estimate"].(map[string]any)["calls"] != 2.0 {
		t.Errorf("the run's record is %v", run)
	}
	if note := recs[3]; note["same_as"] != 1.0 || note["model_id"] != nil {
		t.Errorf("the note is %v", note)
	}
	for _, c := range []map[string]any{recs[2], recs[4]} {
		if c["answer"] != `{"n":`+jsonNumber(c["n"])+`}` || c["input_tokens"] != 1000.0 || c["output_tokens"] != 50.0 || c["latency_ms"] != 1500.0 || c["model_id"] == nil {
			t.Errorf("a call is logged %v", c)
		}
	}
	if end := recs[5]; end["calls"] != 2.0 || end["stopped"] != nil {
		t.Errorf("the end is %v", end)
	}
}

func jsonNumber(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}

// The run reserves the worst case of every call: it starts no call that could take it past its
// cap, and says so.
func TestCap(t *testing.T) {
	var steps []runner.Step
	for n := 1; n <= 10; n++ {
		steps = append(steps, asked(opus, n))
	}
	r := plan(steps...)
	var log bytes.Buffer
	res, err := r.Do(context.Background(), runner.Stub, worst+0.03, &log, clock())
	if err != nil {
		t.Fatal(err)
	}
	if res.Calls == 0 || res.Calls >= 10 || !strings.HasPrefix(res.Stopped, "the cap: ") {
		t.Fatalf("the run ended %+v, want it stopped by the cap after a few of its 10 calls", res)
	}
	if res.SpentUSD+worst <= worst+0.03 || res.SpentUSD-opus.Cost(1000, 50)+worst > worst+0.03 {
		t.Errorf("the run stopped at $%.4f: it stops at the first call it cannot afford", res.SpentUSD)
	}
	recs := records(t, log.Bytes())
	if end := recs[len(recs)-1]; end["stopped"] != res.Stopped || len(recs) != 2+res.Calls+1 {
		t.Errorf("the log ends %v after %d records", end, len(recs))
	}
}

// A run whose context ends stops before the next call.
func TestInterrupted(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var log bytes.Buffer
	if res, err := plan(asked(opus, 1)).Do(ctx, runner.Stub, 10, &log, clock()); err != nil || res.Calls != 0 || res.Stopped != "interrupted" {
		t.Errorf("an interrupted run ended %+v, %v", res, err)
	}
}

// A failed call is logged with its error and counts as an answer, and its tokens are billed; a
// call whose bill is not known counts at its worst. A call whose request cannot be built is not
// asked and costs nothing, and a call the stub has no right answer for fails.
func TestFailures(t *testing.T) {
	r := plan(asked(opus, 1), asked(opus, 2), runner.Step{Config: opus, WorstIn: 8000, Call: &echo{n: 3, noReq: true}}, runner.Step{Config: opus, WorstIn: 8000, Call: &echo{n: 4, noAnswer: true}})
	ask := func(ctx context.Context, st runner.Step, req provider.Request) (provider.Response, error) {
		switch len(req.Text) {
		case 1:
			return provider.Response{InputTokens: 1000, OutputTokens: 20, Partial: true}, errors.New("the stream broke")
		case 2:
			return provider.Response{InputTokens: 1000, OutputTokens: 20}, errors.New("the answer was refused")
		}
		return runner.Stub(ctx, st, req)
	}
	var log bytes.Buffer
	res, err := r.Do(context.Background(), ask, 10, &log, clock())
	if err != nil {
		t.Fatal(err)
	}
	if res.Calls != 3 || !near(res.SpentUSD, worst+opus.Cost(1000, 20)) {
		t.Errorf("the run ended %+v, want three calls, one at its worst and one by its tokens", res)
	}
	recs := records(t, log.Bytes())
	want := map[float64]struct {
		says     string
		cost     float64
		notAsked bool
	}{
		1: {"the stream broke; the call is counted at the most it could cost", worst, false},
		2: {"the answer was refused", opus.Cost(1000, 20), false},
		3: {"echo: no request", 0, true},
		4: {"echo: no right answer", 0, false},
	}
	for _, rec := range recs[2 : len(recs)-1] {
		w := want[rec["n"].(float64)]
		cost, _ := rec["cost_usd"].(float64)
		if rec["error"] != w.says || !near(cost, w.cost) || (rec["not_asked"] == true) != w.notAsked || rec["answer"] != nil {
			t.Errorf("call %v is logged %v", rec["n"], rec)
		}
	}
	if recs[5]["model_id"] != nil {
		t.Errorf("the call without a right answer names a model: %v", recs[5])
	}
}

type failing struct{ after int }

func (f *failing) Write(p []byte) (int, error) {
	if f.after == 0 {
		return 0, errors.New("the disk is full")
	}
	f.after--
	return len(p), nil
}

// A log that cannot be written stops the run: an answer it cannot keep is lost.
func TestLogFails(t *testing.T) {
	res, err := plan(asked(opus, 1), asked(opus, 2), asked(opus, 3)).Do(context.Background(), runner.Stub, 10, &failing{after: 3}, clock())
	if err == nil || res.Calls != 2 {
		t.Errorf("a run with a failing log ended %+v, %v", res, err)
	}
}

// A run's log reads back as it was written, by the kind's reader, with its end; a log whose last
// line was cut off half written reads without that line; a log out of order, with a line that is
// no record or a record its kind does not know, or empty, does not. A line may be longer than the
// scanner's default buffer.
func TestReadLog(t *testing.T) {
	var log bytes.Buffer
	res, err := plan(asked(opus, 1), runner.Step{Config: opus, WorstIn: 8000, Call: &echo{n: 2, pad: 70 * 1024}}).Do(context.Background(), runner.Stub, 10, &log, clock())
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	read := func(kind string, line []byte) error {
		if kind == "pause" {
			return errors.New("no record of the kind \"pause\"")
		}
		kinds = append(kinds, kind)
		return nil
	}
	end, err := runner.ReadLog(bytes.NewReader(log.Bytes()), read)
	if err != nil || end == nil || end.Calls != res.Calls || strings.Join(kinds, " ") != "run item call call" {
		t.Errorf("the log reads back as %v, %+v, %v", kinds, end, err)
	}
	lines := strings.SplitAfter(log.String(), "\n")
	for name, c := range map[string]struct{ text, says string }{
		"empty":           {"", "the log is empty"},
		"an item first":   {lines[1], `line 1 of the log is a "item" record out of its place`},
		"after the end":   {log.String() + lines[1], `line 6 of the log is a "item" record out of its place`},
		"an unknown kind": {lines[0] + `{"kind": "pause"}` + "\n" + lines[1], `line 2 of the log: no record of the kind "pause"`},
		"not JSON":        {lines[0] + "a pause\n" + lines[1], "line 2 of the log is not a record"},
	} {
		if _, err := runner.ReadLog(strings.NewReader(c.text), read); err == nil || !strings.Contains(err.Error(), c.says) {
			t.Errorf("%s: %v, want %q", name, err, c.says)
		}
	}
	kinds = nil
	cut, err := runner.ReadLog(strings.NewReader(lines[0]+lines[1]+lines[2][:10]), read)
	if err != nil || cut != nil || strings.Join(kinds, " ") != "run item" {
		t.Errorf("a log cut off in its third line reads as %v, %+v, %v", kinds, cut, err)
	}
}

// The kind of call of a log is that its first line tells, and none for a log that tells none or
// does not start with a record.
func TestCallKind(t *testing.T) {
	for log, want := range map[string]string{
		`{"kind": "run", "call_kind": "reading"}` + "\n" + `{"kind": "end"}`: "reading",
		`{"kind": "run", "part": "test"}` + "\n":                             "",
		"a pause\n":                                                          "",
		"":                                                                   "",
	} {
		if got := runner.CallKind([]byte(log)); got != want {
			t.Errorf("the log %q is of the kind %q, want %q", log, got, want)
		}
	}
}

// A log of a kind of call reads back into the kind's records: its run, its sheets and its calls in
// order, and its end. A log of another kind of call, a record of no kind the log has a place for,
// and a record off its type are refused.
func TestReadRecords(t *testing.T) {
	type (
		run struct {
			CallKind string `json:"call_kind"`
			Part     string `json:"part"`
		}
		sheet struct{ Sheet int }
		call  struct{ Sheet, Repeat int }
	)
	log := `{"kind": "run", "call_kind": "echo", "part": "dev"}
{"kind": "sheet", "sheet": 4}
{"kind": "call", "sheet": 4, "repeat": 1}
{"kind": "call", "sheet": 4, "repeat": 2}
{"kind": "end", "calls": 2}
`
	l, err := runner.ReadRecords[run, sheet, call](strings.NewReader(log), "echo", "echo")
	if err != nil || l.Run.Part != "dev" || len(l.Sheets) != 1 || l.Sheets[0].Sheet != 4 || len(l.Calls) != 2 || l.Calls[1].Repeat != 2 || l.End == nil || l.End.Calls != 2 {
		t.Fatalf("the log reads back as %+v, %v", l, err)
	}
	for name, c := range map[string]struct{ text, says string }{
		"another kind":  {`{"kind": "run", "call_kind": "reading"}` + "\n", `a log of the kind of call "reading", not of the echo`},
		"a judge's":     {`{"kind": "run", "part": "test"}` + "\n", `a log of the kind of call "", not of the echo`},
		"a frame":       {`{"kind": "run", "call_kind": "echo"}` + "\n" + `{"kind": "frame"}` + "\n", `no record of the kind "frame"`},
		"a run's part":  {`{"kind": "run", "call_kind": "echo", "part": 3}` + "\n", "cannot unmarshal"},
		"a sheet's":     {`{"kind": "run", "call_kind": "echo"}` + "\n" + `{"kind": "sheet", "sheet": "4"}` + "\n", "cannot unmarshal"},
		"a call's":      {`{"kind": "run", "call_kind": "echo"}` + "\n" + `{"kind": "call", "repeat": "1"}` + "\n", "cannot unmarshal"},
		"a kind of run": {`{"kind": "run", "call_kind": 7}` + "\n", "cannot unmarshal"},
		"empty":         {"", "the log is empty"},
	} {
		if _, err := runner.ReadRecords[run, sheet, call](strings.NewReader(c.text), "echo", "echo"); err == nil || !strings.Contains(err.Error(), c.says) {
			t.Errorf("%s: %v, want %q", name, err, c.says)
		}
	}
}

// The readers of a kind are one for each configuration, with the output the kind expects and once
// for those named so; a configuration the kind forgot is a mistake of the code. A reader's record
// tells its model and prices.
func TestReadersOf(t *testing.T) {
	readers := runner.ReadersOf("test", map[string]int{"opus-high": 7, "haiku-low": 8, "haiku-low-no-thinking": 9}, "opus-high")
	if len(readers) != 3 || !readers[0].Once || readers[1].Once || readers[0].Output != 7 || readers[2].Output != 9 {
		t.Errorf("ReadersOf = %+v", readers)
	}
	if rec := readers[1].Record(); rec.Name != "haiku-low" || rec.Model != readers[1].Model.ID || rec.InputUSD != readers[1].Model.Input || rec.Once {
		t.Errorf("the record of haiku-low is %+v", rec)
	}
	defer func() {
		if recover() == nil {
			t.Error("a configuration without its reader is taken")
		}
	}()
	runner.ReadersOf("test", map[string]int{"opus-high": 7})
}
