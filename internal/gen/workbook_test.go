package gen

import (
	"bytes"
	"flag"
	"maps"
	"math/big"
	"math/rand/v2"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/synth"
	"github.com/altnovatrail/tutor/internal/value"
	"github.com/altnovatrail/tutor/internal/workbook"
)

var update = flag.Bool("update", false, "write the generated sets of eval anew")

// The sets the extended measurement runs on (T94): 300 pages and 300 essays, from the seed 1.
const (
	setSize      = 300
	setSeed      = 1
	workbookFile = "../../eval/workbook-gen.json"
	essaysFile   = "../../eval/essays-gen.json"
)

// golden compares a set, as the repository writes it, with its file in eval, or writes the file
// anew with -update.
func golden(t *testing.T, path string, set any) {
	t.Helper()
	data, err := sample.Marshal(set)
	if err != nil {
		t.Fatal(err)
	}
	if *update {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, want) {
		t.Errorf("the set built from the seed %d is not %s: run go test ./internal/gen -update to write it anew", setSeed, path)
	}
}

// readE4Workbook reads the workbook of E4, eval/workbook.json.
func readE4Workbook(t *testing.T) *sample.Workbook {
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

// boxesOf are the boxes of a workbook, page by page.
func boxesOf(wb *sample.Workbook) []sample.WorkbookBox {
	var out []sample.WorkbookBox
	for _, pg := range wb.Pages {
		for _, it := range pg.Items {
			out = append(out, it.Boxes...)
		}
	}
	return out
}

// shapeOf is the shape of an item's line, its numbers left out.
func shapeOf(text string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return -1
		}
		return r
	}, text)
}

// The workbook of eval, eval/workbook-gen.json, is the one Workbook builds from the seed 1, byte for
// byte; go test -update writes it anew.
func TestWorkbookGolden(t *testing.T) {
	wb, err := Workbook(setSize, setSeed)
	if err != nil {
		t.Fatal(err)
	}
	golden(t, workbookFile, wb)
}

// The same seed builds the same workbook, and another seed another.
func TestWorkbookSeed(t *testing.T) {
	a, errA := Workbook(30, 7)
	b, errB := Workbook(30, 7)
	c, errC := Workbook(30, 8)
	if errA != nil || errB != nil || errC != nil {
		t.Fatal(errA, errB, errC)
	}
	da, _ := sample.Marshal(a)
	db, _ := sample.Marshal(b)
	dc, _ := sample.Marshal(c)
	if !bytes.Equal(da, db) || bytes.Equal(da, dc) {
		t.Error("the same seed builds another workbook, or another seed the same")
	}
	if _, err := Workbook(0, 1); err == nil {
		t.Error("a workbook of no pages is built")
	}
}

// The workbook of 300 pages passes its check and its layout, draws page by page and gives every
// filled box a crop; its first 20 pages are of the development, and its items are E4's, in E4's
// order, each of the kind and the shape of line it has there.
func TestWorkbookPages(t *testing.T) {
	wb, err := Workbook(setSize, setSeed)
	if err != nil {
		t.Fatal(err)
	}
	if err := wb.Check(); err != nil {
		t.Fatal(err)
	}
	if err := synth.CheckLayout(wb); err != nil {
		t.Fatal(err)
	}
	e4 := readE4Workbook(t)
	shapes := func(wb *sample.Workbook, k int) map[string]bool {
		out := map[string]bool{}
		for _, pg := range wb.Pages {
			it := pg.Items[k]
			kinds := []string{shapeOf(it.Text)}
			for _, b := range it.Boxes {
				form := ""
				if b.Check.Form != nil {
					form = string(*b.Check.Form)
				}
				kinds = append(kinds, string(b.Check.Kind)+" "+form+" "+strconv.Itoa(len(b.Check.TrapValues)))
			}
			out[strings.Join(kinds, "; ")] = true
		}
		return out
	}
	for k := range items {
		if got, want := shapes(wb, k), shapes(e4, k); !maps.Equal(got, want) {
			t.Errorf("item %d is of the shapes %v, and E4's of %v", k+1, slices.Sorted(maps.Keys(got)), slices.Sorted(maps.Keys(want)))
		}
	}
	for i, pg := range wb.Pages {
		if want := i < 20; (pg.Part == sample.PartDev) != want {
			t.Errorf("page %d is of the part %s", pg.Number, pg.Part)
		}
		drawn, err := synth.PageOf(&wb.Pages[i], setSeed)
		if err != nil {
			t.Fatalf("page %d: %v", pg.Number, err)
		}
		if i%30 == 0 {
			if _, err := workbook.CellsOf(drawn, &wb.Pages[i], render.Digit); err != nil {
				t.Errorf("page %d: %v", pg.Number, err)
			}
		}
	}
}

