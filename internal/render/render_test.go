package render_test

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	"image/png"
	"math"
	"testing"

	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/synth"
)

// The pictures of the synthetic samples, by the SHA-256 of their pixels: each sheet as it was
// finished, under its scenario, and the frame before each of its erasures, under the scenario and
// the erasure's index. The drawing is in whole numbers, and the synthetic samples are drawn without
// fused floating-point operations, so the hashes hold on every machine. A change of the renderer or
// of the synthetic samples changes them, and the pictures are then looked at again.
var golden = map[string]string{
	"correct":               "5ce4e235f3131d398e6a73464660b8444ab988e17c05d40d16670d213cd1a38e",
	"error":                 "7c9000d77e20daf3f75047211a78a9a96fa12214f94e74da381241c35d62136b",
	"similar-digits":        "9ed7ae974add809b6f1349709c86f6dbf8a29b4aa317421c44e081c2524ea0a5",
	"illegible-crossed-out": "d28c7a3b9fcb5236abc521b10002794e68c17df3d8a0d2f6b90eec937ef17080",
	"answer-not-last":       "a6c6489067edd7a4b5702ab38325fbc78269d052d726a2522ed2c104a296e4c6",
	"nearly-empty":          "be1d58a70fe0acfa32813e324e04d0b5bb43af7ce0d3b17611b6bb28cd020421",
	"mirrored-digits":       "ac69690725f4fde3dd801882752a2a838daf0ce6e4493b15bea02d9b98af1c02",
	"erasure-fixed":         "c482ace03de5602aa46acf4fb7f9b5072316230077e7946d8d94ab86e2a3da8a",
	"erasure-fixed/0":       "d94ef5f546871f1deec8030076b68082a908b81eef6e7de2254b33639d5ec505",
	"erasure-fixed/1":       "b79154e8e6bbee37137b7d96c01a39cc6dc97e219d4f5794f12990d6cbca993e",
	"erasure-broken":        "1190b3b06b036c2e16e0335fc1709672d4d0cf92f3475eb749c0a8dcffd92644",
	"erasure-broken/0":      "3d3870b673ec6328d6b472eaa07ce7d1304705deba9440633d2e8e62c86b37a9",
	"injection":             "f135c2fb40c802a14601bf6df04c6db7c11bf49f38be16380f76a5b8c8645ba6",
}

func TestGolden(t *testing.T) {
	ok := drawn(t)
	seen := map[string]bool{}
	check := func(name string, img *image.Gray) {
		seen[name] = true
		got := fmt.Sprintf("%x", sha256.Sum256(img.Pix))
		if want, has := golden[name]; !has {
			t.Errorf("%s: no golden picture, and the pixels hash to %s", name, got)
		} else if got != want {
			t.Errorf("%s: the picture's pixels hash to %s, want %s", name, got, want)
		}
	}
	for _, s := range synth.Samples() {
		if s.Sheet != (sample.Sheet{Width: 1000, Height: 600}) {
			t.Fatalf("%s: the synthetic sheet is %v, want the board's 1000 × 600", s.Scenario, s.Sheet)
		}
		check(string(s.Scenario), ok(render.Final(s)))
		for i := range s.Erasures {
			check(fmt.Sprintf("%s/%d", s.Scenario, i), ok(render.Before(s, i)))
		}
	}
	for name := range golden {
		if !seen[name] {
			t.Errorf("%s: a golden picture of no synthetic sheet", name)
		}
	}
}

// drawn fails the test on an error of the renderer.
func drawn(t *testing.T) func(*image.Gray, error) *image.Gray {
	return func(img *image.Gray, err error) *image.Gray {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return img
	}
}

// line is a sample of strokes on the board's sheet.
func line(strokes []sample.Stroke, erasures ...sample.Erasure) *sample.Sample {
	return &sample.Sample{Sheet: sample.Sheet{Width: 1000, Height: 600}, Strokes: strokes, Erasures: erasures}
}

// at is a stroke through the points xy, begun at the moment t, with the ID "s-t".
func at(t int64, xy ...float64) sample.Stroke {
	st := sample.Stroke{ID: fmt.Sprintf("s-%d", t)}
	for i := 0; i < len(xy); i += 2 {
		st.Points = append(st.Points, sample.Point{X: xy[i], Y: xy[i+1], TMs: t + int64(i), Pressure: 0.5})
	}
	return st
}

