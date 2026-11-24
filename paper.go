package tutor

import (
	"context"
	"fmt"

	"github.com/altnovatrail/tutor/internal/statement"
	"github.com/altnovatrail/tutor/provider"
)

// PaperTask is a task from the learner's paper whose statement is read: the task's crop as app cut it
// from the photo of the page, without the learner's strokes, a JPEG or a PNG of at most 4 MiB
// (PaperWork.statement of spec's contract), and the route. The models see the printed task alone.
type PaperTask struct {
	Route     Route
	Statement []byte
}

// StatementTask is a task the reading of a statement found on the crop: its statement as printed,
// at most 200 bytes, and its formula, or "" when the statement is no formula. Whether the solver
// takes a formula module-kit says.
type StatementTask struct {
	Statement string
	Formula   string
}

// StatementRead is what the reading of a statement gives when Outcome is Answered: the tasks on
// the crop, from none to 20, in the order of the page — one, or each of a column of sums framed
// together.
type StatementRead struct {
	Tasks   []StatementTask
	Outcome Outcome
}

// ReadStatement asks the admitted models of the reading of a statement (CallStatement) for the
// tasks of the crop, each with its statement and its formula, one at a time, a later one only when the one before failed:
// a statement that is no formula is an answer. The crop goes as a photo does
// (provider.Photo): fitted to the size its model takes, and as a JPEG without the metadata of its
// file. A crop that is no JPEG or PNG fails before any call; an answer off the schema fails the
// call.
func (t *Tutor) ReadStatement(ctx context.Context, p PaperTask) StatementRead {
	read, out := askCrop(ctx, t, CallStatement, p, statement.RequestOf, statement.MaxOutputTokens,
		statement.Read, func([]statement.Task) bool { return true })
	tasks := make([]StatementTask, len(read))
	for i, r := range read {
		tasks[i] = StatementTask{Statement: r.Statement, Formula: r.Formula}
	}
	return StatementRead{Tasks: tasks, Outcome: out}
}

// askCrop asks the models of a call on the task's crop, by the request of its prompt, one at a time
// (cascade): read takes a model's answer into what the call gives, and did tells whether it will
// do. The crop is decoded once and fitted to each model's size.
func askCrop[T any](ctx context.Context, t *Tutor, c Call, p PaperTask, request func(provider.Image, int, int) provider.Request, maxOut int64,
	read func(string) (T, error), did func(T) bool) (T, Outcome) {
	var zero T
	calls, a, refused := t.prepare(c, p.Route)
	if refused != nil {
		return zero, *refused
	}
	crops := provider.NewPhotos(p.Statement)
	return cascade(ctx, calls, func(cl call) (T, Outcome) {
		fail := func(err error) (T, Outcome) {
			return zero, Outcome{Kind: Failed, Admission: a, Err: err}
		}
		crop, width, height, err := crops.For(cl.client)
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
