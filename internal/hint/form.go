package hint

import (
	"slices"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/altnovatrail/tutor/internal/leak"
	"github.com/altnovatrail/tutor/internal/sample"
)

// MaxWords is the most words a hint may have: two short sentences of about 15 words each (R39). The
// prompt names it too.
const MaxWords = 30

// The breaches of the form of a hint, as the log names them (concepts 10 spec; R39).
const (
	// BreachQuestions: not exactly one sentence is a question.
	BreachQuestions = "questions"
	// BreachSentences: more than two sentences.
	BreachSentences = "sentences"
	// BreachWords: no word, or more than MaxWords.
	BreachWords = "words"
	// BreachScript: a letter not of the script of the hint's language.
	BreachScript = "script"
)

// Form is what a hint breaks of its form, in the order of the constants above; nil when it keeps
// it. A hint is exactly one question in no more than two sentences, of 1 to MaxWords words, and its
// letters are of the script of its language: Cyrillic in Russian, where a Latin letter the statement
// shows itself, such as the x of an equation, may stand too, and Latin in English. Its sentences,
// questions and words are counted as Count counts them.
func Form(text string, lang sample.Lang, statement string) []string {
	runes := []rune(norm.NFKC.String(text))
	sentences, questions, words := count(runes)
	var out []string
	if questions != 1 {
		out = append(out, BreachQuestions)
	}
	if sentences > 2 {
		out = append(out, BreachSentences)
	}
	if words < 1 || words > MaxWords {
		out = append(out, BreachWords)
	}
	if !ofScript(runes, lang, statement) {
		out = append(out, BreachScript)
	}
	return out
}

// Count counts the sentences, the questions and the words of a text, read after NFKC, so that "…"
// is three points and a full-width "?" a question mark. The feedback on an essay is counted so too
// (T83).
//
// A word is a run of letters, marks and numbers (leak.IsWord), and a point, comma or slash between
// two digits joins them: 0.5 and 4/3 are a word each. A sentence ends at a run of ".", "!" and "?",
// among which closing quotes and brackets may stand, and then white space, a capital letter or the
// end; a question is a sentence whose run holds a "?". Its tail without a point is a sentence too,
// and a run with no word before it ends none. So "0.5" ends nothing, but an abbreviation such as
// «т. е.» ends a sentence: a limit of the rule.
func Count(text string) (sentences, questions, words int) {
	return count([]rune(norm.NFKC.String(text)))
}

// count is Count of a text read after NFKC, as runes.
func count(runes []rune) (sentences, questions, words int) {
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
			hasWord = false
		}
	}
	if hasWord {
		sentences++
	}
	return sentences, questions, words
}

// inNumber is whether the character at i is a point, a comma or a slash between two digits.
func inNumber(runes []rune, i int) bool {
	return strings.ContainsRune(".,/", runes[i]) && i > 0 && i+1 < len(runes) && unicode.IsDigit(runes[i-1]) && unicode.IsDigit(runes[i+1])
}

// ofScript is whether every letter of a text is of the script of its language: Cyrillic in Russian,
// or a Latin letter that the statement shows, read after NFKC as the hint is, whatever its case;
// Latin in English. A mark, such as a stress, is no letter, and neither is a sign such as □.
func ofScript(runes []rune, lang sample.Lang, statement string) bool {
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
		case lang == sample.LangEnglish && unicode.Is(unicode.Latin, r):
		case lang == sample.LangRussian && unicode.Is(unicode.Cyrillic, r):
		case lang == sample.LangRussian && unicode.Is(unicode.Latin, r) && slices.Contains(shown, unicode.ToLower(r)):
		default:
			return false
		}
	}
	return true
}
