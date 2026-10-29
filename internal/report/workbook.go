package report

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/runner"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/value"
	"github.com/altnovatrail/tutor/internal/workbook"
)

// Workbook is the report of a run of the reading of the workbook (T80; docs/architecture.md, 3.4,
// «Рабочая тетрадь», «Критерий тетради»): how each reader read the filled boxes at 32 px, bare and
// captioned, by scenario, by kind of check and by how a fraction is written; the items; whether the
// line of an item helps or harms the reading; and the other heights of digits against 32 px.
type Workbook struct {
	log     *workbook.Log
	readers []runner.ReaderRecord
	pages   []*workbook.PageRecord
	// byPage are the pages by their number, each with its place among them.
	byPage map[int]pageAt
	// Cells are the readings of the boxes at 32 px by reader, condition and group.
	Cells []WorkbookCell
	// Items are the readings of the items at 32 px by reader and condition.
	Items []WorkbookItems
	// Captions are the line of the item against none, by reader.
	Captions []Caption
	// Heights are the other heights against 32 px, bare, by reader.
	Heights []Height
	// Mirrored are the readings of the boxes with a digit written mirrored at 32 px by reader and
	// condition.
	Mirrored []WorkbookMirrored
	// version is the version of the log's prompt, by which a reading is judged (workbook.Want).
	version int
}

// want is the verdict a right reading of a box comes to by the log's prompt (workbook.Want).
func (r *Workbook) want(b *workbook.BoxRecord) value.Slot {
	return workbook.Want(b.Scenario, value.Slot{Verdict: b.Verdict, Reason: b.Reason, Trap: b.Trap}, r.version)
}

// The groups of the boxes a cell of the report counts: all of them, each scenario, each kind of
// check and each writing of a fraction.
const (
	AllBoxes = "all"
	// Number, Fraction and Reduced are the kinds of check: a number, a fraction of any form and one
	// in lowest terms.
	Number   = "number"
	Fraction = "fraction, any form"
	Reduced  = "fraction in lowest terms"
)

// workbookGroups are the groups of a box: all, its scenario, its kind of check and, for a fraction,
// how it is written.
func workbookGroups(b *workbook.BoxRecord) []string {
	groups := []string{AllBoxes, "scenario " + string(b.Scenario)}
	switch {
	case b.Kind == sample.KindNumber:
		groups = append(groups, Number)
	case b.Form == value.Reduced:
		groups = append(groups, Reduced)
	default:
		groups = append(groups, Fraction)
	}
	if b.Fraction != nil {
		groups = append(groups, "written "+string(*b.Fraction))
	}
	return groups
}

// WorkbookMirrored is the readings of the boxes with a digit written mirrored by a reader in a
// condition at 32 px, in every repeat. By the rule of spec v0.3.3 such a digit is not read as a
// digit: the box is ambiguous, unreadable, and the learner writes it again, while a false alarm on it
// counts against the target of 1 % (R56 spec, amendment of 2026-10-29; product 10 spec). Each share
// is over the pages:
//   - Rewrite is "unreadable" on such a box, among its readings, and Marked those of them the reader
//     marked mirrored;
//   - AsDigit is "correct": the digit read as the digit, against the rule and with no harm;
//   - FalseAlarm is "not yet" on such a box, whose value is right;
//   - Elsewhere is the other filled boxes the reader marked mirrored, among their readings.
type WorkbookMirrored struct {
	Reader     string
	Condition  workbook.Condition
	Readings   int
	Rewrite    Share
	Marked     int
	AsDigit    int
	FalseAlarm Share
	Unchecked  int
	Elsewhere  Share
}

// pageAt is a page of the log and its place among the pages.
type pageAt struct {
	*workbook.PageRecord
	at int
}

// workbookGroupOrder is the order the groups are told in.
func workbookGroupOrder() []string {
	out := []string{AllBoxes}
	for _, s := range sample.BoxScenarios {
		out = append(out, "scenario "+string(s))
	}
	return append(out, Number, Fraction, Reduced, "written "+string(sample.Stacked), "written "+string(sample.Inline))
}

