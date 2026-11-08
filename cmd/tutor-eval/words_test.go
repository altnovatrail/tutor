package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The files of the set of the words (T118).
const (
	wordsBank  = "../../eval/lines-gen-1-bank.json"
	wordsFrom  = "../../eval/lines-gen-1.json"
	wordsPlanF = "../../eval/words-sheets.json"
	wordsCases = "../../eval/words-cases.json"
	wordsJudge = "../../eval/words-judge.json"
)

func runJudge(typed string, getenv func(string) string, args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	at := func() time.Time { return time.Date(2026, 11, 8, 12, 30, 0, 0, time.UTC) }
	code := runJudgeWords(context.Background(), args, strings.NewReader(typed), &stdout, &stderr, getenv, at)
	return code, stdout.String(), stderr.String()
}

func noKey(string) string { return "" }

// gen words makes the set's plan and cases again, byte for byte the files of eval, and only as new
// files; it needs its four files, a plan of lines, and a bank the plan's tasks are in.
func TestGenWords(t *testing.T) {
	dir := t.TempDir()
	plan, cases := filepath.Join(dir, "words-sheets.json"), filepath.Join(dir, "words-cases.json")
	var out, errs bytes.Buffer
	if code := runGen([]string{"words", "-seed", "118", "-bank", wordsBank, "-from", wordsFrom, "-plan", plan, "-cases", cases}, &out, &errs); code != exitOK ||
		!strings.HasPrefix(out.String(), "40 sheets and 330 cases of the seed 118") {
		t.Fatalf("gen words = %d: %s%s", code, out.String(), errs.String())
	}
	for made, want := range map[string]string{plan: wordsPlanF, cases: wordsCases} {
		a, _ := os.ReadFile(made)
		b, _ := os.ReadFile(want)
		if !bytes.Equal(a, b) {
			t.Errorf("%s is not %s", made, want)
		}
	}
	for _, c := range []struct {
		args []string
		code int
		says string
	}{
		{[]string{"-bank", wordsBank, "-from", wordsFrom, "-plan", plan, "-cases", filepath.Join(dir, "c.json")}, exitFail, "file exists"},
		{[]string{"-bank", wordsBank, "-from", wordsFrom, "-plan", filepath.Join(dir, "p.json")}, exitUsage, "give -bank and -from"},
		{[]string{"-bank", "nowhere.json", "-from", wordsFrom, "-plan", filepath.Join(dir, "p.json"), "-cases", filepath.Join(dir, "c.json")}, exitFail, "nowhere.json"},
		{[]string{"-bank", "../../eval/free-bank.json", "-from", "../../eval/free-sheets.json", "-plan", filepath.Join(dir, "p.json"), "-cases", filepath.Join(dir, "c.json")}, exitFail, "plan of lines"},
		{[]string{"-seed", "x"}, exitUsage, "invalid value"},
	} {
		errs.Reset()
		if code := runGen(append([]string{"words"}, c.args...), &out, &errs); code != c.code || !strings.Contains(errs.String(), c.says) {
			t.Errorf("gen words %v = %d, %q; want %d, %q", c.args, code, errs.String(), c.code, c.says)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "p.json")); err == nil {
		t.Error("a failed gen words left a plan")
	}
}

