package sheets

import (
	"fmt"
	"strconv"

	"github.com/altnovatrail/tutor/internal/lines"
	"github.com/altnovatrail/tutor/internal/sample"
)

// Differences names how the markup of a sheet differs from its plan sheet: in its lines, its
// answer, its outcome, its first error, its trap and its planned erasure, each at the path of the
// sample's field. The board saves such a sheet all the same, as what was written the author knows
// (R25), and the summary names such sheets.
func Differences(sh *sample.PlanSheet, s *sample.Sample) []sample.Problem {
	notes := []sample.Problem{}
	add := func(path, format string, args ...any) {
		notes = append(notes, sample.Problem{Path: path, Message: fmt.Sprintf(format, args...)})
	}
	if len(s.Lines) != len(sh.Lines) {
		add("lines", "are %d, and the plan sheet's %d", len(s.Lines), len(sh.Lines))
	}
	answer := 0
	for i, l := range s.Lines {
		path := fmt.Sprintf("lines[%d]", i)
		if i < len(sh.Lines) && l.Text != sh.Lines[i] {
			add(path+".text", "is %q, and the plan sheet's line %d is %q", l.Text, i+1, sh.Lines[i])
		}
		if l.Illegible {
			add(path+".illegible", "is set, and the plan sheet marks no line illegible")
		}
		if l.CrossedOut {
			add(path+".crossed_out", "is set, and the plan sheet marks no line crossed out")
		}
		if l.Answer {
			answer = i + 1
		}
	}
	if answer != sh.Answer {
		add("lines", "give the answer on %s, and the plan sheet on line %d", lineOrNone(answer), sh.Answer)
	}
	e, plan := s.Expected, sh.Expected
	if e.Outcome != plan.Outcome {
		add("expected.outcome", "is %q, and the plan sheet's %q", e.Outcome, plan.Outcome)
	}
	if first, planned := lineOf(e.FirstError), lineOf(plan.FirstError); first != planned {
		add("expected.first_error", "is on %s, and the plan sheet's on %s", lineOrNone(first), lineOrNone(planned))
	}
	if e.Trap != plan.Trap {
		add("expected.trap", "is %q, and the plan sheet's %q", e.Trap, plan.Trap)
	}
	if pe := sh.Erasure; pe != nil {
		for i, er := range s.Erasures {
			if er.Planned == nil {
				continue
			}
			path := fmt.Sprintf("erasures[%d].planned", i)
			// What the planned erasure took away, the scenario sets, so it cannot differ from the plan's.
			if er.Planned.Line != pe.Line {
				add(path+".line", "is %d, and the plan sheet's %d", er.Planned.Line, pe.Line)
			}
			if er.Planned.Text != pe.Text {
				add(path+".text", "is %q, and the plan sheet's %q", er.Planned.Text, pe.Text)
			}
			if from := lines.ErasureLine(s, er); from != er.Planned.Line {
				add(path+".line", "is %d, but the erasure took its strokes from %s", er.Planned.Line, lineOrNone(from))
			}
		}
	}
	return notes
}

// lineOf is the number a markup field names, or 0 for none.
func lineOf(n *int) int {
	if n == nil {
		return 0
	}
	return *n
}

// lineOrNone names a line by its number, or says there is none.
func lineOrNone(n int) string {
	if n == 0 {
		return "no line"
	}
	return "line " + strconv.Itoa(n)
}
