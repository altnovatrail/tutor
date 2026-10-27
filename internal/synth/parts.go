package synth

import (
	"math/rand/v2"
	"strings"
	"unicode/utf8"

	"github.com/altnovatrail/tutor/internal/sample"
)

// A sheet of a plan of parts (T56) is written so that a device groups its strokes into its parts
// (R30 in docs/decisions.md): each part goes on without a pause to think — at most about 0.6 s
// between characters and 1.1 s between the rows of a column — and the next one comes after one,
// so that no group holds two parts and a part without its planned erasure is one group. A fix
// written after a pause may be a group of its own, as on a device.
const (
	// maxAdvance is the widest step from one character to the next, in sheet units: a colon before
	// a space then stays within a group's 50 units of the next character, which a larger hand of
	// E2's sheets does not keep (R30).
	maxAdvance = 32
	// partPause is the least pause before a part, in milliseconds: longer than a group's 2 s.
	partPause = 2100
	// sideGap is how far the notes at the side stand from the main column, and colGap how far apart
	// the columns of a row stand, in advances.
	sideGap = 2.0
	colGap  = 1.5
	// rowStep is the step from one row of a column to the next, of the height of a digit.
	rowStep = 1.3
)

// sheetOfParts draws a sheet of a plan of parts in the plan's order, each part where its place puts
// it: the main column on the left, top to bottom; the columns at the side in a row to its right,
// beside the part of the main column written before them, or at the top; a wide part across the
// sheet. A drawing is drawn thing by thing (figures.go). The planned erasure goes in the order the
// plan gives, as on a sheet of lines.
func sheetOfParts(sh *sample.PlanSheet, seed uint64) *sample.Sample {
	w := sheetWriter(sh, seed)
	h, main := w.partsHand(sh)
	w.pointMs = h.pointMs
	w.t = 800 + w.rng.Int64N(2000)
	spots := w.layout(h, sh, main)
	for i, p := range sh.Parts {
		if i > 0 {
			w.pause(partPause + w.rng.Int64N(2400))
			if w.rng.Float64() < 0.2 {
				w.pause(6000 + w.rng.Int64N(6000)) // a long stop, such as the journal names
			}
		}
		x, y := spots[i].x+w.jitter(h.height*0.3), spots[i].y+w.jitter(h.height*0.1)
		var ids []string
		switch {
		case p.Figure.Drawing() != "":
			ids = w.drawing(h, sh, i, spots[i].x, y) // across the place it was laid out for
		case p.Figure.Column != nil:
			ids = w.column(h, sh, i, x, y)
		case erasureIn(sh, i) != nil:
			ids = w.erasing(h, sh, p.Text, x, y)
		default:
			ids, _ = w.write(h, p.Text, x, y)
		}
		w.s.Lines = append(w.s.Lines, sample.Line{Text: p.Text, Strokes: ids, Answer: i+1 == sh.Answer})
	}
	return w.s
}

// sheetWriter is the writer of a plan sheet, its sample begun and its random numbers drawn from the
// seed and the sheet's number.
func sheetWriter(sh *sample.PlanSheet, seed uint64) *writer {
	v := sample.Version
	return &writer{
		s: &sample.Sample{
			Format:   sample.SampleFormat,
			Version:  &v,
			Sheet:    sample.Sheet{Width: sheetWidth, Height: sheetHeight},
			Task:     sh.Task,
			Writer:   sample.WriterSynthetic,
			Scenario: sh.Scenario,
			Strokes:  []sample.Stroke{},
			Erasures: []sample.Erasure{},
			Lines:    []sample.Line{},
			Expected: sh.Expected,
		},
		task: sh.Task,
		rng:  rand.New(rand.NewPCG(seed, uint64(sh.Number))),
	}
}

// partsHand picks the hand of a sheet of parts as newHand picks one, but with the advance at most
// maxAdvance, and so that the main column and the row of columns at its side fit across the sheet
// and the main column fits down it. A hand made smaller keeps its proportions. It returns the
// width of the main column too, in advances.
func (w *writer) partsHand(sh *sample.PlanSheet) (h hand, main float64) {
	h, spacing := w.pickHand(0.015, 60, 40)
	h.advance = min(float64(h.height*h.shape)*spacing, maxAdvance)
	main, side := widths(sh)
	room := float64(sheetWidth-30) - h.left
	if float64(h.advance*(main+side)) > room {
		h.advance = room / (main + side)
	}
	// A drawing of the main column takes up to drawingWidth units of it, beside any notes at the side.
	if side > 0 && drawsInMain(sh) {
		h.advance = min(h.advance, (room-drawingWidth)/side)
		main = max(main, drawingWidth/h.advance)
	}
	h.height = h.advance / float64(h.shape*spacing)
	h.step = h.height * (1.9 + float64(0.5*w.rng.Float64()))
	// The last part of the main column, its tails too, ends above the bottom of the sheet: the
	// drawings take their own height, and the lines share what is left.
	// If even lines a step of 1.6 heights apart do not fit, the hand is made smaller.
	for range 8 {
		lines, drawn := w.mainColumn(h, sh)
		room := float64(sheetHeight-20) - h.top - drawn
		if lines > 0 {
			h.step = min(h.step, room/float64(lines))
		}
		if lines == 0 || h.step >= float64(h.height*1.6) {
			break
		}
		h.height, h.advance, h.step = float64(h.height*0.9), float64(h.advance*0.9), float64(h.step*0.9)
	}
	return h, main
}

