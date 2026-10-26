// Package sample is the format of the reading measurement's samples, of its bank of tasks, of its
// sheet plan and of the split of its sheets: their types, how they are read from JSON and how they
// are checked. A sample is one sheet of handwriting: every stroke with the time and pressure of its
// points, in the order it was written, every erasure, and the markup made while writing. The bank
// holds the tasks the sheets solve, each with its key, its solution line by line, its traps, its
// leak phrases and its false keys. The plan says what the writer writes on each sheet, and with it
// the markup. The split divides the recorded sheets into a development and a test part, and names
// each by its saved file, whose name says the sheet and the time of the save.
//
// The format is tutor's own until spec's eval protocol replaces it (В7 in RUN.md); its fields
// are explained in docs/architecture.md, section 3.1. A reader takes only the version it knows
// and refuses unknown fields, so a file written for another version fails loudly rather than
// losing what it holds.
package sample

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// The names and the version a file declares in its "format" and "version" fields.
const (
	SampleFormat = "tutor-sample"
	BankFormat   = "tutor-bank"
	PlanFormat   = "tutor-sheet-plan"
	Version      = 0
)

// Sample is one sheet of handwriting with its markup.
type Sample struct {
	Format  string `json:"format"`
	Version *int   `json:"version"`
	// Sheet is the size of the sheet in logical units, the same on every device: x grows to the
	// right, y grows downwards, and one unit is one pixel of the measurement's renders at scale 1.
	Sheet Sheet `json:"sheet"`
	// Task is the ID of the task in the bank that the sheet solves.
	Task     string   `json:"task"`
	Writer   Writer   `json:"writer"`
	Scenario Scenario `json:"scenario"`
	// Strokes are in the order they were written.
	Strokes  []Stroke  `json:"strokes"`
	Erasures []Erasure `json:"erasures"`
	// Lines are the lines of the solution, in the order the writer meant them to be read. The
	// markup refers to them by number, from 1.
	Lines []Line `json:"lines"`
	// Injection is the inscription written to steer the model, on sheets of the scenario
	// "injection" only.
	Injection *Injection `json:"injection"`
	Expected  Expected   `json:"expected"`
}

// Sheet is the size of a sheet in logical units.
type Sheet struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// Stroke is one line from pen down to pen up.
type Stroke struct {
	// ID is unique within the sample, such as "s-0031".
	ID     string  `json:"id"`
	Points []Point `json:"points"`
}

// Point is one sample of the pen.
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	// TMs is the time in milliseconds since the sheet was opened.
	TMs int64 `json:"t_ms"`
	// Pressure is from 0 to 1; a device that has none, a mouse, gives 0.5.
	Pressure float64 `json:"pressure"`
}

// Erasure is one touch of the eraser. The eraser removes whole strokes, never a part of one.
type Erasure struct {
	TMs     int64    `json:"t_ms"`
	Strokes []string `json:"strokes"`
	// Planned is the markup of an erasure the scenario called for; any other erasure is a slip
	// of the pen and has none.
	Planned *PlannedErasure `json:"planned"`
}

// PlannedErasure says what an erasure of the scenario removed.
type PlannedErasure struct {
	// Line is the number of the line written in place of what was erased.
	Line int `json:"line"`
	// Erased is what the erased strokes were: a mistake fixed afterwards, or a correct step
	// replaced by a wrong one.
	Erased Erased `json:"erased"`
	// Text is what the erased strokes said.
	Text string `json:"text"`
}

// Line is one line of the solution as the writer meant it.
type Line struct {
	// Text is what the writer meant to write, in the notation of the bank's solutions.
	Text    string   `json:"text"`
	Strokes []string `json:"strokes"`
	// Illegible marks a line written illegibly on purpose: reading it should ask to write it
	// again.
	Illegible bool `json:"illegible"`
	// CrossedOut marks a line the writer struck through: it is not part of the solution.
	CrossedOut bool `json:"crossed_out"`
	// Answer marks the line that gives the final answer, wherever it stands.
	Answer bool `json:"answer"`
}

// Injection is a handwritten inscription that tries to steer the model.
type Injection struct {
	Text    string   `json:"text"`
	Lang    Lang     `json:"lang"`
	Strokes []string `json:"strokes"`
}

// Expected is the outcome the writer meant the sheet to have.
type Expected struct {
	Outcome Outcome `json:"outcome"`
	// FirstError is the number of the first wrong line, on sheets whose outcome is "error".
	FirstError *int `json:"first_error"`
	// Trap is the ID of the bank task's trap that the mistake falls into, if any.
	Trap string `json:"trap"`
}

// Writer is who wrote a sheet.
type Writer string

// The writers of the measurement: the author, in their own hand or imitating a child's (R08).
const (
	WriterAuthor        Writer = "author"
	WriterAuthorAsChild Writer = "author-as-child"
)

// Scenario is what a sheet was written to test (docs/architecture.md, section 3.1).
type Scenario string

