package report

import (
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/altnovatrail/tutor/internal/reading"
	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/states"
	"github.com/altnovatrail/tutor/internal/value"
)

// The heights of digits, the states and the readings of a sheet by each policy of reading on its
// own, in the report of a run of the reading (T72; docs/architecture.md, 3.4, «Чтение в
// наблюдение»; R08; О-8; О-21).

// The groups of sheets the heights are told by: every sheet, and the sheets whose writing has small
// letters apart from those without them. On the first the crop's estimate may fall to the height of
// a small letter, and the digits come out larger than asked (В36 in RUN.md).
const (
	AllSheets    = "all"
	NoSmall      = "without small letters"
	SmallLetters = "with small letters"
)

// Height is a height of digits against 32 px for a reader, on a group of sheets: paired by sheet,
// the finished sheet without a key in the first repeat at both heights. d of a sheet is its false
// "correct", false alarms and extra "unreadable" at the height less those at 32 px, a failed call
// counted as an extra "unreadable". The height is not worse than 32 px if the test does not find it
// worse at p < Alpha and no sheet has a false "correct" at it where it had none at 32 px.
type Height struct {
	Reader          string
	Digit           int
	Group           string
	Test            Test
	Sheets          int
	Errors, At32    int
	NewFalseCorrect int
	// Smaller are the sheets whose crop at the height was drawn smaller than asked, to fit.
	Smaller int
}

// NotWorse is whether the height is not worse than 32 px for the reader.
func (h Height) NotWorse() bool {
	return h.Sheets > 0 && !(h.Test.Helps < Alpha) && h.NewFalseCorrect == 0
}

// StateCell is the readings of the states of a kind by a reader, without a key at 32 px, over the
// calls that did not fail: how many gave "correct" or "not yet" while the answer was not yet written
// whole, out of place; of those where it was, and nothing to be erased later was still on the sheet,
// how many were right; and how the unfinished work was read (О-21): the parts written whole read as
// written, the results given while the answer was not written whole, and the parts only begun read
// as if written whole, by a part of the observation that reads no part written whole. A state that
// still shows what is erased later has a truth the markup does not tell, and gives no verdict to
// score.
type StateCell struct {
	Reader, Kind       string
	Calls, Failed      int
	Early              Share
	AnswerDone, Right  int
	Erasing            int
	DoneRead, DoneOf   int
	Invented, NotDone  int
	BegunAsDone, Begun int
}

// The kinds of a state, as the report tells them.
const (
	AfterLine     = "after a line"
	AtPause       = "at a pause"
	BeforeErasure = "before an erasure"
)

// Policy is how many readings a sheet would take beyond its sending by a policy of reading on its
// own (R10; О-8): the mean, the median and the most over the sheets.
type Policy struct {
	Name         string
	Mean, Median float64
	Most         int
}

// Cost is what a call of a reader costs, for the price of the policies: the mean of its calls of
// the states, or, for a reader that read none, of its readings without a key at 32 px.
type Cost struct {
	Reader  string
	PerCall float64
	From    string
}

// small is whether the writing of a sheet has small letters: a part of writing, not crossed out,
// with a lower-case letter. Drawings are told in words, and their words do not count.
func small(m reading.Marked) bool {
	return slices.ContainsFunc(m.Parts, func(p reading.MarkedPart) bool {
		return p.Kind == reading.KindText && !p.CrossedOut && strings.IndexFunc(p.Text, unicode.IsLower) >= 0
	})
}

// firstOf is the call of the first repeat among calls, or nil.
func firstOf(cs []*reading.CallRecord) *reading.CallRecord {
	for _, c := range cs {
		if c.Repeat == 1 {
			return c
		}
	}
	return nil
}

// errorsAt are a call's false "correct", false alarms and extra "unreadable" together, a failed
// call an extra "unreadable" of a legible sheet, and whether one of them is a false "correct".
func errorsAt(o sample.Outcome, c *reading.CallRecord) (int, bool) {
	if failed(c) {
		if legible(o) {
			return 1, false
		}
		return 0, false
	}
	e := errorsOf(o, c)
	return e.FalseCorrect + e.FalseAlarm + e.Extra, e.FalseCorrect > 0
}

