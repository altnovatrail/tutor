package essay

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/runner"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/provider"
)

// CallKind is the kind of call of the assessment by the rubric, as its log tells it
// (runner.CallKind).
const CallKind = "essay"

// Reader is a configuration of the runner that assesses essays: the output tokens a call is
// expected to take, for the estimate.
type Reader = runner.Reader

// Readers are the configurations of the runner. The text of an essay, six judgements and the
// feedback come to about 600 tokens; Haiku with thinking writes about 600 more, and Opus at high
// effort about 1 500. Opus assesses in every repeat, not once: a judgement by a rubric changes from
// call to call more than the reading of a digit (R45). The four the extended measurement adds (R50) are estimated so: Opus at medium as at high, Sonnet with thinking between Haiku and Opus, without thinking as Haiku without it, and Haiku at medium above Haiku at low.
func Readers() []Reader {
	return runner.ReadersOf("essay", map[string]int{
		"opus-high": 2100, "haiku-low": 1200, "haiku-low-no-thinking": 600,
		"opus-medium": 2100, "sonnet-low": 1500, "sonnet-low-no-thinking": 600, "haiku-medium": 1600,
	})
}

// The input tokens a call is expected to take, for the estimate: the instructions, the schema, the
// text and the block of the task's data, about 900 in all, and the crop's own, ⌈w / 28⌉ × ⌈h / 28⌉
// (render.Tokens). inputBudget is a generous count of them, for the worst case: a crop takes 1 568 at
// most.
const (
	inputsOfText = 900
	inputBudget  = 4000
)

// Call is one call of a run: an essay by its number, a reader by its name, and the repeat, from 1;
// in a run of two calls, also its step, StepRead or StepAssess (T92).
type Call struct {
	Essay  int    `json:"essay"`
	Reader string `json:"reader"`
	Repeat int    `json:"repeat"`
	Step   string `json:"step,omitempty"`
}

// step is a call of the assessment of an essay on the runner.
type step struct {
	Call
	essay  *sample.Essay
	rubric []sample.Criterion
	pic    *Picture
}

// Request is the assessment's request of the essay.
func (s *step) Request() (provider.Request, error) {
	return Request(s.essay, s.rubric, s.pic), nil
}

// Right is the answer of the model that is always right, by the plan.
func (s *step) Right() (string, error) {
	data, err := json.Marshal(Right(s.essay, s.rubric))
	return string(data), err
}

// Record is the call's record: the answer as the model gave it, what it comes to on each criterion,
// how it read the essay, and what its feedback breaks of the rule. An answer off the schema is a
// failure of the call, and every criterion is then unchecked.
func (s *step) Record(o runner.Outcome) any {
	rec := CallRecord{Kind: "call", Call: s.Call, NotAsked: o.NotAsked, Answer: o.Answer, Error: o.Error, Usage: o.Usage}
	var a *Answer
	if !o.NotAsked && o.Error == "" {
		var err error
		if a, err = Read(o.Answer); err != nil {
			rec.Error = err.Error()
		}
	}
	rec.Marks, rec.Extra = Marks(s.rubric, a)
	if a != nil {
		reading := ReadingOf(s.essay, a.Text)
		rec.Reading = &reading
		rec.Breaches, rec.FeedbackWords = Feedback(a.Feedback, s.essay)
	}
	return rec
}

// Essays are the essays of a part: dev or test, or every essay for all.
func Essays(es *sample.Essays, part string) []*sample.Essay {
	var out []*sample.Essay
	for i := range es.Essays {
		if part == "all" || es.Essays[i].Part == part {
			out = append(out, &es.Essays[i])
		}
	}
	return out
}

