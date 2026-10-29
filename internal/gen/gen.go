// Package gen builds the synthetic sets of the extended measurement (T94; R50 in docs/decisions.md):
// pages of the workbook in the format tutor-workbook and essays in the format tutor-essays, as many
// as a run needs, each with its truth by construction. A page's numbers are drawn at random, and
// every box is written as its scenario says; an essay is put together from sentences written here,
// and each criterion it does not meet comes from a change made to it on purpose. No model writes a
// word of them, so that the program, and not a model's word, holds whether an essay keeps to its
// topic and spells its words right. The same seed builds the same set again, on any machine.
package gen

import (
	"math/rand/v2"

	"github.com/altnovatrail/tutor/internal/sample"
)

// The streams of the random numbers of the two sets, so that a page and an essay of the same seed
// draw their numbers apart.
const (
	workbookStream = 94
	essaysStream   = 9400
)

// newRand is the source of the random numbers of a set, from the seed, as synth draws its own.
func newRand(seed, stream uint64) *rand.Rand { return rand.New(rand.NewPCG(seed, stream)) }

// devCount is how many of n pages or essays go to the development: one in fifteen, rounded up, and
// the rest to the test. E4 gave a quarter to the development, but a run of hundreds needs few to
// try a prompt on, and every one more in the test narrows the bound of its measures.
func devCount(n int) int { return (n + 14) / 15 }

// partOf is the part of the measurement of the i-th of n pages or essays, from 0.
func partOf(i, n int) string {
	if i < devCount(n) {
		return sample.PartDev
	}
	return sample.PartTest
}
