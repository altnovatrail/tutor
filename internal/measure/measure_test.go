package measure_test

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

	"github.com/altnovatrail/tutor/internal/judge"
	"github.com/altnovatrail/tutor/internal/measure"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/synth"
	"github.com/altnovatrail/tutor/provider"
)

// sheets are the synthetic sheets a plan can hold, as a part of a run: five without frames, and
// erasure-fixed with two and erasure-broken with one.
func sheets(t *testing.T) []measure.Sheet {
	t.Helper()
	plan, samples := synth.Plan()
	bank := synth.Bank()
	var out []measure.Sheet
	for i, s := range samples {
		for k := range bank.Tasks {
			if bank.Tasks[k].ID == s.Task {
				out = append(out, measure.Sheet{Number: plan.Sheets[i].Number, File: string(s.Scenario) + ".json", Sample: s, Task: &bank.Tasks[k]})
			}
		}
	}
	return out
}

func judgeNamed(name string) measure.Judge {
	for _, j := range measure.Judges() {
		if j.Name == name {
			return j
		}
	}
	panic(name)
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// The estimate counts the calls and what they cost by the prices of provider/claude and the
// expected tokens of architecture 3.1: Opus 5.5 at high costs $0.0084 for form A and $0.0100 for
// form B, as the table of the estimate there says.
func TestEstimate(t *testing.T) {
	all := sheets(t)
	r, err := measure.New("test", all[:1], []measure.Judge{judgeNamed("opus-high")}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if e := r.Estimate(); e.Calls != 2 || !near(e.USD, 0.0084+0.0100) || !near(e.WorstUSD, 2*(8000*4+16000*20)/1e6) {
		t.Errorf("the estimate of one sheet without frames by Opus is %+v, want 2 calls, $0.0184 and at most $0.704", e)
	}
	// All seven sheets by the three judges, twice: A and B of every sheet, C and D of the two with
	// frames; the D of the other five is their B.
	r, err = measure.New("test", all, measure.Judges(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if e := r.Estimate(); e.Calls != 2*3*(5*2+2*4) {
		t.Errorf("the run makes %d calls, want %d", e.Calls, 2*3*(5*2+2*4))
	}
	if _, err := measure.New("test", all, measure.Judges(), 0); err == nil {
		t.Error("a run of no repeats is planned")
	}
}

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

func clock() time.Time { return time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC) }

// A run on the stub goes the whole way without a key or a network: every call is logged with the
// stub's right answer, a mistake's box hits the line of the first error, the D of a sheet without
// frames points at its B, and the run costs what the estimate said.
func TestRunOnTheStub(t *testing.T) {
	all := sheets(t)
	r, err := measure.New("test", all, measure.Judges(), 1)
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	res, err := r.Do(context.Background(), measure.Stub, 10, &log, clock)
	if err != nil {
		t.Fatal(err)
	}
	e := r.Estimate()
	if res.Calls != e.Calls || !near(res.SpentUSD, e.USD) || res.Stopped != "" {
		t.Errorf("the run ended %+v, want all %d calls for the estimate's $%.4f", res, e.Calls, e.USD)
	}
	recs := records(t, log.Bytes())
	if recs[0]["kind"] != "run" || recs[len(recs)-1]["kind"] != "end" {
		t.Fatalf("the log runs from %v to %v", recs[0]["kind"], recs[len(recs)-1]["kind"])
	}
	if recs[0]["prompt"] != judge.Fingerprint() {
		t.Errorf("the run's record names the prompt %v, want its fingerprint %s", recs[0]["prompt"], judge.Fingerprint())
	}
	expected := map[float64]map[string]any{}
	calls, same := 0, 0
	for _, rec := range recs[1 : len(recs)-1] {
		switch rec["kind"] {
		case "sheet":
			expected[rec["sheet"].(float64)] = rec
			if lines := rec["lines"].(map[string]any); lines["marked"] != lines["same"] {
				t.Errorf("sheet %v: the journal finds %v of its %v lines", rec["sheet"], lines["same"], lines["marked"])
			}
		case "call":
			sh := expected[rec["sheet"].(float64)]
			if rec["same_as"] != nil {
				same++
				if rec["form"] != "D" || rec["same_as"] != "B" || sh["frames"].(float64) != 0 {
					t.Errorf("a call stands for another: %v", rec)
				}
				continue
			}
			calls++
			want := map[sample.Outcome]string{sample.OutcomeCorrect: judge.Correct, sample.OutcomeError: judge.Error}[sample.Outcome(sh["outcome"].(string))]
			if rec["verdict"] != want || rec["error"] != nil {
				t.Errorf("sheet %v, form %v: the stub says %v, want %v", rec["sheet"], rec["form"], rec["verdict"], want)
			}
			if want == judge.Error && rec["hit"] != sh["first_error"] {
				t.Errorf("sheet %v, form %v: the box hits line %v, want the first error's %v", rec["sheet"], rec["form"], rec["hit"], sh["first_error"])
			}
			if (rec["form"] == "C" || rec["form"] == "D") && sh["frames"].(float64) == 0 {
				t.Errorf("form %v is asked of sheet %v, which has no frames", rec["form"], rec["sheet"])
			}
		default:
			t.Errorf("a record of the kind %v", rec["kind"])
		}
	}
	if len(expected) != len(all) || calls != e.Calls || same != 3*5 {
		t.Errorf("the log tells %d sheets, %d calls and %d forms D that stand for B, want %d, %d and 15", len(expected), calls, same, len(all), e.Calls)
	}
}

// The run reserves the worst case of every call: it starts no call that could take it past its
// cap, and says so.
func TestCap(t *testing.T) {
	opus := judgeNamed("opus-high")
	r, err := measure.New("test", sheets(t), []measure.Judge{opus}, 1)
	if err != nil {
		t.Fatal(err)
	}
	worst := (8000*4 + 16000*20) / 1e6
	var log bytes.Buffer
	res, err := r.Do(context.Background(), measure.Stub, worst+0.03, &log, clock)
	if err != nil {
		t.Fatal(err)
	}
	if res.Calls == 0 || res.Calls >= r.Estimate().Calls || !strings.HasPrefix(res.Stopped, "the cap") {
		t.Fatalf("the run ended %+v, want it stopped by the cap after a few of its %d calls", res, r.Estimate().Calls)
	}
	if res.SpentUSD+worst <= worst+0.03 {
		t.Errorf("the run stopped at $%.4f, when the next call could still be afforded", res.SpentUSD)
	}
	recs := records(t, log.Bytes())
	var last map[string]any
	for _, rec := range recs {
		if rec["kind"] == "call" && rec["same_as"] == nil {
			last = rec
		}
	}
	if res.SpentUSD-last["cost_usd"].(float64)+worst > worst+0.03 {
		t.Errorf("the last call was started past the cap")
	}
	if end := recs[len(recs)-1]; end["stopped"] != res.Stopped {
		t.Errorf("the log ends %v", end)
	}
}

// A run whose context ends stops before the next call.
func TestInterrupted(t *testing.T) {
	r, err := measure.New("test", sheets(t), measure.Judges(), 1)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var log bytes.Buffer
	if res, err := r.Do(ctx, measure.Stub, 10, &log, clock); err != nil || res.Calls != 0 || res.Stopped != "interrupted" {
		t.Errorf("an interrupted run ended %+v, %v", res, err)
	}
}

// A failed call is logged with its error, and its tokens are billed; a call whose bill is not
// known counts at its worst. An answer off the schema is logged with the reader's error.
func TestFailures(t *testing.T) {
	opus := judgeNamed("opus-high")
	r, err := measure.New("test", sheets(t)[:1], []measure.Judge{opus}, 1)
	if err != nil {
		t.Fatal(err)
	}
	broke := func(_ context.Context, q measure.Question) (provider.Response, error) {
		if q.Form == judge.FormA {
			return provider.Response{InputTokens: 1000, OutputTokens: 20, Partial: true}, errors.New("the stream broke")
		}
		return provider.Response{JSON: `{"verdict": "so_far_correct", "box": null}`, InputTokens: 1000, OutputTokens: 10}, nil
	}
	var log bytes.Buffer
	res, err := r.Do(context.Background(), broke, 10, &log, clock)
	if err != nil {
		t.Fatal(err)
	}
	worst := (8000*4 + 16000*20) / 1e6
	if res.Calls != 2 || !near(res.SpentUSD, worst+(1000*4+10*20)/1e6) {
		t.Errorf("the run ended %+v, want two calls, one at its worst", res)
	}
	recs := records(t, log.Bytes())
	for _, rec := range recs {
		if rec["kind"] != "call" || rec["same_as"] != nil {
			continue
		}
		switch msg, _ := rec["error"].(string); rec["form"] {
		case "A":
			if !strings.Contains(msg, "the stream broke") || !strings.Contains(msg, "the most it could cost") {
				t.Errorf("the broken call is logged %v", rec)
			}
		case "B":
			if !strings.Contains(msg, "no such verdict") || rec["answer"] == nil {
				t.Errorf("the answer off the schema is logged %v", rec)
			}
		}
	}
}

// The stub has no verdict for a sheet without an answer: its call fails, and the run goes on.
func TestStubWithoutVerdict(t *testing.T) {
	s := synth.Sample(sample.ScenarioNearlyEmpty)
	bank := synth.Bank()
	r, err := measure.New("test", []measure.Sheet{{Number: 1, File: "x.json", Sample: s, Task: &bank.Tasks[0]}}, []measure.Judge{judgeNamed("opus-high")}, 1)
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	res, err := r.Do(context.Background(), measure.Stub, 10, &log, clock)
	if err != nil || res.Calls != 2 || res.Stopped != "" {
		t.Fatalf("the run ended %+v, %v", res, err)
	}
	for _, rec := range records(t, log.Bytes()) {
		if msg, _ := rec["error"].(string); rec["kind"] == "call" && rec["same_as"] == nil && !strings.Contains(msg, "no verdict for a sheet of the outcome") {
			t.Errorf("the call is logged %v", rec)
		}
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
	r, err := measure.New("test", sheets(t), measure.Judges(), 1)
	if err != nil {
		t.Fatal(err)
	}
	res, err := r.Do(context.Background(), measure.Stub, 10, &failing{after: 9}, clock)
	if err == nil || res.Calls > 2 {
		t.Errorf("a run with a failing log ended %+v, %v", res, err)
	}
}
