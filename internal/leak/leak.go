// Package leak is the leak check of the module format (R57 spec; the format text of spec, section
// 20, «The leak check»): the program's search for the answer in a text the learner will see, the
// tutor's words before they reach the learner. It searches for the key of every check of a number,
// a fraction, a choice or a time that the texts of its own item do not show, a number by its value
// and a time by its hours and minutes, and for every leak phrase of the task. It is a safeguard,
// not a proof: an answer put in other words, or a number in words, passes it unless a leak phrase
// holds it. The runner checks every hint with it,
// as api does before the tutor's words reach the device (architecture 3.5 spec; T75).
//
// spec's own second implementation is tools/format-check/leaks.mjs, which spec's CI computes the
// test vectors with; this one passes the same vectors (TestVectors). The scans are loops over the
// characters, as there: a regular expression over a text of any length could backtrack.
package leak

import (
	"math/big"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/altnovatrail/tutor/internal/value"
)

// Task is a task as the leak check reads it: the texts of its statement, its items and its leak
// phrases.
type Task struct {
	// Statement is the paragraphs of the statement and the alt of its pictures.
	Statement []string
	Items     []Item
	Phrases   []string
}

// Item is an item of a task: the texts the learner sees with it, besides the statement — its text,
// the text pieces of its line and the options of its answer slots bound choice — and the keys of
// its checks.
type Item struct {
	Texts []string
	Keys  []Key
}

// Key is the key of a check, by the check's kind, as the task writes it. Only number, fraction,
// choice and time have one: a predicate or a rubric gives no Key.
type Key struct {
	Kind  string
	Value string
}

// The kinds of check with a key.
const (
	Number   = "number"
	Fraction = "fraction"
	Choice   = "choice"
	Time     = "time"
)

// Check is the leak check of a task: what it searches for, each normalized once.
type Check struct {
	needles []needle
}

// needle is a key or a leak phrase as the check searches for it: a key of a number or a fraction by
// its magnitude, a key of a time by its hours and minutes, and a key of a choice or a leak phrase by
// its normal form. Written is how the task writes it.
type needle struct {
	written   string
	key       bool
	magnitude *big.Rat
	time      string
	form      []rune
}

// searched is a text in its normal form, with its numbers by their magnitudes and its times by their
// hours and minutes.
type searched struct {
	chars   []rune
	numbers []*big.Rat
	times   []string
}

// New is the leak check of a task: the key of every check of a number, a fraction, a choice or a
// time that is not found in a text the learner sees with its item — the statement and the item's own
// texts —, and every leak phrase, always. A key of a number, a fraction or a time out of its kind's
// notation is searched for nowhere, as spec's implementation does: the format would not take it. A
// key of a choice by tapping is among its item's options, so it is never searched for.
func New(t Task) *Check {
	c := &Check{}
	for _, it := range t.Items {
		var own []searched
		for _, text := range slices.Concat(t.Statement, it.Texts) {
			own = append(own, search(text))
		}
		for _, k := range it.Keys {
			n, ok := needleOf(k)
			if !ok || slices.ContainsFunc(own, func(s searched) bool { return s.holds(n) }) {
				continue
			}
			c.needles = append(c.needles, n)
		}
	}
	for _, p := range t.Phrases {
		c.needles = append(c.needles, needle{written: p, form: form(p)})
	}
	return c
}

// Keys are the keys the check searches for, and Phrases the leak phrases, as the task writes them.
func (c *Check) Keys() []string    { return c.written(true) }
func (c *Check) Phrases() []string { return c.written(false) }

func (c *Check) written(keys bool) []string {
	var out []string
	for _, n := range c.needles {
		if n.key == keys {
			out = append(out, n.written)
		}
	}
	return out
}

