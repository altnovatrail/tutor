package paperset

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/altnovatrail/module-kit/observation"
	"github.com/altnovatrail/module-kit/paper"
	observationv1 "github.com/altnovatrail/spec/contracts/novatrail/observation/v1"
	paperv1 "github.com/altnovatrail/spec/contracts/novatrail/paper/v1"

	"github.com/altnovatrail/tutor/internal/reading"
	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/runner"
	"github.com/altnovatrail/tutor/internal/solving"
	"github.com/altnovatrail/tutor/internal/statement"
	"github.com/altnovatrail/tutor/provider"
)

// The kinds of call of the three runs, as their logs tell them (runner.CallKind).
const (
	CallStatement = "statement"
	CallSolving   = "solving"
	CallPaper     = "paper"
)

// CallKinds are the kinds of call of the set, in their order.
var CallKinds = []string{CallStatement, CallSolving, CallPaper}

// The input tokens a call is expected to take besides its picture's, for the estimate, and a
// generous count of all of them for the worst case; and the output each kind is expected to write.
const (
	inputsOfText = 1500
	inputBudget  = 8000
)

var outputs = map[string]int64{CallStatement: 300, CallSolving: 2000, CallPaper: 600}

// prompts are the fingerprint and the version of each kind's prompt, which the run's record keeps.
var prompts = map[string]struct {
	fingerprint func() string
	version     int
}{
	CallStatement: {statement.Fingerprint, statement.Version},
	CallSolving:   {solving.Fingerprint, solving.Version},
	CallPaper:     {reading.PaperFingerprint, reading.PaperVersion},
}

// File is the set a run asks: its file, by its name and SHA-256, and the set read.
type File struct {
	Name, SHA256 string
	Set          *Set
}

// Call is one call of a run: a case by its number, a configuration by its name, and the repeat,
// from 1.
type Call struct {
	Case   int    `json:"case"`
	Config string `json:"config"`
	Repeat int    `json:"repeat"`
}

// step is a call of a run on the runner, with its case and its pictures.
type step struct {
	Call
	kind     string
	kase     *Case
	pictures *Pictures
}

// photo is a picture as a request of the statement or the solution sends it: the library sends the
// task's crop as provider.Photo makes it, a JPEG, and a PNG goes as one here, which no Fitter
// shrinks.
func photo(p picture) provider.Image { return provider.Image{PNG: p.PNG} }

// Request is the call's request, as the library asks it of its kind.
func (s *step) Request() (provider.Request, error) {
	var req provider.Request
	switch s.kind {
	case CallStatement:
		req = statement.RequestOf(photo(s.pictures.Statement), s.pictures.Statement.Width, s.pictures.Statement.Height)
		req.MaxOutputTokens = statement.MaxOutputTokens
	case CallSolving:
		req = solving.RequestOf(photo(s.pictures.Statement), s.pictures.Statement.Width, s.pictures.Statement.Height)
		req.MaxOutputTokens = solving.MaxOutputTokens
	default:
		req = reading.PaperRequestOf(s.pictures.Work.PNG, s.pictures.Work.Width, s.pictures.Work.Height)
		req.MaxOutputTokens = reading.MaxOutputTokens
	}
	return req, nil
}

// Right is the answer of a model that is always right, by the case's truth: its formula, its key,
// or an observation of the value read, none when there is no work.
func (s *step) Right() (string, error) {
	switch s.kind {
	case CallStatement:
		data, err := json.Marshal(map[string]string{"formula": s.kase.Formula})
		return string(data), err
	case CallSolving:
		return solving.Answered(s.kase.Key.Answer())
	}
	o := reading.Observation{Parts: []reading.Part{}, Results: []reading.Result{}}
	if s.kase.Work != "" {
		o.Parts = append(o.Parts, reading.Part{Kind: reading.KindText, Text: s.kase.Work, Legible: true, OtherReadings: []string{}})
		o.Results = append(o.Results, reading.Result{Part: 1, Value: s.kase.Read, OtherValues: []string{}})
	}
	data, err := json.Marshal(o)
	return string(data), err
}

