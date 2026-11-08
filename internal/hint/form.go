package hint

import (
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/words"
)

// MaxWords is the most words a hint may have: two short sentences of about 15 words each (R39). The
// prompt names it too.
const MaxWords = 30

// The breaches of the form of a hint, as the log names them (concepts 10 spec; R39).
const (
	// BreachQuestions: not exactly one sentence is a question.
	BreachQuestions = string(words.Questions)
	// BreachSentences: more than two sentences.
	BreachSentences = string(words.Sentences)
	// BreachWords: no word, or more than MaxWords.
	BreachWords = string(words.Words)
	// BreachScript: a letter not of the script of the hint's language.
	BreachScript = string(words.Script)
)

// Form is what a hint breaks of its form, in the order of the constants above; nil when it keeps
// it: the form of the measurement (words.HintForm, with MaxWords), the one the prompt of version 1
// names, so that the logs of epic E4 read as they were judged. Its sentences, questions and words
// are counted as words.Count counts them.
func Form(text string, lang sample.Lang, statement string) []string {
	var out []string
	for _, b := range words.HintForm(text, string(lang), statement, MaxWords) {
		out = append(out, string(b))
	}
	return out
}

// PlatformWords is the most words R70 spec lets a hint have in its language (words.HintWords): 30
// in Russian and 35 in English, and no limit, 0, in a language the measurement did not measure.
func PlatformWords(lang sample.Lang) int { return words.HintWords(string(lang)) }

// PlatformBroken is whether a hint breaks the form R70 spec sets, given what Form found it breaks:
// the same rules, with PlatformWords in place of MaxWords — what words.Hint gives, which api calls.
func PlatformBroken(text string, lang sample.Lang, breaches []string) bool {
	for _, b := range breaches {
		if b != BreachWords {
			return true
		}
	}
	_, _, n := words.Count(text)
	return words.Over(n, PlatformWords(lang))
}

// Count counts the sentences, the questions and the words of a text, as words.Count does.
func Count(text string) (sentences, questions, n int) { return words.Count(text) }
