package wordsjudge_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/internal/correct"
	"github.com/altnovatrail/tutor/internal/hint"
	"github.com/altnovatrail/tutor/internal/runner"
	"github.com/altnovatrail/tutor/internal/wordsjudge"
)

// The words after a correct answer of a log are judged as their own request: each call that came,
// with its sheet's statement and its case's note and kind; the stub answers by the code's measures;
// the run's record names the words. A log of no words that came, or of a case it does not tell, is
// refused.
func TestOfCorrect(t *testing.T) {
	l := correctLog()
	r, err := wordsjudge.OfCorrect(l, "c.jsonl", "abc", opus())
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Steps) != 2 {
		t.Fatalf("%d steps", len(r.Steps))
	}
	req, err := r.Steps[1].Call.Request()
	if err != nil || !strings.Contains(req.Text, "Task: Вычисли 2 + 3.\nThe words after a correct answer.\n") || !strings.Contains(req.Text, "Note at the side: мне грустно\n") {
		t.Errorf("the request is %q, %v", req.Text, err)
	}
	var log bytes.Buffer
	if _, err := r.Do(context.Background(), runner.Stub, 10, &log, clock); err != nil {
		t.Fatal(err)
	}
	jl, err := wordsjudge.ReadLog(&log)
	if err != nil {
		t.Fatal(err)
	}
	first, second := jl.Calls[0], jl.Calls[1]
	if jl.Run.Words != correct.CallKind || jl.Run.File != "c.jsonl" || first.Case != "c1" || first.CaseKind != "work" || !first.Verdict.Gender || first.Verdict.MentionsNote ||
		second.CaseKind != "note" || second.NoteKind != hint.NoteSad || !second.Verdict.MentionsNote || second.Hint != "Здесь всё верно. Какой приём помог?" {
		t.Errorf("the log is %+v, %+v, %+v", jl.Run, first, second)
	}
	for name, change := range map[string]func(l *correct.Log){
		"no words came": func(l *correct.Log) { l.Calls = l.Calls[2:] },
		"a case untold": func(l *correct.Log) { l.Calls[0].Case = "c9" },
	} {
		l := correctLog()
		change(l)
		if _, err := wordsjudge.OfCorrect(l, "f", "s", opus()); err == nil {
			t.Errorf("%s: judged", name)
		}
	}
}

// correctLog is a log of the words after a correct answer: a case with the work whose words tell the
// sex, a case of a note taken up, and a call that failed.
func correctLog() *correct.Log {
	return &correct.Log{
		Run: correct.RunRecord{Kind: "run", CallKind: correct.CallKind, Cases: []correct.Case{
			{ID: "c1", Kind: correct.CaseWork, Sheet: 1, Lang: "ru", Work: true, Techniques: []string{}},
			{ID: "c2", Kind: correct.CaseNote, Sheet: 1, Lang: "ru", Work: true, Techniques: []string{}, Note: "мне грустно", NoteKind: hint.NoteSad},
		}},
		Sheets: []correct.SheetRecord{{Kind: "sheet", Set: "s", Sheet: 1, Statement: "Вычисли 2 + 3."}},
		Calls: []correct.CallRecord{
			{Kind: "call", Call: correct.Call{Set: "s", Sheet: 1, Lang: "ru", Tutor: "opus-high", Repeat: 1, Case: "c1"}, Words: "Ты верно нашёл ответ. Какой приём помог?", Gendered: []string{"нашёл"}},
			{Kind: "call", Call: correct.Call{Set: "s", Sheet: 1, Lang: "ru", Tutor: "opus-high", Repeat: 1, Case: "c2"}, Words: "Здесь всё верно. Какой приём помог?", NoteEcho: []string{"грустно"}},
			{Kind: "call", Call: correct.Call{Set: "s", Sheet: 1, Lang: "ru", Tutor: "opus-high", Repeat: 1, Case: "c1"}, Error: "overloaded"},
		},
	}
}
