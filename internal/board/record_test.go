package board

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/synth"
)

func intp(n int) *int { return &n }

// recordPlan returns a new plan of three sheets on the synthetic bank: a correct one, one with an
// error, and one with a planned erasure.
func recordPlan() *sample.Plan {
	v := sample.Version
	return &sample.Plan{Format: sample.PlanFormat, Version: &v, Sheets: []sample.PlanSheet{
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
	}}
}

// recorder starts the record mode of recordPlan in a new directory, its clock standing at one
// moment, and returns it with its handler.
func recorder(t *testing.T) (*Recorder, http.Handler) {
	t.Helper()
	rc, err := NewRecorder(synth.Bank(), recordPlan(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rc.now = func() time.Time { return time.Date(2026, 10, 26, 10, 15, 30, 123_456_789, time.UTC) }
	return rc, rc.Handler(boardAddr)
}

// written is a plan sheet written the way the record page would send it: three short strokes to a
// line, a line every 90 units down the board's sheet, the clock running, and the plan's markup.
func written(sh sample.PlanSheet) *sample.Sample {
	v := sample.Version
	s := &sample.Sample{Format: sample.SampleFormat, Version: &v, Sheet: sample.Sheet{Width: sheetWidth, Height: sheetHeight},
		Task: sh.Task, Writer: sample.WriterAuthor, Scenario: sh.Scenario,
		Strokes: []sample.Stroke{}, Erasures: []sample.Erasure{}, Lines: []sample.Line{}, Expected: sh.Expected}
	var clock int64
	for i, text := range sh.Lines {
		y := 40 + 90*float64(i)
		var ids []string
		for j := range 3 {
			id := fmt.Sprintf("s-%d", len(s.Strokes)+1)
			x := 40 + 30*float64(j)
			s.Strokes = append(s.Strokes, sample.Stroke{ID: id, Points: []sample.Point{
				{X: x, Y: y, TMs: clock, Pressure: 0.4},
				{X: x + 8, Y: y + 15, TMs: clock + 8, Pressure: 0.6},
				{X: x + 12, Y: y + 30, TMs: clock + 16, Pressure: 0.5},
			}})
			ids = append(ids, id)
			clock += 300
		}
		s.Lines = append(s.Lines, sample.Line{Text: text, Strokes: ids, Answer: i+1 == sh.Answer})
	}
	return s
}

// erasing adds to a written sheet the erasure its plan sheet plans: a stroke on the plan's line,
// written after the rest, and one move of the eraser that takes it away, with the plan's markup.
func erasing(s *sample.Sample, sh sample.PlanSheet) *sample.Sample {
	last := s.Strokes[len(s.Strokes)-1].Points
	at := last[len(last)-1].TMs + 300
	y := 40 + 90*float64(sh.Erasure.Line-1) + 10
	s.Strokes = append(s.Strokes, sample.Stroke{ID: "s-erased", Points: []sample.Point{
		{X: 300, Y: y, TMs: at, Pressure: 0.5}, {X: 330, Y: y + 5, TMs: at + 20, Pressure: 0.5}}})
	s.Erasures = append(s.Erasures, sample.Erasure{TMs: at + 500, Strokes: []string{"s-erased"},
		Planned: &sample.PlannedErasure{Line: sh.Erasure.Line, Erased: sh.Erasure.Erased, Text: sh.Erasure.Text}})
	return s
}

// slip adds a stroke at the height y, written after the rest, and an erasure that takes it away.
func slip(s *sample.Sample, id string, y float64) *sample.Sample {
	last := s.Strokes[len(s.Strokes)-1].Points
	at := last[len(last)-1].TMs
	for _, e := range s.Erasures {
		at = max(at, e.TMs)
	}
	at += 300
	s.Strokes = append(s.Strokes, sample.Stroke{ID: id, Points: []sample.Point{{X: 600, Y: y, TMs: at, Pressure: 0.5}}})
	s.Erasures = append(s.Erasures, sample.Erasure{TMs: at + 100, Strokes: []string{id}})
	return s
}

// raw is a sample as JSON, without spaces.
func raw(t *testing.T, s *sample.Sample) string {
	t.Helper()
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// saveBody is the body of a save of a sample's JSON for a plan sheet.
func saveBody(number int, sampleJSON string) SaveRequest {
	return SaveRequest{Number: number, Sample: json.RawMessage(sampleJSON)}
}

// files lists the names in a directory.
func files(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// noted fails unless notes, as JSON, hold one at path whose message holds want.
func noted(t *testing.T, notes any, path, want string) {
	t.Helper()
	list, _ := notes.([]any)
	for _, n := range list {
		if m := n.(map[string]any); m["path"] == path && strings.Contains(fmt.Sprint(m["message"]), want) {
			return
		}
	}
	t.Errorf("no note at %q holding %q in %v", path, want, notes)
}

// The record page is the board's sheet, 1000 × 600, in English but for the statement, which is in
// Russian. Every field of its markup has a label, and no canvas is hidden from a screen reader:
// SonarCloud reads the page as it stands (Web:S6825).
func TestRecordPage(t *testing.T) {
	_, h := recorder(t)
	rec, _ := call(t, h, "GET", "/", nil)
	page := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") || !strings.Contains(page, "<title>Record sheets</title>") {
		t.Fatalf("GET / = %d %s, want the record page", rec.Code, rec.Header().Get("Content-Type"))
	}
	// A hidden part of the form stays hidden whatever its class lays out.
	for _, want := range []string{`<html lang="en">`, `<canvas id="sheet" width="1000" height="600"`, `<canvas id="bands" width="1000" height="600">`,
		`[hidden] { display: none !important; }`} {
		if !strings.Contains(page, want) {
			t.Errorf("the page does not hold %s", want)
		}
	}
	if !regexp.MustCompile(`<p id="statement"[^>]*\blang="ru"`).MatchString(page) {
		t.Error("the statement is not marked as Russian")
	}
	if strings.Contains(page, "aria-hidden") {
		t.Error("the page hides an element from a screen reader")
	}
	fields := regexp.MustCompile(`<(?:input|select|textarea)\b[^>]*\bid="([^"]+)"[^>]*>`).FindAllStringSubmatch(page, -1)
	if len(fields) < 3 {
		t.Fatalf("the page's markup holds %d fields, want the outcome, the first error and the trap", len(fields))
	}
	for _, f := range fields {
		if !strings.Contains(page, `<label for="`+f[1]+`"`) && !strings.Contains(f[0], "aria-label=") {
			t.Errorf("the field %s has no label", f[1])
		}
	}
}

// The plan view holds every plan sheet with the statement of its task, whether the board can save
// it yet, and how many times it has been saved; the files it counts are the board's own.
func TestRecordPlanView(t *testing.T) {
	rc, h := recorder(t)
	for _, name := range []string{"notes.txt", "1-20261026-101530-123-synthetic-linear-01-correct.json", "01-copy.json",
		"01-20261026-101530-123-synthetic-free-01-correct.json"} {
		if err := os.WriteFile(filepath.Join(rc.dir, name), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for range 2 {
		if rec, _ := call(t, h, "POST", "/api/record/save", saveBody(1, raw(t, written(rc.plan.Sheets[0])))); rec.Code != http.StatusOK {
			t.Fatalf("save = %d %s", rec.Code, rec.Body)
		}
	}
	rec, _ := call(t, h, "GET", "/api/record/plan", nil)
	var view PlanView
	if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("GET /api/record/plan = %d %s", rec.Code, rec.Body)
	}
	if view.Sheet != (sample.Sheet{Width: 1000, Height: 600}) || len(view.Sheets) != 3 {
		t.Fatalf("the plan view = %+v, want the board's sheet and three plan sheets", view)
	}
	var got []string
	for _, sh := range view.Sheets {
		got = append(got, fmt.Sprintf("%d %s %s: %q, saved %d", sh.Number, sh.Task, sh.Scenario, sh.Statement, sh.Saved))
	}
	want := []string{
		`1 synthetic-linear-01 correct: "Реши уравнение 3x + 1 = 7.", saved 2`,
		`2 synthetic-linear-01 error: "Реши уравнение 3x + 1 = 7.", saved 0`,
		`3 synthetic-calculation-01 erasure-fixed: "Вычисли (47 + 19) : 6.", saved 0`,
	}
	if !slices.Equal(got, want) {
		t.Errorf("the plan sheets:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if !reflect.DeepEqual(view.Sheets[1].PlanSheet, rc.plan.Sheets[1]) {
		t.Errorf("plan sheet 2 = %+v, want it as the plan has it", view.Sheets[1].PlanSheet)
	}
	// Each sheet carries its task's traps, for the page's choice of the trap a mistake falls into.
	for _, sh := range view.Sheets {
		if want := synth.Bank().Task(sh.Task).Traps; len(want) == 0 || !reflect.DeepEqual(sh.Traps, want) {
			t.Errorf("sheet %d: traps %+v, want its task's %+v", sh.Number, sh.Traps, want)
		}
	}
	if !strings.Contains(rec.Body.String(), `"traps":[{"id":"sign-on-transfer"`) {
		t.Errorf("GET /api/record/plan = %s, want the traps with their IDs", rec.Body)
	}
}

// The board proposes the lines found among the strokes still on the sheet, with the texts, the
// answer and the outcome of the plan sheet, and names a sheet whose lines are not the plan's.
func TestRecordPropose(t *testing.T) {
	rc, h := recorder(t)
	s := written(rc.plan.Sheets[1])
	rec, _ := call(t, h, "POST", "/api/record/propose", ProposeRequest{Number: 2, Strokes: s.Strokes, Erasures: s.Erasures})
	if rec.Code != http.StatusOK {
		t.Fatalf("propose = %d %s", rec.Code, rec.Body)
	}
	var p Proposal
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.Lines, s.Lines) || !reflect.DeepEqual(p.Expected, s.Expected) || len(p.Differences) != 0 {
		t.Errorf("the proposal = %+v, want the plan's markup on the strokes of each line", p)
	}

	stray := sample.Stroke{ID: "s-stray", Points: []sample.Point{{X: 500, Y: 560, TMs: 9000, Pressure: 0.5}}}
	_, out := call(t, h, "POST", "/api/record/propose", ProposeRequest{Number: 2, Strokes: append(slices.Clone(s.Strokes), stray)})
	if lines := out["lines"].([]any); len(lines) != 5 || lines[4].(map[string]any)["text"] != "" {
		t.Errorf("lines with a stray stroke = %v, want a fifth line without a text", lines)
	}
	noted(t, out["differences"], "sample.lines", "the sheet has 5 lines, and its plan sheet 4: a stray stroke")

	erased := []sample.Erasure{{TMs: 9100, Strokes: []string{"s-stray"}}}
	_, out = call(t, h, "POST", "/api/record/propose", ProposeRequest{Number: 2, Strokes: append(slices.Clone(s.Strokes), stray), Erasures: erased})
	if lines := out["lines"].([]any); len(lines) != 4 || len(out["differences"].([]any)) != 0 {
		t.Errorf("the proposal with the stray stroke erased = %v, want the plan's four lines", out)
	}

	_, out = call(t, h, "POST", "/api/record/propose", ProposeRequest{Number: 2, Strokes: s.Strokes[:9]})
	noted(t, out["differences"], "sample.lines", "the sheet has 3 lines, and its plan sheet 4: a long stroke may join two lines")
	_, out = call(t, h, "POST", "/api/record/propose", ProposeRequest{Number: 2})
	noted(t, out["differences"], "sample.lines", "the sheet has no lines yet")

	rec, out = call(t, h, "POST", "/api/record/propose", ProposeRequest{Number: 4})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("a proposal for no sheet = %d, want 422", rec.Code)
	}
	noted(t, out["problems"], "number", "4 is not a sheet of the plan, 1…3")
}

// For each erasure the board names the line most of its strokes stood on. On a sheet with a planned
// erasure it takes for it the move of the eraser that took the most strokes from the plan's line,
// the earlier of two that took as many, and never what Undo took away. It names a sheet with no
// move of the eraser, and one where more than one took strokes from the plan's line.
func TestRecordProposeErasures(t *testing.T) {
	rc, h := recorder(t)
	sh := rc.plan.Sheets[2]
	propose := func(s *sample.Sample, undone ...bool) (Proposal, map[string]any) {
		t.Helper()
		rec, out := call(t, h, "POST", "/api/record/propose", ProposeRequest{Number: sh.Number, Strokes: s.Strokes, Erasures: s.Erasures, Undone: undone})
		var p Proposal
		if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil || rec.Code != http.StatusOK {
			t.Fatalf("propose = %d %s", rec.Code, rec.Body)
		}
		return p, out
	}
	// erasureLines is the line the board names for each erasure of a sheet, as the page sees it in
	// the answer to a check.
	erasureLines := func(s *sample.Sample) []int {
		t.Helper()
		_, out := call(t, h, "POST", "/api/record/check", saveBody(sh.Number, raw(t, s)))
		var got []int
		for _, n := range out["erasure_lines"].([]any) {
			got = append(got, int(n.(float64)))
		}
		return got
	}

	// The planned erasure on line 1, and a slip below every line and one on line 2.
	three := slip(slip(erasing(written(sh), sh), "s-low", 560), "s-on-2", 150)
	p, _ := propose(three)
	if got := erasureLines(three); !slices.Equal(got, []int{1, 0, 2}) || p.Planned == nil || *p.Planned != 0 || len(p.Differences) != 0 {
		t.Errorf("lines %v, planned %v, differences %v; want lines 1, 0 and 2 and the first planned", got, p.Planned, p.Differences)
	}

	// Two moves of the eraser that took as many strokes from line 1: the earlier is taken, and the
	// sheet is named. Made by Undo, the second does not count.
	two := slip(erasing(written(sh), sh), "s-again", 50)
	p, out := propose(two)
	if p.Planned == nil || *p.Planned != 0 {
		t.Errorf("planned = %v, want the earlier of two moves that took as many strokes from line 1", p.Planned)
	}
	noted(t, out["differences"], "sample.erasures", "2 moves of the eraser took strokes from line 1, and the planned erasure is one move")
	if p, _ := propose(two, false, true); p.Planned == nil || *p.Planned != 0 || len(p.Differences) != 0 {
		t.Errorf("with the second by Undo: planned %v, differences %v; want the first and nothing named", p.Planned, p.Differences)
	}
	if p, _ := propose(two, true, false); p.Planned == nil || *p.Planned != 1 {
		t.Errorf("with the first by Undo: planned %v, want the second", p.Planned)
	}

	// The move that took the most strokes from the plan's line, counted stroke by stroke. Extra
	// strokes on lines 1 and 2 are taken away, and the lines of the strokes left stay the plan's.
	wide := func(moves ...[]string) *sample.Sample {
		s := written(sh)
		at := int64(50_000)
		for _, st := range []struct {
			id string
			y  float64
		}{{"x1", 50}, {"x2", 52}, {"y1", 140}, {"y2", 142}, {"z1", 54}, {"z2", 56}, {"z3", 58}} {
			s.Strokes = append(s.Strokes, sample.Stroke{ID: st.id, Points: []sample.Point{{X: 700, Y: st.y, TMs: at, Pressure: 0.5}}})
			at += 100
		}
		for _, m := range moves {
			at += 100
			s.Erasures = append(s.Erasures, sample.Erasure{TMs: at, Strokes: m})
		}
		return s
	}
	first := wide([]string{"x1", "x2", "y1", "y2"}, []string{"z1", "z2", "z3"})
	if p, _ := propose(first); !slices.Equal(erasureLines(first), []int{1, 1}) || p.Planned == nil || *p.Planned != 1 {
		t.Errorf("lines %v, planned %v; want both on line 1, the first by the higher of two lines as many, and the second, with three strokes from it, planned", erasureLines(first), p.Planned)
	}
	second := wide([]string{"x1", "y1", "y2"}, []string{"z1", "z2"})
	if p, _ := propose(second); !slices.Equal(erasureLines(second), []int{2, 1}) || p.Planned == nil || *p.Planned != 1 {
		t.Errorf("lines %v, planned %v; want the first on line 2 and the second, with two strokes from line 1, planned", erasureLines(second), p.Planned)
	}
	if p, _ := propose(wide([]string{"x1", "x2"}, []string{"z1", "z2"})); p.Planned == nil || *p.Planned != 0 {
		t.Errorf("planned %v; want the earlier of two that took as many", p.Planned)
	}

	// No erasure at all, or only Undo's: no move of the eraser, and the sheet is named. Moves that
	// took nothing from the plan's line: nothing planned, and the page asks to pick one.
	_, out = propose(written(sh))
	noted(t, out["differences"], "sample.erasures", "the sheet has no move of the eraser, and its plan sheet plans an erasure on line 1")
	if out["planned"] != nil {
		t.Errorf("planned = %v on a sheet without erasures, want null", out["planned"])
	}
	_, out = propose(erasing(written(sh), sh), true)
	noted(t, out["differences"], "sample.erasures", "Undo makes slips only")
	elsewhere := written(sh)
	elsewhere.Erasures = []sample.Erasure{{TMs: 100_000, Strokes: []string{"s-6"}}}
	if p, _ := propose(elsewhere); p.Planned != nil || len(p.Differences) != 0 {
		t.Errorf("planned %v, differences %v; want nothing planned and nothing named", p.Planned, p.Differences)
	}

	// A sheet without a planned erasure: its slips have lines, and nothing is planned.
	plain := slip(erasing(written(rc.plan.Sheets[1]), sh), "s-low", 560)
	plain.Erasures[0].Planned = nil
	_, out = call(t, h, "POST", "/api/record/propose", ProposeRequest{Number: 2, Strokes: plain.Strokes, Erasures: plain.Erasures})
	if out["planned"] != nil || len(out["differences"].([]any)) != 0 {
		t.Errorf("the proposal for sheet 2 = %v, want nothing planned and nothing named", out)
	}
}

// A sheet with its planned erasure is saved, and a planned erasure other than the plan's is named.
func TestRecordSaveErasure(t *testing.T) {
	rc, h := recorder(t)
	sh := rc.plan.Sheets[2]
	s := slip(erasing(written(sh), sh), "s-low", 560)
	rec, out := call(t, h, "POST", "/api/record/save", saveBody(3, raw(t, s)))
	if rec.Code != http.StatusOK || out["file"] != "03-20261026-101530-123-synthetic-calculation-01-erasure-fixed.json" || len(out["differences"].([]any)) != 0 {
		t.Fatalf("save = %d %v, want the file of sheet 3 without differences", rec.Code, out)
	}
	data, err := os.ReadFile(filepath.Join(rc.dir, out["file"].(string)))
	if err != nil {
		t.Fatal(err)
	}
	back, err := sample.ReadSample(bytes.NewReader(data))
	if err == nil {
		err = back.CheckWithBank(rc.bank)
	}
	if err != nil || !reflect.DeepEqual(back, s) {
		t.Errorf("the saved file = %v; want the sample sent, passing the check with the bank", err)
	}

	s.Erasures[0].Planned = &sample.PlannedErasure{Line: 2, Erased: sample.ErasedMistake, Text: "57"}
	rec, out = call(t, h, "POST", "/api/record/save", saveBody(3, raw(t, s)))
	if rec.Code != http.StatusOK {
		t.Fatalf("save = %d %s", rec.Code, rec.Body)
	}
	noted(t, out["differences"], "sample.erasures[0].planned.line", "is 2, and the plan sheet's 1")
	noted(t, out["differences"], "sample.erasures[0].planned.text", `is "57", and the plan sheet's "56"`)
	noted(t, out["differences"], "sample.erasures[0].planned.line", "is 2, but the erasure took its strokes from line 1")
	if n := len(out["differences"].([]any)); n != 3 {
		t.Errorf("differences = %v, want three", out["differences"])
	}
}

// A saved sheet is a new file, named by its plan sheet, the time and its task and scenario, that
// only its owner reads, and that reads back as the sample sent and passes the check with the bank.
// A second save is a second file, a millisecond later in its name even when the clock stands still.
func TestRecordSave(t *testing.T) {
	rc, h := recorder(t)
	s := written(rc.plan.Sheets[1])
	rec, out := call(t, h, "POST", "/api/record/save", saveBody(2, raw(t, s)))
	if rec.Code != http.StatusOK {
		t.Fatalf("save = %d %s", rec.Code, rec.Body)
	}
	first := out["file"].(string)
	if first != "02-20261026-101530-123-synthetic-linear-01-error.json" || out["saved"].(float64) != 1 || len(out["differences"].([]any)) != 0 {
		t.Errorf("save = %v, want the file of sheet 2, saved once, without differences", out)
	}
	path := filepath.Join(rc.dir, first)
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("the saved file: %v, %v; want it readable by its owner alone", info, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	back, err := sample.ReadSample(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("the saved file does not read back: %v", err)
	}
	if err := back.CheckWithBank(rc.bank); err != nil || !reflect.DeepEqual(back, s) {
		t.Errorf("the saved file = %v, %+v; want the sample sent, passing the check with the bank", err, back)
	}

	rec, out = call(t, h, "POST", "/api/record/save", saveBody(2, raw(t, s)))
	if rec.Code != http.StatusOK || out["file"] != "02-20261026-101530-124-synthetic-linear-01-error.json" || out["saved"].(float64) != 2 {
		t.Errorf("the second save = %d %v, want a second file a millisecond later", rec.Code, out)
	}
	if names := files(t, rc.dir); len(names) != 2 || !slices.IsSorted(names) || names[1] != out["file"] {
		t.Errorf("the directory holds %q, want the second save last by name", names)
	}
}

// A check reads a sheet as a save would and answers its problems, or how it differs from its plan
// sheet, and it writes nothing.
func TestRecordCheck(t *testing.T) {
	rc, h := recorder(t)
	check := func(number int, s string) (int, map[string]any) {
		t.Helper()
		rec, out := call(t, h, "POST", "/api/record/check", saveBody(number, s))
		return rec.Code, out
	}
	good := written(rc.plan.Sheets[1])
	if code, out := check(2, raw(t, good)); code != http.StatusOK || len(out["problems"].([]any)) != 0 || len(out["differences"].([]any)) != 0 {
		t.Errorf("a check of a good sheet = %d %v, want neither problems nor differences", code, out)
	}
	good.Lines[1].Text = "3x = 7 - 1"
	code, out := check(2, raw(t, good))
	noted(t, out["differences"], "sample.lines[1].text", `is "3x = 7 - 1", and the plan sheet's line 2 is "3x = 7 + 1"`)
	if code != http.StatusOK || len(out["problems"].([]any)) != 0 {
		t.Errorf("a check of a sheet off its plan = %d %v, want its differences only", code, out)
	}
	broken := written(rc.plan.Sheets[1])
	broken.Lines[3].Answer = false
	code, out = check(2, raw(t, broken))
	noted(t, out["problems"], "sample.lines", "one line is the answer, not 0")
	if code != http.StatusOK || len(out["differences"].([]any)) != 0 {
		t.Errorf("a check of a broken sheet = %d %v, want its problems only", code, out)
	}
	_, out = call(t, h, "POST", "/api/record/check", map[string]any{"number": 2})
	noted(t, out["problems"], "sample", "is missing")
	if rec, out := call(t, h, "POST", "/api/record/check", saveBody(7, raw(t, good))); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("a check of no sheet = %d %v, want 422", rec.Code, out)
	}
	if names := files(t, rc.dir); len(names) != 0 {
		t.Errorf("a check left %q in the directory", names)
	}
}

// The check names each erasure's line by the bands of the lines as the markup has them, the higher
// on the sheet of two that hold as many, whatever their order in the markup and whatever its
// problems.
func TestRecordCheckErasureLines(t *testing.T) {
	rc, h := recorder(t)
	sh := rc.plan.Sheets[2]
	lineOf := func(s *sample.Sample) string {
		t.Helper()
		_, out := call(t, h, "POST", "/api/record/check", saveBody(sh.Number, raw(t, s)))
		return fmt.Sprint(out["erasure_lines"])
	}
	s := slip(erasing(written(sh), sh), "s-low", 560)
	if got := lineOf(s); got != "[1 0]" {
		t.Errorf("erasure lines = %s, want [1 0]", got)
	}
	// A stroke from the bottom of line 1 to the top of line 2 shares as much with each, and the
	// lines stand bottom first in the markup: the higher line on the sheet is named, line 2 there.
	between := written(sh)
	between.Lines[0], between.Lines[1] = between.Lines[1], between.Lines[0]
	between.Strokes = append(between.Strokes, sample.Stroke{ID: "s-between", Points: []sample.Point{
		{X: 500, Y: 70, TMs: 90_000, Pressure: 0.5}, {X: 500, Y: 130, TMs: 90_010, Pressure: 0.5}}})
	between.Erasures = []sample.Erasure{{TMs: 90_100, Strokes: []string{"s-between"}}}
	if got := lineOf(between); got != "[2]" {
		t.Errorf("erasure lines = %s, want [2]: the line higher on the sheet", got)
	}
	_, out := call(t, h, "POST", "/api/record/check", saveBody(sh.Number, raw(t, between)))
	if len(out["problems"].([]any)) == 0 {
		t.Errorf("the check of markup with the lines swapped = %v, want its problems too", out)
	}
}

// The names go on from the latest file in the directory when the board starts, so a clock that
// stands behind it, after a restart, does not put the next save before it; and a name another
// board took is passed by a millisecond.
func TestRecordSaveNamesGoOn(t *testing.T) {
	dir := t.TempDir()
	later := "01-20261026-101531-000-synthetic-linear-01-correct.json"
	if err := os.WriteFile(filepath.Join(dir, later), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	rc, err := NewRecorder(synth.Bank(), recordPlan(), dir)
	if err != nil {
		t.Fatal(err)
	}
	rc.now = func() time.Time { return time.Date(2026, 10, 26, 10, 15, 30, 0, time.UTC) }
	h := rc.Handler(boardAddr)
	taken := "02-20261026-101531-001-synthetic-linear-01-error.json"
	if err := os.WriteFile(filepath.Join(dir, taken), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	rec, out := call(t, h, "POST", "/api/record/save", saveBody(2, raw(t, written(rc.plan.Sheets[1]))))
	if rec.Code != http.StatusOK || out["file"] != "02-20261026-101531-002-synthetic-linear-01-error.json" {
		t.Errorf("save = %d %v, want a name after the latest file and past the taken one", rec.Code, out)
	}
}

// Every sheet that does not pass the format's strict reader, its plan sheet or the bank is
// refused with every problem at its path in the request, and nothing is written.
func TestRecordSaveRefuses(t *testing.T) {
	rc, h := recorder(t)
	correct := raw(t, written(rc.plan.Sheets[0]))
	withError := raw(t, written(rc.plan.Sheets[1]))
	edit := func(s *sample.Sample, change func(s *sample.Sample)) string {
		change(s)
		return raw(t, s)
	}
	tests := []struct {
		name string
		body any
		path string
		want string
	}{
		{"a missing pressure", saveBody(1, strings.Replace(correct, `,"pressure":0.4`, "", 1)), "sample.strokes[0].points[0].pressure", "is missing"},
		{"a field in another case", saveBody(1, strings.Replace(correct, `"pressure":`, `"Pressure":`, 1)), "sample.strokes[0].points[0]", `unknown field "Pressure"`},
		{"a null", saveBody(1, strings.Replace(correct, `"task":"synthetic-linear-01"`, `"task":null`, 1)), "sample.task", "is null"},
		{"a field the format does not have", saveBody(1, strings.Replace(correct, `{"format":`, `{"colour":"red","format":`, 1)), "sample", `unknown field "colour"`},
		{"a sample that is not an object", saveBody(1, `"a sheet"`), "sample", "is not an object"},
		{"no sample", map[string]any{"number": 1}, "sample", "is missing"},
		{"a sample its own check refuses", saveBody(1, edit(written(rc.plan.Sheets[0]), func(s *sample.Sample) { s.Lines[0].Answer = true })), "sample.lines", "one line is the answer, not 2"},
		{"another task", saveBody(1, edit(written(rc.plan.Sheets[0]), func(s *sample.Sample) { s.Task = synth.TaskCalculation })), "sample.task", `but sheet 1 of the plan solves "synthetic-linear-01"`},
		{"another scenario", saveBody(1, edit(written(rc.plan.Sheets[0]), func(s *sample.Sample) { s.Scenario = sample.ScenarioSimilarDigits })), "sample.scenario", `of the scenario "correct"`},
		{"another writer", saveBody(1, edit(written(rc.plan.Sheets[0]), func(s *sample.Sample) { s.Writer = sample.WriterAuthorAsChild })), "sample.writer", "in their own hand"},
		{"another sheet", saveBody(1, edit(written(rc.plan.Sheets[0]), func(s *sample.Sample) { s.Sheet.Height = 1400 })), "sample.sheet", "not the board's 1000 × 600"},
		{"a trap of another task", saveBody(2, strings.Replace(withError, `"trap":"sign-on-transfer"`, `"trap":"arithmetic"`, 1)), "sample.expected.trap", "not a trap of the task"},
		{"no such sheet", saveBody(9, correct), "number", "9 is not a sheet of the plan"},
		{"an erasure scenario without its planned erasure", saveBody(3, raw(t, written(rc.plan.Sheets[2]))), "sample.erasures", "one planned erasure, not 0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, out := call(t, h, "POST", "/api/record/save", tt.body)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("save = %d %s, want 422", rec.Code, rec.Body)
			}
			noted(t, out["problems"], tt.path, tt.want)
			if names := files(t, rc.dir); len(names) != 0 {
				t.Errorf("a refused sheet left %q in the directory", names)
			}
		})
	}
}

// A sheet whose markup differs from its plan sheet is saved, and the board names the differences.
func TestRecordSaveNamesDifferences(t *testing.T) {
	rc, h := recorder(t)
	s := written(rc.plan.Sheets[1])
	s.Expected.FirstError = intp(3)
	s.Lines[2].Text = "3x = 9"
	rec, out := call(t, h, "POST", "/api/record/save", saveBody(2, raw(t, s)))
	if rec.Code != http.StatusOK {
		t.Fatalf("save = %d %s", rec.Code, rec.Body)
	}
	noted(t, out["differences"], "sample.expected.first_error", "is on line 3, and the plan sheet's on line 2")
	noted(t, out["differences"], "sample.lines[2].text", `is "3x = 9", and the plan sheet's line 3 is "3x = 8"`)
	if n := len(out["differences"].([]any)); n != 2 {
		t.Errorf("differences = %v, want two", out["differences"])
	}
}

func TestDifferences(t *testing.T) {
	plan := recordPlan()
	sh := &plan.Sheets[1]
	s := written(*sh)
	s.Lines = s.Lines[:3]
	s.Lines[1].Answer, s.Lines[0].CrossedOut, s.Lines[2].Illegible = true, true, true
	s.Expected = sample.Expected{Outcome: sample.OutcomeCorrect}
	var got []string
	for _, n := range differences(sh, s) {
		got = append(got, n.Path+": "+n.Message)
	}
	want := []string{
		"sample.lines: are 3, and the plan sheet's 4",
		"sample.lines[0].crossed_out: is set, and the plan sheet marks no line crossed out",
		"sample.lines[2].illegible: is set, and the plan sheet marks no line illegible",
		"sample.lines: give the answer on line 2, and the plan sheet on line 4",
		`sample.expected.outcome: is "correct", and the plan sheet's "error"`,
		"sample.expected.first_error: is on no line, and the plan sheet's on line 2",
		`sample.expected.trap: is "", and the plan sheet's "sign-on-transfer"`,
	}
	if !slices.Equal(got, want) {
		t.Errorf("differences:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// What a pen can give is saved: points at one millisecond, a stroke that starts as the last one
// ends, a stroke of one point, points on the edge of the sheet, and the pressures 0 and 1.
func TestRecordSaveEdgeCases(t *testing.T) {
	rc, h := recorder(t)
	s := written(rc.plan.Sheets[0])
	first := &s.Strokes[0]
	first.Points[1].TMs = first.Points[0].TMs
	first.Points[0].Pressure, first.Points[1].Pressure = 0, 1
	s.Strokes[1].Points[0].TMs = first.Points[2].TMs
	s.Strokes[2].Points = s.Strokes[2].Points[:1]
	last := s.Strokes[len(s.Strokes)-1].Points
	last[0].X, last[0].Y, last[len(last)-1].X, last[len(last)-1].Y = 0, 0, sheetWidth, sheetHeight
	rec, _ := call(t, h, "POST", "/api/record/save", saveBody(1, raw(t, s)))
	if rec.Code != http.StatusOK {
		t.Errorf("save = %d %s, want the sheet saved", rec.Code, rec.Body)
	}
}

// A directory the board cannot write into is a server error, and no file is left behind.
func TestRecordSaveCannotWrite(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes into a read-only directory")
	}
	rc, h := recorder(t)
	if err := os.Chmod(rc.dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(rc.dir, 0o700) })
	rec, out := call(t, h, "POST", "/api/record/save", saveBody(1, raw(t, written(rc.plan.Sheets[0]))))
	if rec.Code != http.StatusInternalServerError || !strings.Contains(fmt.Sprint(out["error"]), "the sheet is not saved") {
		t.Errorf("save = %d %s, want 500", rec.Code, rec.Body)
	}
	if names := files(t, rc.dir); len(names) != 0 {
		t.Errorf("the directory holds %q, want nothing", names)
	}
}

// The record mode answers only its own page at a local address, takes only application/json and
// no more than 8 MiB.
func TestRecordGuards(t *testing.T) {
	rc, h := recorder(t)
	body := func() *bytes.Reader {
		data, _ := json.Marshal(saveBody(1, raw(t, written(rc.plan.Sheets[0]))))
		return bytes.NewReader(data)
	}
	send := func(host, origin, contentType string, r *bytes.Reader) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/api/record/save", r)
		req.Host = host
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		req.Header.Set("Content-Type", contentType)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	tests := []struct {
		name, host, origin, contentType string
		body                            *bytes.Reader
		want                            int
	}{
		{"the board's own page", boardAddr, "http://" + boardAddr, "application/json", body(), http.StatusOK},
		{"another site's page", boardAddr, "https://evil.example", "application/json", body(), http.StatusForbidden},
		{"a rebound domain", "evil.example:8080", "", "application/json", body(), http.StatusForbidden},
		{"a form without a preflight", boardAddr, "", "text/plain", body(), http.StatusUnsupportedMediaType},
		{"a request too large", boardAddr, "", "application/json", bytes.NewReader(append([]byte(`{"number":1,"sample":"`), bytes.Repeat([]byte("A"), maxBody)...)), http.StatusRequestEntityTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if rec := send(tt.host, tt.origin, tt.contentType, tt.body); rec.Code != tt.want {
				t.Errorf("= %d %s, want %d", rec.Code, rec.Body, tt.want)
			}
		})
	}
	if names := files(t, rc.dir); len(names) != 1 || !saved(names[0]) {
		t.Errorf("the directory holds %q, want the one sheet the board's own page saved", names)
	}
}

func TestNewRecorderRefuses(t *testing.T) {
	tests := []struct {
		name   string
		change func(pl *sample.Plan)
		want   string
	}{
		{"a task not in the bank", func(pl *sample.Plan) { pl.Sheets[0].Task = "nowhere" }, `"nowhere" is not in the bank`},
		{"a task unfit for a file name", func(pl *sample.Plan) { pl.Sheets[0].Task = "../x" }, "goes into a file name"},
		{"more sheets than two digits hold", func(pl *sample.Plan) {
			for len(pl.Sheets) < 100 {
				sh := pl.Sheets[0]
				sh.Number = len(pl.Sheets) + 1
				pl.Sheets = append(pl.Sheets, sh)
			}
		}, "in two digits"},
		{"a plan its own check refuses", func(pl *sample.Plan) { pl.Sheets[1].Number = 3 }, "numbered in order from 1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := recordPlan()
			tt.change(plan)
			bank := synth.Bank()
			bank.Tasks = append(bank.Tasks, sample.Task{ID: "../x"})
			if _, err := NewRecorder(bank, plan, t.TempDir()); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("NewRecorder = %v, want an error holding %q", err, tt.want)
			}
		})
	}
}

// The names the board gives its files are the ones it counts: the number of the plan sheet first,
// in two digits.
// saved reports whether a name is that of a saved sheet's file.
func saved(name string) bool {
	_, ok := sample.ParseSheetFile(name)
	return ok
}

// The plan of the author's free-form sheets and the bank of free-form tasks, as seen from this
// package's directory: three sheets of parts, a number line, a scheme and columns at the side (T57).
const (
	authorPlan = "../../eval/free-author.json"
	freeBank   = "../../eval/free-bank.json"
)

// partsRecorder starts the record mode of the plan of the author's free-form sheets in a new
// directory, its clock standing at one moment, and returns it with its handler.
func partsRecorder(t *testing.T) (*Recorder, http.Handler) {
	t.Helper()
	read := func(path string) *os.File {
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = f.Close() })
		return f
	}
	bank, err := sample.ReadBank(read(freeBank))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := sample.ReadPlan(read(authorPlan))
	if err != nil {
		t.Fatal(err)
	}
	rc, err := NewRecorder(bank, plan, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rc.now = func() time.Time { return time.Date(2026, 10, 27, 18, 0, 0, 0, time.UTC) }
	return rc, rc.Handler(boardAddr)
}

