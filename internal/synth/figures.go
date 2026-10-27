package synth

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"unicode/utf8"

	"github.com/altnovatrail/tutor/internal/sample"
)

// A drawing of a plan of parts (T56) — a number line, a row of objects or a scheme of segments —
// is made of things with keys: the axis, a division, a label and the arc of a jump; a house, its
// flag and its number; the label of a segment, a piece of it, the piece's label, the brace and the
// brace's label. Each thing is drawn next to one drawn just before it, within a group's 2 s and 50
// units, so that a drawing is one group of strokes (R30 in docs/decisions.md).
//
// An erasure in a drawing draws the figure as it was before the erasure — what stays first, and the
// things to go last, so that what stays holds together without them — takes them away in one move
// of the eraser, and then draws the things of the final figure that were not there. The keys tell
// the two figures' things apart: a thing of both keeps its key and its strokes. What is drawn after
// the erasure is a group of its own, as on a device, and a thing that only the things taken away
// held to the rest, such as the brace of a scheme whose longest piece goes, stands apart too.

// thing is one thing of a drawing: its key and how it is drawn, returning its strokes.
type thing struct {
	key  string
	draw func() []string
}

// figure is a drawing laid out on the sheet, its frame shared by the figure before an erasure and
// the final one: their things in the order they are drawn, the things a fix draws after the
// erasure, and how far down the drawing reaches from its top.
type figure interface {
	things(f sample.Figure) []thing
	fix(final, before sample.Figure) []thing
	height() float64
}

// drawingWidth is how wide a drawing of the main column may be; a wide one takes the sheet.
const drawingWidth = 560

// sketch lays out the drawing of a part: where it is placed and how wide, with the frame of the
// figure before the planned erasure when the erasure is in the part.
func (w *writer) sketch(h hand, sh *sample.PlanSheet, part int, x, y float64) figure {
	p := sh.Parts[part]
	width := float64(sheetWidth-40) - x
	if p.Place != sample.PlaceWide {
		width = min(width, drawingWidth)
	}
	before := sample.Figure{}
	if e := erasureIn(sh, part); e != nil && e.Figure != nil {
		before = *e.Figure
	}
	switch f := p.Figure; {
	case f.NumberLine != nil:
		return w.numberLine(h, f.NumberLine, before.NumberLine, x, y, width)
	case f.Row != nil:
		return w.row(h, f.Row, before.Row, x, y, width)
	case f.Scheme != nil:
		return w.scheme(h, f.Scheme, before.Scheme, x, y, width)
	}
	panic(fmt.Sprintf("synth: plan sheet %d: part %d is no drawing", sh.Number, part+1))
}

// drawing draws the drawing of a part with its top left corner at (x, y), and the planned erasure
// in it, and returns the strokes of the part that stay. A figure before the erasure that holds
// nothing the final one does not leaves the erasure nothing to take: the sample then fails its
// check, and draw names the sheet as drawn wrong.
func (w *writer) drawing(h hand, sh *sample.PlanSheet, part int, x, y float64) []string {
	fig := w.sketch(h, sh, part, x, y)
	final := sh.Parts[part].Figure
	e := erasureIn(sh, part)
	if e == nil || e.Figure == nil {
		return w.drawAll(fig.things(final))
	}
	keep := keysOf(fig.things(final))
	var stays, goes []thing
	for _, t := range fig.things(*e.Figure) {
		if keep[t.key] {
			stays = append(stays, t)
		} else {
			goes = append(goes, t)
		}
	}
	ids, erased := w.drawAll(stays), w.drawAll(goes)
	w.pause(1500 + w.rng.Int64N(3500)) // the writer sees what is to go
	w.s.Erasures = append(w.s.Erasures, sample.Erasure{
		TMs:     w.t,
		Strokes: erased,
		Planned: &sample.PlannedErasure{Line: e.Line, Erased: e.Erased, Text: e.Text},
	})
	w.pause(400 + w.rng.Int64N(1100))
	return append(ids, w.drawAll(fig.fix(final, *e.Figure))...)
}

