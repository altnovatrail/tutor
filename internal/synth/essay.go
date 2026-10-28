package synth

import (
	"math/rand/v2"
	"strings"
	"unicode/utf8"

	"github.com/altnovatrail/tutor/internal/sample"
)

// The page of an essay (T82; docs/architecture.md, 3.4, «Сочинения») is a handwriting sheet of the
// module format, 1000 × 1400 units (spec's format, section 14). The title stands on its first line,
// and the text below it, broken between words into lines of at most essayMinLine to essayMaxLine
// characters, a number each essay draws from the seed: so the size of the hand varies from essay to
// essay, since the hand is fitted to the longest line (Text).
const (
	EssayWidth   = 1000
	EssayHeight  = 1400
	essayMinLine = 28
	essayMaxLine = 40
)

// essayStream sets the stream an essay draws the width of its lines from apart from the stream of
// its hand, which is that of a sheet of its number.
const essayStream = 1 << 32

// EssayLines are the lines an essay is written in: its title, when it has one, and its text broken
// between words into lines at most width characters long. A word longer than that takes a line of
// its own.
func EssayLines(e *sample.Essay, width int) []string {
	var lines []string
	if e.Title != nil {
		lines = append(lines, *e.Title)
	}
	line := ""
	for _, w := range strings.Fields(e.Text) {
		switch {
		case line == "":
			line = w
		case utf8.RuneCountInString(line)+1+utf8.RuneCountInString(w) <= width:
			line += " " + w
		default:
			lines = append(lines, line)
			line = w
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

// Essay draws an essay as a learner writes it, from the seed, on a page of EssayWidth × EssayHeight:
// the lines it is written in, and its strokes in the order they were written. Each essay draws the
// width of its lines and its hand from the seed and its number. Between two words the hand leaves a
// gap of two spaces, a whole advance: the half advance of a space in a line of sums would run the
// words of a text together. It panics on an essay with no words.
func Essay(e *sample.Essay, seed uint64) (lines []string, strokes []sample.Stroke) {
	rng := rand.New(rand.NewPCG(seed, essayStream+uint64(e.Number)))
	lines = EssayLines(e, essayMinLine+rng.IntN(essayMaxLine-essayMinLine+1))
	gapped := make([]string, len(lines))
	for i, l := range lines {
		gapped[i] = strings.ReplaceAll(l, " ", "  ")
	}
	return lines, textOf(gapped, sample.Sheet{Width: EssayWidth, Height: EssayHeight}, seed, e.Number)
}
