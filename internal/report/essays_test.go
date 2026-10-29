package report

import (
	"bytes"
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/internal/essay"
	"github.com/altnovatrail/tutor/internal/runner"
	"github.com/altnovatrail/tutor/internal/sample"
)

// essaysLog is a log of two essays and two criteria, title and spelling, by one reader in two
// repeats: essay 1 meets both, and essay 2 has no title. The reader takes essay 1's title for unmet
// in the first repeat, a false alarm it does not repeat, misses essay 2's in the first, and the second
// repeat of essay 2 fails. Its first feedback quotes words the essay does not write.
func essaysLog() *essay.Log {
	met := func(v bool) *bool { return &v }
	answer := func(feedback string) string {
		data, _ := json.Marshal(essay.Answer{Text: "x", Criteria: []essay.Judgement{}, Feedback: feedback})
		return string(data)
	}
	return &essay.Log{
		Run: essay.RunRecord{Kind: "run", CallKind: essay.CallKind, Part: "test", File: "eval/essays.json", Seed: 1, Repeats: 2, Prompt: "abcdef0123456789",
			PromptVersion: 1, MaxUSD: 8, Rubric: []sample.Criterion{{ID: "title"}, {ID: "spelling"}},
			Readers: []essay.ReaderRecord{{Name: "opus-high", Model: "claude-opus-5-5", Effort: "high"}}},
		Sheets: []essay.EssayRecord{{Kind: "essay", Essay: 1, Part: "test", Unmet: []string{}}, {Kind: "essay", Essay: 2, Part: "test", Unmet: []string{"title"}}},
		Calls: []essay.CallRecord{
			{Kind: "call", Call: essay.Call{Essay: 1, Reader: "opus-high", Repeat: 1}, Answer: answer(`Try "a robot bakes good bread" here.`),
				Marks:    []essay.Mark{{ID: "title", Met: met(false)}, {ID: "spelling", Met: met(true)}},
				Reading:  &essay.Reading{Words: 10, WordErrors: 1, Misspelled: 0, LowerStarts: 1, LowerStartsKept: 1, Unended: true, UnendedKept: met(true)},
				Breaches: []string{essay.BreachQuote}, FeedbackWords: 6, Usage: runner.Usage{CostUSD: 0.04}},
			{Kind: "call", Call: essay.Call{Essay: 2, Reader: "opus-high", Repeat: 1}, Answer: answer("Good work."),
				Marks:   []essay.Mark{{ID: "title", Met: met(true)}, {ID: "spelling", Met: met(true)}},
				Reading: &essay.Reading{Words: 12, Misspelled: 2, MisspelledKept: 1}, FeedbackWords: 2, Usage: runner.Usage{CostUSD: 0.06}},
			{Kind: "call", Call: essay.Call{Essay: 1, Reader: "opus-high", Repeat: 2}, Answer: answer("Fine."),
				Marks:   []essay.Mark{{ID: "title", Met: met(true)}, {ID: "spelling", Met: met(true)}},
				Reading: &essay.Reading{Words: 10, Unended: true, UnendedKept: met(false)}, FeedbackWords: 1},
			{Kind: "call", Call: essay.Call{Essay: 2, Reader: "opus-high", Repeat: 2}, Error: "timeout",
				Marks: []essay.Mark{{ID: "title", Reason: essay.ReasonFailed}, {ID: "spelling", Reason: essay.ReasonFailed}}},
		},
		End: &runner.End{Kind: "end"},
	}
}

