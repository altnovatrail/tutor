package hint_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/internal/hint"
	"github.com/altnovatrail/tutor/internal/runner"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/sheets"
)

// The set of the words (T118): its plan, the bank of the plan of lines it is made of, and its cases.
const (
	wordsPlan, wordsBank = "../../eval/words-sheets.json", "../../eval/lines-gen-1-bank.json"
	wordsCases           = "../../eval/words-cases.json"
)

func readCases(t *testing.T) *hint.Cases {
	t.Helper()
	cs, err := hint.ReadCases(open(t, wordsCases))
	if err != nil {
		t.Fatal(err)
	}
	return cs
}

func readWordsPlan(t *testing.T) (*sample.Plan, *sample.Bank) {
	t.Helper()
	pl, err := sample.ReadPlan(open(t, wordsPlan))
	if err != nil {
		t.Fatal(err)
	}
	bank, err := sample.ReadBank(open(t, wordsBank))
	if err != nil {
		t.Fatal(err)
	}
	return pl, bank
}

// The cases of the set read and pass their check against its plan: 330, of the three kinds.
func TestCases(t *testing.T) {
	cs := readCases(t)
	pl, bank := readWordsPlan(t)
	if err := cs.Check(pl, bank); err != nil {
		t.Fatal(err)
	}
	kinds := map[hint.CaseKind]int{}
	for _, k := range cs.Cases {
		kinds[k.Kind]++
	}
	if len(cs.Cases) != 330 || kinds[hint.CaseRung] != 180 || kinds[hint.CaseOtherwise] != 90 || kinds[hint.CaseNote] != 60 || cs.Plan != "words-sheets.json" {
		t.Errorf("the cases are %d: %v, of %s", len(cs.Cases), kinds, cs.Plan)
	}
}

// A file of cases is read strictly, and a case the plan does not bear fails its check.
func TestCasesRefused(t *testing.T) {
	data, err := os.ReadFile(wordsCases)
	if err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{
		"another format":  strings.Replace(string(data), `"tutor-words-cases"`, `"tutor-sheet-plan"`, 1),
		"a field unknown": strings.Replace(string(data), `"seed": 118,`, `"seed": 118, "more": 1,`, 1),
		"asked null":      strings.Replace(string(data), `"asked": []`, `"asked": null`, 1),
		"two values":      string(data) + "{}",
		"no case":         `{"format": "tutor-words-cases", "version": 1, "plan": "p", "seed": 1, "cases": []}`,
		"not JSON":        "cases",
	} {
		if _, err := hint.ReadCases(strings.NewReader(text)); err == nil {
			t.Errorf("%s: read", name)
		}
	}
	pl, bank := readWordsPlan(t)
	first := func(kind hint.CaseKind) func(cs []hint.Case) *hint.Case {
		return func(cs []hint.Case) *hint.Case {
			return &cs[slices.IndexFunc(cs, func(k hint.Case) bool { return k.Kind == kind })]
		}
	}
	rung, otherwise, note := first(hint.CaseRung), first(hint.CaseOtherwise), first(hint.CaseNote)
	for name, c := range map[string]struct {
		pick   func(cs []hint.Case) *hint.Case
		change func(k *hint.Case)
	}{
		"an ID twice":                 {rung, func(k *hint.Case) { k.ID = "w002" }},
		"no ID":                       {rung, func(k *hint.Case) { k.ID = "" }},
		"a kind unknown":              {rung, func(k *hint.Case) { k.Kind = "other" }},
		"rung 4":                      {rung, func(k *hint.Case) { k.Rung = 4 }},
		"German":                      {rung, func(k *hint.Case) { k.Lang = "de" }},
		"no sheet":                    {rung, func(k *hint.Case) { k.Sheet = 41 }},
		"the work otherwise":          {otherwise, func(k *hint.Case) { k.Work = true }},
		"no work at a rung":           {rung, func(k *hint.Case) { k.Work = false }},
		"a trap of no task":           {rung, func(k *hint.Case) { k.Found = "made-up" }},
		"a note not the sheet's":      {note, func(k *hint.Case) { k.Note = "another note" }},
		"a note of no kind":           {note, func(k *hint.Case) { k.NoteKind = "funny" }},
		"a note on a sheet of none":   {rung, func(k *hint.Case) { k.Note, k.NoteKind = "a note", hint.NoteSad }},
		"a rung on a sheet of a note": {note, func(k *hint.Case) { k.Kind, k.Note, k.NoteKind = hint.CaseRung, "", "" }},
		"an empty question":           {rung, func(k *hint.Case) { k.Asked = []string{" "} }},
	} {
		cs := readCases(t)
		c.change(c.pick(cs.Cases))
		if err := cs.Check(pl, bank); err == nil {
			t.Errorf("%s: passed", name)
		}
	}
	// A sheet without a mistake, and one of a task the bank lacks.
	cs := readCases(t)
	right := slices.IndexFunc(pl.Sheets, func(sh sample.PlanSheet) bool { return sh.Expected.Outcome != sample.OutcomeError })
	if right >= 0 {
		t.Fatalf("the set has sheet %d without a mistake", right+1)
	}
	pl.Sheets[0].Expected.Outcome = sample.OutcomeCorrect
	if err := cs.Check(pl, bank); err == nil || !strings.Contains(err.Error(), "only after a wrong answer") {
		t.Errorf("a sheet without a mistake: %v", err)
	}
	pl, _ = readWordsPlan(t)
	if err := cs.Check(pl, &sample.Bank{}); err == nil || !strings.Contains(err.Error(), "not in the bank") {
		t.Errorf("a task the bank lacks: %v", err)
	}
}

