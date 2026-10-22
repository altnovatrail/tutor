// Package claude is the adapter of Anthropic's Claude models, through the official Go SDK.
package claude

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/altnovatrail/tutor/provider"
)

// Model is the exact ID of the model the adapter calls (R07 spec).
const Model = "claude-opus-5-5"

// Client calls Claude Opus 5.5.
type Client struct {
	sdk anthropic.Client
}

// New returns a client with the given API key. The options are for tests: a fake server's URL.
func New(apiKey string, opts ...option.RequestOption) *Client {
	opts = append([]option.RequestOption{option.WithAPIKey(apiKey)}, opts...)
	return &Client{sdk: anthropic.NewClient(opts...)}
}

// Name is how the board shows the model.
func (c *Client) Name() string { return "Claude Opus 5.5" }

// Price is Claude Opus 5.5's price per million input and output tokens, in US dollars.
func (c *Client) Price() (input, output float64) { return 4, 20 }

// Ask sends the picture and the text and asks for JSON that follows the schema. Thinking cannot
// be turned off on Claude Opus 5.5 and its default effort is medium; the effort here is high,
// for a verdict that has to be right more than fast.
func (c *Client) Ask(ctx context.Context, r provider.Request) (provider.Response, error) {
	msg, err := c.sdk.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     Model,
		MaxTokens: r.MaxOutputTokens,
		System:    []anthropic.TextBlockParam{{Text: r.Instructions}},
		Messages: []anthropic.MessageParam{anthropic.NewUserMessage(
			anthropic.NewImageBlockBase64("image/png", base64.StdEncoding.EncodeToString(r.PNG)),
			anthropic.NewTextBlock(r.Text),
		)},
		OutputConfig: anthropic.OutputConfigParam{
			Effort: anthropic.OutputConfigEffortHigh,
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
