package hint

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/altnovatrail/tutor/internal/judge"
	"github.com/altnovatrail/tutor/internal/leak"
	"github.com/altnovatrail/tutor/internal/reading"
	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/runner"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/sheets"
	"github.com/altnovatrail/tutor/provider"
	"github.com/altnovatrail/tutor/words"
)

// CallKind is the kind of call of the hint, as its log tells it (runner.CallKind).
const CallKind = "hint"

// Tutor is a configuration of the model that words the hint: a configuration of the runner, and
// the output tokens a call is expected to take, for the estimate.
type Tutor struct {
	runner.Config
	Output int
}

// Tutors are the configurations of the runner, each with the output it is expected to take:
// the box, the trap and a hint of two short sentences come to about 200 tokens. Finding the first
// mistake, the judge of free writing thought about 100 more on Opus and about 240 on Haiku with
// thinking (docs/architecture.md, 3.4, "Hint"). The four configurations the extended measurement
// adds (R50) are estimated so: Opus at medium as at high, Sonnet with thinking between Haiku and
// Opus, without thinking as Haiku without it, and Haiku at medium above Haiku at low. A
// configuration without its output here is a mistake of the code, and Tutors panics on it rather
// than estimate its calls at nothing.
func Tutors() []Tutor {
	outputs := map[string]int{
		"opus-high": 300, "haiku-low": 500, "haiku-low-no-thinking": 200,
		"opus-medium": 300, "sonnet-low": 400, "sonnet-low-no-thinking": 200, "haiku-medium": 700,
	}
	var out []Tutor
	for _, c := range runner.Configs() {
		o, ok := outputs[c.Name]
		if !ok {
			panic("hint: no output for the configuration " + c.Name)
		}
		out = append(out, Tutor{Config: c, Output: o})
	}
	return out
}

// The input tokens a call is expected to take, for the estimate: the instructions, the schema, the
// task and its data, about inputsOfText in all, and the crop's, ⌈w / 28⌉ × ⌈h / 28⌉ (render.Tokens);
// a case of the set of the words adds the techniques, the questions asked and the trap found, about
// inputsOfCase. inputBudget is a generous count of them, for the worst case: a crop takes no more
// than 1 568 (render.CropOf).
const (
	inputsOfText = 1700
	inputsOfCase = 300
	inputBudget  = 8000
)

// Wrong reports whether a sheet is one a hint is asked of: one whose answer is not yet right. A
// hint comes only after a wrong answer (concepts 10 spec).
func Wrong(sh sheets.Chosen) bool { return sh.Sample.Expected.Outcome == sample.OutcomeError }

// Call is one call of a run: a sheet by its set and number, the rung and the language of the hint,
// a tutor by its name, and the repeat, from 1; and in a run of the set of the words, its case by
// its ID (T118).
type Call struct {
	Set    string      `json:"set"`
	Sheet  int         `json:"sheet"`
	Rung   Rung        `json:"rung"`
	Lang   sample.Lang `json:"lang"`
	Tutor  string      `json:"tutor"`
	Repeat int         `json:"repeat"`
	Case   string      `json:"case,omitempty"`
}

// prepared is a sheet of the run with its crop at 32 px, which every hint of it shows, and the leak
// check of its task, which every hint of it goes through.
type prepared struct {
	sheets.Chosen
	shown *reading.Sheet
	check *leak.Check
}

// LeakTask is a task of a bank as the leak check reads it: its statement, one paragraph, one item
// whose answer slot checks a number, the key's value (reading.KeyOf), and its leak phrases. The
// bank has no item text, line or options, so only the statement can show the key (R39).
func LeakTask(t *sample.Task) leak.Task {
	return leak.Task{
		Statement: []string{t.Statement},
		Items:     []leak.Item{{Keys: []leak.Key{{Kind: leak.Number, Value: reading.KeyOf(t.Key)}}}},
		Phrases:   t.Leaks,
	}
}

// step is a call of the hint on the runner, and in a run of the set of the words, its case.
type step struct {
	Call
	sheet *prepared
	kase  *Case
}

