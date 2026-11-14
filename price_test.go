package tutor

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/altnovatrail/spec/types/formatv0/open"

	"github.com/altnovatrail/tutor/provider"
)

var (
	opusPrices  = provider.Prices{Input: 4, Output: 20, CacheRead: 0.2, CacheWrite: map[time.Duration]float64{5 * time.Minute: 5}, Read: "2026-11-11"}
	haikuPrices = provider.Prices{Input: 0.1, Output: 0.5, Read: "2026-11-11"}
)

// pricedPair is a library whose pair of readers are fakes with the prices of Opus and Haiku.
func pricedPair(t *testing.T) (*Tutor, *fake, *fake) {
	t.Helper()
	opus := &fake{resp: provider.Response{JSON: `{}`, Model: "claude-opus-5-5", InputTokens: 1000, OutputTokens: 100}, prices: opusPrices}
	haiku := &fake{resp: provider.Response{JSON: `{}`, Model: "claude-haiku-5-5", InputTokens: 1000, OutputTokens: 400}, prices: haikuPrices}
	return library(t, adultFacts, &fakes{models: map[string]*fake{"claude-opus-5-5": opus, "claude-haiku-5-5": haiku}}), opus, haiku
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-12 }

// The price of a reading is over both readers: expected by the tokens of the run that admitted the
// pair, at worst by the most input of the kind — the bound of the text and the largest crop —, each
// token at the higher of the input and a write to the cache, and its most output.
func TestPrice(t *testing.T) {
	lib, _, _ := pricedPair(t)
	p, a, err := lib.Price(CallSheet, platform)
	if err != nil || !a.Admitted {
		t.Fatalf("the price of a reading: %v, %+v", err, a)
	}
	expected := (1354*4+177*20)/1e6 + (1354*0.1+502*0.5)/1e6
	maxIn := float64(24000 + 147*147)
	worst := (maxIn*5+16000*20)/1e6 + (maxIn*0.1+16000*0.5)/1e6
	if !p.Expected || !p.Worst || !near(p.ExpectedUSD, expected) || !near(p.WorstUSD, worst) {
		t.Errorf("price = %+v; want expected %v and worst %v", p, expected, worst)
	}
	// The words are Opus's alone (T127): their price is one call of it, by the tokens of T127.
	if p, a, err := lib.Price(CallWords, platform); err != nil || !a.Admitted || !near(p.ExpectedUSD, (2356*4+437*20)/1e6) {
		t.Errorf("the price of the words: %+v, %+v, %v", p, a, err)
	}
	// The words after a correct answer are Opus's alone too, on the list's own entry: one call of it,
	// by the tokens of the run that admitted them (T144).
	if p, a, err := lib.Price(CallCorrect, platform); err != nil || !a.Admitted || !p.Expected || !near(p.ExpectedUSD, (1371*4+402*20)/1e6) {
		t.Errorf("the price of the words after a correct answer: %+v, %+v, %v", p, a, err)
	}
	// The hint without the work is the same model with no picture, and has no expected tokens: T127
	// measured the hint with the work. Its worst is the bound of the text and the most output.
	if p, a, err := lib.Price(CallHint, platform); err != nil || !a.Admitted || p.Expected || !p.Worst || !near(p.WorstUSD, (24000*5+16000*20)/1e6) {
		t.Errorf("the price of the hint without the work: %+v, %+v, %v", p, a, err)
	}
	withWords(t)
	if p, a, err := lib.Price(CallWords, platform); err != nil || a.Admitted || p != (CallPrice{}) {
		t.Errorf("the price of the words not admitted: %+v, %+v, %v", p, a, err)
	}
	if p, a, err := lib.Price(CallHint, platform); err != nil || a.Admitted || p != (CallPrice{}) {
		t.Errorf("the price of the hint not admitted: %+v, %+v, %v", p, a, err)
	}
	if _, _, err := lib.Price("voice", platform); err == nil {
		t.Error("a capability no call of the library serves has a price")
	}
}

// A model the measurements do not know has no expected price, and prices that do not hold the worst
// give no worst.
func TestPriceUnknown(t *testing.T) {
	lib, opus, _ := pricedPair(t)
	saved := kinds[CallSheet]
	defer func() { kinds[CallSheet] = saved }()
	k := saved
	k.expected = map[string]tokens{"claude-opus-5-5": {in: 1, out: 1}}
	kinds[CallSheet] = k
	opus.prices.LongAbove = 100
	p, _, err := lib.Price(CallSheet, platform)
	if err != nil || p.Expected || p.Worst {
		t.Errorf("price = %+v, %v; want neither expected nor worst", p, err)
	}
}

