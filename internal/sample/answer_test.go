package sample_test

import (
	"slices"
	"testing"

	"github.com/altnovatrail/tutor/internal/sample"
)

// The value of a written answer is the first number after its last "=", or after «Ответ:».
func TestValueOf(t *testing.T) {
	for in, want := range map[string]string{
		"x = 2":               "2",
		"x = 13/2":            "13/2",
		"17 + 49 − 28 = 38":   "38",
		"56 : 6 = 9 (ост. 2)": "9",
		"Ответ: 5 столбов":    "5",
		"ответ: в точке 10":   "10",
		"Ответ: −3":           "-3",
		"12":                  "12",
		"3x +":                "3",
		"Ответ: пять":         "",
		"x = 2,5":             "2,5",
	} {
		if got := sample.ValueOf(in); got != want {
			t.Errorf("ValueOf(%q) = %q, want %q", in, got, want)
		}
		if got := sample.KeyOf(in); got != want {
			t.Errorf("KeyOf(%q) = %q, want %q", in, got, want)
		}
	}
}

// The numbers of a text are all of them, in order, normalized.
func TestNumbers(t *testing.T) {
	if got, want := sample.Numbers("17 + 49 − 28 = 38, ост. −2 и 3/4"), []string{"17", "49", "28", "38", "-2", "3/4"}; !slices.Equal(got, want) {
		t.Errorf("Numbers = %q, want %q", got, want)
	}
	if got := sample.Numbers("пять"); got != nil {
		t.Errorf("Numbers of no number = %q", got)
	}
}
