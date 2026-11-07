package reading

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode"

	"github.com/altnovatrail/module-kit/observation"
	"github.com/altnovatrail/module-kit/verdict"
	observationv1 "github.com/altnovatrail/spec/contracts/novatrail/observation/v1"
	"github.com/altnovatrail/spec/types/formatv0"

	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/task"
	"github.com/altnovatrail/tutor/internal/value"
)

// The kinds of a part of the observation: writing, or a drawing told in words.
const (
	KindText    = "text"
	KindDrawing = "drawing"
)

// Observation is what a model read on a sheet (concepts 9.1 spec): the parts of the work in the
// order the solution is read, and its final answers. It says nothing of whether the answer is
// right: the verdict is the program's (R03 spec).
type Observation struct {
	Parts   []Part   `json:"parts"`
	Results []Result `json:"results"`
}

// Part is a part of the work as it is written: writing, character for character, or a drawing, a
// number line, a row of things or a scheme, told in a few words with every number and label on
// it, as the markup tells a drawing (T56); whether it is legible, and how else it could be read.
type Part struct {
	Kind          string   `json:"kind"`
	Text          string   `json:"text"`
	Legible       bool     `json:"legible"`
	OtherReadings []string `json:"other_readings"`
}

// Result is a final answer of the work: the number of its part, from 1, the value as it is written,
// the number of the answer without the unknown, the "=" before it and the words after it, and
// the other values it could be read as.
type Result struct {
	Part        int      `json:"part"`
	Value       string   `json:"value"`
	OtherValues []string `json:"other_values"`
}

