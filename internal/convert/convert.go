// Package convert writes the sheets of tutor-sample v0, the files the record mode of the board saved
// until T105, as samples of spec's eval protocol, the message Sample of novatrail.eval.v1 (R75): the
// author's sheets as a person's, at random by ID and by the author's writer's code, and the sheets
// code drew as the samples tutor-eval draw draws, to the byte, once the same code is found to draw
// them from the seed. It is the one reader of tutor-sample v0 left: the pressure of a point, which no
// sample of the protocol holds, it reads only to tell a mouse from a pen.
package convert

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"

	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"

	"github.com/altnovatrail/tutor/internal/evalsample"
	"github.com/altnovatrail/tutor/internal/protocol"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/synth"
)

// Mouse is the pressure that a device without pressure, a mouse, reports at every point while its
// button is down (Pointer Events), and that the record page kept so.
const Mouse = 0.5

// file is a sample of tutor-sample v0 as its files hold it: the sheet of internal/sample with the
// pressure of each point.
type file struct {
	Format    string            `json:"format"`
	Version   *int              `json:"version"`
	Sheet     sample.Sheet      `json:"sheet"`
	Task      string            `json:"task"`
	Writer    sample.Writer     `json:"writer"`
	Scenario  sample.Scenario   `json:"scenario"`
	Strokes   []stroke          `json:"strokes"`
	Erasures  []sample.Erasure  `json:"erasures"`
	Lines     []sample.Line     `json:"lines"`
	Injection *sample.Injection `json:"injection"`
	Expected  sample.Expected   `json:"expected"`
}

type stroke struct {
	ID     string  `json:"id"`
	Points []point `json:"points"`
}

type point struct {
	X   float64 `json:"x"`
	Y   float64 `json:"y"`
	TMs int64   `json:"t_ms"`
	// Pressure is from 0 to 1; a mouse gives Mouse.
	Pressure float64 `json:"pressure"`
}

// Read reads a sample of tutor-sample v0 as strictly as the format was always read, every field
// present and no other, checks it on its own (sample.Check), and gives the sheet without the
// pressure, with the pointer that wrote it: "mouse" when every point has the pressure Mouse, and
// "pen" otherwise. An error from a sample that fails the check is a *sample.FormatError.
func Read(r io.Reader) (*sample.Sample, string, error) {
	var f file
	if err := sample.ReadStrict(r, "sample", &f); err != nil {
		return nil, "", err
	}
	s := &sample.Sample{Format: f.Format, Version: f.Version, Sheet: f.Sheet, Task: f.Task, Writer: f.Writer, Scenario: f.Scenario,
		Strokes: make([]sample.Stroke, len(f.Strokes)), Erasures: f.Erasures, Lines: f.Lines, Injection: f.Injection, Expected: f.Expected}
	mouse, points := true, 0
	var off []sample.Problem
	for i, st := range f.Strokes {
		s.Strokes[i] = sample.Stroke{ID: st.ID, Points: make([]sample.Point, len(st.Points))}
		for k, p := range st.Points {
			s.Strokes[i].Points[k] = sample.Point{X: p.X, Y: p.Y, TMs: p.TMs}
			if !(p.Pressure >= 0 && p.Pressure <= 1) {
				off = append(off, sample.Problem{Path: fmt.Sprintf("strokes[%d].points[%d].pressure", i, k), Message: fmt.Sprintf("%v is not from 0 to 1", p.Pressure)})
			}
			mouse = mouse && p.Pressure == Mouse
			points++
		}
	}
	// Every problem is named, the pressure's with the sheet's own check's.
	if err := s.Check(); err != nil || len(off) > 0 {
		var fe *sample.FormatError
		if err != nil && !errors.As(err, &fe) {
			return nil, "", err
		}
		all := &sample.FormatError{What: "sample", Problems: off}
		if fe != nil {
			all.Problems = append(all.Problems, fe.Problems...)
		}
		return nil, "", all
	}
	if mouse && points > 0 {
		return s, "mouse", nil
	}
	return s, "pen", nil
}

// Options are what a conversion takes besides the file: for a sheet code drew, the path of its plan's
// file, which its ID is of, the seed and the share of the shift it was drawn with (evalsample.ID);
// for a sheet a person wrote, where its ID and its writer's code come from (evalsample.NewID and
// evalsample.WriterCodeAt, so a file is asked for only when a person's sheet is converted).
type Options struct {
	Plan  string
	Seed  uint64
	Shift float64
	ID    func() (string, error)
	Code  func() (string, error)
}

