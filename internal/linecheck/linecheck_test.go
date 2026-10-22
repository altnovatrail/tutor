package linecheck

import (
	"slices"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/synth"
)

func TestNormalize(t *testing.T) {
	tests := map[string]string{
		"3x + 1 = 7":     "3x+1=7",
		"5 − 1 = 4":      "5-1=4",
		"5 – 1 = 4":      "5-1=4",
		"14 × 2 = 28":    "14*2=28",
		"14 · 2 = 28":    "14*2=28",
		"66 ÷ 6 = 11":    "66:6=11",
		"x = 8/3":        "x=8/3",
		" 0,5 x\t= 1\n":  "0,5x=1",
		"Ответ: 5":       "Ответ:5",
		"(47 + 19) : 6 ": "(47+19):6",
	}
	for in, want := range tests {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func linearTask(equation, key string) *sample.Task {
	return &sample.Task{Kind: sample.KindLinearEquation, Key: key, Solution: []string{equation}}
}

func verdicts(r Result) []Verdict {
	var v []Verdict
	for _, l := range r.Lines {
		v = append(v, l.Verdict)
	}
	return v
}

func TestLinearEquation(t *testing.T) {
	const C, W, U = Correct, Wrong, Unchecked
	tests := []struct {
		name      string
		equation  string
		key       string
		lines     []string
		want      []Verdict
		first     int
		known     bool
		answer    Verdict
		reasonHas string // a part of the first non-correct line's reason
	}{
		{"a correct solution", "3x + 1 = 7", "x = 2",
			[]string{"3x + 1 = 7", "3x = 6", "x = 2"}, []Verdict{C, C, C}, 0, true, C, ""},
		{"the sign kept on transfer", "3x + 1 = 7", "x = 2",
			[]string{"3x + 1 = 7", "3x = 7 + 1", "3x = 8", "x = 8/3"}, []Verdict{C, W, W, W}, 2, true, W, "solutions are 8/3"},
		{"one side divided", "3x + 3 = 12", "x = 3",
			[]string{"3x + 3 = 12", "x + 3 = 4", "x = 1"}, []Verdict{C, W, W}, 2, true, W, ""},
		{"the minus before a bracket", "5 − (x + 2) = 1", "x = 2",
			[]string{"5 − (x + 2) = 1", "5 − x + 2 = 1", "−x = −6", "x = 6"}, []Verdict{C, W, W, W}, 2, true, W, ""},
		{"the minus before a bracket, done right", "5 − (x + 2) = 1", "x = 2",
			[]string{"5 − x − 2 = 1", "3 − x = 1", "x = 2"}, []Verdict{C, C, C}, 0, true, C, ""},
		{"arithmetic", "3x + 1 = 7", "x = 2",
			[]string{"3x = 6", "x = 3"}, []Verdict{C, W}, 2, true, W, ""},
		{"brackets multiplied", "2(x + 1) = 6", "x = 2",
			[]string{"2x + 2 = 6", "2x = 4", "2 = x"}, []Verdict{C, C, C}, 0, true, C, ""},
		{"decimals with a comma", "0,5x = 1", "x = 2",
			[]string{"0.5 · x = 1", "x = 1 : 0,5", "x = 2"}, []Verdict{C, C, C}, 0, true, C, ""},
		{"an identity is not the equation", "3x + 1 = 7", "x = 2",
			[]string{"3x + 1 − 3x = 1", "1 = 1"}, []Verdict{W, W}, 1, true, U, "solutions are all"},
		{"an answer not solved for x", "3x + 1 = 7", "x = 2",
			[]string{"3x + 1 = 7", "3x = 6"}, []Verdict{C, C}, 0, true, U, ""},
		{"a line the code cannot read", "3x + 1 = 7", "x = 2",
			[]string{"3x + 1 = 7", "3x² = 6", "x = 3"}, []Verdict{C, U, W}, 0, false, W, "not understood"},
		{"another letter", "3x + 1 = 7", "x = 2",
			[]string{"3y + 1 = 7"}, []Verdict{U}, 0, false, U, "the letter 'y' is not the unknown"},
		{"a product of unknowns", "3x + 1 = 7", "x = 2",
			[]string{"3x · x = 6x"}, []Verdict{U}, 0, false, U, "two unknowns"},
		{"a division by zero", "3x + 1 = 7", "x = 2",
			[]string{"3x = 6 : 0"}, []Verdict{U}, 0, false, U, "division by zero"},
		{"no equals sign", "3x + 1 = 7", "x = 2",
			[]string{"3x + 1"}, []Verdict{U}, 0, false, U, `one "="`},
		{"a broken expression", "3x + 1 = 7", "x = 2",
			[]string{"3x + = 7", "(x + 1 = 2"}, []Verdict{U, U}, 0, false, U, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := Check(linearTask(tt.equation, tt.key), tt.lines, len(tt.lines))
			if got := verdicts(r); !slices.Equal(got, tt.want) {
				t.Errorf("verdicts = %v, want %v (%+v)", got, tt.want, r.Lines)
			}
			if r.FirstError != tt.first || r.FirstErrorKnown != tt.known {
				t.Errorf("first error = %d, known %v; want %d, known %v", r.FirstError, r.FirstErrorKnown, tt.first, tt.known)
			}
			if r.Answer != tt.answer {
				t.Errorf("answer = %v (%s), want %v", r.Answer, r.AnswerReason, tt.answer)
			}
			if tt.reasonHas != "" {
				for _, l := range r.Lines {
					if l.Verdict != Correct {
						if !strings.Contains(l.Reason, tt.reasonHas) {
							t.Errorf("reason = %q, want it to hold %q", l.Reason, tt.reasonHas)
						}
						break
					}
				}
			}
		})
	}
}

func TestCalculationChain(t *testing.T) {
	const C, W, U = Correct, Wrong, Unchecked
	task := &sample.Task{Kind: sample.KindCalculation, Key: "38", Solution: []string{"17 + 49 − 14 × 2 = 38"}}
	tests := []struct {
		name   string
		lines  []string
		want   []Verdict
		first  int
		known  bool
		answer Verdict
	}{
		{"one line", []string{"17 + 49 − 14 × 2 = 17 + 49 − 28 = 38"}, []Verdict{C}, 0, true, C},
		{"lines that go on", []string{"17 + 49 − 14 × 2 = 17 + 49 − 28", "= 66 − 28", "= 38"}, []Verdict{C, C, C}, 0, true, C},
		{"order of operations ignored", []string{"17 + 49 − 14 × 2 = 66 − 14 × 2", "= 52 × 2", "= 104"}, []Verdict{C, W, C}, 2, true, W},
		{"arithmetic", []string{"17 + 49 = 56", "56 − 28 = 28"}, []Verdict{W, C}, 1, true, W},
		{"a label on the answer", []string{"17 + 49 − 28 = 38", "Ответ: 38"}, []Verdict{C, C}, 0, true, C},
		{"going on from an unreadable line", []string{"17 + 4? = 66", "= 38"}, []Verdict{U, U}, 0, false, C},
		{"a letter", []string{"x + 1 = 2"}, []Verdict{U}, 0, false, U},
		{"division and fractions", []string{"7 : 2 = 3,5", "7/2 = 3.5"}, []Verdict{C, C}, 0, true, W},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := Check(task, tt.lines, len(tt.lines))
			if got := verdicts(r); !slices.Equal(got, tt.want) {
				t.Errorf("verdicts = %v, want %v (%+v)", got, tt.want, r.Lines)
			}
			if r.FirstError != tt.first || r.FirstErrorKnown != tt.known {
				t.Errorf("first error = %d, known %v; want %d, known %v", r.FirstError, r.FirstErrorKnown, tt.first, tt.known)
			}
			if r.Answer != tt.answer {
				t.Errorf("answer = %v (%s), want %v", r.Answer, r.AnswerReason, tt.answer)
			}
		})
	}
}

