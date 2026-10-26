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
		{name: "synth without a directory", args: []string{"synth"}, wantCode: exitUsage, wantStderr: "give one directory"},
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

// synthDir writes the synthetic files into a fresh directory and returns it.
func synthDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "synth")
	var stdout, stderr bytes.Buffer
	if got := run([]string{"synth", dir}, &stdout, &stderr); got != exitOK {
		t.Fatalf("synth = %d, stderr %q", got, stderr.String())
	}
	return dir
}

func TestSynthThenCheck(t *testing.T) {
	dir := synthDir(t)
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if want := 1 + len(sample.Scenarios); len(files) != want {
		t.Fatalf("synth wrote %d files, want %d", len(files), want)
	}
	var stdout, stderr bytes.Buffer
	if got := run(append([]string{"check"}, files...), &stdout, &stderr); got != exitOK {
		t.Fatalf("check = %d, stderr %q", got, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "bank.json: ok, bank of 3 tasks") {
		t.Errorf("stdout = %q, want the bank checked", out)
	}
	for _, sc := range sample.Scenarios {
		if !strings.Contains(out, "ok, sample of the scenario "+string(sc)+", task ") {
			t.Errorf("stdout = %q, want the sample of %q checked against the bank", out, sc)
		}
	}
}

func TestSynthNeverOverwritesAndCleansUp(t *testing.T) {
	dir := t.TempDir()
	mine := filepath.Join(dir, "sample-error.json")
	if err := os.WriteFile(mine, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if got := run([]string{"synth", dir}, &stdout, &stderr); got != exitFail {
		t.Errorf("synth into a directory holding one of its files = %d, want %d", got, exitFail)
	}
	if !strings.Contains(stderr.String(), "exists") {
		t.Errorf("stderr = %q, want it to say the file exists", stderr.String())
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("the directory holds %d files after the failed run, want only the one that was there", len(entries))
	}
	if data, err := os.ReadFile(mine); err != nil || string(data) != "mine" {
		t.Errorf("the file that was there = %q, %v; want it untouched", data, err)
	}
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
	data, err := os.ReadFile(correct)
	if err != nil {
		t.Fatal(err)
	}
	broken := write("broken.json", strings.Replace(string(data), `"writer": "author"`, `"writer": "child"`, 1))
	otherTask := write("other-task.json", strings.Replace(string(data), `"task": "synthetic-linear-01"`, `"task": "nowhere"`, 1))
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
		{"a sample alone", []string{correct}, exitOK, "its task is not checked: no bank was given", ""},
		{"a broken sample", []string{broken}, exitFail, "", `broken.json: invalid sample: writer: "child"`},
		{"a sample of no task in the bank", []string{bank, otherTask}, exitFail, "", `other-task.json: invalid sample: task: "nowhere" is not in the bank`},
		{"two banks", []string{bank, bank}, exitFail, "", "a second bank"},
		{"a sample with a broken bank", []string{write("broken-bank.json", `{"format": "tutor-bank", "version": 0, "tasks": []}`), correct}, exitFail, "its task is not checked: the bank is invalid", "broken-bank.json: invalid bank"},
		{"a plan before its bank", []string{plan, bank}, exitOK, "plan.json: ok, plan of 1 sheet, its tasks and traps are in the bank", ""},
		{"a plan alone", []string{plan}, exitOK, "plan.json: ok, plan of 1 sheet, its tasks are not checked: no bank was given", ""},
		{"a broken plan", []string{brokenPlan}, exitFail, "", "broken-plan.json: invalid plan: sheets[0].answer: 4 is not a line of the sheet, 1…3"},
		{"a plan of no task in the bank", []string{bank, planOfOtherTask}, exitFail, "", `plan-of-other-task.json: invalid plan: sheets[0].task: "nowhere" is not in the bank`},
		{"not JSON", []string{write("notes.json", "notes")}, exitFail, "", "not a JSON object"},
		{"another format", []string{write("other.json", `{"format": "csv"}`)}, exitFail, "", `format "csv" is not "tutor-sample", "tutor-bank" or "tutor-sheet-plan"`},
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
