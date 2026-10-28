package workbook_test

import (
	"crypto/sha256"
	"fmt"
	"os"
	"testing"

	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/workbook"
)

// theWorkbook reads the pages of the workbook of the measurement.
func theWorkbook(t *testing.T) *sample.Workbook {
	t.Helper()
	f, err := os.Open("../../eval/workbook.json")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	wb, err := sample.ReadWorkbook(f)
	if err != nil {
		t.Fatal(err)
	}
	return wb
}

// cropsGolden are the SHA-256 of the crops of every filled box of the 20 pages with seed 1, one
// after another, at each height of eval: what a reading of the measurement sees.
var cropsGolden = map[int]string{
	16: "58dd792e3d0acff9ad56e07475f43449a96be0ccd5401edaeab54c9155313a42",
	24: "f825cd201b8bfe8b6f887a200a28d045f09b7817b61c6afc19bf65aafc6e7573",
	32: "556414863c35f8a0f25b199911855a79239b5826e38b42503e697a0676f56b41",
	48: "b1b492a213dd96449e38a1cbfc97c60a58b947055c5870d22ed1b7fda996a4ff",
}

// The cells of a page are its filled boxes, in the order of the page, each with the crop of its
// strokes: digits of the height asked, 200 × 200 at the least, and larger only at 48 px, where an
// inline fraction or a value crossed out runs wider. The crops of the measurement are what they
// were.
func TestCells(t *testing.T) {
	wb := theWorkbook(t)
	for _, digit := range render.Heights() {
		h := sha256.New()
		larger := 0
		for i := range wb.Pages {
			pg := &wb.Pages[i]
			cells, err := workbook.Cells(pg, 1, digit)
			if err != nil {
				t.Fatal(err)
			}
			var want [][2]int
			for k, it := range pg.Items {
				for s, b := range it.Boxes {
					if b.Scenario != sample.BoxBlank {
						want = append(want, [2]int{k + 1, s + 1})
					}
				}
			}
			if len(cells) != len(want) {
				t.Fatalf("page %d: %d cells, want %d", pg.Number, len(cells), len(want))
			}
			for n, c := range cells {
				if [2]int{c.Item, c.Slot} != want[n] || c.Box.Written == nil || c.Crop.Digit != digit {
					t.Errorf("page %d: cell %d is item %d, box %d, at %d px", pg.Number, n+1, c.Item, c.Slot, c.Crop.Digit)
				}
				if c.Crop.Width < render.CropSide || c.Crop.Height < render.CropSide {
					t.Errorf("page %d, item %d, box %d: a crop of %d × %d", pg.Number, c.Item, c.Slot, c.Crop.Width, c.Crop.Height)
				}
				if c.Crop.Width > render.CropSide || c.Crop.Height > render.CropSide {
					larger++
				}
				h.Write(c.PNG)
			}
		}
		if larger > 0 && digit < 48 {
			t.Errorf("at %d px, %d crops are larger than 200 × 200", digit, larger)
		}
		if got := fmt.Sprintf("%x", h.Sum(nil)); got != cropsGolden[digit] {
			t.Errorf("the crops at %d px hash to %s, want %s", digit, got, cropsGolden[digit])
		}
	}
}
