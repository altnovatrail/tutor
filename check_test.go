package tutor

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/altnovatrail/spec/types/admissionv0"

	"github.com/altnovatrail/tutor/internal/checking"
	"github.com/altnovatrail/tutor/provider"
)

// checkTask is a task of the answer of a Check, its read box and its first mistake's box on a work of
// 400 × 200 px.
func checkTask(task int, verdict, confidence, read, answer string) string {
	box := `{"x1": 0, "y1": 0, "x2": 0, "y2": 0}`
	errs := `[]`
	if read != "" {
		box = `{"x1": 100, "y1": 50, "x2": 200, "y2": 100}`
	}
	if verdict == checking.VerdictIncorrect {
		errs = `[{"box": {"x1": 200, "y1": 100, "x2": 400, "y2": 200}, "where": "the answer", "kind": "addition", "note": "one too many"}]`
	}
	return `{"task": ` + string(rune('0'+task)) + `, "assumptions": [], "solution": "the model's", "answer": "` + answer + `", "read": "` + read + `", "read_box": ` + box +
		`, "errors": ` + errs + `, "verdict": "` + verdict + `", "confidence": "` + confidence + `"}`
}

// checkAnswer is the answer of a Check of its tasks, with the tutor's words and hints given.
func checkAnswer(words string, hints []string, tasks ...string) string {
	quoted := make([]string, len(hints))
	for i, h := range hints {
		quoted[i] = `"` + h + `"`
	}
	return `{"tasks": [` + strings.Join(tasks, ", ") + `], "tutor_words": "` + words + `", "hints": [` + strings.Join(quoted, ", ") + `]}`
}

var (
	twoHints = []string{"Where do you begin?", "Look at the tens."}
	// sureCheck is a Check of two tasks Haiku is sure of: the first wrong, the second right.
	sureCheck = checkAnswer("Look again at the first one.", twoHints,
		checkTask(1, checking.VerdictIncorrect, checking.ConfidenceHigh, "13", "12"), checkTask(2, checking.VerdictCorrect, checking.ConfidenceMedium, "7", "7"))
)

func paperCheck(t *testing.T) PaperCheck {
	t.Helper()
	return PaperCheck{Route: platform, Statement: photoJPEG(t, 300, 120), Work: cropPNG(t, 400, 200), Tasks: 2, Language: "en"}
}

// A Check Haiku is sure of is the answer: a slot of each task with its boxes in thousandths of the
// work, the hidden solution and answer, the words and the hints; the crop goes as a photo, the work
// as its PNG, the age category the route's, and no later model is asked.
func TestCheckPaper(t *testing.T) {
	lib, m := cascadeOf(t, sureCheck, `{}`, `{}`)
	got := lib.CheckPaper(context.Background(), paperCheck(t))
	if got.Outcome.Kind != Answered || len(got.Slots) != 2 || got.TutorWords != "Look again at the first one." || len(got.Hints) != 2 || got.Hints[1] != "Look at the tens." ||
		timesAsked(m) != [3]int{1, 0, 0} || len(got.Outcome.Usage) != 1 {
		t.Fatalf("checked %+v, asked %v", got, timesAsked(m))
	}
	one := got.Slots[0]
	if one.Task != 1 || one.Verdict != "incorrect" || one.Confidence != "high" || one.Read != "13" || one.Answer != "12" || one.Solution != "the model's" ||
		len(one.Errors) != 1 || one.Errors[0].Kind != "addition" || one.Assumptions == nil {
		t.Errorf("slot 1 is %+v", one)
	}
	if b := one.ReadBox; b == nil || b.Xmin != 250 || b.Ymin != 250 || b.Xmax != 500 || b.Ymax != 500 {
		t.Errorf("the box read is %v", one.ReadBox)
	}
	if b := one.Errors[0].Box; b == nil || b.Xmin != 500 || b.Ymin != 500 || b.Xmax != 1000 || b.Ymax != 1000 {
		t.Errorf("the box of the mistake is %v", b)
	}
	r := m["claude-haiku-5-5"].asked[0]
	if r.Effort != "high" || r.Instructions != checking.Instructions() || r.MaxOutputTokens != checking.MaxOutputTokens || len(r.Images) != 2 ||
		len(r.Images[0].JPEG) == 0 || len(r.Images[1].PNG) == 0 || !strings.Contains(r.Text, "Image 2 is 400 pixels wide and 200 pixels high.") ||
		!strings.Contains(r.Text, "Tasks in the crop: 2.") || !strings.Contains(r.Text, "Learner's age category: adult.") || !strings.Contains(r.Text, "This is the first Check") {
		t.Errorf("the request is %+v\n%s", r, r.Text)
	}
}

