package essay

import (
	"encoding/json"
	"fmt"
	"io"
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

// Readers are the three configurations of the runner. The text of an essay, six judgements and the
// feedback come to about 600 tokens; Haiku with thinking writes about 600 more, and Opus at high
// effort about 1 500. Opus assesses in every repeat, not once: a judgement by a rubric changes from
// call to call more than the reading of a digit (R45).
func Readers() []Reader {
	return runner.ReadersOf("essay", map[string]int{"opus-high": 2100, "haiku-low": 1200, "haiku-low-no-thinking": 600})
}

// The input tokens a call is expected to take, for the estimate: the instructions, the schema, the
// text and the block of the task's data, about 900 in all, and the crop's own, ⌈w / 28⌉ × ⌈h / 28⌉
// (render.Tokens). inputBudget is a generous count of them, for the worst case: a crop takes 1 568 at
// most.
const (
	inputsOfText = 900
	inputBudget  = 4000
)

// Call is one call of a run: an essay by its number, a reader by its name, and the repeat, from 1.
type Call struct {
	Essay  int    `json:"essay"`
	Reader string `json:"reader"`
	Repeat int    `json:"repeat"`
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
	if repeats < 1 {
		return nil, fmt.Errorf("essay: %d repeats", repeats)
	}
	if len(essays) == 0 {
		return nil, fmt.Errorf("essay: no essay of the part %s", part)
	}
	r := &runner.Run{}
	pics := make([]*Picture, len(essays))
	for i, e := range essays {
		pic, err := PictureOf(e, seed, render.Digit)
		if err != nil {
			return nil, fmt.Errorf("essay: essay %d: %w", e.Number, err)
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
			MaxUSD: h.MaxUSD, Estimate: h.Estimate, Prompt: Fingerprint(), PromptVersion: Version}
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
	// rubric, its readers and repeats, its cap and estimate, and the fingerprint and the version of
	// its prompt.
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
