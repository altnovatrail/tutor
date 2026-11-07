package evalsample

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/altnovatrail/module-kit/observation"
	mkview "github.com/altnovatrail/module-kit/openview"
	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"
	observationv1 "github.com/altnovatrail/spec/contracts/novatrail/observation/v1"
	"google.golang.org/protobuf/proto"

	"github.com/altnovatrail/tutor/internal/protocol"
	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/synth"
	"github.com/altnovatrail/tutor/internal/task"
)

// Workbook is the file of the pages of the workbook as a sample names it, the source file of its
// task reference (R69), and the pages it holds.
type Workbook struct {
	Source
	Workbook *sample.Workbook
}

// ReadWorkbook reads the file of the workbook at a path, as a sample names it (ReadSource), and
// checks it, the format and the layout of every page.
func ReadWorkbook(file string) (*Workbook, error) {
	at, data, err := ReadSource(file)
	if err != nil {
		return nil, err
	}
	return WorkbookOf(at, data)
}

// WorkbookOf is the workbook of the bytes data at the path given, from the repository's root.
func WorkbookOf(at string, data []byte) (*Workbook, error) {
	wb, err := sample.ReadWorkbook(strings.NewReader(string(data)))
	if err == nil {
		err = synth.CheckLayout(wb)
	}
	if err != nil {
		return nil, err
	}
	return &Workbook{Source: SourceOf(at, data), Workbook: wb}, nil
}

// page is the workbook's page of the task's ID, workbook-01 for the first (task.Page), or nil.
func (w *Workbook) page(id string) *sample.WorkbookPage {
	for i := range w.Workbook.Pages {
		if PageTask(&w.Workbook.Pages[i]) == id {
			return &w.Workbook.Pages[i]
		}
	}
	return nil
}

// PageTask is the ID of a page's task, as its sample names it.
func PageTask(pg *sample.WorkbookPage) string { return string(task.Page(pg).ID) }

// ErrNotPage is the error of a sample that is no page of a workbook with its history: a final
// geometry, or a sheet, which the runner of the workbook does not read.
var ErrNotPage = errors.New("evalsample: the sample is no page of a workbook with its history, and the runner of the workbook reads only those")

// OfPage is a page of the workbook that internal/synth drew from the seed, as a sample of
// measurement of the protocol, from an adult, with its history of writing (section 11): its strokes
// numbered from 1 in the order they were written, box by box, their points to a tenth of a unit;
// the page, synth.PageWidth × synth.PageHeight, with every box of the page in the task's order and
// the filled ones, by the rule of boxes, each with its crops at each height of section 11; the task
// of the page by its open view's digest (task.Page); and the markup the page was drawn with
// (pageMarkup). A page whose boxes the rule of boxes does not give the strokes code wrote in them is
// drawn wrong, and OfPage refuses it.
func OfPage(drawn *synth.Page, pg *sample.WorkbookPage, w *Workbook, seed uint64, id string) (*evalv1.Sample, error) {
	ft := task.Page(pg)
	digest, err := mkview.TaskDigest(ft)
	if err != nil {
		return nil, fmt.Errorf("evalsample: the open view of %s: %w", ft.ID, err)
	}
	if len(drawn.Boxes) != boxesOf(pg) {
		return nil, fmt.Errorf("evalsample: page %d is drawn with %d boxes, and its file has %d", pg.Number, len(drawn.Boxes), boxesOf(pg))
	}
	s := &sample.Sample{Strokes: []sample.Stroke{}}
	ids := map[string]int32{}
	boxes := make([]*evalv1.PageBox, len(drawn.Boxes))
	written := make([][]int32, len(drawn.Boxes))
	for i, b := range drawn.Boxes {
		if b.Left != math.Trunc(b.Left) || b.Top != math.Trunc(b.Top) {
			return nil, fmt.Errorf("evalsample: page %d, item %d, box %d stands at %g, %g, and a box of the protocol at whole units", pg.Number, b.Item, b.Slot, b.Left, b.Top)
		}
		boxes[i] = &evalv1.PageBox{AnswerSlot: place(b.Item, b.Slot), Left: int32(b.Left), Top: int32(b.Top), Width: synth.BoxSide, Height: synth.BoxSide}
		for _, st := range b.Strokes {
			ids[st.ID] = int32(len(s.Strokes) + 1)
			written[i] = append(written[i], ids[st.ID])
			s.Strokes = append(s.Strokes, st)
		}
	}
	h, err := historyOf(s, ids)
	if err != nil {
		return nil, err
	}
	strokes, _, f := protocol.Ink(h)
	table := protocol.Table(protocol.FinalOf(f), boxes)
	surface := &evalv1.Workbook{Width: synth.PageWidth, Height: synth.PageHeight, DigitHeight: render.Digit, Boxes: boxes, FilledBoxes: []*evalv1.FilledBox{}}
	k := 0
	for i, b := range drawn.Boxes {
		if len(written[i]) == 0 {
			continue
		}
		if k >= len(table) || !proto.Equal(table[k].GetAnswerSlot(), boxes[i].GetAnswerSlot()) || !slices.Equal(table[k].GetStrokeIds(), written[i]) {
			return nil, fmt.Errorf("evalsample: page %d, item %d, box %d: the rule of boxes does not give the box the strokes written in it", pg.Number, b.Item, b.Slot)
		}
		var of []sample.Stroke
		for _, n := range written[i] {
			of = append(of, strokes[n-1])
		}
		crops, err := CropSet(of, nil, Heights(render.Digit))
		if err != nil {
			return nil, err
		}
		table[k].Crops = crops
		surface.FilledBoxes = append(surface.FilledBoxes, table[k])
		k++
	}
	if k != len(table) {
		return nil, fmt.Errorf("evalsample: page %d: the rule of boxes fills a box that holds no stroke", pg.Number)
	}
	es := drawnSample(seed, id, w.reference(string(ft.ID), string(digest)), h, pageMarkup(pg))
	es.Surface = &evalv1.Sample_Workbook{Workbook: surface}
	return es, nil
}

