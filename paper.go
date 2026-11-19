package tutor

import (
	"context"
	"fmt"

	observationv1 "github.com/altnovatrail/spec/contracts/novatrail/observation/v1"
	paperv1 "github.com/altnovatrail/spec/contracts/novatrail/paper/v1"

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

// ReadStatement asks the admitted model of the reading of a statement (CallStatement) for the
// task's statement as a formula (R135). The crop goes as a photo does (provider.Photo): fitted to the
// size its model takes, and as a JPEG without the metadata of its file. A crop that is no JPEG or
// PNG fails before any call; an answer off the schema fails the call.
func (t *Tutor) ReadStatement(ctx context.Context, p PaperTask) StatementRead {
	resp, out := t.askCrop(ctx, CallStatement, p, statement.RequestOf, statement.MaxOutputTokens)
	if out.Kind != Answered {
		return StatementRead{Outcome: out}
	}
	formula, err := statement.Read(resp.JSON)
	if err != nil {
		out.Kind, out.Err = Failed, fmt.Errorf("tutor: %w", err)
		return StatementRead{Outcome: out}
	}
	return StatementRead{Formula: formula, Outcome: out}
}

// TaskSolved is what the solution of a task gives when Outcome is Answered: the Answer of AI, a
// number or a choice, or nil when the model found no one answer of these kinds. module-kit holds it
// to the rules of a key (paper.Key): an answer that breaks one gives the task no key, as nil does.
type TaskSolved struct {
	Answer  *paperv1.Answer
	Outcome Outcome
}

// SolveTask asks the admitted model of the solution (CallSolving), one model, for the answer of the
// task on its crop: the key of a task whose statement the solver does not take (R162 spec; R135).
// The crop goes as ReadStatement sends it; an answer off the schema fails the call.
func (t *Tutor) SolveTask(ctx context.Context, p PaperTask) TaskSolved {
	resp, out := t.askCrop(ctx, CallSolving, p, solving.RequestOf, solving.MaxOutputTokens)
	if out.Kind != Answered {
		return TaskSolved{Outcome: out}
	}
	answer, err := solving.Read(resp.JSON)
	if err != nil {
		out.Kind, out.Err = Failed, fmt.Errorf("tutor: %w", err)
		return TaskSolved{Outcome: out}
	}
	return TaskSolved{Answer: answer, Outcome: out}
}

// askCrop asks the one model of a call on the task's crop, by the request of its prompt.
func (t *Tutor) askCrop(ctx context.Context, c Call, p PaperTask, request func(provider.Image, int, int) provider.Request, maxOut int64) (provider.Response, Outcome) {
	calls, a, refused := t.prepare(c, p.Route)
	if refused != nil {
		return provider.Response{}, *refused
	}
	fail := func(err error) (provider.Response, Outcome) {
		return provider.Response{}, Outcome{Kind: Failed, Admission: a, Err: err}
	}
	crop, width, height, err := provider.Photo(p.Statement, calls[0].client)
	if err != nil {
		return fail(fmt.Errorf("tutor: the task's crop: %w", err))
	}
	req := request(crop, width, height)
	req.MaxOutputTokens = maxOut
	if err := checkText(req); err != nil {
		return fail(err)
	}
	resps, out := ask(ctx, a, calls, []provider.Request{req}, kinds[c].maxIn)
	if out.Kind != Answered {
		return provider.Response{}, out
	}
	return resps[0], out
}

// PaperReading is the work on a task from paper to read: the PNG of the crop of the work — the
// tracing paper at its place with the strokes on it, and the sheet's visible strokes —, as app sends
// it (PaperWork.crop), and the route. The models see the crop alone, never the key (R67 spec).
type PaperReading struct {
	Route Route
	Crop  []byte
}

// ReadPaper reads the work on a task from paper by the models admitted for its reading (CallPaper) —
// the pair, in parallel, or its one reader in ReadingSingle —, each into the observation of the
// task's one answer slot, at the place 1, 1 (R162 spec): the learner's final answer as written, or
// the label of the one option circled. A reading gives the observations as ReadSheet does, both of
// the pair or none; api brings their values to the key and module-kit joins and judges them.
func (t *Tutor) ReadPaper(ctx context.Context, r PaperReading) SheetRead {
	return t.readCrop(ctx, CallPaper, r.Route, r.Crop, &observationv1.AnswerSlotPlace{Item: 1, Slot: 1}, func(width, height int) provider.Request {
		return reading.PaperRequestOf(r.Crop, width, height)
	})
}