func TestFreeSolution(t *testing.T) {
	task := &sample.Task{Kind: sample.KindFreeSolution, Key: "5", Solution: []string{"20 : 5 = 4", "4 + 1 = 5", "Ответ: 5"}}
	r := Check(task, []string{"20 : 5 = 4", "4 + 1 = 5", "Ответ: 5"}, 3)
	if r.FirstErrorKnown || r.Answer != Correct {
		t.Errorf("first error known %v, answer %v; want unknown and correct", r.FirstErrorKnown, r.Answer)
	}
	for _, l := range r.Lines {
		if l.Verdict != Unchecked {
			t.Errorf("a line of a free solution got %v, want %v: its markup judges it", l.Verdict, Unchecked)
		}
	}
	if r := Check(task, []string{"20 : 5 = 4", "Ответ: 4"}, 2); r.Answer != Wrong {
		t.Errorf("answer 4 to the key 5 = %v, want %v", r.Answer, Wrong)
	}
	if r := Check(task, []string{"20 : 5 = 4"}, 0); r.Answer != Unchecked {
		t.Errorf("no answer line = %v, want %v", r.Answer, Unchecked)
	}
}

// The synthetic bank: each task's own solution is correct throughout, and each false key's
// solution parts from it, as the condition with a false key needs (R03).
func TestSyntheticBank(t *testing.T) {
	for _, task := range synth.Bank().Tasks {
		t.Run(task.ID, func(t *testing.T) {
			r := Check(&task, task.Solution, len(task.Solution))
			if r.Answer != Correct {
				t.Errorf("the task's solution: answer %v (%s), want correct", r.Answer, r.AnswerReason)
			}
			if task.Kind != sample.KindFreeSolution && (r.FirstError != 0 || !r.FirstErrorKnown) {
				t.Errorf("the task's solution: first error %d, known %v; want none", r.FirstError, r.FirstErrorKnown)
			}
			for _, fk := range task.FalseKeys {
				r := Check(&task, fk.Solution, len(fk.Solution))
				if r.Answer != Wrong {
					t.Errorf("false key %q: answer %v, want wrong by the true key", fk.Key, r.Answer)
				}
				if task.Kind != sample.KindFreeSolution && r.FirstError == 0 {
					t.Errorf("false key %q: no wrong line", fk.Key)
				}
			}
		})
	}
}

