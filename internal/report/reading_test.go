package report_test

import (
	"bytes"
	"context"
	"flag"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/altnovatrail/tutor/internal/reading"
	"github.com/altnovatrail/tutor/internal/report"
	"github.com/altnovatrail/tutor/internal/runner"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/sheets"
	"github.com/altnovatrail/tutor/internal/synth"
	"github.com/altnovatrail/tutor/internal/value"
)

var update = flag.Bool("update", false, "write the golden log and report of the reading anew")

// readingLog builds the log of a run of the reading by hand: three readers, Opus once and two
// Haiku, and sheets of the set "s" by their outcomes.
type readingLog struct{ l reading.Log }

func newReadingLog(outcomes ...sample.Outcome) *readingLog {
	b := &readingLog{l: reading.Log{Run: reading.RunRecord{Kind: "run", CallKind: reading.CallKind, Part: "test", Sets: []string{"s"}, Repeats: 3,
		Reads: []string{"keys"}, MaxUSD: 6, Prompt: strings.Repeat("a", 64), PromptVersion: 1,
		Readers: []reading.ReaderRecord{{Name: "opus-high", Model: "claude-opus-5-5", Effort: "high", Once: true}, {Name: "haiku-low", Model: "claude-haiku-5-5", Effort: "low"},
			{Name: "haiku-low-no-thinking", Model: "claude-haiku-5-5", Effort: "low", NoThinking: true}}}}}
	for i, o := range outcomes {
		b.l.Sheets = append(b.l.Sheets, reading.SheetRecord{Kind: "sheet", Set: "s", Sheet: i + 1, Outcome: o, Key: "5",
			Markup: reading.Marked{Parts: []reading.MarkedPart{{Kind: reading.KindText, Text: "2 + 3 = 5", Answer: true}}, Value: "5"}})
	}
	b.l.End = &runner.End{Kind: "end"}
	return b
}

// call adds a call of a sheet at 32 px with its verdict and value, read under the key or not.
func (b *readingLog) call(sheet int, reader string, cond reading.Condition, repeat int, verdict value.Verdict, val string, under bool) *reading.CallRecord {
	c := reading.CallRecord{Kind: "call", Call: reading.Call{Set: "s", Sheet: sheet, Condition: cond, Reader: reader, Repeat: repeat, Digit: 32, Drawn: 32},
		Verdict: string(verdict), Value: val, UnderKey: reading.Under{Result: under}}
	c.LatencyMs, c.CostUSD = int64(1000*sheet), 0.01
	b.l.Calls = append(b.l.Calls, c)
	return &b.l.Calls[len(b.l.Calls)-1]
}

