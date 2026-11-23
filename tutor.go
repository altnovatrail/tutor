package tutor

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/altnovatrail/spec/types/admissionv0"

	"github.com/altnovatrail/tutor/admission"
	"github.com/altnovatrail/tutor/provider"
)

// Config is what api gives tutor when it starts: spec's admission table, the deployment's facts,
// the least stage of admission its learners take — admission.StageAuthor where the operator names
// it, as novatrail does for adults —, the platform's keys at the AI providers, which api reads
// from Secret Manager, and the adapters of the providers whose models the service calls, such as
// claude.Dial of package provider/claude. The library builds no adapter in, so that a service
// carries the SDKs of the adapters it gives alone. A provider with no adapter here is no route at
// all, and one with no key no route of platform AI: Choose takes none of its models.
// Reading is how the library reads a sheet and a workbook page, by the pair or by one reader; an
// empty one is the pair. Debug lets a call's route carry the debug mode, Route.Debug, which api sets
// for an account with the right to it, on a local run or on the deployment; off, such a route
// fails.
type Config struct {
	Table    *admission.Table
	Facts    admission.Facts
	Least    admission.Stage
	Reading  Reading
	Keys     map[admissionv0.Provider]string
	Adapters map[admissionv0.Provider]provider.Dial
	Debug    bool
}

// Reading is how the library reads the handwriting of a sheet and of a workbook page, which api
// chooses: ReadingPair, both readers of the entry chosen, the strong one first,
// whose readings module-kit joins, or ReadingSingle, one reader, Haiku 5.5 at
// medium, the pair's second, whose observation is the reading. The reading of an essay is one
// reader in both.
type Reading string

// The modes of reading.
const (
	ReadingPair   Reading = "pair"
	ReadingSingle Reading = "single"
)

// singleReader is the one reader of ReadingSingle, as the list names it: its provider, ID and effort.
// It is found in the entry chosen as it is, not by its place there, so that an entry whose pair is
// ordered or made otherwise does not give another model.
var singleReader = admission.Model{Provider: "anthropic", ID: "claude-haiku-5-5", Effort: "medium"}

// errNoSingleReader is the failure of a reading in ReadingSingle whose entry chosen has no single
// reader.
var errNoSingleReader = errors.New("tutor: the reading is single, and the entry chosen has no " + singleReader.Name())

// Tutor is the library as api holds it: made once, at the start, and called by many requests at
// once. It keeps nothing of a call.
type Tutor struct {
	table   *admission.Table
	facts   admission.Facts
	least   admission.Stage
	reading Reading
	debug   bool
	keys    map[admissionv0.Provider]string
	// adapters are the adapters of Config, by provider.
	adapters map[admissionv0.Provider]provider.Dial
	// clients are the clients of the platform's keys, made once for each model and kept, so that
	// a request does not build them again; a client of an own key lives only in its request.
	mu      sync.Mutex
	clients map[admission.Model]provider.Model
}

// New makes the library of a config. It refuses a config with no table, a least stage that is
// neither of the two — an empty one is the stage full, the strict one —, a mode of reading that is
// neither of the two — an empty one is the pair —, and a key of a provider the config gives no
// adapter of, which would leave every model of the provider unchosen with no word of why.
func New(c Config) (*Tutor, error) {
	if c.Table == nil {
		return nil, errors.New("tutor: the config has no admission table")
	}
	least := c.Least
	switch least {
	case "":
		least = admission.StageFull
	case admission.StageAuthor, admission.StageFull:
	default:
		return nil, fmt.Errorf("tutor: the least stage %q is neither author nor full", c.Least)
	}
	reading := c.Reading
	switch reading {
	case "":
		reading = ReadingPair
	case ReadingPair, ReadingSingle:
	default:
		return nil, fmt.Errorf("tutor: the mode of reading %q is neither pair nor single", c.Reading)
	}
	adapters := map[admissionv0.Provider]provider.Dial{}
	for p, d := range c.Adapters {
		if d != nil {
			adapters[p] = d
		}
	}
	for _, p := range slices.Sorted(maps.Keys(c.Keys)) {
		if _, ok := adapters[p]; !ok && c.Keys[p] != "" {
			return nil, fmt.Errorf("tutor: the config gives a key of %s and no adapter of it", p)
		}
	}
	return &Tutor{table: c.Table, facts: c.Facts, least: least, reading: reading, debug: c.Debug, keys: maps.Clone(c.Keys), adapters: adapters, clients: map[admission.Model]provider.Model{}}, nil
}

