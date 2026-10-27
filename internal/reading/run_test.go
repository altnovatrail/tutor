package reading_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/altnovatrail/tutor/internal/reading"
	"github.com/altnovatrail/tutor/internal/runner"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/sheets"
)

// chosen are sheets of a plan as a part of a run takes them, of a set by its name.
func chosen(t *testing.T, set, planPath, bankPath string, numbers ...int) []sheets.Chosen {
	t.Helper()
	all := plan(t, planPath, bankPath)
	var out []sheets.Chosen
	for _, n := range numbers {
		p := all[n-1]
		out = append(out, sheets.Chosen{Set: set, Number: n, File: fmt.Sprintf("%02d.json", n), Sample: p.sample, Task: p.task, Plan: p.plan})
	}
	return out
}

func clock() time.Time { return time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC) }

// A run reads every sheet without a key and with the right key, and a correct sheet with each false
// key of its task too; Opus reads once and Haiku in every repeat, repeat by repeat. The estimate
// takes the tokens of a reading.
func TestNew(t *testing.T) {
	// Sheet 1 of the plan of free writing is correct, sheet 2 has a mistake (docs/free-sheets.md).
	free := chosen(t, "free", "../../eval/free-sheets.json", "../../eval/free-bank.json", 1, 2)
	r, err := reading.New("dev", []string{"free"}, free, reading.Readers(), 2)
	if err != nil {
		t.Fatal(err)
	}
	// Per repeat and Haiku: 3 conditions on sheet 1, 2 on sheet 2; Opus once.
	if want := 2*2*(3+2) + 1*(3+2); len(r.Steps) != want {
		t.Errorf("the run makes %d calls, want %d", len(r.Steps), want)
	}
	var order []string
	for _, st := range r.Steps {
		if st.WorstIn != 8000 || st.Call == nil {
			t.Errorf("a step %+v", st)
		}
		rec := st.Call.Record(runner.Outcome{NotAsked: true, Error: "no"}).(reading.CallRecord)
		order = append(order, fmt.Sprintf("%d/%d/%s/%s", rec.Repeat, rec.Sheet, rec.Reader, rec.Condition))
		if (rec.Condition == reading.NoKey) != (st.In == 2000) || rec.Condition != reading.NoKey && rec.Key == "" {
			t.Errorf("the call %+v expects %d tokens in", rec.Call, st.In)
		}
	}
	if got := strings.Join(order[:6], " "); got != "1/1/opus-high/none 1/1/opus-high/right 1/1/opus-high/false 1/1/haiku-low/none 1/1/haiku-low/right 1/1/haiku-low/false" {
		t.Errorf("the run starts %s", got)
	}
	if slicesHas(order, "2/1/opus-high/none") {
		t.Error("Opus reads in the second repeat")
	}
	if slicesHas(order, "1/2/haiku-low/false") {
		t.Error("a sheet with a mistake is read with a false key")
	}
	if _, err := reading.New("dev", nil, free, reading.Readers(), 0); err == nil {
		t.Error("a run of no repeats is planned")
	}
}

func slicesHas(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// A run on the stub over two sets goes the whole way: its log tells the kind of call, the sets and
// the prompt, each sheet with its set, key and markup, and each call with the verdict of its
// sheet's outcome, nothing read under the key, and the tokens of the estimate. A log of the judge is
// no log of the reading.
func TestRunOnTheStub(t *testing.T) {
	lines := chosen(t, "lines", "../../eval/sheets.json", "../../eval/bank.json", 1, 7, 21)
	free := chosen(t, "free", "../../eval/free-sheets.json", "../../eval/free-bank.json", 1, 2, 14)
	r, err := reading.New("dev", []string{"lines", "free"}, append(lines, free...), reading.Readers(), 1)
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	res, err := r.Do(context.Background(), runner.Stub, 10, &log, clock)
	if err != nil {
		t.Fatal(err)
	}
	if e := r.Estimate(); res.Calls != e.Calls || res.Stopped != "" {
		t.Errorf("the run ended %+v, want all %d calls", res, e.Calls)
	}
	if kind := runner.CallKind(log.Bytes()); kind != reading.CallKind {
		t.Errorf("the log is of the kind %q", kind)
	}
	l, err := reading.ReadLog(bytes.NewReader(log.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if l.Run.Prompt != reading.Fingerprint() || len(l.Run.Sets) != 2 || len(l.Run.Readers) != 3 || len(l.Sheets) != 6 || l.End == nil {
		t.Errorf("the log reads back as %+v with %d sheets", l.Run, len(l.Sheets))
	}
	outcome := map[string]sample.Outcome{}
	for _, sh := range l.Sheets {
		outcome[fmt.Sprintf("%s/%d", sh.Set, sh.Sheet)] = sh.Outcome
		if sh.Key == "" || len(sh.FalseKeys) == 0 || len(sh.Markup.Parts) == 0 || sh.Markup.Value == "" {
			t.Errorf("the sheet %s/%d is told %+v", sh.Set, sh.Sheet, sh)
		}
	}
	for _, c := range l.Calls {
		want := truth[outcome[fmt.Sprintf("%s/%d", c.Set, c.Sheet)]]
		if c.Verdict != string(want) || c.Error != "" || c.UnderKey.Result || len(c.UnderKey.Parts) > 0 || c.InputTokens == 0 {
			t.Errorf("the call %+v is logged %s (%s), under %+v, error %q", c.Call, c.Verdict, c.Reason, c.UnderKey, c.Error)
		}
	}
	if len(l.Calls) != res.Calls {
		t.Errorf("%d calls read back, %d made", len(l.Calls), res.Calls)
	}
	judge := `{"kind": "run", "part": "test"}` + "\n"
	if _, err := reading.ReadLog(strings.NewReader(judge)); err == nil || !strings.Contains(err.Error(), "not of the reading") {
		t.Errorf("a log of the judge reads as the reading's: %v", err)
	}
}

// An answer off the schema is a failure of the reading: its verdict is unchecked, and the record
// keeps the answer and the error.
func TestOffTheSchema(t *testing.T) {
	free := chosen(t, "free", "../../eval/free-sheets.json", "../../eval/free-bank.json", 1)
	r, err := reading.New("dev", []string{"free"}, free, reading.Readers()[:1], 1)
	if err != nil {
		t.Fatal(err)
	}
	rec := r.Steps[0].Call.Record(runner.Outcome{Answer: `{"verdict": "correct"}`}).(reading.CallRecord)
	if rec.Verdict != "unchecked" || rec.Answer == "" || !strings.Contains(rec.Error, "not an observation") {
		t.Errorf("an answer off the schema is logged %+v", rec)
	}
}
