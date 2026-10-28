package report

import (
	"fmt"
	"strings"

	"github.com/altnovatrail/tutor/internal/runner"
)

// The head of a report, the same for every kind of call: what its title calls the part of the
// sheets, how it names a configuration of a model, what the calls of its log took, and what it says
// of a log that has no end.

// partOf is what the title of a report calls the part of the sheets its run took: the dev or the
// test part, or all the sheets.
func partOf(part string) string {
	if part == "all" {
		return "all the sheets"
	}
	return fmt.Sprintf("the %s part", part)
}

// configName names a configuration of a model: its name, model and effort, whether it thinks, and
// what else the kind tells of it.
func configName(name, model, effort string, noThinking bool, more ...string) string {
	parts := []string{model, effort}
	if noThinking {
		parts = append(parts, "without thinking")
	}
	return fmt.Sprintf("%s (%s)", name, strings.Join(append(parts, more...), ", "))
}

// tally is what the calls of a log took: how many were made, how many of them failed, and what they
// cost. made tells whether a call was made, failed whether it failed and cost what it cost.
func tally[C any](calls []C, of func(c *C) (made, failed bool, cost float64)) (int, int, float64) {
	n, failed, spent := 0, 0, 0.0
	for i := range calls {
		m, f, c := of(&calls[i])
		if !m {
			continue
		}
		n++
		spent += c
		if f {
			failed++
		}
	}
	return n, failed, spent
}

// setsOf names the sets of a run in its order, each with how many of the report's sheets are of it.
func setsOf(sets []string, order []sheetID) string {
	n := map[string]int{}
	for _, id := range order {
		n[id.set]++
	}
	var names []string
	for _, s := range sets {
		names = append(names, fmt.Sprintf("%s %d", s, n[s]))
	}
	return strings.Join(names, ", ")
}

// endNote is what a report says of a log that has no end, or of a run that stopped short: that the
// report is of what the log holds. Of a run that ended it says nothing.
func endNote(end *runner.End) string {
	switch {
	case end == nil:
		return "\n**The log has no end: the run broke off, and the report is of what it holds.**\n"
	case end.Stopped != "":
		return fmt.Sprintf("\n**The run stopped short, by %s: the report is of what it holds.**\n", end.Stopped)
	}
	return ""
}
