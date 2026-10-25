package board

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/altnovatrail/tutor/provider"
)

// The most a call may take: output tokens, reasoning included, which bound what it can cost, and
// a generous count of input tokens — the picture of the board and the prompt come to about 1 300.
// The output limit is the same at every effort level, so that the levels are compared under one
// limit (R19).
const (
	maxOutputTokens   = 16000
	inputTokensBudget = 4000
)

// The forms of an answer. Since T45 in RUN.md the board asks for the short one: a verdict and,
// for a mistake, its place. Before, a model also wrote the lines it read, the final answer, the
// number of the first wrong line and a comment; the log holds such answers without a form, and
// the summary counts them apart, as the earlier form.
const (
	formShort   = "short"
	formEarlier = "earlier"
)

// verdicts are what a model may say of the work so far, and shapes how it may mark a mistake.
// They are the enums of the schema below.
var (
	verdicts = []string{"correct", "so_far_correct", "error", "unreadable"}
	shapes   = []string{"underline", "circle", "box"}
)

// instructions tell the model what to do with the picture. The person may ask at any step, so
// the work may be unfinished; the model judges what is written against the task's answer and
// solution, which it is given, and marks the first mistake, without explaining it.
const instructions = `You check a handwritten solution to a maths task, which may not be finished yet. The picture shows what is written on the sheet so far.

1. Read the handwriting exactly as it is written. Do not correct anything: a wrong digit or sign stays wrong.
2. Compare what is written with the task's correct answer and reference solution, which you are given. The person may solve the task another way; any correct way is correct. Judge only what is already written: a step that is not written yet is not a mistake, and neither is a last line that is still being written.
3. Give your verdict:
   - "correct": the solution is finished, its final answer is right and nothing written is wrong;
   - "so_far_correct": nothing written is wrong, but the solution is not finished yet;
   - "error": something written is wrong;
   - "unreadable": you cannot read enough of the writing to judge it.
4. For "error", mark the first mistake. Give the box around the wrong writing in pixels of the picture: x1, y1 is its top-left corner and x2, y2 its bottom-right, with x growing to the right and y downwards from the picture's top-left corner. Give the box around the writing whichever way you choose to show it: "underline", "circle" or "box". For any other verdict, mark is null.`

// schema is the answer every model must return. Its properties are written as JSON: the SDKs
// encode a map's keys in sorted order, which would put the mark before the verdict, and a model
// writes the fields in the order of the schema.
var schema = map[string]any{
	"type":                 "object",
	"additionalProperties": false,
	"required":             []string{"verdict", "mark"},
	"properties": json.RawMessage(`{
		"verdict": {"type": "string", "enum": ["correct", "so_far_correct", "error", "unreadable"]},
		"mark": {"anyOf": [
			{"type": "null"},
			{"type": "object", "additionalProperties": false, "required": ["shape", "x1", "y1", "x2", "y2"], "properties": {
				"shape": {"type": "string", "enum": ["underline", "circle", "box"]},
				"x1": {"type": "integer"}, "y1": {"type": "integer"}, "x2": {"type": "integer"}, "y2": {"type": "integer"}
			}}
		]}
	}`),
}

// Verdict is what a model says of the work so far, as it said it: the verdict and, for a
// mistake, its mark.
type Verdict struct {
	Verdict string `json:"verdict"`
	Mark    *Mark  `json:"mark"`
}

// Mark is the place of a mistake on the picture — a box in its pixels, x1, y1 the top-left corner
// and x2, y2 the bottom-right — and how it is shown: underlined, circled or boxed.
type Mark struct {
	Shape string `json:"shape"`
	X1    int    `json:"x1"`
	Y1    int    `json:"y1"`
	X2    int    `json:"x2"`
	Y2    int    `json:"y2"`
}

// Result is one model's answer to one try, as the page shows it and the log keeps it: the model,
// the effort it was asked at and whether without thinking, the form of the answer, the verdict as
// the model gave it and the place the board draws, or what went wrong, and what the call took —
// tokens, of the output those spent thinking, when the first text came, how many times the
// request was sent again, time and money.
type Result struct {
	Model          string   `json:"model"`
	ModelID        string   `json:"model_id,omitempty"`
	Effort         string   `json:"effort,omitempty"`
	NoThinking     bool     `json:"no_thinking,omitempty"`
	Form           string   `json:"form,omitempty"`
	Verdict        *Verdict `json:"verdict,omitempty"`
	Place          *Mark    `json:"place,omitempty"`
	Error          string   `json:"error,omitempty"`
	InputTokens    int64    `json:"input_tokens"`
	OutputTokens   int64    `json:"output_tokens"`
	ThinkingTokens int64    `json:"thinking_tokens"`
	FirstTokenMs   int64    `json:"first_token_ms,omitempty"`
	Retries        int      `json:"retries,omitempty"`
	LatencyMs      int64    `json:"latency_ms"`
	CostUSD        float64  `json:"cost_usd"`
}