// dial makes the client of a model of the list with a key, by the adapter of its provider that the
// config gave.
func (t *Tutor) dial(m admission.Model, key string) (provider.Model, error) {
	d, ok := t.adapters[m.Provider]
	if !ok {
		return nil, fmt.Errorf("tutor: the config gives no adapter of %s", m.Provider)
	}
	return d(m.ID, key)
}

// Route is what every call of the library carries besides its work: the AI route, the learner's age
// category, on the own key the key with its provider, and for an account with its right the debug
// mode. It names no learner, no account and no adult.
type Route struct {
	AI       admissionv0.AiRoute
	Category admissionv0.AgeCategory
	Own      *OwnKey
	Debug    *Debug
}

// OwnKey is the key of an account owner who learns, with its provider. It lives only in
// the memory of the request: its fields are unexported, it prints as its provider alone with any
// verb, and as JSON it is nothing. The key itself stands behind a pointer, so that even an OwnKey
// printed field by field, as fmt prints an unexported field of another struct, shows an address
// rather than the key.
type OwnKey struct {
	provider admissionv0.Provider
	key      *secret
}

// secret is a key, behind a pointer.
type secret struct{ value string }

// NewOwnKey is the own key of a provider.
func NewOwnKey(p admissionv0.Provider, key string) *OwnKey {
	return &OwnKey{provider: p, key: &secret{value: key}}
}

// value is the key, or empty for none.
func (k *OwnKey) value() string {
	if k == nil || k.key == nil {
		return ""
	}
	return k.key.value
}

// Provider is the key's provider.
func (k *OwnKey) Provider() admissionv0.Provider { return k.provider }

// String names the key by its provider, never the key itself.
func (k OwnKey) String() string { return "own key of " + string(k.provider) }

// GoString is String, so that %#v prints no key either.
func (k OwnKey) GoString() string { return k.String() }

// Format prints String with every verb, so that no verb reaches the fields.
func (k OwnKey) Format(f fmt.State, _ rune) { _, _ = f.Write([]byte(k.String())) }

// check refuses a route whose key does not fit it: the own key goes with the route own_key alone,
// and that route has one.
func (r Route) check() error {
	switch {
	case r.AI == admissionv0.AiRouteOwnKey && (r.Own.value() == "" || r.Own.provider == ""):
		return errors.New("tutor: the route is the own key, and it has no key or no provider")
	case r.AI != admissionv0.AiRouteOwnKey && r.Own != nil:
		return fmt.Errorf("tutor: the route is %q, and it carries an own key", r.AI)
	}
	return nil
}

// Choose chooses the models of a capability for a route (admission.Table.Select), the first entry
// of the capability whatever prompt it measured. A call of the library chooses by its kind, which
// names its prompt too (ChooseCall): that is the one api calls before an attempt, with the same
// answer the call gets while the config is the same. It takes only the models of the providers the
// config gives an adapter of: on the own key only those of the key's provider, and on platform AI
// only those of the providers it holds a key of. It refuses a route whose key does not fit it. The
// entry holds every model it has: the mode of reading is of a kind of call, and ChooseCall applies
// it.
func (t *Tutor) Choose(c admissionv0.Capability, r Route) (admission.Entry, admission.Answer, error) {
	return t.choose(c, r, "")
}

// ChooseCall chooses the models of a kind of call of the library for a route: of its capability, of
// the entries that measured its prompt (admission.Table.SelectPrompt), and for the words
// with the work only with the hint on the same model. api calls it before an attempt, and the
// call before it calls a model. The entry holds only the models the call asks: the first alone for
// a call of one model, and the one reader of ReadingSingle for a reading of a sheet or a page.
func (t *Tutor) ChooseCall(c Call, r Route) (admission.Entry, admission.Answer, error) {
	if _, ok := kinds[c]; !ok {
		return admission.Entry{}, admission.Answer{}, fmt.Errorf("tutor: no call %q of the library", c)
	}
	return t.chooseCall(c, r)
}

