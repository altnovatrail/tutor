package evalsample

import (
	"errors"
	"fmt"

	"github.com/altnovatrail/module-kit/observation"
	"github.com/altnovatrail/module-kit/verdict"
	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"
	"github.com/altnovatrail/spec/types/formatv0"

	"github.com/altnovatrail/tutor/internal/lines"
	"github.com/altnovatrail/tutor/internal/protocol"
	"github.com/altnovatrail/tutor/internal/sample"
)

// Refusal is why the runner refuses a sample, by the reasons of section 13 that its report names
// (evalv1.Refusal), and what of the sample it is, in words: nothing of the handwriting, neither a
// point nor the text of a part.
type Refusal struct {
	Reason evalv1.Refusal
	Detail string
	// Sample is the ID the sample's file names, or "" when it names none that can be read: the
	// runner's report names a refused sample by it (R74).
	Sample string
}

func (r *Refusal) Error() string { return fmt.Sprintf("%s: %s", Reason(r.Reason), r.Detail) }

// Reason is a reason of refusal as the summary and the run name it: the end of its name in the
// protocol, in lowercase words.
func Reason(r evalv1.Refusal) string {
	switch r {
	case evalv1.Refusal_REFUSAL_INVALID:
		return "invalid"
	case evalv1.Refusal_REFUSAL_NO_MARKUP:
		return "no markup"
	case evalv1.Refusal_REFUSAL_OTHER_TASK:
		return "another task"
	case evalv1.Refusal_REFUSAL_CHILD:
		return "a child's"
	}
	return r.String()
}

func refuse(r evalv1.Refusal, format string, args ...any) *Refusal {
	return &Refusal{Reason: r, Detail: fmt.Sprintf(format, args...)}
}

// ErrNotSheet is the error of a sample that is no sheet of measurement with its history: a final
// geometry, or a page of a workbook, which the runner of sheets does not read.
var ErrNotSheet = errors.New("evalsample: the sample is no sheet of measurement with its history, and the runner of sheets reads only those")

// Sheet is a sample of the protocol read as a sheet of its plan: the sample tutor's runner takes,
// whether a mouse wrote any of its strokes, and what the runner's report takes of it, its ID and
// the truth of its answer slot (R74).
type Sheet struct {
	Sample *sample.Sample
	Mouse  bool
	ID     string
	Truth  []Truth
}

// Read reads a sample of the protocol, which sample-check has found valid, as the plan sheet its
// file's name saves, of the bank of its set. It refuses the sample, as section 13 has it, when:
//   - it has no markup;
//   - its task is not the plan sheet's: another source than its set's bank, a bank of another digest,
//     no such task, or an open view of another digest;
//   - its markup does not fit its task: its truth is no valid observation of the task's, or gets no
//     verdict (module-kit), or it marks criteria of a sheet with no rubric;
//   - a child wrote it, whom no contract with a provider admits yet (П-3): nothing of it goes to AI.
//
// What the runner takes of a sample is a sheet of measurement with its history: the strokes and
// erasures of the history (protocol.Ink), and its parts as the lines. Its scenario, its planned
// erasure and its trap are its plan sheet's, since the protocol names none (R69); its outcome is the
// verdict of module-kit on the truth, by the task's key, and its first error the markup's. A sample
// of another ink or surface is no sheet the runner reads, and Read gives an error, not a refusal.
func Read(es *evalv1.Sample, sh *sample.PlanSheet, b *Bank) (*Sheet, *Refusal, error) {
	m := es.GetMarkup()
	if m == nil {
		return nil, refuse(evalv1.Refusal_REFUSAL_NO_MARKUP, "the sample has no markup, and the runner measures nothing on it"), nil
	}
	ref := es.GetTask()
	if r := b.refuse(ref); r != nil {
		return nil, r, nil
	}
	t := b.task(ref.GetTask())
	if t == nil {
		return nil, refuse(evalv1.Refusal_REFUSAL_OTHER_TASK, "the bank %s has no task %s", b.Path, ref.GetTask()), nil
	}
	if t.ID != sh.Task {
		return nil, refuse(evalv1.Refusal_REFUSAL_OTHER_TASK, "its task is %s, and its plan sheet's %s", t.ID, sh.Task), nil
	}
	ft, digest, err := sheetTask(t)
	if err != nil {
		return nil, nil, fmt.Errorf("evalsample: the open view of %s: %w", t.ID, err)
	}
	if ref.GetOpenViewDigest() != digest {
		return nil, refuse(evalv1.Refusal_REFUSAL_OTHER_TASK, "the open view of its task %s is of another digest than the bank's", t.ID), nil
	}
	outcome, items, r := outcomeOf(ft, m)
	if r != nil {
		return nil, r, nil
	}
	truth := truthsOf(items, m, nil)
	if es.GetWriter() == evalv1.Writer_WRITER_CHILD {
		return nil, refuse(evalv1.Refusal_REFUSAL_CHILD, "a child wrote it, and nothing of a child's sample goes to AI until a contract with the provider admits children (П-3)"), nil
	}

	h, sheet := es.GetHistory(), es.GetSheet()
	if h == nil || sheet == nil {
		return nil, nil, ErrNotSheet
	}
	strokes, erasures, _ := protocol.Ink(h)
	s := &sample.Sample{
		Format:   sample.SampleFormat,
		Version:  new(sample.Version),
		Sheet:    sample.Sheet{Width: float64(sheet.GetWidth()), Height: float64(sheet.GetHeight())},
		Task:     t.ID,
		Writer:   writerOf(es),
		Scenario: sh.Scenario,
		Strokes:  strokes,
		Erasures: erasures,
		Lines:    []sample.Line{},
		Expected: sample.Expected{Outcome: outcome},
	}
	for _, p := range m.GetParts().GetParts() {
		l := sample.Line{Text: p.GetText(), Illegible: p.GetIllegible(), CrossedOut: p.GetCrossedOut(), Answer: p.GetFinalAnswer(), Strokes: []string{}}
		for _, id := range p.GetStrokeIds() {
			l.Strokes = append(l.Strokes, protocol.StrokeID(id))
		}
		s.Lines = append(s.Lines, l)
	}
	if m.GetParts().FirstError != nil {
		s.Expected.FirstError = new(int(m.GetParts().GetFirstError()))
	}
	if outcome == sample.OutcomeError {
		s.Expected.Trap = sh.Expected.Trap
	}
	planErasure(s, sh)

	mouse := false
	for _, e := range h.GetEvents() {
		mouse = mouse || e.GetWrite().GetPointer() == "mouse"
	}
	return &Sheet{Sample: s, Mouse: mouse, ID: es.GetId(), Truth: truth}, nil, nil
}

