package provider

import (
	"bytes"
	"context"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math"
	"strings"
	"testing"
)

// fitted is a model that fits a photo to half its size, and plain one that does not fit it.
type plain struct{}

func (plain) Name() string                                   { return "plain" }
func (plain) Prices() Prices                                 { return Prices{} }
func (plain) Ask(context.Context, Request) (Response, error) { return Response{}, nil }
func (plain) Check(context.Context) error                    { return nil }

type fitted struct{ plain }

func (fitted) Fit(w, h int) (int, int) { return w / 2, h / 2 }

type broken struct{ plain }

func (broken) Fit(w, h int) (int, int) { return w * 2, h }

// page is a white picture of w × h px with a black stripe across its upper half.
func page(w, h int) *image.RGBA {
	m := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			c := color.RGBA{255, 255, 255, 255}
			if y < h/2 {
				c = color.RGBA{0, 0, 0, 255}
			}
			m.SetRGBA(x, y, c)
		}
	}
	return m
}

func encoded(t *testing.T, m image.Image, format string) []byte {
	t.Helper()
	var b bytes.Buffer
	var err error
	switch format {
	case "png":
		err = png.Encode(&b, m)
	case "gif":
		err = gif.Encode(&b, m, nil)
	default:
		err = jpeg.Encode(&b, m, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestPhoto(t *testing.T) {
	for _, format := range []string{"jpeg", "png"} {
		im, w, h, err := Photo(encoded(t, page(400, 300), format), fitted{})
		if err != nil || w != 200 || h != 150 || len(im.PNG) != 0 || len(im.JPEG) == 0 {
			t.Fatalf("%s: %d × %d, %v", format, w, h, err)
		}
		got, err := jpeg.Decode(bytes.NewReader(im.JPEG))
		if err != nil || got.Bounds().Dx() != 200 || got.Bounds().Dy() != 150 {
			t.Fatalf("%s: the JPEG sent is %v, %v", format, got.Bounds(), err)
		}
		// The stripe keeps its place: dark above the middle, light below.
		if r, _, _, _ := got.At(100, 30).RGBA(); r > 0x2000 {
			t.Errorf("%s: the stripe went light: %x", format, r)
		}
		if r, _, _, _ := got.At(100, 120).RGBA(); r < 0xd000 {
			t.Errorf("%s: the page went dark: %x", format, r)
		}
		if media, data := im.MediaType(); media != "image/jpeg" || len(data) == 0 {
			t.Errorf("%s: the media type is %s", format, media)
		}
	}
	// A model that does not fit a photo takes it at its size, encoded anew.
	if _, w, h, err := Photo(encoded(t, page(40, 30), "jpeg"), plain{}); err != nil || w != 40 || h != 30 {
		t.Errorf("unfitted: %d × %d, %v", w, h, err)
	}
}

// The metadata of the file do not reach the provider: the photo is encoded anew, and an APP1
// segment of Exif in it is not.
func TestPhotoDropsTheMetadata(t *testing.T) {
	plainJPEG := encoded(t, page(40, 30), "jpeg")
	exif := []byte("\xff\xe1\x00\x10Exif\x00\x00GPS-52.37N\x00")
	withExif := append(append(append([]byte{}, plainJPEG[:2]...), exif...), plainJPEG[2:]...)
	im, _, _, err := Photo(withExif, plain{})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(im.JPEG, []byte("Exif")) || bytes.Contains(im.JPEG, []byte("GPS")) {
		t.Error("the metadata went with the photo")
	}
}

func TestPhotoRefuses(t *testing.T) {
	for name, photo := range map[string][]byte{
		"no picture": []byte("a photo"),
		"a GIF":      encoded(t, page(4, 4), "gif"),
		"too large":  bytes.Repeat([]byte{0}, MaxPhoto+1),
		"cut short":  encoded(t, page(40, 30), "png")[:60],
	} {
		if _, _, _, err := Photo(photo, plain{}); err == nil {
			t.Errorf("%s: taken", name)
		}
	}
	// A fit larger than the photo is the adapter's fault, and fails.
	if _, _, _, err := Photo(encoded(t, page(40, 30), "jpeg"), broken{}); err == nil || !strings.Contains(err.Error(), "fits") {
		t.Errorf("a fit larger than the photo: %v", err)
	}
	// A PNG that claims more pixels than any camera gives is refused before they are allocated.
	if _, _, _, err := Photo(claimed(t, 16384, 16384), plain{}); err == nil || !strings.Contains(err.Error(), "16384 × 16384 px") {
		t.Errorf("a picture of 16384 × 16384 px: %v", err)
	}
}

// claimed is a PNG of 4 × 4 px whose header, its checksum made anew, claims w × h.
func claimed(t *testing.T, w, h uint32) []byte {
	t.Helper()
	p := encoded(t, page(4, 4), "png")
	binary.BigEndian.PutUint32(p[16:], w)
	binary.BigEndian.PutUint32(p[20:], h)
	binary.BigEndian.PutUint32(p[29:], crc32.ChecksumIEEE(p[12:29]))
	return p
}

// FitPatches is Anthropic's rule of its guide Vision: its examples, and both tiers.
func TestFitPatches(t *testing.T) {
	for _, tc := range []struct{ w, h, edge, most, wantW, wantH int }{
		{1075, 1520, 1568, 1568, 924, 1307},
		{1920, 1080, 1568, 1568, 1456, 819},
		{1000, 1000, 1568, 1568, 1000, 1000},
		{1075, 1520, 2576, 4784, 1075, 1520},
		{3840, 2160, 2576, 4784, 2576, 1449},
		{2160, 3840, 2576, 4784, 1449, 2576},
		{0, 10, 2576, 4784, 0, 10},
	} {
		if w, h := FitPatches(tc.w, tc.h, 28, tc.edge, tc.most); w != tc.wantW || h != tc.wantH {
			t.Errorf("%d × %d at %d, %d: %d × %d, want %d × %d", tc.w, tc.h, tc.edge, tc.most, w, h, tc.wantW, tc.wantH)
		}
	}
	// A phone's photo of 12 megapixels comes to fit the high tier's 4 784 patches.
	w, h := FitPatches(3024, 4032, 28, 2576, 4784)
	if (w+27)/28*((h+27)/28) > 4784 || h > 2576 || w*4032 > 3024*h+4032 {
		t.Errorf("12 MP: %d × %d", w, h)
	}
}

// The mean over the pixels a shrunk one covers keeps a thin line that a pick of one pixel in many
// would lose.
func TestShrinkKeepsAThinLine(t *testing.T) {
	m := image.NewRGBA(image.Rect(0, 0, 90, 90))
	for y := range 90 {
		for x := range 90 {
			m.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
		}
		m.SetRGBA(44, y, color.RGBA{0, 0, 0, 255})
	}
	got := shrink(m, 30, 30)
	if r := got.RGBAAt(14, 15).R; r > 200 {
		t.Errorf("the line went: %d", r)
	}
	if r := got.RGBAAt(5, 15).R; r != 255 {
		t.Errorf("the paper is %d", r)
	}
}

// A PNG is refused past a square of 3200 px a side, the long side app draws a photo at, and one of
// that square is not refused for its size: a header of 3200 × 3200 over the pixels of 4 × 4 fails
// at decoding, a header of 3201 × 3200 before it.
func TestPhotoCeiling(t *testing.T) {
	if _, _, _, err := Photo(claimed(t, 3201, 3200), plain{}); err == nil || !strings.Contains(err.Error(), "3201 × 3200 px") {
		t.Errorf("past the ceiling: %v", err)
	}
	if _, _, _, err := Photo(claimed(t, 3200, 3200), plain{}); err == nil || !strings.Contains(err.Error(), "cannot be decoded") {
		t.Errorf("at the ceiling: %v", err)
	}
}

// wholeShrink is shrink as the mean over a whole opaque copy of the source: the strips of shrink
// give the same pixels.
func wholeShrink(src image.Image, w, h int) *image.RGBA {
	b := src.Bounds()
	full := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	opaque(full, b.Dy(), src, 0)
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	sx, sy := float64(b.Dx())/float64(w), float64(b.Dy())/float64(h)
	for y := range h {
		y0, y1 := float64(y)*sy, float64(y+1)*sy
		for x := range w {
			x0, x1 := float64(x)*sx, float64(x+1)*sx
			var sum [3]float64
			var area float64
			for py := int(y0); py < int(math.Ceil(y1)) && py < b.Dy(); py++ {
				wy := math.Min(y1, float64(py+1)) - math.Max(y0, float64(py))
				for px := int(x0); px < int(math.Ceil(x1)) && px < b.Dx(); px++ {
					wx := math.Min(x1, float64(px+1)) - math.Max(x0, float64(px))
					i := full.PixOffset(px, py)
					for c := range 3 {
						sum[c] += wx * wy * float64(full.Pix[i+c])
					}
					area += wx * wy
				}
			}
			i := dst.PixOffset(x, y)
			for c := range 3 {
				dst.Pix[i+c] = uint8(math.Round(sum[c] / area))
			}
			dst.Pix[i+3] = 0xff
		}
	}
	return dst
}

// Shrinking by strips of rows gives the pixels of a whole copy: at scales whole and not, on a
// source whose bounds do not start at the origin, and with transparent pixels over white.
func TestShrinkByStrips(t *testing.T) {
	m := image.NewNRGBA(image.Rect(0, 0, 211, 157))
	for y := range 157 {
		for x := range 211 {
			m.SetNRGBA(x, y, color.NRGBA{uint8(x * 7), uint8(y * 13), uint8(x ^ y), uint8(255 - (x+y)%3*100)})
		}
	}
	sub := m.SubImage(image.Rect(5, 9, 200, 150))
	for _, size := range [][2]int{{70, 52}, {100, 75}, {195, 141}, {13, 9}, {1, 1}, {195, 50}} {
		for _, src := range []image.Image{m, sub} {
			w, h := size[0], size[1]
			if w > src.Bounds().Dx() || h > src.Bounds().Dy() {
				continue
			}
			if got, want := shrink(src, w, h), wholeShrink(src, w, h); !bytes.Equal(got.Pix, want.Pix) {
				t.Errorf("%v at %d × %d: the strips differ from the whole", src.Bounds(), w, h)
			}
		}
	}
}

// counted fits a photo to half its size, and counts its fits.
type counted struct {
	plain
	fits *int
}

func (c counted) Fit(w, h int) (int, int) { *c.fits++; return w / 2, h / 2 }

// Photos decodes a photo once and encodes it once for each size: two models of one fit get the same
// JPEG, a model of another fit its own; a photo that is no picture fails for each.
func TestPhotos(t *testing.T) {
	p := NewPhotos(encoded(t, page(400, 300), "png"))
	fits := 0
	a, w, h, err := p.For(counted{fits: &fits})
	if err != nil || w != 200 || h != 150 {
		t.Fatalf("the first: %d × %d, %v", w, h, err)
	}
	b, _, _, err := p.For(counted{fits: &fits})
	if err != nil || &a.JPEG[0] != &b.JPEG[0] || fits != 2 {
		t.Errorf("the second is encoded anew: %v, %d fits", err, fits)
	}
	c, w, h, err := p.For(plain{})
	if err != nil || w != 400 || h != 300 || bytes.Equal(c.JPEG, a.JPEG) {
		t.Errorf("unfitted: %d × %d, %v", w, h, err)
	}
	bad := NewPhotos([]byte("a photo"))
	for range 2 {
		if _, _, _, err := bad.For(plain{}); err == nil {
			t.Error("no picture: taken")
		}
	}
}

// noise is a picture of w × h px whose every pixel is its own colour.
func noise(w, h int) *image.RGBA {
	m := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range m.Pix {
		m.Pix[i] = uint8(i*37 + i/4*11)
	}
	for i := 3; i < len(m.Pix); i += 4 {
		m.Pix[i] = 0xff
	}
	return m
}

// A quarter turn moves the pixels as they are, clockwise: the top row becomes the right column.
func TestTurnByQuarters(t *testing.T) {
	src := noise(5, 3)
	for degrees, from := range map[int]func(x, y int) (int, int){
		90:   func(x, y int) (int, int) { return y, 3 - 1 - x },
		-90:  func(x, y int) (int, int) { return 5 - 1 - y, x },
		270:  func(x, y int) (int, int) { return 5 - 1 - y, x },
		180:  func(x, y int) (int, int) { return 5 - 1 - x, 3 - 1 - y },
		-180: func(x, y int) (int, int) { return 5 - 1 - x, 3 - 1 - y },
		0:    func(x, y int) (int, int) { return x, y },
	} {
		got := Turn(src, degrees)
		w, h := 5, 3
		if degrees%180 != 0 {
			w, h = 3, 5
		}
		if got.Bounds() != image.Rect(0, 0, w, h) {
			t.Fatalf("%d: %v", degrees, got.Bounds())
		}
		for y := range h {
			for x := range w {
				if sx, sy := from(x, y); got.RGBAAt(x, y) != src.RGBAAt(sx, sy) {
					t.Errorf("%d: the pixel %d, %d is not the source's %d, %d", degrees, x, y, sx, sy)
				}
			}
		}
	}
	back := Turn(Turn(Turn(Turn(src, 90), 90), 90), 90)
	if !bytes.Equal(back.Pix, src.Pix) {
		t.Error("four quarter turns are not the picture")
	}
}

// A turn of 30° is onto the smallest upright rectangle that holds the picture, its corners white,
// and a mark of the picture lands where the turn about the centre takes it.
func TestTurnBy30(t *testing.T) {
	const w, h = 400, 300
	src := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range src.Pix {
		src.Pix[i] = map[bool]uint8{true: 0xff, false: 0x80}[i%4 == 3]
	}
	for y := 74; y <= 76; y++ {
		for x := 299; x <= 301; x++ {
			src.SetRGBA(x, y, color.RGBA{0, 0, 0, 255})
		}
	}
	got := Turn(src, 30)
	// 400 cos 30° + 300 sin 30° = 496.4, 400 sin 30° + 300 cos 30° = 459.8.
	if got.Bounds() != image.Rect(0, 0, 496, 460) {
		t.Fatalf("turned onto %v", got.Bounds())
	}
	white := color.RGBA{255, 255, 255, 255}
	for _, c := range []image.Point{{0, 0}, {495, 0}, {0, 459}, {495, 459}} {
		if got.RGBAAt(c.X, c.Y) != white {
			t.Errorf("the corner %v is %v", c, got.RGBAAt(c.X, c.Y))
		}
	}
	if mid := got.RGBAAt(248, 230); mid.R != 0x80 || mid.A != 0xff {
		t.Errorf("the middle is %v", mid)
	}
	// The mark's centre, 300.5, 75.5, from the centre 200, 150, turned clockwise about the new one.
	theta := math.Pi / 6
	dx, dy := 300.5-200, 75.5-150
	want := image.Pt(int(dx*math.Cos(theta)-dy*math.Sin(theta)+248), int(dx*math.Sin(theta)+dy*math.Cos(theta)+230))
	darkest, at := 256, image.Point{}
	for y := range 460 {
		for x := range 496 {
			if r := int(got.RGBAAt(x, y).R); r < darkest {
				darkest, at = r, image.Pt(x, y)
			}
		}
	}
	if d := at.Sub(want); darkest > 0x40 || d.X < -1 || d.X > 1 || d.Y < -1 || d.Y > 1 {
		t.Errorf("the mark is at %v, %d dark; want %v", at, darkest, want)
	}
}

// A turned picture keeps within maxPhotoPixels: a strip that turned at its size would take
// 3260 × 3260 px is shrunk to 3200 × 3200, and one that would not is turned at its size.
func TestTurnKeepsWithinThePixels(t *testing.T) {
	if got := Turn(page(4600, 10), 45); got.Bounds() != image.Rect(0, 0, 3200, 3200) || got.RGBAAt(0, 0) != (color.RGBA{255, 255, 255, 255}) {
		t.Errorf("turned onto %v", got.Bounds())
	}
	if got := Turn(page(200, 20), -45); got.Bounds() != image.Rect(0, 0, 156, 156) {
		t.Errorf("a small strip turned onto %v", got.Bounds())
	}
}

// Turned turns the photo decoded once, once for each angle, and each model takes it as For gives
// it; a turn of 0 is the photo itself, and a photo that is no picture fails turned too.
func TestPhotosTurned(t *testing.T) {
	p := NewPhotos(encoded(t, page(400, 300), "png"))
	if p.Turned(0) != p {
		t.Error("a turn of 0 is another photo")
	}
	q := p.Turned(90)
	if p.Turned(90) != q || p.Turned(-90) == q {
		t.Error("a turn is not kept by its angle")
	}
	if _, w, h, err := q.For(plain{}); err != nil || w != 300 || h != 400 {
		t.Errorf("turned: %d × %d, %v", w, h, err)
	}
	if _, w, h, err := p.For(plain{}); err != nil || w != 400 || h != 300 {
		t.Errorf("the photo itself: %d × %d, %v", w, h, err)
	}
	if _, _, _, err := NewPhotos([]byte("a photo")).Turned(30).For(plain{}); err == nil {
		t.Error("no picture: turned")
	}
}
