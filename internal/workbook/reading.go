package workbook

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/value"
	"github.com/altnovatrail/tutor/provider"
)

// The reading of a page of the workbook (T79; docs/architecture.md, 3.4, «Рабочая тетрадь»): one
// call a page, which shows the model the crop of every filled box, each labelled with its place,
// item and slot, and asks what each box holds, as it is written. The program gives each box its
// verdict by the box's check, and each item by its boxes (R56 spec); a blank box goes to no model,
// and its verdict is unreadable, blank (R53 spec, amendment of 2026-10-27).

// Version is the version of the prompt of the reading of the workbook, frozen before the test of the
// run that measures it, as the reading's was (T73). Version 1, frozen in T80, read a digit written
// mirrored as that digit (R56 spec, amendment of 2026-10-28); version 2 has the reader mark it, and
// the box is then unreadable, so that the learner writes it again (R56 spec, amendment of
// 2026-10-29; T99). MirroredVersion is the first version that marks it.
const (
	Version         = 2
	MirroredVersion = 2
)

// Condition is what a reading of a page is given besides the crops: the bare crops, or each with
// the line of its item as the learner sees it, a caption (T80).
type Condition string

// The conditions of the reading of a page.
const (
	Bare      Condition = "bare"
	Captioned Condition = "captioned"
)

// The instructions of the reading: base, which every request has, and captionBlock, which a
// captioned request adds.
const (
	base = `You read a learner's answers handwritten in the boxes of a workbook page. Each image shows what is written in one box, cut out of the page: the frame of the box and the printed line around it are not shown. Write down what each box holds, as it is written: your answer is an observation, and a program decides from it whether each answer is right. Do not decide it yourself, do not correct a mistake and do not finish what is unfinished.

1. Give one cell for each image, in the order of the images. "item" and "slot" are the numbers the image's label gives: item 3, slot 2 is the second box of the third item.
2. "text" is the answer in the box exactly as it is written: its digits, and a fraction as the numerator, a slash and the denominator, "3/4", whether it is written with a slash or as one number over a bar over another. Leave out writing that is crossed out. Write a digit written back to front as the digit it is the right way round.
3. "legible" is false when you cannot make out the answer for sure. "other_readings" lists the other ways the answer could be read where a digit of it could be read otherwise; it is empty when there are none.
4. "mirrored" is true when a digit in the box is written back to front, as in a mirror, and false when none is.
5. "blank" is true when the box holds no answer at all, and "text" is then empty.`
	captionBlock = `Each image's label also gives the line of its item as the learner sees it, with [ ] for each box. Read only what the image shows.`
)

// The label of a crop and the text of a request.
const (
	labelFormat   = "Image %d: item %d, slot %d."
	captionFormat = "Image %d: item %d, slot %d, of the line «%s»."
	textFormat    = "The page has %d boxes with an answer, one in each image."
)

// Instructions are the instructions of a request: base, and the block of the captions when it has
// them.
func Instructions(captioned bool) string {
	if captioned {
		return base + "\n\n" + captionBlock
	}
	return base
}

// Fingerprint names the prompt: the SHA-256 of every word a reader reads besides those of the
// crops and the lines of the items: the instructions with and without the captions, the labels, the
// text and the schema. The log of a run keeps it.
func Fingerprint() string {
	words := []string{Instructions(false), Instructions(true), labelFormat, captionFormat, textFormat, string(schemaProperties)}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(words, "\x00"))))
}

// Request is what a reading asks of a model about the filled boxes of a page, in a condition: the
// instructions, each crop after its label, and the text. The model, its effort, whether it thinks
// and how many tokens it may write the caller sets.
func Request(pg *sample.WorkbookPage, cells []Cell, c Condition) provider.Request {
	images := make([]provider.Image, len(cells))
	for i, cell := range cells {
		label := fmt.Sprintf(labelFormat, i+1, cell.Item, cell.Slot)
		if c == Captioned {
			label = fmt.Sprintf(captionFormat, i+1, cell.Item, cell.Slot, pg.Items[cell.Item-1].Text)
		}
		images[i] = provider.Image{Label: label, PNG: cell.PNG}
	}
	return provider.Request{
		Instructions: Instructions(c == Captioned),
		Images:       images,
		Text:         fmt.Sprintf(textFormat, len(cells)),
		SchemaName:   "cells",
		Schema:       Schema,
	}
}

