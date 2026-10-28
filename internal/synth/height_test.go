package synth

import (
	"math"
	"os"
	"strings"
	"testing"
	"unicode"

	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/sample"
)

// handHeight is the height of a digit of the hand the sheet is drawn in: Sheet picks the hand
// first, after the writer, so the same seed picks the same one here.
func handHeight(sh *sample.PlanSheet, seed uint64) float64 {
	w := sheetWriter(sh, seed)
	if len(sh.Parts) > 0 {
		h, _ := w.partsHand(sh)
		return h.height
	}
	return w.newHand(sh).height
}

// written is what a plan sheet has written on it, its drawings left out: they are described in
// words, not written.
func written(sh *sample.PlanSheet) string {
	text := strings.Join(sh.Lines, " ")
	for _, p := range sh.Parts {
		if p.Figure.Drawing() == "" {
			text += " " + p.Text
		}
	}
	return text
}

// The crop's estimate of the height of a digit, h_med of the bundles of the strokes (R25 canvas),
// against the height of the synthetic hand, on every sheet of the plans the measurement draws, with
// the seed its sets were drawn with (T69):
//   - on a sheet whose writing has no small letters, it is within a tenth of the hand, on nine
//     sheets in ten: on all 10 of E2's;
//   - on a sheet with small letters — «Ответ: 5 столбов», «в точке 10», 3x — it may fall to the
//     height of a small letter, about half the hand's: the code writes them apart, each a bundle of
//     its own, and when they are half the bundles, the lower median is one of them. That was so on
//     10 of E2's 30 such sheets and on 19 of the free plan's 40, and the digits of such a crop come
//     out up to twice D. The rule is the protocol's, and tutor does not change it (R35).
func TestTheCropFindsTheHeightOfTheHand(t *testing.T) {
	for _, path := range []string{"../../eval/sheets.json", "../../eval/free-sheets.json"} {
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		plan, err := sample.ReadPlan(f)
		_ = f.Close()
		if err != nil {
			t.Fatal(err)
		}
		var sheets, within [2]int // without small letters and with them
		for i := range plan.Sheets {
			sh := &plan.Sheets[i]
			median, err := render.MedianOf(render.Shown(Sheet(sh, 1), math.MaxInt64))
			if err != nil {
				t.Fatal(err)
			}
			hand := handHeight(sh, 1)
			small := 0
			if strings.IndexFunc(written(sh), unicode.IsLower) >= 0 {
				small = 1
			}
			sheets[small]++
			if math.Abs(median-hand) <= 0.1*hand {
				within[small]++
			}
			if !(median >= 0.5*hand && median <= 1.1*hand) {
				t.Errorf("%s, sheet %d: h_med %.1f units, and the hand's digits %.1f", path, sh.Number, median, hand)
			}
		}
		t.Logf("%s: within 10%% on %d of %d sheets without small letters and on %d of %d with them",
			path, within[0], sheets[0], within[1], sheets[1])
		if 10*within[0] < 9*sheets[0] {
			t.Errorf("%s: without small letters, the estimate is within 10%% of the hand on %d sheets of %d, fewer than nine in ten", path, within[0], sheets[0])
		}
	}
}
