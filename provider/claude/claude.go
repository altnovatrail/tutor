// Package claude is the adapter of Anthropic's Claude models, through the official Go SDK.
package claude

import (
	"context"
	"encoding/base64"
	"fmt"
	"slices"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/altnovatrail/tutor/provider"
)

// Model is a Claude model the adapter can call: its exact ID (R07 spec), how the board shows it,
// and its price per million input and output tokens, in US dollars.
type Model struct {
	ID            string
	Name          string
	Input, Output float64
}

// Models are the current Claude models, cheapest first, with the IDs and prices of Anthropic's
// Models overview, read 2026-10-23. Haiku 5.5's price is the one for a request of up to 100 000
// tokens (docs/architecture.md, section 4); the board's are far shorter.
var Models = []Model{
	{ID: "claude-haiku-5-5", Name: "Claude Haiku 5.5", Input: 0.10, Output: 0.50},
	{ID: "claude-sonnet-5-5", Name: "Claude Sonnet 5.5", Input: 2, Output: 10},
	{ID: "claude-opus-5-5", Name: "Claude Opus 5.5", Input: 4, Output: 20},
	{ID: "claude-fable-5-1", Name: "Claude Fable 5.1", Input: 10, Output: 50},
}

// Efforts are the effort levels, least first. Each model in Models takes all five (Effort, read
// 2026-10-23).
var Efforts = []string{"low", "medium", "high", "xhigh", "max"}

// Client calls one Claude model.
type Client struct {
	sdk   anthropic.Client
	model Model
}

// New returns a client of the model with the given API key. The options are for tests: a fake
// server's URL.
func New(apiKey string, m Model, opts ...option.RequestOption) *Client {
	opts = append([]option.RequestOption{option.WithAPIKey(apiKey)}, opts...)
	return &Client{sdk: anthropic.NewClient(opts...), model: m}
}

// Name is how the board shows the model.
func (c *Client) Name() string { return c.model.Name }

// Price is the model's price per million input and output tokens, in US dollars.
func (c *Client) Price() (input, output float64) { return c.model.Input, c.model.Output }

// Ask sends the picture and the text and asks for JSON that follows the schema, at the request's
// effort. Every model in Models thinks adaptively when the request names no thinking, and on Opus
// 5.5 and Fable 5.1 thinking cannot be turned off, so the effort is what sets how much a model
// reasons. Without one, a model takes its own default: medium on Opus 5.5 and Haiku 5.5, high on
// Sonnet 5.5 and Fable 5.1 (Models overview and Effort, read 2026-10-23).
func (c *Client) Ask(ctx context.Context, r provider.Request) (provider.Response, error) {
	if r.Effort != "" && !slices.Contains(Efforts, r.Effort) {
		return provider.Response{}, fmt.Errorf("claude: no effort level %q", r.Effort)
	}
	msg, err := c.sdk.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     c.model.ID,
		MaxTokens: r.MaxOutputTokens,
		System:    []anthropic.TextBlockParam{{Text: r.Instructions}},
		Messages: []anthropic.MessageParam{anthropic.NewUserMessage(
			anthropic.NewImageBlockBase64("image/png", base64.StdEncoding.EncodeToString(r.PNG)),
			anthropic.NewTextBlock(r.Text),
		)},
		OutputConfig: anthropic.OutputConfigParam{
			Effort: anthropic.OutputConfigEffort(r.Effort),
			Format: anthropic.JSONOutputFormatParam{Schema: r.Schema},
		},
	})
	if err != nil {
		return provider.Response{}, err
	}
	// Tokens are billed whatever the answer, so they go back with an error too.
	resp := provider.Response{Model: msg.Model, InputTokens: msg.Usage.InputTokens, OutputTokens: msg.Usage.OutputTokens}
	switch msg.StopReason {
	case anthropic.StopReasonRefusal:
		return resp, fmt.Errorf("%w: the model refused (%s)", provider.ErrNoAnswer, msg.StopDetails.Category)
	case anthropic.StopReasonMaxTokens:
		return resp, fmt.Errorf("%w: the answer ran out of tokens", provider.ErrNoAnswer)
	}
	var text strings.Builder
	for _, block := range msg.Content {
		if t, ok := block.AsAny().(anthropic.TextBlock); ok {
			text.WriteString(t.Text)
		}
	}
	if text.Len() == 0 {
		return resp, fmt.Errorf("%w: the answer has no text", provider.ErrNoAnswer)
	}
	resp.JSON = text.String()
	return resp, nil
}
