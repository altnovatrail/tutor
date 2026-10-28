package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// runMeasure drives tutor-eval run with what the author types and no key, at a fixed moment.
func runMeasure(typed string, args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	at := func() time.Time { return time.Date(2026, 10, 26, 12, 30, 0, 0, time.UTC) }
	code := runRun(context.Background(), args, strings.NewReader(typed), &stdout, &stderr, func(string) string { return "" }, at)
	return code, stdout.String(), stderr.String()
}

// splitSheets records the synthetic sheets and splits them, 2 for development and 5 for the test.
func splitSheets(t *testing.T) []string {
	t.Helper()
	where := recordedSheets(t)
	if code, _, errs := runIt(append([]string{"split", "-seed", "7", "-dev", "2"}, where...)...); code != exitOK {
		t.Fatalf("the split fails: %s", errs)
	}
	return where
}

// A run on the stub goes the whole way without a key or a network: the estimate, the author's
// yes, every call and the log next to the sheets.
func TestRunOnTheStub(t *testing.T) {
	where := splitSheets(t)
	code, out, errs := runMeasure("yes\n", append([]string{"-part", "test", "-stub", "-repeats", "1"}, where...)...)
	if code != exitOK {
		t.Fatalf("the run ends %d: %s%s", code, out, errs)
	}
	if !strings.HasPrefix(out, "The test part: 5 sheets, 2 of them with frames, by 3 judges (opus-high, haiku-low, haiku-low-no-thinking), 1 repeat, in version 1 of the prompt.\n42 calls, expected to cost $0.16, ") ||
		!strings.Contains(out, "The stub answers them by the markup and asks no one") || !strings.Contains(out, "Type yes to start the run: 42 calls made, ") {
		t.Errorf("the run says\n%s", out)
	}
	log := filepath.Join(filepath.Dir(where[1]), "runs", "20261026-123000-000-judge-sheets-test.jsonl")
	if !strings.HasSuffix(out, "spent. The log: "+log+"\n") {
		t.Errorf("the run ends\n%s\nwant the log at %s", out, log)
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]int{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		var rec struct{ Kind string }
		if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
			t.Fatalf("a line of the log is not JSON: %s", sc.Text())
		}
		kinds[rec.Kind]++
	}
	if kinds["run"] != 1 || kinds["sheet"] != 5 || kinds["call"] == 0 || kinds["end"] != 1 {
		t.Errorf("the log holds %v", kinds)
	}
	if info, err := os.Stat(log); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("the log is %v, %v: want it for its owner alone", info.Mode(), err)
	}
	// The same run again would write the same log: it does not overwrite it, and says so before
	// the author is asked.
	if code, out, errs := runMeasure("yes\n", append([]string{"-part", "test", "-stub", "-repeats", "1"}, where...)...); code != exitFail || !strings.Contains(errs, "exists") || out != "" {
		t.Errorf("a second run into the same log ends %d: %s%s", code, out, errs)
	}
}

// Without the author's yes, nothing starts and nothing is written.
func TestRunNotConfirmed(t *testing.T) {
	where := splitSheets(t)
	for _, typed := range []string{"no\n", "", "y\n"} {
		code, out, _ := runMeasure(typed, append([]string{"-part", "dev", "-stub"}, where...)...)
		if code != exitOK || !strings.HasSuffix(out, "The run is not started.\n") {
			t.Errorf("typed %q, the run ends %d:\n%s", typed, code, out)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(where[1]), "runs")); err == nil {
		t.Error("an unconfirmed run wrote a log")
	}
}

// A run that its cap stops says so and fails: a cap below the estimate is told before the author is
// asked.
func TestRunCap(t *testing.T) {
	where := splitSheets(t)
	log := filepath.Join(t.TempDir(), "run.jsonl")
	code, out, errs := runMeasure("yes\n", append([]string{"-part", "test", "-stub", "-max-usd", "0.4", "-log", log}, where...)...)
	if code != exitFail || !strings.Contains(errs, "stopped short by the cap") || !strings.Contains(out, "The cap is $0.40.") || strings.Contains(out, "That is above the") ||
		!strings.Contains(out, "The estimate is above the cap: the run will stop short of its calls unless -max-usd gives it more.") {
		t.Errorf("a capped run ends %d:\n%s%s", code, out, errs)
	}
	// A cap above the plan's is the author's to give, and the run says so before asking.
	code, out, _ = runMeasure("no\n", append([]string{"-part", "test", "-stub", "-max-usd", "60"}, where...)...)
	if code != exitOK || !strings.Contains(out, "That is above the $6.00 that architecture 3.1 and 3.4 plan for -part test.") {
		t.Errorf("a run with a cap of $60 says\n%s", out)
	}
}

