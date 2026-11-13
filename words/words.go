// Package words holds the checks of the tutor's words that api makes before they reach the
// learner, beside the leak check (R70 spec; architecture 1 and 3.5): the form of a hint, the form of
// the words after a correct answer and the rule of feedback by a rubric. The eval of tutor measures
// the models by the same code, so that what it admits is what the platform lets through. The checks
// only count: they read no meaning, and what a word, a sentence and a quote are, this package
// decides.
package words

import (
	"slices"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/altnovatrail/tutor/internal/leak"
)

// Breach is a part of a check that words break, as a log names it.
type Breach string

// The breaches, in the order the checks name them.
const (
	// Questions: not exactly one sentence of a hint is a question.
	Questions Breach = "questions"
	// Sentences: more sentences than the check takes.
	Sentences Breach = "sentences"
	// Last: the question of the words after a correct answer is not their last sentence.
	Last Breach = "last"
	// Words: no word, or more than the limit of the language.
	Words Breach = "words"
	// Quote: a quoted span of QuoteWords words or more that the learner did not write.
	Quote Breach = "quote"
	// Script: a letter that is not of the script of the language.
	Script Breach = "script"
	// Language: the language is no tag a check can read, such as an empty one: the check fails
	// closed rather than drop its limits.
	Language Breach = "language"
)

// The limits of R70 spec that hold in every language.
const (
	// HintSentences is the most sentences a hint has.
	HintSentences = 2
	// FeedbackSentences is the most sentences feedback has.
	FeedbackSentences = 3
	// QuoteWords is how many words a quoted span has at least to be held to the learner's own.
	QuoteWords = 4
)

// HintWords is the most words R70 spec lets a hint have in a language, by the numbers of the
// measurement (T76): 30 in Russian and 35 in English; 0, no limit, in a language the measurement
// did not measure. A language is its tag, such as "en", "ru-RU" or "en_US", and its first subtag
// counts.
func HintWords(language string) int {
	switch base(language) {
	case "ru":
		return 30
	case "en":
		return 35
	}
	return 0
}

// FeedbackWords is the most words R70 spec lets feedback have: 80 in English, the one language the
// measurement of essays measured, and 0, no limit, in another.
func FeedbackWords(language string) int {
	if base(language) == "en" {
		return 80
	}
	return 0
}

// Hint is what a hint breaks of the form R70 spec sets, in the order of the breaches; nil when it
// keeps it: exactly one question, in no more than HintSentences sentences, of 1 to HintWords words
// of its language, and its letters of the script of its language. The statement is the task's, whose
// Latin letters, such as the x of an equation, may stand in a Russian hint.
func Hint(text, language, statement string) []Breach {
	return HintForm(text, language, statement, HintWords(language))
}

// HintForm is Hint with a limit of words of its own, 0 for none: the measurement of epic E4 held its
// hints to 30 words in either language, as its prompt said (R39).
func HintForm(text, language, statement string, maxWords int) []Breach {
	runes := []rune(norm.NFKC.String(text))
	sentences, questions, n, _ := count(runes)
	var out []Breach
	if questions != 1 {
		out = append(out, Questions)
	}
	if sentences > HintSentences {
		out = append(out, Sentences)
	}
	return append(out, rest(runes, n, language, statement, maxWords)...)
}

// Correct is what the words after a correct answer break of their form, in the order of the
// breaches; nil when they keep it: the praise of a step and one question on the technique that
// helped (concepts 7, 10 spec), as option (2) of architecture 2.8 sets it — exactly one question,
// and it the last sentence, so that no more than one sentence of praise stands before it; 1 to
// HintWords words of the language; and the letters of its script, as a hint's.
func Correct(text, language, statement string) []Breach {
	return CorrectForm(text, language, statement, HintWords(language))
}