// The table of prices holds each admitted model of a provider with an adapter once, with its
// adapter's prices; a library without the adapter has none.
func TestPrices(t *testing.T) {
	lib, _, _ := pricedPair(t)
	table, err := lib.Prices()
	if err != nil || len(table) != 2 || table[0].Model != "claude-opus-5-5" || table[1].Model != "claude-haiku-5-5" ||
		table[0].Provider != "anthropic" || table[0].Prices.Input != 4 || table[1].Prices.Read != "2026-11-11" {
		t.Errorf("the table is %+v, %v", table, err)
	}
	none, err := New(Config{Table: lib.table})
	if err != nil {
		t.Fatal(err)
	}
	if table, err := none.Prices(); err != nil || len(table) != 0 {
		t.Errorf("a library without adapters has the table %+v, %v", table, err)
	}
}

// A record of usage carries the cache and the cost by the tokens; a call whose tokens are not its
// whole bill costs its worst.
func TestUsageCost(t *testing.T) {
	lib, opus, haiku := pricedPair(t)
	calls, a, refused := lib.prepare(CallSheet, platform)
	if refused != nil {
		t.Fatal(refused)
	}
	opus.resp.CacheReadTokens = 500
	opus.resp.CacheWrites = []provider.CacheWrite{{TTL: 5 * time.Minute, Tokens: 200}}
	haiku.resp.Partial, haiku.err = true, errors.New("the stream broke off")
	haiku.resp.CacheWrites = []provider.CacheWrite{{TTL: 5 * time.Minute, Tokens: 9000}}
	_, out := ask(context.Background(), a, calls, []provider.Request{{MaxOutputTokens: 16000}, {MaxOutputTokens: 16000}}, 8000)
	if len(out.Usage) != 2 {
		t.Fatalf("usage %+v", out.Usage)
	}
	o, h := out.Usage[0], out.Usage[1]
	if !o.Billed || !o.Priced || o.CacheReadTokens != 500 || len(o.CacheWrites) != 1 || !near(o.USD, (1000*4+500*0.2+200*5+100*20)/1e6) {
		t.Errorf("the usage of the reader that answered is %+v", o)
	}
	// The reader that broke off wrote 9 000 tokens to the cache, more than the kind's most input.
	if h.Billed || !h.Priced || !near(h.USD, (10000*0.1+16000*0.5)/1e6) {
		t.Errorf("the usage of the reader that broke off is %+v; want its worst", h)
	}
	// Prices never read price nothing.
	opus.prices, haiku.err, haiku.resp.Partial = provider.Prices{Input: 4, Output: 20}, nil, false
	if _, out = ask(context.Background(), a, calls, []provider.Request{{}, {}}, 0); out.Usage[0].Priced {
		t.Errorf("prices without the day they were read priced %+v", out.Usage[0])
	}
}

// The words ask the first model of the entry alone, and their price is that one model's.
func TestWordsPriceOneModel(t *testing.T) {
	lib, _, _ := pricedPair(t)
	saved := kinds[CallSheet]
	defer func() { kinds[CallSheet] = saved }()
	k := saved
	k.models = 1
	kinds[CallSheet] = k
	p, _, err := lib.Price(CallSheet, platform)
	if err != nil || !near(p.ExpectedUSD, (1354*4+177*20)/1e6) {
		t.Errorf("the price of one model = %+v, %v", p, err)
	}
	if kinds[CallWords].models != 1 {
		t.Errorf("the words ask %d models", kinds[CallWords].models)
	}
}

// A request longer than the text its worst price holds is refused before any call.
func TestCheckText(t *testing.T) {
	if err := checkText(provider.Request{Instructions: strings.Repeat("a", maxTextBytes/2), Text: strings.Repeat("b", maxTextBytes/2)}); err != nil {
		t.Errorf("a request of the bound is refused: %v", err)
	}
	if err := checkText(provider.Request{Text: strings.Repeat("я", maxTextBytes/2+1)}); err == nil {
		t.Error("a request of more bytes than the bound is not refused")
	}
	lib, opus, _ := pricedPair(t)
	read := lib.ReadSheet(context.Background(), SheetReading{Route: platform, Task: longTask(), Crop: cropPNG(t, 40, 30)})
	if read.Outcome.Kind != Failed || !strings.Contains(read.Outcome.Err.Error(), "bytes of text") || len(opus.asked) != 0 {
		t.Errorf("a long statement: %+v, asked %d", read.Outcome, len(opus.asked))
	}
}

// longTask is a task whose statement takes more bytes than a request may.
func longTask() *open.Task {
	t := sheetOf()
	t.Statement = []open.StatementPart{{Text: text(strings.Repeat("Ann is 3 years older than Ben. ", 1000))}}
	return t
}