// The plan view of a plan of parts carries each sheet's parts, with their kind and place, and a plan
// of lines carries none.
func TestRecordPlanViewParts(t *testing.T) {
	rc, h := partsRecorder(t)
	rec, _ := call(t, h, "GET", "/api/record/plan", nil)
	var view PlanView
	if err := json.Unmarshal(rec.Body.Bytes(), &view); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("GET /api/record/plan = %d %s", rec.Code, rec.Body)
	}
	for i, sh := range view.Sheets {
		want := rc.plan.Sheets[i].Parts
		if len(sh.Parts) != len(want) || len(want) == 0 {
			t.Fatalf("sheet %d: %d parts in the view, want the plan's %d", sh.Number, len(sh.Parts), len(want))
		}
		for j, p := range sh.Parts {
			if p.Kind != want[j].Kind || p.Place != want[j].Place || p.Text != want[j].Text || !slices.Equal(sh.Lines, rc.plan.Sheets[i].Lines) {
				t.Errorf("sheet %d, part %d = %s %s %q, want the plan's %+v", sh.Number, j+1, p.Kind, p.Place, p.Text, want[j])
			}
		}
	}
	_, lines := recorder(t)
	if rec, _ := call(t, lines, "GET", "/api/record/plan", nil); !strings.Contains(rec.Body.String(), `"parts":[]`) {
		t.Errorf("the plan view of a plan of lines = %s, want no parts", rec.Body)
	}
}

