package report

import (
	"math"
	"testing"
)

// The bound of a share on sheets that are all alike is the Clopper–Pearson bound of its counts: by
// the binomial, the values of a table; with no events, 1 − 0.05^(1/n). Events heaped on one sheet
// widen it, and no answers at all give 1.
func TestUpper(t *testing.T) {
	// Each case is one event or none a sheet, so the sheets add no variance of their own.
	spread := func(x, n int) ([]int, []int) {
		events, totals := make([]int, n), make([]int, n)
		for i := range totals {
			totals[i] = 1
			if i < x {
				events[i] = 1
			}
		}
		return events, totals
	}
	for _, c := range []struct {
		x, n int
		want float64
	}{
		{0, 10, 0.258866}, {1, 10, 0.394163}, {5, 10, 0.777559}, {0, 100, 0.029513}, {3, 60, 0.124187}, {2, 30, 0.195326},
	} {
		events, totals := spread(c.x, c.n)
		if got := Upper(events, totals); math.Abs(got-c.want) > 1e-5 {
			t.Errorf("%d of %d: %.6f, want %.6f", c.x, c.n, got, c.want)
		}
	}
	if got, want := Upper([]int{0, 0}, []int{20, 30}), 1-math.Pow(0.05, 1.0/50); math.Abs(got-want) > 1e-9 {
		t.Errorf("no events in 50: %.6f, want %.6f", got, want)
	}
	heaped := Upper([]int{6, 0, 0, 0, 0, 0}, []int{6, 6, 6, 6, 6, 6})
	even := Upper([]int{1, 1, 1, 1, 1, 1}, []int{6, 6, 6, 6, 6, 6})
	if !(heaped > even) {
		t.Errorf("six events on one sheet bound %.4f, no wider than spread over six, %.4f", heaped, even)
	}
	if Upper(nil, nil) != 1 || Upper([]int{4}, []int{4}) != 1 {
		t.Error("no answers, or every answer an event, is not bounded by 1")
	}
	if d := designEffect([]int{1, 1}, []int{4, 4}, 0.25, 8); d != 1 {
		t.Errorf("two sheets with the share alike: Deff %v, want 1", d)
	}
	if got := incompleteBeta(0.3, 2, 5) + incompleteBeta(0.7, 5, 2); math.Abs(got-1) > 1e-12 {
		t.Errorf("I_p(a, b) + I_(1−p)(b, a) = %v, want 1", got)
	}
}
