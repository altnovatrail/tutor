package report

import (
	"fmt"
	"slices"

	"github.com/altnovatrail/tutor/admission"
	"github.com/altnovatrail/tutor/internal/essay"
	"github.com/altnovatrail/tutor/internal/wordsjudge"
)

// The rules of admission of the essay: its reading, CallEssay, and the assessment of the text
// read by the rubric, CallRubric, each with a stage of its own, on a log of a run of the essays in
// two calls. Both give the stage author alone: their set is synthetic, and the stage full waits for
// the essays of adult volunteers (П-16).
const (
	// EssaySet is the set the rules of the essay admit on: the test part of eval/essays-gen.json,
	// essays 21–300, as the runner's report names it, without the dev part, the first 20.
	EssaySet = "essays-gen-test"
	// EssayWordsTarget is the most upper 95 % bound, by essay, of the words the reading read wrong.
	EssayWordsTarget = 0.01
	// RubricTarget is the most upper 95 % bound, by essay, of the false alarms on met criteria, the
	// main measure of an assessment by a rubric.
	RubricTarget = 0.01
)

// EssayHead is what both rules of the essay read of a log first: its reader, the assessor of the text
// read, its set and repeats.
type EssayHead struct {
	Reader, Assessor, Set string
	Repeats               int
}

// EssayDecision is what the rule of the reading makes of a log: the stage it admits the reader to,
// none when it admits it to nothing, and every reason why not; and, by essay, the legible essays read
// illegible, the essays with an error of the learner's read corrected, of those with such errors, and
// the words read wrong, of the essays' words, with its bound; and the readings that failed otherwise,
// with no reading of the essay, illegible or not.
type EssayDecision struct {
	Stage admission.Stage
	Why   []string
	EssayHead
	Illegible, Corrected, Words Share
	Failed                      int
}

// RubricDecision is what the rule of the rubric makes of a log and the judge's logs: the stage, none
// when it admits the assessor to nothing, and every reason why not; by essay, the false alarms on met
// criteria, with the bound the target reads, and the missed "not met"; the criteria, each with κ;
// the assessments that failed, and the judgements a valid answer left unchecked, a criterion judged
// twice or not at all; the judge, its control phrases and what it flagged of the feedback that came;
// and the feedback that came and breaks the platform's rule of feedback. The missed "not met", κ,
// the judgements unchecked and the broken rule are shown beside and have no target.
type RubricDecision struct {
	Stage admission.Stage
	Why   []string
	EssayHead
	Alarm, Missed              Share
	Criteria                   []EssaysCell
	Failed, Unchecked          int
	Judge                      string
	ControlsRight, Controls    int
	Feedbacks, Judged          int
	JudgeFailed                int
	Gender, OffTask, NotForAge WordsCount
	Broken                     int
}

// essayHead reads the head of a log for the rules of the essay, and every reason it refuses both: a
// run in one call, of more readers than one, of another set than EssaySet, one that refused a
// sample, and one that did not run to its end, since an event is one of all the repeats of an essay.
func essayHead(r *Essays) (EssayHead, []string) {
	run := r.log.Run
	h := EssayHead{Set: setName([]string{fileName(run.File)}, run.Part), Repeats: run.Repeats}
	var why []string
	if len(run.Readers) == 1 {
		h.Reader, h.Assessor = run.Readers[0].Name, run.Readers[0].Name
	} else {
		why = append(why, fmt.Sprintf("it names %d readers, not the one reader of an essay", len(run.Readers)))
	}
	if run.Assessor != nil {
		h.Assessor = run.Assessor.Name
	}
	if run.Path != essay.PathTwo {
		why = append(why, "it is not a run in two calls, the reading and the assessment of the text read")
	}
	if h.Set != EssaySet {
		why = append(why, fmt.Sprintf("its set is %s, not %s, the test part of eval/essays-gen.json", h.Set, EssaySet))
	}
	if n := len(run.Refused); n > 0 {
		why = append(why, fmt.Sprintf("it refused %d samples", n))
	}
	switch {
	case r.log.End == nil:
		why = append(why, "its log stops before the run's end")
	case r.log.End.Stopped != "":
		why = append(why, "its run stopped short: "+r.log.End.Stopped)
	}
	return h, why
}

