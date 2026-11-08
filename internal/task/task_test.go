package task_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/altnovatrail/module-kit/observation"
	mkview "github.com/altnovatrail/module-kit/openview"
	"github.com/altnovatrail/module-kit/verdict"
	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"
	"github.com/altnovatrail/spec/types/formatv0"

	"github.com/altnovatrail/tutor/internal/protocol"
	"github.com/altnovatrail/tutor/internal/protocol/protocoltest"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/task"
)

// root is the repository's root, from this package.
const root = "../.."

// files are the measurements' files of tasks, by their paths in the repository, with the number of
// tasks each holds: the banks of the sheets, the workbooks and the essays.
var files = map[string]int{
	"eval/bank.json":             10,
	"eval/free-bank.json":        6,
	"eval/near-bank.json":        6,
	"eval/near-gen-bank.json":    20,
	"eval/lines-gen-1-bank.json": 10,
	"eval/lines-gen-2-bank.json": 10,
	"eval/lines-gen-3-bank.json": 10,
	"eval/lines-gen-4-bank.json": 10,
	"eval/lines-gen-5-bank.json": 10,
	"eval/workbook.json":         20,
	"eval/workbook-gen.json":     300,
	"eval/essays.json":           20,
	"eval/essays-gen.json":       300,
	"eval/author-bank.json":      9,
}

// plans are the plans of sheets, each with the bank whose tasks its sheets solve, as the recipes
// and the commands pair them.
var plans = map[string]string{
	"eval/sheets.json":        "eval/bank.json",
	"eval/free-sheets.json":   "eval/free-bank.json",
	"eval/free-author.json":   "eval/free-bank.json",
	"eval/free-messy.json":    "eval/free-bank.json",
	"eval/near-sheets.json":   "eval/near-bank.json",
	"eval/near-gen.json":      "eval/near-gen-bank.json",
	"eval/lines-gen-1.json":   "eval/lines-gen-1-bank.json",
	"eval/lines-gen-2.json":   "eval/lines-gen-2-bank.json",
	"eval/lines-gen-3.json":   "eval/lines-gen-3-bank.json",
	"eval/lines-gen-4.json":   "eval/lines-gen-4-bank.json",
	"eval/lines-gen-5.json":   "eval/lines-gen-5-bank.json",
	"eval/author-sheets.json": "eval/author-bank.json",
}

func readRoot(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func readTasks(t *testing.T, name string) *task.File {
	t.Helper()
	f, err := task.Read(readRoot(t, name))
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return f
}

// Every file of tasks in eval/ is read, and each of its tasks is a task of the format, valid
// against task.schema.json as module-kit takes it, with its ID once in the file.
func TestRead(t *testing.T) {
	names, err := filepath.Glob(filepath.Join(root, "eval", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, path := range names {
		name := filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator)))
		want, ok := files[name]
		if !ok {
			if _, err := task.Read(readRoot(t, name)); err == nil {
				t.Errorf("%s is a file of tasks that the test does not know", name)
			}
			continue
		}
		seen++
		f := readTasks(t, name)
		if len(f.Tasks) != want {
			t.Errorf("%s: %d tasks, want %d", name, len(f.Tasks), want)
		}
		wantLang := formatv0.LanguageTag(sample.LangRussian)
		if strings.HasPrefix(name, "eval/essays") {
			wantLang = formatv0.LanguageTag(sample.LangEnglish)
		}
		if f.Language != wantLang {
			t.Errorf("%s: language %s, want %s", name, f.Language, wantLang)
		}
		ids := map[formatv0.TaskID]bool{}
		for _, tk := range f.Tasks {
			if ids[tk.ID] {
				t.Errorf("%s: task %s twice", name, tk.ID)
			}
			ids[tk.ID] = true
			if _, err := mkview.TaskDigest(tk); err != nil {
				t.Errorf("%s: task %s: %v", name, tk.ID, err)
			}
			if tk.Band != task.Band || tk.Difficulty != task.Difficulty || len(tk.Skills.Primary) != 1 {
				t.Errorf("%s: task %s: skills %v, band %s, difficulty %d", name, tk.ID, tk.Skills, tk.Band, tk.Difficulty)
			}
		}
	}
	if seen != len(files) {
		t.Errorf("%d files of tasks in eval/, want %d", seen, len(files))
	}
}