// boxesOf counts the boxes of a page.
func boxesOf(pg *sample.WorkbookPage) int {
	n := 0
	for _, it := range pg.Items {
		n += len(it.Boxes)
	}
	return n
}

func place(item, slot int) *observationv1.AnswerSlotPlace {
	return &observationv1.AnswerSlotPlace{Item: int32(item), Slot: int32(slot)}
}

// pageMarkup is the markup of a page as its file writes it (section 11): the truth of every box, in
// the task's order, and the notes of the boxes measured apart. A blank box is blank; a digit written
// mirrored gives no value a reader that reads right can give, and is ambiguous, with the note
// mirrored; a value crossed out and written again beside it is the value left, with the note
// crossed_out; any other box is the value written. A page has neither parts nor marks: its boxes are
// its places.
func pageMarkup(pg *sample.WorkbookPage) *evalv1.Markup {
	truth := &observationv1.Observation{}
	notes := []*evalv1.AnswerSlotNote{}
	for i, it := range pg.Items {
		for k, b := range it.Boxes {
			read := &observationv1.AnswerSlotObservation{AnswerSlot: place(i+1, k+1)}
			switch {
			case b.Written == nil:
				read.Read = unreadable(observationv1.Unreadable_UNREADABLE_BLANK)
			case b.Scenario == sample.BoxMirrored:
				read.Read = unreadable(observationv1.Unreadable_UNREADABLE_AMBIGUOUS)
				notes = append(notes, &evalv1.AnswerSlotNote{AnswerSlot: place(i+1, k+1), Mirrored: true})
			default:
				read.Read = &observationv1.AnswerSlotObservation_Value{Value: *b.Written}
			}
			if b.Crossed != nil {
				notes = append(notes, &evalv1.AnswerSlotNote{AnswerSlot: place(i+1, k+1), CrossedOut: true})
			}
			truth.AnswerSlots = append(truth.AnswerSlots, read)
		}
	}
	return &evalv1.Markup{Truth: truth, Notes: notes}
}

// Page is a sample of the protocol read as a page of the workbook: the page of its file, and its
// boxes as code draws them (synth.Page), each with the strokes the sample's table of boxes puts in
// it, as the history writes them; a blank box has none.
type Page struct {
	Page *sample.WorkbookPage
	Ink  *synth.Page
}