// WorkbookCell is the readings of a group of boxes by a reader in a condition at 32 px, in every
// repeat: how many readings, how many read the value as written, and the errors by verdict, each a
// share with its bound over the pages.
//   - a false "correct" is "correct" on a box whose value is not yet right, counted among those;
//   - a false alarm is "not yet" on a box whose value is right, counted among those;
//   - an extra "unreadable" is "unreadable" on a box with a value, right or not, counted among all
//     the filled boxes, and so is "unchecked": a failed call, a missing or a doubled cell.
//
// A blank box goes to no model, and is counted as read and right.
type WorkbookCell struct {
	Reader    string
	Condition workbook.Condition
	Group     string
	Readings  int
	Read      int
	// Right is the readings whose verdict is that of the box, with its trap.
	Right        int
	FalseCorrect Share
	FalseAlarm   Share
	Extra        Share
	Unchecked    int
}

// WorkbookItems is the readings of the items by a reader in a condition at 32 px: how many, how many
// came to the verdict of the item, and the false "correct" and false alarms of items.
type WorkbookItems struct {
	Reader       string
	Condition    workbook.Condition
	Readings     int
	Right        int
	FalseCorrect Share
	FalseAlarm   Share
}

// Caption is the line of the item against none for a reader, at 32 px, paired by pages over the
// repeats read in both conditions: d of a page is its errors captioned less those bare, the errors being false "correct",
// false alarms, extra "unreadable" and "unchecked". The rule written before the run (T80): a false
// "correct" captioned on a box that has none bare means the line does not go into the reading of a
// box. The line helps when the test finds the reading captioned better at p < Alpha, with no such
// box.
type Caption struct {
	Reader string
	Pages  int
	// Bare and Captioned are the errors in each condition, over the repeats read in both.
	Bare, Captioned int
	// Test is the test over the pages: its Helps is the p of the reading captioned being worse, and
	// its Harms of it being better.
	Test Test
	// OnlyCaptioned are the boxes with a false "correct" captioned and none bare.
	OnlyCaptioned int
}

// Helps is whether the line of the item helps the reading by the rule.
func (c Caption) Helps() bool { return c.OnlyCaptioned == 0 && c.Test.Harms < Alpha }

// verdict is what the rule says of the line.
func (c Caption) verdict() string {
	switch {
	case c.OnlyCaptioned > 0:
		return "the line does not go into the reading: a false \"correct\" with it alone"
	case c.Helps():
		return "the line helps"
	}
	return "not shown that the line helps"
}

// boxErrors are what the verdict of a reading of a box is against the box's own: a false "correct",
// a false alarm, an extra "unreadable", "unchecked", a request to write it again missed, and
// whether it is right.
type boxErrors struct {
	falseCorrect, falseAlarm, extra, unchecked, missed, right bool
}

func (e boxErrors) count() int {
	n := 0
	for _, b := range []bool{e.falseCorrect, e.falseAlarm, e.extra, e.unchecked, e.missed} {
		if b {
			n++
		}
	}
	return n
}

// boxErrorsOf are the errors of a reading of a box against the box as code wrote it: a false
// "correct" and a false alarm by the verdict of its value, and whether it is right and an extra
// "unreadable" by the verdict a right reading comes to (want; workbook.Want). Where that is
// unreadable on a filled box — a digit written mirrored, from prompt version 2 — "unreadable" on it
// is right and no extra one, "correct" misses asking the learner to write it again, and "not yet"
// is a false alarm still.
func boxErrorsOf(truth *workbook.BoxRecord, want value.Slot, got workbook.Box) boxErrors {
	var e boxErrors
	switch {
	case truth.Written == nil:
	case got.Verdict == value.Unchecked:
		e.unchecked = true
	case got.Verdict == value.Unreadable:
		e.extra = want.Verdict != value.Unreadable
	case truth.Verdict == value.NotYet && got.Verdict == value.Correct:
		e.falseCorrect = true
	case truth.Verdict == value.Correct && got.Verdict == value.NotYet:
		e.falseAlarm = true
	case want.Verdict == value.Unreadable && got.Verdict == value.Correct:
		e.missed = true
	}
	e.right = got.Verdict == want.Verdict && got.Trap == want.Trap
	return e
}

