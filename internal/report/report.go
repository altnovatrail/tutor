// Package report turns the log of a run into the measurement's numbers and its criterion (R24;
// docs/architecture.md, 3.1, «Замер хода решения»), asking no model, and writes them as Markdown
// and CSV. A report holds no handwriting: numbers, the numbers and scenarios of sheets, and the
// names of judges and forms.
//
// The report of a run of free writing, in version 2 of the judge's prompt, tells more (T65; 3.4,
// «Замер свободной записи»): the boxes on the sheets with a mistake and form D against form A on
// them, the data for R52 spec; the answers by layout and by where the first error stands; and the
// answers sheet by sheet. The report of a log of E2 is as it was.
package report

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/altnovatrail/tutor/internal/judge"
	"github.com/altnovatrail/tutor/internal/measure"
	"github.com/altnovatrail/tutor/internal/sample"
)

// Alpha is the threshold of the criterion's test (architecture 3.1).
const Alpha = 0.05

// MinSheets is how many sheets with a difference the test needs to be passed at all: with fewer,
// the smallest p it can give is 1/16 (architecture 3.1).
const MinSheets = 5

// key is an answer's place: a sheet, a form, a judge and a repeat.
type key struct {
	sheet  int
	form   judge.Form
	judge  string
	repeat int
}

// Report is what a run's log comes to.
type Report struct {
	log    *measure.Log
	sheets map[int]*measure.SheetRecord
	// answers holds every answer by its place; a form D that form B stands for holds form B's.
	answers map[key]*measure.CallRecord
	judges  []string

	// Main is the criterion, D against A, a judge each, and Helps the overall conclusion.
	Main  []Criterion
	Helps bool
	// Others are the comparisons without a threshold, on the sheets with frames.
	Others []Comparison
	// Cells are the answers by judge and form.
	Cells []Cell
	// Agreements are those of every other judge with the first of the run, in "correct".
	Agreements []Agreement

	// Free tells a run of free writing, in version 2 of the prompt; the rest only such a run has.
	Free bool
	// Boxes are the boxes of the answers on the sheets with a mistake, by judge and form, and
	// OnMistakes form D against form A on those sheets, a judge each: the data for R52 spec.
	Boxes      []Boxes
	OnMistakes []Comparison
	// ByLayout and ByFirstError are the right answers by the layout of a sheet and by where its first
	// error stands, a judge each, and BySheet those of every sheet by every judge.
	ByLayout, ByFirstError []Breakdown
	BySheet                []SheetRow
}

// Boxes is what a judge's answers in a form on the sheets with a mistake come to: of the answers,
// how many say "error", how many of those give a box, and how many boxes are on the first error,
// that is, right answers.
type Boxes struct {
	Judge                       string
	Form                        judge.Form
	Answers, Errors, Boxed, Hit int
}

// Breakdown is the right answers of a judge, of its answers, in each form, over the sheets of a
// group: a layout, or where the first error stands.
type Breakdown struct {
	Group          string
	Judge          string
	Sheets         int
	Right, Answers [4]int // by the forms in their order
}

// SheetRow is the right answers of a sheet by a judge, of its answers, in each form, repeats
// together.
type SheetRow struct {
	Sheet          *measure.SheetRecord
	Judge          string
	Right, Answers [4]int // by the forms in their order
}

// Test is the exact one-sided sign-permutation test of a sum of differences over sheets (R03,
// amendment 2026-10-21): of all the ways to give each nonzero difference a sign, as likely as each
// other if the form does nothing, the share whose sum reaches the one observed, upwards for Helps
// and downwards for Harms.
type Test struct {
	Sum          float64
	Nonzero      int
	Helps, Harms float64
}

// Comparison is form X against form Y by a judge over a set of sheets.
type Comparison struct {
	Judge string
	X, Y  judge.Form
	Test
}

