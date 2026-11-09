package report

import (
	"testing"

	"github.com/altnovatrail/module-kit/verdict"
	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"

	"github.com/altnovatrail/tutor/internal/evalsample"
	"github.com/altnovatrail/tutor/internal/reading"
	"github.com/altnovatrail/tutor/internal/runner"
)

// module-kit's verdict on a valid observation: unchecked is the check's, a cause of the task and not
// of the model (R87), and any other verdict is the reading's, with no cause. tutor's own checks run
// no predicate, so no log of its runs gives the first: only this test does.
func TestChecked(t *testing.T) {
	if r := checked(verdict.SlotVerdict{Verdict: verdict.Unchecked}); r.GetVerdict().GetVerdict() != string(verdict.Unchecked) ||
		r.GetUnchecked() != evalv1.UncheckedCause_UNCHECKED_CAUSE_CHECK {
		t.Errorf("unchecked on a valid observation: %v", r)
	}
	if r := checked(verdict.SlotVerdict{Verdict: verdict.Correct}); r.GetVerdict().GetVerdict() != string(verdict.Correct) ||
		r.GetUnchecked() != evalv1.UncheckedCause_UNCHECKED_CAUSE_UNSPECIFIED {
		t.Errorf("correct: %v", r)
	}
}

// A call recorded unchecked with no failure gives no observation: alone and in a pair, its reading
// is a failure, not the check's, and the pair's join tells why it gives none.
func TestUncheckedRecordIsAFailure(t *testing.T) {
	rds := []runner.ReaderRecord{{Name: "opus-high", Model: "claude-opus-5-5", Effort: "high", Once: true}, {Name: "haiku-low", Model: "claude-haiku-5-5", Effort: "low"}}
	l := &reading.Log{Run: reading.RunRecord{Kind: "run", CallKind: reading.CallKind, Part: "all", Sets: []string{"lines"}, Readers: rds, Repeats: 1}, Sheets: []reading.SheetRecord{
		{Kind: "sheet", Set: "lines", Sheet: 1, Key: "5", Sample: "k-1", Truth: []evalsample.Truth{{Item: 1, Slot: 1, Verdict: "correct"}}},
	}}
	for _, rd := range rds {
		l.Calls = append(l.Calls, reading.CallRecord{Kind: "call", Verdict: "unchecked",
			Call: reading.Call{Set: "lines", Sheet: 1, Reader: rd.Name, Condition: reading.NoKey, Digit: 32, Drawn: 32, Repeat: 1}})
	}
	rs := ReadingReports(l)
	if len(rs) != 3 {
		t.Fatalf("%d reports", len(rs))
	}
	for _, n := range rs {
		if rd := n.Report.GetSamples()[0].GetMeasured().GetAnswerSlots()[0].GetReadings()[0]; rd.GetUnchecked() != evalv1.UncheckedCause_UNCHECKED_CAUSE_FAILURE {
			t.Errorf("%s: %v", n.Name, rd)
		}
	}
	if _, _, err := joinedSlot("5", &l.Calls[0], &l.Calls[1]); err == nil {
		t.Error("two calls with no observation join")
	}
}
