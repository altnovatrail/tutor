// Package value is the check of the kind number of the module format (R56 spec; the format text of
// spec, section 15, «Checks»): it reads a value as it is written in the steps the format gives,
// takes it only in a notation of the kind, and compares it with the key as a rational number,
// exactly. A value equal to a trap value names the trap. The reading's verdict of a sheet comes to
// this check once the observation has a final answer to judge (T68; docs/architecture.md, 3.4).
//
// The normalization follows Unicode 17.0.0, the version of the tables of Go 1.27 and of
// golang.org/x/text v0.42.0 (norm.Version); the format's test vectors use only characters that
// every version from 15.0 on normalizes alike.
package value

import (
	"math/big"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Verdict is the verdict of an answer slot or of an item.
type Verdict string

// The verdicts of the format.
const (
	Correct    Verdict = "correct"
	NotYet     Verdict = "not_yet"
	Unreadable Verdict = "unreadable"
	Unchecked  Verdict = "unchecked"
)

// Reason is why a value is unreadable: nothing was read, or it is not in the kind's notation.
type Reason string

// The reasons of an unreadable value.
const (
	Blank    Reason = "blank"
	Notation Reason = "notation"
)

// Trap is a wrong value that the author knows, and the trap it shows.
type Trap struct {
	Value string
	Trap  string
}

// Number is a check of the kind number: the key, written in the notation, the units that may
// follow the number, and the trap values.
type Number struct {
	Key   string
	Units []string
	Traps []Trap
}

// Slot is the verdict of an answer slot: the verdict, why it is unreadable when it is, and the
// trap the value matches when it is not yet right and matches one.
type Slot struct {
	Verdict Verdict
	Reason  Reason
	Trap    string
}

// Check judges the value read in an answer slot, nil when nothing was read. A key that is not in
// the notation is no key the check can compare with, and the slot is unchecked: the format would
// not take such a check, and a bank of the measurement gives none.
func (n Number) Check(value *string) Slot {
	if value == nil {
		return Slot{Verdict: Unreadable, Reason: Blank}
	}
	v, reason := n.read(*value)
	if reason != "" {
		return Slot{Verdict: Unreadable, Reason: reason}
	}
	key, reason := n.read(n.Key)
	if reason != "" {
		return Slot{Verdict: Unchecked}
	}
	if v.Cmp(key) == 0 {
		return Slot{Verdict: Correct}
	}
	slot := Slot{Verdict: NotYet}
	for _, t := range n.Traps {
		if tv, reason := n.read(t.Value); reason == "" && tv.Cmp(v) == 0 {
			slot.Trap = t.Trap
			break
		}
	}
	return slot
}

// read reads a value as the check reads it, in steps 1 to 7 and the notation: the number, or why
// it is unreadable.
func (n Number) read(value string) (*big.Rat, Reason) {
	text, ok := Read(value, n.Units)
	if !ok {
		return nil, Blank
	}
	r, ok := Rational(text)
	if !ok {
		return nil, Notation
	}
	return r, ""
}

// Item is the verdict of an item, made of the verdicts of its answer slots: correct when every one
// is; otherwise not yet right when one is, since a value read clearly and wrong is no doubt;
// otherwise unreadable when one is; otherwise unchecked.
func Item(slots []Verdict) Verdict {
	switch {
	case len(slots) > 0 && !slices.ContainsFunc(slots, func(v Verdict) bool { return v != Correct }):
		return Correct
	case slices.Contains(slots, NotYet):
		return NotYet
	case slices.Contains(slots, Unreadable):
		return Unreadable
	}
	return Unchecked
}

// Read reads a value as the kind number reads it, in steps 1 to 7 of section 15: the text that is
// left, and false when it is blank.
func Read(value string, units []string) (string, bool) {
	text := strings.Trim(withoutUnit(steps1To5(value), units), " ")
	return text, text != ""
}

// Normalize is steps 1 to 4 of section 15, which a value and its units go through: the
// superscript and subscript digits, and the vulgar fractions a number touches, become U+FFFD;
// NFKC; the dashes and the minus become "-" and the fraction and division slashes "/"; every run
// of white space becomes one space, and white space at either end is dropped.
func Normalize(text string) string {
	text = norm.NFKC.String(scriptsReplaced(text))
	var b strings.Builder
	space := false
	for _, r := range text {
		switch {
		case unicode.Is(unicode.White_Space, r):
			space = true
			continue
		case r == '\u2010' || r >= '\u2012' && r <= '\u2015' || r == '\u2212':
			r = '-'
		case r == '\u2044' || r == '\u2215':
			r = '/'
		}
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		space = false
		b.WriteRune(r)
	}
	return b.String()
}

// scriptsReplaced is step 1: a superscript or subscript digit becomes U+FFFD, which no notation
// takes, so that NFKC does not make 7² into 72; so does a vulgar fraction that a number, a
// character of the general category N, stands right before or right after, or NFKC would make
// 1½ into 11⁄2, eleven halves.
func scriptsReplaced(text string) string {
	runes := []rune(text)
	number := func(i int) bool { return i >= 0 && i < len(runes) && unicode.IsNumber(runes[i]) }
	out := slices.Clone(runes)
	for i, r := range runes {
		script := r == '\u00B2' || r == '\u00B3' || r == '\u00B9' || r >= '\u2070' && r <= '\u209F'
		vulgar := r >= '\u00BC' && r <= '\u00BE' || r >= '\u2150' && r <= '\u215F' || r == '\u2189'
		if script || vulgar && (number(i-1) || number(i+1)) {
			out[i] = utf8.RuneError
		}
	}
	return string(out)
}

// steps1To5 is steps 1 to 5, which a number's value and its units both go through: Normalize, then
// a "=" at the start and every ".", "!" and "?" at the end dropped.
func steps1To5(text string) string {
	return strings.TrimRight(strings.TrimPrefix(Normalize(text), "="), ".!?")
}

// withoutUnit is step 6: of the units that end the text, with a space before it or none, the
// longest, dropped when something stands before it, and otherwise nothing is. Both sides compare
// by the simple lowercase mapping of each character.
func withoutUnit(text string, units []string) string {
	runes := []rune(text)
	lowered := Lowercase(runes)
	var fit []rune
	for _, u := range units {
		unit := Lowercase([]rune(strings.Trim(steps1To5(u), " ")))
		if len(unit) > len(fit) && len(unit) <= len(lowered) && slices.Equal(lowered[len(lowered)-len(unit):], unit) {
			fit = unit
		}
	}
	if len(fit) == 0 {
		return text
	}
	before := string(runes[:len(runes)-len(fit)])
	if strings.Trim(before, " ") == "" {
		return text
	}
	return strings.TrimSuffix(before, " ")
}

// Lowercase maps each character to its simple lowercase mapping, as unicode.ToLower does: a
// character at a time, so that no context, such as a final sigma, counts.
func Lowercase(runes []rune) []rune {
	out := make([]rune, len(runes))
	for i, r := range runes {
		out[i] = unicode.ToLower(r)
	}
	return out
}

// maxLength is the longest a value in the notation may be, in characters.
const maxLength = 16

// The notations of number: an integer, a decimal with a point, and a simple fraction.
var (
	integer  = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)
	decimal  = regexp.MustCompile(`^-?(0|[1-9][0-9]*)\.[0-9]+$`)
	fraction = regexp.MustCompile(`^-?(0|[1-9][0-9]*) ?/ ?[1-9][0-9]*$`)
)

// Rational is a text in a notation of number as a rational number, exactly, and false when the
// text is in none.
func Rational(text string) (*big.Rat, bool) {
	if utf8.RuneCountInString(text) > maxLength {
		return nil, false
	}
	switch {
	case integer.MatchString(text), decimal.MatchString(text):
	case fraction.MatchString(text):
		text = strings.ReplaceAll(text, " ", "")
	default:
		return nil, false
	}
	return new(big.Rat).SetString(text)
}
