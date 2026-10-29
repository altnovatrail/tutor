package gen_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/internal/gen"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/sheets"
	"github.com/altnovatrail/tutor/internal/synth"
)

// sameJSON is whether two values marshal to the same JSON.
func sameJSON(t *testing.T, a, b any) bool {
	t.Helper()
	x, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	y, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	return string(x) == string(y)
}

// drawable checks that the bank and the plan pass their checks, and that code draws every sheet of
// the plan into a sample that passes its check, by the bank, with the plan's markup.
func drawable(t *testing.T, bank *sample.Bank, plan *sample.Plan, shift float64) {
	t.Helper()
	if err := bank.Check(); err != nil {
		t.Fatalf("the bank: %v", err)
	}
	if err := plan.Check(); err != nil {
		t.Fatalf("the plan: %v", err)
	}
	if err := plan.CheckWithBank(bank); err != nil {
		t.Fatalf("the plan by the bank: %v", err)
	}
	for i := range plan.Sheets {
		sh := &plan.Sheets[i]
		s := synth.SheetLeaning(sh, 1, shift)
		if err := s.Check(); err != nil {
			t.Fatalf("sheet %d drawn: %v", sh.Number, err)
		}
		if err := s.CheckWithBank(bank); err != nil {
			t.Fatalf("sheet %d drawn, by the bank: %v", sh.Number, err)
		}
		if diffs := sheets.Differences(sh, s); len(diffs) > 0 {
			t.Fatalf("sheet %d is drawn off the plan at %s: %s", sh.Number, diffs[0].Path, diffs[0].Message)
		}
	}
}

// Ten tasks give 60 sheets, six a task, and their bank and plan pass their checks; code draws every
// sheet. The same seed gives the same bank and plan, and another seed others. A plan of no task, or
// of more sheets than a plan holds, is not made.
func TestSheets(t *testing.T) {
	bank, plan, err := gen.Sheets(10, 1, "lines-gen")
	if err != nil {
		t.Fatal(err)
	}
	if len(bank.Tasks) != 10 || len(plan.Sheets) != 60 {
		t.Fatalf("%d tasks and %d sheets", len(bank.Tasks), len(plan.Sheets))
	}
	drawable(t, bank, plan, synth.SimilarShift)
	scenarios, kinds := map[sample.Scenario]int{}, map[sample.TaskKind]int{}
	for _, sh := range plan.Sheets {
		scenarios[sh.Scenario]++
	}
	for _, task := range bank.Tasks {
		kinds[task.Kind]++
		if !strings.HasPrefix(task.ID, "lines-gen-") {
			t.Errorf("the task %s", task.ID)
		}
	}
	// 4 chains have a second correct sheet, and 6 equations one that checks after the answer.
	want := map[sample.Scenario]int{sample.ScenarioCorrect: 14, sample.ScenarioError: 20, sample.ScenarioErasureFixed: 10, sample.ScenarioErasureBroken: 10, sample.ScenarioAnswerNotLast: 6}
	if !sameJSON(t, scenarios, want) || kinds[sample.KindCalculation] != 4 || kinds[sample.KindLinearEquation] != 6 {
		t.Errorf("the scenarios are %v and the kinds %v", scenarios, kinds)
	}
	again, plan2, _ := gen.Sheets(10, 1, "lines-gen")
	other, _, _ := gen.Sheets(10, 2, "lines-gen")
	if !sameJSON(t, bank, again) || !sameJSON(t, plan, plan2) || sameJSON(t, bank, other) {
		t.Error("the seed does not decide the bank and the plan")
	}
	for _, tasks := range []int{0, 17} {
		if _, _, err := gen.Sheets(tasks, 1, "x"); !errors.Is(err, gen.ErrTasks) {
			t.Errorf("a plan of %d tasks: %v", tasks, err)
		}
	}
}

// The files of the extended measurement are what the generator makes of their seeds, byte for byte:
// the same command makes them again (architecture 3.4, «Наборы T95»).
func TestFiles(t *testing.T) {
	files := map[string]func() (*sample.Bank, *sample.Plan, error){
		"near-gen": func() (*sample.Bank, *sample.Plan, error) { return gen.NearSheets(20, 1, "near-gen") },
	}
	for k := uint64(1); k <= 5; k++ {
		name := fmt.Sprintf("lines-gen-%d", k)
		files[name] = func() (*sample.Bank, *sample.Plan, error) { return gen.Sheets(10, k, name) }
	}
	for name, made := range files {
		bank, plan, err := made()
		if err != nil {
			t.Fatal(err)
		}
		for file, v := range map[string]any{name + "-bank.json": bank, name + ".json": plan} {
			want, err := sample.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			if got, err := os.ReadFile(filepath.Join("..", "..", "eval", file)); err != nil || !bytes.Equal(got, want) {
				t.Errorf("eval/%s is not what the generator makes of its seed: %v", file, err)
			}
		}
	}
}

