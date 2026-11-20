package main

import (
	"fmt"
	"strings"

	"github.com/altnovatrail/tutor/internal/paperset"
	"github.com/altnovatrail/tutor/internal/runner"
)

// paperFile is the set of a task from paper that gen paper made, which the runs of its four calls
// read by default (R135).
const paperFile = "eval/paper-gen.json"

// paperConfigs are the configurations a run of a call of a task from paper asks unless -readers
// names others: the models the library admits for it, the three attempts of its cascade, each
// measured alone (R137).
var paperConfigs = map[string][]string{
	paperset.CallStatement: cascadeConfigs,
	paperset.CallSolving:   cascadeConfigs,
	paperset.CallPaper:     cascadeConfigs,
	paperset.CallFinding:   cascadeConfigs,
}

// cascadeConfigs are the attempts of the cascade of a task from paper, in their order.
var cascadeConfigs = []string{"haiku-high", "sonnet-medium", "opus-medium"}

// planPaper plans a run of a call of a task from paper over the cases of a part of its set, by the
// configurations, repeated (paperset.New).
func planPaper(kind string) func([]sheetFlags, string, options) (*runner.Run, string, error) {
	return func(_ []sheetFlags, part string, o options) (*runner.Run, string, error) {
		digest, err := paperset.Digest(o.paper)
		if err != nil {
			return nil, "", err
		}
		s, err := paperset.Read(o.paper)
		if err != nil {
			return nil, "", err
		}
		names := paperConfigs[kind]
		if o.readersGiven {
			names = o.readers
		}
		configs, err := runner.Chosen(runner.Configs(), func(c runner.Config) runner.Config { return c }, names)
		if err != nil {
			return nil, "", err
		}
		f := &paperset.File{Name: o.paper, SHA256: digest, Set: s}
		r, err := paperset.New(kind, part, f, configs, o.repeats)
		if err != nil {
			return nil, "", err
		}
		return r, fmt.Sprintf("%s: %s of the set %s, drawn with the seed %d, the call %s by %s, %s.\n",
			partName(part), count(len(r.Items), "case"), o.paper, s.Seed, kind, strings.Join(names, " and "), count(o.repeats, "repeat")), nil
	}
}