// ErrOtherDrawing is a sheet that code drew which the code of today, from the seed given, does not
// draw the same: another seed, another share of the shift, or a hand drawn before a change of
// internal/synth. Its sample is not written, since it would not be what its ID says.
var ErrOtherDrawing = errors.New("the code of today does not draw the sheet so from the seed: it was drawn with another seed or share of the shift, or before a change of the drawing; draw it again with tutor-eval draw")

// Converted is a sheet written as a sample of the protocol: the file's bytes, and what a summary
// names of it without the handwriting — who wrote it, the pointer, and its outcome as the verdict of
// module-kit on its truth gives it, by the task's key.
type Converted struct {
	Data    []byte
	Writer  sample.Writer
	Pointer string
	Outcome sample.Outcome
}

// Sheet writes a file of tutor-sample v0 for its plan sheet as a sample of the protocol, with its
// bank, and reads the sample back as the runner will, refusals and all (evalsample.Read):
//   - a sheet a person wrote is a person's sample (evalsample.PersonOf), with its strokes as they were
//     written, by the pointer that wrote them;
//   - a sheet code drew is the sample tutor-eval draw writes for it (evalsample.Of), once the code of
//     today draws it the same from the seed; otherwise ErrOtherDrawing.
//
// A sheet of another task or scenario than its plan sheet's, one that does not pass the check with
// the bank, and one whose markup the protocol does not take — a stroke in no part, an inscription to
// steer the model, a whole solution whose truth its markup does not give — is an error, and nothing
// of it is written.
func Sheet(data []byte, sh *sample.PlanSheet, b *evalsample.Bank, o Options) (*Converted, error) {
	s, pointer, err := Read(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if err := s.CheckWithBank(b.Bank); err != nil {
		return nil, err
	}
	switch {
	case s.Task != sh.Task:
		return nil, fmt.Errorf("its task is %s, and its plan sheet's %s", s.Task, sh.Task)
	case s.Scenario != sh.Scenario:
		return nil, fmt.Errorf("its scenario is %s, and its plan sheet's %s", s.Scenario, sh.Scenario)
	}
	var es *evalv1.Sample
	if s.Writer == sample.WriterSynthetic {
		es, err = drawn(s, sh, b, o)
		pointer = evalsample.Pointer
	} else {
		es, err = person(s, sh, b, pointer, o)
	}
	if err != nil {
		return nil, err
	}
	out, err := protocol.MarshalSample(es)
	if err != nil {
		return nil, err
	}
	back, err := protocol.ReadSample(bytes.NewReader(out))
	if err != nil {
		return nil, err
	}
	read, refused, err := evalsample.Read(back, sh, b)
	switch {
	case err != nil:
		return nil, err
	case refused != nil:
		return nil, fmt.Errorf("the runner would refuse its sample: %w", refused)
	}
	return &Converted{Data: out, Writer: s.Writer, Pointer: pointer, Outcome: read.Sample.Expected.Outcome}, nil
}

// drawn is a sheet code drew as the sample tutor-eval draw writes for it: the same sheet, drawn again
// from the seed by the code of today, by Of.
func drawn(s *sample.Sample, sh *sample.PlanSheet, b *evalsample.Bank, o Options) (*evalv1.Sample, error) {
	again := synth.SheetLeaning(sh, o.Seed, o.Shift)
	if !reflect.DeepEqual(again, s) {
		return nil, ErrOtherDrawing
	}
	id, err := evalsample.ID(o.Plan, o.Seed, o.Shift, sh.Number)
	if err != nil {
		return nil, err
	}
	return evalsample.Of(again, sh, b, o.Seed, id)
}

// person is a sheet a person wrote as a person's sample, its strokes by the pointer that wrote them.
func person(s *sample.Sample, sh *sample.PlanSheet, b *evalsample.Bank, pointer string, o Options) (*evalv1.Sample, error) {
	h, ids, err := evalsample.HistoryOf(s, pointer)
	if err != nil {
		return nil, err
	}
	code, err := o.Code()
	if err != nil {
		return nil, err
	}
	id, err := o.ID()
	if err != nil {
		return nil, err
	}
	return evalsample.PersonOf(s, sh, b, h, ids, id, code)
}
