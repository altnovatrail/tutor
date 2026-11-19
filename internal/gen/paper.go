package gen

import (
	"fmt"
	"math/rand/v2"
	"strconv"

	"github.com/altnovatrail/module-kit/paper"

	"github.com/altnovatrail/tutor/internal/paperset"
)

// paperStream is the stream of the random numbers of the set of paper.
const paperStream = 162

// Paper builds the set of a task from paper (R135) from the seed: n cases, the kinds in turn —
// an expression, an equation, a word problem, a choice and an open task —, and the work of each
// in turn right, wrong and blank, an open task's blank and unchecked, since no line answers it. The key of a formula
// is the solver's on it, so the set holds no answer a program did not compute; the key of a word
// problem and of a choice is built with it. The first devCount cases go to the development.
func Paper(n int, seed uint64) (*paperset.Set, error) {
	rng := newRand(seed, paperStream)
	s := &paperset.Set{Description: "Tasks from paper made by code (R135): the statement, its formula and key, and the learner's line under it.", Seed: seed}
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
	var c paperset.Case
	c.Kind = kind
	switch kind {
	case paperset.KindExpression:
		sign, op := "+", "+"
		if rng.IntN(2) == 1 {
			sign, op = "×", "*"
		}
		c.Statement = []string{"Compute.", fmt.Sprintf("%d %s %d =", a, sign, b)}
		c.Formula = fmt.Sprintf("%d%s%d", a, op, b)
	case paperset.KindEquation:
		c.Statement = []string{"Find the missing number.", fmt.Sprintf("□ + %d = %d", a, a+b)}
		c.Formula = fmt.Sprintf("x+%d=%d", a, a+b)
	case paperset.KindWord:
		c.Statement = []string{fmt.Sprintf("Sam has %d stamps.", a), fmt.Sprintf("He gets %d more.", b), "How many stamps has he now?"}
		c.Key = &paperset.Key{Number: strconv.Itoa(a + b), Units: []string{"stamps"}}
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
		c.Key = &paperset.Key{Options: options, Choice: options[right]}
	case paperset.KindOpen:
		c.Statement = []string{fmt.Sprintf("Draw a line %d cm long.", 2+rng.IntN(8))}
		c.Outcome = paperset.OutcomeUnchecked
		return c, nil
	}
	if c.Formula != "" {
		n, reason := paper.Solve(c.Formula)
		if n == nil {
			return c, fmt.Errorf("gen: the solver refuses the formula %s it made: %s", c.Formula, reason)
		}
		c.Key = &paperset.Key{Number: n.GetValue(), Unknown: n.GetUnknown()}
	}
	right := c.Key.Number
	wrong := ""
	if c.Key.Choice != "" {
		right = c.Key.Choice
		for _, o := range c.Key.Options {
			if o != right {
				wrong = o
				break
			}
		}
	} else {
		v, _ := strconv.Atoi(right)
		wrong = strconv.Itoa(v + 1)
	}
	switch (i / len(paperset.Kinds)) % 3 {
	case 0:
		c.Work, c.Read, c.Outcome = written(c, right), right, paperset.OutcomeCorrect
	case 1:
		c.Work, c.Read, c.Outcome = written(c, wrong), wrong, paperset.OutcomeNotYet
	default:
		c.Outcome = paperset.OutcomeUnreadable
	}
	return c, nil
}

// written is the line a learner writes with a value: x = 7 under an equation, 12 stamps under a word
// problem, and the value alone otherwise.
func written(c paperset.Case, value string) string {
	switch c.Kind {
	case paperset.KindEquation:
		return c.Key.Unknown + " = " + value
	case paperset.KindWord:
		return value + " stamps"
	}
	return value
}
