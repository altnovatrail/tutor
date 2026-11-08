package report

import (
	"cmp"
	"context"
	"path/filepath"
	"slices"
	"strings"

	"github.com/altnovatrail/module-kit/observation"
	"github.com/altnovatrail/module-kit/verdict"
	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"
	observationv1 "github.com/altnovatrail/spec/contracts/novatrail/observation/v1"
	"github.com/altnovatrail/spec/types/formatv0"

	"github.com/altnovatrail/tutor/admission"
	"github.com/altnovatrail/tutor/internal/essay"
	"github.com/altnovatrail/tutor/internal/evalsample"
	"github.com/altnovatrail/tutor/internal/protocol"
	"github.com/altnovatrail/tutor/internal/reading"
	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/runner"
	"github.com/altnovatrail/tutor/internal/workbook"
)

// The runner's report of the eval protocol (contracts/README.md of spec, section 14; R95 spec;
// R74): the message Report of novatrail.eval.v1, made of a run's log. A log tells a report for each
// reader alone, with all its repeats, and for each pair of the readers — the one that reads once,
// Opus, with each that reads in repeats —, joined by module-kit (observation.Join), repeat by
// repeat. Of the calls, it takes what the platform asks: a sheet without a key, a page of the
// workbook bare, the work as it is finished, with its digits at 32 px. A log whose samples are not
// of the protocol, a log of a run before T106 or of the author's sheets of tutor-sample v0, tells
// none.

// Named is a runner's report with the name its file takes beside the log: the reader's, or the
// pair's, strong+fast.
type Named struct {
	Name   string
	Report *evalv1.Report
}

// modelOf is a reader as the report names it: its provider, by its code in the admission table of
// spec, anthropic, since every configuration of the runner is a model of Claude (runner.Configs),
// and its model with its setting — the model's ID, its effort and, without thinking, no-thinking —,
// as tutor's list of admitted models will name it.
func modelOf(rd runner.ReaderRecord) *evalv1.Model {
	m := admission.Model{Provider: "anthropic", ID: rd.Model, Effort: rd.Effort, NoThinking: rd.NoThinking}
	return &evalv1.Model{Provider: string(m.Provider), Model: m.Name()}
}

// head is a report of a capability, by the runner's versions as the run's record names them, of the
// capability's set of the name, read in the repeats.
func head(c evalv1.Capability, tutor, moduleKit, set string, repeats int, readers ...*evalv1.Model) *evalv1.Report {
	return &evalv1.Report{
		Protocol:   protocol.Protocol,
		Runner:     cmp.Or(tutor, runner.NoRelease),
		Capability: c,
		Readers:    readers,
		Verdicts:   &evalv1.VerdictSource{Code: evalv1.VerdictCode_VERDICT_CODE_MODULE_KIT, Version: cmp.Or(moduleKit, runner.NoRelease)},
		EvalSet:    &evalv1.Report_CapabilitySet{CapabilitySet: &evalv1.CapabilityEvalSet{Name: set}},
		Repeats:    int32(repeats),
	}
}

// withRefused gives a report the samples measured and, of those its run refused, the ones that
// name an ID, each with its reason, all ascending by ID.
func withRefused(r *evalv1.Report, measured []*evalv1.SampleResult, refused []runner.Refused) *evalv1.Report {
	out := slices.Clone(measured)
	for _, x := range refused {
		if x.Sample == "" {
			continue
		}
		out = append(out, &evalv1.SampleResult{Sample: x.Sample, Result: &evalv1.SampleResult_Refused{Refused: evalv1.Refusal(evalv1.Refusal_value[x.Reason])}})
	}
	slices.SortFunc(out, func(a, b *evalv1.SampleResult) int { return strings.Compare(a.GetSample(), b.GetSample()) })
	r.Samples = out
	return r
}

// verdictOf is a verdict as the report writes it: the verdict, its reason beside unreadable alone
// and its trap beside not_yet alone.
func verdictOf(v verdict.SlotVerdict) *evalv1.Verdict {
	out := &evalv1.Verdict{Verdict: string(v.Verdict)}
	switch v.Verdict {
	case verdict.Unreadable:
		out.Reason = string(v.Reason)
	case verdict.NotYet:
		if v.Trap != nil {
			out.Trap = string(v.Trap.Trap)
		}
	}
	return out
}