// The line is the board's: #111, 3 px wide round the points, on white.
func TestLook(t *testing.T) {
	ok := drawn(t)
	img := ok(render.Final(line([]sample.Stroke{at(0, 100, 100, 200, 100)})))
	if img.Bounds() != image.Rect(0, 0, 1000, 600) {
		t.Fatalf("bounds %v, want the sheet's 1000 × 600", img.Bounds())
	}
	for _, c := range []struct {
		x, y int
		want uint8
	}{
		{150, 99, render.Ink}, {150, 100, render.Ink}, // the line at y = 100 covers [98.5, 101.5]
		{150, 98, 0x88}, {150, 101, 0x88}, // half of these rows
		{150, 97, 0xff}, {150, 102, 0xff},
		{50, 100, 0xff}, {300, 300, 0xff},
		// The end reaches 1.5 px past the last point, and it is round: the corner past it is white.
		{200, 100, render.Ink}, {201, 100, 0xa6}, {202, 100, 0xff}, {201, 98, 0xff},
	} {
		if got := img.GrayAt(c.x, c.y).Y; got != c.want {
			t.Errorf("pixel (%d, %d) = %#x, want %#x", c.x, c.y, got, c.want)
		}
	}
	// A stroke of one point is a dot: at a corner of pixels, it covers the four round it.
	dot := ok(render.Final(line([]sample.Stroke{at(0, 500, 300)})))
	for _, c := range []struct {
		x, y int
		want uint8
	}{{500, 300, render.Ink}, {499, 299, render.Ink}, {501, 300, 0xa6}, {501, 301, 0xff}, {502, 300, 0xff}} {
		if got := dot.GrayAt(c.x, c.y).Y; got != c.want {
			t.Errorf("the dot's pixel (%d, %d) = %#x, want %#x", c.x, c.y, got, c.want)
		}
	}
	// A line that goes back over itself keeps one shade: the half-covered row is not inked twice.
	back := ok(render.Final(line([]sample.Stroke{at(0, 100, 100, 200, 100, 100, 100)})))
	if got := back.GrayAt(150, 98).Y; got != 0x88 {
		t.Errorf("where the line goes back over itself = %#x, want 0x88, as on a line drawn once", got)
	}
}

// A stroke an erasure took away is gone from the finished sheet, and there on the frame before
// the erasure; a moment before the first point the sheet is white.
func TestMoments(t *testing.T) {
	ok := drawn(t)
	s := line([]sample.Stroke{at(0, 100, 100, 150, 100), at(2000, 100, 300, 150, 300)},
		sample.Erasure{TMs: 1000, Strokes: []string{"s-0"}})
	final, before := ok(render.Final(s)), ok(render.Before(s, 0))
	if final.GrayAt(125, 100).Y != 0xff || final.GrayAt(125, 300).Y != render.Ink {
		t.Errorf("the finished sheet: the erased stroke %#x, the other %#x; want it gone and the other there",
			final.GrayAt(125, 100).Y, final.GrayAt(125, 300).Y)
	}
	if before.GrayAt(125, 100).Y != render.Ink || before.GrayAt(125, 300).Y != 0xff {
		t.Errorf("the frame before the erasure: the erased stroke %#x, the later one %#x; want it there and the later one not yet",
			before.GrayAt(125, 100).Y, before.GrayAt(125, 300).Y)
	}
	if !bytes.Equal(ok(render.At(s, -1)).Pix, ok(render.At(line(nil), 0)).Pix) {
		t.Error("the sheet before its first point is not white")
	}
	// Midway through a stroke, the points written by then: its first point alone, a dot.
	half := ok(render.At(s, 1))
	if half.GrayAt(100, 100).Y != render.Ink || half.GrayAt(140, 100).Y != 0xff {
		t.Errorf("midway through the first stroke: %#x at its start and %#x at its end", half.GrayAt(100, 100).Y, half.GrayAt(140, 100).Y)
	}
	// The frame of a second erasure keeps what the first took away gone.
	two := line([]sample.Stroke{at(0, 100, 100), at(10, 300, 100)},
		sample.Erasure{TMs: 100, Strokes: []string{"s-0"}}, sample.Erasure{TMs: 200, Strokes: []string{"s-10"}})
	if f := ok(render.Before(two, 1)); f.GrayAt(100, 100).Y != 0xff || f.GrayAt(300, 100).Y == 0xff {
		t.Errorf("the frame of the second erasure: %#x, %#x; want the first stroke gone and the second there", f.GrayAt(100, 100).Y, f.GrayAt(300, 100).Y)
	}
}