// Criterion is the main check of a judge (architecture 3.1): D against A over all the sheets, with
// the false "correct" on sheets with a mistake and the false alarms on correct sheets of each form,
// what it comes to, and whether it passes with Holm's correction for the judges.
type Criterion struct {
	Comparison
	FalseCorrectA, FalseCorrectD int
	FalseAlarmA, FalseAlarmD     int
	Outcome                      string // "helps", "harms" or "not shown"
	Holm                         bool
}

// Cell is what a judge's answers in a form come to.
type Cell struct {
	Judge                                string
	Form                                 judge.Form
	Answers, Right                       int
	FalseCorrect, FalseAlarm             int
	Unreadable, Failed                   int
	FixedAlarms, FixedAnswers            int // on erasure-fixed sheets: false alarms, of answers
	BrokenRight, BrokenAnswers           int // on erasure-broken sheets: right answers, of answers
	Split, Repeated                      int // sheets whose repeats disagree, of sheets with two or more
	TimeP50, TimeP95, FirstP50, FirstP95 int64
	MeanUSD                              float64
}

// Agreement is how a judge agrees with the first judge of the run in "correct" (architecture 2.6):
// of its answers "correct" with the first judge's answer at the same place, how many of the first
// judge's say "correct" too, and how many of all its answers say "unreadable".
type Agreement struct {
	Judge, With         string
	Correct, Both       int
	Unreadable, Answers int
}

// New reads a run's log into its report.
func New(l *measure.Log) (*Report, error) {
	r := &Report{log: l, sheets: map[int]*measure.SheetRecord{}, answers: map[key]*measure.CallRecord{}}
	for i := range l.Sheets {
		r.sheets[l.Sheets[i].Sheet] = &l.Sheets[i]
	}
	for _, j := range l.Run.Judges {
		r.judges = append(r.judges, j.Name)
	}
	if len(r.judges) == 0 || len(r.sheets) == 0 {
		return nil, errors.New("report: the log tells no judge or no sheet")
	}
	for i := range l.Calls {
		c := &l.Calls[i]
		k := key{c.Sheet, c.Form, c.Judge, c.Repeat}
		switch {
		case r.sheets[c.Sheet] == nil:
			return nil, fmt.Errorf("report: a call of sheet %d, which the log does not tell", c.Sheet)
		case !slices.Contains(r.judges, c.Judge) || !slices.Contains(judge.Forms, c.Form) || c.Repeat < 1 || c.Repeat > l.Run.Repeats:
			return nil, fmt.Errorf("report: a call of sheet %d by judge %q in form %q, repeat %d, which the run does not have", c.Sheet, c.Judge, c.Form, c.Repeat)
		case r.answers[k] != nil:
			return nil, fmt.Errorf("report: sheet %d has two calls in form %s by %s in repeat %d", c.Sheet, c.Form, c.Judge, c.Repeat)
		}
		r.answers[k] = c
	}
	for k, c := range r.answers {
		if c.SameAs != "" {
			r.answers[k] = r.answers[key{k.sheet, c.SameAs, k.judge, k.repeat}]
		}
	}
	all := r.numbers(func(*measure.SheetRecord) bool { return true })
	framed := r.numbers(func(sh *measure.SheetRecord) bool { return sh.Frames > 0 })
	for _, j := range r.judges {
		cells := map[judge.Form]Cell{}
		for _, f := range judge.Forms {
			cells[f] = r.cell(j, f, all)
			r.Cells = append(r.Cells, cells[f])
		}
		c := Criterion{Comparison: r.compare(j, judge.FormD, judge.FormA, all)}
		c.FalseCorrectA, c.FalseAlarmA = cells[judge.FormA].FalseCorrect, cells[judge.FormA].FalseAlarm
		c.FalseCorrectD, c.FalseAlarmD = cells[judge.FormD].FalseCorrect, cells[judge.FormD].FalseAlarm
		switch {
		case c.Helps < Alpha && c.FalseCorrectD <= c.FalseCorrectA && c.FalseAlarmD <= c.FalseAlarmA:
			c.Outcome = "helps"
		case c.Harms < Alpha && c.FalseCorrectA <= c.FalseCorrectD && c.FalseAlarmA <= c.FalseAlarmD:
			c.Outcome = "harms"
		default:
			c.Outcome = "not shown"
		}
		r.Main = append(r.Main, c)
		for _, pair := range [][2]judge.Form{{judge.FormB, judge.FormA}, {judge.FormC, judge.FormA}, {judge.FormD, judge.FormB}} {
			r.Others = append(r.Others, r.compare(j, pair[0], pair[1], framed))
		}
	}
	r.holm()
	for _, j := range r.judges[1:] {
		r.Agreements = append(r.Agreements, r.agree(j, r.judges[0]))
	}
	if r.Free = measure.PromptOf(l.Run) == judge.V2; r.Free {
		r.free(all)
	}
	return r, nil
}

