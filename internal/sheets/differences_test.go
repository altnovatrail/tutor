package sheets_test

import (
	"reflect"
	"testing"

	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/sheets"
)

// box is a stroke of two points across the box from (x0, y0) to (x1, y1), drawn from t0 on.
func box(id string, x0, y0, x1, y1 float64, t0 int64) sample.Stroke {
	return sample.Stroke{ID: id, Points: []sample.Point{{X: x0, Y: y0, TMs: t0, Pressure: 0.5}, {X: x1, Y: y1, TMs: t0 + 100, Pressure: 0.5}}}
}

// On a sheet of a plan of parts, the planned erasure took its strokes from where they stood on the
// plane, and the notes name parts: a number erased in a column at the side, at the height of a line
// of the main column, stood on the column, which bands down the sheet cannot tell.
func TestDifferencesOnASheetOfParts(t *testing.T) {
	sh := &sample.PlanSheet{Number: 1, Task: "pages", Scenario: sample.ScenarioErasureFixed, Answer: 3, Expected: sample.Expected{Outcome: sample.OutcomeCorrect}}
	sh.SetParts([]sample.PlanPart{
		{Kind: sample.PartText, Place: sample.PlaceLeft, Text: "с 1 по 9: 9 цифр"},
		{Kind: sample.PartText, Place: sample.PlaceRight, Text: "75 − 9 = 66", Figure: sample.Figure{Column: &sample.Column{Top: "75", Op: "−", Bottom: "9", Result: "66"}}},
		{Kind: sample.PartText, Place: sample.PlaceLeft, Text: "Ответ: 141 цифра"},
	})
	sh.Erasure = &sample.PlanErasure{Line: 2, Erased: sample.ErasedMistake, Text: "10", Replacement: "9", PartOfLine: true}
	erased := func(x float64) *sample.Sample {
		return &sample.Sample{
			Strokes: []sample.Stroke{
				box("line", 100, 100, 400, 130, 0),
				box("top", 700, 100, 740, 130, 3000),
				box("old", x, 100, x+30, 130, 3300),
				box("bottom", 710, 140, 740, 170, 6000),
				box("result", 700, 190, 740, 220, 6400),
				box("answer", 100, 300, 400, 330, 9000),
			},
			Erasures: []sample.Erasure{{TMs: 5000, Strokes: []string{"old"}, Planned: &sample.PlannedErasure{Line: 2, Erased: sample.ErasedMistake, Text: "10"}}},
			Lines: []sample.Line{
				{Text: sh.Lines[0], Strokes: []string{"line"}},
				{Text: sh.Lines[1], Strokes: []string{"top", "bottom", "result"}},
				{Text: sh.Lines[2], Strokes: []string{"answer"}, Answer: true},
			},
			Expected: sh.Expected,
		}
	}
	if diffs := sheets.Differences(sh, erased(745)); len(diffs) != 0 {
		t.Errorf("an erasure in the column: %+v, want none", diffs)
	}
	want := []sample.Problem{{Path: "erasures[0].planned.line", Message: "is 2, but the erasure took its strokes from part 1"}}
	if diffs := sheets.Differences(sh, erased(380)); !reflect.DeepEqual(diffs, want) {
		t.Errorf("an erasure in the main column: %+v, want %+v", diffs, want)
	}
	s := erased(745)
	s.Lines[2].Answer, s.Lines[0].Answer = false, true
	want = []sample.Problem{{Path: "lines", Message: "give the answer on part 1, and the plan sheet on part 3"}}
	if diffs := sheets.Differences(sh, s); !reflect.DeepEqual(diffs, want) {
		t.Errorf("the answer moved: %+v, want %+v", diffs, want)
	}
}