// Whatever the seed, the generators make banks and plans that pass their checks.
func TestSeeds(t *testing.T) {
	for seed := uint64(1); seed <= 200; seed++ {
		if _, _, err := gen.Sheets(10, seed, "s"); err != nil {
			t.Fatalf("the sheets of the seed %d: %v", seed, err)
		}
		if _, _, err := gen.NearSheets(20, seed, "n"); err != nil {
			t.Fatalf("the near sheets of the seed %d: %v", seed, err)
		}
	}
}

// Each sheet with an error is wrong from its first wrong line on, and its answer is not the key;
// each correct sheet's answer line ends in the key.
func TestSheetsTruth(t *testing.T) {
	bank, plan, err := gen.Sheets(16, 7, "t")
	if err != nil {
		t.Fatal(err)
	}
	for _, sh := range plan.Sheets {
		key := bank.Task(sh.Task).Key
		answer := sh.Lines[sh.Answer-1]
		value := key
		if i := strings.LastIndex(key, " = "); i >= 0 {
			value = key[i+3:]
		}
		ends := strings.HasSuffix(answer, " = "+value) || answer == key
		switch sh.Expected.Outcome {
		case sample.OutcomeCorrect:
			if !ends {
				t.Errorf("sheet %d is correct, but its answer %q is not the key %q", sh.Number, answer, key)
			}
		case sample.OutcomeError:
			if ends {
				t.Errorf("sheet %d has an error, but its answer %q is the key %q", sh.Number, answer, key)
			}
			if sh.Expected.FirstError == nil || *sh.Expected.FirstError > sh.Answer {
				t.Errorf("sheet %d: the first error %v after the answer %d", sh.Number, sh.Expected.FirstError, sh.Answer)
			}
		}
	}
}

// The near tasks end their keys in a digit of a pair, their false key is the near key, and their two
// sheets are the correct one and the one whose answer is the near key; code draws them at each share
// of the shift.
func TestNearSheets(t *testing.T) {
	bank, plan, err := gen.NearSheets(20, 1, "near-gen")
	if err != nil {
		t.Fatal(err)
	}
	if len(bank.Tasks) != 20 || len(plan.Sheets) != 40 {
		t.Fatalf("%d tasks and %d sheets", len(bank.Tasks), len(plan.Sheets))
	}
	for _, shift := range []float64{0.1, 0.2, 0.3, 0.4} {
		drawable(t, bank, plan, shift)
	}
	for i, task := range bank.Tasks {
		right, wrong := plan.Sheets[2*i], plan.Sheets[2*i+1]
		fk := task.FalseKeys
		if len(fk) != 1 || len(fk[0].Key) != len(task.Key) || fk[0].Key == task.Key || fk[0].Key[:len(fk[0].Key)-1] != task.Key[:len(task.Key)-1] {
			t.Errorf("%s: the key %q and the false keys %v", task.ID, task.Key, fk)
			continue
		}
		last, nearLast := []rune(task.Key), []rune(fk[0].Key)
		if pair, ok := sample.PairOf(last[len(last)-1]); !ok || pair != nearLast[len(nearLast)-1] {
			t.Errorf("%s: %q is not the near key of %q", task.ID, fk[0].Key, task.Key)
		}
		if right.Scenario != sample.ScenarioSimilarDigits || right.Expected.Outcome != sample.OutcomeCorrect || wrong.Expected.Outcome != sample.OutcomeError ||
			!sameJSON(t, wrong.Lines, fk[0].Solution) || !sameJSON(t, right.Lines, task.Solution) {
			t.Errorf("%s: the sheets %+v and %+v", task.ID, right, wrong)
		}
	}
	if _, _, err := gen.NearSheets(50, 1, "x"); !errors.Is(err, gen.ErrTasks) {
		t.Errorf("a near plan of 100 sheets: %v", err)
	}
}
