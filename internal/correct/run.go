package correct

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/altnovatrail/tutor/internal/hint"
	"github.com/altnovatrail/tutor/internal/leak"
	"github.com/altnovatrail/tutor/internal/reading"
	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/runner"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/sheets"
	"github.com/altnovatrail/tutor/provider"
	"github.com/altnovatrail/tutor/words"
)

// CallKind is the kind of call of the words after a correct answer, as its log tells it
// (runner.CallKind).
const CallKind = "correct"

// The input tokens a call is expected to take, for the estimate: the instructions, the schema, the
// task and its data, about inputsOfText in all, and the crop's, ⌈w / 28⌉ × ⌈h / 28⌉ (render.Tokens).
// inputBudget is a generous count of them, for the worst case: a crop takes no more than 1 568
// (render.CropOf). The output the tutors are expected to take is the hint's (hint.Tutors): the words
// are as short, and the model thinks before them as much.
const (
	inputsOfText = 1700
	inputBudget  = 8000
)

// Right reports whether a sheet is one the words are asked of: one whose answer is right.
func Right(sh sheets.Chosen) bool { return sh.Sample.Expected.Outcome == sample.OutcomeCorrect }

// Call is one call of a run: a sheet by its set and number, the language of the words, a tutor by
// its name, the repeat, from 1, and the case by its ID.
type Call struct {
	Set    string      `json:"set"`
	Sheet  int         `json:"sheet"`
	Lang   sample.Lang `json:"lang"`
	Tutor  string      `json:"tutor"`
	Repeat int         `json:"repeat"`
	Case   string      `json:"case"`
}

// prepared is a sheet of the run with its crop at 32 px, which every call with the work shows, and
// the leak check of its task, which every answer goes through.
type prepared struct {
	sheets.Chosen
	shown *reading.Sheet
	check *leak.Check
}

// step is a call of the words on the runner, with its case.
type step struct {
	Call
	sheet *prepared
	kase  *Case
}

// Request is the request of the case, as the library asks it (RequestOf): the work only if the case
// goes with it.
func (s *step) Request() (provider.Request, error) {
	t, k := s.sheet.Task, s.kase
	p := Prompt{Statement: t.Statement, Key: t.Key, Solution: t.Solution, Techniques: k.Techniques, Language: string(k.Lang), Style: hint.Style}
	if k.Work {
		p.PNG, p.Width, p.Height = s.sheet.shown.PNG, s.sheet.shown.Crop.Width, s.sheet.shown.Crop.Height
	}
	return RequestOf(p)
}

// stubWords are the words of the model that is always right, in each language: praise of a step and
// a question on the technique, with no number and nothing that tells the learner's sex, so that they
// leak nothing and keep their form.
var stubWords = map[sample.Lang]string{
	sample.LangRussian: "Решение доведено до ответа аккуратно, шаг за шагом. Какой приём помог его найти?",
	sample.LangEnglish: "The solution is carried to the answer neatly, step by step. Which technique helped to find it?",
}

// Right is the answer of the model that is always right: the stub's words in the call's language.
func (s *step) Right() (string, error) {
	data, err := json.Marshal(struct {
		Words string `json:"words"`
	}{stubWords[s.Lang]})
	return string(data), err
}

// The causes for which the learner reads no words: the call failed or its answer is off the schema,
// the leak check found the key or a leak phrase in the words, or they break their form. With a leak
// and the form broken at once, the cause is the leak. There is no fallback text of these words: api
// then gives none (attempt.proto spec, GetCorrectAnswerWordsResponse).
const (
	WithheldFailed = "failed"
	WithheldLeak   = "leak"
	WithheldForm   = "form"
)

// Record is the call's record: the answer as the model gave it and its words; and the words checked
// as api checks them before they reach the device (R70 spec): what the leak check finds in them, what
// they break of their form (words.Correct), and what the learner reads — the words, or none and why.
// An answer off the schema is a failure of the call, logged with the reader's error. A call not asked
// has none. The measures that code counts go with it: the words by which the rough search finds the
// learner's sex told (hint.Gendered), and what the words take up of a note at the side (hint.Echo).
func (s *step) Record(o runner.Outcome) any {
	rec := CallRecord{Kind: "call", Call: s.Call, NotAsked: o.NotAsked, Answer: o.Answer, Error: o.Error, Usage: o.Usage}
	if o.NotAsked {
		return rec
	}
	if o.Error != "" {
		rec.Withheld = WithheldFailed
		return rec
	}
	w, err := Read(o.Answer)
	if err != nil {
		rec.Error, rec.Withheld = err.Error(), WithheldFailed
		return rec
	}
	rec.Words = w
	rec.LeakedKeys, rec.LeakedPhrases = s.sheet.check.Split(w)
	rec.Gendered = hint.Gendered(w, s.Lang)
	rec.NoteEcho = hint.Echo(w, s.kase.Note, hint.Known(s.sheet.Chosen, s.kase.Note)...)
	for _, b := range words.Correct(w, string(s.Lang), s.sheet.Task.Statement) {
		rec.Form = append(rec.Form, string(b))
	}
	switch {
	case rec.Leaked():
		rec.Withheld = WithheldLeak
	case len(rec.Form) > 0:
		rec.Withheld = WithheldForm
	default:
		rec.Shown = w
	}
	return rec
}

