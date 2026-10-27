// Package measure is the judge's kind of call on the runner (T67): the measurement of the writing
// process (R24). Its run asks every sheet of a part in every form it has, by every judge,
// repeated, in one version of the judge's prompt: version 1 on the sheets of lines of E2, version
// 2 on free writing (T65). The package plans the run, says how each call is asked and logged, and
// reads the run's log back; the runner estimates the run, holds its cap, asks the calls and writes
// the log (internal/runner). The log is as E2 and T66 wrote it.
package measure

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/altnovatrail/tutor/internal/journal"
	"github.com/altnovatrail/tutor/internal/judge"
	"github.com/altnovatrail/tutor/internal/runner"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/sheets"
	"github.com/altnovatrail/tutor/provider"
)

// inputBudget is a generous count of a call's input tokens, for its worst case: a request with
// three pictures of the sheet, the instructions and the journal comes to about 4 000.
const inputBudget = 8000

// The input tokens a call is expected to take, for the estimate, by the version of the prompt. Of
// version 1: about 1 850 for form A, about 400 more with the journal and about 820 more for each
// frame (docs/architecture.md, 3.1, «Смета»). Of version 2, by the measure of T63 from the test of
// E2 — 1.3 tokens a word, number or sign of the instructions and 1.575 of the journal: its
// instructions are about 150 of them longer, and its journal block and journal of parts about 130
// and 120 of them, so about 2 050 for form A and 360 more with the journal (architecture 3.1,
// «Раннер»). A frame is the same picture in both.
var inputs = map[judge.Version]struct{ a, journal, frame int }{
	judge.V1: {a: 1850, journal: 400, frame: 820},
	judge.V2: {a: 2050, journal: 360, frame: 820},
}

// Judge is a configuration of the judge (R24): a configuration of the runner, and the output tokens
// a call is expected to take, for the estimate (architecture 3.1).
type Judge struct {
	runner.Config
	Output int
}

// Judges are the three judges of the measurement, the three configurations of the runner: Opus 5.5
// at high, Haiku 5.5 at low, and Haiku 5.5 at low without thinking (R24). A call is expected to
// write about 50 tokens, and Haiku with thinking about 300 (architecture 3.1). A configuration
// without its output here is a mistake of the code, and Judges panics on it rather than estimate
// its calls at nothing.
func Judges() []Judge {
	outputs := map[string]int{"opus-high": 50, "haiku-low": 300, "haiku-low-no-thinking": 50}
	var out []Judge
	for _, c := range runner.Configs() {
		n, ok := outputs[c.Name]
		if !ok {
			panic("measure: no output tokens for the configuration " + c.Name)
		}
		out = append(out, Judge{Config: c, Output: n})
	}
	return out
}

// expected is the input tokens a call of a form is expected to take in a version of the prompt,
// with the frames it shows.
func expected(v judge.Version, f judge.Form, frames int) int64 {
	in := inputs[v]
	n := in.a
	if f.Journal() {
		n += in.journal
	}
	if f.Frames() {
		n += in.frame * frames
	}
	return int64(n)
}

// Sheet is a sheet of the part: its number in the plan and its file, its sample, which passes its
// check, its task, and its sheet of the plan, which tells the layout of free writing and where its
// first error stands; a sheet without one tells neither. The judge runs on one set, and its log
// tells no set.
type Sheet = sheets.Chosen

// Call is one call of a run: a sheet by its number, a form, a judge by its name and the repeat,
// from 1.
type Call struct {
	Sheet  int        `json:"sheet"`
	Form   judge.Form `json:"form"`
	Judge  string     `json:"judge"`
	Repeat int        `json:"repeat"`
}

// prepared is a sheet of the run with what the forms show of it.
type prepared struct {
	Sheet
	shown *judge.Sheet
}

// step is a call of the judge on the runner: a sheet in a form, by a judge, in a repeat.
type step struct {
	Call
	sheet *prepared
}

// Request is the request of the call's form on its sheet.
func (s *step) Request() (provider.Request, error) {
	return s.Form.Request(s.sheet.Task, s.sheet.shown)
}

// Right is the answer of a judge that is always right, by the markup of the sheet: "correct" on a
// correct sheet, "unreadable" on an unreadable one, and on a sheet with a mistake "error" with the
// box round the points of the line of the first error. A sheet of another outcome, such as one
// without an answer, has no verdict of the judge's, and the stub fails on it.
func (s *step) Right() (string, error) {
	var answer judge.Answer
	switch e := s.sheet.Sample.Expected; e.Outcome {
	case sample.OutcomeCorrect:
		answer.Verdict = judge.Correct
	case sample.OutcomeUnreadable:
		answer.Verdict = judge.Unreadable
	case sample.OutcomeError:
		answer.Verdict = judge.Error
		if e.FirstError != nil {
			answer.Box = judge.Around(s.sheet.Sample, *e.FirstError)
		}
	default:
		return "", fmt.Errorf("measure: the stub has no verdict for a sheet of the outcome %q", e.Outcome)
	}
	data, err := json.Marshal(answer)
	return string(data), err
}

