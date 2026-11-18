package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"

	"github.com/altnovatrail/tutor/internal/protocol"
	"github.com/altnovatrail/tutor/internal/report"
	"github.com/altnovatrail/tutor/internal/sample"
)

// The essays and the document that shows them, as seen from this package's directory.
const (
	theEssays = "../../eval/essays.json"
	essaysDoc = "../../docs/essays.md"
)

// tutor-eval check takes the essays, and tutor-eval essays sums them up with their samples: the
// essays and their parts, the seed of the samples, the criteria unmet in each part, the samples read,
// their words, sentences and lines, and the crops of their pages at 32 px, within the tokens Claude
// takes without scaling an image down, the same as the essays drawn in memory.
func TestEssaysCommand(t *testing.T) {
	t.Parallel()
	code, out, errs := runIt("check", theEssays)
	if want := theEssays + ": ok, 20 essays, 5 of them dev and 15 test\n"; code != exitOK || out != want {
		t.Errorf("check = %d %q %q, want %q", code, out, errs, want)
	}
	dir := drawnOf(t, theEssays)
	code, out, errs = runIt("essays", "-file", theEssays, "-dir", dir)
	for _, want := range []string{
		theEssays + ": 20 essays, 5 of them dev and 15 test; seed 1.\n",
		"Samples in " + dir + ": 20 of 20 read; not drawn: none; refused, and nothing of them goes to AI: none; off their file: none.\n",
		"Unmet criteria, dev and test: title 1 and 4; sentences 1 and 4; capitals-and-stops 1 and 4; linking-words 1 and 4; on-topic 1 and 4; spelling 1 and 4; none 1 and 3.\n",
		"Words 52 to 74, 64.0 on average; sentences 3 to 9, 5.8 on average; lines 9 to 14, 11.4 on average.\n",
		"Crops at 32 px: digits drawn 23 to 32 px, 27.4 on average; width 864 to 1216 px, 1045.5 on average; height 671 to 1330 px, 1069.0 on average; tokens 936 to 1564, 1444.7 on average.\n",
	} {
		if code != exitOK || !strings.Contains(out, want) {
			t.Errorf("essays = %d %q %q, want %q in it", code, out, errs, want)
		}
	}
	if code, _, errs := runIt("essays", "-file", theWorkbook, "-dir", t.TempDir()); code != exitFail || !strings.Contains(errs, `unknown field "pages"`) {
		t.Errorf("essays on the workbook = %d %q", code, errs)
	}
	// An essay that lists a criterion it meets fails the check, in both commands.
	wrong := filepath.Join(t.TempDir(), "wrong.json")
	text := strings.Replace(readDoc(t, theEssays), `"unmet": [],`, `"unmet": ["title"],`, 1)
	if err := os.WriteFile(wrong, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"check", wrong}, {"essays", "-file", wrong, "-dir", t.TempDir()}} {
		if code, _, errs := runIt(args...); code != exitFail || !strings.Contains(errs, "essays[0].unmet: lists the criterion title") {
			t.Errorf("%s on an essay that lists a criterion it meets = %d %q", args[0], code, errs)
		}
	}
}

// The rubric, the counts and the essays of docs/essays.md are the file of the essays, byte for byte.
func TestEssaysDoc(t *testing.T) {
	es, err := sample.ReadEssays(strings.NewReader(readDoc(t, theEssays)))
	if err != nil {
		t.Fatal(err)
	}
	doc := readDoc(t, essaysDoc)
	sameTable(t, essaysDoc, doc, theEssays, essaysRubricTable(es))
	sameTable(t, essaysDoc, doc, theEssays, essaysCountTable(es))
	built := essaysSection(es)
	if at := strings.Index(doc, essaysHeading); at < 0 || doc[at:] != built {
		t.Errorf("the section %q of %s is not the essays of %s; built from it, it is\n%s", essaysHeading, essaysDoc, theEssays, built)
	}
}

