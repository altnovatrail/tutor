package evalsample_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"
	observationv1 "github.com/altnovatrail/spec/contracts/novatrail/observation/v1"
	"google.golang.org/protobuf/proto"

	"github.com/altnovatrail/tutor/internal/evalsample"
	"github.com/altnovatrail/tutor/internal/protocol"
	"github.com/altnovatrail/tutor/internal/sample"
)

// tag is the tag of tutor's tree the tests of a release write.
const tag = "v0.0.1"

// tree writes tutor's tree of eval/ at the tag, as git archive does, from the files of eval/: the
// banks, the workbook, the essays and the file of open tasks. It gives the folder of the trees.
func tree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	at := filepath.Join(dir, tag, "eval")
	if err := os.MkdirAll(at, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"bank.json", "free-bank.json", "workbook.json", "essays.json", "open-tasks.json"} {
		data, err := os.ReadFile(filepath.Join("../../eval", f))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(at, f), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// released names the sample's task by the release of tutor at the tag, as canvas's samples do: the
// bank of its task reference, with the digests the tree gives.
func released(t *testing.T, es *evalv1.Sample, trees string) {
	t.Helper()
	bank := es.GetTask().GetFile().GetBank()
	open, err := os.ReadFile(filepath.Join(trees, tag, evalsample.OpenTasks))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(trees, tag, bank))
	if err != nil {
		t.Fatal(err)
	}
	es.Task.Source = &evalv1.TaskReference_Tutor{Tutor: &evalv1.TutorSource{Tag: tag, FileDigest: evalsample.Digest(open), Bank: bank, BankDigest: evalsample.Digest(data)}}
}

// ref is a task reference of a release of tutor, by its tag and bank alone.
func ref(tg, bank string) *evalv1.TaskReference {
	return &evalv1.TaskReference{Source: &evalv1.TaskReference_Tutor{Tutor: &evalv1.TutorSource{Tag: tg, Bank: bank}}, Task: "x"}
}

