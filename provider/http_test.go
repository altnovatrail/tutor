package provider

import (
	"net/http"
	"testing"
)

func TestHTTPClient(t *testing.T) {
	// A transport the process put in place of the default is not taken.
	saved := http.DefaultTransport
	http.DefaultTransport = roundTrip(func(r *http.Request) (*http.Response, error) { return nil, http.ErrNotSupported })
	t.Cleanup(func() { http.DefaultTransport = saved })

	c := HTTPClient()
	tr, ok := c.Transport.(*http.Transport)
	if !ok || tr.ResponseHeaderTimeout != ResponseHeaderTimeout || tr.Proxy == nil {
		t.Errorf("the transport is %#v, want the adapters' own, with the timeout and the proxy", c.Transport)
	}
	if HTTPClient().Transport != c.Transport {
		t.Error("the clients do not share one transport")
	}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
