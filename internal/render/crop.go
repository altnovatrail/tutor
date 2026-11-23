package render

import (
	"cmp"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"math"
	"regexp"
	"slices"
	"strconv"

	"github.com/altnovatrail/tutor/internal/lines"
	"github.com/altnovatrail/tutor/internal/sample"
)

// The crop of a sheet for checking is cut by the rule of the eval protocol, canvas's as it is. Its
// numbers are canvas's to the bit: the bundles of the strokes, the median height h_med, the height
// D′ the digits are drawn at, the size, the margins, where the crop lies on the sheet and the
// frames of the groups. They are whole numbers, in tenths of a unit for places, and each one that
// is not whole is one division of two of them, which IEEE 754 rounds alike in Go and in JavaScript.
// Its pixels are this renderer's, in whole numbers as the board's are; canvas's raster may draw
// them a level of a channel apart, as canvas's vectors allow.
const (
	// CropMargin is the least margin around the work, px: a wide line of a style widens it to hold
	// half the line and the pixel at its edge.
	CropMargin = 12
	// cropFlat is the height in units below which a stroke is a line, not writing: a dot, a minus, a
	// bar or an underline is in no bundle and joins none.
	cropFlat = 15
	// cropCeiling is the highest median the scale takes, in units: a drawing alone stays a drawing.
	cropCeiling = 100
	// CropSide is the least width and height of a crop, px: Claude may err on images under 200
	// pixels.
	CropSide = 200
	// cropEdge and cropTokens are the longest side, px, and the most visual tokens, ⌈w / 28⌉ ×
	// ⌈h / 28⌉ (Tokens): what every model of Claude takes without scaling the image down.
	cropEdge   = 1568
	cropTokens = 1568
	// Digit is the height of a digit a crop is drawn at unless the module's manifest names another,
	// px.
	Digit = 32
	// MaxDigit is the tallest digit a crop takes, px: its ink then still stays within the margin.
	MaxDigit = 256
)

// Heights are the heights of digits the measurement draws crops at, px; Digit is the main one.
func Heights() []int { return []int{16, 24, Digit, 48} }

// Frame is a group's frame on a crop: its label, and its ink in whole thousandths of the crop's
// height and width, [ymin, xmin, ymax, xmax]. Every pixel the ink of its strokes touches lies within
// it, and min < max.
type Frame struct {
	Label string
	Box   [4]int64
}

// Crop is the crop of a sheet for checking, all but its pixels: what lies where, at what scale.
type Crop struct {
	// Digit is the height of a digit asked for, D, and Drawn the height digits are drawn at, D′: D,
	// or less if the crop at D would be too large.
	Digit, Drawn int
	// Median is h_med, the lower median of the heights of the bundles, in units: cropFlat if there
	// are none.
	Median float64
	// Scale is the crop's pixels to a unit of the sheet: D′ over h_med, or over cropCeiling if h_med
	// is above it.
	Scale float64
	// Line is the width of the widest line of the strokes, px: without a style D′ / 12 and 1 at
	// least, and of a style its width at the scale, 1 at least.
	Line          float64
	Width, Height int
	// Margins are the margins above and to the left of the work, px; below and to the right is what
	// is left.
	Margins [2]int
	// SheetBox is where the crop with its margins lies on the sheet, [ymin, xmin, ymax, xmax], in
	// units.
	SheetBox [4]float64
	// Frames are the frames of the groups, in the order of their labels.
	Frames []Frame

	box  extent // of all the points, in tenths
	unit int64  // the median the scale takes, in tenths
}

// look is how a crop draws a stroke: black and opaque without a style, or by its style — its kind
// of pen, its width in whole tenths of a unit, its colour and its opacity in percent.
type look struct {
	styled  bool
	pen     sample.Pen
	width   int64
	rgb     [3]uint8
	opacity int64
}

// colorPattern is the form of a style's colour, as the eval protocol writes it.
var colorPattern = regexp.MustCompile("^#[0-9a-f]{6}$")