// Request is the hint's request of the call's sheet, rung and language; of a case, with the trap
// found, the techniques and the questions asked that the case gives, and the work only if it goes
// with it, as the library asks it (RequestOf).
func (s *step) Request() (provider.Request, error) {
	k := s.kase
	if k == nil {
		return Request(s.sheet.Task, s.sheet.shown, s.Rung, s.Lang)
	}
	t := s.sheet.Task
	p := Prompt{Statement: t.Statement, Key: t.Key, Solution: t.Solution, Traps: t.Traps, Found: k.Found, Techniques: k.Techniques,
		Rung: k.Rung, Language: string(k.Lang), Style: Style, Asked: k.Asked}
	if k.Work {
		p.PNG, p.Width, p.Height = s.sheet.shown.PNG, s.sheet.shown.Crop.Width, s.sheet.shown.Crop.Height
	}
	return RequestOf(p)
}

// known are the texts whose numbers a hint may have without taking up a note: the statement, the
// solution and the lines of the sheet but the note.
func (s *step) known(note string) []string { return Known(s.sheet.Chosen, note) }

// Known are the texts of a sheet whose numbers words may have without taking up its note (Echo): the
// statement, the solution and the lines of the sheet but the note. The words after a correct answer
// count the echo by the same texts (internal/correct).
func Known(sh sheets.Chosen, note string) []string {
	out := append([]string{sh.Task.Statement}, sh.Task.Solution...)
	for _, l := range sh.Sample.Lines {
		if l.Text != note {
			out = append(out, l.Text)
		}
	}
	return out
}

// work is whether the call's request shows the work: every call but that of a case without it.
func (s *step) work() bool { return s.kase == nil || s.kase.Work }

// stubHints are the hints of the model that is always right: words for each rung in each language,
// with no number and nothing that tells the learner's sex, so that they leak nothing.
var stubHints = map[sample.Lang]map[Rung]string{
	sample.LangRussian: {
		1: "Начало верное. Посмотри ещё раз на обведённое место: всё ли там так?",
		2: "Многое уже сделано. Какое правило здесь нужно вспомнить?",
		3: "Хорошее начало. Как решить такую же задачу с числами поменьше?",
	},
	sample.LangEnglish: {
		1: "The start is right. Look again at the marked place: is everything there as it should be?",
		2: "Much is done already. Which rule do you need to remember here?",
		3: "Good start. How would you solve the same task with smaller numbers?",
	},
}

// Right is the answer of the model that is always right, by the markup of the sheet: the box round
// the points of the part of the first error, on the crop, or none when the call shows no work, the
// trap the sheet was planned with, and the stub's words for the rung. A sheet without a first error
// has no such answer, and the stub fails on it.
func (s *step) Right() (string, error) {
	e := s.sheet.Sample.Expected
	if e.Outcome != sample.OutcomeError || e.FirstError == nil {
		return "", fmt.Errorf("hint: sheet %d of %s has no first error for the stub to box", s.Sheet, s.Set)
	}
	around := judge.Around(s.sheet.Sample, *e.FirstError)
	if around == nil {
		return "", fmt.Errorf("hint: the first error of sheet %d of %s has no points", s.Sheet, s.Set)
	}
	box := OnCrop(*around, s.sheet.shown.Crop)
	answer := struct {
		Box  *judge.Box `json:"box"`
		Trap *string    `json:"trap"`
		Hint string     `json:"hint"`
	}{Hint: stubHints[s.Lang][s.Rung]}
	if s.work() {
		answer.Box = &box
	}
	if e.Trap != "" {
		answer.Trap = &e.Trap
	}
	data, err := json.Marshal(answer)
	return string(data), err
}

