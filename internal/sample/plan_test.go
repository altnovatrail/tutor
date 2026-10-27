package sample_test

import (
	"bytes"
	"errors"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/synth"
)

// testPlan returns a new plan of four sheets on the synthetic bank: a correct sheet, a sheet with
// an error, and one of each erasure scenario, the first of them erasing a part of a line.
func testPlan() *sample.Plan {
	v := sample.Version
	return &sample.Plan{
		Format:  sample.PlanFormat,
		Version: &v,
		Sheets: []sample.PlanSheet{
			{
				Number: 1, Task: synth.TaskLinear, Scenario: sample.ScenarioCorrect,
				Lines: []string{"3x + 1 = 7", "3x = 6", "x = 2"}, Answer: 3,
				Expected: sample.Expected{Outcome: sample.OutcomeCorrect},
			},
			{
				Number: 2, Task: synth.TaskLinear, Scenario: sample.ScenarioError,
				Lines: []string{"3x + 1 = 7", "3x = 7 + 1", "3x = 8", "x = 8/3"}, Answer: 4,
				Expected: sample.Expected{Outcome: sample.OutcomeError, FirstError: intp(2), Trap: "sign-on-transfer"},
			},
			{
				Number: 3, Task: synth.TaskCalculation, Scenario: sample.ScenarioErasureFixed,
				Lines: []string{"47 + 19 = 66", "66 : 6 = 11"}, Answer: 2,
				Expected: sample.Expected{Outcome: sample.OutcomeCorrect},
				Erasure:  &sample.PlanErasure{Line: 1, Erased: sample.ErasedMistake, Text: "56", Replacement: "66", PartOfLine: true},
			},
			{
				Number: 4, Task: synth.TaskFree, Scenario: sample.ScenarioErasureBroken,
				Lines: []string{"20 : 5 = 4", "Ответ: 4"}, Answer: 2,
				Expected: sample.Expected{Outcome: sample.OutcomeError, FirstError: intp(2), Trap: "gaps-for-posts"},
				Erasure:  &sample.PlanErasure{Line: 2, Erased: sample.ErasedCorrectStep, Text: "4 + 1 = 5", Replacement: "Ответ: 4"},
			},
		},
	}
}

func TestPlanRoundTrip(t *testing.T) {
	data, err := sample.Marshal(testPlan())
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got, err := sample.ReadPlan(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("ReadPlan: %v", err)
	}
	if !reflect.DeepEqual(got, testPlan()) {
		t.Error("the plan read back differs from the one written")
	}
	if err := got.CheckWithBank(synth.Bank()); err != nil {
		t.Errorf("CheckWithBank: %v", err)
	}
}