// The synthetic samples: what the check finds in their intended lines is what their markup says.
func TestSyntheticSamples(t *testing.T) {
	bank := synth.Bank()
	for _, s := range synth.Samples() {
		task := bank.Task(s.Task)
		if task.Kind == sample.KindFreeSolution || s.Expected.Outcome == sample.OutcomeUnreadable || s.Expected.Outcome == sample.OutcomeNoAnswer {
			continue
		}
		t.Run(string(s.Scenario), func(t *testing.T) {
			var lines []string
			var numbers []int // the sample's line number of each line checked
			answer := 0
			for i, l := range s.Lines {
				if l.CrossedOut {
					continue
				}
				lines = append(lines, l.Text)
				numbers = append(numbers, i+1)
				if l.Answer {
					answer = len(lines)
				}
			}
			r := Check(task, lines, answer)
			first := 0
			if r.FirstError > 0 {
				first = numbers[r.FirstError-1]
			}
			want := 0
			if s.Expected.FirstError != nil {
				want = *s.Expected.FirstError
			}
			if !r.FirstErrorKnown || first != want {
				t.Errorf("first error = line %d (known %v), the markup says %d", first, r.FirstErrorKnown, want)
			}
			wantAnswer := Correct
			if s.Expected.Outcome == sample.OutcomeError {
				wantAnswer = Wrong
			}
			if r.Answer != wantAnswer {
				t.Errorf("answer = %v (%s), the markup says %v", r.Answer, r.AnswerReason, wantAnswer)
			}
		})
	}
}

// What the review of T14 found: none of these may panic, and each gets the verdict it should.
func TestReviewCases(t *testing.T) {
	eq := linearTask("3x + 1 = 7", "x = 2")
	chain := &sample.Task{Kind: sample.KindCalculation, Key: "38", Solution: []string{"17 + 21 = 38"}}
	tests := []struct {
		name   string
		task   *sample.Task
		lines  []string
		want   []Verdict
		first  int
		known  bool
		answer Verdict
	}{
		{"a minus before nothing", eq, []string{"3x + 1 = -", "-? = 7"}, []Verdict{Unchecked, Unchecked}, 0, false, Unchecked},
		{"a minus before nothing in a chain", chain, []string{"17 + 21 = -"}, []Verdict{Unchecked}, 0, false, Unchecked},
		{"a labelled answer to an equation", eq, []string{"3x = 6", "Ответ: x = 2"}, []Verdict{Correct, Correct}, 0, true, Correct},
		{"the Cyrillic х and a capital X", eq, []string{"3х = 6", "X = 2"}, []Verdict{Correct, Correct}, 0, true, Correct},
		{"an answer that is not what the chain gave", chain, []string{"17 + 21 = 38", "Ответ: 40"}, []Verdict{Correct, Wrong}, 2, true, Wrong},
		{"an answer that restates the chain", chain, []string{"17 + 21 = 38", "Ответ: 38"}, []Verdict{Correct, Correct}, 0, true, Correct},
		{"a task without a solution", &sample.Task{Kind: sample.KindLinearEquation, Key: "x = 2"}, []string{"x = 2"}, []Verdict{Unchecked}, 0, false, Correct},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := Check(tt.task, tt.lines, len(tt.lines))
			if got := verdicts(r); !slices.Equal(got, tt.want) {
				t.Errorf("verdicts = %v, want %v (%+v)", got, tt.want, r.Lines)
			}
			if r.FirstError != tt.first || r.FirstErrorKnown != tt.known {
				t.Errorf("first error = %d, known %v; want %d, known %v", r.FirstError, r.FirstErrorKnown, tt.first, tt.known)
			}
			if r.Answer != tt.answer {
				t.Errorf("answer = %v (%s), want %v", r.Answer, r.AnswerReason, tt.answer)
			}
		})
	}
}
