// Package paperset is the eval set of a task from paper and its three runs (R135): the reading of a
// statement into a formula, the solution of a task by AI, and the reading of the work on a task
// against its key. A case is a task of one of five kinds, with its truth by construction — the
// formula, the key, what the learner writes and the verdict it comes to —, and its pictures are
// drawn from it when a run asks them, the same from the same case on any machine, so the set is one
// file and no folder of samples; a folder of the cases the debug mode exported holds its pictures
// instead (R136). The printed statement is drawn by synth's hand, as the learner's
// writing is: the runs on the stub measure the plumbing, and a paid run needs photos of printed
// pages (Д22).
package paperset

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"

	paperv1 "github.com/altnovatrail/spec/contracts/novatrail/paper/v1"

	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/synth"
)

// The kinds of a case: an expression and an equation, which are formulas; a word problem with a
// number for its answer, a choice among printed options, both no formula; and an open task, a
// drawing, which has no one answer and so no key.
const (
	KindExpression = "expression"
	KindEquation   = "equation"
	KindWord       = "word"
	KindChoice     = "choice"
	KindOpen       = "open"
)

// Kinds are the kinds of a case, in the order the generator makes them.
var Kinds = []string{KindExpression, KindEquation, KindWord, KindChoice, KindOpen}

// The outcomes a case's work comes to: the verdicts of module-kit by the key, and unchecked for a
// task with no key.
const (
	OutcomeCorrect    = "correct"
	OutcomeNotYet     = "not_yet"
	OutcomeUnreadable = "unreadable"
	OutcomeUnchecked  = "unchecked"
)

// Set is the file of the set: what it is, the seed it was made from, and its cases, numbered from 1.
type Set struct {
	Description string `json:"description"`
	Seed        uint64 `json:"seed"`
	Cases       []Case `json:"cases"`
}

// Case is a task of the set: its number, its part of the measurement, its kind, the lines of its
// statement as printed, the formula the reading of the statement gives — empty for no formula —,
// the key — none for an open task —, the line the learner writes under the task — empty for none
// —, the value a reader who is always right reads in it, and the verdict it comes to by the key.
type Case struct {
	Number    int      `json:"number"`
	Part      string   `json:"part"`
	Kind      string   `json:"kind"`
	Statement []string `json:"statement"`
	Formula   string   `json:"formula"`
	Key       *Key     `json:"key"`
	Work      string   `json:"work"`
	Read      string   `json:"read"`
	Outcome   string   `json:"outcome"`
	// pictures are those of a case the debug mode exported, which no code draws.
	pictures *Pictures
}

// Key is a case's key as the file writes it: a number with its units and the letter of its
// unknown, or a choice with its options and the right one.
type Key struct {
	Number  string   `json:"number,omitempty"`
	Units   []string `json:"units,omitempty"`
	Unknown string   `json:"unknown,omitempty"`
	Options []string `json:"options,omitempty"`
	Choice  string   `json:"choice,omitempty"`
}

// Answer is the key as the contract's Answer, or nil for none.
func (k *Key) Answer() *paperv1.Answer {
	switch {
	case k == nil:
		return nil
	case k.Choice != "":
		return &paperv1.Answer{Kind: &paperv1.Answer_Choice{Choice: &paperv1.Choice{Options: slices.Clone(k.Options), Key: k.Choice}}}
	}
	return &paperv1.Answer{Kind: &paperv1.Answer_Number{Number: &paperv1.Number{Value: k.Number, Units: slices.Clone(k.Units), Unknown: k.Unknown}}}
}