// A sample of a release of tutor names its file of tasks at its tag, in the tree of that tag: a bank
// or the workbook, read once, with the tag and the digest of the tree's file of open tasks. A
// reference to no release, a tag of no version, a path out of the tree or of no file is another
// task; the essays are no file the check reads; a tree that is not there, and a file of the tree
// that does not parse, are errors.
func TestReleases(t *testing.T) {
	trees := tree(t)
	rs := evalsample.NewReleases(trees)
	b, w, r, err := rs.Of(ref(tag, "eval/bank.json"))
	if err != nil || r != nil || w != nil || b == nil {
		t.Fatalf("the bank: %v %v %v %v", b, w, r, err)
	}
	open, _ := os.ReadFile(filepath.Join(trees, tag, evalsample.OpenTasks))
	if b.Path != "eval/bank.json" || b.Tag != tag || b.Open != evalsample.Digest(open) || len(b.Bank.Tasks) == 0 {
		t.Errorf("the bank: %s %s %s, %d tasks", b.Path, b.Tag, b.Open, len(b.Bank.Tasks))
	}
	if again, _, _, _ := rs.Of(ref(tag, "eval/bank.json")); again != b {
		t.Error("the bank is read again")
	}
	if _, w, r, err := rs.Of(ref(tag, "eval/workbook.json")); err != nil || r != nil || w == nil || w.Path != "eval/workbook.json" || w.Tag != tag {
		t.Errorf("the workbook: %v %v %v", w, r, err)
	}
	if _, _, _, err := rs.Of(ref(tag, "eval/essays.json")); !errors.Is(err, evalsample.ErrOtherFile) {
		t.Errorf("the essays: %v, want ErrOtherFile", err)
	}
	if _, _, _, err := rs.Of(ref(tag, "eval/essays.json")); !errors.Is(err, evalsample.ErrOtherFile) {
		t.Errorf("the essays again: %v, want ErrOtherFile", err)
	}
	file := &evalv1.TaskReference{Source: &evalv1.TaskReference_File{File: &evalv1.TaskFileSource{Bank: "eval/bank.json"}}}
	for name, c := range map[string]struct {
		ref  *evalv1.TaskReference
		want string
	}{
		"a file of tasks":        {file, "of no release of tutor"},
		"a tag of no version":    {ref("main", "eval/bank.json"), "which is no release of tutor"},
		"a path out of the tree": {ref(tag, "../bank.json"), "which is no file of tutor's tree"},
		"the tree itself":        {ref(tag, "."), "which is no file of tutor's tree"},
		"no such file":           {ref(tag, "eval/none.json"), "tutor v0.0.1 has no file eval/none.json"},
	} {
		if _, _, r, err := rs.Of(c.ref); err != nil || r == nil || r.Reason != evalv1.Refusal_REFUSAL_OTHER_TASK || !strings.Contains(r.Detail, c.want) {
			t.Errorf("%s: %v %v, want %q", name, r, err, c.want)
		}
	}
	if _, _, _, err := rs.Of(ref("v0.0.9", "eval/bank.json")); err == nil || !strings.Contains(err.Error(), "just canvas-fetch") {
		t.Errorf("a tag with no tree: %v", err)
	}
	for name, data := range map[string]string{"not JSON": "{", "a bank that does not parse": `{"format": "tutor-bank"}`} {
		if err := os.WriteFile(filepath.Join(trees, tag, "eval", "bad.json"), []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, _, _, err := evalsample.NewReleases(trees).Of(ref(tag, "eval/bad.json")); err == nil || errors.Is(err, evalsample.ErrOtherFile) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if err := os.Symlink(filepath.Join(trees, tag, "eval", "bank.json"), filepath.Join(trees, "outside.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(trees, "outside.json"), filepath.Join(trees, tag, "eval", "link.json")); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := evalsample.NewReleases(trees).Of(ref(tag, "eval/link.json")); err == nil {
		t.Error("a link out of the tree is followed")
	}
}

// A sample of a release is of its file at its tag: another tag, bank, file of open tasks or bank
// digest, or a source of no release, is another task.
func TestReleaseRefusals(t *testing.T) {
	trees := tree(t)
	b, p := read(t, lines)
	sh := &p.Sheets[0]
	data := file(t, b, sh, 1)
	rb, _, _, err := evalsample.NewReleases(trees).Of(ref(tag, "eval/bank.json"))
	if err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]struct {
		change func(src *evalv1.TutorSource)
		want   string
	}{
		"another tag":                {func(src *evalv1.TutorSource) { src.Tag = "v0.0.2" }, "of tutor v0.0.2"},
		"another bank":               {func(src *evalv1.TutorSource) { src.Bank = "eval/free-bank.json" }, "of the file eval/free-bank.json"},
		"a file of open tasks other": {func(src *evalv1.TutorSource) { src.FileDigest = "sha256:" + strings.Repeat("0", 64) }, "file of open tasks is of another digest"},
		"a bank of another digest":   {func(src *evalv1.TutorSource) { src.BankDigest = "sha256:" + strings.Repeat("0", 64) }, "file of tasks is of another digest"},
	} {
		es := sampleOf(t, data)
		released(t, es, trees)
		c.change(es.GetTask().GetTutor())
		if _, _, r, err := evalsample.ReadSheet(es, rb); err != nil || r == nil || r.Reason != evalv1.Refusal_REFUSAL_OTHER_TASK || !strings.Contains(r.Detail, c.want) {
			t.Errorf("%s: %v %v, want %q", name, r, err, c.want)
		}
	}
	es := sampleOf(t, data)
	if _, _, r, err := evalsample.ReadSheet(es, rb); err != nil || r == nil || !strings.Contains(r.Detail, "of no release of tutor") {
		t.Errorf("a file of tasks against a release: %v %v", r, err)
	}
	// A task of a package of modules is not of the release either (Д164 RUN.md spec; R90).
	es.Task.Source = &evalv1.TaskReference_Modules{Modules: &evalv1.ModuleSource{Tag: "v0.1.0", Module: "grade-3-math", Version: "1.0.0"}}
	if _, _, r, err := evalsample.ReadSheet(es, rb); err != nil || r == nil || !strings.Contains(r.Detail, "of no release of tutor") {
		t.Errorf("a task of modules against a release: %v %v", r, err)
	}
}

// ReadSheet reads a sheet that no plan sheet names: its plan sheet is its markup's, of no scenario,
// its parts and answer the markup's, the outcome module-kit's verdict and the trap its verdict's,
// none where the value is no trap value of the check;
// it reads a final geometry as the strokes it shows; and a sheet that shows no stroke has no answer
// slot a model reads.
func TestReadSheet(t *testing.T) {
	trees := tree(t)
	rs := evalsample.NewReleases(trees)
	b, p := read(t, lines)
	rb, _, _, err := rs.Of(ref(tag, "eval/bank.json"))
	if err != nil {
		t.Fatal(err)
	}
	readOf := func(es *evalv1.Sample, bank *evalsample.Bank) (*evalsample.Sheet, *sample.PlanSheet) {
		t.Helper()
		sh, plan, r, err := evalsample.ReadSheet(es, bank)
		if err != nil || r != nil {
			t.Fatalf("%v %v", r, err)
		}
		return sh, plan
	}
	for _, n := range []int{1, 3} {
		want := &p.Sheets[n-1]
		es := sampleOf(t, file(t, b, want, 1))
		released(t, es, trees)
		sh, plan := readOf(es, rb)
		if plan.Task != want.Task || plan.Scenario != "" || plan.Erasure != nil || plan.Answer != want.Answer || len(plan.Parts) != len(want.Lines) ||
			plan.Expected.Outcome != want.Expected.Outcome || plan.Expected.Trap != sh.Truth[0].Trap || (plan.Expected.FirstError == nil) != (want.Expected.FirstError == nil) {
			t.Errorf("sheet %d: the plan sheet %+v, want of %+v", n, plan, want)
		}
		if len(sh.Truth) != 1 || sh.Sample.Expected.Trap != plan.Expected.Trap || len(sh.Sample.Strokes) == 0 {
			t.Errorf("sheet %d: %d truths, trap %q, %d strokes", n, len(sh.Truth), sh.Sample.Expected.Trap, len(sh.Sample.Strokes))
		}
		for i, l := range plan.Lines {
			if l != plan.Parts[i].Text || plan.Parts[i].Kind != sample.PartText || plan.Parts[i].Place != sample.PlaceLeft {
				t.Errorf("sheet %d, part %d: %+v, line %q", n, i+1, plan.Parts[i], l)
			}
		}

		g := proto.CloneOf(es)
		g.Ink = &evalv1.Sample_Geometry{Geometry: &evalv1.Geometry{Strokes: protocol.FinalOf(protocol.Fold(es.GetHistory()))}}
		gs, _ := readOf(g, rb)
		if len(gs.Sample.Strokes) != len(protocol.Fold(es.GetHistory()).Visible) || len(gs.Sample.Erasures) != 0 || gs.Mouse || len(gs.Truth) != 1 {
			t.Errorf("sheet %d as a geometry: %d strokes, %d erasures, %d truths", n, len(gs.Sample.Strokes), len(gs.Sample.Erasures), len(gs.Truth))
		}
		if _, _, err := evalsample.Read(g, want, rb); !errors.Is(err, evalsample.ErrNotSheet) {
			t.Errorf("Read of a geometry: %v, want ErrNotSheet", err)
		}
	}

	fb, fp := read(t, free)
	rf, _, _, err := rs.Of(ref(tag, "eval/free-bank.json"))
	if err != nil {
		t.Fatal(err)
	}
	es := sampleOf(t, fileOf(t, fb, free.plan, &fp.Sheets[0], 1))
	released(t, es, trees)
	if _, plan := readOf(es, rf); plan.Parts[0].Kind != sample.PartDrawing || plan.Parts[1].Kind != sample.PartText {
		t.Errorf("a drawing: %+v", plan.Parts)
	}

	blank := sampleOf(t, file(t, b, &p.Sheets[0], 1))
	released(t, blank, trees)
	blank.Ink = &evalv1.Sample_History{History: &evalv1.History{}}
	blank.GetSheet().Groups, blank.GetSheet().Crops = nil, nil
	blank.Markup.Writing = &evalv1.Markup_Parts{Parts: &evalv1.WritingParts{}}
	blank.Markup.Truth.AnswerSlots[0].Read = &observationv1.AnswerSlotObservation_Unreadable{Unreadable: observationv1.Unreadable_UNREADABLE_BLANK}
	sh, plan := readOf(blank, rb)
	if sh.Truth != nil || len(sh.Sample.Strokes) != 0 || plan.Answer != 0 || len(plan.Parts) != 0 || plan.Expected.Outcome != sample.OutcomeNoAnswer {
		t.Errorf("a blank sheet: truth %v, %d strokes, the plan sheet %+v", sh.Truth, len(sh.Sample.Strokes), plan)
	}

	page := sampleOf(t, pageFile(t, readWorkbook(t), &readWorkbook(t).Workbook.Pages[0], 1))
	page.Task = es.GetTask()
	page.Markup = es.GetMarkup()
	if _, _, _, err := evalsample.ReadSheet(page, rf); !errors.Is(err, evalsample.ErrNotSheet) {
		t.Errorf("a page as a sheet: %v, want ErrNotSheet", err)
	}
}

// ReadMarkedPage reads a page whose boxes hold its own values: each box written as the markup has
// it, blank, a value correct or wrong by module-kit, rewritten where it is noted crossed out, and the
// key with a digit mirrored where it is noted so; the file's page stays as it is; a final geometry
// is read too. Markup the run of the workbook cannot stand for is ErrUnmarked.
func TestReadMarkedPage(t *testing.T) {
	trees := tree(t)
	_, rw, _, err := evalsample.NewReleases(trees).Of(ref(tag, "eval/workbook.json"))
	if err != nil {
		t.Fatal(err)
	}
	w := readWorkbook(t)
	pg := &w.Workbook.Pages[4] // page 5: blank 3.1, rewritten 4.1, wrong 5.2, mirrored 8.1
	data := pageFile(t, w, pg, 1)
	page := func(change func(es *evalv1.Sample)) (*evalsample.Page, error) {
		t.Helper()
		es := sampleOf(t, data)
		released(t, es, trees)
		if change != nil {
			change(es)
		}
		p, r, err := evalsample.ReadMarkedPage(es, rw)
		if r != nil {
			t.Fatalf("refused: %v", r)
		}
		return p, err
	}
	before, _ := os.ReadFile("../../eval/workbook.json")
	slot := func(es *evalv1.Sample, item, box int) *observationv1.AnswerSlotObservation {
		for _, a := range es.GetMarkup().GetTruth().GetAnswerSlots() {
			if a.GetAnswerSlot().GetItem() == int32(item) && a.GetAnswerSlot().GetSlot() == int32(box) {
				return a
			}
		}
		t.Fatalf("no box %d.%d", item, box)
		return nil
	}
	p, err := page(func(es *evalv1.Sample) {
		slot(es, 1, 1).Read = &observationv1.AnswerSlotObservation_Value{Value: "19"}
	})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]struct {
		scenario sample.BoxScenario
		written  string
	}{
		"1.1": {sample.BoxError, "19"}, "2.1": {sample.BoxCorrect, "7"}, "3.1": {sample.BoxBlank, ""},
		"4.1": {sample.BoxRewritten, "515"}, "5.2": {sample.BoxError, "10"}, "8.1": {sample.BoxMirrored, "1/2"},
	}
	for i, it := range p.Page.Items {
		for k, box := range it.Boxes {
			at := fmt.Sprintf("%d.%d", i+1, k+1)
			c, ok := want[at]
			if !ok {
				continue
			}
			got := ""
			if box.Written != nil {
				got = *box.Written
			}
			if box.Scenario != c.scenario || got != c.written || box.Check.Key != pg.Items[i].Boxes[k].Check.Key {
				t.Errorf("box %s: %s %q, want %s %q", at, box.Scenario, got, c.scenario, c.written)
			}
		}
	}
	if p.Page == pg || *pg.Items[0].Boxes[0].Written != "9" || len(p.Truth) == 0 || p.Ink == nil {
		t.Errorf("the page is the file's, or the file's page changed: %v", *pg.Items[0].Boxes[0].Written)
	}
	if after, _ := os.ReadFile("../../eval/workbook.json"); string(after) != string(before) {
		t.Error("the file of the workbook changed")
	}

	es := sampleOf(t, data)
	released(t, es, trees)
	g := proto.CloneOf(es)
	g.Ink = &evalv1.Sample_Geometry{Geometry: &evalv1.Geometry{Strokes: protocol.FinalOf(protocol.Fold(es.GetHistory()))}}
	if gp, r, err := evalsample.ReadMarkedPage(g, rw); err != nil || r != nil || len(gp.Ink.Boxes) != len(p.Ink.Boxes) {
		t.Errorf("a page of the final geometry: %v %v", r, err)
	}
	if _, _, err := evalsample.ReadPage(g, rw); !errors.Is(err, evalsample.ErrNotPage) {
		t.Errorf("ReadPage of a geometry: %v, want ErrNotPage", err)
	}

	for name, change := range map[string]func(es *evalv1.Sample){
		"a value in a box with no stroke": func(es *evalv1.Sample) {
			slot(es, 3, 1).Read = &observationv1.AnswerSlotObservation_Value{Value: "41"}
		},
		"a filled box read blank": func(es *evalv1.Sample) {
			slot(es, 1, 1).Read = &observationv1.AnswerSlotObservation_Unreadable{Unreadable: observationv1.Unreadable_UNREADABLE_BLANK}
		},
		"a box illegible": func(es *evalv1.Sample) {
			slot(es, 1, 1).Read = &observationv1.AnswerSlotObservation_Unreadable{Unreadable: observationv1.Unreadable_UNREADABLE_ILLEGIBLE}
		},
	} {
		if _, err := page(change); !errors.Is(err, evalsample.ErrUnmarked) {
			t.Errorf("%s: %v, want ErrUnmarked", name, err)
		}
	}
}