// The set of the words goes the whole way on the stub: drawn, asked case by case, judged, and
// reported; the judge's control phrases too. A run of cases takes one set of their plan, and the
// judge one log of the hint or the controls.
func TestWordsOnTheStub(t *testing.T) {
	parent, sets := drawnSets(t, drawnSet{"words-v1", wordsBank, wordsPlanF})
	code, out, errs := runMeasure("yes\n", "-kind", "hint", "-part", "all", "-stub", "-repeats", "1", "-readers", "opus-high", "-max-usd", "10", "-cases", wordsCases, "-set", sets[0])
	if code != exitOK || !strings.HasPrefix(out, "All the sheets: 330 cases of the set of the words in words-cases.json, on 40 sheets with a mistake of 40 in words-v1, by opus-high in 1 repeat, in version 2 of the hint's prompt.\n330 calls") {
		t.Fatalf("the run ends %d: %s%s", code, out, errs)
	}
	log := filepath.Join(parent, "runs", "20261026-123000-000-hint-words-v1-all.jsonl")
	code, out, errs = runJudge("yes\n", noKey, "-stub", "-log", log)
	judged := strings.TrimSuffix(log, ".jsonl") + ".judge.jsonl"
	if code != exitOK || !strings.HasPrefix(out, "The hints of "+log+" that came, each judged once by opus-high, in version 1 of the judge's prompt.\n330 calls") || !strings.HasSuffix(out, "The log: "+judged+"\n") {
		t.Fatalf("the judge ends %d: %s%s", code, out, errs)
	}
	if code, out, errs := runIt("report", log); code != exitOK || !strings.Contains(out, "opus-high: leaked before the check 0 of 330") {
		t.Errorf("the report of the hint: %d %s%s", code, out, errs)
	}
	if code, out, errs := runIt("report", judged); code != exitOK || !strings.HasPrefix(out, "opus-high: 330 hints judged; the sex told 0") || !strings.Contains(out, ".judge.words.md") {
		t.Errorf("the report of the judge: %d %s%s", code, out, errs)
	}
	if code, _, errs := runJudge("yes\n", noKey, "-stub", "-log", log); code != exitFail || !strings.Contains(errs, "exists") {
		t.Errorf("a judge over a log of it: %d %s", code, errs)
	}
	controls := filepath.Join(t.TempDir(), "controls.jsonl")
	if code, out, errs := runJudge("yes\n", noKey, "-stub", "-controls", wordsJudge, "-out", controls); code != exitOK || !strings.HasPrefix(out, "The 32 control phrases of "+wordsJudge) {
		t.Errorf("the controls: %d %s%s", code, out, errs)
	}
	if code, out, errs := runIt("report", controls); code != exitOK || !strings.HasPrefix(out, "The judge is right on all 32 control phrases.\n") {
		t.Errorf("the report of the controls: %d %s%s", code, out, errs)
	}
	if code, out, _ := runJudge("no\n", noKey, "-stub", "-controls", wordsJudge, "-out", filepath.Join(t.TempDir(), "c.jsonl")); code != exitOK || !strings.HasSuffix(out, "The run is not started.\n") {
		t.Errorf("the judge not started: %d %s", code, out)
	}

	// The cases with another kind, over two sets, of another plan, and broken.
	other := filepath.Join(t.TempDir(), "other.json")
	data, _ := os.ReadFile(wordsCases)
	if err := os.WriteFile(other, bytes.Replace(data, []byte(`"plan": "words-sheets.json"`), []byte(`"plan": "sheets.json"`), 1), 0o644); err != nil {
		t.Fatal(err)
	}
	broken := filepath.Join(t.TempDir(), "broken.json")
	if err := os.WriteFile(broken, bytes.Replace(data, []byte(`"sheet": 1,`), []byte(`"sheet": 99,`), 1), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		args []string
		code int
		says string
	}{
		{[]string{"-kind", "reading", "-cases", wordsCases, "-set", sets[0]}, exitUsage, "-cases is for -kind hint"},
		{[]string{"-kind", "hint", "-cases", other, "-set", sets[0]}, exitFail, "of the plan sheets.json"},
		{[]string{"-kind", "hint", "-cases", broken, "-set", sets[0]}, exitFail, "no sheet 99"},
		{[]string{"-kind", "hint", "-cases", "nowhere.json", "-set", sets[0]}, exitFail, "nowhere.json"},
		{[]string{"-kind", "hint", "-cases", wordsJudge, "-set", sets[0]}, exitFail, "the cases"},
	} {
		code, _, errs := runMeasure("yes\n", append([]string{"-part", "all", "-stub", "-repeats", "1"}, c.args...)...)
		if code != c.code || !strings.Contains(errs, c.says) {
			t.Errorf("run %v = %d, %q; want %d, %q", c.args, code, errs, c.code, c.says)
		}
	}
	set := sheetFlags{dir: "d", bank: wordsBank, plan: wordsPlanF}
	if _, _, _, err := casesOf(wordsCases, []sheetFlags{set, set}); err == nil || !strings.Contains(err.Error(), "one plan") {
		t.Errorf("the cases over two sets: %v", err)
	}
	for _, c := range []struct {
		args []string
		code int
		says string
	}{
		{nil, exitUsage, "give -log or -controls"},
		{[]string{"-stub", "-log", log, "-controls", wordsJudge}, exitUsage, "give -log or -controls"},
		{[]string{"-stub", "-controls", wordsJudge, "-judge", "gpt"}, exitUsage, "-judge is \"gpt\""},
		{[]string{"-stub", "-controls", wordsJudge, "-max-usd", "0"}, exitUsage, "-max-usd"},
		{[]string{"-controls", wordsJudge}, exitUsage, "is not set"},
		{[]string{"-stub", "-log", "nowhere.jsonl"}, exitFail, "nowhere.jsonl"},
		{[]string{"-stub", "-log", judged}, exitFail, "not of the hint"},
		{[]string{"-stub", "-controls", wordsCases}, exitFail, "the controls"},
		{[]string{"-stub", "-controls", wordsJudge, "-out", "eval/x.jsonl"}, exitFail, "outside"},
		{[]string{"-stub", "-x"}, exitUsage, "not defined"},
	} {
		if code, _, errs := runJudge("yes\n", noKey, c.args...); code != c.code || !strings.Contains(errs, c.says) {
			t.Errorf("judge-words %v = %d, %q; want %d, %q", c.args, code, errs, c.code, c.says)
		}
	}
}
