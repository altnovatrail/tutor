package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/altnovatrail/tutor/internal/hint"
	"github.com/altnovatrail/tutor/internal/sheets"
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
	if code != exitOK || !strings.HasPrefix(out, "The hints of "+log+" that came, each judged once by sonnet-low, in version 2 of the judge's prompt.\n330 calls") || !strings.HasSuffix(out, "The log: "+judged+"\n") {
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
	if code, out, errs := runJudge("yes\n", noKey, "-stub", "-controls", wordsJudge, "-out", controls); code != exitOK || !strings.HasPrefix(out, "The 39 control phrases of "+wordsJudge) {
		t.Errorf("the controls: %d %s%s", code, out, errs)
	}
	if code, out, errs := runIt("report", controls); code != exitOK || !strings.HasPrefix(out, "The judge passes all 39 control phrases, 39 of them with all four answers as expected.\n") {
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

// The cases a run keeps: of the kinds named, and of the sheets the part holds when it is the sheets
// recorded so far; a run with none left is refused.
func TestKeepCases(t *testing.T) {
	data, err := os.ReadFile(wordsCases)
	if err != nil {
		t.Fatal(err)
	}
	read := func() *hint.Set {
		cs, err := hint.ReadCases(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		return &hint.Set{File: "words-cases.json", Cases: cs}
	}
	held := []sheets.Chosen{{Number: 1}, {Number: 31}}
	set := read()
	left, err := keepCases(set, held, []hint.CaseKind{hint.CaseOtherwise, hint.CaseNote})
	if err != nil || len(set.Cases.Cases) != 3+6 || left != " (180 cases of other kinds and 141 cases of sheets the run does not hold left out)" || set.LeftOut != 321 {
		t.Errorf("kept %d, %q, %v", len(set.Cases.Cases), left, err)
	}
	set = read()
	if left, err := keepCases(set, held, nil); err != nil || !strings.Contains(left, "315 cases of sheets the run does not hold") || len(set.Cases.Cases) != 15 {
		t.Errorf("all the cases: %d, %q, %v", len(set.Cases.Cases), left, err)
	}
	if _, err := keepCases(read(), []sheets.Chosen{{Number: 1}}, []hint.CaseKind{hint.CaseNote}); err == nil || !strings.Contains(err.Error(), "no case is left") {
		t.Errorf("no case left: %v", err)
	}
}

// The flags of the cases: -case-kinds needs -cases and kinds it knows, and the sheets recorded so far
// take a hint only with cases.
func TestCaseFlags(t *testing.T) {
	for _, c := range []struct {
		args []string
		says string
	}{
		{[]string{"-kind", "hint", "-part", "all", "-case-kinds", "rung"}, "-case-kinds is for -cases"},
		{[]string{"-kind", "hint", "-part", "all", "-cases", wordsCases, "-case-kinds", "rung,other"}, "give rung, otherwise or note"},
		{[]string{"-kind", "hint", "-part", "recorded", "-dir", t.TempDir()}, "recorded for the reading, or the cases of a hint"},
	} {
		if code, _, errs := runMeasure("yes\n", append([]string{"-stub"}, c.args...)...); code != exitUsage || !strings.Contains(errs, c.says) {
			t.Errorf("run %v = %d, %q; want %q", c.args, code, errs, c.says)
		}
	}
}

// gen words-cases makes the cases of a written plan again, byte for byte the file of eval, and only
// as a new file.
func TestGenWordsCases(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.json")
	var out, errs bytes.Buffer
	if code := runGen([]string{"words-cases", "-seed", "119", "-bank", "../../eval/free-bank.json", "-plan", "../../eval/free-messy.json", "-cases", path}, &out, &errs); code != exitOK ||
		!strings.HasPrefix(out.String(), "36 cases of the seed 119") {
		t.Fatalf("gen words-cases = %d: %s%s", code, out.String(), errs.String())
	}
	a, _ := os.ReadFile(path)
	b, _ := os.ReadFile("../../eval/words-cases-messy.json")
	if !bytes.Equal(a, b) {
		t.Error("the cases are not eval/words-cases-messy.json")
	}
	for _, c := range []struct {
		args []string
		code int
		says string
	}{
		{[]string{"-bank", "../../eval/free-bank.json", "-plan", "../../eval/free-messy.json", "-cases", path}, exitFail, "file exists"},
		{[]string{"-bank", "../../eval/free-bank.json"}, exitUsage, "give -bank and -plan"},
		{[]string{"-bank", "../../eval/free-bank.json", "-plan", "../../eval/free-author.json", "-cases", filepath.Join(t.TempDir(), "x.json")}, exitFail, "no sheet with a mistake"},
		{[]string{"-x"}, exitUsage, "not defined"},
	} {
		errs.Reset()
		if code := runGen(append([]string{"words-cases"}, c.args...), &out, &errs); code != c.code || !strings.Contains(errs.String(), c.says) {
			t.Errorf("gen words-cases %v = %d, %q; want %d, %q", c.args, code, errs.String(), c.code, c.says)
		}
	}
}

// admit-words reads the judge's control phrases and the logs of the hint with their judges' logs,
// and prints the stage and the counts with every file's sum; it needs both, and every log's judge.
func TestAdmitWordsCommand(t *testing.T) {
	parent, sets := drawnSets(t, drawnSet{"words-v1", wordsBank, wordsPlanF})
	if code, out, errs := runMeasure("yes\n", "-kind", "hint", "-part", "all", "-stub", "-repeats", "1", "-readers", "opus-high", "-max-usd", "10", "-cases", wordsCases, "-case-kinds", "otherwise,note", "-set", sets[0]); code != exitOK {
		t.Fatalf("the run ends %d: %s%s", code, out, errs)
	}
	log := filepath.Join(parent, "runs", "20261026-123000-000-hint-words-v1-all.jsonl")
	controls := filepath.Join(parent, "runs", "controls.jsonl")
	for _, args := range [][]string{{"-stub", "-judge", "sonnet-low", "-log", log}, {"-stub", "-judge", "sonnet-low", "-controls", wordsJudge, "-out", controls}} {
		if code, out, errs := runJudge("yes\n", noKey, args...); code != exitOK {
			t.Fatalf("judge-words %v = %d: %s%s", args, code, out, errs)
		}
	}
	code, out, errs := runIt("admit-words", "-controls", controls, log)
	if code != exitOK || !strings.Contains(out, "- stage: author\n") || !strings.Contains(out, "- judge: sonnet-low, right on 39 of 39 control phrases\n") ||
		!strings.Contains(out, "- calls: 150, hints that came 150") || strings.Count(out, ": sha256 ") != 3 {
		t.Errorf("admit-words = %d: %s%s", code, out, errs)
	}
	for _, c := range []struct {
		args []string
		code int
		says string
	}{
		{[]string{log}, exitUsage, "usage"},
		{[]string{"-controls", controls}, exitUsage, "usage"},
		{[]string{"-controls", "nowhere.jsonl", log}, exitFail, "nowhere.jsonl"},
		{[]string{"-controls", log, log}, exitFail, "not of the judge"},
		{[]string{"-controls", controls, controls}, exitFail, "not of the hint"},
		{[]string{"-controls", controls, filepath.Join(parent, "runs", "missing.jsonl")}, exitFail, "missing.jsonl"},
		{[]string{"-x"}, exitUsage, "not defined"},
	} {
		if code, _, errs := runIt(append([]string{"admit-words"}, c.args...)...); code != c.code || !strings.Contains(errs, c.says) {
			t.Errorf("admit-words %v = %d, %q; want %d, %q", c.args, code, errs, c.code, c.says)
		}
	}
	if err := os.Remove(strings.TrimSuffix(log, ".jsonl") + ".judge.jsonl"); err != nil {
		t.Fatal(err)
	}
	if code, _, errs := runIt("admit-words", "-controls", controls, log); code != exitFail || !strings.Contains(errs, ".judge.jsonl") {
		t.Errorf("a log without its judge: %d %s", code, errs)
	}
}