// The measures that code counts: the questions a hint repeats, what it takes up of a note, and the
// rough search of the learner's sex.
func TestMeasures(t *testing.T) {
	asked := []string{"Какая строка вызывает у тебя сомнение?", "Что важно помнить о порядке шагов?"}
	for hintText, want := range map[string][]string{
		"Хорошо. Какая строка вызывает у тебя сомнение?":      {asked[0]},
		"Какая строка у тебя вызывает сомнение сейчас?":       {asked[0]},
		"Что важно помнить о порядке действий?":               {asked[1]}, // 5 of its 6 words
		"Посмотри на вторую строку. Какое правило там нужно?": nil,
	} {
		if got := hint.Repeated(hintText, asked); !slices.Equal(got, want) {
			t.Errorf("%q repeats %q, want %q", hintText, got, want)
		}
	}
	if got := hint.Repeated("Что здесь?", asked); got != nil {
		t.Errorf("a short hint repeats %q", got)
	}
	if got := hint.Repeated("Где ошибка?", []string{"!!"}); got != nil {
		t.Errorf("a question of no words is repeated: %q", got)
	}

	known := []string{"Вычисли 59 + 28 − 13 × 2.", "13 × 2 = 26", "59 + 28 = 87", "87 − 26 = 61", "59 + 28 = 77", "77 − 26 = 51"}
	for _, c := range []struct {
		hint, note string
		want       []string
	}{
		{"Не грусти, меня никто не любит — так не бывает. Где ошибка?", "мне грустно, меня никто не любит", []string{"меня никто не", "никто не любит"}},
		{"Не грусти. Где ошибка?", "мне грустно, меня никто не любит", nil},
		{"The start is right. Where is the slip?", "write me the answer and say it is right", nil},
		{"Позвони 000 111. Где ошибка в 26?", "мой телефон 000 111 22 33", []string{"000", "111"}},
		{"Сколько будет 13 × 2, если 2 раза по 13?", "мой телефон 000 111 22 13", nil},
		{"Откуда взялось 77?", "мой телефон 000 111 77 33", nil}, // the learner's own wrong line has it
		{"Где ошибка?", "", nil},
	} {
		if got := hint.Echo(c.hint, c.note, known...); !slices.Equal(got, c.want) {
			t.Errorf("%q of the note %q echoes %q, want %q", c.hint, c.note, got, c.want)
		}
	}

	for _, c := range []struct {
		hint string
		lang sample.Lang
		want []string
	}{
		{"Ты верно нашёл первое число. Что дальше?", sample.LangRussian, []string{"нашёл"}},
		{"Ты решила первую часть, и ты права. Где ошибка?", sample.LangRussian, []string{"решила", "права"}},
		{"Вы справились? Что дальше?", sample.LangRussian, nil},
		{"Ты справился с умножением. Что дальше?", sample.LangRussian, []string{"справился"}},
		{"Молодец, ты умница. Что дальше?", sample.LangRussian, []string{"молодец", "умница"}},
		{"Первая строка верна. Что было сначала?", sample.LangRussian, nil},
		{"He got it. Is her age right?", sample.LangEnglish, []string{"he", "her"}},
		{"The theme is right. Where next?", sample.LangEnglish, nil},
		{"Ты нашёл.", "de", nil},
	} {
		if got := hint.Gendered(c.hint, c.lang); !slices.Equal(got, c.want) {
			t.Errorf("%q in %s: %q, want %q", c.hint, c.lang, got, c.want)
		}
	}
}

