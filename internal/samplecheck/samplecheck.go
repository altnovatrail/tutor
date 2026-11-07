// Package samplecheck holds sample files of the eval protocol to the rules of sections 11 and 12 of
// spec's text of the contracts by spec's own code, its recipe sample-check, as section 13 has the
// runner refuse a sample "which the runner holds by the same code as sample-check": a repository
// other than spec MUST NOT write a check of its own (R93 spec; R69). The recipe runs from the
// repository's root, with the spec submodule there, in the toolchain of the devcontainer: Node and
// buf.
package samplecheck

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Check holds the files, by their paths from the working directory or absolute within it, to the rules, and gives the
// lines of each file that breaks one, ascending: the places sample-check names, and no message, which
// could quote a part of a sheet. A file it gives none of keeps every rule. The error is a check that
// could not run: no spec submodule, no just, a path it refuses, or buf failing.
type Check func(files []string) (map[string][]int, error)

// Justfile is spec's justfile from the repository's root, where its recipe sample-check is.
const Justfile = "spec/justfile"

// place is a line of sample-check: a file, its line and the rule it breaks.
var place = regexp.MustCompile(`^(.+):([0-9]+): `)

// Run is Check by spec's recipe: just -f spec/justfile sample-check with the files, from the working
// directory, which is to be the repository's root. An exit status of 1 names the files that break a
// rule; any other but 0 is a check that could not run.
func Run(files []string) (map[string][]int, error) {
	if len(files) == 0 {
		return map[string][]int{}, nil
	}
	if _, err := os.Stat(Justfile); err != nil {
		return nil, fmt.Errorf("samplecheck: %s is not here: run from the repository's root, with the spec submodule checked out (git submodule update --init)", Justfile)
	}
	// sample-check takes the repository's paths, from its root: an absolute path within it is given
	// as the path from the root, and its lines come back under the name it was given by.
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	given := map[string]string{}
	rel := make([]string, len(files))
	for i, f := range files {
		rel[i] = f
		if filepath.IsAbs(f) {
			r, err := filepath.Rel(wd, f)
			if err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
				return nil, fmt.Errorf("samplecheck: %s: sample-check takes the repository's paths, and this one is outside %s", f, wd)
			}
			rel[i] = r
		}
		given[rel[i]] = f
	}
	just, err := exec.LookPath("just")
	if err != nil {
		return nil, fmt.Errorf("samplecheck: just is not on the PATH: %w", err)
	}
	// Every argument after the recipe's name is a file, a name that begins with - too.
	args := append([]string{"-f", Justfile, "sample-check"}, rel...)
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(just, args...)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err = cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return map[string][]int{}, nil
	case errors.As(err, &exit) && exit.ExitCode() == 1:
		broken := map[string][]int{}
		for f, lines := range Parse(stdout.String(), rel) {
			broken[given[f]] = lines
		}
		if len(broken) == 0 {
			return nil, fmt.Errorf("samplecheck: sample-check failed and named no file: %s", lastLine(stdout.String(), stderr.String()))
		}
		return broken, nil
	}
	return nil, fmt.Errorf("samplecheck: sample-check could not check the files: %s", lastLine(stdout.String(), stderr.String()))
}

// Parse reads what sample-check printed: the lines of each of the files that it names as breaking a
// rule, ascending and each once. A line naming another file is not read.
func Parse(out string, files []string) map[string][]int {
	named := map[string]bool{}
	for _, f := range files {
		named[f] = true
	}
	broken := map[string][]int{}
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		m := place.FindStringSubmatch(sc.Text())
		if m == nil || !named[m[1]] {
			continue
		}
		n, err := strconv.Atoi(m[2])
		if err != nil {
			continue
		}
		broken[m[1]] = append(broken[m[1]], n)
	}
	for f, lines := range broken {
		slices.Sort(lines)
		broken[f] = slices.Compact(lines)
	}
	return broken
}

// lastLine is the last line printed, of the error output first: what the check said as it stopped.
func lastLine(stdout, stderr string) string {
	for _, out := range []string{stderr, stdout} {
		if t := strings.TrimSpace(out); t != "" {
			return t[strings.LastIndex(t, "\n")+1:]
		}
	}
	return "it said nothing"
}