// Record is the call's record: the answer as the model gave it, its verdict and box, and the line
// of the markup the box hits. An answer off the schema is logged with the reader's error.
func (s *step) Record(o runner.Outcome) any {
	rec := CallRecord{Kind: "call", Call: s.Call, NotAsked: o.NotAsked, Answer: o.Answer, Error: o.Error, Usage: o.Usage}
	if o.NotAsked || o.Error != "" {
		return rec
	}
	a, err := judge.Read(o.Answer)
	if err != nil {
		rec.Error = err.Error()
		return rec
	}
	rec.Verdict, rec.Box = a.Verdict, a.Box
	if a.Box != nil {
		rec.Hit = judge.Hit(s.sheet.Sample, *a.Box)
	}
	return rec
}

// Run is a run of the measurement, planned: its sheets drawn, its calls in order, all in one version
// of the judge's prompt. The runner estimates it and does it.
type Run struct {
	runner.Run
	framed int
}

// New plans a run of the part's sheets by the judges in a version of the prompt, repeated: it
// draws what the forms show of every sheet and puts the calls in order, repeat by repeat, so that
// a run cut short has the same repeats of every sheet. Of each sheet, by each judge, it asks forms
// A and B and, with frames, C and D. A sheet without frames has no C, and its D is its B
// (architecture 3.1): the run does not ask it again, and the log says so.
func New(part string, sheets []Sheet, judges []Judge, repeats int, v judge.Version) (*Run, error) {
	if repeats < 1 {
		return nil, fmt.Errorf("measure: %d repeats", repeats)
	}
	if _, ok := inputs[v]; !ok {
		return nil, fmt.Errorf("measure: no version %d of the prompt", v)
	}
	r := &Run{}
	var all []*prepared
	for _, sh := range sheets {
		shown, err := judge.Prepare(sh.Sample, v)
		if err != nil {
			return nil, fmt.Errorf("measure: sheet %d: %w", sh.Number, err)
		}
		p := &prepared{Sheet: sh, shown: shown}
		all = append(all, p)
		if len(shown.Frames) > 0 {
			r.framed++
		}
		r.Items = append(r.Items, sheetRecord(p))
	}
	var judged []JudgeRecord
	for _, j := range judges {
		judged = append(judged, JudgeRecord{Name: j.Name, Model: j.Model.ID, Effort: j.Effort, NoThinking: j.NoThinking, InputUSD: j.Model.Input, OutputUSD: j.Model.Output})
	}
	r.Record = func(h runner.Head) any {
		return RunRecord{Kind: "run", Time: h.Time, Part: part, Judges: judged, Repeats: repeats, MaxUSD: h.MaxUSD, Estimate: h.Estimate,
			Prompt: judge.Fingerprint(v), PromptVersion: v}
	}
	for rep := 1; rep <= repeats; rep++ {
		for _, sh := range all {
			for _, j := range judges {
				for _, f := range judge.Forms {
					c := Call{Sheet: sh.Number, Form: f, Judge: j.Name, Repeat: rep}
					switch {
					case len(sh.shown.Frames) > 0 || !f.Frames():
						r.Steps = append(r.Steps, runner.Step{Config: j.Config, In: expected(v, f, len(sh.shown.Frames)), Out: int64(j.Output), WorstIn: inputBudget, Call: &step{Call: c, sheet: sh}})
					case f == judge.FormD:
						r.Steps = append(r.Steps, runner.Step{Config: j.Config, Note: CallRecord{Kind: "call", Call: c, SameAs: judge.FormB}})
					}
				}
			}
		}
	}
	return r, nil
}

// Framed counts the sheets of the run that have frames.
func (r *Run) Framed() int { return r.framed }

// sheetRecord is the record that tells a sheet of the run.
func sheetRecord(sh *prepared) SheetRecord {
	rec := SheetRecord{Kind: "sheet", Sheet: sh.Number, File: sh.File, Writer: sh.Sample.Writer, Task: sh.Sample.Task, Scenario: sh.Sample.Scenario,
		Outcome: sh.Sample.Expected.Outcome, Frames: len(sh.shown.Frames), Lines: journal.Agree(sh.Sample, sh.shown.Journal)}
	fe := sh.Sample.Expected.FirstError
	if fe != nil {
		rec.FirstError = *fe
	}
	if sh.Plan != nil {
		rec.Layout = sh.Plan.Layout()
		if fe != nil && byPlan(sh.Sample, sh.Plan) {
			rec.FirstErrorIn = sh.Plan.Region(*fe)
		}
	}
	return rec
}