// schemaProperties are the properties of the answer, as JSON, in the order a model writes them.
var schemaProperties = json.RawMessage(`{
		"cells": {"type": "array", "items": {"type": "object", "additionalProperties": false, "required": ["item", "slot", "text", "legible", "other_readings", "mirrored", "blank"], "properties": {
			"item": {"type": "integer"},
			"slot": {"type": "integer"},
			"text": {"type": "string"},
			"legible": {"type": "boolean"},
			"other_readings": {"type": "array", "items": {"type": "string"}},
			"mirrored": {"type": "boolean"},
			"blank": {"type": "boolean"}
		}}}
	}`)

// Schema is the answer every reader of a page returns.
var Schema = map[string]any{
	"type":                 "object",
	"additionalProperties": false,
	"required":             []string{"cells"},
	"properties":           schemaProperties,
}

// Reading is what a model read in a box: its place, the answer as it is written, whether it is
// legible, how else it could be read, whether a digit of it is written mirrored, and whether the box
// is blank.
type Reading struct {
	Item          int      `json:"item"`
	Slot          int      `json:"slot"`
	Text          string   `json:"text"`
	Legible       bool     `json:"legible"`
	OtherReadings []string `json:"other_readings"`
	Mirrored      bool     `json:"mirrored"`
	Blank         bool     `json:"blank"`
}

