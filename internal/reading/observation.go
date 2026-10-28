package reading

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/altnovatrail/tutor/internal/sample"
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

// The reasons of the observation; the value's are value.Blank and value.Notation.
const (
	NoResult   Reason = "no_result"
	Results    Reason = "results"
	Illegible  Reason = "illegible"
	Ambiguous  Reason = "ambiguous"
	NotWritten Reason = "not_written"
)

// Verdict is the program's verdict on an observation, against the key of the task's check, the
// value of the bank's key (KeyOf), whatever key the reading was given (docs/architecture.md, 3.4):
//   - no observation, a failed call, is unchecked;
//   - the observation is unreadable when it has no final answer or more than one, when the answer's
//     part is illegible, when the answer could be read as another value, when its value is blank,
//     and when the value is not seen in the writing of its part (concepts 9.1 spec);
//   - otherwise the check of the kind number judges the value: correct, not yet right, or
//     unreadable when the value is in none of its notations (R56 spec).
func Verdict(o *Observation, key string) (value.Verdict, Reason) {
	switch {
	case o == nil:
		return value.Unchecked, ""
	case len(o.Results) == 0:
		return value.Unreadable, NoResult
	case len(o.Results) > 1:
		return value.Unreadable, Results
	}
	r := o.Results[0]
	part := o.Parts[r.Part-1]
	switch {
	case !part.Legible:
		return value.Unreadable, Illegible
	case len(r.OtherValues) > 0:
		return value.Unreadable, Ambiguous
	}
	read, ok := value.Read(r.Value, nil)
	if !ok {
		return value.Unreadable, Reason(value.Blank)
	}
	if !seen(read, part.Text) {
		return value.Unreadable, NotWritten
	}
	slot := value.Number{Key: key}.Check(&r.Value)
	return slot.Verdict, Reason(slot.Reason)
}

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

// numberPattern is a number in a normalized text: an integer, a decimal, with a point or a comma,
// or a simple fraction; a decimal comma is taken whole, so that the check finds it out of the
// notation rather than reads only its whole part. The
// answer word is how the sheets of the measurement begin a final answer that has no "=".
var (
	numberPattern = regexp.MustCompile(`-?[0-9]+(?:[.,/][0-9]+)?`)
	answerWord    = regexp.MustCompile(`(?i)ответ:`)
)

// numbers are the numbers of a text, in the order they stand, normalized.
func numbers(text string) []string {
	return numberPattern.FindAllString(value.Normalize(text), -1)
}

// ValueOf is the number a written answer gives: the first number after its last "=", and in a line
// without one the first number after «Ответ:», or else the first of the line; "" when it has none.
// So the bank's key "x = 2" is the key 2 of the check, and the answer line «56 : 6 = 9 (ост. 2)»
// gives 9. The banks of the measurement write their keys so, and they do not change (R28).
func ValueOf(text string) string {
	t := value.Normalize(text)
	if i := strings.LastIndex(t, "="); i >= 0 {
		t = t[i+1:]
	} else if at := answerWord.FindStringIndex(t); at != nil {
		t = t[at[1]:]
	}
	return numberPattern.FindString(t)
}

// KeyOf is the key of the check of a task's key, or of a false key: its value (ValueOf).
func KeyOf(key string) string { return ValueOf(key) }

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
	key := value.Number{Key: KeyOf(g.Key)}
	marked := m.Value
	for _, r := range o.Results {
		if key.Check(&r.Value).Verdict == value.Correct && key.Check(&marked).Verdict != value.Correct {
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
