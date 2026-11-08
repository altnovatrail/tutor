package report_test

import (
	"bytes"
	"io/fs"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/altnovatrail/module-kit/verdict"
	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"
	observationv1 "github.com/altnovatrail/spec/contracts/novatrail/observation/v1"
	"google.golang.org/protobuf/proto"

	"github.com/altnovatrail/tutor/internal/essay"
	"github.com/altnovatrail/tutor/internal/evalsample"
	"github.com/altnovatrail/tutor/internal/protocol"
	"github.com/altnovatrail/tutor/internal/protocol/protocoltest"
	"github.com/altnovatrail/tutor/internal/reading"
	"github.com/altnovatrail/tutor/internal/report"
	"github.com/altnovatrail/tutor/internal/runner"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/workbook"
)

// tag is a tag of a release, as section 14 writes the runner and the code of the verdicts.
var tag = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

// holds holds a report to the rules of section 14 of spec's contracts text that tutor writes it by,
// and to its file: it reads back, strictly, as it was written (R74).
func holds(t *testing.T, name string, r *evalv1.Report) {
	t.Helper()
	fail := func(format string, args ...any) { t.Errorf("%s: "+format, append([]any{name}, args...)...) }
	data, err := protocol.MarshalReport(r)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	back, err := protocol.ReadReport(bytes.NewReader(data))
	if err != nil || !proto.Equal(back, r) {
		fail("the report does not read back: %v", err)
	}
	reading := r.GetCapability() == evalv1.Capability_CAPABILITY_HANDWRITING_READING
	switch {
	case !tag.MatchString(r.GetRunner()), !tag.MatchString(r.GetVerdicts().GetVersion()):
		fail("the runner %q or the version of the verdicts %q is no tag", r.GetRunner(), r.GetVerdicts().GetVersion())
	case r.GetVerdicts().GetCode() != evalv1.VerdictCode_VERDICT_CODE_MODULE_KIT:
		fail("the verdicts are %s", r.GetVerdicts().GetCode())
	case reading && (len(r.GetReaders()) < 1 || len(r.GetReaders()) > 2 || r.GetAssessor() != nil):
		fail("a run of reading has %d readers, an assessor %v", len(r.GetReaders()), r.GetAssessor())
	case !reading && (len(r.GetReaders()) != 1 || r.GetAssessor() == nil):
		fail("a run of assessment has %d readers, an assessor %v", len(r.GetReaders()), r.GetAssessor())
	case strings.TrimSpace(r.GetCapabilitySet().GetName()) == "", r.GetRepeats() < 1:
		fail("the set %q, %d repeats", r.GetCapabilitySet().GetName(), r.GetRepeats())
	}
	for i, s := range r.GetSamples() {
		if i > 0 && r.GetSamples()[i-1].GetSample() >= s.GetSample() {
			fail("the sample %s comes after %s", s.GetSample(), r.GetSamples()[i-1].GetSample())
		}
		for _, slot := range s.GetMeasured().GetAnswerSlots() {
			verdictHolds(t, name, slot.GetTruth(), false)
			if len(slot.GetReadings()) != int(r.GetRepeats()) {
				fail("%s: %d readings of %d repeats", s.GetSample(), len(slot.GetReadings()), r.GetRepeats())
			}
			for _, rd := range slot.GetReadings() {
				verdictHolds(t, name, rd.GetVerdict(), true)
				if (rd.GetVerdict().GetVerdict() == "unchecked") != (rd.GetUnchecked() != evalv1.UncheckedCause_UNCHECKED_CAUSE_UNSPECIFIED) {
					fail("%s: the cause %s beside %s", s.GetSample(), rd.GetUnchecked(), rd.GetVerdict().GetVerdict())
				}
			}
		}
		if e := s.GetMeasured().GetEssay(); e != nil && len(e.GetReadings()) != int(r.GetRepeats()) {
			fail("%s: %d readings of the essay in %d repeats", s.GetSample(), len(e.GetReadings()), r.GetRepeats())
		}
		if s.GetMeasured() == nil && s.GetRefused() == evalv1.Refusal_REFUSAL_UNSPECIFIED {
			fail("%s is neither measured nor refused", s.GetSample())
		}
	}
}

