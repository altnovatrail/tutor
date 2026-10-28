// Package render draws a sheet of handwriting from its strokes, as the board shows it: ink of the
// board's pen, #111, 3 px wide with round ends and joins, on white, one pixel to a unit of the
// sheet. It draws the sheet at any moment of the writing: as it was finished, and just before each
// erasure, the frame of that erasure (R11, R24). One renderer draws every form of the measurement.
//
// The drawing is the same on every machine: it works in whole numbers alone. A point stands at the
// nearest sixteenth of a pixel, each pixel is sampled at 4 × 4 places, and a pixel takes the ink of
// the share of its samples the line covers. Floating-point arithmetic may round differently where
// a compiler fuses its operations, and a picture would then differ from one architecture to another.
//
// It draws a sample that passes its check (sample.Check), on a sheet of at most MaxSide units a
// side, and refuses any other.
//
// It cuts the crop of a sheet for checking too, by the rule of the eval protocol (crop.go): the work
// with its margins, scaled so that its digits stand a given height, and drawn by the same whole
// numbers, in 48ths of a pixel, with a line of the crop's own width.
package render

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"math"
	"math/bits"

	"github.com/altnovatrail/tutor/internal/sample"
)

// Ink is the grey of the board's pen, #111.
const Ink = 0x11

// MaxSide is the longest side of a sheet the renderer draws, in units of the sheet: up to it, the
// whole numbers of the drawing fit in 64 bits. The board's sheet is 1000 × 600.
const MaxSide = 2048

const (
	grid = 4                  // the samples of a pixel on each axis
	full = 1<<(grid*grid) - 1 // a pixel the line covers whole
)

// pen is how a line is drawn: the parts of a pixel its places are in, half the width of its line
// in them, how far a sample lies from the centre of its pixel at most, in them and rounded up, and
// the grey of its ink. A pixel whose centre is farther than half + reach from a piece of a line has
// no sample on the line, and one whose centre is within half − reach has all of them on it.
type pen struct {
	sub, half, reach int64
	ink              uint8
}

// board is the board's pen: places in sixteenths of a pixel, the line 3 px wide, and a sample at
// most √(6² + 6²) sixteenths from the centre of its pixel.
var board = pen{sub: 16, half: 24, reach: 9, ink: Ink}

// At draws the sheet as it stood at the moment t of its clock: the points written by then, without
// the strokes the erasures made by then took away. What happened at t itself is on it.
func At(s *sample.Sample, t int64) (*image.Gray, error) {
	return draw(s, Shown(s, t))
}

// Shown is what the sheet shows at the moment t of its clock, as At draws it: of each stroke no
// erasure made by then took away, the points written by then; a stroke with none of them is left
// out. The strokes go in the sample's order.
func Shown(s *sample.Sample, t int64) []sample.Stroke {
	var made []sample.Erasure
	for _, e := range s.Erasures {
		if e.TMs <= t {
			made = append(made, e)
		}
	}
	return shown(s, t, erased(made))
}

// shown is what the sheet shows at the moment t without the strokes gone: of each other stroke,
// the points written by t. A stroke's points go in the order of its clock (internal/sample checks
// it).
func shown(s *sample.Sample, t int64, gone map[string]bool) []sample.Stroke {
	var out []sample.Stroke
	for _, st := range s.Strokes {
		if gone[st.ID] {
			continue
		}
		n := 0
		for n < len(st.Points) && st.Points[n].TMs <= t {
			n++
		}
		if n > 0 {
			out = append(out, sample.Stroke{ID: st.ID, Points: st.Points[:n]})
		}
	}
	return out
}

// Final draws the sheet as it was finished: every stroke that no erasure took away.
func Final(s *sample.Sample) (*image.Gray, error) { return At(s, math.MaxInt64) }

// Before draws the sheet just before its erasure i, the frame of that erasure (R11): the strokes
// written by the erasure's moment, the ones it takes away among them, without those the erasures
// before it took away. A stroke that begins at the erasure's very moment and that it does not take
// away is the next stroke, written after it.
func Before(s *sample.Sample, i int) (*image.Gray, error) {
	strokes, err := ShownBefore(s, i)
	if err != nil {
		return nil, err
	}
	return draw(s, strokes)
}

// ShownBefore is what the sheet shows just before its erasure i, as Before draws it.
func ShownBefore(s *sample.Sample, i int) ([]sample.Stroke, error) {
	if i < 0 || i >= len(s.Erasures) {
		return nil, fmt.Errorf("render: the sample has %d erasures, and no erasure %d", len(s.Erasures), i)
	}
	e := s.Erasures[i]
	gone, takes := erased(s.Erasures[:i]), erased(s.Erasures[i:i+1])
	for _, st := range s.Strokes {
		if len(st.Points) > 0 && st.Points[0].TMs >= e.TMs && !takes[st.ID] {
			gone[st.ID] = true
		}
	}
	return shown(s, e.TMs, gone), nil
}