// The groups of the breakdowns, by their names in the report and in their order: a sheet's layout,
// its two drawings one group, and where its first error stands. A sheet that shows no layout, such
// as an author's free solution, is of the group "none", and a sheet with a mistake whose place the
// log does not tell, marked off its plan, of "off the plan". A layout or a place the report does
// not know is a group of its own, by its name in the log, after the others: no sheet drops out.
var (
	layoutGroups = map[sample.Layout]string{sample.LayoutNumberLine: "number line", sample.LayoutRow: "drawing", sample.LayoutScheme: "drawing", sample.LayoutSide: "at the side", "": "none"}
	layoutOrder  = []string{"number line", "drawing", "at the side", "none"}
	regionGroups = map[sample.Region]string{sample.RegionDrawing: "in the drawing", sample.RegionText: "in the text", sample.RegionSide: "at the side", "": "off the plan"}
	regionOrder  = []string{"in the drawing", "in the text", "at the side", "off the plan"}
)

// layoutGroup and regionGroup name the group of a layout and of a place.
func layoutGroup(l sample.Layout) string {
	if g, ok := layoutGroups[l]; ok {
		return g
	}
	return string(l)
}

func regionGroup(r sample.Region) string {
	if g, ok := regionGroups[r]; ok {
		return g
	}
	return string(r)
}

// free counts what only a run of free writing tells, over the sheets.
func (r *Report) free(all []int) {
	mistakes := r.numbers(func(sh *measure.SheetRecord) bool { return sh.Outcome == sample.OutcomeError })
	for _, j := range r.judges {
		for _, f := range judge.Forms {
			b := Boxes{Judge: j, Form: f}
			for _, n := range mistakes {
				for _, a := range r.answersOf(n, f, j) {
					b.Answers++
					if a.Error != "" || a.Verdict != judge.Error {
						continue
					}
					b.Errors++
					if a.Box != nil {
						b.Boxed++
					}
					if right(r.sheets[n], a) {
						b.Hit++
					}
				}
			}
			r.Boxes = append(r.Boxes, b)
		}
		r.OnMistakes = append(r.OnMistakes, r.compare(j, judge.FormD, judge.FormA, mistakes))
	}
	// group breaks the sheets of a set down by the name of their group, the known groups in their
	// order and the others after them.
	group := func(set []int, order []string, name func(*measure.SheetRecord) string) []Breakdown {
		names := slices.Clone(order)
		for _, n := range set {
			if g := name(r.sheets[n]); !slices.Contains(names, g) {
				names = append(names, g)
			}
		}
		var out []Breakdown
		for _, g := range names {
			sheets := slices.DeleteFunc(slices.Clone(set), func(n int) bool { return name(r.sheets[n]) != g })
			if len(sheets) == 0 {
				continue
			}
			for _, j := range r.judges {
				b := Breakdown{Group: g, Judge: j, Sheets: len(sheets)}
				for _, n := range sheets {
					got, asked := r.rights(n, j)
					b.Right, b.Answers = add(b.Right, b.Answers, got, asked)
				}
				out = append(out, b)
			}
		}
		return out
	}
	r.ByLayout = group(all, layoutOrder, func(sh *measure.SheetRecord) string { return layoutGroup(sh.Layout) })
	r.ByFirstError = group(mistakes, regionOrder, func(sh *measure.SheetRecord) string { return regionGroup(sh.FirstErrorIn) })
	for _, n := range all {
		for _, j := range r.judges {
			row := SheetRow{Sheet: r.sheets[n], Judge: j}
			row.Right, row.Answers = r.rights(n, j)
			r.BySheet = append(r.BySheet, row)
		}
	}
}

