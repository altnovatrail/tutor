package sample

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/altnovatrail/module-kit/verdict"
	"github.com/altnovatrail/spec/types/formatv0"
)

// WorkbookFormat is the name a file of the workbook's pages declares in its "format" field.
const WorkbookFormat = "tutor-workbook"

// Workbook is the synthetic pages of the workbook (T78; docs/architecture.md, 3.4, «Рабочая
// тетрадь»): each a page of items, an item a line of text with boxes, as the module format writes
// an item of a workbook (spec's format, section 14), and for each box its check and what code writes
// in it by hand, and why. Code draws the pages in memory from a seed (synth.PageOf): no file of a page
// is written.
type Workbook struct {
	Format  string         `json:"format"`
	Version *int           `json:"version"`
	Pages   []WorkbookPage `json:"pages"`
}

// WorkbookPage is one page of the workbook.
type WorkbookPage struct {
	// Number is the page's place in the workbook, from 1.
	Number int `json:"number"`
	// Part is the part of the measurement the page belongs to: PartDev or PartTest (T80).
	Part  string         `json:"part"`
	Items []WorkbookItem `json:"items"`
}

// The parts of the measurement a page belongs to.
const (
	PartDev  = "dev"
	PartTest = "test"
)

// WorkbookItem is an item of a page: the text of its line, in Russian, with each box as "[ ]", and
// its boxes in the order they stand in the line, its answer slots.
type WorkbookItem struct {
	Text  string        `json:"text"`
	Boxes []WorkbookBox `json:"boxes"`
}

// MaxItems is the most items a page holds: an item to a row, and a page of the workbook has ten
// rows (R35 canvas).
const MaxItems = 10

// BoxMark is how a box stands in the text of an item's line.
const BoxMark = "[ ]"

// WorkbookBox is a box of an item: its check, the scenario of what is written in it, the value
// written, as a person reads it, the value written and crossed out before it, and how a fraction
// is written: stacked or inline.
type WorkbookBox struct {
	Check    BoxCheck    `json:"check"`
	Scenario BoxScenario `json:"scenario"`
	// Written is the value left in the box, as a person reads it: digits, and a fraction as
	// "3/4", however it is written. It is nil when the box is left blank.
	Written *string `json:"written"`
	// Crossed is the value crossed out before Written was written beside it, in a box of the
	// scenario BoxRewritten alone.
	Crossed *string `json:"crossed"`
	// Fraction is how a fraction in the box is written: a box of the kind fraction has one, and a
	// blank box or one of a number none.
	Fraction *FractionWriting `json:"fraction"`
}

// BoxCheck is the check of a box, as the module format gives it (spec's format, section 15): the
// kind, number, fraction or time; the key; the form of a fraction, nil for a number and a time; and
// the trap values.
type BoxCheck struct {
	Kind       CheckKind           `json:"kind"`
	Key        string              `json:"key"`
	Form       *formatv0.CheckForm `json:"form"`
	TrapValues []TrapValue         `json:"trap_values"`
}

// CheckKind is the kind of a box's check.
type CheckKind string

// The check kinds of the workbook (R41 spec), and from format 0.4 a time of day (R111 spec).
const (
	KindNumber   CheckKind = "number"
	KindFraction CheckKind = "fraction"
	KindTime     CheckKind = "time"
)

// TrapValue is a wrong value the check knows, and the trap it shows.
type TrapValue struct {
	Value string `json:"value"`
	Trap  string `json:"trap"`
}

// BoxScenario is what a box is written to test.
type BoxScenario string

// The scenarios of a box (docs/architecture.md, 3.4, «Рабочая тетрадь»).
const (
	// BoxCorrect: the key's value, written plainly.
	BoxCorrect BoxScenario = "correct"
	// BoxError: a typical mistake, a trap value of the check.
	BoxError BoxScenario = "error"
	// BoxNearKey: a value one digit off the key, the digit of its pair, drawn leaning toward the
	// key's digit.
	BoxNearKey BoxScenario = "near-key"
	// BoxSimilarDigits: the key's value, with its last digit of a pair drawn leaning toward the
	// digit of its pair.
	BoxSimilarDigits BoxScenario = "similar-digits"
	// BoxMirrored: the key's value, with its first digit of 2, 5, 7 and 9 drawn mirrored.
	BoxMirrored BoxScenario = "mirrored"
	// BoxRewritten: a wrong value crossed out, and the key's value written beside it.
	BoxRewritten BoxScenario = "rewritten"
	// BoxBlank: nothing written.
	BoxBlank BoxScenario = "blank"
)

