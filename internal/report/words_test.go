package report_test

import (
	"bytes"
	"context"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/altnovatrail/tutor/internal/hint"
	"github.com/altnovatrail/tutor/internal/report"
	"github.com/altnovatrail/tutor/internal/runner"
	"github.com/altnovatrail/tutor/internal/wordsjudge"
)

// wordsStub is a stub run of the set of the words (T118) on four of its sheets, two with a note, by
// Opus, read back: every case of those sheets.
func wordsStub(t *testing.T) (*hint.Log, []byte) {
	t.Helper()
	numbers := []int{1, 2, 31, 32}
	chosen := chosenOf(t, "words-v1", "../../eval/words-sheets.json", "../../eval/lines-gen-1-bank.json", numbers...)
	f, err := os.Open("../../eval/words-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	cs, err := hint.ReadCases(f)
	if err != nil {
		t.Fatal(err)
	}
	cs.Cases = slices.DeleteFunc(cs.Cases, func(k hint.Case) bool { return !slices.Contains(numbers, k.Sheet) })
	r, err := hint.New("all", []string{"words-v1"}, chosen, hint.Tutors()[:1], 1, &hint.Set{File: "words-cases.json", SHA256: "9df2b1a2a1f6", Cases: cs})
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	if _, err := r.Do(context.Background(), runner.Stub, 10, &log, clock); err != nil {
		t.Fatal(err)
	}
	l, err := hint.ReadLog(bytes.NewReader(log.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	return l, log.Bytes()
}

func clock() time.Time { return time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC) }

// A stub run of the set of the words goes to the report of the hint with its section of the set,
// and its judging on the stub to the report of the judge, the Markdown, the CSV and the file of the
// hints flagged, byte for byte the golden ones (testdata/words-stub.*). On the stub nothing is
// repeated, boxed without the work, taken up of a note or told of the sex, and the judge flags
// nothing.
func TestWordsOfTheStub(t *testing.T) {
	l, data := wordsStub(t)
	rep, err := report.NewHint(l)
	if err != nil {
		t.Fatal(err)
	}
	var md bytes.Buffer
	if err := rep.Markdown(&md, "words-stub.jsonl"); err != nil {
		t.Fatal(err)
	}
	cells := rep.Words()
	if len(cells) != 7 {
		t.Fatalf("%d cells", len(cells))
	}
	for _, c := range cells {
		if c.Calls == 0 && c.Kind != hint.CaseNote || c.Repeated.Events+c.BoxNoWork+c.Echo.Events+c.Gendered.Events+c.Leaked.Events+c.Broken.Events != 0 {
			t.Errorf("on the stub, %+v", c)
		}
	}

	r, err := wordsjudge.Of(l, "words-stub.jsonl", "abc", runner.Configs()[0])
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
	var jmd, jcsv, flagged bytes.Buffer
	if err := j.Markdown(&jmd, "words-stub.judge.jsonl"); err != nil {
		t.Fatal(err)
	}
	if err := j.CSV(&jcsv); err != nil {
		t.Fatal(err)
	}
	suffix, _, write := j.Readable()
	if err := write(&flagged, "words-stub.judge.jsonl"); err != nil {
		t.Fatal(err)
	}
	goldens(t, map[string][]byte{"testdata/words-stub.jsonl": data, "testdata/words-stub.md": md.Bytes(), "testdata/words-stub.judge.jsonl": log.Bytes(),
		"testdata/words-stub.judge.md": jmd.Bytes(), "testdata/words-stub.judge.csv": jcsv.Bytes(), "testdata/words-stub.judge" + suffix: flagged.Bytes()})
	for _, c := range j.Cells {
		if c.Flagged.Events != 0 || c.Failed != 0 || c.Disagree != 0 {
			t.Errorf("the judge on the stub: %+v", c)
		}
	}
	if !strings.Contains(j.Summary(), "opus-high: 30 hints judged") || !strings.Contains(flagged.String(), "None.") {
		t.Errorf("the summary is %q", j.Summary())
	}
}

// What the code and the judge flag the reports count: a hint that repeats a question, boxes
// without the work, takes the note up and tells the sex, and one the judge leaves alone but the
// rough search flags; a call without a verdict.
func TestWordsFlagged(t *testing.T) {
	l, _ := wordsStub(t)
	cases := map[string]hint.Case{}
	for _, k := range l.Run.Cases {
		cases[k.ID] = k
	}
	var otherwise, note int
	for i, c := range l.Calls {
		switch cases[c.Case].Kind {
		case hint.CaseOtherwise:
			otherwise = i
		case hint.CaseNote:
			note = i
		}
	}
	o := &l.Calls[otherwise]
	o.Repeated, o.Box, o.Gendered = cases[o.Case].Asked[:1], l.Calls[0].Box, []string{"нашёл"}
	n := &l.Calls[note]
	n.NoteEcho = []string{"меня никто не"}
	rep, err := report.NewHint(l)
	if err != nil {
		t.Fatal(err)
	}
	all := rep.Words()[6]
	if !all.All() || all.Repeated.Events != 1 || all.BoxNoWork != 1 || all.Echo.Events != 1 || all.Gendered.Events != 1 {
		t.Errorf("the cell of all is %+v", all)
	}

	r, err := wordsjudge.Of(l, "f", "s", runner.Configs()[0])
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
	// The judge finds the sex not told where the rough search does, and a hint off the task; one call
	// has no verdict.
	for i := range jl.Calls {
		c := &jl.Calls[i]
		switch {
		case len(c.Gendered) > 0:
			c.Verdict.Gender = false
		case i == 0:
			c.Verdict.OnTask, c.Verdict.Why = false, "it speaks of the weather"
		case i == 1:
			c.Verdict, c.Error = nil, "overloaded"
		}
	}
	j, err := report.NewWordsJudge(jl)
	if err != nil {
		t.Fatal(err)
	}
	var cell report.JudgeCell
	for _, c := range j.Cells {
		if c.All() {
			cell = c
		}
	}
	if cell.Failed != 1 || cell.OffTask.Events != 1 || cell.MentionsNote.Events != 1 || cell.Rough != 1 || cell.Disagree != 1 || cell.Flagged.Events != 2 {
		t.Errorf("the cell of all is %+v", cell)
	}
	var flagged bytes.Buffer
	_, _, write := j.Readable()
	if err := write(&flagged, "f"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"it speaks of the weather", "no verdict: overloaded", "rough search: нашёл", "takes up the note: меня никто не", "- note: «"} {
		if !strings.Contains(flagged.String(), want) {
			t.Errorf("the file of the flagged lacks %q:\n%s", want, flagged.String())
		}
	}
}

// The report of the control phrases says whether the judge gave every answer they expect, and
// names those it did not; a log of the judge of no calls, of controls without their answers or of
// something else is refused.
func TestWordsJudgeControls(t *testing.T) {
	f, err := os.Open("../../eval/words-judge.json")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	c, err := wordsjudge.ReadControls(f)
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	if _, err := wordsjudge.Check(c, "words-judge.json", "abc", runner.Configs()[0]).Do(context.Background(), runner.Stub, 10, &log, clock); err != nil {
		t.Fatal(err)
	}
	read := func() *wordsjudge.Log {
		l, err := wordsjudge.ReadLog(bytes.NewReader(log.Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		return l
	}
	j, err := report.NewWordsJudge(read())
	if err != nil {
		t.Fatal(err)
	}
	var md, csv bytes.Buffer
	if err := j.Markdown(&md, "controls.jsonl"); err != nil {
		t.Fatal(err)
	}
	if err := j.CSV(&csv); err != nil {
		t.Fatal(err)
	}
	if j.Right != len(c.Controls) || len(j.Wrong) != 0 || !strings.Contains(md.String(), "its verdicts count") || !strings.HasPrefix(j.Summary(), "The judge is right on all") ||
		strings.Count(csv.String(), ",yes,") != len(c.Controls) {
		t.Errorf("the controls on the stub: %d right, %v wrong\n%s", j.Right, j.Wrong, md.String())
	}

	l := read()
	l.Calls[0].Verdict.Gender = !l.Calls[0].Verdict.Gender
	l.Calls[1].Verdict, l.Calls[1].Error = nil, "overloaded"
	j, err = report.NewWordsJudge(l)
	if err != nil {
		t.Fatal(err)
	}
	md.Reset()
	csv.Reset()
	var wrong bytes.Buffer
	_, _, write := j.Readable()
	if err := j.Markdown(&md, "controls.jsonl"); err != nil {
		t.Fatal(err)
	}
	if err := j.CSV(&csv); err != nil {
		t.Fatal(err)
	}
	if err := write(&wrong, "controls.jsonl"); err != nil {
		t.Fatal(err)
	}
	ids := []string{c.Controls[0].ID, c.Controls[1].ID}
	if !slices.Equal(j.Wrong, ids) || !strings.Contains(md.String(), "measure nothing") || !strings.Contains(j.Summary(), "wrong on "+strings.Join(ids, ", ")) ||
		strings.Count(csv.String(), ",no,") != 2 || !strings.Contains(wrong.String(), "## "+ids[0]) || !strings.Contains(wrong.String(), "- expected: ") {
		t.Errorf("two controls wrong: %v\n%s\n%s", j.Wrong, md.String(), wrong.String())
	}

	for name, change := range map[string]func(l *wordsjudge.Log){
		"no calls":   func(l *wordsjudge.Log) { l.Calls = nil },
		"no answers": func(l *wordsjudge.Log) { l.Calls[0].Expect = nil },
		"of nothing": func(l *wordsjudge.Log) { l.Run.Of = "sheets" },
	} {
		l := read()
		change(l)
		if _, err := report.NewWordsJudge(l); err == nil {
			t.Errorf("%s: reported", name)
		}
	}
}