// Record is the call's record: the answer as the model gave it, its box, the box's place on the
// crop in thousandths and the part of the markup it hits, its trap and its hint; and the hint
// checked as api checks it before the tutor's words reach the device (architecture 3.5 spec; R39):
// what the leak check finds in it, what it breaks of its form, and what the learner reads — the
// hint, or the fallback text of the rung and why. An answer off the schema is a failure of the
// call, logged with the reader's error, and so is the learner's fallback. A call not asked has
// none. The measures that code counts of the set of the words go with it (T118): the questions
// asked before that the hint repeats, what it takes up of a note at the side, and the words by
// which the rough search finds the learner's sex told; a box given without the work hits nothing.
func (s *step) Record(o runner.Outcome) any {
	rec := CallRecord{Kind: "call", Call: s.Call, NotAsked: o.NotAsked, Answer: o.Answer, Error: o.Error, Usage: o.Usage}
	if o.NotAsked {
		return rec
	}
	if o.Error != "" {
		return s.shown(rec, FallbackFailed)
	}
	a, err := Read(o.Answer, s.sheet.Task.Traps)
	if err != nil {
		rec.Error = err.Error()
		return s.shown(rec, FallbackFailed)
	}
	rec.Box, rec.Trap, rec.Hint = a.Box, a.Trap, a.Hint
	if a.Box != nil && s.work() {
		hit, place, ok := Hit(s.sheet.Sample, *a.Box, s.sheet.shown.Crop)
		rec.Hit = hit
		if ok {
			rec.Place = &place
		}
	}
	rec.LeakedKeys, rec.LeakedPhrases = s.sheet.check.Split(a.Hint)
	rec.Gendered = Gendered(a.Hint, s.Lang)
	if k := s.kase; k != nil {
		rec.Repeated = Repeated(a.Hint, k.Asked)
		rec.NoteEcho = Echo(a.Hint, k.Note, s.known(k.Note)...)
	}
	// Version 2 is held to the form api holds hints to, the limits of R70 spec (words.Hint).
	for _, b := range words.Hint(a.Hint, string(s.Lang), s.sheet.Task.Statement) {
		rec.Form = append(rec.Form, string(b))
	}
	switch {
	case rec.Leaked():
		return s.shown(rec, FallbackLeak)
	case len(rec.Form) > 0:
		return s.shown(rec, FallbackForm)
	}
	return s.shown(rec, "")
}

// shown completes a call's record with what the learner reads: the hint, or, for a cause, the
// fallback text of the call's rung in its language; and what the leak check finds in that, after
// the check. A hint the learner reads passed the check, and a fallback text holds no number and
// none of the tasks' phrases, so it is nothing by construction: the record shows it is.
func (s *step) shown(rec CallRecord, cause string) CallRecord {
	rec.Fallback, rec.Shown = cause, rec.Hint
	if cause != "" {
		rec.Shown = Fallback(s.Rung, s.Lang)
		rec.ShownLeaks = s.sheet.check.Find(rec.Shown)
	}
	return rec
}

// Set is the set of the words a run asks (T118): the file of its cases, by its name and SHA-256,
// and the cases it asks, checked against the plan of the sheets they are of (Cases.Check), and how
// many of the file's it left out (T119).
type Set struct {
	File, SHA256 string
	Cases        *Cases
	LeftOut      int
}

