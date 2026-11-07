package evalsample

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/altnovatrail/module-kit/observation"
	mkview "github.com/altnovatrail/module-kit/openview"
	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"
	observationv1 "github.com/altnovatrail/spec/contracts/novatrail/observation/v1"
	"google.golang.org/protobuf/proto"

	"github.com/altnovatrail/tutor/internal/protocol"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/synth"
	"github.com/altnovatrail/tutor/internal/task"
)

// Essays is the file of the essays as a sample names it, the source file of its task reference
// (R69), and the rubric and essays it holds.
type Essays struct {
	Source
	Essays *sample.Essays
}

// ReadEssays reads the file of the essays at a path, as a sample names it (ReadSource), and checks
// it.
func ReadEssays(file string) (*Essays, error) {
	at, data, err := ReadSource(file)
	if err != nil {
		return nil, err
	}
	return EssaysOf(at, data)
}

// EssaysOf is the essays of the bytes data at the path given, from the repository's root.
func EssaysOf(at string, data []byte) (*Essays, error) {
	es, err := sample.ReadEssays(strings.NewReader(string(data)))
	if err != nil {
		return nil, err
	}
	return &Essays{Source: SourceOf(at, data), Essays: es}, nil
}

// essay is the file's essay of the task's ID, essay-01 for the first (task.Essay), or nil.
func (f *Essays) essay(id string) *sample.Essay {
	for i := range f.Essays.Essays {
		if EssayTask(f.Essays, &f.Essays.Essays[i]) == id {
			return &f.Essays.Essays[i]
		}
	}
	return nil
}

// EssayTask is the ID of an essay's task, as its sample names it.
func EssayTask(es *sample.Essays, e *sample.Essay) string {
	return string(task.Essay(es.Rubric, e).ID)
}

// ErrNotEssay is the error of a sample that is no essay on a sheet with its history: a final
// geometry, or a page of a workbook, which the runner of the essays does not read.
var ErrNotEssay = errors.New("evalsample: the sample is no essay on a sheet with its history, and the runner of the essays reads only those")

// OfEssay is an essay that internal/synth drew from the seed, as a sample of measurement of the
// protocol, from an adult, with its history of writing (section 11): its strokes numbered from 1 in
// the order they were written; the sheet, synth.EssayWidth × synth.EssayHeight, with the groups of
// its strokes and their crops; the essay's task by its open view's digest (task.Essay); and the
// markup it was drawn with (essayMarkup).
func OfEssay(e *sample.Essay, f *Essays, seed uint64, id string) (*evalv1.Sample, error) {
	ft := task.Essay(f.Essays.Rubric, e)
	digest, err := mkview.TaskDigest(ft)
	if err != nil {
		return nil, fmt.Errorf("evalsample: the open view of %s: %w", ft.ID, err)
	}
	_, strokes := synth.Essay(e, seed)
	s := &sample.Sample{Strokes: strokes}
	ids := map[string]int32{}
	for i, st := range strokes {
		ids[st.ID] = int32(i + 1)
	}
	h, err := historyOf(s, ids)
	if err != nil {
		return nil, err
	}
	sheet, err := sheetOf(h, sample.Sheet{Width: synth.EssayWidth, Height: synth.EssayHeight})
	if err != nil {
		return nil, err
	}
	es := drawnSample(seed, id, f.reference(string(ft.ID), string(digest)), h, essayMarkup(f.Essays.Rubric, e))
	es.Surface = &evalv1.Sample_Sheet{Sheet: sheet}
	return es, nil
}

// essayMarkup is the markup of an essay as its file writes it (section 11): the truth of its one
// answer slot, the essay as it is written (sample.Essay.Written), and a mark of every criterion of
// the rubric, by its place from 1: met unless the file names it among those the essay does not meet.
// Code marks a synthetic essay, so no criterion lacks a mark.
func essayMarkup(rubric []sample.Criterion, e *sample.Essay) *evalv1.Markup {
	marks := &evalv1.CriterionMarks{}
	for i, c := range rubric {
		marks.Marks = append(marks.Marks, &evalv1.CriterionMark{Criterion: int32(i + 1), Met: !slices.Contains(e.Unmet, c.ID)})
	}
	return &evalv1.Markup{
		Truth: &observationv1.Observation{AnswerSlots: []*observationv1.AnswerSlotObservation{{
			AnswerSlot: slot(),
			Read:       &observationv1.AnswerSlotObservation_Value{Value: e.Written()},
		}}},
		Notes:   []*evalv1.AnswerSlotNote{},
		Writing: &evalv1.Markup_Marks{Marks: marks},
	}
}

