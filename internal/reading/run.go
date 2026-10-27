package reading

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/altnovatrail/tutor/internal/runner"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/sheets"
	"github.com/altnovatrail/tutor/provider"
)

// CallKind is the kind of call of the reading, as its log tells it (runner.CallKind).
const CallKind = "reading"

// Reader is a configuration of the reader: a configuration of the runner, the output tokens a call
// is expected to take, for the estimate, and whether it reads once whatever the repeats, as Opus
// does in the reduced matrix (R08; R28).
type Reader struct {
	runner.Config
	Output int
	Once   bool
}

// Readers are the three readers, the three configurations of the runner by their names. An
// observation of a sheet comes to about 200 tokens, and Haiku with thinking writes about 300 more;
// Opus reads each condition once (docs/architecture.md, 3.4). A configuration without its reader
// here is a mistake of the code, and Readers panics on it rather than estimate its calls at
// nothing or repeat it unasked.
func Readers() []Reader {
	readers := map[string]struct {
		output int
		once   bool
	}{"opus-high": {200, true}, "haiku-low": {500, false}, "haiku-low-no-thinking": {200, false}}
	var out []Reader
	for _, c := range runner.Configs() {
		rd, ok := readers[c.Name]
		if !ok {
			panic("reading: no reader for the configuration " + c.Name)
		}
		out = append(out, Reader{Config: c, Output: rd.output, Once: rd.once})
	}
	return out
}

// FalseKeyed reports whether a sheet is read with false keys: a correct sheet whose task has them.
func FalseKeyed(sh sheets.Chosen) bool {
	return sh.Sample.Expected.Outcome == sample.OutcomeCorrect && len(sh.Task.FalseKeys) > 0
}

// The input tokens a call is expected to take, for the estimate: the instructions, the schema, the
// picture of 1000 × 600, 792 tokens, and the task, about 2 000 in all, and about 150 more with the
// block of the key. inputBudget is a generous count of them, for the worst case.
const (
	inputs      = 2000
	inputsOfKey = 150
	inputBudget = 8000
)

// Call is one call of a run: a sheet by its set and number, a condition, the key given in it as the
// bank writes it, a reader by its name, and the repeat, from 1.
type Call struct {
	Set       string    `json:"set"`
	Sheet     int       `json:"sheet"`
	Condition Condition `json:"condition"`
	Key       string    `json:"key,omitempty"`
	Reader    string    `json:"reader"`
	Repeat    int       `json:"repeat"`
}

// prepared is a sheet of the run with its picture and its markup.
type prepared struct {
	sheets.Chosen
	shown  *Sheet
	marked Marked
}

// step is a call of the reading on the runner: a sheet in a condition, by a reader, in a repeat.
type step struct {
	Call
	sheet *prepared
	given *Given
}

// Request is the reading's request of the sheet in the call's condition.
func (s *step) Request() (provider.Request, error) {
	return Request(s.sheet.Task, s.sheet.shown, s.given), nil
}

// Right is the observation of the reader who is always right, by the markup.
func (s *step) Right() (string, error) {
	data, err := json.Marshal(Right(s.sheet.marked))
	return string(data), err
}

// Record is the call's record: the answer as the model gave it, the verdict on it against the
// task's key, with its reason and the value of the answer, and what was read under the key given.
// Without a key, what was read under the right key is told too: how often the reader writes the
// right solution where the sheet has something else, without being given it. An answer off the
// schema is a failure of the reading, and its verdict is unchecked.
func (s *step) Record(o runner.Outcome) any {
	rec := CallRecord{Kind: "call", Call: s.Call, NotAsked: o.NotAsked, Answer: o.Answer, Error: o.Error, Usage: o.Usage}
	var obs *Observation
	if !o.NotAsked && o.Error == "" {
		var err error
		if obs, err = Read(o.Answer); err != nil {
			rec.Error = err.Error()
		}
	}
	verdict, reason := Verdict(obs, KeyOf(s.sheet.Task.Key))
	rec.Verdict, rec.Reason = string(verdict), reason
	if obs != nil && len(obs.Results) == 1 {
		rec.Value = obs.Results[0].Value
	}
	given := s.given
	if given == nil {
		given = &Given{Key: s.sheet.Task.Key, Solution: s.sheet.Task.Solution}
	}
	rec.UnderKey = UnderKey(obs, s.sheet.marked, *given)
	return rec
}

