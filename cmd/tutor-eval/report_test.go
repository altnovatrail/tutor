package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"

	"github.com/altnovatrail/tutor/internal/protocol"
	"github.com/altnovatrail/tutor/internal/report"
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

// reportsBeside holds the runner's reports of the protocol that the report of a log writes beside
// it, by their names, and the CSV of their measures: each reads back strictly, of its capability,
// with module-kit's verdicts and samples measured (R74).
func reportsBeside(t *testing.T, log string, capability evalv1.Capability, names ...string) {
	t.Helper()
	base := strings.TrimSuffix(log, ".jsonl")
	if _, err := os.Stat(base + ".protocol.csv"); err != nil {
		t.Errorf("the CSV of the reports: %v", err)
	}
	for _, name := range names {
		data, err := os.ReadFile(report.ReportFile(base, name))
		if err != nil {
			t.Errorf("the report of %s: %v", name, err)
			continue
		}
		r, err := protocol.ReadReport(bytes.NewReader(data))
		if err != nil || r.GetCapability() != capability || r.GetVerdicts().GetCode() != evalv1.VerdictCode_VERDICT_CODE_MODULE_KIT || len(r.GetSamples()) == 0 || r.GetSamples()[0].GetMeasured() == nil {
			t.Errorf("the report of %s, %v: %.300s", name, err, data)
		}
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
	// A log of a kind of call the report does not know is not read as the judge's, and one of the
	// hint or the reading with no sheets makes no report.
	for _, c := range []struct{ kind, says string }{
		{"rubric", "a log of the kind of call \"rubric\""},
		{"hint", "the log has no tutors or no sheets"},
		{"reading", "the log has no readers or no sheets"},
		{"workbook", "the log has no readers or no pages"},
	} {
		other := filepath.Join(t.TempDir(), c.kind+".jsonl")
		if err := os.WriteFile(other, []byte("{\"kind\": \"run\", \"call_kind\": \""+c.kind+"\"}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if code, _, errs := runIt("report", other); code != exitFail || !strings.Contains(errs, c.says) {
			t.Errorf("a log of %s: %d, %s", c.kind, code, errs)
		}
	}
}
