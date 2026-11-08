package report

import (
	"fmt"
	"strings"

	"github.com/altnovatrail/tutor/internal/hint"
	"github.com/altnovatrail/tutor/internal/sample"
)

// WordsCell is what the calls of a tutor of a kind of case of the set of the words came to in a
// language (T118); a cell of all its cases has no kind and no language. Calls are those asked, and
// Came those whose hint came. Each share is bounded over the sheets:
//   - Leaked and Broken, of the hints that came, as in HintCell;
//   - Repeated, of the hints that came of the cases with questions asked: the hint repeats one of
//     them (hint.Repeated);
//   - BoxOf, of the hints that came without the work, BoxNoWork gave a box all the same;
//   - Echo, of the hints that came of the cases of a note: the hint takes the note up (hint.Echo);
//   - Gendered, of the hints that came: the rough search of T76 finds the learner's sex told
//     (hint.Gendered), a signal for the judge of the words, which decides.
type WordsCell struct {
	Tutor            string
	Kind             hint.CaseKind
	Lang             sample.Lang
	Calls, Came      int
	Leaked, Broken   Share
	Repeated         Share
	BoxNoWork, BoxOf int
	Echo, Gendered   Share
}

// All is whether the cell is of all the tutor's cases.
func (c WordsCell) All() bool { return c.Kind == "" }

// Words are the cells of the set of the words of a log of it (T118), by tutor, kind of case and
// language, each tutor's followed by all its cases; none of a log without cases.
func (r *Hint) Words() []WordsCell {
	cases := map[string]*hint.Case{}
	for i := range r.log.Run.Cases {
		cases[r.log.Run.Cases[i].ID] = &r.log.Run.Cases[i]
	}
	if len(cases) == 0 {
		return nil
	}
	var out []WordsCell
	for _, tu := range r.tutors {
		for _, kind := range hint.CaseKinds() {
			for _, lang := range r.log.Run.Langs {
				out = append(out, r.wordsCell(cases, WordsCell{Tutor: tu, Kind: kind, Lang: lang}))
			}
		}
		out = append(out, r.wordsCell(cases, WordsCell{Tutor: tu}))
	}
	return out
}

// wordsCell fills a cell with the calls of its tutor, kind and language, or of all its cases.
func (r *Hint) wordsCell(cases map[string]*hint.Case, c WordsCell) WordsCell {
	var leaked, broken, repeated, askedOf, echo, noteOf, gendered, cameOf []int
	for _, id := range r.order {
		var l, b, rp, ao, e, no, g, n, a int
		for _, call := range r.byTutor[c.Tutor][id] {
			k := cases[call.Case]
			if k == nil || !c.All() && (k.Kind != c.Kind || k.Lang != c.Lang) {
				continue
			}
			a++
			if !came(call) {
				continue
			}
			n++
			l += b2i(call.Leaked())
			b += b2i(len(call.Form) > 0)
			g += b2i(len(call.Gendered) > 0)
			if len(k.Asked) > 0 {
				ao++
				rp += b2i(len(call.Repeated) > 0)
			}
			if !k.Work {
				c.BoxOf++
				c.BoxNoWork += b2i(call.Box != nil)
			}
			if k.Note != "" {
				no++
				e += b2i(len(call.NoteEcho) > 0)
			}
		}
		if a == 0 {
			continue
		}
		c.Calls, c.Came = c.Calls+a, c.Came+n
		leaked, broken, gendered, cameOf = append(leaked, l), append(broken, b), append(gendered, g), append(cameOf, n)
		repeated, askedOf, echo, noteOf = append(repeated, rp), append(askedOf, ao), append(echo, e), append(noteOf, no)
	}
	c.Leaked, c.Broken, c.Gendered = share(leaked, cameOf), share(broken, cameOf), share(gendered, cameOf)
	c.Repeated, c.Echo = share(repeated, askedOf), share(echo, noteOf)
	return c
}

// shareOrDash writes a share, or a dash when it is of nothing.
func shareOrDash(s Share) string {
	if s.Of == 0 {
		return "—"
	}
	return s.String()
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// wordsMarkdown writes the section of the set of the words.
func (r *Hint) wordsMarkdown(b *strings.Builder) {
	cells := r.Words()
	if len(cells) == 0 {
		return
	}
	fmt.Fprintf(b, "\n## The set of the words\n\nThe cases of %s, %d of them, sha256 %s (T118).\n\n", r.log.Run.CasesFile, len(r.log.Run.Cases), short(r.log.Run.CasesSHA256))
	b.WriteString("| Tutor | Case | Language | Calls | Hints that came | Leaked | Broken | Repeats a question asked | A box without the work | Takes up the note | The sex told, rough search |\n|---|---|---|---|---|---|---|---|---|---|---|\n")
	for _, c := range cells {
		kind, lang := string(c.Kind), string(c.Lang)
		if c.All() {
			kind, lang = "all", "all"
		}
		box := "—"
		if c.BoxOf > 0 {
			box = fmt.Sprintf("%d of %d", c.BoxNoWork, c.BoxOf)
		}
		fmt.Fprintf(b, "| %s | %s | %s | %d | %d | %s | %s | %s | %s | %s | %s |\n", c.Tutor, kind, lang, c.Calls, c.Came, c.Leaked, c.Broken, shareOrDash(c.Repeated), box, shareOrDash(c.Echo), c.Gendered)
	}
	fmt.Fprintf(b, "\nA case of the rung shows the work and gives the questions of the rungs below; one of the rung put another way shows none and gives the question of the rung itself too; one of a note shows a sheet with a note at the side that is no part of the solution and not for a child. A hint repeats a question when it holds %.0f %% of its words; it takes up the note when it has %d words of it in a row or a number of it the task has not. The rough search is that of T76, and finds the sex of a person of the task as well as the learner's: the judge of the words decides, tutor-eval judge-words.\n", hint.RepeatShare*100, hint.EchoWords)
}