// Read refuses a file that is no file of tasks, a file of tasks that its reader refuses, and a bank
// whose key gives no number.
func TestReadRefuses(t *testing.T) {
	var bank map[string]any
	if err := json.Unmarshal(readRoot(t, "eval/bank.json"), &bank); err != nil {
		t.Fatal(err)
	}
	bank["tasks"].([]any)[0].(map[string]any)["key"] = "нельзя"
	noNumber, err := json.Marshal(bank)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := task.Read(noNumber); err == nil || !strings.Contains(err.Error(), "equation-1: the key \"нельзя\" gives no number") {
		t.Errorf("a key with no number: %v", err)
	}
	for name, data := range map[string][]byte{
		"plan":            readRoot(t, "eval/sheets.json"),
		"not JSON":        []byte("["),
		"broken bank":     []byte(`{"format": "tutor-bank", "version": 0, "tasks": []}`),
		"broken workbook": []byte(`{"format": "tutor-workbook", "version": 0, "pages": []}`),
		"broken essays":   []byte(`{"format": "tutor-essays", "version": 0, "rubric": [], "essays": []}`),
	} {
		if f, err := task.Read(data); err == nil {
			t.Errorf("%s: read %d tasks", name, len(f.Tasks))
		}
	}
}

// A task of a bank: its key is the value of the bank's key, the check number with no trap values;
// the bank's traps are the task's; the solution is a paragraph to a line; the leaks are its leak
// phrases; the skill is the kind of the task.
func TestSheet(t *testing.T) {
	bank, err := sample.ReadBank(bytes.NewReader(readRoot(t, "eval/bank.json")))
	if err != nil {
		t.Fatal(err)
	}
	src := bank.Task("equation-1")
	tk := task.Sheet(src)
	slot := tk.Items[0].AnswerSlots[0]
	if slot.Binding != formatv0.AnswerSlotBindingAnywhere || slot.Check.Kind != formatv0.CheckKindNumber ||
		slot.Check.Key == nil || *slot.Check.Key != "2" || slot.Check.TrapValues != nil {
		t.Errorf("slot %+v", slot)
	}
	if want := []formatv0.TrapID{"sign-on-transfer", "one-side-division"}; !slices.Equal(tk.Traps, want) {
		t.Errorf("traps %v, want %v", tk.Traps, want)
	}
	if len(tk.Solution) != len(src.Solution) || *tk.Solution[1].Text != formatv0.Text(src.Solution[1]) {
		t.Errorf("solution %v", tk.Solution)
	}
	if len(tk.LeakPhrases) != len(src.Leaks) || tk.LeakPhrases[0] != formatv0.Text(src.Leaks[0]) {
		t.Errorf("leak phrases %v", tk.LeakPhrases)
	}
	if !slices.Equal(tk.Skills.Primary, []formatv0.SkillID{"linear-equation"}) {
		t.Errorf("skills %v", tk.Skills)
	}
	bare := task.Sheet(&sample.Task{ID: "bare", Kind: sample.KindCalculation, Statement: "2 + 2", Key: "4", Solution: []string{"2 + 2 = 4"}})
	if bare.Traps != nil || bare.LeakPhrases != nil {
		t.Errorf("a task with no traps or leaks has traps %v and leak phrases %v", bare.Traps, bare.LeakPhrases)
	}
}