// picture is the PNG of a try and its size in pixels.
type picture struct {
	png           []byte
	width, height int
}

// question is what the board asks of one model: the model, the effort to ask it at, and whether
// to ask it not to think.
type question struct {
	model      provider.Model
	effort     string
	noThinking bool
}

// prompt is the text that goes with the picture: its size — the model gives places in its pixels,
// and Anthropic advises naming the coordinates expected — and the task, its answer and its
// solution.
func prompt(t *Task, p picture) string {
	return fmt.Sprintf("The picture is %d pixels wide and %d pixels high.\nTask: %s\nCorrect answer: %s\nReference solution:\n%s",
		p.width, p.height, t.Statement, t.Key, strings.Join(t.Solution, "\n"))
}

// worstCost is the most one call to the model can cost.
func worstCost(m provider.Model) float64 {
	in, out := m.Price()
	return (inputTokensBudget*in + maxOutputTokens*out) / 1e6
}

// ask asks one model about one picture and turns its answer into a Result. A failure is a Result
// too, with the error in words, so that the page shows it beside the other models' answers; its
// tokens are counted, as they are billed.
func ask(ctx context.Context, q question, t *Task, p picture) Result {
	start := time.Now()
	resp, err := q.model.Ask(ctx, provider.Request{
		Instructions:    instructions,
		Text:            prompt(t, p),
		PNG:             p.png,
		SchemaName:      "verdict",
		Schema:          schema,
		MaxOutputTokens: maxOutputTokens,
		Effort:          q.effort,
		NoThinking:      q.noThinking,
	})
	in, out := q.model.Price()
	res := Result{
		Model:          q.model.Name(),
		ModelID:        resp.Model,
		Effort:         q.effort,
		NoThinking:     q.noThinking,
		Form:           formShort,
		InputTokens:    resp.InputTokens,
		OutputTokens:   resp.OutputTokens,
		ThinkingTokens: resp.ThinkingTokens,
		FirstTokenMs:   resp.FirstToken.Milliseconds(),
		Retries:        resp.Retries,
		LatencyMs:      time.Since(start).Milliseconds(),
		CostUSD:        (float64(resp.InputTokens)*in + float64(resp.OutputTokens)*out) / 1e6,
	}
	if err != nil {
		res.Error = describe(err)
		if resp.Partial || (cutOff(err) && resp.InputTokens == 0 && resp.OutputTokens == 0) {
			// The bill of such a call is not known, though the model may have spent tokens: an
			// answer that broke off before the provider counted its tokens, or a call that the
			// clock or a closed page cut off before any bill came. It is counted at the most it
			// could cost, so that the spending cap still holds.
			res.CostUSD = worstCost(q.model)
			res.Error += "; the call is counted at the most it could cost"
		}
		return res
	}
	v, place, err := read(resp.JSON, p.width, p.height)
	if err != nil {
		res.Error = "the answer is off the schema: " + err.Error()
		return res
	}
	res.Verdict, res.Place = v, place
	return res
}

// read takes a model's answer apart and works out the place the board draws, for a mistake alone:
// the model's box, its corners in either order, held inside the picture — Anthropic advises
// clamping coordinates that fall slightly outside it. A box wholly outside the picture, or with no
// width, or with no height unless it is to be underlined, is no place. The verdict stands either
// way.
func read(answer string, width, height int) (*Verdict, *Mark, error) {
	var v Verdict
	if err := json.Unmarshal([]byte(answer), &v); err != nil {
		return nil, nil, err
	}
	if !slices.Contains(verdicts, v.Verdict) {
		return nil, nil, fmt.Errorf("no such verdict: %q", v.Verdict)
	}
	if v.Verdict != "error" || v.Mark == nil || !slices.Contains(shapes, v.Mark.Shape) {
		return &v, nil, nil
	}
	m := *v.Mark
	m.X1, m.X2 = min(m.X1, m.X2), max(m.X1, m.X2)
	m.Y1, m.Y2 = min(m.Y1, m.Y2), max(m.Y1, m.Y2)
	if m.X2 < 0 || m.X1 > width || m.Y2 < 0 || m.Y1 > height {
		return &v, nil, nil
	}
	m.X1, m.X2 = max(m.X1, 0), min(m.X2, width)
	m.Y1, m.Y2 = max(m.Y1, 0), min(m.Y2, height)
	if m.X1 == m.X2 || (m.Y1 == m.Y2 && m.Shape != "underline") {
		return &v, nil, nil
	}
	return &v, &m, nil
}

// describe puts an error in words for the page.
func describe(err error) string {
	switch {
	case errors.Is(err, provider.ErrNoAnswer):
		return "the model gave no answer: " + err.Error()
	case errors.Is(err, context.DeadlineExceeded):
		return "the model did not answer in time"
	case errors.Is(err, context.Canceled):
		return "the check was cut off before the model answered"
	default:
		return "the call failed: " + err.Error()
	}
}

// cutOff reports whether a call ended because its context did: the clock ran out, or the page
// that asked went away.
func cutOff(err error) bool {
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
}