// PNG encodes a picture as PNG.
func PNG(img image.Image) ([]byte, error) {
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// Tokens is what a picture of w × h pixels costs in tokens of a Claude model's input,
// ⌈w / 28⌉ × ⌈h / 28⌉ (Vision; docs/architecture.md, section 4), as the model takes it unresized.
// A picture the model would resize is not charged so: the Claude adapter has the API refuse it
// (oversized_image: error). The board's sheet, 1000 × 600, costs 792.
func Tokens(w, h int) int { return ((w + 27) / 28) * ((h + 27) / 28) }

// point is a point of a stroke in the parts of a pixel its pen works in.
type point struct{ x, y int64 }

// erased is the set of the strokes the erasures take away.
func erased(es []sample.Erasure) map[string]bool {
	gone := map[string]bool{}
	for _, e := range es {
		for _, id := range e.Strokes {
			gone[id] = true
		}
	}
	return gone
}

// draw draws the strokes on the sheet with the board's pen.
func draw(s *sample.Sample, strokes []sample.Stroke) (*image.Gray, error) {
	w, h := s.Sheet.Width, s.Sheet.Height
	if !(w > 0 && w <= MaxSide && h > 0 && h <= MaxSide) {
		return nil, fmt.Errorf("render: the sheet is %v × %v, and the renderer draws sheets above 0 and up to %d a side", w, h, MaxSide)
	}
	img := image.NewGray(image.Rect(0, 0, int(math.Ceil(w)), int(math.Ceil(h))))
	for i := range img.Pix {
		img.Pix[i] = 0xff
	}
	for _, st := range strokes {
		var pts []point
		for _, p := range st.Points {
			if !(p.X >= 0 && p.X <= w && p.Y >= 0 && p.Y <= h) {
				return nil, fmt.Errorf("render: the stroke %q has a point off the sheet, (%v, %v)", st.ID, p.X, p.Y)
			}
			pts = append(pts, point{int64(math.Round(p.X * float64(board.sub))), int64(math.Round(p.Y * float64(board.sub)))})
		}
		if len(pts) > 0 {
			board.stroke(img, pts)
		}
	}
	return img, nil
}

// stroke draws a line through the points: within half the pen's width of each piece between two
// of them, which makes the ends and the joins round. A stroke of one point is a dot. A sample the
// line covers counts once, however many of its pieces cover it, so the line keeps one shade where
// it goes over itself.
func (p pen) stroke(img *image.Gray, pts []point) {
	b := img.Bounds()
	lo, hi := pts[0], pts[0]
	for _, q := range pts[1:] {
		lo = point{min(lo.x, q.x), min(lo.y, q.y)}
		hi = point{max(hi.x, q.x), max(hi.y, q.y)}
	}
	x0, y0 := max(p.pixel(lo.x-p.half), b.Min.X), max(p.pixel(lo.y-p.half), b.Min.Y)
	x1, y1 := min(p.pixel(hi.x+p.half)+1, b.Max.X), min(p.pixel(hi.y+p.half)+1, b.Max.Y)
	if x0 >= x1 || y0 >= y1 {
		return
	}
	cover := make([]uint16, (x1-x0)*(y1-y0))
	pieces := [][2]point{{pts[0], pts[0]}}
	if len(pts) > 1 {
		pieces = pieces[:0]
		for i := 1; i < len(pts); i++ {
			pieces = append(pieces, [2]point{pts[i-1], pts[i]})
		}
	}
	for _, pc := range pieces {
		a, c := pc[0], pc[1]
		px0, py0 := max(p.pixel(min(a.x, c.x)-p.half), x0), max(p.pixel(min(a.y, c.y)-p.half), y0)
		px1, py1 := min(p.pixel(max(a.x, c.x)+p.half)+1, x1), min(p.pixel(max(a.y, c.y)+p.half)+1, y1)
		for py := py0; py < py1; py++ {
			for px := px0; px < px1; px++ {
				k := (py-y0)*(x1-x0) + px - x0
				cx, cy := int64(px)*p.sub+p.sub/2, int64(py)*p.sub+p.sub/2
				switch {
				case cover[k] == full || !near(cx, cy, a, c, p.half+p.reach):
					continue
				case near(cx, cy, a, c, p.half-p.reach):
					cover[k] = full
					continue
				}
				var mask uint16
				for j := range grid {
					qy := int64(py)*p.sub + p.sub/grid/2 + int64(j)*p.sub/grid
					for i := range grid {
						qx := int64(px)*p.sub + p.sub/grid/2 + int64(i)*p.sub/grid
						if near(qx, qy, a, c, p.half) {
							mask |= 1 << (j*grid + i)
						}
					}
				}
				cover[k] |= mask
			}
		}
	}
	for py := y0; py < y1; py++ {
		for px := x0; px < x1; px++ {
			n := bits.OnesCount16(cover[(py-y0)*(x1-x0)+px-x0])
			if n == 0 {
				continue
			}
			o := &img.Pix[img.PixOffset(px, py)]
			*o = uint8((int(*o)*(grid*grid-n) + int(p.ink)*n + grid*grid/2) / (grid * grid))
		}
	}
}

// near reports whether the point q lies within r of the piece from a to c. On a sheet of up to
// MaxSide a side, in sixteenths of a pixel or, on a crop, in 48ths of one, no product here but
// cross² leaves 64 bits, and r²·l2 stays under 2⁶². A cross of 2³¹ or more is then farther than r,
// and cross² is not taken: on the board's sheet it never comes so far.
func near(qx, qy int64, a, c point, r int64) bool {
	dx, dy := c.x-a.x, c.y-a.y
	px, py := qx-a.x, qy-a.y
	l2 := dx*dx + dy*dy
	dot := px*dx + py*dy
	switch {
	case l2 == 0 || dot <= 0:
		return px*px+py*py <= r*r
	case dot >= l2:
		ex, ey := qx-c.x, qy-c.y
		return ex*ex+ey*ey <= r*r
	}
	cross := px*dy - py*dx
	if cross >= 1<<31 || cross <= -(1<<31) {
		return false
	}
	return cross*cross <= r*r*l2
}

// pixel is the pixel a coordinate in the pen's parts of a pixel falls in, below zero too.
func (p pen) pixel(v int64) int {
	if v < 0 {
		return int((v - p.sub + 1) / p.sub)
	}
	return int(v / p.sub)
}
