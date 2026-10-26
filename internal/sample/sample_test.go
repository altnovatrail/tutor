package sample_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/synth"
)

// roundTripSample writes a sample as JSON and reads it back with the package's reader.
func roundTripSample(t *testing.T, s *sample.Sample) *sample.Sample {
	t.Helper()
	data, err := sample.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got, err := sample.ReadSample(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("ReadSample: %v", err)
	}
	return got
}

func TestSyntheticSamplesOfEveryScenario(t *testing.T) {
	bank := synth.Bank()
	samples := synth.Samples()
	if len(samples) != len(sample.Scenarios) {
		t.Fatalf("synth.Samples() = %d samples, want one per scenario, %d", len(samples), len(sample.Scenarios))
	}
	for i, s := range samples {
		t.Run(string(sample.Scenarios[i]), func(t *testing.T) {
			if s.Scenario != sample.Scenarios[i] {
				t.Fatalf("sample %d is of the scenario %q, want %q", i, s.Scenario, sample.Scenarios[i])
			}
			got := roundTripSample(t, s)
			if !reflect.DeepEqual(got, s) {
				t.Errorf("the sample read back differs from the one written")
			}
			if err := got.CheckWithBank(bank); err != nil {
				t.Errorf("CheckWithBank: %v", err)
			}
		})
	}
}

func TestSyntheticSamplesAreTheSameEachTime(t *testing.T) {
	if !reflect.DeepEqual(synth.Samples(), synth.Samples()) {
		t.Error("two calls of synth.Samples() differ")
	}
}

func TestSyntheticBank(t *testing.T) {
	data, err := sample.Marshal(synth.Bank())
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got, err := sample.ReadBank(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("ReadBank: %v", err)
	}
	if !reflect.DeepEqual(got, synth.Bank()) {
		t.Error("the bank read back differs from the one written")
	}
	kinds := map[sample.TaskKind]bool{}
	for _, task := range got.Tasks {
		kinds[task.Kind] = true
	}
	if len(kinds) != len(sample.TaskKinds) {
		t.Errorf("the bank has tasks of %d kinds, want all %d", len(kinds), len(sample.TaskKinds))
	}
}

// edit returns a copy of a synthetic sample with a change made to it.
func edit(t *testing.T, sc sample.Scenario, change func(s *sample.Sample)) *sample.Sample {
	t.Helper()
	data, err := json.Marshal(synth.Sample(sc))
	if err != nil {
		t.Fatal(err)
	}
	var s sample.Sample
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatal(err)
	}
	change(&s)
	return &s
}

func intp(n int) *int { return &n }

// wantProblem fails unless err is a *FormatError with a problem at path whose message holds want.
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