// Read takes a model's observation apart as strictly as the schema: the parts and the results,
// both of them and nothing else, every field of a part and of a result, and each result on a part
// of the observation. The answer as the model gave it is the run's to keep.
func Read(answer string) (*Observation, error) {
	var raw struct {
		Parts *[]struct {
			Kind          *string   `json:"kind"`
			Text          *string   `json:"text"`
			Legible       *bool     `json:"legible"`
			OtherReadings *[]string `json:"other_readings"`
		} `json:"parts"`
		Results *[]struct {
			Part        *int      `json:"part"`
			Value       *string   `json:"value"`
			OtherValues *[]string `json:"other_values"`
		} `json:"results"`
	}
	dec := json.NewDecoder(bytes.NewReader([]byte(answer)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("reading: the answer is not an observation: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("reading: the answer has more after the observation")
	}
	if raw.Parts == nil || raw.Results == nil {
		return nil, errors.New("reading: the observation has no parts or no results")
	}
	o := &Observation{Parts: []Part{}, Results: []Result{}}
	for i, p := range *raw.Parts {
		if p.Kind == nil || p.Text == nil || p.Legible == nil || p.OtherReadings == nil {
			return nil, fmt.Errorf("reading: part %d lacks a field", i+1)
		}
		if *p.Kind != KindText && *p.Kind != KindDrawing {
			return nil, fmt.Errorf("reading: part %d is of no kind %q", i+1, *p.Kind)
		}
		o.Parts = append(o.Parts, Part{Kind: *p.Kind, Text: *p.Text, Legible: *p.Legible, OtherReadings: *p.OtherReadings})
	}
	for i, r := range *raw.Results {
		if r.Part == nil || r.Value == nil || r.OtherValues == nil {
			return nil, fmt.Errorf("reading: result %d lacks a field", i+1)
		}
		if *r.Part < 1 || *r.Part > len(o.Parts) {
			return nil, fmt.Errorf("reading: result %d stands on part %d, and the observation has %d", i+1, *r.Part, len(o.Parts))
		}
		o.Results = append(o.Results, Result{Part: *r.Part, Value: *r.Value, OtherValues: *r.OtherValues})
	}
	return o, nil
}

// Reason is why a sheet is unreadable: of the observation, or of the value as the check reads it.
type Reason string

// The reasons of the observation; the value's are module-kit's, verdict.Blank and verdict.Notation.
const (
	NoResult   Reason = "no_result"
	Results    Reason = "results"
	Illegible  Reason = "illegible"
	Ambiguous  Reason = "ambiguous"
	NotWritten Reason = "not_written"
)

// ObservationOf is an observation as the eval protocol writes it (contracts/README.md of spec,
// section 10): the one answer slot of a sheet, bound anywhere, with the value of the final answer,
// or the reason the reading gives none, and that reason in tutor's own words, which the log keeps
// (R73). A nil observation, a failed call, is none: nil, with no reason.
//   - no final answer is UNREADABLE_NO_FINAL_ANSWER, no_result, and more than one
//     UNREADABLE_SEVERAL_FINAL_ANSWERS, results;
//   - the answer's part illegible is UNREADABLE_ILLEGIBLE, illegible;
//   - an answer that could be read as another value is UNREADABLE_AMBIGUOUS, ambiguous, and so is a
//     value not seen in the writing of its part, not_written: section 10 names a value that the
//     reader cannot point to in the writing ambiguous;
//   - a value that nothing is left of once it is read, and an empty one, is UNREADABLE_BLANK, with
//     no word of the reading's: module-kit's verdict names it blank;
//   - otherwise the value, as it is written.
func ObservationOf(o *Observation) (*observationv1.Observation, Reason) {
	if o == nil {
		return nil, ""
	}
	reason := Reason("")
	switch {
	case len(o.Results) == 0:
		reason = NoResult
	case len(o.Results) > 1:
		reason = Results
	default:
		r := o.Results[0]
		part := o.Parts[r.Part-1]
		read, ok := value.Read(r.Value)
		switch {
		case !part.Legible:
			reason = Illegible
		case len(r.OtherValues) > 0:
			reason = Ambiguous
		case !ok:
			return observationOf(Reason(verdict.Blank), ""), ""
		case !seen(read, part.Text):
			reason = NotWritten
		default:
			return observationOf("", r.Value), ""
		}
	}
	return observationOf(reason, ""), reason
}

// unreadable is the reason of the protocol that each reason of the reading, and blank, is (section
// 10 of spec's contracts text).
var unreadable = map[Reason]observationv1.Unreadable{
	NoResult:              observationv1.Unreadable_UNREADABLE_NO_FINAL_ANSWER,
	Results:               observationv1.Unreadable_UNREADABLE_SEVERAL_FINAL_ANSWERS,
	Illegible:             observationv1.Unreadable_UNREADABLE_ILLEGIBLE,
	Ambiguous:             observationv1.Unreadable_UNREADABLE_AMBIGUOUS,
	NotWritten:            observationv1.Unreadable_UNREADABLE_AMBIGUOUS,
	Reason(verdict.Blank): observationv1.Unreadable_UNREADABLE_BLANK,
}

// observationOf is the observation of the one answer slot of a sheet: the reason of the protocol
// that a reason of the reading is, or, with none of them, the value.
func observationOf(reason Reason, v string) *observationv1.Observation {
	slot := &observationv1.AnswerSlotObservation{AnswerSlot: &observationv1.AnswerSlotPlace{Item: 1, Slot: 1}}
	if u, ok := unreadable[reason]; ok {
		slot.Read = &observationv1.AnswerSlotObservation_Unreadable{Unreadable: u}
	} else {
		slot.Read = &observationv1.AnswerSlotObservation_Value{Value: v}
	}
	return &observationv1.Observation{AnswerSlots: []*observationv1.AnswerSlotObservation{slot}}
}

// RecordObservation is the observation that the record of a call tells, by the verdict, the reason
// and the value the run wrote for it, so that what is made of it keeps the rules of the run that
// judged it: nil for a call that failed or that gave no verdict, the protocol's reason for a
// reason of the reading or blank, and otherwise the value read.
func RecordObservation(c *CallRecord) *observationv1.Observation {
	if c.NotAsked || c.Error != "" || verdict.Verdict(c.Verdict) == verdict.Unchecked {
		return nil
	}
	return observationOf(c.Reason, c.Value)
}

// Verdict is the program's verdict on an observation, against the key of the task's check, the
// value of the bank's key (KeyOf), whatever key the reading was given (docs/architecture.md, 3.4):
// module-kit's, on the observation as the protocol writes it (ObservationOf; R73). No observation,
// a failed call, is unchecked; a reason of the reading makes it unreadable, with the reading's own
// word for it; and otherwise the check of the kind number judges the value: correct, not yet
// right, or unreadable, blank or in none of its notations (R56 spec). A key that is no number is
// no check module-kit takes, and it gives no verdict, whatever was read: the sheet is unchecked,
// with no reason. The banks' keys are all numbers, and a test holds them so.
func Verdict(o *Observation, key string) (verdict.Verdict, Reason) {
	obs, reason := ObservationOf(o)
	slot, err := SlotVerdict(key, obs)
	if err != nil {
		return verdict.Unchecked, ""
	}
	if reason == "" {
		reason = Reason(slot.Reason)
	}
	return slot.Verdict, reason
}

// SlotVerdict is module-kit's verdict on the one answer slot of a sheet whose check is number with
// the key (task.SheetItem), on an observation as the protocol writes it, nil for none: then the
// slot is unchecked. An error is an observation that is not valid, or a key that is no number.
func SlotVerdict(key string, o *observationv1.Observation) (verdict.SlotVerdict, error) {
	items, err := observation.Verdicts(ctx, []formatv0.Item{task.SheetItem(key)}, o, nil)
	if err != nil {
		return verdict.SlotVerdict{}, err
	}
	return items[0].Slots[0], nil
}

// ctx is the context of a verdict: a check of number runs no predicate that it could stop.
var ctx = context.Background()

// seen reports whether a value, read as the check reads it, stands in the writing of its part:
// both normalized and without spaces, and the value the whole of a number there (R62 canvas-api).
// So 2 is not seen in 13/2, in 2.5 or in x = -2, where the minus is a sign and not a subtraction,
// while 66 is seen in «Ответ: 66.» and 2 in «x = 2, y = 3».
func seen(v, text string) bool {
	v, text = squash(v), squash(text)
	if v == "" {
		return false
	}
	digit := func(i int) bool { return i >= 0 && i < len(text) && text[i] >= '0' && text[i] <= '9' }
	joins := func(c byte) bool { return c == '/' || c == '.' || c == ',' }
	for i := 0; ; {
		j := strings.Index(text[i:], v)
		if j < 0 {
			return false
		}
		at, end := i+j, i+j+len(v)
		// A digit next to the value, or a point, comma or slash between it and a digit, makes it a
		// part of a longer number; so does a minus before it that no digit or bracket stands before.
		before := digit(at-1) || at >= 1 && joins(text[at-1]) && digit(at-2) ||
			at >= 1 && text[at-1] == '-' && !digit(at-2) && (at < 2 || text[at-2] != ')')
		after := digit(end) || end < len(text) && joins(text[end]) && digit(end+1)
		if !before && !after {
			return true
		}
		i = at + 1
	}
}

// squash is a text as the reading compares texts: normalized as a value is (value.Normalize), each
// character to its lowercase, the signs of multiplication and division each one, and without spaces.
func squash(text string) string {
	var b strings.Builder
	for _, r := range value.Normalize(text) {
		switch r {
		case ' ':
			continue
		case '*', '\u00B7', '\u22C5':
			r = '\u00D7'
		case '\u00F7':
			r = ':'
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// numbers are the numbers of a text, in the order they stand, normalized (sample.Numbers).
func numbers(text string) []string { return sample.Numbers(text) }

// ValueOf is the number a written answer gives, as the banks write their keys (sample.ValueOf).
func ValueOf(text string) string { return sample.ValueOf(text) }

// KeyOf is the key of the check of a task's key, or of a false key: its value (sample.KeyOf).
func KeyOf(key string) string { return sample.KeyOf(key) }

// Marked is the markup of a sheet as the reading compares with it: its parts in the order of
// reading, and the value of its answer. A part of a sheet of free writing has the kind of its part
// of the plan; any other part is writing.
type Marked struct {
	Parts []MarkedPart `json:"parts"`
	Value string       `json:"value"`
}

// MarkedPart is a part of the markup: its kind and text, and whether it is crossed out, illegible
// or the answer.
type MarkedPart struct {
	Kind       string `json:"kind"`
	Text       string `json:"text"`
	CrossedOut bool   `json:"crossed_out,omitempty"`
	Illegible  bool   `json:"illegible,omitempty"`
	Answer     bool   `json:"answer,omitempty"`
}

// MarkupOf is the markup of a sample, with the kinds of its parts from its sheet of the plan, if it
// has one with as many parts. The value is that of its answer that is not crossed out.
func MarkupOf(s *sample.Sample, plan *sample.PlanSheet) Marked {
	m := Marked{Parts: []MarkedPart{}}
	byPlan := plan != nil && len(plan.Parts) == len(s.Lines)
	for i, l := range s.Lines {
		p := MarkedPart{Kind: KindText, Text: l.Text, CrossedOut: l.CrossedOut, Illegible: l.Illegible, Answer: l.Answer}
		if byPlan && plan.Parts[i].Kind == sample.PartDrawing {
			p.Kind = KindDrawing
		}
		if p.Answer && !p.CrossedOut && m.Value == "" {
			m.Value = ValueOf(l.Text)
		}
		m.Parts = append(m.Parts, p)
	}
	return m
}

// Right is the observation of a reader who is always right, by the markup: what the stub answers.
// Its parts are those of the markup that are not crossed out, as they are written; its result is
// the answer, with the value of its writing. A sheet without an answer has no result.
func Right(m Marked) *Observation {
	o := &Observation{Parts: []Part{}, Results: []Result{}}
	for _, p := range m.Parts {
		if p.CrossedOut {
			continue
		}
		o.Parts = append(o.Parts, Part{Kind: p.Kind, Text: p.Text, Legible: !p.Illegible, OtherReadings: []string{}})
		if p.Answer {
			o.Results = append(o.Results, Result{Part: len(o.Parts), Value: ValueOf(p.Text), OtherValues: []string{}})
		}
	}
	return o
}

// Given is a key and a solution that a reading is given, right or false.
type Given struct {
	Key      string   `json:"key"`
	Solution []string `json:"solution"`
}

// Under is what of an observation was read under a key and a solution (R03): the parts, by their
// numbers from 1, and whether the final answer was.
type Under struct {
	Parts  []int `json:"parts,omitempty"`
	Result bool  `json:"result,omitempty"`
}

// UnderKey is what of an observation was read as the given solution or key, though the sheet has
// something else there (R03, amendment of 2026-10-20):
//   - a part of writing whose text, compared as squash compares, is a line of the given solution
//     or the given key, while no part of the markup that is not crossed out has that text;
//   - a part that is a drawing, by its numbers in the order they stand, since a drawing told in
//     words is told in other words than the solution's: its numbers are those of a line of the
//     given solution, and of no part of the markup;
//   - the final answer, when its value is the given key's and the markup's is not.
func UnderKey(o *Observation, m Marked, g Given) Under {
	var u Under
	if o == nil {
		return u
	}
	var lines, sheet []string
	var lineNumbers, sheetNumbers [][]string
	for _, l := range append(slices.Clone(g.Solution), g.Key) {
		lines, lineNumbers = append(lines, squash(l)), append(lineNumbers, numbers(l))
	}
	for _, p := range m.Parts {
		if !p.CrossedOut {
			sheet, sheetNumbers = append(sheet, squash(p.Text)), append(sheetNumbers, numbers(p.Text))
		}
	}
	has := func(list [][]string, n []string) bool {
		return slices.ContainsFunc(list, func(l []string) bool { return slices.Equal(l, n) })
	}
	for i, p := range o.Parts {
		if p.Kind == KindDrawing {
			if n := numbers(p.Text); len(n) > 0 && has(lineNumbers, n) && !has(sheetNumbers, n) {
				u.Parts = append(u.Parts, i+1)
			}
			continue
		}
		if t := squash(p.Text); t != "" && slices.Contains(lines, t) && !slices.Contains(sheet, t) {
			u.Parts = append(u.Parts, i+1)
		}
	}
	key := task.SheetItem(KeyOf(g.Key))
	isKey := func(v string) bool {
		got, err := verdict.OfSlot(ctx, verdict.AnswerSlots(key)[0], &v)
		return err == nil && got.Verdict == verdict.Correct
	}
	for _, r := range o.Results {
		if isKey(r.Value) && !isKey(m.Value) {
			u.Result = true
		}
	}
	return u
}

// PartsRead is how many of the parts of writing of the markup that are neither crossed out nor
// written illegibly on purpose an observation reads exactly as written, compared as squash compares,
// and how many there are: each part of writing of the observation counts for one part of the markup
// at most. An illegible part has no text to read, and a reader that guesses it is not right for it. The accuracy of the
// reading of a sheet is the one over the other (T71; docs/architecture.md, 3.1).
func PartsRead(o *Observation, m Marked) (read, of int) {
	var seen []string
	if o != nil {
		for _, p := range o.Parts {
			if p.Kind == KindText {
				seen = append(seen, squash(p.Text))
			}
		}
	}
	for _, p := range m.Parts {
		if p.Kind != KindText || p.CrossedOut || p.Illegible {
			continue
		}
		of++
		if i := slices.Index(seen, squash(p.Text)); i >= 0 {
			read++
			seen = slices.Delete(seen, i, i+1)
		}
	}
	return read, of
}

// ReadsAs reports whether an observation has a part of writing that reads as the text, compared as
// squash compares: so a part the writer had only begun is read as if it were written whole (О-21;
// T72).
func ReadsAs(o *Observation, text string) bool {
	if o == nil {
		return false
	}
	want := squash(text)
	return slices.ContainsFunc(o.Parts, func(p Part) bool { return p.Kind == KindText && squash(p.Text) == want })
}

// Rest is an observation without the parts of writing that read the parts of writing of the
// markup, matched as PartsRead matches them: what of the observation is left once the markup is
// read in it.
func Rest(o *Observation, m Marked) *Observation {
	if o == nil {
		return nil
	}
	rest := &Observation{Parts: slices.Clone(o.Parts), Results: o.Results}
	for _, p := range m.Parts {
		if p.Kind != KindText || p.CrossedOut || p.Illegible {
			continue
		}
		want := squash(p.Text)
		if i := slices.IndexFunc(rest.Parts, func(q Part) bool { return q.Kind == KindText && squash(q.Text) == want }); i >= 0 {
			rest.Parts = slices.Delete(rest.Parts, i, i+1)
		}
	}
	return rest
}
