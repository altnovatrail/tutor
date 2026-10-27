package main

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/internal/sample"
)

// The plans of the free-form measurement, the document that shows them and the section that plans
// the measurement (docs/architecture.md, section 3.4), as seen from this package's directory.
const (
	freePlan        = "../../eval/free-sheets.json"
	authorPlan      = "../../eval/free-author.json"
	freeSheetsDoc   = "../../docs/free-sheets.md"
	architectureDoc = "../../docs/architecture.md"
)

// TestFreeSheetPlans checks the plans of the free-form measurement the way the author does, with
// their bank; that each file is the plan as it is written; that the author's sheets are three
// sheets of code under other numbers, one to a layout, as T54 chose; and that the tables of
// docs/free-sheets.md are the bank and the plans, byte for byte, and so are the two tables of
// the sheets' composition in section 3.4 of docs/architecture.md.
func TestFreeSheetPlans(t *testing.T) {
	code, out, errs := runIt("check", freeBank, freePlan, authorPlan)
	want := freeBank + ": ok, bank of 6 tasks\n" +
		freePlan + ": ok, plan of 40 sheets, its tasks and traps are in the bank\n" +
		authorPlan + ": ok, plan of 3 sheets, its tasks and traps are in the bank\n"
	if code != exitOK || out != want {
		t.Fatalf("check = %d %q %q, want %q", code, out, errs, want)
	}

	bank := readBank(t, freeBank)
	free, author := partsPlanFile(t, freePlan), partsPlanFile(t, authorPlan)
	freeRules(t, free)
	layouts := layoutsOf(t, free)
	copies := copiesOf(t, free, author)
	authorsTrio(t, author, layouts)
	if t.Failed() {
		t.FailNow() // the tables are built from what the checks above hold
	}

	composition, places := compositionTable(bank, free, layouts), placesTable(bank, free, layouts)
	doc := readDoc(t, freeSheetsDoc)
	for _, built := range []string{
		freeTaskTable(t, bank, layouts),
		composition,
		places,
		partsTable(t, free, nil),
		drawingTable(t, free),
		partsTable(t, author, copies),
	} {
		sameTable(t, freeSheetsDoc, doc, freePlan+" and "+authorPlan, built)
	}
	architecture := readDoc(t, architectureDoc)
	for _, built := range []string{composition, places} {
		sameTable(t, architectureDoc, architecture, freePlan, built)
	}
}

// freeRules checks what the tables of docs/free-sheets.md rest on: each sheet is of one of the
// four scenarios of the measurement, and the working at the side is in columns, which stand only
// there, so that a part at the side and a column are one thing.
func freeRules(t *testing.T, plan *sample.Plan) {
	t.Helper()
	for _, sh := range plan.Sheets {
		if !slices.Contains(freeScenarios, sh.Scenario) {
			t.Errorf("sheet %d is of the scenario %q: the free-form measurement plans %q alone", sh.Number, sh.Scenario, freeScenarios)
		}
		for j, p := range sh.Parts {
			if p.Kind == sample.PartText && (p.Place == sample.PlaceRight) != (p.Figure.Column != nil) {
				t.Errorf("part %d of sheet %d is text at %q, a column: %t; the working at the side is in columns, and a column stands at the side", j+1, sh.Number, p.Place, p.Figure.Column != nil)
			}
		}
	}
}