// drawAll draws things one after another, with the short pause of a hand between them.
func (w *writer) drawAll(things []thing) []string {
	var ids []string
	for _, t := range things {
		ids = append(ids, t.draw()...)
		w.pause(60 + w.rng.Int64N(200))
	}
	return ids
}

// fresh are the things of the final figure that the figure before did not hold, in the order the
// final figure draws them.
func fresh(fig figure, final, before sample.Figure) []thing {
	had := keysOf(fig.things(before))
	var out []thing
	for _, t := range fig.things(final) {
		if !had[t.key] {
			out = append(out, t)
		}
	}
	return out
}

// keysOf is the set of the keys of things.
func keysOf(things []thing) map[string]bool {
	keys := make(map[string]bool, len(things))
	for _, t := range things {
		keys[t.key] = true
	}
	return keys
}

// small is a hand for labels: the hand's letters at a height, upright and not climbing.
func small(h hand, height float64) hand {
	s := h
	s.advance = h.advance * (height / h.height)
	s.height = height
	s.tilt, s.slant = 0, 0
	return s
}

// widthOf is how wide a string is written by a hand, nominally.
func widthOf(h hand, s string) float64 {
	return float64(h.advance * float64(utf8.RuneCountInString(s)))
}

// at writes a string by a hand centred across x, with its top at y.
func (w *writer) at(h hand, s string, x, y float64) []string {
	ids, _ := w.write(h, s, x-float64(widthOf(h, s)*0.45), y)
	return ids
}

// trace draws a stroke through the points, each a little off where it is meant, as a hand draws.
func (w *writer) trace(points ...pt) []string {
	for i := range points {
		points[i] = pt{points[i].x + w.jitter(1.2), points[i].y + w.jitter(1.2)}
	}
	return []string{w.stroke(points)}
}

// numberLine is a number line laid out: its divisions d apart from x0, its axis at axisY, and the
// hand of its labels.
type numberLine struct {
	w        *writer
	nl       *sample.NumberLine
	x0, d    float64
	axisY    float64
	label    hand
	tall     float64
	from, to int
}

// The geometry of a number line, in sheet units: the step between divisions at most, how far a
// division reaches either way from the axis, and how far a label stands under it.
const (
	maxDivision = 45
	tickReach   = 7
	labelGap    = 12
	labelApart  = 14
	minLabel    = 14 // the smallest label, so that it reads, though neighbours may then stand close
)

func (w *writer) numberLine(h hand, nl, before *sample.NumberLine, x, y, width float64) *numberLine {
	n := (nl.To-nl.From)/nl.Step + 1
	margin := float64(h.height * 0.6)
	d := min(maxDivision, (width-float64(2*margin))/float64(max(n-1, 1)))
	l := &numberLine{w: w, nl: nl, x0: x + margin, d: d, from: nl.From, to: nl.To}
	l.label = small(h, min(float64(h.height*0.85), max(labelHeight(d, nl, before), minLabel)))
	arc := 0.0
	for _, f := range []*sample.NumberLine{nl, before} {
		if f == nil {
			continue
		}
		for _, j := range f.Jumps {
			arc = max(arc, l.arcHeight(j))
		}
	}
	l.axisY = y + arc + 8
	l.tall = arc + 8 + labelGap + float64(l.label.height*1.15) + 4
	return l
}

func (l *numberLine) height() float64 { return l.tall }

// labelHeight is how tall the labels of a number line may be, so that two labels of neighbouring
// divisions, «10» and «11», stand apart by a gap that shows: at least labelApart units.
func labelHeight(d float64, lines ...*sample.NumberLine) float64 {
	tallest := 1e9
	for _, nl := range lines {
		if nl == nil {
			continue
		}
		labels := slices.Sorted(slices.Values(nl.Labels))
		for i := 1; i < len(labels); i++ {
			apart := float64(float64((labels[i]-labels[i-1])/nl.Step)*d) - labelApart
			// A label of n digits is about 0.55 + 0.72·(n − 1) of its height wide, with the
			// spacing of a hand; the two labels share the room between their divisions.
			digits := max(len(strconv.Itoa(labels[i-1])), len(strconv.Itoa(labels[i])))
			tallest = min(tallest, apart/(0.63+float64(0.72*float64(digits-1))))
		}
	}
	return tallest
}

