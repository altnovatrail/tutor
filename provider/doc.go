// Package provider is the port through which tutor reaches the AI providers. Each provider has
// an adapter that implements it, and a provider's SDK is imported by its adapter alone: that is
// how the SDKs reach api's build only through tutor.
package provider

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Image is a picture of a request, as PNG, and the label that introduces it to the model, such as
// "Image 1: the sheet as it was finished." A picture without a label goes without one: a request of
// one picture needs none.
type Image struct {
	Label string
	PNG   []byte
}

// Request is one question to a model about pictures: instructions, the pictures in the order the
// model is to read them, each after its label, a text after them, the JSON schema the answer must
// follow, the most output tokens the answer may take, reasoning included — which bounds what a
// call can cost — the effort the model should spend on it, one of the levels its adapter names,
// or empty for the model's own default, and whether the model is to answer without thinking
// first. An adapter refuses before any call what it will not send: an answer without thinking at
// that effort, or pictures CheckImages refuses. How many pictures an adapter sends is its own
// limit, and may be less than its model takes.
//
// The SDKs encode a map's keys in sorted order, and a model writes the fields of its answer in
// the order of the schema. A value of the schema may be a json.RawMessage, which goes out as it is
// written: that is how a schema keeps its own order.
type Request struct {
	Instructions    string
	Images          []Image
	Text            string
	SchemaName      string
	Schema          map[string]any
	MaxOutputTokens int64
	Effort          string
	NoThinking      bool
}

// Response is a model's answer: JSON that follows the request's schema, the model the provider
// says answered, the tokens the call took — of the output tokens, those the model spent thinking
// — when the first text of the answer came, counted from when the attempt that answered was sent,
// or zero where the adapter does not measure it, and how many times the SDK sent the request again
// after a failed attempt. A call that fails with ErrNoAnswer still returns its tokens, since they
// are billed. Partial is a streamed answer that began and broke off before the provider gave the
// final count of its tokens: the tokens it holds are those counted so far, and the bill is not
// known.
type Response struct {
	JSON           string
	Model          string
	InputTokens    int64
	OutputTokens   int64
	ThinkingTokens int64
	FirstToken     time.Duration
	Retries        int
	Partial        bool
}

// Model is one model of one provider.
type Model interface {
	// Name is how a person sees the model named, such as "Claude Opus 5.5".
	Name() string
	// Price is the price of a million input and of a million output tokens, in US dollars.
	Price() (input, output float64)
	// Ask sends the request and returns the answer.
	Ask(ctx context.Context, r Request) (Response, error)
}

// ErrNoAnswer is an answer that came back without the JSON asked for: the model refused, or ran
// out of tokens before it finished.
var ErrNoAnswer = errors.New("no answer")

// CheckImages refuses the pictures of a request that an adapter sending at most pictures will not
// send: none, more than most, one without its PNG, or, of two or more, one without its label. Of
// several pictures each has a label, as Anthropic advises, so that the text can name them
// (docs/architecture.md, section 4).
func CheckImages(r Request, most int) error {
	switch {
	case len(r.Images) == 0:
		return errors.New("a request without a picture")
	case len(r.Images) > most:
		return fmt.Errorf("a request of %d pictures, and the adapter sends %d at most", len(r.Images), most)
	}
	for i, im := range r.Images {
		if len(im.PNG) == 0 {
			return fmt.Errorf("picture %d has no PNG", i+1)
		}
		if len(r.Images) > 1 && im.Label == "" {
			return fmt.Errorf("picture %d of %d has no label, and each of several pictures has one", i+1, len(r.Images))
		}
	}
	return nil
}
