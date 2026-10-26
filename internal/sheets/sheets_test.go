package sheets_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/sheets"
	"github.com/altnovatrail/tutor/internal/synth"
)

// planned is the plan of the synthetic samples, checked, and its samples.
func planned(t *testing.T) (*sample.Plan, []*sample.Sample) {
	t.Helper()
	plan, samples := synth.Plan()
	if err := plan.Check(); err != nil {
		t.Fatalf("the plan of the synthetic samples: %v", err)
	}
	if err := plan.CheckWithBank(synth.Bank()); err != nil {
		t.Fatalf("the plan of the synthetic samples with the bank: %v", err)
	}
	return plan, samples
}

// save writes a sample as the board saves it, under its plan sheet's name at a moment.
func save(t *testing.T, dir string, sh *sample.PlanSheet, s *sample.Sample, at time.Time) string {
	t.Helper()
	data, err := sample.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	name := sample.SheetFile(sh, at)
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
		t.Fatal(err)
	}
	return name
}

var at = time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC)

// recorded is a directory with every sheet of the plan saved once, and the plan with it.
func recorded(t *testing.T) (string, *sample.Plan, []*sample.Sample) {
	t.Helper()
	plan, samples := planned(t)
	dir := t.TempDir()
	for i, s := range samples {
		save(t, dir, &plan.Sheets[i], s, at.Add(time.Duration(i)*time.Minute))
	}
	return dir, plan, samples
}

func TestRead(t *testing.T) {
	plan, samples := planned(t)
	dir := t.TempDir()
	// Sheet 1 twice: the later file counts. Sheet 2 once, broken; sheet 3 not at all; the rest once.
	save(t, dir, &plan.Sheets[0], samples[0], at)
	last := save(t, dir, &plan.Sheets[0], samples[0], at.Add(time.Second))
	broken := *samples[1]
	broken.Lines = slices.Clone(broken.Lines)
	broken.Lines[0].Text = ""
	save(t, dir, &plan.Sheets[1], &broken, at)
	for i := 3; i < len(plan.Sheets); i++ {
		save(t, dir, &plan.Sheets[i], samples[i], at)
	}
	// Files that are no saved sheet of this plan: another name, another plan's sheet of number 4,
	// a number past the plan, and the split, which is not counted among them.
	other := []string{"notes.txt", "04-20261026-100000-000-posts-correct.json", "99-20261026-100000-000-posts-correct.json"}
	for _, name := range append(slices.Clone(other), sheets.SplitFile) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	r, err := sheets.Read(dir, plan, synth.Bank())
	if err != nil {
		t.Fatal(err)
	}
	if sh := r.Sheets[0]; sh.File != last || sh.Files != 2 || sh.Sample == nil {
		t.Errorf("sheet 1 = %s, %d files, sample %v; want the later of two files", sh.File, sh.Files, sh.Sample != nil)
	}
	if sh := r.Sheets[1]; sh.Sample != nil || !slices.Equal(sh.Problems, []string{"lines[0].text"}) {
		t.Errorf("sheet 2: problems %q, want the path of the missing text alone", sh.Problems)
	}
	if sh := r.Sheets[2]; sh.File != "" || sh.Files != 0 {
		t.Errorf("sheet 3 = %+v, want no file", sh)
	}
	slices.Sort(other)
	if !slices.Equal(r.Other, other) {
		t.Errorf("other files = %q, want %q", r.Other, other)
	}

	// A sample of another scenario under the plan sheet's name does not pass.
	wrong := *samples[3]
	wrong.Scenario = sample.ScenarioSimilarDigits
	save(t, dir, &plan.Sheets[3], &wrong, at.Add(time.Hour))
	r, _ = sheets.Read(dir, plan, synth.Bank())
	if sh := r.Sheets[3]; sh.Sample != nil || !slices.Contains(sh.Problems, "scenario") {
		t.Errorf("sheet 4 with another scenario: problems %q, want the scenario", sh.Problems)
	}
	if _, err := sheets.Read(filepath.Join(dir, "nowhere"), plan, synth.Bank()); err == nil {
		t.Error("a directory that is not there reads")
	}
}