// verdictHolds holds a verdict to the form of section 14: a reason beside unreadable alone, and a
// trap beside not_yet alone; unchecked only of a reading.
func verdictHolds(t *testing.T, name string, v *evalv1.Verdict, ofReading bool) {
	t.Helper()
	ok := slices.Contains([]string{"correct", "not_yet", "unreadable"}, v.GetVerdict()) || ofReading && v.GetVerdict() == "unchecked"
	if !ok || (v.GetReason() != "") != (v.GetVerdict() == "unreadable") || v.GetTrap() != "" && v.GetVerdict() != "not_yet" {
		t.Errorf("%s: the verdict %+v", name, v)
	}
}

// spec's examples of a report give the measures counted by hand from their records.
func TestMeasuresOfSpecExamples(t *testing.T) {
	ex := protocoltest.Examples(t)
	read := func(name string) *evalv1.Report {
		data, err := fs.ReadFile(ex, name)
		if err != nil {
			t.Fatal(err)
		}
		r, err := protocol.ReadReport(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return r
	}
	m := report.MeasuresOf(read("report-reading.json")).Reading
	all := m.All
	if all.Readings != 6 || all.FalseAlarm != (report.Count{}) || all.FalseCorrect != (report.Count{Events: 1, Of: 5}) || all.Needless != (report.Count{Events: 1, Of: 5}) ||
		all.Kept != (report.Count{}) || all.OtherTrap != (report.Count{Of: 3}) || all.Unchecked[evalv1.UncheckedCause_UNCHECKED_CAUSE_FAILURE] != 1 || m.Mirrored.Readings != 0 || m.CrossedOut.Readings != 0 {
		t.Errorf("the reading of spec's example: %+v", m)
	}
	e := report.MeasuresOf(read("report-assessment.json")).Essay
	if e.Readings != 3 || e.Needless != (report.Count{Events: 1, Of: 3}) || e.FalseAlarm != (report.Count{Events: 1, Of: 7}) || e.Agreement != (report.Count{Events: 8, Of: 9}) || e.None != 1 || len(e.Criteria) != 5 {
		t.Fatalf("the assessment of spec's example: %+v", e)
	}
	for _, c := range e.Criteria {
		want := c.Criterion == 5
		if (c.Kappa != nil) != want || want && (*c.Kappa != 0 || c.FalseAlarm != (report.Count{Events: 1, Of: 2}) || c.Agreement != (report.Count{Events: 1, Of: 2})) {
			t.Errorf("criterion %d: %+v, κ %v", c.Criterion, c, c.Kappa)
		}
		if c.Criterion == 3 && c.None != 1 {
			t.Errorf("criterion 3 has %d marks of none", c.None)
		}
	}
}

func ptr[T any](v T) *T { return &v }

// capped is the end of a run that its cap stopped: what it did not reach is the limit.
var capped = &runner.End{Kind: "end", Stopped: runner.StoppedByCap + ": $0.0100 spent of $0.01, and the next call can cost up to $0.0100"}

// readers are a reader that reads once and one that reads in repeats, as the run's record tells
// them.
var readers = []runner.ReaderRecord{{Name: "opus-high", Model: "claude-opus-5-5", Effort: "high", Once: true}, {Name: "haiku-low", Model: "claude-haiku-5-5", Effort: "low", NoThinking: true}}

// A log of the reading tells a report for each reader alone and for the pair: each sheet of the
// protocol with its truth, ascending by ID, and the refused that name an ID; a reading of each
// repeat of the finished sheet without a key at 32 px, by module-kit's words; a repeat not reached
// is the limit, an answer off the schema invalid, a call not asked a failure, a value not seen in
// its part ambiguous. A sheet of tutor-sample v0, with no ID, is in none.
func TestReadingReports(t *testing.T) {
	l := &reading.Log{Run: reading.RunRecord{Kind: "run", CallKind: reading.CallKind, Part: "dev", Sets: []string{"lines", "free"}, Readers: readers, Repeats: 2,
		Runner: "v0.1.8", ModuleKit: "v0.2.3", Refused: []runner.Refused{{Set: "lines", Sample: "s-0", File: "a.json", Reason: "REFUSAL_CHILD"}, {Set: "free", File: "b.json", Reason: "REFUSAL_INVALID"}}}, End: capped}
	sheet := func(id string, n int, truth string) reading.SheetRecord {
		rec := reading.SheetRecord{Kind: "sheet", Set: "lines", Sheet: n, Key: "5", Sample: id}
		if truth != "" {
			rec.Truth = []evalsample.Truth{{Item: 1, Slot: 1, Verdict: truth}}
		}
		return rec
	}
	l.Sheets = []reading.SheetRecord{sheet("s-2", 2, "not_yet"), sheet("s-1", 1, "correct"), sheet("", 3, "")}
	call := func(n int, reader string, rep int, v string, value string) *reading.CallRecord {
		l.Calls = append(l.Calls, reading.CallRecord{Kind: "call", Call: reading.Call{Set: "lines", Sheet: n, Condition: reading.NoKey, Reader: reader, Repeat: rep, Digit: 32, Drawn: 32}, Verdict: v, Value: value})
		return &l.Calls[len(l.Calls)-1]
	}
	call(1, "opus-high", 1, "correct", "5")
	call(1, "haiku-low", 1, "correct", "5.")
	call(2, "opus-high", 1, "not_yet", "6")
	off := call(2, "haiku-low", 1, "unchecked", "")
	off.Error, off.Invalid = "reading: the answer is not an observation", true
	call(2, "haiku-low", 2, "unreadable", "7").Reason = reading.NotWritten
	call(3, "haiku-low", 1, "correct", "5")
	call(1, "haiku-low", 1, "correct", "5").Condition = reading.RightKey // a reading with a key is no reading of the platform's
	rs := report.ReadingReports(l)
	var names []string
	for _, n := range rs {
		names = append(names, n.Name)
		holds(t, n.Name, n.Report)
	}
	if strings.Join(names, " ") != "opus-high haiku-low opus-high+haiku-low" {
		t.Fatalf("the reports are %v", names)
	}
	haiku := rs[1].Report
	if haiku.GetRepeats() != 2 || haiku.GetReaders()[0].GetModel() != "claude-haiku-5-5 low no-thinking" || haiku.GetReaders()[0].GetProvider() != "anthropic" ||
		haiku.GetCapabilitySet().GetName() != "lines+free-dev" || haiku.GetRunner() != "v0.1.8" || len(haiku.GetSamples()) != 3 || haiku.GetSamples()[0].GetRefused() != evalv1.Refusal_REFUSAL_CHILD {
		t.Fatalf("haiku-low's report: %v", haiku)
	}
	var got []string
	for _, s := range haiku.GetSamples()[1:] {
		for _, rd := range s.GetMeasured().GetAnswerSlots()[0].GetReadings() {
			got = append(got, rd.GetVerdict().GetVerdict()+" "+rd.GetVerdict().GetReason()+" "+rd.GetUnchecked().String())
		}
	}
	want := []string{"correct  UNCHECKED_CAUSE_UNSPECIFIED", "unchecked  UNCHECKED_CAUSE_LIMIT", "unchecked  UNCHECKED_CAUSE_INVALID", "unreadable ambiguous UNCHECKED_CAUSE_UNSPECIFIED"}
	if !slices.Equal(got, want) {
		t.Errorf("haiku-low reads\n%q\nwant\n%q", got, want)
	}
	pair := rs[2].Report
	if pair.GetRepeats() != 1 || len(pair.GetReaders()) != 2 {
		t.Fatalf("the pair's report: %v", pair)
	}
	s1, s2 := pair.GetSamples()[1].GetMeasured().GetAnswerSlots()[0], pair.GetSamples()[2].GetMeasured().GetAnswerSlots()[0]
	if s1.GetReadings()[0].GetVerdict().GetVerdict() != "correct" || s2.GetReadings()[0].GetUnchecked() != evalv1.UncheckedCause_UNCHECKED_CAUSE_INVALID {
		t.Errorf("the pair reads %v and %v", s1, s2)
	}
	r, err := report.NewReading(l)
	if err != nil {
		t.Fatal(err)
	}
	var md, csv bytes.Buffer
	if err := r.Markdown(&md, "x.jsonl"); err != nil {
		t.Fatal(err)
	}
	if err := r.ProtocolCSV(&csv); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(md.String(), "| haiku-low | claude-haiku-5-5 low no-thinking | 2 | 2 | 1 | 2 | 4 | 0 of 1 | 0 of 1 | 1 of 2 | 0 of 0 | 0 of 0 | 0 / 1 / 1 |") ||
		!strings.Contains(md.String(), "in no report, since their files name no ID that can be read: free/b.json") || len(r.Reports()) != 3 {
		t.Errorf("the section of the reports:\n%s", md.String())
	}
	if lines := strings.Count(csv.String(), "\n"); lines != 10 || !strings.HasPrefix(csv.String(), "report,readers,repeats,") {
		t.Errorf("the CSV of the reports has %d lines:\n%s", lines, csv.String())
	}
	// A log of no sample of the protocol tells no report.
	l.Sheets = l.Sheets[2:]
	if rs := report.ReadingReports(l); rs != nil {
		t.Errorf("a log of tutor-sample v0 tells %d reports", len(rs))
	}
}

// A sheet that shows no stroke goes to no model, and its report measures it with no answer slot, as
// a page whose boxes are all empty.
func TestReadingReportsBlankSheet(t *testing.T) {
	l := &reading.Log{Run: reading.RunRecord{Kind: "run", CallKind: reading.CallKind, Part: "all", Sets: []string{"canvas"}, Readers: readers[1:], Repeats: 1, Runner: "v0.1.8", ModuleKit: "v0.2.3"}}
	l.Sheets = []reading.SheetRecord{
		{Kind: "sheet", Set: "canvas", Sheet: 1, Key: "5", Sample: "s-1", Truth: []evalsample.Truth{{Item: 1, Slot: 1, Verdict: "correct"}}},
		{Kind: "sheet", Set: "canvas", Sheet: 2, Key: "5", Sample: "s-2"},
	}
	l.Calls = []reading.CallRecord{{Kind: "call", Call: reading.Call{Set: "canvas", Sheet: 1, Condition: reading.NoKey, Reader: "haiku-low", Repeat: 1, Digit: 32, Drawn: 32}, Verdict: "correct", Value: "5"}}
	rs := report.ReadingReports(l)
	if len(rs) != 1 {
		t.Fatalf("%d reports", len(rs))
	}
	holds(t, rs[0].Name, rs[0].Report)
	got := rs[0].Report.GetSamples()
	if len(got) != 2 || len(got[0].GetMeasured().GetAnswerSlots()) != 1 || got[1].GetSample() != "s-2" || got[1].GetMeasured() == nil || len(got[1].GetMeasured().GetAnswerSlots()) != 0 {
		t.Errorf("the samples: %v", got)
	}
}

// A log of the workbook tells a report of each page's filled boxes with their truths and notes; a
// reading of each by the record of the call bare at 32 px, a digit marked mirrored ambiguous to
// module-kit; an answer with a cell missing is invalid, and the pair joins the two observations of
// the page.
func TestWorkbookReports(t *testing.T) {
	box := func(item, slot int, key string, written *string, v string, traps ...sample.TrapValue) workbook.BoxRecord {
		return workbook.BoxRecord{Item: item, Slot: slot, Kind: sample.KindNumber, Key: key, Written: written, Verdict: verdictOf(v), TrapValues: traps}
	}
	pr := workbook.PageRecord{Kind: "page", Page: 1, Part: "dev", Sample: "synthetic-1-workbook-01", Boxes: []workbook.BoxRecord{
		box(1, 1, "8", ptr("8"), "correct", sample.TrapValue{Value: "9", Trap: "off-by-one"}), box(2, 1, "5", ptr("5"), "correct"), box(3, 1, "3", nil, "unreadable")},
		Truth: []evalsample.Truth{{Item: 1, Slot: 1, Verdict: "correct"}, {Item: 2, Slot: 1, Verdict: "unreadable", Reason: "ambiguous", Mirrored: true}}}
	l := &workbook.Log{Run: workbook.RunRecord{Kind: "run", CallKind: workbook.CallKind, Part: "dev", File: "eval/workbook.json", Readers: readers, Repeats: 2}, Sheets: []workbook.PageRecord{pr}, End: capped}
	blank := workbook.Box{Item: 3, Slot: 1, Verdict: verdictOf("unreadable"), Reason: workbook.Reason("blank")}
	call := func(reader string, rep int, boxes ...workbook.Box) *workbook.CallRecord {
		l.Calls = append(l.Calls, workbook.CallRecord{Kind: "call", Call: workbook.Call{Page: 1, Condition: workbook.Bare, Reader: reader, Repeat: rep, Digit: 32}, Boxes: boxes, Items: make([]verdict.Verdict, 3)})
		return &l.Calls[len(l.Calls)-1]
	}
	call("opus-high", 1, workbook.Box{Item: 1, Slot: 1, Text: ptr("9"), Verdict: verdictOf("not_yet"), Trap: "off-by-one"}, workbook.Box{Item: 2, Slot: 1, Text: ptr("5"), Verdict: verdictOf("unreadable"), Reason: workbook.Mirrored}, blank)
	call("haiku-low", 1, workbook.Box{Item: 1, Slot: 1, Text: ptr("9"), Verdict: verdictOf("not_yet"), Trap: "off-by-one"}, workbook.Box{Item: 2, Slot: 1, Text: ptr("5"), Verdict: verdictOf("correct")}, blank)
	call("haiku-low", 2, workbook.Box{Item: 1, Slot: 1, Verdict: verdictOf("unchecked"), Reason: workbook.Missing}, workbook.Box{Item: 2, Slot: 1, Verdict: verdictOf("unchecked")}, workbook.Box{Item: 3, Slot: 1, Verdict: verdictOf("unchecked")})
	call("haiku-low", 1, workbook.Box{Item: 1, Slot: 1, Text: ptr("8"), Verdict: verdictOf("correct")}, workbook.Box{Item: 2, Slot: 1, Text: ptr("5"), Verdict: verdictOf("correct")}, blank).Condition = workbook.Captioned
	rs := report.WorkbookReports(l)
	if len(rs) != 3 {
		t.Fatalf("%d reports", len(rs))
	}
	for _, n := range rs {
		holds(t, n.Name, n.Report)
	}
	read := func(r *evalv1.Report) []string {
		var out []string
		for _, slot := range r.GetSamples()[0].GetMeasured().GetAnswerSlots() {
			for _, rd := range slot.GetReadings() {
				v := rd.GetVerdict()
				out = append(out, v.GetVerdict()+"/"+v.GetReason()+"/"+v.GetTrap()+"/"+rd.GetUnchecked().String())
			}
		}
		return out
	}
	if got := read(rs[0].Report); !slices.Equal(got, []string{"not_yet//off-by-one/UNCHECKED_CAUSE_UNSPECIFIED", "unreadable/ambiguous//UNCHECKED_CAUSE_UNSPECIFIED"}) {
		t.Errorf("opus-high reads %q", got)
	}
	if got := read(rs[1].Report); !slices.Equal(got, []string{"not_yet//off-by-one/UNCHECKED_CAUSE_UNSPECIFIED", "unchecked///UNCHECKED_CAUSE_INVALID", "correct///UNCHECKED_CAUSE_UNSPECIFIED", "unchecked///UNCHECKED_CAUSE_INVALID"}) {
		t.Errorf("haiku-low reads %q", got)
	}
	// The two read one value 9 in the first box and differ in the second, which joins ambiguous.
	if got := read(rs[2].Report); !slices.Equal(got, []string{"not_yet//off-by-one/UNCHECKED_CAUSE_UNSPECIFIED", "unreadable/ambiguous//UNCHECKED_CAUSE_UNSPECIFIED"}) {
		t.Errorf("the pair reads %q", got)
	}
	m := report.MeasuresOf(rs[1].Report).Reading
	if m.Mirrored.Readings != 2 || m.Mirrored.Kept != (report.Count{Of: 1}) || m.All.Unchecked[evalv1.UncheckedCause_UNCHECKED_CAUSE_INVALID] != 2 {
		t.Errorf("haiku-low's measures: %+v", m)
	}
	if rs[0].Report.GetCapabilitySet().GetName() != "workbook-dev" {
		t.Errorf("the set is %q", rs[0].Report.GetCapabilitySet().GetName())
	}
	w, err := report.NewWorkbook(l)
	if err != nil {
		t.Fatal(err)
	}
	var md bytes.Buffer
	if err := w.Markdown(&md, "w.jsonl"); err != nil || !strings.Contains(md.String(), "| haiku-low, mirrored |") || len(w.Reports()) != 3 {
		t.Errorf("the workbook's section of the reports, %v:\n%s", err, md.String())
	}
	if err := w.ProtocolCSV(&md); err != nil {
		t.Error(err)
	}
}

// verdictOf is a verdict by its name.
func verdictOf(v string) verdict.Verdict { return verdict.Verdict(v) }

// A log of the essays tells a report for each reader, which reads and assesses: each essay with the
// people's marks; an assessment marks each criterion the people marked, none where AI gave no
// mark; no mark of any criterion is NO_MARK; in two calls, an assessment not asked after a reading
// off the schema is invalid; a repeat not reached is the limit.
func TestEssayReports(t *testing.T) {
	rubric := []sample.Criterion{{ID: sample.Criteria[0], Text: "a"}, {ID: sample.Criteria[1], Text: "b"}}
	l := &essay.Log{Run: essay.RunRecord{Kind: "run", CallKind: essay.CallKind, Part: "all", File: "eval/essays.json", Rubric: rubric, Readers: readers[1:], Repeats: 4, Path: "two"}, End: capped,
		Sheets: []essay.EssayRecord{{Kind: "essay", Essay: 1, Sample: "synthetic-1-essays-01", People: []evalsample.Mark{{Criterion: 1, Met: true}, {Criterion: 2, Met: false}}}}}
	call := func(rep int, step string, marks ...essay.Mark) *essay.CallRecord {
		l.Calls = append(l.Calls, essay.CallRecord{Kind: "call", Call: essay.Call{Essay: 1, Reader: "haiku-low", Repeat: rep, Step: step}, Marks: marks})
		return &l.Calls[len(l.Calls)-1]
	}
	call(1, essay.StepRead)
	call(1, essay.StepAssess, essay.Mark{ID: sample.Criteria[0], Met: ptr(false)}, essay.Mark{ID: sample.Criteria[1], Reason: essay.ReasonMissing})
	call(2, essay.StepRead)
	call(2, essay.StepAssess, essay.Mark{ID: sample.Criteria[0], Reason: essay.ReasonMissing}, essay.Mark{ID: sample.Criteria[1], Reason: essay.ReasonMissing})
	r := call(3, essay.StepRead)
	r.Error, r.Invalid = "essay: off the schema", true
	call(3, essay.StepAssess).NotAsked = true
	rs := report.EssayReports(l)
	if len(rs) != 1 {
		t.Fatalf("%d reports", len(rs))
	}
	holds(t, rs[0].Name, rs[0].Report)
	e := rs[0].Report.GetSamples()[0].GetMeasured().GetEssay()
	var got []string
	for _, rd := range e.GetReadings() {
		switch o := rd.GetOutcome().(type) {
		case *evalv1.EssayReading_Assessment:
			for _, c := range o.Assessment.GetCriteria() {
				got = append(got, c.GetAssessment().String())
			}
		case *evalv1.EssayReading_Unchecked:
			got = append(got, o.Unchecked.String())
		}
	}
	want := []string{"CRITERION_ASSESSMENT_NOT_MET", "CRITERION_ASSESSMENT_NONE", "UNCHECKED_CAUSE_NO_MARK", "UNCHECKED_CAUSE_INVALID", "UNCHECKED_CAUSE_LIMIT"}
	if !slices.Equal(got, want) || len(e.GetPeople()) != 2 || e.Truth != nil || rs[0].Report.GetAssessor().GetModel() != "claude-haiku-5-5 low no-thinking" {
		t.Errorf("the essay reads %q, people %v", got, e.GetPeople())
	}
	m := report.MeasuresOf(rs[0].Report).Essay
	if m.FalseAlarm != (report.Count{Events: 1, Of: 1}) || m.None != 1 || m.Needless != (report.Count{Of: 1}) || m.Criteria[0].Kappa == nil || *m.Criteria[0].Kappa != 0 || m.Criteria[1].Kappa != nil {
		t.Errorf("the measures: %+v", m)
	}
	es, err := report.NewEssays(l)
	if err != nil {
		t.Fatal(err)
	}
	var md bytes.Buffer
	if err := es.Markdown(&md, "e.jsonl"); err != nil || !strings.Contains(md.String(), "| haiku-low | 2 | 0 of 0 | 0 of 0 | 1 | — |") || len(es.Reports()) != 1 {
		t.Errorf("the essays' section of the reports, %v:\n%s", err, md.String())
	}
	if err := es.ProtocolCSV(&md); err != nil {
		t.Error(err)
	}
	if rs := report.EssayReports(&essay.Log{Run: l.Run}); rs != nil {
		t.Errorf("a log of no essay of the protocol tells %d reports", len(rs))
	}
}

// The causes of an unchecked reading: a call not asked and a call that failed are failures, and so
// is a check that module-kit refuses, a key out of the notation, which no bank gives; a reader with
// no reading of the platform's tells no report, nor any pair with it; every reader is Anthropic's,
// as every configuration of the runner is a model of Claude. The truth keeps its trap.
func TestReadingReportsCauses(t *testing.T) {
	rds := []runner.ReaderRecord{readers[0], {Name: "gpt-low", Model: "gpt-6-luna", Effort: "low"}, {Name: "sonnet-low", Model: "claude-sonnet-5-5", Effort: "low"}}
	l := &reading.Log{Run: reading.RunRecord{Kind: "run", CallKind: reading.CallKind, Part: "all", Sets: []string{"lines"}, Readers: rds, Repeats: 1}, Sheets: []reading.SheetRecord{
		{Kind: "sheet", Set: "lines", Sheet: 1, Key: "5", Sample: "k-1", Truth: []evalsample.Truth{{Item: 1, Slot: 1, Verdict: "not_yet", Trap: "carry"}}},
		{Kind: "sheet", Set: "lines", Sheet: 2, Key: "x = 5", Sample: "k-2", Truth: []evalsample.Truth{{Item: 1, Slot: 1, Verdict: "correct"}}},
	}}
	add := func(c reading.CallRecord) {
		c.Kind, c.Condition, c.Digit, c.Drawn, c.Repeat = "call", reading.NoKey, 32, 32, 1
		l.Calls = append(l.Calls, c)
	}
	add(reading.CallRecord{Call: reading.Call{Set: "lines", Sheet: 1, Reader: "opus-high"}, NotAsked: true, Verdict: "unchecked"})
	add(reading.CallRecord{Call: reading.Call{Set: "lines", Sheet: 1, Reader: "gpt-low"}, Error: "the provider did not answer", Verdict: "unchecked"})
	add(reading.CallRecord{Call: reading.Call{Set: "lines", Sheet: 2, Reader: "opus-high"}, Verdict: "correct", Value: "5"})
	add(reading.CallRecord{Call: reading.Call{Set: "lines", Sheet: 2, Reader: "gpt-low"}, Verdict: "correct", Value: "5"})
	rs := report.ReadingReports(l)
	var names []string
	for _, n := range rs {
		names = append(names, n.Name)
		holds(t, n.Name, n.Report)
		if n.Report.GetRunner() != runner.NoRelease {
			t.Errorf("%s: the runner of a log that names none is %s", n.Name, n.Report.GetRunner())
		}
		for _, s := range n.Report.GetSamples() {
			if rd := s.GetMeasured().GetAnswerSlots()[0].GetReadings()[0]; rd.GetUnchecked() != evalv1.UncheckedCause_UNCHECKED_CAUSE_FAILURE {
				t.Errorf("%s, %s: %v", n.Name, s.GetSample(), rd)
			}
		}
	}
	if strings.Join(names, " ") != "opus-high gpt-low opus-high+gpt-low" || rs[1].Report.GetReaders()[0].GetProvider() != "anthropic" {
		t.Fatalf("the reports are %v", names)
	}
	if tr := rs[0].Report.GetSamples()[0].GetMeasured().GetAnswerSlots()[0].GetTruth(); tr.GetTrap() != "carry" {
		t.Errorf("the truth %v", tr)
	}
}

// A page's readings: a call that failed, a box unchecked with no reason of the reading, a repeat not
// reached by a run that stopped otherwise than at its cap, a pair one of whose readers did not read
// the page or failed, and a check that module-kit refuses, which neither judges nor joins: each
// unchecked with its cause, a failure.
func TestWorkbookReportsCauses(t *testing.T) {
	rds := []runner.ReaderRecord{readers[0], readers[1], {Name: "sonnet-low", Model: "claude-sonnet-5-5", Effort: "low"}}
	page := func(n int, id string, traps ...sample.TrapValue) workbook.PageRecord {
		return workbook.PageRecord{Kind: "page", Page: n, Part: "dev", Sample: id, Boxes: []workbook.BoxRecord{{Item: 1, Slot: 1, Kind: sample.KindNumber, Key: "8", Written: ptr("8"), Verdict: verdictOf("correct"), TrapValues: traps}},
			Truth: []evalsample.Truth{{Item: 1, Slot: 1, Verdict: "correct"}}}
	}
	l := &workbook.Log{Run: workbook.RunRecord{Kind: "run", CallKind: workbook.CallKind, Part: "dev", File: "eval/workbook.json", Readers: rds, Repeats: 2},
		Sheets: []workbook.PageRecord{page(1, "p-1"), page(2, "p-2"), page(3, "p-3", sample.TrapValue{Value: "{n}", Trap: "placeholder"})}}
	right := func(page int, reader string) workbook.CallRecord {
		return workbook.CallRecord{Kind: "call", Call: workbook.Call{Page: page, Condition: workbook.Bare, Reader: reader, Repeat: 1, Digit: 32}, Items: []verdict.Verdict{"correct"},
			Boxes: []workbook.Box{{Item: 1, Slot: 1, Text: ptr("8"), Verdict: verdictOf("correct")}}}
	}
	failed := right(1, "opus-high")
	failed.Error = "the provider did not answer"
	blankless := right(1, "haiku-low")
	blankless.Boxes[0].Verdict, blankless.Boxes[0].Text = verdictOf("unchecked"), nil
	l.Calls = []workbook.CallRecord{failed, blankless, right(2, "haiku-low"), right(3, "opus-high"), right(3, "haiku-low")}
	rs := report.WorkbookReports(l)
	got := map[string][]string{}
	for _, n := range rs {
		holds(t, n.Name, n.Report)
		for _, s := range n.Report.GetSamples() {
			for _, rd := range s.GetMeasured().GetAnswerSlots()[0].GetReadings() {
				got[n.Name] = append(got[n.Name], s.GetSample()+" "+rd.GetVerdict().GetVerdict()+" "+rd.GetUnchecked().String())
			}
		}
	}
	want := map[string][]string{
		"opus-high": {"p-1 unchecked UNCHECKED_CAUSE_FAILURE", "p-2 unchecked UNCHECKED_CAUSE_FAILURE", "p-3 unchecked UNCHECKED_CAUSE_FAILURE"},
		"haiku-low": {"p-1 unchecked UNCHECKED_CAUSE_FAILURE", "p-1 unchecked UNCHECKED_CAUSE_FAILURE", "p-2 correct UNCHECKED_CAUSE_UNSPECIFIED", "p-2 unchecked UNCHECKED_CAUSE_FAILURE",
			"p-3 unchecked UNCHECKED_CAUSE_FAILURE", "p-3 unchecked UNCHECKED_CAUSE_FAILURE"},
		"opus-high+haiku-low": {"p-1 unchecked UNCHECKED_CAUSE_FAILURE", "p-2 unchecked UNCHECKED_CAUSE_FAILURE", "p-3 unchecked UNCHECKED_CAUSE_FAILURE"},
	}
	if len(got) != len(want) {
		t.Fatalf("the reports: %v", got)
	}
	for name, w := range want {
		if !slices.Equal(got[name], w) {
			t.Errorf("%s reads\n%q\nwant\n%q", name, got[name], w)
		}
	}
}

// An essay whose truth gives no text keeps the reason as its truth, with no people's marks; an
// assessment that failed is a failure, one that marks a criterion met is met; a reader with no call
// tells no report.
func TestEssayReportsCauses(t *testing.T) {
	rubric := []sample.Criterion{{ID: sample.Criteria[0], Text: "a"}}
	l := &essay.Log{Run: essay.RunRecord{Kind: "run", CallKind: essay.CallKind, Part: "all", File: "eval/essays.json", Rubric: rubric, Readers: readers, Repeats: 2},
		Sheets: []essay.EssayRecord{
			{Kind: "essay", Essay: 1, Sample: "e-1", Unreadable: "UNREADABLE_ILLEGIBLE"},
			{Kind: "essay", Essay: 2, Sample: "e-2", People: []evalsample.Mark{{Criterion: 1, Met: true}}},
		}}
	l.Calls = []essay.CallRecord{
		{Kind: "call", Call: essay.Call{Essay: 2, Reader: "haiku-low", Repeat: 1}, Error: "the provider did not answer"},
		{Kind: "call", Call: essay.Call{Essay: 2, Reader: "haiku-low", Repeat: 2}, Marks: []essay.Mark{{ID: sample.Criteria[0], Met: ptr(true)}}},
	}
	rs := report.EssayReports(l)
	if len(rs) != 1 || rs[0].Name != "haiku-low" {
		t.Fatalf("%d reports", len(rs))
	}
	holds(t, rs[0].Name, rs[0].Report)
	s := rs[0].Report.GetSamples()
	if e := s[0].GetMeasured().GetEssay(); e.Truth == nil || *e.Truth != observationv1.Unreadable_UNREADABLE_ILLEGIBLE || len(e.GetPeople()) != 0 {
		t.Errorf("the essay that cannot be read: %v", e)
	}
	r := s[1].GetMeasured().GetEssay().GetReadings()
	if r[0].GetUnchecked() != evalv1.UncheckedCause_UNCHECKED_CAUSE_FAILURE || r[1].GetAssessment().GetCriteria()[0].GetAssessment() != evalv1.CriterionAssessment_CRITERION_ASSESSMENT_MET {
		t.Errorf("the essay read: %v", r)
	}
	m := report.MeasuresOf(rs[0].Report).Essay
	if m.Agreement != (report.Count{Events: 1, Of: 1}) || m.FalseAlarm != (report.Count{Of: 1}) || m.Needless != (report.Count{Of: 1}) {
		t.Errorf("the measures: %+v", m)
	}
}
