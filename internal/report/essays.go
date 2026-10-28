package report

import (
	"encoding/csv"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/altnovatrail/tutor/internal/essay"
)

// Essays is the report of a run of the assessment by the rubric (T83; docs/architecture.md, 3.4,
// «Критерий сочинений»): the measure of О-22 for every reader — the criteria judged unmet that the
// essay meets, the main one, and the agreement with the planned truth, criterion by criterion, with
// Cohen's κ — and how the reader read the essays and kept the rule of the feedback. It has no
// threshold: the measure has no target yet (О-22).
type Essays struct {
	log    *essay.Log
	essays map[int]*essay.EssayRecord
	order  []int
	// Readers are the readers' measures, in the order of the run.
	Readers []EssaysReader
}

// EssaysReader is a reader's measures over every call of the run.
type EssaysReader struct {
	Name          string
	Calls, Failed int
	MeanUSD       float64
	// All is every criterion together, and Criteria each criterion of the rubric, in its order.
	All      EssaysCell
	Criteria []EssaysCell
	// Paired are the criteria of an essay judged in two repeats or more, and Unstable those of them
	// the repeats judge both ways.
	Paired, Unstable int
	Reading          essay.Reading
	// Feedbacks are the calls with feedback, Breaches how many of them break each part of its rule,
	// PlatformBroken how many break the rule R70 spec sets, with its 80 words (essay.PlatformBroken),
	// and MeanWords and MostWords its words.
	Feedbacks      int
	Breaches       map[string]int
	PlatformBroken int
	MeanWords      float64
	MostWords      int
}

// EssaysCell is a reader's judgements of a criterion, or of all of them: the false alarms among the
// judgements of criteria the essay meets, the missed ones among those of criteria it does not, both
// with their bounds over the essays; the judgements that agree with the truth, of all judged; κ, and
// whether it is defined; and the criteria left unchecked.
type EssaysCell struct {
	Criterion  string
	FalseAlarm Share
	Missed     Share
	Agree      int
	Judged     int
	Kappa      float64
	KappaDef   bool
	Unchecked  int
	trueMet    int
	saidMet    int
	bothMet    int
	bothUnmet  int
	alarmsBy   map[int]int
	metBy      map[int]int
	missedBy   map[int]int
	unmetBy    map[int]int
}

// AllCriteria is the cell of every criterion together.
const AllCriteria = "all"

// NewEssays builds the report of a log of the assessment by the rubric.
func NewEssays(l *essay.Log) (*Essays, error) {
	r := &Essays{log: l, essays: map[int]*essay.EssayRecord{}}
	for i := range l.Sheets {
		e := &l.Sheets[i]
		r.essays[e.Essay] = e
		r.order = append(r.order, e.Essay)
	}
	for _, rd := range l.Run.Readers {
		r.Readers = append(r.Readers, r.reader(rd.Name))
	}
	return r, nil
}

// calls are a reader's calls, in the order of the run.
func (r *Essays) calls(reader string) []*essay.CallRecord {
	var out []*essay.CallRecord
	for i := range r.log.Calls {
		if c := &r.log.Calls[i]; c.Reader == reader {
			out = append(out, c)
		}
	}
	return out
}

func newCell(criterion string) EssaysCell {
	return EssaysCell{Criterion: criterion, alarmsBy: map[int]int{}, metBy: map[int]int{}, missedBy: map[int]int{}, unmetBy: map[int]int{}}
}

