package report

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/altnovatrail/tutor/internal/correct"
	"github.com/altnovatrail/tutor/internal/hint"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/words"
)

// Correct is what a run of the words after a correct answer comes to (T143; docs/architecture.md,
// 3.4, "The set of the words after a correct answer"): by tutor, kind of case and language, the words
// the leak check found the key or a leak phrase in, the words that break their form (words.Correct),
// the words the learner does not read and why, the notes taken up and the learner's sex told by the
// rough search, and the time and cost of a call. It holds no handwriting: numbers, the sets and
// numbers of sheets and the words the models wrote are in the log, and only numbers here.
type Correct struct {
	log    *correct.Log
	order  []sheetID
	tutors []string
	cases  map[string]*correct.Case
	// byTutor are the calls that were asked, by tutor and by sheet.
	byTutor map[string]map[sheetID][]*correct.CallRecord

	// Cells are the calls of each tutor by kind of case and language, each tutor's followed by all
	// its calls.
	Cells []CorrectCell
}

// CorrectCell is what the calls of a tutor of a kind of case came to in a language; a cell of all
// its calls has no kind and no language. Calls are those asked, Came those whose words came, read
// off an answer on the schema, and Failed those that did not. Each share is bounded over the sheets:
//   - Leaked, of the words that came: the leak check found the key or a leak phrase, Key and Phrase
//     the words it found each in;
//   - Broken, of the words that came: they break their form, Questions, Sentences, Last, Words and
//     Script the words that break each rule;
//   - Withheld, of the calls asked: the learner reads no words, for a failure, a leak or the form;
//   - Echo, of the words that came of the cases of a note: they take the note up (hint.Echo);
//   - Gendered, of the words that came: the rough search finds the learner's sex told (hint.Gendered),
//     a signal for the judge of the words, which decides.
type CorrectCell struct {
	Tutor                                     string
	Kind                                      correct.CaseKind
	Lang                                      sample.Lang
	Calls, Came, Failed                       int
	Leaked                                    Share
	Key, Phrase                               int
	Broken                                    Share
	Questions, Sentences, Last, Words, Script int
	Withheld                                  Share
	Echo, Gendered                            Share
	TimeP50, TimeP95                          int64
	MeanUSD                                   float64
}

// All is whether the cell is of all the tutor's calls.
func (c CorrectCell) All() bool { return c.Kind == "" }

// NewCorrect builds the report of a log of the words after a correct answer. A log without tutors,
// sheets or cases has nothing to report, and a call of a sheet, a tutor or a case the log does not
// tell is a broken log.
func NewCorrect(l *correct.Log) (*Correct, error) {
	if len(l.Run.Tutors) == 0 || len(l.Sheets) == 0 || len(l.Run.Cases) == 0 {
		return nil, errors.New("report: the log has no tutors, no sheets or no cases")
	}
	r := &Correct{log: l, cases: map[string]*correct.Case{}, byTutor: map[string]map[sheetID][]*correct.CallRecord{}}
	for i := range l.Run.Cases {
		r.cases[l.Run.Cases[i].ID] = &l.Run.Cases[i]
	}
	sheets := map[sheetID]bool{}
	for _, sh := range l.Sheets {
		id := sheetID{sh.Set, sh.Sheet}
		if sheets[id] {
			return nil, fmt.Errorf("report: the sheet %s is in the log twice", id)
		}
		sheets[id] = true
		r.order = append(r.order, id)
	}
	for _, tu := range l.Run.Tutors {
		r.tutors = append(r.tutors, tu.Name)
		r.byTutor[tu.Name] = map[sheetID][]*correct.CallRecord{}
	}
	for i := range l.Calls {
		c := &l.Calls[i]
		id := sheetID{c.Set, c.Sheet}
		switch {
		case !sheets[id]:
			return nil, fmt.Errorf("report: a call of the sheet %s, which the log does not tell", id)
		case r.byTutor[c.Tutor] == nil:
			return nil, fmt.Errorf("report: a call of the tutor %q, which the log does not tell", c.Tutor)
		case r.cases[c.Case] == nil:
			return nil, fmt.Errorf("report: a call of the case %q, which the log does not tell", c.Case)
		case !c.NotAsked:
			r.byTutor[c.Tutor][id] = append(r.byTutor[c.Tutor][id], c)
		}
	}
	for _, tu := range r.tutors {
		for _, kind := range correct.CaseKinds() {
			for _, lang := range l.Run.Langs {
				r.Cells = append(r.Cells, r.cell(CorrectCell{Tutor: tu, Kind: kind, Lang: lang}))
			}
		}
		r.Cells = append(r.Cells, r.cell(CorrectCell{Tutor: tu}))
	}
	return r, nil
}

