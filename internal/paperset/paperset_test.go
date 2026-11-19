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
// the case: the statement's crop for the statement, the solution and the finding, the work's for the
// reading. The cases of several tasks go to every run, and to the finding alone.
func TestStubIsRight(t *testing.T) {
	f := setOf(t, 30)
	several := 0
	for _, c := range f.Set.Cases {
		if c.Kind == paperset.KindSeveral {
			several++
		}
	}
	for _, kind := range paperset.CallKinds {
		r, err := paperset.New(kind, "all", f, haiku(t), 1)
		if err != nil {
			t.Fatal(err)
		}
		want := map[bool]int{true: several, false: 30}[kind == paperset.CallFinding]
		if want == 0 || len(r.Steps) != want || len(r.Items) != want {
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

// one is the answer of the statement of one task with a formula, and solved one of the solution
// of one task by its kind and its fields.
func one(formula string) string {
	return `{"tasks": [{"statement": "a task", "formula": "` + formula + `"}]}`
}

func solved(kind, value, options, key string) string {
	return `{"answers": [{"task": 1, "kind": "` + kind + `", "value": "` + value + `", "units": [], "unknown": "", "options": [` + options + `], "key": "` + key + `"}]}`
}

// A model's mistakes are counted wrong: a formula read of a word problem, or one another than the
// truth's, or another count of tasks; no key of a task with one, or another value; and a reading of
// another value. A case of several tasks is right only when each of its tasks is.
func TestMistakesAreWrong(t *testing.T) {
	f := setOf(t, 6)
	findingIsWrong(t, f)
	several := f.Set.Cases[5]
	if several.Kind != paperset.KindSeveral || len(several.Tasks) < 2 || several.Tasks[0].Outcome != paperset.OutcomeCorrect || several.Tasks[1].Outcome != paperset.OutcomeNotYet {
		t.Fatalf("case 6 is %+v", several)
	}
	steps := func(kind string) []runner.Step {
		r, err := paperset.New(kind, "all", f, haiku(t), 1)
		if err != nil {
			t.Fatal(err)
		}
		return r.Steps
	}
	st := steps(paperset.CallStatement)
	for i, answer := range map[int]string{0: one("69*42"), 2: one("14+14"), 1: one(""), 4: `{"tasks": []}`} {
		if rec := recordOf(st[i], answer); rec.Right {
			t.Errorf("case %d: %s is right", i+1, answer)
		}
	}
	if rec := recordOf(st[0], `{"tasks": [{"statement": "a task", "formula": 1}]}`); rec.Right || rec.Error == "" {
		t.Errorf("off the schema: %+v", rec)
	}
	// Several tasks: each formula as the truth's, one fewer, or one another.
	formulas := func(of []paperset.Task) string {
		var tasks []map[string]string
		for _, task := range of {
			tasks = append(tasks, map[string]string{"statement": task.Statement, "formula": task.Formula})
		}
		data, _ := json.Marshal(map[string]any{"tasks": tasks})
		return string(data)
	}
	if rec := recordOf(st[5], formulas(several.Tasks)); !rec.Right || len(rec.Formulas) != len(several.Tasks) || rec.Solved[0] != several.Tasks[0].Key.Number {
		t.Errorf("several tasks: %+v", rec)
	}
	other := append([]paperset.Task(nil), several.Tasks...)
	other[1].Formula = "x+1=100"
	for name, answer := range map[string]string{"one fewer": formulas(several.Tasks[1:]), "one another": formulas(other)} {
		if rec := recordOf(st[5], answer); rec.Right {
			t.Errorf("several tasks, %s: %+v", name, rec)
		}
	}

	so := steps(paperset.CallSolving)
	none := solved("none", "", "", "")
	for i, answer := range map[int]string{0: none, 2: solved("number", "1", "", ""), 3: solved("choice", "", `"A", "B"`, "A")} {
		if rec := recordOf(so[i], answer); rec.Right {
			t.Errorf("case %d: %s is right", i+1, answer)
		}
	}
	if rec := recordOf(so[4], none); !rec.Right || len(rec.Keys) != 1 || rec.Keys[0] != nil {
		t.Errorf("an open task with no key: %+v", rec)
	}
	// Several tasks are right with the key of each, and wrong with one missing.
	right, err := so[5].Call.Right()
	if err != nil {
		t.Fatal(err)
	}
	if rec := recordOf(so[5], right); !rec.Right || len(rec.Keys) != len(several.Tasks) || rec.Keys[1].Number != several.Tasks[1].Key.Number {
		t.Errorf("several tasks: %+v", rec)
	}
	if rec := recordOf(so[5], solved("number", several.Tasks[0].Key.Number, "", "")); rec.Right || rec.Keys[0] == nil || rec.Keys[1] != nil {
		t.Errorf("several tasks, one solved: %+v", rec)
	}

	pa := steps(paperset.CallPaper)
	wrong := `{"parts": [{"kind": "text", "text": "7", "legible": true, "other_readings": []}], "results": [{"task": 1, "part": 1, "box": [], "value": "7", "other_values": []}]}`
	if rec := recordOf(pa[0], wrong); rec.Right || len(rec.Verdicts) != 1 || rec.Verdicts[0] != paperset.OutcomeNotYet {
		t.Errorf("another value: %+v", rec)
	}
	if rec := recordOf(pa[0], `{}`); rec.Right || rec.Error == "" {
		t.Errorf("off the schema: %+v", rec)
	}
	// Several tasks are right with each read as written, and wrong with the second not read.
	if right, err = pa[5].Call.Right(); err != nil {
		t.Fatal(err)
	}
	if rec := recordOf(pa[5], right); !rec.Right || len(rec.Verdicts) != len(several.Tasks) || rec.Verdicts[1] != paperset.OutcomeNotYet {
		t.Errorf("several tasks: %+v", rec)
	}
	first := `{"parts": [{"kind": "text", "text": "` + several.Tasks[0].Work + `", "legible": true, "other_readings": []}], "results": [{"task": 1, "part": 1, "box": [10, 10, 90, 90], "value": "` + several.Tasks[0].Read + `", "other_values": []}]}`
	if rec := recordOf(pa[5], first); rec.Right || rec.Verdicts[0] != paperset.OutcomeCorrect || rec.Verdicts[1] != paperset.OutcomeUnreadable {
		t.Errorf("several tasks, the second not read: %+v", rec)
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
			answer = solved("none", "", "", "")
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
	if last := rows[len(rows)-1]; last.Kind != "all" || last.Calls != 10 || last.Right != 9 || len(rows) != 7 {
		t.Errorf("the rows are %+v", rows)
	}
	if !strings.Contains(rep.Summary(), "9 of 10 answered right, 90.0%") {
		t.Errorf("the summary is %q", rep.Summary())
	}
	var md, csv bytes.Buffer
	if rep.Markdown(&md, "x.jsonl") != nil || rep.CSV(&csv) != nil || !strings.Contains(md.String(), "| haiku-medium | expression | 2 | 2 | 1 |") || strings.Count(csv.String(), "\n") != 8 {
		t.Errorf("the report is\n%s\n%s", md.String(), csv.String())
	}
}

// Read takes a set as gen writes it, and refuses a case that breaks its form.
func TestRead(t *testing.T) {
	f := setOf(t, 6)
	dir := t.TempDir()
	write := func(s any) string {
		data, _ := json.Marshal(s)
		path := filepath.Join(dir, "set.json")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	if s, err := paperset.Read(write(f.Set)); err != nil || len(s.Cases) != 6 || len(s.Of("dev")) != 1 || len(s.Of("test")) != 5 {
		t.Fatalf("read %v, %v", s, err)
	}
	breaks := map[string]func(c *paperset.Case){
		"the number":   func(c *paperset.Case) { c.Number = 9 },
		"the kind":     func(c *paperset.Case) { c.Kind = "proof" },
		"the part":     func(c *paperset.Case) { c.Part = "train" },
		"the outcome":  func(c *paperset.Case) { c.Tasks[0].Outcome = "fine" },
		"a key":        func(c *paperset.Case) { c.Tasks[0].Key = nil },
		"the formula":  func(c *paperset.Case) { c.Tasks[0].Formula = "" },
		"the read":     func(c *paperset.Case) { c.Tasks[0].Read = "" },
		"no statement": func(c *paperset.Case) { c.Statement = nil },
		"no task's":    func(c *paperset.Case) { c.Tasks[0].Statement = "" },
		"no task":      func(c *paperset.Case) { c.Tasks = nil },
		"two tasks":    func(c *paperset.Case) { c.Tasks = append(c.Tasks, c.Tasks[0]) },
		"several, one": func(c *paperset.Case) { c.Kind = paperset.KindSeveral },
	}
	for name, change := range breaks {
		s := *f.Set
		s.Cases = append([]paperset.Case(nil), f.Set.Cases...)
		s.Cases[0].Tasks = append([]paperset.Task(nil), s.Cases[0].Tasks...)
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

// The finding of the tasks of a case of several is right only with a box round each, apart and on
// the picture: one box round two examples, two round one, or one off the picture is wrong. The
// request goes as data with the request every case of the finding sends.
func findingIsWrong(t *testing.T, f *paperset.File) {
	t.Helper()
	r, err := paperset.New(paperset.CallFinding, "all", f, haiku(t), 1)
	if err != nil || len(r.Steps) != 1 {
		t.Fatalf("the finding: %v, %v", r, err)
	}
	st := r.Steps[0]
	req, err := st.Call.Request()
	if err != nil || !strings.Contains(req.Text, "<request>") || !strings.Contains(req.Text, paperset.FindingRequest) {
		t.Fatalf("the request of the finding: %q, %v", req.Text, err)
	}
	var right struct {
		Tasks []map[string]int `json:"tasks"`
	}
	answer, err := st.Call.Right()
	if err != nil || json.Unmarshal([]byte(answer), &right) != nil || len(right.Tasks) < 2 {
		t.Fatalf("the stub's finding: %s, %v", answer, err)
	}
	box := func(b map[string]int) string {
		data, _ := json.Marshal(b)
		return string(data)
	}
	first, second := right.Tasks[0], right.Tasks[1]
	both := map[string]int{"x1": first["x1"], "y1": first["y1"], "x2": second["x2"], "y2": second["y2"]}
	off := map[string]int{"x1": first["x1"], "y1": first["y1"], "x2": first["x2"] + 1, "y2": first["y2"]}
	rest := []string{}
	for _, b := range right.Tasks[2:] {
		rest = append(rest, box(b))
	}
	tail := strings.Join(rest, ",")
	if tail != "" {
		tail = "," + tail
	}
	statement := map[string]int{"x1": first["x1"], "y1": 0, "x2": first["x2"], "y2": first["y1"] - 1}
	for name, answer := range map[string]string{
		"the statement, not the first": `{"tasks": [` + box(statement) + "," + box(second) + tail + `]}`,
		"one round two":                `{"tasks": [` + box(both) + tail + `]}`,
		"two round one":                `{"tasks": [` + box(first) + "," + box(first) + tail + `]}`,
		"off the picture":              `{"tasks": [` + box(off) + "," + box(second) + tail + `]}`,
		"none":                         `{"tasks": []}`,
	} {
		if rec := recordOf(st, answer); rec.Right {
			t.Errorf("%s: %s is right", name, answer)
		}
	}
}
