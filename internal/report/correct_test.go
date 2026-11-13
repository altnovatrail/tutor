package report_test

import (
	"bytes"
	"context"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/admission"
	"github.com/altnovatrail/tutor/internal/correct"
	"github.com/altnovatrail/tutor/internal/hint"
	"github.com/altnovatrail/tutor/internal/report"
	"github.com/altnovatrail/tutor/internal/runner"
	"github.com/altnovatrail/tutor/internal/wordsjudge"
	"github.com/altnovatrail/tutor/words"
)

// correctStub is a stub run of the set of the words after a correct answer (T143) on four of its
// sheets, two with a note, by Opus, read back: every case of those sheets.
func correctStub(t *testing.T) (*correct.Log, []byte) {
	t.Helper()
	numbers := []int{1, 2, 31, 32}
	chosen := chosenOf(t, "correct-v1", "../../eval/correct-sheets.json", "../../eval/lines-gen-1-bank.json", numbers...)
	f, err := os.Open("../../eval/correct-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	cs, err := correct.ReadCases(f)
	if err != nil {
		t.Fatal(err)
	}
	cs.Cases = slices.DeleteFunc(cs.Cases, func(k correct.Case) bool { return !slices.Contains(numbers, k.Sheet) })
	r, err := correct.New("all", "correct-v1", chosen, hint.Tutors()[:1], 1, &correct.Set{File: "correct-cases.json", SHA256: "5c1d8e", Cases: cs})
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	if _, err := r.Do(context.Background(), runner.Stub, 10, &log, clock); err != nil {
		t.Fatal(err)
	}
	l, err := correct.ReadLog(bytes.NewReader(log.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	return l, log.Bytes()
}

// A stub run of the set of the words after a correct answer goes to its report, the Markdown, the
// CSV and the file of the words to read, and its judging on the stub to the report of the judge, byte
// for byte the golden ones (testdata/correct-stub.*). On the stub nothing leaks, breaks its form,
// takes up a note or tells the sex, and the judge flags nothing.
func TestCorrectOfTheStub(t *testing.T) {
	l, data := correctStub(t)
	rep, err := report.NewCorrect(l)
	if err != nil {
		t.Fatal(err)
	}
	var md, csv bytes.Buffer
	if err := rep.Markdown(&md, "correct-stub.jsonl"); err != nil {
		t.Fatal(err)
	}
	if err := rep.CSV(&csv); err != nil {
		t.Fatal(err)
	}
	if len(rep.Cells) != 7 {
		t.Fatalf("%d cells", len(rep.Cells))
	}
	for _, c := range rep.Cells {
		if c.Calls == 0 || c.Came != c.Calls || c.Leaked.Events+c.Broken.Events+c.Withheld.Events+c.Echo.Events+c.Gendered.Events != 0 {
			t.Errorf("on the stub, %+v", c)
		}
	}
	if s := rep.Summary(); !strings.Contains(s, "opus-high: leaked before the check 0 of 10") {
		t.Errorf("the summary is %q", s)
	}

	r, err := wordsjudge.OfCorrect(l, "correct-stub.jsonl", "abc", runner.Configs()[0])
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	if _, err := r.Do(context.Background(), runner.Stub, 10, &log, clock); err != nil {
		t.Fatal(err)
	}
	jl, err := wordsjudge.ReadLog(bytes.NewReader(log.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	j, err := report.NewWordsJudge(jl)
	if err != nil {
		t.Fatal(err)
	}
	var jmd bytes.Buffer
	if err := j.Markdown(&jmd, "correct-stub.judge.jsonl"); err != nil {
		t.Fatal(err)
	}
	goldens(t, map[string][]byte{"testdata/correct-stub.jsonl": data, "testdata/correct-stub.md": md.Bytes(), "testdata/correct-stub.csv": csv.Bytes(),
		"testdata/correct-stub.judge.md": jmd.Bytes()})
	for _, c := range j.Cells {
		if c.Flagged.Events != 0 || c.Failed != 0 {
			t.Errorf("the judge on the stub: %+v", c)
		}
	}
	if jl.Run.Words != correct.CallKind || !strings.Contains(j.Summary(), "opus-high: 10 words judged") {
		t.Errorf("the judge's log is of %q, its summary %q", jl.Run.Words, j.Summary())
	}
	// Words the judge flags are told by their case, with no rung.
	jl.Calls[0].Verdict.Gender = true
	if j, err = report.NewWordsJudge(jl); err != nil {
		t.Fatal(err)
	}
	var flagged bytes.Buffer
	_, _, write := j.Readable()
	if err := write(&flagged, "f"); err != nil {
		t.Fatal(err)
	}
	if s := flagged.String(); !strings.Contains(s, "\n## "+jl.Calls[0].Case+", opus-high, work, ru, sheet 1 of correct-v1, repeat 1\n") || !strings.Contains(s, "- words: «") || strings.Contains(s, "rung") {
		t.Errorf("the words flagged are told\n%s", s)
	}
}

// What the code flags the report counts: words that leak, break their form or are withheld for it,
// take the note up and tell the sex; a call that failed. A log that is not whole is refused.
func TestCorrectFlagged(t *testing.T) {
	l, _ := correctStub(t)
	cases := map[string]correct.CaseKind{}
	for _, k := range l.Run.Cases {
		cases[k.ID] = k.Kind
	}
	note := slices.IndexFunc(l.Calls, func(c correct.CallRecord) bool { return cases[c.Case] == correct.CaseNote })
	l.Calls[0].LeakedKeys, l.Calls[0].Withheld, l.Calls[0].Shown = []string{"61"}, correct.WithheldLeak, ""
	l.Calls[1].Form, l.Calls[1].Withheld, l.Calls[1].Shown = []string{string(words.Last), string(words.Words)}, correct.WithheldForm, ""
	l.Calls[2].Gendered = []string{"нашёл"}
	l.Calls[3].Error, l.Calls[3].Withheld, l.Calls[3].Words, l.Calls[3].Shown = "overloaded", correct.WithheldFailed, "", ""
	l.Calls[note].NoteEcho = []string{"меня никто не"}
	rep, err := report.NewCorrect(l)
	if err != nil {
		t.Fatal(err)
	}
	all := rep.Cells[len(rep.Cells)-1]
	if !all.All() || all.Failed != 1 || all.Leaked.Events != 1 || all.Key != 1 || all.Broken.Events != 1 || all.Last != 1 || all.Words != 1 ||
		all.Withheld.Events != 3 || all.Gendered.Events != 1 || all.Echo.Events != 1 {
		t.Errorf("the cell of all is %+v", all)
	}
	var md bytes.Buffer
	if err := rep.Markdown(&md, "f"); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(l *correct.Log){
		"no tutors":       func(l *correct.Log) { l.Run.Tutors = nil },
		"a sheet twice":   func(l *correct.Log) { l.Sheets = append(l.Sheets, l.Sheets[0]) },
		"a sheet unknown": func(l *correct.Log) { l.Calls[0].Sheet = 99 },
		"a tutor unknown": func(l *correct.Log) { l.Calls[0].Tutor = "gpt" },
		"a case unknown":  func(l *correct.Log) { l.Calls[0].Case = "c999" },
	} {
		l, _ := correctStub(t)
		change(l)
		if _, err := report.NewCorrect(l); err == nil {
			t.Errorf("%s: reported", name)
		}
	}
}

// admittedCorrect applies the rule to a log of the words after a correct answer and its judge's
// log, each changed as the test asks.
func admittedCorrect(t *testing.T, change func(c *wordsjudge.Log, l *correct.Log, j *wordsjudge.Log)) report.WordsDecision {
	t.Helper()
	l, _ := correctStub(t)
	r, err := wordsjudge.OfCorrect(l, "correct.jsonl", "abc", judgeConfig())
	if err != nil {
		t.Fatal(err)
	}
	j, c := judged(t, r), controlsLog(t)
	change(c, l, j)
	lr, err := report.NewCorrect(l)
	if err != nil {
		t.Fatal(err)
	}
	jr, err := report.NewWordsJudge(j)
	if err != nil {
		t.Fatal(err)
	}
	cr, err := report.NewWordsJudge(c)
	if err != nil {
		t.Fatal(err)
	}
	return report.AdmitCorrect(cr, []report.CorrectSet{{Name: "correct.jsonl", SHA256: "abc", Correct: lr, Judge: jr}})
}

// On the stub the rule admits the words after a correct answer at the stage author, with every count
// it reads; each target that fails, and each log that is not what the rule reads, leaves the tutor
// out, and says why.
func TestAdmitCorrect(t *testing.T) {
	d := admittedCorrect(t, func(*wordsjudge.Log, *correct.Log, *wordsjudge.Log) {})
	if d.Stage != admission.StageAuthor || len(d.Why) != 0 || d.Tutor != "opus-high" || d.Came != 10 || d.Note.Of != 4 || d.Repeated.Of != 0 || d.ControlsRight != d.Controls {
		t.Errorf("the decision is %+v", d)
	}
	for name, c := range map[string]struct {
		change func(c *wordsjudge.Log, l *correct.Log, j *wordsjudge.Log)
		says   string
	}{
		"another prompt": {func(_ *wordsjudge.Log, l *correct.Log, _ *wordsjudge.Log) { l.Run.Prompt = "d8a2f994" }, "not of the words after a correct answer the library asks"},
		"two tutors": {func(_ *wordsjudge.Log, l *correct.Log, _ *wordsjudge.Log) {
			l.Run.Tutors = append(l.Run.Tutors, l.Run.Tutors[0])
		}, "not of the one tutor"},
		"cases not asked": {func(_ *wordsjudge.Log, l *correct.Log, _ *wordsjudge.Log) {
			l.Run.Cases = append(l.Run.Cases, correct.Case{ID: "c999"})
		}, "1 cases planned were asked by none"},
		"a call failed": {func(_ *wordsjudge.Log, l *correct.Log, j *wordsjudge.Log) {
			l.Calls[0].Error, l.Calls[0].Withheld = "overloaded", correct.WithheldFailed
			j.Calls = j.Calls[1:]
		}, "1 calls failed, and their words"},
		"no words came": {func(_ *wordsjudge.Log, l *correct.Log, j *wordsjudge.Log) {
			for i := range l.Calls {
				l.Calls[i].Error = "overloaded"
			}
			j.Calls = j.Calls[:1]
		}, "no words came"},
		"words not judged": {func(_ *wordsjudge.Log, _ *correct.Log, j *wordsjudge.Log) {
			j.Calls[0].Verdict, j.Calls[0].Error = nil, "overloaded"
		}, "judged 9 of the 10 words"},
		"the sex told": {func(_ *wordsjudge.Log, _ *correct.Log, j *wordsjudge.Log) { j.Calls[0].Verdict.Gender = true }, "words tell the learner's sex"},
		"the note echoed": {func(_ *wordsjudge.Log, l *correct.Log, _ *wordsjudge.Log) {
			cases := map[string]correct.CaseKind{}
			for _, k := range l.Run.Cases {
				cases[k.ID] = k.Kind
			}
			l.Calls[slices.IndexFunc(l.Calls, func(c correct.CallRecord) bool { return cases[c.Case] == correct.CaseNote })].NoteEcho = []string{"000"}
		}, "take the note up, by the code"},
		"leaks":           {func(_ *wordsjudge.Log, l *correct.Log, _ *wordsjudge.Log) { l.Calls[0].LeakedKeys = []string{"61"} }, "words leaked before the check"},
		"the form broken": {func(_ *wordsjudge.Log, l *correct.Log, _ *wordsjudge.Log) { l.Calls[0].Form = []string{"last"} }, "broke the form"},
	} {
		d := admittedCorrect(t, c.change)
		if d.Stage != "" || !slices.ContainsFunc(d.Why, func(w string) bool { return strings.Contains(w, c.says) }) {
			t.Errorf("%s: %+v, %q", name, d.Stage, d.Why)
		}
	}
	cr, err := report.NewWordsJudge(controlsLog(t))
	if err != nil {
		t.Fatal(err)
	}
	if d := report.AdmitCorrect(cr, nil); d.Stage != "" || !slices.Contains(d.Why, "no set of the words after a correct answer is given") {
		t.Errorf("no set: %+v", d)
	}
}
