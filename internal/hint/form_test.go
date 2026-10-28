package hint_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/internal/hint"
	"github.com/altnovatrail/tutor/internal/leak"
	"github.com/altnovatrail/tutor/internal/sample"
)

// A hint is exactly one question in no more than two sentences, of 1 to 30 words, in the letters of
// its language; each breach is named on its own.
func TestForm(t *testing.T) {
	const equation = "Реши уравнение 3x + 1 = 7."
	thirty := strings.Repeat("слово ", 29) + "слово?"
	for _, c := range []struct {
		text      string
		lang      sample.Lang
		statement string
		want      []string
	}{
		{"Начало верное. Посмотри ещё раз на обведённое место: всё ли там так?", sample.LangRussian, "", nil},
		{"Good start. How would you solve the same task with smaller numbers?", sample.LangEnglish, "", nil},
		{"Сколько прыжков на прямой?", sample.LangRussian, "", nil},
		{"Сколько здесь прыжков? Посчитай их.", sample.LangRussian, "", nil},
		{"Сколько будет 0.5 м и ещё 2 м?", sample.LangRussian, "", nil},
		{"Ты пишешь «Сколько?» Почему так?", sample.LangRussian, "", []string{hint.BreachQuestions}},
		{"Что даёт «3x − 1»?", sample.LangRussian, equation, nil},
		{"Что даёт «3X − 1»?", sample.LangRussian, equation, nil},
		{"Что даёт «3y − 1»?", sample.LangRussian, equation, []string{hint.BreachScript}},
		{"Сколько прыжков…  А потом?", sample.LangRussian, "", nil},
		{"Посчитай ещё раз?!", sample.LangRussian, "", nil},
		{"Сколько прыжков? А домиков?", sample.LangRussian, "", []string{hint.BreachQuestions}},
		{"Посчитай прыжки.", sample.LangRussian, "", []string{hint.BreachQuestions}},
		{"Хорошо. Посмотри сюда. Что здесь?", sample.LangRussian, "", []string{hint.BreachSentences}},
		{"?", sample.LangRussian, "", []string{hint.BreachQuestions, hint.BreachWords}},
		{thirty, sample.LangRussian, "", nil},
		{"ещё " + thirty, sample.LangRussian, "", []string{hint.BreachWords}},
		{"Где ошибка? Look again.", sample.LangRussian, "", []string{hint.BreachScript}},
		{"Where is the mistake? Посмотри.", sample.LangEnglish, "", []string{hint.BreachScript}},
		{"Где здесь x?", sample.LangEnglish, equation, []string{hint.BreachScript}},
		{"Где о́шибка？", sample.LangRussian, "", nil},
		{"Сколько □ и △?", sample.LangRussian, "", nil},
		{"т. е. сколько?", sample.LangRussian, "", []string{hint.BreachSentences}},
		{"Подумай: «где ошибка?».", sample.LangRussian, "", nil},
		{"Где ошибка?Проверь ещё раз.", sample.LangRussian, "", nil},
		{"Что даёт ｘ?", sample.LangRussian, "Реши уравнение 3ｘ + 1 = 7.", nil},
		{strings.Repeat("0.5 ", 26) + "и 4/3 или 66?", sample.LangRussian, "", nil},
		{strings.Repeat("0.5 ", 27) + "и 4/3 или 66?", sample.LangRussian, "", []string{hint.BreachWords}},
	} {
		if got := hint.Form(c.text, c.lang, c.statement); !slices.Equal(got, c.want) {
			t.Errorf("%q in %s: %q, want %q", c.text, c.lang, got, c.want)
		}
	}
	if !strings.Contains(hint.Instructions(), fmt.Sprintf("two short sentences and %d words", hint.MaxWords)) {
		t.Error("the instructions do not name the limit of words")
	}
}

// The fallback texts: the templates of rungs 1 and 2 differ, rung 3 gives what rung 2 does, and
// each keeps the form of a hint and passes the leak check of every task of the three banks.
func TestFallback(t *testing.T) {
	var tasks []sample.Task
	for _, path := range []string{freeBank, linesBank, nearBank} {
		bank, err := sample.ReadBank(open(t, path))
		if err != nil {
			t.Fatal(err)
		}
		tasks = append(tasks, bank.Tasks...)
	}
	for _, lang := range hint.Langs() {
		if hint.Fallback(1, lang) == hint.Fallback(2, lang) || hint.Fallback(3, lang) != hint.Fallback(2, lang) {
			t.Errorf("the fallbacks in %s are not the rungs' templates", lang)
		}
		for _, rung := range hint.Rungs() {
			text := hint.Fallback(rung, lang)
			if form := hint.Form(text, lang, ""); form != nil {
				t.Errorf("the fallback of rung %d in %s breaks %v", rung, lang, form)
			}
			for i := range tasks {
				if found := leak.New(hint.LeakTask(&tasks[i])).Find(text); found != nil {
					t.Errorf("the fallback of rung %d in %s gives %v of %s away", rung, lang, found, tasks[i].ID)
				}
			}
		}
	}
}

// The leak check of a task of a bank searches for its key, by its value, unless the statement shows
// it, and for its leak phrases: of the three banks only equation-4 and posts have their keys in their
// statements, and there the leak phrases alone keep the answer (R39).
func TestLeakTask(t *testing.T) {
	var freed []string
	for _, path := range []string{freeBank, linesBank, nearBank} {
		bank, err := sample.ReadBank(open(t, path))
		if err != nil {
			t.Fatal(err)
		}
		for i := range bank.Tasks {
			task := &bank.Tasks[i]
			c := leak.New(hint.LeakTask(task))
			switch keys := c.Keys(); {
			case !slices.Equal(c.Phrases(), task.Leaks):
				t.Errorf("%s is searched for the phrases %q", task.ID, c.Phrases())
			case len(keys) == 0:
				if !slices.Contains(freed, task.ID) {
					freed = append(freed, task.ID)
				}
			case len(keys) != 1:
				t.Errorf("%s is searched for the keys %q", task.ID, keys)
			}
		}
	}
	slices.Sort(freed)
	if !slices.Equal(freed, []string{"equation-4", "posts"}) {
		t.Errorf("the keys the statements show are of %v", freed)
	}
	rabbits := &sample.Task{Statement: "Во дворе гуляют куры и кролики. У них вместе 5 голов и 14 ног. Сколько во дворе кроликов?", Key: "2", Leaks: []string{"на 2 ноги больше"}}
	if got := leak.New(hint.LeakTask(rabbits)).Find("Сколько ног у курицы — 2 или 4?"); !slices.Equal(got, []string{"2"}) {
		t.Errorf("the rabbits give %v away", got)
	}
	equation := &sample.Task{Statement: "Реши уравнение 3x + 1 = 7.", Key: "x = 2"}
	if got := leak.New(hint.LeakTask(equation)).Find("Значит, x = 2?"); !slices.Equal(got, []string{"2"}) {
		t.Errorf("the equation gives %v away", got)
	}
}
