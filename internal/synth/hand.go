package synth

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"unicode/utf8"

	"github.com/altnovatrail/tutor/internal/sample"
)

// letters are the Cyrillic letters the lines of the measurement's plan hold, such as «Ответ: 5
// столбов», in the box of the font: a capital takes the height of a digit, a small letter its lower
// part, from 0.45 down, and a tail goes below the line, down to 1.35.
var letters = map[rune]glyph{
	'О': {{{0.5, 0}, {0.1, 0.2}, {0.02, 0.5}, {0.1, 0.8}, {0.5, 1}, {0.9, 0.8}, {0.98, 0.5}, {0.9, 0.2}, {0.5, 0}}},
	'П': {{{0.1, 1}, {0.1, 0}, {0.9, 0}, {0.9, 1}}},
	'а': {{{0.85, 0.6}, {0.6, 0.45}, {0.25, 0.5}, {0.1, 0.75}, {0.3, 1}, {0.6, 0.95}, {0.85, 0.75}}, {{0.85, 0.45}, {0.85, 0.9}, {0.95, 1}}},
	'б': {{{0.85, 0.05}, {0.4, 0.05}, {0.2, 0.3}, {0.12, 0.7}, {0.3, 1}, {0.7, 0.97}, {0.85, 0.75}, {0.6, 0.5}, {0.2, 0.6}}},
	'в': {{{0.15, 0.45}, {0.15, 1}, {0.65, 1}, {0.85, 0.88}, {0.65, 0.72}, {0.15, 0.72}}, {{0.15, 0.45}, {0.6, 0.45}, {0.75, 0.58}, {0.6, 0.72}}},
	'е': {{{0.15, 0.72}, {0.85, 0.72}, {0.75, 0.5}, {0.45, 0.45}, {0.15, 0.6}, {0.15, 0.85}, {0.45, 1}, {0.85, 0.92}}},
	'и': {{{0.15, 0.45}, {0.15, 1}, {0.85, 0.45}, {0.85, 1}}},
	'к': {{{0.15, 0.45}, {0.15, 1}}, {{0.8, 0.45}, {0.2, 0.73}, {0.85, 1}}},
	'л': {{{0.05, 1}, {0.25, 0.9}, {0.4, 0.45}, {0.85, 0.45}, {0.85, 1}}},
	'м': {{{0.08, 1}, {0.18, 0.45}, {0.5, 0.85}, {0.82, 0.45}, {0.92, 1}}},
	'н': {{{0.15, 0.45}, {0.15, 1}}, {{0.15, 0.72}, {0.85, 0.72}}, {{0.85, 0.45}, {0.85, 1}}},
	'о': {{{0.5, 0.45}, {0.15, 0.58}, {0.1, 0.73}, {0.15, 0.9}, {0.5, 1}, {0.85, 0.9}, {0.9, 0.73}, {0.85, 0.58}, {0.5, 0.45}}},
	'р': {{{0.15, 1.35}, {0.15, 0.45}, {0.6, 0.45}, {0.85, 0.6}, {0.6, 0.75}, {0.15, 0.75}}},
	'с': {{{0.85, 0.52}, {0.5, 0.45}, {0.15, 0.6}, {0.12, 0.82}, {0.45, 1}, {0.85, 0.93}}},
	'т': {{{0.08, 0.45}, {0.92, 0.45}}, {{0.5, 0.45}, {0.5, 1}}},
	'у': {{{0.12, 0.45}, {0.5, 0.95}}, {{0.88, 0.45}, {0.35, 1.35}, {0.15, 1.3}}},
}

// hand is how one synthetic sheet is written: the size and the slant of the writing, where its
// lines stand, how fast the pen moves and how long the writer stops. Each sheet draws its own, so
// that the sheets of a set do not all look alike.
type hand struct {
	height  float64 // of a digit
	shape   float64 // the width of a character, of its height
	advance float64 // from one character to the next
	slant   float64 // how far the top of a character leans to the right, of its height
	tilt    float64 // how much a line climbs or falls, per unit to the right
	left    float64 // where the lines start
	top     float64 // where the first line starts
	step    float64 // from one line to the next
	pointMs int64   // between two points of a stroke
}