// Within the millisecond of an erasure: a stroke begun at that moment is the next one, so it is
// not on the frame but is on the sheet at that moment, and a stroke the erasure takes away is on
// the frame, even begun at that moment.
func TestTheErasuresMoment(t *testing.T) {
	ok := drawn(t)
	s := line([]sample.Stroke{at(0, 100, 100), at(1000, 300, 100)}, sample.Erasure{TMs: 1000, Strokes: []string{"s-0"}})
	if f := ok(render.Before(s, 0)); f.GrayAt(100, 100).Y != render.Ink || f.GrayAt(300, 100).Y != 0xff {
		t.Errorf("the frame: the erased stroke %#x, the next %#x; want the erased one there and the next not yet", f.GrayAt(100, 100).Y, f.GrayAt(300, 100).Y)
	}
	if a := ok(render.At(s, 1000)); a.GrayAt(100, 100).Y != 0xff || a.GrayAt(300, 100).Y != render.Ink {
		t.Errorf("the sheet at the erasure's moment: the erased stroke %#x, the next %#x; want the erased one gone and the next there", a.GrayAt(100, 100).Y, a.GrayAt(300, 100).Y)
	}
	same := line([]sample.Stroke{at(1000, 100, 100)}, sample.Erasure{TMs: 1000, Strokes: []string{"s-1000"}})
	if f := ok(render.Before(same, 0)); f.GrayAt(100, 100).Y != render.Ink {
		t.Errorf("the frame of an erasure that takes away a stroke of its own moment: %#x, want the stroke there", f.GrayAt(100, 100).Y)
	}
}

// The renderer refuses a sheet it cannot draw, a point off the sheet and an erasure the sample does
// not have; on the largest sheet it draws, a line across it is right.
func TestBounds(t *testing.T) {
	for _, c := range []struct {
		name string
		s    *sample.Sample
	}{
		{"a sheet wider than MaxSide", &sample.Sample{Sheet: sample.Sheet{Width: render.MaxSide + 1, Height: 600}}},
		{"a sheet of no height", &sample.Sample{Sheet: sample.Sheet{Width: 1000}}},
		{"a sheet of NaN", &sample.Sample{Sheet: sample.Sheet{Width: math.NaN(), Height: 600}}},
		{"a point off the sheet", line([]sample.Stroke{at(0, 100, 100, 1000.5, 100)})},
	} {
		if _, err := render.Final(c.s); err == nil {
			t.Errorf("%s: drawn", c.name)
		}
	}
	for _, i := range []int{-1, 1} {
		if _, err := render.Before(line(nil, sample.Erasure{TMs: 0}), i); err == nil {
			t.Errorf("the frame of erasure %d of a sample with one: drawn", i)
		}
	}
	side := float64(render.MaxSide)
	img := drawn(t)(render.Final(&sample.Sample{Sheet: sample.Sheet{Width: side, Height: side},
		Strokes: []sample.Stroke{at(0, 0, 0, side, side)}}))
	for _, c := range []struct {
		x, y int
		want uint8
	}{{1024, 1024, render.Ink}, {2000, 2000, render.Ink}, {1024, 1030, 0xff}, {0, render.MaxSide - 1, 0xff}, {render.MaxSide - 1, 0, 0xff}} {
		if got := img.GrayAt(c.x, c.y).Y; got != c.want {
			t.Errorf("the diagonal of the largest sheet: pixel (%d, %d) = %#x, want %#x", c.x, c.y, got, c.want)
		}
	}
}

func TestPNG(t *testing.T) {
	img := drawn(t)(render.Final(synth.Sample(sample.ScenarioCorrect)))
	data, err := render.PNG(img)
	if err != nil {
		t.Fatal(err)
	}
	back, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	g, ok := back.(*image.Gray)
	switch {
	case !ok:
		t.Errorf("the PNG decodes to %T, not to a picture in shades of grey", back)
	case !bytes.Equal(g.Pix, img.Pix):
		t.Error("the PNG decodes to other pixels than those drawn")
	}
}

func TestTokens(t *testing.T) {
	for _, c := range []struct{ w, h, want int }{{1000, 600, 792}, {1000, 1000, 1296}, {28, 28, 1}, {29, 28, 2}, {1, 1, 1}} {
		if got := render.Tokens(c.w, c.h); got != c.want {
			t.Errorf("Tokens(%d, %d) = %d, want %d", c.w, c.h, got, c.want)
		}
	}
}