// New plans a run of the assessment over the essays of a part, drawn from the seed, by the readers,
// repeated: it draws every essay once, cuts its crop at 32 px and puts the calls in order, repeat by
// repeat, so that a run cut short has the same repeats of every essay.
func New(part, file string, seed uint64, es *sample.Essays, essays []*sample.Essay, readers []Reader, repeats int) (*runner.Run, error) {
	r, pics, err := prepare(part, file, seed, es, essays, readers, repeats, RunRecord{Prompt: Fingerprint(), PromptVersion: Version})
	if err != nil {
		return nil, err
	}
	for rep := 1; rep <= repeats; rep++ {
		for i, e := range essays {
			for _, rd := range readers {
				st := &step{Call: Call{Essay: e.Number, Reader: rd.Name, Repeat: rep}, essay: e, rubric: es.Rubric, pic: pics[i]}
				in := int64(inputsOfText + render.Tokens(pics[i].Crop.Width, pics[i].Crop.Height))
				r.Steps = append(r.Steps, runner.Step{Config: rd.Config, In: in, Out: int64(rd.Output), WorstIn: inputBudget, Call: st})
			}
		}
	}
	return r, nil
}

// essayRecord is the record that tells an essay of the run: its number and part, the criteria it
// does not meet, its words, what it spells wrong, how many of its sentences begin with a small
// letter and whether the last ends without its mark, and the size of its crop at 32 px. They are the
// texts of the file of the essays, not the handwriting.
func essayRecord(e *sample.Essay, pic *Picture) EssayRecord {
	return EssayRecord{Kind: "essay", Essay: e.Number, Part: e.Part, Unmet: e.Unmet, Words: len(plainWords(essayText(e))), Misspelled: e.Misspelled,
		LowerStarts: len(sample.LowerStarts(e.Text)), Unended: sample.Unended(e.Text), Width: pic.Crop.Width, Height: pic.Crop.Height}
}

// The records of a run's log, a JSON object a line, each with its kind: the run first, then each
// essay, each call, and the end, which is the runner's (runner.End). The run's record tells the kind
// of call, essay, so that tutor-eval report tells its log from the others.
type (
	// RunRecord tells the run: its part, the file of the essays and the seed they are drawn from, the
	// rubric, its readers and repeats, its cap and estimate, the fingerprint and the version of its
	// prompt, and its path, one call or two.
	RunRecord struct {
		Kind          string             `json:"kind"`
		CallKind      string             `json:"call_kind"`
		Time          time.Time          `json:"time"`
		Part          string             `json:"part"`
		File          string             `json:"file"`
		Seed          uint64             `json:"seed"`
		Rubric        []sample.Criterion `json:"rubric"`
		Readers       []ReaderRecord     `json:"readers"`
		Repeats       int                `json:"repeats"`
		MaxUSD        float64            `json:"max_usd"`
		Estimate      runner.Estimate    `json:"estimate"`
		Prompt        string             `json:"prompt"`
		PromptVersion int                `json:"prompt_version"`
		// Path is "two" for a run in two calls, the reading and the assessment of the text (T92), and
		// empty for one call; the prompt and its version are then those of the two calls.
		Path string `json:"path,omitempty"`
	}
	// ReaderRecord is a reader as the run's record tells it: its model and prices.
	ReaderRecord = runner.ReaderRecord
	// EssayRecord tells an essay of the run as the file of the essays has it.
	EssayRecord struct {
		Kind        string               `json:"kind"`
		Essay       int                  `json:"essay"`
		Part        string               `json:"part"`
		Unmet       []string             `json:"unmet"`
		Words       int                  `json:"words"`
		Misspelled  []sample.Misspelling `json:"misspelled"`
		LowerStarts int                  `json:"lower_starts"`
		Unended     bool                 `json:"unended"`
		Width       int                  `json:"width"`
		Height      int                  `json:"height"`
	}
	// CallRecord tells a call: the answer as the model gave it, what it comes to on each criterion
	// and how many of its judgements name no criterion, how it read the essay, what its feedback
	// breaks of the rule and how many words it has, the failure if it failed, and what it took. A
	// call whose request could not be built was not asked, and says so.
	CallRecord struct {
		Kind string `json:"kind"`
		Call
		NotAsked      bool     `json:"not_asked,omitempty"`
		Answer        string   `json:"answer,omitempty"`
		Marks         []Mark   `json:"marks"`
		Extra         int      `json:"extra,omitempty"`
		Reading       *Reading `json:"reading,omitempty"`
		Breaches      []string `json:"breaches,omitempty"`
		FeedbackWords int      `json:"feedback_words,omitempty"`
		Error         string   `json:"error,omitempty"`
		runner.Usage
	}
)

