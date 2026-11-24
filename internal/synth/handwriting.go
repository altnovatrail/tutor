package synth

import (
	"maps"
	"math"
	"slices"
	"unicode"
)

// Point is a point of a line of handwriting, x to the right and y downwards.
type Point struct{ X, Y float64 }

// maths are the characters the tutor writes besides those write draws: the signs of school
// mathematics and the Cyrillic capitals, in the box of the font, a capital and a digit from 0 to 1;
// a superscript and a subscript are generated from the digits and a few signs. They are tutor's own
// strokes, as the rest of the font: no font of another's, and so no licence of another's. They
// stay out of glyphs, so that a sheet of a plan is drawn as it was.
var maths = func() map[rune]glyph {
	m := map[rune]glyph{
		'÷': {{{0.5, 0.35}}, {{0.15, 0.6}, {0.85, 0.6}}, {{0.5, 0.85}}},
		'√': {{{0.02, 0.6}, {0.15, 0.52}, {0.38, 1}, {0.68, -0.08}, {1.05, -0.08}}},
		'∫': {{{0.85, -0.05}, {0.7, -0.15}, {0.55, -0.05}, {0.48, 0.5}, {0.42, 1.05}, {0.27, 1.15}, {0.12, 1.05}}},
		'π': {{{0.05, 0.52}, {0.2, 0.45}, {0.95, 0.45}}, {{0.35, 0.45}, {0.3, 1}}, {{0.7, 0.45}, {0.7, 0.92}, {0.85, 1}}},
		'<': {{{0.85, 0.3}, {0.15, 0.62}, {0.85, 0.94}}},
		'>': {{{0.15, 0.3}, {0.85, 0.62}, {0.15, 0.94}}},
		'≤': {{{0.85, 0.15}, {0.15, 0.42}, {0.85, 0.69}}, {{0.15, 0.92}, {0.85, 0.92}}},
		'≥': {{{0.15, 0.15}, {0.85, 0.42}, {0.15, 0.69}}, {{0.15, 0.92}, {0.85, 0.92}}},
		'≠': {{{0.15, 0.45}, {0.85, 0.45}}, {{0.15, 0.75}, {0.85, 0.75}}, {{0.7, 0.28}, {0.3, 0.92}}},
		'≈': {{{0.1, 0.5}, {0.3, 0.4}, {0.5, 0.5}, {0.7, 0.6}, {0.9, 0.5}}, {{0.1, 0.75}, {0.3, 0.65}, {0.5, 0.75}, {0.7, 0.85}, {0.9, 0.75}}},
		'∞': {{{0.5, 0.65}, {0.3, 0.47}, {0.1, 0.5}, {0.03, 0.65}, {0.1, 0.8}, {0.3, 0.83}, {0.5, 0.65}, {0.7, 0.47}, {0.9, 0.5}, {0.97, 0.65}, {0.9, 0.8}, {0.7, 0.83}, {0.5, 0.65}}},
		'±': {{{0.5, 0.3}, {0.5, 0.75}}, {{0.15, 0.52}, {0.85, 0.52}}, {{0.15, 0.95}, {0.85, 0.95}}},
		'%': {{{0.85, 0.05}, {0.15, 0.95}}, ring(0.25, 0.2, 0.13), ring(0.75, 0.8, 0.13)},
		'°': {ring(0.5, 0.12, 0.12)},
		'|': {{{0.5, -0.05}, {0.5, 1.1}}},
		'[': {{{0.65, -0.05}, {0.35, -0.05}, {0.35, 1.05}, {0.65, 1.05}}},
		']': {{{0.35, -0.05}, {0.65, -0.05}, {0.65, 1.05}, {0.35, 1.05}}},
		'^': {{{0.2, 0.35}, {0.5, 0.05}, {0.8, 0.35}}},
		'_': {{{0.05, 1.05}, {0.95, 1.05}}},
		'*': {{{0.5, 0.25}, {0.5, 0.65}}, {{0.3, 0.33}, {0.7, 0.57}}, {{0.7, 0.33}, {0.3, 0.57}}},
		'∠': {{{0.9, 0.3}, {0.1, 0.95}, {0.95, 0.95}}},
		'α': {{{0.9, 0.45}, {0.75, 0.8}, {0.55, 0.98}, {0.25, 0.98}, {0.1, 0.75}, {0.25, 0.5}, {0.5, 0.45}, {0.7, 0.65}, {0.9, 1}}},
		'Б': {{{0.85, 0}, {0.15, 0}, {0.15, 1}, {0.6, 1}, {0.85, 0.88}, {0.85, 0.62}, {0.6, 0.48}, {0.15, 0.48}}},
		'Г': {{{0.15, 1}, {0.15, 0}, {0.85, 0}}},
		'Д': {{{0.05, 1.2}, {0.05, 0.95}, {0.95, 0.95}, {0.95, 1.2}}, {{0.15, 0.95}, {0.35, 0}, {0.8, 0}, {0.8, 0.95}}},
		'Ж': {{{0.02, 0}, {0.45, 0.5}, {0.02, 1}}, {{0.5, 0}, {0.5, 1}}, {{0.98, 0}, {0.55, 0.5}, {0.98, 1}}},
		'З': {{{0.12, 0.12}, {0.45, 0}, {0.8, 0.12}, {0.8, 0.35}, {0.45, 0.48}, {0.85, 0.62}, {0.85, 0.88}, {0.45, 1}, {0.1, 0.88}}},
		'И': {{{0.15, 0}, {0.15, 1}, {0.85, 0}, {0.85, 1}}},
		'Й': {{{0.15, 0}, {0.15, 1}, {0.85, 0}, {0.85, 1}}, {{0.32, -0.2}, {0.5, -0.1}, {0.68, -0.2}}},
		'Л': {{{0.05, 1}, {0.25, 0.85}, {0.4, 0}, {0.85, 0}, {0.85, 1}}},
		'У': {{{0.1, 0}, {0.5, 0.6}}, {{0.9, 0}, {0.4, 0.95}, {0.2, 1}}},
		'Ф': {{{0.5, 0}, {0.5, 1}}, {{0.5, 0.2}, {0.15, 0.25}, {0.05, 0.45}, {0.15, 0.65}, {0.5, 0.7}, {0.85, 0.65}, {0.95, 0.45}, {0.85, 0.25}, {0.5, 0.2}}},
		'Ц': {{{0.15, 0}, {0.15, 0.95}, {0.8, 0.95}, {0.8, 0}}, {{0.8, 0.95}, {0.95, 0.95}, {0.95, 1.2}}},
		'Ч': {{{0.15, 0}, {0.15, 0.4}, {0.4, 0.55}, {0.85, 0.5}}, {{0.85, 0}, {0.85, 1}}},
		'Ш': {{{0.08, 0}, {0.08, 1}, {0.92, 1}, {0.92, 0}}, {{0.5, 0}, {0.5, 1}}},
		'Щ': {{{0.08, 0}, {0.08, 0.95}, {0.85, 0.95}, {0.85, 0}}, {{0.46, 0}, {0.46, 0.95}}, {{0.85, 0.95}, {0.97, 0.95}, {0.97, 1.2}}},
		'Ъ': {{{0.02, 0}, {0.3, 0}, {0.3, 1}, {0.75, 1}, {0.92, 0.85}, {0.92, 0.62}, {0.75, 0.5}, {0.3, 0.5}}},
		'Ы': {{{0.1, 0}, {0.1, 1}, {0.5, 1}, {0.65, 0.85}, {0.65, 0.65}, {0.5, 0.5}, {0.1, 0.5}}, {{0.88, 0}, {0.88, 1}}},
		'Ь': {{{0.2, 0}, {0.2, 1}, {0.7, 1}, {0.88, 0.85}, {0.88, 0.62}, {0.7, 0.5}, {0.2, 0.5}}},
		'Э': {{{0.15, 0.12}, {0.5, 0}, {0.85, 0.2}, {0.92, 0.5}, {0.85, 0.8}, {0.5, 1}, {0.15, 0.88}}, {{0.4, 0.5}, {0.9, 0.5}}},
		'Ю': {{{0.08, 0}, {0.08, 1}}, {{0.08, 0.5}, {0.32, 0.5}}, {{0.65, 0}, {0.42, 0.15}, {0.35, 0.5}, {0.42, 0.85}, {0.65, 1}, {0.88, 0.85}, {0.95, 0.5}, {0.88, 0.15}, {0.65, 0}}},
		'Я': {{{0.85, 1}, {0.85, 0}, {0.4, 0}, {0.18, 0.12}, {0.18, 0.38}, {0.4, 0.5}, {0.85, 0.5}}, {{0.45, 0.5}, {0.12, 1}}},
		'Ё': append(slices.Clone(latin['E']), []pt{{0.35, -0.15}}, []pt{{0.65, -0.15}}),
	}
	// The Cyrillic capitals drawn as the Latin ones, and the Greek capital delta as the triangle.
	for c, l := range map[rune]rune{'А': 'A', 'В': 'B', 'Е': 'E', 'К': 'K', 'М': 'M', 'Н': 'H', 'Р': 'P', 'С': 'C', 'Т': 'T', 'Х': 'X'} {
		m[c] = latin[l]
	}
	m['Δ'] = letters['△']
	// A superscript is its character at a little over half its height, raised to the top of a
	// digit, and a subscript the same, lowered to its foot.
	for i, r := range []rune("⁰¹²³⁴⁵⁶⁷⁸⁹") {
		m[r] = scaled(font[rune('0'+i)], 0.55, -0.05)
	}
	for i, r := range []rune("₀₁₂₃₄₅₆₇₈₉") {
		m[r] = scaled(font[rune('0'+i)], 0.55, 0.62)
	}
	m['⁺'], m['⁻'], m['ⁿ'] = scaled(font['+'], 0.55, -0.3), scaled(font['−'], 0.55, -0.3), scaled(latin['n'], 0.55, -0.3)
	return m
}()

