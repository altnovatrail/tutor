package synth

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/sample"
)

var picture = flag.String("picture", "", "also write the picture of the English text of the tests, as PNG, to this file: in synthetic, out of git")

// english is the English text of the tests: every letter, small and capital, and every sign of the
// Latin ones and of the font's that English text takes.
var english = []string{
	"The quick brown fox jumps over the lazy dog.",
	"PACK MY BOX WITH FIVE DOZEN LIQUOR JUGS!",
	"Is it \"fine\"? Yes; it's “fine” — isn’t it – (really), 1/2: 3.",
}

// Every Latin letter, small and capital, and every character of the English text has its strokes:
// none of them comes out a squiggle. Every point of a Latin glyph lies in the box of the font, from
// 0 to 1 across and from 0 to 1.35 down; a capital spans the height of a digit, a small letter with
// no stem stays below the top of the small letters, 0.45, but for the dots of i and j, and one with
// no tail above the line, 1.
func TestLatinLetters(t *testing.T) {
	for r := 'a'; r <= 'z'; r++ {
		for _, c := range []rune{r, r - 'a' + 'A'} {
			if glyphOf(c) == nil {
				t.Errorf("%c has no glyph", c)
			}
		}
	}
	for _, line := range english {
		for _, r := range line {
			if r != ' ' && glyphOf(r) == nil {
				t.Errorf("%q has no glyph", r)
			}
		}
	}
	for r, g := range latin {
		top, bottom := 2.0, -1.0
		for _, s := range g {
			for _, p := range s {
				if p.x < 0 || p.x > 1 || p.y < 0 || p.y > 1.35 {
					t.Errorf("%c has the point %v out of the box of the font", r, p)
				}
				if len(s) > 1 { // a dot, the stroke of one point, is above the small letters
					top = min(top, p.y)
				}
				bottom = max(bottom, p.y)
			}
		}
		small := r >= 'a' && r <= 'z'
		switch {
		case r >= 'A' && r <= 'Z' && (top > 0.02 || bottom < 0.98):
			t.Errorf("the capital %c spans %v to %v, not the height of a digit", r, top, bottom)
		case small && !strings.ContainsRune("bdfhklt", r) && top < 0.449:
			t.Errorf("the small %c rises to %v, above the small letters", r, top)
		case small && !strings.ContainsRune("gjpqy", r) && bottom > 1.01:
			t.Errorf("the small %c falls to %v, below the line", r, bottom)
		}
	}
	// A Latin letter drawn as a Cyrillic one takes its strokes, and the font keeps its x.
	for l, c := range map[rune]rune{'a': 'а', 'c': 'с', 'e': 'е', 'o': 'о', 'p': 'р', 'y': 'у', 'O': 'О'} {
		if !reflect.DeepEqual(latin[l], letters[c]) {
			t.Errorf("%c is not drawn as %c", l, c)
		}
	}
	// No character is in two tables, so that the one table write reads is each table's.
	for r := range glyphs {
		n := 0
		for _, set := range []map[rune]glyph{font, letters, latin} {
			if _, ok := set[r]; ok {
				n++
			}
		}
		if n != 1 {
			t.Errorf("%q is in %d tables", r, n)
		}
	}
	if !reflect.DeepEqual(glyphOf('x'), font['x']) {
		t.Error("x is not the font's")
	}
}

