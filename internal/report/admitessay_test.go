package report_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/admission"
	"github.com/altnovatrail/tutor/internal/essay"
	"github.com/altnovatrail/tutor/internal/report"
	"github.com/altnovatrail/tutor/internal/runner"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/wordsjudge"
)

// admissionEssays is a log of a run of admission of the essay in two calls, as the rules read it:
// 200 essays of the test part of eval/essays-gen.json, of 50 words and two criteria each, every fifth
// without its spelling and every other with a word spelled wrong, read by opus-high and assessed by
// haiku-medium in two repeats, every reading and judgement right.
func admissionEssays() *essay.Log {
	met := func(v bool) *bool { return &v }
	l := &essay.Log{
		Run: essay.RunRecord{Kind: "run", CallKind: essay.CallKind, Part: "test", File: "eval/essays-gen.json", Seed: 1, Repeats: 2,
			Prompt: essay.TwoFingerprint(), PromptVersion: essay.TwoVersion, Path: essay.PathTwo, ReadPrompt: essay.ReadFingerprint(), TextPrompt: essay.TextFingerprint(),
			Rubric:   []sample.Criterion{{ID: "title"}, {ID: "spelling"}},
			Readers:  []essay.ReaderRecord{{Name: "opus-high", Model: "claude-opus-5-5", Effort: "high"}},
			Assessor: &essay.ReaderRecord{Name: "haiku-medium", Model: "claude-haiku-5-5", Effort: "medium"}},
		End: &runner.End{Kind: "end"},
	}
	for n := 1; n <= 200; n++ {
		e := essay.EssayRecord{Kind: "essay", Essay: n, Part: "test", Unmet: []string{}, Words: 50}
		if n%5 == 0 {
			e.Unmet = []string{"spelling"}
		}
		if n%2 == 0 {
			e.Misspelled = []sample.Misspelling{{Written: "wrod"}}
		}
		l.Sheets = append(l.Sheets, e)
	}
	for rep := 1; rep <= 2; rep++ {
		for _, e := range l.Sheets {
			call := essay.Call{Essay: e.Essay, Reader: "opus-high", Repeat: rep}
			read, assess := call, call
			read.Step, assess.Step = essay.StepRead, essay.StepAssess
			l.Calls = append(l.Calls,
				essay.CallRecord{Kind: "call", Call: read, Answer: `{"text": "x", "legible": true}`,
					Reading: &essay.Reading{Words: 50, Misspelled: len(e.Misspelled), MisspelledKept: len(e.Misspelled)}},
				essay.CallRecord{Kind: "call", Call: assess, Answer: `{"criteria": [], "feedback": "Good work."}`,
					Marks: []essay.Mark{{ID: "title", Met: met(true)}, {ID: "spelling", Met: met(len(e.Unmet) == 0)}}, FeedbackWords: 2})
		}
	}
	return l
}

// essayDecisions applies both rules of the essay to the log of admission, its judge's log of the
// feedback and the judge's run on its control phrases, each changed as the test asks: change runs on
// the log before its judge judges it, and judgedAs on the judge's logs and the log after.
func essayDecisions(t *testing.T, change func(l *essay.Log), judgedAs func(c, j *wordsjudge.Log, l *essay.Log)) (report.EssayDecision, report.RubricDecision) {
	t.Helper()
	l := admissionEssays()
	change(l)
	topics := map[int]string{}
	for _, e := range l.Sheets {
		topics[e.Essay] = "A day at the sea"
	}
	run, err := wordsjudge.OfFeedback(l, topics, "essays.jsonl", "abc", judgeConfig())
	if err != nil {
		t.Fatal(err)
	}
	j, c := judged(t, run), controlsLog(t)
	judgedAs(c, j, l)
	r, err := report.NewEssays(l)
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
	return report.AdmitEssay(r), report.AdmitRubric(r, "essays.jsonl", "abc", cr, jr)
}

