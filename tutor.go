package tutor

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"

	"github.com/altnovatrail/spec/types/admissionv0"

	"github.com/altnovatrail/tutor/admission"
	"github.com/altnovatrail/tutor/provider"
)

// Config is what api gives tutor when it starts (architecture 3.10): spec's admission table, the
// deployment's facts, the least stage of admission its learners take — admission.StageAuthor only
// where the author alone learns, as in the first app (R76 spec; R80) —, the platform's keys at the
// AI providers, which api reads from Secret Manager (architecture 1), and the adapters of the
// providers whose models the service calls, such as claude.Dial of package provider/claude. The
// library builds no adapter in, so that a service carries the SDKs of the adapters it gives alone
// (Д185 RUN.md spec; R90). A provider with no adapter here is no route at all, and one with no key
// no route of platform AI: Choose takes none of its models.
type Config struct {
	Table    *admission.Table
	Facts    admission.Facts
	Least    admission.Stage
	Keys     map[admissionv0.Provider]string
	Adapters map[admissionv0.Provider]provider.Dial
}

// Tutor is the library as api holds it: made once, at the start, and called by many requests at
// once. It keeps nothing of a call (R09 spec).
type Tutor struct {
	table *admission.Table
	facts admission.Facts
	least admission.Stage
	keys  map[admissionv0.Provider]string
	// adapters are the adapters of Config, by provider.
	adapters map[admissionv0.Provider]provider.Dial
	// clients are the clients of the platform's keys, made once for each model and kept, so that
	// a request does not build them again; a client of an own key lives only in its request.
	mu      sync.Mutex
	clients map[admission.Model]provider.Model
}

// New makes the library of a config. It refuses a config with no table, a least stage that is
// neither of the two — an empty one is the stage full, the strict one —, and a key of a provider the
// config gives no adapter of, which would leave every model of the provider unchosen with no word
// of why.
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
	return &Tutor{table: c.Table, facts: c.Facts, least: least, keys: maps.Clone(c.Keys), adapters: adapters, clients: map[admission.Model]provider.Model{}}, nil
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

// Route is what every call of the library carries besides its work (architecture 3.10): the AI
// route, the learner's age category, and on the own key the key with its provider. It names no
// learner, no account and no adult (R15 spec; privacy 5.1 spec).
type Route struct {
	AI       admissionv0.AiRoute
	Category admissionv0.AgeCategory
	Own      *OwnKey
}

// OwnKey is the key of an account owner who learns, with its provider (R84 spec). It lives only in
// the memory of the request (privacy 5.2 spec): its fields are unexported, it prints as its provider
// alone with any verb, and as JSON it is nothing. The key itself stands behind a pointer, so that
// even an OwnKey printed field by field, as fmt prints an unexported field of another struct, shows
// an address rather than the key.
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
// answer the call gets while the config is the same. It takes only the models of the providers the config gives an
// adapter of: on the own key only those of the key's provider, and on platform AI only those of the
// providers it holds a key of. It refuses a route whose key does not fit it.
func (t *Tutor) Choose(c admissionv0.Capability, r Route) (admission.Entry, admission.Answer, error) {
	return t.choose(c, r, "")
}

// ChooseCall chooses the models of a kind of call of the library for a route: of its capability, of
// the entries that measured its prompt (admission.Table.SelectPrompt; R70 spec). api calls it before
// an attempt, and the call before it calls a model. The entry holds only the models the call asks:
// the first alone for a call of one model.
func (t *Tutor) ChooseCall(c Call, r Route) (admission.Entry, admission.Answer, error) {
	k, ok := kinds[c]
	if !ok {
		return admission.Entry{}, admission.Answer{}, fmt.Errorf("tutor: no call %q of the library", c)
	}
	e, a, err := t.choose(k.capability, r, k.prompt)
	e.Models = k.asked(e.Models)
	return e, a, err
}

