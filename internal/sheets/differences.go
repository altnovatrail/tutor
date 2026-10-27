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
// (R25), and the summary names such sheets. On a sheet of a plan of parts the markup's lines are
// its parts (T56): the notes name them parts, and the strokes of the planned erasure stood where
// their place on the plane puts them (ErasurePlace), not by bands down the sheet.
func Differences(sh *sample.PlanSheet, s *sample.Sample) []sample.Problem {
	notes := []sample.Problem{}
	add := func(path, format string, args ...any) {
		notes = append(notes, sample.Problem{Path: path, Message: fmt.Sprintf(format, args...)})
	}
	unit := "line"
	if len(sh.Parts) > 0 {
		unit = "part"
	}
	named := func(n int) string { return unitOrNone(unit, n) }
	if len(s.Lines) != len(sh.Lines) {
		add("lines", "are %d, and the plan sheet's %d", len(s.Lines), len(sh.Lines))
	}
	answer := 0
	for i, l := range s.Lines {
		path := fmt.Sprintf("lines[%d]", i)
		if i < len(sh.Lines) && l.Text != sh.Lines[i] {
			add(path+".text", "is %q, and the plan sheet's %s %d is %q", l.Text, unit, i+1, sh.Lines[i])
		}
		if l.Illegible {
			add(path+".illegible", "is set, and the plan sheet marks no %s illegible", unit)
		}
		if l.CrossedOut {
			add(path+".crossed_out", "is set, and the plan sheet marks no %s crossed out", unit)
		}
		if l.Answer {
			answer = i + 1
		}
	}
	if answer != sh.Answer {
		add("lines", "give the answer on %s, and the plan sheet on %s %d", named(answer), unit, sh.Answer)
	}
	e, plan := s.Expected, sh.Expected
	if e.Outcome != plan.Outcome {
		add("expected.outcome", "is %q, and the plan sheet's %q", e.Outcome, plan.Outcome)
	}
	if first, planned := lineOf(e.FirstError), lineOf(plan.FirstError); first != planned {
		add("expected.first_error", "is on %s, and the plan sheet's on %s", named(first), named(planned))
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
			if from := ErasurePlace(sh, s, er); from != er.Planned.Line {
				add(path+".line", "is %d, but the erasure took its strokes from %s", er.Planned.Line, named(from))
			}
		}
	}
	return notes
}

// ErasurePlace is the number of the line of a sheet's markup that most of an erasure's strokes
// stood on, or 0 for none: by bands down the sheet (lines.ErasureLine). On a sheet of a plan of
// parts it is the number of the part, by the place of the strokes on the plane
// (lines.ErasurePart), as a free-form solution has no lines down the sheet.
func ErasurePlace(sh *sample.PlanSheet, s *sample.Sample, e sample.Erasure) int {
	if len(sh.Parts) > 0 {
		return lines.ErasurePart(s, e)
	}
	return lines.ErasureLine(s, e)
}

// lineOf is the number a markup field names, or 0 for none.
func lineOf(n *int) int {
	if n == nil {
		return 0
	}
	return *n
}

// unitOrNone names a line or a part by its number, or says there is none.
func unitOrNone(unit string, n int) string {
	if n == 0 {
		return "no " + unit
	}
	return unit + " " + strconv.Itoa(n)
}
