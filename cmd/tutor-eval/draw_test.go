package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/altnovatrail/tutor/internal/sample"
)

// draw draws the measurement's plan into dir and returns what the command printed.
func draw(t *testing.T, dir string, at time.Time) string {
	t.Helper()
	var out, errs bytes.Buffer
	code := runDraw([]string{"-dir", dir, "-bank", "../../eval/bank.json", "-plan", "../../eval/sheets.json"}, &out, &errs, func() time.Time { return at })
	if code != exitOK {
		t.Fatalf("draw = %d, %s", code, errs.String())
	}
	return out.String()
}

// The command draws every sheet of the plan without a file, keeps the file already there, draws
// nothing on a second run, and the summary then finds all 40 sheets passing, 39 drawn by code.
func TestDraw(t *testing.T) {
	dir := t.TempDir()
	at := time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC)
	_, plan, err := readPlan("../../eval/bank.json", "../../eval/sheets.json")
	if err != nil {
		t.Fatal(err)
	}
	// The author's sheet 1: a file of its own, which the command must leave alone.
	mine := filepath.Join(dir, sample.SheetFile(&plan.Sheets[0], at.Add(-time.Hour)))
	if err := os.WriteFile(mine, []byte("the author's sheet"), 0o600); err != nil {
		t.Fatal(err)
	}

	out := draw(t, dir, at)
	if !strings.Contains(out, "Drawn by code: 39 sheets, sheets 2, 3,") || !strings.Contains(out, "Kept as they were: 1 sheet, sheets 1.") {
		t.Errorf("draw printed\n%s", out)
	}
	if data, err := os.ReadFile(mine); err != nil || string(data) != "the author's sheet" {
		t.Errorf("the author's sheet is now %q, %v", data, err)
	}
	if out := draw(t, dir, at.Add(time.Hour)); !strings.Contains(out, "Drawn by code: none") {
		t.Errorf("a second run printed\n%s", out)
	}

	// The author's sheet stands in for a real one: draw it too, in a directory of its own.
	if err := os.Remove(mine); err != nil {
		t.Fatal(err)
	}
	draw(t, dir, at.Add(2*time.Hour))
	var summary, errs bytes.Buffer
	if code := runSheets([]string{"-dir", dir, "-bank", "../../eval/bank.json", "-plan", "../../eval/sheets.json"}, &summary, &errs); code != exitOK {
		t.Fatalf("sheets = %d, %s", code, errs.String())
	}
	for _, want := range []string{
		"Sheets: 40 of the plan's 40 recorded, 40 of them passing the check.",
		"Planned erasures: 14. Slips: 0 erasures on 0 sheets.",
		"Written with a mouse, pressure 0.5 at every point: none.",
		"Marked off the plan: none.",
	} {
		if !strings.Contains(summary.String(), want) {
			t.Errorf("the summary\n%s\nwant it to hold %q", summary.String(), want)
		}
	}
}