// Essay is a sample of the protocol read as an essay of its file: the essay, the strokes the sheet
// shows at the end, in the order they were written, and the seed it was drawn from, or "" for a
// person's.
type Essay struct {
	Essay   *sample.Essay
	Strokes []sample.Stroke
	Seed    string
}

// ReadEssay reads a sample of the protocol, which sample-check has found valid, as the essay of the
// file its task names. It refuses the sample, as section 13 has it, when:
//   - it has no markup;
//   - its task is not of the essays: another source, a file of another digest, no such essay, or an
//     open view of another digest;
//   - its markup does not fit its task: its truth is no valid observation of the essay's (module-kit),
//     it has parts, which an essay has not, a mark names a criterion its rubric has not, or an essay
//     that cannot be read has marks, since it reaches no assessment;
//   - a child wrote it (П-3).
//
// A sample of another ink or surface is no essay the runner reads, and ReadEssay gives ErrNotEssay.
func ReadEssay(es *evalv1.Sample, f *Essays) (*Essay, *Refusal, error) {
	m := es.GetMarkup()
	if m == nil {
		return nil, refuse(evalv1.Refusal_REFUSAL_NO_MARKUP, "the sample has no markup, and the runner measures nothing on it"), nil
	}
	ref := es.GetTask()
	if r := f.refuse(ref); r != nil {
		return nil, r, nil
	}
	e := f.essay(ref.GetTask())
	if e == nil {
		return nil, refuse(evalv1.Refusal_REFUSAL_OTHER_TASK, "the essays %s have no essay of the task %s", f.Path, ref.GetTask()), nil
	}
	ft := task.Essay(f.Essays.Rubric, e)
	digest, err := mkview.TaskDigest(ft)
	if err != nil {
		return nil, nil, fmt.Errorf("evalsample: the open view of %s: %w", ft.ID, err)
	}
	if ref.GetOpenViewDigest() != string(digest) {
		return nil, refuse(evalv1.Refusal_REFUSAL_OTHER_TASK, "the open view of its task %s is of another digest than the essays'", ft.ID), nil
	}
	if m.GetParts() != nil {
		return nil, refuse(evalv1.Refusal_REFUSAL_INVALID, "its markup has parts, and an essay has marks"), nil
	}
	end, err := observation.Essay(ft.Items, m.GetTruth())
	if err != nil {
		return nil, refuse(evalv1.Refusal_REFUSAL_INVALID, "its truth is no valid observation of its task %s", ft.ID), nil
	}
	for _, mark := range m.GetMarks().GetMarks() {
		if c := mark.GetCriterion(); c < 1 || int(c) > len(f.Essays.Rubric) {
			return nil, refuse(evalv1.Refusal_REFUSAL_INVALID, "a mark names the criterion %d, and the rubric of its task %s has %d", c, ft.ID, len(f.Essays.Rubric)), nil
		}
	}
	if end != observation.SubmissionAssessment && len(m.GetMarks().GetMarks()) > 0 {
		return nil, refuse(evalv1.Refusal_REFUSAL_INVALID, "its truth cannot be read, and it has marks, though it reaches no assessment"), nil
	}
	if es.GetWriter() == evalv1.Writer_WRITER_CHILD {
		return nil, refuse(evalv1.Refusal_REFUSAL_CHILD, "a child wrote it, and nothing of a child's sample goes to AI until a contract with the provider admits children (П-3)"), nil
	}
	h := es.GetHistory()
	if h == nil || es.GetSheet() == nil {
		return nil, nil, ErrNotEssay
	}
	strokes, _, fold := protocol.Ink(h)
	return &Essay{Essay: e, Strokes: Shown(strokes, fold), Seed: es.GetSynthetic().GetSeed()}, nil, nil
}

// EssayDifferences are where an essay's sample is off what its file says of the essay, by paths
// alone, nothing of the handwriting: its truth and its marks, which must be essayMarkup's.
func EssayDifferences(es *evalv1.Sample, f *Essays, e *sample.Essay) []string {
	var off []string
	want := essayMarkup(f.Essays.Rubric, e)
	if !proto.Equal(want.GetTruth(), es.GetMarkup().GetTruth()) {
		off = append(off, "markup.truth")
	}
	if len(es.GetMarkup().GetNotes()) > 0 {
		off = append(off, "markup.notes")
	}
	got := es.GetMarkup().GetMarks().GetMarks()
	if len(got) != len(want.GetMarks().GetMarks()) {
		return append(off, "markup.marks")
	}
	for i, mark := range want.GetMarks().GetMarks() {
		if !proto.Equal(mark, got[i]) {
			off = append(off, fmt.Sprintf("markup.marks.marks[%d]", i))
		}
	}
	return off
}