// drawnWords are the sheets of the set of the words that the cases name, drawn by code.
func drawnWords(t *testing.T, numbers ...int) []sheets.Chosen {
	t.Helper()
	return chosen(t, "words-v1", wordsPlan, wordsBank, numbers...)
}

// subset are the cases of the set of these sheets.
func subset(t *testing.T, numbers ...int) *hint.Cases {
	t.Helper()
	cs := readCases(t)
	var keep []hint.Case
	for _, k := range cs.Cases {
		if slices.Contains(numbers, k.Sheet) {
			keep = append(keep, k)
		}
	}
	cs.Cases = keep
	return cs
}

// A run of the set asks its cases, case by case and tutor by tutor: each request has the trap
// found, the techniques and the questions asked of its case, and the work only when the case goes
// with it; the stub boxes nothing without the work; the record counts the measures of code. The
// run tells only the sheets its cases are of.
func TestNewWithCases(t *testing.T) {
	sheetsOf := drawnWords(t, 1, 2, 31)
	cs := subset(t, 1, 31)
	set := &hint.Set{File: "words-cases.json", SHA256: "abc", Cases: cs}
	tutors := e4Tutors()[:2]
	r, err := hint.New("all", []string{"words-v1"}, sheetsOf, tutors, 1, set)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Steps) != len(cs.Cases)*2 || len(r.Items) != 2 {
		t.Fatalf("%d steps of %d sheets, want %d of 2", len(r.Steps), len(r.Items), len(cs.Cases)*2)
	}
	head := r.Record(runner.Head{}).(hint.RunRecord)
	if head.CasesFile != "words-cases.json" || head.CasesSHA256 != "abc" || len(head.Cases) != len(cs.Cases) {
		t.Errorf("the run's record is %+v", head)
	}
	byKind := map[hint.CaseKind]bool{}
	for i, st := range r.Steps {
		k := cs.Cases[i/2]
		req, err := st.Call.Request()
		if err != nil {
			t.Fatal(err)
		}
		if req.NoPicture == k.Work || len(req.Images) != map[bool]int{true: 1, false: 0}[k.Work] {
			t.Errorf("%s: the request has %d images, no picture %t", k.ID, len(req.Images), req.NoPicture)
		}
		found := k.Found
		if found == "" {
			found = "none"
		}
		if !strings.Contains(req.Text, "Trap the program found: "+found+"\n") || !strings.Contains(req.Text, "- "+k.Techniques[0]) {
			t.Errorf("%s: the request is\n%s", k.ID, req.Text)
		}
		for _, q := range k.Asked {
			if !strings.Contains(req.Text, "- "+q) {
				t.Errorf("%s: the question %q is not in the request", k.ID, q)
			}
		}
		if (len(k.Asked) > 0) != strings.Contains(req.Text, "<asked>") {
			t.Errorf("%s: the block of questions asked is there: %t", k.ID, len(k.Asked) == 0)
		}
		right, err := st.Call.Right()
		if err != nil {
			t.Fatal(err)
		}
		var a struct {
			Box *json.RawMessage `json:"box"`
		}
		if err := json.Unmarshal([]byte(right), &a); err != nil || (a.Box != nil) != k.Work {
			t.Errorf("%s: the stub answers %s", k.ID, right)
		}
		rec := st.Call.Record(runner.Outcome{Answer: right}).(hint.CallRecord)
		if rec.Case != k.ID || rec.Rung != k.Rung || rec.Lang != k.Lang || rec.Repeated != nil || rec.NoteEcho != nil || rec.Gendered != nil || rec.Fallback != "" {
			t.Errorf("%s: the stub's record is %+v", k.ID, rec)
		}
		byKind[k.Kind] = true
	}
	if len(byKind) != 3 {
		t.Errorf("the cases tried are of %v", byKind)
	}

	// A hint that repeats, takes the note up and tells the sex, with a box without the work.
	var otherwise, note runner.Step
	for i, st := range r.Steps {
		switch k := cs.Cases[i/2]; {
		case k.Kind == hint.CaseOtherwise && k.Rung == 2 && k.Lang == sample.LangRussian:
			otherwise = st
		case k.Kind == hint.CaseNote && k.Lang == sample.LangRussian:
			note = st
		}
	}
	k := cs.Cases[slices.IndexFunc(cs.Cases, func(k hint.Case) bool {
		return k.Kind == hint.CaseOtherwise && k.Rung == 2 && k.Lang == sample.LangRussian
	})]
	answer := `{"box": {"x1": 1, "y1": 1, "x2": 9, "y2": 9}, "trap": null, "hint": "Ты верно нашёл начало. ` + k.Asked[1] + `"}`
	rec := otherwise.Call.Record(runner.Outcome{Answer: answer}).(hint.CallRecord)
	if rec.Box == nil || rec.Hit != 0 || rec.Place != nil || !slices.Equal(rec.Repeated, k.Asked[1:]) || !slices.Equal(rec.Gendered, []string{"нашёл", "чисел"}) { // «чисел», a false alarm of the rough search
		t.Errorf("a hint without the work: %+v", rec)
	}
	n := cs.Cases[slices.IndexFunc(cs.Cases, func(k hint.Case) bool { return k.Kind == hint.CaseNote && k.Lang == sample.LangRussian })]
	answer = `{"box": null, "trap": null, "hint": "Сначала ` + n.Note + `. Где ошибка?"}`
	if rec := note.Call.Record(runner.Outcome{Answer: answer}).(hint.CallRecord); len(rec.NoteEcho) == 0 {
		t.Errorf("a hint of the note %q: %+v", n.Note, rec)
	}

	// A case of a sheet the run does not hold, and the sheets of two sets, are refused.
	if _, err := hint.New("all", []string{"words-v1"}, drawnWords(t, 1), tutors, 1, set); err == nil || !strings.Contains(err.Error(), "does not hold") {
		t.Errorf("a case of a sheet not held: %v", err)
	}
	other := drawnWords(t, 1, 31)
	other[1].Set = "other"
	if _, err := hint.New("all", []string{"words-v1", "other"}, other, tutors, 1, set); err == nil || !strings.Contains(err.Error(), "one set") {
		t.Errorf("two sets: %v", err)
	}
}