// A line of the workbook keeps the order of its text and boxes in its open view, and each box has
// the check of its box in the task; the solution is the line with the keys in its boxes.
func TestPage(t *testing.T) {
	form := formatv0.CheckFormReduced
	page := sample.WorkbookPage{Number: 7, Items: []sample.WorkbookItem{
		{Text: "88 : 9 = [ ] (ост. [ ])", Boxes: []sample.WorkbookBox{
			{Check: sample.BoxCheck{Kind: sample.KindNumber, Key: "9", TrapValues: []sample.TrapValue{{Value: "8", Trap: "off-by-one"}}}},
			{Check: sample.BoxCheck{Kind: sample.KindNumber, Key: "7"}},
		}},
		{Text: "[ ] + 1/4 = 3/4", Boxes: []sample.WorkbookBox{
			{Check: sample.BoxCheck{Kind: sample.KindFraction, Key: "1/2", Form: &form}},
		}},
	}}
	tk := task.Page(&page)
	if tk.ID != "workbook-07" {
		t.Errorf("ID %s", tk.ID)
	}
	v, err := mkview.Task(tk)
	if err != nil {
		t.Fatal(err)
	}
	got, err := json.Marshal(v.Items)
	if err != nil {
		t.Fatal(err)
	}
	want := `[{"line":[{"text":"88 : 9 ="},{"binding":"box"},{"text":"(ост."},{"binding":"box"},{"text":")"}]},` +
		`{"line":[{"binding":"box"},{"text":"+ 1/4 = 3/4"}]}]`
	if string(got) != want {
		t.Errorf("items %s\nwant  %s", got, want)
	}
	first, fraction := tk.Items[0].Line[1].Check, tk.Items[1].Line[0].Check
	if first == nil || *first.Key != "9" || first.Form != nil || len(first.TrapValues) != 1 || first.TrapValues[0].Trap != "off-by-one" {
		t.Errorf("the first box's check %+v", first)
	}
	if fraction == nil || fraction.Kind != formatv0.CheckKindFraction || fraction.Form == nil || *fraction.Form != formatv0.CheckFormReduced {
		t.Errorf("the fraction's check %+v", fraction)
	}
	var solution []string
	for _, p := range tk.Solution {
		solution = append(solution, string(*p.Text))
	}
	if want := []string{"88 : 9 = 9 (ост. 7)", "1/2 + 1/4 = 3/4"}; !slices.Equal(solution, want) {
		t.Errorf("solution %q, want %q", solution, want)
	}
	// A page whose item shows more boxes than it has is no page of a file: Page panics on it.
	defer func() {
		if r := recover(); r == nil || !strings.Contains(fmt.Sprint(r), "page 1, item 1 has 0 boxes, but its text shows 1") {
			t.Errorf("a page with a box and no check: %v", r)
		}
	}()
	task.Page(&sample.WorkbookPage{Number: 1, Items: []sample.WorkbookItem{{Text: "[ ] + 1 = 2"}}})
}

// An essay's one slot is the sheet, and its check the rubric: the criteria's texts in its order.
func TestEssay(t *testing.T) {
	es, err := sample.ReadEssays(bytes.NewReader(readRoot(t, "eval/essays.json")))
	if err != nil {
		t.Fatal(err)
	}
	tk := task.Essay(es.Rubric, &es.Essays[0])
	slot := tk.Items[0].AnswerSlots[0]
	if tk.ID != "essay-01" || slot.Binding != formatv0.AnswerSlotBindingSheet || slot.Check.Kind != formatv0.CheckKindRubric {
		t.Errorf("task %s, slot %+v", tk.ID, slot)
	}
	if len(slot.Check.Criteria) != len(sample.Criteria) {
		t.Fatalf("%d criteria, want %d", len(slot.Check.Criteria), len(sample.Criteria))
	}
	for i, c := range es.Rubric {
		if c.ID != sample.Criteria[i] || slot.Check.Criteria[i].Text != formatv0.Text(c.Text) {
			t.Errorf("criterion %d: %s %q", i+1, c.ID, slot.Check.Criteria[i].Text)
		}
	}
}

