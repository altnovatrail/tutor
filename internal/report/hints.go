package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/altnovatrail/tutor/internal/hint"
	"github.com/altnovatrail/tutor/internal/sample"
)

// The hints to read: how many the file holds, and how many of them at most are hints the leak check
// caught (T75; R39).
const (
	HintsToRead  = 30
	CaughtToRead = 10
)

// ToRead are the hints of the first repeat the author reads (T76, «Мой шаг»): every hint the leak
// check caught, up to CaughtToRead, so that a false leak shows; then hints that passed both checks,
// taking each tutor, language and rung in turn, in the order of the run, and from each the hint of
// the sheet taken least so far, the first in the log on a tie, until there are HintsToRead or none
// is left.
func (r *Hint) ToRead() []*hint.CallRecord {
	var caught []*hint.CallRecord
	type cell struct {
		tutor string
		lang  sample.Lang
		rung  hint.Rung
	}
	passed := map[cell][]*hint.CallRecord{}
	for i := range r.log.Calls {
		c := &r.log.Calls[i]
		switch {
		case c.Repeat != 1 || !came(c):
		case c.Leaked():
			if len(caught) < CaughtToRead {
				caught = append(caught, c)
			}
		case c.Fallback == "":
			k := cell{c.Tutor, c.Lang, c.Rung}
			passed[k] = append(passed[k], c)
		}
	}
	var cells []cell
	for _, rung := range r.log.Run.Rungs {
		for _, lang := range r.log.Run.Langs {
			for _, tu := range r.tutors {
				cells = append(cells, cell{tu, lang, rung})
			}
		}
	}
	out := caught
	taken := map[sheetID]int{}
	for _, c := range caught {
		taken[sheetID{c.Set, c.Sheet}]++
	}
	for left := true; left && len(out) < HintsToRead; {
		left = false
		for _, k := range cells {
			if len(out) == HintsToRead {
				break
			}
			calls := passed[k]
			if len(calls) == 0 {
				continue
			}
			left = true
			best := 0
			for i, c := range calls {
				if taken[sheetID{c.Set, c.Sheet}] < taken[sheetID{calls[best].Set, calls[best].Sheet}] {
					best = i
				}
			}
			c := calls[best]
			passed[k] = append(calls[:best:best], calls[best+1:]...)
			taken[sheetID{c.Set, c.Sheet}]++
			out = append(out, c)
		}
	}
	return out
}

// Readable names the hints for the author to read: the third file of the report, .hints.md.
func (r *Hint) Readable() (suffix, label string, write func(io.Writer, string) error) {
	return ".hints.md", "The hints to read", r.Hints
}

// Hints writes the hints to read, each with its sheet and task as the plan and the bank tell them —
// the statement, the key, what the leak check searches for and the text the plan gives the first
// error —, the hint, what the checks found and what the learner reads.
func (r *Hint) Hints(w io.Writer, logName string) error {
	var b strings.Builder
	calls := r.ToRead()
	fmt.Fprintf(&b, "# Hints to read: %s\n\n", logName)
	fmt.Fprintf(&b, "%d hints of the first repeat, to read for what the checks cannot see: a leak the check missed, a hint off its rung, a place that is not the mistake's. First every hint the leak check caught, up to %d, so that a false leak shows; then hints that passed both checks, taking each tutor, language and rung in turn and the sheets as different as they can be. The picture of a sheet is the one tutor-eval draw -png draws of its file.\n",
		len(calls), CaughtToRead)
	for i, c := range calls {
		sh := r.sheets[sheetID{c.Set, c.Sheet}]
		fmt.Fprintf(&b, "\n## %d. %s/%d · rung %d · %s · %s\n\n", i+1, c.Set, c.Sheet, c.Rung, c.Lang, c.Tutor)
		fmt.Fprintf(&b, "- Sheet: %s, task %s, %s.\n", sh.File, sh.Task, sh.Scenario)
		fmt.Fprintf(&b, "- Statement: %s\n", sh.Statement)
		fmt.Fprintf(&b, "- Key: %s. The check searches for the keys %s and the phrases %s.\n", sh.Key, quoted(sh.SearchedKeys), quoted(sh.Phrases))
		fmt.Fprintf(&b, "- The first error, as the plan writes it: part %d, «%s»; the sheet's trap: %s.\n", sh.FirstError, sh.FirstErrorText, orNone(sh.Trap))
		fmt.Fprintf(&b, "- The model's trap: %s; its box %s.\n", orNone(c.Trap), boxWords(c, sh))
		fmt.Fprintf(&b, "- The hint: «%s»\n", c.Hint)
		fmt.Fprintf(&b, "- The leak check found the keys %s and the phrases %s. The form breaks: %s.\n", quoted(c.LeakedKeys), quoted(c.LeakedPhrases), orNone(strings.Join(c.Form, ", ")))
		if c.Fallback == "" {
			b.WriteString("- The learner reads the hint.\n")
		} else {
			fmt.Fprintf(&b, "- The learner reads the fallback text, for %s: «%s»\n", c.Fallback, c.Shown)
		}
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// quoted lists texts in quotes, or says there are none.
func quoted(texts []string) string {
	if len(texts) == 0 {
		return "nothing"
	}
	var q []string
	for _, t := range texts {
		q = append(q, "«"+t+"»")
	}
	return strings.Join(q, ", ")
}

// orNone is a text, or "none" for an empty one.
func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

// boxWords tells where a call's box is: none, on the first error, on another part, or on none.
func boxWords(c *hint.CallRecord, sh *hint.SheetRecord) string {
	switch {
	case c.Box == nil:
		return "none"
	case c.Hit == sh.FirstError:
		return "on the first error"
	case c.Hit == 0:
		return "on no part"
	}
	return fmt.Sprintf("on part %d", c.Hit)
}
