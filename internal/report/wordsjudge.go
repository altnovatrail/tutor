package report

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/altnovatrail/tutor/internal/hint"
	"github.com/altnovatrail/tutor/internal/wordsjudge"
)

// WordsJudge is what a run of the judge of the words comes to (T118; docs/architecture.md, 3.4,
// «Набор слов»): of the control phrases, whether the judge gave every answer they expect, and where
// it did not; of the hints of a log of the set of the words, by tutor, kind of case and language,
// the hints it flags for each question, against the rough search of the learner's sex; and the
// file of every hint flagged, by the judge or by the code, for the executor to read. It holds no
// handwriting: the hints the models wrote, the notes of the set and the control phrases.
type WordsJudge struct {
	log *wordsjudge.Log
	// Cells are the hints judged of each tutor by kind of case and language, each tutor's followed
	// by all its hints; none of a run of the controls.
	Cells []JudgeCell
	// Right and Wrong are the control phrases the judge passed (wordsjudge.Verdict.Passes) and those
	// it did not, failed calls among them, and Exact those whose four answers it gave as expected;
	// none of a run of the hints.
	Right, Exact int
	Wrong        []string
}

// JudgeCell is what the judge said of the hints of a tutor of a kind of case in a language; a cell
// of all its hints has no kind and no language. Judged are the hints whose verdict came, and Failed
// those whose did not. Each share is of the hints judged, bounded over the sheets: Gender, the hint
// tells the learner's sex; OffTask, it leaves the task; NotForAge, it does not suit a child;
// MentionsNote, of the hints of a note, it takes the note up; Flagged, any of them. Rough are the
// hints the rough search finds the sex told in, and Disagree those where it and the judge differ.
type JudgeCell struct {
	Tutor                                             string
	Kind                                              hint.CaseKind
	Lang                                              string
	Judged, Failed                                    int
	Gender, OffTask, NotForAge, MentionsNote, Flagged Share
	Rough, Disagree                                   int
}

// All is whether the cell is of all the tutor's hints.
func (c JudgeCell) All() bool { return c.Kind == "" }

// NewWordsJudge builds the report of a log of the judge of the words.
func NewWordsJudge(l *wordsjudge.Log) (*WordsJudge, error) {
	if len(l.Calls) == 0 {
		return nil, errors.New("report: the log of the judge has no calls")
	}
	r := &WordsJudge{log: l}
	switch l.Run.Of {
	case wordsjudge.OfControls:
		for _, c := range l.Calls {
			if c.Expect == nil {
				return nil, fmt.Errorf("report: a call of the controls without its answers, %q", c.Control)
			}
			switch {
			case c.Verdict == nil || !c.Verdict.Passes(c.Expect.Verdict()):
				r.Wrong = append(r.Wrong, c.Control)
			case c.Verdict.Same(c.Expect.Verdict()):
				r.Right++
				r.Exact++
			default:
				r.Right++
			}
		}
	case wordsjudge.OfLog:
		// The languages of the measurement first, then any other the log has, as it first names them.
		var tutors, langs []string
		for _, l := range hint.Langs() {
			langs = append(langs, string(l))
		}
		for _, c := range l.Calls {
			if !slices.Contains(tutors, c.Tutor) {
				tutors = append(tutors, c.Tutor)
			}
			if !slices.Contains(langs, c.Lang) {
				langs = append(langs, c.Lang)
			}
		}
		// The feedback of the essays has no kinds of case: its cells are by the tutor alone.
		feedback := !slices.ContainsFunc(l.Calls, func(c wordsjudge.CallRecord) bool { return c.CaseKind != "" })
		for _, tu := range tutors {
			for _, kind := range hint.CaseKinds() {
				if feedback {
					break
				}
				for _, lang := range langs {
					r.Cells = append(r.Cells, r.cell(JudgeCell{Tutor: tu, Kind: kind, Lang: lang}))
				}
			}
			r.Cells = append(r.Cells, r.cell(JudgeCell{Tutor: tu}))
		}
	default:
		return nil, fmt.Errorf("report: a run of the judge of %q, neither the hints of a log nor the controls", l.Run.Of)
	}
	return r, nil
}