// Every page holds the scenarios of pageScenarios, and the mirrored and rewritten boxes are a share
// of the boxes more than twice E4's. A fraction is written stacked and inline by turns.
func TestWorkbookShares(t *testing.T) {
	wb, err := Workbook(setSize, setSeed)
	if err != nil {
		t.Fatal(err)
	}
	want := map[sample.BoxScenario]int{}
	for _, sc := range pageScenarios {
		want[sc]++
	}
	for _, pg := range wb.Pages {
		got := map[sample.BoxScenario]int{}
		for _, it := range pg.Items {
			for _, b := range it.Boxes {
				got[b.Scenario]++
			}
		}
		if !maps.Equal(got, want) {
			t.Errorf("page %d holds the scenarios %v, not %v", pg.Number, got, want)
		}
	}
	hard := func(wb *sample.Workbook) float64 {
		n := 0
		boxes := boxesOf(wb)
		for _, b := range boxes {
			if b.Scenario == sample.BoxMirrored || b.Scenario == sample.BoxRewritten {
				n++
			}
		}
		return float64(n) / float64(len(boxes))
	}
	if got, e4 := hard(wb), hard(readE4Workbook(t)); got < 2*e4 {
		t.Errorf("the mirrored and rewritten boxes are %.3f of the boxes, and E4's %.3f", got, e4)
	}
	writing := map[sample.FractionWriting]int{}
	for _, b := range boxesOf(wb) {
		if b.Fraction != nil {
			writing[*b.Fraction]++
		}
	}
	if d := writing[sample.Stacked] - writing[sample.Inline]; d < 0 || d > 1 {
		t.Errorf("the fractions are written %v", writing)
	}
}

// Each item draws, over many numbers, boxes the check takes when the key is written, with traps
// that are the mistakes they name.
func TestItems(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	rat := func(v string) *big.Rat {
		r, ok := new(big.Rat).SetString(v)
		if !ok {
			t.Fatalf("%q is no number", v)
		}
		return r
	}
	nums := func(text string) []int {
		var out []int
		for _, f := range strings.FieldsFunc(text, func(r rune) bool { return r < '0' || r > '9' }) {
			n, _ := strconv.Atoi(f)
			out = append(out, n)
		}
		return out
	}
	v := sample.Version
	for k, item := range items {
		for range 300 {
			it := item(rng)
			for j := range it.Boxes {
				it.Boxes[j].Scenario = sample.BoxCorrect
				key := it.Boxes[j].Check.Key
				it.Boxes[j].Written = &key
				if it.Boxes[j].Check.Kind == sample.KindFraction {
					w := sample.Inline
					it.Boxes[j].Fraction = &w
				}
			}
			wb := &sample.Workbook{Format: sample.WorkbookFormat, Version: &v, Pages: []sample.WorkbookPage{{Number: 1, Part: sample.PartTest, Items: []sample.WorkbookItem{it}}}}
			if err := wb.Check(); err != nil {
				t.Fatalf("item %d, %q: %v", k+1, it.Text, err)
			}
			n, b := nums(it.Text), it.Boxes[0].Check
			is := func(v string, num, den int) bool { return rat(v).Cmp(big.NewRat(int64(num), int64(den))) == 0 }
			key, trap := b.Key, b.TrapValues[0].Value
			var ok bool
			switch k {
			case 0: // a × [ ] = p: the trap subtracts
				ok = is(key, n[1], n[0]) && is(trap, n[1]-n[0], 1)
			case 1: // n : d: the trap is the next of the table
				ok = is(key, n[0], n[1]) && is(trap, n[0]/n[1]+1, 1)
			case 2: // [ ] + b = s: the trap adds
				ok = is(key, n[1]-n[0], 1) && is(trap, n[1]+n[0], 1)
			case 3: // a − b of three digits, with a borrow: the trap takes the smaller digit from the larger
				noBorrow := 0
				for place := 100; place >= 1; place /= 10 {
					x, y := n[0]/place%10, n[1]/place%10
					noBorrow = noBorrow*10 + max(x, y) - min(x, y)
				}
				ok = n[0]-n[1] >= 100 && (n[0]%10 < n[1]%10 || n[0]/10%10 < n[1]/10%10) && is(key, n[0]-n[1], 1) && is(trap, noBorrow, 1)
			case 4: // n : d, a quotient and a remainder: one less, and the remainder over the divisor
				q, r := n[0]/n[1], n[0]%n[1]
				ok = is(key, q, 1) && is(trap, q-1, 1) && r > 0 && is(it.Boxes[1].Check.Key, r, 1) && is(it.Boxes[1].Check.TrapValues[0].Value, r+n[1], 1)
			case 5: // a/d + b/d: the trap adds the denominators
				ok = is(key, n[0]+n[2], n[1]) && is(trap, n[0]+n[2], 2*n[1]) && n[0]+n[2] < n[1]
			case 6: // a/d − b/d: the trap adds
				ok = is(key, n[0]-n[2], n[1]) && is(trap, n[0]+n[2], n[1])
			case 7: // x units of a whole: the trap divides by the wrong whole
				whole := map[bool]int{true: 100, false: 1000}[strings.Contains(it.Text, "см")]
				if strings.Contains(it.Text, "минут") {
					whole = 60
				}
				ok = is(key, n[0], whole) && is(trap, n[0], map[int]int{100: 10, 1000: 100, 60: 100}[whole])
			case 8: // n/m reduced: the traps are n/m, and the numerator reduced alone
				g := gcd(n[0], n[1])
				ok = g > 1 && is(key, n[0], n[1]) && trap == frac(n[0], n[1]) && b.TrapValues[1].Value == frac(n[0]/g, n[1])
			case 9: // a/d + b/d in lowest terms: the traps are the sum not reduced, and the denominators added
				ok = is(key, n[0]+n[2], n[1]) && trap == frac(n[0]+n[2], n[1]) && is(b.TrapValues[1].Value, n[0]+n[2], 2*n[1]) && gcd(n[0]+n[2], n[1]) > 1
			}
			if form := b.Form; ok && k >= 8 {
				ok = form != nil && *form == value.Reduced
			}
			if !ok {
				t.Fatalf("item %d, %q, has the check %+v", k+1, it.Text, b)
			}
		}
	}
}

