package evalsample_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"
	observationv1 "github.com/altnovatrail/spec/contracts/novatrail/observation/v1"
	"google.golang.org/protobuf/proto"

	"github.com/altnovatrail/tutor/internal/essay"
	"github.com/altnovatrail/tutor/internal/evalsample"
	"github.com/altnovatrail/tutor/internal/protocol"
	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/synth"
	"github.com/altnovatrail/tutor/internal/workbook"
)

// The files of the workbook and of the essays, by their paths from this package.
const (
	theWorkbook = "../../eval/workbook.json"
	theEssays   = "../../eval/essays.json"
)

func readWorkbook(t *testing.T) *evalsample.Workbook {
	t.Helper()
	w, err := evalsample.ReadWorkbook(theWorkbook)
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func readEssays(t *testing.T) *evalsample.Essays {
	t.Helper()
	f, err := evalsample.ReadEssays(theEssays)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// pageFile is a page drawn from the seed and written as a sample of the protocol, as draw writes it.
func pageFile(t *testing.T, w *evalsample.Workbook, pg *sample.WorkbookPage, seed uint64) []byte {
	t.Helper()
	ink, err := synth.PageOf(pg, seed)
	if err != nil {
		t.Fatal(err)
	}
	id, err := evalsample.ID(theWorkbook, seed, synth.SimilarShift, pg.Number)
	if err != nil {
		t.Fatal(err)
	}
	es, err := evalsample.OfPage(ink, pg, w, seed, id)
	if err != nil {
		t.Fatalf("page %d: %v", pg.Number, err)
	}
	data, err := protocol.MarshalSample(es)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// essayFile is an essay drawn from the seed and written as a sample of the protocol, as draw writes
// it.
func essayFile(t *testing.T, f *evalsample.Essays, e *sample.Essay, seed uint64) []byte {
	t.Helper()
	id, err := evalsample.ID(theEssays, seed, synth.SimilarShift, e.Number)
	if err != nil {
		t.Fatal(err)
	}
	es, err := evalsample.OfEssay(e, f, seed, id)
	if err != nil {
		t.Fatalf("essay %d: %v", e.Number, err)
	}
	data, err := protocol.MarshalSample(es)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func sampleOf(t *testing.T, data []byte) *evalv1.Sample {
	t.Helper()
	es, err := protocol.ReadSample(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return es
}

// The files are named from the root of the repository, wherever they are read from.
func TestOwnSources(t *testing.T) {
	if w := readWorkbook(t); w.Path != "eval/workbook.json" || !strings.HasPrefix(w.Digest, "sha256:") {
		t.Errorf("the workbook is %s, %s", w.Path, w.Digest)
	}
	if f := readEssays(t); f.Path != "eval/essays.json" {
		t.Errorf("the essays are %s", f.Path)
	}
	if _, err := evalsample.ReadWorkbook("none.json"); err == nil {
		t.Error("a workbook that is not there")
	}
	if _, err := evalsample.ReadEssays("none.json"); err == nil {
		t.Error("essays that are not there")
	}
	if _, err := evalsample.WorkbookOf("x.json", []byte(`{}`)); err == nil {
		t.Error("a workbook of no format")
	}
	if _, err := evalsample.EssaysOf("x.json", []byte(`{}`)); err == nil {
		t.Error("essays of no format")
	}
}

// Every page of the workbook drawn with the seed 1 is a sample the runner reads back without a
// refusal and as its file has it: the truth of each box by its scenario, the notes of the boxes
// mirrored and crossed out, the boxes the rule of boxes fills those its file writes in; and the crops
// the runner cuts of its boxes are those of the page drawn in memory, to the byte, at every height.
func TestPages(t *testing.T) {
	w := readWorkbook(t)
	for i := range w.Workbook.Pages {
		pg := &w.Workbook.Pages[i]
		es := sampleOf(t, pageFile(t, w, pg, 1))
		read, refused, err := evalsample.ReadPage(es, w)
		if err != nil || refused != nil {
			t.Fatalf("page %d: %v %v", pg.Number, refused, err)
		}
		if off := evalsample.PageDifferences(es, pg); len(off) > 0 {
			t.Errorf("page %d is off its file at %v", pg.Number, off)
		}
		if es.GetId() != "synthetic-1-workbook-"+twoDigits(pg.Number) || es.GetTask().GetTask() != evalsample.PageTask(pg) || es.GetWorkbook().GetDigitHeight() != render.Digit {
			t.Errorf("page %d: %s, %s", pg.Number, es.GetId(), es.GetTask().GetTask())
		}
		k := 0
		for _, it := range pg.Items {
			for _, b := range it.Boxes {
				truth := es.GetMarkup().GetTruth().GetAnswerSlots()[k]
				k++
				switch b.Scenario {
				case sample.BoxBlank:
					if truth.GetUnreadable() != observationv1.Unreadable_UNREADABLE_BLANK {
						t.Errorf("page %d: a blank box is %v", pg.Number, truth)
					}
				case sample.BoxMirrored:
					if truth.GetUnreadable() != observationv1.Unreadable_UNREADABLE_AMBIGUOUS || !noted(es, truth.GetAnswerSlot(), func(n *evalv1.AnswerSlotNote) bool { return n.GetMirrored() }) {
						t.Errorf("page %d: a box mirrored is %v", pg.Number, truth)
					}
				case sample.BoxRewritten:
					if truth.GetValue() != *b.Written || !noted(es, truth.GetAnswerSlot(), func(n *evalv1.AnswerSlotNote) bool { return n.GetCrossedOut() }) {
						t.Errorf("page %d: a box rewritten is %v", pg.Number, truth)
					}
				default:
					if truth.GetValue() != *b.Written {
						t.Errorf("page %d: a box is %v, and its file writes %s", pg.Number, truth, *b.Written)
					}
				}
			}
		}
		for _, digit := range render.Heights() {
			got, err := workbook.CellsOf(read.Ink, read.Page, digit)
			if err != nil {
				t.Fatal(err)
			}
			want, err := workbook.Cells(pg, 1, digit)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.EqualFunc(got, want, func(a, b workbook.Cell) bool {
				return a.Item == b.Item && a.Slot == b.Slot && bytes.Equal(a.PNG, b.PNG)
			}) {
				t.Errorf("page %d: the crops at %d px are not those drawn in memory", pg.Number, digit)
			}
		}
	}
}

func twoDigits(n int) string { return string([]byte{byte('0' + n/10), byte('0' + n%10)}) }

// noted reports whether a note of the sample notes the answer slot as one says.
func noted(es *evalv1.Sample, at *observationv1.AnswerSlotPlace, as func(*evalv1.AnswerSlotNote) bool) bool {
	return slices.ContainsFunc(es.GetMarkup().GetNotes(), func(n *evalv1.AnswerSlotNote) bool { return proto.Equal(n.GetAnswerSlot(), at) && as(n) })
}

// Every essay drawn with the seed 1 is a sample the runner reads back without a refusal and as its
// file has it — its text as written, and a mark of every criterion of the rubric —, and the crop the
// runner cuts of it is that of the essay drawn in memory, to the byte.
func TestEssays(t *testing.T) {
	f := readEssays(t)
	for i := range f.Essays.Essays {
		e := &f.Essays.Essays[i]
		es := sampleOf(t, essayFile(t, f, e, 1))
		read, refused, err := evalsample.ReadEssay(es, f)
		if err != nil || refused != nil {
			t.Fatalf("essay %d: %v %v", e.Number, refused, err)
		}
		if off := evalsample.EssayDifferences(es, f, e); len(off) > 0 {
			t.Errorf("essay %d is off its file at %v", e.Number, off)
		}
		if read.Seed != "1" || es.GetMarkup().GetTruth().GetAnswerSlots()[0].GetValue() != e.Written() {
			t.Errorf("essay %d: seed %q", e.Number, read.Seed)
		}
		marks := es.GetMarkup().GetMarks().GetMarks()
		if len(marks) != len(f.Essays.Rubric) {
			t.Fatalf("essay %d has %d marks", e.Number, len(marks))
		}
		for k, c := range f.Essays.Rubric {
			if marks[k].GetCriterion() != int32(k+1) || marks[k].GetMet() == slices.Contains(e.Unmet, c.ID) {
				t.Errorf("essay %d: the mark of %s is %v", e.Number, c.ID, marks[k])
			}
		}
		got, err := essay.PictureFrom(synth.EssayLinesOf(e, 1), read.Strokes, render.Digit)
		if err != nil {
			t.Fatal(err)
		}
		want, err := essay.PictureOf(e, 1, render.Digit)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got.PNG, want.PNG) || !slices.Equal(got.Lines, want.Lines) {
			t.Errorf("essay %d: its crop or lines are not those drawn in memory", e.Number)
		}
	}
}

// A page whose boxes the rule of boxes does not give the strokes written in them, or that is drawn
// with other boxes than its file's, is drawn wrong, and is no sample.
func TestOfPageRefuses(t *testing.T) {
	w := readWorkbook(t)
	pg := &w.Workbook.Pages[0]
	ink, err := synth.PageOf(pg, 1)
	if err != nil {
		t.Fatal(err)
	}
	moved := *ink
	moved.Boxes = slices.Clone(ink.Boxes)
	moved.Boxes[0].Left += 300
	short := *ink
	short.Boxes = ink.Boxes[1:]
	half := *ink
	half.Boxes = slices.Clone(ink.Boxes)
	half.Boxes[0].Top += 0.5
	for name, p := range map[string]*synth.Page{"a box moved off its strokes": &moved, "a box short": &short, "a box at half a unit": &half} {
		if _, err := evalsample.OfPage(p, pg, w, 1, "x"); err == nil {
			t.Errorf("%s is a sample", name)
		}
	}
}

// ReadPage refuses a page as section 13 has the runner refuse one, and gives ErrNotPage for a sample
// that is no page with its history.
func TestReadPageRefuses(t *testing.T) {
	w := readWorkbook(t)
	pg := &w.Workbook.Pages[0]
	data := pageFile(t, w, pg, 1)
	for _, c := range []struct {
		name   string
		change func(*evalv1.Sample)
		reason evalv1.Refusal
		says   string
	}{
		{"no markup", func(es *evalv1.Sample) { es.Markup = nil }, evalv1.Refusal_REFUSAL_NO_MARKUP, "no markup"},
		{"another source", func(es *evalv1.Sample) {
			es.Task.Source = &evalv1.TaskReference_Tutor{Tutor: &evalv1.TutorSource{}}
		}, evalv1.Refusal_REFUSAL_OTHER_TASK, "not of a file of tasks"},
		{"another file", func(es *evalv1.Sample) { es.Task.GetFile().Bank = "eval/other.json" }, evalv1.Refusal_REFUSAL_OTHER_TASK, "of the file eval/other.json"},
		{"another digest of the file", func(es *evalv1.Sample) { es.Task.GetFile().FileDigest = "sha256:" + strings.Repeat("0", 64) }, evalv1.Refusal_REFUSAL_OTHER_TASK, "another digest"},
		{"no such page", func(es *evalv1.Sample) { es.Task.Task = "workbook-99" }, evalv1.Refusal_REFUSAL_OTHER_TASK, "no page of the task workbook-99"},
		{"another open view", func(es *evalv1.Sample) { es.Task.OpenViewDigest = "sha256:" + strings.Repeat("0", 64) }, evalv1.Refusal_REFUSAL_OTHER_TASK, "open view"},
		{"parts", func(es *evalv1.Sample) { es.Markup.Writing = &evalv1.Markup_Parts{Parts: &evalv1.WritingParts{}} }, evalv1.Refusal_REFUSAL_INVALID, "parts or marks"},
		{"marks", func(es *evalv1.Sample) { es.Markup.Writing = &evalv1.Markup_Marks{Marks: &evalv1.CriterionMarks{}} }, evalv1.Refusal_REFUSAL_INVALID, "parts or marks"},
		{"a box short", func(es *evalv1.Sample) {
			es.Markup.Truth.AnswerSlots = es.Markup.Truth.AnswerSlots[1:]
		}, evalv1.Refusal_REFUSAL_INVALID, "no valid observation"},
		{"a note of no box", func(es *evalv1.Sample) {
			es.Markup.Notes = append(es.Markup.Notes, &evalv1.AnswerSlotNote{AnswerSlot: &observationv1.AnswerSlotPlace{Item: 40, Slot: 1}, Mirrored: true})
		}, evalv1.Refusal_REFUSAL_INVALID, "names the box 40.1"},
		{"a child's", func(es *evalv1.Sample) { es.Writer = evalv1.Writer_WRITER_CHILD }, evalv1.Refusal_REFUSAL_CHILD, "П-3"},
	} {
		t.Run(c.name, func(t *testing.T) {
			es := sampleOf(t, data)
			c.change(es)
			_, refused, err := evalsample.ReadPage(es, w)
			if err != nil || refused == nil || refused.Reason != c.reason || !strings.Contains(refused.Error(), c.says) {
				t.Errorf("%v %v; want %s and %q", refused, err, c.reason, c.says)
			}
		})
	}
	es := sampleOf(t, data)
	es.Ink = &evalv1.Sample_Geometry{Geometry: &evalv1.Geometry{}}
	if _, _, err := evalsample.ReadPage(es, w); !errors.Is(err, evalsample.ErrNotPage) {
		t.Errorf("a page of a final geometry: %v", err)
	}
}

// PageDifferences names where a page's sample is off its file, by paths alone.
func TestPageDifferences(t *testing.T) {
	w := readWorkbook(t)
	pg := &w.Workbook.Pages[4]
	data := pageFile(t, w, pg, 1)
	for _, c := range []struct {
		change func(*evalv1.Sample)
		want   []string
	}{
		{func(es *evalv1.Sample) {
			es.Markup.Truth.AnswerSlots[2].Read = &observationv1.AnswerSlotObservation_Value{Value: "999"}
		}, []string{"markup.truth.answer_slots[2]"}},
		{func(es *evalv1.Sample) { es.Markup.Truth.AnswerSlots = es.Markup.Truth.AnswerSlots[1:] }, []string{"markup.truth.answer_slots"}},
		{func(es *evalv1.Sample) { es.Markup.Notes = nil }, []string{"markup.notes"}},
		{func(es *evalv1.Sample) { es.GetWorkbook().FilledBoxes = es.GetWorkbook().FilledBoxes[1:] }, []string{"workbook.filled_boxes"}},
		{func(es *evalv1.Sample) { es.GetWorkbook().Boxes = es.GetWorkbook().Boxes[1:] }, []string{"workbook.boxes"}},
	} {
		es := sampleOf(t, data)
		c.change(es)
		if got := evalsample.PageDifferences(es, pg); !slices.Equal(got, c.want) {
			t.Errorf("%v, want %v", got, c.want)
		}
	}
}

// ReadEssay refuses an essay as section 13 has the runner refuse one, and gives ErrNotEssay for a
// sample that is no essay on a sheet with its history.
func TestReadEssayRefuses(t *testing.T) {
	f := readEssays(t)
	e := &f.Essays.Essays[15]
	data := essayFile(t, f, e, 1)
	blank := func(es *evalv1.Sample) {
		es.Markup.Truth.AnswerSlots[0].Read = &observationv1.AnswerSlotObservation_Unreadable{Unreadable: observationv1.Unreadable_UNREADABLE_BLANK}
	}
	for _, c := range []struct {
		name   string
		change func(*evalv1.Sample)
		reason evalv1.Refusal
		says   string
	}{
		{"no markup", func(es *evalv1.Sample) { es.Markup = nil }, evalv1.Refusal_REFUSAL_NO_MARKUP, "no markup"},
		{"another file", func(es *evalv1.Sample) { es.Task.GetFile().Bank = "eval/workbook.json" }, evalv1.Refusal_REFUSAL_OTHER_TASK, "of the file eval/workbook.json"},
		{"no such essay", func(es *evalv1.Sample) { es.Task.Task = "essay-99" }, evalv1.Refusal_REFUSAL_OTHER_TASK, "no essay of the task essay-99"},
		{"another open view", func(es *evalv1.Sample) { es.Task.OpenViewDigest = "sha256:" + strings.Repeat("0", 64) }, evalv1.Refusal_REFUSAL_OTHER_TASK, "open view"},
		{"parts", func(es *evalv1.Sample) { es.Markup.Writing = &evalv1.Markup_Parts{Parts: &evalv1.WritingParts{}} }, evalv1.Refusal_REFUSAL_INVALID, "has parts"},
		{"a truth of no final answer", func(es *evalv1.Sample) {
			es.Markup.Truth.AnswerSlots[0].Read = &observationv1.AnswerSlotObservation_Unreadable{Unreadable: observationv1.Unreadable_UNREADABLE_NO_FINAL_ANSWER}
		}, evalv1.Refusal_REFUSAL_INVALID, "no valid observation"},
		{"a criterion the rubric has not", func(es *evalv1.Sample) {
			es.GetMarkup().GetMarks().Marks = append(es.GetMarkup().GetMarks().Marks, &evalv1.CriterionMark{Criterion: 7, Met: true})
		}, evalv1.Refusal_REFUSAL_INVALID, "criterion 7"},
		{"marks of an essay that cannot be read", blank, evalv1.Refusal_REFUSAL_INVALID, "reaches no assessment"},
		{"a child's", func(es *evalv1.Sample) { es.Writer = evalv1.Writer_WRITER_CHILD }, evalv1.Refusal_REFUSAL_CHILD, "П-3"},
	} {
		t.Run(c.name, func(t *testing.T) {
			es := sampleOf(t, data)
			c.change(es)
			_, refused, err := evalsample.ReadEssay(es, f)
			if err != nil || refused == nil || refused.Reason != c.reason || !strings.Contains(refused.Error(), c.says) {
				t.Errorf("%v %v; want %s and %q", refused, err, c.reason, c.says)
			}
		})
	}
	// A blank essay without marks reaches no assessment, and is read.
	es := sampleOf(t, data)
	blank(es)
	es.Markup.Writing = nil
	if _, refused, err := evalsample.ReadEssay(es, f); refused != nil || err != nil {
		t.Errorf("a blank essay with no marks: %v %v", refused, err)
	}
	es = sampleOf(t, data)
	es.Surface = &evalv1.Sample_Workbook{Workbook: &evalv1.Workbook{}}
	if _, _, err := evalsample.ReadEssay(es, f); !errors.Is(err, evalsample.ErrNotEssay) {
		t.Errorf("an essay on a workbook: %v", err)
	}
}

// EssayDifferences names where an essay's sample is off its file, by paths alone.
func TestEssayDifferences(t *testing.T) {
	f := readEssays(t)
	e := &f.Essays.Essays[15]
	data := essayFile(t, f, e, 1)
	for _, c := range []struct {
		change func(*evalv1.Sample)
		want   []string
	}{
		{func(es *evalv1.Sample) {
			es.Markup.Truth.AnswerSlots[0].Read = &observationv1.AnswerSlotObservation_Value{Value: "x"}
		}, []string{"markup.truth"}},
		{func(es *evalv1.Sample) {
			es.Markup.Notes = []*evalv1.AnswerSlotNote{{AnswerSlot: &observationv1.AnswerSlotPlace{Item: 1, Slot: 1}, CrossedOut: true}}
		}, []string{"markup.notes"}},
		{func(es *evalv1.Sample) { es.GetMarkup().GetMarks().Marks = es.GetMarkup().GetMarks().Marks[1:] }, []string{"markup.marks"}},
		{func(es *evalv1.Sample) { m := es.GetMarkup().GetMarks().Marks[2]; m.Met = !m.Met }, []string{"markup.marks.marks[2]"}},
	} {
		es := sampleOf(t, data)
		c.change(es)
		if got := evalsample.EssayDifferences(es, f, e); !slices.Equal(got, c.want) {
			t.Errorf("%v, want %v", got, c.want)
		}
	}
}

// A directory of samples is read against its file: a sample of each page by its number's name, the
// other files named and not read, a sample sample-check finds breaking a rule refused with its
// lines, one that cannot be read strictly refused, one of another page's task off its file, and the
// seeds of those read; a check that cannot run is an error, and so is a directory that is not there.
func TestReadDirs(t *testing.T) {
	w := readWorkbook(t)
	dir := t.TempDir()
	write := func(name string, data []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(evalsample.PageFile(1), pageFile(t, w, &w.Workbook.Pages[0], 1))
	write(evalsample.PageFile(2), pageFile(t, w, &w.Workbook.Pages[1], 1))
	write(evalsample.PageFile(3), []byte(`{"protocol": "novatrail.eval.v1", "unknown": 1}`))
	write(evalsample.PageFile(4), pageFile(t, w, &w.Workbook.Pages[0], 1))
	write(evalsample.PageFile(5), pageFile(t, w, &w.Workbook.Pages[4], 2))
	nomarkup := sampleOf(t, pageFile(t, w, &w.Workbook.Pages[6], 1))
	nomarkup.Markup = nil
	write(evalsample.PageFile(7), marshal(t, nomarkup))
	geometry := sampleOf(t, pageFile(t, w, &w.Workbook.Pages[7], 1))
	geometry.Ink = &evalv1.Sample_Geometry{Geometry: &evalv1.Geometry{}}
	write(evalsample.PageFile(8), marshal(t, geometry))
	off := sampleOf(t, pageFile(t, w, &w.Workbook.Pages[8], 1))
	off.Markup.Notes = nil
	write(evalsample.PageFile(9), marshal(t, off))
	write("page-1.json", nil)
	write("page-99.json", nil)
	write("essay-01.json", nil)
	if err := os.Mkdir(filepath.Join(dir, evalsample.PageFile(6)), 0o700); err != nil {
		t.Fatal(err)
	}
	var given []string
	d, err := evalsample.ReadPagesDir(dir, w, func(files []string) (map[string][]int, error) {
		given = files
		return map[string][]int{filepath.Join(dir, evalsample.PageFile(2)): {3, 8}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(given) != 8 || len(d.Drawn) != 20 {
		t.Errorf("the check is given %v, of %d pages", given, len(d.Drawn))
	}
	if d.Drawn[0].Read == nil || d.Drawn[0].Seed != "1" {
		t.Errorf("page 1: %+v", d.Drawn[0])
	}
	if r := d.Drawn[1].Refused; r == nil || r.Reason != evalv1.Refusal_REFUSAL_INVALID || !strings.Contains(r.Detail, "at lines 3, 8") {
		t.Errorf("page 2: %v", r)
	}
	if r := d.Drawn[2].Refused; r == nil || !strings.Contains(r.Detail, "cannot be read strictly") {
		t.Errorf("page 3: %v", r)
	}
	if p := d.Drawn[3].Problems; !slices.Equal(p, []string{"task"}) {
		t.Errorf("page 4, of page 1's task: %v", p)
	}
	if d.Drawn[5].File != "" || d.Drawn[9].File != "" {
		t.Errorf("pages 6 and 10: %+v %+v", d.Drawn[5], d.Drawn[9])
	}
	if r := d.Drawn[6].Refused; r == nil || r.Reason != evalv1.Refusal_REFUSAL_NO_MARKUP {
		t.Errorf("page 7, with no markup: %v", r)
	}
	if p := d.Drawn[7].Problems; len(p) != 1 || !strings.Contains(p[0], "no page of a workbook with its history") {
		t.Errorf("page 8, of a final geometry: %v", p)
	}
	if p := d.Drawn[8].Problems; !slices.Equal(p, []string{"markup.notes"}) {
		t.Errorf("page 9, off its file: %v", p)
	}
	slices.Sort(d.Other)
	if want := []string{"essay-01.json", "page-06.json", "page-1.json", "page-99.json"}; !slices.Equal(d.Other, want) {
		t.Errorf("other files %v, want %v", d.Other, want)
	}
	if seeds := d.Seeds(); !slices.Equal(seeds, []string{"1", "2"}) {
		t.Errorf("seeds %v", seeds)
	}
	if _, err := evalsample.ReadPagesDir(dir, w, func([]string) (map[string][]int, error) { return nil, errors.New("buf fails") }); err == nil {
		t.Error("a check that cannot run")
	}
	if _, err := evalsample.ReadPagesDir(filepath.Join(dir, "none"), w, nil); err == nil {
		t.Error("a directory that is not there")
	}

	f := readEssays(t)
	edir := t.TempDir()
	put := func(n int, es *evalv1.Sample) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(edir, evalsample.EssayFile(n)), marshal(t, es), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	put(1, sampleOf(t, essayFile(t, f, &f.Essays.Essays[1], 1)))
	refused := sampleOf(t, essayFile(t, f, &f.Essays.Essays[2], 1))
	refused.Writer = evalv1.Writer_WRITER_CHILD
	put(3, refused)
	surface := sampleOf(t, essayFile(t, f, &f.Essays.Essays[3], 1))
	surface.Surface = &evalv1.Sample_Workbook{Workbook: &evalv1.Workbook{}}
	put(4, surface)
	unmarked := sampleOf(t, essayFile(t, f, &f.Essays.Essays[4], 1))
	unmarked.GetMarkup().GetMarks().Marks[0].Met = !unmarked.GetMarkup().GetMarks().Marks[0].Met
	put(5, unmarked)
	put(6, sampleOf(t, essayFile(t, f, &f.Essays.Essays[5], 1)))
	ed, err := evalsample.ReadEssaysDir(edir, f, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p := ed.Drawn[0].Problems; !slices.Equal(p, []string{"task"}) || ed.Drawn[1].File != "" {
		t.Errorf("essay 1, of essay 2's task: %v", p)
	}
	if r := ed.Drawn[2].Refused; r == nil || r.Reason != evalv1.Refusal_REFUSAL_CHILD {
		t.Errorf("essay 3, a child's: %v", r)
	}
	if p := ed.Drawn[3].Problems; len(p) != 1 || !strings.Contains(p[0], "no essay on a sheet") {
		t.Errorf("essay 4, on a workbook: %v", p)
	}
	if p := ed.Drawn[4].Problems; !slices.Equal(p, []string{"markup.marks.marks[0]"}) {
		t.Errorf("essay 5, off its file: %v", p)
	}
	if ed.Drawn[5].Read == nil || !slices.Equal(ed.Seeds(), []string{"1"}) {
		t.Errorf("essay 6: %+v, seeds %v", ed.Drawn[5], ed.Seeds())
	}
}

func marshal(t *testing.T, es *evalv1.Sample) []byte {
	t.Helper()
	data, err := protocol.MarshalSample(es)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
