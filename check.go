package tutor

import (
	"context"
	"errors"
	"fmt"
	"strings"

	observationv1 "github.com/altnovatrail/spec/contracts/novatrail/observation/v1"

	"github.com/altnovatrail/tutor/admission"
	"github.com/altnovatrail/tutor/internal/checking"
	"github.com/altnovatrail/tutor/provider"
)

// PaperCheck is a press of Check on a task from the learner's paper:
//   - Statement is the task's crop as app cut it from the photo of the page, without the learner's
//     strokes, a JPEG or a PNG of at most 4 MiB (PaperWork.statement of spec's contract);
//   - Work is the PNG of the work: the tracing paper at its place with the strokes on it, and the
//     sheet's strokes in sight (PaperWork.crop), on which every box of the answer lies;
//   - Instruction is the crop of the instruction printed above the frame, a JPEG or a PNG, or nil
//     when the finding gave none (PaperWork.instruction);
//   - Tasks is how many tasks the frame holds, 1 to 20;
//   - Language is the language of the profile's interface, by its tag, and AgeCategory the
//     profile's age category as api names it, the route's when empty;
//   - History is the attempt's Checks before this one, in order, which api keeps.
//
// The model sees the pictures and the text as data, and nothing of who the learner is.
type PaperCheck struct {
	Route       Route
	Statement   []byte
	Work        []byte
	Instruction []byte
	Tasks       int
	Language    string
	AgeCategory string
	History     []CheckTurn
}

// CheckTurn is a Check of the attempt before this one: the verdict of each task, in the order of
// the frame, as SlotChecked names it; the tutor's words it gave, or "" when they were left out; and
// the rungs of its ladder the learner was shown, in order.
type CheckTurn struct {
	Verdicts []string
	Words    string
	Rungs    []string
}

// PaperChecked is what the Check gives when Outcome is Answered: a slot for each task, 1 to
// Tasks, in the order of the frame; the tutor's words, "" where they named the hidden answer of a
// task not yet correct; and the ladder of 2 or 3 hints, each rung that named one given way to its
// fallback text (PaperFallback).
type PaperChecked struct {
	Slots      []SlotChecked
	TutorWords string
	Hints      []string
	Outcome    Outcome
}

// SlotChecked is the Check of one task of the frame, by its number from 1:
//   - Verdict is "correct", "incorrect", "unreadable" or "no_answer", the model's, and Confidence
//     "low", "medium" or "high";
//   - Read is the learner's answer as the model read it, "" for none, and ReadBox its box in whole
//     thousandths of the work, nil when nothing was read;
//   - Assumptions are what the model took for given that the statement does not say;
//   - Errors are the learner's mistakes on the task, the first one first;
//   - Solution and Answer are the model's own, which the learner never sees: api keeps them while
//     the attempt is open, for the leak check of the tutor's words that follow.
type SlotChecked struct {
	Task                      int
	Verdict, Confidence, Read string
	ReadBox                   *observationv1.CropBox
	Assumptions               []string
	Errors                    []CheckError
	Solution, Answer          string
}

// CheckError is a mistake of the learner's on a task: its box on the work, in whole thousandths of
// it, or nil when the model gave it no place; the place in words; its kind; and what is wrong there,
// without the right value.
type CheckError struct {
	Box               *observationv1.CropBox
	Where, Kind, Note string
}

// CheckPaper asks the admitted models of the Check (CallChecking), one at a time, a later one only
// when the one before failed, answered off the schema, gave a verdict of low confidence or read a
// task as unreadable; the last answer stands. The task's crop and the instruction go as photos do
// (provider.Photo), fitted to each model, and the work as the PNG it is, never shrunk, so that its
// boxes are those of the picture app holds. Once the cascade is over, the tutor's words and each
// rung are searched for the hidden answer of every task not yet correct, a number the learner wrote
// in a slot being no leak (PaperBreach's search): words that name one are left out, and such a rung
// is its fallback text. A frame of no task or of more than 20, a crop that is no picture and a work
// that is no PNG fail before any call.
func (t *Tutor) CheckPaper(ctx context.Context, c PaperCheck) PaperChecked {
	fail := func(err error) PaperChecked { return PaperChecked{Outcome: Outcome{Kind: Failed, Err: err}} }
	if c.Tasks < 1 || c.Tasks > checking.MaxTasks {
		return fail(fmt.Errorf("tutor: the Check of %d tasks, not 1 to %d", c.Tasks, checking.MaxTasks))
	}
	if len(c.Statement) == 0 {
		return fail(errors.New("tutor: the Check has no crop of the task"))
	}
	work, err := cropSize(c.Work)
	if err != nil {
		return fail(err)
	}
	calls, a, refused := t.prepare(CallChecking, c.Route)
	if refused != nil {
		return PaperChecked{Outcome: *refused}
	}
	category := c.AgeCategory
	if category == "" {
		category = string(c.Route.Category)
	}
	p := checking.Prompt{Work: checking.Picture{Image: provider.Image{PNG: c.Work}, Width: work.Width, Height: work.Height},
		Tasks: c.Tasks, Language: c.Language, AgeCategory: category}
	for _, h := range c.History {
		p.History = append(p.History, checking.Turn{Verdicts: h.Verdicts, Words: h.Words, Rungs: h.Rungs})
	}
	statements := provider.NewPhotos(c.Statement)
	var instructions *provider.Photos
	if len(c.Instruction) > 0 {
		instructions = provider.NewPhotos(c.Instruction)
	}
	got, out := cascade(ctx, calls, func(cl call) (*checking.Check, Outcome) {
		return checkBy(ctx, a, cl, p, statements, instructions)
	}, sure)
	if out.Kind != Answered {
		return PaperChecked{Outcome: out}
	}
	return checked(got, c.Language, out)
}