// readAsWritten is whether a box was read as it is written: the text read is the value written,
// compared without spaces and after normalization; a blank box is read as written by no call.
func readAsWritten(truth *workbook.BoxRecord, got workbook.Box) bool {
	if truth.Written == nil {
		return true
	}
	if got.Text == nil {
		return false
	}
	squash := func(s string) string { return strings.ReplaceAll(value.Normalize(s), " ", "") }
	return squash(*got.Text) == squash(*truth.Written)
}

// NewWorkbook builds the report of a log of the reading of the workbook.
func NewWorkbook(l *workbook.Log) (*Workbook, error) {
	if len(l.Run.Readers) == 0 || len(l.Sheets) == 0 {
		return nil, fmt.Errorf("the log has no readers or no pages")
	}
	r := &Workbook{log: l, readers: l.Run.Readers, byPage: map[int]pageAt{}, version: l.Run.PromptVersion}
	for i := range l.Sheets {
		pg := &l.Sheets[i]
		if _, ok := r.byPage[pg.Page]; ok {
			return nil, fmt.Errorf("page %d is in the log twice", pg.Page)
		}
		r.byPage[pg.Page] = pageAt{pg, len(r.pages)}
		r.pages = append(r.pages, pg)
	}
	for i := range l.Calls {
		c := &l.Calls[i]
		pg, ok := r.byPage[c.Page]
		if !ok {
			return nil, fmt.Errorf("a call reads page %d, which the log does not tell", c.Page)
		}
		if len(c.Boxes) != len(pg.Boxes) || len(c.Items) != len(itemTruth(pg.PageRecord, nil)) {
			return nil, fmt.Errorf("a call of page %d tells %d boxes and %d items, and the page has %d and %d", c.Page, len(c.Boxes), len(c.Items), len(pg.Boxes), len(itemTruth(pg.PageRecord, nil)))
		}
		for k, b := range c.Boxes {
			if b.Item != pg.Boxes[k].Item || b.Slot != pg.Boxes[k].Slot {
				return nil, fmt.Errorf("a call of page %d tells box %d.%d where the page has %d.%d", c.Page, b.Item, b.Slot, pg.Boxes[k].Item, pg.Boxes[k].Slot)
			}
		}
	}
	for _, rd := range r.readers {
		for _, cond := range []workbook.Condition{workbook.Bare, workbook.Captioned} {
			calls := r.calls(rd.Name, cond, render.Digit)
			if len(calls) == 0 {
				continue
			}
			cells, mirrored := r.cells(rd.Name, cond, calls)
			r.Cells = append(r.Cells, cells...)
			r.Items = append(r.Items, r.items(rd.Name, cond, calls))
			if mirrored != nil {
				r.Mirrored = append(r.Mirrored, *mirrored)
			}
		}
		if c, ok := r.caption(rd.Name); ok {
			r.Captions = append(r.Captions, c)
		}
		for _, digit := range render.Heights() {
			if digit == render.Digit {
				continue
			}
			if h, ok := r.height(rd.Name, digit); ok {
				r.Heights = append(r.Heights, h)
			}
		}
	}
	return r, nil
}

// calls are a reader's calls in a condition at a height, in the order they were made.
func (r *Workbook) calls(reader string, cond workbook.Condition, digit int) []*workbook.CallRecord {
	var out []*workbook.CallRecord
	for i := range r.log.Calls {
		c := &r.log.Calls[i]
		if c.Reader == reader && c.Condition == cond && c.Digit == digit && !c.NotAsked {
			out = append(out, c)
		}
	}
	return out
}

// counts are a cell as it is counted: its numbers and, by page, its events and their totals.
type counts struct {
	WorkbookCell
	fc, fcOf, fa, faOf, ex, exOf []int
}

