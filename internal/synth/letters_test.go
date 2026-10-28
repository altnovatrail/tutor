package synth

import (
	"os"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/internal/sample"
)

// Every character of the text of both free-form plans, and of the text their planned erasures
// take away, has its strokes in the font, and so has every small Russian letter: none of them comes
// out a squiggle.
func TestPlansHaveTheirLetters(t *testing.T) {
	var text strings.Builder
	text.WriteString("абвгдеёжзийклмнопрстуфхцчшщъыьэюя□△")
	for _, path := range []string{"../../eval/free-sheets.json", "../../eval/free-author.json"} {
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
			for _, p := range sh.Parts {
				if p.Kind == sample.PartText && p.Figure.Column == nil {
					text.WriteString(p.Text)
				}
			}
			// What a planned erasure takes away from text is written first, and a frame shows it.
			if e := sh.Erasure; e != nil && sh.Parts[e.Line-1].Kind == sample.PartText {
				text.WriteString(e.Text)
			}
		}
	}
	for _, r := range text.String() {
		if r != ' ' && glyphOf(r) == nil {
			t.Errorf("%q has no glyph", r)
		}
	}
}