// xOf is where the division of a value stands.
func (l *numberLine) xOf(v int) float64 {
	return l.x0 + float64(float64((v-l.nl.From)/l.nl.Step)*l.d)
}

// arcHeight is how high the arc of a jump rises over the axis: higher for a longer jump, and half
// as high again for a jump to the left, so that it shows over the jumps to the right it crosses.
func (l *numberLine) arcHeight(j sample.Jump) float64 {
	span := math.Abs(float64(float64(j.To-j.From) * l.d / float64(l.nl.Step)))
	height := 12 + min(48, float64(span*0.3))
	if j.To < j.From {
		height = float64(height * 1.5)
	}
	return height
}

// things are the things of a number line in the order a hand draws it: the axis, then the
// divisions from the left, each labelled at once when it has a label, and the arc of each jump as
// soon as both its divisions are drawn, in the order of the jumps.
func (l *numberLine) things(f sample.Figure) []thing {
	nl := f.NumberLine
	labelled := map[int]bool{}
	for _, v := range nl.Labels {
		labelled[v] = true
	}
	out := []thing{{"axis", l.axis}}
	next := 0
	for v := nl.From; v <= nl.To; v += nl.Step {
		out = append(out, thing{"tick:" + strconv.Itoa(v), l.tick(v)})
		if labelled[v] {
			out = append(out, l.labelThing(v))
		}
		for ; next < len(nl.Jumps) && max(nl.Jumps[next].From, nl.Jumps[next].To) <= v; next++ {
			out = append(out, l.jump(nl.Jumps[next]))
		}
	}
	return out
}

// fix draws what the final number line holds and the one before did not: the arc of each new jump
// in the order of the jumps, and right after it the new labels at its ends, so that each thing is
// next to the one before it; then the other new labels.
func (l *numberLine) fix(final, before sample.Figure) []thing {
	had := keysOf(l.things(before))
	labelled := map[int]bool{}
	for _, v := range final.NumberLine.Labels {
		labelled[v] = true
	}
	var out []thing
	add := func(t thing) {
		if !had[t.key] {
			had[t.key] = true
			out = append(out, t)
		}
	}
	for _, j := range final.NumberLine.Jumps {
		if t := l.jump(j); !had[t.key] {
			add(t)
			for _, v := range []int{j.To, j.From} {
				if labelled[v] {
					add(l.labelThing(v))
				}
			}
		}
	}
	for _, t := range l.things(final) {
		add(t)
	}
	return out
}

func (l *numberLine) axis() []string {
	return l.w.trace(pt{l.x0 - float64(l.d*0.4), l.axisY}, pt{l.xOf(l.to) + float64(l.d*0.4), l.axisY})
}

func (l *numberLine) tick(v int) func() []string {
	return func() []string {
		x := l.xOf(v)
		return l.w.trace(pt{x, l.axisY - tickReach}, pt{x, l.axisY + tickReach})
	}
}

func (l *numberLine) labelThing(v int) thing {
	return thing{"label:" + strconv.Itoa(v), func() []string {
		return l.w.at(l.label, strconv.Itoa(v), l.xOf(v), l.axisY+labelGap)
	}}
}

// jump is the arc of a jump over the axis, from one division to the other, with a hook at its end
// for the arrow.
func (l *numberLine) jump(j sample.Jump) thing {
	return thing{fmt.Sprintf("jump:%d>%d", j.From, j.To), func() []string {
		a, b := l.xOf(j.From), l.xOf(j.To)
		rise, foot := l.arcHeight(j), l.axisY-4
		const steps = 16
		points := make([]pt, 0, steps+2)
		for k := 0; k <= steps; k++ {
			t := float64(k) / steps
			points = append(points, pt{a + float64((b-a)*t), foot - float64(rise*float64(4*t*(1-t)))})
		}
		dir := 1.0
		if b < a {
			dir = -1
		}
		points = append(points, pt{b - float64(dir*7), foot - 9})
		return l.w.trace(points...)
	}}
}

// maxHouse is the widest step from one house to the next, in sheet units: so that the numbers under
// the houses, which a fix after an erasure may write one after another under houses drawn before,
// stay within a group's 50 units of each other.
const maxHouse = 54

