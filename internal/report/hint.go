package report

import (
	"cmp"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/altnovatrail/tutor/internal/hint"
	"github.com/altnovatrail/tutor/internal/sample"
)

// Hint is what a run of the hint comes to (T75; R39; docs/architecture.md, 3.4, «Подсказка»): by
// tutor, language and rung, the hints the leak check found the answer in, before the check and after
// it, the hints that break their form, the fallback texts by cause, the box on the part of the first
// error and the model's trap against the sheet's, and the time and cost of a call; the leaks by
// task; and about 30 hints for the author to read. It holds no handwriting: numbers, the sets and
// numbers of sheets, the tasks' statements and the plan's texts, and the hints the models wrote.
type Hint struct {
	log    *hint.Log
	sheets map[sheetID]*hint.SheetRecord
	order  []sheetID
	tutors []string
	// byTutor are the calls that were asked, by tutor and by sheet.
	byTutor map[string]map[sheetID][]*hint.CallRecord

	// Cells are the calls of each tutor by language and rung, each tutor's followed by all its calls.
	Cells []HintCell
	// Tasks are the leaks by task, in the order the sheets of the log first name them.
	Tasks []TaskLeaks
}

// HintCell is what the calls of a tutor in a language at a rung came to; a cell of all its calls has
// no language and rung 0. Calls are those asked, and Came those whose hint came, read off an answer
// on the schema. Each share is bounded over the sheets:
//   - Leaked, of the hints that came: the leak check found the key or a leak phrase in it, Key and
//     Phrase the hints it found each in; After, of the calls asked: it found one in what the learner
//     reads, which is nothing by construction;
//   - Broken, of the hints that came: the hint breaks its form, Questions, Sentences, Words and Script
//     the hints that break each rule;
//   - FellBack, of the calls asked: the learner reads the fallback text, Failed, Leak and Form by its
//     cause;
//   - OnError, of the hints that came: the box hits the part of the first error; NoBox gave none;
//   - TrapRight of TrapOf: the model names the sheet's trap, on the sheets planned with one;
//     NoTrapRight of NoTrapOf: it names none, on the sheets without.
type HintCell struct {
	Tutor                                    string
	Lang                                     sample.Lang
	Rung                                     hint.Rung
	Calls, Came                              int
	Leaked                                   Share
	Key, Phrase, After                       int
	Broken                                   Share
	Questions, Sentences, Words, Script      int
	FellBack                                 Share
	Failed, Leak, Form                       int
	OnError                                  Share
	NoBox                                    int
	TrapRight, TrapOf, NoTrapRight, NoTrapOf int
	TimeP50, TimeP95                         int64
	MeanUSD                                  float64
}

// All is whether the cell is of all the tutor's calls.
func (c HintCell) All() bool { return c.Rung == 0 }

// TaskLeaks are the leaks of the hints of a task that came: whether the check searches for its key —
// a key the statement shows it does not —, how many hints came and how many leaked, and what the
// check found, each with how many hints it was found in, the most first, keys before phrases on a
// tie.
type TaskLeaks struct {
	Task        string
	KeySearched bool
	Came        int
	Leaked      int
	Found       []Found
}

// Found is a key or a leak phrase, as the task writes it, and how many hints the check found it in.
type Found struct {
	Text  string
	Key   bool
	Hints int
}

// NewHint builds the report of a log of the hint. A log without tutors or without sheets has
// nothing to report; a log of a run that did not check its hints, as T74's did not, would read as
// though nothing leaked; and a call of a sheet or a tutor the log does not tell is a broken log.
func NewHint(l *hint.Log) (*Hint, error) {
	switch {
	case len(l.Run.Tutors) == 0 || len(l.Sheets) == 0:
		return nil, errors.New("report: the log has no tutors or no sheets")
	case !l.Run.Checked:
		return nil, errors.New("report: the run did not check its hints for leaks and form, as runs before T75 did not: run it again")
	}
	r := &Hint{log: l, sheets: map[sheetID]*hint.SheetRecord{}, byTutor: map[string]map[sheetID][]*hint.CallRecord{}}
	for i := range l.Sheets {
		sh := &l.Sheets[i]
		id := sheetID{sh.Set, sh.Sheet}
		if _, ok := r.sheets[id]; ok {
			return nil, fmt.Errorf("report: the sheet %s is in the log twice", id)
		}
		r.sheets[id] = sh
		r.order = append(r.order, id)
	}
	for _, tu := range l.Run.Tutors {
		r.tutors = append(r.tutors, tu.Name)
		r.byTutor[tu.Name] = map[sheetID][]*hint.CallRecord{}
	}
	for i := range l.Calls {
		c := &l.Calls[i]
		id := sheetID{c.Set, c.Sheet}
		switch {
		case r.sheets[id] == nil:
			return nil, fmt.Errorf("report: a call of the sheet %s, which the log does not tell", id)
		case r.byTutor[c.Tutor] == nil:
			return nil, fmt.Errorf("report: a call of the tutor %q, which the log does not tell", c.Tutor)
		case !c.NotAsked:
			r.byTutor[c.Tutor][id] = append(r.byTutor[c.Tutor][id], c)
		}
	}
	for _, tu := range r.tutors {
		for _, rung := range l.Run.Rungs {
			for _, lang := range l.Run.Langs {
				r.Cells = append(r.Cells, r.cell(tu, lang, rung))
			}
		}
		r.Cells = append(r.Cells, r.cell(tu, "", 0))
	}
	r.Tasks = r.tasks()
	return r, nil
}

