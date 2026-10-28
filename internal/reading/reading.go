// Package reading is the reading of a sheet into an observation (T68; docs/architecture.md, 3.4,
// «Чтение в наблюдение»): a model reads the work as it is written, part by part, and its final
// answer, without deciding whether it is right (concepts 9.1 spec). The program gives the verdict
// on the observation, by the check of the kind number (internal/value), and tells what of it was
// read under a key (R03). The measurement asks the reading in three conditions: without a key,
// with the right key and solution, and on correct sheets with a false one; the key goes in a block
// of data that says it is not what the learner wrote.
package reading

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/provider"
)

// Version is the version of the reading's prompt. It is frozen before the test of the reading
// (T73), as the judge's was (T27, T66).
const Version = 1

// Condition is what a reading is given besides the sheet and its task: no key, the right key and
// solution, or a false key and solution (R03).
type Condition string

// The conditions of the reading.
const (
	NoKey    Condition = "none"
	RightKey Condition = "right"
	FalseKey Condition = "false"
)

// The instructions of the reading: base, which every request has, and keyBlock, which a request
// with a key adds.
const (
	base = `You read a learner's handwritten work on a maths task. Image 1 shows the sheet as the learner left it. Write down what is on it, as it is written: your answer is an observation, and a program decides from it whether the answer is right. Do not decide it yourself, do not correct a mistake and do not finish what is unfinished.

1. Split the work into parts, in the order the solution is read: a line of writing, a calculation, a sum worked in a column, a note at the side, a drawing. Leave out writing that is crossed out.
2. For writing, "kind" is "text", and "text" is the part exactly as it is written: every digit, sign and word, a wrong one too. A sum worked in a column goes in one line, as "14 × 3 = 42". Signs such as □ or △ that stand for unknown numbers stay as they are.
3. For a drawing, such as a number line with jumps, a row of things or a scheme of segments, "kind" is "drawing", and "text" tells in a few words what it shows, with every number and label written on it in the order they stand: "number line: jumps 3 → 7 → 11".
4. "legible" is false when you cannot make out the part for sure. "other_readings" lists the other ways the part could be read where a digit, a number or a sign of it could be read otherwise; it is empty when there are none.
5. "results" are the final answers that the work gives. For each, "part" is the number of its part, counting from 1, and "value" is the number of the answer as it is written, without the unknown and the = before it and without the words, units or remarks after it: for "x = 2" the value is "2", and for "Ответ: 5 столбов" it is "5". "other_values" lists the other numbers the answer could be read as; it is empty when there are none. When the work gives no final answer, "results" is empty; when it gives more than one, give each.`
	keyBlock = "You are also given a key and a solution of the task, in the block <key_and_solution>. Read only what Image 1 shows."
)

// The label of the picture and the text of a request round the task and the key.
const (
	finalLabel = "Image 1: the sheet as the learner left it."
	textFormat = "Image 1 is %d pixels wide and %d pixels high.\nTask: %s"
	keyFormat  = "<key_and_solution>\nThis is not what the learner wrote.\nKey: %s\nSolution:\n%s\n</key_and_solution>"
)

// Instructions are the instructions of a request: base, and the block of the key when it has one.
func Instructions(withKey bool) string {
	if withKey {
		return base + "\n\n" + keyBlock
	}
	return base
}

// Fingerprint names the prompt: the SHA-256 of every word a reader reads besides those of the
// sheet, its task and its key: the instructions with and without the key, the label of the
// picture, the text round the task and the key, and the schema, whose names the model reads too.
// The log of a run keeps it.
func Fingerprint() string {
	words := []string{Instructions(false), Instructions(true), finalLabel, textFormat, keyFormat, string(schemaProperties)}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(words, "\x00"))))
}

// Sheet is what a reading shows of a sheet: the PNG of the crop of the work, cut by the rule of the
// eval protocol with its digits a given height (render.CropOf), and the crop's numbers, without the
// frames of the groups, which a reading does not show (T69).
type Sheet struct {
	PNG  []byte
	Crop *render.Crop
}

// Prepare cuts and draws what a reading shows of the strokes a sheet shows, with its digits digit
// px high: the strokes of the finished sheet (render.Shown), or those of a state of it.
func Prepare(strokes []sample.Stroke, digit int) (*Sheet, error) {
	c, err := render.CropOf(strokes, nil, digit)
	if err != nil {
		return nil, err
	}
	img, err := c.Draw(strokes)
	if err != nil {
		return nil, err
	}
	png, err := render.PNG(img)
	if err != nil {
		return nil, err
	}
	return &Sheet{PNG: png, Crop: c}, nil
}

// Request is what a reading asks of a model about a sheet of a task, given a key and solution or
// nil for none: the instructions, the picture after its label, and the text: the size of the
// picture, the task as the learner sees it, its statement, and the block of the key. The model,
// its effort, whether it thinks and how many tokens it may write the caller sets.
func Request(t *sample.Task, sh *Sheet, g *Given) provider.Request {
	text := fmt.Sprintf(textFormat, sh.Crop.Width, sh.Crop.Height, t.Statement)
	if g != nil {
		text += "\n\n" + fmt.Sprintf(keyFormat, g.Key, strings.Join(g.Solution, "\n"))
	}
	return provider.Request{
		Instructions: Instructions(g != nil),
		Images:       []provider.Image{{Label: finalLabel, PNG: sh.PNG}},
		Text:         text,
		SchemaName:   "observation",
		Schema:       Schema,
	}
}

// schemaProperties are the properties of the observation, as JSON: the SDKs encode a map's keys in
// sorted order, and a model writes the fields in the order of the schema, the parts before the
// results.
var schemaProperties = json.RawMessage(`{
		"parts": {"type": "array", "items": {"type": "object", "additionalProperties": false, "required": ["kind", "text", "legible", "other_readings"], "properties": {
			"kind": {"type": "string", "enum": ["text", "drawing"]},
			"text": {"type": "string"},
			"legible": {"type": "boolean"},
			"other_readings": {"type": "array", "items": {"type": "string"}}
		}}},
		"results": {"type": "array", "items": {"type": "object", "additionalProperties": false, "required": ["part", "value", "other_values"], "properties": {
			"part": {"type": "integer"},
			"value": {"type": "string"},
			"other_values": {"type": "array", "items": {"type": "string"}}
		}}}
	}`)

// Schema is the observation every reader returns.
var Schema = map[string]any{
	"type":                 "object",
	"additionalProperties": false,
	"required":             []string{"parts", "results"},
	"properties":           schemaProperties,
}
