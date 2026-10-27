// Package judge holds what the measurement asks of a model and how it reads the answer (R24): the
// judge's instructions, the request of each form, and the schema of the answer. The judge sees a
// finished sheet with the task's key and reference solution, as on the board (R19), and gives a
// verdict and, for a mistake, the box around the first one in pixels of the first picture.
//
// The four forms differ in what the judge is shown besides the picture (R11): nothing (A), the
// journal of the writing (B), the frames before erasures (C), or both (D). Their instructions are
// one text, and B, C and D add only the block that tells their inputs.
//
// The prompt comes in two versions, each with its own words and fingerprint (T64). Version 1 is
// that of E2, for sheets of lines, frozen after T27; version 2 is that of free writing, where a
// solution may have drawings and writing at the side, and its journal tells parts of the sheet.
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

// Version is a version of the judge's prompt.
type Version int

const (
	V1 Version = 1 // E2: sheets of lines, and the journal of lines (journal.Of)
	V2 Version = 2 // free writing (T64): drawings and writing at the side, and the journal of parts (journal.OfParts)
)

// Versions are the versions of the prompt in their order.
var Versions = []Version{V1, V2}

// prompt is what a version of the prompt has of its own: the text every form has, the blocks that
// tell the inputs of the forms, and the journal it shows, by its version, with its words.
type prompt struct {
	base, journalBlock, framesBlock string
	journal                         func(*sample.Sample) *journal.Journal
	journalVersion                  int
	journalWords                    func() []string
}

// prompt is the prompt of a version; ok is false for a version not in Versions.
func (v Version) prompt() (p prompt, ok bool) {
	switch v {
	case V1:
		return prompt{base: base, journalBlock: journalBlock, framesBlock: framesBlock, journal: journal.Of, journalVersion: 1, journalWords: journal.Words}, true
	case V2:
		return prompt{base: freeBase, journalBlock: partsBlock, framesBlock: freeFramesBlock, journal: journal.OfParts, journalVersion: 2, journalWords: journal.PartsWords}, true
	}
	return prompt{}, false
}

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
// tells. Form D joins the two blocks and says that the journal marks the frames.
var (
	journalBlock = fmt.Sprintf("You are also given the journal of how the sheet was written: its steps in the order the writer made them. A line that was erased whole has a number in it but is not on Image 1. A long pause is a stop of %d seconds or more. The journal shows how the solution went; judge the writing on Image 1. A mistake the writer erased and wrote again is not a mistake of the solution.", journal.Pause/1000)
	framesBlock  = "You are also given frames, the images after Image 1: each shows the sheet just before an erasure, with the writing the erasure then took away still on it. A frame shows how the solution went; what was erased is still not part of the solution, and the box is in pixels of Image 1. A mistake that a frame shows and Image 1 no longer has is no mistake: judge only what Image 1 shows."
	bothEnd      = "The journal says which step each frame stands just before."
)

// freeBase is the text every form of version 2 has, for free writing (T54): that of version 1,
// with what a free solution may hold and that all of it is part of the solution, and with the
// first mistake told in the order of the solution's steps, not of their place on the sheet, and
// boxed round the writing or the drawing. The places of the free sheets of a plan follow the order
// in which a solution is written and read (docs/free-sheets.md).
const freeBase = `You check a handwritten solution to a maths task. The writer has finished it: Image 1 shows the sheet as they left it.

1. Read the handwriting exactly as it is written. Do not correct anything: a wrong digit or sign stays wrong.
2. The solution may be written freely across the sheet, not only in lines from the top down. It may have drawings, such as a number line with jumps, a row of things or a scheme of segments; writing at the side; sums worked in a column, one number under another; and signs such as □ or △ that stand for unknown numbers. All of this is part of the solution, and a mistake in a drawing is a mistake as much as a wrong number is.
3. Writing that was erased is not part of the solution. Judge only what is left on the sheet.
4. Compare what is written and drawn with the task's correct answer and reference solution, which you are given. The reference solution tells its drawings in words. The writer may solve the task another way, with another drawing or with none; any correct way is correct.
5. Give your verdict:
   - "correct": the final answer is right and nothing written or drawn is wrong;
   - "error": something written or drawn is wrong;
   - "unreadable": you cannot read enough of the writing to judge it.
6. For "error", give the box around the first mistake: the writing or the drawing that is wrong. If more than one thing is wrong, box the one that comes first in the order of the solution's steps, wherever it stands on the sheet. A right step before the mistake is not the mistake: box the wrong writing or drawing itself, tightly, without the writing above or below it. Give the box in pixels of Image 1: x1, y1 is its top-left corner and x2, y2 its bottom-right, with x growing to the right and y downwards from the picture's top-left corner. For any other verdict, box is null.`