// criterionNames say what each criterion means, and who checks its truth.
var criterionNames = map[string][2]string{
	sample.CriterionTitle:     {"the essay has a title, on a line of its own, the first", "the program: a title is there — 1 to 6 words, with a capital letter, with no mark at the end"},
	sample.CriterionSentences: {"the essay has no fewer than five sentences", "the program: sentences are counted by the marks at their ends"},
	sample.CriterionCapitals:  {"every sentence starts with a capital letter and ends with a full stop, a question mark or an exclamation mark", "the program"},
	sample.CriterionLinking:   {"the thoughts are linked by words such as because, but, so, then or first", "the program, by a closed list of linking words"},
	sample.CriterionOnTopic:   {"the essay keeps to the topic of the task", "only the plan and the author's proofreading"},
	sample.CriterionSpelling:  {"common words are spelled without mistakes", "the program: the misspelled words are listed in the file and stand in the text"},
}

// essaysRubricTable writes the table of docs/essays.md with the criteria of the rubric: the ID, the
// text a model reads, what it means and who checks its truth.
func essaysRubricTable(es *sample.Essays) string {
	var b strings.Builder
	b.WriteString("| ID | Text of the criterion | What it means | Who checks the truth |\n|---|---|---|---|\n")
	for _, c := range es.Rubric {
		fmt.Fprintf(&b, "| `%s` | %s | %s | %s |\n", c.ID, c.Text, criterionNames[c.ID][0], criterionNames[c.ID][1])
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// essaysCountTable writes the table of docs/essays.md that counts, for each criterion, the essays
// that do not meet it, in all and in each part, and the essays that meet them all.
func essaysCountTable(es *sample.Essays) string {
	unmet := es.Tally()
	var b strings.Builder
	b.WriteString("| Criterion | Not met | Dev | Test |\n|---|---|---|---|\n")
	for _, id := range sample.Criteria {
		n := unmet[id]
		fmt.Fprintf(&b, "| `%s` | %d | %d | %d |\n", id, n[0]+n[1], n[0], n[1])
	}
	n := unmet[""]
	fmt.Fprintf(&b, "| **all met** | %d | %d | %d |", n[0]+n[1], n[0], n[1])
	return b.String()
}

// essaysHeading heads the last section of docs/essays.md, the essays one by one.
const essaysHeading = "## The essays one by one\n"

// essaysSection writes the last section of docs/essays.md: each essay, its part, topic, words and
// sentences, the criteria it does not meet and why, and its title and text as written.
func essaysSection(es *sample.Essays) string {
	var b strings.Builder
	b.WriteString(essaysHeading)
	for _, e := range es.Essays {
		title := "no title"
		if e.Title != nil {
			title = *e.Title
		}
		fmt.Fprintf(&b, "\n### %d. %s\n\n", e.Number, title)
		fmt.Fprintf(&b, "- **Part:** %s.\n", map[string]string{sample.PartDev: "dev", sample.PartTest: "test"}[e.Part])
		fmt.Fprintf(&b, "- **Topic:** %s\n", e.Topic)
		fmt.Fprintf(&b, "- **Words** — %d, **sentences** — %d.\n", len(sample.Words(e.Text)), len(sample.Sentences(e.Text)))
		if len(e.Unmet) == 0 {
			b.WriteString("- **Not met:** none.\n")
		} else {
			b.WriteString("- **Not met:**\n")
			for i, id := range e.Unmet {
				end := ";"
				if i == len(e.Unmet)-1 {
					end = "."
				}
				fmt.Fprintf(&b, "  - `%s` — %s%s\n", id, whyUnmet(&e, id), end)
			}
		}
		b.WriteString("\n")
		if e.Title != nil {
			fmt.Fprintf(&b, "> %s\n>\n", *e.Title)
		}
		fmt.Fprintf(&b, "> %s\n", e.Text)
	}
	return b.String()
}

// whyUnmet says why an essay does not meet a criterion.
func whyUnmet(e *sample.Essay, id string) string {
	switch id {
	case sample.CriterionTitle:
		return "there is no title"
	case sample.CriterionSentences:
		return fmt.Sprintf("%d sentences", len(sample.Sentences(e.Text)))
	case sample.CriterionCapitals:
		var why []string
		ss := sample.Sentences(e.Text)
		for _, i := range sample.LowerStarts(e.Text) {
			words := sample.Words(ss[i-1])
			why = append(why, fmt.Sprintf("sentence %d starts with a small letter: \"%s …\"", i, strings.Join(words[:min(2, len(words))], " ")))
		}
		if sample.Unended(e.Text) {
			why = append(why, "the last sentence has no mark at the end")
		}
		return strings.Join(why, ", ")
	case sample.CriterionLinking:
		return "not one linking word, not even and or or"
	case sample.CriterionOnTopic:
		return "the whole essay is about something else"
	case sample.CriterionSpelling:
		var why []string
		for _, m := range e.Misspelled {
			why = append(why, fmt.Sprintf("%s instead of %s", m.Written, m.Word))
		}
		return "misspelled: " + strings.Join(why, ", ")
	}
	return ""
}

// A run of the assessment by the rubric reads the essays of a part, a call an essay, by every reader
// in every repeat; its report counts no false alarm on the stub and writes the feedback to read. It
// takes no flag of the sheets, no file of the workbook and no -reads.
func TestRunEssays(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	log := filepath.Join(dir, "essays.jsonl")
	drawn := drawnOf(t, theEssays)
	code, out, errs := runMeasure("yes\n", "-kind", "essay", "-essays", theEssays, "-dir", drawn, "-part", "dev", "-stub", "-repeats", "2", "-log", log)
	if code != exitOK || !strings.HasPrefix(out, "The dev part: 5 essays of "+theEssays+", drawn with the seed 1 into "+drawn+", at 32 px, by opus-high and haiku-low and haiku-low-no-thinking in 2 repeats, a call an essay, in version 1 of the prompt of the assessment by the rubric.\n30 calls, expected to cost $") {
		t.Fatalf("the run ends %d:\n%s%s", code, out, errs)
	}
	// The test part of the essays is planned at $8, not the $6 of the sheets (R45).
	if code, out, _ := runMeasure("no\n", "-kind", "essay", "-essays", theEssays, "-dir", drawn, "-part", "test", "-stub", "-log", filepath.Join(dir, "test.jsonl")); code != exitOK || !strings.Contains(out, "The cap is $8.00.\n") || strings.Contains(out, "above the") {
		t.Errorf("the test part of the essays: %d\n%s", code, out)
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(data), `"kind":"essay"`); n != 5 || strings.Count(string(data), `"kind":"call"`) != 30 {
		t.Errorf("the log holds %d essays:\n%.300s", n, data)
	}
	code, out, errs = runIt("report", log)
	if code != exitOK || !strings.Contains(out, "haiku-low: false alarms 0 of 48") || !strings.Contains(out, "The feedback to read: "+filepath.Join(dir, "essays.feedback.md")+"\n") {
		t.Errorf("the report of the essays ends %d: %s%s", code, out, errs)
	}
	reportsBeside(t, log, evalv1.Capability_CAPABILITY_RUBRIC_ASSESSMENT, "opus-high", "haiku-low", "haiku-low-no-thinking")
	for _, c := range []struct {
		args []string
		says string
	}{
		{[]string{"-plan", "x"}, "-plan names sheets, and the essay reads its own file"},
		{[]string{"-workbook", theWorkbook}, "-workbook is not for -kind essay"},
		{[]string{"-reads", "captions"}, "-reads is for -kind reading or workbook"},
	} {
		args := append([]string{"-kind", "essay", "-dir", drawn, "-part", "dev", "-stub"}, c.args...)
		if code, _, errs := runMeasure("yes\n", args...); code != exitUsage || !strings.Contains(errs, c.says) {
			t.Errorf("%v: ends %d, %q; want %q", c.args, code, errs, c.says)
		}
	}
	if code, _, errs := runMeasure("yes\n", "-kind", "essay", "-essays", theWorkbook, "-dir", drawn, "-part", "dev", "-stub", "-log", filepath.Join(dir, "x.jsonl")); code != exitFail || !strings.Contains(errs, `unknown field "pages"`) {
		t.Errorf("a run of the essays on the workbook ends %d, %q", code, errs)
	}
}

// A run of the essays in two calls on the stub goes the whole way, the reading and the assessment of
// its text for every essay and reader, and its report says so; -two is refused for another kind.
func TestRunEssaysTwo(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "two.jsonl")
	code, out, errs := runMeasure("yes\n", "-kind", "essay", "-two", "-essays", theEssays, "-dir", drawnOf(t, theEssays), "-part", "dev", "-stub", "-repeats", "1", "-readers", "sonnet-low", "-log", log)
	if code != exitOK || !strings.Contains(out, "by sonnet-low in 1 repeat, two calls an essay, the reading and the assessment of the text it read, in version 3 of the prompts of the assessment in two calls.\n10 calls") {
		t.Fatalf("the run in two calls ends %d:\n%s%s", code, out, errs)
	}
	if code, out, errs := runIt("report", log); code != exitOK || !strings.Contains(out, "sonnet-low: false alarms 0 of") {
		t.Errorf("the report of two calls ends %d: %s%s", code, out, errs)
	}
	// The judge of the words judges the feedback of a run in two calls, with the topics of its essays.
	if code, out, errs := runJudge("yes\n", func(string) string { return "" }, "-log", log, "-stub"); code != exitOK || !strings.Contains(out, "The feedback of "+log+" that came, each judged once") {
		t.Errorf("the judge of the feedback ends %d: %s%s", code, out, errs)
	}
	// A log whose essays are not those of the file it names is refused: the topics would be another's.
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(dir, "other.jsonl")
	if err := os.WriteFile(other, []byte(strings.Replace(string(data), `"kind":"essay","essay":1,"part":"dev"`, `"kind":"essay","essay":1,"part":"test"`, 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, errs := runJudge("yes\n", func(string) string { return "" }, "-log", other, "-stub"); code != exitFail || !strings.Contains(errs, "is not the one of") {
		t.Errorf("a log of other essays ends %d: %s", code, errs)
	}
	if code, _, errs := runMeasure("yes\n", "-kind", "reading", "-two", "-part", "dev", "-stub"); code != exitUsage || !strings.Contains(errs, "-two is for -kind essay") {
		t.Errorf("-two for the reading ends %d: %s", code, errs)
	}
}

// The run of admission of the essay reads by one configuration and assesses the text read by another,
// -assessor, and its log and its runner's report name both; admit -kind essay rules on its log, with
// the judge's logs of its feedback and of its control phrases, for the reading and for the rubric
// apart. On the dev part of eval/essays.json both are refused by their set alone.
func TestAdmitEssayRun(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "admission.jsonl")
	code, out, errs := runMeasure("yes\n", "-kind", "essay", "-two", "-essays", theEssays, "-dir", drawnOf(t, theEssays), "-part", "dev", "-stub", "-repeats", "2", "-readers", "opus-high", "-assessor", "haiku-medium", "-log", log)
	if code != exitOK || !strings.Contains(out, "by opus-high in 2 repeats, two calls an essay, the reading and the assessment of the text it read by haiku-medium, in version 3") {
		t.Fatalf("the run of admission ends %d:\n%s%s", code, out, errs)
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"assessor":{"name":"haiku-medium","model":"claude-haiku-5-5"`) {
		t.Errorf("the log names no assessor:\n%.600s", data)
	}
	if code, out, errs := runIt("report", log); code != exitOK || !strings.Contains(out, "opus-high: false alarms 0 of") {
		t.Fatalf("the report ends %d: %s%s", code, out, errs)
	}
	if md, err := os.ReadFile(strings.TrimSuffix(log, ".jsonl") + ".md"); err != nil || !strings.Contains(string(md), "The text every reader read is assessed by haiku-medium (claude-haiku-5-5, medium), a configuration of its own") {
		t.Errorf("the report does not name the assessor (%v):\n%.600s", err, md)
	}
	reportsBeside(t, log, evalv1.Capability_CAPABILITY_RUBRIC_ASSESSMENT, "opus-high")
	data, err = os.ReadFile(report.ReportFile(strings.TrimSuffix(log, ".jsonl"), "opus-high"))
	if err != nil {
		t.Fatal(err)
	}
	rep, err := protocol.ReadReport(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if rep.GetReaders()[0].GetModel() != "claude-opus-5-5 high" || rep.GetAssessor().GetModel() != "claude-haiku-5-5 medium" {
		t.Errorf("the runner's report names the readers %v and the assessor %v", rep.GetReaders(), rep.GetAssessor())
	}
	controls := filepath.Join(dir, "controls.jsonl")
	if code, _, errs := runJudge("yes\n", func(string) string { return "" }, "-controls", wordsJudge, "-out", controls, "-stub"); code != exitOK {
		t.Fatalf("the judge of the control phrases ends %d: %s", code, errs)
	}
	if code, _, errs := runIt("admit", "-kind", "essay", "-controls", controls, log); code != exitFail || !strings.Contains(errs, "the judge's log of "+log) {
		t.Errorf("admit without the judge's log of the feedback ends %d: %s", code, errs)
	}
	if code, _, errs := runJudge("yes\n", func(string) string { return "" }, "-log", log, "-stub"); code != exitOK {
		t.Fatalf("the judge of the feedback ends %d: %s", code, errs)
	}
	code, out, errs = runIt("admit", "-kind", "essay", "-controls", controls, log)
	if code != exitOK {
		t.Fatalf("admit -kind essay ends %d: %s", code, errs)
	}
	for _, want := range []string{
		"## admission.jsonl\n\n- reader: opus-high; assessor: haiku-medium\n- set: essays-dev, 2 repeats\n- admission.jsonl: sha256 ",
		"- admission.judge.jsonl, the judge's of its feedback: sha256 ",
		"- controls.jsonl, the judge's of its control phrases: sha256 ",
		"### The reading of an essay, CallEssay\n\n- stage: none\n- not admitted: its set is essays-dev, not essays-gen-test, the test part of eval/essays-gen.json\n",
		"- legible essays read illegible: 0 of 5\n",
		"- words read wrong: 0 of ",
		"### The assessment by the rubric, CallRubric\n\n- stage: none\n- not admitted: its set is essays-dev",
		"- false alarms, met criteria judged unmet: 0 of ",
		"- κ by criterion, without a target: title ",
		"- the feedback in English alone (R124); the judge sonnet-low, right on ",
		"judged 10 of the 10 feedbacks that came, 0 without a verdict\n",
		"- the judge: the learner's sex told 0 of 10, off the task 0 of 10, not for a child 0 of 10\n",
		"- feedback breaking the rule of R70 spec, without a target: 0 of 10\n\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("admit -kind essay says\n%s\nwithout %q", out, want)
		}
	}
	for _, c := range []struct {
		args []string
		code int
		says string
	}{
		{[]string{"-kind", "essay", log}, exitUsage, "-kind essay needs it"},
		{[]string{"-controls", controls, log}, exitUsage, "-controls is for -kind essay"},
		{[]string{"-kind", "essay", "-controls", filepath.Join(dir, "none.jsonl"), log}, exitFail, "none.jsonl"},
		{[]string{"-kind", "essay", "-controls", controls, controls}, exitFail, "controls.jsonl"},
	} {
		if code, _, errs := runIt(append([]string{"admit"}, c.args...)...); code != c.code || !strings.Contains(errs, c.says) {
			t.Errorf("admit %q ends %d: %s", c.args, code, errs)
		}
	}
	for _, c := range []struct {
		args []string
		code int
		says string
	}{
		{[]string{"-assessor", "haiku-medium"}, exitUsage, "-assessor is for -kind essay -two"},
		{[]string{"-two", "-assessor", "gpt-x"}, exitFail, "-assessor: "},
		{[]string{"-two", "-assessor", " "}, exitUsage, "-assessor is empty"},
	} {
		args := append([]string{"-kind", "essay", "-essays", theEssays, "-dir", drawnOf(t, theEssays), "-part", "dev", "-stub", "-log", filepath.Join(dir, "x.jsonl")}, c.args...)
		if code, _, errs := runMeasure("yes\n", args...); code != c.code || !strings.Contains(errs, c.says) {
			t.Errorf("run %q ends %d: %s", c.args, code, errs)
		}
	}
}
