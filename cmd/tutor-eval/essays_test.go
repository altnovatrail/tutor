package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/internal/sample"
)

// The essays (T82; docs/architecture.md, 3.4) and the document that shows them, as seen from this
// package's directory.
const (
	theEssays = "../../eval/essays.json"
	essaysDoc = "../../docs/essays.md"
)

// tutor-eval check takes the essays, and tutor-eval essays sums them up: the essays and their parts,
// the criteria unmet in each part, their words, sentences and lines, and the crops of their pages
// at 32 px, within the tokens Claude takes without scaling an image down.
func TestEssaysCommand(t *testing.T) {
	code, out, errs := runIt("check", theEssays)
	if want := theEssays + ": ok, 20 essays, 5 of them dev and 15 test\n"; code != exitOK || out != want {
		t.Errorf("check = %d %q %q, want %q", code, out, errs, want)
	}
	code, out, errs = runIt("essays", "-file", theEssays)
	for _, want := range []string{
		theEssays + ": 20 essays, 5 of them dev and 15 test; seed 1.\n",
		"Unmet criteria, dev and test: title 1 and 4; sentences 1 and 4; capitals-and-stops 1 and 4; linking-words 1 and 4; on-topic 1 and 4; spelling 1 and 4; none 1 and 3.\n",
		"Words 52 to 74, 64.0 on average; sentences 3 to 9, 5.8 on average; lines 9 to 14, 11.4 on average.\n",
		"Crops at 32 px: digits drawn 23 to 32 px, 27.4 on average; width 864 to 1216 px, 1045.5 on average; height 671 to 1330 px, 1069.0 on average; tokens 936 to 1564, 1444.7 on average.\n",
	} {
		if code != exitOK || !strings.Contains(out, want) {
			t.Errorf("essays = %d %q %q, want %q in it", code, out, errs, want)
		}
	}
	if code, _, errs := runIt("essays", "-file", theWorkbook); code != exitFail || !strings.Contains(errs, `unknown field "pages"`) {
		t.Errorf("essays on the workbook = %d %q", code, errs)
	}
	// An essay that lists a criterion it meets fails the check, in both commands.
	wrong := filepath.Join(t.TempDir(), "wrong.json")
	text := strings.Replace(readDoc(t, theEssays), `"unmet": [],`, `"unmet": ["title"],`, 1)
	if err := os.WriteFile(wrong, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"check", wrong}, {"essays", "-file", wrong}} {
		if code, _, errs := runIt(args...); code != exitFail || !strings.Contains(errs, "essays[0].unmet: lists the criterion title") {
			t.Errorf("%s on an essay that lists a criterion it meets = %d %q", args[0], code, errs)
		}
	}
}