// Find is every key and leak phrase the check finds in a text, as the task writes them, each once,
// keys first; nil when it finds none.
func (c *Check) Find(text string) []string {
	keys, phrases := c.Split(text)
	out := keys
	for _, p := range phrases {
		if !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	return out
}

// Split is what the check finds in a text, the keys apart from the leak phrases, each once, as the
// task writes them: a leak phrase written as a key is still a phrase.
func (c *Check) Split(text string) (keys, phrases []string) {
	s := search(text)
	for _, n := range c.needles {
		switch {
		case !s.holds(n):
		case n.key && !slices.Contains(keys, n.written):
			keys = append(keys, n.written)
		case !n.key && !slices.Contains(phrases, n.written):
			phrases = append(phrases, n.written)
		}
	}
	return keys, phrases
}

// needleOf is a key as the check searches for it, and false for a kind with no key or a key of a
// number, a fraction or a time out of its notation.
func needleOf(k Key) (needle, bool) {
	switch k.Kind {
	case Number, Fraction:
		r, ok := value.Rational(k.Value)
		if !ok || k.Kind == Fraction && !strings.Contains(k.Value, "/") {
			return needle{}, false
		}
		return needle{written: k.Value, key: true, magnitude: new(big.Rat).Abs(r)}, true
	case Choice:
		return needle{written: k.Value, key: true, form: form(k.Value)}, true
	case Time:
		t, ok := timeOf(k.Value)
		if !ok {
			return needle{}, false
		}
		return needle{written: k.Value, key: true, time: t}, true
	}
	return needle{}, false
}

// form is the normal form of a text, of a key of a choice and of a leak phrase: steps 1 to 4 of
// section 15 — step 1, NFKC, the dashes and slashes and the white space (value.Normalize) — and then
// each character to its simple lowercase mapping.
func form(text string) []rune {
	return value.Lowercase([]rune(value.Normalize(text)))
}

// search is a text in its normal form, with its numbers and its times.
func search(text string) searched {
	chars := form(text)
	return searched{chars: chars, numbers: numbersIn(chars), times: timesIn(chars)}
}

// holds is whether a needle is found in the text: a key of a number where a number of the text has
// its magnitude, a key of a time where a time of the text has its hours and minutes, and a phrase as
// a whole.
func (s searched) holds(n needle) bool {
	switch {
	case n.magnitude != nil:
		return slices.ContainsFunc(s.numbers, func(r *big.Rat) bool { return r.Cmp(n.magnitude) == 0 })
	case n.time != "":
		return slices.Contains(s.times, n.time)
	}
	return holdsForm(s.chars, n.form)
}

// IsWord is whether a character is a character of a word: a letter, a mark or a number, the general
// categories L, M and N. The check of a hint's form counts words by it too.
func IsWord(r rune) bool { return unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsNumber(r) }

// word is whether the character at i is a character of a word; outside the text there is none.
func word(chars []rune, i int) bool { return i >= 0 && i < len(chars) && IsWord(chars[i]) }

// digit is whether the character at i is a digit, 0 to 9.
func digit(chars []rune, i int) bool {
	return i >= 0 && i < len(chars) && chars[i] >= '0' && chars[i] <= '9'
}

// holdsForm is whether a phrase in its normal form stands in a text's normal form, with no
// character of a word right outside an end of it that is a character of a word itself. A phrase
// whose normal form is empty is found nowhere.
func holdsForm(chars, phrase []rune) bool {
	if len(phrase) == 0 {
		return false
	}
	opens, closes := word(phrase, 0), word(phrase, len(phrase)-1)
	for at := 0; at+len(phrase) <= len(chars); at++ {
		switch {
		case !slices.Equal(chars[at:at+len(phrase)], phrase):
		case opens && word(chars, at-1):
		case closes && word(chars, at+len(phrase)):
		default:
			return true
		}
	}
	return false
}

// numbersIn are the numbers of a text in its normal form, by their magnitudes: at each digit the
// longest run of the form of a number is taken, and it is a number when no character of a word
// stands right before it and none right after it. Either way the search goes on after the whole
// run, so that no tail of a longer token, as the 5 of x7.5, is read as a number. A sign is no part
// of a number: the check compares magnitudes.
func numbersIn(chars []rune) []*big.Rat {
	var out []*big.Rat
	for at := 0; at < len(chars); {
		if !digit(chars, at) {
			at++
			continue
		}
		end, r := longestRun(chars, at)
		if !word(chars, at-1) && !word(chars, end) {
			out = append(out, r)
		}
		at = end
	}
	return out
}

// longestRun is the longest run of the form of a number that begins at a digit, and its value: a
// simple fraction — digits, an optional space, "/", an optional space and digits that are not all
// zero —, a decimal — digits, "." and digits — or an integer, digits. The digits are read in base
// 10, leading zeros and all: big.Rat.SetString would read a leading 0 of a fraction as octal.
func longestRun(chars []rune, at int) (int, *big.Rat) {
	whole := digitsEnd(chars, at)
	end, r := whole, new(big.Rat).SetInt(integer(chars[at:whole]))
	if whole < len(chars) && chars[whole] == '.' && digit(chars, whole+1) {
		e := digitsEnd(chars, whole+1)
		r = new(big.Rat).SetFrac(integer(slices.Concat(chars[at:whole], chars[whole+1:e])), new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(e-whole-1)), nil))
		end = e
	}
	slash := whole
	if slash < len(chars) && chars[slash] == ' ' {
		slash++
	}
	if slash < len(chars) && chars[slash] == '/' {
		start := slash + 1
		if start < len(chars) && chars[start] == ' ' {
			start++
		}
		e := digitsEnd(chars, start)
		if den := integer(chars[start:e]); e > start && e > end && den.Sign() != 0 {
			end, r = e, new(big.Rat).SetFrac(integer(chars[at:whole]), den)
		}
	}
	return end, r
}