// AdmitEssay applies the rule of the reading of an essay to a log of a run of the essays in two calls:
// one reader, in the prompt of the reading this build asks (essay.ReadFingerprint), on the
// essays code drew legible, with the refusals of essayHead; then the stage author is no legible essay
// read illegible, no reading failed otherwise — not asked, off the schema or with no answer —, so
// that every essay is measured in every repeat, no essay with an error of the learner's — a
// word spelled wrong, a sentence that begins with a small letter, the last without its mark — read
// corrected, and the upper bound of the words read wrong within EssayWordsTarget. An event is an
// essay's when any of its repeats gave it, counted once. An essay's words read wrong are the sum of
// the edit distances of its repeats, as many as its words at most: the log does not tell which word
// a repeat read wrong, and the sum is no fewer than the words read wrong in any repeat.
func AdmitEssay(r *Essays) EssayDecision {
	h, why := essayHead(r)
	d := EssayDecision{EssayHead: h, Why: why}
	if r.log.Run.ReadPrompt != essay.ReadFingerprint() {
		d.Why = append(d.Why, fmt.Sprintf("its reading is of the prompt %s, not of the reading this build asks", short(r.log.Run.ReadPrompt)))
	}
	type counts struct{ came, illegible, corrected, wrong int }
	by := map[int]*counts{}
	for i := range r.log.Calls {
		c := &r.log.Calls[i]
		if c.Step != essay.StepRead || c.Reader != h.Reader {
			continue
		}
		n := by[c.Essay]
		if n == nil {
			n = &counts{}
			by[c.Essay] = n
		}
		switch {
		case c.Illegible || c.Error == essay.ErrIllegible.Error():
			// An older log tells an essay read illegible by its error alone.
			n.came++
			n.illegible++
		case c.Reading != nil:
			n.came++
			n.wrong += c.Reading.WordErrors
			if corrected(*c.Reading) {
				n.corrected++
			}
		default:
			d.Failed++
		}
	}
	var illegible, legible, corrected, erring, wrong, words []int
	for _, number := range r.order {
		e, n := r.essays[number], by[number]
		if e.Unreadable != "" || n == nil || n.came == 0 {
			continue
		}
		illegible, legible = append(illegible, min(n.illegible, 1)), append(legible, 1)
		if e.Words > 0 && n.came > n.illegible {
			wrong, words = append(wrong, min(n.wrong, e.Words)), append(words, e.Words)
		}
		if len(e.Misspelled) > 0 || e.LowerStarts > 0 || e.Unended {
			corrected, erring = append(corrected, min(n.corrected, 1)), append(erring, 1)
		}
	}
	d.Illegible, d.Corrected, d.Words = share(illegible, legible), share(corrected, erring), share(wrong, words)
	if d.Illegible.Events > 0 {
		d.Why = append(d.Why, fmt.Sprintf("%d legible essays were read illegible", d.Illegible.Events))
	}
	if d.Failed > 0 {
		d.Why = append(d.Why, fmt.Sprintf("%d readings failed, and their essays are not measured in every repeat", d.Failed))
	}
	if d.Corrected.Events > 0 {
		d.Why = append(d.Why, fmt.Sprintf("%d essays had an error of the learner's read corrected", d.Corrected.Events))
	}
	if d.Words.Upper > EssayWordsTarget {
		d.Why = append(d.Why, fmt.Sprintf("the upper bound of the words read wrong is %.4f, above %.2f", d.Words.Upper, EssayWordsTarget))
	}
	if len(d.Why) == 0 {
		d.Stage = admission.StageAuthor
	}
	return d
}

// corrected is whether a reading corrected any error the learner wrote: a word spelled wrong, a small
// letter at the start of a sentence or the last sentence without its mark.
func corrected(rd essay.Reading) bool {
	return rd.MisspelledKept < rd.Misspelled || rd.LowerStartsKept < rd.LowerStarts || rd.Unended && (rd.UnendedKept == nil || !*rd.UnendedKept)
}