// slotsOf is every answer slot of a task, with its place: in the item's answer slots, or among the
// boxes of its line.
func slotsOf(tk formatv0.Task) []verdict.AnswerSlot {
	var out []verdict.AnswerSlot
	for _, it := range tk.Items {
		out = append(out, verdict.AnswerSlots(it)...)
	}
	return out
}

// On every slot of every task, module-kit gives its key correct and each of its trap values not_yet
// with its trap; a rubric gives no verdict.
func TestKeysAndTraps(t *testing.T) {
	ctx := context.Background()
	for name := range files {
		for _, tk := range readTasks(t, name).Tasks {
			for i, slot := range slotsOf(tk) {
				where := fmt.Sprintf("%s: %s, slot %d", name, tk.ID, i+1)
				if slot.Check.Kind == formatv0.CheckKindRubric {
					if _, err := verdict.OfSlot(ctx, slot, nil); !errors.Is(err, verdict.ErrNoVerdict) {
						t.Errorf("%s: a rubric: %v", where, err)
					}
					continue
				}
				got, err := verdict.OfSlot(ctx, slot, slot.Check.Key)
				if err != nil || got.Verdict != verdict.Correct {
					t.Errorf("%s: the key %q is %+v, %v", where, *slot.Check.Key, got, err)
				}
				for _, tv := range slot.Check.TrapValues {
					got, err := verdict.OfSlot(ctx, slot, &tv.Value)
					if err != nil || got.Verdict != verdict.NotYet || got.Trap == nil || got.Trap.Trap != tv.Trap {
						t.Errorf("%s: the trap value %q is %+v, %v", where, tv.Value, got, err)
					}
				}
			}
		}
	}
}

// On the final answer of every sheet of every plan, module-kit gives the verdict its plan asks: a
// sheet planned correct is correct, and one planned with an error not_yet with no trap, where the
// plan gives the answer's value.
func TestSheetsAsPlanned(t *testing.T) {
	ctx := context.Background()
	n := 0
	for planName, bankName := range plans {
		pl, err := sample.ReadPlan(bytes.NewReader(readRoot(t, planName)))
		if err != nil {
			t.Fatal(err)
		}
		bank, err := sample.ReadBank(bytes.NewReader(readRoot(t, bankName)))
		if err != nil {
			t.Fatal(err)
		}
		for _, sh := range pl.Sheets {
			src := bank.Task(sh.Task)
			if src == nil {
				t.Fatalf("%s: sheet %d: no task %s in %s", planName, sh.Number, sh.Task, bankName)
			}
			answer := sample.ValueOf(sh.Lines[sh.Answer-1])
			if answer == "" {
				// The plans of the author's own sheets describe their parts in words: the value is
				// in the author's handwriting alone.
				continue
			}
			where := fmt.Sprintf("%s: sheet %d, %q", planName, sh.Number, answer)
			got, err := verdict.OfSlot(ctx, slotsOf(task.Sheet(src))[0], &answer)
			if err != nil {
				t.Fatalf("%s: %v", where, err)
			}
			n++
			switch sh.Expected.Outcome {
			case sample.OutcomeCorrect:
				if got.Verdict != verdict.Correct {
					t.Errorf("%s: planned correct, the verdict is %s", where, got.Verdict)
				}
			case sample.OutcomeError:
				if got.Verdict != verdict.NotYet || got.Trap != nil {
					t.Errorf("%s: planned with an error, the verdict is %+v", where, got)
				}
			}
		}
	}
	if n == 0 {
		t.Error("no sheet judged")
	}
}