// partsPlanFile reads a plan of parts, which its file holds as the plan is written.
func partsPlanFile(t *testing.T, path string) *sample.Plan {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := sample.ReadPlan(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if !plan.HasParts() {
		t.Fatalf("%s is not a plan of version %d, with parts", path, sample.PlanParts)
	}
	written, err := sample.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(written, data) {
		t.Errorf("%s is not the plan as it is written: write it with sample.Marshal", path)
	}
	return plan
}

// The layouts of a sheet of free-form writing, as section 3.4 names them: the task sets the
// layout, and a drawing or a column on the sheet shows it.
const (
	layoutNumberLine = "числовая прямая"
	layoutRow        = "рисунок: ряд предметов"
	layoutScheme     = "рисунок: схема отрезками"
	layoutSide       = "запись сбоку"
)

// layoutOf names the layout of a sheet by its parts, or "" when none shows one.
func layoutOf(sh sample.PlanSheet) string {
	var layout string
	for _, p := range sh.Parts {
		switch f := p.Figure; {
		case f.NumberLine != nil:
			return layoutNumberLine
		case f.Row != nil:
			return layoutRow
		case f.Scheme != nil:
			return layoutScheme
		case f.Column != nil:
			layout = layoutSide
		}
	}
	return layout
}

// group is the layout without what it draws: the composition by layouts joins the two drawings.
func group(layout string) string {
	g, _, _ := strings.Cut(layout, ":")
	return g
}

// layoutsOf finds each task's layout, and checks that every sheet of a task has it.
func layoutsOf(t *testing.T, plan *sample.Plan) map[string]string {
	t.Helper()
	layouts := make(map[string]string)
	for _, sh := range plan.Sheets {
		got := layoutOf(sh)
		if got == "" {
			t.Errorf("sheet %d draws nothing and calculates in no column: it has no layout of free-form writing", sh.Number)
			continue
		}
		if want, ok := layouts[sh.Task]; ok && got != want {
			t.Errorf("sheet %d of %q is laid out as %q, and the task's other sheets as %q", sh.Number, sh.Task, got, want)
		}
		layouts[sh.Task] = got
	}
	return layouts
}

// copiesOf finds the sheet of code each of the author's sheets copies: the one sheet that is the
// same but for its number.
func copiesOf(t *testing.T, free, author *sample.Plan) []int {
	t.Helper()
	copies := make([]int, len(author.Sheets))
	for i, a := range author.Sheets {
		var found []int
		for _, c := range free.Sheets {
			n := c.Number
			c.Number = a.Number
			if reflect.DeepEqual(c, a) {
				found = append(found, n)
			}
		}
		if len(found) != 1 {
			t.Errorf("the author's sheet %d is a copy of %d sheets of code, %v, not of one", a.Number, len(found), found)
			continue
		}
		copies[i] = found[0]
	}
	return copies
}

// authorsTrio checks the author's three sheets against T54 (section 3.4): one to a layout — the
// number line erasure-fixed with the erasure in the drawing, the drawing erasure-broken with the
// erasure in the drawing, and the working at the side an error in a column, with no erasure.
func authorsTrio(t *testing.T, author *sample.Plan, layouts map[string]string) {
	t.Helper()
	want := []string{
		layoutNumberLine + ", erasure-fixed, erased in the drawing",
		"рисунок, erasure-broken, erased in the drawing",
		layoutSide + ", error, first error at the side",
	}
	var got []string
	for _, sh := range author.Sheets {
		s := group(layouts[sh.Task]) + ", " + string(sh.Scenario)
		if e := sh.Erasure; e != nil {
			s += ", erased " + partPlace(sh, e.Line)
		}
		if n := sh.Expected.FirstError; n != nil && sh.Erasure == nil {
			s += ", first error " + partPlace(sh, *n)
		}
		got = append(got, s)
	}
	if !slices.Equal(got, want) {
		t.Errorf("the author's sheets are %q, want %q", got, want)
	}
}

// partPlace says where part n of a sheet stands, in the words of the test. The part is one of the
// sheet's: tutor-eval check has passed the plan before.
func partPlace(sh sample.PlanSheet, n int) string {
	switch p := sh.Parts[n-1]; {
	case p.Kind == sample.PartDrawing:
		return "in the drawing"
	case p.Place == sample.PlaceRight:
		return "at the side"
	}
	return "in the text"
}

// whereRu says the same as partPlace in the words of section 3.4: «в рисунке», «в тексте»,
// «сбоку».
var whereRu = map[string]string{"in the drawing": "в рисунке", "in the text": "в тексте", "at the side": "сбоку"}

// freeTaskTable writes the table of tasks of docs/free-sheets.md from the bank: a task to a line,
// with the layout its sheets show.
func freeTaskTable(t *testing.T, bank *sample.Bank, layouts map[string]string) string {
	t.Helper()
	var b strings.Builder
	b.WriteString("| ID | Раскладка | Условие | Ключ |\n")
	b.WriteString("|---|---|---|---|\n")
	for _, task := range bank.Tasks {
		if strings.Contains(task.Statement, "|") {
			t.Fatalf("the statement of %q holds a vertical bar, which would break its cell", task.ID)
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", cell(t, task.ID), layouts[task.ID], task.Statement, cell(t, task.Key))
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// The scenarios of the free-form measurement, in the order of its tables.
var freeScenarios = []sample.Scenario{sample.ScenarioCorrect, sample.ScenarioError, sample.ScenarioErasureFixed, sample.ScenarioErasureBroken}

// compositionTable writes the composition of the plan by task and scenario, as section 3.4 has
// it: a dash for none, and the totals in bold.
func compositionTable(bank *sample.Bank, plan *sample.Plan, layouts map[string]string) string {
	var b strings.Builder
	b.WriteString("| Задача | Раскладка | `correct` | `error` | `erasure-fixed` | `erasure-broken` | Всего |\n")
	b.WriteString("|---|---|---|---|---|---|---|\n")
	totals := make([]int, len(freeScenarios)+1)
	for _, task := range bank.Tasks {
		counts := make([]int, len(freeScenarios)+1)
		for _, sh := range plan.Sheets {
			if sh.Task == task.ID {
				counts[slices.Index(freeScenarios, sh.Scenario)]++
				counts[len(freeScenarios)]++
			}
		}
		if counts[len(freeScenarios)] == 0 {
			continue
		}
		fmt.Fprintf(&b, "| `%s` | %s |", task.ID, layouts[task.ID])
		for i, n := range counts {
			fmt.Fprintf(&b, " %s |", dash(n))
			totals[i] += n
		}
		b.WriteString("\n")
	}
	b.WriteString("| **всего** | |")
	for _, n := range totals {
		fmt.Fprintf(&b, " **%s** |", dash(n))
	}
	return b.String()
}

// placesTable writes where the first error and the planned erasure stand, by layout, as section
// 3.4 has it.
func placesTable(bank *sample.Bank, plan *sample.Plan, layouts map[string]string) string {
	type counts struct {
		sheets, errors, erasures int
		first, erased            map[string]int
	}
	var order []string
	by := make(map[string]*counts)
	total := &counts{first: map[string]int{}, erased: map[string]int{}}
	for _, task := range bank.Tasks {
		g := group(layouts[task.ID])
		if _, ok := by[g]; !ok && g != "" {
			order = append(order, g)
			by[g] = &counts{first: map[string]int{}, erased: map[string]int{}}
		}
	}
	for _, sh := range plan.Sheets {
		for _, c := range []*counts{by[group(layouts[sh.Task])], total} {
			c.sheets++
			if n := sh.Expected.FirstError; n != nil {
				c.errors++
				c.first[whereRu[partPlace(sh, *n)]]++
			}
			if e := sh.Erasure; e != nil {
				c.erasures++
				c.erased[whereRu[partPlace(sh, e.Line)]]++
			}
		}
	}
	split := func(m map[string]int) string {
		return dash(m["в рисунке"]) + " / " + dash(m["в тексте"]) + " / " + dash(m["сбоку"])
	}
	var b strings.Builder
	b.WriteString("| Раскладка | Листов | С ошибкой | Первая ошибка: в рисунке / в тексте / сбоку | Стираний | Стирание: в рисунке / в тексте / сбоку |\n")
	b.WriteString("|---|---|---|---|---|---|\n")
	for _, g := range order {
		c := by[g]
		fmt.Fprintf(&b, "| %s | %d | %d | %s | %d | %s |\n", g, c.sheets, c.errors, split(c.first), c.erasures, split(c.erased))
	}
	fmt.Fprintf(&b, "| **всего** | **%d** | **%d** | **%s** | **%d** | **%s** |", total.sheets, total.errors, split(total.first), total.erasures, split(total.erased))
	return b.String()
}

func dash(n int) string {
	if n == 0 {
		return "—"
	}
	return strconv.Itoa(n)
}

// partsTable writes the table of the sheets of a plan of parts, as docs/free-sheets.md has it: a
// line to a sheet, its parts numbered and joined by "; ", each with its kind and place and its
// text in backquotes. With copies, the plan is the author's, and each sheet names the sheet of
// code it copies.
func partsTable(t *testing.T, plan *sample.Plan, copies []int) string {
	t.Helper()
	var b strings.Builder
	if copies != nil {
		b.WriteString("| № | Лист кода | Задача | Сценарий | Части на листе | Ответ | Первая ошибка | Стирание по сценарию |\n")
		b.WriteString("|---|---|---|---|---|---|---|---|\n")
	} else {
		b.WriteString("| № | Задача | Сценарий | Части на листе | Ответ | Первая ошибка | Стирание по сценарию |\n")
		b.WriteString("|---|---|---|---|---|---|---|\n")
	}
	for i, sh := range plan.Sheets {
		parts := make([]string, len(sh.Parts))
		for j, p := range sh.Parts {
			parts[j] = fmt.Sprintf("%d) %s", j+1, partCell(t, p))
		}
		b.WriteString("| " + strconv.Itoa(sh.Number) + " |")
		if copies != nil {
			b.WriteString(" " + strconv.Itoa(copies[i]) + " |")
		}
		fmt.Fprintf(&b, " %s | %s | %s | %d | %s | %s |\n", cell(t, sh.Task), cell(t, string(sh.Scenario)), strings.Join(parts, "; "), sh.Answer, firstErrorCell(t, sh.Expected), erasureCell(t, sh))
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// partCell says what a part is and where it stands, and gives its text: a line of the main column
// by its text alone.
func partCell(t *testing.T, p sample.PlanPart) string {
	t.Helper()
	place := map[sample.Place]string{sample.PlaceLeft: "", sample.PlaceRight: "сбоку", sample.PlaceWide: "во всю ширину"}[p.Place]
	var what []string
	switch {
	case p.Kind == sample.PartDrawing:
		what = append(what, "рисунок")
	case p.Figure.Column != nil:
		what = append(what, "столбиком")
	}
	if place != "" {
		what = append(what, place)
	}
	if len(what) == 0 {
		return cell(t, p.Text)
	}
	return strings.Join(what, " ") + ": " + cell(t, p.Text)
}

// drawingTable writes the table «Рисунки» of docs/free-sheets.md: each drawing of the plan, and
// the drawing before the planned erasure that takes some of it.
func drawingTable(t *testing.T, plan *sample.Plan) string {
	t.Helper()
	var b strings.Builder
	b.WriteString("| № | Часть | Рисунок | До стирания |\n")
	b.WriteString("|---|---|---|---|\n")
	for _, sh := range plan.Sheets {
		for j, p := range sh.Parts {
			if p.Kind != sample.PartDrawing {
				continue
			}
			before := "—"
			if e := sh.Erasure; e != nil && e.Line == j+1 && e.Figure != nil {
				before = figureCell(t, *e.Figure)
			}
			fmt.Fprintf(&b, "| %d | %d | %s | %s |\n", sh.Number, j+1, figureCell(t, p.Figure), before)
		}
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// figureCell says in words what a drawing holds: the divisions, labels and jumps of a number
// line, the objects of a row, and the segments of a scheme with their pieces and labels.
func figureCell(t *testing.T, f sample.Figure) string {
	t.Helper()
	switch {
	case f.NumberLine != nil:
		nl := f.NumberLine
		labels := make([]string, len(nl.Labels))
		for i, v := range nl.Labels {
			labels[i] = strconv.Itoa(v)
		}
		jumps := make([]string, len(nl.Jumps))
		for i, j := range nl.Jumps {
			jumps[i] = fmt.Sprintf("%d → %d", j.From, j.To)
		}
		return fmt.Sprintf("прямая от %d до %d с шагом %d; подписи %s; дуги %s", nl.From, nl.To, nl.Step, orNone(labels), orNone(jumps))
	case f.Row != nil:
		r := f.Row
		numbers := "без номеров"
		if r.Numbers {
			numbers = "с номерами"
		}
		objects := map[string]string{sample.ObjectHouse: "домиков"}[r.Object]
		if objects == "" {
			t.Fatalf("a row of %q: the table names the objects of a row it knows", r.Object)
		}
		return fmt.Sprintf("ряд %s: %d слева от отмеченного и %d справа, %s", objects, r.Left, r.Right, numbers)
	case f.Scheme != nil:
		s := f.Scheme
		said := make([]string, 0, len(s.Segments)+1)
		for _, seg := range s.Segments {
			pieces := make([]string, len(seg.Pieces))
			for i, pc := range seg.Pieces {
				pieces[i] = strconv.Itoa(pc.Length) + labelOf(pc.Label)
			}
			segment := "отрезок"
			if seg.Label != "" {
				segment += " " + seg.Label
			}
			said = append(said, segment+" — "+strings.Join(pieces, " + "))
		}
		brace := "скобка без подписи"
		if s.Total != "" {
			brace = "скобка" + labelOf(s.Total)
		}
		return "схема: " + strings.Join(append(said, brace), "; ")
	}
	t.Fatal("a drawing with no figure")
	return ""
}

func labelOf(label string) string {
	if label == "" {
		return ""
	}
	return " «" + label + "»"
}

func orNone(items []string) string {
	if len(items) == 0 {
		return "—"
	}
	return strings.Join(items, ", ")
}
