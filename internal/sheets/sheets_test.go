package sheets_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"

	"github.com/altnovatrail/tutor/internal/evalsample"
	"github.com/altnovatrail/tutor/internal/protocol"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/sheets"
	"github.com/altnovatrail/tutor/internal/synth"
)

// bank is the synthetic bank as a sample of the protocol names it: by a path and the digest of its
// file.
func bank() *evalsample.Bank {
	b, err := sample.Marshal(synth.Bank())
	if err != nil {
		panic(err)
	}
	eb, err := evalsample.BankOf("eval/synthetic-bank.json", b)
	if err != nil {
		panic(err)
	}
	return eb
}

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

// save writes a sheet as the board saves it, a sample of the protocol, under its plan sheet's name at
// a moment: a sheet code drew as draw writes it, and any other as the record mode does, by a pen.
func save(t *testing.T, dir string, sh *sample.PlanSheet, s *sample.Sample, at time.Time) string {
	t.Helper()
	return saveBy(t, dir, sh, s, at, "pen")
}

// saveBy is save, the strokes of a person's sheet by the pointer given.
func saveBy(t *testing.T, dir string, sh *sample.PlanSheet, s *sample.Sample, at time.Time, pointer string) string {
	t.Helper()
	var es *evalv1.Sample
	var err error
	if s.Writer == sample.WriterSynthetic {
		es, err = evalsample.Of(s, sh, bank(), 1, fmt.Sprintf("synthetic-1-plan-%02d", sh.Number))
	} else {
		h, ids, herr := evalsample.HistoryOf(s, pointer)
		if herr != nil {
			t.Fatal(herr)
		}
		es, err = evalsample.PersonOf(s, sh, bank(), h, ids, fmt.Sprintf("test-%02d-%d", sh.Number, at.UnixMilli()), "TESTCODE01")
	}
	if err != nil {
		t.Fatal(err)
	}
	data, err := protocol.MarshalSample(es)
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
	saveProtocol(t, dir, &plan.Sheets[1], at, func(es *evalv1.Sample) { es.Ink = &evalv1.Sample_Geometry{Geometry: &evalv1.Geometry{}} })
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
	r, err := sheets.Read(dir, plan, bank(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if sh := r.Sheets[0]; sh.File != last || sh.Files != 2 || sh.Sample == nil {
		t.Errorf("sheet 1 = %s, %d files, sample %v; want the later of two files", sh.File, sh.Files, sh.Sample != nil)
	}
	if sh := r.Sheets[1]; sh.Sample != nil || !slices.Equal(sh.Problems, []string{"(the sample is no sheet of measurement with its history)"}) {
		t.Errorf("sheet 2: problems %q, want it no sheet of measurement", sh.Problems)
	}
	if sh := r.Sheets[2]; sh.File != "" || sh.Files != 0 {
		t.Errorf("sheet 3 = %+v, want no file", sh)
	}
	slices.Sort(other)
	if !slices.Equal(r.Other, other) {
		t.Errorf("other files = %q, want %q", r.Other, other)
	}

	// A file of tutor-sample v0 under the plan sheet's name does not pass: convert writes it anew.
	if err := os.WriteFile(filepath.Join(dir, sample.SheetFile(&plan.Sheets[3], at.Add(time.Hour))), []byte(`{"format": "tutor-sample", "version": 0}`), 0o600); err != nil {
		t.Fatal(err)
	}
	r, _ = sheets.Read(dir, plan, bank(), nil)
	if sh := r.Sheets[3]; sh.Sample != nil || !slices.Equal(sh.Problems, []string{sheets.V0Problem}) {
		t.Errorf("sheet 4 of tutor-sample v0: problems %q, want it named", sh.Problems)
	}
	if _, err := sheets.Read(filepath.Join(dir, "nowhere"), plan, bank(), nil); err == nil {
		t.Error("a directory that is not there reads")
	}
}

// The summary of the synthetic sheets, all recorded, one twice with a mouse, one off its plan.
func TestSummary(t *testing.T) {
	dir, plan, samples := recorded(t)
	saveBy(t, dir, &plan.Sheets[0], samples[0], at.Add(time.Hour), "mouse")
	off := *samples[1]
	off.Expected.FirstError = new(1)
	save(t, dir, &plan.Sheets[1], &off, at.Add(time.Hour))
	drawn := *samples[2]
	drawn.Writer = sample.WriterSynthetic
	save(t, dir, &plan.Sheets[2], &drawn, at.Add(time.Hour))
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := sheets.Read(dir, plan, bank(), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{
		"Sheets: 7 of the plan's 7 recorded, 7 of them passing the check.",
		"Files: 10 for these sheets, and the last of each counts.",
		"Saved more than once: 1 (2 files), 2 (2 files), 3 (2 files).",
		"Not read: 1 other file, no saved sheet of this plan.",
		"By task: synthetic-calculation-01 2 of 2, synthetic-free-01 1 of 1, synthetic-linear-01 4 of 4.",
		"By scenario: correct 1 of 1, error 1 of 1, similar-digits 1 of 1, answer-not-last 1 of 1, mirrored-digits 1 of 1, erasure-fixed 1 of 1, erasure-broken 1 of 1.",
		"By outcome as marked: correct 5 of 5, error 2 of 2.",
		"Planned erasures: 2. Slips: 1 erasure on 1 sheet.",
		"With frames, that is with an erasure: 2 sheets.",
		"Drawn by code, the writer synthetic: 3.",
		"Written with a mouse, by the pointer of a stroke: 1.",
		"Marked off the plan: 2 (expected.first_error).",
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
	name := saveProtocol(t, dir, &plan.Sheets[4], at.Add(time.Hour), func(es *evalv1.Sample) { es.Ink = &evalv1.Sample_Geometry{Geometry: &evalv1.Geometry{}} })
	r, _ = sheets.Read(dir, plan, bank(), nil)
	got = sheets.Summary(r)
	for _, want := range []string{
		"Sheets: 6 of the plan's 7 recorded, 5 of them passing the check.\nNot recorded yet: 1.\n",
		"Sheet 5 does not pass the check, " + name + ", at (the sample is no sheet of measurement with its history).\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("summary:\n%s\nwant it to hold %q", got, want)
		}
	}
}

func TestSplit(t *testing.T) {
	dir, plan, _ := recorded(t)
	r, err := sheets.Read(dir, plan, bank(), nil)
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
	r, err := sheets.Read(dir, plan, bank(), nil)
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
	r, _ = sheets.Read(dir, plan, bank(), nil)
	if got := sheets.Stale(r, sp); !slices.Equal(got, []int{2, 5}) {
		t.Errorf("stale = %v, want sheets 2 and 5", got)
	}
}

func TestReadRefusesTooLargeAPlan(t *testing.T) {
	plan := &sample.Plan{Sheets: make([]sample.PlanSheet, sample.MaxSheets+1)}
	if _, err := sheets.Read(t.TempDir(), plan, bank(), nil); err == nil || !strings.Contains(err.Error(), "two digits") {
		t.Errorf("a plan of %d sheets: %v", sample.MaxSheets+1, err)
	}
}

// saveProtocol writes a plan sheet drawn from the seed 1 as a sample of the protocol, changed as
// given, under its plan sheet's name.
func saveProtocol(t *testing.T, dir string, sh *sample.PlanSheet, at time.Time, change func(es *evalv1.Sample)) string {
	t.Helper()
	es, err := evalsample.Of(synth.Sheet(sh, 1), sh, bank(), 1, fmt.Sprintf("synthetic-1-plan-%02d", sh.Number))
	if err != nil {
		t.Fatal(err)
	}
	change(es)
	data, err := protocol.MarshalSample(es)
	if err != nil {
		t.Fatal(err)
	}
	name := sample.SheetFile(sh, at)
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
		t.Fatal(err)
	}
	return name
}

// A directory holds samples of the protocol, which the first key of a file tells, and a file of
// tutor-sample v0 there is a problem of its sheet, which convert mends. A sample of the protocol that the check finds breaking a rule, or that cannot be
// read strictly, is refused as invalid, and so is one the runner refuses for its markup; one that is
// no sheet of measurement is a problem of its sheet. The summary names the refused with their
// reasons, and the mouse by the pointer.
func TestReadProtocol(t *testing.T) {
	plan, _ := planned(t)
	dir := t.TempDir()
	at := time.Date(2026, 11, 7, 9, 0, 0, 0, time.UTC)
	keep := func(*evalv1.Sample) {}
	names := map[int]string{}
	names[1] = saveProtocol(t, dir, &plan.Sheets[0], at, keep)
	names[2] = saveProtocol(t, dir, &plan.Sheets[1], at, keep)
	names[3] = saveProtocol(t, dir, &plan.Sheets[2], at, func(es *evalv1.Sample) { es.Markup = nil })
	names[4] = saveProtocol(t, dir, &plan.Sheets[3], at, func(es *evalv1.Sample) {
		es.Ink = &evalv1.Sample_Geometry{Geometry: &evalv1.Geometry{}}
	})
	names[5] = saveProtocol(t, dir, &plan.Sheets[4], at, func(es *evalv1.Sample) {
		es.GetHistory().GetEvents()[0].GetWrite().Pointer = "mouse"
	})
	names[6] = sample.SheetFile(&plan.Sheets[5], at)
	if err := os.WriteFile(filepath.Join(dir, names[6]), []byte(`{"protocol": "novatrail.eval.v1", "no_such_field": 1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	names[7] = sample.SheetFile(&plan.Sheets[6], at)
	if err := os.WriteFile(filepath.Join(dir, names[7]), []byte(`{"format": "tutor-sample", "version": 0}`), 0o600); err != nil {
		t.Fatal(err)
	}

	var asked []string
	check := func(files []string) (map[string][]int, error) {
		asked = files
		return map[string][]int{filepath.Join(dir, names[2]): {4, 9}}, nil
	}
	r, err := sheets.Read(dir, plan, bank(), check)
	if err != nil {
		t.Fatal(err)
	}
	if len(asked) != 6 {
		t.Errorf("the check is asked of %v, the six samples of the protocol", asked)
	}
	for _, c := range []struct {
		n       int
		passes  bool
		refused evalv1.Refusal
		problem bool
	}{{1, true, 0, false}, {2, false, evalv1.Refusal_REFUSAL_INVALID, false}, {3, false, evalv1.Refusal_REFUSAL_NO_MARKUP, false}, {4, false, 0, true}, {5, true, 0, false}, {6, false, evalv1.Refusal_REFUSAL_INVALID, false}, {7, false, 0, true}} {
		sh := r.Sheets[c.n-1]
		var reason evalv1.Refusal
		if sh.Refused != nil {
			reason = sh.Refused.Reason
		}
		if (sh.Sample != nil) != c.passes || reason != c.refused || (len(sh.Problems) > 0) != c.problem {
			t.Errorf("sheet %d: %v, refused %v, problems %v", c.n, sh.Sample != nil, sh.Refused, sh.Problems)
		}
	}
	if !r.Sheets[4].Mouse || r.Sheets[0].Mouse {
		t.Error("the mouse is not told by the pointer")
	}
	summary := sheets.Summary(r)
	for _, want := range []string{
		"Sheet 2 is refused, " + names[2] + ", as invalid: sample-check finds it breaks a rule of sections 11 and 12, at lines 4, 9.",
		"Sheet 3 is refused, " + names[3] + ", as no markup:",
		"Sheet 6 is refused, " + names[6] + ", as invalid: it cannot be read strictly as a sample of the protocol.",
		"Sheet 4 does not pass the check",
		"Written with a mouse, by the pointer of a stroke: 5.",
		"Sheet 7 does not pass the check, " + names[7] + ", at " + sheets.V0Problem + ".",
	} {
		if !strings.Contains(summary, want) {
			t.Errorf("the summary\n%s\nwant it to hold %q", summary, want)
		}
	}
	if !strings.Contains(summary, "Sheet 2 is refused") || strings.Contains(summary, "the stroke") {
		t.Errorf("the summary quotes the check: %s", summary)
	}

	if _, err := sheets.Read(dir, plan, bank(), func([]string) (map[string][]int, error) { return nil, os.ErrPermission }); err == nil {
		t.Error("a check that cannot run reads the directory")
	}
	if r, err := sheets.Read(dir, plan, bank(), nil); err != nil || r.Sheets[1].Sample == nil {
		t.Errorf("with no check, the sample sample-check fails is read: %v", err)
	}
}

// A file is of the protocol when its top level names one, whatever it names; a sample of
// tutor-sample v0, a file that is no JSON object and one cut short are not.
func TestOfProtocol(t *testing.T) {
	for _, c := range []struct {
		data string
		want bool
	}{
		{`{"protocol": "novatrail.eval.v1", "id": "x"}`, true},
		{`{"id": {"protocol": 1}, "protocol": null}`, true},
		{`{"protocol": 1}`, true},
		{`{"format": "tutor-sample", "version": 0, "lines": [{"protocol": "x"}]}`, false},
		{`["protocol"]`, false},
		{`{"format": "tutor-sample", "lines": [`, false},
		{`{"format"`, false},
		{``, false},
	} {
		if got := sheets.OfProtocol([]byte(c.data)); got != c.want {
			t.Errorf("%s: %v, want %v", c.data, got, c.want)
		}
	}
}
