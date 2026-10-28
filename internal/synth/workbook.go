package synth

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/altnovatrail/tutor/internal/sample"
)

// A page of the workbook (T78; docs/architecture.md, 3.4, «Рабочая тетрадь») is laid out by the
// numbers of R35 canvas — a page of 1000 × 1400 units, boxes of 100 × 100, margins of 40, rows 100
// high and 32 apart, the line 56 from the item's left edge and a box 16 from a word — in a layout
// of tutor's own, simpler than canvas's: every item on a row of its own, its boxes where the widths
// of the print before them put them. Only the strokes of a box go to the reading, so the layout
// tells where the hand writes and no more.
const (
	PageWidth  = 1000
	PageHeight = 1400
	// BoxSide is the width and the height of a box.
	BoxSide    = 100
	pageMargin = 40
	pageRow    = 132 // a row of 100 and the 32 between rows
	lineLeft   = pageMargin + 56
	boxGap     = 16 // between a box and a word
	twoBoxGap  = 32 // between two boxes side by side
	printSize  = 30 // the size of the print, in units
	// boxPad is how far within its box the writing stays, at the least.
	boxPad = 12
)

// printHundredths is how wide a character of the print is, in hundredths of the print's size: the
// widths of R35 canvas for digits, signs and the space, and one width for every other character.
func printHundredths(r rune) int {
	switch {
	case r >= '0' && r <= '9':
		return 53
	case strings.ContainsRune("+−×÷=<>", r):
		return 56
	case r == ' ':
		return 26
	case strings.ContainsRune(".,:;!?", r):
		return 28
	case strings.ContainsRune("/()—–-", r):
		return 33
	}
	return 55
}

// printWidth is how wide a piece of the print is, in units, as R35 canvas rounds it up.
func printWidth(s string) float64 {
	sum := 0
	for _, r := range s {
		sum += printHundredths(r)
	}
	return float64((printSize*sum + 99) / 100)
}

// Page is a page of the workbook as code writes it: its boxes, in the order of the page, by item and
// then by box, with the strokes written in each. A blank box has none.
type Page struct {
	Number int
	Boxes  []Box
}

// Box is a box of a page: the place of its item and its own among the item's boxes, from 1, where
// it stands, its top left corner in units of the page, and the strokes written in it.
type Box struct {
	Item, Slot int
	Left, Top  float64
	Strokes    []sample.Stroke
}

// boxesOf lays out the boxes of an item's line on its row: the left edge of each. The line is the
// print between the boxes, and a box stands boxGap from the word before it and twoBoxGap from a
// box. A line wider than the page is an error.
func boxesOf(text string) ([]float64, error) {
	pieces := strings.Split(text, sample.BoxMark)
	x := float64(lineLeft)
	var lefts []float64
	for i, piece := range pieces {
		piece = strings.TrimSpace(piece)
		if piece != "" {
			if i > 0 {
				x += boxGap
			}
			x += printWidth(piece)
		}
		if i == len(pieces)-1 {
			break
		}
		switch {
		case i == 0 && piece == "":
		case piece == "":
			x += twoBoxGap
		default:
			x += boxGap
		}
		lefts = append(lefts, x)
		x += BoxSide
	}
	if x > PageWidth-pageMargin {
		return nil, fmt.Errorf("the line %q is %.0f units wide, and a line of the page is %d at most", text, x-lineLeft, PageWidth-pageMargin-lineLeft)
	}
	return lefts, nil
}

// CheckLayout checks that every page of a workbook can be laid out: its items fit the rows of a page,
// and each line fits across it. It lists every line that does not, by its path in the file.
func CheckLayout(wb *sample.Workbook) error {
	var errs []error
	for i, pg := range wb.Pages {
		if len(pg.Items) > sample.MaxItems {
			errs = append(errs, fmt.Errorf("pages[%d].items: %d items, and a page has %d rows", i, len(pg.Items), sample.MaxItems))
		}
		for k, it := range pg.Items {
			if _, err := boxesOf(it.Text); err != nil {
				errs = append(errs, fmt.Errorf("pages[%d].items[%d].text: %w", i, k, err))
			}
		}
	}
	return errors.Join(errs...)
}