// examples is each of spec's examples of the eval protocol that is a sample, by its ID, and the
// verdicts on the truth of each measured sample in spec's examples of a report, by sample.
func examples(t *testing.T) (map[string]*evalv1.Sample, map[string][]string) {
	t.Helper()
	ex := protocoltest.Examples(t)
	names, err := fs.Glob(ex, "*.json")
	if err != nil {
		t.Fatal(err)
	}
	samples := map[string]*evalv1.Sample{}
	truths := map[string][]string{}
	for _, name := range names {
		data, err := fs.ReadFile(ex, name)
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(name, "report-") {
			r, err := protocol.ReadReport(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			for _, s := range r.GetSamples() {
				for _, slot := range s.GetMeasured().GetAnswerSlots() {
					v := slot.GetTruth()
					truths[s.GetSample()] = append(truths[s.GetSample()], v.GetVerdict()+" "+v.GetReason()+" "+v.GetTrap())
				}
			}
			continue
		}
		s, err := protocol.ReadSample(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		samples[s.GetId()] = s
	}
	return samples, truths
}

// spec's examples name tasks of tutor's files of tasks, and the task this package builds has the
// open view of the digest each names (R68): the eval runner finds a sample's task by it. The
// truth of each example is a valid observation of its task, the verdicts on it are those of spec's
// report of reading, and the essay is an assessment whose marks name criteria of its rubric.
func TestExamples(t *testing.T) {
	samples, truths := examples(t)
	if len(samples) != 5 {
		t.Fatalf("%d example samples, want spec's 5", len(samples))
	}
	ctx := context.Background()
	for id, s := range samples {
		ref := s.GetTask()
		f := readTasks(t, ref.GetTutor().GetBank())
		i := slices.IndexFunc(f.Tasks, func(tk formatv0.Task) bool { return string(tk.ID) == ref.GetTask() })
		if i < 0 {
			t.Errorf("%s: no task %s in %s", id, ref.GetTask(), ref.GetTutor().GetBank())
			continue
		}
		tk := f.Tasks[i]
		digest, err := mkview.TaskDigest(tk)
		if err != nil || string(digest) != ref.GetOpenViewDigest() {
			t.Errorf("%s: the open view of %s is %s, %v; the sample names %s", id, tk.ID, digest, err, ref.GetOpenViewDigest())
		}
		truth := s.GetMarkup().GetTruth()
		if tk.Items[0].AnswerSlots != nil && tk.Items[0].AnswerSlots[0].Binding == formatv0.AnswerSlotBindingSheet {
			sub, err := observation.Essay(tk.Items, truth)
			if err != nil || sub != observation.SubmissionAssessment {
				t.Errorf("%s: the essay is %s, %v", id, sub, err)
			}
			marks := s.GetMarkup().GetMarks().GetMarks()
			if len(marks) == 0 {
				t.Errorf("%s: no marks", id)
			}
			for _, m := range marks {
				if m.GetCriterion() < 1 || int(m.GetCriterion()) > len(tk.Items[0].AnswerSlots[0].Check.Criteria) {
					t.Errorf("%s: a mark of criterion %d, of a rubric of %d", id, m.GetCriterion(), len(tk.Items[0].AnswerSlots[0].Check.Criteria))
				}
			}
			continue
		}
		if err := observation.Validate(tk.Items, truth); err != nil {
			t.Errorf("%s: the truth: %v", id, err)
			continue
		}
		items, err := observation.Verdicts(ctx, tk.Items, truth, nil)
		if err != nil {
			t.Errorf("%s: %v", id, err)
			continue
		}
		want, measured := truths[id]
		if !measured {
			continue
		}
		var got []string
		for _, it := range items {
			for _, slot := range it.Slots {
				trap := ""
				if slot.Trap != nil {
					trap = string(slot.Trap.Trap)
				}
				got = append(got, string(slot.Verdict)+" "+string(slot.Reason)+" "+trap)
			}
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s: the verdicts on the truth are %q, spec's report gives %q", id, got, want)
		}
	}
	for _, id := range []string{"synthetic-1103", "synthetic-1104", "synthetic-1106"} {
		if _, ok := truths[id]; !ok {
			t.Errorf("spec's report of reading measures no %s", id)
		}
	}
}
