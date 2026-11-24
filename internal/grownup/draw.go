package grownup

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/altnovatrail/tutor/internal/synth"
)

// Line is a stroke the tutor writes on the sheet: its points across and down in pixels of the crop
// of the work, in pairs, at least two points; its colour, #rrggbb; and the width of its line in
// pixels.
type Line struct {
	Points []float32
	Color  string
	Width  float32
}

// Drawn is an act as the sheet takes it: the words, the strokes to write in the order to draw
// them, the IDs of the strokes to take off, each once, and the faults that the program put right or
// left out — a point beyond the work, an ID the request did not name, a mark that lacks what its
// kind needs, a character the hand does not write. An act with faults is not what the model meant,
// and a later model may do better.
type Drawn struct {
	Say    string
	Lines  []Line
	Erase  []int32
	Faults []string
}

// Did tells whether the act will do: no fault, and something done — a stroke, a stroke taken off
// or words.
func (d Drawn) Did() bool {
	return len(d.Faults) == 0 && (len(d.Lines) > 0 || len(d.Erase) > 0 || strings.TrimSpace(d.Say) != "")
}

// The defaults of a mark that names no colour or width: the dark of the learner's pen, the width of
// a line, and of the line of text a share of its size, no thinner than thinText.
const (
	defaultColor = "#1d1d1f"
	defaultWidth = 3
	textWidth    = 1.0 / 14
	thinText     = 1.5
)

// The widths a line may take, in pixels: a thinner or a wider one is taken as the nearer.
const (
	thinnest = 0.5
	widest   = 60
)

// The sheet draws a stroke as a smooth curve through the middles of its sides, which rounds a
// corner by half its sides: a glyph of a few points would lose its corners — П reads as Л, и as н,
// Б as 6. So a stroke is laid with a point at least every denseStep pixels, a line of text every
// tenth of its size and no more than that, the points of a stroke no more than densest.
const (
	denseStep = 3
	leastStep = 1
	densest   = 512
)

// lineStep is the step from a line of a mark of text to the next, of its size.
const lineStep = 1.5

// ellipseStep is the length of a side of the polyline of an ellipse, in pixels, between the least
// and the most sides it takes.
const (
	ellipseStep = 4
	leastSides  = 24
	mostSides   = 180
)

// Draw turns an act into what the sheet takes, on a work of width × height pixels whose strokes are
// those the request named: a polyline as it is, an ellipse as a closed polyline, a line of text in
// the hand of internal/synth (synth.Handwriting), a line a mark of text holds on each of its lines,
// each lineStep of its size below the one before. A point off the work by less than a margin — 2 %
// of its longer side, 8 pixels at least — is moved onto its edge; one further off is moved too, and
// is a fault. A stroke of fewer than two points is left out. An ID to take off that the request did
// not name is left out, and is a fault.
func Draw(a *Act, width, height int, named []int32) Drawn {
	d := Drawn{Say: strings.TrimSpace(a.Say)}
	w, h := float64(width), float64(height)
	margin := max(8, 0.02*max(w, h))
	for _, id := range a.Erase {
		switch {
		case id < math.MinInt32 || id > math.MaxInt32 || !slices.Contains(named, int32(id)):
			d.Faults = append(d.Faults, fmt.Sprintf("the stroke %d to take off is none the request named", id))
		case !slices.Contains(d.Erase, int32(id)):
			d.Erase = append(d.Erase, int32(id))
		}
	}
	for i, m := range a.Marks {
		polys, color, lineWidth, fault := shapes(m)
		if fault != "" {
			d.Faults = append(d.Faults, fmt.Sprintf("mark %d: %s", i+1, fault))
		}
		step := float64(denseStep)
		if m.Kind == KindText && m.Size != nil {
			step = max(leastStep, min(denseStep, *m.Size/10))
		}
		off := false
		for _, poly := range polys {
			if len(poly) < 2 {
				continue
			}
			poly = dense(poly, step, densest)
			l := Line{Color: color, Width: float32(min(max(lineWidth, thinnest), widest))}
			for _, p := range poly {
				x, y := min(max(p.X, 0), w), min(max(p.Y, 0), h)
				off = off || math.Abs(x-p.X) > margin || math.Abs(y-p.Y) > margin
				l.Points = append(l.Points, float32(x), float32(y))
			}
			d.Lines = append(d.Lines, l)
		}
		if off {
			d.Faults = append(d.Faults, fmt.Sprintf("mark %d: a point beyond the work", i+1))
		}
	}
	return d
}

