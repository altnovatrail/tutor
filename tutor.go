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
	"github.com/altnovatrail/tutor/provider/claude"
	"github.com/altnovatrail/tutor/provider/gpt"
)

// Config is what api gives tutor when it starts (architecture 3.10): spec's admission table, the
// deployment's facts, the least stage of admission its learners take — admission.StageAuthor only
// where the author alone learns, as in the first app (R76 spec; R80) — and the platform's keys at
// the AI providers, which api reads from Secret Manager (architecture 1). A provider with no key
// here is no route of platform AI: Choose takes none of its models.
type Config struct {
	Table *admission.Table
	Facts admission.Facts
	Least admission.Stage
	Keys  map[admissionv0.Provider]string
}

// Tutor is the library as api holds it: made once, at the start, and called by many requests at
// once. It keeps nothing of a call (R09 spec).
type Tutor struct {
	table *admission.Table
	facts admission.Facts
	least admission.Stage
	keys  map[admissionv0.Provider]string
	dial  dialer
	// clients are the clients of the platform's keys, made once for each model and kept, so that
	// a request does not build them again; a client of an own key lives only in its request.
	mu      sync.Mutex
	clients map[admission.Model]provider.Model
}

// dialer makes the client of a model of the list with a key: tutor makes it, so that the providers'
// SDKs reach api's build through tutor alone (R15 spec). Tests give their own.
type dialer func(m admission.Model, key string) (provider.Model, error)

// New makes the library of a config. It refuses a config with no table, and a least stage that is
// neither of the two: an empty one is the stage full, the strict one.
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
	return &Tutor{table: c.Table, facts: c.Facts, least: least, keys: maps.Clone(c.Keys), dial: dial, clients: map[admission.Model]provider.Model{}}, nil
}

// dial makes the client of a model of Anthropic or OpenAI, by the adapters of tutor.
func dial(m admission.Model, key string) (provider.Model, error) {
	switch m.Provider {
	case "anthropic":
		i := slices.IndexFunc(claude.Models, func(c claude.Model) bool { return c.ID == m.ID })
		if i >= 0 {
			return claude.New(key, claude.Models[i]), nil
		}
	case "openai":
		if m.ID == gpt.Model {
			return gpt.New(key), nil
		}
	}
	return nil, fmt.Errorf("tutor: no adapter calls %s of %s", m.ID, m.Provider)
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

// Choose chooses the models of a capability for a route (admission.Table.Select): api calls it
// before an attempt, and each call of the library before it calls a model, with the same answer
// while the config is the same. On the own key it takes only the models of the key's provider, and
// on platform AI only those of the providers it holds a key of. It refuses a route whose key does
// not fit it.
func (t *Tutor) Choose(c admissionv0.Capability, r Route) (admission.Entry, admission.Answer, error) {
	if err := r.check(); err != nil {
		return admission.Entry{}, admission.Answer{}, err
	}
	providers := []admissionv0.Provider{}
	if r.Own != nil {
		providers = append(providers, r.Own.provider)
	} else {
		for _, p := range slices.Sorted(maps.Keys(t.keys)) {
			if t.keys[p] != "" {
				providers = append(providers, p)
			}
		}
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
// its own way (architecture 3.7).
type LimitReason string

// The reasons of a limit reached.
const (
	LimitOwner   LimitReason = "owner_limit"
	LimitTier    LimitReason = "tier"
	LimitBalance LimitReason = "balance"
	LimitBilling LimitReason = "billing"
)

// Usage is a record of what one call of a model took (architecture 3.7): the provider and the model
// the provider says answered, the input, output and thinking tokens, and whether those tokens are
// the call's whole bill (provider.Billed). The cache and the price in dollars come with T120. It
// holds no content and no identifier, and may be written to logs (R09 spec).
type Usage struct {
	Provider       admissionv0.Provider
	Model          string
	InputTokens    int64
	OutputTokens   int64
	ThinkingTokens int64
	Billed         bool
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

// prepare chooses the models of a capability for a route and makes their clients — with the own key
// by the key, on platform AI by the platform's key of the provider — and gives the admission's
// answer, which an outcome of the call carries. A refusal of the admission is
// the outcome NotAdmitted, before any model is asked and with no usage; a route that does not fit
// and a client that cannot be made are Failed.
func (t *Tutor) prepare(c admissionv0.Capability, r Route) ([]call, admission.Answer, *Outcome) {
	return t.prepareN(c, r, 0)
}

// prepareN is prepare of the first n models of the entry chosen, all of them when n is 0: a call
// that asks one model makes no client of another.
func (t *Tutor) prepareN(c admissionv0.Capability, r Route, n int) ([]call, admission.Answer, *Outcome) {
	e, a, err := t.Choose(c, r)
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
// broken answer too, since it is billed; Billed says whether its tokens are its whole bill. Which refusals are a limit or a refused key, the adapters tell with T121.
func ask(ctx context.Context, a admission.Answer, calls []call, reqs []provider.Request) ([]provider.Response, Outcome) {
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
		if r := resps[i]; errs[i] == nil || errors.Is(errs[i], provider.ErrNoAnswer) || r.Partial || r.InputTokens > 0 || r.OutputTokens > 0 {
			out.Usage = append(out.Usage, Usage{Provider: c.model.Provider, Model: r.Model, InputTokens: r.InputTokens, OutputTokens: r.OutputTokens, ThinkingTokens: r.ThinkingTokens, Billed: provider.Billed(r, errs[i])})
		}
		if errs[i] != nil && out.Err == nil {
			out.Kind, out.Err = Failed, fmt.Errorf("tutor: %s of %s: %w", c.model.Name(), c.model.Provider, errs[i])
		}
	}
	return resps, out
}