// Sheet draws a plan sheet as a writer would write it, from the seed; each plan sheet draws its
// own hand from it, and the same seed draws the same sheet again: its lines top to bottom in
// the plan's order and, on a sheet of an erasure scenario, the planned erasure in the order the
// plan gives — the lines before it, the writing to erase, one move of the eraser, what is written
// in its place and the rest of the lines. Its markup is the plan's, and its writer is "synthetic".
// It panics on a plan sheet the plan's check refuses, such as one without lines.
func Sheet(sh *sample.PlanSheet, seed uint64) *sample.Sample {
	if len(sh.Lines) == 0 {
		panic(fmt.Sprintf("synth: plan sheet %d has no lines", sh.Number))
	}
	v := sample.Version
	w := &writer{
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
	h := w.newHand(sh)
	w.pointMs = h.pointMs
	w.t = 800 + w.rng.Int64N(2000)

	for i, text := range sh.Lines {
		y := h.top + float64(i)*h.step + w.jitter(h.height*0.15)
		x := h.left + w.jitter(h.height*0.4)
		if i > 0 {
			w.pause(1200 + w.rng.Int64N(3300))
			if w.rng.Float64() < 0.2 {
				w.pause(6000 + w.rng.Int64N(6000)) // a long stop, such as the journal names
			}
		}
		var ids []string
		if e := sh.Erasure; e != nil && e.Line == i+1 {
			ids = w.erasing(h, sh, text, x, y)
		} else {
			ids, _ = w.write(h, text, x, y)
		}
		w.s.Lines = append(w.s.Lines, sample.Line{Text: text, Strokes: ids, Answer: i+1 == sh.Answer})
	}
	return w.s
}

// newHand picks the hand of a sheet, so that its longest line fits across the sheet and its lines
// fit down it.
func (w *writer) newHand(sh *sample.PlanSheet) hand {
	h := hand{
		height:  32 + 14*w.rng.Float64(),
		shape:   0.5 + 0.12*w.rng.Float64(),
		slant:   -0.05 + 0.3*w.rng.Float64(),
		tilt:    w.jitter(0.025),
		left:    40 + 120*w.rng.Float64(),
		top:     40 + 50*w.rng.Float64(),
		pointMs: 6 + w.rng.Int64N(6),
	}
	longest := 0
	for _, l := range sh.Lines {
		longest = max(longest, utf8.RuneCountInString(l))
	}
	spacing := 1.25 + 0.15*w.rng.Float64() // the advance, of the width of a character
	if room := float64(sheetWidth-40) - h.left; float64(longest)*h.height*h.shape*spacing > room {
		h.height = room / (float64(longest) * h.shape * spacing)
	}
	h.advance = h.height * h.shape * spacing
	h.step = h.height * (1.9 + 0.7*w.rng.Float64())
	if n := len(sh.Lines); n > 1 {
		// The last line, its tails too, ends above the bottom of the sheet.
		h.step = min(h.step, (float64(sheetHeight-20)-h.top-1.5*h.height)/float64(n-1))
	}
	return h
}

// erasing writes the line of a planned erasure: the erased writing, one move of the eraser over it
// and the line's writing in its place. When the erasure takes a part of the line, the line is
// written up to that part, and its rest after what is written in the part's place. It returns the
// strokes of the line that stay.
func (w *writer) erasing(h hand, sh *sample.PlanSheet, line string, x, y float64) []string {
	e := sh.Erasure
	before, after := "", ""
	if e.PartOfLine {
		at := strings.Index(line, e.Replacement)
		before, after = line[:at], line[at+len(e.Replacement):]
	}
	ids, x := w.write(h, before, x, y)
	erased, _ := w.write(h, e.Text, x, y)
	w.pause(1500 + w.rng.Int64N(3500)) // the writer sees what is to go
	w.s.Erasures = append(w.s.Erasures, sample.Erasure{
		TMs:     w.t,
		Strokes: erased,
		Planned: &sample.PlannedErasure{Line: e.Line, Erased: e.Erased, Text: e.Text},
	})
	w.pause(400 + w.rng.Int64N(1100))
	in, x := w.write(h, e.Replacement, x, y)
	ids = append(ids, in...)
	rest, _ := w.write(h, after, x, y)
	return append(ids, rest...)
}

// write writes a string in the sheet's hand with the top left corner of its first character at
// (x, y), and returns the IDs of its strokes and where the next character would start. The
// characters wander a little in size and height, and the line climbs or falls with the hand's
// tilt.
func (w *writer) write(h hand, s string, x, y float64) ([]string, float64) {
	var ids []string
	drift := 0.0
	for _, r := range s {
		if r == ' ' {
			x += h.advance * 0.5
			continue
		}
		g, ok := font[r]
		if !ok {
			g, ok = letters[r]
		}
		if !ok {
			g = squiggle
		}
		size := h.height * (0.92 + 0.16*w.rng.Float64())
		drift = clamp(drift+w.jitter(h.height*0.04), -h.height*0.12, h.height*0.12)
		top := y + drift + h.tilt*(x-h.left) + (h.height - size)
		for _, poly := range g {
			points := make([]pt, len(poly))
			for i, p := range poly {
				points[i] = pt{
					x + float64(p.x*size*h.shape) + float64(h.slant*(1-p.y)*size) + w.jitter(size*0.03),
					top + float64(p.y*size) + w.jitter(size*0.03),
				}
			}
			ids = append(ids, w.stroke(points))
			w.pause(60 + w.rng.Int64N(200))
		}
		w.pause(90 + w.rng.Int64N(260))
		x += h.advance * (0.9 + 0.2*w.rng.Float64())
	}
	return ids, x
}