func (r *Essays) reader(name string) EssaysReader {
	rd := EssaysReader{Name: name, All: newCell(AllCriteria), Breaches: map[string]int{}}
	byID := map[string]*EssaysCell{}
	for _, c := range r.log.Run.Rubric {
		rd.Criteria = append(rd.Criteria, newCell(c.ID))
	}
	for i := range rd.Criteria {
		byID[rd.Criteria[i].Criterion] = &rd.Criteria[i]
	}
	times := map[[2]int]int{}          // essay and criterion → how many times judged
	ways := map[[2]int]map[bool]bool{} // essay and criterion → the judgements given
	words, spent := 0, 0.0
	for _, c := range r.calls(name) {
		e := r.essays[c.Essay]
		if e == nil || c.NotAsked {
			continue
		}
		rd.Calls++
		spent += c.CostUSD
		if c.Error != "" {
			rd.Failed++
		}
		for k, m := range c.Marks {
			truth := !slices.Contains(e.Unmet, m.ID)
			for _, cell := range []*EssaysCell{&rd.All, byID[m.ID]} {
				if cell != nil {
					cell.add(e.Essay, truth, m.Met)
				}
			}
			if m.Met != nil {
				key := [2]int{c.Essay, k}
				if ways[key] == nil {
					ways[key] = map[bool]bool{}
				}
				times[key]++
				ways[key][*m.Met] = true
			}
		}
		if c.Reading != nil {
			addReading(&rd.Reading, *c.Reading)
		}
		if c.Error == "" && c.Reading != nil {
			rd.Feedbacks++
			words += c.FeedbackWords
			rd.MostWords = max(rd.MostWords, c.FeedbackWords)
			if essay.PlatformBroken(c.Breaches, c.FeedbackWords) {
				rd.PlatformBroken++
			}
			for _, b := range c.Breaches {
				rd.Breaches[b]++
			}
		}
	}
	for key, n := range times {
		if n > 1 {
			rd.Paired++
			if len(ways[key]) == 2 {
				rd.Unstable++
			}
		}
	}
	if rd.Calls > 0 {
		rd.MeanUSD = spent / float64(rd.Calls)
	}
	if rd.Feedbacks > 0 {
		rd.MeanWords = float64(words) / float64(rd.Feedbacks)
	}
	rd.All.finish(r.order)
	for i := range rd.Criteria {
		rd.Criteria[i].finish(r.order)
	}
	return rd
}

// add counts a judgement of a criterion of an essay against its truth: met or not.
func (c *EssaysCell) add(essayNumber int, truth bool, said *bool) {
	if said == nil {
		c.Unchecked++
		return
	}
	c.Judged++
	if truth {
		c.trueMet++
		c.metBy[essayNumber]++
	} else {
		c.unmetBy[essayNumber]++
	}
	if *said {
		c.saidMet++
	}
	switch {
	case truth && *said:
		c.bothMet++
	case !truth && !*said:
		c.bothUnmet++
	case truth:
		c.alarmsBy[essayNumber]++
	default:
		c.missedBy[essayNumber]++
	}
}

// finish sums the cell up: the shares with their bounds over the essays, the agreement and κ.
func (c *EssaysCell) finish(order []int) {
	var alarms, met, missed, unmet []int
	for _, n := range order {
		alarms, met = append(alarms, c.alarmsBy[n]), append(met, c.metBy[n])
		missed, unmet = append(missed, c.missedBy[n]), append(unmet, c.unmetBy[n])
	}
	c.FalseAlarm, c.Missed = share(alarms, met), share(missed, unmet)
	c.Agree = c.bothMet + c.bothUnmet
	c.Kappa, c.KappaDef = kappa(c.Judged, c.Agree, c.trueMet, c.saidMet)
}

// kappa is Cohen's κ of n judgements against the truth, of which agree agree, with the truth met in
// trueMet and the judgement met in saidMet: (po − pe) / (1 − pe). It is not defined when pe is 1, as
// when the truth and the judgements are all of one kind, or when there are none.
func kappa(n, agree, trueMet, saidMet int) (float64, bool) {
	if n == 0 {
		return 0, false
	}
	nf := float64(n)
	po := float64(agree) / nf
	t, s := float64(trueMet)/nf, float64(saidMet)/nf
	pe := t*s + (1-t)*(1-s)
	if pe >= 1 {
		return 0, false
	}
	return (po - pe) / (1 - pe), true
}

// addReading adds the counts of a call's reading of an essay to a reader's: the last sentence
// without its mark is counted by call, unended.
func addReading(to *essay.Reading, r essay.Reading) {
	to.Words += r.Words
	to.WordErrors += r.WordErrors
	to.Misspelled += r.Misspelled
	to.MisspelledKept += r.MisspelledKept
	to.LowerStarts += r.LowerStarts
	to.LowerStartsKept += r.LowerStartsKept
}

// kappaText writes κ, or "—" where it is not defined.
func (c EssaysCell) kappaText() string {
	if !c.KappaDef {
		return "—"
	}
	return strconv.FormatFloat(c.Kappa, 'f', 2, 64)
}