// New plans a run of the hint over the sheets of a part of one or more sets, each with a mistake, by
// the tutors, repeated: it cuts the crop of every sheet at 32 px and puts the calls in order, repeat
// by repeat, sheet by sheet, then rung by rung and language by language, so that a run cut short
// has the same repeats of every sheet. A sheet whose answer is right has no hint, and is refused.
// With a set of the words, it asks the set's cases instead, repeat by repeat, case by case, on the
// sheets of one set; a case of a sheet the part does not hold is refused.
func New(part string, sets []string, chosen []sheets.Chosen, tutors []Tutor, repeats int, set *Set) (*runner.Run, error) {
	if repeats < 1 {
		return nil, fmt.Errorf("hint: %d repeats", repeats)
	}
	if len(chosen) == 0 {
		return nil, errors.New("hint: no sheet with a mistake, and a hint is asked only after a wrong answer")
	}
	r := &runner.Run{}
	var all []*prepared
	for _, sh := range chosen {
		if !Wrong(sh) {
			return nil, fmt.Errorf("hint: sheet %d of %s is %s, and a hint is asked only after a wrong answer", sh.Number, sh.Set, sh.Sample.Expected.Outcome)
		}
		shown, err := reading.Prepare(render.Shown(sh.Sample, math.MaxInt64), render.Digit)
		if err != nil {
			return nil, fmt.Errorf("hint: sheet %d of %s: %w", sh.Number, sh.Set, err)
		}
		p := &prepared{Chosen: sh, shown: shown, check: leak.New(LeakTask(sh.Task))}
		all = append(all, p)
		r.Items = append(r.Items, sheetRecord(p))
	}
	var records []TutorRecord
	for _, tu := range tutors {
		records = append(records, TutorRecord{Name: tu.Name, Model: tu.Model.ID, Effort: tu.Effort, NoThinking: tu.NoThinking, InputUSD: tu.Model.Input, OutputUSD: tu.Model.Output})
	}
	r.Record = func(h runner.Head) any {
		rec := RunRecord{Kind: "run", CallKind: CallKind, Time: h.Time, Part: part, Sets: sets, Tutors: records, Repeats: repeats, Rungs: Rungs(), Langs: Langs(), Style: Style,
			MaxUSD: h.MaxUSD, Estimate: h.Estimate, Prompt: Fingerprint(), PromptVersion: Version, Checked: true}
		if set != nil {
			rec.CasesFile, rec.CasesSHA256, rec.Cases, rec.CasesLeftOut = set.File, set.SHA256, set.Cases.Cases, set.LeftOut
		}
		return rec
	}
	if set != nil {
		return r, plan(r, all, tutors, repeats, set.Cases)
	}
	for rep := 1; rep <= repeats; rep++ {
		for _, sh := range all {
			in := int64(inputsOfText + render.Tokens(sh.shown.Crop.Width, sh.shown.Crop.Height))
			for _, rung := range Rungs() {
				for _, lang := range Langs() {
					for _, tu := range tutors {
						c := Call{Set: sh.Set, Sheet: sh.Number, Rung: rung, Lang: lang, Tutor: tu.Name, Repeat: rep}
						r.Steps = append(r.Steps, runner.Step{Config: tu.Config, In: in, Out: int64(tu.Output), WorstIn: inputBudget, Call: &step{Call: c, sheet: sh}})
					}
				}
			}
		}
	}
	return r, nil
}

// plan puts the calls of the cases of a set of the words in order, repeat by repeat, case by case,
// tutor by tutor, on the sheets of one set: a case of a sheet the run does not hold is refused. The
// run tells only the sheets its cases are of. A call without the work takes no crop's tokens.
func plan(r *runner.Run, all []*prepared, tutors []Tutor, repeats int, cs *Cases) error {
	by := map[int]*prepared{}
	for _, sh := range all {
		if len(by) > 0 && sh.Set != all[0].Set {
			return fmt.Errorf("hint: the cases are of the sheets of one set, and the run has %s and %s", all[0].Set, sh.Set)
		}
		by[sh.Number] = sh
	}
	used := map[int]bool{}
	for i := range cs.Cases {
		if by[cs.Cases[i].Sheet] == nil {
			return fmt.Errorf("hint: the case %s is of sheet %d, which the run does not hold", cs.Cases[i].ID, cs.Cases[i].Sheet)
		}
		used[cs.Cases[i].Sheet] = true
	}
	r.Items = nil
	for _, sh := range all {
		if used[sh.Number] {
			r.Items = append(r.Items, sheetRecord(sh))
		}
	}
	for rep := 1; rep <= repeats; rep++ {
		for i := range cs.Cases {
			k := &cs.Cases[i]
			sh := by[k.Sheet]
			in := int64(inputsOfText + inputsOfCase)
			if k.Work {
				in += int64(render.Tokens(sh.shown.Crop.Width, sh.shown.Crop.Height))
			}
			for _, tu := range tutors {
				c := Call{Set: sh.Set, Sheet: sh.Number, Rung: k.Rung, Lang: k.Lang, Tutor: tu.Name, Repeat: rep, Case: k.ID}
				r.Steps = append(r.Steps, runner.Step{Config: tu.Config, In: in, Out: int64(tu.Output), WorstIn: inputBudget, Call: &step{Call: c, sheet: sh, kase: k}})
			}
		}
	}
	return nil
}