// truthOf is the truth of an answer slot as the log writes it, as a verdict of the report.
func truthOf(t evalsample.Truth) *evalv1.Verdict {
	v := verdict.SlotVerdict{Verdict: verdict.Verdict(t.Verdict), Reason: verdict.Reason(t.Reason)}
	if t.Trap != "" {
		v.Trap = &formatv0.TrapValue{Trap: formatv0.TrapID(t.Trap)}
	}
	return verdictOf(v)
}

// unchecked is a reading that gave no verdict, with its cause.
func unchecked(cause evalv1.UncheckedCause) *evalv1.Reading {
	return &evalv1.Reading{Verdict: &evalv1.Verdict{Verdict: string(verdict.Unchecked)}, Unchecked: cause}
}

// failure is the cause a call gives no verdict for, or none: not asked or failed is a failure of
// AI, and an answer off the schema no valid observation. The provider tells no limit or quota apart
// from another failure — a workspace's limit of use is a 400 invalid_request_error —, so a failed
// call is a failure; the limit is the run's own cap (notReached).
func failure(notAsked bool, err string, invalid bool) evalv1.UncheckedCause {
	switch {
	case notAsked:
		return evalv1.UncheckedCause_UNCHECKED_CAUSE_FAILURE
	case err != "" && invalid:
		return evalv1.UncheckedCause_UNCHECKED_CAUSE_INVALID
	case err != "":
		return evalv1.UncheckedCause_UNCHECKED_CAUSE_FAILURE
	}
	return evalv1.UncheckedCause_UNCHECKED_CAUSE_UNSPECIFIED
}

// readingOf is the reading of a verdict: unchecked with no cause the runner knows — a check that
// module-kit refuses, which no bank gives — is a failure.
func readingOf(v verdict.SlotVerdict) *evalv1.Reading {
	if v.Verdict == verdict.Unchecked {
		return unchecked(evalv1.UncheckedCause_UNCHECKED_CAUSE_FAILURE)
	}
	return &evalv1.Reading{Verdict: verdictOf(v)}
}

// notReached is the cause of a repeat that a run did not reach: the limit when its cap stopped it,
// and a failure when it stopped otherwise — interrupted, or a log it could not write, or no end.
func notReached(end *runner.End) evalv1.UncheckedCause {
	if end != nil && strings.HasPrefix(end.Stopped, runner.StoppedByCap) {
		return evalv1.UncheckedCause_UNCHECKED_CAUSE_LIMIT
	}
	return evalv1.UncheckedCause_UNCHECKED_CAUSE_FAILURE
}

// pairsOf names the pairs of a run's readers: the one that reads once with each that reads in
// repeats (R08), as the reading's report pairs them too.
func pairsOf(readers []runner.ReaderRecord) [][2]runner.ReaderRecord {
	var out [][2]runner.ReaderRecord
	for _, strong := range readers {
		if !strong.Once {
			continue
		}
		for _, fast := range readers {
			if !fast.Once {
				out = append(out, [2]runner.ReaderRecord{strong, fast})
			}
		}
	}
	return out
}

// repeatsOf is how many repeats a reader reads in a run of the repeats: one when it reads once, but
// in a run of admission, plain, where every reader reads in every repeat (reading.Plain).
func repeatsOf(rd runner.ReaderRecord, repeats int, plain bool) int {
	if rd.Once && !plain {
		return 1
	}
	return repeats
}

// setName is the name of a run's set of a capability, by the names of what it read and its part
// (R74): lines+free+near-dev, workbook-test, essays-all.
func setName(names []string, part string) string { return strings.Join(names, "+") + "-" + part }

// fileName is a file's name without its folder and its .json.
func fileName(file string) string { return strings.TrimSuffix(filepath.Base(file), ".json") }

// ctx is the context of a verdict: a check of number or fraction runs no predicate that it could
// stop.
var ctx = context.Background()

