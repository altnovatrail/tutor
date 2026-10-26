package sample_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/altnovatrail/tutor/internal/sample"
)

// The name of a saved sheet's file holds its plan sheet's number, the time of the save to the
// millisecond, its task and its scenario, and reads back as it was written.
func TestSheetFile(t *testing.T) {
	sh := &sample.PlanSheet{Number: 7, Task: "chain-2", Scenario: sample.ScenarioError}
	at := time.Date(2026, 10, 26, 10, 15, 30, 123_456_789, time.FixedZone("UTC+3", 3*3600))
	name := sample.SheetFile(sh, at)
	if name != "07-20261026-071530-123-chain-2-error.json" {
		t.Fatalf("SheetFile = %q, want the time in UTC to the millisecond", name)
	}
	n, ok := sample.ParseSheetFile(name)
	if !ok || n.Number != 7 || !n.At.Equal(at.Truncate(time.Millisecond)) || n.Of != "chain-2-error" || !n.Saves(sh) {
		t.Errorf("ParseSheetFile = %+v, %v; want the name read back", n, ok)
	}
	if n.Saves(&sample.PlanSheet{Number: 7, Task: "chain-2", Scenario: sample.ScenarioCorrect}) {
		t.Error("a file of another scenario saves the sheet")
	}
	for _, name := range []string{
		"7-20261026-101530-123-chain-2-error.json", "07-20261026-101530-12-chain-2-error.json",
		"07-20261026-101530-123-chain-2-error.json~", "07-notes.json", "07-20261399-101530-123-chain-2-error.json",
	} {
		if _, ok := sample.ParseSheetFile(name); ok {
			t.Errorf("%s reads as a saved sheet's file", name)
		}
	}
}

func testSplit() *sample.Split {
	v := sample.Version
	return &sample.Split{Format: sample.SplitFormat, Version: &v, Seed: 7,
		Dev:  []sample.SplitSheet{{Number: 2, File: "02-20261026-101530-123-chain-2-error.json"}},
		Test: []sample.SplitSheet{{Number: 1, File: "01-20261026-101531-000-posts-correct.json"}}}
}

func TestReadSplit(t *testing.T) {
	data, err := sample.Marshal(testSplit())
	if err != nil {
		t.Fatal(err)
	}
	sp, err := sample.ReadSplit(strings.NewReader(string(data)))
	if err != nil || sp.Seed != 7 || len(sp.Dev) != 1 || len(sp.Test) != 1 {
		t.Fatalf("ReadSplit = %+v, %v", sp, err)
	}
	if _, err := sample.ReadSplit(strings.NewReader(strings.Replace(string(data), `"seed"`, `"Seed"`, 1))); err == nil {
		t.Error("a field in another case is read")
	}
}

func TestSplitCheck(t *testing.T) {
	tests := []struct {
		name   string
		change func(sp *sample.Split)
		path   string
		want   string
	}{
		{"another format", func(sp *sample.Split) { sp.Format = "tutor-plan" }, "format", `not "tutor-split"`},
		{"no development part", func(sp *sample.Split) { sp.Dev = []sample.SplitSheet{} }, "dev", "is empty"},
		{"a sheet in both parts", func(sp *sample.Split) { sp.Test[0] = sp.Dev[0] }, "test[0].number", "already in dev[0]"},
		{"a file of another number", func(sp *sample.Split) { sp.Dev[0].Number = 3 }, "dev[0].file", "not the name of a saved file of sheet 3"},
		{"no file", func(sp *sample.Split) { sp.Test[0].File = "notes.json" }, "test[0].file", "not the name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sp := testSplit()
			tt.change(sp)
			has(t, sp.Check(), tt.path, tt.want)
		})
	}
	if err := testSplit().Check(); err != nil {
		t.Errorf("a good split: %v", err)
	}
}

func TestSplitCheckWithPlan(t *testing.T) {
	plan := &sample.Plan{Sheets: []sample.PlanSheet{
		{Number: 1, Task: "posts", Scenario: sample.ScenarioCorrect},
		{Number: 2, Task: "chain-2", Scenario: sample.ScenarioError},
		{Number: 3, Task: "logs", Scenario: sample.ScenarioCorrect},
	}}
	err := testSplit().CheckWithPlan(plan)
	has(t, err, sample.Root, "sheet 3 of the plan is in neither part")
	sp := testSplit()
	sp.Test = append(sp.Test, sample.SplitSheet{Number: 3, File: "03-20261026-101532-000-posts-correct.json"}, sample.SplitSheet{Number: 9, File: "09-20261026-101533-000-logs-correct.json"})
	err = sp.CheckWithPlan(plan)
	has(t, err, "test[1].file", "not a file of sheet 3 of the plan, whose task and scenario are logs-correct")
	has(t, err, "test[2].number", "9 is not a sheet of the plan")
}

// has fails unless err is a format error with a problem at path whose message holds want.
func has(t *testing.T, err error, path, want string) {
	t.Helper()
	var fe *sample.FormatError
	if !errors.As(err, &fe) {
		t.Fatalf("error %v, want a format error", err)
	}
	for _, p := range fe.Problems {
		if p.Path == path && strings.Contains(p.Message, want) {
			return
		}
	}
	t.Errorf("no problem at %q holding %q in %v", path, want, err)
}