// sheetRecord is the record that tells a sheet of the run.
func sheetRecord(sh *prepared) SheetRecord {
	s, c := sh.Sample, sh.shown.Crop
	rec := SheetRecord{Kind: "sheet", Set: sh.Set, Sheet: sh.Number, File: sh.File, Writer: s.Writer, Task: s.Task, Scenario: s.Scenario,
		Outcome: s.Expected.Outcome, Trap: s.Expected.Trap, Key: sh.Task.Key, Statement: sh.Task.Statement, SearchedKeys: sh.check.Keys(), Phrases: sh.check.Phrases(),
		Digit: c.Digit, Drawn: c.Drawn, Width: c.Width, Height: c.Height}
	if fe := s.Expected.FirstError; fe != nil {
		rec.FirstError = *fe
		if *fe >= 1 && *fe <= len(s.Lines) {
			rec.FirstErrorText = s.Lines[*fe-1].Text
		}
		// Where the first error stands the plan tells only of a sheet whose parts are the plan's and
		// whose first error is the plan's, as in the judge's log (measure.sheetRecord).
		if sh.Plan != nil && len(s.Lines) == len(sh.Plan.Parts) && sh.Plan.Expected.FirstError != nil && *sh.Plan.Expected.FirstError == *fe {
			rec.FirstErrorIn = sh.Plan.Region(*fe)
		}
	}
	if sh.Plan != nil {
		rec.Layout = sh.Plan.Layout()
	}
	return rec
}

