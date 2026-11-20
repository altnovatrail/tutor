package tutor

import (
	"context"
	"slices"
	"strings"
	"testing"

	paperv1 "github.com/altnovatrail/spec/contracts/novatrail/paper/v1"

	"github.com/altnovatrail/tutor/internal/paperwords"
)

func numberKey(v string) *paperv1.Answer {
	return &paperv1.Answer{Kind: &paperv1.Answer_Number{Number: &paperv1.Number{Value: v}}}
}

var frame = []PaperWordsTask{{Statement: "37 + 48 =", Key: numberKey("85"), Read: "75"}, {Statement: "□ − 3 = 7", Key: numberKey("10"), Read: "10"}}

// The words of a rung go by the cascade: Haiku's words that leak a key are not taken, Sonnet's are,
// with the box of the mistake in thousandths of the crop of the work, and Opus is not asked.
func TestPaperWordsOfARung(t *testing.T) {
	lib, m := cascadeOf(t,
		`{"box": null, "answers": [], "hint": "Is it 85?"}`,
		`{"box": {"x1": 20, "y1": 10, "x2": 100, "y2": 50}, "answers": [], "hint": "Look at the tens: what do they add up to?"}`,
		`{"box": null, "answers": [], "hint": "Never asked?"}`)
	got := lib.PaperWords(context.Background(), PaperWordsRequest{Route: platform, Ask: PaperRung, Crop: cropPNG(t, 200, 100),
		Tasks: frame, Task: 1, Rung: 1, Language: "en", Style: "an adult"})
	if got.Outcome.Kind != Answered || got.Question != "Look at the tens: what do they add up to?" || timesAsked(m) != [3]int{1, 1, 0} {
		t.Fatalf("the words are %+v, asked %v", got, timesAsked(m))
	}
	if b := got.Box; b == nil || b.Xmin != 100 || b.Ymin != 100 || b.Xmax != 500 || b.Ymax != 500 {
		t.Errorf("the box is %v", got.Box)
	}
	r := m["claude-haiku-5-5"].asked[0]
	if r.Instructions != paperwords.Instructions() || r.SchemaName != "paper_words" || len(r.Images) != 1 || len(r.Images[0].PNG) == 0 ||
		!strings.Contains(r.Text, "about task 1") || !strings.Contains(r.Text, "Key: 85") || r.MaxOutputTokens != paperwords.MaxOutputTokens {
		t.Errorf("the request is %+v", r)
	}
}

// The opening question sends the task's crop as a photo, and the model's own answer is searched
// for in it: a question that gives it away is not taken. A rung put another way sends no picture.
func TestPaperOpeningAndAgain(t *testing.T) {
	lib, m := cascadeOf(t,
		`{"box": null, "answers": ["12"], "hint": "Half of 20 is 12, right?"}`,
		`{"box": null, "answers": ["10"], "hint": "What is cut, and into how many parts?"}`,
		`{"box": null, "answers": [], "hint": "x?"}`)
	rope := []PaperWordsTask{{Statement: "A rope of 20 cm is cut in half. How long is each part?"}}
	got := lib.PaperWords(context.Background(), PaperWordsRequest{Route: platform, Ask: PaperOpening, Statement: photoJPEG(t, 60, 40),
		Tasks: rope, Language: "en", Style: "an adult"})
	if got.Outcome.Kind != Answered || got.Question != "What is cut, and into how many parts?" || got.Box != nil || !slices.Equal(got.Answers, []string{"10"}) || timesAsked(m) != [3]int{1, 1, 0} {
		t.Fatalf("the opening question is %+v, asked %v", got, timesAsked(m))
	}
	if r := m["claude-haiku-5-5"].asked[0]; len(r.Images) != 1 || len(r.Images[0].JPEG) == 0 || !strings.Contains(r.Text, "the opening question") {
		t.Errorf("the request is %+v", r)
	}
	// Before the tasks are read the opening question goes with none, and its own answers alone
	// catch a leak.
	lib, m = cascadeOf(t, `{"box": null, "answers": ["10"], "hint": "Is each part 10 cm long?"}`,
		`{"box": null, "answers": ["10"], "hint": "What do you need to find?"}`, `{}`)
	got = lib.PaperWords(context.Background(), PaperWordsRequest{Route: platform, Ask: PaperOpening, Statement: photoJPEG(t, 60, 40),
		Language: "en", Style: "an adult"})
	if got.Question != "What do you need to find?" || timesAsked(m) != [3]int{1, 1, 0} {
		t.Errorf("the opening question with no tasks is %+v, asked %v", got, timesAsked(m))
	}
	lib, m = cascadeOf(t, `{"box": null, "answers": [], "hint": "Which part of the sum comes first?"}`, `{}`, `{}`)
	got = lib.PaperWords(context.Background(), PaperWordsRequest{Route: platform, Ask: PaperAgain, Tasks: frame, Task: 1, Rung: 2,
		Language: "en", Style: "an adult", Asked: []string{"Where is it?"}})
	if got.Outcome.Kind != Answered || got.Question == "" || !m["claude-haiku-5-5"].asked[0].NoPicture {
		t.Errorf("the rung again is %+v", got)
	}
}

