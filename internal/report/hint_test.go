package report_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/altnovatrail/tutor/internal/hint"
	"github.com/altnovatrail/tutor/internal/judge"
	"github.com/altnovatrail/tutor/internal/report"
	"github.com/altnovatrail/tutor/internal/runner"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/sheets"
)

// hintLog is a log of the hint built by hand: two tutors, rung 1 in both languages, and two sheets
// of the set "s": sheet 1 of task "a", whose key 2 the check searches for, with the trap "t1" and the
// first error in part 2; sheet 2 of task "b", whose key its statement shows, with no trap and the
// first error in part 1.
func hintLog() *hint.Log {
	l := &hint.Log{Run: hint.RunRecord{Kind: "run", CallKind: hint.CallKind, Part: "test", Sets: []string{"s"}, Repeats: 1, Rungs: []hint.Rung{1},
		Langs: []sample.Lang{sample.LangRussian, sample.LangEnglish}, MaxUSD: 5, Prompt: strings.Repeat("b", 64), PromptVersion: 1, Checked: true,
		Tutors: []hint.TutorRecord{{Name: "opus-high", Model: "claude-opus-5-5", Effort: "high"}, {Name: "haiku-low", Model: "claude-haiku-5-5", Effort: "low"}}},
		Sheets: []hint.SheetRecord{
			{Kind: "sheet", Set: "s", Sheet: 1, File: "01.json", Task: "a", Scenario: sample.ScenarioError, Outcome: sample.OutcomeError, FirstError: 2,
				FirstErrorText: "3 + 4 = 8", Trap: "t1", Key: "x = 2", Statement: "Реши.", SearchedKeys: []string{"2"}, Phrases: []string{"phrase"}},
			{Kind: "sheet", Set: "s", Sheet: 2, File: "02.json", Task: "b", Scenario: sample.ScenarioError, Outcome: sample.OutcomeError, FirstError: 1,
				FirstErrorText: "5 − 1", Key: "5", Statement: "Через 5 м.", Phrases: []string{"p"}},
		},
		End: &runner.End{Kind: "end"}}
	box := &judge.Box{X1: 1, Y1: 1, X2: 9, Y2: 9}
	call := func(sheet int, lang sample.Lang, tutor string) hint.CallRecord {
		return hint.CallRecord{Kind: "call", Call: hint.Call{Set: "s", Sheet: sheet, Rung: 1, Lang: lang, Tutor: tutor, Repeat: 1},
			Usage: runner.Usage{LatencyMs: 1000, CostUSD: 0.01}}
	}
	// opus-high: on sheet 1 a hint that gives the key away and one that breaks its form; on sheet 2 a
	// failed call and a hint that passes, boxed on the first error.
	c := call(1, sample.LangRussian, "opus-high")
	c.Box, c.Hit, c.Trap, c.Hint, c.LeakedKeys, c.Fallback, c.Shown = box, 2, "t1", "Ответ 2?", []string{"2"}, hint.FallbackLeak, hint.Fallback(1, sample.LangRussian)
	l.Calls = append(l.Calls, c)
	c = call(1, sample.LangEnglish, "opus-high")
	c.Hint, c.Form, c.Fallback, c.Shown = "Look. Count. Again. Now?", []string{hint.BreachSentences, hint.BreachWords}, hint.FallbackForm, hint.Fallback(1, sample.LangEnglish)
	l.Calls = append(l.Calls, c)
	c = call(2, sample.LangRussian, "opus-high")
	c.Error, c.Fallback, c.Shown = "timeout", hint.FallbackFailed, hint.Fallback(1, sample.LangRussian)
	l.Calls = append(l.Calls, c)
	c = call(2, sample.LangEnglish, "opus-high")
	c.Box, c.Hit, c.Hint, c.Shown = box, 1, "Where does the first step go?", "Where does the first step go?"
	l.Calls = append(l.Calls, c)
	// haiku-low: on sheet 1 a hint with the key and the phrase, boxed on no part; a call not asked.
	c = call(1, sample.LangRussian, "haiku-low")
	c.Box, c.Hint, c.LeakedKeys, c.LeakedPhrases, c.Fallback, c.Shown = box, "phrase: 2?", []string{"2"}, []string{"phrase"}, hint.FallbackLeak, hint.Fallback(1, sample.LangRussian)
	l.Calls = append(l.Calls, c)
	c = call(1, sample.LangEnglish, "haiku-low")
	c.NotAsked, c.Error = true, "no request"
	l.Calls = append(l.Calls, c)
	return l
}