func TestCheckRefusesBrokenSamples(t *testing.T) {
	tests := []struct {
		name     string
		scenario sample.Scenario
		change   func(s *sample.Sample)
		path     string
		want     string
	}{
		{"another format", sample.ScenarioCorrect, func(s *sample.Sample) { s.Format = "tutor-bank" }, "format", `not "tutor-sample"`},
		{"no version", sample.ScenarioCorrect, func(s *sample.Sample) { s.Version = nil }, "version", "is missing"},
		{"a newer version", sample.ScenarioCorrect, func(s *sample.Sample) { s.Version = intp(1) }, "version", "1 is not supported"},
		{"no sheet", sample.ScenarioCorrect, func(s *sample.Sample) { s.Sheet = sample.Sheet{} }, "sheet", "above 0"},
		{"no task", sample.ScenarioCorrect, func(s *sample.Sample) { s.Task = "" }, "task", "is missing"},
		{"unknown writer", sample.ScenarioCorrect, func(s *sample.Sample) { s.Writer = "child" }, "writer", `"child"`},
		{"unknown scenario", sample.ScenarioCorrect, func(s *sample.Sample) { s.Scenario = "typo" }, "scenario", `"typo"`},
		{"no strokes", sample.ScenarioCorrect, func(s *sample.Sample) { s.Strokes = nil; s.Lines = nil }, "strokes", "are missing"},
		{"a stroke without an ID", sample.ScenarioCorrect, func(s *sample.Sample) { s.Strokes[1].ID = "" }, "strokes[1].id", "is missing"},
		{"a repeated stroke ID", sample.ScenarioCorrect, func(s *sample.Sample) { s.Strokes[1].ID = s.Strokes[0].ID }, "strokes[1].id", "earlier stroke"},
		{"a stroke without points", sample.ScenarioCorrect, func(s *sample.Sample) { s.Strokes[2].Points = nil }, "strokes[2].points", "are missing"},
		{"a point off the sheet", sample.ScenarioCorrect, func(s *sample.Sample) { s.Strokes[0].Points[0].X = 5000 }, "strokes[0].points[0]", "off the sheet"},
		{"a negative time", sample.ScenarioCorrect, func(s *sample.Sample) { s.Strokes[0].Points[0].TMs = -1 }, "strokes[0].points[0].t_ms", "below 0"},
		{"pressure above 1", sample.ScenarioCorrect, func(s *sample.Sample) { s.Strokes[0].Points[1].Pressure = 1.5 }, "strokes[0].points[1].pressure", "from 0 to 1"},
		{"time going back in a stroke", sample.ScenarioCorrect, func(s *sample.Sample) { s.Strokes[0].Points[2].TMs = 0 }, "strokes[0].points[2].t_ms", "goes back"},
		{"strokes out of order", sample.ScenarioCorrect, func(s *sample.Sample) { s.Strokes[0], s.Strokes[1] = s.Strokes[1], s.Strokes[0] }, "strokes[1]", "order they were written"},
		{"erasing an unknown stroke", sample.ScenarioErasureFixed, func(s *sample.Sample) { s.Erasures[0].Strokes[0] = "s-9999" }, "erasures[0].strokes[0]", "not a stroke"},
		{"erasing a stroke twice", sample.ScenarioErasureFixed, func(s *sample.Sample) {
			s.Erasures[1].Strokes[0] = s.Erasures[0].Strokes[0]
		}, "erasures[1].strokes[0]", "erased twice"},
		{"erasing before the stroke ends", sample.ScenarioErasureFixed, func(s *sample.Sample) { s.Erasures[0].TMs = 0 }, "erasures[0].strokes[0]", "after the erasure"},
		{"erasures out of order", sample.ScenarioErasureFixed, func(s *sample.Sample) { s.Erasures[1].TMs = s.Erasures[0].TMs - 1 }, "erasures[1].t_ms", "goes back"},
		{"erasing while writing", sample.ScenarioErasureFixed, func(s *sample.Sample) {
			last := s.Strokes[len(s.Strokes)-1]
			s.Erasures[1].TMs = (last.Points[0].TMs + last.Points[len(last.Points)-1].TMs) / 2
		}, "erasures[1].t_ms", "cannot write and erase at once"},
		{"a planned erasure outside its scenarios", sample.ScenarioErasureFixed, func(s *sample.Sample) { s.Scenario = sample.ScenarioCorrect }, "erasures", "only the scenarios"},
		{"an erasure scenario without its plan", sample.ScenarioErasureBroken, func(s *sample.Sample) { s.Erasures[0].Planned = nil }, "erasures", "one planned erasure, not 0"},
		{"a planned erasure of another kind", sample.ScenarioErasureBroken, func(s *sample.Sample) { s.Erasures[0].Planned.Erased = sample.ErasedMistake }, "erasures[0].planned.erased", `erases a "correct-step"`},
		{"a trap on a correct sheet", sample.ScenarioCorrect, func(s *sample.Sample) { s.Expected.Trap = "sign-on-transfer" }, "expected.trap", "only a sheet"},
		{"an illegible answer on a correct sheet", sample.ScenarioCorrect, func(s *sample.Sample) { s.Lines[2].Illegible = true }, "lines[2]", "is an illegible answer"},
		{"a repeated ID of a stroke without points", sample.ScenarioCorrect, func(s *sample.Sample) {
			s.Strokes[0].Points = nil
			s.Strokes[1].ID = s.Strokes[0].ID
		}, "strokes[1].id", "earlier stroke"},
		{"a planned erasure of no line", sample.ScenarioErasureBroken, func(s *sample.Sample) { s.Erasures[0].Planned.Line = 9 }, "erasures[0].planned.line", "not a line"},
		{"a planned erasure without its text", sample.ScenarioErasureBroken, func(s *sample.Sample) { s.Erasures[0].Planned.Text = "" }, "erasures[0].planned.text", "is missing"},
		{"a line of erased strokes", sample.ScenarioErasureFixed, func(s *sample.Sample) {
			s.Lines[1].Strokes[0] = s.Erasures[1].Strokes[0]
		}, "lines[1].strokes[0]", "is erased"},
		{"a stroke in two lines", sample.ScenarioCorrect, func(s *sample.Sample) { s.Lines[1].Strokes[0] = s.Lines[0].Strokes[0] }, "lines[1].strokes[0]", "already belongs to lines[0]"},
		{"a line without its text", sample.ScenarioCorrect, func(s *sample.Sample) { s.Lines[0].Text = "" }, "lines[0].text", "is missing"},
		{"two answers", sample.ScenarioCorrect, func(s *sample.Sample) { s.Lines[0].Answer = true }, "lines", "not 2"},
		{"no answer", sample.ScenarioCorrect, func(s *sample.Sample) { s.Lines[2].Answer = false }, "lines", "not 0"},
		{"a crossed-out answer", sample.ScenarioCorrect, func(s *sample.Sample) { s.Lines[2].CrossedOut = true }, "lines[2]", "crossed out"},
		{"the answer last on a sheet of answer-not-last", sample.ScenarioAnswerNotLast, func(s *sample.Sample) {
			s.Lines[2].Answer = false
			s.Lines[3].Answer = true
		}, "lines[3]", "a line follows the answer"},
		{"an injection scenario without it", sample.ScenarioInjection, func(s *sample.Sample) { s.Injection = nil }, "injection", "is missing"},
		{"an injection outside its scenario", sample.ScenarioInjection, func(s *sample.Sample) { s.Scenario = sample.ScenarioCorrect }, "injection", "only the scenario"},
		{"an injection in another language", sample.ScenarioInjection, func(s *sample.Sample) { s.Injection.Lang = "de" }, "injection.lang", `"de"`},
		{"an injection stroke in a line", sample.ScenarioInjection, func(s *sample.Sample) { s.Injection.Strokes[0] = s.Lines[0].Strokes[0] }, "injection.strokes[0]", "already belongs"},
		{"an unknown outcome", sample.ScenarioCorrect, func(s *sample.Sample) { s.Expected.Outcome = "maybe" }, "expected.outcome", `"maybe"`},
		{"an error without its line", sample.ScenarioError, func(s *sample.Sample) { s.Expected.FirstError = nil }, "expected.first_error", "is missing"},
		{"a first error on a correct sheet", sample.ScenarioCorrect, func(s *sample.Sample) { s.Expected.FirstError = intp(1) }, "expected.first_error", "only a sheet"},
		{"a first error of no line", sample.ScenarioError, func(s *sample.Sample) { s.Expected.FirstError = intp(7) }, "expected.first_error", "not a line"},
		{"a first error on a crossed-out line", sample.ScenarioIllegible, func(s *sample.Sample) {
			s.Expected.Outcome = sample.OutcomeError
			s.Expected.FirstError = intp(1)
		}, "expected.first_error", "crossed out"},
		{"an outcome against the scenario", sample.ScenarioError, func(s *sample.Sample) {
			s.Expected.Outcome = sample.OutcomeCorrect
			s.Expected.FirstError = nil
		}, "expected.outcome", `the scenario "error" is "error"`},
		{"an unreadable sheet with a legible answer", sample.ScenarioIllegible, func(s *sample.Sample) { s.Lines[2].Illegible = false }, "lines[2]", "marked illegible"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := edit(t, tt.scenario, tt.change)
			wantProblem(t, s.Check(), tt.path, tt.want)
			// The reader refuses the file the same way.
			data, err := sample.Marshal(s)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := sample.ReadSample(bytes.NewReader(data)); err == nil {
				t.Error("ReadSample took the broken sample")
			}
		})
	}
}

