package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strings"

	"github.com/altnovatrail/tutor/internal/paperset"
	"github.com/altnovatrail/tutor/internal/runner"
)

// paperFile is the set of a task from paper that gen paper made, which the runs of its three calls
// read by default (R135).
const paperFile = "eval/paper-gen.json"

// paperConfigs are the configurations a run of a call of a task from paper asks unless -readers
// names others: the models the library admits for it, Haiku at medium for the statement and the
// solution, and the sheet's pair for the work (R135).
var paperConfigs = map[string][]string{
	paperset.CallStatement: {"haiku-medium"},
	paperset.CallSolving:   {"haiku-medium"},
	paperset.CallPaper:     {"opus-high", "haiku-medium"},
}

// planPaper plans a run of a call of a task from paper over the cases of a part of its set, by the
// configurations, repeated (paperset.New).
func planPaper(kind string) func([]sheetFlags, string, options) (*runner.Run, string, error) {
	return func(_ []sheetFlags, part string, o options) (*runner.Run, string, error) {
		data, err := os.ReadFile(o.paper)
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
		f := &paperset.File{Name: o.paper, SHA256: fmt.Sprintf("%x", sha256.Sum256(data)), Set: s}
		r, err := paperset.New(kind, part, f, configs, o.repeats)
		if err != nil {
			return nil, "", err
		}
		return r, fmt.Sprintf("%s: %s of the set %s, drawn with the seed %d, the call %s by %s, %s.\n",
			partName(part), count(len(s.Of(part)), "case"), o.paper, s.Seed, kind, strings.Join(names, " and "), count(o.repeats, "repeat")), nil
	}
}