// heights are the heights against 32 px of every reader that read them, by group.
func (r *Reading) heights() []Height {
	var out []Height
	r.heightReaders = nil
	for _, rd := range r.readers {
		for _, digit := range render.Heights() {
			if digit == render.Digit {
				continue
			}
			by := r.atHeight[readerDigit{rd.Name, digit}]
			if len(by) == 0 {
				continue
			}
			if !slices.Contains(r.heightReaders, rd.Name) {
				r.heightReaders = append(r.heightReaders, rd.Name)
			}
			for _, group := range []string{AllSheets, NoSmall, SmallLetters} {
				out = append(out, r.height(rd.Name, digit, group, by))
			}
		}
	}
	return out
}

func (r *Reading) height(reader string, digit int, group string, by map[sheetID]*reading.CallRecord) Height {
	h := Height{Reader: reader, Digit: digit, Group: group}
	main := r.calls(reader, reading.NoKey)
	var nums, dens []int64
	for _, id := range r.order {
		sh := r.sheets[id]
		if group != AllSheets && small(sh.Markup) != (group == SmallLetters) {
			continue
		}
		at, base := by[id], firstOf(main[id])
		if at == nil || base == nil {
			continue
		}
		h.Sheets++
		if at.Drawn < at.Digit {
			h.Smaller++
		}
		e, fc := errorsAt(sh.Outcome, at)
		e32, fc32 := errorsAt(sh.Outcome, base)
		h.Errors, h.At32 = h.Errors+e, h.At32+e32
		if fc && !fc32 {
			h.NewFalseCorrect++
		}
		nums, dens = append(nums, int64(e-e32)), append(dens, 1)
	}
	h.Test = SignTest(nums, dens)
	return h
}

// Advice is the height of digits the report advises spec to take by default (R51 spec): the
// smallest of 16 and 24 px not worse than 32 px, on every sheet, for every reader that read any
// height; a reader that did not read that one leaves it unadvised. Otherwise 32 px. Decided is
// whether the log is of the test part, the only one that decides; read is whether the run read the
// heights at all.
func (r *Reading) Advice() (digit int, decided, read bool) {
	decided, read = r.log.Run.Part == "test", len(r.Heights) > 0
	for _, d := range []int{16, 24} {
		ok := read
		for _, reader := range r.heightReaders {
			i := slices.IndexFunc(r.Heights, func(h Height) bool { return h.Reader == reader && h.Digit == d && h.Group == AllSheets })
			ok = ok && i >= 0 && r.Heights[i].NotWorse()
		}
		if ok {
			return d, decided, read
		}
	}
	return render.Digit, decided, read
}

// kindsOf are the kinds of a state: it may be of two at once.
func kindsOf(st states.State) []string {
	var out []string
	if st.Line > 0 {
		out = append(out, AfterLine)
	}
	if st.Pause > 0 {
		out = append(out, AtPause)
	}
	if st.Erasure > 0 {
		out = append(out, BeforeErasure)
	}
	return out
}

// stateCells are the readings of the states of every reader that read them, a kind each.
func (r *Reading) stateCells() []StateCell {
	var out []StateCell
	for _, rd := range r.readers {
		calls := r.ofStates[rd.Name]
		if len(calls) == 0 {
			continue
		}
		for _, kind := range []string{AfterLine, AtPause, BeforeErasure} {
			if c := r.stateCell(rd.Name, kind, calls); c.Calls > 0 {
				out = append(out, c)
			}
		}
	}
	return out
}