// The blocks of version 2. The journal of parts says itself what its parts are and how they are
// numbered (journal.OfParts); its block says what of it is not on Image 1 and what its places mean.
var (
	partsBlock      = fmt.Sprintf("You are also given the journal of how the sheet was written: its steps in the order the writer made them. Erased writing in it is not on Image 1, and neither are the strokes an erasure took from a part. The place the journal gives a part, such as top right, names the third of the height and the third of the width of Image 1 where the part stands. A long pause is a stop of %d seconds or more. The journal shows how the solution went; judge the writing and the drawings on Image 1. A mistake the writer erased and then wrote or drew again is not a mistake of the solution.", journal.Pause/1000)
	freeFramesBlock = "You are also given frames, the images after Image 1: each shows the sheet just before an erasure, with the writing or drawing the erasure then took away still on it. A frame shows how the solution went; what was erased is still not part of the solution, and the box is in pixels of Image 1. A mistake that a frame shows and Image 1 no longer has is no mistake: judge only what Image 1 shows."
)

// The labels of the pictures and the text of a request around the task.
const (
	finalLabel = "Image 1: the sheet as the writer left it."
	frameLabel = "Image %d: frame %d, an earlier state of the sheet, just before an erasure. What the erasure took away is not on Image 1 and is not part of the solution."
	textFormat = "Image 1 is %d pixels wide and %d pixels high.\nTask: %s\nCorrect answer: %s\nReference solution:\n%s"
)

// Fingerprint names a version of the prompt: the SHA-256 of every word a judge reads besides those
// of the sheet and its task — the instructions of the four forms, the labels of the pictures, the
// text round the task and the words of the journal. The log of a run keeps it, so that runs of one
// prompt can be told from runs of another: the prompt is frozen before the test (T27, T66). A
// version not in Versions has none: its fingerprint is empty.
func Fingerprint(v Version) string {
	p, ok := v.prompt()
	if !ok {
		return ""
	}
	words := []string{finalLabel, frameLabel, textFormat}
	for _, f := range Forms {
		words = append(words, f.Instructions(v))
	}
	words = append(words, p.journalWords()...)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(words, "\x00"))))
}

// Instructions are the instructions of a form in a version of the prompt: the text every form has,
// and the block of the form. A version not in Versions has none.
func (f Form) Instructions(v Version) string {
	p, ok := v.prompt()
	if !ok {
		return ""
	}
	switch f {
	case FormB:
		return p.base + "\n\n" + p.journalBlock
	case FormC:
		return p.base + "\n\n" + p.framesBlock
	case FormD:
		return p.base + "\n\n" + p.journalBlock + "\n\n" + p.framesBlock + "\n\n" + bothEnd
	}
	return p.base
}

// Sheet is what the forms show of one sheet in a version of the prompt: the PNG of the finished
// sheet and its size, the journal of the writing, of the version's kind, and the PNGs of the frames
// the journal chose, in its order. Prepare draws it from the sample, so that the picture, its size,
// the frames and the journal's marks agree.
type Sheet struct {
	Version       Version
	Final         []byte
	Width, Height int
	Journal       *journal.Journal
	Frames        [][]byte
}

// Prepare draws what the forms of a version of the prompt show of a sample that passes its check.
func Prepare(s *sample.Sample, v Version) (*Sheet, error) {
	p, ok := v.prompt()
	if !ok {
		return nil, fmt.Errorf("judge: no version %d of the prompt", v)
	}
	final, err := render.Final(s)
	if err != nil {
		return nil, err
	}
	sh := &Sheet{Version: v, Width: final.Bounds().Dx(), Height: final.Bounds().Dy(), Journal: p.journal(s)}
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

// Request is what form f asks of a model about a sheet of a task, in the sheet's version of the
// prompt: the instructions, the pictures each after its label, and the text: the size of Image 1,
// the task, its correct answer and reference solution, and for forms B and D the journal, which in
// D marks the frames. The model, its effort, whether it thinks and how many tokens it may write the
// caller sets. Forms C and D need frames: on a sheet without them the measurement does not ask C,
// and D is B (architecture 3.1). The sheet's journal must be of its version's kind.
func (f Form) Request(t *sample.Task, sh *Sheet) (provider.Request, error) {
	switch {
	case !slices.Contains(Forms, f):
		return provider.Request{}, fmt.Errorf("judge: no form %q", f)
	case sh.Journal == nil || len(sh.Frames) != len(sh.Journal.Frames):
		return provider.Request{}, errors.New("judge: the sheet's frames are not those of its journal: draw it with Prepare")
	case f.Frames() && len(sh.Frames) == 0:
		return provider.Request{}, fmt.Errorf("judge: form %s shows frames, and the sheet has none", f)
	}
	if p, ok := sh.Version.prompt(); !ok || sh.Journal.Version() != p.journalVersion {
		return provider.Request{}, fmt.Errorf("judge: the sheet's journal is not that of version %d of the prompt: draw the sheet with Prepare", sh.Version)
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
		Instructions: f.Instructions(sh.Version),
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
