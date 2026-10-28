package reading

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/runner"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/sheets"
	"github.com/altnovatrail/tutor/internal/states"
	"github.com/altnovatrail/tutor/provider"
)

// CallKind is the kind of call of the reading, as its log tells it (runner.CallKind).
const CallKind = "reading"

// Reader is a configuration of the reader: a configuration of the runner, the output tokens a call
// is expected to take, for the estimate, and whether it reads once whatever the repeats, as Opus
// does in the reduced matrix, without the other heights of digits and the states (R08; R28).
type Reader struct {
	runner.Config
	Output int
	Once   bool
}

// Readers are the three readers, the three configurations of the runner by their names. An
// observation of a sheet comes to about 200 tokens, and Haiku with thinking writes about 300 more;
// Opus reads each condition once, and neither the other heights nor the states (docs/architecture.md,
// 3.4). A configuration without its reader
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

// The input tokens a call is expected to take, for the estimate: the instructions, the schema and
// the task, about 1 210 in all, the picture's, ⌈w / 28⌉ × ⌈h / 28⌉ of its crop (render.Tokens), and
// about 150 more with the block of the key. inputBudget is a generous count of them, for the worst
// case: a crop takes no more than 1 568 (render.CropOf).
const (
	inputsOfText = 1210
	inputsOfKey  = 150
	inputBudget  = 8000
)

// Reads are what a run reads of its sheets: the conditions of the key on the finished sheet with its
// digits 32 px high, keys; the finished sheet without a key at the other heights of digits, heights;
// and its states without a key, states (T69). The heights and the states are read once, by the
// readers that do not read once (R08; R28).
type Reads struct {
	Keys, Heights, States bool
}

// ParseReads reads what a run reads from a list of keys, heights and states, joined by commas.
func ParseReads(list string) (Reads, error) {
	var r Reads
	for _, name := range strings.Split(list, ",") {
		switch name {
		case "keys":
			r.Keys = true
		case "heights":
			r.Heights = true
		case "states":
			r.States = true
		default:
			return r, fmt.Errorf("reading: %q is nothing a run reads: give keys, heights or states, joined by commas", name)
		}
	}
	return r, nil
}

// Names are what a run reads, by name, in their order.
func (r Reads) Names() []string {
	var out []string
	for _, n := range []struct {
		on   bool
		name string
	}{{r.Keys, "keys"}, {r.Heights, "heights"}, {r.States, "states"}} {
		if n.on {
			out = append(out, n.name)
		}
	}
	return out
}

// Call is one call of a run: a sheet by its set and number, a condition, the key given in it as the
// bank writes it, a reader by its name, the repeat, from 1, the height of digits asked for and the
// one its crop is drawn at, which is less when the crop would be too large (render.CropOf), and the
// state of the sheet read, from 1, or none for the finished sheet.
type Call struct {
	Set       string    `json:"set"`
	Sheet     int       `json:"sheet"`
	Condition Condition `json:"condition"`
	Key       string    `json:"key,omitempty"`
	Reader    string    `json:"reader"`
	Repeat    int       `json:"repeat"`
	Digit     int       `json:"digit"`
	Drawn     int       `json:"drawn"`
	State     int       `json:"state,omitempty"`
}

// prepared is a sheet of the run with its markup, its states, the readings each policy of reading
// on its own would take of it, and h_med of the finished sheet.
type prepared struct {
	sheets.Chosen
	marked   Marked
	states   []states.State
	policies states.Policies
	median   float64
}

// step is a call of the reading on the runner: a sheet or a state of it, at a height of digits, in a
// condition, by a reader, in a repeat. right is the markup of what it shows, as far as the writer
// had written it.
type step struct {
	Call
	sheet *prepared
	shown *Sheet
	given *Given
	right Marked
}

// Request is the reading's request of what the call shows in its condition.
func (s *step) Request() (provider.Request, error) {
	return Request(s.sheet.Task, s.shown, s.given), nil
}

// Right is the observation of the reader who is always right, by the markup of what it shows.
func (s *step) Right() (string, error) {
	data, err := json.Marshal(Right(s.right))
	return string(data), err
}

