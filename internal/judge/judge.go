// Package judge holds what the measurement asks of a model and how it reads the answer (R24): the
// judge's instructions, the request of each form, and the schema of the answer. The judge sees a
// finished sheet with the task's key and reference solution, as on the board (R19), and gives a
// verdict and, for a mistake, the box around the first one in pixels of the first picture.
//
// The four forms differ in what the judge is shown besides the picture (R11): nothing (A), the
// journal of the writing (B), the frames before erasures (C), or both (D). Their instructions are
// one text, and B, C and D add only the block that tells their inputs.
package judge

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/altnovatrail/tutor/internal/journal"
	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/provider"
)

// Form is a form of the measurement: what the judge is shown besides the picture of the sheet.
type Form string

const (
	FormA Form = "A" // the picture alone
	FormB Form = "B" // the picture and the journal of the writing
	FormC Form = "C" // the picture and the frames before erasures
	FormD Form = "D" // the picture, the journal and the frames
)

// Forms are the forms in their order.
var Forms = []Form{FormA, FormB, FormC, FormD}

// Journal reports whether the form shows the journal, and Frames whether it shows the frames.
func (f Form) Journal() bool { return f == FormB || f == FormD }
func (f Form) Frames() bool  { return f == FormC || f == FormD }

// The verdicts a judge may give of a finished sheet.
const (
	Correct    = "correct"
	Error      = "error"
	Unreadable = "unreadable"
)

// Verdicts are the verdicts in the order of the schema.
var Verdicts = []string{Correct, Error, Unreadable}

// base is the text of the instructions every form has: the board's (internal/board), for a sheet
// the writer has finished. Erased writing is not part of the solution in any form. It does not
// share the board's text: the measurement's prompt is frozen with its run (T27), and a change to
// the board's must not change it.
const base = `You check a handwritten solution to a maths task. The writer has finished it: Image 1 shows the sheet as they left it.

1. Read the handwriting exactly as it is written. Do not correct anything: a wrong digit or sign stays wrong.
2. Writing that was erased is not part of the solution. Judge only what is left on the sheet.
3. Compare what is written with the task's correct answer and reference solution, which you are given. The writer may solve the task another way; any correct way is correct.
4. Give your verdict:
   - "correct": the final answer is right and nothing written is wrong;
   - "error": something written is wrong;
   - "unreadable": you cannot read enough of the writing to judge it.
5. For "error", give the box around the first mistake, the writing where the solution first goes wrong, in pixels of Image 1: x1, y1 is its top-left corner and x2, y2 its bottom-right, with x growing to the right and y downwards from the picture's top-left corner. For any other verdict, box is null.`

// The blocks that tell the inputs of a form, after the text every form has. The journal says
// itself how its lines are numbered (journal.Journal.Text); its block names the pause the journal
// tells.
var (
	journalBlock = fmt.Sprintf("You are also given the journal of how the sheet was written: its steps in the order the writer made them. A line that was erased whole has a number in it but is not on Image 1. A long pause is a stop of %d seconds or more. The journal shows how the solution went; judge the writing on Image 1. A mistake the writer erased and wrote again is not a mistake of the solution.", journal.Pause/1000)
	framesBlock  = "You are also given frames, the images after Image 1: each shows the sheet just before an erasure, with the writing the erasure then took away still on it. A frame shows how the solution went; what was erased is still not part of the solution, and the box is in pixels of Image 1. A mistake that a frame shows and Image 1 no longer has is no mistake: judge only what Image 1 shows."
	bothBlock    = journalBlock + "\n\n" + framesBlock + "\n\nThe journal says which step each frame stands just before."
)

// The labels of the pictures and the text of a request around the task.
const (
	finalLabel = "Image 1: the sheet as the writer left it."
	frameLabel = "Image %d: frame %d, an earlier state of the sheet, just before an erasure. What the erasure took away is not on Image 1 and is not part of the solution."
	textFormat = "Image 1 is %d pixels wide and %d pixels high.\nTask: %s\nCorrect answer: %s\nReference solution:\n%s"
)

