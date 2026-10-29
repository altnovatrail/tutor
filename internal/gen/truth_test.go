package gen_test

import (
	"fmt"
	"math/big"
	"slices"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/internal/gen"
	"github.com/altnovatrail/tutor/internal/sample"
)

// evaluate is the value of an expression of the lines at x: numbers, x with a coefficient before it
// ("5x"), + − × : and / between numbers and parentheses, as a rational.
func evaluate(expr string, x *big.Rat) (*big.Rat, error) {
	p := &parser{runes: []rune(strings.ReplaceAll(expr, " ", "")), x: x}
	v, err := p.sum()
	if err == nil && p.at < len(p.runes) {
		err = fmt.Errorf("%q: %q is left over", expr, string(p.runes[p.at:]))
	}
	return v, err
}

// parser reads an expression of the lines, rune by rune.
type parser struct {
	runes []rune
	at    int
	x     *big.Rat
}

func (p *parser) peek() rune {
	if p.at < len(p.runes) {
		return p.runes[p.at]
	}
	return 0
}

func (p *parser) sum() (*big.Rat, error) {
	v, err := p.product()
	for err == nil && (p.peek() == '+' || p.peek() == '−') {
		op := p.peek()
		p.at++
		var w *big.Rat
		if w, err = p.product(); err == nil {
			if op == '+' {
				v = new(big.Rat).Add(v, w)
			} else {
				v = new(big.Rat).Sub(v, w)
			}
		}
	}
	return v, err
}

func (p *parser) product() (*big.Rat, error) {
	v, err := p.factor()
	for err == nil && (p.peek() == '×' || p.peek() == ':' || p.peek() == '/') {
		op := p.peek()
		p.at++
		var w *big.Rat
		if w, err = p.factor(); err == nil {
			switch {
			case op == '×':
				v = new(big.Rat).Mul(v, w)
			case w.Sign() == 0:
				err = fmt.Errorf("a division by 0")
			default:
				v = new(big.Rat).Quo(v, w)
			}
		}
	}
	return v, err
}

func (p *parser) factor() (*big.Rat, error) {
	switch r := p.peek(); {
	case r == '(':
		p.at++
		v, err := p.sum()
		if err == nil && p.peek() != ')' {
			err = fmt.Errorf("no closing bracket")
		}
		p.at++
		return v, err
	case r == 'x':
		p.at++
		return new(big.Rat).Set(p.x), nil
	case r >= '0' && r <= '9':
		start := p.at
		for p.peek() >= '0' && p.peek() <= '9' {
			p.at++
		}
		v, _ := new(big.Rat).SetString(string(p.runes[start:p.at]))
		if p.peek() == 'x' {
			p.at++
			v = new(big.Rat).Mul(v, p.x)
		}
		return v, nil
	}
	return nil, fmt.Errorf("%q at %d", string(p.runes), p.at)
}

// truth tells which lines of a task's sheets are right: of an equation, a line that its root keeps
// true, both sides equal at x = the key; of a chain, a line whose arithmetic is right and whose result
// is a number of the solution — so a step of the wrong order of operations, right as arithmetic, is
// wrong. A check after the answer is read as its equation.
type truth struct {
	task   *sample.Task
	root   *big.Rat
	values []*big.Rat
}

func newTruth(t *testing.T, task *sample.Task) truth {
	t.Helper()
	tr := truth{task: task}
	if task.Kind == sample.KindLinearEquation {
		root, ok := new(big.Rat).SetString(strings.TrimPrefix(task.Key, "x = "))
		if !ok {
			t.Fatalf("%s: the key %q", task.ID, task.Key)
		}
		tr.root = root
		return tr
	}
	for _, line := range task.Solution {
		_, right, _ := strings.Cut(line, " = ")
		v, ok := new(big.Rat).SetString(right)
		if !ok {
			t.Fatalf("%s: the line %q", task.ID, line)
		}
		tr.values = append(tr.values, v)
	}
	return tr
}

