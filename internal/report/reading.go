package report

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/altnovatrail/tutor/internal/reading"
	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/value"
)

// Reading is what a run of the reading comes to (T71, T72; docs/architecture.md, 3.4, «Чтение в
// наблюдение»): the threshold of R03 for every reader, the readings by reader and condition, the
// rule of two readers (R08), the other heights of digits, the states and the readings by each
// policy of reading on its own (О-8, О-21). It holds no handwriting: numbers, the sets and numbers of sheets,
// and the names of readers and conditions.
type Reading struct {
	log     *reading.Log
	sheets  map[sheetID]*reading.SheetRecord
	order   []sheetID
	readers []reading.ReaderRecord
	// byCall are the calls of the finished sheets at 32 px that were asked, by reader and condition,
	// and by sheet.
	byCall map[readerCondition]map[sheetID][]*reading.CallRecord
	// atHeight are the readings without a key of the finished sheets in the first repeat at the
	// other heights, by reader and height and by sheet; ofStates the readings of the states that
	// were asked, by reader.
	atHeight map[readerDigit]map[sheetID]*reading.CallRecord
	ofStates map[string][]*reading.CallRecord
	// heightReaders are the readers that read some other height.
	heightReaders []string

	// Thresholds are the threshold of R03, a reader each.
	Thresholds []Threshold
	// Cells are the readings of the finished sheets at 32 px, by reader and condition.
	Cells []ReadingCell
	// Pairs are the strong reader, Opus, with each fast one (R08).
	Pairs []Pair
	// Heights are the other heights of digits against 32 px, a reader, a height and a group of
	// sheets each; States the readings of the states, a reader and a kind each; Policies the
	// readings of a sheet by each policy of reading on its own, and Costs the price of a call of
	// each reader (T72).
	Heights  []Height
	States   []StateCell
	Policies []Policy
	Costs    []Cost
}

// readerDigit is a reader at a height of digits.
type readerDigit struct {
	reader string
	digit  int
}

// readerCondition is a reader in a condition.
type readerCondition struct {
	reader    string
	condition reading.Condition
}

// sheetID names a sheet of a run of the reading: its set and its number in it.
type sheetID struct {
	set   string
	sheet int
}

func (s sheetID) String() string { return fmt.Sprintf("%s/%d", s.set, s.sheet) }

// Threshold is the threshold of R03 for a reader, as it lies on the observation (R03, amendment of
// 2026-10-28): the test over the sheets of d, the share of false "correct" and false alarms with the
// right key less that without a key, over the repeats of the sheet whose calls did not fail; the
// calls read under the right key and under a false key on a correct sheet; and the sheets with a
// false "correct" with the right key where no repeat without a key had one. The calls read under the
// right key without being given it are the baseline, outside the threshold. Decides is whether the
// log is of the test part, the only one that answers Т-23.
type Threshold struct {
	Reader string
	Test
	Sheets                             int
	UnderRight, UnderFalse, UnderNone  int
	NewFalseCorrect                    int
	RightCalls, FalseCalls, NoKeyCalls int
	Decides                            bool
}

// Measured is whether the threshold was measured at all: some sheet was read both without a key and
// with the right one.
func (t Threshold) Measured() bool { return t.Sheets > 0 }

// Worse is whether the key makes the reading worse: the test finds more false "correct" and false
// alarms with the key than without, at p < Alpha.
func (t Threshold) Worse() bool { return t.Helps < Alpha }

// Holds is whether the reader passes every point of the threshold: it was measured, the key makes
// nothing worse, and nothing is read under a key. One call is admissible for it, with its admission
// by eval (R07 spec), only if it holds on the test part.
func (t Threshold) Holds() bool {
	return t.Measured() && !t.Worse() && t.UnderRight == 0 && t.UnderFalse == 0 && t.NewFalseCorrect == 0
}

// Passes is whether one call is admissible for the reader: the threshold holds on the test part.
func (t Threshold) Passes() bool { return t.Decides && t.Holds() }

// verdict is what the report says of the threshold of a reader.
func (t Threshold) verdict() string {
	switch {
	case !t.Measured():
		return "not measured: no sheet was read both without a key and with the right one"
	case t.Decides && t.Holds():
		return "one call is admissible"
	case t.Decides:
		return "one call is not admissible"
	case t.Holds():
		return "the threshold holds here; only the test part decides"
	}
	return "the threshold fails here; only the test part decides"
}

