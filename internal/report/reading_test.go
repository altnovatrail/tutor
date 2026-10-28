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
	"github.com/altnovatrail/tutor/internal/states"
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
	r, err := reading.New("dev", []string{"lines", "free"}, chosen, reading.Readers(), 2, reading.Reads{Keys: true, Heights: true, States: true})
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

// heightCall adds a reading without a key of a finished sheet at a height, the first repeat.
func (b *readingLog) heightCall(sheet, digit, drawn int, verdict value.Verdict) *reading.CallRecord {
	c := b.call(sheet, "haiku-low", reading.NoKey, 1, verdict, "", false)
	c.Digit, c.Drawn = digit, drawn
	return c
}

// A height against 32 px, paired by sheet: at 16 px Haiku gives false alarms on five correct sheets
// that it read right at 32 px, and the test finds it worse; at 24 px it says "correct" on a sheet
// with a mistake where it said "not yet" at 32 px, a new false "correct". Neither is advised, so the
// advice is 32 px, and only the test part decides it. Sheets with small letters are told apart, and
// a crop drawn smaller than asked is counted.
func TestReadingHeights(t *testing.T) {
	b := newReadingLog(sample.OutcomeCorrect, sample.OutcomeCorrect, sample.OutcomeCorrect, sample.OutcomeCorrect, sample.OutcomeCorrect, sample.OutcomeError)
	b.l.Sheets[0].Markup.Parts[0].Text = "Ответ: 5 столбов"
	for sheet := 1; sheet <= 5; sheet++ {
		b.call(sheet, "haiku-low", reading.NoKey, 1, value.Correct, "5", false)
		b.heightCall(sheet, 16, 16, value.NotYet)
		b.heightCall(sheet, 24, 24, value.Correct)
	}
	b.call(6, "haiku-low", reading.NoKey, 1, value.NotYet, "6", false)
	b.heightCall(6, 16, 16, value.NotYet)
	b.heightCall(6, 24, 22, value.Correct)
	r := b.report(t)
	find := func(digit int, group string) report.Height {
		for _, h := range r.Heights {
			if h.Reader == "haiku-low" && h.Digit == digit && h.Group == group {
				return h
			}
		}
		t.Fatalf("no height %d of %s", digit, group)
		return report.Height{}
	}
	if h := find(16, report.AllSheets); h.Sheets != 6 || h.Errors != 5 || h.At32 != 0 || h.Test.Nonzero != 5 || h.NotWorse() {
		t.Errorf("16 px: %+v", h)
	}
	if h := find(24, report.AllSheets); h.NewFalseCorrect != 1 || h.Smaller != 1 || h.NotWorse() {
		t.Errorf("24 px: %+v", h)
	}
	if small, none := find(16, report.SmallLetters), find(16, report.NoSmall); small.Sheets != 1 || none.Sheets != 5 {
		t.Errorf("by small letters: %d and %d sheets", small.Sheets, none.Sheets)
	}
	if digit, decided, read := r.Advice(); digit != 32 || !decided || !read {
		t.Errorf("the advice %d, decided %v, read %v", digit, decided, read)
	}
	// At 24 px without the new false "correct", 24 px is advised; on the dev part it is not decided;
	// without heights there is no advice to give.
	b.l.Calls[len(b.l.Calls)-1].Verdict = string(value.NotYet)
	if digit, _, _ := b.report(t).Advice(); digit != 24 {
		t.Errorf("the advice %d, want 24", digit)
	}
	b.l.Run.Part = "dev"
	if _, decided, _ := b.report(t).Advice(); decided {
		t.Error("the dev part decides the advice")
	}
	// A reader that read some height but not 24 px leaves 24 px unadvised.
	other := b.call(1, "haiku-low-no-thinking", reading.NoKey, 1, value.Correct, "", false)
	other.Digit, other.Drawn = 48, 48
	if digit, _, _ := b.report(t).Advice(); digit != 32 {
		t.Errorf("the advice %d with a reader that did not read 24 px, want 32", digit)
	}
	if _, _, read := newReadingLog(sample.OutcomeCorrect).report(t).Advice(); read {
		t.Error("a run without heights advises one")
	}
}