// BoxScenarios lists the scenarios of a box, in the order of the documents.
var BoxScenarios = []BoxScenario{BoxCorrect, BoxError, BoxNearKey, BoxSimilarDigits, BoxMirrored, BoxRewritten, BoxBlank}

// FractionWriting is how a fraction is written in a box.
type FractionWriting string

// The writings of a fraction.
const (
	// Stacked: the numerator over a bar, the denominator under it.
	Stacked FractionWriting = "stacked"
	// Inline: the numerator, a slash and the denominator in a row, as "3/4".
	Inline FractionWriting = "inline"
)

// similarDigits are the pairs of digits a hand writes alike, each the other's (T70; R36).
var similarDigits = map[rune]rune{'1': '7', '7': '1', '4': '9', '9': '4', '3': '8', '8': '3', '5': '6', '6': '5'}

// PairOf is the digit of a digit's pair, and false for a digit of no pair or no digit at all.
func PairOf(r rune) (rune, bool) {
	s, ok := similarDigits[r]
	return s, ok
}

// Mirrorable are the digits a child writes mirrored (docs/measurements.md, 1.4).
const Mirrorable = "2579"

// writtenValue is how a value of a number or a fraction is written in a box: digits, or a simple
// fraction of them; and writtenTime how a time is, hours, ":" and minutes, in the notation of a time
// of the format (spec's format, section 15; R111 spec), so that a trap value can hold 5:60.
var (
	writtenValue = regexp.MustCompile(`^(0|[1-9][0-9]*)(/[1-9][0-9]*)?$`)
	writtenTime  = regexp.MustCompile(`^[0-9]{1,2}:([0-9]{2}|[1-9][0-9]{2})$`)
)

// written is whether a value is written as the pages write a value of a check's kind.
func written(kind CheckKind, value string) bool {
	if kind == KindTime {
		return writtenTime.MatchString(value)
	}
	return writtenValue.MatchString(value)
}

// Format is the check of a box as the module format writes it: the workbook's check is the
// format's already, and a number has no form.
func (c BoxCheck) Format() formatv0.Check {
	key := c.Key
	out := formatv0.Check{Kind: formatv0.CheckKind(c.Kind), Key: &key}
	if c.Form != nil {
		form := *c.Form
		out.Form = &form
	}
	for _, tv := range c.TrapValues {
		out.TrapValues = append(out.TrapValues, formatv0.TrapValue{Value: tv.Value, Trap: formatv0.TrapID(tv.Trap)})
	}
	return out
}

// Slot is a box with its check as module-kit judges it: an answer slot bound box.
func (c BoxCheck) Slot() verdict.AnswerSlot {
	return verdict.AnswerSlot{Binding: string(formatv0.LinePartBindingBox), Check: c.Format()}
}

// Verdict is module-kit's verdict on a value read in a box of this check, nil when nothing was read
// (verdict.OfSlot): correct, not yet right with its trap, or unreadable, blank or out of the
// notation. An error is a check that the format does not take, such as a key out of the notation.
func (c BoxCheck) Verdict(value *string) (verdict.SlotVerdict, error) {
	return verdict.OfSlot(context.Background(), c.Slot(), value)
}

// Verdict is the verdict of a box read as it is written: unreadable for a blank box, and otherwise
// what its check gives the value written. It is what a right reading of the box comes to. A check
// that the format does not take gives unchecked, and the check of the file names it.
func (b WorkbookBox) Verdict() verdict.SlotVerdict {
	v, err := b.Check.Verdict(b.Written)
	if err != nil {
		return verdict.SlotVerdict{Verdict: verdict.Unchecked}
	}
	return v
}

// Mirrored is the place, among the characters of the value written, of the digit a box of the
// scenario BoxMirrored draws mirrored: the first of 2, 5, 7 and 9; false when there is none.
func Mirrored(written string) (int, bool) {
	for at, r := range []rune(written) {
		if strings.ContainsRune(Mirrorable, r) {
			return at, true
		}
	}
	return 0, false
}

// Leaning is the digit a box draws leaning toward another, by its place among the characters of the
// value, and the digit it leans toward: in a box of the scenario BoxNearKey the one digit that
// differs from the key, toward the key's digit; in one of BoxSimilarDigits the last digit of a pair,
// toward the digit of its pair — unlike the answer line of a sheet, whose digit leans only when the
// line's last digit has a pair (synth.SimilarDigit), since a box holds a value alone. It is false
// for any other box, and for one whose value has no such digit.
func (b WorkbookBox) Leaning() (at int, toward rune, ok bool) {
	if b.Written == nil {
		return 0, 0, false
	}
	w := []rune(*b.Written)
	switch b.Scenario {
	case BoxNearKey:
		k := []rune(b.Check.Key)
		if len(k) != len(w) {
			return 0, 0, false
		}
		at = -1
		for i := range w {
			if w[i] != k[i] {
				if at >= 0 {
					return 0, 0, false
				}
				at = i
			}
		}
		if at < 0 || similarDigits[w[at]] != k[at] {
			return 0, 0, false
		}
		return at, k[at], true
	case BoxSimilarDigits:
		for i := len(w) - 1; i >= 0; i-- {
			if s, ok := similarDigits[w[i]]; ok {
				return i, s, true
			}
		}
	}
	return 0, 0, false
}