// Fingerprint names the prompt of the measurement: the SHA-256 of every word a judge reads besides
// those of the sheet and its task — the instructions of the four forms, the labels of the pictures,
// the text round the task and the words of the journal. The log of a run keeps it, so that runs of
// one prompt can be told from runs of another: the prompt is frozen before the test (T27).
func Fingerprint() string {
	words := []string{finalLabel, frameLabel, textFormat}
	for _, f := range Forms {
		words = append(words, f.Instructions())
	}
	words = append(words, journal.Words()...)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(words, "\x00"))))
}

// Instructions are the instructions of a form: the text every form has, and the block of the form.
func (f Form) Instructions() string {
	switch f {
	case FormB:
		return base + "\n\n" + journalBlock
	case FormC:
		return base + "\n\n" + framesBlock
	case FormD:
		return base + "\n\n" + bothBlock
	}
	return base
}

// Sheet is what the forms show of one sheet: the PNG of the finished sheet and its size, the
// journal of the writing, and the PNGs of the frames the journal chose, in its order. Prepare draws
// it from the sample, so that the picture, its size, the frames and the journal's marks agree.
type Sheet struct {
	Final         []byte
	Width, Height int
	Journal       *journal.Journal
	Frames        [][]byte
}

// Prepare draws what the forms show of a sample that passes its check.
func Prepare(s *sample.Sample) (*Sheet, error) {
	final, err := render.Final(s)
	if err != nil {
		return nil, err
	}
	sh := &Sheet{Width: final.Bounds().Dx(), Height: final.Bounds().Dy(), Journal: journal.Of(s)}
	if sh.Final, err = render.PNG(final); err != nil {
		return nil, err
	}
	for _, f := range sh.Journal.Frames {
		img, err := render.Before(s, f.Erasure)
		if err != nil {
			return nil, err
		}
		png, err := render.PNG(img)
		if err != nil {
			return nil, err
		}
		sh.Frames = append(sh.Frames, png)
	}
	return sh, nil
}

// Request is what form f asks of a model about a sheet of a task: the instructions, the pictures
// each after its label, and the text: the size of Image 1, the task, its correct answer and
// reference solution, and for forms B and D the journal, which in D marks the frames. The model,
// its effort, whether it thinks and how many tokens it may write the caller sets. Forms C and D
// need frames: on a sheet without them the measurement does not ask C, and D is B
// (architecture 3.1).
func (f Form) Request(t *sample.Task, sh *Sheet) (provider.Request, error) {
	switch {
	case !slices.Contains(Forms, f):
		return provider.Request{}, fmt.Errorf("judge: no form %q", f)
	case sh.Journal == nil || len(sh.Frames) != len(sh.Journal.Frames):
		return provider.Request{}, errors.New("judge: the sheet's frames are not those of its journal: draw it with Prepare")
	case f.Frames() && len(sh.Frames) == 0:
		return provider.Request{}, fmt.Errorf("judge: form %s shows frames, and the sheet has none", f)
	}
	images := []provider.Image{{Label: finalLabel, PNG: sh.Final}}
	if f.Frames() {
		for i, png := range sh.Frames {
			images = append(images, provider.Image{
				Label: fmt.Sprintf(frameLabel, i+2, i+1),
				PNG:   png,
			})
		}
	}
	text := fmt.Sprintf(textFormat, sh.Width, sh.Height, t.Statement, t.Key, strings.Join(t.Solution, "\n"))
	if f.Journal() {
		text += "\n\n" + strings.TrimRight(sh.Journal.Text(f == FormD), "\n")
	}
	return provider.Request{
		Instructions: f.Instructions(),
		Images:       images,
		Text:         text,
		SchemaName:   "verdict",
		Schema:       Schema,
	}, nil
}

// Schema is the answer every judge returns. Its properties are written as JSON, as on the board:
// the SDKs encode a map's keys in sorted order, and a model writes the fields in the order of the
// schema, the verdict first.
var Schema = map[string]any{
	"type":                 "object",
	"additionalProperties": false,
	"required":             []string{"verdict", "box"},
	"properties": json.RawMessage(`{
		"verdict": {"type": "string", "enum": ["correct", "error", "unreadable"]},
		"box": {"anyOf": [
			{"type": "null"},
			{"type": "object", "additionalProperties": false, "required": ["x1", "y1", "x2", "y2"], "properties": {
				"x1": {"type": "integer"}, "y1": {"type": "integer"}, "x2": {"type": "integer"}, "y2": {"type": "integer"}
			}}
		]}
	}`),
}

