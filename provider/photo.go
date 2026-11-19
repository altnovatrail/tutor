package provider

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"math"
)

// Fitter is a model that takes a photo at a size of its own: an adapter whose provider would shrink
// a larger picture itself says how large a photo comes to the model unshrunk, so that the model's
// pixels are the photo's and a box it gives in them lands where it points (R134).
type Fitter interface {
	// Fit is the largest size, keeping the aspect, at which a photo of width × height px comes to
	// the model as it is sent: the size itself when it fits.
	Fit(width, height int) (int, int)
}

// MaxPhoto is the most bytes of a photo that Photo decodes: the 4 MiB of FindTasks of spec's
// contract between app and api (contracts/README.md spec, section 15).
const MaxPhoto = 4 << 20

// maxPhotoPixels bounds the pixels a photo may decode to, so that a small file that claims a huge
// picture is refused before its pixels are allocated: 50 megapixels, more than a phone's camera
// gives.
const maxPhotoPixels = 50_000_000

// photoQuality is the quality of the JPEG a photo is sent as: high enough that the print of a page
// keeps its strokes, as Anthropic warns that heavy JPEG compression makes text hard to read.
const photoQuality = 90

// Photo is a photo of a page as a request sends it to model: decoded from a JPEG or a PNG of at most
// MaxPhoto bytes, shrunk to the size the model takes it at unshrunk when the model is a Fitter, and
// encoded anew as a JPEG, so that no metadata of the file it came in — where it was taken, the
// device, the time — reaches the provider (privacy 6 spec). It returns the picture and its size in
// pixels, by which the text of the request names it and a box the model gives is placed (Place).
func Photo(photo []byte, m Model) (Image, int, int, error) {
	if len(photo) > MaxPhoto {
		return Image{}, 0, 0, fmt.Errorf("the photo takes %d bytes, more than %d", len(photo), MaxPhoto)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(photo))
	if err != nil {
		return Image{}, 0, 0, fmt.Errorf("the photo is no picture: %w", err)
	}
	if format != "jpeg" && format != "png" {
		return Image{}, 0, 0, fmt.Errorf("the photo is a %s, not a JPEG or a PNG", format)
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width*cfg.Height > maxPhotoPixels {
		return Image{}, 0, 0, fmt.Errorf("the photo is %d × %d px", cfg.Width, cfg.Height)
	}
	var src image.Image
	if format == "jpeg" {
		src, err = jpeg.Decode(bytes.NewReader(photo))
	} else {
		src, err = png.Decode(bytes.NewReader(photo))
	}
	if err != nil {
		return Image{}, 0, 0, fmt.Errorf("the photo cannot be decoded: %w", err)
	}
	w, h := cfg.Width, cfg.Height
	if f, ok := m.(Fitter); ok {
		w, h = f.Fit(w, h)
	}
	if w < 1 || h < 1 || w > cfg.Width || h > cfg.Height {
		return Image{}, 0, 0, fmt.Errorf("the model fits a photo of %d × %d px to %d × %d", cfg.Width, cfg.Height, w, h)
	}
	var out bytes.Buffer
	if err := jpeg.Encode(&out, shrink(src, w, h), &jpeg.Options{Quality: photoQuality}); err != nil {
		return Image{}, 0, 0, fmt.Errorf("the photo cannot be encoded: %w", err)
	}
	return Image{JPEG: out.Bytes()}, w, h, nil
}

// shrink is the picture at w × h px, no larger than it is: each pixel the mean of the source's
// pixels it covers, weighted by how much of each it covers, which keeps the thin strokes of print
// that a pick of one pixel in many would lose. A picture at its own size is only made opaque RGBA.
func shrink(src image.Image, w, h int) *image.RGBA {
	b := src.Bounds()
	full := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	// A page photo has no transparency that matters; white under it keeps a PNG's empty corners
	// white, as the page is.
	draw.Draw(full, full.Bounds(), image.White, image.Point{}, draw.Src)
	draw.Draw(full, full.Bounds(), src, b.Min, draw.Over)
	if w == b.Dx() && h == b.Dy() {
		return full
	}
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

// FitPatches is the largest size, keeping the aspect of width × height px, whose patches of side px
// — ⌈w / side⌉ × ⌈h / side⌉ — are no more than most, and neither of whose sides, rounded up to a
// whole patch, exceeds edge; the size itself when it fits. It is the rule by which Anthropic gives
// the size it shrinks a picture to, its reference implementation searched along the long edge, the
// short one rounded half to even (sources 1.19); an adapter gives its provider's side, edge and
// most.
func FitPatches(width, height, side, edge, most int) (int, int) {
	fits := func(w, h int) bool {
		pw, ph := (w+side-1)/side, (h+side-1)/side
		return pw*side <= edge && ph*side <= edge && pw*ph <= most
	}
	if width < 1 || height < 1 || fits(width, height) {
		return width, height
	}
	if height > width {
		h, w := FitPatches(height, width, side, edge, most)
		return w, h
	}
	aspect := float64(width) / float64(height)
	short := func(long int) int { return max(int(math.RoundToEven(float64(long)/aspect)), 1) }
	lo, hi := 1, width // lo fits; hi does not
	for lo+1 < hi {
		mid := (lo + hi) / 2
		if fits(mid, short(mid)) {
			lo = mid
		} else {
			hi = mid
		}
	}
	return lo, short(lo)
}

// errNoPicture is a picture of a request with neither its PNG nor its JPEG.
var errNoPicture = errors.New("has no PNG and no JPEG")
