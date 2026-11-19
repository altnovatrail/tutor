package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gen paper writes a set, and a run of each call of a task from paper asks its cases on the stub by
// the models the library admits, all of them right, which the report counts. The set draws its own
// pictures: -dir is refused, and -paper is no flag of a kind that reads sheets.
func TestPaperOnTheStub(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	set := filepath.Join(dir, "paper.json")
	if code, out, errs := runIt("gen", "paper", "-n", "15", "-seed", "3", "-file", set); code != exitOK || out != "15 cases of the seed 3: "+set+".\n" {
		t.Fatalf("gen paper ends %d: %s%s", code, out, errs)
	}
	for kind, says := range map[string]string{
		"statement": "statement, haiku-high: 15 of 15 answered right, 100.0%",
		"solving":   "solving, haiku-high: 15 of 15 answered right, 100.0%",
		"paper":     "paper, haiku-high: 15 of 15 answered right, 100.0%",
	} {
		log := filepath.Join(dir, kind+".jsonl")
		code, out, errs := runMeasure("yes\n", "-kind", kind, "-paper", set, "-part", "all", "-stub", "-repeats", "1", "-log", log)
		if code != exitOK || !strings.HasPrefix(out, "All the sheets: 15 cases of the set "+set+", drawn with the seed 3, the call "+kind+" by ") {
			t.Fatalf("the run of %s ends %d:\n%s%s", kind, code, out, errs)
		}
		if code, out, errs = runIt("report", log); code != exitOK || !strings.Contains(out, says) {
			t.Errorf("the report of %s ends %d: %s%s", kind, code, out, errs)
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, "paper.jsonl"))
	if err != nil || strings.Count(string(data), `"kind":"case"`) != 15 || strings.Count(string(data), `"kind":"call"`) != 45 {
		t.Errorf("the log of the work holds:\n%.300s, %v", data, err)
	}
	for _, c := range []struct {
		args []string
		says string
	}{
		{[]string{"-kind", "paper", "-paper", set, "-dir", dir, "-part", "all", "-stub"}, "-dir names samples, and the paper draws its pictures from the set"},
		{[]string{"-kind", "reading", "-paper", set, "-part", "all", "-stub"}, "-workbook, -essays and -paper are for"},
		{[]string{"-kind", "solving", "-paper", set, "-bank", set, "-part", "all", "-stub"}, "-bank names sheets"},
		{[]string{"-kind", "solving", "-paper", filepath.Join(dir, "none.json"), "-part", "all", "-stub", "-log", filepath.Join(dir, "x.jsonl")}, "no such file"},
	} {
		if code, _, errs := runMeasure("no\n", c.args...); code == exitOK || !strings.Contains(errs, c.says) {
			t.Errorf("%v ends %d: %s", c.args, code, errs)
		}
	}
	// The configurations -readers names are those asked.
	if code, out, errs := runMeasure("no\n", "-kind", "statement", "-paper", set, "-part", "all", "-stub", "-readers", "opus-high", "-log", filepath.Join(dir, "o.jsonl")); code != exitOK || !strings.Contains(out, "by opus-high, 3 repeats") {
		t.Errorf("-readers: %d %s%s", code, out, errs)
	}
}