// Record is the call's record: the answer as the model gave it, the verdict on it against the
// task's key, with its reason and the value of the answer, and what was read under the key given.
// Without a key, what was read under the right key is told too: how often the reader writes the
// right solution where the sheet has something else, without being given it. It is told against
// the markup of the finished sheet, on a state too: what of a state was read as written though not
// yet written the report tells by how far each part was written (T72). An answer off the schema is
// a failure of the reading, and its verdict is unchecked.
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
// repeated: it cuts the crops of every sheet and puts the calls in order, repeat by repeat, so that
// a run cut short has the same repeats of every sheet. Of each sheet, by each reader, as reads says:
//   - keys: the finished sheet at 32 px, without a key and with the right key, and on a correct
//     sheet with each false key of its task (R03). A reader that reads once asks only in the first
//     repeat;
//   - heights: the finished sheet without a key at 16, 24 and 48 px, a height whose crop is drawn at
//     the height of one before it, 32 px first, left out;
//   - states: each state of the sheet, without a key, at 32 px.
//
// The heights and the states are asked in the first repeat, by the readers that do not read once.
func New(part string, sets []string, chosen []sheets.Chosen, readers []Reader, repeats int, reads Reads) (*runner.Run, error) {
	if repeats < 1 {
		return nil, fmt.Errorf("reading: %d repeats", repeats)
	}
	if len(reads.Names()) == 0 {
		return nil, fmt.Errorf("reading: the run reads nothing")
	}
	r := &runner.Run{}
	// The heights and the states are read by the readers that do not read once: without them, their
	// crops are not cut.
	cut := reads
	if !slices.ContainsFunc(readers, func(rd Reader) bool { return !rd.Once }) {
		cut.Heights, cut.States = false, false
	}
	var all []sheetPlan
	for _, sh := range chosen {
		pl, err := plan(sh, cut)
		if err != nil {
			return nil, fmt.Errorf("reading: sheet %d of %s: %w", sh.Number, sh.Set, err)
		}
		all = append(all, pl)
		r.Items = append(r.Items, sheetRecord(pl.sheet))
	}
	var records []ReaderRecord
	for _, rd := range readers {
		records = append(records, ReaderRecord{Name: rd.Name, Model: rd.Model.ID, Effort: rd.Effort, NoThinking: rd.NoThinking, Once: rd.Once, InputUSD: rd.Model.Input, OutputUSD: rd.Model.Output})
	}
	r.Record = func(h runner.Head) any {
		return RunRecord{Kind: "run", CallKind: CallKind, Time: h.Time, Part: part, Sets: sets, Readers: records, Repeats: repeats, Reads: reads.Names(), MaxUSD: h.MaxUSD, Estimate: h.Estimate,
			Prompt: Fingerprint(), PromptVersion: Version}
	}
	add := func(rd Reader, rep int, proto *step) {
		st := *proto
		st.Reader, st.Repeat = rd.Name, rep
		in := int64(inputsOfText + render.Tokens(st.shown.Crop.Width, st.shown.Crop.Height))
		if st.given != nil {
			in += inputsOfKey
		}
		r.Steps = append(r.Steps, runner.Step{Config: rd.Config, In: in, Out: int64(rd.Output), WorstIn: inputBudget, Call: &st})
	}
	for rep := 1; rep <= repeats; rep++ {
		for _, sh := range all {
			for _, rd := range readers {
				if rd.Once && rep > 1 {
					continue
				}
				for _, st := range sh.keyed {
					add(rd, rep, st)
				}
				if rep == 1 && !rd.Once {
					for _, st := range sh.once {
						add(rd, rep, st)
					}
				}
			}
		}
	}
	return r, nil
}

// Planned is what a planned run of the reading asks besides the conditions of the key: how many
// calls read the finished sheets at the other heights of digits, and how many states of the sheets
// are read, in how many calls.
type Planned struct {
	Heights, States, StateCalls int
}

// PlannedOf counts what a run that New planned asks besides the conditions of the key.
func PlannedOf(r *runner.Run) Planned {
	var p Planned
	seen := map[Call]bool{}
	for _, s := range r.Steps {
		st, ok := s.Call.(*step)
		switch {
		case !ok:
		case st.State > 0:
			p.StateCalls++
			c := Call{Set: st.Set, Sheet: st.Sheet, State: st.State}
			if !seen[c] {
				seen[c] = true
				p.States++
			}
		case st.Digit != render.Digit:
			p.Heights++
		}
	}
	return p
}