// looksOf are how the strokes are drawn, each by its style. A style the crop cannot draw — no kind
// of pen, a colour not "#rrggbb", a width out of 0.1 to 50 units or an opacity out of 10 to 100 — is
// refused.
func looksOf(strokes []sample.Stroke) ([]look, error) {
	out := make([]look, len(strokes))
	for i, st := range strokes {
		y := st.Style
		if y == nil {
			continue
		}
		w := lines.Tenths(y.Width)
		switch {
		case y.Pen != sample.PenPen && y.Pen != sample.PenLiner && y.Pen != sample.PenMarker:
			return nil, fmt.Errorf("render: the stroke %q has a style of no kind of pen", st.ID)
		case !colorPattern.MatchString(y.Color):
			return nil, fmt.Errorf("render: the stroke %q has the colour %q, and a style's is #rrggbb", st.ID, y.Color)
		case w < 1 || w > 500:
			return nil, fmt.Errorf("render: the stroke %q has a line %v units wide, and a style's is 0.1 to 50", st.ID, y.Width)
		case y.Opacity < 10 || y.Opacity > 100:
			return nil, fmt.Errorf("render: the stroke %q has an opacity of %d%%, and a style's is 10 to 100", st.ID, y.Opacity)
		}
		rgb, _ := hex.DecodeString(y.Color[1:])
		out[i] = look{styled: true, pen: y.Pen, width: w, rgb: [3]uint8(rgb), opacity: int64(y.Opacity)}
	}
	return out, nil
}

// thickness is N, the width of a stroke's line on a crop with digits drawn px high and the scale
// taking unit tenths, in px times 12 × unit: without a style unit × max(drawn, 12), and of a style
// 12 × max(width × drawn, unit), its width at the scale and 1 px at least.
func (l look) thickness(drawn, unit int64) int64 {
	if !l.styled {
		return unit * max(drawn, 12)
	}
	return 12 * max(l.width*drawn, unit)
}

// widest is N_max of the looks, the largest thickness.
func widest(looks []look, drawn, unit int64) int64 {
	n := int64(0)
	for _, l := range looks {
		n = max(n, l.thickness(drawn, unit))
	}
	return n
}

// extent is a box in whole tenths of a unit.
type extent struct{ minX, minY, maxX, maxY int64 }

// around is the box around boxes; there is at least one.
func around(es []extent) extent {
	b := es[0]
	for _, e := range es[1:] {
		b = extent{min(b.minX, e.minX), min(b.minY, e.minY), max(b.maxX, e.maxX), max(b.maxY, e.maxY)}
	}
	return b
}

// ErrNoInk is the error of a crop of no strokes: a sheet that shows none goes to no check.
var ErrNoInk = errors.New("render: the sheet shows no strokes, and a crop is cut around strokes")

// extentsOf are the boxes of the strokes in tenths. A stroke without points, an ID twice and a
// point that is no place on a sheet of up to MaxSide a side are no sheet's.
func extentsOf(strokes []sample.Stroke) ([]extent, error) {
	seen := map[string]bool{}
	out := make([]extent, len(strokes))
	for i, st := range strokes {
		if len(st.Points) == 0 {
			return nil, fmt.Errorf("render: the stroke %q has no points", st.ID)
		}
		if seen[st.ID] {
			return nil, fmt.Errorf("render: the stroke %q is on the sheet twice", st.ID)
		}
		seen[st.ID] = true
		for j, p := range st.Points {
			if !(p.X >= 0 && p.X <= MaxSide && p.Y >= 0 && p.Y <= MaxSide) {
				return nil, fmt.Errorf("render: the stroke %q has a point off the sheet, (%v, %v)", st.ID, p.X, p.Y)
			}
			x, y := lines.Tenths(p.X), lines.Tenths(p.Y)
			if j == 0 {
				out[i] = extent{x, y, x, y}
				continue
			}
			e := &out[i]
			e.minX, e.minY, e.maxX, e.maxY = min(e.minX, x), min(e.minY, y), max(e.maxX, x), max(e.maxY, y)
		}
	}
	return out, nil
}