// The measures of О-22 count the false alarms among the judgements of criteria the essays meet and
// the missed "unmet" among the others, κ over every judgement, the criteria judged both ways, the
// reading and the feedback's breaches; a failed call leaves its criteria unchecked.
func TestEssaysReport(t *testing.T) {
	r, err := NewEssays(essaysLog())
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Readers) != 1 {
		t.Fatalf("readers %+v", r.Readers)
	}
	rd := r.Readers[0]
	all, title := rd.All, rd.Criteria[0]
	if all.Judged != 6 || all.Unchecked != 2 || all.FalseAlarm.Events != 1 || all.FalseAlarm.Of != 5 || all.Missed.Events != 1 || all.Missed.Of != 1 || all.Agree != 4 {
		t.Errorf("all: %+v", all)
	}
	if title.Judged != 3 || title.FalseAlarm.Events != 1 || title.FalseAlarm.Of != 2 || title.Missed.Events != 1 || title.Agree != 1 {
		t.Errorf("title: %+v", title)
	}
	// κ of the title: po 1/3, the truth met 2 of 3 and the judgement met 2 of 3, pe 5/9: −0.5.
	if !title.KappaDef || math.Abs(title.Kappa+0.5) > 1e-9 {
		t.Errorf("κ of the title = %v, %v", title.Kappa, title.KappaDef)
	}
	if spelling := rd.Criteria[1]; spelling.KappaDef || spelling.kappaText() != "—" {
		t.Errorf("κ of spelling, all met: %+v", spelling)
	}
	if rd.Calls != 4 || rd.Failed != 1 || rd.Paired != 2 || rd.Unstable != 1 || rd.Feedbacks != 3 || rd.Breaches[essay.BreachQuote] != 1 || rd.MostWords != 6 || math.Abs(rd.MeanWords-3) > 1e-9 {
		t.Errorf("the reader: %+v", rd)
	}
	if rd.Reading.WordErrors != 1 || rd.Reading.Words != 32 || rd.Reading.MisspelledKept != 1 || rd.Reading.LowerStartsKept != 1 {
		t.Errorf("the reading: %+v", rd.Reading)
	}
	var md, table, fb bytes.Buffer
	if err := r.Markdown(&md, "t.jsonl"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"# The essays: the test part",
		"| opus-high | all | 6 | 1 of 5, ≤ ",
		"| opus-high | title | 3 | 1 of 2, ≤ ",
		"| opus-high | 4 | 1 | 1 of 2 | 1 of 32 | 1 of 2 | 1 of 1 | 1 of 2 | 3 | 0 | 0 | 1 | 0 | 1 | 3.0 | 6 | 0.02500 |",
	} {
		if !strings.Contains(md.String(), want) {
			t.Errorf("the report lacks %q:\n%s", want, md.String())
		}
	}
	if err := r.CSV(&table); err != nil {
		t.Fatal(err)
	}
	if lines := strings.Split(strings.TrimSpace(table.String()), "\n"); len(lines) != 9 || lines[1] != "1,test,opus-high,1,title,met,unmet," || lines[8] != "2,test,opus-high,2,spelling,met,unchecked,failed" {
		t.Errorf("the CSV is\n%s", table.String())
	}
	if s := r.Summary(); !strings.Contains(s, "opus-high: false alarms 1 of 5") || !strings.Contains(s, "feedbacks breaking the rule 1 of 3") {
		t.Errorf("the summary is %q", s)
	}
	suffix, label, write := r.Readable()
	if suffix != ".feedback.md" || label != "The feedback to read" {
		t.Errorf("Readable = %q, %q", suffix, label)
	}
	if err := write(&fb, "t.jsonl"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# The feedback of opus-high", "## Essay 1", "- Judged otherwise: title judged unmet.", "- Breaks of the rule: quote.", `> Try "a robot bakes good bread" here.`, "- Unmet by the plan: title."} {
		if !strings.Contains(fb.String(), want) {
			t.Errorf("the feedback lacks %q:\n%s", want, fb.String())
		}
	}
	if strings.Contains(fb.String(), "## Essay 2\n\n- Unmet by the plan: title.\n- Judged otherwise: title judged met.\n- The call failed") {
		t.Error("the feedback shows a call of the second repeat")
	}
	// A feedback breaks the rule of R70 spec as it breaks the measurement's, but for its 80 words: the
	// quote of the first feedback, and the third, of more words than that.
	long := essaysLog()
	long.Calls[2].FeedbackWords = essay.PlatformWords + 1
	if r, _ := NewEssays(long); r.Readers[0].PlatformBroken != 2 || r.Readers[0].MostWords != essay.PlatformWords+1 || !strings.Contains(r.Summary(), "that of R70 spec, with 80 words, 2") {
		t.Errorf("a feedback of %d words: %+v", essay.PlatformWords+1, r.Readers[0])
	}
	// A call of an essay the log does not tell is in no count and no feedback.
	stray := essaysLog()
	stray.Calls = append(stray.Calls, essay.CallRecord{Kind: "call", Call: essay.Call{Essay: 9, Reader: "opus-high", Repeat: 1}})
	r, _ = NewEssays(stray)
	fb.Reset()
	if err := r.Feedback(&fb, "t.jsonl"); err != nil || strings.Contains(fb.String(), "Essay 9") || r.Readers[0].Calls != 4 {
		t.Errorf("a stray call: %v, %d calls\n%s", err, r.Readers[0].Calls, fb.String())
	}
	empty := essaysLog()
	empty.Run.Readers = nil
	r, _ = NewEssays(empty)
	fb.Reset()
	if err := r.Feedback(&fb, "t.jsonl"); err != nil || !strings.Contains(fb.String(), "no reader") {
		t.Errorf("the feedback of no reader: %q, %v", fb.String(), err)
	}
}

// κ is not defined with no judgement, or when the truth and the judgements are all of one kind; it
// is 1 when they agree in all, and 0 when they agree no more than chance.
func TestKappa(t *testing.T) {
	for _, c := range []struct {
		n, agree, trueMet, saidMet int
		want                       float64
		def                        bool
	}{
		{0, 0, 0, 0, 0, false},
		{4, 4, 4, 4, 0, false},
		{4, 4, 2, 2, 1, true},
		{4, 2, 2, 2, 0, true},
	} {
		if got, def := kappa(c.n, c.agree, c.trueMet, c.saidMet); def != c.def || math.Abs(got-c.want) > 1e-9 {
			t.Errorf("kappa(%d, %d, %d, %d) = %v, %v", c.n, c.agree, c.trueMet, c.saidMet, got, def)
		}
	}
}