// ReadPage reads a sample of the protocol, which sample-check has found valid, as the page of the
// workbook its task names. It refuses the sample, as section 13 has it, when:
//   - it has no markup;
//   - its task is not of the workbook: another source, a file of another digest, no such page, or an
//     open view of another digest;
//   - its markup does not fit its task: its truth is no valid observation of the page's, or gets no
//     verdict (module-kit); it has parts or marks, which a page has not; or a note names no box;
//   - a child wrote it (П-3).
//
// A sample of another ink or surface is no page the runner reads, and ReadPage gives ErrNotPage.
func ReadPage(es *evalv1.Sample, w *Workbook) (*Page, *Refusal, error) {
	m := es.GetMarkup()
	if m == nil {
		return nil, refuse(evalv1.Refusal_REFUSAL_NO_MARKUP, "the sample has no markup, and the runner measures nothing on it"), nil
	}
	ref := es.GetTask()
	if r := w.refuse(ref); r != nil {
		return nil, r, nil
	}
	pg := w.page(ref.GetTask())
	if pg == nil {
		return nil, refuse(evalv1.Refusal_REFUSAL_OTHER_TASK, "the workbook %s has no page of the task %s", w.Path, ref.GetTask()), nil
	}
	ft := task.Page(pg)
	digest, err := mkview.TaskDigest(ft)
	if err != nil {
		return nil, nil, fmt.Errorf("evalsample: the open view of %s: %w", ft.ID, err)
	}
	if ref.GetOpenViewDigest() != string(digest) {
		return nil, refuse(evalv1.Refusal_REFUSAL_OTHER_TASK, "the open view of its task %s is of another digest than the workbook's", ft.ID), nil
	}
	switch {
	case m.GetParts() != nil || m.GetMarks() != nil:
		return nil, refuse(evalv1.Refusal_REFUSAL_INVALID, "its markup has parts or marks, and a page of a workbook has neither"), nil
	case observation.Validate(ft.Items, m.GetTruth()) != nil:
		return nil, refuse(evalv1.Refusal_REFUSAL_INVALID, "its truth is no valid observation of its task %s", ft.ID), nil
	}
	if _, err := observation.Verdicts(ctx, ft.Items, m.GetTruth(), nil); err != nil {
		return nil, refuse(evalv1.Refusal_REFUSAL_INVALID, "its truth gets no verdict of its task %s", ft.ID), nil
	}
	for _, n := range m.GetNotes() {
		if !slotOf(pg, n.GetAnswerSlot()) {
			return nil, refuse(evalv1.Refusal_REFUSAL_INVALID, "a note names the box %d.%d, which its task %s has not", n.GetAnswerSlot().GetItem(), n.GetAnswerSlot().GetSlot(), ft.ID), nil
		}
	}
	if es.GetWriter() == evalv1.Writer_WRITER_CHILD {
		return nil, refuse(evalv1.Refusal_REFUSAL_CHILD, "a child wrote it, and nothing of a child's sample goes to AI until a contract with the provider admits children (П-3)"), nil
	}
	h, surface := es.GetHistory(), es.GetWorkbook()
	if h == nil || surface == nil {
		return nil, nil, ErrNotPage
	}
	strokes, _, _ := protocol.Ink(h)
	byID := map[int32]sample.Stroke{}
	for _, st := range strokes {
		n, _ := protocol.StrokeNumber(st.ID)
		byID[n] = st
	}
	ink := &synth.Page{Number: pg.Number}
	for _, b := range surface.GetBoxes() {
		box := synth.Box{Item: int(b.GetAnswerSlot().GetItem()), Slot: int(b.GetAnswerSlot().GetSlot()), Left: float64(b.GetLeft()), Top: float64(b.GetTop())}
		for _, fb := range surface.GetFilledBoxes() {
			if proto.Equal(fb.GetAnswerSlot(), b.GetAnswerSlot()) {
				for _, id := range fb.GetStrokeIds() {
					box.Strokes = append(box.Strokes, byID[id])
				}
			}
		}
		ink.Boxes = append(ink.Boxes, box)
	}
	return &Page{Page: pg, Ink: ink}, nil, nil
}

// slotOf reports whether a place is a box of the page.
func slotOf(pg *sample.WorkbookPage, p *observationv1.AnswerSlotPlace) bool {
	i, k := int(p.GetItem()), int(p.GetSlot())
	return i >= 1 && i <= len(pg.Items) && k >= 1 && k <= len(pg.Items[i-1].Boxes)
}

// PageDifferences are where a page's sample is off what its file says of the page, by paths alone,
// nothing of the handwriting: the boxes, which must be the page's in its order; the truth of each
// box and the notes, which must be pageMarkup's; and the filled boxes, which must be those the file
// writes a value in. A sample off its file is a problem of its page, as a sheet off its plan is
// (sheets.Differences).
func PageDifferences(es *evalv1.Sample, pg *sample.WorkbookPage) []string {
	var off []string
	want := pageMarkup(pg)
	got := es.GetMarkup().GetTruth().GetAnswerSlots()
	if len(got) != len(want.GetTruth().GetAnswerSlots()) {
		off = append(off, "markup.truth.answer_slots")
	} else {
		for i, a := range want.GetTruth().GetAnswerSlots() {
			if !proto.Equal(a, got[i]) {
				off = append(off, fmt.Sprintf("markup.truth.answer_slots[%d]", i))
			}
		}
	}
	if !slices.EqualFunc(want.GetNotes(), es.GetMarkup().GetNotes(), func(a, b *evalv1.AnswerSlotNote) bool { return proto.Equal(a, b) }) {
		off = append(off, "markup.notes")
	}
	surface := es.GetWorkbook()
	var places, filled []string
	for i, it := range pg.Items {
		for k, b := range it.Boxes {
			at := fmt.Sprintf("%d.%d", i+1, k+1)
			places = append(places, at)
			if b.Written != nil {
				filled = append(filled, at)
			}
		}
	}
	var boxes, full []string
	for _, b := range surface.GetBoxes() {
		boxes = append(boxes, fmt.Sprintf("%d.%d", b.GetAnswerSlot().GetItem(), b.GetAnswerSlot().GetSlot()))
	}
	for _, b := range surface.GetFilledBoxes() {
		full = append(full, fmt.Sprintf("%d.%d", b.GetAnswerSlot().GetItem(), b.GetAnswerSlot().GetSlot()))
	}
	if !slices.Equal(boxes, places) {
		off = append(off, "workbook.boxes")
	}
	if !slices.Equal(full, filled) {
		off = append(off, "workbook.filled_boxes")
	}
	return off
}
