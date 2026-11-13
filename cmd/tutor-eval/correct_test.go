package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The files of the set of the words after a correct answer (T143).
const (
	correctPlanF = "../../eval/correct-sheets.json"
	correctCases = "../../eval/correct-cases.json"
)

// gen correct makes the set's plan and cases again, byte for byte the files of eval.
func TestGenCorrect(t *testing.T) {
	dir := t.TempDir()
	plan, cases := filepath.Join(dir, "correct-sheets.json"), filepath.Join(dir, "correct-cases.json")
	var out, errs bytes.Buffer
	if code := runGen([]string{"correct", "-seed", "143", "-bank", wordsBank, "-from", wordsFrom, "-plan", plan, "-cases", cases}, &out, &errs); code != exitOK ||
		!strings.HasPrefix(out.String(), "40 sheets and 110 cases of the seed 143") {
		t.Fatalf("gen correct = %d: %s%s", code, out.String(), errs.String())
	}
	for made, want := range map[string]string{plan: correctPlanF, cases: correctCases} {
		a, _ := os.ReadFile(made)
		b, _ := os.ReadFile(want)
		if !bytes.Equal(a, b) {
			t.Errorf("%s is not %s", made, want)
		}
	}
	errs.Reset()
	if code := runGen([]string{"correct", "-bank", wordsBank, "-from", wordsFrom, "-plan", filepath.Join(dir, "p.json"), "-cases", filepath.Join(dir, "c.json"), "-x"}, &out, &errs); code != exitUsage {
		t.Errorf("gen correct with a flag unknown = %d: %s", code, errs.String())
	}
}

// The set of the words after a correct answer goes the whole way on the stub: drawn, asked case by
// case, judged, reported and admitted by the rule. A run of it needs its cases, and admit-words takes
// logs of one kind.
func TestCorrectOnTheStub(t *testing.T) {
	t.Parallel()
	parent, sets := drawnSets(t, drawnSet{"correct-v1", wordsBank, correctPlanF})
	code, out, errs := runMeasure("yes\n", "-kind", "correct", "-part", "all", "-stub", "-repeats", "1", "-readers", "opus-high", "-max-usd", "10", "-cases", correctCases, "-set", sets[0])
	if code != exitOK || !strings.HasPrefix(out, "All the sheets: 110 cases of the set of the words after a correct answer in correct-cases.json, on 40 sheets whose answer is right of 40 in correct-v1, by opus-high in 1 repeat, in version 1 of their prompt.\n110 calls") {
		t.Fatalf("the run ends %d: %s%s", code, out, errs)
	}
	log := filepath.Join(parent, "runs", "20261026-123000-000-correct-correct-v1-all.jsonl")
	code, out, errs = runJudge("yes\n", noKey, "-stub", "-judge", "sonnet-low", "-log", log)
	judged := strings.TrimSuffix(log, ".jsonl") + ".judge.jsonl"
	if code != exitOK || !strings.Contains(out, "110 calls") {
		t.Fatalf("the judge ends %d: %s%s", code, out, errs)
	}
	if code, out, errs := runIt("report", log); code != exitOK || !strings.Contains(out, "opus-high: leaked before the check 0 of 110") {
		t.Errorf("the report: %d %s%s", code, out, errs)
	}
	if code, out, errs := runIt("report", judged); code != exitOK || !strings.HasPrefix(out, "opus-high: 110 words judged; the sex told 0") {
		t.Errorf("the report of the judge: %d %s%s", code, out, errs)
	}
	controls := filepath.Join(parent, "runs", "controls.jsonl")
	if code, out, errs := runJudge("yes\n", noKey, "-stub", "-judge", "sonnet-low", "-controls", wordsJudge, "-out", controls); code != exitOK {
		t.Fatalf("the controls: %d %s%s", code, out, errs)
	}
	code, out, errs = runIt("admit-words", "-controls", controls, log)
	if code != exitOK || !strings.HasPrefix(out, "# The admission of the words after a correct answer\n\n- stage: author\n") || !strings.Contains(out, "- calls: 110, words that came 110") ||
		!strings.Contains(out, "- the code: the note echoed 0 of 20\n") || strings.Count(out, ": sha256 ") != 3 {
		t.Errorf("admit-words = %d: %s%s", code, out, errs)
	}

	// A log of the hint among them, and a log of another kind.
	if _, _, errs := runIt("admit-words", "-controls", controls, log, judged); !strings.Contains(errs, "not of the hint or of the words after a correct answer") {
		t.Errorf("admit-words of a judge's log: %s", errs)
	}
	hintLog := filepath.Join(t.TempDir(), "hint.jsonl")
	if err := os.WriteFile(hintLog, []byte(`{"kind": "run", "call_kind": "hint", "part": "all"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, errs := runIt("admit-words", "-controls", controls, log, hintLog); !strings.Contains(errs, `a log of the call kind "hint" among logs of "correct"`) {
		t.Errorf("admit-words of two kinds: %s", errs)
	}
	for _, c := range []struct {
		args []string
		code int
		says string
	}{
		{[]string{"-kind", "correct", "-set", sets[0]}, exitUsage, "give -cases"},
		{[]string{"-kind", "correct", "-cases", correctCases, "-case-kinds", "rung", "-set", sets[0]}, exitUsage, "give work, choice or note"},
		{[]string{"-kind", "correct", "-cases", wordsCases, "-set", sets[0]}, exitFail, "the cases"},
	} {
		code, _, errs := runMeasure("yes\n", append([]string{"-part", "all", "-stub", "-repeats", "1"}, c.args...)...)
		if code != c.code || !strings.Contains(errs, c.says) {
			t.Errorf("run %v = %d, %q; want %d, %q", c.args, code, errs, c.code, c.says)
		}
	}
}
