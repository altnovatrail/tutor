package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"
	observationv1 "github.com/altnovatrail/spec/contracts/novatrail/observation/v1"

	"github.com/altnovatrail/tutor/internal/evalsample"
	"github.com/altnovatrail/tutor/internal/protocol"
)

// canvasTag is the tag of tutor's tree the tests of the canvas check write.
const canvasTag = "v0.0.1"

// canvasSet writes a set as canvas's synthetic samples are, of tasks of a release of tutor, in a
// directory of its own: drawn samples of internal/evalsample's testdata — sheets of lines and of free
// writing, and a page of the workbook — named by the release at canvasTag, with a sheet and the page
// of the final geometry of the eval set, a child's sheet, and a sheet with no stroke; and tutor's
// tree of eval/ at the tag. change, when given, changes a sample by its file's name before it is
// written. It gives the directory of the samples and that of the trees.
func canvasSet(t *testing.T, change func(name string, es *evalv1.Sample)) (dir, trees string) {
	t.Helper()
	root := t.TempDir()
	trees = filepath.Join(root, "trees")
	at := filepath.Join(trees, canvasTag, "eval")
	if err := os.MkdirAll(at, 0o700); err != nil {
		t.Fatal(err)
	}
	digests := map[string]string{}
	for _, f := range []string{"bank.json", "free-bank.json", "workbook.json", "essays.json", "open-tasks.json"} {
		data, err := os.ReadFile(filepath.Join("../../eval", f))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(at, f), data, 0o600); err != nil {
			t.Fatal(err)
		}
		digests["eval/"+f] = evalsample.Digest(data)
	}
	dir = filepath.Join(root, "canvas-test")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	read := func(name string) *evalv1.Sample {
		t.Helper()
		data, err := os.ReadFile(filepath.Join("../../internal/evalsample/testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		es, err := protocol.ReadSample(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		bank := es.GetTask().GetFile().GetBank()
		es.Task.Source = &evalv1.TaskReference_Tutor{Tutor: &evalv1.TutorSource{Tag: canvasTag, FileDigest: digests[evalsample.OpenTasks], Bank: bank, BankDigest: digests[bank]}}
		return es
	}
	final := func(es *evalv1.Sample, writer evalv1.Writer) *evalv1.Sample {
		es.Ink = &evalv1.Sample_Geometry{Geometry: &evalv1.Geometry{Strokes: protocol.FinalOf(protocol.Fold(es.GetHistory()))}}
		es.Purpose, es.Writer, es.WriterCode, es.License = evalv1.Purpose_PURPOSE_EVAL_SET, writer, nil, new("Apache-2.0")
		return es
	}
	blank := read("lines-01-correct.json")
	blank.Id = "synthetic-blank"
	blank.Ink = &evalv1.Sample_History{History: &evalv1.History{}}
	blank.GetSheet().Groups, blank.GetSheet().Crops = nil, nil
	blank.Markup.Writing = &evalv1.Markup_Parts{Parts: &evalv1.WritingParts{}}
	blank.Markup.Truth.AnswerSlots[0].Read = &observationv1.AnswerSlotObservation_Unreadable{Unreadable: observationv1.Unreadable_UNREADABLE_BLANK}
	child := final(read("lines-03-error.json"), evalv1.Writer_WRITER_CHILD)
	child.Id = "synthetic-child"
	set := map[string]*evalv1.Sample{
		"a-lines.json": read("lines-01-correct.json"),
		"b-error.json": final(read("lines-03-error.json"), evalv1.Writer_WRITER_ADULT_AS_CHILD),
		"c-free.json":  read("free-03-erasure-fixed.json"),
		"d-page.json":  final(read("workbook-05.json"), evalv1.Writer_WRITER_ADULT),
		"e-child.json": child,
		"f-blank.json": blank,
		"notes.txt":    nil,
	}
	for name, es := range set {
		if es == nil {
			if err := os.WriteFile(filepath.Join(dir, name), []byte("not a sample"), 0o600); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if change != nil {
			change(name, es)
		}
		data, err := protocol.MarshalSample(es)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir, trees
}

// canvasRun runs tutor-eval canvas on a set, its logs in a directory beside it.
func canvasRun(dir, trees string, more ...string) (int, string, string) {
	return runIt(append([]string{"canvas", "-dir", dir, "-at", trees, "-runs", filepath.Join(filepath.Dir(dir), "runs")}, more...)...)
}

// The check reads every sample of canvas's by the task it names at its tag of tutor — sheets of lines
// and of free writing, a page of the workbook, of a history or of the final geometry — refuses the
// child's alone, runs the sheets and the page on the stub, and joins their reports into one of the
// set for each reader and pair, which measure every sample read, the blank sheet with no answer
// slot, and refuse the child's.
func TestCanvas(t *testing.T) {
	dir, trees := canvasSet(t, nil)
	code, out, errs := canvasRun(dir, trees)
	if code != exitOK {
		t.Fatalf("canvas = %d\n%s%s", code, out, errs)
	}
	for _, want := range []string{
		"a-lines.json: synthetic-1-sheets-01, of the task ",
		"in eval/bank.json of tutor v0.0.1: read.",
		"d-page.json: synthetic-1-workbook-05, of the task workbook-05 in eval/workbook.json of tutor v0.0.1: read.",
		"e-child.json: synthetic-child, of the task ",
		"refused as a child's: a child wrote it",
		"f-blank.json: synthetic-blank, of the task ",
		"Samples: 6; read 4 sheets and 1 page; refused 1; not read or refused for more than a child's, 0.",
		"The runner's reports of the whole set, the sheets and the pages in one: ",
		"The canvas check passes: ",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the output has no %q:\n%s", want, out)
		}
	}
	runs := filepath.Join(filepath.Dir(dir), "runs")
	joined, _ := filepath.Glob(filepath.Join(runs, "*-canvas-test.*.report.json"))
	logs, _ := filepath.Glob(filepath.Join(runs, "*.jsonl"))
	if len(joined) != 5 || len(logs) != 2 {
		t.Fatalf("%d reports of the set and %d logs: %v %v", len(joined), len(logs), joined, logs)
	}
	for _, l := range logs {
		head, err := os.ReadFile(l)
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(l, "-workbook.jsonl") && (!strings.Contains(string(head), filepath.Join("trees", canvasTag, "eval", "workbook.json")) || !strings.Contains(string(head), `"set":"canvas-test-workbook"`)) {
			t.Errorf("the log of the pages names its file and set so: %.300s", head)
		}
	}
	data, err := os.ReadFile(joined[0])
	if err != nil {
		t.Fatal(err)
	}
	rep, err := protocol.ReadReport(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]*evalv1.SampleResult{}
	for _, s := range rep.GetSamples() {
		got[s.GetSample()] = s
	}
	if rep.GetCapabilitySet().GetName() != "canvas-test" || len(got) != 6 || got["synthetic-child"].GetRefused() != evalv1.Refusal_REFUSAL_CHILD ||
		got["synthetic-blank"].GetMeasured() == nil || len(got["synthetic-blank"].GetMeasured().GetAnswerSlots()) != 0 ||
		len(got["synthetic-1-workbook-05"].GetMeasured().GetAnswerSlots()) == 0 {
		t.Errorf("the report of the set: %v", rep)
	}
}

// The check fails on a sample it cannot read, or refuses for more than a child's, and names it; a tree
// of tutor that is not there, or a check that cannot run, stops it.
func TestCanvasFails(t *testing.T) {
	zeros := "sha256:" + strings.Repeat("0", 64)
	only := func(file string, change func(es *evalv1.Sample)) func(string, *evalv1.Sample) {
		return func(name string, es *evalv1.Sample) {
			if name == file {
				change(es)
			}
		}
	}
	for name, c := range map[string]struct {
		change func(string, *evalv1.Sample)
		want   string
	}{
		"a bank of another digest": {only("a-lines.json", func(es *evalv1.Sample) { es.GetTask().GetTutor().BankDigest = zeros }),
			"a-lines.json: synthetic-1-sheets-01, of the task posts in eval/bank.json of tutor v0.0.1, refused as another task: its file of tasks is of another digest"},
		"a file of open tasks of another digest": {only("a-lines.json", func(es *evalv1.Sample) { es.GetTask().GetTutor().FileDigest = zeros }),
			"refused as another task: its file of open tasks is of another digest than tutor v0.0.1's"},
		"no such task": {only("a-lines.json", func(es *evalv1.Sample) { es.Task.Task = "no-such-task" }),
			"refused as another task: the bank eval/bank.json has no task no-such-task"},
		"an open view of another digest": {only("d-page.json", func(es *evalv1.Sample) { es.Task.OpenViewDigest = zeros }),
			"refused as another task: the open view of its task workbook-05 is of another digest than the workbook's"},
		"no markup": {only("c-free.json", func(es *evalv1.Sample) { es.Markup = nil }),
			"refused as no markup"},
		"a truth of no answer slot": {only("a-lines.json", func(es *evalv1.Sample) { es.Markup.Truth.AnswerSlots = nil }),
			"refused as invalid: its truth is no valid observation"},
		"a file of tasks": {only("a-lines.json", func(es *evalv1.Sample) {
			es.Task.Source = &evalv1.TaskReference_File{File: &evalv1.TaskFileSource{Bank: "eval/bank.json"}}
		}), "refused as another task: its task is of no release of tutor"},
		"a path out of the tree": {only("a-lines.json", func(es *evalv1.Sample) { es.GetTask().GetTutor().Bank = "../x.json" }),
			"which is no file of tutor's tree"},
		"the essays": {only("a-lines.json", func(es *evalv1.Sample) { es.GetTask().GetTutor().Bank = "eval/essays.json" }),
			"not read: evalsample: the file of its task is neither a bank of sheets nor the workbook"},
		"a page the workbook's run cannot stand for": {only("d-page.json", func(es *evalv1.Sample) {
			es.Markup.Truth.AnswerSlots[0].Read = &observationv1.AnswerSlotObservation_Unreadable{Unreadable: observationv1.Unreadable_UNREADABLE_ILLEGIBLE}
		}), "d-page.json: synthetic-1-workbook-05, of the task workbook-05 in eval/workbook.json of tutor v0.0.1, not read: evalsample: the run of the workbook cannot stand for the markup of the page: the box 1.1"},
		"a page on a sheet": {only("d-page.json", func(es *evalv1.Sample) {
			es.Surface = &evalv1.Sample_Sheet{Sheet: &evalv1.Sheet{Width: 1000, Height: 600}}
		}), "d-page.json: synthetic-1-workbook-05, of the task workbook-05 in eval/workbook.json of tutor v0.0.1, not read: evalsample: the sample is no page of a workbook"},
		"a sheet on a page": {only("a-lines.json", func(es *evalv1.Sample) {
			es.Surface = &evalv1.Sample_Workbook{Workbook: &evalv1.Workbook{}}
		}), "a-lines.json: synthetic-1-sheets-01, of the task posts in eval/bank.json of tutor v0.0.1, not read: evalsample: the sample is no sheet"},
	} {
		t.Run(name, func(t *testing.T) {
			dir, trees := canvasSet(t, c.change)
			code, out, errs := canvasRun(dir, trees)
			if code != exitFail || !strings.Contains(out, c.want) || !strings.Contains(out, "The canvas check fails: 1 sample of ") {
				t.Errorf("canvas = %d, want %q:\n%s%s", code, c.want, out, errs)
			}
		})
	}

	t.Run("a sheet sample-check refuses, and one not strict", func(t *testing.T) {
		dir, trees := canvasSet(t, nil)
		if err := os.WriteFile(filepath.Join(dir, "g-loose.json"), []byte(`{"id": "synthetic-loose"}`), 0o600); err != nil {
			t.Fatal(err)
		}
		checking(t, func(paths []string) (map[string][]int, error) {
			return map[string][]int{filepath.Join(dir, "c-free.json"): {3}}, nil
		})
		code, out, errs := canvasRun(dir, trees)
		if code != exitFail || !strings.Contains(out, "c-free.json: a sample, refused as invalid: sample-check finds it breaks a rule of sections 11 and 12, at line 3.") ||
			!strings.Contains(out, "g-loose.json: a sample, refused as invalid: it cannot be read strictly as a sample of the protocol.") ||
			!strings.Contains(out, "The canvas check fails: 2 samples of ") {
			t.Errorf("canvas = %d\n%s%s", code, out, errs)
		}
	})
	t.Run("a page twice", func(t *testing.T) {
		dir, trees := canvasSet(t, nil)
		data, err := os.ReadFile(filepath.Join(dir, "d-page.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "d2-page.json"), bytes.Replace(data, []byte(`"synthetic-1-workbook-05"`), []byte(`"synthetic-again"`), 1), 0o600); err != nil {
			t.Fatal(err)
		}
		code, out, errs := canvasRun(dir, trees)
		if code != exitFail || !strings.Contains(out, "d2-page.json: synthetic-again, of the task workbook-05 in eval/workbook.json of tutor v0.0.1, not read: its page 5 is d-page.json's too") {
			t.Errorf("canvas = %d\n%s%s", code, out, errs)
		}
	})
	t.Run("only refused samples", func(t *testing.T) {
		dir, trees := canvasSet(t, func(_ string, es *evalv1.Sample) { es.Writer = evalv1.Writer_WRITER_CHILD })
		code, out, errs := canvasRun(dir, trees)
		if code != exitFail || !strings.Contains(errs, "no sample of the directory is read") {
			t.Errorf("canvas = %d\n%s%s", code, out, errs)
		}
	})
	t.Run("a tag with no tree", func(t *testing.T) {
		dir, trees := canvasSet(t, only("a-lines.json", func(es *evalv1.Sample) { es.GetTask().GetTutor().Tag = "v0.0.9" }))
		code, out, errs := canvasRun(dir, trees)
		if code != exitFail || !strings.Contains(errs, "tutor's tree at v0.0.9 is not in") {
			t.Errorf("canvas = %d\n%s%s", code, out, errs)
		}
	})
	t.Run("a check that cannot run", func(t *testing.T) {
		dir, trees := canvasSet(t, nil)
		checking(t, func([]string) (map[string][]int, error) { return nil, errors.New("no buf") })
		if code, out, errs := canvasRun(dir, trees); code != exitFail || !strings.Contains(errs, "no buf") {
			t.Errorf("canvas = %d\n%s%s", code, out, errs)
		}
	})
	t.Run("no sample", func(t *testing.T) {
		dir := t.TempDir()
		if code, out, errs := canvasRun(dir, dir); code != exitFail || !strings.Contains(errs, "holds no sample") {
			t.Errorf("canvas = %d\n%s%s", code, out, errs)
		}
	})
}

// The bank a sample names is the bank at its tag, not the work tree's: a bank of the tree that the
// work tree no longer has still reads the samples made on it.
func TestCanvasBankAtTheTag(t *testing.T) {
	dir, trees := canvasSet(t, nil)
	tree := filepath.Join(trees, canvasTag, "eval", "bank.json")
	data, err := os.ReadFile(tree)
	if err != nil {
		t.Fatal(err)
	}
	changed := append(bytes.TrimRight(data, "\n"), []byte("\n\n")...)
	if err := os.WriteFile(tree, changed, 0o600); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		old, _ := os.ReadFile(p)
		if err := os.WriteFile(p, bytes.ReplaceAll(old, []byte(evalsample.Digest(data)), []byte(evalsample.Digest(changed))), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if code, out, errs := canvasRun(dir, trees); code != exitOK {
		t.Errorf("canvas on a bank of the tag = %d\n%s%s", code, out, errs)
	}
}

// The check takes a directory and the trees, and keeps to synthetic in the work tree.
func TestCanvasFlags(t *testing.T) {
	for _, args := range [][]string{{"canvas"}, {"canvas", "-dir", "x"}, {"canvas", "-at", "x"}, {"canvas", "-dir", "x", "-at", "y", "-repeats", "0"}} {
		if code, _, errs := runIt(args...); code != exitUsage {
			t.Errorf("%v = %d %s", args, code, errs)
		}
	}
	if code, _, errs := runIt("canvas", "-dir", ".", "-at", "x"); code != exitUsage || !strings.Contains(errs, "outside its synthetic folder") {
		t.Errorf("a directory in the work tree = %d %s", code, errs)
	}
	if code, _, errs := runIt("canvas", "-dir", t.TempDir(), "-at", "x", "-runs", "."); code != exitUsage || !strings.Contains(errs, "outside its synthetic folder") {
		t.Errorf("runs in the work tree = %d %s", code, errs)
	}
	if code, _, errs := runIt("canvas", "-dir", t.TempDir(), "-at", "x", "-runs", t.TempDir(), "-name", "set*"); code != exitUsage || !strings.Contains(errs, "give -name of letters") {
		t.Errorf("a name of a pattern = %d %s", code, errs)
	}
}

// A run that cannot write its log, or that asks nothing — a set whose one page has no box filled —
// stops the check.
func TestCanvasRunFails(t *testing.T) {
	dir, trees := canvasSet(t, nil)
	blocked := filepath.Join(filepath.Dir(dir), "file")
	if err := os.WriteFile(blocked, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if code, out, errs := runIt("canvas", "-dir", dir, "-at", trees, "-runs", filepath.Join(blocked, "runs")); code != exitFail || !strings.Contains(errs, "tutor-eval canvas: ") {
		t.Errorf("runs under a file = %d\n%s%s", code, out, errs)
	}
	dir, trees = canvasSet(t, func(name string, es *evalv1.Sample) {
		if name != "d-page.json" {
			return
		}
		es.Ink = &evalv1.Sample_Geometry{Geometry: &evalv1.Geometry{}}
		es.GetWorkbook().FilledBoxes = nil
		es.Markup.Notes = nil
		for _, a := range es.Markup.Truth.AnswerSlots {
			a.Read = &observationv1.AnswerSlotObservation_Unreadable{Unreadable: observationv1.Unreadable_UNREADABLE_BLANK}
		}
	})
	if code, out, errs := canvasRun(dir, trees); code != exitFail || !strings.Contains(errs, "the run asks nothing") {
		t.Errorf("a blank page alone = %d\n%s%s", code, out, errs)
	}
}

// The reports of the set must measure each sample read and refuse each refused for its reason, with
// no reading unchecked; and the runs must have written reports to join.
func TestCheckReports(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, samples ...*evalv1.SampleResult) string {
		t.Helper()
		data, err := protocol.MarshalReport(&evalv1.Report{Protocol: protocol.Protocol, Runner: "v0.1.10", Capability: evalv1.Capability_CAPABILITY_HANDWRITING_READING,
			Readers: []*evalv1.Model{{Provider: "anthropic", Model: "m"}}, Verdicts: &evalv1.VerdictSource{Code: evalv1.VerdictCode_VERDICT_CODE_MODULE_KIT, Version: "v0.2.3"},
			EvalSet: &evalv1.Report_CapabilitySet{CapabilitySet: &evalv1.CapabilityEvalSet{Name: "x"}}, Repeats: 1, Samples: samples})
		if err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, data, 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	measured := func(id string, readings ...*evalv1.Reading) *evalv1.SampleResult {
		return &evalv1.SampleResult{Sample: id, Result: &evalv1.SampleResult_Measured{Measured: &evalv1.Measurement{AnswerSlots: []*evalv1.AnswerSlotResult{{Readings: readings}}}}}
	}
	refused := &evalv1.SampleResult{Sample: "s-2", Result: &evalv1.SampleResult_Refused{Refused: evalv1.Refusal_REFUSAL_CHILD}}
	read := []canvasSample{{file: "a", id: "s-1"}, {file: "b", id: "s-2", refused: &evalsample.Refusal{Reason: evalv1.Refusal_REFUSAL_CHILD}}, {file: "c", problem: "no"}}
	ok := &evalv1.Reading{Verdict: &evalv1.Verdict{Verdict: "correct"}}
	if err := checkReports([]string{write("ok.json", measured("s-1", ok), refused), write("ok2.json", refused, measured("s-1", ok))}, read); err != nil {
		t.Errorf("reports that hold: %v", err)
	}
	if err := checkReports([]string{write("ok3.json", measured("s-1", ok), refused), write("pages.json", refused)}, read); err == nil || !strings.Contains(err.Error(), "pages.json does not name the sample s-1") {
		t.Errorf("a report of the set without a sample another has: %v", err)
	}
	for name, c := range map[string]struct {
		samples []*evalv1.SampleResult
		want    string
	}{
		"an unchecked reading":               {[]*evalv1.SampleResult{measured("s-1", &evalv1.Reading{Unchecked: evalv1.UncheckedCause_UNCHECKED_CAUSE_FAILURE}), refused}, "is unchecked"},
		"a sample in none":                   {[]*evalv1.SampleResult{refused}, "does not name the sample s-1 of a"},
		"a sample measured that was refused": {[]*evalv1.SampleResult{measured("s-1", ok), measured("s-2", ok)}, "does not refuse the sample s-2"},
		"a sample read and refused":          {[]*evalv1.SampleResult{{Sample: "s-1", Result: &evalv1.SampleResult_Refused{Refused: evalv1.Refusal_REFUSAL_INVALID}}, refused}, "does not measure the sample s-1"},
	} {
		if err := checkReports([]string{write(strings.ReplaceAll(name, " ", "-")+".json", c.samples...)}, read); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want %q", name, err, c.want)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := checkReports([]string{filepath.Join(dir, "bad.json")}, read); err == nil {
		t.Error("a report that does not parse")
	}
	if err := checkReports([]string{filepath.Join(dir, "none.json")}, read); err == nil {
		t.Error("a report that is not there")
	}
	if _, err := joinReports(filepath.Join(dir, "set"), "x", []string{filepath.Join(dir, "log.jsonl")}); err == nil || !strings.Contains(err.Error(), "no runner's report") {
		t.Errorf("logs with no report: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "log.r.report.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := joinReports(filepath.Join(dir, "set"), "x", []string{filepath.Join(dir, "log.jsonl")}); err == nil {
		t.Error("a report to join that does not parse")
	}
}