// New plans a run of the reading over the sheets of a part of one or more sets, by the readers,
// repeated: it draws the picture of every sheet and puts the calls in order, repeat by repeat, so
// that a run cut short has the same repeats of every sheet. Of each sheet, by each reader, it asks
// without a key and with the right key, and on a correct sheet with each false key of its task
// (R03). A reader that reads once asks only in the first repeat.
func New(part string, sets []string, chosen []sheets.Chosen, readers []Reader, repeats int) (*runner.Run, error) {
	if repeats < 1 {
		return nil, fmt.Errorf("reading: %d repeats", repeats)
	}
	r := &runner.Run{}
	var all []*prepared
	for _, sh := range chosen {
		shown, err := Prepare(sh.Sample)
		if err != nil {
			return nil, fmt.Errorf("reading: sheet %d of %s: %w", sh.Number, sh.Set, err)
		}
		p := &prepared{Chosen: sh, shown: shown, marked: MarkupOf(sh.Sample, sh.Plan)}
		all = append(all, p)
		r.Items = append(r.Items, sheetRecord(p))
	}
	var records []ReaderRecord
	for _, rd := range readers {
		records = append(records, ReaderRecord{Name: rd.Name, Model: rd.Model.ID, Effort: rd.Effort, NoThinking: rd.NoThinking, Once: rd.Once, InputUSD: rd.Model.Input, OutputUSD: rd.Model.Output})
	}
	r.Record = func(h runner.Head) any {
		return RunRecord{Kind: "run", CallKind: CallKind, Time: h.Time, Part: part, Sets: sets, Readers: records, Repeats: repeats, MaxUSD: h.MaxUSD, Estimate: h.Estimate,
			Prompt: Fingerprint(), PromptVersion: Version}
	}
	for rep := 1; rep <= repeats; rep++ {
		for _, sh := range all {
			for _, rd := range readers {
				if rd.Once && rep > 1 {
					continue
				}
				for _, g := range conditions(sh) {
					c := Call{Set: sh.Set, Sheet: sh.Number, Condition: g.condition, Reader: rd.Name, Repeat: rep}
					in := int64(inputs)
					if g.given != nil {
						c.Key, in = g.given.Key, inputs+inputsOfKey
					}
					r.Steps = append(r.Steps, runner.Step{Config: rd.Config, In: in, Out: int64(rd.Output), WorstIn: inputBudget, Call: &step{Call: c, sheet: sh, given: g.given}})
				}
			}
		}
	}
	return r, nil
}

// condition is a condition of a sheet with the key and solution it gives.
type condition struct {
	condition Condition
	given     *Given
}

// conditions are the conditions a sheet is read in: without a key, with the right key, and on a
// correct sheet with each false key of its task.
func conditions(sh *prepared) []condition {
	t := sh.Task
	out := []condition{{NoKey, nil}, {RightKey, &Given{Key: t.Key, Solution: t.Solution}}}
	if FalseKeyed(sh.Chosen) {
		for _, f := range t.FalseKeys {
			out = append(out, condition{FalseKey, &Given{Key: f.Key, Solution: f.Solution}})
		}
	}
	return out
}

// sheetRecord is the record that tells a sheet of the run.
func sheetRecord(sh *prepared) SheetRecord {
	rec := SheetRecord{Kind: "sheet", Set: sh.Set, Sheet: sh.Number, File: sh.File, Writer: sh.Sample.Writer, Task: sh.Sample.Task, Scenario: sh.Sample.Scenario,
		Outcome: sh.Sample.Expected.Outcome, Key: KeyOf(sh.Task.Key), FalseKeys: []string{}, Markup: sh.marked}
	for _, f := range sh.Task.FalseKeys {
		rec.FalseKeys = append(rec.FalseKeys, KeyOf(f.Key))
	}
	if sh.Plan != nil {
		rec.Layout = sh.Plan.Layout()
	}
	return rec
}