// chooseCall is ChooseCall: the one choice of a kind of call that the call, ChooseCall, Price and
// CheckKey make, with the models the call asks (Tutor.asked), so that what api's gateway admits and
// reserves a quota for, and the models a key is checked for, are those the call asks. The
// words with the work word the hint in the same call, so they are admitted only when the hint is
// admitted on the model of the analysis: when the model the hint without the work chooses, by
// the same prompt, is the analysis's first, and of a cascade, the words of a task from paper, when
// its models are the analysis's, in their order. Otherwise the answer is the hint's refusal,
// or model_not_admitted when the hint is another model's — another ID or another effort (R111).
func (t *Tutor) chooseCall(c Call, r Route) (admission.Entry, admission.Answer, error) {
	k := kinds[c]
	e, a, err := t.choose(k.capability, r, k.prompt)
	if err != nil {
		return e, a, err
	}
	// The debug mode bypasses the list: its models serve the call, and the admission's refusal
	// stands only where it names none.
	if d := debugOf(c, r); d != nil && (a.Admitted || d.First != nil) {
		e.Models = d.models(c, k.asked(e.Models))
		a.Admitted, a.Reason = true, nil
		return e, a, nil
	}
	if !a.Admitted {
		return e, a, nil
	}
	if e.Models, err = t.asked(c, e.Models); err != nil {
		return admission.Entry{}, a, err
	}
	hc, ok := wordsHint[c]
	if !ok {
		return e, a, nil
	}
	he, ha, err := t.chooseCall(hc, r)
	if err != nil || !ha.Admitted {
		return admission.Entry{}, ha, err
	}
	if len(he.Models) == 0 || len(e.Models) == 0 || he.Models[0] != e.Models[0] || kinds[c].cascade && !slices.Equal(he.Models, e.Models) {
		refusal := admission.ReasonModelNotAdmitted
		return admission.Entry{}, admission.Answer{Answer: admissionv0.Answer{Reason: &refusal}, Version: a.Version}, nil
	}
	return e, a, nil
}

// wordsHint is the kind of the hint without the work that a kind of words with the work words in
// the same call.
var wordsHint = map[Call]Call{CallWords: CallHint, CallPaperWords: CallPaperHint}

// asked are the models of an entry a kind of call asks (kind.asked), and for a reading by the pair in
// ReadingSingle the one reader alone, found as it is. An entry without it is errNoSingleReader: the
// mode asks no other model in its place.
func (t *Tutor) asked(c Call, models []admission.Model) ([]admission.Model, error) {
	k := kinds[c]
	if t.reading != ReadingSingle || !k.pair {
		return k.asked(models), nil
	}
	if !slices.Contains(models, singleReader) {
		return nil, fmt.Errorf("%w, for the call %s", errNoSingleReader, c)
	}
	return []admission.Model{singleReader}, nil
}

// choose is Choose, of the entries whose prompt is the one named, or of every entry when it is
// empty.
func (t *Tutor) choose(c admissionv0.Capability, r Route, prompt string) (admission.Entry, admission.Answer, error) {
	if err := r.check(); err != nil {
		return admission.Entry{}, admission.Answer{}, err
	}
	if r.Debug != nil && !t.debug {
		return admission.Entry{}, admission.Answer{}, errDebugOff
	}
	providers := []admissionv0.Provider{}
	if r.Own != nil {
		if _, ok := t.adapters[r.Own.provider]; ok {
			providers = append(providers, r.Own.provider)
		}
	} else {
		for _, p := range slices.Sorted(maps.Keys(t.keys)) {
			if _, ok := t.adapters[p]; ok && t.keys[p] != "" {
				providers = append(providers, p)
			}
		}
	}
	if prompt != "" {
		e, a := t.table.SelectPrompt(t.facts, c, r.AI, r.Category, t.least, providers, prompt)
		return e, a, nil
	}
	e, a := t.table.Select(t.facts, c, r.AI, r.Category, t.least, providers)
	return e, a, nil
}

// Kind is how a call of the library ended: the models answered; AI failed; a provider refused for a
// limit, a balance or billing, whose reason Outcome.Limit names; the admission refused the call,
// before any model was asked; or the provider refused the own key, wrong or revoked.
// None but the first is the learner's mistake.
type Kind int

// The kinds of outcome.
const (
	Answered Kind = iota + 1
	Failed
	LimitReached
	NotAdmitted
	KeyRefused
)

func (k Kind) String() string {
	switch k {
	case Answered:
		return "answered"
	case Failed:
		return "failed"
	case LimitReached:
		return "limit reached"
	case NotAdmitted:
		return "not admitted"
	case KeyRefused:
		return "key refused"
	}
	return fmt.Sprintf("kind %d", int(k))
}

// LimitReason is why a provider refused for a limit: one only the owner of the key can lift, each
// in its own way. It is the adapters' own, provider.Limit, so that a reason they tell is one the
// library gives.
type LimitReason = provider.Limit