// Record is the call's record: the answer as the model gave it, and what the program makes of it
// against the case's truth. Of the statement, the formula and what the solver gives on it — right
// when it is the answer the solver gives on the true formula, or when both are no formula, since a
// formula read of a word problem is the reading's mistake. Of the solution, the key it makes (paper
// .Key) — right when it judges the case's right value correct, or when neither has a key. Of the
// work, the verdict by the case's key on the observation brought to it — right when it is the
// case's outcome; with no key it is unchecked. An answer off the schema fails the call.
func (s *step) Record(o runner.Outcome) any {
	rec := CallRecord{Kind: "call", Call: s.Call, NotAsked: o.NotAsked, Answer: o.Answer, Error: o.Error, Usage: o.Usage}
	if o.NotAsked || o.Error != "" {
		return rec
	}
	truth := s.kase.Key.Answer()
	switch s.kind {
	case CallStatement:
		formula, err := statement.Read(o.Answer)
		if err != nil {
			rec.Error = err.Error()
			return rec
		}
		rec.Formula = &formula
		rec.Solved = solvedOf(formula)
		rec.Right = rec.Solved == solvedOf(s.kase.Formula) && (formula == "") == (s.kase.Formula == "")
	case CallSolving:
		answer, err := solving.Read(o.Answer)
		if err != nil {
			rec.Error = err.Error()
			return rec
		}
		key := paper.Key(nil, answer)
		rec.Key = keyOf(key.GetAnswer())
		rec.Right = (truth == nil) == (key.GetAnswer() == nil) && (truth == nil || judgesRight(key.GetAnswer(), truth))
	default:
		read, err := reading.Read(o.Answer)
		if err != nil {
			rec.Error = err.Error()
			return rec
		}
		obs, _ := reading.ObservationOf(read)
		obs.AnswerSlots[0].AnswerSlot = &observationv1.AnswerSlotPlace{Item: 1, Slot: 1}
		rec.Verdict, rec.Reason = OutcomeUnchecked, ""
		if truth != nil {
			v, err := verdictOf(truth, obs)
			if err != nil {
				rec.Error = err.Error()
				return rec
			}
			rec.Verdict, rec.Reason = v.Verdict, v.Reason
		}
		rec.Right = rec.Verdict == s.kase.Outcome
	}
	return rec
}

// solvedOf is what the solver gives on a formula: its value, or "refused: " and its reason; and
// "" for no formula.
func solvedOf(formula string) string {
	if formula == "" {
		return ""
	}
	n, reason := paper.Solve(formula)
	if n == nil {
		return "refused: " + string(reason)
	}
	return n.GetValue()
}

// judgesRight is whether a key of AI judges correct the value a truth's key holds, and is of its
// kind: so a key of 0.5 is the truth's 1/2, and one of other options is not its choice.
func judgesRight(key, truth *paperv1.Answer) bool {
	value := truth.GetNumber().GetValue()
	if c := truth.GetChoice(); c != nil {
		if key.GetChoice() == nil || len(key.GetChoice().GetOptions()) != len(c.GetOptions()) {
			return false
		}
		value = c.GetKey()
	} else if key.GetNumber() == nil {
		return false
	}
	o := &observationv1.Observation{AnswerSlots: []*observationv1.AnswerSlotObservation{{
		AnswerSlot: &observationv1.AnswerSlotPlace{Item: 1, Slot: 1},
		Read:       &observationv1.AnswerSlotObservation_Value{Value: value},
	}}}
	v, err := verdictOf(key, o)
	return err == nil && v.Verdict == OutcomeCorrect
}

// slotVerdict is the verdict of the one answer slot of a task from paper and its reason.
type slotVerdict struct{ Verdict, Reason string }

// verdictOf is the verdict by a key on an observation, as api gives it: the values brought to the
// key, the observation validated and judged against the task's one item.
func verdictOf(key *paperv1.Answer, o *observationv1.Observation) (slotVerdict, error) {
	items := paper.Items(key)
	brought := paper.Brought(key, o)
	if err := observation.Validate(items, brought); err != nil {
		return slotVerdict{}, err
	}
	got, err := observation.Verdicts(context.Background(), items, brought, nil)
	if err != nil {
		return slotVerdict{}, err
	}
	s := got[0].Slots[0]
	return slotVerdict{Verdict: string(s.Verdict), Reason: string(s.Reason)}, nil
}

// keyOf is a key as the file of a set writes it, or nil for none.
func keyOf(a *paperv1.Answer) *Key {
	switch {
	case a.GetNumber() != nil:
		n := a.GetNumber()
		return &Key{Number: n.GetValue(), Units: n.GetUnits(), Unknown: n.GetUnknown()}
	case a.GetChoice() != nil:
		return &Key{Options: a.GetChoice().GetOptions(), Choice: a.GetChoice().GetKey()}
	}
	return nil
}