// Log is a run's log read back: the run, its essays, in the field Sheets of the runner's records,
// its calls in the order they were made, and how the run ended, or nil when the log stops before the
// end.
type Log = runner.Records[RunRecord, EssayRecord, CallRecord]

// ReadLog reads the log of a run of the assessment by the rubric back, as runner.ReadLog reads a
// log. A log of another kind of call is refused (runner.ReadRecords).
func ReadLog(r io.Reader) (*Log, error) {
	return runner.ReadItems[RunRecord, EssayRecord, CallRecord](r, CallKind, "assessment by the rubric", "essay")
}

// PathTwo is the path of a run in two calls, as its record tells it.
const PathTwo = "two"

// The output tokens a call of each step is expected to take, by configuration, for the estimate: a
// reading writes the essay down, about 150 tokens, besides what it thinks; an assessment of the text
// writes six judgements and the feedback, about 450, besides what it thinks. Opus at high effort
// thinks about 500 more in a reading and 1 100 in an assessment, Haiku with thinking about 400 and
// 450 (T92). A configuration without its output here is a mistake of the code.
var (
	readOutputs = map[string]int{"opus-high": 700, "haiku-low": 600, "haiku-low-no-thinking": 200,
		"opus-medium": 700, "sonnet-low": 500, "sonnet-low-no-thinking": 200, "haiku-medium": 800}
	textOutputs = map[string]int{"opus-high": 1600, "haiku-low": 900, "haiku-low-no-thinking": 450,
		"opus-medium": 1600, "sonnet-low": 1100, "sonnet-low-no-thinking": 450, "haiku-medium": 1200}
)

// The input tokens a call of each step is expected to take besides the crop: the instructions, the
// schema and the size of the crop for a reading; and for an assessment of the text, the instructions,
// the schema, the essay as text and the block of the task's data.
const (
	inputsOfReading = 350
	inputsOfText2   = 1000
)

// pairText is what the reading of an essay wrote down, which the assessment of the text after it in
// the run asks about; ok is false when the reading failed or its answer was off the schema.
type pairText struct {
	text string
	ok   bool
}

// readStep is a call of the reading of an essay on the runner.
type readStep struct {
	Call
	essay *sample.Essay
	pic   *Picture
	read  *pairText
}

// Request is the reading's request of the essay.
func (s *readStep) Request() (provider.Request, error) { return ReadRequest(s.pic), nil }

// Right is the reading of the model that is always right: the essay as it is written.
func (s *readStep) Right() (string, error) { return RightReading(s.essay), nil }

// Record is the reading's record: the answer as the model gave it and how it read the essay. Its text
// goes to the assessment of the text after it; a reading that wrote no text has failed, and its
// assessment is not asked.
func (s *readStep) Record(o runner.Outcome) any {
	rec := CallRecord{Kind: "call", Call: s.Call, NotAsked: o.NotAsked, Answer: o.Answer, Error: o.Error, Usage: o.Usage}
	if !o.NotAsked && o.Error == "" {
		text, err := ReadReading(o.Answer)
		if err == nil && strings.TrimSpace(text) == "" {
			err = fmt.Errorf("essay: the reading wrote no text")
		}
		if err != nil {
			rec.Error = err.Error()
			return rec
		}
		s.read.text, s.read.ok = text, true
		reading := ReadingOf(s.essay, text)
		rec.Reading = &reading
	}
	return rec
}

// assessStep is a call of the assessment of the text of an essay on the runner, after its reading.
type assessStep struct {
	Call
	essay  *sample.Essay
	rubric []sample.Criterion
	read   *pairText
}

// Request is the assessment's request of the text the reading wrote down; with no text, it is not
// asked.
func (s *assessStep) Request() (provider.Request, error) {
	if !s.read.ok {
		return provider.Request{}, fmt.Errorf("essay: the reading of essay %d failed, and there is no text to assess", s.Essay)
	}
	return TextRequest(s.essay, s.rubric, s.read.text), nil
}

// Right is the assessment of the model that is always right, by the plan.
func (s *assessStep) Right() (string, error) { return RightText(s.essay, s.rubric), nil }

