package report

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"
	"google.golang.org/protobuf/proto"
)

// Joined is the runner's report of one eval set of a capability that tutor reads in runs of their
// own, each over a part of it: the sheets in a run of the reading and the pages in a run of the
// workbook, as of canvas's synthetic samples (R76). Each part's report of one reader, or one pair,
// gives its samples, and the report joined names the set and holds them all, ascending by ID, as a
// report of one run would (contracts/README.md of spec, section 14). The reports must be of one
// runner, capability, verdicts, readers and repeats, and share no sample.
func Joined(set string, parts ...*evalv1.Report) (*evalv1.Report, error) {
	if len(parts) == 0 {
		return nil, errors.New("report: no report to join")
	}
	first := parts[0]
	out := &evalv1.Report{Protocol: first.GetProtocol(), Runner: first.GetRunner(), Capability: first.GetCapability(), Readers: first.GetReaders(), Assessor: first.GetAssessor(),
		Verdicts: first.GetVerdicts(), EvalSet: &evalv1.Report_CapabilitySet{CapabilitySet: &evalv1.CapabilityEvalSet{Name: set}}, Repeats: first.GetRepeats()}
	seen := map[string]bool{}
	for i, p := range parts {
		if _, ok := p.GetEvalSet().(*evalv1.Report_CapabilitySet); !ok {
			return nil, fmt.Errorf("report: the report %d to join is of no eval set of a capability", i+1)
		}
		if !sameHead(p, first) {
			return nil, fmt.Errorf("report: the report %d to join is of another runner, capability, verdicts, readers or repeats than the first", i+1)
		}
		for _, s := range p.GetSamples() {
			if seen[s.GetSample()] {
				return nil, fmt.Errorf("report: the sample %s is in two of the reports to join", s.GetSample())
			}
			seen[s.GetSample()] = true
			out.Samples = append(out.Samples, s)
		}
	}
	slices.SortFunc(out.Samples, func(a, b *evalv1.SampleResult) int { return strings.Compare(a.GetSample(), b.GetSample()) })
	return out, nil
}

// sameHead reports whether two reports are of one protocol, runner, capability, readers, assessor,
// verdicts and repeats: all a report says but its eval set and its samples.
func sameHead(a, b *evalv1.Report) bool {
	return a.GetProtocol() == b.GetProtocol() && a.GetRunner() == b.GetRunner() && a.GetCapability() == b.GetCapability() && a.GetRepeats() == b.GetRepeats() &&
		slices.EqualFunc(a.GetReaders(), b.GetReaders(), func(x, y *evalv1.Model) bool { return proto.Equal(x, y) }) &&
		proto.Equal(a.GetAssessor(), b.GetAssessor()) && proto.Equal(a.GetVerdicts(), b.GetVerdicts())
}
