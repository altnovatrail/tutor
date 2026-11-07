// Package value is how tutor reads a value as the module format's checks read it (R56 spec; the
// format text of spec, section 15, «Checks»): the normalization of steps 1 to 4, a number's value
// read in steps 1 to 5 and 7, and a text in a notation of number as a rational number, exactly. The
// reading compares the texts of parts by it and finds a value in its part's writing, the leak check
// finds the key's number, and the reports compare what was read with what was written. The verdict
// is module-kit's: the checks of number and fraction went to it, with the copy of the vectors of
// verdicts (T108; R73).
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

// Read reads a value as the kind number reads it with no units, in steps 1 to 5 and 7 of section
// 15: the text that is left, and false when it is blank.
func Read(value string) (string, bool) {
	text := strings.Trim(steps1To5(value), " ")
	return text, text != ""
}

// Normalize is steps 1 to 4 of section 15, which a value goes through: the
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

// steps1To5 is steps 1 to 5, which a number's value goes through: Normalize, then
// a "=" at the start and every ".", "!" and "?" at the end dropped.
func steps1To5(text string) string {
	return strings.TrimRight(strings.TrimPrefix(Normalize(text), "="), ".!?")
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