// On a sheet of parts the board proposes a part to each group of strokes, in the order of first
// writing, with the plan's texts and answer. On the sheets code draws for the author's plan no group
// holds strokes of two parts, and a part without the planned erasure is one group; a drawing redrawn
// after the erasure comes as more, one after another, and joined they are the markup code drew,
// which saves without differences. The planned erasure is the one on the plan's part, by the place
// of its strokes on the plane.
func TestRecordProposeParts(t *testing.T) {
	rc, h := partsRecorder(t)
	ran, split := 0, 0 // the sheets tried, and those whose groups are more than their parts
	for _, sh := range rc.plan.Sheets {
		for _, seed := range []uint64{1, 2, 77} {
			t.Run(fmt.Sprintf("sheet %d seed %d", sh.Number, seed), func(t *testing.T) {
				ran++
				s := synth.Sheet(&sh, seed)
				undone := make([]bool, len(s.Erasures))
				planned := -1
				for i, e := range s.Erasures {
					undone[i] = e.Planned == nil // code's slips stand for what Undo takes away
					if e.Planned != nil {
						planned = i
					}
				}
				rec, proposed := call(t, h, "POST", "/api/record/propose", ProposeRequest{Number: sh.Number, Strokes: s.Strokes, Erasures: s.Erasures, Undone: undone})
				var p Proposal
				if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil || rec.Code != http.StatusOK {
					t.Fatalf("propose = %d %s", rec.Code, rec.Body)
				}
				partOf := map[string]int{}
				for k, l := range s.Lines {
					for _, id := range l.Strokes {
						partOf[id] = k
					}
				}
				order := map[string]int{}
				for i, st := range s.Strokes {
					order[st.ID] = i
				}
				joined := make([][]string, len(s.Lines))
				groups := make([]int, len(s.Lines))
				last := -1
				for i, l := range p.Lines {
					k := partOf[l.Strokes[0]]
					for _, id := range l.Strokes {
						if partOf[id] != k {
							t.Fatalf("group %d holds strokes of parts %d and %d", i+1, k+1, partOf[id]+1)
						}
					}
					if k < last || (k != last && k != last+1) {
						t.Fatalf("group %d is of part %d after a group of part %d: the groups of a part do not come one after another", i+1, k+1, last+1)
					}
					last = k
					groups[k]++
					joined[k] = append(joined[k], l.Strokes...)
					if want := sh.Lines[min(i, len(sh.Lines)-1)]; i < len(sh.Lines) && l.Text != want {
						t.Errorf("group %d has the text %q, want the plan's %q", i+1, l.Text, want)
					}
				}
				for k, n := range groups {
					if n != 1 && (sh.Erasure == nil || sh.Erasure.Line != k+1) {
						t.Errorf("part %d without the planned erasure is %d groups, want one", k+1, n)
					}
					slices.SortFunc(joined[k], func(a, b string) int { return order[a] - order[b] })
					if !slices.Equal(joined[k], s.Lines[k].Strokes) {
						t.Errorf("part %d joined = %v, want the strokes code drew %v", k+1, joined[k], s.Lines[k].Strokes)
					}
				}
				switch {
				case len(p.Lines) > len(sh.Lines):
					split++
					noted(t, proposed["differences"], "sample.lines", fmt.Sprintf("the sheet has %d parts, and its plan sheet %d: a part drawn on after a pause of more than 2 seconds comes as two groups, so join them", len(p.Lines), len(sh.Lines)))
				case len(p.Differences) != 0:
					t.Errorf("differences = %v, want none", p.Differences)
				}
				if sh.Erasure != nil && (p.Planned == nil || *p.Planned != planned) {
					t.Errorf("planned = %v, want the erasure %d", p.Planned, planned)
				}

				s.Writer = sample.WriterAuthor
				_, out := call(t, h, "POST", "/api/record/check", saveBody(sh.Number, raw(t, s)))
				if sh.Erasure != nil {
					if got := out["erasure_lines"].([]any); int(got[planned].(float64)) != sh.Erasure.Line {
						t.Errorf("erasure lines = %v, want the planned one on part %d", got, sh.Erasure.Line)
					}
				}
				rec, out = call(t, h, "POST", "/api/record/save", saveBody(sh.Number, raw(t, s)))
				if rec.Code != http.StatusOK || len(out["differences"].([]any)) != 0 {
					t.Errorf("save = %d %v, want the sheet saved without differences", rec.Code, out)
				}
			})
		}
	}
	if ran == 3*len(rc.plan.Sheets) && split == 0 {
		t.Error("no sheet came in more groups than parts, so joining them is not tested")
	}
}