// cellOf is the cell of all the calls of a tutor.
func cellOf(t *testing.T, r *report.Hint, tutor string) report.HintCell {
	t.Helper()
	for _, c := range r.Cells {
		if c.Tutor == tutor && c.All() {
			return c
		}
	}
	t.Fatalf("no cell of %s", tutor)
	return report.HintCell{}
}

// The cells on known numbers: the leaks by key and phrase, the breaches of form, the fallback texts
// by cause, the box on the first error and the trap against the sheet's; the leaks by task; and the
// hints to read, the caught ones first.
func TestHint(t *testing.T) {
	r, err := report.NewHint(hintLog())
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Cells) != 2*(2+1) {
		t.Fatalf("%d cells", len(r.Cells))
	}
	opus := cellOf(t, r, "opus-high")
	if opus.Calls != 4 || opus.Came != 3 || opus.Leaked.Events != 1 || opus.Leaked.Of != 3 || opus.Key != 1 || opus.Phrase != 0 || opus.After != 0 ||
		opus.Broken.Events != 1 || opus.Sentences != 1 || opus.Words != 1 || opus.Questions != 0 || opus.Script != 0 ||
		opus.FellBack.Events != 3 || opus.FellBack.Of != 4 || opus.Failed != 1 || opus.Leak != 1 || opus.Form != 1 ||
		opus.OnError.Events != 2 || opus.NoBox != 1 || opus.TrapRight != 1 || opus.TrapOf != 2 || opus.NoTrapRight != 1 || opus.NoTrapOf != 1 ||
		opus.TimeP50 != 1000 || opus.MeanUSD != 0.01 {
		t.Errorf("opus-high: %+v", opus)
	}
	haiku := cellOf(t, r, "haiku-low")
	if haiku.Calls != 1 || haiku.Came != 1 || haiku.Leaked.Events != 1 || haiku.Key != 1 || haiku.Phrase != 1 || haiku.Leak != 1 || haiku.OnError.Events != 0 || haiku.NoBox != 0 {
		t.Errorf("haiku-low: %+v", haiku)
	}
	if c := r.Cells[1]; c.Tutor != "opus-high" || c.Lang != sample.LangEnglish || c.Rung != 1 || c.Calls != 2 || c.Form != 1 || c.Failed != 0 || c.OnError.Events != 1 {
		t.Errorf("opus-high in English at rung 1: %+v", c)
	}
	if len(r.Tasks) != 2 || r.Tasks[0].Task != "a" || !r.Tasks[0].KeySearched || r.Tasks[0].Came != 3 || r.Tasks[0].Leaked != 2 ||
		len(r.Tasks[0].Found) != 2 || r.Tasks[0].Found[0] != (report.Found{Text: "2", Key: true, Hints: 2}) || r.Tasks[1].KeySearched || r.Tasks[1].Came != 1 {
		t.Errorf("the tasks: %+v", r.Tasks)
	}
	var read []string
	for _, c := range r.ToRead() {
		read = append(read, c.Tutor+"/"+string(c.Lang)+"/"+strings.Repeat("·", c.Sheet))
	}
	if strings.Join(read, " ") != "opus-high/ru/· haiku-low/ru/· opus-high/en/··" {
		t.Errorf("the hints to read: %v", read)
	}
	var md, hints bytes.Buffer
	if err := r.Markdown(&md, "t.jsonl"); err != nil {
		t.Fatal(err)
	}
	if err := r.Hints(&hints, "t.jsonl"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# The hint: the test part", "opus-high (claude-opus-5-5, high)", "| a | yes | 3 | 2 | the key «2» 2, the phrase «phrase» 1 |", "| b | no | 1 | 0 |  |", "Calls: 5 made, 1 of them failed"} {
		if !strings.Contains(md.String(), want) {
			t.Errorf("the report lacks %q:\n%s", want, md.String())
		}
	}
	for _, want := range []string{"3 hints of the first repeat", "## 2. s/1 · rung 1 · ru · haiku-low", "the fallback text, for leak", "its box on no part", "The learner reads the hint.", "its box on the first error"} {
		if !strings.Contains(hints.String(), want) {
			t.Errorf("the hints lack %q:\n%s", want, hints.String())
		}
	}
	if s := r.Summary(); !strings.Contains(s, "opus-high: leaked before the check 1 of 3, after it 0; form broken 1 of 3, by R70 spec 1; fallback 3 of 4; box on the first error 2 of 3.") {
		t.Errorf("the summary:\n%s", s)
	}
}