// ReadingReports are the reports of a run of the reading of sheets, CAPABILITY_HANDWRITING_READING:
// each sheet's one answer slot, with its truth, and a reading for each repeat of the finished
// sheet without a key at 32 px.
func ReadingReports(l *reading.Log) []Named {
	type key struct {
		set           string
		sheet, repeat int
		reader        string
	}
	calls := map[key]*reading.CallRecord{}
	asked := map[string]bool{}
	for i := range l.Calls {
		c := &l.Calls[i]
		if c.Condition == reading.NoKey && c.Digit == render.Digit && c.State == 0 {
			calls[key{c.Set, c.Sheet, c.Repeat, c.Reader}] = c
			asked[c.Reader] = true
		}
	}
	var sheets []*reading.SheetRecord
	for i := range l.Sheets {
		// A sheet that shows no stroke has no answer slot a model reads: it is measured with none,
		// as a page whose boxes are all empty is.
		if l.Sheets[i].Sample != "" && len(l.Sheets[i].Truth) <= 1 {
			sheets = append(sheets, &l.Sheets[i])
		}
	}
	if len(sheets) == 0 {
		return nil
	}
	set := setName(l.Run.Sets, l.Run.Part)
	report := func(name string, repeats int, read func(sh *reading.SheetRecord, rep int) *evalv1.Reading, readers ...*evalv1.Model) Named {
		var measured []*evalv1.SampleResult
		for _, sh := range sheets {
			var slots []*evalv1.AnswerSlotResult
			for _, t := range sh.Truth {
				slot := &evalv1.AnswerSlotResult{AnswerSlot: &observationv1.AnswerSlotPlace{Item: int32(t.Item), Slot: int32(t.Slot)}, Truth: truthOf(t), Mirrored: t.Mirrored, CrossedOut: t.CrossedOut}
				for rep := 1; rep <= repeats; rep++ {
					slot.Readings = append(slot.Readings, read(sh, rep))
				}
				slots = append(slots, slot)
			}
			measured = append(measured, &evalv1.SampleResult{Sample: sh.Sample, Result: &evalv1.SampleResult_Measured{Measured: &evalv1.Measurement{AnswerSlots: slots}}})
		}
		r := head(evalv1.Capability_CAPABILITY_HANDWRITING_READING, l.Run.Runner, l.Run.ModuleKit, set, repeats, readers...)
		return Named{Name: name, Report: withRefused(r, measured, l.Run.Refused)}
	}
	alone := func(c *reading.CallRecord, k string) *evalv1.Reading {
		if c == nil {
			return unchecked(notReached(l.End))
		}
		if cause := failure(c.NotAsked, c.Error, c.Invalid); cause != evalv1.UncheckedCause_UNCHECKED_CAUSE_UNSPECIFIED {
			return unchecked(cause)
		}
		v, err := reading.SlotVerdict(k, reading.RecordObservation(c))
		if err != nil {
			return unchecked(evalv1.UncheckedCause_UNCHECKED_CAUSE_FAILURE)
		}
		return readingOf(v)
	}
	plain := slices.Contains(l.Run.Reads, reading.Plain)
	var out []Named
	for _, rd := range l.Run.Readers {
		if !asked[rd.Name] {
			continue
		}
		out = append(out, report(rd.Name, repeatsOf(rd, l.Run.Repeats, plain), func(sh *reading.SheetRecord, rep int) *evalv1.Reading {
			return alone(calls[key{sh.Set, sh.Sheet, rep, rd.Name}], sh.Key)
		}, modelOf(rd)))
	}
	for _, p := range pairsOf(l.Run.Readers) {
		if !asked[p[0].Name] || !asked[p[1].Name] {
			continue
		}
		repeats := min(repeatsOf(p[0], l.Run.Repeats, plain), repeatsOf(p[1], l.Run.Repeats, plain))
		out = append(out, report(p[0].Name+"+"+p[1].Name, repeats, func(sh *reading.SheetRecord, rep int) *evalv1.Reading {
			a, b := calls[key{sh.Set, sh.Sheet, rep, p[0].Name}], calls[key{sh.Set, sh.Sheet, rep, p[1].Name}]
			for _, c := range []*reading.CallRecord{a, b} {
				if r := alone(c, sh.Key); r.GetUnchecked() != evalv1.UncheckedCause_UNCHECKED_CAUSE_UNSPECIFIED {
					return r
				}
			}
			v, _ := joinedSlot(sh.Key, a, b)
			return readingOf(v)
		}, modelOf(p[0]), modelOf(p[1])))
	}
	return out
}

