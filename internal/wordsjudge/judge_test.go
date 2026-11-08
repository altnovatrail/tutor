package wordsjudge_test

import (
	"bytes"
	"context"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/altnovatrail/tutor/internal/hint"
	"github.com/altnovatrail/tutor/internal/runner"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/wordsjudge"
)

func clock() time.Time { return time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC) }

func opus() wordsjudge.Judge {
	return runner.Configs()[0]
}

// The judge's prompt is frozen before its first paid run (T119): a change of a word is a new
// version, whose control phrases run again.
func TestFrozenV1(t *testing.T) {
	const frozen = "8e4663bc66b22daab84670721cd982ac3d1a59faa4803139f90b8a1b4326ae81"
	if got := wordsjudge.Fingerprint(); got != frozen || wordsjudge.Version != 1 {
		t.Errorf("the fingerprint of version %d is %s, not the one frozen, %s", wordsjudge.Version, got, frozen)
	}
}

// A request is text alone: the task, the rung, the language, the note or none, and the hint in its
// block. A hint of no words is not asked.
func TestRequest(t *testing.T) {
	r, err := wordsjudge.Request(wordsjudge.Item{Hint: "Где ошибка?", Lang: "ru", Rung: 2, Statement: "Вычисли 2 + 3.", Note: "мне грустно"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Text != "Task: Вычисли 2 + 3.\nRung: 2.\nLanguage: ru.\nNote at the side: мне грустно\n<hint>\nГде ошибка?\n</hint>" || !r.NoPicture || len(r.Images) != 0 || r.SchemaName != "words_judge" {
		t.Errorf("the request is %+v", r)
	}
	r, _ = wordsjudge.Request(wordsjudge.Item{Hint: "Where?", Lang: "en", Rung: 1})
	if !strings.Contains(r.Text, "Note at the side: none\n") {
		t.Errorf("a request without a note: %q", r.Text)
	}
	if _, err := wordsjudge.Request(wordsjudge.Item{Hint: " "}); err == nil {
		t.Error("a hint of no words is asked")
	}
}

// An answer reads strictly: every field, none besides, one value.
func TestRead(t *testing.T) {
	v, err := wordsjudge.Read(`{"gender": true, "on_task": true, "for_age": true, "mentions_note": false, "why": "«нашёл» tells a boy"}`)
	if err != nil || !v.Gender || !v.Flagged() || v.Why == "" {
		t.Errorf("the verdict is %+v, %v", v, err)
	}
	if v, _ := wordsjudge.Read(`{"gender": false, "on_task": true, "for_age": true, "mentions_note": false, "why": ""}`); v.Flagged() {
		t.Errorf("a clean verdict is flagged: %+v", v)
	}
	for _, a := range []string{
		`{"gender": false, "on_task": true, "for_age": true, "mentions_note": false}`,
		`{"gender": false, "on_task": true, "for_age": true, "mentions_note": false, "why": "", "more": 1}`,
		`{"gender": false, "on_task": true, "for_age": true, "mentions_note": false, "why": ""} {}`,
		`not json`,
	} {
		if _, err := wordsjudge.Read(a); err == nil {
			t.Errorf("%s reads", a)
		}
	}
}

func readControls(t *testing.T) *wordsjudge.Controls {
	t.Helper()
	f, err := os.Open("../../eval/words-judge.json")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	c, err := wordsjudge.ReadControls(f)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// The control phrases try each question both ways, in both languages, with every kind of note
// taken up and kept silent on, and the traps of the rough search: «начала» as a noun, a person of
// the task, «young man».
func TestControls(t *testing.T) {
	c := readControls(t)
	type side struct {
		q    string
		flag bool
		lang string
	}
	seen := map[side]bool{}
	for _, k := range c.Controls {
		e := k.Expect
		for q, flag := range map[string]bool{"gender": e.Gender, "on_task": !e.OnTask, "for_age": !e.ForAge, "mentions_note": e.MentionsNote} {
			if q != "mentions_note" || k.Item.Note != "" {
				seen[side{q, flag, k.Item.Lang}] = true
			}
		}
		if k.Item.Note != "" && e.Gender {
			t.Errorf("%s: a note and the sex told at once tries two things", k.ID)
		}
	}
	for _, q := range []string{"gender", "on_task", "for_age", "mentions_note"} {
		for _, lang := range []string{"ru", "en"} {
			for _, flag := range []bool{true, false} {
				if !seen[side{q, flag, lang}] {
					t.Errorf("no control of %s %t in %s", q, flag, lang)
				}
			}
		}
	}
	for _, id := range []string{"ru-noun-beginnings", "ru-task-person", "en-task-her", "en-young-man"} {
		if !slices.ContainsFunc(c.Controls, func(k wordsjudge.Control) bool { return k.ID == id }) {
			t.Errorf("no control %s", id)
		}
	}
	notes := map[string]bool{}
	for _, k := range c.Controls {
		notes[k.Item.Note] = notes[k.Item.Note] || k.Expect.MentionsNote
	}
	if len(notes) < 1+len(hint.NoteKinds()) {
		t.Errorf("the controls have %d notes", len(notes)-1)
	}
}

// A file of controls is read strictly.
func TestReadControlsRefused(t *testing.T) {
	data, err := os.ReadFile("../../eval/words-judge.json")
	if err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{
		"another format":       strings.Replace(string(data), `"tutor-words-judge-controls"`, `"x"`, 1),
		"a field unknown":      strings.Replace(string(data), `"version": 1,`, `"version": 1, "more": 1,`, 1),
		"an ID twice":          strings.Replace(string(data), `"id": "ru-short-f"`, `"id": "ru-past-m"`, 1),
		"no hint":              strings.Replace(string(data), `"hint": "Ты верно нашёл первое число. Что нужно сделать дальше?"`, `"hint": ""`, 1),
		"a note expected none": strings.Replace(string(data), `"mentions_note": false`, `"mentions_note": true`, 1),
		"two values":           string(data) + "{}",
		"none":                 `{"format": "tutor-words-judge-controls", "version": 1, "controls": []}`,
		"not JSON":             "x",
	} {
		if _, err := wordsjudge.ReadControls(strings.NewReader(text)); err == nil {
			t.Errorf("%s: read", name)
		}
	}
}

// The judge that is always right gives every control's answers, and the log reads back.
func TestCheck(t *testing.T) {
	c := readControls(t)
	r := wordsjudge.Check(c, "words-judge.json", "abc", opus())
	if len(r.Steps) != len(c.Controls) {
		t.Fatalf("%d steps", len(r.Steps))
	}
	var log bytes.Buffer
	if _, err := r.Do(context.Background(), runner.Stub, 10, &log, clock); err != nil {
		t.Fatal(err)
	}
	l, err := wordsjudge.ReadLog(&log)
	if err != nil {
		t.Fatal(err)
	}
	if l.Run.Of != wordsjudge.OfControls || l.Run.File != "words-judge.json" || l.Run.Prompt != wordsjudge.Fingerprint() || l.Run.Judge.Model != "claude-opus-5-5" || len(l.Calls) != len(c.Controls) {
		t.Errorf("the log is %+v", l.Run)
	}
	for _, call := range l.Calls {
		if call.Verdict == nil || !call.Verdict.Same(call.Expect.Verdict()) {
			t.Errorf("%s: %+v", call.Control, call)
		}
	}
}

// hintLog is a log of a run of the set of the words: two cases of a sheet, one hint that tells the
// sex and takes the note up, one that failed, and one not asked.
func hintLog() *hint.Log {
	return &hint.Log{
		Run: hint.RunRecord{Cases: []hint.Case{
			{ID: "w1", Kind: hint.CaseNote, Sheet: 1, Rung: 1, Lang: sample.LangRussian, Work: true, Note: "мне грустно, меня никто не любит", NoteKind: hint.NoteSad},
			{ID: "w2", Kind: hint.CaseRung, Sheet: 1, Rung: 2, Lang: sample.LangEnglish, Work: true},
		}},
		Sheets: []hint.SheetRecord{{Set: "words-v1", Sheet: 1, Statement: "Вычисли 2 + 3."}},
		Calls: []hint.CallRecord{
			{Call: hint.Call{Set: "words-v1", Sheet: 1, Rung: 1, Lang: sample.LangRussian, Tutor: "opus-high", Repeat: 1, Case: "w1"}, Hint: "Ты нашёл, меня никто не любит?", Gendered: []string{"нашёл"}, NoteEcho: []string{"меня никто не"}},
			{Call: hint.Call{Set: "words-v1", Sheet: 1, Rung: 2, Lang: sample.LangEnglish, Tutor: "opus-high", Repeat: 1, Case: "w2"}, Error: "overloaded"},
			{Call: hint.Call{Set: "words-v1", Sheet: 1, Rung: 2, Lang: sample.LangEnglish, Tutor: "opus-high", Repeat: 1, Case: "w2"}, NotAsked: true},
			{Call: hint.Call{Set: "words-v1", Sheet: 1, Rung: 2, Lang: sample.LangEnglish, Tutor: "haiku-low", Repeat: 1, Case: "w2"}, Hint: "Where is the slip?"},
		},
	}
}

// The judge of a log judges every hint that came, with the statement of its sheet and the note of its
// case; the judge that is always right answers by the code's measures. A log without cases, without a
// hint, or with a call of a case it does not tell is refused.
func TestOf(t *testing.T) {
	r, err := wordsjudge.Of(hintLog(), "t118.jsonl", "abc", opus())
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Steps) != 2 {
		t.Fatalf("%d steps", len(r.Steps))
	}
	req, err := r.Steps[0].Call.Request()
	if err != nil || !strings.Contains(req.Text, "Task: Вычисли 2 + 3.\n") || !strings.Contains(req.Text, "Note at the side: мне грустно, меня никто не любит\n") {
		t.Errorf("the request is %q, %v", req.Text, err)
	}
	var log bytes.Buffer
	if _, err := r.Do(context.Background(), runner.Stub, 10, &log, clock); err != nil {
		t.Fatal(err)
	}
	l, err := wordsjudge.ReadLog(&log)
	if err != nil {
		t.Fatal(err)
	}
	first, second := l.Calls[0], l.Calls[1]
	if l.Run.Of != wordsjudge.OfLog || first.Case != "w1" || first.CaseKind != hint.CaseNote || first.NoteKind != hint.NoteSad || first.Tutor != "opus-high" ||
		!first.Verdict.Gender || !first.Verdict.MentionsNote || !first.Verdict.OnTask || second.Verdict.Flagged() || second.Tutor != "haiku-low" {
		t.Errorf("the log is %+v, %+v", first, second)
	}

	for name, change := range map[string]func(l *hint.Log){
		"no cases":      func(l *hint.Log) { l.Run.Cases = nil },
		"no hint":       func(l *hint.Log) { l.Calls = l.Calls[1:3] },
		"a case untold": func(l *hint.Log) { l.Calls[0].Case = "w9" },
	} {
		l := hintLog()
		change(l)
		if _, err := wordsjudge.Of(l, "f", "s", opus()); err == nil {
			t.Errorf("%s: judged", name)
		}
	}
}

// An answer off the schema fails the call, with the reader's error, and a failed call keeps its
// error; a log of another kind is refused.
func TestRecord(t *testing.T) {
	r, err := wordsjudge.Of(hintLog(), "f", "s", opus())
	if err != nil {
		t.Fatal(err)
	}
	rec := r.Steps[0].Call.Record(runner.Outcome{Answer: `{"gender": true}`}).(wordsjudge.CallRecord)
	if rec.Verdict != nil || !strings.Contains(rec.Error, "off the schema") || rec.Kind != "call" {
		t.Errorf("an answer off the schema: %+v", rec)
	}
	rec = r.Steps[0].Call.Record(runner.Outcome{Error: "overloaded"}).(wordsjudge.CallRecord)
	if rec.Verdict != nil || rec.Error != "overloaded" {
		t.Errorf("a failed call: %+v", rec)
	}
	if _, err := wordsjudge.ReadLog(strings.NewReader(`{"kind": "run", "call_kind": "hint"}` + "\n")); err == nil {
		t.Error("a log of the hint reads as the judge's")
	}
}