func TestCheckRefusesBrokenPlans(t *testing.T) {
	tests := []struct {
		name   string
		change func(pl *sample.Plan)
		path   string
		want   string
	}{
		// The plan's own rules.
		{"another format", func(pl *sample.Plan) { pl.Format = sample.SampleFormat }, "format", `not "tutor-sheet-plan"`},
		{"no version", func(pl *sample.Plan) { pl.Version = nil }, "version", "is missing"},
		{"a newer version", func(pl *sample.Plan) { pl.Version = intp(2) }, "version", "2 is not supported; this reader knows versions 0 and 1"},
		{"no sheets", func(pl *sample.Plan) { pl.Sheets = []sample.PlanSheet{} }, "sheets", "are missing"},
		{"sheets out of order", func(pl *sample.Plan) { pl.Sheets[1].Number = 3 }, "sheets[1].number", "numbered in order from 1"},
		{"a sheet without lines", func(pl *sample.Plan) { pl.Sheets[0].Lines = []string{} }, "sheets[0].lines", "are missing"},
		{"an answer of no line", func(pl *sample.Plan) { pl.Sheets[0].Answer = 4 }, "sheets[0].answer", "4 is not a line of the sheet, 1…3"},
		{"no replacement", func(pl *sample.Plan) { pl.Sheets[3].Erasure.Replacement = "" }, "sheets[3].erasure.replacement", "is missing"},
		{"a replacement that is the erased text", func(pl *sample.Plan) { pl.Sheets[3].Erasure.Replacement = "4 + 1 = 5" }, "sheets[3].erasure.replacement", "is the erased text"},
		{"a replacement that is not its line", func(pl *sample.Plan) { pl.Sheets[3].Erasure.Replacement = "Ответ: 5" }, "sheets[3].erasure.replacement", `is not line 2, "Ответ: 4"`},
		{"a part that is not in its line", func(pl *sample.Plan) { pl.Sheets[2].Erasure.Replacement = "65" }, "sheets[2].erasure.replacement", "is not a part of line 1"},
		{"a part that is the whole line", func(pl *sample.Plan) { pl.Sheets[2].Erasure.Replacement = "47 + 19 = 66" }, "sheets[2].erasure.replacement", "is not a part of line 1"},
		{"a part that the line holds twice", func(pl *sample.Plan) { pl.Sheets[2].Erasure.Replacement = "6" }, "sheets[2].erasure.replacement", "that the line holds once"},
		// What a plan sheet cannot hold: no illegible or crossed-out line, no injection, no sheet
		// without an answer line.
		{"a sheet of illegible lines", func(pl *sample.Plan) { pl.Sheets[0].Scenario = sample.ScenarioIllegible }, "sheets[0].scenario", "cannot be planned: a plan sheet marks no line illegible"},
		{"a nearly empty sheet", func(pl *sample.Plan) {
			pl.Sheets[0].Scenario = sample.ScenarioNearlyEmpty
			pl.Sheets[0].Expected.Outcome = sample.OutcomeNoAnswer
		}, "sheets[0].scenario", "cannot be planned: a plan sheet always has an answer line"},
		{"a sheet with an injection", func(pl *sample.Plan) { pl.Sheets[0].Scenario = sample.ScenarioInjection }, "sheets[0].scenario", "cannot be planned: a plan sheet holds no injection"},
		{"an unreadable sheet", func(pl *sample.Plan) {
			pl.Sheets[0].Scenario = sample.ScenarioSimilarDigits
			pl.Sheets[0].Expected.Outcome = sample.OutcomeUnreadable
		}, "sheets[0].expected.outcome", "cannot be planned: a plan sheet marks no line illegible"},
		{"a sheet with no answer", func(pl *sample.Plan) {
			pl.Sheets[0].Scenario = sample.ScenarioSimilarDigits
			pl.Sheets[0].Expected.Outcome = sample.OutcomeNoAnswer
		}, "sheets[0].expected.outcome", "cannot be planned: a plan sheet always has an answer line"},
		// The rules of a sample's markup, through the stand-in.
		{"no task", func(pl *sample.Plan) { pl.Sheets[0].Task = "" }, "sheets[0].task", "is missing"},
		{"an unknown scenario", func(pl *sample.Plan) { pl.Sheets[0].Scenario = "typo" }, "sheets[0].scenario", `"typo"`},
		{"a line without its text", func(pl *sample.Plan) { pl.Sheets[1].Lines[2] = "" }, "sheets[1].lines[2]", "is missing"},
		{"an erasure outside its scenarios", func(pl *sample.Plan) { pl.Sheets[1].Erasure = pl.Sheets[3].Erasure }, "sheets[1].erasure", "only the scenarios"},
		{"an erasure scenario without its erasure", func(pl *sample.Plan) { pl.Sheets[2].Erasure = nil }, "sheets[2].erasure", "one planned erasure, not 0"},
		{"an erasure of another kind", func(pl *sample.Plan) { pl.Sheets[3].Erasure.Erased = sample.ErasedMistake }, "sheets[3].erasure.erased", `erases a "correct-step"`},
		{"an erasure of no line", func(pl *sample.Plan) { pl.Sheets[3].Erasure.Line = 3 }, "sheets[3].erasure.line", "3 is not a line"},
		{"an erasure without its text", func(pl *sample.Plan) { pl.Sheets[3].Erasure.Text = "" }, "sheets[3].erasure.text", "is missing"},
		{"an unknown outcome", func(pl *sample.Plan) { pl.Sheets[0].Expected.Outcome = "maybe" }, "sheets[0].expected.outcome", `"maybe"`},
		{"an outcome against the scenario", func(pl *sample.Plan) { pl.Sheets[1].Expected = sample.Expected{Outcome: sample.OutcomeCorrect} }, "sheets[1].expected.outcome", `the scenario "error" is "error"`},
		{"an error without its line", func(pl *sample.Plan) { pl.Sheets[1].Expected.FirstError = nil }, "sheets[1].expected.first_error", "is missing"},
		{"a first error on a correct sheet", func(pl *sample.Plan) { pl.Sheets[0].Expected.FirstError = intp(1) }, "sheets[0].expected.first_error", "only a sheet"},
		{"a first error of no line", func(pl *sample.Plan) { pl.Sheets[1].Expected.FirstError = intp(5) }, "sheets[1].expected.first_error", "5 is not a line"},
		{"a trap on a correct sheet", func(pl *sample.Plan) { pl.Sheets[0].Expected.Trap = "sign-on-transfer" }, "sheets[0].expected.trap", "only a sheet"},
		{"the answer last on a sheet of answer-not-last", func(pl *sample.Plan) { pl.Sheets[0].Scenario = sample.ScenarioAnswerNotLast }, "sheets[0].answer", "a line follows the answer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pl := testPlan()
			tt.change(pl)
			wantProblem(t, pl.Check(), tt.path, tt.want)
			// The reader refuses the file the same way.
			data, err := sample.Marshal(pl)
			if err != nil {
				t.Fatal(err)
			}
			_, err = sample.ReadPlan(bytes.NewReader(data))
			wantProblem(t, err, tt.path, tt.want)
		})
	}
}

