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
// pixels are the photo's and a box it gives in them lands where it points.
type Fitter interface {
	// Fit is the largest size, keeping the aspect, at which a photo of width × height px comes to
	// the model as it is sent: the size itself when it fits.
	Fit(width, height int) (int, int)
}

// MaxPhoto is the most bytes of a photo that Photo decodes: the 4 MiB FindTasks of spec's contract
// between app and api takes.
const MaxPhoto = 4 << 20

// maxPhotoPixels bounds the pixels a photo may decode to, so that a small file that claims a huge
// picture is refused before its pixels are allocated: a square of 3200 px a side, the long side app
// draws a photo at, so that no photo app sends, nor a task's crop cut from one, is refused, and a
// decoded photo of 8 bits a channel takes some 40 MB at most.
const maxPhotoPixels = 3200 * 3200

// photoQuality is the quality of the JPEG a photo is sent as: high enough that the print of a page
// keeps its strokes, as Anthropic warns that heavy JPEG compression makes text hard to read.
const photoQuality = 90

// Photo is a photo of a page as a request sends it to model: decoded from a JPEG or a PNG of at most
// MaxPhoto bytes, shrunk to the size the model takes it at unshrunk when the model is a Fitter, and
// encoded anew as a JPEG, so that no metadata of the file it came in — where it was taken, the
// device, the time — reaches the provider. It returns the picture and its size in pixels, by which
// the text of the request names it and a box the model gives is placed (Place). The EXIF
// orientation of a JPEG is not applied: app draws every photo upright and sends no metadata.
func Photo(photo []byte, m Model) (Image, int, int, error) {
	return NewPhotos(photo).For(m)
}

// Photos is one photo as each model of a cascade takes it, as Photo makes it: decoded once, on the
// first For or Turned, turned once for each angle, and encoded once for each size a model fits it
// to. It is not safe for concurrent use.
type Photos struct {
	photo  []byte
	src    image.Image
	err    error
	sized  map[image.Point]Image
	turned map[int]*Photos
}

// NewPhotos is the photo, not yet decoded.
func NewPhotos(photo []byte) *Photos {
	return &Photos{photo: photo, sized: map[image.Point]Image{}, turned: map[int]*Photos{}}
}

// decode decodes the photo on its first use: a photo that failed to decode fails each time.
func (p *Photos) decode() error {
	if p.src == nil && p.err == nil {
		p.src, p.err = decoded(p.photo)
	}
	return p.err
}

// For is the photo as Photo gives it to m: a photo that failed to decode fails each time.
func (p *Photos) For(m Model) (Image, int, int, error) {
	if err := p.decode(); err != nil {
		return Image{}, 0, 0, err
	}
	width, height := p.src.Bounds().Dx(), p.src.Bounds().Dy()
	w, h := width, height
	if f, ok := m.(Fitter); ok {
		w, h = f.Fit(w, h)
	}
	if w < 1 || h < 1 || w > width || h > height {
		return Image{}, 0, 0, fmt.Errorf("the model fits a photo of %d × %d px to %d × %d", width, height, w, h)
	}
	if im, ok := p.sized[image.Pt(w, h)]; ok {
		return im, w, h, nil
	}
	var out bytes.Buffer
	if err := jpeg.Encode(&out, shrink(p.src, w, h), &jpeg.Options{Quality: photoQuality}); err != nil {
		return Image{}, 0, 0, fmt.Errorf("the photo cannot be encoded: %w", err)
	}
	im := Image{JPEG: out.Bytes()}
	p.sized[image.Pt(w, h)] = im
	return im, w, h, nil
}