// On a log of admission that reads and judges every essay right, both rules give the stage author,
// with every count they read: the essays by essay, the criteria by criterion of an essay, each once
// over the repeats.
func TestAdmitEssay(t *testing.T) {
	e, r := essayDecisions(t, func(*essay.Log) {}, func(_, _ *wordsjudge.Log, _ *essay.Log) {})
	if e.Stage != admission.StageAuthor || len(e.Why) != 0 || e.Reader != "opus-high" || e.Assessor != "haiku-medium" || e.Set != report.EssaySet || e.Repeats != 2 ||
		e.Illegible.Of != 200 || e.Corrected.Of != 100 || e.Words.Of != 10000 || e.Words.Upper > report.EssayWordsTarget || e.Failed != 0 {
		t.Errorf("the reading's decision is %+v", e)
	}
	if r.Stage != admission.StageAuthor || len(r.Why) != 0 || r.Alarm.Of != 360 || r.Missed.Of != 40 || r.Alarm.Upper > report.RubricTarget ||
		len(r.Criteria) != 2 || r.Criteria[1].KappaText() != "1.00" || r.Feedbacks != 400 || r.Judged != 400 || r.Judge != "sonnet-low" || r.ControlsRight != r.Controls {
		t.Errorf("the rubric's decision is %+v", r)
	}
}

// Each target of the reading that fails, and each log that is not what the rules read, leaves the
// reader out and says why; a reading that failed is no event, and the word errors of an essay count
// no more than its words.
func TestAdmitEssayRefuses(t *testing.T) {
	read := func(l *essay.Log, n int) *essay.CallRecord {
		for i := range l.Calls {
			if c := &l.Calls[i]; c.Essay == n && c.Step == essay.StepRead && c.Repeat == 2 {
				return c
			}
		}
		panic(n)
	}
	for _, c := range []struct {
		name   string
		change func(l *essay.Log)
		why    string
	}{
		{"illegible", func(l *essay.Log) { c := read(l, 3); c.Reading, c.Illegible, c.Error = nil, true, "illegible" }, "1 legible essays were read illegible"},
		{"illegible", func(l *essay.Log) { c := read(l, 3); c.Reading, c.Error = nil, essay.ErrIllegible.Error() }, "1 legible essays were read illegible"},
		{"failed", func(l *essay.Log) { c := read(l, 3); c.Reading, c.Error, c.Invalid = nil, "off the schema", true }, "1 readings failed, and their essays are not measured in every repeat"},
		{"not asked", func(l *essay.Log) { c := read(l, 3); c.Reading, c.NotAsked = nil, true }, "1 readings failed"},
		{"corrected", func(l *essay.Log) { read(l, 4).Reading.MisspelledKept = 0 }, "1 essays had an error of the learner's read corrected"},
		{"small start", func(l *essay.Log) { l.Sheets[4].LowerStarts, read(l, 5).Reading.LowerStarts = 1, 1 }, "1 essays had an error of the learner's read corrected"},
		{"unended", func(l *essay.Log) { l.Sheets[6].Unended, read(l, 7).Reading.Unended = true, true }, "1 essays had an error of the learner's read corrected"},
		{"words", func(l *essay.Log) {
			for n := 1; n <= 200; n++ {
				read(l, n).Reading.WordErrors = 1
			}
		}, "the upper bound of the words read wrong is 0.0"},
		{"prompt", func(l *essay.Log) { l.Run.ReadPrompt = "0123456789abcdef" }, "its reading is of the prompt 0123456789ab"},
		{"readers", func(l *essay.Log) { l.Run.Readers = append(l.Run.Readers, essay.ReaderRecord{Name: "haiku-medium"}) }, "it names 2 readers"},
		{"one call", func(*essay.Log) {}, "it is not a run in two calls"},
		{"set", func(l *essay.Log) { l.Run.Part = "dev" }, "its set is essays-gen-dev, not essays-gen-test"},
		{"refused", func(l *essay.Log) { l.Run.Refused = []runner.Refused{{Sample: "s", Reason: "REFUSAL_SCHEMA"}} }, "it refused 1 samples"},
		{"stopped", func(l *essay.Log) { l.End.Stopped = "the cap: $1" }, "its run stopped short: the cap: $1"},
		{"no end", func(l *essay.Log) { l.End = nil }, "its log stops before the run's end"},
	} {
		after := func(_, _ *wordsjudge.Log, _ *essay.Log) {}
		if c.name == "one call" {
			// The judge reads the feedback of a run in two calls alone: the path changes after it.
			after = func(_, _ *wordsjudge.Log, l *essay.Log) { l.Run.Path = "" }
		}
		e, _ := essayDecisions(t, c.change, after)
		if e.Stage != "" || !strings.Contains(strings.Join(e.Why, "; "), c.why) {
			t.Errorf("%s: the reading's decision is %s %q, without %q", c.name, e.Stage, e.Why, c.why)
		}
	}
	e, _ := essayDecisions(t, func(l *essay.Log) {
		c := read(l, 6)
		c.Reading, c.Error = nil, "timeout"
		read(l, 8).Reading.WordErrors = 80
	}, func(_, _ *wordsjudge.Log, _ *essay.Log) {})
	if e.Failed != 1 || e.Illegible.Events != 0 || e.Words.Events != 50 {
		t.Errorf("a failed reading and errors above the words: %+v", e)
	}
}