func (r *Reading) stateCell(reader, kind string, calls []*reading.CallRecord) StateCell {
	c := StateCell{Reader: reader, Kind: kind}
	early, earlyOf := map[sheetID]int{}, map[sheetID]int{}
	for _, call := range calls {
		id := sheetID{call.Set, call.Sheet}
		sh := r.sheets[id]
		st := sh.States[call.State-1] // NewReading refuses a call of a state the sheet does not tell
		if !slices.Contains(kindsOf(st), kind) {
			continue
		}
		c.Calls++
		if failed(call) {
			c.Failed++
			continue
		}
		answer := slices.IndexFunc(sh.Markup.Parts, func(p reading.MarkedPart) bool { return p.Answer && !p.CrossedOut })
		done := answer >= 0 && st.Parts[answer] == states.Done
		obs, _ := reading.Read(call.Answer)
		v := value.Verdict(call.Verdict)
		switch {
		case done && st.Erased > 0:
			c.Erasing++
		case done:
			c.AnswerDone++
			if v == expected(sh.Outcome) {
				c.Right++
			}
		default:
			earlyOf[id]++
			if v == value.Correct || v == value.NotYet {
				early[id]++
			}
			c.NotDone++
			if obs != nil && len(obs.Results) > 0 {
				c.Invented++
			}
		}
		written := reading.Marked{Parts: []reading.MarkedPart{}}
		for i, p := range sh.Markup.Parts {
			if st.Parts[i] == states.Done {
				written.Parts = append(written.Parts, p)
			}
		}
		read, of := reading.PartsRead(obs, written)
		c.DoneRead, c.DoneOf = c.DoneRead+read, c.DoneOf+of
		rest := reading.Rest(obs, written)
		for i, p := range sh.Markup.Parts {
			if st.Parts[i] == states.Begun && p.Kind == reading.KindText && !p.CrossedOut && !p.Illegible {
				c.Begun++
				if reading.ReadsAs(rest, p.Text) {
					c.BegunAsDone++
				}
			}
		}
	}
	var events, of []int
	for _, id := range r.order {
		if earlyOf[id] > 0 {
			events, of = append(events, early[id]), append(of, earlyOf[id])
		}
	}
	c.Early = share(events, of)
	return c
}

// policies are the readings a sheet would take by each policy of reading on its own, if the log
// tells them: a sheet of it has states or a reading by some policy.
func (r *Reading) policies() []Policy {
	told := slices.ContainsFunc(r.order, func(id sheetID) bool {
		sh := r.sheets[id]
		return len(sh.States) > 0 || sh.Policies != (states.Policies{})
	})
	if !told {
		return nil
	}
	var out []Policy
	for _, p := range []struct {
		name string
		of   func(states.Policies) int
	}{
		{"after each line", func(p states.Policies) int { return p.AfterLine }},
		{"at each pause of 0.4 s", func(p states.Policies) int { return p.Pause400 }},
		{"at each pause of 2 s", func(p states.Policies) int { return p.Pause2000 }},
		{"at each pause of 6 s", func(p states.Policies) int { return p.Pause6000 }},
	} {
		var n []int
		sum := 0
		for _, id := range r.order {
			v := p.of(r.sheets[id].Policies)
			n, sum = append(n, v), sum+v
		}
		slices.Sort(n)
		median := float64(n[(len(n)-1)/2]+n[len(n)/2]) / 2
		out = append(out, Policy{Name: p.name, Mean: float64(sum) / float64(len(n)), Median: median, Most: n[len(n)-1]})
	}
	return out
}

// costs are what a call of each reader costs, for the price of the policies.
func (r *Reading) costs() []Cost {
	var out []Cost
	for _, rd := range r.readers {
		c := Cost{Reader: rd.Name, From: "its states"}
		calls := r.ofStates[rd.Name]
		if len(calls) == 0 {
			c.From = "its readings without a key"
			for _, cs := range r.calls(rd.Name, reading.NoKey) {
				calls = append(calls, cs...)
			}
		}
		if len(calls) == 0 {
			continue
		}
		for _, call := range calls {
			c.PerCall += call.CostUSD
		}
		c.PerCall /= float64(len(calls))
		out = append(out, c)
	}
	return out
}

