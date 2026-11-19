package gen

import (
	"reflect"
	"testing"

	"github.com/altnovatrail/module-kit/paper"

	"github.com/altnovatrail/tutor/internal/paperset"
)

// The set of paper is the same of the same seed, takes the kinds in turn and the work of each in
// turn right, wrong and blank, and the key of a formula is the solver's.
func TestPaper(t *testing.T) {
	s, err := Paper(30, 5)
	if err != nil {
		t.Fatal(err)
	}
	again, _ := Paper(30, 5)
	if !reflect.DeepEqual(s, again) || len(s.Cases) != 30 || s.Seed != 5 {
		t.Fatal("the set of a seed is not the same twice")
	}
	outcomes := map[string]int{}
	for i, c := range s.Cases {
		if c.Kind != paperset.Kinds[i%5] || c.Number != i+1 {
			t.Errorf("case %d is %s", i+1, c.Kind)
		}
		outcomes[c.Outcome]++
		if c.Formula != "" {
			n, _ := paper.Solve(c.Formula)
			if n.GetValue() != c.Key.Number {
				t.Errorf("case %d: the key %s, and the solver gives %v", i+1, c.Key.Number, n)
			}
		}
	}
	if outcomes[paperset.OutcomeCorrect] != 8 || outcomes[paperset.OutcomeNotYet] != 8 || outcomes[paperset.OutcomeUnreadable] != 8 || outcomes[paperset.OutcomeUnchecked] != 6 {
		t.Errorf("the outcomes are %v", outcomes)
	}
	if s.Cases[1].Work != "x = "+s.Cases[1].Key.Number || s.Cases[2].Work != s.Cases[2].Key.Number+" stamps" {
		t.Errorf("the work is %q and %q", s.Cases[1].Work, s.Cases[2].Work)
	}
}
