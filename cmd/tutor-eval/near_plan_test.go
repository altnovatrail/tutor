package main

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/internal/reading"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/synth"
)

// The bank and the plan of the near set (T70; docs/architecture.md, 3.4), and the document that
// shows them as tables, as seen from this package's directory.
const (
	nearBank      = "../../eval/near-bank.json"
	nearPlan      = "../../eval/near-sheets.json"
	nearSheetsDoc = "../../docs/near-sheets.md"
)

// swapLast is a text with its last digit turned into the digit of its pair, or false if the last
// digit has no pair.
func swapLast(s string) (string, bool) {
	at, toward, ok := synth.SimilarDigit(s)
	if !ok {
		return "", false
	}
	r := []rune(s)
	r[at] = toward
	return string(r), true
}

// The near set as the author checks it: tutor-eval check takes its bank and plan. Its tasks are
// copies of tasks of E2's bank, word for word but for the false keys, so that the bank of E2 stays
// as it is (R28). Each task has one false key, its near key: the key with its last digit turned into
// the digit of its pair, and a solution that is the task's with the same digit turned in its last
// line, so that the key and the sheet differ in that digit alone. The plan has two sheets of each
// task, of the scenario similar-digits: a correct one, whose answer is the key, and one with the same
// lines but for a mistake in the answer, which is the near key, its first error. The tables of docs/near-sheets.md
// are the bank and the plan, byte for byte.
func TestNearSet(t *testing.T) {
	code, out, errs := runIt("check", nearBank, nearPlan)
	if want := nearBank + ": ok, bank of 6 tasks\n" + nearPlan + ": ok, plan of 12 sheets, its tasks and traps are in the bank\n"; code != exitOK || out != want {
		t.Errorf("check = %d %q %q, want %q", code, out, errs, want)
	}
	e2, near := readBank(t, measurementBank), readBank(t, nearBank)
	for _, task := range near.Tasks {
		orig := e2.Task(task.ID)
		if orig == nil {
			t.Fatalf("the near task %q is not in E2's bank", task.ID)
		}
		copied, want := task, *orig
		copied.FalseKeys, want.FalseKeys = nil, nil
		if !reflect.DeepEqual(copied, want) {
			t.Errorf("the near task %q is not E2's but for its false keys", task.ID)
		}
		key, ok := swapLast(task.Key)
		last, okLast := swapLast(task.Solution[len(task.Solution)-1])
		solution := append(slices.Clone(task.Solution[:len(task.Solution)-1]), last)
		if len(task.FalseKeys) != 1 || !ok || !okLast || task.FalseKeys[0].Key != key || !slices.Equal(task.FalseKeys[0].Solution, solution) {
			t.Errorf("the near task %q has the false keys %+v, want %q with %q", task.ID, task.FalseKeys, key, solution)
		}
	}
	plan := readPlanFile(t, nearPlan)
	count := map[string][]sample.Outcome{}
	correct := map[string][]string{} // the lines of the correct sheet of each task
	for _, sh := range plan.Sheets {
		count[sh.Task] = append(count[sh.Task], sh.Expected.Outcome)
		answer := sh.Lines[sh.Answer-1]
		if sh.Scenario != sample.ScenarioSimilarDigits || sh.Answer != len(sh.Lines) || sh.Erasure != nil {
			t.Errorf("sheet %d is %+v", sh.Number, sh)
		}
		if _, _, ok := synth.SimilarDigit(answer); !ok {
			t.Errorf("sheet %d: the answer %q has no digit to lean", sh.Number, answer)
		}
		task := near.Task(sh.Task)
		if task == nil {
			t.Fatalf("sheet %d: the task %q is not in the near bank", sh.Number, sh.Task)
		}
		switch sh.Expected.Outcome {
		case sample.OutcomeCorrect:
			if reading.ValueOf(answer) != reading.KeyOf(task.Key) || sh.Expected.FirstError != nil {
				t.Errorf("sheet %d, correct, answers %q, and the key is %q", sh.Number, answer, task.Key)
			}
			correct[sh.Task] = sh.Lines
		case sample.OutcomeError:
			right, ok := correct[sh.Task]
			if !ok {
				t.Fatalf("sheet %d, with a mistake, comes before the correct sheet of its task", sh.Number)
			}
			wrong, _ := swapLast(right[len(right)-1])
			want := append(slices.Clone(right[:len(right)-1]), wrong)
			if !slices.Equal(sh.Lines, want) || reading.ValueOf(answer) != reading.KeyOf(task.FalseKeys[0].Key) ||
				sh.Expected.FirstError == nil || *sh.Expected.FirstError != sh.Answer || sh.Expected.Trap != "" {
				t.Errorf("sheet %d, with a mistake, has the lines %q, want %q with the answer its first error", sh.Number, sh.Lines, want)
			}
		}
	}
	for _, task := range near.Tasks {
		if got := count[task.ID]; !slices.Equal(got, []sample.Outcome{sample.OutcomeCorrect, sample.OutcomeError}) {
			t.Errorf("the task %q has the sheets %v, want a correct one and one with a mistake", task.ID, got)
		}
	}
	doc := readDoc(t, nearSheetsDoc)
	sameTable(t, nearSheetsDoc, doc, nearBank, nearTaskTable(t, near))
	sameTable(t, nearSheetsDoc, doc, nearPlan, nearSheetTable(t, plan))
}

