package reading

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	observationv1 "github.com/altnovatrail/spec/contracts/novatrail/observation/v1"

	"github.com/altnovatrail/tutor/provider"
)

// PaperVersion is the version of the prompt of the reading of a task from paper. A change to a word
// a model reads is a new version, and the entry of the list that admitted the prompt admits it no
// longer. TestFrozenPaperV2 holds it.
const PaperVersion = 2

// paperBase is the reading's base for a task from paper: the crop shows the printed task under the
// learner's strokes, so the reader is told which ink is the learner's, and an option marked on the
// page is an answer whose value is its label.
const paperBase = `You read a learner's handwritten work on the tasks of their own book or worksheet. Image 1 shows the tasks as printed on the page, with the learner's writing over them and beside them; the text lists the printed tasks, each by its number, between <tasks> and </tasks>. The printed task is not the learner's work: read only what the learner wrote or marked. Write down what is on it, as it is written: your answer is an observation, and a program decides from it whether the answer is right. Do not decide it yourself, do not correct a mistake and do not finish what is unfinished.

1. Split the learner's work into parts, in the order the solution is read: a line of writing, a calculation, a sum worked in a column, a note at the side, a drawing, a mark on the page. Leave out writing that is crossed out.
2. For writing, "kind" is "text", and "text" is the part exactly as it is written: every digit, sign and word, a wrong one too. A sum worked in a column goes in one line, as "14 × 3 = 42". Signs such as □ or △ that stand for unknown numbers stay as they are.
3. For a drawing, "kind" is "drawing", and "text" tells in a few words what it shows, with every number and label written on it in the order they stand. An option of the page that the learner circled, ticked or underlined is a drawing too, and "text" names its label as printed: "circled: B".
4. "legible" is false when you cannot make out the part for sure. "other_readings" lists the other ways the part could be read where a digit, a number or a sign of it could be read otherwise; it is empty when there are none.
5. "results" are the final answers that the work gives. For each, "task" is the number of the task of the list it answers, by where it is written and what it answers; "part" is the number of its part, counting from 1; "box" is where the answer is written on Image 1, as [ymin, xmin, ymax, xmax] in whole thousandths of Image 1's height and width; and "value" is the number of the answer as it is written, without the unknown and the = before it and without the words, units or remarks after it: for "x = 2" the value is "2". When the learner answers by marking one printed option, "value" is that option's label as printed, without its brackets: for (B) circled, "B". "other_values" lists the other values the answer could be read as; it is empty when there are none. When the work gives no final answer to a task and marks no option of it, it has no result; when it gives more than one to a task — two answers, or two options marked —, give each.`

// The label of the crop of a task from paper and the text of its request: the size alone, since
// the task is the one printed under the writing.
const (
	paperLabel      = "Image 1: the tasks on the page, with the learner's writing over them."
	paperTextFormat = "Image 1 is %d pixels wide and %d pixels high. The tasks:\n<tasks>\n%s</tasks>"
	paperTaskFormat = "%d. %s\n"
)

// PaperInstructions are the instructions of the reading of a task from paper: its base and the rules
// of safety every prompt of the platform has. It gives no key.
func PaperInstructions() string { return paperBase + "\n" + safety }

// PaperFingerprint names the prompt of the reading of a task from paper, as Fingerprint names the
// sheet's: the instructions, the label, the text and the schema.
func PaperFingerprint() string {
	words := []string{PaperInstructions(), paperLabel, paperTextFormat, paperTaskFormat, string(paperSchemaProperties)}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(words, "\x00"))))
}

// PaperRequestOf is the request of a reading of the crop of the work on a task from paper, given as
// its PNG and its size, with the statements of its tasks, in order, as the reading of the statement
// gave them: data, a model's reading of the page, never instructions. It is the request the runner
// and the library ask alike. Its answer is read by PaperRead and
// PaperObservationOf.
func PaperRequestOf(png []byte, width, height int, tasks []string) provider.Request {
	var list strings.Builder
	for i, t := range tasks {
		fmt.Fprintf(&list, paperTaskFormat, i+1, strings.ReplaceAll(t, "\n", " "))
	}
	return provider.Request{
		Instructions: PaperInstructions(),
		Images:       []provider.Image{{Label: paperLabel, PNG: png}},
		Text:         fmt.Sprintf(paperTextFormat, width, height, list.String()),
		SchemaName:   "observation",
		Schema:       PaperSchema,
	}
}

