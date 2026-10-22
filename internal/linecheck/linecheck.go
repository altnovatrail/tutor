// Package linecheck is the measurement's verdict line by line: it says whether each line of a
// solution is correct, finds the first wrong one and checks the answer by the key. It stands in
// for module-kit, whose code has no check of this kind yet, and only in the measurement (R05; В10
// in RUN.md): the platform's verdicts come from module-kit (R03 spec).
//
// A line the code cannot read as the bank's notation gets no verdict — "unchecked", not "wrong"
// (R04 spec) — and a report counts such lines apart from the model's.
package linecheck

import (
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"unicode"

	"github.com/altnovatrail/tutor/internal/sample"
)

// Verdict is the verdict on a line or on the answer.
type Verdict string

// The verdicts of the line check.
const (
	Correct Verdict = "correct"
	// Wrong is "not yet" to the learner (concepts 9.1 in spec).
	Wrong Verdict = "wrong"
	// Unchecked: the code could not read the line, so it gives no verdict (R04 spec).
	Unchecked Verdict = "unchecked"
)

// Line is the verdict on one line, with the reason when it is not correct.
type Line struct {
	Verdict Verdict
	Reason  string
	// NoClaim marks the first line of a chain when it only states a value, such as "38": there is
	// nothing to compare it with, so it is unchecked but cannot hide a first error. A line that only
	// states a value after another line, such as "Ответ: 38", is compared with the value that line
	// ends with.
	NoClaim bool
}

// Result is the verdict on a whole solution.
type Result struct {
	Lines []Line
	// FirstError is the number, from 1, of the first wrong line, and 0 when there is none. It is
	// known only when every line before it was checked: an unchecked line before it might be the
	// first error itself. A free solution's first error comes from its markup, not from here.
	FirstError      int
	FirstErrorKnown bool
	// Answer is the verdict on the answer line by the task's key.
	Answer       Verdict
	AnswerReason string
}