// The reasons of a limit reached.
const (
	LimitOwner   = provider.LimitOwner
	LimitTier    = provider.LimitTier
	LimitBalance = provider.LimitBalance
	LimitBilling = provider.LimitBilling
)

// Usage is a record of what one call of a model took: the provider and the model the provider says
// answered; the input tokens without the cache, those written to the cache by how long it keeps
// them and those read from it, which add up to the input with no token counted twice; the output
// tokens, and of them those spent thinking; whether those tokens are the call's whole bill
// (provider.Billed); its cost in US dollars by the table of prices (Tutor.Prices) — at its worst
// when the tokens are not its whole bill —, which Priced says the prices hold, and it is an
// estimate: the provider bills; how long the call took, from when the library asked the adapter to
// when the answer or the failure came back, the SDK's retries counted in — the span the eval runner
// times as latency_ms —, and how many times the SDK sent it again; and when the first text
// of the answer came, counted from the attempt that answered, or zero where none came or the
// adapter does not measure it, as GPT's does not. It holds no content and no identifier, and may be
// written to logs.
type Usage struct {
	Provider        admissionv0.Provider
	Model           string
	InputTokens     int64
	CacheWrites     []provider.CacheWrite
	CacheReadTokens int64
	OutputTokens    int64
	ThinkingTokens  int64
	Billed          bool
	USD             float64
	Priced          bool
	Latency         time.Duration
	Retries         int
	FirstToken      time.Duration
}

// Outcome is how a call of the library ended: its kind; the reason of a limit, and when access
// returns, where the provider's terms say it — at Anthropic, the cap of the tier, at 00:00 UTC on
// the first day of the next month by the provider's clock —, zero where they do not, which api
// gives the key's owner as access_returns; the admission's answer, which on NotAdmitted refuses;
// and a record of usage for each call of a model that came back, in the order of the models, none
// on NotAdmitted. Err says what failed, for the logs: it carries no content and no key.
type Outcome struct {
	Kind          Kind
	Limit         LimitReason
	AccessReturns time.Time
	Admission     admission.Answer
	Usage         []Usage
	Err           error
}

// call is a model of the entry chosen with its client, and in the debug mode the kind of the
// library's call, the reader of the work and the mode, which records it. attempt is the model's
// place among the attempts of a cascade, from 1, and 0 where the call asks its models at once.
type call struct {
	model   admission.Model
	client  provider.Model
	kind    Call
	reader  int
	attempt int
	debug   *Debug
}

// prepare chooses the models of a kind of call for a route (chooseCall), those the kind asks, and
// makes their clients — with the own key by the key, on platform AI by the platform's key of the
// provider —, and gives the admission's answer, which an outcome of the call carries. A refusal of
// the admission is the outcome NotAdmitted, before any model is asked and with no usage; a route
// that does not fit, an entry without the one reader of ReadingSingle and a client that cannot be
// made are Failed. A call that asks one model makes no client of another.
func (t *Tutor) prepare(c Call, r Route) ([]call, admission.Answer, *Outcome) {
	e, a, err := t.chooseCall(c, r)
	if err != nil {
		return nil, a, &Outcome{Kind: Failed, Err: err}
	}
	if !a.Admitted {
		return nil, a, &Outcome{Kind: NotAdmitted, Admission: a}
	}
	// The models of a cascade are its attempts, one after another, unless the debug mode chose the
	// models of the call.
	cascading := kinds[c].cascade && !debugOf(c, r).chose(c)
	calls := make([]call, 0, len(e.Models))
	for _, m := range e.Models {
		client, err := t.client(m, r.Own)
		if err != nil {
			return nil, a, &Outcome{Kind: Failed, Admission: a, Err: err}
		}
		cl := call{model: m, client: client, kind: c, debug: debugOf(c, r)}
		switch {
		case cascading:
			cl.attempt = len(calls) + 1
			if c == CallPaper {
				cl.reader = 1
			}
		case c == CallPaper:
			cl.reader = len(calls) + 1
		}
		calls = append(calls, cl)
	}
	return calls, a, nil
}