// Read takes a model's answer apart as strictly as the schema: the cells and nothing else, every
// field of each. Which cells it holds, Verdicts weighs; the answer as the model gave it is the
// run's to keep.
func Read(answer string) ([]Reading, error) {
	var raw struct {
		Cells *[]struct {
			Item          *int      `json:"item"`
			Slot          *int      `json:"slot"`
			Text          *string   `json:"text"`
			Legible       *bool     `json:"legible"`
			OtherReadings *[]string `json:"other_readings"`
			Mirrored      *bool     `json:"mirrored"`
			Blank         *bool     `json:"blank"`
		} `json:"cells"`
	}
	dec := json.NewDecoder(bytes.NewReader([]byte(answer)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("workbook: the answer is not the cells of a page: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("workbook: the answer has more after the cells")
	}
	if raw.Cells == nil {
		return nil, errors.New("workbook: the answer has no cells")
	}
	out := []Reading{}
	for i, c := range *raw.Cells {
		if c.Item == nil || c.Slot == nil || c.Text == nil || c.Legible == nil || c.OtherReadings == nil || c.Mirrored == nil || c.Blank == nil {
			return nil, fmt.Errorf("workbook: cell %d lacks a field", i+1)
		}
		out = append(out, Reading{Item: *c.Item, Slot: *c.Slot, Text: *c.Text, Legible: *c.Legible, OtherReadings: *c.OtherReadings, Mirrored: *c.Mirrored, Blank: *c.Blank})
	}
	return out, nil
}

// Reason is why a box is unreadable or unchecked: of the reading, or of the value as the check reads
// it.
type Reason string

// The reasons of the reading; the value's are value.Blank and value.Notation.
const (
	Illegible Reason = "illegible"
	Ambiguous Reason = "ambiguous"
	// Mirrored is a box with a digit the reader says is written mirrored: ambiguous by the rule of a
	// digit written mirrored, and the learner writes it again (R56 spec, amendment of 2026-10-29).
	Mirrored Reason = "mirrored"
	// Missing is a filled box the answer has no cell for, and Twice one it has two cells for: both
	// make the box unchecked, a failure of the reading.
	Missing Reason = "missing"
	Twice   Reason = "twice"
)

// Box is the verdict of a box of a page: its place, the answer read in it, and its verdict with
// the reason or the trap.
type Box struct {
	Item    int           `json:"item"`
	Slot    int           `json:"slot"`
	Text    *string       `json:"text,omitempty"`
	Verdict value.Verdict `json:"verdict"`
	Reason  Reason        `json:"reason,omitempty"`
	Trap    string        `json:"trap,omitempty"`
}

// Verdicts are the program's verdicts on the boxes of a page, in its order, from what a model read
// in them, nil for a failed call:
//   - a blank box, which no model is shown, is unreadable, blank;
//   - a failed call leaves every other box unchecked, and so does a box the answer has no cell for,
//     or two;
//   - a box read blank is unreadable, blank; one with a digit read mirrored, unreadable, mirrored;
//     one read illegible or with other readings, unreadable;
//   - otherwise the box's check judges the answer: correct, not yet right with its trap, or
//     unreadable when the answer is not in the notation of its kind (R56 spec).
//
// A cell of a place the page has no filled box for is passed over: it judges nothing.
func Verdicts(pg *sample.WorkbookPage, read []Reading) []Box {
	type place struct{ item, slot int }
	cells := map[place][]Reading{}
	for _, r := range read {
		cells[place{r.Item, r.Slot}] = append(cells[place{r.Item, r.Slot}], r)
	}
	var out []Box
	for i, it := range pg.Items {
		for k, b := range it.Boxes {
			box := Box{Item: i + 1, Slot: k + 1}
			got := cells[place{i + 1, k + 1}]
			switch {
			case b.Written == nil:
				box.Verdict, box.Reason = value.Unreadable, Reason(value.Blank)
			case read == nil:
				box.Verdict = value.Unchecked
			case len(got) == 0:
				box.Verdict, box.Reason = value.Unchecked, Missing
			case len(got) > 1:
				box.Verdict, box.Reason = value.Unchecked, Twice
			default:
				box = judged(box, b, got[0])
			}
			out = append(out, box)
		}
	}
	return out
}

// judged is the verdict on a filled box from the one cell read in it.
func judged(box Box, b sample.WorkbookBox, r Reading) Box {
	text := r.Text
	box.Text = &text
	switch {
	case r.Blank:
		box.Verdict, box.Reason = value.Unreadable, Reason(value.Blank)
	case r.Mirrored:
		box.Verdict, box.Reason = value.Unreadable, Mirrored
	case !r.Legible:
		box.Verdict, box.Reason = value.Unreadable, Illegible
	case len(r.OtherReadings) > 0:
		box.Verdict, box.Reason = value.Unreadable, Ambiguous
	default:
		v := b.Check.Checker().Check(&text)
		box.Verdict, box.Reason, box.Trap = v.Verdict, Reason(v.Reason), v.Trap
	}
	return box
}

// Items are the verdicts of the items of a page, each made of the verdicts of its boxes
// (value.Item), in the order of the page.
func Items(pg *sample.WorkbookPage, boxes []Box) []value.Verdict {
	out := make([]value.Verdict, len(pg.Items))
	for i := range pg.Items {
		var of []value.Verdict
		for _, b := range boxes {
			if b.Item == i+1 {
				of = append(of, b.Verdict)
			}
		}
		out[i] = value.Item(of)
	}
	return out
}

// Right is the answer of the reader who is always right, by what code wrote in the filled boxes of
// the page: each read as written, legible, with no other reading, and marked mirrored when code drew
// a digit of it mirrored.
func Right(cells []Cell) []Reading {
	out := []Reading{}
	for _, c := range cells {
		out = append(out, Reading{Item: c.Item, Slot: c.Slot, Text: *c.Box.Written, Legible: true, OtherReadings: []string{}, Mirrored: c.Box.Scenario == sample.BoxMirrored})
	}
	return out
}

// Want is the verdict a right reading of a box of a scenario comes to by the prompt of a version,
// from the verdict of the value written in it (sample.WorkbookBox.Verdict): that verdict, but from
// MirroredVersion on, for a box with a digit written mirrored, unreadable, mirrored, so that the
// learner writes it again (R56 spec, amendment of 2026-10-29). The run and its report both judge a
// reading by it.
func Want(scenario sample.BoxScenario, of value.Slot, version int) value.Slot {
	if version >= MirroredVersion && scenario == sample.BoxMirrored {
		return value.Slot{Verdict: value.Unreadable, Reason: value.Reason(Mirrored)}
	}
	return of
}