// The scenarios of section 3.1.
const (
	ScenarioCorrect        Scenario = "correct"
	ScenarioError          Scenario = "error"
	ScenarioSimilarDigits  Scenario = "similar-digits"
	ScenarioIllegible      Scenario = "illegible-crossed-out"
	ScenarioAnswerNotLast  Scenario = "answer-not-last"
	ScenarioNearlyEmpty    Scenario = "nearly-empty"
	ScenarioMirroredDigits Scenario = "mirrored-digits"
	ScenarioErasureFixed   Scenario = "erasure-fixed"
	ScenarioErasureBroken  Scenario = "erasure-broken"
	ScenarioInjection      Scenario = "injection"
)

// Scenarios lists every scenario, in the order of section 3.1.
var Scenarios = []Scenario{
	ScenarioCorrect, ScenarioError, ScenarioSimilarDigits, ScenarioIllegible, ScenarioAnswerNotLast,
	ScenarioNearlyEmpty, ScenarioMirroredDigits, ScenarioErasureFixed, ScenarioErasureBroken,
	ScenarioInjection,
}

// Erased is what a planned erasure removed.
type Erased string

// The two planned erasures of R11.
const (
	// ErasedMistake: a mistake was erased and fixed; the result is correct.
	ErasedMistake Erased = "mistake"
	// ErasedCorrectStep: a correct step was erased and replaced by a wrong one.
	ErasedCorrectStep Erased = "correct-step"
)

// Outcome is the outcome a sheet is meant to have.
type Outcome string

// The outcomes.
const (
	// OutcomeCorrect: every line of the solution is correct, and so is the answer.
	OutcomeCorrect Outcome = "correct"
	// OutcomeError: a line is wrong; Expected.FirstError names the first one.
	OutcomeError Outcome = "error"
	// OutcomeUnreadable: the answer is illegible on purpose, and reading it should ask to write it
	// again.
	OutcomeUnreadable Outcome = "unreadable"
	// OutcomeNoAnswer: the sheet gives no answer at all.
	OutcomeNoAnswer Outcome = "no-answer"
)

// Lang is the language of a text.
type Lang string

// The languages of the measurement's texts.
const (
	LangRussian Lang = "ru"
	LangEnglish Lang = "en"
)

// ReadSample reads a sample from JSON and checks it on its own; CheckWithBank checks it against
// its task. An error from a sample that fails the check is a *FormatError.
func ReadSample(r io.Reader) (*Sample, error) {
	var s Sample
	if err := readStrict(r, "sample", &s); err != nil {
		return nil, readError("sample", err)
	}
	if err := s.Check(); err != nil {
		return nil, err
	}
	return &s, nil
}

// ReadBank reads a bank of tasks from JSON and checks it. An error from a bank that fails the
// check is a *FormatError.
func ReadBank(r io.Reader) (*Bank, error) {
	var b Bank
	if err := readStrict(r, "bank", &b); err != nil {
		return nil, readError("bank", err)
	}
	if err := b.Check(); err != nil {
		return nil, err
	}
	return &b, nil
}

// ReadPlan reads a sheet plan from JSON and checks it on its own; CheckWithBank checks it against
// the bank. An error from a plan that fails the check is a *FormatError.
func ReadPlan(r io.Reader) (*Plan, error) {
	var pl Plan
	if err := readStrict(r, "plan", &pl); err != nil {
		return nil, readError("plan", err)
	}
	if err := pl.Check(); err != nil {
		return nil, err
	}
	return &pl, nil
}

// Marshal writes a sample, a bank, a plan or a split as indented JSON, ending with a newline.
func Marshal(v any) ([]byte, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// readError passes a *FormatError on as it is and names the file's kind on any other error.
func readError(what string, err error) error {
	var fe *FormatError
	if errors.As(err, &fe) {
		return err
	}
	return fmt.Errorf("read %s: %w", what, err)
}

// Problem is one thing wrong with a file: where it is, as a path of JSON field names and array
// indexes, and what is wrong.
type Problem struct {
	Path    string
	Message string
}

// FormatError lists every problem a check found, not only the first.
type FormatError struct {
	// What names the file's kind: "sample", "bank" or "plan".
	What     string
	Problems []Problem
}

// Error names the file's kind and lists the problems, one to a line when there are several.
func (e *FormatError) Error() string {
	var b bytes.Buffer
	if len(e.Problems) == 1 {
		fmt.Fprintf(&b, "invalid %s: %s", e.What, e.Problems[0])
		return b.String()
	}
	fmt.Fprintf(&b, "invalid %s, %d problems:", e.What, len(e.Problems))
	for _, p := range e.Problems {
		fmt.Fprintf(&b, "\n  %s", p)
	}
	return b.String()
}

func (p Problem) String() string {
	if p.Path == "" {
		return p.Message
	}
	return p.Path + ": " + p.Message
}

// problems collects what a check finds.
type problems struct {
	what string
	list []Problem
}

func (p *problems) add(path, format string, args ...any) {
	p.list = append(p.list, Problem{Path: path, Message: fmt.Sprintf(format, args...)})
}

func (p *problems) err() error {
	if len(p.list) == 0 {
		return nil
	}
	return &FormatError{What: p.what, Problems: p.list}
}