// The Latin letters and signs change no sheet or page drawn before them: no text a plan writes holds
// one but x, which the font has, and no value in a box of the workbook either
// (TestSheetsOfLinesAsBefore and TestPagesAsBefore hold them). The text of a drawing is its
// description in words, which code draws as a figure, not as writing; its labels and the numbers of
// a column are written.
func TestLatinChangesNoPlan(t *testing.T) {
	check := func(where, text string) {
		for _, r := range text {
			if _, ok := latin[r]; ok {
				t.Errorf("%s: %q holds the Latin %c", where, text, r)
			}
		}
	}
	wf, err := os.Open("../../eval/workbook.json")
	if err != nil {
		t.Fatal(err)
	}
	wb, err := sample.ReadWorkbook(wf)
	_ = wf.Close()
	if err != nil {
		t.Fatal(err)
	}
	for _, pg := range wb.Pages {
		for _, it := range pg.Items {
			for _, b := range it.Boxes {
				for _, v := range []*string{b.Written, b.Crossed} {
					if v != nil {
						check(fmt.Sprintf("page %d", pg.Number), *v)
					}
				}
			}
		}
	}
	for _, path := range []string{"../../eval/sheets.json", "../../eval/free-sheets.json", "../../eval/free-author.json", "../../eval/near-sheets.json"} {
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		pl, err := sample.ReadPlan(f)
		_ = f.Close()
		if err != nil {
			t.Fatal(err)
		}
		for _, sh := range pl.Sheets {
			texts := append([]string(nil), sh.Lines...)
			if len(sh.Parts) > 0 {
				texts = nil
				for _, part := range sh.Parts {
					if part.Kind == sample.PartText {
						texts = append(texts, part.Text)
					}
					if c := part.Figure.Column; c != nil {
						texts = append(texts, c.Top, c.Op, c.Bottom, c.Result)
					}
					if sc := part.Figure.Scheme; sc != nil {
						texts = append(texts, sc.Total)
						for _, seg := range sc.Segments {
							texts = append(texts, seg.Label)
							for _, p := range seg.Pieces {
								texts = append(texts, p.Label)
							}
						}
					}
				}
			}
			if e := sh.Erasure; e != nil {
				texts = append(texts, e.Text, e.Replacement)
			}
			for _, text := range texts {
				check(fmt.Sprintf("%s, sheet %d", path, sh.Number), text)
			}
		}
	}
}

// The English text, drawn with seed 1, is what it was: the hash of its strokes and of its picture
// hold it on every machine (see the package's comment). The same seed draws the same strokes, and
// another seed other ones. With -picture the test writes the picture for the author to look at.
func TestText(t *testing.T) {
	board := sample.Sheet{Width: sheetWidth, Height: sheetHeight}
	strokes := Text(english, board, 1)
	if !reflect.DeepEqual(strokes, Text(english, board, 1)) || reflect.DeepEqual(strokes, Text(english, board, 2)) {
		t.Error("the same seed draws other strokes, or another seed the same ones")
	}
	data, err := json.Marshal(strokes)
	if err != nil {
		t.Fatal(err)
	}
	s := &sample.Sample{Sheet: board, Strokes: strokes}
	img, err := render.Final(s)
	if err != nil {
		t.Fatal(err)
	}
	png, err := render.PNG(img)
	if err != nil {
		t.Fatal(err)
	}
	if *picture != "" {
		if err := os.WriteFile(*picture, png, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct{ what, got, want string }{
		{"strokes", fmt.Sprintf("%x", sha256.Sum256(data)), "d918953ef2ae6ac6f27436bd7be4054cdc2b21688ef8847939af9b7353de5015"},
		{"picture", fmt.Sprintf("%x", sha256.Sum256(img.Pix)), "9da2a0651616effadada1fa60d3ba4b1536d58eea0907372070b02663804e803"},
	} {
		if c.got != c.want {
			t.Errorf("the %s of the English text hash to %s, want %s", c.what, c.got, c.want)
		}
	}
	defer func() {
		if recover() == nil {
			t.Error("no lines are drawn")
		}
	}()
	Text(nil, board, 1)
}

// Many lines of text fit down a page with their lines apart: a hand whose step is less than 1.6 of
// its height writes smaller, and its points stay on the page, which is the page asked for.
func TestTextApart(t *testing.T) {
	h := spaced(hand{height: 40, advance: 30, step: 32})
	if h.step != 32 || h.height != 20 || h.advance != 15 {
		t.Errorf("a hand 40 high with a step of 32 is %+v", h)
	}
	if h := spaced(hand{height: 30, advance: 20, step: 60}); h.height != 30 || h.advance != 20 {
		t.Errorf("a hand with room is %+v", h)
	}
	lines := make([]string, 15)
	for i := range lines {
		lines[i] = "We went to the park and saw a big dog, too."
	}
	page := sample.Sheet{Width: 1000, Height: 1000}
	for _, s := range Text(lines, page, 3) {
		for _, p := range s.Points {
			if p.X <= 0 || p.X >= 1000 || p.Y <= 0 || p.Y >= 1000 {
				t.Fatalf("the stroke %s reaches (%v, %v), on the edge of the page", s.ID, p.X, p.Y)
			}
		}
	}
}