// cells counts the readings of the boxes in the calls, in one pass, into every group each box is
// of, a share each over the pages, in the order of the groups, and into the mirrored digits; a group
// no box of which was read has no cell, and with no box of a digit written mirrored read, there are
// no mirrored digits.
func (r *Workbook) cells(reader string, cond workbook.Condition, calls []*workbook.CallRecord) ([]WorkbookCell, *WorkbookMirrored) {
	n := len(r.pages)
	by := map[string]*counts{}
	m := mirroredCounts{WorkbookMirrored: WorkbookMirrored{Reader: reader, Condition: cond},
		rw: make([]int, n), fa: make([]int, n), of: make([]int, n), el: make([]int, n), elOf: make([]int, n)}
	for _, call := range calls {
		pg := r.byPage[call.Page]
		for k, got := range call.Boxes {
			truth := &pg.Boxes[k]
			want := r.want(truth)
			m.add(truth, got, pg.at)
			for _, g := range workbookGroups(truth) {
				c := by[g]
				if c == nil {
					c = &counts{WorkbookCell: WorkbookCell{Reader: reader, Condition: cond, Group: g},
						fc: make([]int, n), fcOf: make([]int, n), fa: make([]int, n), faOf: make([]int, n), ex: make([]int, n), exOf: make([]int, n)}
					by[g] = c
				}
				c.add(truth, want, got, pg.at)
			}
		}
	}
	var out []WorkbookCell
	for _, g := range workbookGroupOrder() {
		if c := by[g]; c != nil {
			c.FalseCorrect, c.FalseAlarm, c.Extra = share(c.fc, c.fcOf), share(c.fa, c.faOf), share(c.ex, c.exOf)
			out = append(out, c.WorkbookCell)
		}
	}
	if m.Readings == 0 {
		return out, nil
	}
	m.Rewrite, m.FalseAlarm, m.Elsewhere = share(m.rw, m.of), share(m.fa, m.of), share(m.el, m.elOf)
	return out, &m.WorkbookMirrored
}

// add counts a reading of a box of the page at its place.
func (c *counts) add(truth *workbook.BoxRecord, want value.Slot, got workbook.Box, at int) {
	c.Readings++
	e := boxErrorsOf(truth, want, got)
	if readAsWritten(truth, got) {
		c.Read++
	}
	if e.right {
		c.Right++
	}
	if e.unchecked {
		c.Unchecked++
	}
	if truth.Written == nil {
		return
	}
	c.exOf[at]++
	if e.extra || e.unchecked {
		c.ex[at]++
	}
	switch truth.Verdict {
	case value.NotYet:
		c.fcOf[at]++
		if e.falseCorrect {
			c.fc[at]++
		}
	case value.Correct:
		c.faOf[at]++
		if e.falseAlarm {
			c.fa[at]++
		}
	}
}

// mirroredCounts are the mirrored digits as they are counted: their numbers and, by page, the
// events of each share and what they are counted among.
type mirroredCounts struct {
	WorkbookMirrored
	rw, fa, of, el, elOf []int
}

// add counts a reading of a filled box of the page at its place: of a box with a digit written
// mirrored, by its verdict; of another, whether it is marked mirrored.
func (m *mirroredCounts) add(truth *workbook.BoxRecord, got workbook.Box, at int) {
	switch {
	case truth.Written == nil:
	case truth.Scenario != sample.BoxMirrored:
		m.elOf[at]++
		if got.Reason == workbook.Mirrored {
			m.el[at]++
		}
	default:
		m.Readings++
		m.of[at]++
		switch got.Verdict {
		case value.Unreadable:
			m.rw[at]++
			if got.Reason == workbook.Mirrored {
				m.Marked++
			}
		case value.Correct:
			m.AsDigit++
		case value.NotYet:
			m.fa[at]++
		case value.Unchecked:
			m.Unchecked++
		}
	}
}