// The instruction above the frame goes as Image 3, the age category api names and the history as
// they are given.
func TestCheckPaperInstructionAndHistory(t *testing.T) {
	lib, m := cascadeOf(t, sureCheck, `{}`, `{}`)
	c := paperCheck(t)
	c.Instruction, c.AgeCategory = photoJPEG(t, 300, 30), "under_13"
	c.History = []CheckTurn{{Verdicts: []string{"incorrect", "correct"}, Words: "Look again.", Rungs: []string{"Where do you begin?"}}}
	if got := lib.CheckPaper(context.Background(), c); got.Outcome.Kind != Answered {
		t.Fatalf("checked %+v", got)
	}
	r := m["claude-haiku-5-5"].asked[0]
	if len(r.Images) != 3 || len(r.Images[2].JPEG) == 0 || !strings.Contains(r.Text, "Learner's age category: under_13.") ||
		!strings.Contains(r.Text, "Verdicts: task 1 incorrect; task 2 correct") || !strings.Contains(r.Text, `Hints shown: "Where do you begin?"`) {
		t.Errorf("the request is %+v\n%s", r, r.Text)
	}
	c.Instruction = []byte("no picture")
	if got := lib.CheckPaper(context.Background(), c); got.Outcome.Kind != Failed {
		t.Errorf("an instruction that is no picture: %+v", got)
	}
}

// The cascade goes on at a verdict of low confidence, an unreadable task or an answer off the
// schema, and the last answer stands.
func TestCascadeOfTheCheck(t *testing.T) {
	low := checkAnswer("Check the first one again.", twoHints,
		checkTask(1, checking.VerdictIncorrect, checking.ConfidenceLow, "13", "12"), checkTask(2, checking.VerdictCorrect, checking.ConfidenceHigh, "7", "7"))
	unreadable := checkAnswer("", twoHints,
		checkTask(1, checking.VerdictUnreadable, checking.ConfidenceHigh, "", "12"), checkTask(2, checking.VerdictCorrect, checking.ConfidenceHigh, "7", "7"))
	for name, c := range map[string]struct {
		haiku, sonnet string
		asked         [3]int
		verdict       string
	}{
		"low, then sure":         {low, sureCheck, [3]int{1, 1, 0}, "incorrect"},
		"unreadable, then sure":  {unreadable, sureCheck, [3]int{1, 1, 0}, "incorrect"},
		"off, then sure":         {`{"tasks": []}`, sureCheck, [3]int{1, 1, 0}, "incorrect"},
		"low by all, the last's": {low, low, [3]int{1, 1, 1}, "unreadable"},
	} {
		opus := c.sonnet
		if name == "low by all, the last's" {
			opus = unreadable
		}
		lib, m := cascadeOf(t, c.haiku, c.sonnet, opus)
		got := lib.CheckPaper(context.Background(), paperCheck(t))
		if got.Outcome.Kind != Answered || timesAsked(m) != c.asked {
			t.Errorf("%s: checked %+v, asked %v", name, got, timesAsked(m))
			continue
		}
		if got.Slots[0].Verdict != c.verdict {
			t.Errorf("%s: the answer that stands is %+v", name, got.Slots)
		}
	}
	lib, m := cascadeOf(t, `{}`, `{}`, `{}`)
	if got := lib.CheckPaper(context.Background(), paperCheck(t)); got.Outcome.Kind != Failed || timesAsked(m) != [3]int{1, 1, 1} || len(got.Outcome.Usage) != 3 {
		t.Errorf("off the schema by all: %+v", got)
	}
}

