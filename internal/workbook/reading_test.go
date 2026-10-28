package workbook_test

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/value"
	"github.com/altnovatrail/tutor/internal/workbook"
	"github.com/altnovatrail/tutor/provider"
)

var update = flag.Bool("update", false, "write the golden requests anew")

func ptr[T any](v T) *T { return &v }

// testPage is a page of three items: a number, written; a fraction in lowest terms, written; and a
// division with a remainder in two boxes, the second blank.
func testPage() *sample.WorkbookPage {
	return &sample.WorkbookPage{Number: 1, Part: sample.PartDev, Items: []sample.WorkbookItem{
		{Text: "7 × [ ] = 56", Boxes: []sample.WorkbookBox{{
			Check: sample.BoxCheck{Kind: sample.KindNumber, Key: "8", TrapValues: []sample.TrapValue{{Value: "49", Trap: "wrong-operation"}}}, Scenario: sample.BoxCorrect, Written: ptr("8"),
		}}},
		{Text: "Сократи дробь: 6/8 = [ ]", Boxes: []sample.WorkbookBox{{
			Check:    sample.BoxCheck{Kind: sample.KindFraction, Key: "3/4", Form: ptr(value.Reduced), TrapValues: []sample.TrapValue{{Value: "6/8", Trap: "not-in-lowest-terms"}}},
			Scenario: sample.BoxCorrect, Written: ptr("3/4"), Fraction: ptr(sample.Stacked),
		}}},
		{Text: "17 : 5 = [ ] (ост. [ ])", Boxes: []sample.WorkbookBox{
			{Check: sample.BoxCheck{Kind: sample.KindNumber, Key: "3"}, Scenario: sample.BoxCorrect, Written: ptr("3")},
			{Check: sample.BoxCheck{Kind: sample.KindNumber, Key: "2"}, Scenario: sample.BoxBlank},
		}},
	}}
}

func reading(item, slot int, text string) workbook.Reading {
	return workbook.Reading{Item: item, Slot: slot, Text: text, Legible: true, OtherReadings: []string{}}
}

// The program's verdicts on the boxes of a page: the box's check judges what was read, a blank box
// is unreadable without a call, and the reading's doubts and failures each give their own verdict.
func TestVerdicts(t *testing.T) {
	pg := testPage()
	blank := workbook.Box{Item: 3, Slot: 2, Verdict: value.Unreadable, Reason: workbook.Reason(value.Blank)}
	for _, c := range []struct {
		name string
		read []workbook.Reading
		want []workbook.Box
	}{
		{"read right", []workbook.Reading{reading(1, 1, "8"), reading(2, 1, "3/4"), reading(3, 1, "3")}, []workbook.Box{
			{Item: 1, Slot: 1, Text: ptr("8"), Verdict: value.Correct}, {Item: 2, Slot: 1, Text: ptr("3/4"), Verdict: value.Correct},
			{Item: 3, Slot: 1, Text: ptr("3"), Verdict: value.Correct}, blank,
		}},
		{"a trap, a fraction to reduce and a decimal", []workbook.Reading{reading(1, 1, "49"), reading(2, 1, "9/12"), reading(3, 1, "3.0")}, []workbook.Box{
			{Item: 1, Slot: 1, Text: ptr("49"), Verdict: value.NotYet, Trap: "wrong-operation"},
			{Item: 2, Slot: 1, Text: ptr("9/12"), Verdict: value.NotYet, Trap: "not-in-lowest-terms"},
			{Item: 3, Slot: 1, Text: ptr("3.0"), Verdict: value.Correct}, blank,
		}},
		{"a decimal in a box of a fraction", []workbook.Reading{reading(1, 1, "8"), reading(2, 1, "0.75"), reading(3, 1, "3")}, []workbook.Box{
			{Item: 1, Slot: 1, Text: ptr("8"), Verdict: value.Correct},
			{Item: 2, Slot: 1, Text: ptr("0.75"), Verdict: value.Unreadable, Reason: workbook.Reason(value.Notation)},
			{Item: 3, Slot: 1, Text: ptr("3"), Verdict: value.Correct}, blank,
		}},
		{"doubts", []workbook.Reading{
			{Item: 1, Slot: 1, Text: "8", Legible: false, OtherReadings: []string{}},
			{Item: 2, Slot: 1, Text: "3/4", Legible: true, OtherReadings: []string{"3/9"}},
			{Item: 3, Slot: 1, Text: "", Legible: true, OtherReadings: []string{}, Blank: true},
			reading(3, 2, "2"), // a cell for a blank box judges nothing
		}, []workbook.Box{
			{Item: 1, Slot: 1, Text: ptr("8"), Verdict: value.Unreadable, Reason: workbook.Illegible},
			{Item: 2, Slot: 1, Text: ptr("3/4"), Verdict: value.Unreadable, Reason: workbook.Ambiguous},
			{Item: 3, Slot: 1, Text: ptr(""), Verdict: value.Unreadable, Reason: workbook.Reason(value.Blank)}, blank,
		}},
		{"a cell missing and a cell twice", []workbook.Reading{reading(1, 1, "8"), reading(1, 1, "8"), reading(3, 1, "3"), reading(9, 1, "1")}, []workbook.Box{
			{Item: 1, Slot: 1, Verdict: value.Unchecked, Reason: workbook.Twice},
			{Item: 2, Slot: 1, Verdict: value.Unchecked, Reason: workbook.Missing},
			{Item: 3, Slot: 1, Text: ptr("3"), Verdict: value.Correct}, blank,
		}},
		{"a failed call", nil, []workbook.Box{
			{Item: 1, Slot: 1, Verdict: value.Unchecked}, {Item: 2, Slot: 1, Verdict: value.Unchecked}, {Item: 3, Slot: 1, Verdict: value.Unchecked}, blank,
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := workbook.Verdicts(pg, c.read)
			if fmt.Sprintf("%+v", show(got)) != fmt.Sprintf("%+v", show(c.want)) {
				t.Errorf("Verdicts =\n%v\nwant\n%v", show(got), show(c.want))
			}
		})
	}
	// An item is right only when every box of it is: the division with its remainder blank is
	// unreadable, and not yet right when a box of it is.
	got := workbook.Items(pg, workbook.Verdicts(pg, []workbook.Reading{reading(1, 1, "8"), reading(2, 1, "6/8"), reading(3, 1, "3")}))
	if want := []value.Verdict{value.Correct, value.NotYet, value.Unreadable}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("Items = %v, want %v", got, want)
	}
	got = workbook.Items(pg, workbook.Verdicts(pg, []workbook.Reading{reading(1, 1, "8"), reading(2, 1, "3/4"), reading(3, 1, "4")}))
	if want := []value.Verdict{value.Correct, value.Correct, value.NotYet}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("Items = %v, want %v", got, want)
	}
}