// AdmitRubric applies the rule of the assessment by the rubric to a log of a run of the essays
// in two calls, by its file's name and SHA-256, with the judge's run on its control phrases and its
// log of the log's feedback: the assessor, in the prompt of the assessment of a text this build asks
// (essay.TextFingerprint), on the text the reading of this build read (essay.ReadFingerprint), with
// the refusals of essayHead; the judge, in the prompt of this build, passed every control phrase and
// judged every feedback that came, of this log; then the stage author is no assessment
// failed — its reading failed, it was not asked, or it came off the schema —, so that every essay is
// assessed in every repeat, the upper bound of the false alarms on met criteria within
// RubricTarget, and the judge flags no feedback for telling the learner's sex, leaving the task or not
// suiting a child. The unit is the criterion of an essay: a false alarm when any repeat judged a met
// criterion unmet, counted once, of the met criteria judged at least once, the bound by essay. The
// feedback is in English alone, the language the run asks: another waits for its own measurement.
func AdmitRubric(r *Essays, name, sum string, controls, judge *WordsJudge) RubricDecision {
	h, why := essayHead(r)
	d := RubricDecision{EssayHead: h, Why: why}
	if r.log.Run.TextPrompt != essay.TextFingerprint() {
		d.Why = append(d.Why, fmt.Sprintf("its assessment is of the prompt %s, not of the assessment of a text this build asks", short(r.log.Run.TextPrompt)))
	}
	if r.log.Run.ReadPrompt != essay.ReadFingerprint() {
		d.Why = append(d.Why, fmt.Sprintf("the text it assesses is read in the prompt %s, not in the reading this build asks", short(r.log.Run.ReadPrompt)))
	}
	type judged struct{ alarm, missed, met, unmet bool }
	by := map[int]map[string]*judged{}
	for i := range r.all {
		c := &r.all[i]
		e := r.essays[c.Essay]
		if c.Reader != h.Reader || e == nil {
			continue
		}
		if by[c.Essay] == nil {
			by[c.Essay] = map[string]*judged{}
		}
		if c.Error != "" {
			d.Failed++
			continue
		}
		for _, m := range c.Marks {
			if m.Met == nil {
				d.Unchecked++
				continue
			}
			j := by[c.Essay][m.ID]
			if j == nil {
				j = &judged{}
				by[c.Essay][m.ID] = j
			}
			if slices.Contains(e.Unmet, m.ID) {
				j.unmet, j.missed = true, j.missed || *m.Met
			} else {
				j.met, j.alarm = true, j.alarm || !*m.Met
			}
		}
	}
	var alarms, met, missed, unmet []int
	for _, number := range r.order {
		var a, m, x, u int
		for _, j := range by[number] {
			a, m, x, u = a+b2i(j.alarm), m+b2i(j.met), x+b2i(j.missed), u+b2i(j.unmet)
		}
		alarms, met, missed, unmet = append(alarms, a), append(met, m), append(missed, x), append(unmet, u)
	}
	d.Alarm, d.Missed = share(alarms, met), share(missed, unmet)
	for _, rd := range r.Readers {
		if rd.Name == h.Reader {
			d.Criteria = rd.Criteria
		}
	}
	if d.Failed > 0 {
		d.Why = append(d.Why, fmt.Sprintf("%d assessments failed, and their essays are not assessed in every repeat", d.Failed))
	}
	if d.Alarm.Upper > RubricTarget {
		d.Why = append(d.Why, fmt.Sprintf("the upper bound of the false alarms on met criteria is %.4f, above %.2f", d.Alarm.Upper, RubricTarget))
	}
	d.judge(r, name, sum, controls, judge)
	if len(d.Why) == 0 {
		d.Stage = admission.StageAuthor
	}
	return d
}

// judge reads the judge's runs for the rule of the rubric, by the checks of the rule of the words
// (WordsDecision): its control phrases, and its log of this log's feedback, every feedback that came
// judged; and holds the feedback to no flag of the judge's.
func (d *RubricDecision) judge(r *Essays, name, sum string, controls, judge *WordsJudge) {
	var w WordsDecision
	cr := w.controls(controls)
	for i := range r.log.Calls {
		if c := &r.log.Calls[i]; c.Reader == d.Reader && wordsjudge.FeedbackOf(c) != "" {
			d.Feedbacks++
			d.Broken += b2i(essay.PlatformBroken(c.Breaches, c.FeedbackWords))
		}
	}
	w.judged(name, sum, judge, cr, d.Feedbacks, "feedback", "feedbacks")
	d.Judge, d.ControlsRight, d.Controls, d.JudgeFailed = w.Judge, w.ControlsRight, w.Controls, w.JudgeFailed
	d.Gender, d.OffTask, d.NotForAge, d.Judged = w.Gender, w.OffTask, w.NotForAge, w.Gender.Of
	d.Why = append(d.Why, w.Why...)
	for _, t := range []struct {
		what string
		c    WordsCount
	}{{"tell the learner's sex", d.Gender}, {"leave the task", d.OffTask}, {"are not for a child", d.NotForAge}} {
		if t.c.Events > 0 {
			d.Why = append(d.Why, fmt.Sprintf("%s feedbacks %s, by the judge", t.c, t.what))
		}
	}
}