// PageOf draws a page of the workbook as a learner writes it, from the seed: one hand for the page,
// drawn from the seed and the page's number, item by item and box by box, with a pause to read
// each item. A box holds its value written plainly, or as its scenario says: a digit leaning toward
// another, a digit mirrored, a wrong value crossed out and the value written beside it. A fraction
// is written stacked or inline, as its box says. The writing stays within its box, so that by the
// rule of boxes of R37 canvas each stroke is its box's. The same seed draws the same page again. A
// page that cannot be laid out (CheckLayout) is an error.
func PageOf(pg *sample.WorkbookPage, seed uint64) (*Page, error) {
	if len(pg.Items) > sample.MaxItems {
		return nil, fmt.Errorf("synth: page %d has %d items, and a page has %d rows", pg.Number, len(pg.Items), sample.MaxItems)
	}
	w := &writer{
		s:   &sample.Sample{Sheet: sample.Sheet{Width: PageWidth, Height: PageHeight}, Strokes: []sample.Stroke{}},
		rng: rand.New(rand.NewPCG(seed, uint64(pg.Number))),
	}
	h, spacing := w.pickHand(0.01, 0, 0)
	w.pointMs = h.pointMs
	w.t = 800 + w.rng.Int64N(2000)
	out := &Page{Number: pg.Number}
	for i, it := range pg.Items {
		top := float64(pageMargin + i*pageRow)
		w.pause(2500 + w.rng.Int64N(4000)) // the learner reads the item and works it out
		lefts, err := boxesOf(it.Text)
		if err != nil {
			return nil, fmt.Errorf("synth: page %d, item %d: %w", pg.Number, i+1, err)
		}
		for k, b := range it.Boxes {
			box := Box{Item: i + 1, Slot: k + 1, Left: lefts[k], Top: top}
			if b.Written != nil {
				if k > 0 {
					w.pause(800 + w.rng.Int64N(1500))
				}
				start := len(w.s.Strokes)
				w.box(h, spacing, b, lefts[k], top)
				box.Strokes = slices.Clone(w.s.Strokes[start:])
			}
			out.Boxes = append(out.Boxes, box)
		}
	}
	return out, nil
}

// boxValue is a value to write in a box: its text, how a fraction of it is written, and the digit of
// it drawn leaning or mirrored, by its place among the characters of the text, or −1.
type boxValue struct {
	text    string
	stacked bool
	leanAt  int
	toward  rune
	flipAt  int
}

// box writes what a box holds, and returns the IDs of its strokes: its value, or a wrong value
// crossed out and its value beside it.
func (w *writer) box(h hand, spacing float64, b sample.WorkbookBox, left, top float64) []string {
	v := boxValue{text: *b.Written, stacked: b.Fraction != nil && *b.Fraction == sample.Stacked, leanAt: -1, flipAt: -1}
	if at, toward, ok := b.Leaning(); ok {
		v.leanAt, v.toward = at, toward
	}
	if at, ok := sample.Mirrored(v.text); ok && b.Scenario == sample.BoxMirrored {
		v.flipAt = at
	}
	room := float64(BoxSide - 2*boxPad)
	if b.Crossed == nil {
		height := w.fit(h, spacing, v, room, 44+w.jitter(6))
		width, tall := w.size(h, spacing, v, height)
		x := left + (BoxSide-width)/2 + w.jitter(3)
		y := top + (BoxSide-tall)/2 + w.jitter(3)
		return w.value(h, spacing, v, height, x, y)
	}
	crossed := boxValue{text: *b.Crossed, stacked: v.stacked, leanAt: -1, flipAt: -1}
	// Both values side by side, a gap of 0.6 of a digit between them.
	height := 40 + w.jitter(4)
	for {
		wc, tc := w.size(h, spacing, crossed, height)
		wv, _ := w.size(h, spacing, v, height)
		if wc+float64(height*0.6)+wv <= room && tc <= room || height < 8 {
			break
		}
		height *= 0.95
	}
	wc, tc := w.size(h, spacing, crossed, height)
	wv, tv := w.size(h, spacing, v, height)
	x := left + (BoxSide-(wc+float64(height*0.6)+wv))/2 + w.jitter(2)
	yc := top + (BoxSide-tc)/2 + w.jitter(2)
	ids := w.value(h, spacing, crossed, height, x, yc)
	w.pause(1500 + w.rng.Int64N(2000)) // the learner sees the mistake
	if crossed.stacked {
		// A stacked fraction is struck through from its foot to its head.
		ids = append(ids, w.trace(pt{x - 2, yc + tc + 1}, pt{x + wc + 2, yc - 1})...)
	} else {
		ids = append(ids, w.trace(pt{x - 3, yc + float64(tc*0.55)}, pt{x + wc + 2, yc + float64(tc*0.5)})...)
	}
	w.pause(400 + w.rng.Int64N(800))
	xv := x + wc + float64(height*0.6)
	return append(ids, w.value(h, spacing, v, height, xv, top+(BoxSide-tv)/2+w.jitter(2))...)
}

