package main

import (
	"encoding/json"
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// drawnOnce is a directory out of the work tree where the pages of the workbook and the essays of
// eval/ are drawn once with the seed 1, for every test that reads them: drawing the essays takes
// seconds.
var drawnOnce struct {
	sync.Once
	dir string
	err error
}

func TestMain(m *testing.M) {
	code := m.Run()
	if drawnOnce.dir != "" {
		_ = os.RemoveAll(drawnOnce.dir)
	}
	os.Exit(code)
}

// drawnOf is the directory of the samples of the workbook or the essays of eval/, drawn with the
// seed 1: theWorkbook's or theEssays'. A test that changes them changes a copy (copied).
func drawnOf(t *testing.T, file string) string {
	t.Helper()
	drawnOnce.Do(func() {
		if drawnOnce.dir, drawnOnce.err = os.MkdirTemp("", "tutor-eval-drawn-"); drawnOnce.err != nil {
			return
		}
		for _, args := range [][]string{
			{"draw", "-workbook", theWorkbook, "-dir", filepath.Join(drawnOnce.dir, "workbook")},
			{"draw", "-essays", theEssays, "-dir", filepath.Join(drawnOnce.dir, "essays")},
		} {
			if code, _, errs := runIt(args...); code != exitOK {
				drawnOnce.err = errors.New(errs)
				return
			}
		}
	})
	if drawnOnce.err != nil {
		t.Fatal(drawnOnce.err)
	}
	return filepath.Join(drawnOnce.dir, strings.TrimSuffix(filepath.Base(file), ".json"))
}

// copied is a copy of a directory of samples, which a test may change.
func copied(t *testing.T, dir string) string {
	t.Helper()
	to := filepath.Join(t.TempDir(), filepath.Base(dir))
	if err := os.CopyFS(to, os.DirFS(dir)); err != nil {
		t.Fatal(err)
	}
	return to
}

// edit changes the JSON of a sample file.
func edit(t *testing.T, path string, change func(map[string]any)) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var o map[string]any
	if err := json.Unmarshal(data, &o); err != nil {
		t.Fatal(err)
	}
	change(o)
	if data, err = json.Marshal(o); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// draw -workbook draws every page of the workbook as a sample of the protocol, page-NN.json, and
// with -png the crop of every filled box at 32 px, 200 × 200; drawn again it draws none, and keeps
// what is there. A crop that cannot be saved saves no sample of its page.
func TestDrawPages(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir, pics := filepath.Join(root, "workbook"), filepath.Join(root, "png")
	code, out, errs := runIt("draw", "-workbook", theWorkbook, "-dir", dir, "-png", pics)
	if code != exitOK || !strings.HasPrefix(out, "Drawn by code with the seed 1: 20 pages, pages 1, 2,") || !strings.HasSuffix(out, "Kept as they were: none.\nCrops of the filled boxes at 32 px: 200 files, into "+pics+".\n") {
		t.Fatalf("draw -workbook = %d %q %q", code, out, errs)
	}
	if files, _ := filepath.Glob(filepath.Join(dir, "page-*.json")); len(files) != 20 {
		t.Errorf("%d samples of pages", len(files))
	}
	f, err := os.Open(filepath.Join(pics, "page-01-item-05-box-2.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if cfg, err := png.DecodeConfig(f); err != nil || cfg.Width != 200 || cfg.Height != 200 {
		t.Errorf("the crop of page 1, item 5, box 2 is %+v, %v", cfg, err)
	}
	if code, out, _ := runIt("draw", "-workbook", theWorkbook, "-dir", dir); code != exitOK || !strings.HasPrefix(out, "Drawn by code with the seed 1: none, into "+dir+".\nKept as they were: 20 pages,") {
		t.Errorf("draw -workbook again = %d %q", code, out)
	}
	// Another seed does not mix its samples with those of the seed 1.
	if code, _, errs := runIt("draw", "-workbook", theWorkbook, "-dir", dir, "-seed", "2"); code != exitUsage || !strings.Contains(errs, "holds samples drawn with the seed 1: draw -seed 2 into a directory of its own") {
		t.Errorf("draw -seed 2 = %d %q", code, errs)
	}
	blocked, crops := filepath.Join(root, "blocked"), filepath.Join(root, "crops")
	if err := os.MkdirAll(filepath.Join(crops, "page-01-item-01-box-1.png"), 0o700); err != nil {
		t.Fatal(err)
	}
	if code, _, errs := runIt("draw", "-workbook", theWorkbook, "-dir", blocked, "-png", crops); code != exitFail || !strings.Contains(errs, "page 1 is not saved") {
		t.Errorf("draw where a crop cannot go = %d %q", code, errs)
	}
	if entries, _ := os.ReadDir(blocked); len(entries) != 0 {
		t.Errorf("a page whose crop could not be saved left %d files", len(entries))
	}
}

// draw -essays draws every essay as a sample of the protocol, essay-NN.json, and with -png the crop
// of each at 32 px.
func TestDrawEssays(t *testing.T) {
	t.Parallel()
	dir := drawnOf(t, theEssays)
	if files, _ := filepath.Glob(filepath.Join(dir, "essay-*.json")); len(files) != 20 {
		t.Errorf("%d samples of essays", len(files))
	}
	to := copied(t, dir)
	if err := os.Remove(filepath.Join(to, "essay-03.json")); err != nil {
		t.Fatal(err)
	}
	pics := filepath.Join(t.TempDir(), "png")
	code, out, errs := runIt("draw", "-essays", theEssays, "-dir", to, "-png", pics)
	if code != exitOK || out != "Drawn by code with the seed 1: 1 essay, essays 3, into "+to+".\nKept as they were: 19 essays, essays 1, 2, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20.\nCrops of the essays at 32 px: 1 file, into "+pics+".\n" {
		t.Fatalf("draw -essays = %d %q %q", code, out, errs)
	}
	if _, err := os.Stat(filepath.Join(pics, "essay-03.png")); err != nil {
		t.Error(err)
	}
}

// draw refuses the flags of the sheets with a file of its own, two files at once, a directory of the
// samples or the crops outside synthetic in the work tree, and the crops among the samples.
func TestDrawOwnRefuses(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "workbook")
	for _, c := range []struct {
		name string
		args []string
		says string
	}{
		{"a bank", []string{"-workbook", theWorkbook, "-dir", dir, "-bank", measurementBank}, "-bank is for the sheets"},
		{"a share of the shift", []string{"-essays", theEssays, "-dir", dir, "-shift", "0.5"}, "-shift is for the sheets"},
		{"two files", []string{"-workbook", theWorkbook, "-essays", theEssays, "-dir", dir}, "give -workbook or -essays"},
		{"no file", []string{"-workbook", "", "-dir", dir}, "name the file to draw"},
		{"no directory", []string{"-workbook", theWorkbook, "-dir", ""}, "-dir is empty"},
		// The tests run in cmd/tutor-eval, where synthetic is a folder git does not ignore.
		{"the default directory here", []string{"-workbook", theWorkbook}, "cmd/tutor-eval/synthetic/workbook is in the git work tree"},
		{"the author's folder", []string{"-workbook", theWorkbook, "-dir", "../../samples/workbook-test"}, "outside its synthetic folder"},
		{"no directory of the crops", []string{"-workbook", theWorkbook, "-dir", dir, "-png", ""}, "-png is empty"},
		{"the crops in the work tree", []string{"-workbook", theWorkbook, "-dir", dir, "-png", "png"}, "cmd/tutor-eval/png is in the git work tree"},
		{"the crops among the samples", []string{"-workbook", theWorkbook, "-dir", dir, "-png", filepath.Join(dir, "png")}, "is among the samples of"},
	} {
		t.Run(c.name, func(t *testing.T) {
			code, _, errs := runIt(append([]string{"draw"}, c.args...)...)
			if code != exitUsage || !strings.Contains(errs, c.says) {
				t.Errorf("draw = %d, %q; want %d and %q", code, errs, exitUsage, c.says)
			}
		})
	}
	for _, made := range []string{"png", "synthetic", "../../samples/workbook-test", dir} {
		if _, err := os.Stat(made); !os.IsNotExist(err) {
			t.Errorf("a refused draw made %s: %v", made, err)
		}
	}
	if code, _, errs := runIt("draw", "-essays", measurementPlan, "-dir", dir); code != exitFail || !strings.Contains(errs, `unknown field "sheets"`) {
		t.Errorf("draw -essays on a sheet plan = %d %q", code, errs)
	}
}

// The summaries and the runs read a directory of samples as the runner refuses them (section 13):
// one with no markup, of another digest of its open view, or breaking a rule of sample-check is
// named with its reason, and a run asks of the others; one off its file, or not drawn, stops a run;
// samples of two seeds stop both; and other files are named and not read.
func TestDrawnRefusals(t *testing.T) {
	dir := copied(t, drawnOf(t, theWorkbook))
	edit(t, filepath.Join(dir, "page-01.json"), func(o map[string]any) { delete(o, "markup") })
	edit(t, filepath.Join(dir, "page-02.json"), func(o map[string]any) {
		o["task"].(map[string]any)["open_view_digest"] = "sha256:" + strings.Repeat("0", 64)
	})
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	checking(t, func(paths []string) (map[string][]int, error) {
		return map[string][]int{filepath.Join(dir, "page-03.json"): {21}}, nil
	})
	code, out, errs := runIt("workbook", "-file", theWorkbook, "-dir", dir)
	for _, want := range []string{
		"Samples in " + dir + ": 17 of 20 read; not drawn: none; refused, and nothing of them goes to AI: page 1 (no markup: the sample has no markup, and the runner measures nothing on it); page 2 (another task: the open view of its task workbook-02 is of another digest than the workbook's); page 3 (invalid: sample-check finds it breaks a rule of sections 11 and 12, at line 21); off their file: none.\n",
		"Not read, no sample of the file: notes.txt.\n",
		"Crops at 32 px: 170 crops,",
	} {
		if code != exitOK || !strings.Contains(out, want) {
			t.Errorf("workbook = %d %q %q, want %q in it", code, out, errs, want)
		}
	}
	run := func(more ...string) (int, string, string) {
		return runMeasure("yes\n", append([]string{"-kind", "workbook", "-workbook", theWorkbook, "-dir", dir, "-part", "all", "-stub", "-repeats", "1", "-log", filepath.Join(t.TempDir(), "w.jsonl")}, more...)...)
	}
	if code, out, errs := run(); code != exitOK || !strings.HasPrefix(out, "Refused, and nothing of them goes to AI: page 1 (no markup:") || !strings.Contains(out, "All the sheets: 17 pages of the workbook") {
		t.Errorf("run = %d %s%s", code, out, errs)
	}

	edit(t, filepath.Join(dir, "page-04.json"), func(o map[string]any) {
		slots := o["markup"].(map[string]any)["truth"].(map[string]any)["answer_slots"].([]any)
		slots[0] = map[string]any{"answer_slot": map[string]any{"item": 1, "slot": 1}, "value": "999"}
	})
	if code, out, _ := runIt("workbook", "-file", theWorkbook, "-dir", dir); code != exitOK || !strings.Contains(out, "off their file: page 4 (markup.truth.answer_slots[0]).") {
		t.Errorf("workbook on a sample off its file = %d %q", code, out)
	}
	if code, _, errs := run(); code != exitFail || !strings.Contains(errs, "the sample of page 4 is off its file at markup.truth.answer_slots[0]") {
		t.Errorf("run on a sample off its file = %d %q", code, errs)
	}
	if err := os.Remove(filepath.Join(dir, "page-04.json")); err != nil {
		t.Fatal(err)
	}
	if code, out, _ := runIt("workbook", "-file", theWorkbook, "-dir", dir); code != exitOK || !strings.Contains(out, "not drawn: 4;") {
		t.Errorf("workbook on a page not drawn = %d %q", code, out)
	}
	if code, _, errs := run(); code != exitFail || !strings.Contains(errs, "page 4 is not drawn: tutor-eval draw draws it") {
		t.Errorf("run on a page not drawn = %d %q", code, errs)
	}
	edit(t, filepath.Join(dir, "page-05.json"), func(o map[string]any) { o["synthetic"] = map[string]any{"seed": "2"} })
	if code, _, errs := runIt("workbook", "-file", theWorkbook, "-dir", dir); code != exitFail || !strings.Contains(errs, "holds samples drawn with the seeds 1, 2") {
		t.Errorf("workbook on two seeds = %d %q", code, errs)
	}

	checking(t, func([]string) (map[string][]int, error) { return nil, errors.New("buf fails") })
	if code, _, errs := runIt("essays", "-file", theEssays, "-dir", drawnOf(t, theEssays)); code != exitFail || !strings.Contains(errs, "buf fails") {
		t.Errorf("essays with a check that cannot run = %d %q", code, errs)
	}
}

// An essay refused is named, and the run asks of the others.
func TestDrawnEssayRefused(t *testing.T) {
	dir := copied(t, drawnOf(t, theEssays))
	edit(t, filepath.Join(dir, "essay-02.json"), func(o map[string]any) { o["task"].(map[string]any)["task"] = "essay-99" })
	code, out, errs := runMeasure("yes\n", "-kind", "essay", "-essays", theEssays, "-dir", dir, "-part", "dev", "-stub", "-repeats", "1", "-readers", "haiku-low", "-log", filepath.Join(t.TempDir(), "e.jsonl"))
	if code != exitOK || !strings.HasPrefix(out, "Refused, and nothing of them goes to AI: essay 2 (another task: the essays eval/essays.json have no essay of the task essay-99).\nThe dev part: 4 essays of") {
		t.Errorf("run = %d %s%s", code, out, errs)
	}
	if code, out, _ := runIt("essays", "-file", theEssays, "-dir", t.TempDir()); code != exitOK || !strings.Contains(out, "; no sample drawn.\n") || strings.Contains(out, "Words") {
		t.Errorf("essays on an empty directory = %d %q", code, out)
	}
}

// draw, the summaries and the runs stop on what they cannot read or write: a directory of samples
// that is not there, one that cannot be made, a file whose name gives no ID of a sample, and a crop
// of an essay that cannot be saved.
func TestDrawnFails(t *testing.T) {
	none := filepath.Join(t.TempDir(), "none")
	for _, args := range [][]string{
		{"workbook", "-file", theWorkbook, "-dir", none},
		{"essays", "-file", theEssays, "-dir", none},
	} {
		if code, _, errs := runIt(args...); code != exitFail || !strings.Contains(errs, "no such file") {
			t.Errorf("%s on no directory = %d %q", args[0], code, errs)
		}
	}
	for _, kind := range []string{"workbook", "essay"} {
		if code, _, errs := runMeasure("yes\n", "-kind", kind, "-dir", none, "-part", "dev", "-stub", "-log", filepath.Join(t.TempDir(), "x.jsonl")); code != exitFail || !strings.Contains(errs, "no such file") {
			t.Errorf("run -kind %s on no directory = %d %q", kind, code, errs)
		}
	}
	if code, _, errs := runIt("essays"); code != exitUsage || !strings.Contains(errs, "cmd/tutor-eval/synthetic/essays is in the git work tree") {
		t.Errorf("essays in its default directory here = %d %q", code, errs)
	}
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"-workbook", theWorkbook, "-dir", filepath.Join(file, "workbook")},
		{"-essays", theEssays, "-dir", t.TempDir(), "-png", filepath.Join(file, "png")},
	} {
		if code, _, errs := runIt(append([]string{"draw"}, args...)...); code != exitFail || !strings.Contains(errs, "not a directory") {
			t.Errorf("draw %v = %d %q", args, code, errs)
		}
	}
	long := strings.Repeat("long-", 12)
	for flag, src := range map[string]string{"-workbook": theWorkbook, "-essays": theEssays} {
		named := filepath.Join(t.TempDir(), long+filepath.Base(src))
		if err := os.WriteFile(named, []byte(readDoc(t, src)), 0o600); err != nil {
			t.Fatal(err)
		}
		if code, _, errs := runIt("draw", flag, named, "-dir", t.TempDir()); code != exitFail || !strings.Contains(errs, "give the plan a shorter name") {
			t.Errorf("draw %s of a long name = %d %q", flag, code, errs)
		}
	}
	crops := filepath.Join(t.TempDir(), "crops")
	if err := os.MkdirAll(filepath.Join(crops, "essay-01.png"), 0o700); err != nil {
		t.Fatal(err)
	}
	if code, _, errs := runIt("draw", "-essays", theEssays, "-dir", t.TempDir(), "-png", crops); code != exitFail || !strings.Contains(errs, "essay 1 is not saved") {
		t.Errorf("draw -essays where a crop cannot go = %d %q", code, errs)
	}
}

