package provider

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
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
	huge := encoded(t, page(4, 4), "png")
	huge[16], huge[17], huge[18], huge[19] = 0, 0, 0x40, 0
	huge[20], huge[21], huge[22], huge[23] = 0, 0, 0x40, 0
	if _, _, _, err := Photo(huge, plain{}); err == nil {
		t.Error("a picture of 16384 × 16384 px was taken")
	}
}

// FitPatches is Anthropic's rule (sources 1.19): its examples, and both tiers.
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
