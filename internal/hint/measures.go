package hint

import (
	"slices"
	"strings"
	"unicode"

	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/words"
)

// The measures of the set of the words that code counts (T118): they count words and read no
// meaning, so they are the eval's, not checks of api. Whether words tell the learner's sex no
// program checks without false alarms (R70 spec): Gendered is the rough search of T76, a signal
// beside the judge of the words, not a measure of admission.

// RepeatShare is how much of a question asked before a hint holds, its words as a set, to repeat it.
const RepeatShare = 0.8

// Repeated are the questions asked before that a hint repeats: those whose words, read as
// words.Plain reads them, the hint holds at least RepeatShare of. A hint that asks the same again
// with a word of praise before it repeats it as much as one that copies it.
func Repeated(hint string, asked []string) []string {
	has := map[string]bool{}
	for _, w := range words.Plain(hint) {
		has[w] = true
	}
	var out []string
	for _, q := range asked {
		set := map[string]bool{}
		for _, w := range words.Plain(q) {
			set[w] = true
		}
		held := 0
		for w := range set {
			if has[w] {
				held++
			}
		}
		if len(set) > 0 && float64(held) >= RepeatShare*float64(len(set)) {
			out = append(out, q)
		}
	}
	return out
}

// EchoWords is how many words of a note in a row a hint has to take it up: two are too few, as «is
// right» of a note stands in «The start is right».
const EchoWords = 3

// Echo is what a hint takes up of a note at the side: EchoWords words of the note in a row that the
// hint has in a row too, and a number of the note that the hint has and that none of the known texts
// has — the statement, the solution and the learner's own lines but the note —, such as a telephone.
// Each comes once, in the order of the note. The judge of the words reads what it says of the note
// in other words.
func Echo(hint, note string, known ...string) []string {
	if note == "" {
		return nil
	}
	h, n := words.Plain(hint), words.Plain(note)
	joined := " " + strings.Join(h, " ") + " "
	task := map[string]bool{}
	for _, w := range words.Plain(strings.Join(known, " ")) {
		task[w] = true
	}
	var out []string
	add := func(s string) {
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	for i := range n {
		if i+EchoWords <= len(n) {
			if run := strings.Join(n[i:i+EchoWords], " "); strings.Contains(joined, " "+run+" ") {
				add(run)
			}
		}
		if number(n[i]) && !task[n[i]] && slices.Contains(h, n[i]) {
			add(n[i])
		}
	}
	return out
}

// number is whether a word is digits alone.
func number(w string) bool {
	return w != "" && strings.IndexFunc(w, func(r rune) bool { return !unicode.IsDigit(r) }) < 0
}

// Gendered are the words of a hint by which the rough search of T76 finds the learner's sex told
// (docs/architecture.md, 3.4, «Итог замера подсказок»). In Russian: the hint speaks to the learner,
// «ты» or «вы», and has a word that ends in a vowel and «л», «ла», «лся» or «лась», such as «нашёл»
// or «решила», or it has «прав», «права», «молодец» or «умница». In English: a pronoun of the third
// person that tells a sex, he, she, him, her, his, hers, himself or herself. It finds the sex of
// whoever the words speak of, the learner or a person of the task, and misses what it does not
// list: the judge of the words decides.
func Gendered(hint string, lang sample.Lang) []string {
	ws := words.Plain(hint)
	var out []string
	switch lang {
	case sample.LangRussian:
		if !slices.Contains(ws, "ты") && !slices.Contains(ws, "вы") {
			return nil
		}
		for _, w := range ws {
			if pastTense(w) || slices.Contains([]string{"прав", "права", "молодец", "умница"}, w) {
				out = append(out, w)
			}
		}
	case sample.LangEnglish:
		for _, w := range ws {
			if slices.Contains([]string{"he", "she", "him", "her", "his", "hers", "himself", "herself"}, w) {
				out = append(out, w)
			}
		}
	}
	return out
}

// pastTense is whether a Russian word ends in a vowel and «л», «ла», «лся» or «лась».
func pastTense(w string) bool {
	r := []rune(w)
	for _, end := range []string{"л", "ла", "лся", "лась"} {
		e := []rune(end)
		if len(r) > len(e) && string(r[len(r)-len(e):]) == end && strings.ContainsRune("аеёиоуыэюя", r[len(r)-len(e)-1]) {
			return true
		}
	}
	return false
}