// row is a row of houses laid out: the marked house at xMarked, the houses s apart, their walls
// standing on base, and the hand of their numbers.
type row struct {
	w       *writer
	xMarked float64
	s, wide float64 // the step between houses and how wide a house is
	base    float64
	label   hand
	tall    float64
}

func (w *writer) row(h hand, r, before *sample.Row, x, y, width float64) *row {
	left, right, numbers := r.Left, r.Right, r.Numbers
	if before != nil {
		left, right, numbers = max(left, before.Left), max(right, before.Right), numbers || before.Numbers
	}
	s := min(float64(h.height*1.9), maxHouse, width/float64(left+1+right))
	g := &row{w: w, s: s, wide: float64(s * 0.66)}
	g.xMarked = x + float64(s*float64(left)) + float64(s*0.5)
	g.base = y + float64(g.wide*1.65) + 2
	g.label = small(h, min(float64(h.height*0.8), float64(s*0.42)))
	g.tall = g.base - y + 4
	if numbers {
		g.tall += 6 + float64(g.label.height*1.15)
	}
	return g
}

func (g *row) height() float64 { return g.tall }

// things are the things of a row in the order a hand draws it: the houses from the left, the flag
// right after the marked house, and each house's number right under it when the row is numbered.
func (g *row) things(f sample.Figure) []thing {
	r := f.Row
	var out []thing
	for k := -r.Left; k <= r.Right; k++ {
		out = append(out, thing{"house:" + strconv.Itoa(k), g.house(k)})
		if k == 0 {
			out = append(out, thing{"flag", g.flag})
		}
		if r.Numbers {
			n := k + r.Left + 1
			out = append(out, thing{fmt.Sprintf("number:%d=%d", k, n), g.number(k, n)})
		}
	}
	return out
}

// fix draws the new things of a row from the left, as the row is drawn: each next to the one
// before it.
func (g *row) fix(final, before sample.Figure) []thing { return fresh(g, final, before) }

func (g *row) cx(k int) float64 { return g.xMarked + float64(g.s*float64(k)) }

// house draws a house of two strokes, its walls and floor, then its roof.
func (g *row) house(k int) func() []string {
	return func() []string {
		cx, half, wall := g.cx(k), float64(g.wide*0.5), g.base-float64(g.wide*0.6)
		body := g.w.trace(pt{cx - half, wall}, pt{cx - half, g.base}, pt{cx + half, g.base}, pt{cx + half, wall})
		roof := g.w.trace(pt{cx - half - 2, wall}, pt{cx, g.base - float64(g.wide*1.05)}, pt{cx + half + 2, wall})
		return append(body, roof...)
	}
}

func (g *row) flag() []string {
	cx, apex := g.cx(0), g.base-float64(g.wide*1.05)
	return g.w.trace(pt{cx, apex}, pt{cx, apex - float64(g.wide*0.6)}, pt{cx + float64(g.wide*0.4), apex - float64(g.wide*0.45)}, pt{cx, apex - float64(g.wide*0.3)})
}

func (g *row) number(k, n int) func() []string {
	return func() []string { return g.w.at(g.label, strconv.Itoa(n), g.cx(k), g.base+6) }
}

// schemeHand is the tallest digit of a scheme, in sheet units.
const schemeHand = 32

// scheme is a scheme of segments laid out: its segments one under another from first, pitch
// apart, their pieces from xs at unit units a unit of the scheme, and the hands of its labels.
type scheme struct {
	w           *writer
	h, label    hand
	x, xs, unit float64
	first       float64
	pitch       float64
	tall        float64
}

func (w *writer) scheme(h hand, sc, before *sample.Scheme, x, y, width float64) *scheme {
	// The rows of a scheme hold together by their pieces alone when no label stands before them: a
	// hand of at most schemeHand keeps them within a group's 50 units of each other.
	h = small(h, min(h.height, schemeHand))
	longest := 0
	for _, f := range []*sample.Scheme{sc, before} {
		if f != nil {
			longest = max(longest, schemeLength(f))
		}
	}
	g := &scheme{w: w, h: h, x: x, xs: x + float64(h.height*1.3), pitch: float64(h.height * 1.6)}
	g.unit = min(22, (width-float64(h.height*3.9))/float64(max(longest, 1)))
	g.first = y + h.height
	g.label = small(h, float64(h.height*0.75))
	g.tall = h.height + float64(g.pitch*float64(len(sc.Segments)-1)) + float64(h.height*0.4)
	return g
}