func show(boxes []workbook.Box) []string {
	var out []string
	for _, b := range boxes {
		text := "-"
		if b.Text != nil {
			text = "«" + *b.Text + "»"
		}
		out = append(out, fmt.Sprintf("%d.%d %s %s %s %s", b.Item, b.Slot, text, b.Verdict, b.Reason, b.Trap))
	}
	return out
}

// An answer is read as strictly as the schema: the cells and nothing else, every field of each.
func TestRead(t *testing.T) {
	got, err := workbook.Read(`{"cells": [{"item": 1, "slot": 1, "text": "8", "legible": true, "other_readings": ["3"], "blank": false}]}`)
	if err != nil || len(got) != 1 || got[0].Text != "8" || got[0].OtherReadings[0] != "3" {
		t.Fatalf("Read = %+v, %v", got, err)
	}
	for _, bad := range []string{
		``, `[]`, `{}`, `{"cells": [{"item": 1}]}`, `{"cells": [], "notes": ""}`, `{"cells": []} {}`,
		`{"cells": [{"item": 1, "slot": 1, "text": "8", "legible": true, "other_readings": [], "blank": false, "value": "8"}]}`,
	} {
		if _, err := workbook.Read(bad); err == nil {
			t.Errorf("Read(%q) takes it", bad)
		}
	}
	if got, err := workbook.Read(`{"cells": []}`); err != nil || len(got) != 0 {
		t.Errorf("an answer of no cells: %v, %v", got, err)
	}
}

// written writes a request as the golden files keep it: the instructions, each picture's label and
// the text, and no pixels.
func written(r provider.Request) string {
	var b strings.Builder
	fmt.Fprintf(&b, "=== instructions\n%s\n", r.Instructions)
	for _, im := range r.Images {
		fmt.Fprintf(&b, "=== picture: the crop of a box\n%s\n", im.Label)
	}
	fmt.Fprintf(&b, "=== text\n%s\n", r.Text)
	return b.String()
}

// The golden requests of the two conditions are the files testdata/bare.txt and
// testdata/captioned.txt, on page 1 of the workbook; the author reads them there. They differ only by
// the block of the captions and the lines of the items in the labels. The right reader's answer,
// read, comes to the verdicts a right reading comes to.
func TestGolden(t *testing.T) {
	pg := &theWorkbook(t).Pages[0]
	cells, err := workbook.Cells(pg, 1, render.Digit)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []workbook.Condition{workbook.Bare, workbook.Captioned} {
		r := workbook.Request(pg, cells, c)
		if r.SchemaName != "cells" || len(r.Images) != len(cells) || len(cells) != 10 {
			t.Errorf("%s: the request is %+v", c, r)
		}
		got := written(r)
		path := filepath.Join("testdata", string(c)+".txt")
		if *update {
			if err := os.MkdirAll("testdata", 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if got != string(want) {
			t.Errorf("%s: the request is\n%s\nwant, as %s,\n%s", c, got, path, want)
		}
	}
	for i, b := range workbook.Verdicts(pg, workbook.Right(cells)) {
		box := pg.Items[b.Item-1].Boxes[b.Slot-1]
		if want := box.Verdict(); b.Verdict != want.Verdict || string(b.Reason) != string(want.Reason) || b.Trap != want.Trap {
			t.Errorf("box %d: the right reader comes to %+v, and the box to %+v", i+1, b, want)
		}
	}
	if fp := workbook.Fingerprint(); len(fp) != 64 || fp == "" {
		t.Errorf("the fingerprint is %q", fp)
	}
}

// Version 1 of the prompt is frozen after the dev round of T80, before its test part runs: any
// change to a word a reader reads changes the fingerprint, and this test fails.
func TestFrozenV1(t *testing.T) {
	const frozen = "40a9244ba646d1edef4c99c6dc5060b5b38ccab74ad56ddf76792545b6590e8d"
	if got := workbook.Fingerprint(); got != frozen || workbook.Version != 1 {
		t.Errorf("the fingerprint of version %d is %s, not the one frozen after the dev round of T80, %s", workbook.Version, got, frozen)
	}
}