// The tutor's words and each rung are searched for the hidden answer of every task not yet correct:
// words that name one are left out, and such a rung is its fallback text. An answer of a task that is
// correct, and a number the learner wrote, are no leak.
func TestCheckPaperLeaks(t *testing.T) {
	leaky := checkAnswer("The first one is 12, not 13.", []string{"Is it 12?", "You wrote 7 in the second: look at the first.", "Look at the units."},
		checkTask(1, checking.VerdictIncorrect, checking.ConfidenceHigh, "13", "12 cm"), checkTask(2, checking.VerdictCorrect, checking.ConfidenceHigh, "7", "7"))
	lib, _ := cascadeOf(t, leaky, `{}`, `{}`)
	got := lib.CheckPaper(context.Background(), paperCheck(t))
	if got.Outcome.Kind != Answered || got.TutorWords != "" || len(got.Hints) != 3 {
		t.Fatalf("checked %+v", got)
	}
	if got.Hints[0] != PaperFallback(1, "en") || got.Hints[1] != "You wrote 7 in the second: look at the first." || got.Hints[2] != "Look at the units." {
		t.Errorf("the hints are %q", got.Hints)
	}
	// A rung left empty is its fallback text too, in the language of the interface.
	empty := checkAnswer("Почти.", []string{"", "Где ошибка?"}, checkTask(1, checking.VerdictIncorrect, checking.ConfidenceHigh, "13", "12"), checkTask(2, checking.VerdictNoAnswer, checking.ConfidenceHigh, "", "8"))
	lib, _ = cascadeOf(t, empty, `{}`, `{}`)
	c := paperCheck(t)
	c.Language = "ru"
	if got := lib.CheckPaper(context.Background(), c); got.TutorWords != "Почти." || got.Hints[0] != PaperFallback(1, "ru") || got.Hints[1] != "Где ошибка?" {
		t.Errorf("an empty rung: %+v", got)
	}
}

// A request the Check does not take fails before any call, and without the author's word it is not
// admitted; a limit ends the cascade.
func TestCheckPaperFails(t *testing.T) {
	lib, m := cascadeOf(t, sureCheck, sureCheck, sureCheck)
	for name, change := range map[string]func(*PaperCheck){
		"no task":       func(c *PaperCheck) { c.Tasks = 0 },
		"21 tasks":      func(c *PaperCheck) { c.Tasks = 21 },
		"no crop":       func(c *PaperCheck) { c.Statement = nil },
		"a work of JPG": func(c *PaperCheck) { c.Work = photoJPEG(t, 40, 30) },
	} {
		c := paperCheck(t)
		change(&c)
		if got := lib.CheckPaper(context.Background(), c); got.Outcome.Kind != Failed {
			t.Errorf("%s: %+v", name, got)
		}
	}
	if timesAsked(m) != [3]int{} {
		t.Errorf("asked %v", timesAsked(m))
	}
	c := paperCheck(t)
	c.Statement = []byte("no crop")
	if got := lib.CheckPaper(context.Background(), c); got.Outcome.Kind != Failed || got.Outcome.Err == nil {
		t.Errorf("a crop that is no picture: %+v", got)
	}
	c.Language = ""
	c.Statement = photoJPEG(t, 30, 30)
	if got := lib.CheckPaper(context.Background(), c); got.Outcome.Kind != Failed {
		t.Errorf("no language: %+v", got)
	}
	m["claude-haiku-5-5"].err = &provider.LimitError{Limit: provider.LimitOwner}
	if got := lib.CheckPaper(context.Background(), paperCheck(t)); got.Outcome.Kind != LimitReached {
		t.Errorf("a limit: %+v", got)
	}
	m["claude-haiku-5-5"].err = errors.New("down")
	if got := lib.CheckPaper(context.Background(), paperCheck(t)); got.Outcome.Kind != Answered || len(got.Outcome.Usage) != 2 || got.Outcome.Usage[1].Model != "claude-sonnet-5-5" {
		t.Errorf("Haiku down: %+v", got)
	}
	child := paperCheck(t)
	child.Route = Route{AI: admissionv0.AiRoutePlatformAi, Category: admissionv0.AgeCategoryUnder13}
	if got := lib.CheckPaper(context.Background(), child); got.Outcome.Kind != NotAdmitted {
		t.Errorf("a child without the facts for minors: %+v", got)
	}
	withoutWord(t)
	lib, m = cascadeOf(t, sureCheck, sureCheck, sureCheck)
	if got := lib.CheckPaper(context.Background(), paperCheck(t)); got.Outcome.Kind != NotAdmitted || timesAsked(m) != [3]int{} {
		t.Errorf("without the word: %+v", got)
	}
}
