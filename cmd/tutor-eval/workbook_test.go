package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/value"
)

// The pages of the workbook (T78; docs/architecture.md, 3.4) and the document that shows them as
// tables, as seen from this package's directory.
const (
	theWorkbook = "../../eval/workbook.json"
	workbookDoc = "../../docs/workbook.md"
)

// tutor-eval check takes the pages of the workbook, and tutor-eval workbook sums them up: the
// pages and their parts, the boxes by scenario, and the crops of the filled boxes at each height,
// 640 tokens a page of ten crops of 200 × 200.
func TestWorkbookCommand(t *testing.T) {
	code, out, errs := runIt("check", theWorkbook)
	if want := theWorkbook + ": ok, workbook of 20 pages, 5 of them dev and 15 test; 200 items, 220 boxes\n"; code != exitOK || out != want {
		t.Errorf("check = %d %q %q, want %q", code, out, errs, want)
	}
	code, out, errs = runIt("workbook", "-file", theWorkbook)
	for _, want := range []string{
		theWorkbook + ": 20 pages, 5 of them dev and 15 test; 200 items, 220 boxes; seed 1.\n",
		"Boxes by scenario: correct 100 (",
		"; blank 20 (number 11, fraction 9: stacked 0, inline 0).\n",
		"Crops at 32 px: 200 crops, 0 of them larger than 200 × 200, at most 200 px wide and 200 px high; 640 tokens of crops a page, at most 640.\n",
	} {
		if code != exitOK || !strings.Contains(out, want) {
			t.Errorf("workbook = %d %q %q, want %q in it", code, out, errs, want)
		}
	}
	if code, _, errs := runIt("workbook", "-file", measurementPlan); code != exitFail || !strings.Contains(errs, `unknown field "sheets"`) {
		t.Errorf("workbook on a sheet plan = %d %q", code, errs)
	}
	// A line wider than the page passes the check of the format, and both commands refuse it.
	wide := filepath.Join(t.TempDir(), "wide.json")
	text := strings.Replace(readDoc(t, theWorkbook), `"text": "6 × [ ] = 30"`, `"text": "`+strings.Repeat("очень длинная строка пункта ", 3)+`[ ]"`, 1)
	if err := os.WriteFile(wide, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"check", wide}, {"workbook", "-file", wide}} {
		if code, _, errs := runIt(args...); code != exitFail || !strings.Contains(errs, "pages[0].items[0].text: the line") {
			t.Errorf("%s on a line wider than the page = %d %q", args[0], code, errs)
		}
	}
}