// The records of a run's log, a JSON object a line, each with its kind: the run first, then each
// sheet, each call, and the end, which is the runner's (runner.End). The run's record tells the
// kind of call, hint, so that tutor-eval report tells its log from the others.
type (
	// RunRecord tells the run: its part and sets, its tutors and repeats, the rungs, languages and
	// style of its hints, its cap and estimate, the fingerprint and the version of its prompt, and
	// that it checks every hint for leaks and its form (T75): a log of T74 does not, and its calls
	// tell nothing of what the checks would find. A run of the set of the words tells the file of
	// its cases, by its name and SHA-256, the cases it asks and how many of the file's it left out
	// (T118; T119).
	RunRecord struct {
		Kind          string          `json:"kind"`
		CallKind      string          `json:"call_kind"`
		Time          time.Time       `json:"time"`
		Part          string          `json:"part"`
		Sets          []string        `json:"sets"`
		Tutors        []TutorRecord   `json:"tutors"`
		Repeats       int             `json:"repeats"`
		Rungs         []Rung          `json:"rungs"`
		Langs         []sample.Lang   `json:"langs"`
		Style         string          `json:"style"`
		MaxUSD        float64         `json:"max_usd"`
		Estimate      runner.Estimate `json:"estimate"`
		Prompt        string          `json:"prompt"`
		PromptVersion int             `json:"prompt_version"`
		Checked       bool            `json:"checked"`
		CasesFile     string          `json:"cases_file,omitempty"`
		CasesSHA256   string          `json:"cases_sha256,omitempty"`
		Cases         []Case          `json:"cases,omitempty"`
		CasesLeftOut  int             `json:"cases_left_out,omitempty"`
	}
	// TutorRecord is a tutor as the run's record tells it: its model and prices.
	TutorRecord struct {
		Name       string  `json:"name"`
		Model      string  `json:"model"`
		Effort     string  `json:"effort"`
		NoThinking bool    `json:"no_thinking,omitempty"`
		InputUSD   float64 `json:"input_usd_per_mtok"`
		OutputUSD  float64 `json:"output_usd_per_mtok"`
	}
	// SheetRecord tells a sheet: its set, number and file, who wrote it, its task, scenario and
	// outcome, its layout of free writing, its first error, by the number of its part or line, the
	// text the plan gives it and, on a sheet of parts, where it stands, the trap the sheet was
	// planned with, which the report holds the model's trap against, the key, the statement, what
	// the leak check searches every hint for — the key, unless the statement shows it, and apart
	// from it the leak phrases —, and its crop: the height of digits asked for and drawn at, and the size. The texts
	// are the plan's and the bank's, not handwriting.
	SheetRecord struct {
		Kind           string          `json:"kind"`
		Set            string          `json:"set"`
		Sheet          int             `json:"sheet"`
		File           string          `json:"file"`
		Writer         sample.Writer   `json:"writer,omitempty"`
		Task           string          `json:"task"`
		Scenario       sample.Scenario `json:"scenario"`
		Outcome        sample.Outcome  `json:"outcome"`
		Layout         sample.Layout   `json:"layout,omitempty"`
		FirstError     int             `json:"first_error"`
		FirstErrorIn   sample.Region   `json:"first_error_in,omitempty"`
		FirstErrorText string          `json:"first_error_text,omitempty"`
		Trap           string          `json:"trap"`
		Key            string          `json:"key"`
		Statement      string          `json:"statement"`
		SearchedKeys   []string        `json:"searched_keys"`
		Phrases        []string        `json:"leak_phrases"`
		Digit          int             `json:"digit"`
		Drawn          int             `json:"drawn"`
		Width          int             `json:"width"`
		Height         int             `json:"height"`
	}
	// CallRecord tells a call: the answer as the model gave it, its box in pixels of the crop, the
	// box's place in thousandths of the crop and the part it hits, 0 for none, its trap and its hint,
	// the failure if it failed, and what it took; and its hint checked: the keys and, apart, the leak
	// phrases the leak check found in it, what it breaks of its form, why the learner reads the
	// fallback text instead, if they do, what the learner reads, and what the leak check finds in a
	// fallback text; and the measures code counts (T118): the words by which the rough search finds
	// the learner's sex told, and of a case, the questions asked before that the hint repeats and
	// what it takes up of the note at the side. A call whose request could not be built was not
	// asked, and says so: it has no fallback either.
	CallRecord struct {
		Kind string `json:"kind"`
		Call
		NotAsked      bool       `json:"not_asked,omitempty"`
		Answer        string     `json:"answer,omitempty"`
		Box           *judge.Box `json:"box,omitempty"`
		Place         *[4]int64  `json:"place,omitempty"`
		Hit           int        `json:"hit"`
		Trap          string     `json:"trap,omitempty"`
		Hint          string     `json:"hint,omitempty"`
		Error         string     `json:"error,omitempty"`
		LeakedKeys    []string   `json:"leaked_keys,omitempty"`
		LeakedPhrases []string   `json:"leaked_phrases,omitempty"`
		Form          []string   `json:"form,omitempty"`
		Fallback      string     `json:"fallback,omitempty"`
		Shown         string     `json:"shown,omitempty"`
		ShownLeaks    []string   `json:"shown_leaks,omitempty"`
		Gendered      []string   `json:"gendered,omitempty"`
		Repeated      []string   `json:"repeated,omitempty"`
		NoteEcho      []string   `json:"note_echo,omitempty"`
		runner.Usage
	}
)

// Leaked is whether the leak check found a key or a leak phrase in the call's hint.
func (c *CallRecord) Leaked() bool { return len(c.LeakedKeys) > 0 || len(c.LeakedPhrases) > 0 }

// Log is a run's log read back: the run, its sheets, its calls in the order they were made, and how
// the run ended, or nil when the log stops before the end.
type Log = runner.Records[RunRecord, SheetRecord, CallRecord]

// ReadLog reads the log of a run of the hint back, as runner.ReadLog reads a log: every line a
// record of a kind it knows, the run's first and the end, if there is one, last, and a last line
// cut off half written left out. A log of another kind of call is refused (runner.ReadRecords).
func ReadLog(r io.Reader) (*Log, error) {
	return runner.ReadRecords[RunRecord, SheetRecord, CallRecord](r, CallKind, "hint")
}
