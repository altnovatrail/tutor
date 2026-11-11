package tutor

import (
	"errors"
	"fmt"
	"slices"

	"github.com/altnovatrail/spec/types/admissionv0"

	"github.com/altnovatrail/tutor/admission"
	"github.com/altnovatrail/tutor/internal/hint"
	"github.com/altnovatrail/tutor/internal/reading"
	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/workbook"
	"github.com/altnovatrail/tutor/provider"
)

// maxTextBytes is the most bytes the instructions and the text of a request of the library take
// together; a request of more is refused before any call (checkText). A token takes at least one byte
// of the text, so the text takes no more tokens than this, and the worst price of a kind holds: the
// largest crop the library takes, ⌈maxCropSide / 28⌉² tokens of its picture (render.Tokens), and
// this. A request of the words, the longest, takes about 5 400 bytes (internal/hint's testdata).
const maxTextBytes = 24000

// checkText refuses a request whose instructions, text and labels of its pictures are longer than
// maxTextBytes.
func checkText(r provider.Request) error {
	n := len(r.Instructions) + len(r.Text)
	for _, im := range r.Images {
		n += len(im.Label)
	}
	if n > maxTextBytes {
		return fmt.Errorf("tutor: the request takes %d bytes of text, more than the %d its worst price holds", n, maxTextBytes)
	}
	return nil
}

// tokens are the input and output tokens of a call of one model.
type tokens struct{ in, out int64 }

// Call is a kind of call of the library: the reading of a sheet, of a workbook page, and the call of
// the words. Each serves a capability with a prompt of its own, and an entry of the list admits it
// only when it measured that prompt (R70 spec).
type Call string

// The kinds of call of the library.
const (
	CallSheet    Call = "sheet"
	CallWorkbook Call = "workbook"
	CallWords    Call = "words"
)

// kind is a kind of call of the library: the capability it serves and the fingerprint of its
// prompt, the most input tokens it takes, the most it may write, how many models of the entry
// chosen it asks, every one when 0, and the tokens a call of each model is expected to take, by the
// model's exact ID, the averages of the measurements of the model at the effort admitted.
type kind struct {
	capability    admissionv0.Capability
	prompt        string
	maxIn, maxOut int64
	models        int
	expected      map[string]tokens
}

// kinds are the kinds of call of the library. The sheet's expected tokens are those of the run that
// admitted its pair, 180 calls of each reader (T126); the words', those of the 300 hints of the run
// of the words (T119); the workbook has no admitted model yet. A model without them has no expected
// price, and api takes the value of its own config then (В13 api). The words serve the analysis of
// the steps with the work and the hint without it, by one prompt; their price is that of the
// analysis.
var kinds = map[Call]kind{
	CallSheet: {
		capability: admissionv0.CapabilityHandwritingReading,
		prompt:     reading.Fingerprint(),
		maxIn:      maxTextBytes + tokensOf(maxCropSide, 1),
		maxOut:     reading.MaxOutputTokens,
		expected: map[string]tokens{
			"claude-opus-5-5":  {in: 1354, out: 177},
			"claude-haiku-5-5": {in: 1354, out: 502},
		},
	},
	CallWorkbook: {
		capability: admissionv0.CapabilityHandwritingReading,
		prompt:     workbook.Fingerprint(),
		maxIn:      maxTextBytes + tokensOf(maxBoxSide, maxBoxes),
		maxOut:     reading.MaxOutputTokens,
	},
	CallWords: {
		capability: admissionv0.CapabilitySolutionAnalysis,
		prompt:     hint.Fingerprint(),
		maxIn:      maxTextBytes + tokensOf(maxCropSide, 1),
		maxOut:     wordsMaxOutput,
		models:     1,
		expected:   map[string]tokens{"claude-opus-5-5": {in: 2255, out: 358}},
	},
}

// tokensOf are the tokens of the pictures of n crops of side pixels on each side (render.Tokens).
func tokensOf(side, n int) int64 { return int64(n * render.Tokens(side, side)) }

// CallPrice is what a call of a capability is expected to cost and can cost at most, in US dollars,
// over every model of the entry the call asks — both readers of a pair — (architecture 3.7): api
// reserves the quota of platform AI by them before the call. The worst is the most input of the
// kind, each token at the higher of its price and that of a write to the cache, and its most
// output. Expected is false when a model of the entry has no expected tokens; Worst is false when
// the prices do not hold the worst, and api takes the value of its own config then.
type CallPrice struct {
	ExpectedUSD, WorstUSD float64
	Expected, Worst       bool
}