// New plans a run of a kind of call over the cases of a part of a set, by the configurations,
// repeated: it draws the pictures of every case and puts the calls in order, repeat by repeat, case
// by case, configuration by configuration, so that a run cut short has the same repeats of every
// case.
func New(kind, part string, f *File, configs []runner.Config, repeats int) (*runner.Run, error) {
	if _, ok := prompts[kind]; !ok {
		return nil, fmt.Errorf("paperset: no kind of call %q", kind)
	}
	cases := f.Set.Of(part)
	switch {
	case repeats < 1:
		return nil, fmt.Errorf("paperset: %d repeats", repeats)
	case len(cases) == 0:
		return nil, fmt.Errorf("paperset: no case of the part %s", part)
	case len(configs) == 0:
		return nil, errors.New("paperset: no configuration to ask")
	}
	r := &runner.Run{}
	drawn := make([]*Pictures, len(cases))
	for i, c := range cases {
		p, err := Draw(c, f.Set.Seed)
		if err != nil {
			return nil, fmt.Errorf("paperset: %w", err)
		}
		drawn[i] = p
		r.Items = append(r.Items, CaseRecord{Kind: "case", Case: c, Width: p.Work.Width, Height: p.Work.Height})
	}
	var records []ConfigRecord
	for _, c := range configs {
		records = append(records, ConfigRecord{Name: c.Name, Model: c.Model.ID, Effort: c.Effort, NoThinking: c.NoThinking, InputUSD: c.Model.Input, OutputUSD: c.Model.Output})
	}
	p := prompts[kind]
	r.Record = func(h runner.Head) any {
		return RunRecord{Kind: "run", CallKind: kind, Time: h.Time, Part: part, File: f.Name, SHA256: f.SHA256, Seed: f.Set.Seed, Configs: records, Repeats: repeats,
			MaxUSD: h.MaxUSD, Estimate: h.Estimate, Prompt: p.fingerprint(), PromptVersion: p.version, Runner: h.Runner, ModuleKit: h.ModuleKit}
	}
	for rep := 1; rep <= repeats; rep++ {
		for i := range cases {
			pic := drawn[i].Statement
			if kind == CallPaper {
				pic = drawn[i].Work
			}
			in := int64(inputsOfText + render.Tokens(pic.Width, pic.Height))
			for _, c := range configs {
				call := Call{Case: cases[i].Number, Config: c.Name, Repeat: rep}
				r.Steps = append(r.Steps, runner.Step{Config: c, In: in, Out: outputs[kind], WorstIn: inputBudget, Call: &step{Call: call, kind: kind, kase: &cases[i], pictures: drawn[i]}})
			}
		}
	}
	return r, nil
}

// The records of a run's log, a JSON object a line, each with its kind: the run first, then each
// case, each call, and the end, which is the runner's (runner.End). The run's record tells its kind
// of call — statement, solving or paper —, so that tutor-eval report tells its log from the others.
type (
	// RunRecord tells the run: its kind of call, part, set by its file, SHA-256 and seed, its
	// configurations and repeats, its cap and estimate, the fingerprint and the version of its
	// prompt, and the versions of the build that ran it.
	RunRecord struct {
		Kind          string          `json:"kind"`
		CallKind      string          `json:"call_kind"`
		Time          time.Time       `json:"time"`
		Part          string          `json:"part"`
		File          string          `json:"file"`
		SHA256        string          `json:"sha256"`
		Seed          uint64          `json:"seed"`
		Configs       []ConfigRecord  `json:"configs"`
		Repeats       int             `json:"repeats"`
		MaxUSD        float64         `json:"max_usd"`
		Estimate      runner.Estimate `json:"estimate"`
		Prompt        string          `json:"prompt"`
		PromptVersion int             `json:"prompt_version"`
		Runner        string          `json:"runner,omitempty"`
		ModuleKit     string          `json:"module_kit,omitempty"`
	}
	// ConfigRecord tells a configuration: its name, model, effort, whether it thinks, and the prices
	// of its tokens in US dollars a million.
	ConfigRecord struct {
		Name       string  `json:"name"`
		Model      string  `json:"model"`
		Effort     string  `json:"effort"`
		NoThinking bool    `json:"no_thinking,omitempty"`
		InputUSD   float64 `json:"input_usd"`
		OutputUSD  float64 `json:"output_usd"`
	}
	// CaseRecord tells a case of the run, as the set writes it, with the size of the crop of its work.
	CaseRecord struct {
		Kind   string `json:"kind"`
		Case   Case   `json:"case"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	}
	// CallRecord tells a call: the answer as the model gave it, the failure if it failed, and what it
	// took; what the program made of it — the formula and what the solver gives on it, the key, or
	// the verdict and its reason —, and whether that is right by the case's truth.
	CallRecord struct {
		Kind string `json:"kind"`
		Call
		NotAsked bool    `json:"not_asked,omitempty"`
		Answer   string  `json:"answer,omitempty"`
		Error    string  `json:"error,omitempty"`
		Formula  *string `json:"formula,omitempty"`
		Solved   string  `json:"solved,omitempty"`
		Key      *Key    `json:"key,omitempty"`
		Verdict  string  `json:"verdict,omitempty"`
		Reason   string  `json:"reason,omitempty"`
		Right    bool    `json:"right"`
		runner.Usage
	}
)

// Log is a run's log read back: the run, its cases, its calls in the order they were made, and how
// the run ended, or nil when the log stops before the end.
type Log = runner.Records[RunRecord, CaseRecord, CallRecord]

// ReadLog reads the log of a run of a kind of call of the set back, as runner.ReadLog reads a log,
// its items records of the kind "case". A log of another kind of call is refused.
func ReadLog(r io.Reader, kind string) (*Log, error) {
	return runner.ReadItems[RunRecord, CaseRecord, CallRecord](r, kind, "task from paper", "case")
}