// Bundles are the bundles of the strokes: the strokes no lower than cropFlat, joined by a chain of
// boxes that touch or overlap, whenever each was written. Each is the indexes of its strokes in
// those given, ascending, and they go by their least index.
func Bundles(strokes []sample.Stroke) ([][]int, error) {
	es, err := extentsOf(strokes)
	if err != nil {
		return nil, err
	}
	return bundled(es), nil
}

// bundled finds the bundles among the boxes by union and find: two boxes 0 tenths apart or less,
// across and down, join.
func bundled(es []extent) [][]int {
	var tall []int
	for i, e := range es {
		if e.maxY-e.minY >= cropFlat*10 {
			tall = append(tall, i)
		}
	}
	root := make([]int, len(es))
	for i := range root {
		root[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		if root[i] != i {
			root[i] = find(root[i])
		}
		return root[i]
	}
	for k, i := range tall {
		for _, j := range tall[k+1:] {
			a, b := es[i], es[j]
			if a.minX <= b.maxX && b.minX <= a.maxX && a.minY <= b.maxY && b.minY <= a.maxY {
				ri, rj := find(i), find(j)
				root[max(ri, rj)] = min(ri, rj)
			}
		}
	}
	at := map[int]int{}
	var out [][]int
	for _, i := range tall {
		r := find(i)
		n, ok := at[r]
		if !ok {
			n = len(out)
			at[r] = n
			out = append(out, nil)
		}
		out[n] = append(out[n], i)
	}
	return out
}

// medianTenths is h_med in tenths: the height of the bundle in the lower middle, ⌊(n − 1) / 2⌋ from
// the lowest, or cropFlat if there is none.
func medianTenths(es []extent) int64 {
	var heights []int64
	for _, b := range bundled(es) {
		lo, hi := es[b[0]].minY, es[b[0]].maxY
		for _, i := range b[1:] {
			lo, hi = min(lo, es[i].minY), max(hi, es[i].maxY)
		}
		heights = append(heights, hi-lo)
	}
	if len(heights) == 0 {
		return cropFlat * 10
	}
	slices.Sort(heights)
	return heights[(len(heights)-1)/2]
}

// MedianOf is h_med of the strokes, in units: the height the crop scales to that of a digit.
func MedianOf(strokes []sample.Stroke) (float64, error) {
	es, err := extentsOf(strokes)
	if err != nil {
		return 0, err
	}
	return float64(medianTenths(es)) / 10, nil
}

// size is the size of a crop with digits drawn px high, the margins that centre the work in it, and
// N_max, the thickness of its widest line.
type size struct {
	drawn, width, height, top, left, widest int64
}

// sizeOf is the size of the crop at digits drawn px high: its margin half the widest line and a
// pixel, CropMargin at least.
func sizeOf(box extent, unit, drawn int64, looks []look) size {
	n := widest(looks, drawn, unit)
	m := max(CropMargin, ceilDiv(n, 24*unit)+1)
	w := ceilDiv((box.maxX-box.minX)*drawn, unit) + 2*m
	h := ceilDiv((box.maxY-box.minY)*drawn, unit) + 2*m
	return size{
		drawn:  drawn,
		width:  max(w, CropSide),
		height: max(h, CropSide),
		top:    m + max(CropSide-h, 0)/2,
		left:   m + max(CropSide-w, 0)/2,
		widest: n,
	}
}

// fits reports whether every model of Claude takes the crop as it is: no side too long and no
// tokens too many.
func (s size) fits() bool {
	return max(s.width, s.height) <= cropEdge && Tokens(int(s.width), int(s.height)) <= cropTokens
}

// ceilDiv is ⌈a / b⌉ of whole numbers, a ≥ 0 and b > 0.
func ceilDiv(a, b int64) int64 { return (a + b - 1) / b }

// CropOf cuts the crop of the strokes a sheet shows, with its digits digit px high, and the frames
// of their groups: each stroke in exactly one of them. Groups may be nil, and the crop then has no
// frames. A sheet that shows no strokes has no crop: ErrNoInk. The width of each stroke's line is
// its style's, and the margins and the frames hold the widest.
func CropOf(strokes []sample.Stroke, groups []lines.StrokeGroup, digit int) (*Crop, error) {
	if digit < 1 || digit > MaxDigit {
		return nil, fmt.Errorf("render: a digit %d px high, and a crop takes whole px from 1 to %d", digit, MaxDigit)
	}
	es, err := extentsOf(strokes)
	if err != nil {
		return nil, err
	}
	looks, err := looksOf(strokes)
	if err != nil {
		return nil, err
	}
	members, err := membersOf(groups, strokes, es, looks)
	if err != nil {
		return nil, err
	}
	if len(es) == 0 {
		return nil, ErrNoInk
	}
	box, median := around(es), medianTenths(es)
	unit := min(median, cropCeiling*10)
	s := sizeOf(box, unit, int64(digit), looks)
	// At a digit of 1 px every sheet of up to MaxSide fits: (20480 / 150 + 24) px is less than 200,
	// and a line 50 units wide is under 4 px there, within the least margin.
	for !s.fits() && s.drawn > 1 {
		s = sizeOf(box, unit, s.drawn-1, looks)
	}
	c := &Crop{
		Digit: digit, Drawn: int(s.drawn), Median: float64(median) / 10, Scale: float64(10*s.drawn) / float64(unit),
		Line: float64(s.widest) / float64(12*unit), Width: int(s.width), Height: int(s.height), Margins: [2]int{int(s.top), int(s.left)},
		box: box, unit: unit,
	}
	c.SheetBox = [4]float64{
		float64(box.minY*s.drawn-s.top*unit) / float64(10*s.drawn),
		float64(box.minX*s.drawn-s.left*unit) / float64(10*s.drawn),
		float64(box.minY*s.drawn-s.top*unit+s.height*unit) / float64(10*s.drawn),
		float64(box.minX*s.drawn-s.left*unit+s.width*unit) / float64(10*s.drawn),
	}
	// Places on the crop as whole numbers, X × 24 × unit. A frame reaches past the points by half the
	// widest line of its group and by the pixel at its edge, which the raster still shades:
	// N / (24 × unit) + 1 px.
	across := func(x int64) int64 { return 24 * ((x-box.minX)*s.drawn + s.left*unit) }
	down := func(y int64) int64 { return 24 * ((y-box.minY)*s.drawn + s.top*unit) }
	for _, m := range members {
		f := around(m.extents)
		reach := widest(m.looks, s.drawn, unit) + 24*unit
		c.Frames = append(c.Frames, Frame{Label: m.label, Box: [4]int64{
			1000 * (down(f.minY) - reach) / (24 * unit * s.height),
			1000 * (across(f.minX) - reach) / (24 * unit * s.width),
			ceilDiv(1000*(down(f.maxY)+reach), 24*unit*s.height),
			ceilDiv(1000*(across(f.maxX)+reach), 24*unit*s.width),
		}})
	}
	return c, nil
}

// member is a group's label and the boxes and looks of its strokes.
type member struct {
	label   string
	extents []extent
	looks   []look
}

// membersOf are the groups' strokes, in the order of their labels: A to Z, then AA and on, so a
// shorter label goes first. The groups are those of the strokes given: each stroke in one of them,
// and no stroke of a group missing from the sheet.
func membersOf(groups []lines.StrokeGroup, strokes []sample.Stroke, es []extent, looks []look) ([]member, error) {
	if groups == nil {
		return nil, nil
	}
	at := map[string]int{}
	for i, st := range strokes {
		at[st.ID] = i
	}
	grouped := map[string]bool{}
	var out []member
	for _, g := range groups {
		if len(g.Strokes) == 0 {
			return nil, fmt.Errorf("render: the group %s has no strokes", g.Label)
		}
		m := member{label: g.Label}
		for _, id := range g.Strokes {
			i, ok := at[id]
			switch {
			case !ok:
				return nil, fmt.Errorf("render: the group %s has a stroke %q not on the sheet", g.Label, id)
			case grouped[id]:
				return nil, fmt.Errorf("render: the stroke %q is in two groups", id)
			}
			grouped[id] = true
			m.extents = append(m.extents, es[i])
			m.looks = append(m.looks, looks[i])
		}
		out = append(out, m)
	}
	for _, st := range strokes {
		if !grouped[st.ID] {
			return nil, fmt.Errorf("render: the stroke %q is in no group", st.ID)
		}
	}
	slices.SortStableFunc(out, func(a, b member) int {
		return cmp.Or(cmp.Compare(len(a.label), len(b.label)), cmp.Compare(a.label, b.label))
	})
	return out, nil
}

// cropSub is the parts of a pixel the crop's raster places points in: in 48ths, half a line without
// a style, max(D′, 12) / 24 px, is a whole number, as are the places of the 4 × 4 samples of a
// pixel. Half a line of a style is taken to the nearest 48th.
const cropSub = 48

// Draw draws strokes on the crop as the board's renderer draws a sheet, in whole numbers: each
// point at the nearest 48th of a pixel of the crop, by the crop's own whole numbers, each line with
// round ends and joins, on white, by the strokes' IDs ascending (drawOrder). A stroke without a style is black and
// opaque, its line max(D′, 12) / 12 px wide; one of a style is its width at the crop's scale, 1 px at
// least — a pen's with no pressure and no speed —, its colour at its opacity over what lies under
// it, and a marker's what lies under it multiplied by its colour, at its opacity. Each stroke is
// one layer, no darker where it crosses itself. The picture is grey when no stroke has a style, and
// RGB when one has. The strokes are those the crop was cut around, or others on the same sheet: what
// falls off the crop is cut off.
func (c *Crop) Draw(strokes []sample.Stroke) (image.Image, error) {
	looks, err := looksOf(strokes)
	if err != nil {
		return nil, err
	}
	bounds := image.Rect(0, 0, c.Width, c.Height)
	var gray *image.Gray
	var rgba *image.RGBA
	if slices.ContainsFunc(looks, func(l look) bool { return l.styled }) {
		rgba = image.NewRGBA(bounds)
		for i := range rgba.Pix {
			rgba.Pix[i] = 0xff
		}
	} else {
		gray = image.NewGray(bounds)
		for i := range gray.Pix {
			gray.Pix[i] = 0xff
		}
	}
	top, left := int64(c.Margins[0]), int64(c.Margins[1])
	for _, k := range drawOrder(strokes) {
		st := strokes[k]
		var pts []point
		for _, pt := range st.Points {
			if !(pt.X >= 0 && pt.X <= MaxSide && pt.Y >= 0 && pt.Y <= MaxSide) {
				return nil, fmt.Errorf("render: the stroke %q has a point off the sheet, (%v, %v)", st.ID, pt.X, pt.Y)
			}
			pts = append(pts, point{
				nearest(cropSub*((lines.Tenths(pt.X)-c.box.minX)*int64(c.Drawn)+left*c.unit), c.unit),
				nearest(cropSub*((lines.Tenths(pt.Y)-c.box.minY)*int64(c.Drawn)+top*c.unit), c.unit),
			})
		}
		if len(pts) == 0 {
			continue
		}
		l := looks[k]
		p := pen{sub: cropSub, half: nearest(2*l.thickness(int64(c.Drawn), c.unit), c.unit), reach: 26}
		if gray != nil {
			p.stroke(gray, pts)
			continue
		}
		p.cover(bounds, pts, func(px, py, n int) { l.paint(rgba, px, py, int64(n)) })
	}
	if gray != nil {
		return gray, nil
	}
	return rgba, nil
}

// drawOrder is the order a crop draws the strokes in, by their places among those given: by their IDs
// ascending, as numbers, when each is one, as the protocol's are; else as they are given.
func drawOrder(strokes []sample.Stroke) []int {
	order := make([]int, len(strokes))
	ids := make([]uint64, len(strokes))
	numbered := true
	for i, st := range strokes {
		order[i] = i
		n, err := strconv.ParseUint(st.ID, 10, 63)
		ids[i], numbered = n, numbered && err == nil
	}
	if numbered {
		slices.SortStableFunc(order, func(a, b int) int { return cmp.Compare(ids[a], ids[b]) })
	}
	return order
}

// paint lays the look's ink on a pixel that n of its grid × grid samples cover, in whole numbers:
// each channel o becomes o × (1 − a) + ink × a, a its share n / 16 times its opacity, and ink its
// colour, or of a marker o × its colour / 255; without a style ink is black and a whole.
func (l look) paint(img *image.RGBA, px, py int, n int64) {
	rgb, opacity := l.rgb, l.opacity
	if !l.styled {
		opacity = 100
	}
	a := n * opacity
	const whole = grid * grid * 100
	o := img.Pix[img.PixOffset(px, py):]
	for ch := range 3 {
		under, ink := int64(o[ch]), int64(rgb[ch])
		if l.pen == sample.PenMarker {
			ink = (under*ink + 127) / 255
		}
		o[ch] = uint8((under*(whole-a) + ink*a + whole/2) / whole)
	}
}

// PNG draws strokes on the crop, as Draw does, and encodes the picture as the eval protocol writes
// a crop's PNG: 8 bits, not interlaced, grey, colour type 0, when no stroke has a style and RGB,
// colour type 2, when one has, and the chunks IHDR, IDAT and IEND alone — no ancillary chunk, which
// could hold a text, metadata or a time. Go's encoder writes a grey picture and an opaque RGBA one
// so, its IDAT in pieces of one zlib stream.
func (c *Crop) PNG(strokes []sample.Stroke) ([]byte, error) {
	img, err := c.Draw(strokes)
	if err != nil {
		return nil, err
	}
	return PNG(img)
}

// InkBox is the box of the points the crop was cut around, in whole tenths of a unit, [ymin, xmin,
// ymax, xmax]: the ink_box of the eval protocol's set of crops.
func (c *Crop) InkBox() [4]int64 { return [4]int64{c.box.minY, c.box.minX, c.box.maxY, c.box.maxX} }

// MedianTenths is h_med in whole tenths of a unit, as it is: the bundle_median of the protocol's set
// of crops, which the scale takes no higher than cropCeiling.
func (c *Crop) MedianTenths() int64 { return int64(math.Round(c.Median * 10)) }

// nearest is the whole number nearest a / b, b > 0, a half up, below zero too.
func nearest(a, b int64) int64 {
	n := 2*a + b
	q := n / (2 * b)
	if n%(2*b) < 0 {
		q--
	}
	return q
}

// Cropped cuts the crop of the strokes a sheet shows, with its digits digit px high, and draws
// them: their groups are those lines.Groups finds among them, as a device's are.
func Cropped(strokes []sample.Stroke, digit int) (*Crop, image.Image, error) {
	c, err := CropOf(strokes, lines.Groups(strokes, nil), digit)
	if err != nil {
		return nil, nil, err
	}
	img, err := c.Draw(strokes)
	return c, img, err
}

// CropFinal cuts and draws the crop of the sheet as it was finished.
func CropFinal(s *sample.Sample, digit int) (*Crop, image.Image, error) {
	return Cropped(Shown(s, math.MaxInt64), digit)
}