// items counts the readings of the items in the calls: right when the verdict of the item is the one
// its boxes come to as a right reading reads them (workbook.Want), and a false "correct" or a false
// alarm by the verdicts of the values written in them.
func (r *Workbook) items(reader string, cond workbook.Condition, calls []*workbook.CallRecord) WorkbookItems {
	it := WorkbookItems{Reader: reader, Condition: cond}
	n := len(r.pages)
	fc, fcOf, fa, faOf := make([]int, n), make([]int, n), make([]int, n), make([]int, n)
	for _, call := range calls {
		pg := r.byPage[call.Page]
		at, truth, want := pg.at, itemTruth(pg.PageRecord, nil), itemTruth(pg.PageRecord, r.want)
		for i, got := range call.Items {
			it.Readings++
			if got == want[i] {
				it.Right++
			}
			switch truth[i] {
			case value.NotYet:
				fcOf[at]++
				if got == value.Correct {
					fc[at]++
				}
			case value.Correct:
				faOf[at]++
				if got == value.NotYet {
					fa[at]++
				}
			}
		}
	}
	it.FalseCorrect, it.FalseAlarm = share(fc, fcOf), share(fa, faOf)
	return it
}

// itemTruth is the verdict of each item of a page made of its boxes': of the values written in them,
// or, with want, of what a right reading of each comes to.
func itemTruth(pg *workbook.PageRecord, want func(*workbook.BoxRecord) value.Slot) []value.Verdict {
	var out []value.Verdict
	for k := 0; k < len(pg.Boxes); {
		item := pg.Boxes[k].Item
		var of []value.Verdict
		for ; k < len(pg.Boxes) && pg.Boxes[k].Item == item; k++ {
			v := pg.Boxes[k].Verdict
			if want != nil {
				v = want(&pg.Boxes[k]).Verdict
			}
			of = append(of, v)
		}
		out = append(out, value.Item(of))
	}
	return out
}

// pageErrors are the errors of a call's boxes, and the boxes with a false "correct" in it.
func (r *Workbook) pageErrors(call *workbook.CallRecord) (int, map[[2]int]bool) {
	pg := r.byPage[call.Page]
	n, fc := 0, map[[2]int]bool{}
	for k, got := range call.Boxes {
		e := boxErrorsOf(&pg.Boxes[k], r.want(&pg.Boxes[k]), got)
		n += e.count()
		if e.falseCorrect {
			fc[[2]int{got.Item, got.Slot}] = true
		}
	}
	return n, fc
}

// pageRepeat is a page and a repeat.
type pageRepeat struct{ page, repeat int }

// byRepeat are a reader's calls in a condition at 32 px by page and repeat.
func (r *Workbook) byRepeat(reader string, cond workbook.Condition) map[pageRepeat]*workbook.CallRecord {
	out := map[pageRepeat]*workbook.CallRecord{}
	for _, c := range r.calls(reader, cond, render.Digit) {
		out[pageRepeat{c.Page, c.Repeat}] = c
	}
	return out
}

// caption pairs a reader's readings captioned and bare at 32 px by pages, over the repeats read in
// both conditions, so that the errors of each condition are of as many calls; false when the reader
// read in one condition alone.
func (r *Workbook) caption(reader string) (Caption, bool) {
	c := Caption{Reader: reader}
	bare, captioned := r.byRepeat(reader, workbook.Bare), r.byRepeat(reader, workbook.Captioned)
	if len(bare) == 0 || len(captioned) == 0 {
		return c, false
	}
	var nums, dens []int64
	for _, pg := range r.pages {
		eb, ec, paired := 0, 0, false
		fcBare, fcCaptioned := map[[2]int]bool{}, map[[2]int]bool{}
		for key, b := range bare {
			k := captioned[key]
			if key.page != pg.Page || k == nil {
				continue
			}
			paired = true
			for _, side := range []struct {
				call   *workbook.CallRecord
				errors *int
				fc     map[[2]int]bool
			}{{b, &eb, fcBare}, {k, &ec, fcCaptioned}} {
				e, fc := r.pageErrors(side.call)
				*side.errors += e
				for box := range fc {
					side.fc[box] = true
				}
			}
		}
		if !paired {
			continue
		}
		c.Pages++
		c.Bare, c.Captioned = c.Bare+eb, c.Captioned+ec
		for k := range fcCaptioned {
			if !fcBare[k] {
				c.OnlyCaptioned++
			}
		}
		nums, dens = append(nums, int64(ec-eb)), append(dens, 1)
	}
	c.Test = SignTest(nums, dens)
	return c, c.Pages > 0
}