// The records of a run's log, a JSON object a line, each with its kind. The first tells the run,
// then one tells each sheet, one each call, and the last how the run ended. A call of form D that
// form B stands for has same_as and nothing else of a call's own (New). The judge's record of the
// run tells no kind of call: a log without one is the judge's (runner.CallKind).
type (
	// RunRecord tells the run: its part, judges and repeats, its cap and estimate, and the
	// fingerprint and the version of its prompt (judge.Fingerprint). The logs of E2 tell no
	// version: their prompt is version 1 (PromptOf).
	RunRecord struct {
		Kind          string          `json:"kind"`
		Time          time.Time       `json:"time"`
		Part          string          `json:"part"`
		Judges        []JudgeRecord   `json:"judges"`
		Repeats       int             `json:"repeats"`
		MaxUSD        float64         `json:"max_usd"`
		Estimate      runner.Estimate `json:"estimate"`
		Prompt        string          `json:"prompt"`
		PromptVersion judge.Version   `json:"prompt_version,omitempty"`
	}
	// JudgeRecord is a judge as the run's record tells it: its model and prices.
	JudgeRecord struct {
		Name       string  `json:"name"`
		Model      string  `json:"model"`
		Effort     string  `json:"effort"`
		NoThinking bool    `json:"no_thinking,omitempty"`
		InputUSD   float64 `json:"input_usd_per_mtok"`
		OutputUSD  float64 `json:"output_usd_per_mtok"`
	}
	// SheetRecord tells a sheet: its number and file, who wrote it, its task and scenario, the
	// outcome and the first error its markup says, its frames and the agreement of the journal's
	// lines with its markup. Of free writing, lines tells the parts of the journal against those of
	// the markup, under the name the logs of E2 have, and the record tells the layout of the sheet
	// and where its first error stands, unless the sheet is marked off its plan. The logs of E2 tell
	// none of the three.
	SheetRecord struct {
		Kind         string            `json:"kind"`
		Sheet        int               `json:"sheet"`
		File         string            `json:"file"`
		Writer       sample.Writer     `json:"writer,omitempty"`
		Task         string            `json:"task"`
		Scenario     sample.Scenario   `json:"scenario"`
		Outcome      sample.Outcome    `json:"outcome"`
		FirstError   int               `json:"first_error,omitempty"`
		Frames       int               `json:"frames"`
		Lines        journal.Agreement `json:"lines"`
		Layout       sample.Layout     `json:"layout,omitempty"`
		FirstErrorIn sample.Region     `json:"first_error_in,omitempty"`
	}
	// CallRecord tells a call: the answer as the model gave it, its verdict and box, the line of
	// the markup the box hits, the failure if it failed, and what it took. A call whose request
	// could not be built was not asked, and says so.
	CallRecord struct {
		Kind string `json:"kind"`
		Call
		SameAs   judge.Form `json:"same_as,omitempty"`
		NotAsked bool       `json:"not_asked,omitempty"`
		Answer   string     `json:"answer,omitempty"`
		Verdict  string     `json:"verdict,omitempty"`
		Box      *judge.Box `json:"box,omitempty"`
		Hit      int        `json:"hit,omitempty"`
		Error    string     `json:"error,omitempty"`
		runner.Usage
	}
	// EndRecord tells how the run ended.
	EndRecord = runner.End
)

// byPlan reports whether a sample's markup has the parts and the first error of its sheet of the
// plan, so that the plan tells where its first error stands. A sheet the board saved marked off
// the plan may not (sheets.Differences): where its first error stands, the log does not say.
func byPlan(s *sample.Sample, sh *sample.PlanSheet) bool {
	pe, se := sh.Expected.FirstError, s.Expected.FirstError
	return len(s.Lines) == len(sh.Parts) && pe != nil && se != nil && *pe == *se
}

// PromptOf is the version of the prompt a run asked in: that its record tells, or version 1 for a
// log of E2, which tells none.
func PromptOf(r RunRecord) judge.Version {
	if r.PromptVersion == 0 {
		return judge.V1
	}
	return r.PromptVersion
}

// Log is a run's log read back: the run, its sheets, its calls in the order they were made, and how
// the run ended, or nil when the log stops before the end: a run that broke off.
type Log struct {
	Run    RunRecord
	Sheets []SheetRecord
	Calls  []CallRecord
	End    *EndRecord
}

// ReadLog reads the log of a run of the judge back, as runner.ReadLog reads a log: every line a
// record of a kind it knows, the run's first and the end, if there is one, last, and a last line
// cut off half written left out.
func ReadLog(r io.Reader) (*Log, error) {
	var l Log
	end, err := runner.ReadLog(r, func(kind string, line []byte) error {
		switch kind {
		case "run":
			return json.Unmarshal(line, &l.Run)
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
