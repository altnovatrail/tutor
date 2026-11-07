package value_test

import (
	"strings"
	"testing"

	"golang.org/x/text/unicode/norm"

	"github.com/altnovatrail/tutor/internal/value"
)

// Read is steps 1 to 5 and 7: a "=" before the value and the marks after it go, and nothing left is
// blank.
func TestRead(t *testing.T) {
	for in, want := range map[string]string{"= 13.": "13", " 7⁄ 2 ": "7/ 2", "−12!?": "-12"} {
		if got, ok := value.Read(in); !ok || got != want {
			t.Errorf("Read(%q) = %q, %v, want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"", " = . ", "?!"} {
		if got, ok := value.Read(in); ok {
			t.Errorf("Read(%q) = %q, not blank", in, got)
		}
	}
}

// Rational takes an integer, a decimal with a point and a simple fraction, exactly, of 16 characters
// at most, and no other text.
func TestRational(t *testing.T) {
	for in, want := range map[string]string{"12": "12/1", "-0.25": "-1/4", "6 / 4": "3/2"} {
		if r, ok := value.Rational(in); !ok || r.RatString() != want && r.String() != want {
			t.Errorf("Rational(%q) = %v, %v, want %s", in, r, ok, want)
		}
	}
	for _, in := range []string{"1,5", "01", "1/0", "x = 2", "12345678901234567", ".5"} {
		if r, ok := value.Rational(in); ok {
			t.Errorf("Rational(%q) = %v", in, r)
		}
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