// right is whether a line is right.
func (tr truth) right(t *testing.T, line string) bool {
	t.Helper()
	left, rightSide, ok := strings.Cut(strings.TrimPrefix(line, "Проверка: "), " = ")
	if !ok {
		t.Fatalf("%s: the line %q has no =", tr.task.ID, line)
	}
	x := tr.root
	if x == nil {
		x = new(big.Rat)
	}
	l, err := evaluate(left, x)
	if err != nil {
		t.Fatalf("%s: %v", tr.task.ID, err)
	}
	r, err := evaluate(rightSide, x)
	if err != nil {
		t.Fatalf("%s: %v", tr.task.ID, err)
	}
	if l.Cmp(r) != 0 {
		return false
	}
	return tr.root != nil || slices.ContainsFunc(tr.values, func(v *big.Rat) bool { return v.Cmp(r) == 0 })
}

// firstWrong is the number of the first wrong line, from 1, or 0 when every line is right.
func (tr truth) firstWrong(t *testing.T, lines []string) int {
	t.Helper()
	for i, line := range lines {
		if !tr.right(t, line) {
			return i + 1
		}
	}
	return 0
}

// The truth of every generated sheet, by what its lines say: a correct sheet is right in every line;
// a sheet with an error is right up to its first error and wrong on it; a mistake erased was wrong
// and its replacement is right; a right step erased was right and its replacement is wrong, the
// first error of the sheet. Over many seeds, so that no case of the numbers escapes.
func TestTruth(t *testing.T) {
	for seed := uint64(1); seed <= 60; seed++ {
		for _, made := range []func() (*sample.Bank, *sample.Plan, error){
			func() (*sample.Bank, *sample.Plan, error) { return gen.Sheets(16, seed, "s") },
			func() (*sample.Bank, *sample.Plan, error) { return gen.NearSheets(49, seed, "n") },
		} {
			bank, plan, err := made()
			if err != nil {
				t.Fatal(err)
			}
			for i := range plan.Sheets {
				sheetTruth(t, bank, &plan.Sheets[i])
			}
		}
	}
}

// sheetTruth checks the truth of one sheet.
func sheetTruth(t *testing.T, bank *sample.Bank, sh *sample.PlanSheet) {
	t.Helper()
	tr := newTruth(t, bank.Task(sh.Task))
	first := tr.firstWrong(t, sh.Lines)
	switch sh.Expected.Outcome {
	case sample.OutcomeCorrect:
		if first != 0 {
			t.Errorf("%s, sheet %d (%s): line %d of %q is wrong", sh.Task, sh.Number, sh.Scenario, first, sh.Lines)
		}
	case sample.OutcomeError:
		if sh.Expected.FirstError == nil || first != *sh.Expected.FirstError {
			t.Errorf("%s, sheet %d (%s): the first wrong line is %d of %q, the plan says %v", sh.Task, sh.Number, sh.Scenario, first, sh.Lines, sh.Expected.FirstError)
		}
		if sh.Scenario != sample.ScenarioSimilarDigits && sh.Expected.Trap == "" {
			t.Errorf("%s, sheet %d: an error with no trap", sh.Task, sh.Number)
		}
	}
	e := sh.Erasure
	if e == nil {
		return
	}
	final := sh.Lines[e.Line-1]
	before := e.Text
	if e.PartOfLine {
		before = strings.Replace(final, e.Replacement, e.Text, 1)
	}
	wasRight, isRight := tr.right(t, before), tr.right(t, final)
	switch e.Erased {
	case sample.ErasedMistake:
		if wasRight || !isRight {
			t.Errorf("%s, sheet %d: the mistake erased %q is right %v, its replacement %q right %v", sh.Task, sh.Number, before, wasRight, final, isRight)
		}
	case sample.ErasedCorrectStep:
		if !wasRight || isRight || sh.Expected.FirstError == nil || *sh.Expected.FirstError != e.Line {
			t.Errorf("%s, sheet %d: the step erased %q is right %v, its replacement %q right %v, the first error %v", sh.Task, sh.Number, before, wasRight, final, isRight, sh.Expected.FirstError)
		}
	}
}
