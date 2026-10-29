package report_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/internal/report"
	"github.com/altnovatrail/tutor/internal/runner"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/value"
	"github.com/altnovatrail/tutor/internal/workbook"
)

func str(s string) *string { return &s }

// workbookLog is a log of three pages, each of a box written right, "8" of the key 8, one written
// wrong, "3", a near key of the key 8, and a blank one; read by haiku-low bare and captioned at 32 px
// in one repeat, and bare at 16 px.
func workbookLog() *workbook.Log {
	l := &workbook.Log{Run: workbook.RunRecord{Kind: "run", CallKind: workbook.CallKind, Part: "test", File: "eval/workbook.json", Seed: 1,
		Readers: []runner.ReaderRecord{{Name: "haiku-low", Model: "claude-haiku-5-5", Effort: "low"}}, Repeats: 1, Reads: []string{"captions", "heights"}, MaxUSD: 4},
		End: &runner.End{Kind: "end"}}
	for n := 1; n <= 3; n++ {
		l.Sheets = append(l.Sheets, workbook.PageRecord{Kind: "page", Page: n, Part: "test", Boxes: []workbook.BoxRecord{
			{Item: 1, Slot: 1, Kind: sample.KindNumber, Key: "8", Scenario: sample.BoxCorrect, Written: str("8"), Verdict: value.Correct},
			{Item: 2, Slot: 1, Kind: sample.KindNumber, Key: "8", Scenario: sample.BoxNearKey, Written: str("3"), Verdict: value.NotYet},
			{Item: 3, Slot: 1, Kind: sample.KindFraction, Form: value.Reduced, Key: "1/2", Scenario: sample.BoxBlank, Verdict: value.Unreadable, Reason: value.Blank},
		}})
		for _, c := range []struct {
			cond  workbook.Condition
			digit int
		}{{workbook.Bare, 32}, {workbook.Captioned, 32}, {workbook.Bare, 16}} {
			l.Calls = append(l.Calls, workbook.CallRecord{Kind: "call", Call: workbook.Call{Page: n, Condition: c.cond, Reader: "haiku-low", Repeat: 1, Digit: c.digit},
				Boxes: []workbook.Box{
					{Item: 1, Slot: 1, Text: str("8"), Verdict: value.Correct},
					{Item: 2, Slot: 1, Text: str("3"), Verdict: value.NotYet},
					{Item: 3, Slot: 1, Verdict: value.Unreadable, Reason: workbook.Reason(value.Blank)},
				}, Items: []value.Verdict{value.Correct, value.NotYet, value.Unreadable}})
		}
	}
	return l
}

// call is the call of a page in a condition at a height.
func call(l *workbook.Log, page int, cond workbook.Condition, digit int) *workbook.CallRecord {
	for i := range l.Calls {
		c := &l.Calls[i]
		if c.Page == page && c.Condition == cond && c.Digit == digit {
			return c
		}
	}
	return nil
}