// choose is Choose, of the entries whose prompt is the one named, or of every entry when it is
// empty.
func (t *Tutor) choose(c admissionv0.Capability, r Route, prompt string) (admission.Entry, admission.Answer, error) {
	if err := r.check(); err != nil {
		return admission.Entry{}, admission.Answer{}, err
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

// Kind is how a call of the library ended (architecture 3.10): the models answered; AI failed; a
// provider refused for a limit, a balance or billing, whose reason Outcome.Limit names
// (architecture 3.7); the admission refused the call, before any model was asked (architecture
// 3.9); or the provider refused the own key, wrong or revoked (R84 spec). None but the first is
// the learner's mistake (R04 spec).
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

// LimitReason is why a provider refused for a limit: one only the owner of the key can lift, each in
// its own way (architecture 3.7). It is the adapters' own, provider.Limit, so that a reason they
// tell is one the library gives.
type LimitReason = provider.Limit

// The reasons of a limit reached.
const (
	LimitOwner   = provider.LimitOwner
	LimitTier    = provider.LimitTier
	LimitBalance = provider.LimitBalance
	LimitBilling = provider.LimitBilling
)

// Usage is a record of what one call of a model took (architecture 3.7): the provider and the model
// the provider says answered; the input tokens without the cache, those written to the cache by how
// long it keeps them and those read from it, which add up to the input with no token counted twice;
// the output tokens, and of them those spent thinking; whether those tokens are the call's whole
// bill (provider.Billed); and its cost in US dollars by the table of prices (Tutor.Prices) — at its
// worst when the tokens are not its whole bill —, which Priced says the prices hold. It is an
// estimate: the provider bills. It holds no content and no identifier, and may be written to logs
// (R09 spec).
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
}

// Outcome is how a call of the library ended: its kind; the reason of a limit; the admission's
// answer, which on NotAdmitted refuses; and a record of usage for each call of a model that came
// back, in the order of the models, none on NotAdmitted. Err says what failed, for the logs: it
// carries no content and no key.
type Outcome struct {
	Kind      Kind
	Limit     LimitReason
	Admission admission.Answer
	Usage     []Usage
	Err       error
}

// call is a model of the entry chosen with its client.
type call struct {
	model  admission.Model
	client provider.Model
}

// prepare chooses the models of a capability for a route — of the entries that measured the prompt
// named, or of every entry when it is empty —, the first n of them, all when n is 0, and makes their
// clients — with the own key by the key, on platform AI by the platform's key of the provider —, and
// gives the admission's answer, which an outcome of the call carries. A refusal of the admission is
// the outcome NotAdmitted, before any model is asked and with no usage; a route that does not fit
// and a client that cannot be made are Failed. A call that asks one model makes no client of another.
func (t *Tutor) prepare(c admissionv0.Capability, r Route, n int, prompt string) ([]call, admission.Answer, *Outcome) {
	e, a, err := t.choose(c, r, prompt)
	if err != nil {
		return nil, a, &Outcome{Kind: Failed, Err: err}
	}
	if !a.Admitted {
		return nil, a, &Outcome{Kind: NotAdmitted, Admission: a}
	}
	models := e.Models
	if n > 0 && n < len(models) {
		models = models[:n]
	}
	calls := make([]call, 0, len(models))
	for _, m := range models {
		client, err := t.client(m, r.Own)
		if err != nil {
			return nil, a, &Outcome{Kind: Failed, Admission: a, Err: err}
		}
		calls = append(calls, call{model: m, client: client})
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

// ask asks every model of the calls at once — the readers of a pair go in parallel (Д89 RUN.md
// spec) — each its request with the model's effort and thinking as the list admitted them. It
// returns the answers in the order of the calls and the outcome: Answered when every model
// answered, else Failed, with the usage of each call that came back with tokens, a refused or a
// broken answer too, since it is billed; Billed says whether its tokens are its whole bill, and
// a call that is not is priced at its worst: maxIn, the most input of the kind, and the request's
// most output. A provider's refusal for a limit of any model is the outcome LimitReached with its
// reason, and a refusal of the key KeyRefused (provider.LimitError, provider.ErrKeyRefused); the
// usage of a model that answered stays, since it is billed.
func ask(ctx context.Context, a admission.Answer, calls []call, reqs []provider.Request, maxIn int64) ([]provider.Response, Outcome) {
	if len(reqs) != len(calls) {
		return nil, Outcome{Kind: Failed, Admission: a, Err: fmt.Errorf("tutor: %d requests for %d models", len(reqs), len(calls))}
	}
	resps := make([]provider.Response, len(calls))
	errs := make([]error, len(calls))
	var wg sync.WaitGroup
	for i, c := range calls {
		req := reqs[i]
		req.Effort, req.NoThinking = c.model.Effort, c.model.NoThinking
		wg.Go(func() {
			// A panic of an adapter or its SDK fails the call rather than the process of api, which
			// would not recover it in a goroutine of its own.
			defer func() {
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
		if r := resps[i]; errs[i] == nil || errors.Is(errs[i], provider.ErrNoAnswer) || r.Partial || r.Counted() {
			out.Usage = append(out.Usage, usageOf(c.model, c.client, r, errs[i], maxIn, reqs[i].MaxOutputTokens))
		}
		if errs[i] == nil {
			continue
		}
		// A refusal for a limit ends the call whatever the others did, and a refused key comes next:
		// the pair gives both observations or the outcome, never one (architecture 3.7).
		kind, limit := Failed, LimitReason("")
		var le *provider.LimitError
		switch {
		case errors.As(errs[i], &le):
			kind, limit = LimitReached, le.Limit
		case errors.Is(errs[i], provider.ErrKeyRefused):
			kind = KeyRefused
		}
		if out.Err == nil || rank(kind) > rank(out.Kind) {
			out.Kind, out.Limit, out.Err = kind, limit, fmt.Errorf("tutor: %s of %s: %w", c.model.Name(), c.model.Provider, errs[i])
		}
	}
	return resps, out
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
