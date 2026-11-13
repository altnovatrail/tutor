// Package gpt is the adapter of OpenAI's GPT models, through the official Go SDK and the
// Responses API.
package gpt

import (
	"cmp"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

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

// Dial is the adapter as the library takes it (provider.Dial): the client of Model, with the key.
func Dial(id, key string) (provider.Model, error) {
	if id != Model {
		return nil, fmt.Errorf("gpt: the adapter calls no model %s", id)
	}
	return New(key), nil
}

// New returns a client with the given API key. The options are for tests: a fake server's URL.
func New(apiKey string, opts ...option.RequestOption) *Client {
	opts = append([]option.RequestOption{option.WithAPIKey(apiKey)}, opts...)
	return &Client{sdk: openai.NewClient(opts...)}
}

// Name is how the board shows the model.
func (c *Client) Name() string { return "GPT-6 Astra" }

// PricesRead is the day GPT-6 Astra's prices were read on OpenAI's Pricing page, its standard
// tier (docs/sources.md, 1.14).
const PricesRead = "2026-11-11"

// The times the cache keeps a write for, each at its own price.
const (
	TTL30m = 30 * time.Minute
	TTL12h = 12 * time.Hour
)

// LongAbove is the input tokens over which GPT-6 Astra prices a prompt otherwise: the page gives
// those prices by a script alone, and they were not read, so a longer prompt has no estimate.
const LongAbove = 272000

// Prices are GPT-6 Astra's prices per million tokens, in US dollars, of its standard tier: input,
// output, a read from the cache and a write to it for 30 minutes and for 12 hours.
func (c *Client) Prices() provider.Prices {
	return provider.Prices{Input: 10, Output: 50, CacheRead: 1,
		CacheWrite: map[time.Duration]float64{TTL30m: 12.50, TTL12h: 25}, LongAbove: LongAbove, Read: PricesRead}
}

// Ask sends a picture and the text and asks for JSON that follows the schema. It sends one picture,
// after its label, and refuses more before any call. That is the adapter's limit, not the model's:
// GPT is not used on the board (R20), and the measurement of the writing process asks Claude alone
// (R21). The picture goes
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
	if err := provider.CheckImages(r, 1); err != nil {
		return provider.Response{}, fmt.Errorf("gpt: %w", err)
	}
	// The SDK sends a request again after some failures; each attempt passes through here.
	attempts := 0
	count := option.WithMiddleware(func(req *http.Request, next option.MiddlewareNext) (*http.Response, error) {
		attempts++
		return next(req)
	})
	// A refusal for a limit or of the key is not sent again (provider.NoRetry).
	var refused provider.Refusal
	noRetry := option.WithMiddleware(provider.NoRetry(Refusal, &refused))
	var content responses.ResponseInputMessageContentListParam
	for _, im := range r.Images {
		if im.Label != "" {
			content = append(content, responses.ResponseInputContentParamOfInputText(im.Label))
		}
		content = append(content, responses.ResponseInputContentUnionParam{OfInputImage: &responses.ResponseInputImageParam{
			Detail:   responses.ResponseInputImageDetailHigh,
			ImageURL: openai.String("data:image/png;base64," + base64.StdEncoding.EncodeToString(im.PNG)),
		}})
	}
	content = append(content, responses.ResponseInputContentParamOfInputText(r.Text))
	resp, err := c.sdk.Responses.New(ctx, responses.ResponseNewParams{
		Model:        Model,
		Instructions: openai.String(r.Instructions),
		Input: responses.ResponseNewParamsInputUnion{OfInputItemList: responses.ResponseInputParam{
			responses.ResponseInputItemParamOfMessage(content, responses.EasyInputMessageRoleUser),
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
	}, count, noRetry)
	if err != nil {
		return provider.Response{Retries: max(attempts-1, 0)}, refused.Of(err)
	}
	// Tokens are billed whatever the answer, so they go back with an error too. OpenAI counts the
	// tokens read from the cache and written to it in the input, and they come off it here
	// (docs/architecture.md, 3.7; docs/sources.md, 1.14).
	d := resp.Usage.InputTokensDetails
	// A write the usage does not divide by its time counts at 30 minutes, the shortest it keeps.
	m30, h12 := d.CacheWriteTokensDetails.Ttl30mTokens, d.CacheWriteTokensDetails.Ttl12hTokens
	if rest := d.CacheWriteTokens - m30 - h12; rest > 0 {
		m30 += rest
	}
	var writes []provider.CacheWrite
	for _, w := range []provider.CacheWrite{{TTL: TTL30m, Tokens: m30}, {TTL: TTL12h, Tokens: h12}} {
		if w.Tokens > 0 {
			writes = append(writes, w)
		}
	}
	out := provider.Response{
		Model:           resp.Model,
		InputTokens:     max(resp.Usage.InputTokens-d.CachedTokens-d.CacheWriteTokens, 0),
		CacheWrites:     writes,
		CacheReadTokens: d.CachedTokens,
		OutputTokens:    resp.Usage.OutputTokens,
		ThinkingTokens:  resp.Usage.OutputTokensDetails.ReasoningTokens,
		Retries:         max(attempts-1, 0),
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

// Refusal reads an error response of the Responses API by its code (docs/architecture.md, 3.7;
// docs/sources.md, 1.15): a balance spent, credit_balance_exhausted; a spend limit of the project or the
// organization, project_spend_limit_exceeded or organization_spend_limit_exceeded; the usage limit
// OpenAI assigns, organization_usage_limit_exceeded, each a 429; a 429 of the type insufficient_quota
// with any other code, a refusal of billing whose cause the code does not name; and the key refused,
// 401. OpenAI tells the cause of a refusal of billing by error.code, «The broader error.type can still
// be insufficient_quota».
func Refusal(status int, body []byte) provider.Refusal {
	var e struct {
		Error struct {
			Code string `json:"code"`
			Type string `json:"type"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &e)
	switch {
	case status == http.StatusTooManyRequests && e.Error.Code == "credit_balance_exhausted":
		return provider.Refusal{Limit: provider.LimitBalance}
	case status == http.StatusTooManyRequests && (e.Error.Code == "project_spend_limit_exceeded" || e.Error.Code == "organization_spend_limit_exceeded"):
		return provider.Refusal{Limit: provider.LimitOwner}
	case status == http.StatusTooManyRequests && e.Error.Code == "organization_usage_limit_exceeded":
		return provider.Refusal{Limit: provider.LimitTier}
	case status == http.StatusTooManyRequests && e.Error.Type == "insufficient_quota":
		return provider.Refusal{Limit: provider.LimitBilling}
	case status == http.StatusUnauthorized:
		return provider.Refusal{Key: true}
	}
	return provider.Refusal{}
}

// Check checks the key by asking for the model's description, which writes no token and so is
// billed nothing: OpenAI bills tokens (Pricing), and the list of models names no price — our
// reading of the pages, read 2026-11-11. A 403 is a key without rights to the model
// (provider.Checked).
func (c *Client) Check(ctx context.Context) error {
	var refused provider.Refusal
	_, err := c.sdk.Models.Get(ctx, Model, option.WithMiddleware(provider.NoRetry(Refusal, &refused)))
	status := 0
	var apiErr *openai.Error
	if errors.As(err, &apiErr) {
		status = apiErr.StatusCode
	}
	return provider.Checked(err, refused, status)
}
