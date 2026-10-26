package main

import (
	"bytes"
	"os"
	"slices"
	"testing"

	"github.com/altnovatrail/tutor/internal/board"
	"github.com/altnovatrail/tutor/internal/sample"
)

// measurementBank is the bank of the reading measurement (docs/architecture.md, section 3.1), as
// seen from this package's directory, where go test runs.
const measurementBank = "../../eval/bank.json"

// TestMeasurementBank checks the measurement's bank the way the author does, with tutor-eval
// check, and that it holds what the sheet plan, docs/sheets.md, names: these ten tasks of these
// kinds. The board's five are among them as the board has them, under the same IDs, so a model
// judging them is told the same statement, key and solution on the board and in the measurement.
func TestMeasurementBank(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if got := run([]string{"check", measurementBank}, &stdout, &stderr); got != exitOK {
		t.Fatalf("check %s = %d, stderr %q", measurementBank, got, stderr.String())
	}
	if want := measurementBank + ": ok, bank of 10 tasks\n"; stdout.String() != want {
		t.Errorf("stdout = %q, want %q", stdout.String(), want)
	}

	data, err := os.ReadFile(measurementBank)
	if err != nil {
		t.Fatal(err)
	}
	bank, err := sample.ReadBank(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	var tasks []string
	for _, task := range bank.Tasks {
		tasks = append(tasks, task.ID+" "+string(task.Kind))
	}
	want := []string{
		"equation-1 linear-equation", "equation-2 linear-equation", "equation-3 linear-equation", "equation-4 linear-equation",
		"chain-1 calculation", "chain-2 calculation", "chain-3 calculation",
		"posts free-solution", "rabbits free-solution", "logs free-solution",
	}
	if !slices.Equal(tasks, want) {
		t.Errorf("the bank's tasks are %q, the sheet plan's %q", tasks, want)
	}

	for _, want := range board.Tasks {
		got := bank.Task(want.ID)
		switch {
		case got == nil:
			t.Errorf("the bank has no task %q", want.ID)
		case got.Statement != want.Statement || got.Key != want.Key || !slices.Equal(got.Solution, want.Solution):
			t.Errorf("the bank's task %q is %q, key %q, solution %q; the board's is %q, key %q, solution %q",
				want.ID, got.Statement, got.Key, got.Solution, want.Statement, want.Key, want.Solution)
		}
	}
}
