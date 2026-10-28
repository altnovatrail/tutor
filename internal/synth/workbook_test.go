package synth_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/synth"
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

// extent is the frame of a stroke's points: its least and greatest x and y.
type extent struct{ x0, y0, x1, y1 float64 }

func extentOf(s sample.Stroke) extent {
	e := extent{s.Points[0].X, s.Points[0].Y, s.Points[0].X, s.Points[0].Y}
	for _, p := range s.Points {
		e.x0, e.y0, e.x1, e.y1 = min(e.x0, p.X), min(e.y0, p.Y), max(e.x1, p.X), max(e.y1, p.Y)
	}
	return e
}

// holds is whether a stroke is a box's by the rule of boxes of R37 canvas: the middle of its frame
// in the box widened by 120 thousandths of its size, and the frame no wider and no higher than 1600
// thousandths of the box.
func holds(e extent, b synth.Box) bool {
	const side, widen = synth.BoxSide, 0.12 * synth.BoxSide
	mx, my := (e.x0+e.x1)/2, (e.y0+e.y1)/2
	return e.x1-e.x0 <= 1.6*side && e.y1-e.y0 <= 1.6*side &&
		mx >= b.Left-widen && mx <= b.Left+side+widen && my >= b.Top-widen && my <= b.Top+side+widen
}

// Every page of the workbook, drawn, has a box for each box of its items, in their order; a blank
// box has no strokes and every other box some. The writing of a box stays within it, so that by the
// rule of boxes each stroke is its box's and no other's; and the page is written in time order.
func TestPagesOfTheWorkbook(t *testing.T) {
	wb := theWorkbook(t)
	for i := range wb.Pages {
		pg := &wb.Pages[i]
		drawn := page(t, pg, 1)
		var want [][2]int
		for k, it := range pg.Items {
			for s := range it.Boxes {
				want = append(want, [2]int{k + 1, s + 1})
			}
		}
		if len(drawn.Boxes) != len(want) || drawn.Number != pg.Number {
			t.Fatalf("page %d has %d boxes drawn, want %d", pg.Number, len(drawn.Boxes), len(want))
		}
		last := int64(-1)
		for n, b := range drawn.Boxes {
			box := pg.Items[b.Item-1].Boxes[b.Slot-1]
			if [2]int{b.Item, b.Slot} != want[n] || (len(b.Strokes) == 0) != (box.Scenario == sample.BoxBlank) {
				t.Errorf("page %d, box %d is item %d, box %d, with %d strokes, for a box of the scenario %s", pg.Number, n+1, b.Item, b.Slot, len(b.Strokes), box.Scenario)
			}
			for _, s := range b.Strokes {
				e := extentOf(s)
				if e.x0 < b.Left || e.x1 > b.Left+synth.BoxSide || e.y0 < b.Top || e.y1 > b.Top+synth.BoxSide || !holds(e, b) {
					t.Errorf("page %d, item %d, box %d: the stroke %s spans %+v, out of its box at (%v, %v)", pg.Number, b.Item, b.Slot, s.ID, e, b.Left, b.Top)
				}
				for m, other := range drawn.Boxes {
					if m != n && holds(e, other) {
						t.Errorf("page %d: the stroke %s of item %d, box %d, is item %d, box %d's too", pg.Number, s.ID, b.Item, b.Slot, other.Item, other.Slot)
					}
				}
				if s.Points[0].TMs <= last {
					t.Errorf("page %d: the stroke %s begins at %d ms, before the stroke before it ends", pg.Number, s.ID, s.Points[0].TMs)
				}
				last = s.Points[len(s.Points)-1].TMs
			}
		}
	}
}

// The same seed draws the same page, and another seed another.
func TestPageAgain(t *testing.T) {
	pg := &theWorkbook(t).Pages[0]
	if !reflect.DeepEqual(page(t, pg, 3), page(t, pg, 3)) {
		t.Error("the same seed draws two different pages")
	}
	if reflect.DeepEqual(page(t, pg, 3), page(t, pg, 4)) {
		t.Error("two seeds draw the same page")
	}
}

// The pages of the workbook with seed 1 are what the measurement reads (T80): their hash holds
// them, on every machine (see the package's comment).
func TestPagesAsBefore(t *testing.T) {
	wb := theWorkbook(t)
	h := sha256.New()
	for i := range wb.Pages {
		data, err := json.Marshal(page(t, &wb.Pages[i], 1))
		if err != nil {
			t.Fatal(err)
		}
		h.Write(data)
	}
	if got, want := fmt.Sprintf("%x", h.Sum(nil)), "b5f1c90a6685c5811db46c4613ba5f84fe18cd96c6591870d5e78f2cd811efa8"; got != want {
		t.Errorf("the 20 pages of the workbook with seed 1 hash to %s, want %s", got, want)
	}
}

// A box of the scenario mirrored draws its digit mirrored: the top stroke of a 7, left to right
// when plain, runs right to left.
func TestMirroredBox(t *testing.T) {
	page := func(scenario sample.BoxScenario) []sample.Stroke {
		wb := sample.WorkbookPage{Number: 1, Part: sample.PartDev, Items: []sample.WorkbookItem{{Text: "56 : 8 = [ ]", Boxes: []sample.WorkbookBox{{
			Check: sample.BoxCheck{Kind: sample.KindNumber, Key: "7"}, Scenario: scenario, Written: ptr("7"),
		}}}}}
		return page(t, &wb, 1).Boxes[0].Strokes
	}
	plain, mirrored := page(sample.BoxCorrect), page(sample.BoxMirrored)
	if len(plain) != 1 || len(mirrored) != 1 {
		t.Fatalf("a 7 of %d and %d strokes", len(plain), len(mirrored))
	}
	p, m := plain[0].Points, mirrored[0].Points
	if !(p[5].X > p[0].X && m[5].X < m[0].X) {
		t.Errorf("the 7 is not mirrored: plain goes from x=%v to x=%v, mirrored from x=%v to x=%v", p[0].X, p[5].X, m[0].X, m[5].X)
	}
}

func ptr[T any](v T) *T { return &v }

// page draws a page, and fails the test if it cannot.
func page(t *testing.T, pg *sample.WorkbookPage, seed uint64) *synth.Page {
	t.Helper()
	p, err := synth.PageOf(pg, seed)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// A line wider than the page, or more items than a page has rows, cannot be laid out: CheckLayout
// names each by its path, and PageOf draws no such page.
func TestCheckLayout(t *testing.T) {
	wb := theWorkbook(t)
	if err := synth.CheckLayout(wb); err != nil {
		t.Fatalf("the workbook of the measurement: %v", err)
	}
	pg := &wb.Pages[0]
	pg.Items[3].Text = strings.Repeat("очень длинная строка пункта ", 3) + "[ ]"
	err := synth.CheckLayout(wb)
	if err == nil || !strings.Contains(err.Error(), "pages[0].items[3].text: the line") {
		t.Errorf("CheckLayout = %v", err)
	}
	if _, err := synth.PageOf(pg, 1); err == nil || !strings.Contains(err.Error(), "page 1, item 4") {
		t.Errorf("PageOf = %v", err)
	}
	pg.Items = append(pg.Items, pg.Items[0])
	if err := synth.CheckLayout(wb); err == nil || !strings.Contains(err.Error(), "pages[0].items: 11 items") {
		t.Errorf("CheckLayout of 11 items = %v", err)
	}
	if _, err := synth.PageOf(pg, 1); err == nil {
		t.Error("PageOf draws 11 items")
	}
}
