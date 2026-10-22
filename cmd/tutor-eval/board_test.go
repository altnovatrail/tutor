package main

import (
	"bytes"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestBoardCommand(t *testing.T) {
	keys := map[string]string{anthropicKeyVar: "a-key", openAIKeyVar: "o-key"}
	tests := []struct {
		name       string
		args       []string
		env        map[string]string
		serveErr   error
		wantCode   int
		wantServed bool
		wantOut    string
	}{
		{"both keys", nil, keys, nil, exitOK, true, "http://127.0.0.1:8080"},
		{"one key", []string{"-addr", "localhost:9000"}, map[string]string{openAIKeyVar: "o-key"}, nil, exitOK, true, "Claude is left out"},
		{"no keys", nil, nil, nil, exitUsage, false, "no model to ask"},
		{"a public address", []string{"-addr", "0.0.0.0:8080"}, keys, nil, exitUsage, false, "not a local address"},
		{"a host name", []string{"-addr", "example.com:8080"}, keys, nil, exitUsage, false, "not a local address"},
		{"no cap", []string{"-max-usd", "0"}, keys, nil, exitUsage, false, "-max-usd must be above 0"},
		{"a stray argument", []string{"extra"}, keys, nil, exitUsage, false, "unexpected arguments"},
		{"the port is taken", nil, keys, errors.New("address already in use"), exitFail, true, "address already in use"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			served := false
			serve := func(s *http.Server) error {
				served = true
				if s.Handler == nil {
					t.Error("the server has no handler")
				}
				return tt.serveErr
			}
			args := append([]string{"-dir", t.TempDir()}, tt.args...)
			got := runBoard(args, &stdout, &stderr, func(k string) string { return tt.env[k] }, serve)
			if got != tt.wantCode || served != tt.wantServed {
				t.Errorf("code %d, served %v; want %d, %v (stderr %q)", got, served, tt.wantCode, tt.wantServed, stderr.String())
			}
			if out := stdout.String() + stderr.String(); !strings.Contains(out, tt.wantOut) {
				t.Errorf("output %q, want it to hold %q", out, tt.wantOut)
			}
		})
	}
}

func TestCheckIgnored(t *testing.T) {
	// The tests run in cmd/tutor-eval, two levels below the root of tutor, whose .gitignore ignores
	// the samples folder at the root only.
	if err := checkIgnored("../../samples/board"); err != nil {
		t.Errorf("the samples folder at the root: %v, want it taken", err)
	}
	if err := checkIgnored("samples/board"); err == nil || !strings.Contains(err.Error(), "outside its samples folder") {
		t.Errorf("a samples folder below the root = %v, want it refused", err)
	}
	if err := checkIgnored(t.TempDir()); err != nil {
		t.Errorf("a folder outside any work tree: %v, want it taken", err)
	}
	// A folder whose name only starts like the samples folder is not in it.
	if err := checkIgnored("../../samples-copy/board"); err == nil {
		t.Error("a folder beside the samples folder was taken")
	}
	if err := checkIgnored("../../samples"); err != nil {
		t.Errorf("the samples folder itself: %v, want it taken", err)
	}
}
