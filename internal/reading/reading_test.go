package reading_test

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/internal/reading"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/synth"
	"github.com/altnovatrail/tutor/internal/value"
	"github.com/altnovatrail/tutor/provider"
)

var update = flag.Bool("update", false, "write the golden requests anew")

func open(t *testing.T, path string) *os.File {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

// planned is a plan of the measurement and its bank: its sheets drawn by code with seed 1, as
// tutor-eval draw draws them, and each with its task and its sheet of the plan.
type planned struct {
	sample *sample.Sample
	task   *sample.Task
	plan   *sample.PlanSheet
}

func plan(t *testing.T, planPath, bankPath string) []planned {
	t.Helper()
	pl, err := sample.ReadPlan(open(t, planPath))
	if err != nil {
		t.Fatal(err)
	}
	bank, err := sample.ReadBank(open(t, bankPath))
	if err != nil {
		t.Fatal(err)
	}
	var out []planned
	for i := range pl.Sheets {
		sh := &pl.Sheets[i]
		s := synth.Sheet(sh, 1)
		k := slices.IndexFunc(bank.Tasks, func(task sample.Task) bool { return task.ID == s.Task })
		out = append(out, planned{sample: s, task: &bank.Tasks[k], plan: sh})
	}
	return out
}

// every is every sheet the measurement has, with its task: the 40 sheets of lines of E2 and the 40
// of free writing, drawn by code, and the synthetic sample of every scenario, with the synthetic
// bank.
func every(t *testing.T) []planned {
	t.Helper()
	out := plan(t, "../../eval/sheets.json", "../../eval/bank.json")
	out = append(out, plan(t, "../../eval/free-sheets.json", "../../eval/free-bank.json")...)
	bank := synth.Bank()
	for _, s := range synth.Samples() {
		k := slices.IndexFunc(bank.Tasks, func(task sample.Task) bool { return task.ID == s.Task })
		out = append(out, planned{sample: s, task: &bank.Tasks[k]})
	}
	return out
}

// truth is the verdict a reading should come to on a sheet of an outcome.
var truth = map[sample.Outcome]value.Verdict{
	sample.OutcomeCorrect:    value.Correct,
	sample.OutcomeError:      value.NotYet,
	sample.OutcomeUnreadable: value.Unreadable,
	sample.OutcomeNoAnswer:   value.Unreadable,
}

// The reader who is always right comes, on every sheet of the measurement, to the verdict of its
// outcome against the key of its task, and reads nothing under the key in any condition: not under
// the right key nor under any false one. Its observation reads back as it is.
func TestRight(t *testing.T) {
	sheets := every(t)
	if len(sheets) != 80+len(sample.Scenarios) {
		t.Fatalf("%d sheets", len(sheets))
	}
	for _, p := range sheets {
		name := fmt.Sprintf("%s, task %s, %s", p.sample.Scenario, p.task.ID, p.sample.Expected.Outcome)
		m := reading.MarkupOf(p.sample, p.plan)
		right := reading.Right(m)
		if got, reason := reading.Verdict(right, reading.KeyOf(p.task.Key)); got != truth[p.sample.Expected.Outcome] {
			t.Errorf("%s: the right reader comes to %s (%s)", name, got, reason)
		}
		given := []reading.Given{{Key: p.task.Key, Solution: p.task.Solution}}
		for _, f := range p.task.FalseKeys {
			given = append(given, reading.Given{Key: f.Key, Solution: f.Solution})
		}
		for _, g := range given {
			if u := reading.UnderKey(right, m, g); u.Result || len(u.Parts) > 0 {
				t.Errorf("%s: the right reader reads %+v under the key %s", name, u, g.Key)
			}
		}
		data, err := json.Marshal(right)
		if err != nil {
			t.Fatal(err)
		}
		if back, err := reading.Read(string(data)); err != nil || !reflect.DeepEqual(back, right) {
			t.Errorf("%s: the observation reads back as %+v, %v", name, back, err)
		}
	}
}

// Every key and false key of the banks of the measurement, as the check takes it (KeyOf), is a
// value in the notation that its check calls right.
func TestKeys(t *testing.T) {
	banks := []*sample.Bank{synth.Bank()}
	for _, path := range []string{"../../eval/bank.json", "../../eval/free-bank.json"} {
		b, err := sample.ReadBank(open(t, path))
		if err != nil {
			t.Fatal(err)
		}
		banks = append(banks, b)
	}
	for _, b := range banks {
		for _, task := range b.Tasks {
			keys := []string{task.Key}
			for _, f := range task.FalseKeys {
				keys = append(keys, f.Key)
			}
			for _, k := range keys {
				key := reading.KeyOf(k)
				if got := (value.Number{Key: key}).Check(&key); got.Verdict != value.Correct {
					t.Errorf("task %s: the key %q is %q to the check, which gives %+v", task.ID, k, key, got)
				}
			}
		}
	}
}

// The value of a written answer is the first number after its last "=", or after «Ответ:».
func TestValueOf(t *testing.T) {
	for in, want := range map[string]string{
		"x = 2":               "2",
		"x = 13/2":            "13/2",
		"17 + 49 − 28 = 38":   "38",
		"56 : 6 = 9 (ост. 2)": "9",
		"Ответ: 5 столбов":    "5",
		"ответ: в точке 10":   "10",
		"Ответ: −3":           "-3",
		"12":                  "12",
		"3x +":                "3",
		"Ответ: пять":         "",
		"x = 2,5":             "2,5",
	} {
		if got := reading.ValueOf(in); got != want {
			t.Errorf("ValueOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func observation(parts []reading.Part, results ...reading.Result) *reading.Observation {
	for i := range parts {
		if parts[i].OtherReadings == nil {
			parts[i].OtherReadings = []string{}
		}
	}
	for i := range results {
		if results[i].OtherValues == nil {
			results[i].OtherValues = []string{}
		}
	}
	if results == nil {
		results = []reading.Result{}
	}
	return &reading.Observation{Parts: parts, Results: results}
}

func text(s string, legible bool) reading.Part {
	return reading.Part{Kind: reading.KindText, Text: s, Legible: legible}
}

// The verdict comes from the observation and the value: every reason of an unreadable sheet, and
// the value's check against the task's key.
func TestVerdict(t *testing.T) {
	two := []reading.Part{text("47 + 19 = 66", true), text("66 : 6 = 11", true)}
	for _, c := range []struct {
		name   string
		o      *reading.Observation
		want   value.Verdict
		reason reading.Reason
	}{
		{"a failed call", nil, value.Unchecked, ""},
		{"no answer", observation(two), value.Unreadable, reading.NoResult},
		{"two answers", observation(two, reading.Result{Part: 1, Value: "66"}, reading.Result{Part: 2, Value: "11"}), value.Unreadable, reading.Results},
		{"illegible", observation([]reading.Part{text("66 : 6 = 11", false)}, reading.Result{Part: 1, Value: "11"}), value.Unreadable, reading.Illegible},
		{"another value", observation(two, reading.Result{Part: 2, Value: "11", OtherValues: []string{"17"}}), value.Unreadable, reading.Ambiguous},
		{"blank", observation(two, reading.Result{Part: 2, Value: " = "}), value.Unreadable, reading.Reason(value.Blank)},
		{"not written", observation(two, reading.Result{Part: 2, Value: "12"}), value.Unreadable, reading.NotWritten},
		{"inside a number", observation([]reading.Part{text("x = 13/2", true)}, reading.Result{Part: 1, Value: "2"}), value.Unreadable, reading.NotWritten},
		{"not in notation", observation([]reading.Part{text("Ответ: 1,5", true)}, reading.Result{Part: 1, Value: "1,5"}), value.Unreadable, reading.Reason(value.Notation)},
		{"right", observation(two, reading.Result{Part: 2, Value: "11"}), value.Correct, ""},
		{"right, with the =", observation(two, reading.Result{Part: 2, Value: "= 11."}), value.Correct, ""},
		{"a full stop after", observation([]reading.Part{text("Ответ: 11.", true)}, reading.Result{Part: 1, Value: "11"}), value.Correct, ""},
		{"a comma after", observation([]reading.Part{text("x = 11, y = 3", true)}, reading.Result{Part: 1, Value: "11"}), value.Correct, ""},
		{"after a subtraction", observation([]reading.Part{text("22 − 11", true)}, reading.Result{Part: 1, Value: "11"}), value.Correct, ""},
		{"without its minus", observation([]reading.Part{text("x = −11", true)}, reading.Result{Part: 1, Value: "11"}), value.Unreadable, reading.NotWritten},
		{"the whole of a decimal", observation([]reading.Part{text("x = 11.5", true)}, reading.Result{Part: 1, Value: "11"}), value.Unreadable, reading.NotWritten},
		{"wrong", observation([]reading.Part{text("66 : 6 = 12", true)}, reading.Result{Part: 1, Value: "12"}), value.NotYet, ""},
	} {
		if got, reason := reading.Verdict(c.o, "11"); got != c.want || reason != c.reason {
			t.Errorf("%s: %s (%s), want %s (%s)", c.name, got, reason, c.want, c.reason)
		}
	}
}

// What is read under a key is what the sheet does not have and the given solution or key does: a
// line of writing by its text, a drawing by its numbers, the answer by its value.
func TestUnderKey(t *testing.T) {
	m := reading.Marked{Value: "12", Parts: []reading.MarkedPart{
		{Kind: reading.KindText, Text: "47 + 19 = 56", CrossedOut: true},
		{Kind: reading.KindText, Text: "47 + 19 = 72"},
		{Kind: reading.KindDrawing, Text: "схема: отрезок 6 и отрезок 20"},
		{Kind: reading.KindText, Text: "72 : 6 = 12", Answer: true},
	}}
	right := reading.Given{Key: "11", Solution: []string{"47 + 19 = 66", "рисунок: длиннее на 6, вместе 20", "66 : 6 = 11"}}
	for _, c := range []struct {
		name string
		o    *reading.Observation
		want reading.Under
	}{
		{"as written", observation([]reading.Part{text("47 + 19 = 72", true), {Kind: reading.KindDrawing, Text: "схема: 6 и 20", Legible: true}, text("72:6=12", true)}, reading.Result{Part: 3, Value: "12"}), reading.Under{}},
		{"a line of the solution", observation([]reading.Part{text("47+19 = 66", true)}), reading.Under{Parts: []int{1}}},
		{"the crossed-out line", observation([]reading.Part{text("47 + 19 = 56", true)}), reading.Under{}},
		{"a drawing by the solution's numbers", observation([]reading.Part{{Kind: reading.KindDrawing, Text: "схема: 6 и 20", Legible: true}}), reading.Under{}},
		{"the answer of the key", observation([]reading.Part{text("72 : 6 = 11", true)}, reading.Result{Part: 1, Value: "11"}), reading.Under{Result: true}},
		{"no observation", nil, reading.Under{}},
	} {
		if got := reading.UnderKey(c.o, m, right); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %+v, want %+v", c.name, got, c.want)
		}
	}
	// A drawing read with numbers that the given solution has and the sheet has not is read under it.
	g := reading.Given{Key: "13", Solution: []string{"схема: длиннее на 6, вместе 32"}}
	o := observation([]reading.Part{{Kind: reading.KindDrawing, Text: "отрезки: 6 и 32", Legible: true}})
	if got := reading.UnderKey(o, m, g); !reflect.DeepEqual(got, reading.Under{Parts: []int{1}}) {
		t.Errorf("a drawing under a false key: %+v", got)
	}
}

// The model's answer is read as strictly as the schema.
func TestRead(t *testing.T) {
	good := `{"parts": [{"kind": "text", "text": "x = 2", "legible": true, "other_readings": []}], "results": [{"part": 1, "value": "2", "other_values": []}]}`
	if _, err := reading.Read(good); err != nil {
		t.Fatalf("a good observation: %v", err)
	}
	for name, answer := range map[string]string{
		"not JSON":          `observation`,
		"no results":        `{"parts": []}`,
		"null parts":        `{"parts": null, "results": []}`,
		"an unknown field":  `{"parts": [], "results": [], "verdict": "correct"}`,
		"a part's field":    `{"parts": [{"kind": "text", "text": "x", "legible": true}], "results": []}`,
		"a kind":            `{"parts": [{"kind": "table", "text": "x", "legible": true, "other_readings": []}], "results": []}`,
		"a result's field":  `{"parts": [{"kind": "text", "text": "x", "legible": true, "other_readings": []}], "results": [{"part": 1, "value": "2"}]}`,
		"a part not there":  `{"parts": [{"kind": "text", "text": "x", "legible": true, "other_readings": []}], "results": [{"part": 2, "value": "2", "other_values": []}]}`,
		"a part from zero":  `{"parts": [{"kind": "text", "text": "x", "legible": true, "other_readings": []}], "results": [{"part": 0, "value": "2", "other_values": []}]}`,
		"a value of a type": `{"parts": [], "results": [{"part": 1, "value": 2, "other_values": []}]}`,
		"more after it":     `{"parts": [], "results": []} {"parts": [], "results": []}`,
	} {
		if _, err := reading.Read(answer); err == nil {
			t.Errorf("%s: read", name)
		}
	}
}

// written puts a request in words: its instructions, its picture by its label, and its text.
func written(r provider.Request) string {
	var b strings.Builder
	fmt.Fprintf(&b, "=== instructions\n%s\n", r.Instructions)
	for _, im := range r.Images {
		fmt.Fprintf(&b, "=== picture: the finished sheet\n%s\n", im.Label)
	}
	fmt.Fprintf(&b, "=== text\n%s\n", r.Text)
	return b.String()
}

// The golden requests of the three conditions are the files testdata/lines/*.txt, on sheet 11 of
// the plan of E2, chain-1, correct, and testdata/free/*.txt, on sheet 14 of the plan of free
// writing, stamps, erasure-fixed, with the scheme of segments marked □ and △; the author reads
// them there. The conditions differ only by the block of the key.
func TestGolden(t *testing.T) {
	for _, c := range []struct {
		dir             string
		planPath, banks string
		number          int
	}{
		{"testdata/lines", "../../eval/sheets.json", "../../eval/bank.json", 11},
		{"testdata/free", "../../eval/free-sheets.json", "../../eval/free-bank.json", 14},
	} {
		p := plan(t, c.planPath, c.banks)[c.number-1]
		if p.sample.Expected.Outcome != sample.OutcomeCorrect {
			t.Fatalf("%s: the sheet is not correct, and has no false key", c.dir)
		}
		sh, err := reading.Prepare(p.sample)
		if err != nil {
			t.Fatal(err)
		}
		f := p.task.FalseKeys[0]
		for cond, g := range map[reading.Condition]*reading.Given{
			reading.NoKey:    nil,
			reading.RightKey: {Key: p.task.Key, Solution: p.task.Solution},
			reading.FalseKey: {Key: f.Key, Solution: f.Solution},
		} {
			r := reading.Request(p.task, sh, g)
			if r.SchemaName != "observation" || len(r.Images) != 1 {
				t.Errorf("%s, %s: the request is %+v", c.dir, cond, r)
			}
			got := written(r)
			path := filepath.Join(c.dir, string(cond)+".txt")
			if *update {
				if err := os.MkdirAll(c.dir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				continue
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if got != string(want) {
				t.Errorf("%s: the request is\n%s\nwant, as %s,\n%s", cond, got, path, want)
			}
		}
	}
}

// The fingerprint names the words of the prompt and the schema, and a request with a key differs
// from one without only by the block of the key.
func TestFingerprint(t *testing.T) {
	if f := reading.Fingerprint(); len(f) != 64 || f != reading.Fingerprint() {
		t.Errorf("the fingerprint is %q", f)
	}
	without, with := reading.Instructions(false), reading.Instructions(true)
	if !strings.HasPrefix(with, without+"\n\n") || strings.Contains(without, "key_and_solution") {
		t.Errorf("the instructions with a key are not those without it and the block of the key")
	}
}