// Answer is what a judge said: the verdict and, for a mistake, the box around it, in pixels of
// Image 1.
type Answer struct {
	Verdict string `json:"verdict"`
	Box     *Box   `json:"box"`
}

// Box is a box in pixels of a picture: x1, y1 its top-left corner and x2, y2 its bottom-right. It
// may reach past the picture; the line it hits counts the points inside it.
type Box struct {
	X1 int `json:"x1"`
	Y1 int `json:"y1"`
	X2 int `json:"x2"`
	Y2 int `json:"y2"`
}

// Read takes a judge's answer apart as strictly as the schema: the verdict and the box, both of
// them and nothing else, the verdict one of the schema's and the box null or four corners. Only an
// "error" keeps its box, its corners put in order; an "error" without a box stands, with no place
// for the mistake. The answer as the model gave it is the run's to keep.
func Read(answer string) (*Answer, error) {
	var raw struct {
		Verdict string          `json:"verdict"`
		Box     json.RawMessage `json:"box"`
	}
	if err := strict([]byte(answer), &raw); err != nil {
		return nil, fmt.Errorf("judge: the answer is off the schema: %w", err)
	}
	if !slices.Contains(Verdicts, raw.Verdict) {
		return nil, fmt.Errorf("judge: no such verdict: %q", raw.Verdict)
	}
	if raw.Box == nil {
		return nil, errors.New("judge: the answer has no box, not even null")
	}
	a := &Answer{Verdict: raw.Verdict}
	if string(raw.Box) == "null" {
		return a, nil
	}
	var b struct {
		X1 *int `json:"x1"`
		Y1 *int `json:"y1"`
		X2 *int `json:"x2"`
		Y2 *int `json:"y2"`
	}
	if err := strict(raw.Box, &b); err != nil {
		return nil, fmt.Errorf("judge: the box is off the schema: %w", err)
	}
	if b.X1 == nil || b.Y1 == nil || b.X2 == nil || b.Y2 == nil {
		return nil, errors.New("judge: the box lacks a corner")
	}
	if a.Verdict == Error {
		a.Box = &Box{X1: min(*b.X1, *b.X2), Y1: min(*b.Y1, *b.Y2), X2: max(*b.X1, *b.X2), Y2: max(*b.Y1, *b.Y2)}
	}
	return a, nil
}

// Margin is how far round a box the points of a line still count, in pixels (architecture 3.1).
const Margin = 8

// Hit is the line of a sample's markup a box hits: the line with the most points inside the box
// and Margin round it, of the points still on the sheet, by its number. It is 0 when no line has
// more points there than every other: none, or two with as many.
func Hit(s *sample.Sample, b Box) int {
	best, most, tie := 0, 0, false
	for i, points := range linePoints(s) {
		n := 0
		for _, p := range points {
			if p.X >= float64(b.X1-Margin) && p.X <= float64(b.X2+Margin) && p.Y >= float64(b.Y1-Margin) && p.Y <= float64(b.Y2+Margin) {
				n++
			}
		}
		switch {
		case n > most:
			best, most, tie = i+1, n, false
		case n == most && n > 0:
			tie = true
		}
	}
	if tie {
		return 0
	}
	return best
}

// Around is the box round the points of a line of a sample's markup, by its number: the box a judge
// that is always right gives for a mistake on that line. A line without points has none.
func Around(s *sample.Sample, line int) *Box {
	var b *Box
	for _, p := range linePoints(s)[line-1] {
		x, y := int(p.X), int(p.Y)
		if b == nil {
			b = &Box{X1: x, Y1: y, X2: x, Y2: y}
		}
		b.X1, b.Y1, b.X2, b.Y2 = min(b.X1, x), min(b.Y1, y), max(b.X2, x+1), max(b.Y2, y+1)
	}
	return b
}

// linePoints are the points of each line of a sample's markup, which are all on the sheet.
func linePoints(s *sample.Sample) [][]sample.Point {
	byID := make(map[string][]sample.Point, len(s.Strokes))
	for _, st := range s.Strokes {
		byID[st.ID] = st.Points
	}
	out := make([][]sample.Point, len(s.Lines))
	for i, l := range s.Lines {
		for _, id := range l.Strokes {
			out[i] = append(out[i], byID[id]...)
		}
	}
	return out
}

// strict decodes one JSON object with no field it does not know.
func strict(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
