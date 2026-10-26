package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The report of a run on the stub: the stub is always right, so no form differs from another, and
// that the writing process helps is not shown. The report goes beside the log, once.
func TestReport(t *testing.T) {
	where := splitSheets(t)
	log := filepath.Join(t.TempDir(), "run.jsonl")
	if code, out, errs := runMeasure("yes\n", append([]string{"-part", "test", "-stub", "-repeats", "1", "-log", log}, where...)...); code != exitOK {
		t.Fatalf("the run ends %d: %s%s", code, out, errs)
	}
	code, out, errs := runIt("report", log)
	if code != exitOK {
		t.Fatalf("the report ends %d: %s", code, errs)
	}
	md, csv := strings.TrimSuffix(log, ".jsonl")+".md", strings.TrimSuffix(log, ".jsonl")+".csv"
	for _, want := range []string{
		"opus-high: not shown; D against A, Σd +0.00 over 0 sheets with d ≠ 0, p 1.\n",
		"That the writing process helps Claude is not shown.\n",
		"The report: " + md + " and " + csv + "\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the report says\n%s\nwithout %q", out, want)
		}
	}
	text, err := os.ReadFile(md)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(text), "# The measurement of the writing process: the test part\n") || !strings.Contains(string(text), "cannot pass the test") {
		t.Errorf("the Markdown is\n%s", text)
	}
	if table, err := os.ReadFile(csv); err != nil || !strings.HasPrefix(string(table), "judge,form,answers,right,") || strings.Count(string(table), "\n") != 1+3*4 {
		t.Errorf("the CSV is %v\n%s", err, table)
	}
	if code, _, errs := runIt("report", log); code != exitFail || !strings.Contains(errs, "exists") {
		t.Errorf("a second report ends %d: %s", code, errs)
	}
}

func TestReportRefuses(t *testing.T) {
	for _, c := range []struct {
		args []string
		code int
		says string
	}{
		{nil, exitUsage, "usage: tutor-eval report LOG"},
		{[]string{"a", "b"}, exitUsage, "usage: tutor-eval report LOG"},
		{[]string{"-x"}, exitUsage, "usage: tutor-eval report LOG"},
		{[]string{"-h"}, exitOK, "usage: tutor-eval report LOG"},
		{[]string{filepath.Join(t.TempDir(), "none.jsonl")}, exitFail, "no such file"},
	} {
		code, _, errs := runIt(append([]string{"report"}, c.args...)...)
		if code != c.code || !strings.Contains(errs, c.says) {
			t.Errorf("%v: ends %d, %s; want %d and %q", c.args, code, errs, c.code, c.says)
		}
	}
	bad := filepath.Join(t.TempDir(), "bad.jsonl")
	if err := os.WriteFile(bad, []byte("{\"kind\": \"sheet\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, errs := runIt("report", bad); code != exitFail || !strings.Contains(errs, "out of its place") {
		t.Errorf("a log that does not start with its run: %d, %s", code, errs)
	}
}