// markdownUnfinished writes the heights, the states and the policies.
func (r *Reading) markdownUnfinished(b *strings.Builder) {
	if len(r.Heights) > 0 {
		b.WriteString("\n## Heights of digits (R08; R51 spec)\n\nWithout a key, the finished sheets in the first repeat, each height against 32 px on the same sheets.\n\n| Reader | Height, px | Sheets | Errors at the height | Errors at 32 px | Σd | Sheets with d ≠ 0 | p, worse | New false \"correct\" | Drawn smaller, to fit | Not worse than 32 px |\n|---|---|---|---|---|---|---|---|---|---|---|\n")
		for _, h := range r.Heights {
			name := h.Group
			if name == AllSheets {
				name = "all the sheets"
			}
			fmt.Fprintf(b, "| %s | %d, %s | %d | %d | %d | %+.0f | %d | %s | %d | %d | %s |\n", h.Reader, h.Digit, name, h.Sheets, h.Errors, h.At32, h.Test.Sum, h.Test.Nonzero, p(h.Test.Helps), h.NewFalseCorrect, h.Smaller, yes(h.NotWorse()))
		}
		digit, decided, _ := r.Advice()
		fmt.Fprintf(b, "\nAn error is a false \"correct\", a false alarm or an extra \"unreadable\", a failed call counted as one. A height is not worse than 32 px if the exact one-sided sign-permutation test over the sheets does not find it worse at p < %.2f and no sheet has a false \"correct\" at it where it had none at 32 px. The advice is the smallest of 16 and 24 px not worse on all the sheets for every reader that read the heights, and otherwise 32 px. On sheets with small letters the crop's estimate may fall to the height of a small letter, and the digits come out larger than asked (В36).\n", Alpha)
		if decided {
			fmt.Fprintf(b, "\n**The height of digits to advise spec: %d px.**\n", digit)
		} else {
			fmt.Fprintf(b, "\nHere the advice would be %d px; only the test part decides.\n", digit)
		}
	}
	if len(r.States) > 0 {
		b.WriteString("\n## States: the reading of unfinished work (О-8, О-21)\n\nWithout a key, at 32 px. A state of two kinds counts in both.\n\n| Reader | Kind | Calls | Failed | \"Correct\" or \"not yet\" before the answer is written | The answer written: right | The answer written, with what is erased later | Parts written whole, read as written | Results before the answer is written | Begun parts read as if written whole |\n|---|---|---|---|---|---|---|---|---|---|\n")
		for _, c := range r.States {
			fmt.Fprintf(b, "| %s | %s | %d | %d | %s | %d of %d | %d | %d of %d | %d of %d | %d of %d |\n", c.Reader, c.Kind, c.Calls, c.Failed, c.Early, c.Right, c.AnswerDone, c.Erasing, c.DoneRead, c.DoneOf, c.Invented, c.NotDone, c.BegunAsDone, c.Begun)
		}
		b.WriteString("\nThe answer is written when every stroke of its part is. A verdict before that is out of place, its share bounded over the sheets; so is a result given then, which the work does not yet give. A state that still shows what is erased later is not scored for its verdict: the markup does not tell what it shows. All but the calls are counted over the calls that did not fail.\n")
	}
	if len(r.Policies) > 0 {
		b.WriteString("\n## Readings of a sheet by each policy of reading on its own (О-8)\n\nBeyond the sending, from the strokes and erasures alone (R10).\n\n| Policy | Mean a sheet | Median | Most |")
		for _, c := range r.Costs {
			fmt.Fprintf(b, " %s, $ a sheet |", c.Reader)
		}
		b.WriteString("\n|---|---|---|---|" + strings.Repeat("---|", len(r.Costs)) + "\n")
		for _, pol := range r.Policies {
			fmt.Fprintf(b, "| %s | %.2f | %g | %d |", pol.Name, pol.Mean, pol.Median, pol.Most)
			for _, c := range r.Costs {
				fmt.Fprintf(b, " %.5f |", pol.Mean*c.PerCall)
			}
			b.WriteString("\n")
		}
		var from []string
		for _, c := range r.Costs {
			from = append(from, fmt.Sprintf("%s by %s, $%.5f", c.Reader, c.From, c.PerCall))
		}
		fmt.Fprintf(b, "\nA sheet's price by a policy is its mean readings times the mean cost of a call: %s.\n", strings.Join(from, "; "))
	}
}
