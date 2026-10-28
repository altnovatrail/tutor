package essay

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image/png"
	"os"
	"testing"

	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/sample"
)

// readEssays reads the essays of the measurement.
func readEssays(t *testing.T) *sample.Essays {
	t.Helper()
	f, err := os.Open("../../eval/essays.json")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	es, err := sample.ReadEssays(f)
	if err != nil {
		t.Fatal(err)
	}
	return es
}

// What a model sees of the first essay at 32 px is what it was: its lines, and a crop the size of
// the PNG, within the tokens Claude takes without scaling the image down, whose hash holds it on
// every machine. A digit no crop takes is refused.
func TestPictureOf(t *testing.T) {
	e := &readEssays(t).Essays[0]
	pic, err := PictureOf(e, 1, render.Digit)
	if err != nil {
		t.Fatal(err)
	}
	if pic.Lines[0] != *e.Title || len(pic.Lines) < 2 {
		t.Errorf("lines %q", pic.Lines)
	}
	img, err := png.Decode(bytes.NewReader(pic.PNG))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != pic.Crop.Width || b.Dy() != pic.Crop.Height || render.Tokens(b.Dx(), b.Dy()) > 1568 {
		t.Errorf("the crop is %d × %d, and its picture %v", pic.Crop.Width, pic.Crop.Height, b)
	}
	if got, want := fmt.Sprintf("%x", sha256.Sum256(pic.PNG)), "217401ea15971f83545c16ff8d689f9de0fe939f136f4b17d6eb89677ddc1fd2"; got != want {
		t.Errorf("the crop of essay 1 hashes to %s, want %s", got, want)
	}
	if _, err := PictureOf(e, 1, 0); err == nil {
		t.Error("a digit 0 px high is taken")
	}
}