// In a run of two calls an assessment is the reading and the assessment of its text: the reading
// tells how the model read, the assessment its judgements and feedback, and the cost and the time are
// both together. A reading that failed leaves its assessment not asked and every criterion unchecked.
func TestEssaysTwo(t *testing.T) {
	met := func(v bool) *bool { return &v }
	l := &essay.Log{
		Run: essay.RunRecord{Kind: "run", CallKind: essay.CallKind, Part: "test", File: "f", Seed: 1, Repeats: 1, Prompt: "abcdef0123456789",
			PromptVersion: essay.TwoVersion, Path: essay.PathTwo, MaxUSD: 7, Rubric: []sample.Criterion{{ID: "title"}, {ID: "spelling"}},
			Readers: []essay.ReaderRecord{{Name: "sonnet-low", Model: "claude-sonnet-5-5", Effort: "low"}}},
		Sheets: []essay.EssayRecord{{Kind: "essay", Essay: 1, Part: "test", Unmet: []string{}}, {Kind: "essay", Essay: 2, Part: "test", Unmet: []string{"title"}}},
		Calls: []essay.CallRecord{
			{Kind: "call", Call: essay.Call{Essay: 1, Reader: "sonnet-low", Repeat: 1, Step: essay.StepRead}, Answer: `{"text": "x"}`,
				Reading: &essay.Reading{Words: 10}, Usage: runner.Usage{CostUSD: 0.01, LatencyMs: 1000}},
			{Kind: "call", Call: essay.Call{Essay: 1, Reader: "sonnet-low", Repeat: 1, Step: essay.StepAssess}, Answer: `{"criteria": [], "feedback": "Good."}`,
				Marks: []essay.Mark{{ID: "title", Met: met(true)}, {ID: "spelling", Met: met(false)}}, FeedbackWords: 5, Usage: runner.Usage{CostUSD: 0.02, LatencyMs: 2000}},
			{Kind: "call", Call: essay.Call{Essay: 2, Reader: "sonnet-low", Repeat: 1, Step: essay.StepRead}, Error: "timeout", Usage: runner.Usage{CostUSD: 0.01, LatencyMs: 500}},
			{Kind: "call", Call: essay.Call{Essay: 2, Reader: "sonnet-low", Repeat: 1, Step: essay.StepAssess}, NotAsked: true, Error: "no text",
				Marks: []essay.Mark{{ID: "title", Reason: essay.ReasonFailed}, {ID: "spelling", Reason: essay.ReasonFailed}}},
		},
		End: &runner.End{Kind: "end"},
	}
	r, err := NewEssays(l)
	if err != nil {
		t.Fatal(err)
	}
	rd := r.Readers[0]
	if rd.Calls != 2 || rd.Failed != 1 || rd.All.Judged != 2 || rd.All.Unchecked != 2 || rd.All.FalseAlarm.Events != 1 || rd.Feedbacks != 1 || rd.MeanWords != 5 ||
		rd.Reading.Words != 10 || math.Abs(rd.MeanUSD-0.02) > 1e-9 || rd.TimeP50 != 500 || rd.TimeP95 != 3000 {
		t.Errorf("the reader of two calls: %+v", rd)
	}
	var md bytes.Buffer
	if err := r.Markdown(&md, "t.jsonl"); err != nil || !strings.Contains(md.String(), "The run assesses in two calls") || !strings.Contains(md.String(), "| 0.02000 | 500 / 3000 |") {
		t.Errorf("the report of two calls (%v):\n%s", err, md.String())
	}
	// The feedback to read shows each assessment once, with its feedback; the CSV has every criterion of
	// the pair whose reading failed, unchecked.
	var fb, table bytes.Buffer
	if err := r.Feedback(&fb, "t.jsonl"); err != nil || strings.Count(fb.String(), "## Essay 1") != 1 || !strings.Contains(fb.String(), "> Good.") {
		t.Errorf("the feedback of two calls (%v):\n%s", err, fb.String())
	}
	if err := r.CSV(&table); err != nil || !strings.Contains(table.String(), "2,test,sonnet-low,1,title,unmet,unchecked,failed") || strings.Count(table.String(), "\n") != 5 {
		t.Errorf("the CSV of two calls (%v):\n%s", err, table.String())
	}
	// The one-call report has no column of time.
	one, _ := NewEssays(essaysLog())
	var plain bytes.Buffer
	if err := one.Markdown(&plain, "t.jsonl"); err != nil || strings.Contains(plain.String(), "Time p50") {
		t.Errorf("the report of one call shows the time (%v)", err)
	}
}
