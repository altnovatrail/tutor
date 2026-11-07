package main

import (
	"bytes"
	"encoding/json"
	"io"
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

// v0File is a sheet as the record mode saved it until T105, a file of tutor-sample v0: every point
// with the pressure given.
func v0File(t *testing.T, s *sample.Sample, pressure float64) []byte {
	t.Helper()
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	for _, st := range doc["strokes"].([]any) {
		for _, p := range st.(map[string]any)["points"].([]any) {
			p.(map[string]any)["pressure"] = pressure
		}
	}
	out, err := sample.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// v0Sheets writes, under root, the synthetic bank and the plan of the synthetic sheets, and a
// directory of their sheets of tutor-sample v0 as the record mode saved them, written by the author
// with a mouse, but for sheet 2, drawn by code from the seed 1, with the split and a file of notes.
// It returns the directory and the flags of the bank and the plan.
func v0Sheets(t *testing.T, root string) (string, []string) {
	t.Helper()
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	plan, samples := synth.Plan()
	for name, v := range map[string]any{"bank.json": synth.Bank(), "plan.json": plan} {
		data, err := sample.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	dir := filepath.Join(root, "sheets")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC)
	for i, s := range samples {
		data := v0File(t, s, 0.5)
		if i == 1 {
			data = v0File(t, synth.SheetLeaning(&plan.Sheets[1], 1, synth.SimilarShift), 0.6)
		}
		if err := os.WriteFile(filepath.Join(dir, sample.SheetFile(&plan.Sheets[i], at)), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for name, data := range map[string]string{sheets.SplitFile: `{"format": "tutor-split", "version": 0, "seed": 1, "dev": [], "test": []}`, "notes.txt": "notes"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir, []string{"-bank", filepath.Join(root, "bank.json"), "-plan", filepath.Join(root, "plan.json")}
}

func convertIt(args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := runConvert(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// The command writes every sheet of tutor-sample v0 as a sample of the protocol under its name, in
// -from and -v1 by default: the author's by a mouse, with the code it makes, and the sheet code drew
// by a pen; it copies the split, reads no other file, leaves the old files alone, and the summary
// takes the new directory as it took the old. A second run writes nothing.
func TestConvert(t *testing.T) {
	root := t.TempDir()
	from, files := v0Sheets(t, root)
	before, err := os.ReadFile(filepath.Join(from, "notes.txt"))
	if err != nil {
		t.Fatal(err)
	}
	code := filepath.Join(root, "writer-code")
	status, out, errs := convertIt(append([]string{"-from", from, "-writer-code", code}, files...)...)
	to := from + "-v1"
	if status != exitOK {
		t.Fatalf("convert = %d, %s %s", status, out, errs)
	}
	for _, want := range []string{
		"Converted: 7 sheets, into " + to + ".\n",
		"Sheet 1: written by the author, by mouse; correct by module-kit's verdict on its truth.\n",
		"Sheet 2: drawn by code, by pen; error by module-kit's verdict on its truth.\n",
		"Not read: 1 other file, no saved sheet of this plan.\n",
		"The split is copied: it names the same files.\n",
		"Not held to the rules by sample-check: " + to + " is out of the work tree",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("convert printed\n%s\nwant it to hold %q", out, want)
		}
	}
	if data, err := os.ReadFile(code); err != nil || len(strings.TrimSpace(string(data))) != 12 {
		t.Errorf("the writer's code = %q, %v; want one made", data, err)
	}
	if after, _ := os.ReadFile(filepath.Join(from, "notes.txt")); !bytes.Equal(after, before) {
		t.Error("the old directory changed")
	}
	if status, summary, errs := runIt(append([]string{"sheets", "-dir", to}, files...)...); status != exitOK ||
		!strings.Contains(summary, "Sheets: 7 of the plan's 7 recorded, 7 of them passing the check.") || !strings.Contains(summary, "Written with a mouse, by the pointer of a stroke: 1, 3, 4, 5, 6, 7.") {
		t.Errorf("sheets of the converted = %d %s %s", status, summary, errs)
	}
	if status, out, _ := convertIt(append([]string{"-from", from, "-writer-code", code}, files...)...); status != exitOK ||
		!strings.Contains(out, "Converted: 0 sheets") || !strings.Contains(out, "Kept as they were, already in "+to+": 7 sheets.") || strings.Contains(out, "split") {
		t.Errorf("a second run = %d %s", status, out)
	}
}

// A sheet the converter cannot write is named with why, the others are written, and the run fails;
// a sheet that sample-check finds breaking a rule is not written either. A check that cannot run
// stops the run, and leaves nothing pending.
func TestConvertRefuses(t *testing.T) {
	// A work tree of its own, whose samples folder the check takes.
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	synthetic := filepath.Join(root, samplesFolder)
	from, files := v0Sheets(t, synthetic)
	plan, samples := synth.Plan()
	wrong := *samples[2]
	wrong.Scenario = sample.ScenarioCorrect
	if err := os.WriteFile(filepath.Join(from, sample.SheetFile(&plan.Sheets[2], time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC))), v0File(t, &wrong, 0.5), 0o600); err != nil {
		t.Fatal(err)
	}
	// A run cut short left a file pending: the next one takes its place.
	if err := os.MkdirAll(from+"-v1", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(from+"-v1", pending+sample.SheetFile(&plan.Sheets[3], time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC))), []byte("half"), 0o600); err != nil {
		t.Fatal(err)
	}
	var asked []string
	checking(t, func(paths []string) (map[string][]int, error) {
		asked = paths
		return map[string][]int{paths[0]: {12}}, nil
	})
	status, out, errs := convertIt(append([]string{"-from", from, "-writer-code", filepath.Join(synthetic, "code")}, files...)...)
	if status != exitFail || len(asked) != 6 || !strings.HasPrefix(filepath.Base(asked[0]), pending) {
		t.Errorf("convert = %d, asked %q; %s %s", status, asked, out, errs)
	}
	for _, want := range []string{
		"Converted: 5 sheets",
		"Refused, and not written: sheet 3, ",
		"its scenario is correct, and its plan sheet's similar-digits.",
		"Refused, and not written: sheet 1, ",
		"sample-check finds its sample breaks a rule of sections 11 and 12, at lines [12].",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("convert printed\n%s\nwant it to hold %q", out, want)
		}
	}
	entries, err := os.ReadDir(from + "-v1")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), pending) {
			t.Errorf("%s is left pending", e.Name())
		}
	}
	if len(entries) != 6 {
		t.Errorf("the new directory holds %d files, want the five sheets and the split", len(entries))
	}
	checking(t, func([]string) (map[string][]int, error) { return nil, os.ErrPermission })
	if status, _, errs := convertIt(append([]string{"-from", from, "-to", filepath.Join(synthetic, "again"), "-writer-code", filepath.Join(synthetic, "code")}, files...)...); status != exitFail || !strings.Contains(errs, "permission") {
		t.Errorf("convert with a check that cannot run = %d %s", status, errs)
	}
	if entries, _ := os.ReadDir(filepath.Join(synthetic, "again")); len(entries) != 0 {
		t.Errorf("a run stopped by the check left %d files", len(entries))
	}

	// Into synthetic goes no sheet a person wrote, wherever it comes from, and no file of their code.
	checking(t, func([]string) (map[string][]int, error) { return map[string][]int{}, nil })
	outside, _ := v0Sheets(t, filepath.Join(t.TempDir(), "outside"))
	status, out, errs = convertIt(append([]string{"-from", outside, "-to", filepath.Join(root, syntheticFolder, "sheets")}, files...)...)
	if status != exitFail || !strings.Contains(out, "Converted: 1 sheet,") || strings.Count(out, "nothing of a person's goes into synthetic") != 6 {
		t.Errorf("convert of a person's sheets into synthetic = %d %s %s", status, out, errs)
	}
	if status, _, errs := convertIt(append([]string{"-from", outside, "-to", filepath.Join(root, syntheticFolder, "more"), "-writer-code", filepath.Join(root, syntheticFolder, "code")}, files...)...); status != exitUsage || !strings.Contains(errs, "-writer-code") {
		t.Errorf("a code in synthetic = %d %q", status, errs)
	}
}