// The summary of the synthetic sheets, all recorded, one twice with a mouse, one off its plan.
func TestSummary(t *testing.T) {
	dir, plan, samples := recorded(t)
	mouse := *samples[0]
	mouse.Strokes = slices.Clone(mouse.Strokes)
	for i, st := range mouse.Strokes {
		points := slices.Clone(st.Points)
		for j := range points {
			points[j].Pressure = 0.5
		}
		mouse.Strokes[i].Points = points
	}
	save(t, dir, &plan.Sheets[0], &mouse, at.Add(time.Hour))
	off := *samples[1]
	off.Expected.Trap = ""
	save(t, dir, &plan.Sheets[1], &off, at.Add(time.Hour))
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := sheets.Read(dir, plan, synth.Bank())
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{
		"Sheets: 7 of the plan's 7 recorded, 7 of them passing the check.",
		"Files: 9 for these sheets, and the last of each counts.",
		"Saved more than once: 1 (2 files), 2 (2 files).",
		"Not read: 1 other file, no saved sheet of this plan.",
		"By task: synthetic-calculation-01 2 of 2, synthetic-free-01 1 of 1, synthetic-linear-01 4 of 4.",
		"By scenario: correct 1 of 1, error 1 of 1, similar-digits 1 of 1, answer-not-last 1 of 1, mirrored-digits 1 of 1, erasure-fixed 1 of 1, erasure-broken 1 of 1.",
		"By outcome as marked: correct 5 of 5, error 2 of 2.",
		"Planned erasures: 2. Slips: 1 erasure on 1 sheet.",
		"With frames, that is with an erasure: 2 sheets.",
		"Written with a mouse, pressure 0.5 at every point: 1.",
		"Marked off the plan: 2 (expected.trap).",
	}, "\n") + "\n"
	if got := sheets.Summary(r); got != want {
		t.Errorf("summary:\n%s\nwant:\n%s", got, want)
	}

	// Nothing of the handwriting comes into the summary: no text of a line, no erased text.
	got := sheets.Summary(r)
	for _, s := range samples {
		for _, l := range s.Lines {
			if strings.Contains(got, l.Text) {
				t.Errorf("the summary holds the text of a line, %q", l.Text)
			}
		}
	}

	// A sheet not recorded, and one that does not pass.
	for _, name := range []string{sample.SheetFile(&plan.Sheets[0], at), sample.SheetFile(&plan.Sheets[0], at.Add(time.Hour))} {
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	broken := *samples[4]
	broken.Lines = slices.Clone(broken.Lines)
	broken.Lines[0].Strokes = []string{}
	name := save(t, dir, &plan.Sheets[4], &broken, at.Add(time.Hour))
	r, _ = sheets.Read(dir, plan, synth.Bank())
	got = sheets.Summary(r)
	for _, want := range []string{
		"Sheets: 6 of the plan's 7 recorded, 5 of them passing the check.\nNot recorded yet: 1.\n",
		"Sheet 5 does not pass the check, " + name + ", at lines[0].strokes.\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("summary:\n%s\nwant it to hold %q", got, want)
		}
	}
}

func TestSplit(t *testing.T) {
	dir, plan, _ := recorded(t)
	r, err := sheets.Read(dir, plan, synth.Bank())
	if err != nil {
		t.Fatal(err)
	}
	sp, err := sheets.Split(r, 7, 3)
	if err != nil {
		t.Fatal(err)
	}
	if err := sp.Check(); err != nil {
		t.Fatalf("the split does not pass its check: %v", err)
	}
	if err := sp.CheckWithPlan(plan); err != nil {
		t.Fatalf("the split does not pass the check with the plan: %v", err)
	}
	if len(sp.Dev) != 3 || len(sp.Test) != 4 || sp.Seed != 7 {
		t.Errorf("split = %d and %d sheets, seed %d; want 3 and 4, seed 7", len(sp.Dev), len(sp.Test), sp.Seed)
	}
	// The same seed, the same split; the parts go by number.
	again, _ := sheets.Split(r, 7, 3)
	if !slices.Equal(again.Dev, sp.Dev) || !slices.Equal(again.Test, sp.Test) {
		t.Errorf("a second split with the same seed differs: %v, %v", again, sp)
	}
	if !slices.IsSortedFunc(sp.Dev, func(a, b sample.SplitSheet) int { return a.Number - b.Number }) {
		t.Errorf("the development part %v is not in the order of numbers", sp.Dev)
	}
	// With seven scenarios of one sheet each, the shares go by the order of the scenarios: the
	// first three take the development part.
	var dev []int
	for _, s := range sp.Dev {
		dev = append(dev, s.Number)
	}
	if !slices.Equal(dev, []int{1, 2, 3}) {
		t.Errorf("development sheets %v, want the first three scenarios' sheets", dev)
	}

	// The seed decides which sheets of a scenario go: four correct sheets, two of them for the
	// development part, are not the same two for every seed.
	four := &sample.Plan{Sheets: []sample.PlanSheet{}}
	r4 := &sheets.Recorded{}
	for i := range 4 {
		sh := sample.PlanSheet{Number: i + 1, Task: "posts", Scenario: sample.ScenarioCorrect}
		four.Sheets = append(four.Sheets, sh)
		r4.Sheets = append(r4.Sheets, sheets.Sheet{Plan: &four.Sheets[i], File: sample.SheetFile(&sh, at), Sample: &sample.Sample{}})
	}
	seen := map[string]bool{}
	for seed := range int64(20) {
		sp, err := sheets.Split(r4, seed, 2)
		if err != nil {
			t.Fatal(err)
		}
		seen[sheetsOf(sp.Dev)] = true
	}
	if len(seen) < 2 {
		t.Errorf("twenty seeds gave %d development parts, want the seed to matter", len(seen))
	}

	// No split while a sheet is missing, nor a part without sheets.
	r.Sheets[2].Sample = nil
	if _, err := sheets.Split(r, 7, 3); err == nil || !strings.Contains(err.Error(), "this one is not: 3") {
		t.Errorf("a split with sheet 3 missing: %v", err)
	}
	r.Sheets[2].Sample = &sample.Sample{}
	for _, dev := range []int{0, 7} {
		if _, err := sheets.Split(r, 7, dev); err == nil || !strings.Contains(err.Error(), "it takes 1 to 6 of the plan's 7 sheets") {
			t.Errorf("a development part of %d sheets: %v", dev, err)
		}
	}
}

