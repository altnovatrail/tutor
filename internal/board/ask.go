package board

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/altnovatrail/tutor/provider"
)

// The most a call may take: output tokens, reasoning included, which bound what it can cost, and
// a generous count of input tokens — the picture of the board and the prompt come to about 1 300.
const (
	maxOutputTokens   = 16000
	inputTokensBudget = 4000
)

// instructions tell the model what to do with the picture. The model reads what is written
// exactly as it is, then judges it against the task's answer and solution, which it is given.
// The comment is for the author, who reads Russian.
const instructions = `You check a handwritten solution to a maths task. The picture shows what was written on the sheet.

1. Read every line of the handwriting, top to bottom, exactly as it is written. Do not correct anything: a wrong digit or sign stays wrong. Write ? for whatever you cannot make out.
2. Compare what is written with the task's correct answer and reference solution, which you are given. The person may solve the task another way; any correct way is correct.
3. Give your verdict:
   - "correct": the final answer is right and no line has a mistake;
   - "error": a line or the final answer is wrong;
   - "unreadable": you cannot read enough to judge, in particular the final answer.
4. For "error", give the number of the first wrong line, counting the lines you read from 1; otherwise null.
5. Write a comment in Russian for the adult who wrote the solution: one or two short sentences saying what you read and why you judged so.`

// schema is the answer every model must return.
var schema = map[string]any{
	"type":                 "object",
	"additionalProperties": false,
	"required":             []string{"lines", "answer", "verdict", "first_error_line", "comment"},
	"properties": map[string]any{
		"lines":            map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		"answer":           map[string]any{"type": "string"},
		"verdict":          map[string]any{"type": "string", "enum": []string{"correct", "error", "unreadable"}},
		"first_error_line": map[string]any{"anyOf": []any{map[string]any{"type": "integer"}, map[string]any{"type": "null"}}},
		"comment":          map[string]any{"type": "string"},
	},
}

// Verdict is what a model says about a solution.
type Verdict struct {
	Lines          []string `json:"lines"`
	Answer         string   `json:"answer"`
	Verdict        string   `json:"verdict"`
	FirstErrorLine *int     `json:"first_error_line"`
	Comment        string   `json:"comment"`
}

// Result is one model's answer to one try, as the page shows it and the log keeps it.
type Result struct {
	Model     string   `json:"model"`
	ModelID   string   `json:"model_id,omitempty"`
	Verdict   *Verdict `json:"verdict,omitempty"`
	Error     string   `json:"error,omitempty"`
	LatencyMs int64    `json:"latency_ms"`
	CostUSD   float64  `json:"cost_usd"`
}

// prompt is the text that goes with the picture: the task, its answer and its solution.
func prompt(t *Task) string {
	return fmt.Sprintf("Task: %s\nCorrect answer: %s\nReference solution:\n%s", t.Statement, t.Key, strings.Join(t.Solution, "\n"))
}

// worstCost is the most one call to the model can cost.
func worstCost(m provider.Model) float64 {
	in, out := m.Price()
	return (inputTokensBudget*in + maxOutputTokens*out) / 1e6
}

// ask asks one model about one picture and turns its answer into a Result. A failure is a Result
// too, with the error in words, so that the page shows it beside the other model's answer; its
// tokens are counted, as they are billed.
func ask(ctx context.Context, m provider.Model, t *Task, png []byte) Result {
	start := time.Now()
	resp, err := m.Ask(ctx, provider.Request{
		Instructions:    instructions,
		Text:            prompt(t),
		PNG:             png,
		SchemaName:      "verdict",
		Schema:          schema,
		MaxOutputTokens: maxOutputTokens,
	})
	in, out := m.Price()
	res := Result{
		Model:     m.Name(),
		ModelID:   resp.Model,
		LatencyMs: time.Since(start).Milliseconds(),
		CostUSD:   (float64(resp.InputTokens)*in + float64(resp.OutputTokens)*out) / 1e6,
	}
	if err != nil {
		res.Error = describe(err)
		return res
	}
	var v Verdict
	if err := json.Unmarshal([]byte(resp.JSON), &v); err != nil {
		res.Error = "the answer is off the schema: " + err.Error()
		return res
	}
	res.Verdict = &v
	return res
}

// describe puts an error in words for the page.
func describe(err error) string {
	switch {
	case errors.Is(err, provider.ErrNoAnswer):
		return "the model gave no answer: " + err.Error()
	case errors.Is(err, context.DeadlineExceeded):
		return "the model did not answer in time"
	default:
		return "the call failed: " + err.Error()
	}
}
