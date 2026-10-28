package hint_test

import (
	"testing"

	"github.com/altnovatrail/tutor/internal/hint"
	"github.com/altnovatrail/tutor/internal/judge"
	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/sample"
)

var traps = []sample.Trap{{ID: "jump-length", Description: "отложил прыжок не той длины"}}

// An answer reads as the schema has it: the box with its corners in order, or none; the trap of
// the task, or none; and the hint as written.
func TestRead(t *testing.T) {
	a, err := hint.Read(`{"box": {"x1": 40, "y1": 30, "x2": 10, "y2": 5}, "trap": "jump-length", "hint": "Где прыжок?"}`, traps)
	if err != nil || *a.Box != (judge.Box{X1: 10, Y1: 5, X2: 40, Y2: 30}) || a.Trap != "jump-length" || a.Hint != "Где прыжок?" {
		t.Errorf("the answer reads %+v, %v", a, err)
	}
	a, err = hint.Read(`{"box": null, "trap": null, "hint": ""}`, nil)
	if err != nil || a.Box != nil || a.Trap != "" || a.Hint != "" {
		t.Errorf("an answer of nulls reads %+v, %v", a, err)
	}
	for name, answer := range map[string]string{
		"not JSON":            `box`,
		"no box":              `{"trap": null, "hint": "?"}`,
		"no trap":             `{"box": null, "hint": "?"}`,
		"no hint":             `{"box": null, "trap": null}`,
		"a field of its own":  `{"box": null, "trap": null, "hint": "?", "verdict": "error"}`,
		"a corner short":      `{"box": {"x1": 1, "y1": 1, "x2": 2}, "trap": null, "hint": "?"}`,
		"a corner of its own": `{"box": {"x1": 1, "y1": 1, "x2": 2, "y2": 2, "z": 0}, "trap": null, "hint": "?"}`,
		"a box of a type":     `{"box": "here", "trap": null, "hint": "?"}`,
		"a trap of a type":    `{"box": null, "trap": 3, "hint": "?"}`,
		"a trap not there":    `{"box": null, "trap": "jump-count", "hint": "?"}`,
		"a hint of a type":    `{"box": null, "trap": null, "hint": 3}`,
		"more after it":       `{"box": null, "trap": null, "hint": "?"} {}`,
	} {
		if _, err := hint.Read(answer, traps); err == nil {
			t.Errorf("%s: read", name)
		}
	}
}

// A box in pixels stands on the crop in thousandths of its size, clamped to the picture, as the
// adapter tells it (R53 spec); a box with no area on the picture has no place.
func TestPlace(t *testing.T) {
	c := &render.Crop{Width: 400, Height: 200, SheetBox: [4]float64{100, 50, 300, 450}, Scale: 1}
	for _, tc := range []struct {
		box   judge.Box
		place [4]int64
		ok    bool
	}{
		{judge.Box{X1: 100, Y1: 50, X2: 200, Y2: 150}, [4]int64{250, 250, 750, 500}, true},
		{judge.Box{X1: -20, Y1: -5, X2: 401, Y2: 999}, [4]int64{0, 0, 1000, 1000}, true},
		{judge.Box{X1: 1, Y1: 1, X2: 2, Y2: 2}, [4]int64{5, 2, 10, 5}, true},
		{judge.Box{X1: 500, Y1: 10, X2: 600, Y2: 20}, [4]int64{}, false},
		{judge.Box{X1: 10, Y1: 30, X2: 20, Y2: 30}, [4]int64{}, false},
	} {
		p, ok := hint.Place(tc.box, c)
		if p != tc.place || ok != tc.ok {
			t.Errorf("the box %+v stands at %v, %v; want %v, %v", tc.box, p, ok, tc.place, tc.ok)
		}
	}
	if b := hint.OnSheet([4]int64{250, 250, 750, 500}, c); b != (judge.Box{X1: 150, Y1: 150, X2: 250, Y2: 250}) {
		t.Errorf("the place stands on the sheet at %+v", b)
	}
	if b := hint.OnCrop(judge.Box{X1: 150, Y1: 150, X2: 250, Y2: 250}, c); b != (judge.Box{X1: 100, Y1: 50, X2: 200, Y2: 150}) {
		t.Errorf("the box stands on the crop at %+v", b)
	}
	if hit, _, ok := hint.Hit(nil, judge.Box{X1: 500, Y1: 10, X2: 600, Y2: 20}, c); hit != 0 || ok {
		t.Errorf("a box off the crop hits %d, %v", hit, ok)
	}
}

// The box round the first error, in pixels of the crop at 32 px, goes to thousandths of the crop,
// then to units of the sheet, and hits the part or line of the first error, on every sheet with a
// mistake of the three plans: the way the stub's box goes, and a model's.
func TestRoundTrip(t *testing.T) {
	n := 0
	for _, set := range []struct{ name, plan, bank string }{{"free", freePlan, freeBank}, {"lines", linesPlan, linesBank}, {"near", nearPlan, nearBank}} {
		for _, sh := range drawn(t, set.name, set.plan, set.bank) {
			if !hint.Wrong(sh) {
				continue
			}
			n++
			fe := *sh.Sample.Expected.FirstError
			crop := shown(t, sh).Crop
			box := hint.OnCrop(*judge.Around(sh.Sample, fe), crop)
			hit, place, ok := hint.Hit(sh.Sample, box, crop)
			if hit != fe || !ok {
				t.Errorf("%s/%d: the box %+v at %v hits %d, want %d", set.name, sh.Number, box, place, hit, fe)
			}
		}
	}
	if n != 20+19+6 {
		t.Errorf("%d sheets with a mistake", n)
	}
}