// ring is a small closed circle of the box, as a stroke.
func ring(cx, cy, r float64) []pt {
	var out []pt
	for i := 0; i <= 12; i++ {
		a := float64(2*math.Pi*float64(i)) / 12
		out = append(out, pt{cx + float64(r*math.Cos(a)), cy + float64(r*math.Sin(a))})
	}
	return out
}

// scaled is a glyph made smaller by the share given and moved down by dy, its left edge kept.
func scaled(g glyph, share, dy float64) glyph {
	out := make(glyph, len(g))
	for i, poly := range g {
		out[i] = make([]pt, len(poly))
		for j, p := range poly {
			out[i][j] = pt{0.1 + float64(p.x*share), float64(p.y*share) + dy}
		}
	}
	return out
}

// handGlyphs are every character Handwriting writes.
var handGlyphs = func() map[rune]glyph {
	out := maps.Clone(glyphs)
	maps.Copy(out, maths)
	return out
}()

// alike are characters Handwriting writes as another that looks the same.
var alike = map[rune]rune{
	'‐': '-', '‑': '-', '⋅': '·', '∙': '·', '′': '\'', 'ˣ': 'x', '∆': 'Δ',
}

// The shape of the handwriting, of its size, the height of a capital: how wide a character is,
// the room after it, how far a space moves on, and how far the top of a character leans to the right.
const (
	handShape = 0.55
	handGap   = 0.3
	handSpace = 0.4
	handSlant = 0.08
)

