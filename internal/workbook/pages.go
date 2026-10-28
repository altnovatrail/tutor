// Package workbook is the measurement of the workbook in epic E4 (T78–T80; docs/architecture.md,
// 3.4, «Рабочая тетрадь»): its pages, which code draws in memory from eval/workbook.json, and the
// crop of each filled box, which is all a reading of the page sees (R53 spec, amendment of
// 2026-10-27): the ink of the box's strokes alone, cut by the rule of the eval protocol, without the
// box's frame and the print of its line. A blank box gives no crop.
package workbook

import (
	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/synth"
)

// Cell is a filled box of a page as a reading sees it: the places of its item and of the box among
// the item's boxes, from 1, the box, and the crop of its strokes with its PNG.
type Cell struct {
	Item, Slot int
	Box        sample.WorkbookBox
	Crop       *render.Crop
	PNG        []byte
}

// Cells draws a page from the seed and cuts the crop of each filled box with its digits digit px
// high, in the order of the page: by item, then by box. A blank box is not among them.
func Cells(pg *sample.WorkbookPage, seed uint64, digit int) ([]Cell, error) {
	drawn, err := synth.PageOf(pg, seed)
	if err != nil {
		return nil, err
	}
	return CellsOf(drawn, pg, digit)
}

// CellsOf cuts the crop of each filled box of a page drawn, as Cells does: a page drawn once gives
// its cells at every height.
func CellsOf(drawn *synth.Page, pg *sample.WorkbookPage, digit int) ([]Cell, error) {
	var cells []Cell
	for _, b := range drawn.Boxes {
		if len(b.Strokes) == 0 {
			continue
		}
		c, err := render.CropOf(b.Strokes, nil, digit)
		if err != nil {
			return nil, err
		}
		img, err := c.Draw(b.Strokes)
		if err != nil {
			return nil, err
		}
		png, err := render.PNG(img)
		if err != nil {
			return nil, err
		}
		cells = append(cells, Cell{Item: b.Item, Slot: b.Slot, Box: pg.Items[b.Item-1].Boxes[b.Slot-1], Crop: c, PNG: png})
	}
	return cells, nil
}