// The states of a sheet, read without a key: a verdict before the answer is written is out of
// place, and so is a result then; a part only begun read as written whole is counted; the parts
// written whole are read or not; a state after a line and at a pause at once counts in both kinds;
// where the answer is written, the verdict is right or not.
func TestReadingStates(t *testing.T) {
	b := newReadingLog(sample.OutcomeCorrect, sample.OutcomeError)
	for i := range b.l.Sheets {
		b.l.Sheets[i].Markup.Parts = []reading.MarkedPart{{Kind: reading.KindText, Text: "2 + 3 = 5"}, {Kind: reading.KindText, Text: "Ответ: 5", Answer: true}}
		b.l.Sheets[i].States = []states.State{
			{Event: 3, Line: 1, Pause: 2, Parts: []states.Progress{states.Done, states.Begun}},
			{Event: 5, Erasure: 1, Parts: []states.Progress{states.Done, states.Done}},
		}
	}
	state := func(sheet, n int, verdict value.Verdict, answer string) {
		c := b.call(sheet, "haiku-low", reading.NoKey, 1, verdict, "", false)
		c.State, c.Answer = n, answer
	}
	both := `{"parts": [{"kind": "text", "text": "2+3=5", "legible": true, "other_readings": []}, {"kind": "text", "text": "Ответ: 5", "legible": true, "other_readings": []}], "results": [{"part": 2, "value": "5", "other_values": []}]}`
	none := `{"parts": [{"kind": "text", "text": "2 + 3 = 6", "legible": true, "other_readings": []}], "results": []}`
	state(1, 1, value.Correct, both)    // early, invented, the begun answer read as written whole
	state(2, 1, value.Unreadable, none) // in place; the written part misread
	state(1, 2, value.Correct, both)    // the answer written, right
	state(2, 2, value.Correct, both)    // the answer written, a false "correct"
	r := b.report(t)
	cells := map[string]report.StateCell{}
	for _, c := range r.States {
		cells[c.Kind] = c
	}
	for _, kind := range []string{report.AfterLine, report.AtPause} {
		c := cells[kind]
		if c.Calls != 2 || c.Early.Events != 1 || c.Early.Of != 2 || c.Invented != 1 || c.NotDone != 2 || c.BegunAsDone != 1 || c.Begun != 2 || c.DoneRead != 1 || c.DoneOf != 2 {
			t.Errorf("%s: %+v", kind, c)
		}
	}
	if c := cells[report.BeforeErasure]; c.Calls != 2 || c.AnswerDone != 2 || c.Right != 1 || c.Early.Of != 0 || c.DoneRead != 4 || c.DoneOf != 4 {
		t.Errorf("before an erasure: %+v", c)
	}
	// A state that still shows what is erased later gives no verdict to score; a failed call counts
	// in no share; a begun part is not read as written whole by the part that reads a part written
	// whole.
	b.l.Sheets[1].States[1].Erased = 2
	b.l.Calls[1].Error = "cut off"
	b.l.Sheets[0].Markup.Parts[1].Text = "2 + 3 = 5"
	cells = map[string]report.StateCell{}
	for _, c := range b.report(t).States {
		cells[c.Kind] = c
	}
	if c := cells[report.BeforeErasure]; c.AnswerDone != 1 || c.Erasing != 1 || c.Right != 1 {
		t.Errorf("before an erasure, with what is erased later: %+v", c)
	}
	if c := cells[report.AfterLine]; c.Failed != 1 || c.NotDone != 1 || c.Begun != 1 || c.BegunAsDone != 0 {
		t.Errorf("after a line, with a failed call: %+v", c)
	}
}

// A log whose states do not tell every part of the markup, or whose call names a state the sheet
// does not tell, makes no report.
func TestReadingRefusesStates(t *testing.T) {
	b := newReadingLog(sample.OutcomeCorrect)
	b.l.Sheets[0].States = []states.State{{Event: 1, Parts: []states.Progress{states.Done, states.Done}}}
	if _, err := report.NewReading(&b.l); err == nil {
		t.Error("a state of two parts on a markup of one: a report")
	}
	b = newReadingLog(sample.OutcomeCorrect)
	b.call(1, "haiku-low", reading.NoKey, 1, value.Correct, "5", false).State = 1
	if _, err := report.NewReading(&b.l); err == nil {
		t.Error("a call of a state the sheet does not tell: a report")
	}
}

// The readings of a sheet by each policy: the mean, the median and the most over the sheets, and
// their price by the mean cost of a call, of the states for a reader that read them and of the
// readings without a key for one that did not.
func TestReadingPolicies(t *testing.T) {
	b := newReadingLog(sample.OutcomeCorrect, sample.OutcomeCorrect, sample.OutcomeError)
	for i, p := range []states.Policies{{AfterLine: 2, Pause400: 9}, {AfterLine: 4, Pause400: 3, Pause6000: 1}, {AfterLine: 3, Pause2000: 2}} {
		b.l.Sheets[i].Policies = p
	}
	b.call(1, "opus-high", reading.NoKey, 1, value.Correct, "5", false).CostUSD = 0.02
	c := b.call(1, "haiku-low", reading.NoKey, 1, value.Correct, "5", false)
	c.State, c.CostUSD = 1, 0.001
	b.l.Sheets[0].States = []states.State{{Event: 1, Line: 1, Parts: []states.Progress{states.Done}}}
	r := b.report(t)
	if p := r.Policies[0]; p.Name != "after each line" || p.Mean != 3 || p.Median != 3 || p.Most != 4 {
		t.Errorf("after each line: %+v", p)
	}
	if p := r.Policies[1]; p.Mean != 4 || p.Median != 3 || p.Most != 9 {
		t.Errorf("at each pause of 0.4 s: %+v", p)
	}
	b.l.Sheets = append(b.l.Sheets, reading.SheetRecord{Kind: "sheet", Set: "s", Sheet: 4, Outcome: sample.OutcomeCorrect, Policies: states.Policies{AfterLine: 4}})
	if p := b.report(t).Policies[0]; p.Median != 3.5 {
		t.Errorf("the median of 2, 3, 4 and 4 readings is %v, want 3.5", p.Median)
	}
	if p := newReadingLog(sample.OutcomeCorrect).report(t).Policies; p != nil {
		t.Errorf("a log that tells no states nor readings by a policy has the policies %+v", p)
	}
	costs := map[string]report.Cost{}
	for _, c := range r.Costs {
		costs[c.Reader] = c
	}
	if c := costs["opus-high"]; c.PerCall != 0.02 || !strings.Contains(c.From, "without a key") {
		t.Errorf("Opus: %+v", c)
	}
	if c := costs["haiku-low"]; c.PerCall != 0.001 || c.From != "its states" {
		t.Errorf("Haiku: %+v", c)
	}
	if _, ok := costs["haiku-low-no-thinking"]; ok {
		t.Error("a reader that read nothing has a cost")
	}
}