// WorkbookReports are the reports of a run of the reading of the workbook,
// CAPABILITY_HANDWRITING_READING: each page's filled boxes, which a model reads, with their truths,
// and a reading of each for each repeat of the page bare at 32 px.
func WorkbookReports(l *workbook.Log) []Named {
	type key struct {
		page, repeat int
		reader       string
	}
	calls := map[key]*workbook.CallRecord{}
	asked := map[string]bool{}
	for i := range l.Calls {
		c := &l.Calls[i]
		if c.Condition == workbook.Bare && c.Digit == render.Digit {
			calls[key{c.Page, c.Repeat, c.Reader}] = c
			asked[c.Reader] = true
		}
	}
	var pages []*workbook.PageRecord
	for i := range l.Sheets {
		if l.Sheets[i].Sample != "" {
			pages = append(pages, &l.Sheets[i])
		}
	}
	if len(pages) == 0 {
		return nil
	}
	set := setName([]string{cmp.Or(l.Run.Set, fileName(l.Run.File))}, l.Run.Part)
	// verdicts are module-kit's verdicts on an observation of a page, nil for none, by the slots of
	// its items, each answer slot by its place.
	verdicts := func(pr *workbook.PageRecord, obs *observationv1.Observation) map[[2]int]verdict.SlotVerdict {
		items, err := observation.Verdicts(ctx, workbook.ItemsOf(pr), obs, nil)
		if err != nil {
			return nil
		}
		out := map[[2]int]verdict.SlotVerdict{}
		for i, it := range items {
			for k, v := range it.Slots {
				out[[2]int{i + 1, k + 1}] = v
			}
		}
		return out
	}
	// cause is why a call of a page gives no verdict, or none: a cell missing or twice is no valid
	// observation of the page.
	cause := func(c *workbook.CallRecord) evalv1.UncheckedCause {
		if cs := failure(c.NotAsked, c.Error, c.Invalid); cs != evalv1.UncheckedCause_UNCHECKED_CAUSE_UNSPECIFIED {
			return cs
		}
		if slices.ContainsFunc(c.Boxes, func(b workbook.Box) bool { return b.Reason == workbook.Missing || b.Reason == workbook.Twice }) {
			return evalv1.UncheckedCause_UNCHECKED_CAUSE_INVALID
		}
		return evalv1.UncheckedCause_UNCHECKED_CAUSE_UNSPECIFIED
	}
	report := func(name string, repeats int, read func(pr *workbook.PageRecord, rep int) func(evalsample.Truth) *evalv1.Reading, readers ...*evalv1.Model) Named {
		var measured []*evalv1.SampleResult
		for _, pr := range pages {
			var slots []*evalv1.AnswerSlotResult
			for _, t := range pr.Truth {
				slots = append(slots, &evalv1.AnswerSlotResult{AnswerSlot: &observationv1.AnswerSlotPlace{Item: int32(t.Item), Slot: int32(t.Slot)}, Truth: truthOf(t), Mirrored: t.Mirrored, CrossedOut: t.CrossedOut})
			}
			for rep := 1; rep <= repeats; rep++ {
				of := read(pr, rep)
				for i, t := range pr.Truth {
					slots[i].Readings = append(slots[i].Readings, of(t))
				}
			}
			measured = append(measured, &evalv1.SampleResult{Sample: pr.Sample, Result: &evalv1.SampleResult_Measured{Measured: &evalv1.Measurement{AnswerSlots: slots}}})
		}
		r := head(evalv1.Capability_CAPABILITY_HANDWRITING_READING, l.Run.Runner, l.Run.ModuleKit, set, repeats, readers...)
		return Named{Name: name, Report: withRefused(r, measured, l.Run.Refused)}
	}
	// readingsOf are the readings of the boxes of a page by the observation of its readers, or of
	// none, with the cause.
	readingsOf := func(pr *workbook.PageRecord, cs evalv1.UncheckedCause, obs *observationv1.Observation) func(evalsample.Truth) *evalv1.Reading {
		if cs != evalv1.UncheckedCause_UNCHECKED_CAUSE_UNSPECIFIED {
			return func(evalsample.Truth) *evalv1.Reading { return unchecked(cs) }
		}
		vs := verdicts(pr, obs)
		if vs == nil {
			// A check that module-kit refuses, which no file of the workbook gives, gives none.
			return func(evalsample.Truth) *evalv1.Reading {
				return unchecked(evalv1.UncheckedCause_UNCHECKED_CAUSE_FAILURE)
			}
		}
		return func(t evalsample.Truth) *evalv1.Reading { return readingOf(vs[[2]int{t.Item, t.Slot}]) }
	}
	none := func(cs evalv1.UncheckedCause) func(evalsample.Truth) *evalv1.Reading {
		return func(evalsample.Truth) *evalv1.Reading { return unchecked(cs) }
	}
	var out []Named
	for _, rd := range l.Run.Readers {
		if !asked[rd.Name] {
			continue
		}
		out = append(out, report(rd.Name, repeatsOf(rd, l.Run.Repeats, false), func(pr *workbook.PageRecord, rep int) func(evalsample.Truth) *evalv1.Reading {
			c := calls[key{pr.Page, rep, rd.Name}]
			if c == nil {
				return none(notReached(l.End))
			}
			return readingsOf(pr, cause(c), workbook.RecordObservation(pr, c))
		}, modelOf(rd)))
	}
	for _, p := range pairsOf(l.Run.Readers) {
		if !asked[p[0].Name] || !asked[p[1].Name] {
			continue
		}
		repeats := min(repeatsOf(p[0], l.Run.Repeats, false), repeatsOf(p[1], l.Run.Repeats, false))
		out = append(out, report(p[0].Name+"+"+p[1].Name, repeats, func(pr *workbook.PageRecord, rep int) func(evalsample.Truth) *evalv1.Reading {
			a, b := calls[key{pr.Page, rep, p[0].Name}], calls[key{pr.Page, rep, p[1].Name}]
			for _, c := range []*workbook.CallRecord{a, b} {
				if c == nil {
					return none(notReached(l.End))
				}
				if cs := cause(c); cs != evalv1.UncheckedCause_UNCHECKED_CAUSE_UNSPECIFIED {
					return none(cs)
				}
			}
			// A box judged with no text, or a check that module-kit refuses, joins nothing: as either
			// reader alone, the page is unchecked, a failure.
			j, err := observation.Join(workbook.ItemsOf(pr), workbook.RecordObservation(pr, a), workbook.RecordObservation(pr, b))
			if err != nil {
				return none(evalv1.UncheckedCause_UNCHECKED_CAUSE_FAILURE)
			}
			return readingsOf(pr, evalv1.UncheckedCause_UNCHECKED_CAUSE_UNSPECIFIED, j)
		}, modelOf(p[0]), modelOf(p[1])))
	}
	return out
}

