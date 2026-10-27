package value_test

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"golang.org/x/text/unicode/norm"
	"gopkg.in/yaml.v3"

	"github.com/altnovatrail/tutor/internal/value"
)

// The test vectors of verdicts of the kind number are copies of format/vectors/verdicts/number of
// spec at v0.2.43 (commit 8cc3536), where they last changed in b217758, under Apache-2.0 as spec
// is. CI checks the repository out without the spec submodule, so the copies are what it tests;
// TestCopies holds them to spec's wherever the submodule is there.
const vectors = "testdata/verdicts/number"

// vector is a vector file: an item whose answer slots hold checks, as answer_slots of a sheet or as
// the boxes of a workbook's line, and the readings of it with their verdicts.
type vector struct {
	Item struct {
		AnswerSlots []slotOfItem `yaml:"answer_slots"`
		Line        []slotOfItem `yaml:"line"`
	} `yaml:"item"`
	Readings []struct {
		AnswerSlots []struct {
			Value   *string `yaml:"value"`
			Verdict string  `yaml:"verdict"`
			Reason  string  `yaml:"reason"`
			Trap    string  `yaml:"trap"`
		} `yaml:"answer_slots"`
		Verdict string `yaml:"verdict"`
	} `yaml:"readings"`
}

type slotOfItem struct {
	Binding string `yaml:"binding"`
	Check   struct {
		Kind       string   `yaml:"kind"`
		Key        string   `yaml:"key"`
		Units      []string `yaml:"units"`
		TrapValues []struct {
			Value string `yaml:"value"`
			Trap  string `yaml:"trap"`
		} `yaml:"trap_values"`
	} `yaml:"check"`
}

// checks are the checks of the item's answer slots: on a sheet its answer_slots, on a workbook the
// boxes of its line, the parts of the line that have a binding.
func (v *vector) checks(t *testing.T) []value.Number {
	t.Helper()
	slots := v.Item.AnswerSlots
	if slots == nil {
		for _, part := range v.Item.Line {
			if part.Binding != "" {
				slots = append(slots, part)
			}
		}
	}
	var out []value.Number
	for _, s := range slots {
		if s.Check.Kind != "number" {
			t.Fatalf("a check of the kind %q", s.Check.Kind)
		}
		n := value.Number{Key: s.Check.Key, Units: s.Check.Units}
		for _, tv := range s.Check.TrapValues {
			n.Traps = append(n.Traps, value.Trap{Value: tv.Value, Trap: tv.Trap})
		}
		out = append(out, n)
	}
	return out
}

// Every reading of every vector of the kind number gets its verdicts: of each answer slot, with
// its reason and its trap, and of the item.
func TestVectors(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join(vectors, "*.yaml"))
	if err != nil || len(paths) != 7 {
		t.Fatalf("the vectors are %v, %v", paths, err)
	}
	readings, slots := 0, 0
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var v vector
		if err := yaml.Unmarshal(data, &v); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		checks := v.checks(t)
		for i, r := range v.Readings {
			readings++
			if len(r.AnswerSlots) != len(checks) {
				t.Fatalf("%s, reading %d: %d values for %d answer slots", path, i+1, len(r.AnswerSlots), len(checks))
			}
			var verdicts []value.Verdict
			for k, want := range r.AnswerSlots {
				slots++
				got := checks[k].Check(want.Value)
				if string(got.Verdict) != want.Verdict || string(got.Reason) != want.Reason || got.Trap != want.Trap {
					t.Errorf("%s, reading %d, slot %d, %s: got %+v, want %s %s %s", path, i+1, k+1, show(want.Value), got, want.Verdict, want.Reason, want.Trap)
				}
				verdicts = append(verdicts, got.Verdict)
			}
			if got := value.Item(verdicts); string(got) != r.Verdict {
				t.Errorf("%s, reading %d: the item is %s, want %s", path, i+1, got, r.Verdict)
			}
		}
	}
	if readings != 53 || slots != 62 {
		t.Errorf("the vectors hold %d readings and %d answer slots, want 53 and 62", readings, slots)
	}
}

func show(v *string) string {
	if v == nil {
		return "null"
	}
	return "«" + *v + "»"
}

// The copies are spec's vectors as they are, file for file, where the spec submodule is checked
// out; in CI, which has no submodule, the test is skipped.
func TestCopies(t *testing.T) {
	spec := filepath.Join("..", "..", "spec", "format", "vectors", "verdicts", "number")
	theirs, err := os.ReadDir(spec)
	if err != nil {
		t.Skipf("no spec submodule: %v", err)
	}
	ours, err := os.ReadDir(vectors)
	if err != nil {
		t.Fatal(err)
	}
	names := func(entries []os.DirEntry) []string {
		var out []string
		for _, e := range entries {
			out = append(out, e.Name())
		}
		return out
	}
	if !slices.Equal(names(ours), names(theirs)) {
		t.Fatalf("the copies are %v, and spec has %v", names(ours), names(theirs))
	}
	for _, name := range names(ours) {
		a, _ := os.ReadFile(filepath.Join(vectors, name))
		b, _ := os.ReadFile(filepath.Join(spec, name))
		if !bytes.Equal(a, b) {
			t.Errorf("%s is not spec's: copy it again, and say which tag in the comment of the vectors", name)
		}
	}
}

// Nothing read is blank; a value of more than 16 characters is not in the notation, even when it is
// a number; a key out of the notation checks nothing; a trap names itself only on a value not yet
// right.
func TestCheck(t *testing.T) {
	s := func(v string) *string { return &v }
	n := value.Number{Key: "11", Traps: []value.Trap{{Value: "12", Trap: "carry"}, {Value: "11", Trap: "never"}}}
	for _, c := range []struct {
		value *string
		want  value.Slot
	}{
		{nil, value.Slot{Verdict: value.Unreadable, Reason: value.Blank}},
		{s(" = . "), value.Slot{Verdict: value.Unreadable, Reason: value.Blank}},
		{s("11"), value.Slot{Verdict: value.Correct}},
		{s("12"), value.Slot{Verdict: value.NotYet, Trap: "carry"}},
		{s("13"), value.Slot{Verdict: value.NotYet}},
		{s("12345678901234567"), value.Slot{Verdict: value.Unreadable, Reason: value.Notation}},
		{s("1½"), value.Slot{Verdict: value.Unreadable, Reason: value.Notation}},
		{s("x = 11"), value.Slot{Verdict: value.Unreadable, Reason: value.Notation}},
	} {
		if got := n.Check(c.value); got != c.want {
			t.Errorf("%s: %+v, want %+v", show(c.value), got, c.want)
		}
	}
	if got := (value.Number{Key: "x = 2"}).Check(s("2")); got.Verdict != value.Unchecked {
		t.Errorf("a key out of the notation gives %+v", got)
	}
	if got := value.Item(nil); got != value.Unchecked {
		t.Errorf("an item without answer slots is %s", got)
	}
}

// Normalize is steps 1 to 4: what the reading compares the texts of parts by.
func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{
		"  47\u00a0+\t19 \u2212 2 ": "47 + 19 - 2",
		"x = 13\u20442":             "x = 13/2",
		"\uff11\uff12 cm\u00b2":     "12 cm\ufffd",
		"\u00bd":                    "1/2",
	} {
		if got := value.Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

// The tables of normalization are of a version of Unicode from 15.0 on, as the vectors need.
func TestUnicode(t *testing.T) {
	major, _, _ := strings.Cut(norm.Version, ".")
	if len(major) < 2 || major < "15" {
		t.Errorf("the tables of NFKC are of Unicode %s", norm.Version)
	}
}
