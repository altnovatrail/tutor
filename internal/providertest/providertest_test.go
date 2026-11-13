package providertest

import (
	"fmt"
	"io"
	"net/http"
	"testing"
)

// failures keeps what Check reports, so that a test can hold a check that fails.
type failures struct {
	testing.TB
	got []string
}

func (f *failures) Helper() {}

func (f *failures) Errorf(format string, args ...any) {
	f.got = append(f.got, fmt.Sprintf(format, args...))
}

func TestSent(t *testing.T) {
	s := &Sent{Answer: func(r *http.Request) (string, string) { return "answer of " + r.URL.Path, "text/plain" }}
	c := &http.Client{Transport: s}
	req, _ := http.NewRequest(http.MethodGet, "https://api.example/v1/a", nil)
	req.Header.Set("X-Key", "k")
	req.Header.Set("X-Env", "1")
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "answer of /v1/a" || resp.Header.Get("Content-Type") != "text/plain" {
		t.Errorf("the answer is %q, %q", body, resp.Header.Get("Content-Type"))
	}

	s.Check(t, Want{URLs: []string{"https://api.example/v1/a"}, KeyHeader: "X-Key", Key: "k", Absent: []string{"X-Other"}})

	for _, tc := range []struct {
		name string
		want Want
		n    int
	}{
		{"count", Want{URLs: []string{"a", "b"}}, 1},
		{"address, key and header", Want{URLs: []string{"https://elsewhere/v1/a"}, KeyHeader: "X-Key", Key: "other", Absent: []string{"X-Env"}}, 3},
	} {
		f := &failures{TB: t}
		s.Check(f, tc.want)
		if len(f.got) != tc.n {
			t.Errorf("%s: %d failures, want %d: %q", tc.name, len(f.got), tc.n, f.got)
		}
	}
}
