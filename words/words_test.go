package words

import (
	"slices"
	"strings"
	"testing"
)

// The form of a hint, part by part, with each limit at its edge.
func TestHint(t *testing.T) {
	ru30 := strings.Repeat("слово ", 29) + "что?"
	ru31 := strings.Repeat("слово ", 30) + "что?"
	en35 := strings.Repeat("word ", 34) + "what?"
	en36 := strings.Repeat("word ", 35) + "what?"
	cases := []struct {
		name, text, lang, statement string
		want                        []Breach
	}{
		{"one question", "Что прибавили к 7?", "ru", "", nil},
		{"two sentences", "Посмотри на вторую строку. Что прибавили к 7?", "ru", "", nil},
		{"three sentences", "Хорошо. Посмотри на строку. Что прибавили?", "ru", "", []Breach{Sentences}},
		{"no question", "Посмотри на вторую строку.", "ru", "", []Breach{Questions}},
		{"two questions", "Что прибавили? Почему?", "ru", "", []Breach{Questions}},
		{"no word", "?", "en", "", []Breach{Questions, Words}},
		{"30 words in Russian", ru30, "ru", "", nil},
		{"31 words in Russian", ru31, "ru", "", []Breach{Words}},
		{"35 words in English", en35, "en-GB", "", nil},
		{"36 words in English", en36, "en", "", []Breach{Words}},
		{"no limit in Spanish", strings.Repeat("palabra ", 60) + "¿qué?", "es", "", nil},
		{"Latin in Russian", "Чему равен y?", "ru", "Реши 3x + 1 = 7.", []Breach{Script}},
		{"Latin the statement shows", "Чему равен X?", "ru", "Реши 3x + 1 = 7.", nil},
		{"Cyrillic in English", "What is х?", "en", "", []Breach{Script}},
		{"a number is one word", "Is it 0.5 or 4/3?", "en", "", nil},
		{"an underscored tag", "What is х?", "en_US", "", []Breach{Script}},
		{"no language", "Что такое?", "", "", []Breach{Language}},
		{"no tag", "Что такое?", "r2-d2", "", []Breach{Language}},
	}
	for _, c := range cases {
		if got := Hint(c.text, c.lang, c.statement); !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
	// The measurement's form: 30 words in English too.
	if got := HintForm(en35, "en", "", 30); !slices.Equal(got, []Breach{Words}) {
		t.Errorf("35 words against 30: %v", got)
	}
	if HintWords("ru") != 30 || HintWords("EN-us") != 35 || HintWords("de") != 0 {
		t.Errorf("the limits are %d, %d, %d", HintWords("ru"), HintWords("EN-us"), HintWords("de"))
	}
}

// The rule of feedback, part by part, against the text read.
func TestFeedback(t *testing.T) {
	read := "My best day. We went to the sea and we swam all day long."
	en80 := strings.Repeat("good ", 79) + "work."
	en81 := strings.Repeat("good ", 80) + "work."
	cases := []struct {
		name, text, lang string
		want             []Breach
	}{
		{"three sentences", "You wrote well. Your title is clear. Add one more detail.", "en", nil},
		{"four sentences", "Good. Clear. Short. Add more.", "en", []Breach{Sentences}},
		{"80 words", en80, "en", nil},
		{"81 words", en81, "en", []Breach{Words}},
		{"no word", "", "en", []Breach{Words}},
		{"a quote of the learner", `I liked "we swam all day long".`, "en", nil},
		{"a quote in other case and marks", `I liked “We SWAM all day.”`, "en", nil},
		{"a quote not the learner's", `Try "we swam in the warm sea".`, "en", []Breach{Quote}},
		{"a short quote is no quote", `Write "the sea was" better.`, "en", nil},
		{"Cyrillic in English", "Хорошо written.", "en", []Breach{Script}},
		{"no limit in Spanish", strings.Repeat("bien ", 100) + "hecho.", "es", nil},
		{"an apostrophe in a quote", "I liked ‘we didn’t swim in the warm sea at all’.", "en", []Breach{Quote}},
		{"an apostrophe in the learner's quote", "I liked ‘we swam all day long’ and that’s all.", "en", nil},
		{"a German quote", "Try „we swam in the warm sea“ here.", "en", []Breach{Quote}},
		{"a full-width quote", "Try ＂we swam in the warm sea＂ here.", "en", []Breach{Quote}},
		{"an unclosed quote", "Try \"we swam in the warm sea", "en", []Breach{Quote}},
		{"a letter as written", "Good 𝑥 work.", "en", []Breach{Script}},
		{"no language", "Good work.", "", []Breach{Language}},
	}
	for _, c := range cases {
		if got := Feedback(c.text, read, c.lang); !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
	if got, n := FeedbackForm(strings.Repeat("good ", 61), read, "en", 60); !slices.Equal(got, []Breach{Words}) || n != 61 {
		t.Errorf("61 words against 60: %v, %d", got, n)
	}
	if FeedbackWords("en") != 80 || FeedbackWords("ru") != 0 {
		t.Errorf("the limits are %d and %d", FeedbackWords("en"), FeedbackWords("ru"))
	}
}

func TestCount(t *testing.T) {
	cases := []struct {
		text                    string
		sentences, questions, n int
	}{
		{"Посмотри на строку. Что прибавили?", 2, 1, 5},
		{"Is it 0.5? Or 4/3?", 2, 2, 5},
		{"Wait… what?", 2, 1, 2},
		{"No end", 1, 0, 2},
		{"«Да.» Нет?", 2, 1, 2},
		{"", 0, 0, 0},
	}
	for _, c := range cases {
		s, q, n := Count(c.text)
		if s != c.sentences || q != c.questions || n != c.n {
			t.Errorf("%q: %d sentences, %d questions, %d words; want %d, %d, %d", c.text, s, q, n, c.sentences, c.questions, c.n)
		}
	}
	if got := quoted(`a "b c" and «d» and “e” and "unclosed`); !slices.Equal(got, []string{"b c", "d", "e", "unclosed"}) {
		t.Errorf("quoted = %q", got)
	}
	if got := Plain("Don’t, SWIM!"); !slices.Equal(got, []string{"don't", "swim"}) {
		t.Errorf("Plain = %q", got)
	}
}