// rights counts the right answers of a sheet by a judge, and its answers, in each form.
func (r *Report) rights(n int, j string) (ok, of [4]int) {
	for i, f := range judge.Forms {
		for _, a := range r.answersOf(n, f, j) {
			of[i]++
			if right(r.sheets[n], a) {
				ok[i]++
			}
		}
	}
	return ok, of
}

// add adds the counts of a sheet to those of a group.
func add(ok, of, sheetOK, sheetOf [4]int) ([4]int, [4]int) {
	for i := range ok {
		ok[i] += sheetOK[i]
		of[i] += sheetOf[i]
	}
	return ok, of
}

// numbers lists the sheets that pass the test, by their number.
func (r *Report) numbers(pass func(*measure.SheetRecord) bool) []int {
	var out []int
	for n, sh := range r.sheets {
		if pass(sh) {
			out = append(out, n)
		}
	}
	slices.Sort(out)
	return out
}

// right reports whether an answer is right by the markup: "correct" on a correct sheet, "error"
// with the box on the line of the first error on a sheet with a mistake, and "unreadable" on an
// unreadable one. A failed call is a wrong answer.
func right(sh *measure.SheetRecord, c *measure.CallRecord) bool {
	if c.Error != "" {
		return false
	}
	switch sh.Outcome {
	case sample.OutcomeCorrect:
		return c.Verdict == judge.Correct
	case sample.OutcomeError:
		return c.Verdict == judge.Error && c.Hit == sh.FirstError
	case sample.OutcomeUnreadable:
		return c.Verdict == judge.Unreadable
	}
	return false
}

// answersOf are the answers a sheet got in a form by a judge, repeat by repeat.
func (r *Report) answersOf(sheet int, f judge.Form, j string) []*measure.CallRecord {
	var out []*measure.CallRecord
	for rep := 1; rep <= r.log.Run.Repeats; rep++ {
		if c := r.answers[key{sheet, f, j, rep}]; c != nil {
			out = append(out, c)
		}
	}
	return out
}

// compare tests form x against form y by a judge over the sheets: d of a sheet is the share of
// right answers in x less that in y, over the sheet's answers in each.
func (r *Report) compare(j string, x, y judge.Form, sheets []int) Comparison {
	var nums, dens []int64
	for _, n := range sheets {
		ax, ay := r.answersOf(n, x, j), r.answersOf(n, y, j)
		if len(ax) == 0 || len(ay) == 0 {
			continue
		}
		rx, ry := 0, 0
		for _, c := range ax {
			if right(r.sheets[n], c) {
				rx++
			}
		}
		for _, c := range ay {
			if right(r.sheets[n], c) {
				ry++
			}
		}
		nums = append(nums, int64(rx*len(ay)-ry*len(ax)))
		dens = append(dens, int64(len(ax)*len(ay)))
	}
	return Comparison{Judge: j, X: x, Y: y, Test: SignTest(nums, dens)}
}

// SignTest is the exact one-sided sign-permutation test of the differences nums[i]/dens[i]. It
// puts them over one denominator and counts the sums of every way to sign the nonzero ones, so it
// needs no sampling. The counts are exact up to 53 nonzero differences, and as near as a float
// holds beyond.
func SignTest(nums, dens []int64) Test {
	l := int64(1)
	for _, d := range dens {
		l = l / gcd(l, d) * d
	}
	var t Test
	var v []int64
	observed := int64(0)
	for i, n := range nums {
		x := n * (l / dens[i])
		observed += x
		t.Sum += float64(n) / float64(dens[i])
		if x != 0 {
			v = append(v, abs(x))
		}
	}
	t.Nonzero = len(v)
	if len(v) == 0 {
		t.Helps, t.Harms = 1, 1
		return t
	}
	total := int64(0)
	for _, x := range v {
		total += x
	}
	// ways[s+total] counts the signings whose sum is s.
	ways := make([]float64, 2*total+1)
	ways[total] = 1
	for _, x := range v {
		next := make([]float64, len(ways))
		for s, w := range ways {
			if w == 0 {
				continue
			}
			if s+int(x) < len(next) {
				next[s+int(x)] += w
			}
			if s-int(x) >= 0 {
				next[s-int(x)] += w
			}
		}
		ways = next
	}
	var up, down float64
	for s, w := range ways {
		if int64(s)-total >= observed {
			up += w
		}
		if int64(s)-total <= observed {
			down += w
		}
	}
	all := math.Ldexp(1, len(v))
	t.Helps, t.Harms = up/all, down/all
	return t
}