// ReadingCell is the readings of the finished sheets at 32 px by a reader in a condition: how many
// were right, false "correct" — "correct" on a sheet whose outcome is not —, false alarms — "not
// yet" on a correct sheet —, extra "unreadable" — on a sheet whose outcome is correct or an error,
// a failed call counted with them (R08) — and failed; each share bounded over the sheets, among the
// calls that can have it: the false "correct" and false alarms among those that did not fail. How many were read under the key, how many parts
// of writing of the markup were read as written, and the time and cost of a call.
type ReadingCell struct {
	Reader                          string
	Condition                       reading.Condition
	Calls                           int
	Right                           int
	Failed                          int
	Under                           int
	FalseCorrect, FalseAlarm, Extra Share
	PartsRead, PartsOf              int
	TimeP50, TimeP95                int64
	MeanUSD                         float64
}

// Share is a count of events among the answers that can have them, and its upper bound of 95 % over
// the sheets.
type Share struct {
	Events, Of int
	Upper      float64
}

func (s Share) String() string { return fmt.Sprintf("%d of %d, ≤ %.3f", s.Events, s.Of, s.Upper) }

// Pair is the rule of two readers (R08; Т-33) on the readings without a key of the finished sheets
// at 32 px, of the first repeat: a sheet is released when both readers give one result and its
// values, normalized as the check of the kind number reads them, are one, and its verdict is
// theirs; otherwise it is "unreadable". Coverage is the share released, and the errors among the
// released are counted against those of each reader alone. The fast reader's "correct" and the
// strong one's with it give p of the hybrid: the share the fast reader sends on (architecture 2.6).
type Pair struct {
	Strong, Fast                          string
	Sheets, Released                      int
	FalseCorrect, FalseAlarm, Extra       int
	StrongAlone, FastAlone                Errors
	FastCorrect, BothCorrect, FastSendsOn int
}

// Errors are a reader's errors alone: false "correct", false alarms and extra "unreadable".
type Errors struct {
	FalseCorrect, FalseAlarm, Extra int
}

// expected is the verdict the outcome of a sheet asks of a reading.
func expected(o sample.Outcome) value.Verdict {
	switch o {
	case sample.OutcomeCorrect:
		return value.Correct
	case sample.OutcomeError:
		return value.NotYet
	}
	return value.Unreadable
}

// legible is whether a sheet's outcome asks for a verdict: correct or an error.
func legible(o sample.Outcome) bool { return o == sample.OutcomeCorrect || o == sample.OutcomeError }

// errorsOf are a call's errors against its sheet's outcome.
func errorsOf(o sample.Outcome, c *reading.CallRecord) Errors {
	v := value.Verdict(c.Verdict)
	var e Errors
	switch {
	case v == value.Correct && o != sample.OutcomeCorrect:
		e.FalseCorrect = 1
	case v == value.NotYet && o == sample.OutcomeCorrect:
		e.FalseAlarm = 1
	case v == value.Unreadable && legible(o):
		e.Extra = 1
	}
	return e
}

// failed is whether a call failed: it gave no verdict.
func failed(c *reading.CallRecord) bool { return c.Error != "" }

// under is whether a call read anything under the key.
func under(c *reading.CallRecord) bool { return c.UnderKey.Result || len(c.UnderKey.Parts) > 0 }