// paperSchemaProperties are the properties of the observation of a task from paper: the sheet's,
// each result with the task it answers and its box.
var paperSchemaProperties = json.RawMessage(`{
		"parts": {"type": "array", "items": {"type": "object", "additionalProperties": false, "required": ["kind", "text", "legible", "other_readings"], "properties": {
			"kind": {"type": "string", "enum": ["text", "drawing"]},
			"text": {"type": "string"},
			"legible": {"type": "boolean"},
			"other_readings": {"type": "array", "items": {"type": "string"}}
		}}},
		"results": {"type": "array", "items": {"type": "object", "additionalProperties": false, "required": ["task", "part", "box", "value", "other_values"], "properties": {
			"task": {"type": "integer"},
			"part": {"type": "integer"},
			"box": {"type": "array", "items": {"type": "integer"}},
			"value": {"type": "string"},
			"other_values": {"type": "array", "items": {"type": "string"}}
		}}}
	}`)

// PaperSchema is the observation every reader of a task from paper returns.
var PaperSchema = map[string]any{
	"type":                 "object",
	"additionalProperties": false,
	"required":             []string{"parts", "results"},
	"properties":           paperSchemaProperties,
}

// PaperResult is a final answer of the work on a task from paper: a sheet's, with the number of
// the task it answers, from 1, and where it is written on the crop, [ymin, xmin, ymax, xmax] in
// whole thousandths, or nil when the model gave no box that is one.
type PaperResult struct {
	Result
	Task int   `json:"task"`
	Box  []int `json:"box"`
}

// PaperObservation is what a model read of the work on a task from paper: the parts of the work,
// and the final answers, each with its task.
type PaperObservation struct {
	Parts   []Part        `json:"parts"`
	Results []PaperResult `json:"results"`
}

// PaperRead takes a model's observation of a task from paper apart as strictly as its schema, as
// Read takes a sheet's: each result on a part of the observation and of a task of the tasks it
// was given. A box that is no rectangle in the crop is dropped, not refused: it only places a mark.
func PaperRead(answer string, tasks int) (*PaperObservation, error) {
	var raw struct {
		Parts *[]struct {
			Kind          *string   `json:"kind"`
			Text          *string   `json:"text"`
			Legible       *bool     `json:"legible"`
			OtherReadings *[]string `json:"other_readings"`
		} `json:"parts"`
		Results *[]struct {
			Task        *int      `json:"task"`
			Part        *int      `json:"part"`
			Box         *[]int    `json:"box"`
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
	o := &PaperObservation{Parts: []Part{}, Results: []PaperResult{}}
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
		if r.Task == nil || r.Part == nil || r.Box == nil || r.Value == nil || r.OtherValues == nil {
			return nil, fmt.Errorf("reading: result %d lacks a field", i+1)
		}
		if *r.Part < 1 || *r.Part > len(o.Parts) {
			return nil, fmt.Errorf("reading: result %d stands on part %d, and the observation has %d", i+1, *r.Part, len(o.Parts))
		}
		if *r.Task < 1 || *r.Task > tasks {
			return nil, fmt.Errorf("reading: result %d answers task %d, and the crop has %d", i+1, *r.Task, tasks)
		}
		res := PaperResult{Result: Result{Part: *r.Part, Value: *r.Value, OtherValues: *r.OtherValues}, Task: *r.Task}
		if b := *r.Box; len(b) == 4 && 0 <= b[0] && b[0] < b[2] && b[2] <= 1000 && 0 <= b[1] && b[1] < b[3] && b[3] <= 1000 {
			res.Box = b
		}
		o.Results = append(o.Results, res)
	}
	return o, nil
}

// PaperObservationOf is the observation of a task from paper as the protocol writes it: an answer
// slot for each of its tasks, the task n at the place n, 1, each by the results of its task as
// ObservationOf makes a sheet's one slot of its results, and with the box of its value, when it
// has one. A nil observation, a failed call, is none.
func PaperObservationOf(o *PaperObservation, tasks int) *observationv1.Observation {
	if o == nil {
		return nil
	}
	obs := &observationv1.Observation{}
	for n := 1; n <= tasks; n++ {
		one := &Observation{Parts: o.Parts, Results: []Result{}}
		var box []int
		for _, r := range o.Results {
			if r.Task == n {
				one.Results = append(one.Results, r.Result)
				box = r.Box
			}
		}
		slot := mustObservationOf(one).AnswerSlots[0]
		slot.AnswerSlot = &observationv1.AnswerSlotPlace{Item: int32(n), Slot: 1}
		if _, ok := slot.GetRead().(*observationv1.AnswerSlotObservation_Value); ok && len(one.Results) == 1 && box != nil {
			slot.CropBox = &observationv1.CropBox{Ymin: int32(box[0]), Xmin: int32(box[1]), Ymax: int32(box[2]), Xmax: int32(box[3])}
		}
		obs.AnswerSlots = append(obs.AnswerSlots, slot)
	}
	return obs
}

// mustObservationOf is ObservationOf of an observation that is one.
func mustObservationOf(o *Observation) *observationv1.Observation {
	obs, _ := ObservationOf(o)
	return obs
}
