package synth

import (
	"fmt"
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
	// The rest of the small letters, so that no word of a plan comes out a squiggle, and the signs
	// the author wrote for the unknowns in the pilot, a square and a triangle (T54).
	'г': {{{0.2, 1}, {0.2, 0.45}, {0.85, 0.45}}},
	'д': {{{0.05, 1.2}, {0.05, 0.95}, {0.95, 0.95}, {0.95, 1.2}}, {{0.15, 0.95}, {0.32, 0.45}, {0.8, 0.45}, {0.8, 0.95}}},
	'ё': {{{0.15, 0.72}, {0.85, 0.72}, {0.75, 0.5}, {0.45, 0.45}, {0.15, 0.6}, {0.15, 0.85}, {0.45, 1}, {0.85, 0.92}}, {{0.33, 0.3}}, {{0.67, 0.3}}},
	'ж': {{{0.05, 0.45}, {0.38, 0.72}, {0.05, 1}}, {{0.5, 0.45}, {0.5, 1}}, {{0.95, 0.45}, {0.62, 0.72}, {0.95, 1}}},
	'з': {{{0.15, 0.52}, {0.5, 0.45}, {0.8, 0.56}, {0.5, 0.72}, {0.85, 0.86}, {0.5, 1}, {0.12, 0.94}}},
	'й': {{{0.15, 0.45}, {0.15, 1}, {0.85, 0.45}, {0.85, 1}}, {{0.32, 0.28}, {0.5, 0.36}, {0.68, 0.28}}},
	'п': {{{0.15, 1}, {0.15, 0.45}, {0.85, 0.45}, {0.85, 1}}},
	'ф': {{{0.5, 0.1}, {0.5, 1.32}}, {{0.5, 0.5}, {0.2, 0.55}, {0.08, 0.72}, {0.2, 0.92}, {0.5, 0.97}, {0.8, 0.92}, {0.92, 0.72}, {0.8, 0.55}, {0.5, 0.5}}},
	'х': {{{0.1, 0.45}, {0.9, 1}}, {{0.9, 0.45}, {0.1, 1}}},
	'ц': {{{0.15, 0.45}, {0.15, 0.95}, {0.8, 0.95}, {0.8, 0.45}}, {{0.8, 0.95}, {0.95, 0.95}, {0.95, 1.2}}},
	'ч': {{{0.15, 0.45}, {0.15, 0.68}, {0.4, 0.78}, {0.85, 0.72}}, {{0.85, 0.45}, {0.85, 1}}},
	'ш': {{{0.08, 0.45}, {0.08, 1}, {0.92, 1}, {0.92, 0.45}}, {{0.5, 0.45}, {0.5, 1}}},
	'щ': {{{0.08, 0.45}, {0.08, 0.95}, {0.85, 0.95}, {0.85, 0.45}}, {{0.46, 0.45}, {0.46, 0.95}}, {{0.85, 0.95}, {0.97, 0.95}, {0.97, 1.2}}},
	'ъ': {{{0.05, 0.45}, {0.3, 0.45}, {0.3, 1}, {0.75, 1}, {0.9, 0.88}, {0.75, 0.72}, {0.3, 0.72}}},
	'ы': {{{0.1, 0.45}, {0.1, 1}, {0.45, 1}, {0.6, 0.88}, {0.45, 0.72}, {0.1, 0.72}}, {{0.85, 0.45}, {0.85, 1}}},
	'ь': {{{0.2, 0.45}, {0.2, 1}, {0.7, 1}, {0.88, 0.88}, {0.7, 0.72}, {0.2, 0.72}}},
	'э': {{{0.15, 0.52}, {0.5, 0.45}, {0.85, 0.6}, {0.88, 0.82}, {0.5, 1}, {0.15, 0.93}}, {{0.4, 0.72}, {0.86, 0.72}}},
	'ю': {{{0.1, 0.45}, {0.1, 1}}, {{0.1, 0.72}, {0.35, 0.72}}, {{0.65, 0.45}, {0.42, 0.55}, {0.35, 0.72}, {0.42, 0.9}, {0.65, 1}, {0.88, 0.9}, {0.95, 0.72}, {0.88, 0.55}, {0.65, 0.45}}},
	'я': {{{0.85, 1}, {0.85, 0.45}, {0.4, 0.45}, {0.2, 0.58}, {0.4, 0.72}, {0.85, 0.72}}, {{0.45, 0.72}, {0.15, 1}}},
	'□': {{{0.1, 0.2}, {0.9, 0.2}, {0.9, 1}, {0.1, 1}, {0.1, 0.2}}},
	'△': {{{0.5, 0.15}, {0.95, 1}, {0.05, 1}, {0.5, 0.15}}},
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
// A sheet of a plan of parts is written part by part, each where its place puts it (sheetOfParts).
// On a sheet of the scenario "similar-digits", the last digit of the answer line leans toward the
// digit of its pair (SimilarShift); a digit of no pair, an answer line with the planned erasure and
// a sheet of parts do not lean.
// It panics on a plan sheet the plan's check refuses, such as one without lines, and on one code
// cannot draw yet (Drawable).
func Sheet(sh *sample.PlanSheet, seed uint64) *sample.Sample {
	return SheetLeaning(sh, seed, SimilarShift)
}

// SheetLeaning is Sheet with the digit of the answer of a sheet of the scenario "similar-digits"
// leaning the share shift toward the digit of its pair: draw takes it with -shift, to draw the near
// sheets of the extended measurement at shares 0.1 to 0.4 (T93; R50).
func SheetLeaning(sh *sample.PlanSheet, seed uint64, shift float64) *sample.Sample {
	if len(sh.Parts) > 0 {
		return sheetOfParts(sh, seed)
	}
	if len(sh.Lines) == 0 {
		panic(fmt.Sprintf("synth: plan sheet %d has no lines", sh.Number))
	}
	w := sheetWriter(sh, seed)
	h := w.newHand(sh)
	w.pointMs = h.pointMs
	w.t = 800 + w.rng.Int64N(2000)

	for i, text := range sh.Lines {
		x, y := w.nextLine(h, i)
		var ids []string
		if e := sh.Erasure; e != nil && e.Line == i+1 {
			ids = w.erasing(h, sh, text, x, y)
		} else {
			if sh.Scenario == sample.ScenarioSimilarDigits && i+1 == sh.Answer {
				if at, toward, ok := SimilarDigit(text); ok {
					w.lean = &lean{at: at, toward: toward, shift: shift}
				}
			}
			ids, _ = w.write(h, text, x, y)
			w.lean = nil
		}
		w.s.Lines = append(w.s.Lines, sample.Line{Text: text, Strokes: ids, Answer: i+1 == sh.Answer})
	}
	return w.s
}

// nextLine is where the line i of a sheet starts, its top left corner, and the pause before it: the
// lines go top to bottom a step apart, each a little off its place, and before every line but the
// first the writer stops, now and then for long.
func (w *writer) nextLine(h hand, i int) (x, y float64) {
	y = h.top + float64(float64(i)*h.step) + w.jitter(h.height*0.15)
	x = h.left + w.jitter(h.height*0.4)
	if i > 0 {
		w.pause(1200 + w.rng.Int64N(3300))
		if w.rng.Float64() < 0.2 {
			w.pause(6000 + w.rng.Int64N(6000)) // a long stop, such as the journal names
		}
	}
	return x, y
}

// newHand picks the hand of a sheet of the board, so that its longest line fits across the sheet
// and its lines fit down it.
func (w *writer) newHand(sh *sample.PlanSheet) hand { return w.handOn(sh, sheetWidth, sheetHeight) }

// handOn is newHand on a sheet of the size given.
func (w *writer) handOn(sh *sample.PlanSheet, width, height float64) hand {
	h, spacing := w.pickHand(0.025, 120, 50)
	longest := 0
	for _, l := range sh.Lines {
		longest = max(longest, utf8.RuneCountInString(l))
	}
	if room := float64(width-40) - h.left; float64(longest)*h.height*h.shape*spacing > room {
		h.height = room / (float64(longest) * h.shape * spacing)
	}
	h.advance = h.height * h.shape * spacing
	h.step = h.height * (1.9 + float64(0.7*w.rng.Float64()))
	if n := len(sh.Lines); n > 1 {
		// The last line, its tails too, ends above the bottom of the sheet.
		h.step = min(h.step, (float64(height-20)-h.top-float64(1.5*h.height))/float64(n-1))
	}
	return h
}

// pickHand draws the size, the slant, the place and the speed of a hand: how much its lines climb
// or fall at most, per unit to the right, how far right of 40 units they start at most and how far
// down of 40 units the first one starts, and the spacing of its characters — the advance, of the
// width of a character. Each sheet fits the hand to itself after.
func (w *writer) pickHand(tilt, left, top float64) (h hand, spacing float64) {
	h = hand{
		height:  32 + float64(14*w.rng.Float64()),
		shape:   0.5 + float64(0.12*w.rng.Float64()),
		slant:   -0.05 + float64(0.3*w.rng.Float64()),
		tilt:    w.jitter(tilt),
		left:    40 + float64(left*w.rng.Float64()),
		top:     40 + float64(top*w.rng.Float64()),
		pointMs: 6 + w.rng.Int64N(6),
	}
	return h, 1.25 + float64(0.15*w.rng.Float64())
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
	n := -1 // the place of the character among those of the line
	for _, r := range s {
		n++
		if r == ' ' {
			x += float64(h.advance * 0.5)
			continue
		}
		g := glyphOf(r)
		if g == nil {
			g = squiggle
		}
		if w.lean != nil && n == w.lean.at {
			g = shifted(g, font[w.lean.toward], w.lean.shift)
		}
		size := h.height * (0.92 + float64(0.16*w.rng.Float64()))
		drift = clamp(drift+w.jitter(h.height*0.04), -h.height*0.12, h.height*0.12)
		top := y + drift + float64(h.tilt*(x-h.left)) + (h.height - size)
		flip := w.flip != nil && n == *w.flip
		for _, poly := range g {
			points := make([]pt, len(poly))
			for i, p := range poly {
				gx := p.x
				if flip {
					gx = 1 - gx
				}
				points[i] = pt{
					x + float64(gx*size*h.shape) + float64(h.slant*(1-p.y)*size) + w.jitter(size*0.03),
					top + float64(p.y*size) + w.jitter(size*0.03),
				}
			}
			ids = append(ids, w.stroke(points))
			w.pause(60 + w.rng.Int64N(200))
		}
		w.pause(90 + w.rng.Int64N(260))
		x += float64(h.advance * (0.9 + float64(0.2*w.rng.Float64())))
	}
	return ids, x
}