func (b *readingLog) report(t *testing.T) *report.Reading {
	t.Helper()
	r, err := report.NewReading(&b.l)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// thresholdOf is the threshold of a reader.
func thresholdOf(t *testing.T, r *report.Reading, reader string) report.Threshold {
	t.Helper()
	for _, th := range r.Thresholds {
		if th.Reader == reader {
			return th
		}
	}
	t.Fatalf("no threshold of %s", reader)
	return report.Threshold{}
}

// The threshold of R03 a reader each, on known numbers:
//   - haiku-low gives false alarms with the right key on five correct sheets, in each of its three
//     repeats, and none without: d, a share, is +1 on five sheets, p 1/32, and the key makes its
//     reading worse;
//   - Opus reads a correct sheet under a false key: it fails on that alone, and what it reads under
//     the right key without being given it is only the baseline;
//   - haiku-low-no-thinking says "correct" with the right key on a sheet with a mistake where it said
//     "not yet" without: a new false "correct", though one sheet with d ≠ 0 cannot make the test.
func TestReadingThreshold(t *testing.T) {
	b := newReadingLog(sample.OutcomeCorrect, sample.OutcomeCorrect, sample.OutcomeCorrect, sample.OutcomeCorrect, sample.OutcomeCorrect, sample.OutcomeError)
	for sheet := 1; sheet <= 5; sheet++ {
		for rep := 1; rep <= 3; rep++ {
			b.call(sheet, "haiku-low", reading.NoKey, rep, value.Correct, "5", false)
			b.call(sheet, "haiku-low", reading.RightKey, rep, value.NotYet, "6", false)
		}
		b.call(sheet, "opus-high", reading.NoKey, 1, value.Correct, "5", sheet == 1)
		b.call(sheet, "opus-high", reading.RightKey, 1, value.Correct, "5", false)
		b.call(sheet, "opus-high", reading.FalseKey, 1, value.Correct, "5", sheet == 2)
	}
	b.call(6, "haiku-low-no-thinking", reading.NoKey, 1, value.NotYet, "6", false)
	b.call(6, "haiku-low-no-thinking", reading.RightKey, 1, value.Correct, "5", true)
	r := b.report(t)

	haiku := thresholdOf(t, r, "haiku-low")
	if haiku.Sum != 5 || haiku.Nonzero != 5 || math.Abs(haiku.Helps-1.0/32) > 1e-12 || !haiku.Worse() || haiku.Passes() || haiku.Sheets != 5 {
		t.Errorf("haiku-low: %+v", haiku)
	}
	opus := thresholdOf(t, r, "opus-high")
	if opus.Worse() || opus.UnderFalse != 1 || opus.FalseCalls != 5 || opus.UnderNone != 1 || opus.UnderRight != 0 || opus.Passes() {
		t.Errorf("opus-high: %+v", opus)
	}
	quick := thresholdOf(t, r, "haiku-low-no-thinking")
	if quick.NewFalseCorrect != 1 || quick.UnderRight != 1 || quick.Worse() || quick.Passes() {
		t.Errorf("haiku-low-no-thinking: %+v", quick)
	}
	// Without the false key and the readings under keys, Opus passes.
	b = newReadingLog(sample.OutcomeCorrect, sample.OutcomeError)
	b.call(1, "opus-high", reading.NoKey, 1, value.Correct, "5", false)
	b.call(1, "opus-high", reading.RightKey, 1, value.Correct, "5", false)
	b.call(2, "opus-high", reading.NoKey, 1, value.NotYet, "6", false)
	b.call(2, "opus-high", reading.RightKey, 1, value.NotYet, "6", false)
	if th := thresholdOf(t, b.report(t), "opus-high"); !th.Passes() || th.Nonzero != 0 {
		t.Errorf("a reader the key changes nothing for: %+v", th)
	}
	// d is a share over the calls that gave a verdict: a failed call and a run cut short between
	// the conditions do not move it. Two readings without a key, one of them a false alarm, against
	// one with the right key, a false alarm, and one failed: d is 1 − 1/2.
	b = newReadingLog(sample.OutcomeCorrect)
	b.call(1, "haiku-low", reading.NoKey, 1, value.NotYet, "6", false)
	b.call(1, "haiku-low", reading.NoKey, 2, value.Correct, "5", false)
	b.call(1, "haiku-low", reading.RightKey, 1, value.NotYet, "6", false)
	b.call(1, "haiku-low", reading.RightKey, 2, value.Unchecked, "", false).Error = "cut off"
	if th := thresholdOf(t, b.report(t), "haiku-low"); th.Sum != 0.5 || th.Nonzero != 1 {
		t.Errorf("d over the calls that gave a verdict: %+v", th)
	}
	// A log without the conditions of the key does not measure the threshold, and a part other than
	// the test does not decide it.
	b = newReadingLog(sample.OutcomeCorrect)
	b.call(1, "haiku-low", reading.NoKey, 1, value.Correct, "5", false)
	if th := thresholdOf(t, b.report(t), "haiku-low"); th.Measured() || th.Holds() || th.Passes() {
		t.Errorf("a threshold without the right key: %+v", th)
	}
	b = newReadingLog(sample.OutcomeCorrect)
	b.l.Run.Part = "dev"
	b.call(1, "opus-high", reading.NoKey, 1, value.Correct, "5", false)
	b.call(1, "opus-high", reading.RightKey, 1, value.Correct, "5", false)
	if th := thresholdOf(t, b.report(t), "opus-high"); !th.Holds() || th.Passes() {
		t.Errorf("a threshold of the dev part: %+v", th)
	}
}

// The readings by reader and condition count right answers, false "correct" among the sheets that
// are not correct, false alarms among the correct ones and extra "unreadable" among the legible
// ones, each with its bound over the sheets, and the failures, the parts of writing read and the
// time.
func TestReadingCells(t *testing.T) {
	b := newReadingLog(sample.OutcomeCorrect, sample.OutcomeCorrect, sample.OutcomeError, sample.OutcomeUnreadable)
	b.call(1, "haiku-low", reading.NoKey, 1, value.Correct, "5", false).Answer = `{"parts": [{"kind": "text", "text": "2+3=5", "legible": true, "other_readings": []}], "results": [{"part": 1, "value": "5", "other_values": []}]}`
	b.call(2, "haiku-low", reading.NoKey, 1, value.NotYet, "6", false)
	b.call(3, "haiku-low", reading.NoKey, 1, value.Correct, "5", false)
	b.call(4, "haiku-low", reading.NoKey, 1, value.Unreadable, "", false)
	b.call(1, "haiku-low", reading.NoKey, 2, value.Unreadable, "", false)
	c := b.call(2, "haiku-low", reading.NoKey, 2, value.Unchecked, "", false)
	c.Error = "timeout"
	r := b.report(t)
	var cell report.ReadingCell
	for _, x := range r.Cells {
		if x.Reader == "haiku-low" && x.Condition == reading.NoKey {
			cell = x
		}
	}
	if cell.Calls != 6 || cell.Right != 2 || cell.Failed != 1 || cell.PartsRead != 1 || cell.PartsOf != 6 {
		t.Errorf("the cell %+v", cell)
	}
	// The failed call of the correct sheet 2 is no false alarm, and it is an extra "unreadable".
	if cell.FalseCorrect.Events != 1 || cell.FalseCorrect.Of != 2 || cell.FalseAlarm.Events != 1 || cell.FalseAlarm.Of != 3 || cell.Extra.Events != 2 || cell.Extra.Of != 5 {
		t.Errorf("the shares %v, %v, %v", cell.FalseCorrect, cell.FalseAlarm, cell.Extra)
	}
	if want := report.Upper([]int{1, 0}, []int{1, 1}); cell.FalseCorrect.Upper != want {
		t.Errorf("the bound of false \"correct\" %v, want %v", cell.FalseCorrect.Upper, want)
	}
	if cell.TimeP50 != 2000 || cell.TimeP95 != 4000 {
		t.Errorf("time %d and %d", cell.TimeP50, cell.TimeP95)
	}
}

// Two readers release a sheet when both give one value and their verdicts agree: not when the
// values differ, nor when one failed. p of the hybrid is the share of sheets Haiku does not confirm
// "correct" on.
func TestReadingPairs(t *testing.T) {
	b := newReadingLog(sample.OutcomeCorrect, sample.OutcomeCorrect, sample.OutcomeError, sample.OutcomeError)
	b.call(1, "opus-high", reading.NoKey, 1, value.Correct, "5", false)
	b.call(1, "haiku-low", reading.NoKey, 1, value.Correct, "5.", false)
	b.call(2, "opus-high", reading.NoKey, 1, value.Correct, "5", false)
	b.call(2, "haiku-low", reading.NoKey, 1, value.NotYet, "6", false)
	b.call(3, "opus-high", reading.NoKey, 1, value.NotYet, "6", false)
	b.call(3, "haiku-low", reading.NoKey, 1, value.Correct, "5", false)
	b.call(4, "opus-high", reading.NoKey, 1, value.NotYet, "6", false)
	b.call(4, "haiku-low", reading.NoKey, 1, value.NotYet, "6", false).Error = "cut off"
	b.call(4, "haiku-low", reading.NoKey, 2, value.NotYet, "6", false)
	r := b.report(t)
	var p report.Pair
	for _, x := range r.Pairs {
		if x.Fast == "haiku-low" {
			p = x
		}
	}
	if p.Strong != "opus-high" || p.Sheets != 4 || p.Released != 1 || p.FalseCorrect != 0 || p.FastCorrect != 2 || p.BothCorrect != 1 || p.FastSendsOn != 2 || p.P() != 0.5 {
		t.Errorf("the pair %+v", p)
	}
	if p.FastAlone != (report.Errors{FalseCorrect: 1, FalseAlarm: 1}) || p.StrongAlone != (report.Errors{}) {
		t.Errorf("alone: %v and %v", p.StrongAlone, p.FastAlone)
	}
	if len(r.Pairs) != 2 {
		t.Errorf("%d pairs, want Opus with each Haiku", len(r.Pairs))
	}
}

// A log without readers or sheets, a sheet twice and a call of a sheet, a reader or a condition the
// log does not tell make no report.
func TestReadingRefuses(t *testing.T) {
	for name, spoil := range map[string]func(*readingLog){
		"no sheets":      func(b *readingLog) { b.l.Sheets = nil },
		"no readers":     func(b *readingLog) { b.l.Run.Readers = nil },
		"a sheet twice":  func(b *readingLog) { b.l.Sheets = append(b.l.Sheets, b.l.Sheets[0]) },
		"another sheet":  func(b *readingLog) { b.call(9, "opus-high", reading.NoKey, 1, value.Correct, "5", false) },
		"another reader": func(b *readingLog) { b.call(1, "sonnet", reading.NoKey, 1, value.Correct, "5", false) },
		"another condition": func(b *readingLog) {
			b.call(1, "opus-high", reading.Condition("half"), 1, value.Correct, "5", false)
		},
	} {
		b := newReadingLog(sample.OutcomeCorrect)
		spoil(b)
		if _, err := report.NewReading(&b.l); err == nil {
			t.Errorf("%s: a report", name)
		}
	}
}

// A stub run of the reading over sheets of E2 and of free writing goes to a report, the Markdown and
// the CSV both, byte for byte the golden ones (testdata/reading-stub.*); the log itself is golden
// too. On the stub every reader holds the threshold, which the dev part does not decide, and two
// readers release every sheet.
func TestReadingLogOfTheStub(t *testing.T) {
	var chosen []sheets.Chosen
	for _, set := range []struct {
		name, plan, bank string
		numbers          []int
	}{{"lines", "../../eval/sheets.json", "../../eval/bank.json", []int{1, 7, 11, 21}}, {"free", "../../eval/free-sheets.json", "../../eval/free-bank.json", []int{1, 2, 14}}} {
		chosen = append(chosen, chosenOf(t, set.name, set.plan, set.bank, set.numbers...)...)
	}
	r, err := reading.New("dev", []string{"lines", "free"}, chosen, reading.Readers(), 2, reading.Reads{Keys: true})
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	clock := func() time.Time { return time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC) }
	if _, err := r.Do(context.Background(), runner.Stub, 10, &log, clock); err != nil {
		t.Fatal(err)
	}
	l, err := reading.ReadLog(bytes.NewReader(log.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	rep, err := report.NewReading(l)
	if err != nil {
		t.Fatal(err)
	}
	var md, csv bytes.Buffer
	if err := rep.Markdown(&md, "reading-stub.jsonl"); err != nil {
		t.Fatal(err)
	}
	if err := rep.CSV(&csv); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{"testdata/reading-stub.jsonl": log.Bytes(), "testdata/reading-stub.md": md.Bytes(), "testdata/reading-stub.csv": csv.Bytes()}
	for name, got := range files {
		if *update {
			if err := os.WriteFile(name, got, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s differs:\n%s", name, got)
		}
	}
	for _, th := range rep.Thresholds {
		if !th.Holds() || th.Passes() || th.Sum != 0 {
			t.Errorf("on the stub of the dev part, %+v", th)
		}
	}
	for _, p := range rep.Pairs {
		if p.Released != p.Sheets || p.Sheets != 7 {
			t.Errorf("on the stub, the pair %+v", p)
		}
	}
	if s := rep.Summary(); strings.Count(s, "the threshold holds here; only the test part decides") != 3 {
		t.Errorf("the summary:\n%s", s)
	}
}

// chosenOf are sheets of a plan drawn by code with the seed 1, as a part of a run takes them.
func chosenOf(t *testing.T, set, planPath, bankPath string, numbers ...int) []sheets.Chosen {
	t.Helper()
	read := func(path string, f func(*os.File) error) {
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = file.Close() }()
		if err := f(file); err != nil {
			t.Fatal(err)
		}
	}
	var bank *sample.Bank
	var plan *sample.Plan
	read(bankPath, func(f *os.File) (err error) { bank, err = sample.ReadBank(f); return err })
	read(planPath, func(f *os.File) (err error) { plan, err = sample.ReadPlan(f); return err })
	var out []sheets.Chosen
	for _, n := range numbers {
		sh := &plan.Sheets[n-1]
		out = append(out, sheets.Chosen{Set: set, Number: n, File: "sheet.json", Sample: synth.Sheet(sh, 1), Task: bank.Task(sh.Task), Plan: sh})
	}
	return out
}