func gcd(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

// holm marks the judges that pass Holm's correction for all the judges as architecture 3.1 wrote it
// before the first run (R05): the judge's own p below Alpha over the number of judges, the
// threshold of Holm's first step. The writing process helps Claude if a judge that helps passes.
func (r *Report) holm() {
	for i := range r.Main {
		if r.Main[i].Helps < Alpha/float64(len(r.Main)) {
			r.Main[i].Holm = true
			r.Helps = r.Helps || r.Main[i].Outcome == "helps"
		}
	}
}

// cell sums a judge's answers in a form over the sheets. A false "correct" is "correct" on a sheet
// with a mistake, and a false alarm "error" on a correct sheet; "unreadable" is neither.
func (r *Report) cell(j string, f judge.Form, sheets []int) Cell {
	c := Cell{Judge: j, Form: f}
	var times, firsts []int64
	asked := 0
	for _, n := range sheets {
		sh := r.sheets[n]
		got := r.answersOf(n, f, j)
		rights := 0
		for _, a := range got {
			c.Answers++
			ok := right(sh, a)
			if ok {
				c.Right++
				rights++
			}
			switch {
			case a.Error != "":
				c.Failed++
			case a.Verdict == judge.Unreadable:
				c.Unreadable++
			case sh.Outcome == sample.OutcomeError && a.Verdict == judge.Correct:
				c.FalseCorrect++
			case sh.Outcome == sample.OutcomeCorrect && a.Verdict == judge.Error:
				c.FalseAlarm++
			}
			switch sh.Scenario {
			case sample.ScenarioErasureFixed:
				c.FixedAnswers++
				if a.Error == "" && a.Verdict == judge.Error {
					c.FixedAlarms++
				}
			case sample.ScenarioErasureBroken:
				c.BrokenAnswers++
				if ok {
					c.BrokenRight++
				}
			}
		}
		if len(got) > 1 {
			c.Repeated++
			if rights != 0 && rights != len(got) {
				c.Split++
			}
		}
		// Time and cost are of the calls the form itself made: a form D that form B stands for
		// holds form B's call, and made none.
		for _, a := range got {
			if a.Form == f && !a.NotAsked {
				asked++
				c.MeanUSD += a.CostUSD
				times = append(times, a.LatencyMs)
				if a.FirstTokenMs > 0 {
					firsts = append(firsts, a.FirstTokenMs)
				}
			}
		}
	}
	if asked > 0 {
		c.MeanUSD /= float64(asked)
	}
	c.TimeP50, c.TimeP95 = percentile(times, 0.50), percentile(times, 0.95)
	c.FirstP50, c.FirstP95 = percentile(firsts, 0.50), percentile(firsts, 0.95)
	return c
}

// percentile is the nearest-rank percentile q of the values, or 0 of none.
func percentile(values []int64, q float64) int64 {
	if len(values) == 0 {
		return 0
	}
	s := slices.Clone(values)
	slices.Sort(s)
	return s[max(int(math.Ceil(q*float64(len(s))))-1, 0)]
}

// agree finds how judge j agrees with judge with in "correct", over the calls of j, each once: a
// form D that form B stands for adds no call of its own.
func (r *Report) agree(j, with string) Agreement {
	a := Agreement{Judge: j, With: with}
	for k, c := range r.answers {
		if k.judge != j || c == nil || c.Form != k.form || c.Error != "" {
			continue
		}
		a.Answers++
		if c.Verdict == judge.Unreadable {
			a.Unreadable++
		}
		o := r.answers[key{k.sheet, k.form, with, k.repeat}]
		if c.Verdict != judge.Correct || o == nil || o.Error != "" {
			continue
		}
		a.Correct++
		if o.Verdict == judge.Correct {
			a.Both++
		}
	}
	return a
}

// Markdown writes the report.
func (r *Report) Markdown(w io.Writer, logName string) error {
	l := r.log
	var b strings.Builder
	title, of := "the writing process", fmt.Sprintf("the %s part", l.Run.Part)
	if r.Free {
		title = "free writing"
	}
	if l.Run.Part == "all" {
		of = "all the sheets"
	}
	fmt.Fprintf(&b, "# The measurement of %s: %s\n\n", title, of)
	var judges []string
	for _, j := range l.Run.Judges {
		thinking := ""
		if j.NoThinking {
			thinking = ", without thinking"
		}
		judges = append(judges, fmt.Sprintf("%s (%s, %s%s)", j.Name, j.Model, j.Effort, thinking))
	}
	framed := len(r.numbers(func(sh *measure.SheetRecord) bool { return sh.Frames > 0 }))
	made, failed, spent := 0, 0, 0.0
	for _, c := range l.Calls {
		if c.SameAs == "" && !c.NotAsked {
			made++
			spent += c.CostUSD
			if c.Error != "" {
				failed++
			}
		}
	}
	prompt := short(l.Run.Prompt)
	if r.Free {
		prompt += ", version 2"
	}
	fmt.Fprintf(&b, "The log %s, of the prompt %s. Judges: %s. Repeats: %d. Sheets: %d, %d of them with frames. Calls: %d made, %d of them failed; $%.2f spent of the $%.2f cap.\n",
		logName, prompt, strings.Join(judges, "; "), l.Run.Repeats, len(r.sheets), framed, made, failed, spent, l.Run.MaxUSD)
	switch {
	case l.End == nil:
		b.WriteString("\n**The log has no end: the run broke off, and the report is of what it holds.**\n")
	case l.End.Stopped != "":
		fmt.Fprintf(&b, "\n**The run stopped short, by %s: the report is of what it holds.**\n", l.End.Stopped)
	}

	b.WriteString("\n## Does the writing process help? Form D against form A\n\n")
	b.WriteString("| Judge | Σd | Sheets with d ≠ 0 | p, helps | p, harms | False \"correct\", A → D | False alarms, A → D | Outcome | Passes Holm |\n|---|---|---|---|---|---|---|---|---|\n")
	var few []string
	for _, c := range r.Main {
		fmt.Fprintf(&b, "| %s | %+.2f | %d | %s | %s | %d → %d | %d → %d | %s | %s |\n", c.Judge, c.Sum, c.Nonzero, p(c.Helps), p(c.Harms),
			c.FalseCorrectA, c.FalseCorrectD, c.FalseAlarmA, c.FalseAlarmD, c.Outcome, yes(c.Holm))
		if c.Nonzero < MinSheets {
			few = append(few, fmt.Sprintf("%s has %d, and its smallest p is 1/%d", c.Judge, c.Nonzero, 1<<c.Nonzero))
		}
	}
	fmt.Fprintf(&b, "\nd of a sheet is the share of right answers in form D less that in form A; on a sheet without frames, form D is form B. A right answer is \"correct\" on a correct sheet, and on a sheet with a mistake \"error\" with the box on the line of the first error; a failed call is a wrong answer. The test is the exact one-sided sign-permutation test over the sheets, at p < %.2f; a judge helps if it passes, with no more false \"correct\" and no more false alarms in D than in A, and harms the other way round.\n", Alpha)
	if len(few) > 0 {
		fmt.Fprintf(&b, "\nWith fewer than %d sheets with d ≠ 0 a judge cannot pass the test: %s.\n", MinSheets, strings.Join(few, "; "))
	}
	if r.Helps {
		fmt.Fprintf(&b, "\n**The writing process helps Claude:** a judge that helps passes Holm's correction for %d judges, p < %.4f.\n", len(r.Main), Alpha/float64(len(r.Main)))
	} else {
		fmt.Fprintf(&b, "\n**That the writing process helps Claude is not shown:** no judge that helps passes Holm's correction for %d judges, p < %.4f.\n", len(r.Main), Alpha/float64(len(r.Main)))
	}

	b.WriteString("\n## Without a threshold\n\n### On the sheets with frames\n\n| Judge | Forms | Σd | Sheets with d ≠ 0 | p, helps | p, harms |\n|---|---|---|---|---|---|\n")
	for _, c := range r.Others {
		fmt.Fprintf(&b, "| %s | %s against %s | %+.2f | %d | %s | %s |\n", c.Judge, c.X, c.Y, c.Sum, c.Nonzero, p(c.Helps), p(c.Harms))
	}
	b.WriteString("\n### Answers by judge and form\n\n| Judge | Form | Answers | Right | False \"correct\" | False alarms | \"Unreadable\" | Failed | Repeats that disagree |\n|---|---|---|---|---|---|---|---|---|\n")
	for _, c := range r.Cells {
		fmt.Fprintf(&b, "| %s | %s | %d | %d | %d | %d | %d | %d | %d of %d sheets |\n", c.Judge, c.Form, c.Answers, c.Right, c.FalseCorrect, c.FalseAlarm, c.Unreadable, c.Failed, c.Split, c.Repeated)
	}
	b.WriteString("\n### Erasures\n\n| Judge | Form | False alarms on erasure-fixed | Right on erasure-broken |\n|---|---|---|---|\n")
	for _, c := range r.Cells {
		fmt.Fprintf(&b, "| %s | %s | %d of %d | %d of %d |\n", c.Judge, c.Form, c.FixedAlarms, c.FixedAnswers, c.BrokenRight, c.BrokenAnswers)
	}
	b.WriteString("\n### Time and cost of a call\n\n| Judge | Form | Time p50, ms | Time p95, ms | First token p50, ms | First token p95, ms | Cost, $ |\n|---|---|---|---|---|---|---|\n")
	for _, c := range r.Cells {
		fmt.Fprintf(&b, "| %s | %s | %d | %d | %d | %d | %.5f |\n", c.Judge, c.Form, c.TimeP50, c.TimeP95, c.FirstP50, c.FirstP95, c.MeanUSD)
	}
	if len(r.Agreements) > 0 {
		fmt.Fprintf(&b, "\n### Agreement with %s in \"correct\"\n\n| Judge | Says \"correct\" | %s says \"correct\" too | Says \"unreadable\" |\n|---|---|---|---|\n", r.judges[0], r.judges[0])
		for _, a := range r.Agreements {
			fmt.Fprintf(&b, "| %s | %d | %d | %d of %d |\n", a.Judge, a.Correct, a.Both, a.Unreadable, a.Answers)
		}
	}
	if r.Free {
		r.freeMarkdown(&b)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// freeMarkdown writes what only the report of a run of free writing has.
func (r *Report) freeMarkdown(b *strings.Builder) {
	b.WriteString("\n## Free writing\n\n### Boxes on the sheets with a mistake\n\n| Judge | Form | Answers | \"error\" | With a box | Box on the first error |\n|---|---|---|---|---|---|\n")
	for _, c := range r.Boxes {
		fmt.Fprintf(b, "| %s | %s | %d | %d | %d | %d |\n", c.Judge, c.Form, c.Answers, c.Errors, c.Boxed, c.Hit)
	}
	b.WriteString("\nA box is on the first error when the part of the markup it holds the most points of is the part of the first error, the drawing whole if the mistake is in a drawing. These and the test below are the data for R52 spec: whether to come back to the writing process.\n")
	b.WriteString("\n### Form D against form A on the sheets with a mistake\n\n| Judge | Σd | Sheets with d ≠ 0 | p, helps | p, harms |\n|---|---|---|---|---|\n")
	for _, c := range r.OnMistakes {
		fmt.Fprintf(b, "| %s | %+.2f | %d | %s | %s |\n", c.Judge, c.Sum, c.Nonzero, p(c.Helps), p(c.Harms))
	}
	breakdown := func(title, by string, rows []Breakdown) {
		fmt.Fprintf(b, "\n### %s\n\nRight answers of the answers, by form.\n\n| %s | Sheets | Judge | A | B | C | D |\n|---|---|---|---|---|---|---|\n", title, by)
		for _, row := range rows {
			fmt.Fprintf(b, "| %s | %d | %s | %s |\n", row.Group, row.Sheets, row.Judge, ofForms(row.Right, row.Answers))
		}
		if len(rows) == 0 {
			b.WriteString("| none | 0 | | | | | |\n")
		}
	}
	breakdown("By layout", "Layout", r.ByLayout)
	breakdown("By where the first error stands", "First error", r.ByFirstError)
	b.WriteString("\n### By sheet\n\nRight answers of the answers, by form, repeats together; on a sheet without frames, form D is form B and form C is not asked.\n\n| Sheet | Writer | Task | Scenario | Layout | First error | Judge | A | B | C | D |\n|---|---|---|---|---|---|---|---|---|---|---|\n")
	for _, row := range r.BySheet {
		sh := row.Sheet
		first := "—"
		if sh.FirstError > 0 {
			first = fmt.Sprintf("part %d, %s", sh.FirstError, regionGroup(sh.FirstErrorIn))
		}
		fmt.Fprintf(b, "| %d | %s | %s | %s | %s | %s | %s | %s |\n", sh.Sheet, orDash(string(sh.Writer)), sh.Task, sh.Scenario, layoutGroup(sh.Layout), first, row.Judge, ofForms(row.Right, row.Answers))
	}
}

// ofForms writes the right answers of the answers in each form, a cell each, or a dash for a form
// with no answers.
func ofForms(right, answers [4]int) string {
	cells := make([]string, len(right))
	for i := range right {
		cells[i] = "—"
		if answers[i] > 0 {
			cells[i] = fmt.Sprintf("%d of %d", right[i], answers[i])
		}
	}
	return strings.Join(cells, " | ")
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// CSV writes the answers by judge and form, a row each.
func (r *Report) CSV(w io.Writer) error {
	cw := csv.NewWriter(w)
	rows := [][]string{{"judge", "form", "answers", "right", "false_correct", "false_alarms", "unreadable", "failed",
		"fixed_alarms", "fixed_answers", "broken_right", "broken_answers", "repeats_disagree", "repeated_sheets",
		"time_p50_ms", "time_p95_ms", "first_token_p50_ms", "first_token_p95_ms", "cost_usd"}}
	for _, c := range r.Cells {
		rows = append(rows, []string{c.Judge, string(c.Form), itoa(c.Answers), itoa(c.Right), itoa(c.FalseCorrect), itoa(c.FalseAlarm),
			itoa(c.Unreadable), itoa(c.Failed), itoa(c.FixedAlarms), itoa(c.FixedAnswers), itoa(c.BrokenRight), itoa(c.BrokenAnswers),
			itoa(c.Split), itoa(c.Repeated), strconv.FormatInt(c.TimeP50, 10), strconv.FormatInt(c.TimeP95, 10),
			strconv.FormatInt(c.FirstP50, 10), strconv.FormatInt(c.FirstP95, 10), strconv.FormatFloat(c.MeanUSD, 'f', 6, 64)})
	}
	if err := cw.WriteAll(rows); err != nil {
		return err
	}
	return cw.Error()
}

func itoa(n int) string { return strconv.Itoa(n) }

// p writes a p-value to four significant digits.
func p(v float64) string { return strconv.FormatFloat(v, 'g', 4, 64) }

func yes(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// short is the start of a fingerprint, enough to tell prompts apart.
func short(fingerprint string) string { return fingerprint[:min(12, len(fingerprint))] }