func TestReadSampleRefusesWhatJSONCannotHold(t *testing.T) {
	good, err := sample.Marshal(synth.Sample(sample.ScenarioCorrect))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		data string
		want string
	}{
		{"an unknown field", strings.Replace(string(good), `"task":`, `"colour": "red", "task":`, 1), `unknown field "colour"`},
		{"data after the object", string(good) + "{}", "unexpected data"},
		{"not JSON", "<sample/>", "invalid character"},
		{"a string for a number", strings.Replace(string(good), `"width": 1000`, `"width": "1000"`, 1), "sheet.width: is not a number"},
		{"a text cut short", string(good[:len(good)/2]), "the JSON text ends too early"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := sample.ReadSample(strings.NewReader(tt.data))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("ReadSample error = %v, want one holding %q", err, tt.want)
			}
		})
	}
}

func TestCheckListsEveryProblem(t *testing.T) {
	s := edit(t, sample.ScenarioCorrect, func(s *sample.Sample) {
		s.Task = ""
		s.Writer = "child"
		s.Strokes[0].Points[0].Pressure = 2
	})
	err := s.Check()
	var fe *sample.FormatError
	if !errors.As(err, &fe) || len(fe.Problems) != 3 {
		t.Fatalf("Check() = %v, want three problems", err)
	}
	if msg := err.Error(); !strings.HasPrefix(msg, "invalid sample, 3 problems:\n  task: ") {
		t.Errorf("Error() = %q, want one problem to a line", msg)
	}
}