// The report counts the readings of the boxes by condition and group, with each error where it
// belongs: a false "correct" among boxes written wrong, a false alarm among those written right,
// "unreadable" and "unchecked" among the filled ones; a blank box is right.
func TestWorkbookCells(t *testing.T) {
	l := workbookLog()
	c := call(l, 1, workbook.Bare, 32)
	c.Boxes[1] = workbook.Box{Item: 2, Slot: 1, Text: str("8"), Verdict: value.Correct} // near key read as the key
	c.Items[1] = value.Correct
	call(l, 2, workbook.Bare, 32).Boxes[0] = workbook.Box{Item: 1, Slot: 1, Text: str("3"), Verdict: value.NotYet}
	call(l, 2, workbook.Bare, 32).Items[0] = value.NotYet
	call(l, 3, workbook.Bare, 32).Boxes[0] = workbook.Box{Item: 1, Slot: 1, Verdict: value.Unchecked, Reason: workbook.Missing}
	call(l, 3, workbook.Bare, 32).Items[0] = value.Unchecked
	r, err := report.NewWorkbook(l)
	if err != nil {
		t.Fatal(err)
	}
	find := func(cond workbook.Condition, group string) report.WorkbookCell {
		for _, c := range r.Cells {
			if c.Condition == cond && c.Group == group {
				return c
			}
		}
		t.Fatalf("no cell of %s, %s", cond, group)
		return report.WorkbookCell{}
	}
	all := find(workbook.Bare, report.AllBoxes)
	if all.Readings != 9 || all.Right != 6 || all.Read != 6 || all.FalseCorrect.Events != 1 || all.FalseCorrect.Of != 3 ||
		all.FalseAlarm.Events != 1 || all.FalseAlarm.Of != 3 || all.Extra.Events != 1 || all.Extra.Of != 6 || all.Unchecked != 1 {
		t.Errorf("bare, all: %+v", all)
	}
	if near := find(workbook.Bare, "scenario near-key"); near.Readings != 3 || near.FalseCorrect.Events != 1 {
		t.Errorf("bare, near key: %+v", near)
	}
	if blank := find(workbook.Bare, "scenario blank"); blank.Readings != 3 || blank.Right != 3 || blank.Extra.Of != 0 {
		t.Errorf("bare, blank: %+v", blank)
	}
	if reduced := find(workbook.Bare, report.Reduced); reduced.Readings != 3 {
		t.Errorf("bare, in lowest terms: %+v", reduced)
	}
	if captioned := find(workbook.Captioned, report.AllBoxes); captioned.Right != 9 {
		t.Errorf("captioned, all: %+v", captioned)
	}
	var bare report.WorkbookItems
	for _, it := range r.Items {
		if it.Condition == workbook.Bare {
			bare = it
		}
	}
	if bare.Readings != 9 || bare.Right != 6 || bare.FalseCorrect.Events != 1 || bare.FalseCorrect.Of != 3 || bare.FalseAlarm.Events != 1 || bare.FalseAlarm.Of != 3 {
		t.Errorf("bare items: %+v", bare)
	}
	var md, table bytes.Buffer
	if err := r.Markdown(&md, "run.jsonl"); err != nil || !strings.Contains(md.String(), "| haiku-low | bare | all | 9 | 6 | 6 | 1 of 3, ") {
		t.Errorf("the Markdown is\n%s, %v", md.String(), err)
	}
	if err := r.CSV(&table); err != nil || !strings.Contains(table.String(), "haiku-low,bare,all,9,6,6,1,3,") {
		t.Errorf("the CSV is\n%s, %v", table.String(), err)
	}
	if !strings.Contains(r.Summary(), "haiku-low, bare: 6 of 9 readings right") {
		t.Errorf("the summary is %q", r.Summary())
	}
}

// The line of the item: a false "correct" with it on a box that has none bare means it does not go
// into the reading; fewer errors with it on every page is not shown to help on three pages, but
// with no new false "correct".
func TestWorkbookCaptions(t *testing.T) {
	l := workbookLog()
	call(l, 2, workbook.Captioned, 32).Boxes[1] = workbook.Box{Item: 2, Slot: 1, Text: str("8"), Verdict: value.Correct}
	r, err := report.NewWorkbook(l)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Captions) != 1 || r.Captions[0].OnlyCaptioned != 1 || r.Captions[0].Helps() || r.Captions[0].Captioned != 1 || r.Captions[0].Bare != 0 {
		t.Fatalf("the line of the item: %+v", r.Captions)
	}
	if !strings.Contains(r.Summary(), "the line does not go into the reading") {
		t.Errorf("the summary is %q", r.Summary())
	}
	l = workbookLog()
	for n := 1; n <= 3; n++ {
		call(l, n, workbook.Bare, 32).Boxes[0] = workbook.Box{Item: 1, Slot: 1, Verdict: value.Unreadable, Reason: workbook.Illegible}
	}
	if r, err = report.NewWorkbook(l); err != nil {
		t.Fatal(err)
	}
	if c := r.Captions[0]; c.OnlyCaptioned != 0 || c.Bare != 3 || c.Captioned != 0 || c.Test.Nonzero != 3 || c.Helps() {
		t.Errorf("fewer errors with the line on three pages: %+v", c)
	}
	// A repeat read bare alone, as in a run cut short, is not paired: the errors of each condition
	// are of as many calls.
	extra := *call(l, 1, workbook.Bare, 32)
	extra.Repeat, extra.Boxes = 2, append([]workbook.Box(nil), extra.Boxes...)
	extra.Boxes[1] = workbook.Box{Item: 2, Slot: 1, Verdict: value.Unreadable, Reason: workbook.Illegible}
	l.Calls = append(l.Calls, extra)
	if r, err = report.NewWorkbook(l); err != nil {
		t.Fatal(err)
	}
	if c := r.Captions[0]; c.Bare != 3 || c.Pages != 3 {
		t.Errorf("a repeat read bare alone is paired: %+v", c)
	}
}

