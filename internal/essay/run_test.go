package essay

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/altnovatrail/tutor/internal/runner"
)

// A run asks every reader of every essay of its part in every repeat, Opus too, and draws each essay
// once. A run of no repeat or of no essay is not planned.
func TestNew(t *testing.T) {
	es := readEssays(t)
	readers := Readers()
	for _, c := range []struct {
		part          string
		essays, calls int
		repeats       int
	}{
		{"dev", 5, 5 * 3 * 1, 1},
		{"test", 15, 15 * 3 * 3, 3},
		{"all", 20, 20 * 3 * 2, 2},
	} {
		essays := Essays(es, c.part)
		r, err := New(c.part, "eval/essays.json", 1, es, essays, readers, c.repeats)
		if err != nil {
			t.Fatal(err)
		}
		if len(essays) != c.essays || len(r.Items) != c.essays || len(r.Steps) != c.calls {
			t.Errorf("%s: %d essays, %d items and %d calls, want %d and %d", c.part, len(essays), len(r.Items), len(r.Steps), c.essays, c.calls)
		}
	}
	if _, err := New("dev", "", 1, es, nil, readers, 1); err == nil {
		t.Error("a run of no essay is planned")
	}
	if _, err := New("dev", "", 1, es, Essays(es, "dev"), readers, 0); err == nil {
		t.Error("a run of no repeat is planned")
	}
}

// On the stub, a run goes the whole way: its log reads back, of the kind essay, with the run, the
// essays and every call, and every call comes to the planned truth, reads the essay as written and
// keeps the rule of the feedback.
func TestRunOnTheStub(t *testing.T) {
	es := readEssays(t)
	r, err := New("dev", "eval/essays.json", 1, es, Essays(es, "dev"), Readers(), 2)
	if err != nil {
		t.Fatal(err)
	}
	clock := func() time.Time { return time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC) }
	var log bytes.Buffer
	res, err := r.Do(context.Background(), runner.Stub, 10, &log, clock)
	if err != nil {
		t.Fatal(err)
	}
	if e := r.Estimate(); res.Calls != e.Calls || res.Stopped != "" {
		t.Errorf("the run ended %+v, want all %d calls", res, e.Calls)
	}
	if kind := runner.CallKind(log.Bytes()); kind != CallKind {
		t.Errorf("the log is of the kind %q", kind)
	}
	l, err := ReadLog(&log)
	if err != nil {
		t.Fatal(err)
	}
	if l.Run.Prompt != Fingerprint() || len(l.Run.Rubric) != 6 || len(l.Sheets) != 5 || len(l.Calls) != res.Calls || l.End == nil {
		t.Fatalf("the log holds %+v, %d essays and %d calls", l.Run, len(l.Sheets), len(l.Calls))
	}
	unmet := map[int][]string{}
	for _, e := range l.Sheets {
		unmet[e.Essay] = e.Unmet
	}
	for _, c := range l.Calls {
		if c.Error != "" || len(c.Marks) != 6 || c.Reading == nil || c.Reading.WordErrors != 0 || len(c.Breaches) != 0 || c.FeedbackWords == 0 {
			t.Fatalf("essay %d, %s: %+v", c.Essay, c.Reader, c)
		}
		for _, m := range c.Marks {
			met := true
			for _, id := range unmet[c.Essay] {
				met = met && id != m.ID
			}
			if m.Met == nil || *m.Met != met {
				t.Errorf("essay %d, %s: %s is %+v", c.Essay, c.Reader, m.ID, m)
			}
		}
	}
	if _, err := ReadLog(bytes.NewReader([]byte(`{"kind":"run","call_kind":"workbook"}` + "\n"))); err == nil {
		t.Error("a log of the workbook reads as the essays'")
	}
}

// A call that failed, or whose answer is off the schema, leaves every criterion unchecked and has no
// reading and no feedback; one not asked says so.
func TestRecordOfFailures(t *testing.T) {
	es := readEssays(t)
	r, err := New("dev", "", 1, es, Essays(es, "dev")[:1], Readers()[:1], 1)
	if err != nil {
		t.Fatal(err)
	}
	st := r.Steps[0].Call
	for _, o := range []runner.Outcome{{Error: "timeout"}, {Answer: `{"text": "x"}`}, {NotAsked: true}} {
		rec := st.Record(o).(CallRecord)
		if rec.Error == "" && !o.NotAsked || rec.NotAsked != o.NotAsked || rec.Reading != nil || len(rec.Marks) != 6 || rec.Marks[0].Met != nil {
			t.Errorf("%+v comes to %+v", o, rec)
		}
	}
}