// EssayReports are the reports of a run of the assessment of essays, CAPABILITY_RUBRIC_ASSESSMENT,
// one for each reader, which reads and assesses the essay, in one call or two: each essay with the
// people's marks of its markup, and a reading for each repeat.
func EssayReports(l *essay.Log) []Named {
	type key struct {
		essay, repeat int
		reader        string
	}
	calls := map[key]*essay.CallRecord{}
	read := map[key]*essay.CallRecord{}
	asked := map[string]bool{}
	for i := range l.Calls {
		c := &l.Calls[i]
		k := key{c.Essay, c.Repeat, c.Reader}
		if c.Step == essay.StepRead {
			read[k] = c
			continue
		}
		calls[k] = c
		asked[c.Reader] = true
	}
	var essays []*essay.EssayRecord
	for i := range l.Sheets {
		if l.Sheets[i].Sample != "" {
			essays = append(essays, &l.Sheets[i])
		}
	}
	if len(essays) == 0 {
		return nil
	}
	set := setName([]string{fileName(l.Run.File)}, l.Run.Part)
	// readingOf is what a repeat gave an essay: its assessment, with AI's mark of each criterion the
	// people marked, or unchecked, with the cause.
	readingOf := func(e *essay.EssayRecord, c, r *essay.CallRecord) *evalv1.EssayReading {
		uncheckedBy := func(cs evalv1.UncheckedCause) *evalv1.EssayReading {
			return &evalv1.EssayReading{Outcome: &evalv1.EssayReading_Unchecked{Unchecked: cs}}
		}
		if c == nil {
			return uncheckedBy(notReached(l.End))
		}
		if r != nil && c.NotAsked {
			// In two calls, an assessment not asked follows a reading that gave no text.
			if cs := failure(r.NotAsked, r.Error, r.Invalid); cs != evalv1.UncheckedCause_UNCHECKED_CAUSE_UNSPECIFIED {
				return uncheckedBy(cs)
			}
		}
		if cs := failure(c.NotAsked, c.Error, c.Invalid); cs != evalv1.UncheckedCause_UNCHECKED_CAUSE_UNSPECIFIED {
			return uncheckedBy(cs)
		}
		marks := map[string]*bool{}
		for _, m := range c.Marks {
			marks[m.ID] = m.Met
		}
		if !slices.ContainsFunc(c.Marks, func(m essay.Mark) bool { return m.Met != nil }) {
			return uncheckedBy(evalv1.UncheckedCause_UNCHECKED_CAUSE_NO_MARK)
		}
		assessed := &evalv1.AssessedEssay{}
		for _, p := range e.People {
			a := evalv1.CriterionAssessment_CRITERION_ASSESSMENT_NONE
			if p.Criterion >= 1 && p.Criterion <= len(l.Run.Rubric) {
				switch met := marks[l.Run.Rubric[p.Criterion-1].ID]; {
				case met == nil:
				case *met:
					a = evalv1.CriterionAssessment_CRITERION_ASSESSMENT_MET
				default:
					a = evalv1.CriterionAssessment_CRITERION_ASSESSMENT_NOT_MET
				}
			}
			assessed.Criteria = append(assessed.Criteria, &evalv1.AssessedCriterion{Criterion: int32(p.Criterion), Assessment: a})
		}
		return &evalv1.EssayReading{Outcome: &evalv1.EssayReading_Assessment{Assessment: assessed}}
	}
	var out []Named
	for _, rd := range l.Run.Readers {
		if !asked[rd.Name] {
			continue
		}
		var measured []*evalv1.SampleResult
		for _, e := range essays {
			res := &evalv1.EssayResult{}
			if u, ok := observationv1.Unreadable_value[e.Unreadable]; ok && e.Unreadable != "" {
				truth := observationv1.Unreadable(u)
				res.Truth = &truth
			}
			for _, p := range e.People {
				res.People = append(res.People, &evalv1.CriterionMark{Criterion: int32(p.Criterion), Met: p.Met})
			}
			for rep := 1; rep <= l.Run.Repeats; rep++ {
				k := key{e.Essay, rep, rd.Name}
				res.Readings = append(res.Readings, readingOf(e, calls[k], read[k]))
			}
			measured = append(measured, &evalv1.SampleResult{Sample: e.Sample, Result: &evalv1.SampleResult_Measured{Measured: &evalv1.Measurement{Essay: res}}})
		}
		m := modelOf(rd)
		r := head(evalv1.Capability_CAPABILITY_RUBRIC_ASSESSMENT, l.Run.Runner, l.Run.ModuleKit, set, l.Run.Repeats, m)
		r.Assessor = m
		out = append(out, Named{Name: rd.Name, Report: withRefused(r, measured, l.Run.Refused)})
	}
	return out
}
