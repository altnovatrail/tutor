package reading

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/altnovatrail/tutor/provider"
)

// PaperVersion is the version of the prompt of the reading of a task from paper (R135). A change to
// a word a model reads is a new version, and the entry of the list that admitted the prompt admits
// it no longer (R70 spec). TestFrozenPaperV1 holds it.
const PaperVersion = 1

// paperBase is the reading's base for a task from paper: the crop shows the printed task under the
// learner's strokes, so the reader is told which ink is the learner's, and an option marked on the
// page is an answer whose value is its label.
const paperBase = `You read a learner's handwritten work on a task from their own book or worksheet. Image 1 shows the task as printed on the page, with the learner's writing over it and beside it. The printed task is not the learner's work: read only what the learner wrote or marked. Write down what is on it, as it is written: your answer is an observation, and a program decides from it whether the answer is right. Do not decide it yourself, do not correct a mistake and do not finish what is unfinished.

1. Split the learner's work into parts, in the order the solution is read: a line of writing, a calculation, a sum worked in a column, a note at the side, a drawing, a mark on the page. Leave out writing that is crossed out.
2. For writing, "kind" is "text", and "text" is the part exactly as it is written: every digit, sign and word, a wrong one too. A sum worked in a column goes in one line, as "14 × 3 = 42". Signs such as □ or △ that stand for unknown numbers stay as they are.
3. For a drawing, "kind" is "drawing", and "text" tells in a few words what it shows, with every number and label written on it in the order they stand. An option of the page that the learner circled, ticked or underlined is a drawing too, and "text" names its label as printed: "circled: B".
4. "legible" is false when you cannot make out the part for sure. "other_readings" lists the other ways the part could be read where a digit, a number or a sign of it could be read otherwise; it is empty when there are none.
5. "results" are the final answers that the work gives. For each, "part" is the number of its part, counting from 1, and "value" is the number of the answer as it is written, without the unknown and the = before it and without the words, units or remarks after it: for "x = 2" the value is "2". When the learner answers by marking one printed option, "value" is that option's label as printed, without its brackets: for (B) circled, "B". "other_values" lists the other values the answer could be read as; it is empty when there are none. When the work gives no final answer and marks no option, "results" is empty; when it gives more than one — two answers, or two options marked —, give each.`

// The label of the crop of a task from paper and the text of its request: the size alone, since
// the task is the one printed under the writing.
const (
	paperLabel      = "Image 1: the task on the page, with the learner's writing over it."
	paperTextFormat = "Image 1 is %d pixels wide and %d pixels high."
)

// PaperInstructions are the instructions of the reading of a task from paper: its base and the rules
// of safety every prompt of the platform has (R54). It gives no key.
func PaperInstructions() string { return paperBase + "\n" + safety }

// PaperFingerprint names the prompt of the reading of a task from paper, as Fingerprint names the
// sheet's: the instructions, the label, the text and the schema.
func PaperFingerprint() string {
	words := []string{PaperInstructions(), paperLabel, paperTextFormat, string(schemaProperties)}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(words, "\x00"))))
}

// PaperRequestOf is the request of a reading of the crop of the work on a task from paper, given as
// its PNG and its size: the request the runner and the library ask alike (R70 spec). Its answer is
// an observation, read by Read and ObservationOf as a sheet's.
func PaperRequestOf(png []byte, width, height int) provider.Request {
	return provider.Request{
		Instructions: PaperInstructions(),
		Images:       []provider.Image{{Label: paperLabel, PNG: png}},
		Text:         fmt.Sprintf(paperTextFormat, width, height),
		SchemaName:   "observation",
		Schema:       Schema,
	}
}
