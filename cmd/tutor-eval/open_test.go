package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// open writes, from the files of the measurement's tasks, the file of open views that the
// repository holds; it never writes over a file, and refuses a run without its files and a file that
// is no file of tasks, or a run outside the repository's root.
func TestOpen(t *testing.T) {
	root := filepath.Join("..", "..")
	t.Chdir(root)
	file := filepath.Join(t.TempDir(), "open-tasks.json")
	tasks := []string{"eval/bank.json", "eval/free-bank.json", "eval/workbook.json", "eval/essays.json"}
	var out, errs bytes.Buffer
	if code := runOpen(append([]string{"-file", file}, tasks...), &out, &errs); code != exitOK ||
		out.String() != file+": ok, open views of 56 tasks from 4 files\n" {
		t.Fatalf("open = %d: %s%s", code, out.String(), errs.String())
	}
	got, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("eval/open-tasks.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Error("open wrote another file than eval/open-tasks.json: run just open-tasks")
	}
	errs.Reset()
	if code := runOpen([]string{"-file", file, "eval/bank.json"}, &out, &errs); code != exitFail || !strings.Contains(errs.String(), "file exists") {
		t.Errorf("open over a file = %d: %s", code, errs.String())
	}
	if after, _ := os.ReadFile(file); !bytes.Equal(after, want) {
		t.Error("open wrote over the file")
	}
	fresh := filepath.Join(t.TempDir(), "fresh.json")
	for _, c := range []struct {
		args []string
		code int
		says string
	}{
		{nil, exitUsage, "give -file"},
		{[]string{"-file", fresh}, exitUsage, "give -file"},
		{[]string{"eval/bank.json"}, exitUsage, "give -file"},
		{[]string{"-file", fresh, "eval/sheets.json"}, exitFail, `eval/sheets.json: format "tutor-sheet-plan" is not`},
		{[]string{"-file", fresh, "eval/bank.json", "eval/bank.json"}, exitFail, `"eval/bank.json" is named twice`},
		{[]string{"-file", fresh, "eval/none.json"}, exitFail, "no such file"},
		{[]string{"-unknown"}, exitUsage, "flag provided but not defined"},
	} {
		errs.Reset()
		if code := runOpen(c.args, &out, &errs); code != c.code || !strings.Contains(errs.String(), c.says) {
			t.Errorf("open %q = %d: %s", c.args, code, errs.String())
		}
	}
	if _, err := os.Stat(fresh); err == nil {
		t.Error("open wrote a file it refused")
	}
	if code := runOpen([]string{"-h"}, &out, &errs); code != exitOK {
		t.Errorf("open -h = %d", code)
	}
	// Run from another folder, it would name the banks by paths that are not theirs in the repository.
	t.Run("not at the root", func(t *testing.T) {
		t.Chdir("eval")
		errs.Reset()
		if code := runOpen([]string{"-file", fresh, "bank.json"}, &out, &errs); code != exitUsage || !strings.Contains(errs.String(), "from the repository's root") {
			t.Errorf("open in eval/ = %d: %s", code, errs.String())
		}
	})
}