// CorrectForm is Correct with a limit of words of its own, 0 for none.
func CorrectForm(text, language, statement string, maxWords int) []Breach {
	runes := []rune(norm.NFKC.String(text))
	sentences, questions, n, last := count(runes)
	var out []Breach
	if questions != 1 {
		out = append(out, Questions)
	}
	if sentences > HintSentences {
		out = append(out, Sentences)
	}
	if questions > 0 && !last {
		out = append(out, Last)
	}
	return append(out, rest(runes, n, language, statement, maxWords)...)
}

// rest is what a text of n words breaks of the parts the forms of a hint and of the words after a
// correct answer share: the words, the script and the language.
func rest(runes []rune, n int, language, statement string, maxWords int) []Breach {
	var out []Breach
	if Over(n, maxWords) {
		out = append(out, Words)
	}
	if !ofScript(runes, language, statement) {
		out = append(out, Script)
	}
	if !readable(language) {
		out = append(out, Language)
	}
	return out
}

// Over is whether n words break a limit: none at all, or more than the limit, 0 being no limit.
func Over(n, limit int) bool { return n < 1 || limit > 0 && n > limit }

// Feedback is what feedback by a rubric breaks of the rule R70 spec sets, in the order of the
// breaches; nil when it keeps it: no more than FeedbackSentences sentences, 1 to FeedbackWords words
// of its language, every quoted span of QuoteWords words or more the learner's own words in a row in
// the text read — the text the assessment got, not the essay as written, which the platform does not
// have —, and its letters of the script of its language.
func Feedback(text, read, language string) []Breach {
	breaches, _ := FeedbackForm(text, read, language, FeedbackWords(language))
	return breaches
}

// FeedbackForm is Feedback with a limit of words of its own, 0 for none, against the learner's own
// text, whatever it is, with the words the feedback has: the measurement of epic E4 held feedback to
// 60 words, against the essay as it was written (R45). Its letters are read as written, not after
// NFKC, as the measurement read them.
func FeedbackForm(text, own, language string, maxWords int) (breaches []Breach, words int) {
	sentences, _, n := Count(text)
	var out []Breach
	if sentences > FeedbackSentences {
		out = append(out, Sentences)
	}
	if Over(n, maxWords) {
		out = append(out, Words)
	}
	ownWords := " " + strings.Join(Plain(own), " ") + " "
	for _, q := range quoted(text) {
		if w := Plain(q); len(w) >= QuoteWords && !strings.Contains(ownWords, " "+strings.Join(w, " ")+" ") {
			out = append(out, Quote)
			break
		}
	}
	if !ofScript([]rune(text), language, "") {
		out = append(out, Script)
	}
	if !readable(language) {
		out = append(out, Language)
	}
	return out, n
}

// base is the first subtag of a language tag, in small letters: before a "-" or a "_".
func base(language string) string {
	b, _, _ := strings.Cut(strings.ReplaceAll(language, "_", "-"), "-")
	return strings.ToLower(b)
}

// Readable is whether a language tag has a first subtag a check can read: two or three letters.
// A check of a tag that is not fails closed, with the breach Language.
func Readable(language string) bool { return readable(language) }

// readable is Readable.
func readable(language string) bool {
	b := base(language)
	if len(b) < 2 || len(b) > 3 {
		return false
	}
	for _, r := range b {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}

// Count counts the sentences, the questions and the words of a text, read after NFKC, so that "…"
// is three points and a full-width "?" a question mark.
//
// A word is a run of letters, marks and numbers (leak.IsWord), and a point, comma or slash between
// two digits joins them: 0.5 and 4/3 are a word each. A sentence ends at a run of ".", "!" and "?",
// among which closing quotes and brackets may stand, and then white space, a capital letter or the
// end; a question is a sentence whose run holds a "?". Its tail without a point is a sentence too,
// and a run with no word before it ends none. So "0.5" ends nothing, but an abbreviation such as
// «т. е.» ends a sentence: a limit of the rule.
func Count(text string) (sentences, questions, words int) {
	sentences, questions, words, _ = count([]rune(norm.NFKC.String(text)))
	return sentences, questions, words
}

// count is Count of a text read after NFKC, as runes, and whether its last sentence is a question.
func count(runes []rune) (sentences, questions, words int, last bool) {
	inWord, hasWord := false, false
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case leak.IsWord(r):
			if !inWord {
				words++
			}
			inWord, hasWord = true, true
			continue
		case inWord && inNumber(runes, i):
			continue
		}
		inWord = false
		if !strings.ContainsRune(".!?", r) {
			continue
		}
		end, question := i, false
		for end < len(runes) && strings.ContainsRune(`.!?»"”’)]`, runes[end]) {
			question = question || runes[end] == '?'
			end++
		}
		i = end - 1
		if hasWord && (end == len(runes) || unicode.IsSpace(runes[end]) || unicode.IsUpper(runes[end])) {
			sentences++
			if question {
				questions++
			}
			last, hasWord = question, false
		}
	}
	if hasWord {
		sentences++
		last = false
	}
	return sentences, questions, words, last
}

