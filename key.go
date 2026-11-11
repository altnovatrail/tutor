package tutor

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/altnovatrail/spec/types/admissionv0"

	"github.com/altnovatrail/tutor/admission"
	"github.com/altnovatrail/tutor/provider"
)

// KeyCheck is how the check of an own key ended (R84 spec; architecture 3.7), as spec's contract
// names it (OwnKeyCheck): the provider took it; the key is wrong, revoked or expired; it is right and
// has no rights to a model it is needed for; the provider is closed to an own key at the profile, and
// no call went; or the provider did not answer, and the check says nothing of the key.
type KeyCheck int

// The ends of a check of a key.
const (
	KeyAccepted KeyCheck = iota + 1
	KeyWrong
	KeyNoRights
	KeyProviderClosed
	KeyUnanswered
)

func (k KeyCheck) String() string {
	switch k {
	case KeyAccepted:
		return "accepted"
	case KeyWrong:
		return "wrong"
	case KeyNoRights:
		return "no rights"
	case KeyProviderClosed:
		return "provider closed"
	case KeyUnanswered:
		return "unanswered"
	}
	return fmt.Sprintf("key check %d", int(k))
}

// keyChoices are the capabilities a call of the library serves, each with the prompt of the call,
// whose models an own key is needed for: the reading of a sheet and of a workbook page, and the
// words, which serve the analysis of the steps with the work and the hint without it (kinds).
func keyChoices() []kind {
	out := []kind{{capability: admissionv0.CapabilityHint, prompt: kinds[CallWords].prompt}}
	for _, c := range []Call{CallSheet, CallWorkbook, CallWords} {
		out = append(out, kinds[c])
	}
	return out
}

// CheckKey checks an own key when its owner connects it (R84 spec): api checks it through tutor and
// never calls a provider round it. The route is the own key's, with the profile's age category. The
// key is checked for every model the library would call with it — the models each kind of call
// chooses (ChooseCall), and those of the hint without the work, both readers of a pair —, each by
// a request of the provider without any learner's content that costs nothing (provider.Model.Check).
// When none is open, the provider is closed to an own key at the profile, and no call goes. One wrong key is the end, then one model
// without rights, then a provider that did not answer. A refusal for a limit means the provider took
// the key, and the limit shows on the calls. The error says what failed, for the logs, and never
// holds the key.
func (t *Tutor) CheckKey(ctx context.Context, r Route) (KeyCheck, error) {
	if r.AI != admissionv0.AiRouteOwnKey {
		return KeyUnanswered, fmt.Errorf("tutor: the check of a key goes on the route own_key, not %q", r.AI)
	}
	if err := r.check(); err != nil {
		return KeyUnanswered, err
	}
	var models []admission.Model
	for _, c := range keyChoices() {
		e, a, err := t.choose(c.capability, r, c.prompt)
		if err != nil {
			return KeyUnanswered, err
		}
		if !a.Admitted {
			continue
		}
		for _, m := range e.Models {
			if !slices.Contains(models, m) {
				models = append(models, m)
			}
		}
	}
	if len(models) == 0 {
		return KeyProviderClosed, nil
	}
	end, errs := KeyAccepted, []error{}
	for _, m := range models {
		k, err := t.checkModel(ctx, m, r.Own)
		if keyRank(k) > keyRank(end) {
			end = k
		}
		if err != nil {
			errs = append(errs, err)
		}
	}
	if end == KeyAccepted {
		return end, nil
	}
	return end, errors.Join(errs...)
}

// checkModel checks the key for one model. A panic of the adapter or its SDK fails the check
// rather than the process of api, as in ask.
func (t *Tutor) checkModel(ctx context.Context, m admission.Model, k *OwnKey) (end KeyCheck, err error) {
	defer func() {
		if p := recover(); p != nil {
			end, err = KeyUnanswered, fmt.Errorf("tutor: %s of %s: the adapter panicked: %v", m.Name(), k, p)
		}
	}()
	client, err := t.dial(m, k.value())
	if err != nil {
		return KeyUnanswered, err
	}
	err = client.Check(ctx)
	var le *provider.LimitError
	switch {
	case err == nil, errors.As(err, &le):
		return KeyAccepted, nil
	case errors.Is(err, provider.ErrKeyRefused):
		return KeyWrong, fmt.Errorf("tutor: %s of %s: %w", m.Name(), k, err)
	case errors.Is(err, provider.ErrNoRights):
		return KeyNoRights, fmt.Errorf("tutor: %s of %s: %w", m.Name(), k, err)
	}
	return KeyUnanswered, fmt.Errorf("tutor: %s of %s: %w", m.Name(), k, err)
}

// keyRank orders the ends of the checks of the models, the one a check of the key takes last.
func keyRank(k KeyCheck) int {
	switch k {
	case KeyWrong:
		return 3
	case KeyNoRights:
		return 2
	case KeyUnanswered:
		return 1
	}
	return 0
}
