package gen

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"

	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/synth"
	"github.com/altnovatrail/tutor/internal/value"
)

// pageScenarios are the scenarios of the eleven boxes of a page, the most constrained first, as
// they are given out. E4 gave one box of a page to each scenario but correct and five to that one
// (R40). Here three boxes are mirrored and two rewritten: the digits written mirrored and the
// values crossed out are where E4 found its false alarms, and the 2026-10-28 amendment to R56
// spec measures them on a line of their own, so they take five boxes of eleven, not two.
var pageScenarios = []sample.BoxScenario{
	sample.BoxMirrored, sample.BoxMirrored, sample.BoxMirrored,
	sample.BoxNearKey, sample.BoxSimilarDigits, sample.BoxError,
	sample.BoxRewritten, sample.BoxRewritten, sample.BoxBlank,
	sample.BoxCorrect, sample.BoxCorrect,
}

// items are the ten items of a page, in the order of E4's pages (R40): five of the kind number,
// three fractions in any form and two in lowest terms, each with its own numbers.
var items = []func(*rand.Rand) sample.WorkbookItem{
	multiplication, division, addend, subtraction, remainder,
	fractionSum, fractionDifference, share, reduce, reducedSum,
}

// Workbook builds the pages of a workbook, each of the ten items of E4's pages with numbers of its
// own and its eleven boxes written as pageScenarios says, from the seed. The first pages go to the
// development, devCount of them, and the rest to the test. A fraction is written stacked or inline
// by turns, box after box across the workbook, as in E4. The workbook passes its check and its
// layout, or Workbook returns an error.
func Workbook(pages int, seed uint64) (*sample.Workbook, error) {
	if pages < 1 {
		return nil, fmt.Errorf("gen: a workbook has at least one page, not %d", pages)
	}
	rng := newRand(seed, workbookStream)
	v := sample.Version
	wb := &sample.Workbook{Format: sample.WorkbookFormat, Version: &v, Pages: make([]sample.WorkbookPage, pages)}
	for i := range wb.Pages {
		pg, err := pageOf(rng)
		if err != nil {
			return nil, fmt.Errorf("gen: page %d: %w", i+1, err)
		}
		pg.Number, pg.Part = i+1, partOf(i, pages)
		wb.Pages[i] = pg
	}
	stacked := true
	for i := range wb.Pages {
		for k := range wb.Pages[i].Items {
			for j := range wb.Pages[i].Items[k].Boxes {
				b := &wb.Pages[i].Items[k].Boxes[j]
				if b.Check.Kind != sample.KindFraction || b.Written == nil {
					continue
				}
				w := sample.Inline
				if stacked {
					w = sample.Stacked
				}
				b.Fraction, stacked = &w, !stacked
			}
		}
	}
	if err := wb.Check(); err != nil {
		return nil, err
	}
	if err := synth.CheckLayout(wb); err != nil {
		return nil, err
	}
	return wb, nil
}

// pageOf builds the items of a page and writes its boxes. Numbers that leave a scenario no box it
// can be written in are drawn again.
func pageOf(rng *rand.Rand) (sample.WorkbookPage, error) {
	for range 100 {
		pg := sample.WorkbookPage{Items: make([]sample.WorkbookItem, len(items))}
		var boxes []*sample.WorkbookBox
		for k, item := range items {
			pg.Items[k] = item(rng)
			for j := range pg.Items[k].Boxes {
				boxes = append(boxes, &pg.Items[k].Boxes[j])
			}
		}
		if len(boxes) != len(pageScenarios) {
			return pg, fmt.Errorf("the items have %d boxes, and the scenarios are %d", len(boxes), len(pageScenarios))
		}
		if assign(rng, boxes) {
			for _, b := range boxes {
				write(rng, b)
			}
			return pg, nil
		}
	}
	return sample.WorkbookPage{}, errors.New("no numbers give every scenario a box")
}

