package wordsjudge

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/altnovatrail/tutor/internal/hint"
	"github.com/altnovatrail/tutor/internal/runner"
	"github.com/altnovatrail/tutor/provider"
)

// CallKind is the kind of call of the judge of the words, as its log tells it (runner.CallKind).
const CallKind = "words-judge"

// The tokens a call is expected to take, for the estimate: the instructions and the item come to
// about input, and the four answers with a sentence, and the thinking before them, to about output.
// inputBudget is a generous count of the input, for the worst case.
const (
	input       = 1000
	output      = 400
	inputBudget = 4000
)

// ControlsFormat names the file of the control phrases, and ControlsVersion is its version.
const (
	ControlsFormat  = "tutor-words-judge-controls"
	ControlsVersion = 1
)

// Controls are the control phrases of the judge: items whose answers are known, the executor's,
// written for the measure and no learner's (T118). A judge wrong on any of them measures nothing.
type Controls struct {
	Format   string    `json:"format"`
	Version  int       `json:"version"`
	Controls []Control `json:"controls"`
}

// Control is a control phrase: its ID, the item and the answers the judge must give.
type Control struct {
	ID     string `json:"id"`
	Item   Item   `json:"item"`
	Expect Expect `json:"expect"`
}

// Expect are the four answers a judge must give.
type Expect struct {
	Gender       bool `json:"gender"`
	OnTask       bool `json:"on_task"`
	ForAge       bool `json:"for_age"`
	MentionsNote bool `json:"mentions_note"`
}

// Verdict is the verdict the answers make, with no why.
func (e Expect) Verdict() Verdict {
	return Verdict{Gender: e.Gender, OnTask: e.OnTask, ForAge: e.ForAge, MentionsNote: e.MentionsNote}
}

// ReadControls reads a file of control phrases strictly, each with an ID of its own, a hint, a
// language and a rung, and an item without a note expecting no note taken up.
func ReadControls(r io.Reader) (*Controls, error) {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	var c Controls
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("the controls: %w", err)
	}
	if dec.More() {
		return nil, errors.New("the controls: more than one value in the file")
	}
	if c.Format != ControlsFormat || c.Version != ControlsVersion || len(c.Controls) == 0 {
		return nil, fmt.Errorf("the controls: format %q version %d with %d controls, not %q version %d with some", c.Format, c.Version, len(c.Controls), ControlsFormat, ControlsVersion)
	}
	seen := map[string]bool{}
	for i, k := range c.Controls {
		switch {
		case k.ID == "" || seen[k.ID]:
			return nil, fmt.Errorf("the controls: control %d has an ID empty or given twice", i+1)
		case k.Item.Hint == "" || k.Item.Lang == "" || k.Item.Rung < 1:
			return nil, fmt.Errorf("the controls: control %s has no hint, language or rung", k.ID)
		case k.Item.Note == "" && k.Expect.MentionsNote:
			return nil, fmt.Errorf("the controls: control %s has no note, and expects it taken up", k.ID)
		}
		seen[k.ID] = true
	}
	return &c, nil
}

// step is a call of the judge: the item, and what the record tells of it.
type step struct {
	item Item
	rec  CallRecord
	// right is what a judge that is always right answers: the control's answers, or of a hint, the
	// code's measures (Of).
	right Verdict
}

// Request is the judge's request of the item.
func (s *step) Request() (provider.Request, error) { return Request(s.item) }

// Right is the answer of the judge that is always right.
func (s *step) Right() (string, error) {
	data, err := json.Marshal(s.right)
	return string(data), err
}

// Record is the call's record, with the verdict read off an answer on the schema; an answer off it
// is a failure of the call, logged with the reader's error.
func (s *step) Record(o runner.Outcome) any {
	rec := s.rec
	rec.Kind, rec.NotAsked, rec.Answer, rec.Error, rec.Usage = "call", o.NotAsked, o.Answer, o.Error, o.Usage
	if o.NotAsked || o.Error != "" {
		return rec
	}
	v, err := Read(o.Answer)
	if err != nil {
		rec.Error = err.Error()
		return rec
	}
	rec.Verdict = &v
	return rec
}

// Judge is the configuration of the model that judges.
type Judge = runner.Config

// Of plans the judging of the hints of a log of the set of the words (T118): every hint that came,
// read off an answer on the schema, once, in the order of the log, with the statement of its sheet's
// task and the note of its case. A log without cases is no log of the set, and is refused. The judge
// that is always right answers by the code's measures: the sex told when the rough search finds it,
// the note taken up when the hint echoes it, and nothing else flagged. file and sum name the log.
func Of(l *hint.Log, file, sum string, judge Judge) (*runner.Run, error) {
	if len(l.Run.Cases) == 0 {
		return nil, errors.New("wordsjudge: the log has no cases: the judge reads the hints of the set of the words")
	}
	cases := map[string]*hint.Case{}
	for i := range l.Run.Cases {
		cases[l.Run.Cases[i].ID] = &l.Run.Cases[i]
	}
	statements := map[string]string{}
	for _, sh := range l.Sheets {
		statements[fmt.Sprintf("%s/%d", sh.Set, sh.Sheet)] = sh.Statement
	}
	r := &runner.Run{}
	for _, c := range l.Calls {
		if c.NotAsked || c.Error != "" || c.Hint == "" {
			continue
		}
		k := cases[c.Case]
		if k == nil {
			return nil, fmt.Errorf("wordsjudge: a call of the case %q, which the log does not tell", c.Case)
		}
		it := Item{Hint: c.Hint, Lang: string(c.Lang), Rung: int(c.Rung), Statement: statements[fmt.Sprintf("%s/%d", c.Set, c.Sheet)], Note: k.Note}
		rec := CallRecord{Tutor: c.Tutor, Set: c.Set, Sheet: c.Sheet, Case: c.Case, CaseKind: k.Kind, NoteKind: k.NoteKind, Lang: string(c.Lang), Rung: int(c.Rung),
			Repeat: c.Repeat, Hint: c.Hint, Note: k.Note, Gendered: c.Gendered, NoteEcho: c.NoteEcho}
		right := Verdict{Gender: len(c.Gendered) > 0, OnTask: true, ForAge: true, MentionsNote: len(c.NoteEcho) > 0}
		r.Steps = append(r.Steps, runner.Step{Config: judge, In: input, Out: output, WorstIn: inputBudget, Call: &step{item: it, rec: rec, right: right}})
	}
	if len(r.Steps) == 0 {
		return nil, errors.New("wordsjudge: no hint came in the log to judge")
	}
	r.Record = record(OfLog, file, sum, judge)
	return r, nil
}

