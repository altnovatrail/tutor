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

// The test vectors of verdicts of the kinds number and fraction are copies of
// format/vectors/verdicts/number and format/vectors/verdicts/fraction of spec at v0.3.2 (commit
// 9d64b22), where they last changed in b217758, under Apache-2.0 as spec is. CI checks the
// repository out without the spec submodule, so the copies are what it tests; TestCopies holds
// them to spec's wherever the submodule is there.
const vectors = "testdata/verdicts"

// kinds are the check kinds whose vectors are copied, each a folder of vectors.
var kinds = []string{"fraction", "number"}

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
		Form       string   `yaml:"form"`
		TrapValues []struct {
			Value string `yaml:"value"`
			Trap  string `yaml:"trap"`
		} `yaml:"trap_values"`
	} `yaml:"check"`
}

// checker is a check of a kind: what it gives a value read.
type checker interface {
	Check(value *string) value.Slot
}

// checks are the checks of the item's answer slots: on a sheet its answer_slots, on a workbook the
// boxes of its line, the parts of the line that have a binding.
func (v *vector) checks(t *testing.T) []checker {
	t.Helper()
	slots := v.Item.AnswerSlots
	if slots == nil {
		for _, part := range v.Item.Line {
			if part.Binding != "" {
				slots = append(slots, part)
			}
		}
	}
	var out []checker
	for _, s := range slots {
		var traps []value.Trap
		for _, tv := range s.Check.TrapValues {
			traps = append(traps, value.Trap{Value: tv.Value, Trap: tv.Trap})
		}
		switch s.Check.Kind {
		case "number":
			out = append(out, value.Number{Key: s.Check.Key, Units: s.Check.Units, Traps: traps})
		case "fraction":
			out = append(out, value.Fraction{Key: s.Check.Key, Form: value.Form(s.Check.Form), Traps: traps})
		default:
			t.Fatalf("a check of the kind %q", s.Check.Kind)
		}
	}
	return out
}

// Every reading of every vector of the kinds number and fraction gets its verdicts: of each answer
// slot, with its reason and its trap, and of the item.
func TestVectors(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join(vectors, "*", "*.yaml"))
	if err != nil || len(paths) != 9 {
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
	if readings != 65 || slots != 74 {
		t.Errorf("the vectors hold %d readings and %d answer slots, want 65 and 74", readings, slots)
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
	for _, kind := range kinds {
		t.Run(kind, func(t *testing.T) { sameAsSpec(t, kind) })
	}
}

func sameAsSpec(t *testing.T, kind string) {
	spec := filepath.Join("..", "..", "spec", "format", "vectors", "verdicts", kind)
	theirs, err := os.ReadDir(spec)
	if err != nil {
		t.Skipf("no spec submodule: %v", err)
	}
	ours, err := os.ReadDir(filepath.Join(vectors, kind))
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
		a, _ := os.ReadFile(filepath.Join(vectors, kind, name))
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

// A fraction asked in lowest terms is right only in them, and a trap of its value matches every
// other fraction of it; a key that is no simple fraction, or not in lowest terms where they are
// asked, checks nothing; a whole number and a decimal are not in the notation of a fraction.
func TestFraction(t *testing.T) {
	s := func(v string) *string { return &v }
	f := value.Fraction{Key: "2/3", Form: value.Reduced, Traps: []value.Trap{{Value: "4/6", Trap: "not-in-lowest-terms"}, {Value: "2/9", Trap: "adds-denominators"}}}
	for _, c := range []struct {
		value *string
		want  value.Slot
	}{
		{nil, value.Slot{Verdict: value.Unreadable, Reason: value.Blank}},
		{s("2/3"), value.Slot{Verdict: value.Correct}},
		{s(" 2 / 3 "), value.Slot{Verdict: value.Correct}},
		{s("6/9"), value.Slot{Verdict: value.NotYet, Trap: "not-in-lowest-terms"}},
		{s("2/9"), value.Slot{Verdict: value.NotYet, Trap: "adds-denominators"}},
		{s("1/3"), value.Slot{Verdict: value.NotYet}},
		{s("2"), value.Slot{Verdict: value.Unreadable, Reason: value.Notation}},
		{s("0.5"), value.Slot{Verdict: value.Unreadable, Reason: value.Notation}},
		{s("1 1/2"), value.Slot{Verdict: value.Unreadable, Reason: value.Notation}},
	} {
		if got := f.Check(c.value); got != c.want {
			t.Errorf("%s: %+v, want %+v", show(c.value), got, c.want)
		}
	}
	if got := (value.Fraction{Key: "4/6"}).Check(s("2/3")); got.Verdict != value.Correct {
		t.Errorf("any form: 2/3 against 4/6 gives %+v", got)
	}
	if got := (value.Fraction{Key: "3/4", Form: "lowest"}).Check(s("6/8")); got.Verdict != value.Unchecked {
		t.Errorf("a form of no name gives %+v", got)
	}
	for _, key := range []string{"4/6", "0.5", "x = 2/3"} {
		if got := (value.Fraction{Key: key, Form: value.Reduced}).Check(s("2/3")); got.Verdict != value.Unchecked {
			t.Errorf("the key %s in lowest terms gives %+v", key, got)
		}
	}
}