// Set is the set a run asks: the file of its cases, by its name and SHA-256, and the cases it asks,
// checked against the plan of the sheets they are of (Cases.Check), and how many of the file's it
// left out.
type Set struct {
	File, SHA256 string
	Cases        *Cases
	LeftOut      int
}

// New plans a run of the words over the cases of a set, on the sheets of a part of one set, each
// with its answer right, by the tutors, repeated: it cuts the crop of every sheet at 32 px and puts
// the calls in order, repeat by repeat, case by case, tutor by tutor, so that a run cut short has the
// same repeats of every case. A sheet whose answer is not right has no such words, and is refused; so
// is a case of a sheet the part does not hold. The run tells only the sheets its cases are of. A call
// without the work takes no crop's tokens.
func New(part, set string, chosen []sheets.Chosen, tutors []hint.Tutor, repeats int, cs *Set) (*runner.Run, error) {
	switch {
	case repeats < 1:
		return nil, fmt.Errorf("correct: %d repeats", repeats)
	case cs == nil || len(cs.Cases.Cases) == 0:
		return nil, errors.New("correct: no case to ask")
	case len(chosen) == 0:
		return nil, errors.New("correct: no sheet whose answer is right, and the words after a correct answer are asked only after a right one")
	}
	by := map[int]*prepared{}
	var all []*prepared
	for _, sh := range chosen {
		switch {
		case !Right(sh):
			return nil, fmt.Errorf("correct: sheet %d of %s is %s, and the words after a correct answer are asked only after a right one", sh.Number, sh.Set, sh.Sample.Expected.Outcome)
		case sh.Set != set:
			return nil, fmt.Errorf("correct: the cases are of the sheets of one set, %s, and the run has %s", set, sh.Set)
		}
		shown, err := reading.Prepare(render.Shown(sh.Sample, math.MaxInt64), render.Digit)
		if err != nil {
			return nil, fmt.Errorf("correct: sheet %d of %s: %w", sh.Number, sh.Set, err)
		}
		p := &prepared{Chosen: sh, shown: shown, check: leak.New(hint.LeakTask(sh.Task))}
		by[sh.Number] = p
		all = append(all, p)
	}
	used := map[int]bool{}
	for _, k := range cs.Cases.Cases {
		if by[k.Sheet] == nil {
			return nil, fmt.Errorf("correct: the case %s is of sheet %d, which the run does not hold", k.ID, k.Sheet)
		}
		used[k.Sheet] = true
	}
	r := &runner.Run{}
	for _, sh := range all {
		if used[sh.Number] {
			r.Items = append(r.Items, sheetRecord(sh))
		}
	}
	var records []hint.TutorRecord
	for _, tu := range tutors {
		records = append(records, hint.TutorRecord{Name: tu.Name, Model: tu.Model.ID, Effort: tu.Effort, NoThinking: tu.NoThinking, InputUSD: tu.Model.Input, OutputUSD: tu.Model.Output})
	}
	r.Record = func(h runner.Head) any {
		return RunRecord{Kind: "run", CallKind: CallKind, Time: h.Time, Part: part, Sets: []string{set}, Tutors: records, Repeats: repeats, Langs: hint.Langs(), Style: hint.Style,
			MaxUSD: h.MaxUSD, Estimate: h.Estimate, Prompt: Fingerprint(), PromptVersion: Version,
			CasesFile: cs.File, CasesSHA256: cs.SHA256, Cases: cs.Cases.Cases, CasesLeftOut: cs.LeftOut}
	}
	for rep := 1; rep <= repeats; rep++ {
		for i := range cs.Cases.Cases {
			k := &cs.Cases.Cases[i]
			sh := by[k.Sheet]
			in := int64(inputsOfText)
			if k.Work {
				in += int64(render.Tokens(sh.shown.Crop.Width, sh.shown.Crop.Height))
			}
			for _, tu := range tutors {
				c := Call{Set: sh.Set, Sheet: sh.Number, Lang: k.Lang, Tutor: tu.Name, Repeat: rep, Case: k.ID}
				r.Steps = append(r.Steps, runner.Step{Config: tu.Config, In: in, Out: int64(tu.Output), WorstIn: inputBudget, Call: &step{Call: c, sheet: sh, kase: k}})
			}
		}
	}
	return r, nil
}

// sheetRecord is the record that tells a sheet of the run.
func sheetRecord(sh *prepared) SheetRecord {
	s, c := sh.Sample, sh.shown.Crop
	rec := SheetRecord{Kind: "sheet", Set: sh.Set, Sheet: sh.Number, File: sh.File, Writer: s.Writer, Task: s.Task, Scenario: s.Scenario,
		Outcome: s.Expected.Outcome, Key: sh.Task.Key, Statement: sh.Task.Statement, SearchedKeys: sh.check.Keys(), Phrases: sh.check.Phrases(),
		Digit: c.Digit, Drawn: c.Drawn, Width: c.Width, Height: c.Height}
	if sh.Plan != nil {
		rec.Layout = sh.Plan.Layout()
	}
	return rec
}