// Turned is the photo turned by degrees clockwise, from -179 to 180, as Turn turns it, for each
// model as For gives it: the picture decoded once, turned once for each angle. A turn of 0 is the
// photo itself, and a photo that failed to decode fails each time.
func (p *Photos) Turned(degrees int) *Photos {
	if degrees == 0 {
		return p
	}
	if t, ok := p.turned[degrees]; ok {
		return t
	}
	t := &Photos{sized: map[image.Point]Image{}, turned: map[int]*Photos{}}
	if t.err = p.decode(); t.err == nil {
		t.src = Turn(p.src, degrees)
	}
	p.turned[degrees] = t
	return t
}

// Turn is src turned by degrees clockwise on the screen, its y down, about its centre onto the
// smallest upright rectangle that holds it, W' = round(W·|cos θ| + H·|sin θ|) by
// H' = round(W·|sin θ| + H·|cos θ|), the corners white, as app turns its photo by the turn
// FindTasks of spec's contract gives: each pixel's centre is taken back to the source
// and sampled bilinearly, white beyond it. A quarter turn moves the pixels as they are. A turned
// picture of more than maxPhotoPixels is shrunk to them, keeping its aspect, so that it takes no
// more memory than the largest photo decoded: a turn of 45° of a photo of 3200 × 2400 px would
// take some 3960 × 3960. The frames are in thousandths, at any scale, and the model is sent a
// picture smaller still.
func Turn(src image.Image, degrees int) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	full := image.NewRGBA(image.Rect(0, 0, w, h))
	opaque(full, h, src, 0)
	degrees = ((degrees % 360) + 360) % 360
	if degrees%90 == 0 {
		return quarter(full, degrees/90)
	}
	theta := float64(degrees) * math.Pi / 180
	sin, cos := math.Sin(theta), math.Cos(theta)
	tw := math.Round(float64(w)*math.Abs(cos) + float64(h)*math.Abs(sin))
	th := math.Round(float64(w)*math.Abs(sin) + float64(h)*math.Abs(cos))
	scale := math.Min(1, math.Sqrt(maxPhotoPixels/(tw*th)))
	dw, dh := max(int(math.Round(tw*scale)), 1), max(int(math.Round(th*scale)), 1)
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	cx, cy := float64(w)/2, float64(h)/2
	for y := range dh {
		dy := (float64(y)+0.5)*th/float64(dh) - th/2
		for x := range dw {
			dx := (float64(x)+0.5)*tw/float64(dw) - tw/2
			sx, sy := dx*cos+dy*sin+cx, -dx*sin+dy*cos+cy
			bilinear(dst.Pix[dst.PixOffset(x, y):], full, sx, sy)
		}
	}
	return dst
}