// shapes are the polylines of a mark, its colour and its width, and what it lacks of its kind, or
// "".
func shapes(m Mark) (polys [][]synth.Point, color string, width float64, fault string) {
	color, width = defaultColor, defaultWidth
	if m.Color != nil {
		color = strings.ToLower(*m.Color)
	}
	if m.Width != nil {
		width = *m.Width
	}
	switch m.Kind {
	case KindPolyline:
		var poly []synth.Point
		for _, p := range m.Points {
			poly = append(poly, synth.Point{X: p[0], Y: p[1]})
		}
		return [][]synth.Point{poly}, color, width, ""
	case KindEllipse:
		if m.X == nil || m.Y == nil || m.RX == nil || m.RY == nil || *m.RX <= 0 || *m.RY <= 0 {
			return nil, color, width, "an ellipse lacks its centre or a radius"
		}
		return [][]synth.Point{ellipse(*m.X, *m.Y, *m.RX, *m.RY)}, color, width, ""
	case KindText:
		if m.X == nil || m.Y == nil || m.Text == nil || m.Size == nil || *m.Size <= 0 {
			return nil, color, width, "a text lacks its place, its words or its size"
		}
		if m.Width == nil {
			width = max(*m.Size*textWidth, thinText)
		}
		var missing []rune
		for i, line := range strings.Split(*m.Text, "\n") {
			strokes, lacks := synth.Handwriting(line, *m.X, *m.Y+float64(i)*lineStep*(*m.Size), *m.Size)
			polys = append(polys, strokes...)
			for _, r := range lacks {
				if !slices.Contains(missing, r) {
					missing = append(missing, r)
				}
			}
		}
		if len(missing) > 0 {
			fault = fmt.Sprintf("the hand writes no %q", string(missing))
		}
		return polys, color, width, fault
	}
	return nil, color, width, fmt.Sprintf("no kind %q", m.Kind)
}

// dense is a polyline with points added along its sides, so that none is longer than step, or than
// the step that keeps it to most points; its own points all kept. One of most points or more is as
// it is.
func dense(poly []synth.Point, step float64, most int) []synth.Point {
	if len(poly) >= most {
		return poly
	}
	length := 0.0
	for i := 1; i < len(poly); i++ {
		length += math.Hypot(poly[i].X-poly[i-1].X, poly[i].Y-poly[i-1].Y)
	}
	// Each side gains at most ceil(side/step) - 1 points: within most once step is length/(most - 2n).
	if spare := most - 2*len(poly); spare > 0 {
		step = max(step, length/float64(spare))
	} else {
		return poly
	}
	out := make([]synth.Point, 0, most)
	out = append(out, poly[0])
	for i := 1; i < len(poly); i++ {
		a, b := poly[i-1], poly[i]
		n := int(math.Ceil(math.Hypot(b.X-a.X, b.Y-a.Y) / step))
		for k := 1; k < n; k++ {
			t := float64(k) / float64(n)
			out = append(out, synth.Point{X: a.X + (b.X-a.X)*t, Y: a.Y + (b.Y-a.Y)*t})
		}
		out = append(out, b)
	}
	return out
}

// ellipse is an ellipse as a closed polyline, a side about ellipseStep long.
func ellipse(cx, cy, rx, ry float64) []synth.Point {
	n := int(math.Ceil(2 * math.Pi * max(rx, ry) / ellipseStep))
	n = min(max(n, leastSides), mostSides)
	poly := make([]synth.Point, 0, n+1)
	for i := 0; i <= n; i++ {
		a := 2 * math.Pi * float64(i) / float64(n)
		poly = append(poly, synth.Point{X: cx + rx*math.Cos(a), Y: cy + ry*math.Sin(a)})
	}
	return poly
}