// TestPlanCheckNamesACauseOnce: a plan sheet without lines, or with an answer of no line, gives its
// stand-in no strokes or no answer, and the stand-in's check does not name that again; a sheet a
// plan cannot hold is not checked as a sample at all.
func TestPlanCheckNamesACauseOnce(t *testing.T) {
	for name, change := range map[string]func(sh *sample.PlanSheet){
		"no lines":             func(sh *sample.PlanSheet) { sh.Lines = []string{} },
		"an answer of no line": func(sh *sample.PlanSheet) { sh.Answer = 0 },
		"an injection":         func(sh *sample.PlanSheet) { sh.Scenario = sample.ScenarioInjection },
		"an unreadable answer": func(sh *sample.PlanSheet) {
			sh.Scenario, sh.Expected.Outcome = sample.ScenarioSimilarDigits, sample.OutcomeUnreadable
		},
	} {
		t.Run(name, func(t *testing.T) {
			pl := testPlan()
			change(&pl.Sheets[0])
			var fe *sample.FormatError
			if err := pl.Check(); !errors.As(err, &fe) || len(fe.Problems) != 1 {
				t.Errorf("Check() = %v, want one problem", err)
			}
		})
	}
}

func TestReadPlanIsStrict(t *testing.T) {
	good, err := sample.Marshal(testPlan())
	if err != nil {
		t.Fatal(err)
	}
	text := string(good)
	lines := regexp.MustCompile(`"lines": \[[^\]]*\]`).FindStringIndex(text) // the lines of the first sheet
	tests := []struct {
		name string
		data string
		path string
		want string
	}{
		{"a missing field", strings.Replace(text, `,
        "part_of_line": true`, "", 1), "sheets[2].erasure.part_of_line", "is missing"},
		{"a field spelt in another case", strings.Replace(text, `"answer": `, `"Answer": `, 1), "sheets[0]", `unknown field "Answer"`},
		{"a null where a value is required", text[:lines[0]] + `"lines": null` + text[lines[1]:], "sheets[0].lines", "is null"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := sample.ReadPlan(strings.NewReader(tt.data))
			wantProblem(t, err, tt.path, tt.want)
		})
	}
}

func TestPlanCheckWithBank(t *testing.T) {
	bank := synth.Bank()
	pl := testPlan()
	pl.Sheets[0].Task = "no-such-task"
	wantProblem(t, pl.CheckWithBank(bank), "sheets[0].task", "not in the bank")
	pl = testPlan()
	pl.Sheets[1].Expected.Trap = "no-such-trap"
	wantProblem(t, pl.CheckWithBank(bank), "sheets[1].expected.trap", "not a trap of the task")
}