func TestCheckWithBank(t *testing.T) {
	bank := synth.Bank()
	s := edit(t, sample.ScenarioCorrect, func(s *sample.Sample) { s.Task = "no-such-task" })
	wantProblem(t, s.CheckWithBank(bank), "task", "not in the bank")
	s = edit(t, sample.ScenarioError, func(s *sample.Sample) { s.Expected.Trap = "no-such-trap" })
	wantProblem(t, s.CheckWithBank(bank), "expected.trap", "not a trap of the task")
}

// editBank returns a copy of the synthetic bank with a change made to it.
func editBank(t *testing.T, change func(b *sample.Bank)) *sample.Bank {
	t.Helper()
	data, err := json.Marshal(synth.Bank())
	if err != nil {
		t.Fatal(err)
	}
	var b sample.Bank
	if err := json.Unmarshal(data, &b); err != nil {
		t.Fatal(err)
	}
	change(&b)
	return &b
}

func TestCheckRefusesBrokenBanks(t *testing.T) {
	tests := []struct {
		name   string
		change func(b *sample.Bank)
		path   string
		want   string
	}{
		{"another format", func(b *sample.Bank) { b.Format = "tutor-sample" }, "format", `not "tutor-bank"`},
		{"a newer version", func(b *sample.Bank) { b.Version = intp(2) }, "version", "2 is not supported"},
		{"no tasks", func(b *sample.Bank) { b.Tasks = nil }, "tasks", "are missing"},
		{"a repeated task ID", func(b *sample.Bank) { b.Tasks[1].ID = b.Tasks[0].ID }, "tasks[1].id", "earlier task"},
		{"an unknown kind", func(b *sample.Bank) { b.Tasks[0].Kind = "essay" }, "tasks[0].kind", `"essay"`},
		{"no statement", func(b *sample.Bank) { b.Tasks[0].Statement = "" }, "tasks[0].statement", "is missing"},
		{"no key", func(b *sample.Bank) { b.Tasks[0].Key = "" }, "tasks[0].key", "is missing"},
		{"no solution", func(b *sample.Bank) { b.Tasks[0].Solution = nil }, "tasks[0].solution", "is missing"},
		{"an empty solution line", func(b *sample.Bank) { b.Tasks[0].Solution[1] = "" }, "tasks[0].solution[1]", "is empty"},
		{"a repeated trap ID", func(b *sample.Bank) { b.Tasks[0].Traps[1].ID = b.Tasks[0].Traps[0].ID }, "tasks[0].traps[1].id", "earlier trap"},
		{"a trap without a description", func(b *sample.Bank) { b.Tasks[0].Traps[0].Description = "" }, "tasks[0].traps[0].description", "is missing"},
		{"an empty leak", func(b *sample.Bank) { b.Tasks[0].Leaks[0] = "" }, "tasks[0].leaks[0]", "is empty"},
		{"no false key", func(b *sample.Bank) { b.Tasks[0].FalseKeys = nil }, "tasks[0].false_keys", "are missing"},
		{"a false key that is the key", func(b *sample.Bank) { b.Tasks[0].FalseKeys[0].Key = b.Tasks[0].Key }, "tasks[0].false_keys[0].key", "the task's key"},
		{"a false solution of other steps", func(b *sample.Bank) {
			b.Tasks[0].FalseKeys[0].Solution = b.Tasks[0].FalseKeys[0].Solution[:1]
		}, "tasks[0].false_keys[0].solution", "same steps"},
		{"a false solution that is the solution", func(b *sample.Bank) {
			b.Tasks[0].FalseKeys[0].Solution = append([]string(nil), b.Tasks[0].Solution...)
		}, "tasks[0].false_keys[0].solution", "is the task's solution"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := editBank(t, tt.change)
			wantProblem(t, b.Check(), tt.path, tt.want)
		})
	}
}

