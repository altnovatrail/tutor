package synth

import (
	"math"
	"os"
	"slices"
	"testing"

	"github.com/altnovatrail/tutor/internal/sample"
)

// The pairs of similar digits are each the other's, and a sheet of the near set leans the last
// digit of its answer line toward the digit of its pair.
func TestSimilar(t *testing.T) {
	pairs := 0
	for _, r := range "0123456789" {
		s, ok := Similar(r)
		if !ok {
			continue
		}
		pairs++
		if back, ok := Similar(s); !ok || back != r {
			t.Errorf("%c leans toward %c, and %c toward %c", r, s, s, back)
		}
	}
	if _, ok := Similar('0'); ok || pairs != 8 {
		t.Errorf("0 has a pair, or %d digits have one, not 8", pairs)
	}
	for _, c := range []struct {
		line   string
		at     int
		toward rune
		ok     bool
	}{
		{"66 : 6 = 11", 10, '7', true},
		{"x = 4", 4, '9', true},
		{"Ответ: 5 столбов", 7, '6', true},
		{"x = 10", 0, 0, false},
		{"Ответ:", 0, 0, false},
	} {
		at, toward, ok := SimilarDigit(c.line)
		if ok != c.ok || ok && (at != c.at || toward != c.toward) {
			t.Errorf("%q: %d, %c, %v; want %d, %c, %v", c.line, at, toward, ok, c.at, c.toward, c.ok)
		}
	}
}

// A digit leaning no way is the digit of the font, corners and all, and one leaning all the way is
// the digit of its pair, the stroke taken the way nearer the first: 4 written from its foot and 9
// toward it. Every digit of a pair is one stroke, which is what lets them mix.
func TestShifted(t *testing.T) {
	for _, from := range "0123456789" {
		to, ok := Similar(from)
		if !ok {
			continue
		}
		if len(font[from]) != 1 || len(font[to]) != 1 {
			t.Fatalf("%c or %c is drawn in more strokes than one", from, to)
		}
		if got := shifted(font[from], font[to], 0)[0]; !holds(got, font[from][0]) {
			t.Errorf("%c leaning no way is %v, not the font's %v", from, got, font[from][0])
		}
		back := slices.Clone(font[to][0])
		slices.Reverse(back)
		if got := shifted(font[from], font[to], 1)[0]; !holds(got, font[to][0]) && !holds(got, back) {
			t.Errorf("%c leaning all the way is %v, not %c", from, got, to)
		}
	}
	got := shifted(font['9'], font['4'], 1)[0]
	if !near(got[len(got)-1], pt{0.7, 1}) {
		t.Errorf("9 leaning toward 4 ends at %v, want the foot of 4", got[len(got)-1])
	}
	if r := at([]pt{{0, 0}, {0, 0}, {1, 0}}, evenly(3)); !slices.Equal(r, []pt{{0, 0}, {0.5, 0}, {1, 0}}) {
		t.Errorf("a stroke with a point twice is taken at %v", r)
	}
}

// holds reports whether a stroke is the polyline: its ends are the polyline's, every corner of the
// polyline is one of its points, and every one of its points lies on the polyline.
func holds(stroke, poly []pt) bool {
	if !near(stroke[0], poly[0]) || !near(stroke[len(stroke)-1], poly[len(poly)-1]) {
		return false
	}
	for _, c := range poly {
		if !slices.ContainsFunc(stroke, func(p pt) bool { return near(p, c) }) {
			return false
		}
	}
	for _, p := range stroke {
		on := false
		for i := 1; i < len(poly) && !on; i++ {
			a, b := poly[i-1], poly[i]
			cross := (b.x-a.x)*(p.y-a.y) - (b.y-a.y)*(p.x-a.x)
			on = math.Abs(cross) < 1e-9 && p.x >= min(a.x, b.x)-1e-9 && p.x <= max(a.x, b.x)+1e-9 && p.y >= min(a.y, b.y)-1e-9 && p.y <= max(a.y, b.y)+1e-9
		}
		if !on {
			return false
		}
	}
	return true
}

// near reports whether two points are one, but for the last bits of their sums.
func near(a, b pt) bool { return math.Abs(a.x-b.x) < 1e-12 && math.Abs(a.y-b.y) < 1e-12 }

// Every sheet of the near set is drawn with its answer leaning and passes the check with its bank:
// drawn at the share 0, it is the same up to the stroke of the digit that leans, of its answer line.
func TestNearSheets(t *testing.T) {
	read := func(path string, f func(*os.File) error) {
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = file.Close() }()
		if err := f(file); err != nil {
			t.Fatal(err)
		}
	}
	var bank *sample.Bank
	var plan *sample.Plan
	read("../../eval/near-bank.json", func(f *os.File) (err error) { bank, err = sample.ReadBank(f); return err })
	read("../../eval/near-sheets.json", func(f *os.File) (err error) { plan, err = sample.ReadPlan(f); return err })
	for i := range plan.Sheets {
		sh := plan.Sheets[i]
		s := Sheet(&sh, 1)
		if err := s.Check(); err != nil {
			t.Fatalf("sheet %d: %v", sh.Number, err)
		}
		if err := s.CheckWithBank(bank); err != nil {
			t.Fatalf("sheet %d: %v", sh.Number, err)
		}
		// Leaning no way, the strokes before the digit that leans are drawn the same, and the first
		// stroke drawn otherwise is of the answer line: the digit, and then, its points drawn anew,
		// whatever follows it.
		p := SheetLeaning(&sh, 1, 0)
		first := slices.IndexFunc(s.Strokes, func(st sample.Stroke) bool {
			j := slices.IndexFunc(p.Strokes, func(q sample.Stroke) bool { return q.ID == st.ID })
			return j < 0 || !slices.Equal(st.Points, p.Strokes[j].Points)
		})
		if first < 0 || !slices.Contains(s.Lines[sh.Answer-1].Strokes, s.Strokes[first].ID) {
			t.Errorf("sheet %d: the first stroke that leans is %d, not of the answer line", sh.Number, first)
		}
	}
}
