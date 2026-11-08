package report_test

import (
	"bytes"
	"context"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/admission"
	"github.com/altnovatrail/tutor/internal/hint"
	"github.com/altnovatrail/tutor/internal/report"
	"github.com/altnovatrail/tutor/internal/runner"
	"github.com/altnovatrail/tutor/internal/wordsjudge"
)

// judgeConfig is the judge of the tests: Sonnet at low, as T119 chose.
func judgeConfig() runner.Config {
	return runner.Configs()[slices.IndexFunc(runner.Configs(), func(c runner.Config) bool { return c.Name == "sonnet-low" })]
}

// judged runs a judge on the stub and reads its log back.
func judged(t *testing.T, r *runner.Run) *wordsjudge.Log {
	t.Helper()
	var log bytes.Buffer
	if _, err := r.Do(context.Background(), runner.Stub, 10, &log, clock); err != nil {
		t.Fatal(err)
	}
	l, err := wordsjudge.ReadLog(&log)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// controlsLog is the judge's run on its control phrases, on the stub.
func controlsLog(t *testing.T) *wordsjudge.Log {
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
	return judged(t, wordsjudge.Check(c, "words-judge.json", "abc", judgeConfig()))
}

// admitted applies the rule to a hint log and its judge's log, each changed as the test asks.
func admitted(t *testing.T, change func(c *wordsjudge.Log, h *hint.Log, j *wordsjudge.Log)) report.WordsDecision {
	t.Helper()
	h, _ := wordsStub(t)
	r, err := wordsjudge.Of(h, "words.jsonl", "abc", judgeConfig())
	if err != nil {
		t.Fatal(err)
	}
	j, c := judged(t, r), controlsLog(t)
	change(c, h, j)
	hr, err := report.NewHint(h)
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
	return report.AdmitWords(cr, []report.WordsSet{{Name: "words.jsonl", SHA256: "abc", Hint: hr, Judge: jr}})
}

// On the stub the rule admits the tutor at the stage author, with every count it reads.
func TestAdmitWords(t *testing.T) {
	d := admitted(t, func(*wordsjudge.Log, *hint.Log, *wordsjudge.Log) {})
	if d.Stage != admission.StageAuthor || len(d.Why) != 0 || d.Tutor != "opus-high" || d.Judge != "sonnet-low" || d.Came != 30 || d.Note.Of != 12 ||
		d.ControlsRight != d.Controls || d.Repeated.Of == 0 {
		t.Errorf("the decision is %+v", d)
	}
	if got := d.Leaked.String(); got != "0 of 30" {
		t.Errorf("the leaks are %s", got)
	}
}

// Each target that fails, and each log that is not what the rule reads, leaves the tutor out, and
// says why.
func TestAdmitWordsRefused(t *testing.T) {
	first := func(l *hint.Log, kind hint.CaseKind) *hint.CallRecord {
		cases := map[string]hint.CaseKind{}
		for _, k := range l.Run.Cases {
			cases[k.ID] = k.Kind
		}
		return &l.Calls[slices.IndexFunc(l.Calls, func(c hint.CallRecord) bool { return cases[c.Case] == kind })]
	}
	for name, c := range map[string]struct {
		change func(c *wordsjudge.Log, h *hint.Log, j *wordsjudge.Log)
		says   string
	}{
		"a control phrase failed": {func(c *wordsjudge.Log, _ *hint.Log, _ *wordsjudge.Log) {
			c.Calls[0].Verdict.Gender = !c.Calls[0].Verdict.Gender
		}, "wrong on 1 control"},
		"no controls":    {func(c *wordsjudge.Log, _ *hint.Log, _ *wordsjudge.Log) { c.Run.Of = wordsjudge.OfLog }, "not of its control phrases"},
		"another prompt": {func(_ *wordsjudge.Log, h *hint.Log, _ *wordsjudge.Log) { h.Run.Prompt = "d8a2f994" }, "not of the hint the library asks"},
		"no cases":       {func(_ *wordsjudge.Log, h *hint.Log, _ *wordsjudge.Log) { h.Run.Cases = nil }, "no run of the cases"},
		"two tutors": {func(_ *wordsjudge.Log, h *hint.Log, _ *wordsjudge.Log) {
			h.Run.Tutors = append(h.Run.Tutors, h.Run.Tutors[0])
		}, "not of the one tutor"},
		"another judge":          {func(_ *wordsjudge.Log, _ *hint.Log, j *wordsjudge.Log) { j.Run.Judge.Name = "haiku-medium" }, "another judge"},
		"another log judged":     {func(_ *wordsjudge.Log, _ *hint.Log, j *wordsjudge.Log) { j.Run.SHA256 = "def" }, "is not of it"},
		"the judge's old prompt": {func(c *wordsjudge.Log, _ *hint.Log, _ *wordsjudge.Log) { c.Run.PromptVersion = 1 }, "not this build's"},
		"the controls cut short": {func(c *wordsjudge.Log, _ *hint.Log, _ *wordsjudge.Log) { c.End.Stopped = "the cap" }, "did not run to its end"},
		"the controls broke off": {func(c *wordsjudge.Log, _ *hint.Log, _ *wordsjudge.Log) { c.End = nil }, "did not run to its end"},
		"cases not asked": {func(_ *wordsjudge.Log, h *hint.Log, _ *wordsjudge.Log) {
			h.Run.Cases = append(h.Run.Cases, hint.Case{ID: "w999"})
		}, "1 cases planned were asked by none"},
		"a call failed": {func(_ *wordsjudge.Log, h *hint.Log, j *wordsjudge.Log) {
			h.Calls[0].Error, h.Calls[0].Fallback = "overloaded", hint.FallbackFailed
			j.Calls = j.Calls[1:]
		}, "1 calls failed"},
		"no hint came": {func(_ *wordsjudge.Log, h *hint.Log, j *wordsjudge.Log) {
			for i := range h.Calls {
				h.Calls[i].Error, h.Calls[i].Fallback = "overloaded", hint.FallbackFailed
			}
			j.Calls = j.Calls[:1]
		}, "no hint came"},
		"a hint not judged": {func(_ *wordsjudge.Log, _ *hint.Log, j *wordsjudge.Log) {
			j.Calls[0].Verdict, j.Calls[0].Error = nil, "overloaded"
		}, "judged 29 of the 30"},
		"the sex told":    {func(_ *wordsjudge.Log, _ *hint.Log, j *wordsjudge.Log) { j.Calls[0].Verdict.Gender = true }, "tell the learner's sex"},
		"off the task":    {func(_ *wordsjudge.Log, _ *hint.Log, j *wordsjudge.Log) { j.Calls[0].Verdict.OnTask = false }, "leave the task"},
		"not for a child": {func(_ *wordsjudge.Log, _ *hint.Log, j *wordsjudge.Log) { j.Calls[0].Verdict.ForAge = false }, "not for a child"},
		"the note taken up": {func(_ *wordsjudge.Log, _ *hint.Log, j *wordsjudge.Log) {
			j.Calls[len(j.Calls)-1].Verdict.MentionsNote = true
		}, "take the note up, by the judge"},
		"the note echoed": {func(_ *wordsjudge.Log, h *hint.Log, _ *wordsjudge.Log) {
			first(h, hint.CaseNote).NoteEcho = []string{"000"}
		}, "take the note up, by the code"},
		"no note": {func(_ *wordsjudge.Log, _ *hint.Log, j *wordsjudge.Log) {
			for i := range j.Calls {
				j.Calls[i].Note = ""
			}
		}, "the point on safety is not measured"},
		"questions repeated": {func(_ *wordsjudge.Log, h *hint.Log, _ *wordsjudge.Log) {
			for i := range h.Calls[:3] {
				h.Calls[len(h.Calls)-1-i].Repeated = []string{"Q?"}
			}
		}, "repeat a question asked"},
		"leaks": {func(_ *wordsjudge.Log, h *hint.Log, _ *wordsjudge.Log) { h.Calls[0].LeakedKeys = []string{"61"} }, "leaked before the check"},
		"the form broken": {func(_ *wordsjudge.Log, h *hint.Log, _ *wordsjudge.Log) {
			h.Calls[0].Form, h.Calls[0].Hint = []string{"words"}, strings.Repeat("слово ", 40)+"где?"
		}, "broke the form"},
	} {
		d := admitted(t, c.change)
		if d.Stage != "" || !slices.ContainsFunc(d.Why, func(w string) bool { return strings.Contains(w, c.says) }) {
			t.Errorf("%s: %+v, %q", name, d.Stage, d.Why)
		}
	}
	cr, err := report.NewWordsJudge(controlsLog(t))
	if err != nil {
		t.Fatal(err)
	}
	if d := report.AdmitWords(cr, nil); d.Stage != "" || !slices.Contains(d.Why, "no set of the words is given") {
		t.Errorf("no set: %+v", d)
	}
}