// The log goes out of git: a run refuses a log in the work tree outside samples and synthetic, and
// says so before the author is asked.
func TestRunLogInTheWorkTree(t *testing.T) {
	where := splitSheets(t)
	code, out, errs := runMeasure("yes\n", append([]string{"-part", "test", "-stub", "-log", "runs/run.jsonl"}, where...)...)
	if code != exitFail || out != "" || !strings.Contains(errs, "cmd/tutor-eval/runs is in the git work tree") {
		t.Errorf("a run with its log in cmd/tutor-eval ends %d: %s%s", code, out, errs)
	}
	if _, err := os.Stat("runs"); !os.IsNotExist(err) {
		t.Errorf("a refused run made runs in cmd/tutor-eval: %v", err)
	}
}

// A run on the author's sheets, in samples, logs there alone: nothing of them goes to synthetic, the
// folder agents work with. One on sheets in synthetic logs in either. A work tree of its own, with
// both folders, stands for tutor's.
func TestRunLogFolder(t *testing.T) {
	tree := t.TempDir()
	where := splitSheets(t)
	for _, p := range []string{".git", samplesFolder} {
		if err := os.Mkdir(filepath.Join(tree, p), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	mine := filepath.Join(tree, samplesFolder, "sheets")
	if err := os.Rename(where[1], mine); err != nil {
		t.Fatal(err)
	}
	files := where[2:]
	for _, c := range []struct {
		dir, log string
		code     int
		says     string
	}{
		{mine, filepath.Join(tree, syntheticFolder, "runs", "run.jsonl"), exitFail, "outside its samples folder"},
		{mine, "", exitOK, "The run is not started."},
	} {
		args := append([]string{"-part", "dev", "-stub", "-dir", c.dir}, files...)
		if c.log != "" {
			args = append(args, "-log", c.log)
		}
		code, out, errs := runMeasure("no\n", args...)
		if code != c.code || !strings.Contains(out+errs, c.says) {
			t.Errorf("a run on %s logged in %q ends %d: %s%s; want %d and %q", c.dir, c.log, code, out, errs, c.code, c.says)
		}
	}

	drawn := filepath.Join(tree, syntheticFolder, "lines")
	if err := os.MkdirAll(filepath.Dir(drawn), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(mine, drawn); err != nil {
		t.Fatal(err)
	}
	args := append([]string{"-part", "dev", "-stub", "-dir", drawn, "-log", filepath.Join(tree, samplesFolder, "runs", "run.jsonl")}, files...)
	if code, out, errs := runMeasure("no\n", args...); code != exitOK || !strings.HasSuffix(out, "The run is not started.\n") {
		t.Errorf("a run on sheets in synthetic logged in samples ends %d: %s%s", code, out, errs)
	}

	// The folder itself is no directory of sheets: a run would log beside it, outside the folder.
	code, _, errs := runIt(append([]string{"sheets", "-dir", filepath.Join(tree, syntheticFolder)}, files...)...)
	if code != exitUsage || !strings.Contains(errs, "is the synthetic folder itself") {
		t.Errorf("sheets in the synthetic folder itself = %d %q", code, errs)
	}
}

func TestRunRefuses(t *testing.T) {
	where := recordedSheets(t)
	for _, c := range []struct {
		args []string
		code int
		says string
	}{
		{[]string{"-stub"}, exitUsage, `-part is ""`},
		{[]string{"-part", "every", "-stub"}, exitUsage, `-part is "every": give dev, test or all`},
		{[]string{"-part", "test", "-stub", "-kind", "rubric"}, exitUsage, `-kind is "rubric": give essay or hint or judge or reading or workbook`},
		{[]string{"-part", "test", "-stub", "-repeats", "0"}, exitUsage, "-repeats must be 1 or more"},
		{[]string{"-part", "test", "-stub", "-max-usd", "0"}, exitUsage, "-max-usd above 0"},
		{[]string{"-part", "test", "-stub", "-readers", "opus-high,gpt-6"}, exitUsage, `-readers is "opus-high,gpt-6": no configuration "gpt-6": give opus-high, haiku-low`},
		{[]string{"-part", "test", "-stub", "-readers", ""}, exitUsage, `no configuration ""`},
		{[]string{"-part", "test"}, exitUsage, "TUTOR_ANTHROPIC_API_KEY is not set"},
		{[]string{"-part", "test", "-stub"}, exitFail, "the split is not made yet"},
	} {
		code, _, errs := runMeasure("yes\n", append(c.args, where...)...)
		if code != c.code || !strings.Contains(errs, c.says) {
			t.Errorf("%v: ends %d, %s; want %d and %q", c.args, code, errs, c.code, c.says)
		}
	}
}