// cell fills a cell with the hints of its tutor, kind and language, or of all its hints, by sheet.
func (r *WordsJudge) cell(c JudgeCell) JudgeCell {
	type counts struct{ judged, gender, off, age, notes, note, flagged int }
	by := map[sheetID]*counts{}
	var order []sheetID
	for i := range r.log.Calls {
		call := &r.log.Calls[i]
		if call.Tutor != c.Tutor || !c.All() && (call.CaseKind != c.Kind || call.Lang != c.Lang) {
			continue
		}
		v := call.Verdict
		if v == nil {
			c.Failed++
			continue
		}
		id := sheetID{call.Set, call.Sheet}
		n := by[id]
		if n == nil {
			n = &counts{}
			by[id] = n
			order = append(order, id)
		}
		n.judged++
		n.gender += b2i(v.Gender)
		n.off += b2i(!v.OnTask)
		n.age += b2i(!v.ForAge)
		n.flagged += b2i(v.Flagged())
		if call.Note != "" {
			n.notes++
			n.note += b2i(v.MentionsNote)
		}
		rough := len(call.Gendered) > 0
		c.Rough += b2i(rough)
		c.Disagree += b2i(rough != v.Gender)
	}
	var judged, gender, off, age, notes, note, flagged []int
	for _, id := range order {
		n := by[id]
		c.Judged += n.judged
		judged, gender, off, age = append(judged, n.judged), append(gender, n.gender), append(off, n.off), append(age, n.age)
		notes, note, flagged = append(notes, n.notes), append(note, n.note), append(flagged, n.flagged)
	}
	c.Gender, c.OffTask, c.NotForAge, c.Flagged = share(gender, judged), share(off, judged), share(age, judged), share(flagged, judged)
	c.MentionsNote = share(note, notes)
	return c
}

// where names the kind and language of a cell, "all" for a cell of all the tutor's hints.
func (c JudgeCell) where() (string, string) {
	if c.All() {
		return "all", "all"
	}
	return string(c.Kind), c.Lang
}

// Markdown writes the report.
func (r *WordsJudge) Markdown(w io.Writer, logName string) error {
	l := r.log
	var b strings.Builder
	jd := l.Run.Judge
	made, failed, spent := tally(l.Calls, func(c *wordsjudge.CallRecord) (bool, bool, float64) { return !c.NotAsked, c.Verdict == nil, c.CostUSD })
	of := "the hints of a log"
	if l.Run.Of == wordsjudge.OfControls {
		of = "the control phrases"
	}
	fmt.Fprintf(&b, "# The judge of the words: %s\n\nThe log %s, of the prompt %s, version %d, judging %s, sha256 %s. Judge: %s. Calls: %d made, %d of them without a verdict; $%.2f spent of the $%.2f cap.\n",
		of, logName, short(l.Run.Prompt), l.Run.PromptVersion, l.Run.File, short(l.Run.SHA256), configName(jd.Name, jd.Model, jd.Effort, jd.NoThinking), made, failed, spent, l.Run.MaxUSD)
	b.WriteString(endNote(l.End))
	if l.Run.Of == wordsjudge.OfControls {
		fmt.Fprintf(&b, "\n## The control phrases\n\nThe judge passed %d of %d: it told the learner's sex as expected and flagged the hint as expected; it gave all four answers as expected on %d. ", r.Right, len(l.Calls), r.Exact)
		if len(r.Wrong) == 0 {
			b.WriteString("It measures the words: its verdicts count.\n")
		} else {
			fmt.Fprintf(&b, "It is wrong on %s, and its verdicts on the hints measure nothing until it is right on all.\n", strings.Join(r.Wrong, ", "))
		}
		_, err := io.WriteString(w, b.String())
		return err
	}
	b.WriteString("\n## The verdicts\n\n| Tutor | Case | Language | Judged | No verdict | Tells the sex | Leaves the task | Not for a child | Takes up the note | Flagged | The sex told, rough search | Rough search and judge differ |\n|---|---|---|---|---|---|---|---|---|---|---|---|\n")
	for _, c := range r.Cells {
		kind, lang := c.where()
		fmt.Fprintf(&b, "| %s | %s | %s | %d | %d | %s | %s | %s | %s | %s | %d | %d |\n", c.Tutor, kind, lang, c.Judged, c.Failed, c.Gender, c.OffTask, c.NotForAge, shareOrDash(c.MentionsNote), c.Flagged, c.Rough, c.Disagree)
	}
	b.WriteString("\nThe judge reads each hint that came with the task's statement and the note at the side, if there is one, and says whether it tells the learner's sex, keeps to the task, suits a child and takes the note up. Its verdicts count only when it gave every answer its control phrases expect (tutor-eval judge-words -controls). Each share is bounded at 95 % over the sheets. Every hint flagged, by the judge or by the code, is in the file .words.md.\n")
	_, err := io.WriteString(w, b.String())
	return err
}