// outcomeOf is the outcome of a sheet as the verdict of module-kit on its truth gives it, by the
// task's key, with the verdicts it gave: correct is correct, not yet right is an error, an answer illegible or ambiguous is
// unreadable, and no answer, several, or a blank sheet is no answer. A truth that is no valid
// observation of the task, or gets no verdict, is a markup that does not fit its task, and so is a
// mark of criteria on a sheet whose task has no rubric (section 13).
func outcomeOf(ft formatv0.Task, m *evalv1.Markup) (sample.Outcome, []verdict.ItemVerdict, *Refusal) {
	if m.GetMarks() != nil {
		return "", nil, refuse(evalv1.Refusal_REFUSAL_INVALID, "its markup marks criteria, and its task %s has no rubric", ft.ID)
	}
	if err := observation.Validate(ft.Items, m.GetTruth()); err != nil {
		return "", nil, refuse(evalv1.Refusal_REFUSAL_INVALID, "its truth is no valid observation of its task %s", ft.ID)
	}
	items, err := observation.Verdicts(ctx, ft.Items, m.GetTruth(), nil)
	if err != nil || len(items) != 1 || len(items[0].Slots) != 1 {
		return "", nil, refuse(evalv1.Refusal_REFUSAL_INVALID, "its truth gets no verdict of its task %s", ft.ID)
	}
	v := items[0].Slots[0]
	switch v.Verdict {
	case verdict.Correct:
		return sample.OutcomeCorrect, items, nil
	case verdict.NotYet:
		return sample.OutcomeError, items, nil
	case verdict.Unreadable:
		if v.Reason == verdict.Illegible || v.Reason == verdict.Ambiguous || v.Reason == verdict.Notation {
			return sample.OutcomeUnreadable, items, nil
		}
		return sample.OutcomeNoAnswer, items, nil
	}
	return "", nil, refuse(evalv1.Refusal_REFUSAL_INVALID, "its truth gets the verdict %s of its task %s", v.Verdict, ft.ID)
}

// writerOf is who wrote a sample, as tutor's code names a writer: the code that drew it, or the
// author in their own hand or imitating a child's.
func writerOf(es *evalv1.Sample) sample.Writer {
	switch {
	case es.GetSynthetic() != nil:
		return sample.WriterSynthetic
	case es.GetWriter() == evalv1.Writer_WRITER_ADULT_AS_CHILD:
		return sample.WriterAuthorAsChild
	}
	return sample.WriterAuthor
}

// planErasure marks the planned erasure of a sheet whose plan sheet plans one: the last erasure whose
// strokes stood where the plan's erasure took them from, the line or the part it names, as
// sheets.ErasurePlace finds it. The other erasures are slips of the pen.
func planErasure(s *sample.Sample, sh *sample.PlanSheet) {
	pe := sh.Erasure
	if pe == nil {
		return
	}
	for i := len(s.Erasures) - 1; i >= 0; i-- {
		place := lines.ErasureLine(s, s.Erasures[i])
		if len(sh.Parts) > 0 {
			place = lines.ErasurePart(s, s.Erasures[i])
		}
		if place == pe.Line {
			s.Erasures[i].Planned = &sample.PlannedErasure{Line: pe.Line, Erased: pe.Erased, Text: pe.Text}
			return
		}
	}
}