// inNumber is whether the character at i is a point, a comma or a slash between two digits.
func inNumber(runes []rune, i int) bool {
	return strings.ContainsRune(".,/", runes[i]) && i > 0 && i+1 < len(runes) && unicode.IsDigit(runes[i-1]) && unicode.IsDigit(runes[i+1])
}

// ofScript is whether every letter of a text is of the script of its language: Cyrillic in Russian,
// or a Latin letter that the statement shows, read after NFKC as the text is, whatever its case;
// Latin in English. A language of another script has no check here: its script is not known to the
// platform yet. A mark, such as a stress, is no letter, and neither is a sign such as □.
func ofScript(runes []rune, language, statement string) bool {
	lang := base(language)
	if lang != "ru" && lang != "en" {
		return true
	}
	var shown []rune
	for _, r := range norm.NFKC.String(statement) {
		if unicode.Is(unicode.Latin, r) {
			shown = append(shown, unicode.ToLower(r))
		}
	}
	for _, r := range runes {
		if !unicode.IsLetter(r) {
			continue
		}
		switch {
		case lang == "en" && unicode.Is(unicode.Latin, r):
		case lang == "ru" && unicode.Is(unicode.Cyrillic, r):
		case lang == "ru" && unicode.Is(unicode.Latin, r) && slices.Contains(shown, unicode.ToLower(r)):
		default:
			return false
		}
	}
	return true
}

// quotes are the quotation marks a feedback may quote the learner in, each opening mark with the
// marks that close it, read after NFKC, so that a full-width ＂ is a straight one. A straight single
// quote is an apostrophe too, as in "don't", and opens no quote.
var quotes = map[rune]string{'"': `"`, '“': "”", '«': "»", '‘': "’", '„': "“”", '‹': "›"}

// quoted are the spans of a text between a quotation mark and the mark that closes it. A ’ between
// two letters is an apostrophe, as in "didn’t", and closes nothing; a quote that is never closed
// runs to the end of the text, so that what it holds is checked all the same.
func quoted(text string) []string {
	var out []string
	rs := []rune(norm.NFKC.String(text))
	for i := 0; i < len(rs); i++ {
		closing, ok := quotes[rs[i]]
		if !ok {
			continue
		}
		j := i + 1
		for ; j < len(rs); j++ {
			if !strings.ContainsRune(closing, rs[j]) {
				continue
			}
			if rs[j] == '’' && j+1 < len(rs) && unicode.IsLetter(rs[j-1]) && unicode.IsLetter(rs[j+1]) {
				continue
			}
			break
		}
		out = append(out, string(rs[i+1:min(j, len(rs))]))
		i = j
	}
	return out
}

// Plain are the words of a text in small letters, without the marks around them: what quotes are
// compared by, and the reading of an essay too. A typographic apostrophe is the straight one.
func Plain(text string) []string {
	var out []string
	for _, f := range strings.Fields(strings.ReplaceAll(text, "’", "'")) {
		if w := strings.ToLower(strings.TrimFunc(f, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })); w != "" {
			out = append(out, w)
		}
	}
	return out
}