// ReadWorkbook reads the pages of the workbook from JSON and checks them. An error from a file
// that fails the check is a *FormatError.
func ReadWorkbook(r io.Reader) (*Workbook, error) {
	var wb Workbook
	if err := readStrict(r, "workbook", &wb); err != nil {
		return nil, readError("workbook", err)
	}
	if err := wb.Check(); err != nil {
		return nil, err
	}
	return &wb, nil
}

// Check checks the pages of the workbook and returns a *FormatError listing every problem, or nil:
// the pages numbered from 1, each of a part; every item with a text whose boxes are as many as its
// boxes; every check one the module format would take; and every box written as its scenario says.
func (wb *Workbook) Check() error {
	p := &problems{what: "workbook"}
	checkHeader(p, wb.Format, WorkbookFormat, wb.Version)
	if len(wb.Pages) == 0 {
		p.add("pages", "are missing: a workbook has at least one page")
	}
	for i, pg := range wb.Pages {
		path := fmt.Sprintf("pages[%d]", i)
		if pg.Number != i+1 {
			p.add(path+".number", "is %d, but the page is number %d in the workbook", pg.Number, i+1)
		}
		if pg.Part != PartDev && pg.Part != PartTest {
			p.add(path+".part", "is %q, not %q or %q", pg.Part, PartDev, PartTest)
		}
		if len(pg.Items) == 0 || len(pg.Items) > MaxItems {
			p.add(path+".items", "are %d, and a page has 1 to %d items", len(pg.Items), MaxItems)
		}
		for k, it := range pg.Items {
			checkItem(p, fmt.Sprintf("%s.items[%d]", path, k), it)
		}
	}
	return p.err()
}

func checkItem(p *problems, path string, it WorkbookItem) {
	if strings.TrimSpace(it.Text) == "" {
		p.add(path+".text", "is empty")
	}
	if n := strings.Count(it.Text, BoxMark); n != len(it.Boxes) {
		p.add(path, "has %d boxes, but its text shows %d as %q", len(it.Boxes), n, BoxMark)
	}
	if len(it.Boxes) == 0 {
		p.add(path+".boxes", "are missing: an item has at least one box")
	}
	for k, b := range it.Boxes {
		bp := fmt.Sprintf("%s.boxes[%d]", path, k)
		if checkCheck(p, bp+".check", b.Check) {
			checkBox(p, bp, b)
		}
	}
}

// checkCheck checks a box's check as the module format checks a task's: the kind and its form, a
// key in the notation of the kind and, asked in lowest terms, in them (task.fraction-key), and trap
// values all not yet right and none equal to another as the check compares them
// (task.trap-values). It reports whether the check can judge the box's values.
func checkCheck(p *problems, path string, c BoxCheck) bool {
	switch {
	case (c.Kind == KindNumber || c.Kind == KindTime) && c.Form != nil:
		p.add(path+".form", "is %q, but a %s has no form", *c.Form, c.Kind)
	case c.Kind == KindFraction && (c.Form == nil || *c.Form != formatv0.CheckFormAny && *c.Form != formatv0.CheckFormReduced):
		p.add(path+".form", "is not %q or %q", formatv0.CheckFormAny, formatv0.CheckFormReduced)
	case c.Kind != KindNumber && c.Kind != KindFraction && c.Kind != KindTime:
		p.add(path+".kind", "is %q, not %q, %q or %q", c.Kind, KindNumber, KindFraction, KindTime)
		return false
	}
	v, err := c.Verdict(&c.Key)
	if err != nil && written(c.Kind, c.Key) {
		p.add(path, "is no check the format takes: %v", err)
		return false
	}
	if err != nil || v.Verdict != verdict.Correct || !written(c.Kind, c.Key) {
		p.add(path+".key", "%q is no key of a %s, as the format and the pages write it", c.Key, c.Kind)
		return false
	}
	seen := map[string]bool{}
	for i, t := range c.TrapValues {
		tp := fmt.Sprintf("%s.trap_values[%d]", path, i)
		if !trapName.MatchString(t.Trap) {
			p.add(tp+".trap", "%q is no trap's ID: lowercase words joined by hyphens", t.Trap)
		}
		if seen[t.Trap] {
			p.add(tp+".trap", "%q names a trap another trap value names", t.Trap)
		}
		seen[t.Trap] = true
		v, _ := c.Verdict(&t.Value) // the key is one: the check is taken
		switch {
		case !written(c.Kind, t.Value):
			p.add(tp+".value", "%q is not written as the pages write values", t.Value)
		case v.Verdict != verdict.NotYet:
			p.add(tp+".value", "%q is %s, and a trap value is not yet right", t.Value, v.Verdict)
		case v.Trap == nil || string(v.Trap.Trap) != t.Trap:
			p.add(tp+".value", "%q is equal to the trap value of %q as the check compares them", t.Value, trapOf(v))
		}
	}
	return true
}