// The hints to read tell a box on another part and no box, and the report tells a log without an
// end and a run that stopped short.
func TestHintEnds(t *testing.T) {
	l := hintLog()
	l.Calls[3].Hit = 3
	c := &l.Calls[5]
	c.NotAsked, c.Error, c.Hint, c.Shown = false, "", "Где ошибка?", "Где ошибка?"
	l.End = nil
	r, err := report.NewHint(l)
	if err != nil {
		t.Fatal(err)
	}
	var md, hints bytes.Buffer
	if err := r.Markdown(&md, "t.jsonl"); err != nil {
		t.Fatal(err)
	}
	if err := r.Hints(&hints, "t.jsonl"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(md.String(), "The log has no end") || !strings.Contains(hints.String(), "its box on part 3") || !strings.Contains(hints.String(), "its box none") {
		t.Errorf("the report:\n%s\nthe hints:\n%s", md.String(), hints.String())
	}
	l.End = &runner.End{Kind: "end", Result: runner.Result{Stopped: "the cap"}}
	md.Reset()
	if err := r.Markdown(&md, "t.jsonl"); err != nil || !strings.Contains(md.String(), "The run stopped short, by the cap") {
		t.Errorf("a run stopped short: %v\n%s", err, md.String())
	}
}

// A log without tutors or sheets, of a run that did not check its hints, with a sheet twice, or
// with a call of a sheet or a tutor it does not tell is refused.
func TestHintRefused(t *testing.T) {
	for name, broken := range map[string]func(l *hint.Log){
		"no tutors":     func(l *hint.Log) { l.Run.Tutors = nil },
		"no sheets":     func(l *hint.Log) { l.Sheets = nil },
		"a sheet twice": func(l *hint.Log) { l.Sheets = append(l.Sheets, l.Sheets[0]) },
		"a sheet":       func(l *hint.Log) { l.Calls[0].Sheet = 9 },
		"a tutor":       func(l *hint.Log) { l.Calls[0].Tutor = "fable" },
		"no checks":     func(l *hint.Log) { l.Run.Checked = false },
	} {
		l := hintLog()
		broken(l)
		if _, err := report.NewHint(l); err == nil {
			t.Errorf("a log with %s is reported", name)
		}
	}
}

// A stub run of the hint over sheets with a mistake of the three sets goes to a report, the
// Markdown, the CSV and the hints to read, byte for byte the golden ones (testdata/hint-stub.*);
// the log itself is golden too. On the stub nothing leaks, every hint keeps its form, every box is
// on the first error and every trap the sheet's.
func TestHintLogOfTheStub(t *testing.T) {
	var chosen []sheets.Chosen
	for _, set := range []struct {
		name, plan, bank string
		numbers          []int
	}{
		{"lines", "../../eval/sheets.json", "../../eval/bank.json", []int{3, 9}},
		{"free", "../../eval/free-sheets.json", "../../eval/free-bank.json", []int{2, 8, 12}},
		{"near", "../../eval/near-sheets.json", "../../eval/near-bank.json", []int{12}},
	} {
		chosen = append(chosen, chosenOf(t, set.name, set.plan, set.bank, set.numbers...)...)
	}
	r, err := hint.New("dev", []string{"lines", "free", "near"}, chosen, runner.E4(hint.Tutors(), func(tu hint.Tutor) runner.Config { return tu.Config }), 1)
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	clock := func() time.Time { return time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC) }
	if _, err := r.Do(context.Background(), runner.Stub, 10, &log, clock); err != nil {
		t.Fatal(err)
	}
	l, err := hint.ReadLog(bytes.NewReader(log.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	rep, err := report.NewHint(l)
	if err != nil {
		t.Fatal(err)
	}
	var md, csv, hints bytes.Buffer
	if err := rep.Markdown(&md, "hint-stub.jsonl"); err != nil {
		t.Fatal(err)
	}
	if err := rep.CSV(&csv); err != nil {
		t.Fatal(err)
	}
	if err := rep.Hints(&hints, "hint-stub.jsonl"); err != nil {
		t.Fatal(err)
	}
	goldens(t, map[string][]byte{"testdata/hint-stub.jsonl": log.Bytes(), "testdata/hint-stub.md": md.Bytes(), "testdata/hint-stub.csv": csv.Bytes(),
		"testdata/hint-stub.hints.md": hints.Bytes()})
	for _, c := range rep.Cells {
		if c.Calls == 0 || c.Leaked.Events != 0 || c.Broken.Events != 0 || c.FellBack.Events != 0 || c.OnError.Events != c.Came || c.TrapRight != c.TrapOf || c.NoTrapRight != c.NoTrapOf {
			t.Errorf("on the stub, %+v", c)
		}
	}
	if n := len(rep.ToRead()); n != report.HintsToRead {
		t.Errorf("%d hints to read", n)
	}
}