// mainColumn is what the main column takes down the sheet: how many of its steps a hand's step
// sets — one after each line but the last — and how much the rest takes, the drawings and the
// last part, in sheet units.
func (w *writer) mainColumn(h hand, sh *sample.PlanSheet) (steps int, rest float64) {
	last := -1
	for i, p := range sh.Parts {
		if p.Place != sample.PlaceRight {
			last = i
		}
	}
	for i, p := range sh.Parts {
		switch {
		case p.Place == sample.PlaceRight:
		case i == last && p.Figure.Drawing() == "":
			rest += noteHeight(h, p)
		case i == last:
			rest += w.sketch(h, sh, i, h.left, 0).height()
		case p.Figure.Drawing() != "" || p.Figure.Column != nil:
			rest += w.partHeight(h, sh, i)
		default:
			steps++
		}
	}
	return steps, rest
}

// widths are how wide, in advances, the widest text of the main column is, and the widest row of
// notes at the side with the gap before it, 0 when the sheet has none.
func widths(sh *sample.PlanSheet) (main, side float64) {
	row := 0.0
	for i, p := range sh.Parts {
		if p.Place != sample.PlaceRight {
			if p.Figure.Drawing() == "" {
				main = max(main, noteWidth(p, erasureIn(sh, i)))
			}
			row = 0
			continue
		}
		if row > 0 {
			row += colGap
		}
		row += noteWidth(p, erasureIn(sh, i))
		side = max(side, sideGap+row)
	}
	return main, side
}

// drawsInMain reports whether a drawing of a sheet stands in its main column, not across the sheet.
func drawsInMain(sh *sample.PlanSheet) bool {
	for _, p := range sh.Parts {
		if p.Figure.Drawing() != "" && p.Place == sample.PlaceLeft {
			return true
		}
	}
	return false
}

// textWidth is how wide a string is written, in advances, at the widest a hand writes it: a space
// is half an advance, and a character at most 1.1.
func textWidth(s string) float64 {
	width := 0.0
	for _, r := range s {
		if r == ' ' {
			width += 0.5
		} else {
			width += 1.1
		}
	}
	return width
}

// noteWidth is how wide a part of text is, in advances: its column, or its line. e is the planned
// erasure when it is in the part, else nil: what it takes away is written there first, and may be
// wider than what stays.
func noteWidth(p sample.PlanPart, e *sample.PlanErasure) float64 {
	if c := p.Figure.Column; c != nil {
		return float64(columnDigits(c, e)) + 0.3 // the bar reaches past the digits
	}
	width := textWidth(p.Text)
	switch {
	case e == nil:
	case e.PartOfLine:
		width = max(width, textWidth(strings.Replace(p.Text, e.Replacement, e.Text, 1)))
	default:
		width = max(width, textWidth(e.Text))
	}
	return width
}

// columnDigits is how many characters wide a column is: its widest number, the second one with the
// sign before it, and the number that the planned erasure e, when it is in the column, takes away,
// written first where its replacement then stands.
func columnDigits(c *sample.Column, e *sample.PlanErasure) int {
	n := max(utf8.RuneCountInString(c.Top), utf8.RuneCountInString(c.Bottom)+1, utf8.RuneCountInString(c.Result))
	if e != nil {
		wrong := utf8.RuneCountInString(e.Text)
		if e.Replacement == c.Bottom {
			wrong++ // the sign stands before it
		}
		n = max(n, wrong)
	}
	return n
}

// erasureIn is the planned erasure of a sheet when it is in its part i, from 0, else nil.
func erasureIn(sh *sample.PlanSheet, i int) *sample.PlanErasure {
	if e := sh.Erasure; e != nil && e.Line == i+1 {
		return e
	}
	return nil
}