// Another height is not worse than 32 px when it adds no false "correct" and the test finds no
// worse; one false "correct" at it alone makes it worse.
func TestWorkbookHeights(t *testing.T) {
	l := workbookLog()
	r, err := report.NewWorkbook(l)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Heights) != 1 || r.Heights[0].Digit != 16 || !r.Heights[0].NotWorse() || r.Heights[0].Sheets != 3 {
		t.Fatalf("heights: %+v", r.Heights)
	}
	call(l, 3, workbook.Bare, 16).Boxes[1] = workbook.Box{Item: 2, Slot: 1, Text: str("8"), Verdict: value.Correct}
	if r, err = report.NewWorkbook(l); err != nil {
		t.Fatal(err)
	}
	if h := r.Heights[0]; h.NotWorse() || h.NewFalseCorrect != 1 || h.Errors != 1 || h.At32 != 0 {
		t.Errorf("a false \"correct\" at 16 px alone: %+v", h)
	}
	// The heights pair the first repeat: a page whose call at 32 px in the first repeat is missing
	// is not paired with another repeat.
	call(l, 3, workbook.Bare, 32).Repeat = 2
	if r, err = report.NewWorkbook(l); err != nil {
		t.Fatal(err)
	}
	if h := r.Heights[0]; h.Sheets != 2 || h.NewFalseCorrect != 0 {
		t.Errorf("a page without its first repeat at 32 px is paired: %+v", h)
	}
}

// A log that does not hold together is no report.
func TestWorkbookRefused(t *testing.T) {
	for name, change := range map[string]func(l *workbook.Log){
		"no readers":     func(l *workbook.Log) { l.Run.Readers = nil },
		"no pages":       func(l *workbook.Log) { l.Sheets = nil },
		"a page twice":   func(l *workbook.Log) { l.Sheets = append(l.Sheets, l.Sheets[0]) },
		"a call of none": func(l *workbook.Log) { l.Calls[0].Page = 9 },
		"a box short":    func(l *workbook.Log) { l.Calls[0].Boxes = l.Calls[0].Boxes[:2] },
		"an item more":   func(l *workbook.Log) { l.Calls[0].Items = append(l.Calls[0].Items, value.Correct) },
		"boxes swapped": func(l *workbook.Log) {
			l.Calls[0].Boxes[0], l.Calls[0].Boxes[1] = l.Calls[0].Boxes[1], l.Calls[0].Boxes[0]
		},
	} {
		l := workbookLog()
		change(l)
		if _, err := report.NewWorkbook(l); err == nil {
			t.Errorf("%s: a report", name)
		}
	}
}