// Check plans the judging of the control phrases, each once: the judge that is always right gives
// their answers. file and sum name the file of the controls.
func Check(c *Controls, file, sum string, judge Judge) *runner.Run {
	r := &runner.Run{Record: record(OfControls, file, sum, judge)}
	for _, k := range c.Controls {
		want := k.Expect
		rec := CallRecord{Control: k.ID, Expect: &want, Lang: k.Item.Lang, Rung: k.Item.Rung, Hint: k.Item.Hint, Note: k.Item.Note}
		r.Steps = append(r.Steps, runner.Step{Config: judge, In: input, Out: output, WorstIn: inputBudget, Call: &step{item: k.Item, rec: rec, right: want.Verdict()}})
	}
	return r
}

// What a run of the judge judges: the hints of a log, or the control phrases.
const (
	OfLog      = "log"
	OfControls = "controls"
)

// record is the record of a run of the judge.
func record(of, file, sum string, judge Judge) func(h runner.Head) any {
	return func(h runner.Head) any {
		return RunRecord{Kind: "run", CallKind: CallKind, Time: h.Time, Of: of, File: file, SHA256: sum,
			Judge:  JudgeRecord{Name: judge.Name, Model: judge.Model.ID, Effort: judge.Effort, NoThinking: judge.NoThinking, InputUSD: judge.Model.Input, OutputUSD: judge.Model.Output},
			MaxUSD: h.MaxUSD, Estimate: h.Estimate, Prompt: Fingerprint(), PromptVersion: Version}
	}
}

// The records of a run's log: the run first, then each call, and the end, which is the runner's.
type (
	// RunRecord tells the run: what it judges, the hints of a log or the control phrases, the file by
	// its name and SHA-256, the judge, the cap and the estimate, and the fingerprint and version of
	// the judge's prompt.
	RunRecord struct {
		Kind          string          `json:"kind"`
		CallKind      string          `json:"call_kind"`
		Time          time.Time       `json:"time"`
		Of            string          `json:"of"`
		File          string          `json:"file"`
		SHA256        string          `json:"sha256"`
		Judge         JudgeRecord     `json:"judge"`
		MaxUSD        float64         `json:"max_usd"`
		Estimate      runner.Estimate `json:"estimate"`
		Prompt        string          `json:"prompt"`
		PromptVersion int             `json:"prompt_version"`
	}
	// JudgeRecord is the judge as the run's record tells it: its model and prices.
	JudgeRecord struct {
		Name       string  `json:"name"`
		Model      string  `json:"model"`
		Effort     string  `json:"effort"`
		NoThinking bool    `json:"no_thinking,omitempty"`
		InputUSD   float64 `json:"input_usd_per_mtok"`
		OutputUSD  float64 `json:"output_usd_per_mtok"`
	}
	// CallRecord tells a call: of a hint, its tutor, sheet, case and its kind, the kind of its note,
	// its language, rung and repeat, the hint and the note, and the code's measures of it; of a
	// control phrase, its ID and the answers expected, with its language, rung, hint and note; and the
	// judge's answer as it gave it, the verdict read off it, the failure if it failed, and what it
	// took.
	CallRecord struct {
		Kind     string        `json:"kind"`
		Tutor    string        `json:"tutor,omitempty"`
		Set      string        `json:"set,omitempty"`
		Sheet    int           `json:"sheet,omitempty"`
		Case     string        `json:"case,omitempty"`
		CaseKind hint.CaseKind `json:"case_kind,omitempty"`
		NoteKind hint.NoteKind `json:"note_kind,omitempty"`
		Control  string        `json:"control,omitempty"`
		Expect   *Expect       `json:"expect,omitempty"`
		Lang     string        `json:"lang"`
		Rung     int           `json:"rung"`
		Repeat   int           `json:"repeat,omitempty"`
		Hint     string        `json:"hint"`
		Note     string        `json:"note,omitempty"`
		Gendered []string      `json:"gendered,omitempty"`
		NoteEcho []string      `json:"note_echo,omitempty"`
		NotAsked bool          `json:"not_asked,omitempty"`
		Answer   string        `json:"answer,omitempty"`
		Error    string        `json:"error,omitempty"`
		Verdict  *Verdict      `json:"verdict,omitempty"`
		runner.Usage
	}
)

// Log is a run's log read back: the run, its calls in the order they were made, and how it ended.
// A run of the judge has no sheets.
type Log = runner.Records[RunRecord, struct{}, CallRecord]

// ReadLog reads the log of a run of the judge back, as runner.ReadRecords reads a log.
func ReadLog(r io.Reader) (*Log, error) {
	return runner.ReadRecords[RunRecord, struct{}, CallRecord](r, CallKind, "judge of the words")
}
