package sample

import (
	"regexp"
	"strings"

	"github.com/altnovatrail/tutor/internal/value"
)

// numberPattern is a number in a normalized text: an integer, a decimal, with a point or a comma,
// or a simple fraction; a decimal comma is taken whole, so that the check finds it out of the
// notation rather than reads only its whole part. The answer word is how the sheets of the
// measurement begin a final answer that has no "=".
var (
	numberPattern = regexp.MustCompile(`-?[0-9]+(?:[.,/][0-9]+)?`)
	answerWord    = regexp.MustCompile(`(?i)ответ:`)
)

// Numbers are the numbers of a text, in the order they stand, normalized (value.Normalize).
func Numbers(text string) []string {
	return numberPattern.FindAllString(value.Normalize(text), -1)
}

// ValueOf is the number a written answer gives: the first number after its last "=", and in a line
// without one the first number after «Ответ:», or else the first of the line; "" when it has none.
// So the bank's key "x = 2" is the key 2 of the check, and the answer line «56 : 6 = 9 (ост. 2)»
// gives 9. The banks of the measurement write their keys so, and they do not change (R28).
func ValueOf(text string) string {
	t := value.Normalize(text)
	if i := strings.LastIndex(t, "="); i >= 0 {
		t = t[i+1:]
	} else if at := answerWord.FindStringIndex(t); at != nil {
		t = t[at[1]:]
	}
	return numberPattern.FindString(t)
}

// KeyOf is the key of the check of a task's key, or of a false key: its value (ValueOf).
func KeyOf(key string) string { return ValueOf(key) }