// fit is the height of a digit at which a value fits the room across, at most the height wanted.
func (w *writer) fit(h hand, spacing float64, v boxValue, room, wanted float64) float64 {
	height := wanted
	for {
		width, tall := w.size(h, spacing, v, height)
		if width <= room && tall <= room || height < 8 {
			return height
		}
		height *= 0.95
	}
}

// size is how wide and how tall a value is written with digits of the height: a row of characters,
// or a stacked fraction — its numerator, a bar and its denominator, each row of the digits of a
// floor, the floors a fifth of a digit from the bar. It allows for the slant and for the size of a
// character wandering.
func (w *writer) size(h hand, spacing float64, v boxValue, height float64) (width, tall float64) {
	row := func(s string) float64 {
		n := float64(utf8.RuneCountInString(s))
		advance := float64(height * h.shape * spacing)
		return float64((n-1)*advance*1.1) + float64(1.08*height*(h.shape+math.Max(h.slant, 0)))
	}
	if !v.stacked {
		return row(v.text), float64(1.36 * height)
	}
	top, bottom, _ := strings.Cut(v.text, "/")
	return math.Max(row(top), row(bottom)) + float64(0.3*height), float64(2.76 * height)
}

// value writes a value with digits of the height, its top left corner at (x, y), and returns the
// IDs of its strokes. A stacked fraction is written numerator first, then the bar, then the
// denominator, each floor centred over the bar.
func (w *writer) value(h hand, spacing float64, v boxValue, height, x, y float64) []string {
	bh := h
	bh.height, bh.advance, bh.left = height, float64(height*h.shape*spacing), x
	if !v.stacked {
		return w.writeRow(bh, v.text, x, y+float64(0.12*height), v.leanAt, v.toward, v.flipAt)
	}
	top, bottom, _ := strings.Cut(v.text, "/")
	skip := utf8.RuneCountInString(top) + 1 // the characters of the bottom start after the slash
	width, _ := w.size(h, spacing, v, height)
	floor := func(s string) float64 { return x + (width-w.rowWidth(h, spacing, s, height))/2 }
	ids := w.writeRow(bh, top, floor(top), y+float64(0.12*height), v.leanAt, v.toward, v.flipAt)
	w.pause(150 + w.rng.Int64N(300))
	bar := y + float64(1.38*height)
	ids = append(ids, w.trace(pt{x, bar}, pt{x + width, bar - float64(0.03*width)})...)
	w.pause(150 + w.rng.Int64N(300))
	return append(ids, w.writeRow(bh, bottom, floor(bottom), y+float64(1.6*height), v.leanAt-skip, v.toward, v.flipAt-skip)...)
}

// rowWidth is how wide a row of characters is written with digits of the height.
func (w *writer) rowWidth(h hand, spacing float64, s string, height float64) float64 {
	width, _ := w.size(h, spacing, boxValue{text: s}, height)
	return width
}

// writeRow writes a row of characters by a hand, with the character at leanAt drawn leaning toward the
// digit toward and the one at flipAt mirrored; a place outside the row draws none.
func (w *writer) writeRow(h hand, s string, x, y float64, leanAt int, toward rune, flipAt int) []string {
	n := utf8.RuneCountInString(s)
	if leanAt >= 0 && leanAt < n {
		w.lean = &lean{at: leanAt, toward: toward, shift: SimilarShift}
	}
	if flipAt >= 0 && flipAt < n {
		w.flip = &flipAt
	}
	ids, _ := w.write(h, s, x, y)
	w.lean, w.flip = nil, nil
	return ids
}
