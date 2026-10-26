package main

import (
	"bytes"
	"fmt"
	"os"
	"slices"
	"testing"

	"github.com/altnovatrail/tutor/internal/board"
	"github.com/altnovatrail/tutor/internal/sample"
)

// The banks of tasks, as seen from this package's directory, where go test runs: the bank of the
// reading measurement (docs/architecture.md, section 3.1) and the bank of epic E4's free-form
// writing (section 3.4).
const (
	measurementBank = "../../eval/bank.json"
	freeBank        = "../../eval/free-bank.json"
)

// readBank reads a bank of this repository.
func readBank(t *testing.T, path string) *sample.Bank {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	bank, err := sample.ReadBank(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return bank
}

// checkedBank checks a bank the way the author does, with tutor-eval check, which must find it
// holds n tasks, and reads it.
func checkedBank(t *testing.T, path string, n int) *sample.Bank {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if got := run([]string{"check", path}, &stdout, &stderr); got != exitOK {
		t.Fatalf("check %s = %d, stderr %q", path, got, stderr.String())
	}
	if want := fmt.Sprintf("%s: ok, bank of %d tasks\n", path, n); stdout.String() != want {
		t.Errorf("stdout = %q, want %q", stdout.String(), want)
	}
	return readBank(t, path)
}

// tasksOf lists a bank's tasks as "ID kind".
func tasksOf(bank *sample.Bank) []string {
	var tasks []string
	for _, task := range bank.Tasks {
		tasks = append(tasks, task.ID+" "+string(task.Kind))
	}
	return tasks
}

// TestMeasurementBank checks that the measurement's bank holds what the sheet plan,
// docs/sheets.md, names: these ten tasks of these kinds.
func TestMeasurementBank(t *testing.T) {
	bank := checkedBank(t, measurementBank, 10)
	want := []string{
		"equation-1 linear-equation", "equation-2 linear-equation", "equation-3 linear-equation", "equation-4 linear-equation",
		"chain-1 calculation", "chain-2 calculation", "chain-3 calculation",
		"posts free-solution", "rabbits free-solution", "logs free-solution",
	}
	if got := tasksOf(bank); !slices.Equal(got, want) {
		t.Errorf("the bank's tasks are %q, the sheet plan's %q", got, want)
	}
}

// TestFreeBank checks that the bank of free-form writing holds the six free solutions of the
// board's pilot: two on a number line, two with a drawing and two with the working at the side
// (RUN.md, T52).
func TestFreeBank(t *testing.T) {
	bank := checkedBank(t, freeBank, 6)
	want := []string{
		"grasshopper free-solution", "backwards free-solution",
		"houses free-solution", "stamps free-solution",
		"pages free-solution", "album free-solution",
	}
	if got := tasksOf(bank); !slices.Equal(got, want) {
		t.Errorf("the bank's tasks are %q, want %q", got, want)
	}
}

// TestBoardTasks checks that the board's tasks are, in this order, its first five, which are in
// the measurement's bank, and then the bank of free-form writing, as README.md and
// docs/architecture.md, section 3.3, number them: tasks 6 to 11 are the free-form ones. Each is
// its copy in one of the two banks, under the same ID, so a model judging it is told the same
// statement, key and solution on the board and in a measurement.
func TestBoardTasks(t *testing.T) {
	banks := []*sample.Bank{readBank(t, measurementBank), readBank(t, freeBank)}
	want := []string{"equation-1", "equation-2", "chain-1", "chain-2", "posts"}
	for _, task := range banks[1].Tasks {
		want = append(want, task.ID)
	}
	var got []string
	for _, task := range board.Tasks {
		got = append(got, task.ID)
	}
	if !slices.Equal(got, want) {
		t.Errorf("the board's tasks are %q, want %q", got, want)
	}

	for _, want := range board.Tasks {
		var copies []*sample.Task
		for _, bank := range banks {
			if got := bank.Task(want.ID); got != nil {
				copies = append(copies, got)
			}
		}
		if len(copies) != 1 {
			t.Errorf("the board's task %q is in %d banks, want one", want.ID, len(copies))
			continue
		}
		if got := copies[0]; got.Statement != want.Statement || got.Key != want.Key || !slices.Equal(got.Solution, want.Solution) {
			t.Errorf("the bank's task %q is %q, key %q, solution %q; the board's is %q, key %q, solution %q",
				want.ID, got.Statement, got.Key, got.Solution, want.Statement, want.Key, want.Solution)
		}
	}
}

// TestTraps checks the rule of docs/architecture.md, section 3.1, across both banks: one mistake
// has one trap ID and one description, as a module's trap does (concepts 7 in spec). The same ID
// with another description fails, and so does the same description under another ID.
func TestTraps(t *testing.T) {
	byID, byDescription := make(map[string]string), make(map[string]string)
	for _, path := range []string{measurementBank, freeBank} {
		for _, task := range readBank(t, path).Tasks {
			for _, trap := range task.Traps {
				if d, ok := byID[trap.ID]; !ok {
					byID[trap.ID] = trap.Description
				} else if d != trap.Description {
					t.Errorf("the trap %q of the task %q is %q, and earlier %q", trap.ID, task.ID, trap.Description, d)
				}
				if id, ok := byDescription[trap.Description]; !ok {
					byDescription[trap.Description] = trap.ID
				} else if id != trap.ID {
					t.Errorf("the trap %q of the task %q is %q, as the trap %q is", trap.ID, task.ID, trap.Description, id)
				}
			}
		}
	}
}