// Normalize writes a line the one way the check and the reading accuracy compare it in
// (docs/architecture.md, section 3.1): no spaces, the minus, times and division signs each as one
// character — "-", "*" and ":" — and the unknown x as the Latin small letter, whether it was read
// as the Cyrillic "х" or as a capital. The fraction bar "/" stays as written.
func Normalize(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsSpace(r):
		case strings.ContainsRune("xXхХ", r):
			b.WriteRune('x')
		case strings.ContainsRune("−–—‐‑", r):
			b.WriteRune('-')
		case strings.ContainsRune("×·⋅∙", r):
			b.WriteRune('*')
		case r == '÷':
			b.WriteRune(':')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// answerLabel is the word a written answer starts with: "Ответ: 5".
var answerLabel = regexp.MustCompile(`^(?i)\s*(ответ|answer)\s*[:.]?`)

// Check gives the verdict on the lines of a solution to a task, in the order they are read. answer
// is the number, from 1, of the line that gives the answer, or 0 when there is none. Lines that are
// not part of the solution, such as crossed-out ones, are left out before the call.
func Check(task *sample.Task, lines []string, answer int) Result {
	res := Result{Lines: make([]Line, len(lines))}
	switch task.Kind {
	case sample.KindLinearEquation:
		checkEquation(task, lines, &res)
	case sample.KindCalculation:
		checkChain(lines, &res)
	default:
		for i := range res.Lines {
			res.Lines[i] = Line{Verdict: Unchecked, Reason: "a free solution's lines are judged by its markup"}
		}
	}
	if task.Kind != sample.KindFreeSolution {
		res.FirstErrorKnown = true
		for i, l := range res.Lines {
			if l.Verdict == Unchecked && !l.NoClaim {
				res.FirstErrorKnown = false
				break
			}
			if l.Verdict == Wrong {
				res.FirstError = i + 1
				break
			}
		}
	}
	res.Answer, res.AnswerReason = checkAnswer(task, lines, answer)
	return res
}

// unknownOf is the letter of the unknown in an equation: its only letter.
func unknownOf(equation string) (rune, error) {
	var u rune
	for _, r := range Normalize(equation) {
		if unicode.IsLetter(r) {
			if u != 0 && r != u {
				return 0, fmt.Errorf("two letters, %q and %q", u, r)
			}
			u = r
		}
	}
	if u == 0 {
		return 0, fmt.Errorf("no unknown")
	}
	return u, nil
}

// solutions is the set of solutions of a linear equation a*u + b = 0: one number, every number,
// or none.
type solutions struct {
	kind  string // "one", "all", "none"
	value *big.Rat
}

func (s solutions) equal(t solutions) bool {
	return s.kind == t.kind && (s.kind != "one" || s.value.Cmp(t.value) == 0)
}

func (s solutions) String() string {
	if s.kind == "one" {
		return s.value.RatString()
	}
	return s.kind
}

// equation reads a line "left = right" in the unknown and returns both sides.
func equation(line string, u rune) (left, right linear, err error) {
	parts := strings.Split(Normalize(line), "=")
	if len(parts) != 2 {
		return linear{}, linear{}, notChecked("an equation has one \"=\", this line %d", len(parts)-1)
	}
	if left, err = parseExpr(parts[0], u); err != nil {
		return linear{}, linear{}, err
	}
	if right, err = parseExpr(parts[1], u); err != nil {
		return linear{}, linear{}, err
	}
	return left, right, nil
}

func solve(left, right linear) solutions {
	d := left.add(right.neg())
	switch {
	case d.a.Sign() != 0:
		return solutions{kind: "one", value: new(big.Rat).Neg(new(big.Rat).Quo(d.b, d.a))}
	case d.b.Sign() == 0:
		return solutions{kind: "all"}
	default:
		return solutions{kind: "none"}
	}
}

// checkEquation judges each line of a linear equation's solution: a line is correct when it has
// the same solutions as the task's equation, the first line of its solution in the bank.
func checkEquation(task *sample.Task, lines []string, res *Result) {
	if len(task.Solution) == 0 {
		for i := range res.Lines {
			res.Lines[i] = Line{Verdict: Unchecked, Reason: "the task has no solution to take its equation from"}
		}
		return
	}
	origin := task.Solution[0]
	u, err := unknownOf(origin)
	var want solutions
	if err == nil {
		var l, r linear
		if l, r, err = equation(origin, u); err == nil {
			want = solve(l, r)
		}
	}
	if err != nil {
		for i := range res.Lines {
			res.Lines[i] = Line{Verdict: Unchecked, Reason: fmt.Sprintf("the task's equation %q cannot be read: %v", origin, err)}
		}
		return
	}
	for i, line := range lines {
		l, r, err := equation(answerLabel.ReplaceAllString(line, ""), u)
		if err != nil {
			res.Lines[i] = Line{Verdict: Unchecked, Reason: err.Error()}
			continue
		}
		if got := solve(l, r); got.equal(want) {
			res.Lines[i] = Line{Verdict: Correct}
		} else {
			res.Lines[i] = Line{Verdict: Wrong, Reason: fmt.Sprintf("its solutions are %s, the equation's %s", got, want)}
		}
	}
}

// checkChain judges each line of a chain of calculations: every part between "=" signs has the
// same value. A line that starts with "=" goes on from the value the line before it ends with.
func checkChain(lines []string, res *Result) {
	var last *big.Rat // the value the previous line ends with, nil when it has none
	for i, line := range lines {
		values, err := chainValues(answerLabel.ReplaceAllString(line, ""), last)
		switch {
		case err != nil:
			res.Lines[i] = Line{Verdict: Unchecked, Reason: err.Error()}
			last = nil
			continue
		case len(values) < 2 && last == nil:
			res.Lines[i] = Line{Verdict: Unchecked, Reason: "no \"=\": the line only states a value", NoClaim: true}
		case len(values) < 2 && values[0].Cmp(last) == 0:
			res.Lines[i] = Line{Verdict: Correct}
		case len(values) < 2:
			res.Lines[i] = Line{Verdict: Wrong, Reason: fmt.Sprintf("%s, while the line before ends with %s", values[0].RatString(), last.RatString())}
		default:
			res.Lines[i] = Line{Verdict: Correct}
			for _, v := range values[1:] {
				if v.Cmp(values[0]) != 0 {
					res.Lines[i] = Line{Verdict: Wrong, Reason: fmt.Sprintf("%s is not %s", v.RatString(), values[0].RatString())}
					break
				}
			}
		}
		last = values[len(values)-1]
	}
}

// chainValues reads the parts of a calculation line. A leading "=" stands for previous.
func chainValues(line string, previous *big.Rat) ([]*big.Rat, error) {
	parts := strings.Split(Normalize(line), "=")
	var values []*big.Rat
	for j, part := range parts {
		if j == 0 && part == "" && len(parts) > 1 {
			if previous == nil {
				return nil, notChecked("the line goes on from a line with no value")
			}
			values = append(values, previous)
			continue
		}
		l, err := parseExpr(part, 0)
		if err != nil {
			return nil, err
		}
		values = append(values, l.b)
	}
	return values, nil
}

// checkAnswer judges the answer line by the task's key.
func checkAnswer(task *sample.Task, lines []string, answer int) (Verdict, string) {
	if answer < 1 || answer > len(lines) {
		return Unchecked, "no answer line"
	}
	text := answerLabel.ReplaceAllString(lines[answer-1], "")
	if task.Kind == sample.KindLinearEquation {
		return equationAnswer(task, text)
	}
	got, err := lastValue(text)
	if err != nil {
		return Unchecked, err.Error()
	}
	want, err := lastValue(task.Key)
	if err != nil {
		if Normalize(text) == Normalize(task.Key) {
			return Correct, ""
		}
		return Unchecked, fmt.Sprintf("the key %q is not a number", task.Key)
	}
	if got.Cmp(want) != 0 {
		return Wrong, fmt.Sprintf("%s, the key %s", got.RatString(), want.RatString())
	}
	return Correct, ""
}

// lastValue is the value a line ends with: its last part after "=", or the whole line. Every part
// must read, so that a line the check cannot read gives no answer either. A leading "=", which goes
// on from the line before, is dropped: the answer is what the line ends with.
func lastValue(text string) (*big.Rat, error) {
	values, err := chainValues(strings.TrimPrefix(strings.TrimSpace(text), "="), nil)
	if err != nil {
		return nil, err
	}
	return values[len(values)-1], nil
}

// equationAnswer judges an equation's answer: the unknown alone on one side and a number on the
// other, equal to the key's. An answer in another form is not judged.
func equationAnswer(task *sample.Task, text string) (Verdict, string) {
	u, err := unknownOf(task.Key)
	if err != nil {
		return Unchecked, fmt.Sprintf("the key %q: %v", task.Key, err)
	}
	kl, kr, err := equation(task.Key, u)
	if err != nil {
		return Unchecked, fmt.Sprintf("the key %q: %v", task.Key, err)
	}
	l, r, err := equation(text, u)
	if err != nil {
		return Unchecked, err.Error()
	}
	isolated := func(x, y linear) bool {
		return x.a.Cmp(big.NewRat(1, 1)) == 0 && x.b.Sign() == 0 && y.isConstant()
	}
	if !isolated(l, r) && !isolated(r, l) {
		return Unchecked, fmt.Sprintf("not in the form %c = number", u)
	}
	if got, want := solve(l, r), solve(kl, kr); !got.equal(want) {
		return Wrong, fmt.Sprintf("%c = %s, the key %s", u, got, want)
	}
	return Correct, ""
}