// A request a kind does not take fails before any call, and without the author's word the words
// are not admitted.
func TestPaperWordsFail(t *testing.T) {
	lib, m := cascadeOf(t, `{}`, `{}`, `{}`)
	for name, w := range map[string]PaperWordsRequest{
		"no style":   {Route: platform, Ask: PaperRung, Crop: cropPNG(t, 20, 20), Tasks: frame, Task: 1, Rung: 1, Language: "en"},
		"no crop":    {Route: platform, Ask: PaperRung, Tasks: frame, Task: 1, Rung: 1, Language: "en", Style: "s"},
		"no task":    {Route: platform, Ask: PaperOpening, Language: "en", Style: "s"},
		"no picture": {Route: platform, Ask: PaperOpening, Tasks: frame, Language: "en", Style: "s"},
	} {
		if got := lib.PaperWords(context.Background(), w); got.Outcome.Kind != Failed {
			t.Errorf("%s: %+v", name, got)
		}
	}
	if timesAsked(m) != [3]int{} {
		t.Errorf("asked %v", timesAsked(m))
	}
	withoutWord(t)
	lib, m = cascadeOf(t, `{}`, `{}`, `{}`)
	if got := lib.PaperWords(context.Background(), PaperWordsRequest{Route: platform, Ask: PaperAgain, Tasks: frame, Task: 1, Rung: 1,
		Language: "en", Style: "s"}); got.Outcome.Kind != NotAdmitted || timesAsked(m) != [3]int{} {
		t.Errorf("without the word: %+v", got)
	}
}

func TestPaperBreach(t *testing.T) {
	for _, c := range []struct {
		text, want string
		own        []string
	}{
		{"Look at the tens: what do they add up to?", "", nil},
		{"Is the answer 85?", BreachLeak, nil},
		{"Is it ten?", "", nil},
		{"Is each part 12 cm?", BreachLeak, []string{"12"}},
		{"Is it B?", "", []string{"B"}},
		{"", BreachForm, nil},
		{"Look again. Check it.", BreachForm, nil},
		{"Где ошибка?", BreachForm, nil},
	} {
		if got := PaperBreach(c.text, "en", frame, c.own); got != c.want {
			t.Errorf("%q: %q, not %q", c.text, got, c.want)
		}
	}
	// A key the statement prints is not searched for.
	printed := []PaperWordsTask{{Statement: "Which is 7: 5 + 2 or 5 + 3?", Key: numberKey("7")}}
	if got := PaperBreach("Which sum gives 7?", "en", printed, nil); got != "" {
		t.Errorf("a printed key: %q", got)
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
			if PaperBreach(text, check, frame, nil) != "" {
				t.Errorf("rung %d in %s: %q breaks the checks", rung, lang, text)
			}
		}
	}
	if PaperFallback(3, "ru") != PaperFallback(2, "ru") || PaperFallback(1, "en") == PaperFallback(2, "en") || PaperFallback(0, "ru") == PaperFallback(0, "en") {
		t.Error("the fallback texts are not by rung and language")
	}
}
