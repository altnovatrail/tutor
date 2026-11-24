package tutor

import (
	"context"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/internal/paperwords"
)

var frame = []PaperWordsTask{{Statement: "37 + 48 =", Answer: "85", Solution: "37 + 48 = 85", Read: "75"}, {Statement: "□ − 3 = 7", Read: "10"}}

// A rung put another way goes by the cascade with no picture: Haiku's words that leak a hidden
// answer are not taken, Sonnet's are, and Opus is not asked. The tasks go with the Check's answers
// and solutions.
func TestPaperWordsAgain(t *testing.T) {
	lib, m := cascadeOf(t,
		`{"hint": "Is it 85?"}`,
		`{"hint": "Look at the tens: what do they add up to?"}`,
		`{"hint": "Never asked?"}`)
	got := lib.PaperWords(context.Background(), PaperWordsRequest{Route: platform, Ask: PaperAgain, Tasks: frame, Task: 1, Rung: 2,
		Language: "en", Style: "an adult", Asked: []string{"Where is it?"}})
	if got.Outcome.Kind != Answered || got.Question != "Look at the tens: what do they add up to?" || got.Box != nil || timesAsked(m) != [3]int{1, 1, 0} {
		t.Fatalf("the words are %+v, asked %v", got, timesAsked(m))
	}
	r := m["claude-haiku-5-5"].asked[0]
	if r.Instructions != paperwords.Instructions() || r.SchemaName != "paper_words" || len(r.Images) != 0 || !r.NoPicture ||
		!strings.Contains(r.Text, "about task 1") || !strings.Contains(r.Text, "Answer: 85") || !strings.Contains(r.Text, "Solution: 37 + 48 = 85") ||
		r.MaxOutputTokens != paperwords.MaxOutputTokens {
		t.Errorf("the request is %+v", r)
	}
}

// A request the words do not take fails before any call, and without the author's word the words
// are not admitted.
func TestPaperWordsFail(t *testing.T) {
	lib, m := cascadeOf(t, `{}`, `{}`, `{}`)
	for name, w := range map[string]PaperWordsRequest{
		"no style":      {Route: platform, Ask: PaperAgain, Tasks: frame, Task: 1, Rung: 1, Language: "en"},
		"another kind":  {Route: platform, Tasks: frame, Task: 1, Rung: 1, Language: "en", Style: "s"},
		"no task":       {Route: platform, Ask: PaperAgain, Task: 1, Rung: 1, Language: "en", Style: "s"},
		"no rung":       {Route: platform, Ask: PaperAgain, Tasks: frame, Task: 1, Rung: 4, Language: "en", Style: "s"},
		"a long prompt": {Route: platform, Ask: PaperAgain, Tasks: frame, Task: 1, Rung: 1, Language: "en", Style: strings.Repeat("s", maxTextBytes)},
	} {
		if got := lib.PaperWords(context.Background(), w); got.Outcome.Kind != Failed {
			t.Errorf("%s: %+v", name, got)
		}
	}
	if timesAsked(m) != [3]int{} {
		t.Errorf("asked %v", timesAsked(m))
	}
	if got := lib.PaperWords(context.Background(), PaperWordsRequest{Route: platform, Ask: PaperAgain, Tasks: frame, Task: 1, Rung: 1, Language: "en", Style: "s"}); got.Outcome.Kind != Failed || timesAsked(m) != [3]int{1, 1, 1} {
		t.Errorf("off the schema: %+v", got)
	}
	withoutWord(t)
	lib, m = cascadeOf(t, `{}`, `{}`, `{}`)
	if got := lib.PaperWords(context.Background(), PaperWordsRequest{Route: platform, Ask: PaperAgain, Tasks: frame, Task: 1, Rung: 1,
		Language: "en", Style: "s"}); got.Outcome.Kind != NotAdmitted || timesAsked(m) != [3]int{} {
		t.Errorf("without the word: %+v", got)
	}
}

// The leak check searches the hidden answer of each task given, each number of it in any
// notation, but a number the statements print or the learner wrote; an answer of a word is a phrase,
// unless the learner sees it, and a single letter is not searched for.
func TestPaperBreach(t *testing.T) {
	for _, c := range []struct {
		text, want string
		tasks      []PaperWordsTask
	}{
		{"Look at the tens: what do they add up to?", "", frame},
		{"Is the answer 85?", BreachLeak, frame},
		{"Is it eighty?", "", frame},
		{"You wrote 75 and 10: which is the first?", "", frame},
		{"Is it 3?", "", frame},
		{"Is each part 12 cm?", BreachLeak, []PaperWordsTask{{Answer: "x = 12 cm"}}},
		{"Is each part 1/2 of it?", BreachLeak, []PaperWordsTask{{Answer: "0.5"}}},
		{"Is it B?", "", []PaperWordsTask{{Answer: "B"}}},
		{"Is it a noun?", BreachLeak, []PaperWordsTask{{Answer: "noun"}}},
		{"Which word is a noun?", "", []PaperWordsTask{{Statement: "Find the noun.", Answer: "noun"}}},
		{"", BreachForm, frame},
		{"Look again. Check it.", BreachForm, frame},
		{"Где ошибка?", BreachForm, frame},
	} {
		if got := PaperBreach(c.text, "en", c.tasks); got != c.want {
			t.Errorf("%q: %q, not %q", c.text, got, c.want)
		}
	}
	// A number the statement prints is not searched for.
	printed := []PaperWordsTask{{Statement: "Which is 7: 5 + 2 or 5 + 3?", Answer: "7"}}
	if got := PaperBreach("Which sum gives 7?", "en", printed); got != "" {
		t.Errorf("a printed answer: %q", got)
	}
}

func TestPaperFallback(t *testing.T) {
	for _, lang := range []string{"en", "ru", "en-GB", "de"} {
		for rung := 0; rung <= 3; rung++ {
			text := PaperFallback(rung, lang)
			check := lang
			if lang == "de" {
				check = "en"
			}
			if PaperBreach(text, check, frame) != "" {
				t.Errorf("rung %d in %s: %q breaks the checks", rung, lang, text)
			}
		}
	}
	if PaperFallback(3, "ru") != PaperFallback(2, "ru") || PaperFallback(1, "en") == PaperFallback(2, "en") || PaperFallback(0, "ru") == PaperFallback(0, "en") {
		t.Error("the fallback texts are not by rung and language")
	}
}
