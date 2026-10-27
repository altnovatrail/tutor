package sample_test

import (
	"bytes"
	"errors"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/internal/sample"
)

// The golden plan of version 1 and the files of the measurement, as seen from this package.
const (
	partsPlanFile = "testdata/parts-plan.json"
	freeBankFile  = "../../eval/free-bank.json"
	linesPlanFile = "../../eval/sheets.json"
)

func text(place sample.Place, t string) sample.PlanPart {
	return sample.PlanPart{Kind: sample.PartText, Place: place, Text: t}
}

func column(top, op, bottom, result string) sample.PlanPart {
	p := text(sample.PlaceRight, top+" "+op+" "+bottom+" = "+result)
	p.Figure.Column = &sample.Column{Top: top, Op: op, Bottom: bottom, Result: result}
	return p
}

func drawing(place sample.Place, t string, f sample.Figure) sample.PlanPart {
	return sample.PlanPart{Kind: sample.PartDrawing, Place: place, Text: t, Figure: f}
}

// testPartsPlan returns a new plan of version 1 on tasks of the bank of free solutions, the one
// in testdata/parts-plan.json: a number line with a jump erased and drawn again, a row of houses
// whose first error is in the drawing, a scheme signed with □ and △, and calculations in columns,
// one number of which is erased.
func testPartsPlan() *sample.Plan {
	v := sample.PlanParts
	pl := &sample.Plan{Format: sample.PlanFormat, Version: &v, Sheets: []sample.PlanSheet{
		{
			Number: 1, Task: "grasshopper", Scenario: sample.ScenarioErasureFixed, Answer: 4,
			Expected: sample.Expected{Outcome: sample.OutcomeCorrect},
			Erasure: &sample.PlanErasure{Line: 1, Erased: sample.ErasedMistake, Text: "7 → 12", Replacement: "7 → 11", PartOfLine: true,
				Figure: &sample.Figure{NumberLine: &sample.NumberLine{From: 0, To: 16, Step: 1, Labels: []int{0, 3, 7, 12}, Jumps: []sample.Jump{{From: 3, To: 7}, {From: 7, To: 12}}}}},
		},
		{
			Number: 2, Task: "houses", Scenario: sample.ScenarioError, Answer: 3,
			Expected: sample.Expected{Outcome: sample.OutcomeError, FirstError: intp(1), Trap: "place-for-count"},
		},
		{
			Number: 3, Task: "stamps", Scenario: sample.ScenarioCorrect, Answer: 4,
			Expected: sample.Expected{Outcome: sample.OutcomeCorrect},
		},
		{
			Number: 4, Task: "album", Scenario: sample.ScenarioErasureBroken, Answer: 5,
			Expected: sample.Expected{Outcome: sample.OutcomeError, FirstError: intp(3), Trap: "arithmetic"},
			Erasure:  &sample.PlanErasure{Line: 3, Erased: sample.ErasedCorrectStep, Text: "70", Replacement: "60", PartOfLine: true},
		},
	}}
	pl.Sheets[0].SetParts([]sample.PlanPart{
		drawing(sample.PlaceWide, "числовая прямая: прыжки вправо 3 → 7 → 11 → 15 и прыжок влево 15 → 10", sample.Figure{NumberLine: &sample.NumberLine{
			From: 0, To: 16, Step: 1, Labels: []int{0, 3, 7, 10, 11, 15},
			Jumps: []sample.Jump{{From: 3, To: 7}, {From: 7, To: 11}, {From: 11, To: 15}, {From: 15, To: 10}},
		}}),
		text(sample.PlaceLeft, "3 + 4 + 4 + 4 = 15"),
		text(sample.PlaceLeft, "15 − 5 = 10"),
		text(sample.PlaceLeft, "Ответ: в точке 10"),
	})
	pl.Sheets[1].SetParts([]sample.PlanPart{
		drawing(sample.PlaceWide, "ряд домиков — 4 слева от домика с флажком, он сам и 5 справа", sample.Figure{Row: &sample.Row{Object: sample.ObjectHouse, Left: 4, Right: 5, Numbers: true}}),
		text(sample.PlaceLeft, "4 + 1 + 5 = 10"),
		text(sample.PlaceLeft, "Ответ: 10 домиков"),
	})
	pl.Sheets[2].SetParts([]sample.PlanPart{
		drawing(sample.PlaceLeft, "схема: отрезок младшего □ и отрезок старшего △, длиннее на 6, — вместе 20", sample.Figure{Scheme: &sample.Scheme{
			Segments: []sample.Segment{
				{Label: "□", Pieces: []sample.Piece{{Length: 7}}},
				{Label: "△", Pieces: []sample.Piece{{Length: 7}, {Length: 6, Label: "6"}}},
			},
			Total: "20",
		}}),
		text(sample.PlaceLeft, "20 − 6 = 14"),
		text(sample.PlaceLeft, "14 : 2 = 7"),
		text(sample.PlaceLeft, "Ответ: 7 марок"),
	})
	pl.Sheets[3].SetParts([]sample.PlanPart{
		column("14", "×", "3", "42"),
		column("42", "×", "2", "84"),
		column("14", "×", "5", "60"),
		text(sample.PlaceLeft, "84 + 60 = 144"),
		text(sample.PlaceLeft, "Ответ: 144 рубля"),
	})
	return pl
}

