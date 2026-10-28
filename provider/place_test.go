package provider_test

import (
	"testing"

	"github.com/altnovatrail/tutor/provider"
)

// A box in pixels stands on the picture in whole thousandths of its size, clamped to it, the low
// corner rounded down and the high one up (R53 spec). A box with no area on the picture has no
// place, even where its edges would round to two thousandths apart, and neither has a box on a
// picture of no size.
func TestPlace(t *testing.T) {
	for _, c := range []struct {
		x1, y1, x2, y2, w, h int
		place                [4]int64
		ok                   bool
	}{
		{100, 50, 200, 150, 400, 200, [4]int64{250, 250, 750, 500}, true},
		{-20, -5, 401, 999, 400, 200, [4]int64{0, 0, 1000, 1000}, true},
		{1, 1, 2, 2, 300, 300, [4]int64{3, 3, 7, 7}, true},
		{5, 10, 5, 40, 300, 300, [4]int64{}, false},
		{10, 7, 40, 7, 300, 300, [4]int64{}, false},
		{500, 10, 600, 20, 400, 200, [4]int64{}, false},
		{-30, -30, -10, -10, 400, 200, [4]int64{}, false},
		{1, 1, 2, 2, 0, 200, [4]int64{}, false},
	} {
		p, ok := provider.Place(c.x1, c.y1, c.x2, c.y2, c.w, c.h)
		if p != c.place || ok != c.ok {
			t.Errorf("the box %v on %d × %d stands at %v, %v; want %v, %v", [4]int{c.x1, c.y1, c.x2, c.y2}, c.w, c.h, p, ok, c.place, c.ok)
		}
	}
}
