package synth

import (
	"maps"
	"slices"

	"github.com/altnovatrail/tutor/internal/sample"
)

// latin are the letters of English text and the signs it takes besides those of the font, for the
// essays of epic E4 (T81; T82), in the box of the font as the Cyrillic letters are: a capital takes
// the height of a digit, from 0 to 1; a small letter its lower part, from 0.45 down; a stem of b, d,
// f, h, k and l rises to the top, that of t to 0.2, and a tail of g, j, p, q and y goes below the
// line, down to 1.35. They are tutor's own strokes, drawn for these tests: no font of another's is
// in them, and so no licence of another's (R12 spec). The small letters a Cyrillic letter is drawn
// alike as, a, c, e, o, p and y, take its strokes, and so do O and the bowl of Q of О; x is the
// font's, as in the equations, and stands 0.1 above the other small letters.
var latin = map[rune]glyph{
	'A': {{{0.05, 1}, {0.5, 0}, {0.95, 1}}, {{0.25, 0.6}, {0.75, 0.6}}},
	'B': {{{0.15, 1}, {0.15, 0}, {0.65, 0}, {0.85, 0.12}, {0.85, 0.35}, {0.65, 0.48}, {0.15, 0.48}}, {{0.65, 0.48}, {0.9, 0.62}, {0.9, 0.85}, {0.65, 1}, {0.15, 1}}},
	'C': {{{0.88, 0.15}, {0.65, 0}, {0.35, 0}, {0.1, 0.2}, {0.05, 0.5}, {0.1, 0.8}, {0.35, 1}, {0.65, 1}, {0.88, 0.85}}},
	'D': {{{0.15, 0}, {0.15, 1}, {0.55, 1}, {0.85, 0.8}, {0.92, 0.5}, {0.85, 0.2}, {0.55, 0}, {0.15, 0}}},
	'E': {{{0.85, 0}, {0.15, 0}, {0.15, 1}, {0.85, 1}}, {{0.15, 0.5}, {0.7, 0.5}}},
	'F': {{{0.85, 0}, {0.15, 0}, {0.15, 1}}, {{0.15, 0.5}, {0.7, 0.5}}},
	'G': {{{0.88, 0.15}, {0.65, 0}, {0.35, 0}, {0.1, 0.2}, {0.05, 0.5}, {0.1, 0.8}, {0.35, 1}, {0.65, 1}, {0.9, 0.85}, {0.9, 0.55}, {0.55, 0.55}}},
	'H': {{{0.15, 0}, {0.15, 1}}, {{0.85, 0}, {0.85, 1}}, {{0.15, 0.5}, {0.85, 0.5}}},
	'I': {{{0.5, 0}, {0.5, 1}}, {{0.3, 0}, {0.7, 0}}, {{0.3, 1}, {0.7, 1}}},
	'J': {{{0.75, 0}, {0.75, 0.8}, {0.6, 1}, {0.35, 1}, {0.15, 0.85}}},
	'K': {{{0.15, 0}, {0.15, 1}}, {{0.85, 0}, {0.15, 0.6}}, {{0.4, 0.42}, {0.9, 1}}},
	'L': {{{0.15, 0}, {0.15, 1}, {0.85, 1}}},
	'M': {{{0.08, 1}, {0.12, 0}, {0.5, 0.65}, {0.88, 0}, {0.92, 1}}},
	'N': {{{0.15, 1}, {0.15, 0}, {0.85, 1}, {0.85, 0}}},
	'O': letters['О'],
	'P': {{{0.15, 1}, {0.15, 0}, {0.65, 0}, {0.88, 0.15}, {0.88, 0.35}, {0.65, 0.5}, {0.15, 0.5}}},
	'Q': append(slices.Clone(letters['О']), []pt{{0.6, 0.75}, {0.95, 1.05}}),
	'a': letters['а'],
	'c': letters['с'],
	'e': letters['е'],
	'o': letters['о'],
	'p': letters['р'],
	'y': letters['у'],
	'R': {{{0.15, 1}, {0.15, 0}, {0.65, 0}, {0.88, 0.15}, {0.88, 0.35}, {0.65, 0.5}, {0.15, 0.5}}, {{0.5, 0.5}, {0.9, 1}}},
	'S': {{{0.85, 0.15}, {0.6, 0}, {0.3, 0}, {0.1, 0.18}, {0.15, 0.4}, {0.5, 0.5}, {0.85, 0.62}, {0.9, 0.85}, {0.65, 1}, {0.3, 1}, {0.1, 0.85}}},
	'T': {{{0.05, 0}, {0.95, 0}}, {{0.5, 0}, {0.5, 1}}},
	'U': {{{0.12, 0}, {0.12, 0.75}, {0.3, 1}, {0.7, 1}, {0.88, 0.75}, {0.88, 0}}},
	'V': {{{0.05, 0}, {0.5, 1}, {0.95, 0}}},
	'W': {{{0.02, 0}, {0.25, 1}, {0.5, 0.35}, {0.75, 1}, {0.98, 0}}},
	'X': {{{0.1, 0}, {0.9, 1}}, {{0.9, 0}, {0.1, 1}}},
	'Y': {{{0.08, 0}, {0.5, 0.5}, {0.92, 0}}, {{0.5, 0.5}, {0.5, 1}}},
	'Z': {{{0.1, 0}, {0.9, 0}, {0.1, 1}, {0.9, 1}}},
	'b': {{{0.15, 0}, {0.15, 1}}, {{0.15, 0.6}, {0.4, 0.45}, {0.75, 0.5}, {0.88, 0.72}, {0.75, 0.95}, {0.4, 1}, {0.15, 0.9}}},
	'd': {{{0.85, 0}, {0.85, 1}}, {{0.85, 0.6}, {0.6, 0.45}, {0.25, 0.5}, {0.12, 0.72}, {0.25, 0.95}, {0.6, 1}, {0.85, 0.9}}},
	'f': {{{0.8, 0.05}, {0.6, 0}, {0.45, 0.1}, {0.4, 0.3}, {0.4, 1}}, {{0.15, 0.45}, {0.7, 0.45}}},
	'g': {{{0.85, 0.6}, {0.6, 0.45}, {0.25, 0.5}, {0.12, 0.68}, {0.25, 0.88}, {0.6, 0.92}, {0.85, 0.78}}, {{0.85, 0.45}, {0.85, 1.2}, {0.65, 1.35}, {0.3, 1.35}, {0.15, 1.25}}},
	'h': {{{0.15, 0}, {0.15, 1}}, {{0.15, 0.62}, {0.4, 0.45}, {0.7, 0.45}, {0.85, 0.6}, {0.85, 1}}},
	'i': {{{0.5, 0.45}, {0.5, 1}}, {{0.5, 0.28}}},
	'j': {{{0.6, 0.45}, {0.6, 1.2}, {0.45, 1.35}, {0.2, 1.3}}, {{0.6, 0.28}}},
	'k': {{{0.15, 0}, {0.15, 1}}, {{0.8, 0.45}, {0.15, 0.75}}, {{0.4, 0.65}, {0.85, 1}}},
	'l': {{{0.45, 0}, {0.45, 0.9}, {0.6, 1}}},
	'm': {{{0.08, 1}, {0.08, 0.45}}, {{0.08, 0.58}, {0.25, 0.45}, {0.42, 0.5}, {0.5, 0.62}, {0.5, 1}}, {{0.5, 0.62}, {0.65, 0.47}, {0.85, 0.5}, {0.92, 0.65}, {0.92, 1}}},
	'n': {{{0.15, 1}, {0.15, 0.45}}, {{0.15, 0.6}, {0.4, 0.45}, {0.7, 0.47}, {0.85, 0.62}, {0.85, 1}}},
	'q': {{{0.85, 0.6}, {0.6, 0.45}, {0.25, 0.5}, {0.12, 0.65}, {0.25, 0.8}, {0.6, 0.82}, {0.85, 0.7}}, {{0.85, 0.45}, {0.85, 1.35}}},
	'r': {{{0.2, 1}, {0.2, 0.45}}, {{0.2, 0.65}, {0.45, 0.47}, {0.8, 0.47}}},
	's': {{{0.82, 0.5}, {0.55, 0.45}, {0.25, 0.48}, {0.18, 0.6}, {0.5, 0.72}, {0.82, 0.82}, {0.78, 0.95}, {0.45, 1}, {0.15, 0.95}}},
	't': {{{0.45, 0.2}, {0.45, 0.9}, {0.6, 1}, {0.8, 0.95}}, {{0.15, 0.45}, {0.8, 0.45}}},
	'u': {{{0.15, 0.45}, {0.15, 0.85}, {0.35, 1}, {0.65, 1}, {0.85, 0.85}}, {{0.85, 0.45}, {0.85, 1}}},
	'v': {{{0.1, 0.45}, {0.5, 1}, {0.9, 0.45}}},
	'w': {{{0.02, 0.45}, {0.25, 1}, {0.5, 0.6}, {0.75, 1}, {0.98, 0.45}}},
	'z': {{{0.12, 0.45}, {0.88, 0.45}, {0.12, 1}, {0.88, 1}}},
	// The signs of English text that the font has not: the semicolon, the marks of an exclamation
	// and a question, the apostrophes and the quotation marks, straight and curly, and the dashes.
	';':  {{{0.5, 0.35}}, {{0.55, 0.9}, {0.4, 1.1}}},
	'!':  {{{0.5, 0}, {0.5, 0.7}}, {{0.5, 0.97}}},
	'?':  {{{0.15, 0.2}, {0.3, 0.02}, {0.65, 0}, {0.85, 0.18}, {0.8, 0.38}, {0.5, 0.55}, {0.5, 0.72}}, {{0.5, 0.97}}},
	'\'': {{{0.5, 0}, {0.45, 0.25}}},
	'’':  {{{0.55, 0}, {0.5, 0.15}, {0.4, 0.28}}},
	'"':  {{{0.35, 0}, {0.33, 0.25}}, {{0.65, 0}, {0.63, 0.25}}},
	'“':  {{{0.45, 0}, {0.35, 0.2}}, {{0.75, 0}, {0.65, 0.2}}},
	'”':  {{{0.35, 0}, {0.25, 0.2}}, {{0.65, 0}, {0.55, 0.2}}},
	'–':  {{{0.1, 0.6}, {0.9, 0.6}}},
	'—':  {{{0, 0.6}, {1, 0.6}}},
}