// With -png the command saves the crop of every filled box at 32 px, 200 × 200, and saves them again
// over the crops of a run before.
func TestWorkbookPictures(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "png")
	code, out, errs := runIt("workbook", "-file", theWorkbook, "-png", dir)
	if code != exitOK || !strings.HasSuffix(out, "Crops of the filled boxes at 32 px: 200 files, into "+dir+".\n") {
		t.Fatalf("workbook -png = %d %q %q", code, out, errs)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 200 {
		t.Fatalf("the crops are %d files, %v", len(entries), err)
	}
	f, err := os.Open(filepath.Join(dir, "page-01-item-05-box-2.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if cfg, err := png.DecodeConfig(f); err != nil || cfg.Width != 200 || cfg.Height != 200 {
		t.Errorf("the crop of page 1, item 5, box 2 is %+v, %v", cfg, err)
	}
	if code, _, errs := runIt("workbook", "-file", theWorkbook, "-png", dir); code != exitOK {
		t.Errorf("workbook -png over the crops = %d %q", code, errs)
	}
	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.MkdirAll(filepath.Join(blocked, "page-01-item-01-box-1.png"), 0o700); err != nil {
		t.Fatal(err)
	}
	if code, _, errs := runIt("workbook", "-file", theWorkbook, "-png", blocked); code != exitFail || !strings.Contains(errs, "page 1") {
		t.Errorf("workbook -png where a crop cannot go = %d %q", code, errs)
	}
}

func TestWorkbookRefuses(t *testing.T) {
	for _, c := range []struct {
		name string
		args []string
		says string
	}{
		{"no directory for the crops", []string{"-png", ""}, "-png is empty"},
		// The tests run in cmd/tutor-eval, where png is a folder git does not ignore.
		{"the crops in the work tree", []string{"-png", "png"}, "cmd/tutor-eval/png is in the git work tree"},
		{"the crops among the author's sheets", []string{"-png", "../../samples/workbook-test"}, "outside its synthetic folder"},
	} {
		t.Run(c.name, func(t *testing.T) {
			code, _, errs := runIt(append([]string{"workbook", "-file", theWorkbook}, c.args...)...)
			if code != exitUsage || !strings.Contains(errs, c.says) {
				t.Errorf("workbook = %d, %q; want %d and %q", code, errs, exitUsage, c.says)
			}
		})
	}
	for _, made := range []string{"png", "../../samples/workbook-test"} {
		if _, err := os.Stat(made); !os.IsNotExist(err) {
			t.Errorf("a refused workbook made %s: %v", made, err)
		}
	}
}

// The tables of docs/workbook.md are the pages of the workbook, byte for byte.
func TestWorkbookDoc(t *testing.T) {
	wb, err := sample.ReadWorkbook(strings.NewReader(readDoc(t, theWorkbook)))
	if err != nil {
		t.Fatal(err)
	}
	doc := readDoc(t, workbookDoc)
	sameTable(t, workbookDoc, doc, theWorkbook, workbookCountTable(wb))
	sameTable(t, workbookDoc, doc, theWorkbook, workbookBoxTable(t, wb))
}

// scenarioNames are the scenarios of a box as the document names them.
var scenarioNames = map[sample.BoxScenario]string{
	sample.BoxCorrect: "верно", sample.BoxError: "ошибка", sample.BoxNearKey: "почти-ключ",
	sample.BoxSimilarDigits: "похожие цифры", sample.BoxMirrored: "зеркальная цифра",
	sample.BoxRewritten: "зачёркнуто", sample.BoxBlank: "пусто",
}

// workbookCountTable writes the table of docs/workbook.md that counts the boxes of each scenario:
// all of them, of a number and of a fraction, of a fraction in lowest terms, stacked and inline, and
// in each part.
func workbookCountTable(wb *sample.Workbook) string {
	type tally struct{ all, number, fraction, reduced, stacked, inline, dev, test int }
	by := map[sample.BoxScenario]*tally{}
	var total tally
	for _, pg := range wb.Pages {
		for _, it := range pg.Items {
			for _, b := range it.Boxes {
				t := by[b.Scenario]
				if t == nil {
					t = &tally{}
					by[b.Scenario] = t
				}
				for _, t := range []*tally{t, &total} {
					t.all++
					if b.Check.Kind == sample.KindNumber {
						t.number++
					} else {
						t.fraction++
					}
					if b.Check.Form != nil && *b.Check.Form == value.Reduced {
						t.reduced++
					}
					if b.Fraction != nil && *b.Fraction == sample.Stacked {
						t.stacked++
					} else if b.Fraction != nil {
						t.inline++
					}
					if pg.Part == sample.PartDev {
						t.dev++
					} else {
						t.test++
					}
				}
			}
		}
	}
	var s strings.Builder
	s.WriteString("| Сценарий | Окошек | Число | Дробь | Из дробей несократимая | Столбиком | В строку | Разработка | Тест |\n|---|---|---|---|---|---|---|---|---|\n")
	row := func(name string, t tally) {
		fmt.Fprintf(&s, "| %s | %d | %d | %d | %d | %d | %d | %d | %d |\n", name, t.all, t.number, t.fraction, t.reduced, t.stacked, t.inline, t.dev, t.test)
	}
	for _, sc := range sample.BoxScenarios {
		if t := by[sc]; t != nil {
			row(scenarioNames[sc], *t)
		}
	}
	row("**всего**", total)
	return strings.TrimSuffix(s.String(), "\n")
}

// workbookBoxTable writes the table of docs/workbook.md with a box to a line: its page, its item's
// number and line, the box's place, its check, key and trap values, what is written in it, its
// scenario with the digit it leans or mirrors, and the verdict a right reading of it comes to.
func workbookBoxTable(t *testing.T, wb *sample.Workbook) string {
	t.Helper()
	var s strings.Builder
	s.WriteString("| Стр. | № | Строка пункта | Окошко | Проверка | Ключ | Ловушки | Написано | Сценарий | Вердикт |\n|---|---|---|---|---|---|---|---|---|---|\n")
	for _, pg := range wb.Pages {
		for i, it := range pg.Items {
			for k, b := range it.Boxes {
				check := "число"
				if b.Check.Kind == sample.KindFraction {
					check = "дробь"
					if *b.Check.Form == value.Reduced {
						check = "несократимая дробь"
					}
				}
				traps := make([]string, len(b.Check.TrapValues))
				for n, tv := range b.Check.TrapValues {
					traps[n] = cell(t, tv.Value) + " " + tv.Trap
				}
				fmt.Fprintf(&s, "| %d | %d | %s | %d | %s | %s | %s | %s | %s | %s |\n", pg.Number, i+1, cell(t, it.Text), k+1, check,
					cell(t, b.Check.Key), strings.Join(traps, ", "), writtenCell(t, b), scenarioCell(b), verdictCell(b.Verdict()))
			}
		}
	}
	return strings.TrimSuffix(s.String(), "\n")
}

// writtenCell is what a box holds: a dash for a blank box, the value, how a fraction is written,
// and the value crossed out before it.
func writtenCell(t *testing.T, b sample.WorkbookBox) string {
	t.Helper()
	if b.Written == nil {
		return "—"
	}
	how := ""
	switch {
	case b.Fraction == nil:
	case *b.Fraction == sample.Stacked:
		how = " столбиком"
	default:
		how = " в строку"
	}
	if b.Crossed != nil {
		return cell(t, *b.Crossed) + how + " зачёркнуто, " + cell(t, *b.Written) + how
	}
	return cell(t, *b.Written) + how
}

// scenarioCell is a box's scenario, with the digit it leans toward another or mirrors.
func scenarioCell(b sample.WorkbookBox) string {
	name := scenarioNames[b.Scenario]
	if at, toward, ok := b.Leaning(); ok {
		return fmt.Sprintf("%s: %c → %c", name, []rune(*b.Written)[at], toward)
	}
	if at, ok := sample.Mirrored(*nonNil(b.Written)); ok && b.Scenario == sample.BoxMirrored {
		return fmt.Sprintf("%s: %c", name, []rune(*b.Written)[at])
	}
	return name
}

func nonNil(s *string) *string {
	if s == nil {
		return new(string)
	}
	return s
}

// verdictCell is a verdict as the document names it, with its reason or its trap.
func verdictCell(v value.Slot) string {
	name := map[value.Verdict]string{value.Correct: "верно", value.NotYet: "пока нет", value.Unreadable: "не прочитать", value.Unchecked: "без проверки"}[v.Verdict]
	switch {
	case v.Trap != "":
		return name + ", " + v.Trap
	case v.Reason != "":
		return name + ", " + string(v.Reason)
	}
	return name
}

// A run of the reading of the workbook on the stub goes the whole way, a call a page, and logs the
// pages and the calls; it takes no sheets, logs in synthetic by default, and reads captions and
// heights alone.
func TestRunWorkbook(t *testing.T) {
	log := filepath.Join(t.TempDir(), "workbook.jsonl")
	code, out, errs := runMeasure("yes\n", "-kind", "workbook", "-workbook", theWorkbook, "-part", "dev", "-stub", "-repeats", "1", "-log", log)
	if code != exitOK || !strings.HasPrefix(out, "The dev part: 5 pages of the workbook "+theWorkbook+", drawn with the seed 1, bare and with the lines of the items at 32 px, by opus-high once and haiku-low and haiku-low-no-thinking in 1 repeat, a call a page, in version 1 of the prompt of the workbook.\n30 calls, expected to cost $") {
		t.Fatalf("the run ends %d:\n%s%s", code, out, errs)
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(data), `"kind":"page"`); n != 5 || strings.Count(string(data), `"kind":"call"`) != 30 {
		t.Errorf("the log holds %d pages:\n%.300s", n, data)
	}
	if code, out, errs := runIt("report", log); code != exitOK || !strings.Contains(out, "haiku-low, bare: 55 of 55 readings right") {
		t.Errorf("the report of the workbook ends %d: %s%s", code, out, errs)
	}
	for _, c := range []struct {
		args []string
		says string
	}{
		{[]string{"-dir", "x"}, "-dir names sheets, and the workbook reads its own file"},
		{[]string{"-essays", theEssays}, "-essays is not for -kind workbook"},
		{[]string{"-set", "a,b,c"}, "-set names sheets"},
		{[]string{"-reads", "keys"}, `-reads is "keys": for the workbook, give captions or heights`},
		// The tests run in cmd/tutor-eval, where synthetic is a folder git does not ignore.
		{nil, "cmd/tutor-eval/synthetic/runs is in the git work tree"},
	} {
		args := append([]string{"-kind", "workbook", "-workbook", theWorkbook, "-part", "dev", "-stub"}, c.args...)
		if code, _, errs := runMeasure("yes\n", args...); code == exitOK || !strings.Contains(errs, c.says) {
			t.Errorf("%v: ends %d, %q; want %q", c.args, code, errs, c.says)
		}
	}
	if code, _, errs := runMeasure("yes\n", "-part", "dev", "-stub", "-seed", "2"); code != exitUsage || !strings.Contains(errs, "-workbook, -essays and -seed are for -kind workbook and essay") {
		t.Errorf("the judge with -seed ends %d, %q", code, errs)
	}
	if _, err := os.Stat("synthetic"); !os.IsNotExist(err) {
		t.Errorf("a refused run made synthetic in cmd/tutor-eval: %v", err)
	}
}
