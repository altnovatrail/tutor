// Package claude is the adapter of Anthropic's Claude models, through the official Go SDK.
package claude

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/altnovatrail/tutor/provider"
)

// Model is a Claude model the adapter can call: its exact ID (R07 spec), how the board shows it,
// its price per million input and output tokens, in US dollars, and the kind of thinking that turns
// its thinking off, or nothing if its thinking cannot be turned off.
type Model struct {
	ID            string
	Name          string
	Input, Output float64
	ThinkingOff   string
}

// Models are the current Claude models, cheapest first, with the IDs and prices of Anthropic's
// Models overview, read 2026-10-23. Haiku 5.5's price is the one for a request of up to 100 000
// tokens (docs/architecture.md, section 4); the board's are far shorter. Haiku 5.5 turns its
// thinking off with "disabled"; Sonnet 5.5 refuses that and turns it off with "between_tools",
// which without tools leaves an answer of text alone; on Opus 5.5 and Fable 5.1 thinking cannot
// be turned off (Thinking, read 2026-10-25).
var Models = []Model{
	{ID: "claude-haiku-5-5", Name: "Claude Haiku 5.5", Input: 0.10, Output: 0.50, ThinkingOff: "disabled"},
	{ID: "claude-sonnet-5-5", Name: "Claude Sonnet 5.5", Input: 2, Output: 10, ThinkingOff: "between_tools"},
	{ID: "claude-opus-5-5", Name: "Claude Opus 5.5", Input: 4, Output: 20},
	{ID: "claude-fable-5-1", Name: "Claude Fable 5.1", Input: 10, Output: 50},
}

// Efforts are the effort levels, least first. Each model in Models takes all five (Effort, read
// 2026-10-23).
var Efforts = []string{"low", "medium", "high", "xhigh", "max"}

// ThinkingOffEfforts are the levels at which thinking can be turned off: high and below. At xhigh
// and max, a request that turns it off fails (Thinking, read 2026-10-25).
var ThinkingOffEfforts = []string{"low", "medium", "high"}

// NoThinkingEfforts are the levels at which the model can be asked to answer without thinking:
// none if its thinking cannot be turned off.
func (m Model) NoThinkingEfforts() []string {
	if m.ThinkingOff == "" {
		return nil
	}
	return slices.Clone(ThinkingOffEfforts)
}

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

// Ask sends the pictures and the text and asks for JSON that follows the schema, at the request's
// effort, and streams the answer, so that it sees when the answer's first text comes. Every model
// in Models thinks adaptively when the request names no thinking, and on Opus 5.5 and Fable 5.1
// thinking cannot be turned off, so the effort is what sets how much a model reasons. Without one,
// a model takes its own default: medium on Opus 5.5 and Haiku 5.5, high on Sonnet 5.5 and Fable 5.1
// (Models overview and Effort, read 2026-10-23). A request without thinking names its effort,
// since thinking turns off only at some levels. A picture is never shrunk: a picture larger than
// the model takes fails the call, rather than leave the model's pixels off the picture's.
func (c *Client) Ask(ctx context.Context, r provider.Request) (provider.Response, error) {
	if err := c.check(r); err != nil {
		return provider.Response{}, err
	}
	// The SDK sends a request again after some failures; each attempt passes through here, and the
	// time of the first text is counted from the attempt that answered.
	attempts := 0
	var sent time.Time
	count := option.WithMiddleware(func(req *http.Request, next option.MiddlewareNext) (*http.Response, error) {
		attempts++
		sent = time.Now()
		return next(req)
	})
	stream := c.sdk.Messages.NewStreaming(ctx, c.params(r), count)
	defer func() { _ = stream.Close() }()
	var (
		msg     anthropic.Message
		first   time.Duration
		began   bool // message_start came, with the input tokens
		counted bool // message_delta came, with the final count of the output tokens
		err     error
	)
	for err == nil && stream.Next() {
		event := stream.Current()
		err = msg.Accumulate(event)
		switch {
		case event.Type == "message_start":
			began = true
		case event.Type == "message_delta":
			counted = true
		case first == 0 && event.Type == "content_block_delta" && event.Delta.Type == "text_delta":
			first = time.Since(sent)
		}
	}
	if err == nil {
		err = stream.Err()
	}
	// Tokens are billed whatever the answer, so they go back with an error too.
	resp := provider.Response{
		Model:          msg.Model,
		InputTokens:    msg.Usage.InputTokens,
		OutputTokens:   msg.Usage.OutputTokens,
		ThinkingTokens: msg.Usage.OutputTokensDetails.ThinkingTokens,
		FirstToken:     first,
		Retries:        max(attempts-1, 0),
		Partial:        began && !counted,
	}
	switch {
	case err != nil:
		return resp, err
	case !counted:
		return resp, errors.New("claude: the answer stopped before it was finished")
	}
	switch msg.StopReason {
	case anthropic.StopReasonRefusal:
		return resp, fmt.Errorf("%w: the model refused (%s)", provider.ErrNoAnswer, msg.StopDetails.Category)
	case anthropic.StopReasonMaxTokens:
		return resp, fmt.Errorf("%w: the answer ran out of tokens", provider.ErrNoAnswer)
	}
	var text strings.Builder
	for _, block := range msg.Content {
		if block.Type == "text" {
			text.WriteString(block.Text)
		}
	}
	if text.Len() == 0 {
		return resp, fmt.Errorf("%w: the answer has no text", provider.ErrNoAnswer)
	}
	resp.JSON = text.String()
	return resp, nil
}

