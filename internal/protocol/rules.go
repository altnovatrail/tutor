package protocol

import (
	"slices"

	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"
	observationv1 "github.com/altnovatrail/spec/contracts/novatrail/observation/v1"

	"github.com/altnovatrail/tutor/internal/lines"
)

// The numbers of the rules of boxes and of snapping (section 12), as the vectors repeat them. A
// stroke's box is no wider and no taller than BoxLimit thousandths of a box of the page that holds
// it, and its middle lies in the box widened by BoxMargin thousandths of the box's size. A box of a
// model, widened by SnapMargin thousandths of the height of the digits drawn, takes a stroke when
// it holds SnapAbove / SnapBelow of its points or more. The margin and the share are parameters of
// eval, which eval may choose anew by its own data, and a new choice comes in a new package.
const (
	BoxMargin  = 120
	BoxLimit   = 1600
	SnapMargin = 250
	SnapAbove  = 2
	SnapBelow  = 3
	// snapCeiling is the highest h_med the scale of a crop takes, in tenths: 100 units, the rule of
	// the crop's (section 12), which internal/render follows too.
	snapCeiling = 1000
)

// tenthsBox is the box of points in whole tenths of a unit; ok is false when there is no point.
func tenthsBox(points []*evalv1.FinalPoint) (b *evalv1.TenthsBox, ok bool) {
	for _, p := range points {
		x, y := int32(lines.Tenths(p.GetX())), int32(lines.Tenths(p.GetY()))
		if b == nil {
			b = &evalv1.TenthsBox{Ymin: y, Xmin: x, Ymax: y, Xmax: x}
			continue
		}
		b.Ymin, b.Xmin, b.Ymax, b.Xmax = min(b.Ymin, y), min(b.Xmin, x), max(b.Ymax, y), max(b.Xmax, x)
	}
	return b, b != nil
}

// BoxOf is the box of a workbook's page that a stroke of these points fills, by the rule of boxes
// (section 12, "The boxes of a workbook"): its index in the page's boxes, in the task's order, or −1
// when no box holds the stroke and it is the draft. A box holds the stroke when its size and its
// middle do; of those, the stroke's is the one with the most of its points in the box widened, and
// of those the earliest.
func BoxOf(points []*evalv1.FinalPoint, boxes []*evalv1.PageBox) int {
	b, ok := tenthsBox(points)
	if !ok {
		return -1
	}
	x0, x1, y0, y1 := int64(b.Xmin), int64(b.Xmax), int64(b.Ymin), int64(b.Ymax)
	best, most := -1, 0
	for i, box := range boxes {
		L, T, W, H := 10*int64(box.GetLeft()), 10*int64(box.GetTop()), 10*int64(box.GetWidth()), 10*int64(box.GetHeight())
		left, right := 1000*L-BoxMargin*W, 1000*(L+W)+BoxMargin*W
		top, bottom := 1000*T-BoxMargin*H, 1000*(T+H)+BoxMargin*H
		size := 1000*(x1-x0) <= BoxLimit*W && 1000*(y1-y0) <= BoxLimit*H
		middle := 1000*(x0+x1) >= 2*left && 1000*(x0+x1) <= 2*right && 1000*(y0+y1) >= 2*top && 1000*(y0+y1) <= 2*bottom
		if !size || !middle {
			continue
		}
		held := 0
		for _, p := range points {
			x, y := 1000*lines.Tenths(p.GetX()), 1000*lines.Tenths(p.GetY())
			if x >= left && x <= right && y >= top && y <= bottom {
				held++
			}
		}
		if best < 0 || held > most {
			best, most = i, held
		}
	}
	return best
}

// Table is the table "box → stroke IDs" of a workbook's page: the boxes its strokes fill, in the
// task's order, each with the IDs of its strokes ascending and without its crops. A stroke of the
// draft is in none.
func Table(strokes []*evalv1.FinalStroke, boxes []*evalv1.PageBox) []*evalv1.FilledBox {
	filled := make([][]int32, len(boxes))
	for _, st := range strokes {
		if i := BoxOf(st.GetPoints(), boxes); i >= 0 {
			filled[i] = append(filled[i], st.GetId())
		}
	}
	var out []*evalv1.FilledBox
	for i, ids := range filled {
		if len(ids) > 0 {
			slices.Sort(ids)
			out = append(out, &evalv1.FilledBox{AnswerSlot: boxes[i].GetAnswerSlot(), StrokeIds: ids})
		}
	}
	return out
}

// Snap is what a box of a model on a crop of the work takes (section 12, "Snapping"): the IDs of
// the strokes it takes, ascending, and the box of their points in tenths; or none, nil and nil,
// when it takes no stroke. The crop is one of the set, and the strokes are those its set was drawn
// of. The box is widened first by SnapMargin thousandths of the crop's height drawn and takes a
// stroke when it holds k of its n points, k ≥ 1 and SnapBelow × k ≥ SnapAbove × n. In eval, the
// strokes are those a history of writing shows, never a sample's final geometry, whose points may
// be other than those that came (the 2026-11-08 amendment to R94 of spec; R77).
func Snap(set *evalv1.CropSet, crop *evalv1.Crop, box *observationv1.CropBox, strokes []*evalv1.FinalStroke) ([]int32, *evalv1.TenthsBox) {
	B := set.GetInkBox()
	H := min(int64(set.GetBundleMedian()), snapCeiling)
	D, w, h := int64(crop.GetDrawnHeight()), int64(crop.GetWidth()), int64(crop.GetHeight())
	my, mx := int64(crop.GetMarginTop()), int64(crop.GetMarginLeft())
	mu := SnapMargin * D
	holds := func(p *evalv1.FinalPoint) bool {
		X := 1000 * ((lines.Tenths(p.GetX())-int64(B.GetXmin()))*D + mx*H)
		Y := 1000 * ((lines.Tenths(p.GetY())-int64(B.GetYmin()))*D + my*H)
		return (int64(box.GetXmin())*w-mu)*H <= X && X <= (int64(box.GetXmax())*w+mu)*H &&
			(int64(box.GetYmin())*h-mu)*H <= Y && Y <= (int64(box.GetYmax())*h+mu)*H
	}
	var ids []int32
	var points []*evalv1.FinalPoint
	for _, st := range strokes {
		k, n := 0, len(st.GetPoints())
		for _, p := range st.GetPoints() {
			if holds(p) {
				k++
			}
		}
		if k >= 1 && SnapBelow*k >= SnapAbove*n {
			ids = append(ids, st.GetId())
			points = append(points, st.GetPoints()...)
		}
	}
	if len(ids) == 0 {
		return nil, nil
	}
	slices.Sort(ids)
	b, _ := tenthsBox(points)
	return ids, b
}