// assign gives each box of a page a scenario of pageScenarios, in the order of a random shuffle of
// the boxes: each scenario the first box left that can hold it. It reports false when a shuffle
// after a few leaves a scenario no box.
func assign(rng *rand.Rand, boxes []*sample.WorkbookBox) bool {
	for range 20 {
		order := rng.Perm(len(boxes))
		given := make([]bool, len(boxes))
		ok := true
		for _, sc := range pageScenarios {
			found := false
			for _, i := range order {
				if !given[i] && possible(sc, boxes[i].Check) {
					boxes[i].Scenario, given[i], found = sc, true, true
					break
				}
			}
			if !found {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// possible is whether a box of the check can be written as the scenario says: mirrored, with a
// digit of 2, 5, 7 and 9 in the key; near the key or with a digit leaning, with a digit of a pair;
// an error or rewritten, with a trap value.
func possible(sc sample.BoxScenario, c sample.BoxCheck) bool {
	switch sc {
	case sample.BoxMirrored:
		_, ok := sample.Mirrored(c.Key)
		return ok
	case sample.BoxNearKey:
		return len(nearKeys(c)) > 0
	case sample.BoxSimilarDigits:
		return len(pairedDigits(c.Key)) > 0
	case sample.BoxError, sample.BoxRewritten:
		return len(c.TrapValues) > 0
	}
	return true
}

// pairedDigits are the places, among the characters of a value, of its digits of a pair.
func pairedDigits(v string) []int {
	var at []int
	for i, r := range []rune(v) {
		if _, ok := sample.PairOf(r); ok {
			at = append(at, i)
		}
	}
	return at
}

// write writes a box as its scenario says: the key, plainly or with a digit leaning or mirrored;
// the first trap value; one of its near keys, at random; the first trap value crossed out and the
// key beside it; or nothing.
func write(rng *rand.Rand, b *sample.WorkbookBox) {
	key := b.Check.Key
	switch b.Scenario {
	case sample.BoxCorrect, sample.BoxSimilarDigits, sample.BoxMirrored:
		b.Written = &key
	case sample.BoxError:
		w := b.Check.TrapValues[0].Value
		b.Written = &w
	case sample.BoxNearKey:
		near := nearKeys(b.Check)
		w := near[rng.IntN(len(near))]
		b.Written = &w
	case sample.BoxRewritten:
		crossed := b.Check.TrapValues[0].Value
		b.Written, b.Crossed = &key, &crossed
	}
}

// nearKeys are the near keys of a box: its key with one of its digits of a pair swapped for the
// digit of its pair, each place in turn, but for a value that is one of the box's trap values. A
// near key that a trap value catches would give the box the verdict of an error, with its trap, and
// a near key of E4 is never one (docs/workbook.md).
func nearKeys(c sample.BoxCheck) []string {
	var out []string
	for _, i := range pairedDigits(c.Key) {
		rs := []rune(c.Key)
		rs[i], _ = sample.PairOf(rs[i])
		near := string(rs)
		if !slices.ContainsFunc(c.TrapValues, func(tv sample.TrapValue) bool { return tv.Value == near }) {
			out = append(out, near)
		}
	}
	return out
}

// between is a whole number from lo to hi, both included.
func between(rng *rand.Rand, lo, hi int) int { return lo + rng.IntN(hi-lo+1) }

// numberBox is a box of the kind number with its key and one trap value.
func numberBox(key, trap int, name string) sample.WorkbookBox {
	return sample.WorkbookBox{Check: sample.BoxCheck{
		Kind:       sample.KindNumber,
		Key:        strconv.Itoa(key),
		TrapValues: []sample.TrapValue{{Value: strconv.Itoa(trap), Trap: name}},
	}}
}

// fractionBox is a box of the kind fraction, in a form, with its key and trap values.
func fractionBox(key string, form value.Form, traps ...sample.TrapValue) sample.WorkbookBox {
	return sample.WorkbookBox{Check: sample.BoxCheck{Kind: sample.KindFraction, Key: key, Form: &form, TrapValues: traps}}
}

// frac writes a fraction as the pages do, "3/4".
func frac(n, d int) string { return strconv.Itoa(n) + "/" + strconv.Itoa(d) }

// lowest writes a fraction in lowest terms.
func lowest(n, d int) string {
	g := gcd(n, d)
	return frac(n/g, d/g)
}

// gcd is the greatest common divisor of two whole numbers above zero.
func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// item is an item of its text and boxes.
func item(text string, boxes ...sample.WorkbookBox) sample.WorkbookItem {
	return sample.WorkbookItem{Text: text, Boxes: boxes}
}

// multiplication: a missing factor, "6 × [ ] = 30"; the trap subtracts the factor from the product
// instead of dividing. 2 × 2 is left out, where the trap is the key.
func multiplication(rng *rand.Rand) sample.WorkbookItem {
	a, k := between(rng, 2, 9), between(rng, 2, 9)
	for a == 2 && k == 2 {
		a, k = between(rng, 2, 9), between(rng, 2, 9)
	}
	p := a * k
	return item(fmt.Sprintf("%d × [ ] = %d", a, p), numberBox(k, p-a, "wrong-operation"))
}

// division: "30 : 5 = [ ]"; the trap is the next number of the table.
func division(rng *rand.Rand) sample.WorkbookItem {
	d, q := between(rng, 2, 9), between(rng, 2, 9)
	return item(fmt.Sprintf("%d : %d = [ ]", q*d, d), numberBox(q, q+1, "table-neighbour"))
}

// addend: a missing addend, "[ ] + 22 = 52"; the trap adds instead of subtracting.
func addend(rng *rand.Rand) sample.WorkbookItem {
	x, b := between(rng, 11, 59), between(rng, 11, 39)
	return item(fmt.Sprintf("[ ] + %d = %d", b, x+b), numberBox(x, x+b+b, "adds-instead"))
}

// subtraction: three digits from three, with a borrow, and a difference of three digits,
// "639 − 472 = [ ]"; the trap takes the smaller digit from the larger in every place, without the
// borrow.
func subtraction(rng *rand.Rand) sample.WorkbookItem {
	for {
		a, b := between(rng, 300, 999), between(rng, 100, 799)
		if a-b < 100 {
			continue
		}
		trap := 0
		for place := 100; place >= 1; place /= 10 {
			x, y := a/place%10, b/place%10
			trap = trap*10 + max(x, y) - min(x, y)
		}
		if trap != a-b {
			return item(fmt.Sprintf("%d − %d = [ ]", a, b), numberBox(a-b, trap, "no-borrow"))
		}
	}
}

// remainder: a division with a remainder, two boxes, "17 : 5 = [ ] (ост. [ ])"; the traps are a
// quotient one too small, and the remainder that comes with it, larger than the divisor.
func remainder(rng *rand.Rand) sample.WorkbookItem {
	d, q := between(rng, 3, 9), between(rng, 2, 9)
	r := between(rng, 1, d-1)
	return item(fmt.Sprintf("%d : %d = [ ] (ост. [ ])", q*d+r, d),
		numberBox(q, q-1, "quotient-too-small"), numberBox(r, r+d, "remainder-too-big"))
}

// fractionSum: fractions of one denominator added, any form, "7/11 + 1/11 = [ ]"; the trap adds the
// denominators too.
func fractionSum(rng *rand.Rand) sample.WorkbookItem {
	d := between(rng, 5, 12)
	a := between(rng, 1, d-2)
	b := between(rng, 1, d-1-a)
	return item(fmt.Sprintf("%s + %s = [ ]", frac(a, d), frac(b, d)),
		fractionBox(frac(a+b, d), value.AnyForm, sample.TrapValue{Value: frac(a+b, 2*d), Trap: "adds-denominators"}))
}

// fractionDifference: fractions of one denominator subtracted, any form, "4/10 − 2/10 = [ ]"; the
// trap adds instead.
func fractionDifference(rng *rand.Rand) sample.WorkbookItem {
	d := between(rng, 5, 12)
	a := between(rng, 2, d-1)
	b := between(rng, 1, a-1)
	return item(fmt.Sprintf("%s − %s = [ ]", frac(a, d), frac(b, d)),
		fractionBox(frac(a-b, d), value.AnyForm, sample.TrapValue{Value: frac(a+b, d), Trap: "adds-instead"}))
}

// A share of a whole: centimetres of a metre, grams of a kilogram or minutes of an hour, the line's
// words, the parts of the whole, how many of them the item may name and the trap of a whole of the
// wrong size.
var shares = []struct {
	line   string
	whole  int
	amount []int
	wrong  int
	trap   string
}{
	{"%d см — это [ ] метра", 100, []int{4, 5, 10, 20, 25, 40, 50, 60, 75}, 10, "ten-centimetres"},
	{"%d г — это [ ] кг", 1000, []int{100, 125, 200, 250, 400, 500, 600, 750, 800}, 100, "hundred-grams"},
	{"%d минут — это [ ] часа", 60, []int{5, 6, 10, 12, 15, 20, 30, 40, 45}, 100, "hundred-minutes"},
}

// share: a share of an hour, a kilogram or a metre, any form, "25 см — это [ ] метра", its key in
// lowest terms; the trap takes a whole of the wrong size.
func share(rng *rand.Rand) sample.WorkbookItem {
	s := shares[rng.IntN(len(shares))]
	x := s.amount[rng.IntN(len(s.amount))]
	return item(fmt.Sprintf(s.line, x),
		fractionBox(lowest(x, s.whole), value.AnyForm, sample.TrapValue{Value: frac(x, s.wrong), Trap: s.trap}))
}

// reduce: a fraction to reduce, in lowest terms, "Сократи дробь: 8/14 = [ ]"; the traps are the
// fraction as it is, and the numerator reduced alone.
func reduce(rng *rand.Rand) sample.WorkbookItem {
	for {
		q := between(rng, 2, 9)
		p := between(rng, 1, q-1)
		if gcd(p, q) != 1 {
			continue
		}
		g := between(rng, 2, 4)
		n, m := p*g, q*g
		return item(fmt.Sprintf("Сократи дробь: %s = [ ]", frac(n, m)),
			fractionBox(frac(p, q), value.Reduced,
				sample.TrapValue{Value: frac(n, m), Trap: "not-in-lowest-terms"},
				sample.TrapValue{Value: frac(p, m), Trap: "reduces-numerator-only"}))
	}
}

// reducedSum: fractions of one denominator added, the sum in lowest terms, which the line asks for,
// "3/10 + 3/10 = [ ] (несократимой дробью)"; the traps are the sum not reduced, and the sum with the
// denominators added.
func reducedSum(rng *rand.Rand) sample.WorkbookItem {
	for {
		d := between(rng, 4, 12)
		a := between(rng, 1, d-2)
		b := between(rng, 1, d-1-a)
		if gcd(a+b, d) == 1 {
			continue
		}
		return item(fmt.Sprintf("%s + %s = [ ] (несократимой дробью)", frac(a, d), frac(b, d)),
			fractionBox(lowest(a+b, d), value.Reduced,
				sample.TrapValue{Value: frac(a+b, d), Trap: "not-in-lowest-terms"},
				sample.TrapValue{Value: frac(a+b, 2*d), Trap: "adds-denominators"}))
	}
}