// quarter is src turned by n quarter turns clockwise, each pixel moved as it is.
func quarter(src *image.RGBA, n int) *image.RGBA {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	dw, dh := w, h
	if n%2 == 1 {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := range dh {
		for x := range dw {
			sx, sy := x, y
			switch n {
			case 1:
				sx, sy = y, h-1-x
			case 2:
				sx, sy = w-1-x, h-1-y
			case 3:
				sx, sy = w-1-y, x
			}
			copy(dst.Pix[dst.PixOffset(x, y):dst.PixOffset(x, y)+4], src.Pix[src.PixOffset(sx, sy):])
		}
	}
	return dst
}

// bilinear writes into px the colour of src at the point x, y, its pixels' centres at the halves,
// the four pixels nearest it weighed by how near each is, a pixel beyond src white.
func bilinear(px []uint8, src *image.RGBA, x, y float64) {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	if x < -1 || y < -1 || x > float64(w)+1 || y > float64(h)+1 {
		copy(px, []uint8{0xff, 0xff, 0xff, 0xff})
		return
	}
	fx, fy := x-0.5, y-0.5
	x0, y0 := int(math.Floor(fx)), int(math.Floor(fy))
	ax, ay := fx-float64(x0), fy-float64(y0)
	var sum [3]float64
	for j := range 2 {
		wy := ay
		if j == 0 {
			wy = 1 - ay
		}
		for i := range 2 {
			wx := ax
			if i == 0 {
				wx = 1 - ax
			}
			sx, sy := x0+i, y0+j
			for c := range 3 {
				v := 255.0
				if sx >= 0 && sx < w && sy >= 0 && sy < h {
					v = float64(src.Pix[src.PixOffset(sx, sy)+c])
				}
				sum[c] += wx * wy * v
			}
		}
	}
	for c := range 3 {
		px[c] = uint8(math.Round(sum[c]))
	}
	px[3] = 0xff
}

// decoded is the picture of a JPEG or a PNG of at most MaxPhoto bytes and maxPhotoPixels pixels.
func decoded(photo []byte) (image.Image, error) {
	if len(photo) > MaxPhoto {
		return nil, fmt.Errorf("the photo takes %d bytes, more than %d", len(photo), MaxPhoto)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(photo))
	if err != nil {
		return nil, fmt.Errorf("the photo is no picture: %w", err)
	}
	if format != "jpeg" && format != "png" {
		return nil, fmt.Errorf("the photo is a %s, not a JPEG or a PNG", format)
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width*cfg.Height > maxPhotoPixels {
		return nil, fmt.Errorf("the photo is %d × %d px", cfg.Width, cfg.Height)
	}
	var src image.Image
	if format == "jpeg" {
		src, err = jpeg.Decode(bytes.NewReader(photo))
	} else {
		src, err = png.Decode(bytes.NewReader(photo))
	}
	if err != nil {
		return nil, fmt.Errorf("the photo cannot be decoded: %w", err)
	}
	return src, nil
}

// opaque draws rows of src, from its row top, over white into dst, from dst's top: a page photo has
// no transparency that matters, and white under it keeps a PNG's empty corners white, as the page
// is.
func opaque(dst *image.RGBA, rows int, src image.Image, top int) {
	r := image.Rect(0, 0, dst.Bounds().Dx(), rows)
	draw.Draw(dst, r, image.White, image.Point{}, draw.Src)
	draw.Draw(dst, r, src, image.Pt(src.Bounds().Min.X, src.Bounds().Min.Y+top), draw.Over)
}

// shrink is the picture at w × h px, no larger than it is: each pixel the mean of the source's
// pixels it covers, weighted by how much of each it covers, which keeps the thin strokes of print
// that a pick of one pixel in many would lose. A picture at its own size is only made opaque RGBA.
// A smaller one is read a strip of the source's rows at a time, so that no opaque copy of the whole
// source is made.
func shrink(src image.Image, w, h int) *image.RGBA {
	b := src.Bounds()
	if w == b.Dx() && h == b.Dy() {
		full := image.NewRGBA(image.Rect(0, 0, w, h))
		opaque(full, h, src, 0)
		return full
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	sx, sy := float64(b.Dx())/float64(w), float64(b.Dy())/float64(h)
	strip := image.NewRGBA(image.Rect(0, 0, b.Dx(), int(math.Ceil(sy))+2))
	for y := range h {
		y0, y1 := float64(y)*sy, float64(y+1)*sy
		top, bottom := int(y0), min(int(math.Ceil(y1)), b.Dy(), int(y0)+strip.Bounds().Dy())
		opaque(strip, bottom-top, src, top)
		for x := range w {
			x0, x1 := float64(x)*sx, float64(x+1)*sx
			var sum [3]float64
			var area float64
			for py := top; py < bottom; py++ {
				wy := math.Min(y1, float64(py+1)) - math.Max(y0, float64(py))
				for px := int(x0); px < int(math.Ceil(x1)) && px < b.Dx(); px++ {
					wx := math.Min(x1, float64(px+1)) - math.Max(x0, float64(px))
					i := strip.PixOffset(px, py-top)
					for c := range 3 {
						sum[c] += wx * wy * float64(strip.Pix[i+c])
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
// short one rounded half to even, in its guide Vision; an adapter gives its provider's side, edge
// and most.
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