// trapOf is the trap a verdict names, or none.
func trapOf(v verdict.SlotVerdict) string {
	if v.Trap == nil {
		return ""
	}
	return string(v.Trap.Trap)
}

// trapName is how a trap is named: lowercase words joined by hyphens.
var trapName = regexp.MustCompile(`^[a-z]+(-[a-z]+)*$`)

// checkBox checks that a box is written as its scenario says.
func checkBox(p *problems, path string, b WorkbookBox) {
	fraction := b.Check.Kind == KindFraction
	if b.Scenario == BoxBlank {
		if b.Written != nil || b.Crossed != nil || b.Fraction != nil {
			p.add(path, "is %s, and holds no value written, crossed out or how a fraction is written", BoxBlank)
		}
		return
	}
	if b.Written == nil {
		p.add(path+".written", "is null, but only a box of the scenario %s is blank", BoxBlank)
		return
	}
	if !written(b.Check.Kind, *b.Written) {
		p.add(path+".written", "%q is not written as the pages write values: digits, digits/digits, or hours:minutes for a time", *b.Written)
		return
	}
	switch {
	case fraction && b.Fraction == nil:
		p.add(path+".fraction", "is null, but a fraction is written stacked or inline")
	case fraction && *b.Fraction != Stacked && *b.Fraction != Inline:
		p.add(path+".fraction", "is %q, not %q or %q", *b.Fraction, Stacked, Inline)
	case !fraction && b.Fraction != nil:
		p.add(path+".fraction", "is %q, but a %s is no fraction", *b.Fraction, b.Check.Kind)
	}
	if (b.Crossed != nil) != (b.Scenario == BoxRewritten) {
		p.add(path+".crossed", "is given in a box of the scenario %s alone, and there it is", BoxRewritten)
	}
	v := b.Verdict()
	switch b.Scenario {
	case BoxCorrect, BoxSimilarDigits, BoxMirrored, BoxRewritten:
		if v.Verdict != verdict.Correct {
			p.add(path+".written", "%q is %s, but a box of the scenario %s holds the key's value", *b.Written, v.Verdict, b.Scenario)
		}
	case BoxError:
		if v.Verdict != verdict.NotYet || v.Trap == nil {
			p.add(path+".written", "%q is no trap value, but a box of the scenario %s holds one", *b.Written, b.Scenario)
		}
	case BoxNearKey:
		if v.Verdict != verdict.NotYet {
			p.add(path+".written", "%q is %s, but a box of the scenario %s is not yet right", *b.Written, v.Verdict, b.Scenario)
		}
	default:
		p.add(path+".scenario", "is %q, not one of %v", b.Scenario, BoxScenarios)
		return
	}
	if _, _, ok := b.Leaning(); !ok && (b.Scenario == BoxNearKey || b.Scenario == BoxSimilarDigits) {
		p.add(path+".written", "%q has no digit for the scenario %s: one digit off the key, its pair's, for %s, and a digit of a pair for %s", *b.Written, b.Scenario, BoxNearKey, BoxSimilarDigits)
	}
	if _, ok := Mirrored(*b.Written); !ok && b.Scenario == BoxMirrored {
		p.add(path+".written", "%q has none of the digits %s to mirror", *b.Written, Mirrorable)
	}
	if b.Crossed != nil {
		if !written(b.Check.Kind, *b.Crossed) {
			p.add(path+".crossed", "%q is not written as the pages write values", *b.Crossed)
		} else if c, _ := b.Check.Verdict(b.Crossed); c.Verdict != verdict.NotYet {
			p.add(path+".crossed", "%q is %s, but what is crossed out is a wrong value", *b.Crossed, c.Verdict)
		}
	}
}