// Price is the price of a kind of call on a route, by the models ChooseCall chooses and the prices
// of their adapters, with the admission's answer: a refused route has no price. The prices need no
// key, and a client made with none calls nothing, so the own key is not dialled for them.
func (t *Tutor) Price(c Call, r Route) (CallPrice, admission.Answer, error) {
	k, ok := kinds[c]
	if !ok {
		return CallPrice{}, admission.Answer{}, fmt.Errorf("tutor: no call %q of the library", c)
	}
	e, a, err := t.choose(k.capability, r, k.prompt)
	if err != nil || !a.Admitted {
		return CallPrice{}, a, err
	}
	models := e.Models
	if k.models > 0 && k.models < len(models) {
		models = models[:k.models]
	}
	p := CallPrice{Expected: true, Worst: true}
	for _, m := range models {
		client, err := t.dial(m, "")
		if err != nil {
			return CallPrice{}, a, err
		}
		prices := client.Prices()
		worst, ok := worstCost(prices, k.maxIn, k.maxOut)
		p.WorstUSD, p.Worst = p.WorstUSD+worst, p.Worst && ok
		tok, known := k.expected[m.ID]
		usd, ok := prices.Cost(provider.Response{InputTokens: tok.in, OutputTokens: tok.out})
		p.ExpectedUSD, p.Expected = p.ExpectedUSD+usd, p.Expected && known && ok
	}
	return p, a, nil
}

// ModelPrice is a line of the table of prices: a model of the list of admitted models, by its
// provider and exact ID, with its prices and the day they were read.
type ModelPrice struct {
	Provider admissionv0.Provider
	Model    string
	Prices   provider.Prices
}

// Prices are the table of prices (architecture 3.7): each model of the list of admitted models
// whose provider the config gives an adapter of, once, in the order of the list. It grows from the
// prices of the adapters rather than standing beside them as a second copy.
func (t *Tutor) Prices() ([]ModelPrice, error) {
	var out []ModelPrice
	var errs []error
	for _, e := range admission.Admitted {
		for _, m := range e.Models {
			if _, ok := t.adapters[m.Provider]; !ok || slices.ContainsFunc(out, func(p ModelPrice) bool { return p.Provider == m.Provider && p.Model == m.ID }) {
				continue
			}
			// The prices need no key: a client made with none calls nothing.
			client, err := t.dial(m, "")
			if err != nil {
				errs = append(errs, err)
				continue
			}
			out = append(out, ModelPrice{Provider: m.Provider, Model: m.ID, Prices: client.Prices()})
		}
	}
	return out, errors.Join(errs...)
}

// worstCost is the most a call of in input tokens and out output tokens can cost by the prices: every
// input token at the higher of its price and that of a write to the cache, at the long prices when
// the prompt is long.
func worstCost(p provider.Prices, in, out int64) (float64, bool) {
	if p.LongAbove > 0 && in > p.LongAbove {
		if p.Long == nil {
			return 0, false
		}
		long := *p.Long
		long.Read, long.LongAbove, long.Long = p.Read, 0, nil
		p = long
	}
	_, ok := p.Cost(provider.Response{InputTokens: in, OutputTokens: out})
	return (float64(in)*p.WorstInput() + float64(out)*p.Output) / 1e6, ok
}

// usageOf is the record of usage of a call that came back, with its cost: by the tokens it took, or
// at its worst when they are not its whole bill (provider.Billed) — the most input of its kind, the
// cache counted in, and the request's most output.
func usageOf(m admission.Model, client provider.Model, r provider.Response, err error, maxIn, maxOut int64) Usage {
	u := Usage{Provider: m.Provider, Model: r.Model, InputTokens: r.InputTokens, CacheWrites: slices.Clone(r.CacheWrites), CacheReadTokens: r.CacheReadTokens,
		OutputTokens: r.OutputTokens, ThinkingTokens: r.ThinkingTokens, Billed: provider.Billed(r, err)}
	prices := client.Prices()
	if u.Billed {
		u.USD, u.Priced = prices.Cost(r)
		return u
	}
	in := r.InputTokens + r.CacheReadTokens
	for _, w := range r.CacheWrites {
		in += w.Tokens
	}
	u.USD, u.Priced = worstCost(prices, max(maxIn, in), maxOut)
	return u
}