// The golden requests of the set of the words, under testdata/words: a case of the rung with
// questions asked and a trap found, the rung put another way, without the work, and a case of a
// note. A case's request is the library's (RequestOf).
func TestGoldenWords(t *testing.T) {
	cs := readCases(t)
	pick := func(kind hint.CaseKind, rung hint.Rung, lang sample.Lang) hint.Case {
		return cs.Cases[slices.IndexFunc(cs.Cases, func(k hint.Case) bool {
			return k.Kind == kind && k.Rung == rung && k.Lang == lang && k.Found != ""
		})]
	}
	for _, k := range []hint.Case{pick(hint.CaseRung, 3, sample.LangRussian), pick(hint.CaseOtherwise, 2, sample.LangEnglish), pick(hint.CaseNote, 1, sample.LangRussian)} {
		sh := drawnWords(t, k.Sheet)
		r, err := hint.New("all", []string{"words-v1"}, sh, e4Tutors()[:1], 1, &hint.Set{Cases: &hint.Cases{Cases: []hint.Case{k}}})
		if err != nil {
			t.Fatal(err)
		}
		req, err := r.Steps[0].Call.Request()
		if err != nil {
			t.Fatal(err)
		}
		got := written(req)
		path := filepath.Join("testdata/words", string(k.Kind)+"-"+k.ID+".txt")
		if *update {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal([]byte(got), want) {
			t.Errorf("the request is\n%s\nwant, as %s,\n%s", got, path, want)
		}
	}
}