// Markdown writes the report.
func (r *Essays) Markdown(w io.Writer, logName string) error {
	l := r.log
	var b strings.Builder
	fmt.Fprintf(&b, "# The essays: %s\n\n", partOf(l.Run.Part))
	var readers []string
	for _, rd := range l.Run.Readers {
		readers = append(readers, configName(rd.Name, rd.Model, rd.Effort, rd.NoThinking))
	}
	var nums []string
	for _, n := range r.order {
		nums = append(nums, strconv.Itoa(n))
	}
	made, failed, spent := tally(l.Calls, func(c *essay.CallRecord) (bool, bool, float64) { return !c.NotAsked, c.Error != "", c.CostUSD })
	fmt.Fprintf(&b, "The log %s, of the prompt %s, version %d. The essays of %s, drawn with the seed %d: %s. Readers: %s. Repeats: %d. Calls: %d made, %d of them failed; $%.2f spent of the $%.2f cap.\n",
		logName, short(l.Run.Prompt), l.Run.PromptVersion, l.Run.File, l.Run.Seed, strings.Join(nums, ", "), strings.Join(readers, "; "), l.Run.Repeats, made, failed, spent, l.Run.MaxUSD)
	b.WriteString(endNote(l.End))

	b.WriteString("\n## The criteria\n\n| Reader | Criterion | Judged | False alarms | Missed \"unmet\" | Agree | κ | Unchecked |\n|---|---|---|---|---|---|---|---|\n")
	for _, rd := range r.Readers {
		for _, c := range append([]EssaysCell{rd.All}, rd.Criteria...) {
			fmt.Fprintf(&b, "| %s | %s | %d | %s | %s | %d | %s | %d |\n", rd.Name, c.Criterion, c.Judged, c.FalseAlarm, c.Missed, c.Agree, c.kappaText(), c.Unchecked)
		}
	}
	b.WriteString("\nA false alarm is a criterion judged unmet that the essay meets, counted among the judgements of criteria the essay meets: the main measure of О-22. A missed \"unmet\" is a criterion judged met that the essay does not meet, counted among the judgements of criteria it does not meet. The truth is the plan's. κ is Cohen's, over every judgement of the criterion, in every repeat. A criterion is unchecked when its call failed, or the answer judges it not once or more than once. A share's bound is the one-sided upper bound of 95 % over the essays, Clopper–Pearson on n / Deff.\n")

	b.WriteString("\n## Repeats, reading and feedback\n\n| Reader | Calls | Failed | Criteria judged both ways | Word errors | Misspellings kept | Small starts kept | Unended kept | Feedbacks | Sentences | Words | Quote | Script | Broken by R70 spec | Words, mean | Words, most | Cost, $ |\n|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|\n")
	for _, rd := range r.Readers {
		fmt.Fprintf(&b, "| %s | %d | %d | %d of %d | %d of %d | %d of %d | %d of %d | %s | %d | %d | %d | %d | %d | %d | %.1f | %d | %.5f |\n",
			rd.Name, rd.Calls, rd.Failed, rd.Unstable, rd.Paired, rd.Reading.WordErrors, rd.Reading.Words, rd.Reading.MisspelledKept, rd.Reading.Misspelled,
			rd.Reading.LowerStartsKept, rd.Reading.LowerStarts, r.unended(rd.Name), rd.Feedbacks,
			rd.Breaches[essay.BreachSentences], rd.Breaches[essay.BreachWords], rd.Breaches[essay.BreachQuote], rd.Breaches[essay.BreachScript], rd.PlatformBroken, rd.MeanWords, rd.MostWords, rd.MeanUSD)
	}
	fmt.Fprintf(&b, "\nCriteria judged both ways: of the criteria of an essay judged in two repeats or more, those judged met in one and unmet in another. Word errors: the edit distance in words between the essay as written, its title and text, and the text the model read, in small letters and without the marks around the words, of the essay's words. Kept: what the learner wrote wrong that the model read as it is written — the words spelled wrong, the sentences that begin with a small letter and the last sentence without its mark. The feedback's rule (R05 spec): at most %d sentences and %d words, and a quoted span of %d words or more only as the essay writes it; the columns count the feedbacks that break each part, and a feedback in another script than Latin breaks the last. R70 spec sets the same rule with %d words, and «Broken by R70 spec» counts the feedbacks that break it.\n",
		essay.MaxSentences, essay.MaxWords, essay.QuoteWords, essay.PlatformWords)
	_, err := io.WriteString(w, b.String())
	return err
}

// unended writes how many of a reader's readings of an essay whose last sentence ends without its
// mark kept it so, of them.
func (r *Essays) unended(reader string) string {
	kept, of := 0, 0
	for _, c := range r.calls(reader) {
		if c.Reading != nil && c.Reading.Unended {
			of++
			if c.Reading.UnendedKept != nil && *c.Reading.UnendedKept {
				kept++
			}
		}
	}
	return fmt.Sprintf("%d of %d", kept, of)
}

