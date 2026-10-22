// Package gpt is the adapter of OpenAI's GPT models, through the official Go SDK and the
// Responses API.
package gpt

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"

	"github.com/altnovatrail/tutor/provider"
)

// Model is the exact ID of the model the adapter calls (R07 spec).
const Model = "gpt-6-astra"

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
// OpenAI; the reasoning effort is high, as Claude's is.
func (c *Client) Ask(ctx context.Context, r provider.Request) (provider.Response, error) {
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
		Reasoning:       shared.ReasoningParam{Effort: shared.ReasoningEffortHigh},
		MaxOutputTokens: openai.Int(r.MaxOutputTokens),
		Store:           openai.Bool(false),
	})
	if err != nil {
		return provider.Response{}, err
	}
	// Tokens are billed whatever the answer, so they go back with an error too.
	out := provider.Response{Model: resp.Model, InputTokens: resp.Usage.InputTokens, OutputTokens: resp.Usage.OutputTokens}
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