// height pairs a reader's readings bare at a height with those at 32 px, in the first repeat, by
// pages, as the reading's heights are (T72): d of a page is its errors at the height less those at
// 32 px, and the height is not worse if the test does not find it worse at p < Alpha and no box has
// a false "correct" at it where it had none at 32 px. False when the reader did not read the height.
func (r *Workbook) height(reader string, digit int) (Height, bool) {
	h := Height{Reader: reader, Digit: digit, Group: AllBoxes}
	first := func(digit int) map[int]*workbook.CallRecord {
		out := map[int]*workbook.CallRecord{}
		for _, c := range r.calls(reader, workbook.Bare, digit) {
			if c.Repeat == 1 {
				out[c.Page] = c
			}
		}
		return out
	}
	at, base := first(digit), first(render.Digit)
	if len(at) == 0 {
		return h, false
	}
	var nums, dens []int64
	for _, pg := range r.pages {
		a, b := at[pg.Page], base[pg.Page]
		if a == nil || b == nil {
			continue
		}
		h.Sheets++
		e, fc := r.pageErrors(a)
		e32, fc32 := r.pageErrors(b)
		h.Errors, h.At32 = h.Errors+e, h.At32+e32
		for k := range fc {
			if !fc32[k] {
				h.NewFalseCorrect++
			}
		}
		nums, dens = append(nums, int64(e-e32)), append(dens, 1)
	}
	h.Test = SignTest(nums, dens)
	return h, h.Sheets > 0
}