func (g *scheme) height() float64 { return g.tall }

// schemeLength is how long the longest segment of a scheme is, in its units.
func schemeLength(sc *sample.Scheme) int {
	longest := 0
	for _, seg := range sc.Segments {
		n := 0
		for _, p := range seg.Pieces {
			n += p.Length
		}
		longest = max(longest, n)
	}
	return longest
}

// things are the things of a scheme in the order a hand draws it: segment by segment from the top,
// its label before it, then its pieces from the left, each labelled at once when it has a label;
// then the brace at the right of the longest segment and the brace's label.
func (g *scheme) things(f sample.Figure) []thing {
	sc := f.Scheme
	var out []thing
	for i, seg := range sc.Segments {
		y := g.first + float64(g.pitch*float64(i))
		if seg.Label != "" {
			out = append(out, thing{fmt.Sprintf("label:%d=%s", i, seg.Label), g.text(g.h, seg.Label, g.x, y-float64(g.h.height*0.85))})
		}
		start := 0
		for _, p := range seg.Pieces {
			key := fmt.Sprintf("piece:%d:%d:%d", i, start, p.Length)
			out = append(out, thing{key, g.piece(start, p.Length, y)})
			if p.Label != "" {
				mid := g.xs + float64(g.unit*(float64(start)+float64(p.Length)/2))
				top := y - float64(g.h.height*0.25) - float64(g.label.height*1.15)
				out = append(out, thing{key + "=" + p.Label, func() []string { return g.w.at(g.label, p.Label, mid, top) }})
			}
			start += p.Length
		}
	}
	longest := schemeLength(sc)
	out = append(out, thing{"brace:" + strconv.Itoa(longest), g.brace(longest, len(sc.Segments))})
	if sc.Total != "" {
		xb := g.xs + float64(g.unit*float64(longest)) + float64(g.h.height*0.4)
		mid := g.first + float64(g.pitch*float64(len(sc.Segments)-1)/2)
		out = append(out, thing{fmt.Sprintf("total:%d=%s", longest, sc.Total), g.text(g.h, sc.Total, xb+float64(g.h.height*0.8), mid-float64(g.h.height*0.5))})
	}
	return out
}

// fix draws the new things of a scheme in the order the scheme is drawn: each next to the one
// before it.
func (g *scheme) fix(final, before sample.Figure) []thing { return fresh(g, final, before) }

func (g *scheme) text(h hand, s string, x, y float64) func() []string {
	return func() []string {
		ids, _ := g.w.write(h, s, x, y)
		return ids
	}
}

// piece draws a piece of a segment as one stroke: its ends a little up, as a bracket.
func (g *scheme) piece(start, length int, y float64) func() []string {
	return func() []string {
		a := g.xs + float64(g.unit*float64(start))
		b := g.xs + float64(g.unit*float64(start+length))
		up := y - float64(g.h.height*0.25)
		return g.w.trace(pt{a, up}, pt{a, y}, pt{b, y}, pt{b, up})
	}
}

// brace draws the brace that joins the segments, at the right of the longest one.
func (g *scheme) brace(longest, segments int) func() []string {
	return func() []string {
		x := g.xs + float64(g.unit*float64(longest)) + float64(g.h.height*0.4)
		top := g.first - float64(g.h.height*0.3)
		bottom := g.first + float64(g.pitch*float64(segments-1)) + float64(g.h.height*0.2)
		mid, bend := (top+bottom)/2, float64(g.h.height*0.3)
		return g.w.trace(pt{x, top}, pt{x + bend, top + bend/2}, pt{x + bend, mid - bend/3}, pt{x + float64(bend*1.8), mid},
			pt{x + bend, mid + bend/3}, pt{x + bend, bottom - bend/2}, pt{x, bottom})
	}
}