// The records of a run's log, a JSON object a line, each with its kind: the run first, then each
// sheet, each call, and the end, which is the runner's (runner.End). The run's record tells the
// kind of call, reading, so that tutor-eval report tells its log from the judge's.
type (
	// RunRecord tells the run: its part and sets, its readers and repeats, its cap and estimate,
	// and the fingerprint and the version of its prompt.
	RunRecord struct {
		Kind          string          `json:"kind"`
		CallKind      string          `json:"call_kind"`
		Time          time.Time       `json:"time"`
		Part          string          `json:"part"`
		Sets          []string        `json:"sets"`
		Readers       []ReaderRecord  `json:"readers"`
		Repeats       int             `json:"repeats"`
		MaxUSD        float64         `json:"max_usd"`
		Estimate      runner.Estimate `json:"estimate"`
		Prompt        string          `json:"prompt"`
		PromptVersion int             `json:"prompt_version"`
	}
	// ReaderRecord is a reader as the run's record tells it: its model and prices, and whether it
	// reads once.
	ReaderRecord struct {
		Name       string  `json:"name"`
		Model      string  `json:"model"`
		Effort     string  `json:"effort"`
		NoThinking bool    `json:"no_thinking,omitempty"`
		Once       bool    `json:"once,omitempty"`
		InputUSD   float64 `json:"input_usd_per_mtok"`
		OutputUSD  float64 `json:"output_usd_per_mtok"`
	}
	// SheetRecord tells a sheet: its set, number and file, who wrote it, its task, scenario and
	// outcome, its layout of free writing, the key of its check and those of its false keys, and its
	// markup: the parts and the value of the answer, by which the report measures the reading. They
	// are the texts of the plan, not the handwriting.
	SheetRecord struct {
		Kind      string          `json:"kind"`
		Set       string          `json:"set"`
		Sheet     int             `json:"sheet"`
		File      string          `json:"file"`
		Writer    sample.Writer   `json:"writer,omitempty"`
		Task      string          `json:"task"`
		Scenario  sample.Scenario `json:"scenario"`
		Outcome   sample.Outcome  `json:"outcome"`
		Layout    sample.Layout   `json:"layout,omitempty"`
		Key       string          `json:"key"`
		FalseKeys []string        `json:"false_keys"`
		Markup    Marked          `json:"markup"`
	}
	// CallRecord tells a call: the answer as the model gave it, the verdict on it and its reason,
	// the value of its answer, what was read under the key, the failure if it failed, and what it
	// took. A call whose request could not be built was not asked, and says so.
	CallRecord struct {
		Kind string `json:"kind"`
		Call
		NotAsked bool   `json:"not_asked,omitempty"`
		Answer   string `json:"answer,omitempty"`
		Verdict  string `json:"verdict"`
		Reason   Reason `json:"reason,omitempty"`
		Value    string `json:"value,omitempty"`
		UnderKey Under  `json:"under_key"`
		Error    string `json:"error,omitempty"`
		runner.Usage
	}
)

// Log is a run's log read back: the run, its sheets, its calls in the order they were made, and how
// the run ended, or nil when the log stops before the end.
type Log struct {
	Run    RunRecord
	Sheets []SheetRecord
	Calls  []CallRecord
	End    *runner.End
}

// ReadLog reads the log of a run of the reading back, as runner.ReadLog reads a log: every line a
// record of a kind it knows, the run's first and the end, if there is one, last, and a last line
// cut off half written left out. A log of another kind of call is refused.
func ReadLog(r io.Reader) (*Log, error) {
	var l Log
	end, err := runner.ReadLog(r, func(kind string, line []byte) error {
		switch kind {
		case "run":
			if err := json.Unmarshal(line, &l.Run); err != nil {
				return err
			}
			if l.Run.CallKind != CallKind {
				return fmt.Errorf("a log of the kind of call %q, not of the reading", l.Run.CallKind)
			}
			return nil
		case "sheet":
			var rec SheetRecord
			err := json.Unmarshal(line, &rec)
			l.Sheets = append(l.Sheets, rec)
			return err
		case "call":
			var rec CallRecord
			err := json.Unmarshal(line, &rec)
			l.Calls = append(l.Calls, rec)
			return err
		}
		return fmt.Errorf("no record of the kind %q", kind)
	})
	if err != nil {
		return nil, err
	}
	l.End = end
	return &l, nil
}
