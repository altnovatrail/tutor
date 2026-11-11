package provider_test

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/provider"
)

// byStatus classifies a 402 as billing and a 401 as the key, whatever the body says.
func byStatus(status int, _ []byte) provider.Refusal {
	switch status {
	case http.StatusPaymentRequired:
		return provider.Refusal{Limit: provider.LimitBilling}
	case http.StatusUnauthorized:
		return provider.Refusal{Key: true}
	}
	return provider.Refusal{}
}

// passes runs the middleware on a response as the next step gives it.
func passes(t *testing.T, resp *http.Response, last *provider.Refusal) *http.Response {
	t.Helper()
	mw := provider.NoRetry(byStatus, last)
	got, err := mw(&http.Request{}, func(*http.Request) (*http.Response, error) { return resp, nil })
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// A refusal gets x-should-retry: false, and its body comes back to the SDK as it came; any other
// response keeps its headers, and a success is not read at all.
func TestNoRetry(t *testing.T) {
	for _, status := range []int{http.StatusPaymentRequired, http.StatusUnauthorized, http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusOK} {
		var last provider.Refusal
		body := `{"error":{"message":"` + strings.Repeat("x", 70000) + `"}}`
		got := passes(t, &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, &last)
		refusal := byStatus(status, nil) != provider.Refusal{}
		if (got.Header.Get("x-should-retry") == "false") != refusal || (last != provider.Refusal{}) != refusal {
			t.Errorf("%d: header %q, refusal %+v", status, got.Header.Get("x-should-retry"), last)
		}
		if b, err := io.ReadAll(got.Body); err != nil || string(b) != body {
			t.Errorf("%d: the body comes back as %d bytes, %v", status, len(b), err)
		}
		if err := got.Body.Close(); err != nil {
			t.Errorf("%d: close: %v", status, err)
		}
	}
	// A response with neither headers nor a body passes; one whose body breaks off keeps its error.
	var last provider.Refusal
	if got := passes(t, &http.Response{StatusCode: http.StatusUnauthorized, Body: io.NopCloser(strings.NewReader("{}"))}, &last); got.Header.Get("x-should-retry") != "false" {
		t.Error("a refusal without headers is not marked")
	}
	passes(t, &http.Response{StatusCode: http.StatusUnauthorized}, &last)
	broken := errors.New("the connection broke")
	got := passes(t, &http.Response{StatusCode: http.StatusPaymentRequired, Header: http.Header{}, Body: io.NopCloser(io.MultiReader(strings.NewReader(`{"err`), failing{broken}))}, &provider.Refusal{})
	if b, err := io.ReadAll(got.Body); !errors.Is(err, broken) || string(b) != `{"err` || got.Header.Get("x-should-retry") != "" {
		t.Errorf("a body that broke off: %q, %v, header %q", b, err, got.Header.Get("x-should-retry"))
	}
}

// A refusal wraps the provider's error into its own.
func TestRefusalOf(t *testing.T) {
	cause := errors.New("402")
	var le *provider.LimitError
	if err := (provider.Refusal{Limit: provider.LimitBilling}).Of(cause); !errors.As(err, &le) || le.Limit != provider.LimitBilling || !errors.Is(err, cause) || !strings.Contains(err.Error(), "billing") {
		t.Errorf("a limit: %v", err)
	}
	if err := (provider.Refusal{Key: true}).Of(cause); !errors.Is(err, provider.ErrKeyRefused) || !errors.Is(err, cause) {
		t.Errorf("a key: %v", err)
	}
	if err := (provider.Refusal{}).Of(cause); err == nil || err.Error() != cause.Error() || errors.Unwrap(err) != nil {
		t.Errorf("no refusal: %v", err)
	}
}

type failing struct{ err error }

func (f failing) Read([]byte) (int, error) { return 0, f.err }