// CSV writes every judgement of a criterion, a row each: the essay, its part, the reader, the
// repeat, the criterion, the truth and the judgement, met, unmet or unchecked with its reason.
func (r *Essays) CSV(w io.Writer) error {
	cw := csv.NewWriter(w)
	rows := [][]string{{"essay", "part", "reader", "repeat", "criterion", "truth", "judged", "reason"}}
	metText := map[bool]string{true: "met", false: "unmet"}
	for _, c := range r.log.Calls {
		e := r.essays[c.Essay]
		if e == nil || c.NotAsked {
			continue
		}
		for _, m := range c.Marks {
			judged := "unchecked"
			if m.Met != nil {
				judged = metText[*m.Met]
			}
			rows = append(rows, []string{itoa(c.Essay), e.Part, c.Reader, itoa(c.Repeat), m.ID, metText[!slices.Contains(e.Unmet, m.ID)], judged, m.Reason})
		}
	}
	if err := cw.WriteAll(rows); err != nil {
		return err
	}
	return cw.Error()
}

// Summary is what the report says on the terminal: for every reader, the false alarms and the missed
// "unmet" of all the criteria, κ, the word errors of the reading and the feedbacks that break the
// rule.
func (r *Essays) Summary() string {
	var b strings.Builder
	for _, rd := range r.Readers {
		broke := 0
		for _, c := range r.calls(rd.Name) {
			if len(c.Breaches) > 0 {
				broke++
			}
		}
		fmt.Fprintf(&b, "%s: false alarms %s, missed \"unmet\" %s, κ %s; word errors %d of %d; feedbacks breaking the rule %d of %d, that of R70 spec, with %d words, %d.\n",
			rd.Name, rd.All.FalseAlarm, rd.All.Missed, rd.All.kappaText(), rd.Reading.WordErrors, rd.Reading.Words, broke, rd.Feedbacks, essay.PlatformWords, rd.PlatformBroken)
	}
	return b.String()
}

// Readable names the feedback for the author to read: the third file of the report, .feedback.md.
func (r *Essays) Readable() (suffix, label string, write func(io.Writer, string) error) {
	return ".feedback.md", "The feedback to read", r.Feedback
}

// Feedback writes the feedback for the author to read: of the first repeat, by the first reader of
// the run, Opus in the measurement, every essay with the criteria it does not meet by the plan, those
// the reader judged otherwise, the feedback as the model wrote it and what it breaks of the rule.
func (r *Essays) Feedback(w io.Writer, logName string) error {
	var b strings.Builder
	if len(r.log.Run.Readers) == 0 {
		_, err := io.WriteString(w, "# The feedback\n\nThe run has no reader.\n")
		return err
	}
	reader := r.log.Run.Readers[0].Name
	fmt.Fprintf(&b, "# The feedback of %s\n\nThe log %s: the first repeat of every essay of %s. The planned truth is the file of the essays (docs/essays.md).\n", reader, logName, partOf(r.log.Run.Part))
	for _, c := range r.calls(reader) {
		if c.Repeat != 1 {
			continue
		}
		e := r.essays[c.Essay]
		if e == nil {
			continue
		}
		fmt.Fprintf(&b, "\n## Essay %d\n\n", c.Essay)
		unmet := "none"
		if len(e.Unmet) > 0 {
			unmet = strings.Join(e.Unmet, ", ")
		}
		fmt.Fprintf(&b, "- Unmet by the plan: %s.\n", unmet)
		var other []string
		for _, m := range c.Marks {
			truth := !slices.Contains(e.Unmet, m.ID)
			switch {
			case m.Met == nil:
				other = append(other, m.ID+" unchecked")
			case *m.Met != truth:
				other = append(other, fmt.Sprintf("%s judged %s", m.ID, map[bool]string{true: "met", false: "unmet"}[*m.Met]))
			}
		}
		if len(other) == 0 {
			other = []string{"none"}
		}
		fmt.Fprintf(&b, "- Judged otherwise: %s.\n", strings.Join(other, ", "))
		if c.Error != "" || c.Reading == nil {
			fmt.Fprintf(&b, "- The call failed: %s\n", c.Error)
			continue
		}
		breaches := "none"
		if len(c.Breaches) > 0 {
			breaches = strings.Join(c.Breaches, ", ")
		}
		fmt.Fprintf(&b, "- Breaks of the rule: %s.\n\n> %s\n", breaches, strings.ReplaceAll(feedbackOf(c.Answer), "\n", "\n> "))
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// feedbackOf is the feedback of an answer as the model wrote it, or nothing when the answer is not
// the schema's.
func feedbackOf(answer string) string {
	a, err := essay.Read(answer)
	if err != nil {
		return ""
	}
	return a.Feedback
}
