package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

// failingWriter refuses every write, as a closed pipe would.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("closed") }

func TestRunUnwritableVersion(t *testing.T) {
	if got := run([]string{"-version"}, failingWriter{}, io.Discard); got != 1 {
		t.Errorf("run with an unwritable stdout = %d, want 1", got)
	}
}

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string // a prefix of stdout
		wantStderr string // a part of stderr
	}{
		{name: "version", args: []string{"-version"}, wantCode: 0, wantStdout: "tutor-eval "},
		{name: "help", args: []string{"-h"}, wantCode: 0, wantStderr: "-version"},
		{name: "no runs yet", args: nil, wantCode: 2, wantStderr: "no runs are defined yet"},
		{name: "unknown flag", args: []string{"-nope"}, wantCode: 2, wantStderr: "-nope"},
		{name: "stray argument", args: []string{"-version", "extra"}, wantCode: 2, wantStderr: "unexpected arguments"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := run(tt.args, &stdout, &stderr); got != tt.wantCode {
				t.Errorf("run(%q) = %d, want %d", tt.args, got, tt.wantCode)
			}
			if !strings.HasPrefix(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout = %q, want the prefix %q", stdout.String(), tt.wantStdout)
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}
