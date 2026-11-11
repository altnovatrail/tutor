// Package provider is the port through which tutor reaches the AI providers. Each provider has
// an adapter that implements it, and a provider's SDK is imported by its adapter alone: that is
// how the SDKs reach api's build only through tutor. The library does not build the adapters in:
// a service gives it the Dial of each adapter it takes, so that a service of one provider's models
// carries no other provider's SDK (Д185 RUN.md spec; R90).
package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
	// NoPicture marks a request of a text alone, such as the assessment of a text an earlier call
	// read (T92): it has no picture, and its text is not blank. Any other request has a picture.
	NoPicture bool
}

// Response is a model's answer: JSON that follows the request's schema, the model the provider
// says answered, the tokens the call took — the input tokens without the cache, those written to
// the cache by how long it keeps them and those read from it, which add up to the input with no
// token counted twice, and of the output tokens, those the model spent thinking —, when the first
// text of the answer came, counted from when the attempt that answered was sent,
// or zero where the adapter does not measure it, and how many times the SDK sent the request again
// after a failed attempt. A call that fails with ErrNoAnswer still returns its tokens, since they
// are billed. Partial is a streamed answer that began and broke off before the provider gave the
// final count of its tokens: the tokens it holds are those counted so far, and the bill is not
// known.
type Response struct {
	JSON            string
	Model           string
	InputTokens     int64
	CacheWrites     []CacheWrite
	CacheReadTokens int64
	OutputTokens    int64
	ThinkingTokens  int64
	FirstToken      time.Duration
	Retries         int
	Partial         bool
}

// Model is one model of one provider.
type Model interface {
	// Name is how a person sees the model named, such as "Claude Opus 5.5".
	Name() string
	// Prices are what its tokens cost (docs/architecture.md, 3.7).
	Prices() Prices
	// Ask sends the request and returns the answer.
	Ask(ctx context.Context, r Request) (Response, error)
	// Check checks the client's key by a request of the provider without any learner's content, which
	// costs nothing (R84 spec; docs/architecture.md, 3.7): nil when the provider took the key, else
	// ErrKeyRefused for a wrong, revoked or expired key, ErrNoRights for a key without rights to the
	// model, or the error of a provider that did not answer (Checked). An adapter does not let its
	// SDK send it again after a refusal (NoRetry).
	Check(ctx context.Context) error
}

// Dial is an adapter as the library takes it: it makes the client of a model of its provider by the
// model's exact ID (R07 spec) and a key, and refuses an ID it does not call.
type Dial func(id, key string) (Model, error)

// ErrNoAnswer is an answer that came back without the JSON asked for: the model refused, or ran
// out of tokens before it finished.
var ErrNoAnswer = errors.New("no answer")

// CheckImages refuses the pictures of a request that an adapter sending at most pictures will not
// send: none, more than most, one without its PNG, or, of two or more, one without its label. A
// request marked NoPicture is the one that has none, and it is refused with a picture or with a
// blank text, which asks nothing (T92). Of several pictures each has a label, as Anthropic advises,
// so that the text can name them (docs/architecture.md, section 4).
func CheckImages(r Request, most int) error {
	switch {
	case r.NoPicture && len(r.Images) > 0:
		return errors.New("a request of a text alone that has a picture")
	case r.NoPicture && strings.TrimSpace(r.Text) == "":
		return errors.New("a request of a text alone whose text is blank")
	case r.NoPicture:
		return nil
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

// Billed reports whether the tokens of a call's response are its whole bill. They are not for an
// answer that broke off before the provider gave the final count of its tokens (Partial), nor for
// a call that its context cut off before any bill came: the model may have spent tokens all the
// same. A caller that caps its spending counts such a call at the most it could cost.
func Billed(resp Response, err error) bool {
	if err == nil {
		return true
	}
	cutOff := errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
	unknown := resp.Partial || cutOff && !resp.Counted()
	return !unknown
}

// Counted reports whether the response holds any token: of the input, the cache or the output. A
// prompt read whole from the cache leaves the input without the cache at none.
func (r Response) Counted() bool {
	if r.InputTokens > 0 || r.CacheReadTokens > 0 || r.OutputTokens > 0 {
		return true
	}
	for _, w := range r.CacheWrites {
		if w.Tokens > 0 {
			return true
		}
	}
	return false
}

// CacheWrite is the input tokens a call wrote to the cache to keep for TTL: a provider prices a
// write by how long the cache keeps it.
type CacheWrite struct {
	TTL    time.Duration
	Tokens int64
}

// WorstInput is the most a token of the input can cost: its own price or that of a write to the
// cache, whichever is higher, for a call whose bill is not known.
func (p Prices) WorstInput() float64 {
	worst := p.Input
	for _, w := range p.CacheWrite {
		worst = max(worst, w)
	}
	return worst
}

// Prices are what a model's tokens cost, in US dollars a million, as its provider's pricing page
// gives them on the day Read names, as YYYY-MM-DD (docs/architecture.md, 3.7 and section 4):
// input, output, a read from the cache and a write to it by how long the cache keeps it. Where a
// prompt longer than LongAbove tokens, the cache counted in, pays other prices, Long holds them,
// or nil when they were not read; LongAbove is 0 where the model has no such prices.
type Prices struct {
	Input, Output, CacheRead float64
	CacheWrite               map[time.Duration]float64
	LongAbove                int64
	Long                     *Prices
	Read                     string
}

// Cost is the cost in US dollars of the tokens of a response, and whether the prices hold them all:
// not when they were never read, Read empty, nor when the prompt is longer than LongAbove and the
// long prices were not read, nor when the cache kept a write for a time the prices name no price
// of. It is an estimate: the provider bills.
func (p Prices) Cost(r Response) (float64, bool) {
	read := p.Read != ""
	prompt := r.InputTokens + r.CacheReadTokens
	for _, w := range r.CacheWrites {
		prompt += w.Tokens
	}
	if p.LongAbove > 0 && prompt > p.LongAbove {
		if p.Long == nil {
			return 0, false
		}
		p = *p.Long
	}
	ok := read
	usd := float64(r.InputTokens)*p.Input + float64(r.CacheReadTokens)*p.CacheRead + float64(r.OutputTokens)*p.Output
	for _, w := range r.CacheWrites {
		price, known := p.CacheWrite[w.TTL]
		ok = ok && (known || w.Tokens == 0)
		usd += float64(w.Tokens) * price
	}
	return usd / 1e6, ok
}
