package tutor

import (
	"context"
	"fmt"

	observationv1 "github.com/altnovatrail/spec/contracts/novatrail/observation/v1"
	paperv1 "github.com/altnovatrail/spec/contracts/novatrail/paper/v1"

	"github.com/altnovatrail/module-kit/paper"

	"github.com/altnovatrail/tutor/internal/reading"
	"github.com/altnovatrail/tutor/internal/solving"
	"github.com/altnovatrail/tutor/internal/statement"
	"github.com/altnovatrail/tutor/provider"
)

// PaperTask is a task from the learner's paper whose key is derived: the task's crop as app cut it
// from the photo of the page, without the learner's strokes, a JPEG or a PNG of at most 4 MiB
// (PaperWork.statement of spec's contract), and the route. The models see the printed task alone.
type PaperTask struct {
	Route     Route
	Statement []byte
}

// StatementRead is what the reading of a statement gives when Outcome is Answered: the formula, or
// "" when the statement is no formula. Whether the solver takes it module-kit says.
type StatementRead struct {
	Formula string
	Outcome Outcome
}

// ReadStatement asks the admitted models of the reading of a statement (CallStatement) for the
// task's statement as a formula (R135), one at a time, a later one only when the one before failed:
// a statement that is no formula is an answer (R137). The crop goes as a photo does
// (provider.Photo): fitted to the size its model takes, and as a JPEG without the metadata of its
// file. A crop that is no JPEG or PNG fails before any call; an answer off the schema fails the
// call.
func (t *Tutor) ReadStatement(ctx context.Context, p PaperTask) StatementRead {
	formula, out := askCrop(ctx, t, CallStatement, p, statement.RequestOf, statement.MaxOutputTokens,
		statement.Read, func(string) bool { return true })
	return StatementRead{Formula: formula, Outcome: out}
}

// TaskSolved is what the solution of a task gives when Outcome is Answered: the Answer of AI, a
// number or a choice, or nil when the model found no one answer of these kinds. module-kit holds it
// to the rules of a key (paper.Key): an answer that breaks one gives the task no key, as nil does.
type TaskSolved struct {
	Answer  *paperv1.Answer
	Outcome Outcome
}

// SolveTask asks the admitted models of the solution (CallSolving), one solution of one model, for
// the answer of the task on its crop: the key of a task whose statement the solver does not take
// (R162 spec; R135). A later model is asked when the one before failed or gave no answer a key may
// hold — none, or one that breaks a rule of module-kit (paper.Key) —, and the last one's answer
// stands (R137). The crop goes as ReadStatement sends it; an answer off the schema fails the call.
func (t *Tutor) SolveTask(ctx context.Context, p PaperTask) TaskSolved {
	answer, out := askCrop(ctx, t, CallSolving, p, solving.RequestOf, solving.MaxOutputTokens,
		solving.Read, func(a *paperv1.Answer) bool { return paper.Key(nil, a).GetAnswer() != nil })
	return TaskSolved{Answer: answer, Outcome: out}
}

// askCrop asks the models of a call on the task's crop, by the request of its prompt, one at a time
// (cascade): read takes a model's answer into what the call gives, and did tells whether it will
// do. The crop is fitted to each model's size.
func askCrop[T any](ctx context.Context, t *Tutor, c Call, p PaperTask, request func(provider.Image, int, int) provider.Request, maxOut int64,
	read func(string) (T, error), did func(T) bool) (T, Outcome) {
	var zero T
	calls, a, refused := t.prepare(c, p.Route)
	if refused != nil {
		return zero, *refused
	}
	return cascade(ctx, calls, func(cl call) (T, Outcome) {
		fail := func(err error) (T, Outcome) {
			return zero, Outcome{Kind: Failed, Admission: a, Err: err}
		}
		crop, width, height, err := provider.Photo(p.Statement, cl.client)
		if err != nil {
			return fail(fmt.Errorf("tutor: the task's crop: %w", err))
		}
		req := request(crop, width, height)
		req.MaxOutputTokens = maxOut
		if err := checkText(req); err != nil {
			return fail(err)
		}
		resps, out := ask(ctx, a, []call{cl}, []provider.Request{req}, kinds[c].maxIn)
		if out.Kind != Answered {
			return zero, out
		}
		got, err := read(resps[0].JSON)
		if err != nil {
			out.Kind, out.Err = Failed, fmt.Errorf("tutor: %s: %w", cl.model.Name(), err)
			return zero, out
		}
		return got, out
	}, did)
}

// PaperReading is the work on a task from paper to read: the PNG of the crop of the work — the
// tracing paper at its place with the strokes on it, and the sheet's visible strokes —, as app sends
// it (PaperWork.crop), and the route. The models see the crop alone, never the key (R67 spec).
type PaperReading struct {
	Route Route
	Crop  []byte
}

// ReadPaper reads the work on a task from paper by the models admitted for its reading (CallPaper),
// in any reading mode, one at a time, a later one only when the one before failed or read the
// answer as unreadable (R137), into the observation of the task's one answer slot, at the place
// 1, 1 (R162 spec): the learner's final answer as written, or the label of the one option circled.
// api brings its value to the key and module-kit judges it.
func (t *Tutor) ReadPaper(ctx context.Context, r PaperReading) SheetRead {
	return t.readCrop(ctx, CallPaper, r.Route, r.Crop, &observationv1.AnswerSlotPlace{Item: 1, Slot: 1}, func(width, height int) provider.Request {
		return reading.PaperRequestOf(r.Crop, width, height)
	})
}
