package paperset

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/altnovatrail/module-kit/paper"

	"github.com/altnovatrail/tutor/internal/checking"
	"github.com/altnovatrail/tutor/internal/finding"
	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/runner"
	"github.com/altnovatrail/tutor/internal/statement"
	"github.com/altnovatrail/tutor/provider"
)

// The kinds of call of the three runs, as their logs tell them (runner.CallKind). The finding asks
// only the cases of several tasks that code draws, each a column of examples under one instruction,
// with the learner's request to frame each example apart.
const (
	CallStatement = "statement"
	CallCheck     = "check"
	CallFinding   = "finding"
)

// CallKinds are the kinds of call of the set, in their order.
var CallKinds = []string{CallStatement, CallCheck, CallFinding}

// FindingRequest is the learner's request every case of the finding sends.
const FindingRequest = "each example a task of its own"

// The language and the age category every Check of the set asks with: those of the author's own
// profile, the only one whose work a case may hold.
const (
	checkLanguage = "en"
	checkCategory = "adult"
)

// The input tokens a call is expected to take besides its pictures', for the estimate, and a
// generous count of all of them for the worst case; and the output each kind is expected to write.
const (
	inputsOfText = 1500
	inputBudget  = 12000
)

var outputs = map[string]int64{CallStatement: 300, CallCheck: 2500, CallFinding: 400}