// With -png the command saves the crop of every page at 32 px, and saves them again over the crops of
// a run before.
func TestEssaysPictures(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "png")
	code, out, errs := runIt("essays", "-file", theEssays, "-png", dir)
	if code != exitOK || !strings.HasSuffix(out, "Crops of the pages at 32 px: 20 files, into "+dir+".\n") {
		t.Fatalf("essays -png = %d %q %q", code, out, errs)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 20 {
		t.Fatalf("the crops are %d files, %v", len(entries), err)
	}
	f, err := os.Open(filepath.Join(dir, "essay-01.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if cfg, err := png.DecodeConfig(f); err != nil || cfg.Width < 800 || cfg.Height < 600 {
		t.Errorf("the crop of essay 1 is %+v, %v", cfg, err)
	}
	if code, _, errs := runIt("essays", "-file", theEssays, "-png", dir); code != exitOK {
		t.Errorf("essays -png over the crops = %d %q", code, errs)
	}
	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.MkdirAll(filepath.Join(blocked, "essay-01.png"), 0o700); err != nil {
		t.Fatal(err)
	}
	if code, _, errs := runIt("essays", "-file", theEssays, "-png", blocked); code != exitFail || !strings.Contains(errs, "essay 1") {
		t.Errorf("essays -png where a crop cannot go = %d %q", code, errs)
	}
}

func TestEssaysRefuses(t *testing.T) {
	for _, c := range []struct {
		name string
		args []string
		says string
	}{
		{"no directory for the crops", []string{"-png", ""}, "-png is empty"},
		// The tests run in cmd/tutor-eval, where png is a folder git does not ignore.
		{"the crops in the work tree", []string{"-png", "png"}, "cmd/tutor-eval/png is in the git work tree"},
		{"the crops among the author's sheets", []string{"-png", "../../samples/essays-test"}, "outside its synthetic folder"},
	} {
		t.Run(c.name, func(t *testing.T) {
			code, _, errs := runIt(append([]string{"essays", "-file", theEssays}, c.args...)...)
			if code != exitUsage || !strings.Contains(errs, c.says) {
				t.Errorf("essays = %d, %q; want %d and %q", code, errs, exitUsage, c.says)
			}
		})
	}
	for _, made := range []string{"png", "../../samples/essays-test"} {
		if _, err := os.Stat(made); !os.IsNotExist(err) {
			t.Errorf("a refused essays made %s: %v", made, err)
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

// criterionNames say in Russian what each criterion means, and who checks its truth.
var criterionNames = map[string][2]string{
	sample.CriterionTitle:     {"у сочинения есть заголовок, на своей первой строке", "программа: заголовок есть — от 1 до 6 слов, с заглавной буквы, без знака в конце"},
	sample.CriterionSentences: {"в сочинении не меньше пяти предложений", "программа: предложения считаются по знакам конца"},
	sample.CriterionCapitals:  {"каждое предложение начинается с заглавной буквы и кончается точкой, вопросительным или восклицательным знаком", "программа"},
	sample.CriterionLinking:   {"мысли связаны словами вроде because, but, so, then или first", "программа, по закрытому списку связующих слов"},
	sample.CriterionOnTopic:   {"сочинение держится темы задачи", "только план и вычитка автора"},
	sample.CriterionSpelling:  {"частые слова написаны без ошибок", "программа: слова с ошибкой перечислены в файле и стоят в тексте"},
}

// essaysRubricTable writes the table of docs/essays.md with the criteria of the rubric: the ID, the
// text a model reads, what it means and who checks its truth.
func essaysRubricTable(es *sample.Essays) string {
	var b strings.Builder
	b.WriteString("| ID | Текст критерия | Что значит | Истину сверяет |\n|---|---|---|---|\n")
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
	b.WriteString("| Критерий | Не выполнен | Разработка | Тест |\n|---|---|---|---|\n")
	for _, id := range sample.Criteria {
		n := unmet[id]
		fmt.Fprintf(&b, "| `%s` | %d | %d | %d |\n", id, n[0]+n[1], n[0], n[1])
	}
	n := unmet[""]
	fmt.Fprintf(&b, "| **выполнены все** | %d | %d | %d |", n[0]+n[1], n[0], n[1])
	return b.String()
}

// essaysHeading heads the last section of docs/essays.md, the essays one by one.
const essaysHeading = "## Сочинения по одному\n"

// essaysSection writes the last section of docs/essays.md: each essay, its part, topic, words and
// sentences, the criteria it does not meet and why, and its title and text as written.
func essaysSection(es *sample.Essays) string {
	var b strings.Builder
	b.WriteString(essaysHeading)
	for _, e := range es.Essays {
		title := "без заголовка"
		if e.Title != nil {
			title = *e.Title
		}
		fmt.Fprintf(&b, "\n### %d. %s\n\n", e.Number, title)
		fmt.Fprintf(&b, "- **Часть:** %s.\n", map[string]string{sample.PartDev: "разработка", sample.PartTest: "тест"}[e.Part])
		fmt.Fprintf(&b, "- **Тема:** %s\n", e.Topic)
		fmt.Fprintf(&b, "- **Слов** — %d, **предложений** — %d.\n", len(sample.Words(e.Text)), len(sample.Sentences(e.Text)))
		if len(e.Unmet) == 0 {
			b.WriteString("- **Не выполнено:** ничего.\n")
		} else {
			b.WriteString("- **Не выполнено:**\n")
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

// whyUnmet says in Russian why an essay does not meet a criterion.
func whyUnmet(e *sample.Essay, id string) string {
	switch id {
	case sample.CriterionTitle:
		return "заголовка нет"
	case sample.CriterionSentences:
		return fmt.Sprintf("%d предложения", len(sample.Sentences(e.Text)))
	case sample.CriterionCapitals:
		var why []string
		ss := sample.Sentences(e.Text)
		for _, i := range sample.LowerStarts(e.Text) {
			words := sample.Words(ss[i-1])
			why = append(why, fmt.Sprintf("предложение %d начинается со строчной: «%s …»", i, strings.Join(words[:min(2, len(words))], " ")))
		}
		if sample.Unended(e.Text) {
			why = append(why, "последнее предложение — без знака в конце")
		}
		return strings.Join(why, ", ")
	case sample.CriterionLinking:
		return "ни одного связующего слова, и даже and и or нет"
	case sample.CriterionOnTopic:
		return "всё сочинение — о другом"
	case sample.CriterionSpelling:
		var why []string
		for _, m := range e.Misspelled {
			why = append(why, fmt.Sprintf("%s вместо %s", m.Written, m.Word))
		}
		return "с ошибкой: " + strings.Join(why, ", ")
	}
	return ""
}
