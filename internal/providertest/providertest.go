// Package providertest is what the tests of the adapters share: a transport that answers every
// request itself and keeps what reached the network, and the check that the requests went where a
// client that takes nothing from the environment sends them (R105).
package providertest

import (
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
)

// Sent is a transport that keeps each request — its address and headers — and answers it with
// what Answer gives for it: the body and its content type. A request to whatever address is seen,
// and none leaves the test.
type Sent struct {
	Answer func(*http.Request) (body, contentType string)

	mu       sync.Mutex
	requests []*http.Request
}

// RoundTrip keeps the request and answers it.
func (s *Sent) RoundTrip(r *http.Request) (*http.Response, error) {
	s.mu.Lock()
	s.requests = append(s.requests, r)
	s.mu.Unlock()
	body, kind := s.Answer(r)
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {kind}},
		Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
}

// Want is what the requests of a test must be: their addresses, in order, the header that carries
// the key with the key's value, and the headers none of them may have.
type Want struct {
	URLs      []string
	KeyHeader string
	Key       string
	Absent    []string
}

// Check holds the requests sent so far to w.
func (s *Sent) Check(t testing.TB, w Want) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.requests) != len(w.URLs) {
		t.Errorf("%d requests, want %d", len(s.requests), len(w.URLs))
		return
	}
	for i, r := range s.requests {
		if got := r.URL.Scheme + "://" + r.URL.Host + r.URL.Path; got != w.URLs[i] {
			t.Errorf("the request went to %s, want %s", got, w.URLs[i])
		}
		if got := r.Header.Get(w.KeyHeader); got != w.Key {
			t.Errorf("the request has %s: %q, want %q", w.KeyHeader, got, w.Key)
		}
		for _, h := range w.Absent {
			if v := r.Header.Get(h); v != "" {
				t.Errorf("the request has %s: %s", h, v)
			}
		}
	}
}