// client is the client of a model: of the own key, made for the request; of the platform's key,
// made once and kept.
func (t *Tutor) client(m admission.Model, own *OwnKey) (provider.Model, error) {
	if own != nil {
		return t.dial(m, own.value())
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if c, ok := t.clients[m]; ok {
		return c, nil
	}
	// Choose takes on platform AI only the providers the library holds a key of.
	c, err := t.dial(m, t.keys[m.Provider])
	if err != nil {
		return nil, err
	}
	t.clients[m] = c
	return c, nil
}

// ask asks every model of the calls at once — the readers of a pair go in parallel — each its
// request with the model's effort and thinking as the list admitted them, and times each, so that
// its record of usage says which model held the call. It returns the answers in the order of the
// calls and the outcome: Answered when every model answered, else Failed, with the usage of each
// call that came back with tokens, a refused or a broken answer too, since it is billed; Billed
// says whether its tokens are its whole bill, and a call that is not is priced at its worst: maxIn,
// the most input of the kind, and the request's most output. A provider's refusal for a limit of
// any model is the outcome LimitReached with its reason, and a refusal of the key KeyRefused
// (provider.LimitError, provider.ErrKeyRefused); the usage of a model that answered stays, since it
// is billed.
func ask(ctx context.Context, a admission.Answer, calls []call, reqs []provider.Request, maxIn int64) ([]provider.Response, Outcome) {
	if len(reqs) != len(calls) {
		return nil, Outcome{Kind: Failed, Admission: a, Err: fmt.Errorf("tutor: %d requests for %d models", len(reqs), len(calls))}
	}
	resps := make([]provider.Response, len(calls))
	errs := make([]error, len(calls))
	took := make([]time.Duration, len(calls))
	var wg sync.WaitGroup
	for i, c := range calls {
		req := reqs[i]
		req.Effort, req.NoThinking = c.model.Effort, c.model.NoThinking
		wg.Go(func() {
			start := time.Now()
			// A panic of an adapter or its SDK fails the call rather than the process of api, which
			// would not recover it in a goroutine of its own.
			defer func() {
				took[i] = time.Since(start)
				if p := recover(); p != nil {
					errs[i] = fmt.Errorf("the adapter panicked: %v", p)
				}
			}()
			resps[i], errs[i] = c.client.Ask(ctx, req)
		})
	}
	wg.Wait()
	out := Outcome{Kind: Answered, Admission: a}
	for i, c := range calls {
		// A call that came back with tokens is billed whatever ended it: a refused answer, one that
		// broke off midway, one the context cut short.
		var used *Usage
		if r := resps[i]; errs[i] == nil || errors.Is(errs[i], provider.ErrNoAnswer) || r.Partial || r.Counted() {
			u := usageOf(c.model, c.client, r, errs[i], took[i], maxIn, reqs[i].MaxOutputTokens)
			out.Usage, used = append(out.Usage, u), &u
		}
		if c.debug != nil && c.debug.Record != nil {
			c.debug.Record(DebugCall{Call: c.kind, Reader: c.reader, Attempt: max(c.attempt, 1), Model: c.model, Answer: resps[i].JSON, Usage: used, Err: errs[i]})
		}
		if errs[i] == nil {
			continue
		}
		// A refusal for a limit ends the call whatever the others did, and a refused key comes
		// next: a reading by the pair gives both observations or the outcome, never one. Of two
		// limits, the one that holds the call longer: one with no time to return, which only the
		// owner lifts, then the later time.
		kind, limit, returns := Failed, LimitReason(""), time.Time{}
		var le *provider.LimitError
		switch {
		case errors.As(errs[i], &le):
			kind, limit, returns = LimitReached, le.Limit, le.Returns
		case errors.Is(errs[i], provider.ErrKeyRefused):
			kind = KeyRefused
		}
		if out.Err == nil || rank(kind) > rank(out.Kind) || kind == LimitReached && out.Kind == LimitReached && longer(returns, out.AccessReturns) {
			out.Kind, out.Limit, out.AccessReturns, out.Err = kind, limit, returns, fmt.Errorf("tutor: %s of %s: %w", c.model.Name(), c.model.Provider, errs[i])
		}
	}
	return resps, out
}

// longer tells whether a limit whose access returns at a holds the call longer than one whose
// access returns at b: zero is no time, longer than any.
func longer(a, b time.Time) bool {
	return !b.IsZero() && (a.IsZero() || a.After(b))
}

// rank orders the kinds of a failed call, the one an outcome takes last: a limit, then a refused
// key, then a failure of AI.
func rank(k Kind) int {
	switch k {
	case LimitReached:
		return 3
	case KeyRefused:
		return 2
	case Failed:
		return 1
	}
	return 0
}