// Each target of the rubric that fails, and a judge that is not what the rule reads, leaves the
// assessor out and says why; a false alarm of two repeats is one, and the missed "not met" have no
// target.
func TestAdmitRubricRefuses(t *testing.T) {
	judgement := func(l *essay.Log, n, rep, k int) *bool {
		for i := range l.Calls {
			if c := &l.Calls[i]; c.Essay == n && c.Step == essay.StepAssess && c.Repeat == rep {
				return c.Marks[k].Met
			}
		}
		panic(n)
	}
	none := func(*essay.Log) {}
	same := func(_, _ *wordsjudge.Log, _ *essay.Log) {}
	for _, c := range []struct {
		name     string
		change   func(l *essay.Log)
		judgedAs func(c, j *wordsjudge.Log, l *essay.Log)
		why      string
	}{
		{"alarms", func(l *essay.Log) {
			for n := 1; n <= 10; n++ {
				*judgement(l, n, 1, 0) = false
			}
		}, same, "the upper bound of the false alarms on met criteria is 0.0"},
		{"prompt", func(l *essay.Log) { l.Run.TextPrompt = "0123456789abcdef" }, same, "its assessment is of the prompt 0123456789ab"},
		{"reading's prompt", func(l *essay.Log) { l.Run.ReadPrompt = "0123456789abcdef" }, same, "the text it assesses is read in the prompt 0123456789ab"},
		{"failed", func(l *essay.Log) { l.Calls[3].Error = "timeout" }, same, "1 assessments failed, and their essays are not assessed in every repeat"},
		{"gender", none, func(_, j *wordsjudge.Log, _ *essay.Log) { j.Calls[0].Verdict.Gender = true }, "1 of 400 feedbacks tell the learner's sex, by the judge"},
		{"off task", none, func(_, j *wordsjudge.Log, _ *essay.Log) { j.Calls[1].Verdict.OnTask = false }, "1 of 400 feedbacks leave the task, by the judge"},
		{"not for a child", none, func(_, j *wordsjudge.Log, _ *essay.Log) { j.Calls[2].Verdict.ForAge = false }, "1 of 400 feedbacks are not for a child, by the judge"},
		{"controls", none, func(c, _ *wordsjudge.Log, _ *essay.Log) { c.Calls[0].Verdict = nil }, "the judge is wrong on 1 control phrases"},
		{"not judged", none, func(_, j *wordsjudge.Log, _ *essay.Log) { j.Calls[3].Verdict = nil }, "the judge judged 399 of the 400 feedbacks that came in essays.jsonl"},
		{"another log", none, func(_, j *wordsjudge.Log, _ *essay.Log) { j.Run.SHA256 = "def" }, "the judge's log of essays.jsonl is not of it"},
		{"set", func(l *essay.Log) { l.Run.File = "eval/essays.json" }, same, "its set is essays-test"},
	} {
		_, r := essayDecisions(t, c.change, c.judgedAs)
		if r.Stage != "" || !strings.Contains(strings.Join(r.Why, "; "), c.why) {
			t.Errorf("%s: the rubric's decision is %s %q, without %q", c.name, r.Stage, r.Why, c.why)
		}
	}
	_, r := essayDecisions(t, func(l *essay.Log) {
		*judgement(l, 1, 1, 0), *judgement(l, 1, 2, 0) = false, false
		*judgement(l, 5, 2, 1) = true
		l.Calls[1].Marks[1].Met = nil
		l.Calls[3].Breaches, l.Calls[3].FeedbackWords = []string{essay.BreachSentences}, 2
	}, same)
	// One false alarm of 360 met criteria, on 200 essays, is above the bound: no fewer than ≈ 480 met
	// criteria pass with one.
	if strings.Join(r.Why, "; ") != "the upper bound of the false alarms on met criteria is 0.0131, above 0.01" || r.Alarm.Events != 1 || r.Missed.Events != 1 || r.Unchecked != 1 || r.Broken != 1 {
		t.Errorf("a false alarm of two repeats, a missed \"not met\" and a judgement unchecked: %s %q, %+v %+v %d", r.Stage, r.Why, r.Alarm, r.Missed, r.Unchecked)
	}
	if got := fmt.Sprint(r.Gender); got != "0 of 400" {
		t.Errorf("the sex told is %s", got)
	}
}