// Read reads the file of a set strictly, or a folder of the cases the debug mode exported (exported):
// no field it does not name, the cases numbered from 1 in order, each of a known kind, part and
// outcome, with a statement, or the pictures of an exported case, a key exactly when it is not open,
// and a formula only of a formula's kind.
func Read(path string) (*Set, error) {
	var s Set
	if info, err := os.Stat(path); err != nil {
		return nil, err
	} else if info.IsDir() {
		e, err := exported(path)
		if err != nil {
			return nil, err
		}
		s = *e
	} else {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&s); err != nil {
			return nil, fmt.Errorf("%s: not a set of paper: %w", path, err)
		}
	}
	if len(s.Cases) == 0 {
		return nil, fmt.Errorf("%s: a set of paper with no case", path)
	}
	for i, c := range s.Cases {
		var problems []string
		if c.Number != i+1 {
			problems = append(problems, fmt.Sprintf("the number is %d", c.Number))
		}
		if !slices.Contains(Kinds, c.Kind) {
			problems = append(problems, fmt.Sprintf("the kind %q is none of the set's", c.Kind))
		}
		if c.Part != sample.PartDev && c.Part != sample.PartTest {
			problems = append(problems, fmt.Sprintf("the part %q is neither dev nor test", c.Part))
		}
		if !slices.Contains([]string{OutcomeCorrect, OutcomeNotYet, OutcomeUnreadable, OutcomeUnchecked}, c.Outcome) {
			problems = append(problems, fmt.Sprintf("the outcome %q is none of the set's", c.Outcome))
		}
		if (c.Work == "") != (c.Read == "") {
			problems = append(problems, "it has a value read exactly when it has work")
		}
		if len(c.Statement) == 0 && c.pictures == nil {
			problems = append(problems, "it has no statement")
		}
		if (c.Key == nil) != (c.Kind == KindOpen) || (c.Key == nil) != (c.Outcome == OutcomeUnchecked) {
			problems = append(problems, "it has a key exactly when it is not open, and is unchecked then alone")
		}
		if (c.Formula != "") != (c.Kind == KindExpression || c.Kind == KindEquation) {
			problems = append(problems, "it has a formula exactly when it is an expression or an equation")
		}
		if len(problems) > 0 {
			return nil, fmt.Errorf("%s: case %d: %s", path, i+1, strings.Join(problems, "; "))
		}
	}
	return &s, nil
}

// Of are the cases of a part, dev or test, or all of them for all.
func (s *Set) Of(part string) []Case {
	if part == "all" {
		return s.Cases
	}
	var out []Case
	for _, c := range s.Cases {
		if c.Part == part {
			out = append(out, c)
		}
	}
	return out
}

// The sheet the pictures of a case are drawn on.
const (
	sheetWidth  = 1000
	sheetHeight = 600
)

// Pictures are what a run shows of a case: the PNG of the task's crop, the statement alone, and of
// the crop of the work, the statement with the learner's line under it, each with its size.
type Pictures struct {
	Statement, Work picture
}

type picture struct {
	// A picture is a PNG, or the JPEG of the task's crop of an exported case, as provider.Photo
	// sends a photo.
	PNG, JPEG     []byte
	Width, Height int
}

// Draw draws a case's pictures from the set's seed: the statement in one hand, and with the work
// under it the same lines in the same hand, so that the work's crop holds the statement as the
// tracing paper does.
func Draw(c Case, seed uint64) (*Pictures, error) {
	if c.pictures != nil {
		return c.pictures, nil
	}
	sheet := sample.Sheet{Width: sheetWidth, Height: sheetHeight}
	of := func(lines []string) (picture, error) {
		strokes := synth.Text(lines, sheet, seed+uint64(c.Number))
		crop, err := render.CropOf(strokes, nil, render.Digit)
		if err != nil {
			return picture{}, err
		}
		png, err := crop.PNG(strokes)
		return picture{PNG: png, Width: crop.Width, Height: crop.Height}, err
	}
	statement, err := of(c.Statement)
	if err != nil {
		return nil, fmt.Errorf("case %d: the statement: %w", c.Number, err)
	}
	work := statement
	if c.Work != "" {
		if work, err = of(append(slices.Clone(c.Statement), c.Work)); err != nil {
			return nil, fmt.Errorf("case %d: the work: %w", c.Number, err)
		}
	}
	return &Pictures{Statement: statement, Work: work}, nil
}