// The shares of the plan of the measurement: 40 sheets in five scenarios, 10 for development.
func TestSplitShares(t *testing.T) {
	counts := []struct {
		scenario sample.Scenario
		n, dev   int
	}{
		{sample.ScenarioCorrect, 12, 3}, {sample.ScenarioError, 12, 3}, {sample.ScenarioAnswerNotLast, 2, 0},
		{sample.ScenarioErasureFixed, 7, 2}, {sample.ScenarioErasureBroken, 7, 2},
	}
	plan := &sample.Plan{Sheets: []sample.PlanSheet{}}
	for _, c := range counts {
		for range c.n {
			plan.Sheets = append(plan.Sheets, sample.PlanSheet{Number: len(plan.Sheets) + 1, Task: "posts", Scenario: c.scenario})
		}
	}
	r := &sheets.Recorded{}
	for i := range plan.Sheets {
		sh := &plan.Sheets[i]
		r.Sheets = append(r.Sheets, sheets.Sheet{Plan: sh, File: sample.SheetFile(sh, at), Sample: &sample.Sample{}})
	}
	sp, err := sheets.Split(r, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range counts {
		got := 0
		for _, s := range sp.Dev {
			if plan.Sheets[s.Number-1].Scenario == c.scenario {
				got++
			}
		}
		if got != c.dev {
			t.Errorf("%s: %d sheets for development, want %d", c.scenario, got, c.dev)
		}
	}
}

func sheetsOf(part []sample.SplitSheet) string {
	var b strings.Builder
	for _, s := range part {
		b.WriteString(s.File)
	}
	return b.String()
}

// A split names the scenarios it leaves out of a part, and the sheets whose file no longer counts.
func TestLopsidedAndStale(t *testing.T) {
	dir, plan, samples := recorded(t)
	r, err := sheets.Read(dir, plan, synth.Bank())
	if err != nil {
		t.Fatal(err)
	}
	sp, err := sheets.Split(r, 7, 3)
	if err != nil {
		t.Fatal(err)
	}
	noDev, noTest := sheets.Lopsided(plan, sp)
	// Seven scenarios of one sheet each: three go wholly to development and four wholly to the test.
	if len(noDev) != 4 || len(noTest) != 3 {
		t.Errorf("no development sheet for %v, no test sheet for %v; want four and three scenarios", noDev, noTest)
	}
	if got := sheets.Stale(r, sp); len(got) != 0 {
		t.Errorf("stale right after the split: %v", got)
	}
	// Sheet 2 saved again, and the file of sheet 5 gone.
	save(t, dir, &plan.Sheets[1], samples[1], at.Add(time.Hour))
	if err := os.Remove(filepath.Join(dir, r.Sheets[4].File)); err != nil {
		t.Fatal(err)
	}
	r, _ = sheets.Read(dir, plan, synth.Bank())
	if got := sheets.Stale(r, sp); !slices.Equal(got, []int{2, 5}) {
		t.Errorf("stale = %v, want sheets 2 and 5", got)
	}
}

func TestReadRefusesTooLargeAPlan(t *testing.T) {
	plan := &sample.Plan{Sheets: make([]sample.PlanSheet, sample.MaxSheets+1)}
	if _, err := sheets.Read(t.TempDir(), plan, synth.Bank()); err == nil || !strings.Contains(err.Error(), "two digits") {
		t.Errorf("a plan of %d sheets: %v", sample.MaxSheets+1, err)
	}
}