// came is whether a hint came: the call was asked and gave an answer on the schema.
func came(c *hint.CallRecord) bool { return !c.NotAsked && c.Error == "" }

// cell is what the calls of a tutor came to in a language at a rung; with no language and rung 0,
// all its calls.
func (r *Hint) cell(tutor string, lang sample.Lang, rung hint.Rung) HintCell {
	c := HintCell{Tutor: tutor, Lang: lang, Rung: rung}
	var leaked, broken, fell, hit, cameOf, askedOf []int
	var times []int64
	spent := 0.0
	for _, id := range r.order {
		sh := r.sheets[id]
		var l, b, f, h, n, a int
		for _, call := range r.byTutor[tutor][id] {
			if rung != 0 && (call.Rung != rung || call.Lang != lang) {
				continue
			}
			a++
			times = append(times, call.LatencyMs)
			spent += call.CostUSD
			if len(call.ShownLeaks) > 0 {
				c.After++
			}
			switch call.Fallback {
			case hint.FallbackFailed:
				c.Failed++
			case hint.FallbackLeak:
				c.Leak++
			case hint.FallbackForm:
				c.Form++
			}
			if call.Fallback != "" {
				f++
			}
			if !came(call) {
				continue
			}
			n++
			if call.Leaked() {
				l++
			}
			if len(call.LeakedKeys) > 0 {
				c.Key++
			}
			if len(call.LeakedPhrases) > 0 {
				c.Phrase++
			}
			if len(call.Form) > 0 {
				b++
			}
			for _, br := range call.Form {
				switch br {
				case hint.BreachQuestions:
					c.Questions++
				case hint.BreachSentences:
					c.Sentences++
				case hint.BreachWords:
					c.Words++
				case hint.BreachScript:
					c.Script++
				}
			}
			switch {
			case call.Box == nil:
				c.NoBox++
			case call.Hit == sh.FirstError:
				h++
			}
			if sh.Trap != "" {
				c.TrapOf++
				if call.Trap == sh.Trap {
					c.TrapRight++
				}
			} else {
				c.NoTrapOf++
				if call.Trap == "" {
					c.NoTrapRight++
				}
			}
		}
		if a == 0 {
			continue
		}
		c.Calls, c.Came = c.Calls+a, c.Came+n
		leaked, broken, fell, hit = append(leaked, l), append(broken, b), append(fell, f), append(hit, h)
		cameOf, askedOf = append(cameOf, n), append(askedOf, a)
	}
	c.Leaked, c.Broken, c.FellBack, c.OnError = share(leaked, cameOf), share(broken, cameOf), share(fell, askedOf), share(hit, cameOf)
	c.TimeP50, c.TimeP95 = percentile(times, 0.50), percentile(times, 0.95)
	if c.Calls > 0 {
		c.MeanUSD = spent / float64(c.Calls)
	}
	return c
}

// tasks are the leaks of every task, in the order the sheets of the log first name the tasks.
func (r *Hint) tasks() []TaskLeaks {
	var out []TaskLeaks
	at := map[string]int{}
	found := map[string]map[Found]int{}
	for _, id := range r.order {
		sh := r.sheets[id]
		k, ok := at[sh.Task]
		if !ok {
			k = len(out)
			at[sh.Task] = k
			out = append(out, TaskLeaks{Task: sh.Task, KeySearched: len(sh.SearchedKeys) > 0})
			found[sh.Task] = map[Found]int{}
		}
		for _, tu := range r.tutors {
			for _, call := range r.byTutor[tu][id] {
				if !came(call) {
					continue
				}
				out[k].Came++
				if call.Leaked() {
					out[k].Leaked++
				}
				for _, f := range call.LeakedKeys {
					found[sh.Task][Found{Text: f, Key: true}]++
				}
				for _, f := range call.LeakedPhrases {
					found[sh.Task][Found{Text: f}]++
				}
			}
		}
	}
	for i := range out {
		for f, n := range found[out[i].Task] {
			f.Hints = n
			out[i].Found = append(out[i].Found, f)
		}
		slices.SortFunc(out[i].Found, func(a, b Found) int {
			key := func(f Found) int {
				if f.Key {
					return 0
				}
				return 1
			}
			return cmp.Or(b.Hints-a.Hints, key(a)-key(b), strings.Compare(a.Text, b.Text))
		})
	}
	return out
}