// The records of a run's log, a JSON object a line, each with its kind: the run first, then each
// sheet, each call, and the end, which is the runner's (runner.End). The run's record tells the
// kind of call, correct, so that tutor-eval report tells its log from the others.
type (
	// RunRecord tells the run: its part and set, its tutors and repeats, the languages and style of
	// its words, its cap and estimate, the fingerprint and the version of its prompt, and the file of
	// its cases, by its name and SHA-256, the cases it asks and how many of the file's it left out.
	RunRecord struct {
		Kind          string             `json:"kind"`
		CallKind      string             `json:"call_kind"`
		Time          time.Time          `json:"time"`
		Part          string             `json:"part"`
		Sets          []string           `json:"sets"`
		Tutors        []hint.TutorRecord `json:"tutors"`
		Repeats       int                `json:"repeats"`
		Langs         []sample.Lang      `json:"langs"`
		Style         string             `json:"style"`
		MaxUSD        float64            `json:"max_usd"`
		Estimate      runner.Estimate    `json:"estimate"`
		Prompt        string             `json:"prompt"`
		PromptVersion int                `json:"prompt_version"`
		CasesFile     string             `json:"cases_file"`
		CasesSHA256   string             `json:"cases_sha256"`
		Cases         []Case             `json:"cases"`
		CasesLeftOut  int                `json:"cases_left_out,omitempty"`
	}
	// SheetRecord tells a sheet: its set, number and file, who wrote it, its task, scenario and
	// outcome, its layout of free writing, the key, the statement, what the leak check searches the
	// words for — the key, unless the statement shows it, and apart from it the leak phrases —, and its
	// crop: the height of digits asked for and drawn at, and the size. The texts are the plan's and the
	// bank's, not handwriting.
	SheetRecord struct {
		Kind         string          `json:"kind"`
		Set          string          `json:"set"`
		Sheet        int             `json:"sheet"`
		File         string          `json:"file"`
		Writer       sample.Writer   `json:"writer,omitempty"`
		Task         string          `json:"task"`
		Scenario     sample.Scenario `json:"scenario"`
		Outcome      sample.Outcome  `json:"outcome"`
		Layout       sample.Layout   `json:"layout,omitempty"`
		Key          string          `json:"key"`
		Statement    string          `json:"statement"`
		SearchedKeys []string        `json:"searched_keys"`
		Phrases      []string        `json:"leak_phrases"`
		Digit        int             `json:"digit"`
		Drawn        int             `json:"drawn"`
		Width        int             `json:"width"`
		Height       int             `json:"height"`
	}
	// CallRecord tells a call: the answer as the model gave it, its words, the failure if it failed,
	// and what it took; the words checked: the keys and, apart, the leak phrases the leak check found
	// in them, what they break of their form, why the learner reads none, if so, and what the learner
	// reads; and the measures code counts: the words by which the rough search finds the learner's sex
	// told, and what the words take up of the note at the side. A call whose request could not be
	// built was not asked, and says so.
	CallRecord struct {
		Kind string `json:"kind"`
		Call
		NotAsked      bool     `json:"not_asked,omitempty"`
		Answer        string   `json:"answer,omitempty"`
		Words         string   `json:"words,omitempty"`
		Error         string   `json:"error,omitempty"`
		LeakedKeys    []string `json:"leaked_keys,omitempty"`
		LeakedPhrases []string `json:"leaked_phrases,omitempty"`
		Form          []string `json:"form,omitempty"`
		Withheld      string   `json:"withheld,omitempty"`
		Shown         string   `json:"shown,omitempty"`
		Gendered      []string `json:"gendered,omitempty"`
		NoteEcho      []string `json:"note_echo,omitempty"`
		runner.Usage
	}
)

// Leaked is whether the leak check found a key or a leak phrase in the call's words.
func (c *CallRecord) Leaked() bool { return len(c.LeakedKeys) > 0 || len(c.LeakedPhrases) > 0 }

// Came is whether the call's words came: it was asked and gave an answer on the schema.
func (c *CallRecord) Came() bool { return !c.NotAsked && c.Error == "" }

// Log is a run's log read back: the run, its sheets, its calls in the order they were made, and how
// the run ended, or nil when the log stops before the end.
type Log = runner.Records[RunRecord, SheetRecord, CallRecord]

// ReadLog reads the log of a run of the words after a correct answer back, as runner.ReadLog reads a
// log: every line a record of a kind it knows, the run's first and the end, if there is one, last,
// and a last line cut off half written left out. A log of another kind of call is refused
// (runner.ReadRecords).
func ReadLog(r io.Reader) (*Log, error) {
	return runner.ReadRecords[RunRecord, SheetRecord, CallRecord](r, CallKind, "words after a correct answer")
}