// Record is the assessment's record: what it comes to on each criterion and what its feedback breaks
// of the rule. A call not asked, failed or off the schema leaves every criterion unchecked.
func (s *assessStep) Record(o runner.Outcome) any {
	rec := CallRecord{Kind: "call", Call: s.Call, NotAsked: o.NotAsked, Answer: o.Answer, Error: o.Error, Usage: o.Usage}
	var a *Answer
	if !o.NotAsked && o.Error == "" {
		var err error
		if a, err = ReadText(o.Answer, s.read.text); err != nil {
			rec.Error = err.Error()
		}
	}
	rec.Marks, rec.Extra = Marks(s.rubric, a)
	if a != nil {
		rec.Breaches, rec.FeedbackWords = Feedback(a.Feedback, s.essay)
	}
	return rec
}

// NewTwo plans a run of the assessment in two calls over the essays of a part, drawn from the seed,
// by the readers, repeated: for each essay, reader and repeat, the reading of its crop at 32 px and
// right after it the assessment of the text the reading wrote down (T92). The calls go repeat by
// repeat, as New puts them.
func NewTwo(part, file string, seed uint64, es *sample.Essays, essays []*sample.Essay, readers []Reader, repeats int) (*runner.Run, error) {
	for _, rd := range readers {
		if _, ok := readOutputs[rd.Name]; !ok {
			panic("essay: no output of a reading for the configuration " + rd.Name)
		}
		if _, ok := textOutputs[rd.Name]; !ok {
			panic("essay: no output of an assessment of the text for the configuration " + rd.Name)
		}
	}
	r, pics, err := prepare(part, file, seed, es, essays, readers, repeats, RunRecord{Prompt: TwoFingerprint(), PromptVersion: TwoVersion, Path: PathTwo})
	if err != nil {
		return nil, err
	}
	for rep := 1; rep <= repeats; rep++ {
		for i, e := range essays {
			for _, rd := range readers {
				read := &pairText{}
				call := Call{Essay: e.Number, Reader: rd.Name, Repeat: rep}
				reading, assessment := call, call
				reading.Step, assessment.Step = StepRead, StepAssess
				in := int64(inputsOfReading + render.Tokens(pics[i].Crop.Width, pics[i].Crop.Height))
				r.Steps = append(r.Steps,
					runner.Step{Config: rd.Config, In: in, Out: int64(readOutputs[rd.Name]), WorstIn: inputBudget, Call: &readStep{Call: reading, essay: e, pic: pics[i], read: read}},
					runner.Step{Config: rd.Config, In: inputsOfText2, Out: int64(textOutputs[rd.Name]), WorstIn: inputBudget, Call: &assessStep{Call: assessment, essay: e, rubric: es.Rubric, read: read}})
			}
		}
	}
	return r, nil
}

// prepare is what a run of the assessment draws before its calls, in one call or two: it refuses a
// run of no repeat or no essay, draws every essay once and cuts its crop at 32 px, records each essay,
// and records the run with its readers and the prompt, version and path given (prompt).
func prepare(part, file string, seed uint64, es *sample.Essays, essays []*sample.Essay, readers []Reader, repeats int, prompt RunRecord) (*runner.Run, []*Picture, error) {
	if repeats < 1 {
		return nil, nil, fmt.Errorf("essay: %d repeats", repeats)
	}
	if len(essays) == 0 {
		return nil, nil, fmt.Errorf("essay: no essay of the part %s", part)
	}
	r := &runner.Run{}
	pics := make([]*Picture, len(essays))
	for i, e := range essays {
		pic, err := PictureOf(e, seed, render.Digit)
		if err != nil {
			return nil, nil, fmt.Errorf("essay: essay %d: %w", e.Number, err)
		}
		pics[i] = pic
		r.Items = append(r.Items, essayRecord(e, pic))
	}
	records := make([]ReaderRecord, len(readers))
	for i, rd := range readers {
		records[i] = rd.Record()
	}
	r.Record = func(h runner.Head) any {
		return RunRecord{Kind: "run", CallKind: CallKind, Time: h.Time, Part: part, File: file, Seed: seed, Rubric: es.Rubric, Readers: records, Repeats: repeats,
			MaxUSD: h.MaxUSD, Estimate: h.Estimate, Prompt: prompt.Prompt, PromptVersion: prompt.PromptVersion, Path: prompt.Path}
	}
	return r, pics, nil
}
