// Package gpt is the adapter of OpenAI's GPT models, through the official Go SDK and the
// Responses API.
package gpt

import (
	"cmp"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"slices"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"

	"github.com/altnovatrail/tutor/provider"
)

// Model is the exact ID of the model the adapter calls (R07 spec).
const Model = "gpt-6-astra"

// Efforts are the effort levels the adapter takes: high alone, as on the board, where GPT is not
// chosen among levels (R20).
var Efforts = []string{"high"}

// Client calls GPT-6 Astra.
type Client struct {
	sdk openai.Client
}

// New returns a client with the given API key. The options are for tests: a fake server's URL.
func New(apiKey string, opts ...option.RequestOption) *Client {
	opts = append([]option.RequestOption{option.WithAPIKey(apiKey)}, opts...)
	return &Client{sdk: openai.NewClient(opts...)}
}

// Name is how the board shows the model.
func (c *Client) Name() string { return "GPT-6 Astra" }

// Price is GPT-6 Astra's price per million input and output tokens, in US dollars.
func (c *Client) Price() (input, output float64) { return 10, 50 }

// Ask sends the picture and the text and asks for JSON that follows the schema. The picture goes
// with the detail "high", so that the model does not shrink it; the answer is not stored at
// OpenAI; the reasoning effort is high, the request's or by default. GPT-6 Astra always reasons,
// so a request without thinking is refused before any call. The answer is not streamed: GPT is not
// used on the board (R20), so when its first text comes is not measured.
func (c *Client) Ask(ctx context.Context, r provider.Request) (provider.Response, error) {
	if r.Effort != "" && !slices.Contains(Efforts, r.Effort) {
		return provider.Response{}, fmt.Errorf("gpt: no effort level %q", r.Effort)
	}
	if r.NoThinking {
		return provider.Response{}, errors.New("gpt: GPT-6 Astra cannot answer without reasoning")
	}
	// The SDK sends a request again after some failures; each attempt passes through here.
	attempts := 0
	count := option.WithMiddleware(func(req *http.Request, next option.MiddlewareNext) (*http.Response, error) {
		attempts++
		return next(req)
	})
	image := "data:image/png;base64," + base64.StdEncoding.EncodeToString(r.PNG)
	resp, err := c.sdk.Responses.New(ctx, responses.ResponseNewParams{
		Model:        Model,
		Instructions: openai.String(r.Instructions),
		Input: responses.ResponseNewParamsInputUnion{OfInputItemList: responses.ResponseInputParam{
			responses.ResponseInputItemParamOfMessage(responses.ResponseInputMessageContentListParam{
				{OfInputImage: &responses.ResponseInputImageParam{
					Detail:   responses.ResponseInputImageDetailHigh,
					ImageURL: openai.String(image),
				}},
				responses.ResponseInputContentParamOfInputText(r.Text),
			}, responses.EasyInputMessageRoleUser),
		}},
		Text: responses.ResponseTextConfigParam{Format: responses.ResponseFormatTextConfigUnionParam{
			OfJSONSchema: &responses.ResponseFormatTextJSONSchemaConfigParam{
				Name:   r.SchemaName,
				Schema: r.Schema,
				Strict: openai.Bool(true),
			},
		}},
		Reasoning:       shared.ReasoningParam{Effort: shared.ReasoningEffort(cmp.Or(r.Effort, Efforts[0]))},
		MaxOutputTokens: openai.Int(r.MaxOutputTokens),
		Store:           openai.Bool(false),
	}, count)
	if err != nil {
		return provider.Response{Retries: max(attempts-1, 0)}, err
	}
	// Tokens are billed whatever the answer, so they go back with an error too.
	out := provider.Response{
		Model:          resp.Model,
		InputTokens:    resp.Usage.InputTokens,
		OutputTokens:   resp.Usage.OutputTokens,
		ThinkingTokens: resp.Usage.OutputTokensDetails.ReasoningTokens,
		Retries:        max(attempts-1, 0),
	}
	if resp.Status == responses.ResponseStatusIncomplete {
		return out, fmt.Errorf("%w: the answer is incomplete (%s)", provider.ErrNoAnswer, resp.IncompleteDetails.Reason)
	}
	text := resp.OutputText()
	if text == "" {
		return out, fmt.Errorf("%w: the answer has no text, the model may have refused", provider.ErrNoAnswer)
	}
	out.JSON = text
	return out, nil
}
