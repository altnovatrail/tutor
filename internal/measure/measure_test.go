package measure_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"slices"
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
	r, err := measure.New("test", all[:1], []measure.Judge{judgeNamed("opus-high")}, 1, judge.V1)
	if err != nil {
		t.Fatal(err)
	}
	if e := r.Estimate(); e.Calls != 2 || !near(e.USD, 0.0084+0.0100) || !near(e.WorstUSD, 2*(8000*4+16000*20)/1e6) {
		t.Errorf("the estimate of one sheet without frames by Opus is %+v, want 2 calls, $0.0184 and at most $0.704", e)
	}
	// All seven sheets by the three judges, twice: A and B of every sheet, C and D of the two with
	// frames; the D of the other five is their B.
	r, err = measure.New("test", all, measure.Judges(), 2, judge.V1)
	if err != nil {
		t.Fatal(err)
	}
	if e := r.Estimate(); e.Calls != 2*3*(5*2+2*4) {
		t.Errorf("the run makes %d calls, want %d", e.Calls, 2*3*(5*2+2*4))
	}
	if _, err := measure.New("test", all, measure.Judges(), 0, judge.V1); err == nil {
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
	r, err := measure.New("test", all, measure.Judges(), 1, judge.V1)
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
	if recs[0]["prompt"] != judge.Fingerprint(judge.V1) {
		t.Errorf("the run's record names the prompt %v, want its fingerprint %s", recs[0]["prompt"], judge.Fingerprint(judge.V1))
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
	r, err := measure.New("test", sheets(t), []measure.Judge{opus}, 1, judge.V1)
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
	r, err := measure.New("test", sheets(t), measure.Judges(), 1, judge.V1)
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
	r, err := measure.New("test", sheets(t)[:1], []measure.Judge{opus}, 1, judge.V1)
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
	r, err := measure.New("test", []measure.Sheet{{Number: 1, File: "x.json", Sample: s, Task: &bank.Tasks[0]}}, []measure.Judge{judgeNamed("opus-high")}, 1, judge.V1)
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
	r, err := measure.New("test", sheets(t), measure.Judges(), 1, judge.V1)
	if err != nil {
		t.Fatal(err)
	}
	res, err := r.Do(context.Background(), measure.Stub, 10, &failing{after: 9}, clock)
	if err == nil || res.Calls > 2 {
		t.Errorf("a run with a failing log ended %+v, %v", res, err)
	}
}

// A run's log reads back as it was written, and a log whose last line was cut off half written
// reads without that line; a log out of order, of an unknown kind or empty does not.
func TestReadLog(t *testing.T) {
	r, err := measure.New("test", sheets(t), measure.Judges(), 1, judge.V1)
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	res, err := r.Do(context.Background(), measure.Stub, 10, &log, clock)
	if err != nil {
		t.Fatal(err)
	}
	l, err := measure.ReadLog(bytes.NewReader(log.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if l.Run.Part != "test" || len(l.Run.Judges) != 3 || len(l.Sheets) != 7 || l.End == nil || l.End.Calls != res.Calls || l.Run.Prompt != judge.Fingerprint(judge.V1) {
		t.Errorf("the log reads back as %+v with %d sheets and %d calls", l.Run, len(l.Sheets), len(l.Calls))
	}
	made := 0
	for _, c := range l.Calls {
		if c.SameAs == "" {
			made++
		}
	}
	if made != res.Calls {
		t.Errorf("%d calls read back, %d made", made, res.Calls)
	}
	lines := strings.SplitAfter(log.String(), "\n")
	for name, text := range map[string]string{
		"empty":           "",
		"a sheet first":   lines[1],
		"after the end":   log.String() + lines[1],
		"an unknown kind": lines[0] + `{"kind": "pause"}` + "\n" + lines[1],
		"not JSON":        lines[0] + "a pause\n" + lines[1],
	} {
		if _, err := measure.ReadLog(strings.NewReader(text)); err == nil {
			t.Errorf("%s: read", name)
		}
	}
	cut, err := measure.ReadLog(strings.NewReader(lines[0] + lines[1] + lines[2][:10]))
	if err != nil || len(cut.Sheets) != 1 || cut.End != nil {
		t.Errorf("a log cut off in its third line reads as %+v, %v", cut, err)
	}
}

// freeSheets are sheets of the plan of free writing, drawn with seed 1, by their numbers, with
// their sheets of the plan.
func freeSheets(t *testing.T, numbers ...int) []measure.Sheet {
	t.Helper()
	open := func(path string) *os.File {
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = f.Close() })
		return f
	}
	plan, err := sample.ReadPlan(open("../../eval/free-sheets.json"))
	if err != nil {
		t.Fatal(err)
	}
	bank, err := sample.ReadBank(open("../../eval/free-bank.json"))
	if err != nil {
		t.Fatal(err)
	}
	var out []measure.Sheet
	for _, n := range numbers {
		sh := &plan.Sheets[n-1]
		s := synth.Sheet(sh, 1)
		i := slices.IndexFunc(bank.Tasks, func(task sample.Task) bool { return task.ID == s.Task })
		out = append(out, measure.Sheet{Number: n, File: fmt.Sprintf("%02d.json", n), Sample: s, Task: &bank.Tasks[i], Plan: sh})
	}
	return out
}

// A run of free writing asks in version 2 of the prompt: its estimate is of version 2, and its log
// tells the version, and of each sheet who wrote it, its layout and where its first error stands.
// The stub's box on a sheet with a mistake hits the part of the first error.
func TestFreeWritingRun(t *testing.T) {
	// Sheet 1, grasshopper, correct; sheet 2, album, its first error in part 2, at the side;
	// sheet 3, houses, erasure-fixed, with a frame (docs/free-sheets.md).
	free := freeSheets(t, 1, 2, 3)
	opus := judgeNamed("opus-high")
	r, err := measure.New("all", free[:1], []measure.Judge{opus}, 1, judge.V2)
	if err != nil {
		t.Fatal(err)
	}
	// Opus 5.5: 2 050 tokens in and 50 out for form A, and 360 more in with the journal.
	if e := r.Estimate(); e.Calls != 2 || !near(e.USD, (2050*4+50*20)/1e6+(2410*4+50*20)/1e6) {
		t.Errorf("the estimate of one sheet without frames by Opus in version 2 is %+v", e)
	}
	r, err = measure.New("all", free, []measure.Judge{opus}, 1, judge.V2)
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	if _, err := r.Do(context.Background(), measure.Stub, 10, &log, clock); err != nil {
		t.Fatal(err)
	}
	l, err := measure.ReadLog(bytes.NewReader(log.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if l.Run.PromptVersion != judge.V2 || measure.PromptOf(l.Run) != judge.V2 || l.Run.Prompt != judge.Fingerprint(judge.V2) || l.Run.Part != "all" {
		t.Errorf("the run is %+v, want version 2 of the prompt and its fingerprint", l.Run)
	}
	want := []struct {
		layout sample.Layout
		first  int
		in     sample.Region
		frames int
	}{{sample.LayoutNumberLine, 0, "", 0}, {sample.LayoutSide, 2, sample.RegionSide, 0}, {sample.LayoutRow, 0, "", 1}}
	for i, sh := range l.Sheets {
		if w := want[i]; sh.Writer != sample.WriterSynthetic || sh.Layout != w.layout || sh.FirstError != w.first || sh.FirstErrorIn != w.in || sh.Frames != w.frames {
			t.Errorf("sheet %d is %+v, want %+v", sh.Sheet, sh, w)
		}
	}
	for _, c := range l.Calls {
		if c.Sheet == 2 && c.SameAs == "" && (c.Verdict != judge.Error || c.Hit != 2) {
			t.Errorf("the stub on sheet 2 in form %s says %q and hits part %d, want error on part 2", c.Form, c.Verdict, c.Hit)
		}
	}
	if _, err := measure.New("all", free, []measure.Judge{opus}, 1, 3); err == nil {
		t.Error("a run is planned in a version 3 of the prompt")
	}

	// A sheet marked off its plan, its first error on another part, has its layout, and no place
	// of the first error: the plan does not tell it.
	off := freeSheets(t, 2)
	first := 4
	off[0].Sample.Expected.FirstError = &first
	r, err = measure.New("all", off, []measure.Judge{opus}, 1, judge.V2)
	if err != nil {
		t.Fatal(err)
	}
	log.Reset()
	if _, err := r.Do(context.Background(), measure.Stub, 10, &log, clock); err != nil {
		t.Fatal(err)
	}
	if l, err = measure.ReadLog(bytes.NewReader(log.Bytes())); err != nil || l.Sheets[0].Layout != sample.LayoutSide || l.Sheets[0].FirstErrorIn != "" || l.Sheets[0].FirstError != 4 {
		t.Errorf("a sheet marked off its plan is %+v, %v", l.Sheets, err)
	}
}