// CSV writes the cells, a row each, or of the controls, a row a control phrase.
func (r *WordsJudge) CSV(w io.Writer) error {
	cw := csv.NewWriter(w)
	f := func(v float64) string { return strconv.FormatFloat(v, 'f', 4, 64) }
	var rows [][]string
	if r.log.Run.Of == wordsjudge.OfControls {
		rows = append(rows, []string{"control", "lang", "right", "exact", "gender", "on_task", "for_age", "mentions_note"})
		for _, c := range r.log.Calls {
			right, exact, v := "no", "no", c.Verdict
			if v != nil && v.Passes(c.Expect.Verdict()) {
				right = "yes"
			}
			if v != nil && v.Same(c.Expect.Verdict()) {
				exact = "yes"
			}
			if v == nil {
				v = &wordsjudge.Verdict{}
			}
			rows = append(rows, []string{c.Control, c.Lang, right, exact, strconv.FormatBool(v.Gender), strconv.FormatBool(v.OnTask), strconv.FormatBool(v.ForAge), strconv.FormatBool(v.MentionsNote)})
		}
	} else {
		rows = append(rows, []string{"tutor", "case", "lang", "judged", "no_verdict", "gender", "gender_upper", "off_task", "off_task_upper", "not_for_age", "not_for_age_upper",
			"mentions_note", "mentions_note_of", "mentions_note_upper", "flagged", "flagged_upper", "rough", "disagree"})
		for _, c := range r.Cells {
			kind, lang := c.where()
			rows = append(rows, []string{c.Tutor, kind, lang, itoa(c.Judged), itoa(c.Failed), itoa(c.Gender.Events), f(c.Gender.Upper), itoa(c.OffTask.Events), f(c.OffTask.Upper),
				itoa(c.NotForAge.Events), f(c.NotForAge.Upper), itoa(c.MentionsNote.Events), itoa(c.MentionsNote.Of), f(c.MentionsNote.Upper), itoa(c.Flagged.Events), f(c.Flagged.Upper),
				itoa(c.Rough), itoa(c.Disagree)})
		}
	}
	if err := cw.WriteAll(rows); err != nil {
		return err
	}
	return cw.Error()
}

// Summary is what the report says on the terminal.
func (r *WordsJudge) Summary() string {
	if r.log.Run.Of == wordsjudge.OfControls {
		if len(r.Wrong) == 0 {
			return fmt.Sprintf("The judge passes all %d control phrases, %d of them with all four answers as expected.\n", r.Right, r.Exact)
		}
		return fmt.Sprintf("The judge passes %d of %d control phrases, not %s.\n", r.Right, len(r.log.Calls), strings.Join(r.Wrong, ", "))
	}
	var b strings.Builder
	for _, c := range r.Cells {
		if c.All() {
			fmt.Fprintf(&b, "%s: %d hints judged; the sex told %d, off the task %d, not for a child %d, the note taken up %d of %d; flagged %d; the rough search and the judge differ on %d.\n",
				c.Tutor, c.Judged, c.Gender.Events, c.OffTask.Events, c.NotForAge.Events, c.MentionsNote.Events, c.MentionsNote.Of, c.Flagged.Events, c.Disagree)
		}
	}
	return b.String()
}

// Readable is the file of the hints flagged, by the judge or by the code, and of the control
// phrases the judge got wrong, for the executor to read.
func (r *WordsJudge) Readable() (suffix, label string, write func(w io.Writer, logName string) error) {
	return ".words.md", "The hints flagged, to read", r.flagged
}

// flagged writes every call the judge or the code flags, and every call without a verdict.
func (r *WordsJudge) flagged(w io.Writer, logName string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# Flagged by the judge of the words: %s\n\nEvery call of the log whose verdict flags it, whose hint the code flags — the sex told by the rough search, the note taken up —, or that has no verdict; of the control phrases, every one whose four answers the judge did not give as expected, passed or not.\n", logName)
	n := 0
	for _, c := range r.log.Calls {
		v := c.Verdict
		wrong := c.Expect != nil && (v == nil || !v.Same(c.Expect.Verdict()))
		flagged := c.Expect == nil && (v == nil || v.Flagged() || len(c.Gendered) > 0 || len(c.NoteEcho) > 0)
		if !wrong && !flagged {
			continue
		}
		n++
		if c.Control != "" {
			fmt.Fprintf(&b, "\n## %s, %s, rung %d\n\n", c.Control, c.Lang, c.Rung)
			fmt.Fprintf(&b, "- expected: %s\n", answers(c.Expect.Verdict()))
		} else {
			fmt.Fprintf(&b, "\n## %s, %s, %s, %s, rung %d, sheet %d of %s, repeat %d\n\n", c.Case, c.Tutor, c.CaseKind, c.Lang, c.Rung, c.Sheet, c.Set, c.Repeat)
		}
		if c.Note != "" {
			kind := ""
			if c.NoteKind != "" {
				kind = " (" + string(c.NoteKind) + ")"
			}
			fmt.Fprintf(&b, "- note: «%s»%s\n", c.Note, kind)
		}
		fmt.Fprintf(&b, "- hint: «%s»\n", c.Hint)
		if v != nil {
			fmt.Fprintf(&b, "- judge: %s; %s\n", answers(*v), v.Why)
		} else {
			fmt.Fprintf(&b, "- judge: no verdict: %s\n", c.Error)
		}
		if len(c.Gendered) > 0 {
			fmt.Fprintf(&b, "- rough search: %s\n", strings.Join(c.Gendered, ", "))
		}
		if len(c.NoteEcho) > 0 {
			fmt.Fprintf(&b, "- takes up the note: %s\n", strings.Join(c.NoteEcho, ", "))
		}
	}
	if n == 0 {
		b.WriteString("\nNone.\n")
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// answers writes the four answers of a verdict.
func answers(v wordsjudge.Verdict) string {
	return fmt.Sprintf("gender %t, on the task %t, for a child %t, the note taken up %t", v.Gender, v.OnTask, v.ForAge, v.MentionsNote)
}
