package tutor

import (
	"context"
	"errors"
	"fmt"

	"github.com/altnovatrail/tutor/admission"
	"github.com/altnovatrail/tutor/internal/grownup"
	"github.com/altnovatrail/tutor/provider"
)

// SheetStroke is a stroke the crop of the work shows, as the drawing reads it: its ID on its sheet,
// and its points across and down in pixels of the crop of the work, in pairs x1, y1, x2, y2, …, as
// app thinned them (SheetStroke of spec's contract).
type SheetStroke struct {
	ID     int32
	Points []float32
}

// TutorStroke is a stroke the tutor writes on the sheet in grown-up mode: its points across and
// down in pixels of the crop of the work, in pairs, at least two points; its colour, #rrggbb; and
// the width of its line in pixels of the crop (TutorStroke of spec's contract).
type TutorStroke struct {
	Points []float32
	Color  string
	Width  float32
}

// GrownUpRequest is a request of a grown-up in grown-up mode on a task from paper:
//   - Question is the request as the grown-up sent it, not blank, at most 500 code points;
//   - Turns are the conversation on the task before it, oldest first, at most 8, each at most 1000
//     code points, Learner marking the grown-up's own turns;
//   - Statement is the task's crop without the learner's strokes, a JPEG or a PNG
//     (PaperWork.statement), and Work the PNG of the crop of the work (PaperWork.crop), in whose
//     pixels every point is;
//   - Strokes are the strokes the work shows, in the order of the sheet, at most 3000, each with at
//     most 128 points;
//   - Language is the language of the screens, by its tag, the words' language.
//
// All of it is data the model reads, never instructions, and nothing of it names the learner or
// the grown-up.
type GrownUpRequest struct {
	Route     Route
	Question  string
	Turns     []PaperTurn
	Statement []byte
	Work      []byte
	Strokes   []SheetStroke
	Language  string
	// PixelsPerMM is how many pixels of Work make a millimetre of the task's page; 0 when unknown.
	PixelsPerMM float32
}

// GrownUpAnswer is what the drawing gives when Outcome is Answered: the tutor's words about what it
// did, the strokes to write in the order to draw them, and the IDs of the strokes to take off, each
// one the request named.
type GrownUpAnswer struct {
	Words   string
	Strokes []TutorStroke
	Erase   []int32
	Outcome Outcome
}

// GrownUp asks the admitted models of the drawing (CallDrawing), one at a time, a later one only
// when the one before failed, called no tool, gave an act off the schema, or its act will not do:
// a point beyond the work, an ID to take off that the request did not name, a mark that lacks what
// its kind needs, a character the hand does not write, or nothing done at all (grownup.Drawn.Did).
// The last act that answered stands, put right where it can be — a point moved onto the work, the
// IDs not named and the characters not written left out —, and one that does nothing at all fails
// the call. The task's crop goes as a photo does (provider.Photo), fitted to each model, and the
// work as the PNG it is, never shrunk, so that its pixels are those of the strokes. A request
// spec's contract refuses, a crop that is no picture and a work that is no PNG fail before any
// call.
func (t *Tutor) GrownUp(ctx context.Context, g GrownUpRequest) GrownUpAnswer {
	fail := func(err error) GrownUpAnswer { return GrownUpAnswer{Outcome: Outcome{Kind: Failed, Err: err}} }
	if len(g.Statement) == 0 {
		return fail(errors.New("tutor: the drawing has no crop of the task"))
	}
	work, err := cropSize(g.Work)
	if err != nil {
		return fail(err)
	}
	p := grownup.Prompt{Work: grownup.Picture{Image: provider.Image{PNG: g.Work}, Width: work.Width, Height: work.Height},
		Request: g.Question, Language: g.Language, PixelsPerMM: float64(g.PixelsPerMM)}
	named := make([]int32, 0, len(g.Strokes))
	for _, s := range g.Strokes {
		p.Strokes = append(p.Strokes, grownup.Stroke{ID: s.ID, Points: s.Points})
		named = append(named, s.ID)
	}
	for _, turn := range g.Turns {
		p.Turns = append(p.Turns, grownup.Turn{GrownUp: turn.Learner, Text: turn.Text})
	}
	if err := grownup.Check(p); err != nil {
		return fail(fmt.Errorf("tutor: %w", err))
	}
	calls, a, refused := t.prepare(CallDrawing, g.Route)
	if refused != nil {
		return GrownUpAnswer{Outcome: *refused}
	}
	statements := provider.NewPhotos(g.Statement)
	got, out := cascade(ctx, calls, func(cl call) (grownup.Drawn, Outcome) {
		return drawBy(ctx, a, cl, p, statements, named)
	}, grownup.Drawn.Did)
	if out.Kind != Answered {
		return GrownUpAnswer{Outcome: out}
	}
	if len(got.Lines) == 0 && len(got.Erase) == 0 && got.Say == "" {
		out.Kind, out.Err = Failed, errors.New("tutor: no model's act did anything on the sheet or said anything")
		return GrownUpAnswer{Outcome: out}
	}
	answer := GrownUpAnswer{Words: got.Say, Erase: got.Erase, Outcome: out}
	for _, l := range got.Lines {
		answer.Strokes = append(answer.Strokes, TutorStroke{Points: l.Points, Color: l.Color, Width: l.Width})
	}
	return answer
}

// drawBy is one attempt of the drawing, by one model: the task's crop fitted to it, the request,
// and the act read and put on the work.
func drawBy(ctx context.Context, a admission.Answer, cl call, p grownup.Prompt, statements *provider.Photos, named []int32) (grownup.Drawn, Outcome) {
	fail := func(err error) (grownup.Drawn, Outcome) {
		return grownup.Drawn{}, Outcome{Kind: Failed, Admission: a, Err: err}
	}
	crop, width, height, err := statements.For(cl.client)
	if err != nil {
		return fail(fmt.Errorf("tutor: the task's crop: %w", err))
	}
	p.Statement = grownup.Picture{Image: crop, Width: width, Height: height}
	req, err := grownup.RequestOf(p)
	if err != nil {
		return fail(fmt.Errorf("tutor: %w", err))
	}
	req.MaxOutputTokens = grownup.DefaultMaxOutputTokens
	if err := checkTextWithin(req, grownup.MaxTextBytes); err != nil {
		return fail(err)
	}
	resps, out := ask(ctx, a, []call{cl}, []provider.Request{req}, kinds[CallDrawing].maxIn)
	if out.Kind != Answered {
		return grownup.Drawn{}, out
	}
	act, err := grownup.Read(resps[0].JSON)
	if err != nil {
		out.Kind, out.Err = Failed, fmt.Errorf("tutor: %s: %w", cl.model.Name(), err)
		return grownup.Drawn{}, out
	}
	return grownup.Draw(act, p.Work.Width, p.Work.Height, named), out
}