// glyphs are every character write draws: the font's, the Cyrillic letters and the Latin ones,
// which no character is in two of.
var glyphs = func() map[rune]glyph {
	out := maps.Clone(font)
	maps.Copy(out, letters)
	maps.Copy(out, latin)
	return out
}()

// glyphOf is the strokes of a character as write draws it, or nil for a character it has none of,
// which write draws as a squiggle.
func glyphOf(r rune) glyph { return glyphs[r] }

// textStep is the least step from a line of text to the next, of the height of a digit: the tails
// of a line, down to 1.35, then clear the capitals of the next with room to spare.
const textStep = 1.6

// spaced is a hand whose lines are at least textStep of its height apart: when its step is less, its
// characters are made smaller, its proportions kept.
func spaced(h hand) hand {
	if least := float64(textStep * h.height); h.step < least {
		f := h.step / least
		h.height, h.advance = float64(h.height*f), float64(h.advance*f)
	}
	return h
}

// Text draws lines of text as a writer would write them, from the seed, top to bottom on a sheet of
// the size given: a hand of its own, fitted so that the longest line fits across the sheet, and,
// when the lines are many, made smaller so that a line is at least 1.6 of its height below the one
// before it and the tails of one line clear the capitals of the next. The lines are laid out and
// paused between as on a sheet of the plan (nextLine). It returns the strokes in the order they
// were written. The essays of epic E4 are drawn with it (T81; T82). It panics on no lines, and on a
// sheet the strokes of a sample cannot lie on.
func Text(lines []string, sheet sample.Sheet, seed uint64) []sample.Stroke {
	if len(lines) == 0 {
		panic("synth: no lines of text")
	}
	sh := &sample.PlanSheet{Number: 1, Lines: lines}
	w := sheetWriter(sh, seed)
	w.s.Sheet = sheet
	h := spaced(w.handOn(sh, sheet.Width, sheet.Height))
	w.pointMs = h.pointMs
	w.t = 800 + w.rng.Int64N(2000)
	for i, line := range lines {
		x, y := w.nextLine(h, i)
		w.write(h, line, x, y)
	}
	return w.s.Strokes
}
