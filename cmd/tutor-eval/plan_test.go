package main

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/internal/sample"
)

// The sheet plan of the reading measurement and the document that shows it as a table
// (docs/architecture.md, section 3.1), as seen from this package's directory.
const (
	measurementPlan = "../../eval/sheets.json"
	sheetsDoc       = "../../docs/sheets.md"
)

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
	doc, err := os.ReadFile(sheetsDoc)
	if err != nil {
		t.Fatal(err)
	}
	built := strings.Split(sheetTable(t, plan), "\n")
	written := tableIn(string(doc), built[0])
	if written == nil {
		t.Fatalf("%s has no table headed %q", sheetsDoc, built[0])
	}
	for i := range max(len(built), len(written)) {
		var b, w string
		if i < len(built) {
			b = built[i]
		}
		if i < len(written) {
			w = written[i]
		}
		if b != w {
			t.Fatalf("line %d of the sheet table differs:\n%s has    %q\n%s gives %q", i+1, sheetsDoc, w, measurementPlan, b)
		}
	}
}

// sheetTable writes the table «Листы» of docs/sheets.md from a plan, as the document has it: a
// line to a sheet, every text in backquotes, the lines of a sheet joined by "; ", and a dash
// where the sheet has no first error or no planned erasure. A text with a vertical bar or a
// backquote would break its cell, and the test stops on it.
func sheetTable(t *testing.T, plan *sample.Plan) string {
	t.Helper()
	erased := map[sample.Erased]string{sample.ErasedMistake: "ошибка", sample.ErasedCorrectStep: "верный шаг"}
	code := func(s string) string {
		if strings.ContainsAny(s, "|`") {
			t.Fatalf("%q cannot stand in a cell of the sheet table: it holds a vertical bar or a backquote", s)
		}
		return "`" + s + "`"
	}
	var b strings.Builder
	b.WriteString("| № | Задача | Сценарий | Строки на листе | Ответ | Первая ошибка | Стирание по сценарию |\n")
	b.WriteString("|---|---|---|---|---|---|---|\n")
	for _, sh := range plan.Sheets {
		lines := make([]string, len(sh.Lines))
		for i, l := range sh.Lines {
			lines[i] = code(l)
		}
		firstError := "—"
		if e := sh.Expected; e.FirstError != nil {
			firstError = strconv.Itoa(*e.FirstError)
			if e.Trap != "" {
				firstError += ": " + code(e.Trap)
			}
		}
		erasure := "—"
		if e := sh.Erasure; e != nil {
			erasure = fmt.Sprintf("%d: %s %s → %s", e.Line, erased[e.Erased], code(e.Text), code(e.Replacement))
			if e.PartOfLine {
				erasure += ", часть строки"
			}
		}
		fmt.Fprintf(&b, "| %d | %s | %s | %s | %d | %s | %s |\n",
			sh.Number, code(sh.Task), code(string(sh.Scenario)), strings.Join(lines, "; "), sh.Answer, firstError, erasure)
	}
	return strings.TrimSuffix(b.String(), "\n")
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
