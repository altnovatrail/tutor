package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/synth"
)

// failingWriter refuses every write, as a closed pipe would.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("closed") }

func TestRunUnwritableVersion(t *testing.T) {
	if got := run([]string{"-version"}, failingWriter{}, io.Discard); got != exitFail {
		t.Errorf("run with an unwritable stdout = %d, want %d", got, exitFail)
	}
}

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string // a prefix of stdout
		wantStderr string // a part of stderr
	}{
		{name: "version", args: []string{"-version"}, wantCode: exitOK, wantStdout: "tutor-eval "},
		{name: "help", args: []string{"-h"}, wantCode: exitOK, wantStderr: "tutor-eval check FILE..."},
		{name: "no command", args: nil, wantCode: exitUsage, wantStderr: "usage:"},
		{name: "unknown flag", args: []string{"-nope"}, wantCode: exitUsage, wantStderr: "-nope"},
		{name: "stray argument", args: []string{"-version", "extra"}, wantCode: exitUsage, wantStderr: "unexpected arguments"},
		{name: "unknown command", args: []string{"grade"}, wantCode: exitUsage, wantStderr: `unknown command "grade"`},
		{name: "check without files", args: []string{"check"}, wantCode: exitUsage, wantStderr: "no files given"},
		{name: "convert without a directory", args: []string{"convert"}, wantCode: exitUsage, wantStderr: "-from is empty"},
		{name: "synth, which went with T105", args: []string{"synth", "dir"}, wantCode: exitUsage, wantStderr: `unknown command "synth"`},
		{name: "check a missing file", args: []string{"check", "no-such-file.json"}, wantCode: exitFail, wantStderr: "no-such-file.json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := run(tt.args, &stdout, &stderr); got != tt.wantCode {
				t.Errorf("run(%q) = %d, want %d", tt.args, got, tt.wantCode)
			}
			if !strings.HasPrefix(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout = %q, want the prefix %q", stdout.String(), tt.wantStdout)
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}

// synthDir writes the synthetic bank and a synthetic sample of tutor-sample v0 into a fresh directory
// and returns it.
func synthDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	data, err := sample.Marshal(synth.Bank())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bank.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sample-correct.json"), []byte(`{"format": "tutor-sample", "version": 0}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCheckFailures(t *testing.T) {
	dir := synthDir(t)
	bank := filepath.Join(dir, "bank.json")
	correct := filepath.Join(dir, "sample-correct.json")
	write := func(name, content string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	planText := `{"format": "tutor-sheet-plan", "version": 0, "sheets": [{"number": 1, "task": "synthetic-linear-01",
		"scenario": "correct", "lines": ["3x + 1 = 7", "3x = 6", "x = 2"], "answer": 3,
		"expected": {"outcome": "correct", "first_error": null, "trap": ""}, "erasure": null}]}`
	plan := write("plan.json", planText)
	brokenPlan := write("broken-plan.json", strings.Replace(planText, `"answer": 3`, `"answer": 4`, 1))
	planOfOtherTask := write("plan-of-other-task.json", strings.Replace(planText, `"task": "synthetic-linear-01"`, `"task": "nowhere"`, 1))

	tests := []struct {
		name       string
		files      []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{"a sample of tutor-sample v0", []string{bank, correct}, exitFail, "bank.json: ok, bank of 3 tasks", "sample-correct.json: a sample of tutor-sample v0, which no reader but tutor-eval convert takes since T105"},
		{"two banks", []string{bank, bank}, exitFail, "", "a second bank"},
		{"a plan with a broken bank", []string{write("broken-bank.json", `{"format": "tutor-bank", "version": 0, "tasks": []}`), plan}, exitFail, "its tasks are not checked: the bank is invalid", "broken-bank.json: invalid bank"},
		{"a plan before its bank", []string{plan, bank}, exitOK, "plan.json: ok, plan of 1 sheet, its tasks and traps are in the bank", ""},
		{"a plan alone", []string{plan}, exitOK, "plan.json: ok, plan of 1 sheet, its tasks are not checked: no bank was given", ""},
		{"a broken plan", []string{brokenPlan}, exitFail, "", "broken-plan.json: invalid plan: sheets[0].answer: 4 is not a line of the sheet, 1…3"},
		{"a plan of no task in the bank", []string{bank, planOfOtherTask}, exitFail, "", `plan-of-other-task.json: invalid plan: sheets[0].task: "nowhere" is not in the bank`},
		{"not JSON", []string{write("notes.json", "notes")}, exitFail, "", "not a JSON object"},
		{"a sample of the protocol", []string{write("protocol.json", `{"protocol": "novatrail.eval.v1"}`)}, exitFail, "", "protocol.json: a file of spec's eval protocol: just -f spec/justfile sample-check"},
		{"another format", []string{write("other.json", `{"format": "csv"}`)}, exitFail, "", `format "csv" is not "tutor-bank", "tutor-sheet-plan", "tutor-workbook" or "tutor-essays"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := run(append([]string{"check"}, tt.files...), &stdout, &stderr); got != tt.wantCode {
				t.Errorf("check = %d, want %d; stderr %q", got, tt.wantCode, stderr.String())
			}
			if !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout = %q, want it to contain %q", stdout.String(), tt.wantStdout)
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}
