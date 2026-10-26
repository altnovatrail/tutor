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
	sub  = 16 // the sixteenths of a pixel the drawing works in
	grid = 4  // the samples of a pixel on each axis
	half = 24 // half the width of the pen's line, 3 px, in sixteenths
	// reach is how far a sample lies from the centre of its pixel at most, √(6² + 6²) sixteenths,
	// rounded up. A pixel whose centre is farther than half + reach from a piece of a line has no
	// sample on the line, and one whose centre is within half − reach has all of them on it.
	reach = 9
	full  = 1<<(grid*grid) - 1 // a pixel the line covers whole
)

// At draws the sheet as it stood at the moment t of its clock: the points written by then, without
// the strokes the erasures made by then took away. What happened at t itself is on it.
func At(s *sample.Sample, t int64) (*image.Gray, error) {
	var made []sample.Erasure
	for _, e := range s.Erasures {
		if e.TMs <= t {
			made = append(made, e)
		}
	}
	return draw(s, t, erased(made))
}

// Final draws the sheet as it was finished: every stroke that no erasure took away.
func Final(s *sample.Sample) (*image.Gray, error) { return At(s, math.MaxInt64) }

// Before draws the sheet just before its erasure i, the frame of that erasure (R11): the strokes
// written by the erasure's moment, the ones it takes away among them, without those the erasures
// before it took away. A stroke that begins at the erasure's very moment and that it does not take
// away is the next stroke, written after it.
func Before(s *sample.Sample, i int) (*image.Gray, error) {
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
	return draw(s, e.TMs, gone)
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

// point is a point of a stroke in sixteenths of a pixel.
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

func draw(s *sample.Sample, t int64, gone map[string]bool) (*image.Gray, error) {
	w, h := s.Sheet.Width, s.Sheet.Height
	if !(w > 0 && w <= MaxSide && h > 0 && h <= MaxSide) {
		return nil, fmt.Errorf("render: the sheet is %v × %v, and the renderer draws sheets above 0 and up to %d a side", w, h, MaxSide)
	}
	img := image.NewGray(image.Rect(0, 0, int(math.Ceil(w)), int(math.Ceil(h))))
	for i := range img.Pix {
		img.Pix[i] = 0xff
	}
	for _, st := range s.Strokes {
		if gone[st.ID] {
			continue
		}
		var pts []point
		// A stroke's points go in the order of its clock (internal/sample checks it).
		for _, p := range st.Points {
			if p.TMs > t {
				break
			}
			if !(p.X >= 0 && p.X <= w && p.Y >= 0 && p.Y <= h) {
				return nil, fmt.Errorf("render: the stroke %q has a point off the sheet, (%v, %v)", st.ID, p.X, p.Y)
			}
			pts = append(pts, point{int64(math.Round(p.X * sub)), int64(math.Round(p.Y * sub))})
		}
		if len(pts) > 0 {
			stroke(img, pts)
		}
	}
	return img, nil
}

// stroke draws a line through the points: within half the pen's width of each piece between two
// of them, which makes the ends and the joins round. A stroke of one point is a dot. A sample the
// line covers counts once, however many of its pieces cover it, so the line keeps one shade where
// it goes over itself.
func stroke(img *image.Gray, pts []point) {
	b := img.Bounds()
	lo, hi := pts[0], pts[0]
	for _, p := range pts[1:] {
		lo = point{min(lo.x, p.x), min(lo.y, p.y)}
		hi = point{max(hi.x, p.x), max(hi.y, p.y)}
	}
	x0, y0 := max(pixel(lo.x-half), b.Min.X), max(pixel(lo.y-half), b.Min.Y)
	x1, y1 := min(pixel(hi.x+half)+1, b.Max.X), min(pixel(hi.y+half)+1, b.Max.Y)
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
		px0, py0 := max(pixel(min(a.x, c.x)-half), x0), max(pixel(min(a.y, c.y)-half), y0)
		px1, py1 := min(pixel(max(a.x, c.x)+half)+1, x1), min(pixel(max(a.y, c.y)+half)+1, y1)
		for py := py0; py < py1; py++ {
			for px := px0; px < px1; px++ {
				k := (py-y0)*(x1-x0) + px - x0
				cx, cy := int64(px)*sub+sub/2, int64(py)*sub+sub/2
				switch {
				case cover[k] == full || !near(cx, cy, a, c, half+reach):
					continue
				case near(cx, cy, a, c, half-reach):
					cover[k] = full
					continue
				}
				var mask uint16
				for j := range grid {
					qy := int64(py)*sub + sub/grid/2 + int64(j)*sub/grid
					for i := range grid {
						qx := int64(px)*sub + sub/grid/2 + int64(i)*sub/grid
						if near(qx, qy, a, c, half) {
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
			*o = uint8((int(*o)*(grid*grid-n) + Ink*n + grid*grid/2) / (grid * grid))
		}
	}
}

// near reports whether the point q lies within r of the piece from a to c. On a sheet of up to
// MaxSide a side, no product here leaves 64 bits: the largest, cross², stays under 2⁶².
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
	return cross*cross <= r*r*l2
}

// pixel is the pixel a coordinate in sixteenths falls in, below zero too.
func pixel(v int64) int {
	if v < 0 {
		return int((v - sub + 1) / sub)
	}
	return int(v / sub)
}
