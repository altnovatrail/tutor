package samplecheck_test

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/internal/samplecheck"
)

// What sample-check prints is read by the files named: the lines of each, ascending and each once,
// and nothing of a line that names another file or no place.
func TestParse(t *testing.T) {
	out := strings.Join([]string{
		"sample-check: 4 errors",
		"a/x.json:380: the stroke 1 is in 0 parts",
		"a/x.json:21: the writer is a child",
		"a/x.json:380: the stroke 2 is in 0 parts",
		"b.json:7: rule",
		"other.json:3: not named",
		"a/x.json:no: not a line",
	}, "\n")
	got := samplecheck.Parse(out, []string{"a/x.json", "b.json", "c.json"})
	if !slices.Equal(got["a/x.json"], []int{21, 380}) || !slices.Equal(got["b.json"], []int{7}) || len(got) != 2 {
		t.Errorf("%v", got)
	}
}

// fakeJust puts on the PATH a just that prints what it is told and exits with the status given,
// and runs the test from a root with spec's justfile in it.
func fakeJust(t *testing.T, stdout, stderr string, status int) {
	t.Helper()
	bin := t.TempDir()
	script := "#!/bin/sh\nprintf '%s' \"$OUT\"\nprintf '%s' \"$ERR\" >&2\nexit $STATUS\n"
	if err := os.WriteFile(filepath.Join(bin, "just"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("OUT", stdout)
	t.Setenv("ERR", stderr)
	t.Setenv("STATUS", string(rune('0'+status)))
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "spec"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, samplecheck.Justfile), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
}

// Run passes the files sample-check passes, names the lines of those it finds breaking a rule, and
// is an error when the check cannot run or says nothing of the files it fails.
func TestRun(t *testing.T) {
	files := []string{"a.json", "b.json"}
	fakeJust(t, "sample-check: ok — 2 samples\n", "", 0)
	if got, err := samplecheck.Run(files); err != nil || len(got) != 0 {
		t.Errorf("ok: %v %v", got, err)
	}
	if got, err := samplecheck.Run(nil); err != nil || len(got) != 0 {
		t.Errorf("no files: %v %v", got, err)
	}

	fakeJust(t, "b.json:12: the part 1 has no stroke\nsample-check: 1 error\n", "cd … && node …\n", 1)
	if got, err := samplecheck.Run(files); err != nil || !slices.Equal(slices.Sorted(maps.Keys(got)), []string{"b.json"}) || !slices.Equal(got["b.json"], []int{12}) {
		t.Errorf("a rule broken: %v %v", got, err)
	}

	fakeJust(t, "sample-check: 1 error\n", "", 1)
	if _, err := samplecheck.Run(files); err == nil || !strings.Contains(err.Error(), "named no file") {
		t.Errorf("a failure that names no file: %v", err)
	}

	fakeJust(t, "", "cd … && node …\nsample-check: buf build fails\n", 2)
	if _, err := samplecheck.Run(files); err == nil || !strings.Contains(err.Error(), "buf build fails") {
		t.Errorf("a check that cannot run: %v", err)
	}

	fakeJust(t, "", "", 2)
	if _, err := samplecheck.Run(files); err == nil || !strings.Contains(err.Error(), "it said nothing") {
		t.Errorf("a check that says nothing: %v", err)
	}

	if _, err := samplecheck.Run([]string{filepath.Join(t.TempDir(), "a.json")}); err == nil || !strings.Contains(err.Error(), "is outside") {
		t.Errorf("an absolute path outside the root: %v", err)
	}
	// An absolute path within the root goes to the check from the root, and comes back as given.
	fakeJust(t, "b.json:12: the part 1 has no stroke\n", "", 1)
	wd, _ := os.Getwd()
	inside := filepath.Join(wd, "b.json")
	if got, err := samplecheck.Run([]string{"a.json", inside}); err != nil || !slices.Equal(got[inside], []int{12}) || len(got) != 1 {
		t.Errorf("an absolute path within the root: %v %v", got, err)
	}

	t.Setenv("PATH", t.TempDir())
	if _, err := samplecheck.Run(files); err == nil || !strings.Contains(err.Error(), "just is not on the PATH") {
		t.Errorf("no just: %v", err)
	}

	t.Chdir(t.TempDir())
	if _, err := samplecheck.Run(files); err == nil || !strings.Contains(err.Error(), "git submodule update --init") {
		t.Errorf("no spec: %v", err)
	}
}