// sheetPlan is a sheet of a run with the calls of it a reader makes: in the conditions of the key,
// in every repeat, and at the other heights and in its states, once.
type sheetPlan struct {
	sheet       *prepared
	keyed, once []*step
}

// plan cuts the crops of a sheet that the run reads and makes its calls, but for their reader and
// repeat.
func plan(sh sheets.Chosen, reads Reads) (sheetPlan, error) {
	s := sh.Sample
	finished := render.Shown(s, math.MaxInt64)
	main, err := Prepare(finished, render.Digit)
	if err != nil {
		return sheetPlan{}, err
	}
	sts := states.Of(s, sh.Plan != nil && len(sh.Plan.Parts) > 0)
	p := &prepared{Chosen: sh, marked: MarkupOf(s, sh.Plan), states: sts, policies: states.PoliciesOf(s, sts), median: main.Crop.Median}
	out := sheetPlan{sheet: p}
	call := func(shown *Sheet, state int) Call {
		return Call{Set: sh.Set, Sheet: sh.Number, Condition: NoKey, Digit: shown.Crop.Digit, Drawn: shown.Crop.Drawn, State: state}
	}
	if reads.Keys {
		for _, g := range conditions(p) {
			c := call(main, 0)
			c.Condition = g.condition
			if g.given != nil {
				c.Key = g.given.Key
			}
			out.keyed = append(out.keyed, &step{Call: c, sheet: p, shown: main, given: g.given, right: p.marked})
		}
	}
	if reads.Heights {
		drawn := []int{main.Crop.Drawn}
		for _, d := range render.Heights() {
			if d == render.Digit {
				continue
			}
			shown, err := Prepare(finished, d)
			if err != nil {
				return sheetPlan{}, err
			}
			if slices.Contains(drawn, shown.Crop.Drawn) {
				continue
			}
			drawn = append(drawn, shown.Crop.Drawn)
			out.once = append(out.once, &step{Call: call(shown, 0), sheet: p, shown: shown, right: p.marked})
		}
	}
	if reads.States {
		for i, st := range sts {
			shown, err := Prepare(st.Shown, render.Digit)
			if err != nil {
				return sheetPlan{}, err
			}
			out.once = append(out.once, &step{Call: call(shown, i+1), sheet: p, shown: shown, right: writtenBy(p.marked, st.Parts)})
		}
	}
	return out, nil
}

// writtenBy is the markup of a sheet as far as the writer had written it at a state: the parts
// written whole by then, and the value of the answer only once its part was, as MarkupOf takes it.
func writtenBy(m Marked, parts []states.Progress) Marked {
	out := Marked{Parts: []MarkedPart{}}
	for i, p := range m.Parts {
		if parts[i] != states.Done {
			continue
		}
		out.Parts = append(out.Parts, p)
		if p.Answer && !p.CrossedOut && out.Value == "" {
			out.Value = ValueOf(p.Text)
		}
	}
	return out
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
		Outcome: sh.Sample.Expected.Outcome, Key: KeyOf(sh.Task.Key), FalseKeys: []string{}, Markup: sh.marked,
		Median: sh.median, States: sh.states, Policies: sh.policies}
	if rec.States == nil {
		rec.States = []states.State{}
	}
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
	// RunRecord tells the run: its part and sets, its readers and repeats, what it reads, its cap and
	// estimate, and the fingerprint and the version of its prompt.
	RunRecord struct {
		Kind          string          `json:"kind"`
		CallKind      string          `json:"call_kind"`
		Time          time.Time       `json:"time"`
		Part          string          `json:"part"`
		Sets          []string        `json:"sets"`
		Readers       []ReaderRecord  `json:"readers"`
		Repeats       int             `json:"repeats"`
		Reads         []string        `json:"reads"`
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
	// are the texts of the plan, not the handwriting. It tells, too, h_med of the finished sheet in
	// units, by which its crops are scaled; its states, each with how far the writer had written
	// each part of the markup, whether the run reads them or not; and how many readings each policy
	// of reading on its own would take of it (О-8; T69).
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
		Median    float64         `json:"median"`
		States    []states.State  `json:"states"`
		Policies  states.Policies `json:"policies"`
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
