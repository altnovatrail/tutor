package paperset_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/internal/gen"
	"github.com/altnovatrail/tutor/internal/paperset"
	"github.com/altnovatrail/tutor/internal/runner"
)

func setOf(t *testing.T, n int) *paperset.File {
	t.Helper()
	s, err := gen.Paper(n, 7)
	if err != nil {
		t.Fatal(err)
	}
	return &paperset.File{Name: "paper.json", SHA256: "0", Set: s}
}

func haiku(t *testing.T) []runner.Config {
	t.Helper()
	configs, err := runner.Chosen(runner.Configs(), func(c runner.Config) runner.Config { return c }, []string{"haiku-medium"})
	if err != nil {
		t.Fatal(err)
	}
	return configs
}

// recordOf is the record of a call that answered answer.
func recordOf(st runner.Step, answer string) paperset.CallRecord {
	return st.Call.Record(runner.Outcome{Answer: answer}).(paperset.CallRecord)
}

// The answer of the stub is right on every case of every kind, and the run's pictures are those of
// the case: the statement's crop for the statement and the solution, the work's for the reading.
func TestStubIsRight(t *testing.T) {
	f := setOf(t, 30)
	for _, kind := range paperset.CallKinds {
		r, err := paperset.New(kind, "all", f, haiku(t), 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(r.Steps) != 30 || len(r.Items) != 30 {
			t.Fatalf("%s: %d steps, %d items", kind, len(r.Steps), len(r.Items))
		}
		for _, st := range r.Steps {
			right, err := st.Call.Right()
			if err != nil {
				t.Fatal(err)
			}
			req, err := st.Call.Request()
			if err != nil || len(req.Images) != 1 {
				t.Fatalf("%s: the request %+v, %v", kind, req, err)
			}
			if rec := recordOf(st, right); !rec.Right || rec.Error != "" {
				t.Errorf("%s: the stub's %s is %+v", kind, right, rec)
			}
		}
	}
}

// A model's mistakes are counted wrong: a formula read of a word problem, or one another than the
// truth's; no key of a task with one, or another value; and a reading of another value.
func TestMistakesAreWrong(t *testing.T) {
	f := setOf(t, 5)
	steps := func(kind string) []runner.Step {
		r, err := paperset.New(kind, "all", f, haiku(t), 1)
		if err != nil {
			t.Fatal(err)
		}
		return r.Steps
	}
	st := steps(paperset.CallStatement)
	for i, answer := range map[int]string{0: `{"formula": "69*42"}`, 2: `{"formula": "14+14"}`, 1: `{"formula": ""}`} {
		if rec := recordOf(st[i], answer); rec.Right {
			t.Errorf("case %d: %s is right", i+1, answer)
		}
	}
	if rec := recordOf(st[0], `{"formula": 1}`); rec.Right || rec.Error == "" {
		t.Errorf("off the schema: %+v", rec)
	}
	so := steps(paperset.CallSolving)
	none := `{"kind": "none", "value": "", "units": [], "unknown": "", "options": [], "key": ""}`
	for i, answer := range map[int]string{0: none, 2: `{"kind": "number", "value": "1", "units": [], "unknown": "", "options": [], "key": ""}`, 3: `{"kind": "choice", "value": "", "units": [], "unknown": "", "options": ["A", "B"], "key": "A"}`} {
		if rec := recordOf(so[i], answer); rec.Right {
			t.Errorf("case %d: %s is right", i+1, answer)
		}
	}
	if rec := recordOf(so[4], none); !rec.Right || rec.Key != nil {
		t.Errorf("an open task with no key: %+v", rec)
	}
	pa := steps(paperset.CallPaper)
	wrong := `{"parts": [{"kind": "text", "text": "7", "legible": true, "other_readings": []}], "results": [{"part": 1, "value": "7", "other_values": []}]}`
	if rec := recordOf(pa[0], wrong); rec.Right || rec.Verdict != paperset.OutcomeNotYet {
		t.Errorf("another value: %+v", rec)
	}
	if rec := recordOf(pa[0], `{}`); rec.Right || rec.Error == "" {
		t.Errorf("off the schema: %+v", rec)
	}
}

// The report counts the calls of each configuration by the kinds of case.
func TestReport(t *testing.T) {
	f := setOf(t, 10)
	r, err := paperset.New(paperset.CallSolving, "all", f, haiku(t), 1)
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	_ = enc.Encode(r.Record(runner.Head{}))
	for _, item := range r.Items {
		_ = enc.Encode(item)
	}
	for i, st := range r.Steps {
		answer, _ := st.Call.Right()
		if i == 0 {
			answer = `{"kind": "none", "value": "", "units": [], "unknown": "", "options": [], "key": ""}`
		}
		_ = enc.Encode(st.Call.Record(runner.Outcome{Answer: answer}))
	}
	l, err := paperset.ReadLog(&b, paperset.CallSolving)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := paperset.NewReport(l)
	if err != nil {
		t.Fatal(err)
	}
	rows := rep.Rows()
	if last := rows[len(rows)-1]; last.Kind != "all" || last.Calls != 10 || last.Right != 9 || len(rows) != 6 {
		t.Errorf("the rows are %+v", rows)
	}
	if !strings.Contains(rep.Summary(), "9 of 10 answered right, 90.0%") {
		t.Errorf("the summary is %q", rep.Summary())
	}
	var md, csv bytes.Buffer
	if rep.Markdown(&md, "x.jsonl") != nil || rep.CSV(&csv) != nil || !strings.Contains(md.String(), "| haiku-medium | expression | 2 | 2 | 1 |") || strings.Count(csv.String(), "\n") != 7 {
		t.Errorf("the report is\n%s\n%s", md.String(), csv.String())
	}
}

// Read takes a set as gen writes it, and refuses a case that breaks its form.
func TestRead(t *testing.T) {
	f := setOf(t, 5)
	dir := t.TempDir()
	write := func(s any) string {
		data, _ := json.Marshal(s)
		path := filepath.Join(dir, "set.json")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	if s, err := paperset.Read(write(f.Set)); err != nil || len(s.Cases) != 5 || len(s.Of("dev")) != 1 || len(s.Of("test")) != 4 {
		t.Fatalf("read %v, %v", s, err)
	}
	breaks := map[string]func(c *paperset.Case){
		"the number":   func(c *paperset.Case) { c.Number = 9 },
		"the kind":     func(c *paperset.Case) { c.Kind = "proof" },
		"the part":     func(c *paperset.Case) { c.Part = "train" },
		"the outcome":  func(c *paperset.Case) { c.Outcome = "fine" },
		"a key":        func(c *paperset.Case) { c.Key = nil },
		"the formula":  func(c *paperset.Case) { c.Formula = "" },
		"the read":     func(c *paperset.Case) { c.Read = "" },
		"no statement": func(c *paperset.Case) { c.Statement = nil },
	}
	for name, change := range breaks {
		s := *f.Set
		s.Cases = append([]paperset.Case(nil), f.Set.Cases...)
		change(&s.Cases[0])
		if _, err := paperset.Read(write(&s)); err == nil {
			t.Errorf("%s: read", name)
		}
	}
	if _, err := paperset.Read(write(map[string]any{"cases": []any{}})); err == nil {
		t.Error("no case: read")
	}
	if _, err := paperset.Read(write(map[string]any{"cases": 1})); err == nil {
		t.Error("not a set: read")
	}
}