// mirroredLog is workbookLog with a fourth item of a box with a digit written mirrored, "5" of the
// key 5, in a log of the prompt version: bare at 32 px it is read on page 1 as unreadable and marked
// mirrored, on page 2 as the digit and on page 3 as "not yet"; on page 1 the box written right is
// marked mirrored too.
func mirroredLog(version int) *workbook.Log {
	l := workbookLog()
	l.Run.PromptVersion = version
	for i := range l.Sheets {
		l.Sheets[i].Boxes = append(l.Sheets[i].Boxes, workbook.BoxRecord{Item: 4, Slot: 1, Kind: sample.KindNumber, Key: "5", Scenario: sample.BoxMirrored, Written: str("5"), Verdict: value.Correct})
	}
	for i := range l.Calls {
		c := &l.Calls[i]
		c.Boxes = append(c.Boxes, workbook.Box{Item: 4, Slot: 1, Text: str("5"), Verdict: value.Correct})
		c.Items = append(c.Items, value.Correct)
	}
	for page, b := range map[int]workbook.Box{
		1: {Item: 4, Slot: 1, Text: str("5"), Verdict: value.Unreadable, Reason: workbook.Mirrored},
		3: {Item: 4, Slot: 1, Text: str("2"), Verdict: value.NotYet},
	} {
		c := call(l, page, workbook.Bare, 32)
		c.Boxes[3], c.Items[3] = b, b.Verdict
	}
	c := call(l, 1, workbook.Bare, 32)
	c.Boxes[0], c.Items[0] = workbook.Box{Item: 1, Slot: 1, Text: str("8"), Verdict: value.Unreadable, Reason: workbook.Mirrored}, value.Unreadable
	return l
}

// The mirrored digits are counted by the rule of spec v0.3.3 in a log of every version: asked to
// write again, read as the digit, false alarms, and other boxes marked mirrored. In a log of version
// 2 "unreadable" on a box with one is right and not an extra "unreadable"; in one of version 1, which
// read a mirrored digit as that digit, it is an extra one, and the digit read as the digit is right.
func TestWorkbookMirrored(t *testing.T) {
	for _, version := range []int{1, workbook.MirroredVersion} {
		r, err := report.NewWorkbook(mirroredLog(version))
		if err != nil {
			t.Fatal(err)
		}
		var m report.WorkbookMirrored
		for _, x := range r.Mirrored {
			if x.Condition == workbook.Bare {
				m = x
			}
		}
		if m.Readings != 3 || m.Rewrite.Events != 1 || m.Rewrite.Of != 3 || m.Marked != 1 || m.AsDigit != 1 || m.FalseAlarm.Events != 1 || m.FalseAlarm.Of != 3 ||
			m.Unchecked != 0 || m.Elsewhere.Events != 1 || m.Elsewhere.Of != 6 {
			t.Errorf("version %d, bare, mirrored: %+v", version, m)
		}
		var cell report.WorkbookCell
		for _, c := range r.Cells {
			if c.Condition == workbook.Bare && c.Group == "scenario mirrored" {
				cell = c
			}
		}
		wantExtra := 0
		if version < workbook.MirroredVersion {
			wantExtra = 1
		}
		if cell.Readings != 3 || cell.Right != 1 || cell.FalseAlarm.Events != 1 || cell.Extra.Events != wantExtra {
			t.Errorf("version %d, bare, scenario mirrored: %+v", version, cell)
		}
		var md bytes.Buffer
		if err := r.Markdown(&md, "run.jsonl"); err != nil || !strings.Contains(md.String(), "## Mirrored digits at 32 px") ||
			!strings.Contains(md.String(), "| haiku-low | bare | 3 | 1 of 3, ") || !strings.Contains(md.String(), fmt.Sprintf("This log's prompt is version %d.", version)) {
			t.Errorf("version %d: the Markdown is\n%s, %v", version, md.String(), err)
		}
		if !strings.Contains(r.Summary(), "haiku-low, bare, mirrored digits: asked to write again 1 of 3") {
			t.Errorf("version %d: the summary is %q", version, r.Summary())
		}
	}
	if r, err := report.NewWorkbook(workbookLog()); err != nil || len(r.Mirrored) != 0 || strings.Contains(r.Summary(), "mirrored") {
		t.Errorf("a log with no mirrored digit tells them: %+v, %v", r, err)
	}
}