// readPlanFile reads a sheet plan.
func readPlanFile(t *testing.T, path string) *sample.Plan {
	t.Helper()
	pl, err := sample.ReadPlan(strings.NewReader(readDoc(t, path)))
	if err != nil {
		t.Fatal(err)
	}
	return pl
}

// nearTaskTable writes the table of tasks of docs/near-sheets.md from the bank: a task to a line,
// with its key, its near key and the digits of their pair.
func nearTaskTable(t *testing.T, b *sample.Bank) string {
	t.Helper()
	var s strings.Builder
	s.WriteString("| ID | Ключ | Почти-ложный ключ | Похожие цифры |\n|---|---|---|---|\n")
	for _, task := range b.Tasks {
		at, toward, ok := synth.SimilarDigit(task.Key)
		if !ok {
			t.Fatalf("the key %q of %q has no digit of a pair", task.Key, task.ID)
		}
		fmt.Fprintf(&s, "| %s | %s | %s | %c → %c |\n", cell(t, task.ID), cell(t, task.Key), cell(t, task.FalseKeys[0].Key), []rune(task.Key)[at], toward)
	}
	return strings.TrimSuffix(s.String(), "\n")
}

// nearSheetTable writes the table of sheets of docs/near-sheets.md from the plan: a sheet to a line,
// its lines joined by "; ", its outcome, and the digit of its answer that leans toward its pair.
func nearSheetTable(t *testing.T, pl *sample.Plan) string {
	t.Helper()
	var s strings.Builder
	s.WriteString("| № | Задача | Строки на листе | Исход | Цифра ответа клонится к |\n|---|---|---|---|---|\n")
	for _, sh := range pl.Sheets {
		lines := make([]string, len(sh.Lines))
		for i, l := range sh.Lines {
			lines[i] = cell(t, l)
		}
		answer := sh.Lines[sh.Answer-1]
		at, toward, ok := synth.SimilarDigit(answer)
		if !ok {
			t.Fatalf("sheet %d: the answer %q has no digit of a pair", sh.Number, answer)
		}
		fmt.Fprintf(&s, "| %d | %s | %s | %s | %c → %c |\n", sh.Number, cell(t, sh.Task), strings.Join(lines, "; "), cell(t, string(sh.Expected.Outcome)), []rune(answer)[at], toward)
	}
	return strings.TrimSuffix(s.String(), "\n")
}