func TestReadSampleIsStrict(t *testing.T) {
	good, err := sample.Marshal(synth.Sample(sample.ScenarioCorrect))
	if err != nil {
		t.Fatal(err)
	}
	text := string(good)
	firstPressure := strings.Index(text, `"pressure": `)
	tests := []struct {
		name string
		data string
		path string
		want string
	}{
		{"a missing field", regexp.MustCompile(`,\s+"pressure": [0-9.]+`).ReplaceAllLiteralString(text[:firstPressure+40], "") + text[firstPressure+40:], "strokes[0].points[0].pressure", "is missing"},
		{"a field spelt in another case", strings.Replace(text, `"pressure": `, `"Pressure": `, 1), "strokes[0].points[0]", `unknown field "Pressure"`},
		{"a field given twice", strings.Replace(text, `"task": `, `"writer": "author", "task": `, 1), "writer", "appears twice"},
		{"a null where a value is required", strings.Replace(text, `"task": "synthetic-linear-01"`, `"task": null`, 1), "task", "is null"},
		{"a fraction where a whole number is required", regexp.MustCompile(`("t_ms": [0-9]+)`).ReplaceAllString(text[:firstPressure], "${1}.5") + text[firstPressure:], "strokes[0].points[0].t_ms", "not a whole number"},
		{"an object where an array is required", strings.Replace(text, `"erasures": []`, `"erasures": {}`, 1), "erasures", "is not an array"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := sample.ReadSample(strings.NewReader(tt.data))
			wantProblem(t, err, tt.path, tt.want)
		})
	}
}

func TestReadSampleNamesEveryMissingField(t *testing.T) {
	_, err := sample.ReadSample(strings.NewReader(`{"format": "tutor-sample", "version": 0, "strokes": [{"id": "s-1", "points": [{"x": 1, "y": 2}]}]}`))
	for _, path := range []string{"strokes[0].points[0].t_ms", "strokes[0].points[0].pressure", "sheet", "expected"} {
		wantProblem(t, err, path, "is missing")
	}
}
