package convert

import (
	"bytes"
	"errors"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"

	"github.com/altnovatrail/tutor/internal/evalsample"
	"github.com/altnovatrail/tutor/internal/protocol"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/synth"
)

// v0 writes a sheet as a file of tutor-sample v0, as the record mode saved one: every point with the
// pressure pressure gives it.
func v0(t *testing.T, s *sample.Sample, pressure func(stroke, point int) float64) []byte {
	t.Helper()
	f := file{Format: s.Format, Version: s.Version, Sheet: s.Sheet, Task: s.Task, Writer: s.Writer, Scenario: s.Scenario,
		Strokes: make([]stroke, len(s.Strokes)), Erasures: s.Erasures, Lines: s.Lines, Injection: s.Injection, Expected: s.Expected}
	for i, st := range s.Strokes {
		f.Strokes[i] = stroke{ID: st.ID, Points: make([]point, len(st.Points))}
		for k, p := range st.Points {
			f.Strokes[i].Points[k] = point{X: p.X, Y: p.Y, TMs: p.TMs, Pressure: pressure(i, k)}
		}
	}
	data, err := sample.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func mouse(int, int) float64        { return Mouse }
func pen(stroke, point int) float64 { return 0.4 + float64((stroke+point)%5)/10 }

// wantProblem fails unless err is a *sample.FormatError with a problem at path whose message holds
// want.
func wantProblem(t *testing.T, err error, path, want string) {
	t.Helper()
	var fe *sample.FormatError
	if !errors.As(err, &fe) {
		t.Fatalf("error = %v, want a *FormatError", err)
	}
	for _, p := range fe.Problems {
		if p.Path == path && strings.Contains(p.Message, want) {
			return
		}
	}
	t.Errorf("no problem at %q holding %q in:\n%v", path, want, err)
}

// Every synthetic sample of every scenario reads back as written, without its pressure, by a mouse
// where every point has the pressure 0.5 and by a pen otherwise.
func TestReadEverySyntheticSample(t *testing.T) {
	for i, s := range synth.Samples() {
		by, want := pen, "pen"
		if i%2 == 1 {
			by, want = mouse, "mouse"
		}
		got, pointer, err := Read(bytes.NewReader(v0(t, s, by)))
		if err != nil {
			t.Fatalf("%s: %v", s.Scenario, err)
		}
		if !reflect.DeepEqual(got, s) || pointer != want {
			t.Errorf("%s: read back by %s, want the sample written, by %s", s.Scenario, pointer, want)
		}
	}
}

// The reader is as strict as the format always was: every field present and spelt as its name, once,
// no other, nothing after the object, a pressure from 0 to 1, and a sample its own check takes.
func TestReadRefuses(t *testing.T) {
	good := string(v0(t, synth.Sample(sample.ScenarioCorrect), pen))
	firstPressure := strings.Index(good, `"pressure": `)
	tests := []struct {
		name, data, path, want string
	}{
		{"a missing field", regexp.MustCompile(`,\s+"pressure": [0-9.]+`).ReplaceAllLiteralString(good[:firstPressure+40], "") + good[firstPressure+40:], "strokes[0].points[0].pressure", "is missing"},
		{"a field spelt in another case", strings.Replace(good, `"pressure": `, `"Pressure": `, 1), "strokes[0].points[0]", `unknown field "Pressure"`},
		{"a field given twice", strings.Replace(good, `"task": `, `"writer": "author", "task": `, 1), "writer", "appears twice"},
		{"a null where a value is required", strings.Replace(good, `"task": "synthetic-linear-01"`, `"task": null`, 1), "task", "is null"},
		{"a fraction where a whole number is required", regexp.MustCompile(`("t_ms": [0-9]+)`).ReplaceAllString(good[:firstPressure], "${1}.5") + good[firstPressure:], "strokes[0].points[0].t_ms", "not a whole number"},
		{"an object where an array is required", strings.Replace(good, `"erasures": []`, `"erasures": {}`, 1), "erasures", "is not an array"},
		{"an unknown field", strings.Replace(good, `"task":`, `"colour": "red", "task":`, 1), "", `unknown field "colour"`},
		{"a pressure above 1", strings.Replace(good, `"pressure": 0.4`, `"pressure": 1.5`, 1), "strokes[0].points[0].pressure", "1.5 is not from 0 to 1"},
		{"a sample its own check refuses", strings.Replace(good, `"version": 0`, `"version": 1`, 1), "version", "1 is not supported"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := Read(strings.NewReader(tt.data))
			if tt.path == "" {
				if err == nil || !strings.Contains(err.Error(), tt.want) {
					t.Errorf("Read error = %v, want one holding %q", err, tt.want)
				}
				return
			}
			wantProblem(t, err, tt.path, tt.want)
		})
	}
	// Every problem at once: the pressure's and the sheet's own check's.
	both := strings.Replace(strings.Replace(good, `"pressure": 0.4`, `"pressure": 1.5`, 1), `"version": 0`, `"version": 1`, 1)
	_, _, err := Read(strings.NewReader(both))
	wantProblem(t, err, "strokes[0].points[0].pressure", "1.5 is not from 0 to 1")
	wantProblem(t, err, "version", "1 is not supported")
	for _, data := range []string{good + "{}", "<sample/>", good[:len(good)/2]} {
		if _, _, err := Read(strings.NewReader(data)); err == nil {
			t.Errorf("Read took %.20q…", data)
		}
	}
	_, _, err = Read(strings.NewReader(`{"format": "tutor-sample", "version": 0, "strokes": [{"id": "s-1", "points": [{"x": 1, "y": 2}]}]}`))
	for _, path := range []string{"strokes[0].points[0].t_ms", "strokes[0].points[0].pressure", "sheet", "expected"} {
		wantProblem(t, err, path, "is missing")
	}
}