// A sheet of parts with a part missing, or with nothing written, is named in parts, with what to do.
func TestRecordProposePartsNames(t *testing.T) {
	rc, h := partsRecorder(t)
	sh := rc.plan.Sheets[2] // the columns at the side, without an erasure
	s := synth.Sheet(&sh, 1)
	drop := map[string]bool{}
	for _, id := range s.Lines[len(s.Lines)-1].Strokes {
		drop[id] = true
	}
	var kept []sample.Stroke
	for _, st := range s.Strokes {
		if !drop[st.ID] {
			kept = append(kept, st)
		}
	}
	_, out := call(t, h, "POST", "/api/record/propose", ProposeRequest{Number: sh.Number, Strokes: kept, Erasures: s.Erasures})
	noted(t, out["differences"], "sample.lines", fmt.Sprintf("the sheet has %d parts, and its plan sheet %d: two parts written close together without a pause come as one group", len(sh.Lines)-1, len(sh.Lines)))
	_, out = call(t, h, "POST", "/api/record/propose", ProposeRequest{Number: sh.Number})
	noted(t, out["differences"], "sample.lines", "the sheet has no parts yet")

	// A planned erasure on a sheet without a move of the eraser is named on its part.
	first := rc.plan.Sheets[0]
	drawn := synth.Sheet(&first, 1)
	_, out = call(t, h, "POST", "/api/record/propose", ProposeRequest{Number: first.Number, Strokes: drawn.Strokes})
	noted(t, out["differences"], "sample.erasures", fmt.Sprintf("its plan sheet plans an erasure on part %d", first.Erasure.Line))

	// Two moves of the eraser on its parts: the one that took more strokes is taken, whatever group
	// they stood on, and the sheet is named, as the planned erasure is one move.
	slipID := drawn.Lines[len(drawn.Lines)-1].Strokes[0]
	moves := append(slices.Clone(drawn.Erasures), sample.Erasure{TMs: 1 << 40, Strokes: []string{slipID}})
	rec, out := call(t, h, "POST", "/api/record/propose", ProposeRequest{Number: first.Number, Strokes: drawn.Strokes, Erasures: moves})
	var p Proposal
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil || p.Planned == nil || *p.Planned != 0 {
		t.Errorf("planned = %v (%v), want the first move, which took more strokes", p.Planned, err)
	}
	noted(t, out["differences"], "sample.erasures", "2 moves of the eraser took strokes from the parts of the sheet, and the planned erasure is one move")
	if p, _ := call(t, h, "POST", "/api/record/propose", ProposeRequest{Number: first.Number, Strokes: drawn.Strokes, Erasures: moves, Undone: []bool{false, true}}); p.Code != http.StatusOK || strings.Contains(p.Body.String(), "moves of the eraser") {
		t.Errorf("with the second by Undo = %d %s, want nothing named", p.Code, p.Body)
	}
}