// spot is where a part begins: the top left corner of its first character.
type spot struct{ x, y float64 }

// layout puts the parts of a sheet on it, as sheetOfParts says; main is the width of the main
// column, in advances. A row at the side stands below the rows at the side before it, and a wide
// part below them all, so that nothing is written over what is already there.
func (w *writer) layout(h hand, sh *sample.PlanSheet, main float64) []spot {
	sideX := h.left + float64(h.advance*(main+sideGap))
	spots := make([]spot, len(sh.Parts))
	y := h.top      // where the next part of the main column goes
	beside := h.top // the top of the part of the main column written last
	free := h.top   // below the rows at the side so far
	x, rowTop, inRow := sideX, h.top, false
	for i, p := range sh.Parts {
		if p.Place == sample.PlaceRight {
			if !inRow {
				// A row ends above the bottom of the sheet, whichever part it stands beside.
				x, rowTop, inRow = sideX, min(max(beside, free), float64(sheetHeight-20)-noteHeight(h, p)), true
			}
			spots[i] = spot{x, rowTop}
			x += float64(h.advance * (noteWidth(p, erasureIn(sh, i)) + colGap))
			free = max(free, rowTop+noteHeight(h, p)+float64(h.height*0.5))
			continue
		}
		inRow = false
		if p.Place == sample.PlaceWide {
			y = max(y, free)
		}
		spots[i] = spot{h.left, y}
		beside = y
		y += w.partHeight(h, sh, i)
		if p.Place == sample.PlaceWide {
			beside, free = y, max(free, y) // what comes after it stands below it
		}
	}
	return spots
}

// noteHeight is how tall a part of text is: its column, three rows and a bar, or its line.
func noteHeight(h hand, p sample.PlanPart) float64 {
	if p.Figure.Column != nil {
		return float64(h.height * (2*rowStep + 1.1))
	}
	return float64(h.height * 1.35)
}

// partHeight is how far down the next part of the main column stands from the top of this one: a
// step of the hand for a line, and for a drawing or a column its height and half a digit's.
func (w *writer) partHeight(h hand, sh *sample.PlanSheet, i int) float64 {
	p := sh.Parts[i]
	if p.Figure.Drawing() != "" {
		return w.sketch(h, sh, i, h.left, 0).height() + float64(h.height*0.5)
	}
	return max(h.step, noteHeight(h, p)+float64(h.height*0.5))
}

// column writes a calculation in a column with its top left corner at (x, y), as the author wrote
// it in the pilot: the first number, under it the sign and the second number, a bar, and the
// result under the bar, the numbers to the column's right edge and the sign at its left. With the
// part's planned erasure, the number the plan writes again goes first as it was, is erased in one
// move of the eraser and is written again in its place; the result comes after it.
func (w *writer) column(h hand, sh *sample.PlanSheet, part int, x, y float64) []string {
	c := sh.Parts[part].Figure.Column
	e := erasureIn(sh, part)
	width := float64(columnDigits(c, e))
	right := x + float64(h.advance*width)
	row := float64(h.height * rowStep)
	var ids []string
	number := func(s string, top float64) {
		at := func(s string) float64 { return right - float64(h.advance*float64(utf8.RuneCountInString(s))) }
		if e != nil && s == e.Replacement {
			wrong, _ := w.write(h, e.Text, at(e.Text), top)
			w.pause(1500 + w.rng.Int64N(3500)) // the writer sees what is to go
			w.s.Erasures = append(w.s.Erasures, sample.Erasure{
				TMs:     w.t,
				Strokes: wrong,
				Planned: &sample.PlannedErasure{Line: e.Line, Erased: e.Erased, Text: e.Text},
			})
			w.pause(400 + w.rng.Int64N(1100))
			e = nil // one number of three is written again, once
		}
		in, _ := w.write(h, s, at(s), top)
		ids = append(ids, in...)
		w.pause(150 + w.rng.Int64N(300))
	}
	number(c.Top, y)
	sign, _ := w.write(h, c.Op, right-float64(h.advance*width), y+row)
	ids = append(ids, sign...)
	w.pause(100 + w.rng.Int64N(200))
	number(c.Bottom, y+row)
	bar := y + float64(2*row) - float64(h.height*0.15)
	ids = append(ids, w.stroke([]pt{
		{right - float64(h.advance*(width+0.2)) + w.jitter(2), bar + w.jitter(1.5)},
		{right + float64(h.advance*0.15) + w.jitter(2), bar + w.jitter(1.5)},
	}))
	w.pause(200 + w.rng.Int64N(300))
	number(c.Result, y+float64(2*row)+float64(h.height*0.1))
	return ids
}