// NewReading builds the report of a run of the reading from its log. It refuses a log without
// readers or sheets, and one whose calls name a sheet, a reader or a condition the log does not
// have.
func NewReading(l *reading.Log) (*Reading, error) {
	if len(l.Run.Readers) == 0 || len(l.Sheets) == 0 {
		return nil, errors.New("report: the log has no readers or no sheets")
	}
	r := &Reading{log: l, sheets: map[sheetID]*reading.SheetRecord{}, readers: l.Run.Readers}
	for i := range l.Sheets {
		sh := &l.Sheets[i]
		id := sheetID{sh.Set, sh.Sheet}
		if _, ok := r.sheets[id]; ok {
			return nil, fmt.Errorf("report: the sheet %s is in the log twice", id)
		}
		for n, st := range sh.States {
			if len(st.Parts) != len(sh.Markup.Parts) {
				return nil, fmt.Errorf("report: the state %d of the sheet %s tells %d parts, and its markup has %d", n+1, id, len(st.Parts), len(sh.Markup.Parts))
			}
		}
		r.sheets[id] = sh
		r.order = append(r.order, id)
	}
	names := map[string]bool{}
	for _, rd := range r.readers {
		names[rd.Name] = true
	}
	r.byCall = map[readerCondition]map[sheetID][]*reading.CallRecord{}
	r.atHeight, r.ofStates = map[readerDigit]map[sheetID]*reading.CallRecord{}, map[string][]*reading.CallRecord{}
	for i := range l.Calls {
		c := &l.Calls[i]
		id := sheetID{c.Set, c.Sheet}
		switch {
		case r.sheets[id] == nil:
			return nil, fmt.Errorf("report: a call of the sheet %s, which the log does not tell", id)
		case !names[c.Reader]:
			return nil, fmt.Errorf("report: a call of the reader %q, which the log does not tell", c.Reader)
		case c.Condition != reading.NoKey && c.Condition != reading.RightKey && c.Condition != reading.FalseKey:
			return nil, fmt.Errorf("report: a call in the condition %q", c.Condition)
		}
		switch {
		case c.State > len(r.sheets[id].States) || c.State < 0:
			return nil, fmt.Errorf("report: a call of the state %d of the sheet %s, which tells %d", c.State, id, len(r.sheets[id].States))
		case c.NotAsked:
			continue
		case c.State > 0:
			r.ofStates[c.Reader] = append(r.ofStates[c.Reader], c)
			continue
		case c.Digit != render.Digit:
			if c.Condition == reading.NoKey && c.Repeat == 1 {
				rd := readerDigit{c.Reader, c.Digit}
				if r.atHeight[rd] == nil {
					r.atHeight[rd] = map[sheetID]*reading.CallRecord{}
				}
				r.atHeight[rd][id] = c
			}
			continue
		}
		rc := readerCondition{c.Reader, c.Condition}
		if r.byCall[rc] == nil {
			r.byCall[rc] = map[sheetID][]*reading.CallRecord{}
		}
		r.byCall[rc][id] = append(r.byCall[rc][id], c)
	}
	for _, rd := range r.readers {
		r.Thresholds = append(r.Thresholds, r.threshold(rd.Name))
		for _, cond := range []reading.Condition{reading.NoKey, reading.RightKey, reading.FalseKey} {
			if c := r.cell(rd.Name, cond); c.Calls > 0 {
				r.Cells = append(r.Cells, c)
			}
		}
	}
	r.Heights, r.States, r.Policies, r.Costs = r.heights(), r.stateCells(), r.policies(), r.costs()
	for _, strong := range r.readers {
		if !strong.Once {
			continue
		}
		for _, fast := range r.readers {
			if !fast.Once {
				r.Pairs = append(r.Pairs, r.pair(strong.Name, fast.Name))
			}
		}
	}
	return r, nil
}

// calls are the calls a reader made of the finished sheets at 32 px in a condition, by sheet.
func (r *Reading) calls(reader string, cond reading.Condition) map[sheetID][]*reading.CallRecord {
	return r.byCall[readerCondition{reader, cond}]
}

// count is how many calls of a sheet did not fail, and their false "correct" and false alarms
// together, and their false "correct" alone.
func count(o sample.Outcome, cs []*reading.CallRecord) (asked, errs, falseCorrect int) {
	for _, c := range cs {
		if failed(c) {
			continue
		}
		asked++
		e := errorsOf(o, c)
		errs += e.FalseCorrect + e.FalseAlarm
		falseCorrect += e.FalseCorrect
	}
	return asked, errs, falseCorrect
}

func (r *Reading) threshold(reader string) Threshold {
	t := Threshold{Reader: reader, Decides: r.log.Run.Part == "test"}
	none, right, falseKey := r.calls(reader, reading.NoKey), r.calls(reader, reading.RightKey), r.calls(reader, reading.FalseKey)
	var nums, dens []int64
	for _, id := range r.order {
		o := r.sheets[id].Outcome
		for _, c := range none[id] {
			t.NoKeyCalls++
			if under(c) {
				t.UnderNone++
			}
		}
		for _, c := range right[id] {
			t.RightCalls++
			if under(c) {
				t.UnderRight++
			}
		}
		for _, c := range falseKey[id] {
			if o == sample.OutcomeCorrect {
				t.FalseCalls++
				if under(c) {
					t.UnderFalse++
				}
			}
		}
		nRight, with, withCorrect := count(o, right[id])
		nNone, without, withoutCorrect := count(o, none[id])
		if nRight == 0 || nNone == 0 {
			continue
		}
		t.Sheets++
		if withCorrect > 0 && withoutCorrect == 0 {
			t.NewFalseCorrect++
		}
		// d is the share with the key less that without, over the calls that gave a verdict.
		nums, dens = append(nums, int64(with*nNone-without*nRight)), append(dens, int64(nRight*nNone))
	}
	t.Test = SignTest(nums, dens)
	return t
}