// The directories a conversion may not take.
func TestConvertDirs(t *testing.T) {
	root := t.TempDir()
	from, files := v0Sheets(t, root)
	// The tests run in cmd/tutor-eval, where samples/writer-code is not in the samples folder at the
	// root: each run but the one about it gets a file of the code out of the work tree.
	files = append(files, "-writer-code", filepath.Join(root, "code"))
	for _, c := range []struct {
		name string
		args []string
		want string
	}{
		{"no -from", nil, "-from is empty"},
		{"-to in -from", []string{"-from", from, "-to", filepath.Join(from, "v1")}, "one is in the other"},
		{"the same directory", []string{"-from", from, "-to", from}, "one is in the other"},
		{"the author's out of samples", []string{"-from", "../../samples/sheets", "-to", filepath.Join(root, "out")}, "nothing of the author's goes elsewhere"},
		{"a directory in the work tree outside samples and synthetic", []string{"-from", from, "-to", "out"}, "outside its samples and synthetic folders"},
		{"a code in the work tree outside samples", []string{"-from", from, "-writer-code", "code"}, "-writer-code: "},
	} {
		if status, _, errs := convertIt(append(slices.Clone(files), c.args...)...); status != exitUsage || !strings.Contains(errs, c.want) {
			t.Errorf("%s: %d %q, want it refused for %q", c.name, status, errs, c.want)
		}
	}
	if status, _, errs := convertIt("-from", from, "-to", filepath.Join(root, "out"), "-bank", filepath.Join(root, "nowhere.json"), "-writer-code", filepath.Join(root, "code")); status != exitFail || !strings.Contains(errs, "nowhere.json") {
		t.Errorf("convert without a bank = %d %q", status, errs)
	}
	if status, _, errs := convertIt("-from", filepath.Join(root, "nowhere"), "-to", filepath.Join(root, "out"), "-bank", files[1], "-plan", files[3], "-writer-code", filepath.Join(root, "code")); status != exitFail || !strings.Contains(errs, "nowhere") {
		t.Errorf("convert of no directory = %d %q", status, errs)
	}
	broken := filepath.Join(root, "broken-code")
	if err := os.WriteFile(broken, []byte("abc\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if status, out, _ := convertIt("-from", from, "-to", filepath.Join(root, "out"), "-bank", files[1], "-plan", files[3], "-writer-code", broken); status != exitFail || !strings.Contains(out, "holds no writer's code") || !strings.Contains(out, "Converted: 1 sheet,") {
		t.Errorf("convert with a broken file of the code = %d %s, want the sheet of code alone written", status, out)
	}
	if status := runConvert(append([]string{"-from", from, "-to", filepath.Join(root, "out2")}, files...), failingWriter{}, io.Discard); status != exitFail {
		t.Errorf("convert with an unwritable stdout = %d", status)
	}
	if err := os.WriteFile(filepath.Join(root, "file"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if status, _, errs := convertIt(append([]string{"-from", from, "-to", filepath.Join(root, "file", "out")}, files...)...); status != exitFail || !strings.Contains(errs, "not a directory") {
		t.Errorf("convert into a directory that cannot be made = %d %q", status, errs)
	}
	if status, _, _ := convertIt("-nope"); status != exitUsage {
		t.Errorf("an unknown flag = %d", status)
	}
}

// The file of the writer's code by default: the author's for a directory in samples, none for one in
// synthetic, where no person's sheet goes, and beside the directory otherwise, as for a test.
func TestWriterCodeOf(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "sheets")
	for _, c := range []struct {
		dir, file string
		given     bool
		want      string
	}{
		{"../../samples/sheets-v1", writerCodeFile, false, writerCodeFile},
		{"../../synthetic/free-v1", writerCodeFile, false, ""},
		{tmp, writerCodeFile, false, filepath.Join(filepath.Dir(tmp), "writer-code")},
		{tmp, "mine", true, "mine"},
	} {
		if got := writerCodeOf(c.dir, c.file, c.given); got != c.want {
			t.Errorf("writerCodeOf(%s) = %q, want %q", c.dir, got, c.want)
		}
	}
}
