package main

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/altnovatrail/tutor/internal/sample"
)

// The sheet plan of the reading measurement and the document that shows it as a table
// (docs/architecture.md, section 3.1), and the golden plan of parts of internal/sample, as seen from
// this package's directory.
const (
	measurementPlan = "../../eval/sheets.json"
	sheetsDoc       = "../../docs/sheets.md"
	partsPlan       = "../../internal/sample/testdata/parts-plan.json"
)

// tutor-eval check reads a plan of parts against its bank, the record mode of the board takes it,
// and draw draws its sheets: the number line with a jump erased, the row with the first error in the
// drawing, the scheme with □ and △ and the columns with a number erased. The measurement runs all of
// them on the stub in version 2 of the prompt, without a split, and the report tells free writing;
// with a sheet not recorded, it does not start.
func TestPlanOfParts(t *testing.T) {
	code, out, errs := runIt("check", freeBank, partsPlan)
	if want := freeBank + ": ok, bank of 6 tasks\n" + partsPlan + ": ok, plan of 4 sheets, its tasks and traps are in the bank\n"; code != exitOK || out != want {
		t.Errorf("check = %d %q %q, want %q", code, out, errs, want)
	}

	dir := filepath.Join(t.TempDir(), "sheets")
	files := []string{"-dir", dir, "-bank", freeBank, "-plan", partsPlan}
	var stdout, stderr bytes.Buffer
	if code, _, errs := runMeasure("yes\n", append([]string{"-part", "all", "-stub"}, files...)...); code != exitFail || !strings.Contains(errs, dir) {
		t.Errorf("run before any sheet is drawn = %d %q", code, errs)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("a run that did not start made %s: %v", dir, err)
	}

	served := false
	serve := func(*http.Server) error { served = true; return nil }
	recorded := filepath.Join(t.TempDir(), "recorded")
	if code := runBoard([]string{"-record", "-dir", recorded, "-bank", freeBank, "-plan", partsPlan}, &stdout, &stderr, func(string) string { return "" }, serve); code != exitOK || !served || !strings.Contains(stdout.String(), "Sheets go to "+recorded) {
		t.Errorf("board -record = %d, served %v, %q %q", code, served, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(recorded); err != nil {
		t.Errorf("the record mode did not make its directory: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if code := runDraw(files, &stdout, &stderr, time.Now); code != exitOK || !strings.Contains(stdout.String(), "Drawn by code: 4 sheets, sheets 1, 2, 3, 4, into") {
		t.Errorf("draw = %d %q %q", code, stdout.String(), stderr.String())
	}
	var summary bytes.Buffer
	if code := runSheets(files, &summary, &stderr); code != exitOK || !strings.Contains(summary.String(), "Sheets: 4 of the plan's 4 recorded, 4 of them passing the check.") || !strings.Contains(summary.String(), "Marked off the plan: none.") {
		t.Errorf("sheets = %d\n%s%s", code, summary.String(), stderr.String())
	}

	code, out, errs = runMeasure("yes\n", append([]string{"-part", "all", "-stub", "-repeats", "1"}, files...)...)
	if code != exitOK || !strings.HasPrefix(out, "All the sheets: 4 sheets, 2 of them with frames, by 3 judges, 1 repeat, in version 2 of the prompt.\n") {
		t.Fatalf("run -part all = %d\n%s%s", code, out, errs)
	}
	log := filepath.Join(filepath.Dir(dir), "runs", "20261026-123000-000-all.jsonl")
	stdout.Reset()
	if code := runReport([]string{log}, &stdout, &stderr); code != exitOK {
		t.Fatalf("report = %d %s", code, stderr.String())
	}
	md, err := os.ReadFile(strings.TrimSuffix(log, ".jsonl") + ".md")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# The measurement of free writing: all the sheets", ", version 2. ", "### Boxes on the sheets with a mistake", "### By sheet"} {
		if !strings.Contains(string(md), want) {
			t.Errorf("the report lacks %q:\n%s", want, md)
		}
	}

	drawn, err := filepath.Glob(filepath.Join(dir, "04-*.json"))
	if err != nil || len(drawn) != 1 {
		t.Fatalf("sheet 4 is drawn as %v, %v", drawn, err)
	}
	if err := os.Remove(drawn[0]); err != nil {
		t.Fatal(err)
	}
	if code, _, errs := runMeasure("yes\n", append([]string{"-part", "all", "-stub"}, files...)...); code != exitFail || !strings.Contains(errs, "sheet 4 is not recorded") {
		t.Errorf("run -part all without sheet 4 = %d %q", code, errs)
	}
}

// TestSheetPlan checks the measurement's plan the way the author does, against its bank, and that
// the table «Листы» of docs/sheets.md is the plan, byte for byte: the table built from the file
// is the one in the document.
func TestSheetPlan(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if got := run([]string{"check", measurementBank, measurementPlan}, &stdout, &stderr); got != exitOK {
		t.Fatalf("check %s %s = %d, stderr %q", measurementBank, measurementPlan, got, stderr.String())
	}
	want := measurementBank + ": ok, bank of 10 tasks\n" + measurementPlan + ": ok, plan of 40 sheets, its tasks and traps are in the bank\n"
	if stdout.String() != want {
		t.Errorf("stdout = %q, want %q", stdout.String(), want)
	}

	data, err := os.ReadFile(measurementPlan)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := sample.ReadPlan(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	sameTable(t, sheetsDoc, readDoc(t, sheetsDoc), measurementPlan, sheetTable(t, plan))
}

// sheetTable writes the table «Листы» of docs/sheets.md from a plan, as the document has it: a
// line to a sheet, every text in backquotes, the lines of a sheet joined by "; ", and a dash
// where the sheet has no first error or no planned erasure.
func sheetTable(t *testing.T, plan *sample.Plan) string {
	t.Helper()
	var b strings.Builder
	b.WriteString("| № | Задача | Сценарий | Строки на листе | Ответ | Первая ошибка | Стирание по сценарию |\n")
	b.WriteString("|---|---|---|---|---|---|---|\n")
	for _, sh := range plan.Sheets {
		lines := make([]string, len(sh.Lines))
		for i, l := range sh.Lines {
			lines[i] = cell(t, l)
		}
		fmt.Fprintf(&b, "| %d | %s | %s | %s | %d | %s | %s |\n",
			sh.Number, cell(t, sh.Task), cell(t, string(sh.Scenario)), strings.Join(lines, "; "), sh.Answer, firstErrorCell(t, sh.Expected), erasureCell(t, sh))
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// cell writes a text of a sheet plan in backquotes, for a table of docs/sheets.md or
// docs/free-sheets.md. A text with a vertical bar, a backquote or a semicolon would break its cell
// or the list of lines or parts in it, and the test stops on it.
func cell(t *testing.T, s string) string {
	t.Helper()
	if strings.ContainsAny(s, "|`;") {
		t.Fatalf("%q cannot stand in a cell of a table of sheets: it holds a vertical bar, a backquote or a semicolon", s)
	}
	return "`" + s + "`"
}

// firstErrorCell writes a sheet's first error: the number of its line or part and the trap, or a
// dash for none.
func firstErrorCell(t *testing.T, e sample.Expected) string {
	t.Helper()
	if e.FirstError == nil {
		return "—"
	}
	s := strconv.Itoa(*e.FirstError)
	if e.Trap != "" {
		s += ": " + cell(t, e.Trap)
	}
	return s
}

// erasureCell writes a sheet's planned erasure: the line or part written in place of the erased,
// what was erased and its text and, after the arrow, what replaced it, and then where it stands
// when it takes less than a line: in a drawing, in a column or in a part of a line. A dash stands
// for none.
func erasureCell(t *testing.T, sh sample.PlanSheet) string {
	t.Helper()
	e := sh.Erasure
	if e == nil {
		return "—"
	}
	erased := map[sample.Erased]string{sample.ErasedMistake: "ошибка", sample.ErasedCorrectStep: "верный шаг"}
	s := fmt.Sprintf("%d: %s %s → %s", e.Line, erased[e.Erased], cell(t, e.Text), cell(t, e.Replacement))
	var part sample.PlanPart
	if e.Line >= 1 && e.Line <= len(sh.Parts) {
		part = sh.Parts[e.Line-1]
	}
	switch {
	case part.Kind == sample.PartDrawing:
		return s + ", в рисунке"
	case part.Figure.Column != nil:
		return s + ", в столбике"
	case e.PartOfLine:
		return s + ", часть строки"
	}
	return s
}

// sameTable checks that a Markdown document holds a table built from a file, byte for byte. It
// finds the table by its header line; with none, it shows the table built.
func sameTable(t *testing.T, docPath, doc, from, built string) {
	t.Helper()
	lines := strings.Split(built, "\n")
	written := tableIn(doc, lines[0])
	if written == nil {
		t.Errorf("%s has no table headed %q; built from %s, it is\n%s", docPath, lines[0], from, built)
		return
	}
	for i := range max(len(lines), len(written)) {
		var b, w string
		if i < len(lines) {
			b = lines[i]
		}
		if i < len(written) {
			w = written[i]
		}
		if b != w {
			t.Errorf("line %d of the table headed %q differs:\n%s has    %q\n%s gives %q", i+1, lines[0], docPath, w, from, b)
			return
		}
	}
}

func readDoc(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// tableIn returns the lines of the table in a Markdown document that starts with the given
// header line, up to the first line that is not a row of it, or nil if there is none.
func tableIn(doc, header string) []string {
	lines := strings.Split(doc, "\n")
	for i, l := range lines {
		if l != header {
			continue
		}
		end := i
		for end < len(lines) && strings.HasPrefix(lines[end], "|") {
			end++
		}
		return lines[i:end]
	}
	return nil
}