// A sample kept that the runner no longer reads, as one of a file since changed, draw does not draw
// again, and names it; a draw whose sample cannot be saved takes back the crops it made, and keeps
// those that were there before.
func TestDrawKeeps(t *testing.T) {
	changed := filepath.Join(t.TempDir(), "workbook.json")
	if err := os.WriteFile(changed, []byte(readDoc(t, theWorkbook)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dir := copied(t, drawnOf(t, theWorkbook))
	if code, out, errs := runIt("draw", "-workbook", changed, "-dir", dir); code != exitOK || !strings.Contains(out, "Of them the runner refuses, or finds off their file: 20 pages, pages 1, 2,") || !strings.Contains(out, "tutor-eval workbook says why.\n") {
		t.Errorf("draw on samples of a file since changed = %d %q %q", code, out, errs)
	}

	to := copied(t, drawnOf(t, theEssays))
	if err := os.Remove(filepath.Join(to, "essay-03.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(to, "essay-03.json"), 0o700); err != nil {
		t.Fatal(err)
	}
	pics := t.TempDir()
	before := filepath.Join(pics, "essay-03.png")
	if err := os.WriteFile(before, []byte("before"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, errs := runIt("draw", "-essays", theEssays, "-dir", to, "-png", pics); code != exitFail || !strings.Contains(errs, "essay 3 is not saved") {
		t.Errorf("draw where the sample cannot go = %d %q", code, errs)
	}
	if _, err := os.Stat(before); err != nil {
		t.Errorf("a crop that was there before is gone: %v", err)
	}
}
