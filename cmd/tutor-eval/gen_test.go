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

// gen writes a bank and a plan of generated sheets as new files, which check passes and draw draws;
// it never writes over a file, and refuses what it does not make and a run without its files.
func TestGen(t *testing.T) {
	dir := t.TempDir()
	bank, plan := filepath.Join(dir, "bank.json"), filepath.Join(dir, "plan.json")
	var out, errs bytes.Buffer
	if code := runGen([]string{"sheets", "-tasks", "2", "-seed", "3", "-prefix", "t", "-bank", bank, "-plan", plan}, &out, &errs); code != exitOK ||
		out.String() != "2 tasks and 12 sheets of lines, of the seed 3: the bank "+bank+" and the plan "+plan+".\n" {
		t.Fatalf("gen sheets = %d: %s%s", code, out.String(), errs.String())
	}
	out.Reset()
	if code := check([]string{bank, plan}, &out, &errs); code != exitOK {
		t.Errorf("check of the generated files = %d: %s%s", code, out.String(), errs.String())
	}
	sheetsDir := filepath.Join(dir, "sheets")
	out.Reset()
	if code := runDraw([]string{"-dir", sheetsDir, "-bank", bank, "-plan", plan}, &out, &errs, func() time.Time { return time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC) }); code != exitOK ||
		!strings.Contains(out.String(), "Drawn by code: 12 sheets") {
		t.Errorf("draw of the generated plan = %d: %s%s", code, out.String(), errs.String())
	}
	before, _ := os.ReadFile(bank)
	errs.Reset()
	if code := runGen([]string{"sheets", "-tasks", "1", "-bank", bank, "-plan", filepath.Join(dir, "other.json")}, &out, &errs); code != exitFail || !strings.Contains(errs.String(), "file exists") {
		t.Errorf("gen over a file = %d: %s", code, errs.String())
	}
	if after, _ := os.ReadFile(bank); !bytes.Equal(before, after) {
		t.Error("gen wrote over the bank")
	}
	// With the plan there and not the bank, gen writes neither.
	fresh := filepath.Join(dir, "fresh-bank.json")
	errs.Reset()
	if code := runGen([]string{"near", "-tasks", "1", "-bank", fresh, "-plan", plan}, &out, &errs); code != exitFail || !strings.Contains(errs.String(), "file exists") {
		t.Errorf("gen onto a plan = %d: %s", code, errs.String())
	}
	if _, err := os.Stat(fresh); err == nil {
		t.Error("gen wrote the bank of a plan it could not write")
	}
	for _, c := range []struct {
		args []string
		says string
	}{
		{nil, "give sheets, near, workbook or essays"},
		{[]string{"tasks"}, `"tasks" is nothing gen makes`},
		{[]string{"near", "-bank", bank}, "give -bank and -plan"},
		{[]string{"near", "-tasks", "0", "-bank", "a", "-plan", "b"}, "0 tasks"},
		{[]string{"sheets", "-tasks", "17", "-bank", "a", "-plan", "b"}, "17 tasks make 102 sheets, and a plan holds 99 at most"},
		{[]string{"near", "-tasks", "50", "-bank", "a", "-plan", "b"}, "50 tasks make 100 sheets"},
	} {
		errs.Reset()
		if code := runGen(c.args, &out, &errs); code != exitUsage || !strings.Contains(errs.String(), c.says) {
			t.Errorf("gen %v = %d, %q; want %q", c.args, code, errs.String(), c.says)
		}
	}
}

// draw leans the answer's last digit of a near sheet as far as -shift says, and refuses a share
// outside 0 to 1: the same plan drawn at two shares differs, the same share draws the same sheet.
func TestDrawShift(t *testing.T) {
	dir := t.TempDir()
	bank, plan := filepath.Join(dir, "bank.json"), filepath.Join(dir, "plan.json")
	var out, errs bytes.Buffer
	if code := runGen([]string{"near", "-tasks", "1", "-bank", bank, "-plan", plan}, &out, &errs); code != exitOK {
		t.Fatalf("gen near = %d: %s", code, errs.String())
	}
	at := func() time.Time { return time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC) }
	drawn := func(shift string) []byte {
		t.Helper()
		d := filepath.Join(dir, "at-"+shift)
		var o, e bytes.Buffer
		if code := runDraw([]string{"-dir", d, "-bank", bank, "-plan", plan, "-shift", shift}, &o, &e, at); code != exitOK {
			t.Fatalf("draw -shift %s = %d: %s", shift, code, e.String())
		}
		files, _ := filepath.Glob(filepath.Join(d, "*.json"))
		if len(files) != 2 {
			t.Fatalf("draw -shift %s drew %v", shift, files)
		}
		data, _ := os.ReadFile(files[0])
		return data
	}
	if bytes.Equal(drawn("0.1"), drawn("0.4")) {
		t.Error("the shares 0.1 and 0.4 draw the same sheet")
	}
	if s, err := sample.ReadSample(bytes.NewReader(drawn("0.3"))); err != nil || s.Scenario != sample.ScenarioSimilarDigits {
		t.Errorf("the sheet drawn at 0.3: %v", err)
	}
	// A share given to a directory that holds sheets already is refused.
	errs.Reset()
	if code := runDraw([]string{"-dir", filepath.Join(dir, "at-0.1"), "-bank", bank, "-plan", plan, "-shift", "0.3"}, &out, &errs, at); code != exitUsage || !strings.Contains(errs.String(), "holds sheets already") {
		t.Errorf("draw -shift into a directory of sheets = %d, %q", code, errs.String())
	}
	for _, shift := range []string{"-0.1", "1.5"} {
		errs.Reset()
		if code := runDraw([]string{"-dir", filepath.Join(dir, "x"), "-bank", bank, "-plan", plan, "-shift", shift}, &out, &errs, at); code != exitUsage || !strings.Contains(errs.String(), "give a share from 0 to 1") {
			t.Errorf("draw -shift %s = %d, %q", shift, code, errs.String())
		}
	}
}

// gen writes pages of the workbook and essays as new files that check passes, and refuses a run of
// none or without its file.
func TestGenFiles(t *testing.T) {
	dir := t.TempDir()
	var out, errs bytes.Buffer
	for _, c := range []struct{ kind, says string }{{"workbook", "2 pages of the seed 5"}, {"essays", "2 essays of the seed 5"}} {
		path := filepath.Join(dir, c.kind+".json")
		out.Reset()
		if code := runGen([]string{c.kind, "-n", "2", "-seed", "5", "-file", path}, &out, &errs); code != exitOK || !strings.HasPrefix(out.String(), c.says) {
			t.Fatalf("gen %s = %d: %s%s", c.kind, code, out.String(), errs.String())
		}
		out.Reset()
		if code := check([]string{path}, &out, &errs); code != exitOK {
			t.Errorf("check of the generated %s = %d: %s%s", c.kind, code, out.String(), errs.String())
		}
		errs.Reset()
		if code := runGen([]string{c.kind, "-n", "2", "-file", path}, &out, &errs); code != exitFail || !strings.Contains(errs.String(), "file exists") {
			t.Errorf("gen %s over a file = %d: %s", c.kind, code, errs.String())
		}
	}
	for _, c := range []struct {
		args []string
		says string
	}{
		{[]string{"essays"}, "give -file"},
		{[]string{"workbook", "-n", "0", "-file", "x"}, "make at least one page"},
	} {
		errs.Reset()
		if code := runGen(c.args, &out, &errs); code != exitUsage || !strings.Contains(errs.String(), c.says) {
			t.Errorf("gen %v = %d, %q; want %q", c.args, code, errs.String(), c.says)
		}
	}
}
