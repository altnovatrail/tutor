package tutor

import (
	"context"
	"errors"
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

// StatementTask is a task the reading of a statement found on the crop: its statement as printed,
// at most 200 bytes, and its formula, or "" when the statement is no formula. Whether the solver
// takes a formula module-kit says.
type StatementTask struct {
	Statement string
	Formula   string
}

// StatementRead is what the reading of a statement gives when Outcome is Answered: the tasks on
// the crop, from none to 20, in the order of the page — one, or each of a column of sums framed
// together (R162 spec).
type StatementRead struct {
	Tasks   []StatementTask
	Outcome Outcome
}

// ReadStatement asks the admitted models of the reading of a statement (CallStatement) for the
// tasks of the crop, each with its statement and its formula (R135), one at a time, a later one only when the one before failed:
// a statement that is no formula is an answer (R137). The crop goes as a photo does
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

// TaskToSolve is a task of the crop whose key AI derives: its number on the crop, from 1, and its
// statement as the reading of the statement gave it.
type TaskToSolve struct {
	Number    int
	Statement string
}

// TasksSolved is what the solution of tasks gives when Outcome is Answered: the Answer of AI of
// each task asked, by its number, a number or a choice, or nil when the model found no one answer of
// these kinds. module-kit holds each to the rules of a key (paper.Key): an answer that breaks one
// gives its task no key, as nil does.
type TasksSolved struct {
	Answers map[int]*paperv1.Answer
	Outcome Outcome
}

// SolveTask asks the admitted models of the solution (CallSolving), one solution of one model, for
// the answers of the tasks of the crop it names: the keys of the tasks whose statement the solver
// does not take (R162 spec; R135), all in one call. A later model is asked when the one before
// failed or left a task asked with no answer a key may hold — none, or one that breaks a rule of
// module-kit (paper.Key) —, and the last one's answers stand (R137). The crop goes as
// ReadStatement sends it; an answer off the schema fails the call.
func (t *Tutor) SolveTask(ctx context.Context, p PaperTask, tasks []TaskToSolve) TasksSolved {
	asked := make([]solving.Task, len(tasks))
	for i, task := range tasks {
		asked[i] = solving.Task{Number: task.Number, Statement: task.Statement}
	}
	answers, out := askCrop(ctx, t, CallSolving, p, func(crop provider.Image, width, height int) provider.Request {
		return solving.RequestOf(crop, width, height, asked)
	}, solving.MaxOutputTokens, func(answer string) (map[int]*paperv1.Answer, error) {
		return solving.Read(answer, asked)
	}, func(answers map[int]*paperv1.Answer) bool {
		for _, task := range asked {
			if paper.Key(nil, answers[task.Number]).GetAnswer() == nil {
				return false
			}
		}
		return true
	})
	return TasksSolved{Answers: answers, Outcome: out}
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
// it (PaperWork.crop), the statements of the tasks on it, in order, as the reading of the statement
// gave them, and the route. The models see the crop and the statements, never a key (R67 spec).
type PaperReading struct {
	Route Route
	Crop  []byte
	Tasks []string
}

// ReadPaper reads the work on a task from paper by the models admitted for its reading (CallPaper),
// in any reading mode, one at a time, a later one only when the one before failed or read an
// answer as illegible or ambiguous (R137), into the observation of an answer slot for each task,
// the task n at the place n, 1 (R162 spec): the learner's final answer to it as written, or the
// label of the one option circled, with its box on the crop. api brings each value to its task's
// key and module-kit judges it. A reading with no task fails before any call.
func (t *Tutor) ReadPaper(ctx context.Context, r PaperReading) SheetRead {
	if len(r.Tasks) == 0 {
		return SheetRead{Outcome: Outcome{Kind: Failed, Err: errors.New("tutor: the reading of paper has no task")}}
	}
	return t.readCrop(ctx, CallPaper, r.Route, r.Crop, func(width, height int) provider.Request {
		return reading.PaperRequestOf(r.Crop, width, height, r.Tasks)
	}, func(answer string) (*observationv1.Observation, error) {
		o, err := reading.PaperRead(answer, len(r.Tasks))
		if err != nil {
			return nil, err
		}
		return reading.PaperObservationOf(o, len(r.Tasks)), nil
	})
}