// checkBy is one attempt of the Check, by one model: the crops fitted to it, the request, and the
// answer read against the frame's tasks and the work's size.
func checkBy(ctx context.Context, a admission.Answer, cl call, p checking.Prompt, statements, instructions *provider.Photos) (*checking.Check, Outcome) {
	fail := func(err error) (*checking.Check, Outcome) { return nil, Outcome{Kind: Failed, Admission: a, Err: err} }
	crop, width, height, err := statements.For(cl.client)
	if err != nil {
		return fail(fmt.Errorf("tutor: the task's crop: %w", err))
	}
	p.Statement = checking.Picture{Image: crop, Width: width, Height: height}
	if instructions != nil {
		crop, width, height, err := instructions.For(cl.client)
		if err != nil {
			return fail(fmt.Errorf("tutor: the instruction's crop: %w", err))
		}
		p.Instruction = &checking.Picture{Image: crop, Width: width, Height: height}
	}
	req, err := checking.RequestOf(p)
	if err != nil {
		return fail(fmt.Errorf("tutor: %w", err))
	}
	req.MaxOutputTokens = checking.MaxOutputTokens
	if err := checkText(req); err != nil {
		return fail(err)
	}
	resps, out := ask(ctx, a, []call{cl}, []provider.Request{req}, kinds[CallChecking].maxIn)
	if out.Kind != Answered {
		return nil, out
	}
	got, err := checking.Read(resps[0].JSON, p.Tasks, p.Work.Width, p.Work.Height)
	if err != nil {
		out.Kind, out.Err = Failed, fmt.Errorf("tutor: %s: %w", cl.model.Name(), err)
		return nil, out
	}
	return got, out
}

// sure tells whether a Check will do: no verdict of low confidence and no task unreadable, which a
// stronger model may judge or read.
func sure(c *checking.Check) bool {
	for _, s := range c.Slots {
		if s.Confidence == checking.ConfidenceLow || s.Verdict == checking.VerdictUnreadable {
			return false
		}
	}
	return true
}

// checked is the Check as the library gives it, its tutor's words and rungs held to the leak check:
// the hidden answer of each task not yet correct is searched for, and what every slot read is the
// learner's, so no leak.
func checked(c *checking.Check, language string, out Outcome) PaperChecked {
	got := PaperChecked{TutorWords: c.TutorWords, Outcome: out}
	var tasks []PaperWordsTask
	for _, s := range c.Slots {
		slot := SlotChecked{Task: s.Task, Verdict: s.Verdict, Confidence: s.Confidence, Read: s.Read, ReadBox: s.ReadBox,
			Assumptions: s.Assumptions, Errors: []CheckError{}, Solution: s.Solution, Answer: s.Answer}
		for _, e := range s.Errors {
			slot.Errors = append(slot.Errors, CheckError{Box: e.Box, Where: e.Where, Kind: e.Kind, Note: e.Note})
		}
		got.Slots = append(got.Slots, slot)
		task := PaperWordsTask{Read: s.Read}
		if s.Verdict != checking.VerdictCorrect {
			task.Answer = s.Answer
		}
		tasks = append(tasks, task)
	}
	if leaks(got.TutorWords, tasks) {
		got.TutorWords = ""
	}
	for i, h := range c.Hints {
		if strings.TrimSpace(h) == "" || leaks(h, tasks) {
			h = PaperFallback(i+1, language)
		}
		got.Hints = append(got.Hints, h)
	}
	return got
}