// The golden file is the plan written, byte for byte, and reads back as the plan, with the texts
// of its parts as the lines of its sheets.
func TestPartsPlanFile(t *testing.T) {
	golden, err := os.ReadFile(partsPlanFile)
	if err != nil {
		t.Fatal(err)
	}
	data, err := sample.Marshal(testPartsPlan())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, golden) {
		t.Errorf("the plan is written as\n%s\nand %s holds\n%s", data, partsPlanFile, golden)
	}
	got, err := sample.ReadPlan(bytes.NewReader(golden))
	if err != nil {
		t.Fatalf("ReadPlan: %v", err)
	}
	if !reflect.DeepEqual(got, testPartsPlan()) {
		t.Error("the plan read from the file differs from the one written")
	}
	if want := []string{"4 + 1 + 5 = 10", "Ответ: 10 домиков"}; !reflect.DeepEqual(got.Sheets[1].Lines[1:], want) {
		t.Errorf("the lines of sheet 2 are %q, want the texts of its parts", got.Sheets[1].Lines)
	}
	bank, err := os.ReadFile(freeBankFile)
	if err != nil {
		t.Fatal(err)
	}
	b, err := sample.ReadBank(bytes.NewReader(bank))
	if err != nil {
		t.Fatal(err)
	}
	if err := got.CheckWithBank(b); err != nil {
		t.Errorf("CheckWithBank: %v", err)
	}
}

// A plan of version 0 is written as it was before plans of parts: the plan of the measurement of
// the writing process reads and is written back byte for byte.
func TestLinesPlanFileUnchanged(t *testing.T) {
	file, err := os.ReadFile(linesPlanFile)
	if err != nil {
		t.Fatal(err)
	}
	pl, err := sample.ReadPlan(bytes.NewReader(file))
	if err != nil {
		t.Fatal(err)
	}
	if pl.HasParts() || pl.Sheets[0].Parts != nil {
		t.Error("a plan of version 0 has parts")
	}
	data, err := sample.Marshal(pl)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, file) {
		t.Errorf("%s is written back otherwise", linesPlanFile)
	}
}

// A line or lines, as a word alone: the messages of a plan of parts speak of parts, and of the
// number line as a drawing.
var lineWord = regexp.MustCompile(`\blines?\b`)