// MaxImages is the most pictures the adapter sends in one request. Above 20, a stricter limit of
// size holds for every picture of the request (Vision, read 2026-10-26; docs/architecture.md,
// section 4), and with oversized_image set to error a picture over it would fail the call. A
// request of the measurement holds three at most.
const MaxImages = 20

// check refuses, before any call, what the adapter will not send: pictures provider.CheckImages
// refuses, an effort level the model does not know, or an answer without thinking at a level where
// its thinking does not turn off.
func (c *Client) check(r provider.Request) error {
	if err := provider.CheckImages(r, MaxImages); err != nil {
		return fmt.Errorf("claude: %w", err)
	}
	if r.Effort != "" && !slices.Contains(Efforts, r.Effort) {
		return fmt.Errorf("claude: no effort level %q", r.Effort)
	}
	if r.NoThinking && (c.model.ThinkingOff == "" || !slices.Contains(ThinkingOffEfforts, r.Effort)) {
		return fmt.Errorf("claude: %s cannot answer without thinking at the effort %q", c.model.Name, r.Effort)
	}
	return nil
}

// params is the request as the API takes it: one message of the pictures in their order, each a
// block of its own after a block of its label, as Anthropic advises for several pictures ("Image
// 1:", "Image 2:"; docs/architecture.md, section 4), and the text after them.
func (c *Client) params(r provider.Request) anthropic.MessageNewParams {
	content := make([]anthropic.ContentBlockParamUnion, 0, 2*len(r.Images)+1)
	for _, im := range r.Images {
		if im.Label != "" {
			content = append(content, anthropic.NewTextBlock(im.Label))
		}
		image := anthropic.NewImageBlockBase64("image/png", base64.StdEncoding.EncodeToString(im.PNG))
		image.OfImage.Transformations.OversizedImage = anthropic.ImageTransformationsParamOversizedImageError
		content = append(content, image)
	}
	content = append(content, anthropic.NewTextBlock(r.Text))
	p := anthropic.MessageNewParams{
		Model:     c.model.ID,
		MaxTokens: r.MaxOutputTokens,
		System:    []anthropic.TextBlockParam{{Text: r.Instructions}},
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(content...)},
		OutputConfig: anthropic.OutputConfigParam{
			Effort: anthropic.OutputConfigEffort(r.Effort),
			Format: anthropic.JSONOutputFormatParam{Schema: r.Schema},
		},
	}
	if r.NoThinking {
		switch c.model.ThinkingOff {
		case "disabled":
			off := anthropic.NewThinkingConfigDisabledParam()
			p.Thinking.OfDisabled = &off
		case "between_tools":
			off := anthropic.NewThinkingConfigBetweenToolsParam()
			p.Thinking.OfBetweenTools = &off
		}
	}
	return p
}