// prompts are the fingerprint and the version of each kind's prompt, which the run's record keeps.
var prompts = map[string]struct {
	fingerprint func() string
	version     int
}{
	CallStatement: {statement.Fingerprint, statement.Version},
	CallCheck:     {checking.Fingerprint, checking.Version},
	CallFinding:   {finding.Fingerprint, finding.Version},
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

// photo is a picture as a request of the statement or the Check sends it: the library sends the
// task's crop as provider.Photo makes it, a JPEG, as an exported case holds it, and a PNG goes as
// one here, which no Fitter shrinks.
func photo(p picture) provider.Image { return provider.Image{PNG: p.PNG, JPEG: p.JPEG} }

// Request is the call's request, as the library asks it of its kind.
func (s *step) Request() (provider.Request, error) {
	var req provider.Request
	switch s.kind {
	case CallStatement:
		req = statement.RequestOf(photo(s.pictures.Statement), s.pictures.Statement.Width, s.pictures.Statement.Height)
		req.MaxOutputTokens = statement.MaxOutputTokens
	case CallFinding:
		req = finding.RequestOf(photo(s.pictures.Statement), s.pictures.Statement.Width, s.pictures.Statement.Height, FindingRequest, nil)
		req.MaxOutputTokens = finding.MaxOutputTokens
	default:
		p := checking.Prompt{
			Statement: checking.Picture{Image: photo(s.pictures.Statement), Width: s.pictures.Statement.Width, Height: s.pictures.Statement.Height},
			Work:      checking.Picture{Image: provider.Image{PNG: s.pictures.Work.PNG}, Width: s.pictures.Work.Width, Height: s.pictures.Work.Height},
			Tasks:     len(s.kase.Tasks), Language: checkLanguage, AgeCategory: checkCategory,
		}
		if in := s.pictures.Instruction; in != nil {
			p.Instruction = &checking.Picture{Image: photo(*in), Width: in.Width, Height: in.Height}
		}
		var err error
		if req, err = checking.RequestOf(p); err != nil {
			return provider.Request{}, err
		}
		req.MaxOutputTokens = checking.MaxOutputTokens
	}
	return req, nil
}

// expected is the verdict of the Check a task of the case comes to by its truth, or "" for any: no
// work drawn is no answer; of an exported case, work so far correct is no answer too, and so is a
// task unchecked, which the Check leaves with no verdict when it reads no answer; and a task code
// draws unchecked, an open one, may take any.
func (s *step) expected(t Task) string {
	exported := s.kase.pictures != nil
	switch {
	case t.Outcome == OutcomeUnchecked && !exported:
		return ""
	case t.Outcome == OutcomeUnchecked, t.Outcome == OutcomeSoFarCorrect, t.Work == "" && !exported:
		return checking.VerdictNoAnswer
	case t.Outcome == OutcomeCorrect:
		return checking.VerdictCorrect
	case t.Outcome == OutcomeNotYet:
		return checking.VerdictIncorrect
	}
	return checking.VerdictUnreadable
}

// Right is the answer of a model that is always right, by the case's truth: its tasks with their
// statements and formulas, the verdict and the value read of each task, or a box round each task's
// line of the statement's picture.
func (s *step) Right() (string, error) {
	switch s.kind {
	case CallFinding:
		boxes := []map[string]any{}
		for _, b := range s.lines() {
			boxes = append(boxes, map[string]any{"x1": b.X1, "y1": b.Y1, "x2": b.X2, "y2": b.Y2, "instruction": map[string]int{"x1": 0, "y1": 0, "x2": 0, "y2": 0}})
		}
		data, err := json.Marshal(map[string]any{"tasks": boxes})
		return string(data), err
	case CallStatement:
		tasks := []map[string]string{}
		for _, t := range s.kase.Tasks {
			tasks = append(tasks, map[string]string{"statement": t.Statement, "formula": t.Formula})
		}
		data, err := json.Marshal(map[string]any{"tasks": tasks})
		return string(data), err
	}
	slots := make([]checking.Slot, len(s.kase.Tasks))
	for i, t := range s.kase.Tasks {
		slots[i] = checking.Slot{Task: i + 1, Verdict: s.expected(t), Read: t.Read, Answer: written(t.Key)}
		if slots[i].Verdict == "" {
			slots[i].Verdict = checking.VerdictCorrect
		}
		if slots[i].Verdict == checking.VerdictNoAnswer {
			slots[i].Read = ""
		}
	}
	return checking.Answered(slots, "Look at your work once more.", []string{"Where do you begin?", "Look at the first line."})
}

// written is a key as the learner writes it: the number, or the label of the option, "" for none.
func written(k *Key) string {
	switch {
	case k == nil:
		return ""
	case k.Choice != "":
		return k.Choice
	}
	return k.Number
}

// Record is the call's record: the answer as the model gave it, and what the program makes of it
// against the truth of each task of the case. Of the statement, each formula and what the solver
// gives on it — right when the tasks are as many as the case's, and each formula gives the answer
// the solver gives on the true one, or both are no formula, since a formula read of a word problem
// is the reading's mistake. Of the Check, each task's verdict and value read — right when each
// verdict is the one its truth comes to (step.expected). Of the finding, how many boxes. An answer
// off the schema fails the call.
func (s *step) Record(o runner.Outcome) any {
	rec := CallRecord{Kind: "call", Call: s.Call, NotAsked: o.NotAsked, Answer: o.Answer, Error: o.Error, Usage: o.Usage}
	if o.NotAsked || o.Error != "" {
		return rec
	}
	tasks := s.kase.Tasks
	switch s.kind {
	case CallFinding:
		found, err := finding.Read(o.Answer)
		if err != nil {
			rec.Error = err.Error()
			return rec
		}
		boxes := make([]finding.Box, len(found))
		for i, f := range found {
			boxes[i] = f.Box
		}
		rec.Boxes = len(boxes)
		rec.Right = len(boxes) == len(tasks) && s.apart(boxes) && s.onLines(boxes)
	case CallStatement:
		read, err := statement.Read(o.Answer)
		if err != nil {
			rec.Error = err.Error()
			return rec
		}
		rec.Formulas, rec.Solved = []string{}, []string{}
		rec.Right = len(read) == len(tasks)
		for i, r := range read {
			rec.Formulas, rec.Solved = append(rec.Formulas, r.Formula), append(rec.Solved, solvedOf(r.Formula))
			if i < len(tasks) {
				rec.Right = rec.Right && solvedOf(r.Formula) == solvedOf(tasks[i].Formula) && (r.Formula == "") == (tasks[i].Formula == "")
			}
		}
	default:
		check, err := checking.Read(o.Answer, len(tasks), s.pictures.Work.Width, s.pictures.Work.Height)
		if err != nil {
			rec.Error = err.Error()
			return rec
		}
		rec.Verdicts, rec.Reads, rec.Right = []string{}, []string{}, true
		for i, slot := range check.Slots {
			rec.Verdicts, rec.Reads = append(rec.Verdicts, slot.Verdict), append(rec.Reads, slot.Read)
			if want := s.expected(tasks[i]); want != "" {
				rec.Right = rec.Right && slot.Verdict == want
			}
		}
	}
	return rec
}

// lines are the boxes of the tasks' lines on the statement's picture, below the lines of its
// instruction: the picture split into its lines evenly, as synth writes them.
func (s *step) lines() []finding.Box {
	p, n := s.pictures.Statement, len(s.kase.Statement)+len(s.kase.Tasks)
	boxes := make([]finding.Box, len(s.kase.Tasks))
	for i := range boxes {
		line := len(s.kase.Statement) + i
		boxes[i] = finding.Box{X1: 0, Y1: line * p.Height / n, X2: p.Width, Y2: (line + 1) * p.Height / n}
	}
	return boxes
}

// apart is whether boxes lie on the statement's picture and no two of them share more than a strip
// of their edge: a box round two examples, or two round one, is the finding's mistake.
func (s *step) apart(boxes []finding.Box) bool {
	p := s.pictures.Statement
	for i, a := range boxes {
		if a.X1 < 0 || a.Y1 < 0 || a.X2 > p.Width || a.Y2 > p.Height || a.X2 <= a.X1 || a.Y2 <= a.Y1 {
			return false
		}
		for _, b := range boxes[:i] {
			w, h := min(a.X2, b.X2)-max(a.X1, b.X1), min(a.Y2, b.Y2)-max(a.Y1, b.Y1)
			if w > 0 && h > 0 && h*4 > min(a.Y2-a.Y1, b.Y2-b.Y1) {
				return false
			}
		}
	}
	return true
}

// onLines is whether the middle of each task's line lies in one of boxes alone: with as many boxes
// as tasks, each box frames a task of its own, and none frames the statement's lines instead.
func (s *step) onLines(boxes []finding.Box) bool {
	for _, l := range s.lines() {
		x, y := (l.X1+l.X2)/2, (l.Y1+l.Y2)/2
		in := 0
		for _, b := range boxes {
			if b.X1 <= x && x <= b.X2 && b.Y1 <= y && y <= b.Y2 {
				in++
			}
		}
		if in != 1 {
			return false
		}
	}
	return true
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

// tokensOf are the tokens of the pictures a kind of call sends of a case: the statement's crop, and
// to the Check the work and the instruction's crop too.
func tokensOf(kind string, p *Pictures) int {
	n := render.Tokens(p.Statement.Width, p.Statement.Height)
	if kind == CallCheck {
		n += render.Tokens(p.Work.Width, p.Work.Height)
		if p.Instruction != nil {
			n += render.Tokens(p.Instruction.Width, p.Instruction.Height)
		}
	}
	return n
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
	if kind == CallStatement {
		// The reading of a statement is judged by the statement of each task, which an exported
		// case of a Check holds only when Recognise read one.
		cases = slices.DeleteFunc(slices.Clone(cases), func(c Case) bool {
			return slices.ContainsFunc(c.Tasks, func(t Task) bool { return t.Statement == "" })
		})
	}
	if kind == CallFinding {
		// The finding is judged by the lines synth writes the tasks on (step.lines): the photo of a
		// case the debug mode exported has lines no code knows.
		cases = slices.DeleteFunc(slices.Clone(cases), func(c Case) bool { return c.Kind != KindSeveral || c.pictures != nil })
		if len(cases) == 0 {
			return nil, fmt.Errorf("paperset: no case of several tasks drawn by code in the part %s, the only cases the finding runs on", part)
		}
	}
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
			in := int64(inputsOfText + tokensOf(kind, drawn[i]))
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
// of call — statement, check or finding —, so that tutor-eval report tells its log from the others.
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
	// took; what the program made of it, task by task — the formulas and what the solver gives on
	// each, the Check's verdicts and values read, or how many boxes the finding gave —, and whether
	// that is right by the case's truth.
	CallRecord struct {
		Kind string `json:"kind"`
		Call
		NotAsked bool     `json:"not_asked,omitempty"`
		Answer   string   `json:"answer,omitempty"`
		Error    string   `json:"error,omitempty"`
		Formulas []string `json:"formulas,omitempty"`
		Solved   []string `json:"solved,omitempty"`
		Verdicts []string `json:"verdicts,omitempty"`
		Reads    []string `json:"reads,omitempty"`
		Boxes    int      `json:"boxes,omitempty"`
		Right    bool     `json:"right"`
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
