package render_test

import (
	"crypto/sha256"
	"fmt"
	"image"
	"os"
	"testing"

	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/synth"
)

// The pictures of two sheets of the free-form plan, eval/free-sheets.json, drawn with seed 1, by the
// SHA-256 of their pixels, and the frame before each one's erasure: sheet 7, a number line with a
// jump erased and drawn again, and sheet 8, a scheme with a piece moved from one segment to the
// other (T60). A change of the drawings, of the hand or of the renderer changes them, and the
// pictures are then looked at again.
var goldenFree = map[string]string{
	"7":   "c5e0b4aa1d3735ac766dd5dd8d086b3bea41307524bddb5c726c0a6b09415695",
	"7/0": "847162d14fb93e88a5f8942de76032c2de177b651858b27ef92aff5dcfbdff8e",
	"8":   "2b6148d8a01bda63772f458148ba9628561685719c86962df5cd54dc67dc72e5",
	"8/0": "ba64e96bcc6185503fa6255b427415b482deaf5d570b39846de5d1aecc64f82b",
}

func TestGoldenFreeSheets(t *testing.T) {
	f, err := os.Open("../../eval/free-sheets.json")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := sample.ReadPlan(f)
	_ = f.Close()
	if err != nil {
		t.Fatal(err)
	}
	ok := drawn(t)
	check := func(name string, img *image.Gray) {
		if got := fmt.Sprintf("%x", sha256.Sum256(img.Pix)); got != goldenFree[name] {
			t.Errorf("sheet %s: the picture's pixels hash to %s, want %s", name, got, goldenFree[name])
		}
	}
	for _, n := range []int{7, 8} {
		s := synth.Sheet(&plan.Sheets[n-1], 1)
		check(fmt.Sprint(n), ok(render.Final(s)))
		check(fmt.Sprintf("%d/0", n), ok(render.Before(s, 0)))
	}
}