// cell fills a cell with the calls of its tutor, kind and language, or of all its calls.
func (r *Correct) cell(c CorrectCell) CorrectCell {
	var leaked, broken, withheld, echo, noteOf, gendered, cameOf, askedOf []int
	var times []int64
	spent := 0.0
	for _, id := range r.order {
		var l, b, wh, e, no, g, n, a int
		for _, call := range r.byTutor[c.Tutor][id] {
			k := r.cases[call.Case]
			if !c.All() && (k.Kind != c.Kind || k.Lang != c.Lang) {
				continue
			}
			a++
			times = append(times, call.LatencyMs)
			spent += call.CostUSD
			wh += b2i(call.Withheld != "")
			if !call.Came() {
				c.Failed++
				continue
			}
			n++
			l += b2i(call.Leaked())
			c.Key += b2i(len(call.LeakedKeys) > 0)
			c.Phrase += b2i(len(call.LeakedPhrases) > 0)
			b += b2i(len(call.Form) > 0)
			for _, br := range call.Form {
				switch words.Breach(br) {
				case words.Questions:
					c.Questions++
				case words.Sentences:
					c.Sentences++
				case words.Last:
					c.Last++
				case words.Words:
					c.Words++
				case words.Script:
					c.Script++
				}
			}
			g += b2i(len(call.Gendered) > 0)
			if k.Note != "" {
				no++
				e += b2i(len(call.NoteEcho) > 0)
			}
		}
		if a == 0 {
			continue
		}
		c.Calls, c.Came = c.Calls+a, c.Came+n
		leaked, broken, withheld, gendered = append(leaked, l), append(broken, b), append(withheld, wh), append(gendered, g)
		echo, noteOf, cameOf, askedOf = append(echo, e), append(noteOf, no), append(cameOf, n), append(askedOf, a)
	}
	c.Leaked, c.Broken, c.Withheld = share(leaked, cameOf), share(broken, cameOf), share(withheld, askedOf)
	c.Echo, c.Gendered = share(echo, noteOf), share(gendered, cameOf)
	c.TimeP50, c.TimeP95 = percentile(times, 0.50), percentile(times, 0.95)
	if c.Calls > 0 {
		c.MeanUSD = spent / float64(c.Calls)
	}
	return c
}

// where names the kind and language of a cell, "all" for a cell of all the tutor's calls.
func (c CorrectCell) where() (string, string) {
	if c.All() {
		return "all", "all"
	}
	return string(c.Kind), string(c.Lang)
}