// timesIn are the times of a text in its normal form, each by its hours and minutes, as timeOf
// gives them: at each digit, the run of digits, ":" and digits, which is a time when it is in the
// notation of a time and no character of a word stands right before it and none right after it;
// either way the search goes on after the run, as for numbers. So 07:30 holds the time 7:30, and
// 17:30 and a score 7:3 do not.
func timesIn(chars []rune) []string {
	var out []string
	for at := 0; at < len(chars); {
		if !digit(chars, at) {
			at++
			continue
		}
		hours := digitsEnd(chars, at)
		if hours >= len(chars) || chars[hours] != ':' || !digit(chars, hours+1) {
			at = hours
			continue
		}
		end := digitsEnd(chars, hours+1)
		if t, ok := timeOf(string(chars[at:end])); ok && !word(chars, at-1) && !word(chars, end) {
			out = append(out, t)
		}
		at = end
	}
	return out
}

// timeOf is a time in the notation of a time (the format text of spec, section 15; R111 spec) —
// hours of one or two digits, ":" and minutes of two digits or of three that begin with no zero, so
// that 5:60 and 10:100 are times a trap can show — by its hours and minutes as numbers, "h:m", so
// that 07:30 is 7:30; and false out of the notation.
func timeOf(text string) (string, bool) {
	hours, minutes, ok := strings.Cut(text, ":")
	if !ok || len(hours) < 1 || len(hours) > 2 || !allDigits(hours) || !allDigits(minutes) {
		return "", false
	}
	if len(minutes) != 2 && (len(minutes) != 3 || minutes[0] == '0') {
		return "", false
	}
	h, _ := strconv.Atoi(hours)
	m, _ := strconv.Atoi(minutes)
	return strconv.Itoa(h) + ":" + strconv.Itoa(m), true
}

// allDigits is whether a text is digits, 0 to 9, and at least one.
func allDigits(text string) bool {
	return text != "" && strings.IndexFunc(text, func(r rune) bool { return r < '0' || r > '9' }) < 0
}

// digitsEnd is where the digits from at end.
func digitsEnd(chars []rune, at int) int {
	for digit(chars, at) {
		at++
	}
	return at
}

// integer is digits as an integer in base 10; no digits are 0.
func integer(digits []rune) *big.Int {
	n, ok := new(big.Int).SetString(string(digits), 10)
	if !ok {
		return new(big.Int)
	}
	return n
}
