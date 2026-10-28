package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/altnovatrail/tutor/internal/essay"
	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/sample"
)

// essaysFile is the file of the essays (T82).
const essaysFile = "eval/essays.json"

// runEssays checks the essays, draws them in memory and prints their summary: the essays and their
// parts, their words, sentences and lines, the criteria each part leaves unmet, and the crops of the
// pages at 32 px. With -png it saves the crop of every page at 32 px, what a model sees, in a
// directory of synthetic or out of the work tree.
func runEssays(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("tutor-eval essays", flag.ContinueOnError)
	file := flags.String("file", essaysFile, "the essays")
	seed := flags.Uint64("seed", 1, "the number the hands of the essays come from")
	pngDir := flags.String("png", "", "also save the crop of every page at 32 px, as PNG, in this directory: in synthetic, or out of the work tree")
	if code, ok := parse(flags, args, stderr); !ok {
		return code
	}
	pictured := given(flags)["png"]
	if pictured {
		if err := cropsDir(*pngDir); err != nil {
			_, _ = fmt.Fprintf(stderr, "tutor-eval essays: %v\n", err)
			return exitUsage
		}
	}
	es, err := readFile(*file, sample.ReadEssays)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval essays: %v\n", err)
		return exitFail
	}
	var out bytes.Buffer
	fmt.Fprintf(&out, "%s: %s; seed %d.\n", *file, essaysSize(es), *seed)
	fmt.Fprintf(&out, "Unmet criteria, dev and test: %s.\n", essaysTally(es))
	var words, sentences, lines, drawn, widths, heights, tokens span
	for i := range es.Essays {
		e := &es.Essays[i]
		pic, err := essay.PictureOf(e, *seed, render.Digit)
		if err == nil && pictured {
			err = saveEssay(*pngDir, e.Number, pic.PNG)
		}
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "tutor-eval essays: essay %d: %v\n", e.Number, err)
			return exitFail
		}
		words.add(len(sample.Words(e.Text)))
		sentences.add(len(sample.Sentences(e.Text)))
		lines.add(len(pic.Lines))
		drawn.add(pic.Crop.Drawn)
		widths.add(pic.Crop.Width)
		heights.add(pic.Crop.Height)
		tokens.add(render.Tokens(pic.Crop.Width, pic.Crop.Height))
	}
	fmt.Fprintf(&out, "Words %s; sentences %s; lines %s.\n", words.in(""), sentences.in(""), lines.in(""))
	fmt.Fprintf(&out, "Crops at %d px: digits drawn %s; width %s; height %s; tokens %s.\n", render.Digit, drawn.in(" px"), widths.in(" px"), heights.in(" px"), tokens.in(""))
	if pictured {
		fmt.Fprintf(&out, "Crops of the pages at %d px: %s, into %s.\n", render.Digit, count(len(es.Essays), "file"), *pngDir)
	}
	if _, err := stdout.Write(out.Bytes()); err != nil {
		return exitFail
	}
	return exitOK
}

// saveEssay saves the crop of an essay's page as essay-NN.png, over a crop saved before: the crops
// are drawn by code, the same from the same seed.
func saveEssay(dir string, number int, png []byte) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, fmt.Sprintf("essay-%02d.png", number)), png, 0o600)
}

// essaysSize says how many essays there are, by part.
func essaysSize(es *sample.Essays) string {
	dev := 0
	for _, e := range es.Essays {
		if e.Part == sample.PartDev {
			dev++
		}
	}
	return fmt.Sprintf("%s, %d of them %s and %d %s", count(len(es.Essays), "essay"), dev, sample.PartDev, len(es.Essays)-dev, sample.PartTest)
}

// essaysTally counts, for each criterion of the rubric, the essays of each part that do not meet
// it, and the essays that meet them all.
func essaysTally(es *sample.Essays) string {
	unmet := es.Tally()
	parts := make([]string, 0, len(sample.Criteria)+1)
	for _, id := range sample.Criteria {
		parts = append(parts, fmt.Sprintf("%s %d and %d", id, unmet[id][0], unmet[id][1]))
	}
	parts = append(parts, fmt.Sprintf("none %d and %d", unmet[""][0], unmet[""][1]))
	return strings.Join(parts, "; ")
}

// span is the least, the mean and the most of whole numbers.
type span struct{ n, sum, lo, hi int }

func (s *span) add(v int) {
	if s.n == 0 || v < s.lo {
		s.lo = v
	}
	s.hi = max(s.hi, v)
	s.n++
	s.sum += v
}

// in writes the span in a unit, such as " px": "23 to 32 px, 27.4 on average".
func (s span) in(unit string) string {
	mean := 0.0
	if s.n > 0 {
		mean = float64(s.sum) / float64(s.n)
	}
	return fmt.Sprintf("%d to %d%s, %.1f on average", s.lo, s.hi, unit, mean)
}