// where names the language and rung of a cell, "all" for a cell of all the tutor's calls.
func (c HintCell) where() (string, string) {
	if c.All() {
		return "all", "all"
	}
	return string(c.Lang), strconv.Itoa(int(c.Rung))
}

// Markdown writes the report.
func (r *Hint) Markdown(w io.Writer, logName string) error {
	l := r.log
	var b strings.Builder
	fmt.Fprintf(&b, "# The hint: %s\n\n", partOf(l.Run.Part))
	var tutors []string
	for _, tu := range l.Run.Tutors {
		tutors = append(tutors, configName(tu.Name, tu.Model, tu.Effort, tu.NoThinking))
	}
	var langs, rungs []string
	for _, lang := range l.Run.Langs {
		langs = append(langs, string(lang))
	}
	for _, rung := range l.Run.Rungs {
		rungs = append(rungs, strconv.Itoa(int(rung)))
	}
	made, failed, spent := tally(l.Calls, func(c *hint.CallRecord) (bool, bool, float64) { return !c.NotAsked, c.Error != "", c.CostUSD })
	fmt.Fprintf(&b, "The log %s, of the prompt %s, version %d. Tutors: %s. Repeats: %d. Rungs: %s. Languages: %s. Sheets with a mistake: %d (%s). Calls: %d made, %d of them failed; $%.2f spent of the $%.2f cap.\n",
		logName, short(l.Run.Prompt), l.Run.PromptVersion, strings.Join(tutors, "; "), l.Run.Repeats, strings.Join(rungs, ", "), strings.Join(langs, ", "),
		len(r.order), setsOf(l.Run.Sets, r.order), made, failed, spent, l.Run.MaxUSD)
	b.WriteString(endNote(l.End))

	b.WriteString("\n## Leaks, before the check and after it, and the fallback texts\n\n")
	b.WriteString("| Tutor | Language | Rung | Calls | Hints that came | Leaked before the check | The key | A leak phrase | Leaked after the check | Fallback | Failed | For a leak | For the form |\n|---|---|---|---|---|---|---|---|---|---|---|---|---|\n")
	for _, c := range r.Cells {
		lang, rung := c.where()
		fmt.Fprintf(&b, "| %s | %s | %s | %d | %d | %s | %d | %d | %d | %s | %d | %d | %d |\n", c.Tutor, lang, rung, c.Calls, c.Came, c.Leaked, c.Key, c.Phrase, c.After,
			c.FellBack, c.Failed, c.Leak, c.Form)
	}
	b.WriteString("\nThe leak check is the program's, as api runs it before the tutor's words reach the device (the format text of spec, «The leak check»): it searches a hint for the task's key by its value, unless the statement shows the key, and for its leak phrases. A leak before the check is a hint that came in which it found either; after the check is what the learner reads — the hint, or the fallback text of the rung —, in which it finds nothing by construction. What the check does not search for, such as a number in words or the answer put another way, only a reader finds. The learner reads the fallback text when the call failed or its answer was off the schema, when the check found a leak, or when the hint breaks its form; a hint that leaks and breaks its form falls back for the leak. Each share is bounded at 95 % over the sheets.\n")

	b.WriteString("\n## The form\n\n")
	b.WriteString("| Tutor | Language | Rung | Hints that came | Broken | Not one question | Over two sentences | Words | Script |\n|---|---|---|---|---|---|---|---|---|\n")
	for _, c := range r.Cells {
		lang, rung := c.where()
		fmt.Fprintf(&b, "| %s | %s | %s | %d | %s | %d | %d | %d | %d |\n", c.Tutor, lang, rung, c.Came, c.Broken, c.Questions, c.Sentences, c.Words, c.Script)
	}
	fmt.Fprintf(&b, "\nA hint is exactly one question in no more than two sentences, of 1 to %d words, in the letters of its language: Cyrillic in Russian, where a Latin letter the statement shows may stand too, and Latin in English (R39).\n", hint.MaxWords)

	b.WriteString("\n## The place and the trap\n\n")
	b.WriteString("| Tutor | Language | Rung | Hints that came | Box on the first error | No box | The sheet's trap, on sheets with one | No trap, on sheets without |\n|---|---|---|---|---|---|---|---|\n")
	for _, c := range r.Cells {
		lang, rung := c.where()
		fmt.Fprintf(&b, "| %s | %s | %s | %d | %s | %d | %d of %d | %d of %d |\n", c.Tutor, lang, rung, c.Came, c.OnError, c.NoBox, c.TrapRight, c.TrapOf, c.NoTrapRight, c.NoTrapOf)
	}
	b.WriteString("\nThe box is on the first error when it hits the part of the first error of the markup, as the judge's box does; the trap is the one the sheet was planned with, which the call did not get.\n")

	b.WriteString("\n## Leaks by task\n\n")
	b.WriteString("| Task | Its key searched for | Hints that came | Leaked | Found |\n|---|---|---|---|---|\n")
	for _, t := range r.Tasks {
		var found []string
		for _, f := range t.Found {
			what := "the phrase"
			if f.Key {
				what = "the key"
			}
			found = append(found, fmt.Sprintf("%s «%s» %d", what, f.Text, f.Hints))
		}
		fmt.Fprintf(&b, "| %s | %s | %d | %d | %s |\n", t.Task, yes(t.KeySearched), t.Came, t.Leaked, strings.Join(found, ", "))
	}
	b.WriteString("\nA key the statement of its task shows is not searched for, and its leak phrases alone keep the answer (the format text of spec, «The leak check»).\n")

	b.WriteString("\n## Time and cost\n\n| Tutor | Calls | Time p50, ms | Time p95, ms | Mean cost, $ |\n|---|---|---|---|---|\n")
	for _, c := range r.Cells {
		if c.All() {
			fmt.Fprintf(&b, "| %s | %d | %d | %d | %.4f |\n", c.Tutor, c.Calls, c.TimeP50, c.TimeP95, c.MeanUSD)
		}
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// CSV writes the cells, a row each, a cell of all a tutor's calls with "all" for its language and
// rung.
func (r *Hint) CSV(w io.Writer) error {
	cw := csv.NewWriter(w)
	rows := [][]string{{"tutor", "lang", "rung", "calls", "came", "leaked", "leaked_upper", "key", "phrase", "leaked_after", "broken", "broken_upper",
		"questions", "sentences", "words", "script", "fallback", "fallback_upper", "failed", "fallback_leak", "fallback_form",
		"box_on_error", "box_on_error_upper", "no_box", "trap_right", "trap_of", "no_trap_right", "no_trap_of", "time_p50_ms", "time_p95_ms", "cost_usd"}}
	f := func(v float64) string { return strconv.FormatFloat(v, 'f', 4, 64) }
	for _, c := range r.Cells {
		lang, rung := c.where()
		rows = append(rows, []string{c.Tutor, lang, rung, itoa(c.Calls), itoa(c.Came), itoa(c.Leaked.Events), f(c.Leaked.Upper), itoa(c.Key), itoa(c.Phrase), itoa(c.After),
			itoa(c.Broken.Events), f(c.Broken.Upper), itoa(c.Questions), itoa(c.Sentences), itoa(c.Words), itoa(c.Script),
			itoa(c.FellBack.Events), f(c.FellBack.Upper), itoa(c.Failed), itoa(c.Leak), itoa(c.Form),
			itoa(c.OnError.Events), f(c.OnError.Upper), itoa(c.NoBox), itoa(c.TrapRight), itoa(c.TrapOf), itoa(c.NoTrapRight), itoa(c.NoTrapOf),
			strconv.FormatInt(c.TimeP50, 10), strconv.FormatInt(c.TimeP95, 10), strconv.FormatFloat(c.MeanUSD, 'f', 6, 64)})
	}
	if err := cw.WriteAll(rows); err != nil {
		return err
	}
	return cw.Error()
}

// Summary is what the report says on the terminal: for every tutor, its leaks before the check and
// after it, its broken forms, its fallback texts and its boxes on the first error.
func (r *Hint) Summary() string {
	var b strings.Builder
	for _, c := range r.Cells {
		if c.All() {
			fmt.Fprintf(&b, "%s: leaked before the check %d of %d, after it %d; form broken %d of %d; fallback %d of %d; box on the first error %d of %d.\n",
				c.Tutor, c.Leaked.Events, c.Came, c.After, c.Broken.Events, c.Came, c.FellBack.Events, c.Calls, c.OnError.Events, c.Came)
		}
	}
	return b.String()
}
