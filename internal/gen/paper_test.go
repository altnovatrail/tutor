package gen

import (
	"reflect"
	"testing"

	"github.com/altnovatrail/module-kit/paper"

	"github.com/altnovatrail/tutor/internal/paperset"
)

// The set of paper is the same of the same seed, takes the kinds in turn and the work of each task
// in turn right, wrong and blank, and the key of a formula is the solver's; several tasks are 2 to 4
// equations, each with its key and its work in turn from the case's.
func TestPaper(t *testing.T) {
	s, err := Paper(30, 5)
	if err != nil {
		t.Fatal(err)
	}
	again, _ := Paper(30, 5)
	if !reflect.DeepEqual(s, again) || len(s.Cases) != 30 || s.Seed != 5 {
		t.Fatal("the set of a seed is not the same twice")
	}
	inTurn := []string{paperset.OutcomeCorrect, paperset.OutcomeNotYet, paperset.OutcomeUnreadable}
	outcomes := map[string]int{}
	for i, c := range s.Cases {
		if c.Kind != paperset.Kinds[i%len(paperset.Kinds)] || c.Number != i+1 {
			t.Errorf("case %d is %s", i+1, c.Kind)
		}
		several := c.Kind == paperset.KindSeveral
		if n := len(c.Tasks); several && (n < 2 || n > 4) || !several && n != 1 {
			t.Errorf("case %d has %d tasks", i+1, n)
		}
		for j, task := range c.Tasks {
			if several {
				if task.Formula == "" || task.Key == nil || task.Outcome != inTurn[(i/len(paperset.Kinds)+j)%3] {
					t.Errorf("case %d, task %d: %+v", i+1, j+1, task)
				}
			} else {
				outcomes[task.Outcome]++
			}
			if task.Formula != "" {
				n, _ := paper.Solve(task.Formula)
				if n.GetValue() != task.Key.Number {
					t.Errorf("case %d, task %d: the key %s, and the solver gives %v", i+1, j+1, task.Key.Number, n)
				}
			}
		}
	}
	if outcomes[paperset.OutcomeCorrect] != 8 || outcomes[paperset.OutcomeNotYet] != 8 || outcomes[paperset.OutcomeUnreadable] != 4 || outcomes[paperset.OutcomeUnchecked] != 5 {
		t.Errorf("the outcomes of one task are %v", outcomes)
	}
	if eq, word := s.Cases[1].Tasks[0], s.Cases[2].Tasks[0]; eq.Work != "x = "+eq.Key.Number || word.Work != word.Key.Number+" stamps" {
		t.Errorf("the work is %q and %q", eq.Work, word.Work)
	}
	if several := s.Cases[5].Tasks[0]; several.Work != "x = "+several.Key.Number || several.Statement == "" {
		t.Errorf("the work of several tasks is %+v", several)
	}
}