// A box can hold a scenario when its key has the digit the scenario needs.
func TestPossible(t *testing.T) {
	check := func(key string) sample.BoxCheck {
		return sample.BoxCheck{Kind: sample.KindNumber, Key: key, TrapValues: []sample.TrapValue{{Value: "1", Trap: "a"}}}
	}
	for _, c := range []struct {
		sc   sample.BoxScenario
		key  string
		want bool
	}{
		{sample.BoxMirrored, "36", false},
		{sample.BoxMirrored, "1/25", true},
		{sample.BoxNearKey, "20", false},
		{sample.BoxNearKey, "2/10", true},
		{sample.BoxSimilarDigits, "202", false},
		{sample.BoxError, "6", true},
		{sample.BoxBlank, "0", true},
	} {
		if got := possible(c.sc, check(c.key)); got != c.want {
			t.Errorf("possible(%s, %q) = %v", c.sc, c.key, got)
		}
	}
	if possible(sample.BoxRewritten, sample.BoxCheck{Key: "5"}) {
		t.Error("a box of no trap can be rewritten")
	}
}

// A value near the key swaps one digit of a pair for its pair's, each digit of a pair in its turn,
// and the check of the file reads it so; a near value that is one of the box's trap values is none:
// it would make the box an error.
func TestNearKey(t *testing.T) {
	c := sample.BoxCheck{Kind: sample.KindFraction, Key: "8/14"}
	if got := nearKeys(c); !slices.Equal(got, []string{"3/14", "8/74", "8/19"}) {
		t.Errorf("nearKeys(8/14) = %v", got)
	}
	for _, w := range nearKeys(c) {
		b := sample.WorkbookBox{Check: c, Scenario: sample.BoxNearKey, Written: &w}
		if _, _, ok := b.Leaning(); !ok {
			t.Errorf("the near key %q does not read as one", w)
		}
	}
	c.TrapValues = []sample.TrapValue{{Value: "8/74", Trap: "t"}}
	if got := nearKeys(c); !slices.Equal(got, []string{"3/14", "8/19"}) {
		t.Errorf("nearKeys(8/14) with the trap 8/74 = %v", got)
	}
	if possible(sample.BoxNearKey, sample.BoxCheck{Key: "3", TrapValues: []sample.TrapValue{{Value: "8", Trap: "t"}}}) {
		t.Error("a box whose only near key is a trap value can be near-key")
	}
}