// Markdown writes the report.
func (r *Workbook) Markdown(w io.Writer, logName string) error {
	l := r.log
	var b strings.Builder
	fmt.Fprintf(&b, "# The workbook: %s\n\n", partOf(l.Run.Part))
	var readers []string
	for _, rd := range r.readers {
		var more []string
		if rd.Once {
			more = append(more, "once")
		}
		readers = append(readers, configName(rd.Name, rd.Model, rd.Effort, rd.NoThinking, more...))
	}
	var nums []string
	for _, pg := range r.pages {
		nums = append(nums, strconv.Itoa(pg.Page))
	}
	made, failed, spent := tally(l.Calls, func(c *workbook.CallRecord) (bool, bool, float64) { return !c.NotAsked, c.Error != "", c.CostUSD })
	fmt.Fprintf(&b, "The log %s, of the prompt %s, version %d. The pages of %s, drawn with the seed %d: %s. Readers: %s. Repeats: %d. Reads: %s. Calls: %d made, %d of them failed; $%.2f spent of the $%.2f cap.\n",
		logName, short(l.Run.Prompt), l.Run.PromptVersion, l.Run.File, l.Run.Seed, strings.Join(nums, ", "), strings.Join(readers, "; "), l.Run.Repeats, strings.Join(l.Run.Reads, ", "), made, failed, spent, l.Run.MaxUSD)
	b.WriteString(endNote(l.End))

	b.WriteString("\n## The boxes at 32 px\n\n| Reader | Condition | Group | Readings | Read as written | Right | False \"correct\" | False alarms | Extra \"unreadable\" | Unchecked |\n|---|---|---|---|---|---|---|---|---|---|\n")
	for _, c := range r.Cells {
		fmt.Fprintf(&b, "| %s | %s | %s | %d | %d | %d | %s | %s | %s | %d |\n", c.Reader, c.Condition, c.Group, c.Readings, c.Read, c.Right, c.FalseCorrect, c.FalseAlarm, c.Extra, c.Unchecked)
	}
	fmt.Fprintf(&b, "\nA reading of a box is right when its verdict, and its trap, is the one a right reading comes to by the log's prompt: the box's as written, but from version %d on \"unreadable\" on a box with a digit written mirrored, which is then no extra \"unreadable\". ", workbook.MirroredVersion)
	b.WriteString("A false \"correct\" is counted among the readings of boxes whose value is not yet right, a false alarm among those of boxes whose value is right, and an extra \"unreadable\" among the readings of filled boxes, \"unchecked\" counted with it; a blank box goes to no model, and its reading is right. Read as written: the text read is the value written, without spaces and after normalization. A share's bound is the one-sided upper bound of 95 % over the pages, Clopper–Pearson on n / Deff.\n")

	if len(r.Mirrored) > 0 {
		b.WriteString("\n## Mirrored digits at 32 px\n\n| Reader | Condition | Readings | Asked to write again | Of them marked mirrored | Read as the digit | False alarms | Unchecked | Other boxes marked mirrored |\n|---|---|---|---|---|---|---|---|---|\n")
		for _, m := range r.Mirrored {
			fmt.Fprintf(&b, "| %s | %s | %d | %s | %d | %d | %s | %d | %s |\n", m.Reader, m.Condition, m.Readings, m.Rewrite, m.Marked, m.AsDigit, m.FalseAlarm, m.Unchecked, m.Elsewhere)
		}
		fmt.Fprintf(&b, "\nThe boxes with a digit written mirrored. By the rule of spec v0.3.3 such a digit is not read as a digit: the box is ambiguous, \"unreadable\", and the learner is asked to write it again, while a false alarm on it counts against the target of 1 %% (R56 spec, amendment of 2026-10-29). Asked to write again is \"unreadable\" on such a box, whatever the reader said of it, for the learner is asked to write it again all the same, and a false alarm is \"not yet\", each among its readings; read as the digit is \"correct\", which from version %d on misses asking. Marked mirrored is the reader saying a digit is written mirrored, which prompts of version %d and later ask, so that marked is what shows the reader saw the mirroring; the other boxes marked mirrored are counted among the readings of the other filled boxes. This log's prompt is version %d.\n", workbook.MirroredVersion, workbook.MirroredVersion, l.Run.PromptVersion)
	}

	b.WriteString("\n## The items at 32 px\n\n| Reader | Condition | Readings | Right | False \"correct\" | False alarms |\n|---|---|---|---|---|---|\n")
	for _, it := range r.Items {
		fmt.Fprintf(&b, "| %s | %s | %d | %d | %s | %s |\n", it.Reader, it.Condition, it.Readings, it.Right, it.FalseCorrect, it.FalseAlarm)
	}
	b.WriteString("\nAn item is right when its verdict, made of its boxes', is the one its boxes come to as a right reading reads them; a false \"correct\" and a false alarm of an item are by the values written in its boxes.\n")

	if len(r.Captions) > 0 {
		b.WriteString("\n## The line of the item against none\n\n| Reader | Pages | Errors bare | Errors captioned | p, captioned worse | p, captioned better | Boxes with a false \"correct\" captioned alone | The rule |\n|---|---|---|---|---|---|---|---|\n")
		for _, c := range r.Captions {
			fmt.Fprintf(&b, "| %s | %d | %d | %d | %s | %s | %d | %s |\n", c.Reader, c.Pages, c.Bare, c.Captioned, p(c.Test.Helps), p(c.Test.Harms), c.OnlyCaptioned, c.verdict())
		}
		fmt.Fprintf(&b, "\nd of a page is its errors captioned less those bare, over the repeats read in both conditions; the errors are false \"correct\", false alarms, extra \"unreadable\", \"unchecked\" and, from prompt version %d on, a digit written mirrored read as the digit. The test is the exact one-sided sign-permutation test over the pages, at p < %.2f. The rule written before the run (T80): a false \"correct\" captioned on a box that has none bare means the line of the item does not go into the reading of a box; the line helps when the reading captioned is better, with no such box.\n", workbook.MirroredVersion, Alpha)
	}

	if len(r.Heights) > 0 {
		b.WriteString("\n## Other heights of digits against 32 px\n\n| Reader | Height, px | Pages | Errors at the height | At 32 px | Pages with d ≠ 0 | p, worse | New false \"correct\" | Not worse |\n|---|---|---|---|---|---|---|---|---|\n")
		for _, h := range r.Heights {
			fmt.Fprintf(&b, "| %s | %d | %d | %d | %d | %d | %s | %d | %s |\n", h.Reader, h.Digit, h.Sheets, h.Errors, h.At32, h.Test.Nonzero, p(h.Test.Helps), h.NewFalseCorrect, yes(h.NotWorse()))
		}
		fmt.Fprintf(&b, "\nBare, the first repeat, paired by pages: d of a page is its errors at the height less those at 32 px, counted as for the line of the item. The height is not worse if the test does not find it worse at p < %.2f and no box has a false \"correct\" at it where it had none at 32 px (T72).\n", Alpha)
	}

	b.WriteString("\n## Time and cost of a call\n\n| Reader | Condition | Height, px | Calls | Time p50, ms | Time p95, ms | Cost, $ |\n|---|---|---|---|---|---|---|\n")
	for _, rd := range r.readers {
		for _, digit := range render.Heights() {
			for _, cond := range []workbook.Condition{workbook.Bare, workbook.Captioned} {
				calls := r.calls(rd.Name, cond, digit)
				if len(calls) == 0 {
					continue
				}
				var times []int64
				cost := 0.0
				for _, c := range calls {
					times = append(times, c.LatencyMs)
					cost += c.CostUSD
				}
				fmt.Fprintf(&b, "| %s | %s | %d | %d | %d | %d | %.5f |\n", rd.Name, cond, digit, len(calls), percentile(times, 0.5), percentile(times, 0.95), cost/float64(len(calls)))
			}
		}
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// CSV writes the readings of the boxes by reader, condition and group, a row each.
func (r *Workbook) CSV(w io.Writer) error {
	cw := csv.NewWriter(w)
	rows := [][]string{{"reader", "condition", "group", "readings", "read_as_written", "right", "false_correct", "false_correct_of", "false_correct_upper",
		"false_alarms", "false_alarms_of", "false_alarms_upper", "extra_unreadable", "extra_unreadable_of", "extra_unreadable_upper", "unchecked"}}
	f := func(v float64) string { return strconv.FormatFloat(v, 'f', 4, 64) }
	for _, c := range r.Cells {
		rows = append(rows, []string{c.Reader, string(c.Condition), c.Group, itoa(c.Readings), itoa(c.Read), itoa(c.Right),
			itoa(c.FalseCorrect.Events), itoa(c.FalseCorrect.Of), f(c.FalseCorrect.Upper),
			itoa(c.FalseAlarm.Events), itoa(c.FalseAlarm.Of), f(c.FalseAlarm.Upper),
			itoa(c.Extra.Events), itoa(c.Extra.Of), f(c.Extra.Upper), itoa(c.Unchecked)})
	}
	if err := cw.WriteAll(rows); err != nil {
		return err
	}
	return cw.Error()
}

// Summary is what the report says on the terminal: for every reader, its readings of all the boxes
// bare at 32 px, and what the rule says of the line of the item.
func (r *Workbook) Summary() string {
	var b strings.Builder
	for _, c := range r.Cells {
		if c.Group == AllBoxes && c.Condition == workbook.Bare {
			fmt.Fprintf(&b, "%s, bare: %d of %d readings right; false \"correct\" %s, false alarms %s, extra \"unreadable\" %s.\n", c.Reader, c.Right, c.Readings, c.FalseCorrect, c.FalseAlarm, c.Extra)
		}
	}
	for _, m := range r.Mirrored {
		if m.Condition == workbook.Bare {
			fmt.Fprintf(&b, "%s, bare, mirrored digits: asked to write again %s, read as the digit %d, false alarms %s; other boxes marked mirrored %s.\n", m.Reader, m.Rewrite, m.AsDigit, m.FalseAlarm, m.Elsewhere)
		}
	}
	for _, c := range r.Captions {
		fmt.Fprintf(&b, "%s, the line of the item: %s.\n", c.Reader, c.verdict())
	}
	return b.String()
}
