package gen

import (
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"

	"github.com/altnovatrail/module-kit/paper"

	"github.com/altnovatrail/tutor/internal/paperset"
)

// paperStream is the stream of the random numbers of the set of paper.
const paperStream = 162

// Paper builds the set of a task from paper from the seed: n cases, the kinds in turn — an
// expression, an equation, a word problem, a choice, an open task and several tasks, 2 to 4
// equations under one instruction —, and the work of each task in turn right, wrong and blank, an
// open task's blank and unchecked, since no key judges a line. The key of a formula is the solver's
// on it, so the set holds no answer a program did not compute; the key of a word problem and of a
// choice is built with it. The first devCount cases go to the development.
func Paper(n int, seed uint64) (*paperset.Set, error) {
	rng := newRand(seed, paperStream)
	s := &paperset.Set{Description: "Tasks from paper made by code: the statement, the formula and key of each task, and the learner's line under it.", Seed: seed}
	for i := range n {
		c, err := paperCase(rng, i)
		if err != nil {
			return nil, err
		}
		c.Number, c.Part = i+1, partOf(i, n)
		s.Cases = append(s.Cases, c)
	}
	return s, nil
}

// paperCase is the i-th case, from 0, before its number and part.
func paperCase(rng *rand.Rand, i int) (paperset.Case, error) {
	kind := paperset.Kinds[i%len(paperset.Kinds)]
	a, b := 10+rng.IntN(90), 10+rng.IntN(90)
	c := paperset.Case{Kind: kind}
	var t paperset.Task
	switch kind {
	case paperset.KindExpression:
		sign, op := "+", "+"
		if rng.IntN(2) == 1 {
			sign, op = "×", "*"
		}
		c.Statement = []string{"Compute.", fmt.Sprintf("%d %s %d =", a, sign, b)}
		t.Formula = fmt.Sprintf("%d%s%d", a, op, b)
	case paperset.KindEquation:
		c.Statement = []string{"Find the missing number.", fmt.Sprintf("□ + %d = %d", a, a+b)}
		t.Formula = fmt.Sprintf("x+%d=%d", a, a+b)
	case paperset.KindWord:
		c.Statement = []string{fmt.Sprintf("Sam has %d stamps.", a), fmt.Sprintf("He gets %d more.", b), "How many stamps has he now?"}
		t.Key = &paperset.Key{Number: strconv.Itoa(a + b), Units: []string{"stamps"}}
	case paperset.KindChoice:
		options := []string{"A", "B", "C", "D"}
		values := rng.Perm(4)
		right := 0
		line := ""
		for j, v := range values {
			line += fmt.Sprintf("(%s) %d  ", options[j], a+v*b)
			if v == 3 {
				right = j
			}
		}
		c.Statement = []string{"Which number is the largest?", line[:len(line)-2]}
		t.Key = &paperset.Key{Options: options, Choice: options[right]}
	case paperset.KindOpen:
		c.Statement = []string{fmt.Sprintf("Draw a line %d cm long.", 2+rng.IntN(8))}
		t.Statement, t.Outcome = strings.Join(c.Statement, " "), paperset.OutcomeUnchecked
		c.Tasks = []paperset.Task{t}
		return c, nil
	case paperset.KindSeveral:
		c.Statement = []string{"Find the missing number."}
		for j := range 2 + rng.IntN(3) {
			p, q := 1+rng.IntN(9), 10+rng.IntN(10)
			t := paperset.Task{Statement: fmt.Sprintf("□ + %d = %d", p, q), Formula: fmt.Sprintf("x+%d=%d", p, q)}
			if err := worked(&t, kind, i/len(paperset.Kinds)+j); err != nil {
				return c, err
			}
			c.Tasks = append(c.Tasks, t)
		}
		return c, nil
	}
	t.Statement = strings.Join(c.Statement, " ")
	if err := worked(&t, kind, i/len(paperset.Kinds)); err != nil {
		return c, err
	}
	c.Tasks = []paperset.Task{t}
	return c, nil
}

// worked gives a task with a key, or a formula the solver computes it from, its work: the turn-th
// in turn right, wrong and blank.
func worked(t *paperset.Task, kind string, turn int) error {
	if t.Formula != "" {
		n, reason := paper.Solve(t.Formula)
		if n == nil {
			return fmt.Errorf("gen: the solver refuses the formula %s it made: %s", t.Formula, reason)
		}
		t.Key = &paperset.Key{Number: n.GetValue(), Unknown: n.GetUnknown()}
	}
	right := t.Key.Number
	wrong := ""
	if t.Key.Choice != "" {
		right = t.Key.Choice
		for _, o := range t.Key.Options {
			if o != right {
				wrong = o
				break
			}
		}
	} else {
		v, _ := strconv.Atoi(right)
		wrong = strconv.Itoa(v + 1)
	}
	switch turn % 3 {
	case 0:
		t.Work, t.Read, t.Outcome = written(t, kind, right), right, paperset.OutcomeCorrect
	case 1:
		t.Work, t.Read, t.Outcome = written(t, kind, wrong), wrong, paperset.OutcomeNotYet
	default:
		t.Outcome = paperset.OutcomeUnreadable
	}
	return nil
}

// written is the line a learner writes with a value: x = 7 under an equation, 12 stamps under a word
// problem, and the value alone otherwise.
func written(t *paperset.Task, kind string, value string) string {
	switch kind {
	case paperset.KindEquation, paperset.KindSeveral:
		return t.Key.Unknown + " = " + value
	case paperset.KindWord:
		return value + " stamps"
	}
	return value
}
