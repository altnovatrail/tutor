package provider

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// Limit is why a provider refused a call for a limit: one only the owner of the key can lift, each
// in its own way (docs/architecture.md, 3.7).
type Limit string

// The limits a provider refuses for.
const (
	// LimitOwner is a spend limit the owner set: of the organization, or of a workspace or project.
	LimitOwner Limit = "owner_limit"
	// LimitTier is the provider's own cap of the usage tier.
	LimitTier Limit = "tier"
	// LimitBalance is a prepaid balance spent.
	LimitBalance Limit = "balance"
	// LimitBilling is a fault of billing or payment.
	LimitBilling Limit = "billing"
)

// LimitError is a provider's refusal for a limit, with the provider's error that said it. Sending
// the call again does not help until the owner of the key lifts the limit, so the adapter does not
// let its SDK send it again.
type LimitError struct {
	Limit Limit
	Err   error
}

func (e *LimitError) Error() string {
	return fmt.Sprintf("the provider refused for a limit (%s): %v", e.Limit, e.Err)
}
func (e *LimitError) Unwrap() error { return e.Err }

// ErrKeyRefused is a provider's refusal of the key itself: it is wrong, revoked or expired (R84
// spec). Sending the call again does not help either.
var ErrKeyRefused = errors.New("the provider refused the key")

// Refusal is what an adapter reads from a provider's error response: the limit it refused for, or
// whether it refused the key, or neither.
type Refusal struct {
	Limit Limit
	Key   bool
}

// Of is the refusal as the error of a call that failed with err, or err itself when the response
// was no refusal.
func (r Refusal) Of(err error) error {
	switch {
	case r.Limit != "":
		return &LimitError{Limit: r.Limit, Err: err}
	case r.Key:
		return fmt.Errorf("%w: %w", ErrKeyRefused, err)
	}
	return err
}

// errorBytes is the most of an error response's body NoRetry reads to classify it: an error of
// either provider is a small JSON object, and the rest of a longer body is left to the SDK.
const errorBytes = 1 << 16

// NoRetry is the middleware of an adapter's SDK that reads each error response of the provider by
// classify, keeps the last refusal it found in *last, and marks a refusal with the header
// x-should-retry: false, which both SDKs heed (internal/requestconfig of each): a refusal for a
// limit or of the key is not sent again, while a plain 429, 408, 409 or 5xx still is. It reads the
// start of the body of an error response alone, and gives the SDK the body as it came, the error
// of a read that broke off too, so that the SDK handles it as its own.
func NoRetry(classify func(status int, body []byte) Refusal, last *Refusal) func(*http.Request, func(*http.Request) (*http.Response, error)) (*http.Response, error) {
	return func(req *http.Request, next func(*http.Request) (*http.Response, error)) (*http.Response, error) {
		resp, err := next(req)
		if err != nil || resp == nil || resp.StatusCode < 400 || resp.Body == nil {
			return resp, err
		}
		body, rerr := io.ReadAll(io.LimitReader(resp.Body, errorBytes))
		rest := resp.Body
		if rerr != nil {
			rest = io.NopCloser(failing{rerr})
			_ = resp.Body.Close()
		}
		resp.Body = struct {
			io.Reader
			io.Closer
		}{io.MultiReader(bytes.NewReader(body), rest), resp.Body}
		if rerr != nil {
			return resp, err
		}
		if r := classify(resp.StatusCode, body); r != (Refusal{}) {
			*last = r
			if resp.Header == nil {
				resp.Header = http.Header{}
			}
			resp.Header.Set("x-should-retry", "false")
		}
		return resp, err
	}
}

// failing is a reader that fails with its error.
type failing struct{ err error }

func (f failing) Read([]byte) (int, error) { return 0, f.err }
