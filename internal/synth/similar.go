package synth

import (
	"math"
	"slices"
)

// SimilarShift is how far toward the digit of its pair the digit of a sheet of the scenario
// "similar-digits" is drawn: 0 is the digit as the font has it, 1 the digit of its pair. The
// development of the reading sets it, the largest share at which Opus without a key still reads
// nine sheets in ten right (T70, T73).
const SimilarShift = 0.3

// similar are the pairs of digits a hand writes alike, each the other's: so the digit of a key and
// the digit of its near key, which a sheet of the near set mixes (T70).
var similar = map[rune]rune{'1': '7', '7': '1', '4': '9', '9': '4', '3': '8', '8': '3', '5': '6', '6': '5'}

// Similar is the digit of a digit's pair, and false for a digit of no pair or no digit at all.
func Similar(r rune) (rune, bool) {
	s, ok := similar[r]
	return s, ok
}

// SimilarDigit is the digit a sheet of the scenario "similar-digits" draws shifted in its answer
// line: the last digit of the line, by its place among the characters of the line, and the digit
// of its pair it leans toward; false when the last digit has no pair or the line no digit.
func SimilarDigit(line string) (at int, toward rune, ok bool) {
	at, n := -1, 0
	var last rune
	for _, r := range line {
		if r >= '0' && r <= '9' {
			at, last = n, r
		}
		n++
	}
	if at < 0 {
		return 0, 0, false
	}
	toward, ok = similar[last]
	return at, toward, ok
}

// morphPoints is how many places evenly along its length a digit is taken at when it leans toward
// another, besides the corners of both: the two strokes are taken at the same places, and mixed.
const morphPoints = 24

// shifted is the digit from drawn the share t of the way toward the digit to. Every digit of the
// font is one stroke, so the two strokes are mixed point by point, at the same places along their
// length: every corner of either and morphPoints places evenly between, so that at the share 0 the
// digit is the font's, corners and all. The stroke of to is taken the way, forward or back, whose
// points lie nearer those of from: 4 is written from its foot, and 9 toward it.
func shifted(from, to glyph, t float64) glyph {
	a, b := from[0], to[0]
	even := evenly(morphPoints)
	back := slices.Clone(b)
	slices.Reverse(back)
	if apart(at(a, even), at(back, even)) < apart(at(a, even), at(b, even)) {
		b = back
	}
	places := slices.Concat(even, corners(a), corners(b))
	slices.Sort(places)
	places = slices.Compact(places)
	pa, pb := at(a, places), at(b, places)
	out := make([]pt, len(places))
	for i := range out {
		out[i] = pt{pa[i].x + float64(t*(pb[i].x-pa[i].x)), pa[i].y + float64(t*(pb[i].y-pa[i].y))}
	}
	return glyph{out}
}

// apart is how far apart two strokes of as many points lie: the sum of the squared distances
// between their points.
func apart(a, b []pt) float64 {
	sum := 0.0
	for i := range a {
		dx, dy := a[i].x-b[i].x, a[i].y-b[i].y
		sum += float64(dx*dx) + float64(dy*dy)
	}
	return sum
}

// evenly are n places evenly along a stroke, as shares of its length from 0 to 1.
func evenly(n int) []float64 {
	out := make([]float64, n)
	for k := range out {
		out[k] = float64(k) / float64(n-1)
	}
	return out
}

// lengths are how far along a polyline each of its points stands, from its first.
func lengths(poly []pt) []float64 {
	out := make([]float64, len(poly))
	for i := 1; i < len(poly); i++ {
		dx, dy := poly[i].x-poly[i-1].x, poly[i].y-poly[i-1].y
		out[i] = out[i-1] + math.Sqrt(float64(dx*dx)+float64(dy*dy))
	}
	return out
}

// corners are the places of a polyline's points, as shares of its length.
func corners(poly []pt) []float64 {
	l := lengths(poly)
	out := make([]float64, len(l))
	for i, v := range l {
		out[i] = v / l[len(l)-1]
	}
	return out
}

// at is a polyline taken at the places, shares of its length in order: a point on it at each. Each
// product is rounded on its own, so that it fuses with nothing (see the package's comment).
func at(poly []pt, places []float64) []pt {
	l := lengths(poly)
	total := l[len(l)-1]
	out := make([]pt, len(places))
	j := 1
	for k, place := range places {
		along := float64(total * place)
		for j < len(poly)-1 && l[j] < along {
			j++
		}
		span := l[j] - l[j-1]
		f := 0.0
		if span > 0 {
			f = math.Min(1, math.Max(0, (along-l[j-1])/span))
		}
		a, b := poly[j-1], poly[j]
		out[k] = pt{a.x + float64(f*(b.x-a.x)), a.y + float64(f*(b.y-a.y))}
	}
	return out
}

// lean is a digit of a line drawn leaning toward another: its place among the characters of the
// line, and the digit it leans toward.
type lean struct {
	at     int
	toward rune
	shift  float64
}