func TestCheckRefusesBrokenPartsPlans(t *testing.T) {
	nl := func(pl *sample.Plan) *sample.NumberLine { return pl.Sheets[0].Parts[0].Figure.NumberLine }
	scheme := func(pl *sample.Plan) *sample.Scheme { return pl.Sheets[2].Parts[0].Figure.Scheme }
	tests := []struct {
		name   string
		change func(pl *sample.Plan)
		path   string
		want   string
		// memory marks what only a plan in memory can get wrong: the file of its version has no place
		// for it, so the plan read back is right.
		memory bool
	}{
		// Parts and their figures.
		{"an unknown kind", func(pl *sample.Plan) { pl.Sheets[0].Parts[1].Kind = "note" }, "sheets[0].parts[1].kind", `"note" is not "text" or "drawing"`, false},
		{"an unknown place", func(pl *sample.Plan) { pl.Sheets[0].Parts[1].Place = "middle" }, "sheets[0].parts[1].place", `"middle" is not "left", "right" or "wide"`, false},
		{"a drawing without its figure", func(pl *sample.Plan) { pl.Sheets[0].Parts[0].Figure.NumberLine = nil }, "sheets[0].parts[0].figure", "sets 0 of number_line, row and scheme", false},
		{"a drawing of two figures", func(pl *sample.Plan) { pl.Sheets[0].Parts[0].Figure.Row = &sample.Row{Object: sample.ObjectHouse} }, "sheets[0].parts[0].figure", "sets 2 of number_line, row and scheme", false},
		{"text with a drawing", func(pl *sample.Plan) {
			pl.Sheets[0].Parts[1].Figure.Row = &sample.Row{Object: sample.ObjectHouse}
		}, "sheets[0].parts[1].figure", "sets a drawing, but the part is text", false},
		{"a drawing with a column", func(pl *sample.Plan) {
			pl.Sheets[0].Parts[0].Figure.Column = &sample.Column{Top: "1", Op: "+", Bottom: "1", Result: "2"}
		}, "sheets[0].parts[0].figure.column", "is set, but the part is a drawing", false},
		{"a line of no step", func(pl *sample.Plan) { nl(pl).Step = 0 }, "sheets[0].parts[0].figure.number_line.step", "0 is below 1", false},
		{"a line that ends before it starts", func(pl *sample.Plan) { nl(pl).To = 0 }, "sheets[0].parts[0].figure.number_line.to", "0 is not above from, 0", false},
		{"a line that ends off a division", func(pl *sample.Plan) { nl(pl).Step = 3 }, "sheets[0].parts[0].figure.number_line.to", "16 is not a division: the number line goes from 0 by 3", false},
		{"a line too long for the sheet", func(pl *sample.Plan) { nl(pl).To = 40 }, "sheets[0].parts[0].figure.number_line", "has more than 31 divisions", false},
		// Too long and off a division at once: the check names both.
		{"a line too long that ends off a division", func(pl *sample.Plan) { nl(pl).To, nl(pl).Step = 100, 3 }, "sheets[0].parts[0].figure.number_line.to", "100 is not a division", false},
		{"a label off the line", func(pl *sample.Plan) { nl(pl).Labels[5] = 17 }, "sheets[0].parts[0].figure.number_line.labels[5]", "17 is not a division of the number line, from 0 to 16 by 1", false},
		{"a label twice", func(pl *sample.Plan) { nl(pl).Labels[5] = 11 }, "sheets[0].parts[0].figure.number_line.labels[5]", "11 is signed twice", false},
		{"a jump off the line", func(pl *sample.Plan) { nl(pl).Jumps[3].To = -5 }, "sheets[0].parts[0].figure.number_line.jumps[3].to", "-5 is not a division", false},
		{"a jump to where it starts", func(pl *sample.Plan) { nl(pl).Jumps[3].To = 15 }, "sheets[0].parts[0].figure.number_line.jumps[3]", "starts and ends at 15", false},
		{"a row of an unknown object", func(pl *sample.Plan) { pl.Sheets[1].Parts[0].Figure.Row.Object = "tree" }, "sheets[1].parts[0].figure.row.object", `"tree" is not "house"`, false},
		{"a row with fewer than none to the left", func(pl *sample.Plan) { pl.Sheets[1].Parts[0].Figure.Row.Left = -1 }, "sheets[1].parts[0].figure.row.left", "-1 is below 0", false},
		{"a scheme without segments", func(pl *sample.Plan) { scheme(pl).Segments = []sample.Segment{} }, "sheets[2].parts[0].figure.scheme.segments", "are missing", false},
		{"a segment without pieces", func(pl *sample.Plan) { scheme(pl).Segments[0].Pieces = []sample.Piece{} }, "sheets[2].parts[0].figure.scheme.segments[0].pieces", "are missing", false},
		{"a piece of no length", func(pl *sample.Plan) { scheme(pl).Segments[1].Pieces[1].Length = 0 }, "sheets[2].parts[0].figure.scheme.segments[1].pieces[1].length", "0 is below 1", false},
		{"a segment signed with a word", func(pl *sample.Plan) { scheme(pl).Segments[0].Label = "мл." }, "sheets[2].parts[0].figure.scheme.segments[0].label", `"мл." is not a number, □ or △`, false},
		{"a brace signed with a question mark", func(pl *sample.Plan) { scheme(pl).Total = "?" }, "sheets[2].parts[0].figure.scheme.total", `"?" is not a number, □ or △`, false},
		{"a column with a hyphen for minus", func(pl *sample.Plan) { pl.Sheets[3].Parts[0].Figure.Column.Op = "-" }, "sheets[3].parts[0].figure.column.op", `"-" (U+002D) is not "+" (U+002B), "−" (U+2212) or "×" (U+00D7)`, false},
		{"a column of a number with a letter", func(pl *sample.Plan) { pl.Sheets[3].Parts[0].Figure.Column.Top = "1a" }, "sheets[3].parts[0].figure.column.top", `"1a" is not a number written in digits`, false},
		{"a column that is not its text", func(pl *sample.Plan) { pl.Sheets[3].Parts[0].Figure.Column.Result = "43" }, "sheets[3].parts[0].text", `"14 × 3 = 42" is not its column written out, "14 × 3 = 43"`, false},
		{"an answer in a drawing", func(pl *sample.Plan) { pl.Sheets[1].Answer = 1 }, "sheets[1].answer", "part 1 is a drawing: the answer is written in text", false},
		// The rules of a sample's markup, over the parts.
		{"a part without its text", func(pl *sample.Plan) { pl.Sheets[0].Parts[1].Text = ""; pl.Sheets[0].Lines[1] = "" }, "sheets[0].parts[1].text", "is missing", false},
		{"a first error of no part", func(pl *sample.Plan) { pl.Sheets[1].Expected.FirstError = intp(5) }, "sheets[1].expected.first_error", "5 is not a part of the sheet, 1…3", false},
		{"an erasure of no part", func(pl *sample.Plan) { pl.Sheets[3].Erasure.Line = 9 }, "sheets[3].erasure.line", "9 is not a part of the sheet, 1…5", false},
		{"an answer of no part", func(pl *sample.Plan) { pl.Sheets[0].Answer = 5 }, "sheets[0].answer", "5 is not a part of the sheet, 1…4", false},
		{"a sheet without parts", func(pl *sample.Plan) { pl.Sheets[1].SetParts([]sample.PlanPart{}) }, "sheets[1].parts", "are missing: a sheet has at least one part", false},
		{"the answer last on a sheet of answer-not-last", func(pl *sample.Plan) { pl.Sheets[2].Scenario = sample.ScenarioAnswerNotLast }, "sheets[2].answer", "a part follows the answer", false},
		{"a sheet of illegible parts", func(pl *sample.Plan) { pl.Sheets[2].Scenario = sample.ScenarioIllegible }, "sheets[2].scenario", "a plan sheet marks no part illegible or crossed out", false},
		// Erasures in a drawing and in a column.
		{"an erasure in a drawing without the figure before it", func(pl *sample.Plan) { pl.Sheets[0].Erasure.Figure = nil }, "sheets[0].erasure.figure", "is missing: an erasure in the drawing of part 1", false},
		{"an erasure of all of a drawing", func(pl *sample.Plan) {
			e := pl.Sheets[0].Erasure
			e.PartOfLine, e.Text, e.Replacement = false, "числовая прямая: прыжки вправо 3 → 7 → 12", pl.Sheets[0].Lines[0]
		}, "sheets[0].erasure.part_of_line", "is false, but an erasure in a drawing takes some of its elements", false},
		{"a figure before an erasure in text", func(pl *sample.Plan) {
			pl.Sheets[3].Erasure.Figure = &sample.Figure{Row: &sample.Row{Object: sample.ObjectHouse}}
		}, "sheets[3].erasure.figure", "is set, but part 3 is text", false},
		{"a figure before of another drawing", func(pl *sample.Plan) {
			pl.Sheets[0].Erasure.Figure = &sample.Figure{Row: &sample.Row{Object: sample.ObjectHouse, Left: 1}}
		}, "sheets[0].erasure.figure", "is a row, and part 1 a number_line", false},
		{"a figure before on another frame", func(pl *sample.Plan) { pl.Sheets[0].Erasure.Figure.NumberLine.To = 20 }, "sheets[0].erasure.figure", "is on another frame than part 1", false},
		{"a figure before that is the figure after", func(pl *sample.Plan) {
			f := *pl.Sheets[0].Parts[0].Figure.NumberLine
			pl.Sheets[0].Erasure.Figure = &sample.Figure{NumberLine: &f}
		}, "sheets[0].erasure.figure", "is the figure of part 1: an erasure in a drawing changes it", false},
		{"a figure before that is broken", func(pl *sample.Plan) { pl.Sheets[0].Erasure.Figure.NumberLine.Jumps[1].To = 17 }, "sheets[0].erasure.figure.number_line.jumps[1].to", "17 is not a division", false},
		{"a drawn element the description does not hold", func(pl *sample.Plan) { pl.Sheets[0].Erasure.Replacement = "7 → 13" }, "sheets[0].erasure.replacement", `is not a piece of part 1`, false},
		{"a number the column does not hold", func(pl *sample.Plan) { pl.Sheets[3].Erasure.Replacement = "6" }, "sheets[3].erasure.replacement", `"6" is not a number that the column of part 3 holds once`, false},
		{"an erasure of all of a column", func(pl *sample.Plan) {
			e := pl.Sheets[3].Erasure
			e.PartOfLine, e.Text, e.Replacement = false, "14 × 5 = 70", pl.Sheets[3].Lines[2]
		}, "sheets[3].erasure.part_of_line", "is false, but an erasure in a column takes one of its numbers", false},
		{"an erased word in a column", func(pl *sample.Plan) { pl.Sheets[3].Erasure.Text = "seventy" }, "sheets[3].erasure.text", `"seventy" is not a number`, false},
		{"a row too long for the sheet", func(pl *sample.Plan) { pl.Sheets[1].Parts[0].Figure.Row.Right = 1000000000 }, "sheets[1].parts[0].figure.row", "holds more than 15 objects", false},
		{"a scheme of too many segments", func(pl *sample.Plan) {
			for range 5 {
				scheme(pl).Segments = append(scheme(pl).Segments, sample.Segment{Pieces: []sample.Piece{{Length: 1}}})
			}
		}, "sheets[2].parts[0].figure.scheme.segments", "are 7: a sheet holds 6 at most", false},
		{"a piece too long for the sheet", func(pl *sample.Plan) { scheme(pl).Segments[0].Pieces[0].Length = 1000000000 }, "sheets[2].parts[0].figure.scheme.segments[0].pieces[0].length", "1000000000 is above 40", false},
		{"a segment too long for the sheet", func(pl *sample.Plan) { scheme(pl).Segments[1].Pieces[0].Length = 35 }, "sheets[2].parts[0].figure.scheme.segments[1]", "is 41 units long: a segment is 40 units long at most", false},
		// What only a plan in memory gets wrong.
		{"lines that are not the texts of the parts", func(pl *sample.Plan) { pl.Sheets[0].Lines[1] = "3 + 4 = 7" }, "sheets[0]", "the texts of its parts are not its Lines", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pl := testPartsPlan()
			tt.change(pl)
			err := pl.Check()
			wantProblem(t, err, tt.path, tt.want)
			var fe *sample.FormatError
			if errors.As(err, &fe) {
				for _, pr := range fe.Problems {
					if lineWord.MatchString(strings.ReplaceAll(pr.Message, "number line", "")) {
						t.Errorf("a problem of a plan of parts speaks of lines: %s", pr)
					}
				}
			}
			if tt.memory {
				return
			}
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

// A plan of version 0 with what only a plan of parts holds would lose it in its file.
func TestCheckRefusesPartsInLinesPlans(t *testing.T) {
	pl := testPlan()
	pl.Sheets[0].SetParts([]sample.PlanPart{text(sample.PlaceLeft, "3x + 1 = 7"), text(sample.PlaceLeft, "3x = 6"), text(sample.PlaceLeft, "x = 2")})
	wantProblem(t, pl.Check(), "sheets[0]", "has parts, but only a plan of version 1 has parts")
	pl = testPlan()
	pl.Sheets[2].Erasure.Figure = &sample.Figure{Row: &sample.Row{Object: sample.ObjectHouse}}
	wantProblem(t, pl.Check(), "sheets[2].erasure", "has a figure, but only a plan of version 1 has figures")
}

// A sheet of parts without parts, or with an answer of no part, is named once, as a sheet of lines
// is.
func TestPartsPlanCheckNamesACauseOnce(t *testing.T) {
	for name, change := range map[string]func(sh *sample.PlanSheet){
		"no parts":             func(sh *sample.PlanSheet) { sh.SetParts([]sample.PlanPart{}) },
		"an answer of no part": func(sh *sample.PlanSheet) { sh.Answer = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			pl := testPartsPlan()
			change(&pl.Sheets[2])
			var fe *sample.FormatError
			if err := pl.Check(); !errors.As(err, &fe) || len(fe.Problems) != 1 {
				t.Errorf("Check() = %v, want one problem", err)
			}
		})
	}
}

// The reader takes the version the file declares, spelt exactly so and a whole number: a version
// it does not know it names alone, and any other declaration the reader of version 0 words.
func TestReadPlanVersion(t *testing.T) {
	golden, err := os.ReadFile(partsPlanFile)
	if err != nil {
		t.Fatal(err)
	}
	text := string(golden)
	version := `"version": 1,`
	tests := []struct {
		name, data, path, want string
		alone                  bool
	}{
		{"a version to come", strings.Replace(text, version, `"version": 2,`, 1), "version", "2 is not supported; this reader knows versions 0 and 1", true},
		{"a version spelt in another case", strings.Replace(text, version, `"Version": 1,`, 1), "version", "is missing", false},
		// 1.0 is a version of parts written as a fraction: the reader of parts names it alone.
		{"a version with a fraction", strings.Replace(text, version, `"version": 1.0,`, 1), "version", "1.0 is not a whole number", true},
		{"a version to come of another format", strings.Replace(strings.Replace(text, version, `"version": 3,`, 1), `"tutor-sheet-plan"`, `"tutor-bank"`, 1), "format", `is "tutor-bank", not "tutor-sheet-plan"`, false},
		{"a version in quotes", strings.Replace(text, version, `"version": "1",`, 1), "version", "is not a number", false},
		// With no version, a plan is read as one of version 0, and its parts are no field of that.
		{"no version", strings.Replace(text, version, `"version": null,`, 1), "sheets[0]", `unknown field "parts"`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := sample.ReadPlan(strings.NewReader(tt.data))
			wantProblem(t, err, tt.path, tt.want)
			var fe *sample.FormatError
			if tt.alone && (!errors.As(err, &fe) || len(fe.Problems) != 1) {
				t.Errorf("ReadPlan = %v, want the version named alone", err)
			}
		})
	}
}

// A plan of parts is read as strictly as any file: every field of a part, of its figure and of
// the erasure, spelt exactly, and null only where a field may be absent.
func TestReadPartsPlanIsStrict(t *testing.T) {
	golden, err := os.ReadFile(partsPlanFile)
	if err != nil {
		t.Fatal(err)
	}
	text := string(golden)
	tests := []struct {
		name, data, path, want string
	}{
		{"lines in place of parts", strings.Replace(text, `"parts": [`, `"lines": [`, 1), "sheets[0]", `unknown field "lines"`},
		{"a part without its place", strings.Replace(text, `"place": "wide",`, ``, 1), "sheets[0].parts[0].place", "is missing"},
		{"a figure that is null", strings.Replace(text, `"figure": {
            "number_line": null,
            "row": null,
            "scheme": null,
            "column": null
          }`, `"figure": null`, 1), "sheets[0].parts[1].figure", "is null"},
		{"an erasure without its figure", strings.Replace(text, `"part_of_line": true,
        "figure": {`, `"part_of_line": true, "figures": {`, 1), "sheets[0].erasure", `unknown field "figures"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := sample.ReadPlan(strings.NewReader(tt.data))
			wantProblem(t, err, tt.path, tt.want)
		})
	}
}