// Writes reports whether Handwriting writes a character: a space, or one of its glyphs.
func Writes(r rune) bool {
	if unicode.IsSpace(r) {
		return true
	}
	if a, ok := alike[r]; ok {
		r = a
	}
	_, ok := handGlyphs[r]
	return ok
}

// Handwriting writes a line of text in the hand of the font, as polylines: x is the left end of its
// baseline and baseline the foot of a capital or a digit, whose height is size; a small letter takes
// its lower part and a tail goes below the line. Each character takes the room its strokes need, and
// a dot is a short stroke, so that every stroke has two points. A character it does not write is
// left out, with the room of a space, and named among missing, each once, in its order.
func Handwriting(s string, x, baseline, size float64) (strokes [][]Point, missing []rune) {
	top := baseline - size
	width := float64(handShape * size)
	for _, r := range s {
		if unicode.IsSpace(r) {
			x += float64(handSpace * size)
			continue
		}
		if a, ok := alike[r]; ok {
			r = a
		}
		g, ok := handGlyphs[r]
		if !ok {
			if !slices.Contains(missing, r) {
				missing = append(missing, r)
			}
			x += float64(handSpace * size)
			continue
		}
		right := 0.5
		for _, poly := range g {
			for _, p := range poly {
				right = max(right, p.x)
			}
		}
		for _, poly := range g {
			points := make([]Point, 0, max(len(poly), 2))
			for _, p := range poly {
				points = append(points, Point{x + float64(p.x*width) + float64(handSlant*(1-p.y)*size), top + float64(p.y*size)})
			}
			if len(points) == 1 {
				// A dot, as a pen leaves it: a stroke a twentieth of the size long.
				points = append(points, Point{points[0].X + float64(size/20), points[0].Y})
			}
			strokes = append(strokes, points)
		}
		x += float64((right + handGap) * width)
	}
	return strokes, missing
}