func (r *Reading) cell(reader string, cond reading.Condition) ReadingCell {
	c := ReadingCell{Reader: reader, Condition: cond}
	by := r.calls(reader, cond)
	var fc, fcOf, fa, faOf, ex, exOf []int
	var times []int64
	spent := 0.0
	for _, id := range r.order {
		o, markup := r.sheets[id].Outcome, r.sheets[id].Markup
		calls := by[id]
		if len(calls) == 0 {
			continue
		}
		var e Errors
		asked := 0
		for _, call := range calls {
			c.Calls++
			times = append(times, call.LatencyMs)
			spent += call.CostUSD
			if value.Verdict(call.Verdict) == expected(o) {
				c.Right++
			}
			if under(call) {
				c.Under++
			}
			var obs *reading.Observation
			if failed(call) {
				c.Failed++
				e.Extra++ // a failed call of a legible sheet counts as an extra "unreadable" (R08)
			} else {
				asked++
				got := errorsOf(o, call)
				e.FalseCorrect, e.FalseAlarm, e.Extra = e.FalseCorrect+got.FalseCorrect, e.FalseAlarm+got.FalseAlarm, e.Extra+got.Extra
				obs, _ = reading.Read(call.Answer)
			}
			read, of := reading.PartsRead(obs, markup)
			c.PartsRead, c.PartsOf = c.PartsRead+read, c.PartsOf+of
		}
		if o != sample.OutcomeCorrect {
			fc, fcOf = append(fc, e.FalseCorrect), append(fcOf, asked)
		}
		if o == sample.OutcomeCorrect {
			fa, faOf = append(fa, e.FalseAlarm), append(faOf, asked)
		}
		if legible(o) {
			ex, exOf = append(ex, e.Extra), append(exOf, len(calls))
		}
	}
	c.FalseCorrect, c.FalseAlarm, c.Extra = share(fc, fcOf), share(fa, faOf), share(ex, exOf)
	c.TimeP50, c.TimeP95 = percentile(times, 0.50), percentile(times, 0.95)
	if c.Calls > 0 {
		c.MeanUSD = spent / float64(c.Calls)
	}
	return c
}

// share is the events of sheets among their answers, with the bound over the sheets.
func share(events, of []int) Share {
	s := Share{Upper: Upper(events, of)}
	for i := range of {
		s.Events, s.Of = s.Events+events[i], s.Of+of[i]
	}
	return s
}

func (r *Reading) pair(strong, fast string) Pair {
	p := Pair{Strong: strong, Fast: fast}
	s, f := r.calls(strong, reading.NoKey), r.calls(fast, reading.NoKey)
	add := func(to *Errors, e Errors) {
		to.FalseCorrect, to.FalseAlarm, to.Extra = to.FalseCorrect+e.FalseCorrect, to.FalseAlarm+e.FalseAlarm, to.Extra+e.Extra
	}
	for _, id := range r.order {
		a, b := firstOf(s[id]), firstOf(f[id])
		if a == nil || b == nil {
			continue
		}
		o := r.sheets[id].Outcome
		p.Sheets++
		add(&p.StrongAlone, errorsOf(o, a))
		add(&p.FastAlone, errorsOf(o, b))
		if value.Verdict(b.Verdict) == value.Correct {
			p.FastCorrect++
			if value.Verdict(a.Verdict) == value.Correct {
				p.BothCorrect++
			}
		} else {
			p.FastSendsOn++
		}
		if !released(a, b) {
			continue
		}
		p.Released++
		e := errorsOf(o, a)
		p.FalseCorrect, p.FalseAlarm, p.Extra = p.FalseCorrect+e.FalseCorrect, p.FalseAlarm+e.FalseAlarm, p.Extra+e.Extra
	}
	return p
}

// released is whether two readings of a sheet agree: neither failed, each gave one result, and
// their values, read as the check of the kind number reads them, and their verdicts are one.
func released(a, b *reading.CallRecord) bool {
	if a.Error != "" || b.Error != "" || a.Value == "" || b.Value == "" || a.Verdict != b.Verdict {
		return false
	}
	va, okA := value.Read(a.Value, nil)
	vb, okB := value.Read(b.Value, nil)
	return okA && okB && va == vb
}

