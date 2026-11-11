package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// A run of admission on the stub, plain, by the pair: every reader reads every sheet in every repeat
// without a key, the report gives the pair's runner's report, and admit rules on it. The drawn sets
// hold too few slots for a stage, and admit says so with the numbers and the file's SHA-256.
func TestAdmitRun(t *testing.T) {
	t.Parallel()
	parent, sets := drawnSets(t)
	// Two sheets of the plan are not written: -part all refuses, and -part recorded takes the rest.
	dir := strings.SplitN(sets[0], ",", 2)[0]
	for _, n := range []string{"05", "06"} {
		files, err := filepath.Glob(filepath.Join(dir, n+"-*.json"))
		if err != nil || len(files) == 0 {
			t.Fatalf("sheet %s of the drawn set: %v, %v", n, files, err)
		}
		for _, file := range files {
			if err := os.Remove(file); err != nil {
				t.Fatal(err)
			}
		}
	}
	if code, _, errs := runMeasure("yes\n", "-kind", "judge", "-part", "recorded", "-stub", "-set", sets[0]); code != exitUsage || !strings.Contains(errs, "recorded for the reading") {
		t.Errorf("-part recorded of the judge ends %d: %s", code, errs)
	}
	if code, _, errs := runMeasure("yes\n", "-kind", "reading", "-part", "all", "-stub", "-reads", "plain", "-set", sets[0]); code != exitFail || !strings.Contains(errs, "sheet 5 is not recorded") {
		t.Errorf("-part all with sheets missing ends %d: %s", code, errs)
	}
	code, out, errs := runMeasure("yes\n", "-kind", "reading", "-part", "recorded", "-stub", "-repeats", "2", "-reads", "plain", "-readers", "opus-high,haiku-medium", "-set", sets[0])
	if code != exitOK {
		t.Fatalf("the run ends %d: %s%s", code, out, errs)
	}
	if !strings.Contains(out, "The sheets recorded: 38 sheets of 1 set (lines 38), the run of admission: by opus-high and haiku-medium in 2 repeats each, without a key at 32 px, in version ") {
		t.Errorf("the run says\n%s", out)
	}
	logs, err := filepath.Glob(filepath.Join(parent, "runs", "*-reading-lines-recorded.jsonl"))
	if err != nil || len(logs) != 1 {
		t.Fatalf("the logs are %v, %v", logs, err)
	}
	if code, out, errs := runIt("report", logs[0]); code != exitOK {
		t.Fatalf("the report ends %d: %s%s", code, out, errs)
	}
	pair := strings.TrimSuffix(logs[0], ".jsonl") + ".opus-high+haiku-medium.report.json"
	code, out, errs = runIt("admit", pair)
	if code != exitOK {
		t.Fatalf("admit ends %d: %s", code, errs)
	}
	for _, want := range []string{
		"## " + filepath.Base(pair),
		"- stage: none: ",
		"20 slots a false alarm could happen on, fewer than 30",
		"- readers: anthropic claude-opus-5-5 high + anthropic claude-haiku-5-5 medium",
		"- set: lines-recorded, 2 repeats",
		"- false alarms: 0 of ",
		"- false correct: 0 of ",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("admit says\n%s\nwithout %q", out, want)
		}
	}
	if !regexp.MustCompile(`- sha256: [0-9a-f]{64}\n`).MatchString(out) {
		t.Errorf("admit names no SHA-256:\n%s", out)
	}
}

func TestAdmitArgs(t *testing.T) {
	if code, _, errs := runIt("admit"); code != exitUsage || !strings.Contains(errs, "usage") {
		t.Errorf("admit with no report: %d %s", code, errs)
	}
	if code, _, _ := runIt("admit", "-h"); code != exitOK {
		t.Errorf("admit -h: %d", code)
	}
	if code, _, errs := runIt("admit", filepath.Join(t.TempDir(), "none.json")); code != exitFail || errs == "" {
		t.Errorf("admit on no file: %d %s", code, errs)
	}
	bad := filepath.Join(t.TempDir(), "bad.report.json")
	if err := os.WriteFile(bad, []byte(`{"protocol": "novatrail.eval.v1", "nothing": 1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, errs := runIt("admit", bad); code != exitFail || !strings.Contains(errs, "bad.report.json") {
		t.Errorf("admit on a file that is no report: %d %s", code, errs)
	}
}