// bankOf is a bank as a sample names it, by a path in the repository and the digest of its file.
func bankOf(t *testing.T, path string, b *sample.Bank) *evalsample.Bank {
	t.Helper()
	data, err := sample.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	eb, err := evalsample.BankOf(path, data)
	if err != nil {
		t.Fatal(err)
	}
	return eb
}

// readFile reads a file of the repository's eval folder with its reader.
func readFile[T any](t *testing.T, path string, read func(*bytes.Reader) (T, error)) T {
	t.Helper()
	data, err := os.ReadFile("../../" + path)
	if err != nil {
		t.Fatal(err)
	}
	v, err := read(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func readPlan(t *testing.T, path string) *sample.Plan {
	return readFile(t, path, func(r *bytes.Reader) (*sample.Plan, error) { return sample.ReadPlan(r) })
}

func readBank(t *testing.T, path string) *evalsample.Bank {
	t.Helper()
	data, err := os.ReadFile("../../" + path)
	if err != nil {
		t.Fatal(err)
	}
	b, err := evalsample.BankOf(path, data)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// fixed are options that give every person's sheet one ID and one code.
func fixed() Options {
	return Options{ID: func() (string, error) { return "test-sample", nil }, Code: func() (string, error) { return "TESTCODE01", nil }}
}

// back reads a converted sheet as the runner will.
func back(t *testing.T, c *Converted, sh *sample.PlanSheet, b *evalsample.Bank) (*evalv1.Sample, *sample.Sample) {
	t.Helper()
	es, err := protocol.ReadSample(bytes.NewReader(c.Data))
	if err != nil {
		t.Fatal(err)
	}
	read, refused, err := evalsample.Read(es, sh, b)
	if err != nil || refused != nil {
		t.Fatalf("the runner reads the converted sheet: %v, %v", err, refused)
	}
	return es, read.Sample
}

// The author's sheet is a person's sample of measurement, with the ID and the code the options give,
// its strokes as they were written, by the pointer the pressure tells, and its markup.
func TestSheetOfThePerson(t *testing.T) {
	plan, samples := synth.Plan()
	b := bankOf(t, "eval/synthetic-bank.json", synth.Bank())
	sh, s := &plan.Sheets[0], samples[0]
	c, err := Sheet(v0(t, s, mouse), sh, b, fixed())
	if err != nil {
		t.Fatal(err)
	}
	es, read := back(t, c, sh, b)
	if es.GetPerson() == nil || es.GetId() != "test-sample" || es.GetWriterCode() != "TESTCODE01" || es.GetWriter() != evalv1.Writer_WRITER_ADULT ||
		c.Writer != sample.WriterAuthor || c.Pointer != "mouse" || c.Outcome != sample.OutcomeCorrect {
		t.Errorf("the converted sheet: %v by %v, ID %q, code %q; %+v", es.GetOrigin(), es.GetWriter(), es.GetId(), es.GetWriterCode(), c)
	}
	if p := es.GetHistory().GetEvents()[0].GetWrite().GetPointer(); p != "mouse" {
		t.Errorf("the first stroke's pointer = %q, want mouse", p)
	}
	if len(read.Strokes) != len(s.Strokes) || !reflect.DeepEqual(read.Strokes[3].Points, s.Strokes[3].Points) || len(read.Lines) != len(s.Lines) || read.Lines[0].Text != s.Lines[0].Text {
		t.Errorf("the sheet read back = %+v, want the strokes and lines written", read)
	}
	asChild := *s
	asChild.Writer = sample.WriterAuthorAsChild
	if c, err := Sheet(v0(t, &asChild, pen), sh, b, fixed()); err != nil || c.Pointer != "pen" {
		t.Errorf("the sheet the author wrote as a child = %v, %v", c, err)
	} else if es, _ := back(t, c, sh, b); es.GetWriter() != evalv1.Writer_WRITER_ADULT_AS_CHILD {
		t.Errorf("the writer = %v, want an adult as a child", es.GetWriter())
	}

	// What keeps a sheet from its sample: another task or scenario than its plan sheet's, a stroke it
	// shows in no line, an inscription, the bank, and an ID or a code that cannot be had.
	stray := *s
	stray.Strokes = append(slices.Clone(s.Strokes), sample.Stroke{ID: "s-stray", Points: []sample.Point{{X: 500, Y: 590, TMs: 1 << 30}}})
	inj := synth.Sample(sample.ScenarioInjection)
	injSheet := *sh
	injSheet.Task, injSheet.Scenario = inj.Task, inj.Scenario
	broke := errors.New("no randomness")
	noID, noCode := fixed(), fixed()
	noID.ID = func() (string, error) { return "", broke }
	noCode.Code = func() (string, error) { return "", broke }
	trapped := *samples[1]
	trapped.Expected.Trap = "no-such-trap"
	for _, c := range []struct {
		name string
		data []byte
		sh   *sample.PlanSheet
		o    Options
		want string
	}{
		{"another task", v0(t, s, pen), &plan.Sheets[2], fixed(), "its task is"},
		{"another scenario", v0(t, s, pen), &plan.Sheets[1], fixed(), "its scenario is correct"},
		{"a stroke in no line", v0(t, &stray, pen), sh, fixed(), `the stroke "s-stray", which the sheet shows, is in no part`},
		{"an inscription", v0(t, inj, pen), &injSheet, fixed(), "an inscription to steer the model"},
		{"a trap the bank has not", v0(t, &trapped, pen), &plan.Sheets[1], fixed(), "not a trap of the task"},
		{"no ID", v0(t, s, pen), sh, noID, "no randomness"},
		{"no code", v0(t, s, pen), sh, noCode, "no randomness"},
		{"not a sample", []byte("{}"), sh, fixed(), "is missing"},
	} {
		if _, err := Sheet(c.data, c.sh, b, c.o); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want an error holding %q", c.name, err, c.want)
		}
	}
}

// A sheet code drew is the sample draw writes for it from the seed, to the byte, and one the code of
// today does not draw so is refused.
func TestSheetDrawnByCode(t *testing.T) {
	b, plan := readBank(t, "eval/bank.json"), readPlan(t, "eval/sheets.json")
	for _, n := range []int{2, 9} {
		sh := &plan.Sheets[n-1]
		s := synth.SheetLeaning(sh, 1, synth.SimilarShift)
		c, err := Sheet(v0(t, s, pen), sh, b, Options{Plan: "eval/sheets.json", Seed: 1, Shift: synth.SimilarShift})
		if err != nil {
			t.Fatal(err)
		}
		id, err := evalsample.ID("eval/sheets.json", 1, synth.SimilarShift, n)
		if err != nil {
			t.Fatal(err)
		}
		es, err := evalsample.Of(s, sh, b, 1, id)
		if err != nil {
			t.Fatal(err)
		}
		want, err := protocol.MarshalSample(es)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(c.Data, want) || c.Writer != sample.WriterSynthetic || c.Pointer != evalsample.Pointer {
			t.Errorf("sheet %d: the converted file is not what draw writes, or %+v", n, c)
		}
		if _, err := Sheet(v0(t, s, pen), sh, b, Options{Plan: "eval/sheets.json", Seed: 2, Shift: synth.SimilarShift}); !errors.Is(err, ErrOtherDrawing) {
			t.Errorf("sheet %d with another seed: %v, want ErrOtherDrawing", n, err)
		}
	}
	long := strings.Repeat("x", 80) + ".json"
	sh := &plan.Sheets[1]
	if _, err := Sheet(v0(t, synth.SheetLeaning(sh, 1, synth.SimilarShift), pen), sh, b, Options{Plan: long, Seed: 1, Shift: synth.SimilarShift}); err == nil {
		t.Error("a plan whose name makes no ID converts")
	}
}

// whole is the author's free solution of a plan sheet of one part, as written by the hand code draws
// for a sheet of the same task without an erasure: every stroke it shows in that one part, with the
// text given, the outcome of the plan sheet.
func whole(t *testing.T, sh *sample.PlanSheet, text string) *sample.Sample {
	t.Helper()
	free := readPlan(t, "eval/free-sheets.json")
	for i := range free.Sheets {
		code := &free.Sheets[i]
		if code.Task != sh.Task || code.Erasure != nil {
			continue
		}
		s := synth.Sheet(code, 1)
		var ids []string
		for _, l := range s.Lines {
			ids = append(ids, l.Strokes...)
		}
		order := map[string]int{}
		for k, st := range s.Strokes {
			order[st.ID] = k
		}
		slices.SortFunc(ids, func(a, b string) int { return order[a] - order[b] })
		s.Writer, s.Scenario, s.Expected = sample.WriterAuthor, sh.Scenario, sh.Expected
		s.Lines = []sample.Line{{Text: text, Strokes: ids, Answer: true}}
		return s
	}
	t.Fatalf("no sheet of code of the task %s without an erasure", sh.Task)
	return nil
}

// The truth of a whole solution is the key's on a sheet marked correct, and on one marked wrong the
// number after the last word «ответ» of its markup; a markup that names another, or none, or the
// key's where the sheet is wrong, gives no truth, and the sheet is refused.
func TestSheetWholeSolution(t *testing.T) {
	b, plan := readBank(t, "eval/free-bank.json"), readPlan(t, "eval/free-messy.json")
	correct, wrong := &plan.Sheets[2], &plan.Sheets[6] // houses, its key 9: written right, and counted twice
	if correct.Task != "houses" || wrong.Task != "houses" || wrong.Expected.Outcome != sample.OutcomeError {
		t.Fatalf("sheets 3 and 7 of the messy plan are %s and %s", correct.Task, wrong.Task)
	}
	value := func(c *Converted) string {
		es, err := protocol.ReadSample(bytes.NewReader(c.Data))
		if err != nil {
			t.Fatal(err)
		}
		return es.GetMarkup().GetTruth().GetAnswerSlots()[0].GetValue()
	}
	for _, c := range []struct {
		name    string
		sh      *sample.PlanSheet
		text    string
		value   string
		outcome sample.Outcome
		refused string
	}{
		{"correct, the plan's words", correct, "свободное решение целиком", "9", sample.OutcomeCorrect, ""},
		{"correct, the answer named", correct, "свободное решение целиком — Ответ: 9", "9", sample.OutcomeCorrect, ""},
		{"wrong, the answer named", wrong, "свободное решение целиком, с ошибкой: посчитал в 2 раза больше — ответ 10", "10", sample.OutcomeError, ""},
		{"correct, another answer named", correct, "свободное решение — ответ 12", "", "", "marked correct, and its markup names the answer 12"},
		{"wrong, no answer named", wrong, "свободное решение целиком, с ошибкой", "", "", "names no number after the word «ответ»"},
		{"wrong, the key named", wrong, "с ошибкой — ответ 9", "", "", "the answer its markup names is the key's, 9"},
	} {
		got, err := Sheet(v0(t, whole(t, c.sh, c.text), mouse), c.sh, b, fixed())
		switch {
		case c.refused != "":
			if err == nil || !strings.Contains(err.Error(), c.refused) {
				t.Errorf("%s: %v, want an error holding %q", c.name, err, c.refused)
			}
		case err != nil:
			t.Errorf("%s: %v", c.name, err)
		case value(got) != c.value || got.Outcome != c.outcome:
			t.Errorf("%s: the truth %q, %s; want %q, %s", c.name, value(got), got.Outcome, c.value, c.outcome)
		}
	}
}