// P is p of the hybrid: the share of sheets the fast reader does not confirm "correct" on, which
// go on to the strong one (architecture 2.6).
func (p Pair) P() float64 {
	if p.Sheets == 0 {
		return 0
	}
	return float64(p.FastSendsOn) / float64(p.Sheets)
}

// Markdown writes the report.
func (r *Reading) Markdown(w io.Writer, logName string) error {
	l := r.log
	var b strings.Builder
	fmt.Fprintf(&b, "# The reading: %s\n\n", partOf(l.Run.Part))
	var readers []string
	for _, rd := range r.readers {
		var more []string
		if rd.Once {
			more = append(more, "once")
		}
		readers = append(readers, configName(rd.Name, rd.Model, rd.Effort, rd.NoThinking, more...))
	}
	made, failed, spent := tally(l.Calls, func(c *reading.CallRecord) (bool, bool, float64) { return !c.NotAsked, c.Error != "", c.CostUSD })
	fmt.Fprintf(&b, "The log %s, of the prompt %s, version %d. Readers: %s. Repeats: %d. Reads: %s. Sheets: %d (%s). Calls: %d made, %d of them failed; $%.2f spent of the $%.2f cap.\n",
		logName, short(l.Run.Prompt), l.Run.PromptVersion, strings.Join(readers, "; "), l.Run.Repeats, strings.Join(l.Run.Reads, ", "), len(r.order), setsOf(l.Run.Sets, r.order), made, failed, spent, l.Run.MaxUSD)
	b.WriteString(endNote(l.End))

	b.WriteString("\n## Does the key make the reading worse? The threshold of R03 (Т-23)\n\n")
	b.WriteString("| Reader | Sheets | Σd | Sheets with d ≠ 0 | p, worse | Under the right key | Under a false key, on correct sheets | Sheets with a new false \"correct\" | Under no key, against the right key | One call admissible |\n|---|---|---|---|---|---|---|---|---|---|\n")
	var few []string
	for _, t := range r.Thresholds {
		fmt.Fprintf(&b, "| %s | %d | %+.2f | %d | %s | %d of %d | %d of %d | %d | %d of %d | %s |\n", t.Reader, t.Sheets, t.Sum, t.Nonzero, p(t.Helps),
			t.UnderRight, t.RightCalls, t.UnderFalse, t.FalseCalls, t.NewFalseCorrect, t.UnderNone, t.NoKeyCalls, t.verdict())
		if t.Nonzero < MinSheets {
			few = append(few, fmt.Sprintf("%s has %d", t.Reader, t.Nonzero))
		}
	}
	fmt.Fprintf(&b, "\nd of a sheet is the share of its false \"correct\" and false alarms with the right key less that without a key, over the repeats whose calls did not fail; a false \"correct\" is \"correct\" on a sheet whose outcome is not, and a false alarm \"not yet\" on a correct sheet. The test is the exact one-sided sign-permutation test over the sheets, and the key makes the reading worse at p < %.2f. One call is admissible for a reader, with its admission by eval, only if the key makes nothing worse, no call with the right key and none with a false key on a correct sheet reads anything under the key, and no sheet has a false \"correct\" with the right key where it had none without a key (R03). What is read under the right key without a key is the baseline, outside the threshold. Only the test part decides (R03).\n", Alpha)
	if len(few) > 0 {
		fmt.Fprintf(&b, "\nWith fewer than %d sheets with d ≠ 0 the test cannot find the reading worse: %s.\n", MinSheets, strings.Join(few, "; "))
	}

	b.WriteString("\n## Readings of the finished sheets at 32 px\n\n| Reader | Condition | Calls | Right | False \"correct\" | False alarms | Extra \"unreadable\" | Failed | Under the key | Parts of writing read |\n|---|---|---|---|---|---|---|---|---|---|\n")
	for _, c := range r.Cells {
		fmt.Fprintf(&b, "| %s | %s | %d | %d | %s | %s | %s | %d | %d | %d of %d |\n", c.Reader, c.Condition, c.Calls, c.Right, c.FalseCorrect, c.FalseAlarm, c.Extra, c.Failed, c.Under, c.PartsRead, c.PartsOf)
	}
	b.WriteString("\nA share's bound is the one-sided upper bound of 95 % over the sheets, Clopper–Pearson on n / Deff. False \"correct\" are counted among the readings that did not fail of sheets whose outcome is not correct, and false alarms among those of correct sheets; extra \"unreadable\" among all the readings of sheets whose outcome is correct or an error, a failed one counted as one (R08). A part of writing is read when the observation has it as written, compared without spaces and after normalization.\n")

	if len(r.Pairs) > 0 {
		b.WriteString("\n## Two readers (R08; Т-33)\n\nWithout a key, the finished sheets at 32 px, the first repeat.\n\n| Readers | Sheets | Released | Released, false \"correct\" | Released, false alarms | Released, extra \"unreadable\" | The strong alone | The fast alone | The fast says \"correct\" | The strong too | p of the hybrid |\n|---|---|---|---|---|---|---|---|---|---|---|\n")
		for _, pr := range r.Pairs {
			fmt.Fprintf(&b, "| %s and %s | %d | %d | %d | %d | %d | %s | %s | %d | %d | %.3f |\n", pr.Strong, pr.Fast, pr.Sheets, pr.Released, pr.FalseCorrect, pr.FalseAlarm, pr.Extra,
				pr.StrongAlone, pr.FastAlone, pr.FastCorrect, pr.BothCorrect, pr.P())
		}
		b.WriteString("\nA sheet is released when both readers give one result and its values, read as the check of the kind number reads them, and their verdicts are one; otherwise it is \"unreadable\". A reader alone is counted as false \"correct\" / false alarms / extra \"unreadable\" on the same sheets. p of the hybrid is the share of sheets the fast reader does not confirm \"correct\" on, which go on to the strong one (architecture 2.6).\n")
	}

	r.markdownUnfinished(&b)

	b.WriteString("\n## Time and cost of a call\n\n| Reader | Condition | Time p50, ms | Time p95, ms | Cost, $ |\n|---|---|---|---|---|\n")
	for _, c := range r.Cells {
		fmt.Fprintf(&b, "| %s | %s | %d | %d | %.5f |\n", c.Reader, c.Condition, c.TimeP50, c.TimeP95, c.MeanUSD)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func (e Errors) String() string {
	return fmt.Sprintf("%d / %d / %d", e.FalseCorrect, e.FalseAlarm, e.Extra)
}

// CSV writes the readings by reader and condition, a row each.
func (r *Reading) CSV(w io.Writer) error {
	cw := csv.NewWriter(w)
	rows := [][]string{{"reader", "condition", "calls", "right", "false_correct", "false_correct_of", "false_correct_upper", "false_alarms", "false_alarms_of", "false_alarms_upper",
		"extra_unreadable", "extra_unreadable_of", "extra_unreadable_upper", "failed", "under_key", "parts_read", "parts_of", "time_p50_ms", "time_p95_ms", "cost_usd"}}
	f := func(v float64) string { return strconv.FormatFloat(v, 'f', 4, 64) }
	for _, c := range r.Cells {
		rows = append(rows, []string{c.Reader, string(c.Condition), itoa(c.Calls), itoa(c.Right),
			itoa(c.FalseCorrect.Events), itoa(c.FalseCorrect.Of), f(c.FalseCorrect.Upper),
			itoa(c.FalseAlarm.Events), itoa(c.FalseAlarm.Of), f(c.FalseAlarm.Upper),
			itoa(c.Extra.Events), itoa(c.Extra.Of), f(c.Extra.Upper),
			itoa(c.Failed), itoa(c.Under), itoa(c.PartsRead), itoa(c.PartsOf),
			strconv.FormatInt(c.TimeP50, 10), strconv.FormatInt(c.TimeP95, 10), strconv.FormatFloat(c.MeanUSD, 'f', 6, 64)})
	}
	if err := cw.WriteAll(rows); err != nil {
		return err
	}
	return cw.Error()
}

// Summary is what the report says on the terminal: for every reader, whether one call is admissible.
func (r *Reading) Summary() string {
	var b strings.Builder
	for _, t := range r.Thresholds {
		fmt.Fprintf(&b, "%s: %s; Σd %+.2f over %d sheets with d ≠ 0, p %.4g; under the right key %d, under a false key %d, new false \"correct\" on %d sheets.\n",
			t.Reader, t.verdict(), t.Sum, t.Nonzero, t.Helps, t.UnderRight, t.UnderFalse, t.NewFalseCorrect)
	}
	return b.String()
}