// Markdown writes the report.
func (r *Correct) Markdown(w io.Writer, logName string) error {
	l := r.log
	var b strings.Builder
	fmt.Fprintf(&b, "# The words after a correct answer: %s\n\n", partOf(l.Run.Part))
	var tutors []string
	for _, tu := range l.Run.Tutors {
		tutors = append(tutors, configName(tu.Name, tu.Model, tu.Effort, tu.NoThinking))
	}
	made, failed, spent := tally(l.Calls, func(c *correct.CallRecord) (bool, bool, float64) { return !c.NotAsked, c.Error != "", c.CostUSD })
	fmt.Fprintf(&b, "The log %s, of the prompt %s, version %d, on the cases of %s, %d of them, sha256 %s. Tutors: %s. Repeats: %d. Sheets: %d (%s). Calls: %d made, %d of them failed; $%.2f spent of the $%.2f cap.\n",
		logName, short(l.Run.Prompt), l.Run.PromptVersion, l.Run.CasesFile, len(l.Run.Cases), short(l.Run.CasesSHA256), strings.Join(tutors, "; "), l.Run.Repeats,
		len(r.order), setsOf(l.Run.Sets, r.order), made, failed, spent, l.Run.MaxUSD)
	b.WriteString(endNote(l.End))

	b.WriteString("\n## Leaks, the form and the words withheld\n\n")
	b.WriteString("| Tutor | Case | Language | Calls | Words that came | Leaked before the check | The key | A leak phrase | Broken | Withheld |\n|---|---|---|---|---|---|---|---|---|---|\n")
	for _, c := range r.Cells {
		kind, lang := c.where()
		fmt.Fprintf(&b, "| %s | %s | %s | %d | %d | %s | %d | %d | %s | %s |\n", c.Tutor, kind, lang, c.Calls, c.Came, c.Leaked, c.Key, c.Phrase, c.Broken, c.Withheld)
	}
	b.WriteString("\nThe leak check is the program's, as api runs it before the tutor's words reach the device (the format text of spec, «The leak check»): it searches the words for the task's key by its value, unless the statement shows the key, and for its leak phrases. The learner reads no words when the call failed or its answer was off the schema, when the check found a leak, or when the words break their form: these words have no fallback text. Each share is bounded at 95 % over the sheets.\n")

	b.WriteString("\n## The form\n\n")
	b.WriteString("| Tutor | Case | Language | Words that came | Not one question | Over two sentences | The question not last | Words | Script |\n|---|---|---|---|---|---|---|---|---|\n")
	for _, c := range r.Cells {
		kind, lang := c.where()
		fmt.Fprintf(&b, "| %s | %s | %s | %d | %d | %d | %d | %d | %d |\n", c.Tutor, kind, lang, c.Came, c.Questions, c.Sentences, c.Last, c.Words, c.Script)
	}
	fmt.Fprintf(&b, "\nThe words are exactly one question, their last sentence, with no more than one sentence before it, of 1 to %d words in Russian and %d in English, in the letters of their language: option (2) of architecture 2.8, as api holds them (words.Correct).\n", words.HintWords("ru"), words.HintWords("en"))

	b.WriteString("\n## The note and the learner's sex\n\n")
	b.WriteString("| Tutor | Case | Language | Takes up the note | The sex told, rough search |\n|---|---|---|---|---|\n")
	for _, c := range r.Cells {
		kind, lang := c.where()
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", c.Tutor, kind, lang, shareOrDash(c.Echo), c.Gendered)
	}
	fmt.Fprintf(&b, "\nA case of work shows the work; one of choice shows none, as for a task whose answer slots are bound choice alone; one of a note shows a sheet with a note at the side that is no part of the solution and not for a child. The words take up the note when they have %d words of it in a row or a number of it the task has not. The rough search is that of T76, and finds the sex of a person of the task as well as the learner's: the judge of the words decides, tutor-eval judge-words.\n", hint.EchoWords)

	b.WriteString("\n## Time and cost\n\n| Tutor | Calls | Time p50, ms | Time p95, ms | Mean cost, $ |\n|---|---|---|---|---|\n")
	for _, c := range r.Cells {
		if c.All() {
			fmt.Fprintf(&b, "| %s | %d | %d | %d | %.4f |\n", c.Tutor, c.Calls, c.TimeP50, c.TimeP95, c.MeanUSD)
		}
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// CSV writes the cells, a row each, a cell of all a tutor's calls with "all" for its kind and
// language.
func (r *Correct) CSV(w io.Writer) error {
	cw := csv.NewWriter(w)
	rows := [][]string{{"tutor", "case", "lang", "calls", "came", "failed", "leaked", "leaked_upper", "key", "phrase", "broken", "broken_upper",
		"questions", "sentences", "last", "words", "script", "withheld", "withheld_upper", "echo", "echo_of", "gendered", "time_p50_ms", "time_p95_ms", "cost_usd"}}
	f := func(v float64) string { return strconv.FormatFloat(v, 'f', 4, 64) }
	for _, c := range r.Cells {
		kind, lang := c.where()
		rows = append(rows, []string{c.Tutor, kind, lang, itoa(c.Calls), itoa(c.Came), itoa(c.Failed), itoa(c.Leaked.Events), f(c.Leaked.Upper), itoa(c.Key), itoa(c.Phrase),
			itoa(c.Broken.Events), f(c.Broken.Upper), itoa(c.Questions), itoa(c.Sentences), itoa(c.Last), itoa(c.Words), itoa(c.Script),
			itoa(c.Withheld.Events), f(c.Withheld.Upper), itoa(c.Echo.Events), itoa(c.Echo.Of), itoa(c.Gendered.Events),
			strconv.FormatInt(c.TimeP50, 10), strconv.FormatInt(c.TimeP95, 10), strconv.FormatFloat(c.MeanUSD, 'f', 6, 64)})
	}
	if err := cw.WriteAll(rows); err != nil {
		return err
	}
	return cw.Error()
}

// Summary is what the report says on the terminal: for every tutor, its leaks before the check, its
// broken forms and the words withheld.
func (r *Correct) Summary() string {
	var b strings.Builder
	for _, c := range r.Cells {
		if c.All() {
			fmt.Fprintf(&b, "%s: leaked before the check %d of %d; form broken %d of %d; withheld %d of %d; the note taken up %d of %d.\n",
				c.Tutor, c.Leaked.Events, c.Came, c.Broken.Events, c.Came, c.Withheld.Events, c.Calls, c.Echo.Events, c.Echo.Of)
		}
	}
	return b.String()
}
