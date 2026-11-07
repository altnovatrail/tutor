package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/altnovatrail/tutor/internal/essay"
	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/synth"
)

// essaysFile is the file of the essays (T82).
const essaysFile = "eval/essays.json"

// runEssays checks the essays, reads the samples of them that tutor-eval draw drew, held to the rules
// of sections 11 and 12 and refused as the runner refuses them (R71), and prints their summary: the
// essays and their parts, the criteria each part leaves unmet, the samples read, refused and off
// their file, and of the essays read their words, sentences and lines and the crops of their pages
// at 32 px, cut from their samples' strokes.
func runEssays(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("tutor-eval essays", flag.ContinueOnError)
	file := flags.String("file", essaysFile, "the essays")
	dirFlag := flags.String("dir", "", "the directory of the samples of the essays, which tutor-eval draw -essays draws: in synthetic, since sample-check reads the repository's paths alone; by default synthetic/ and the file's name")
	if code, ok := parse(flags, args, stderr); !ok {
		return code
	}
	dir, err := drawnDirOf(*dirFlag, *file, given(flags)["dir"])
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval essays: %v\n", err)
		return exitUsage
	}
	f, d, seed, err := readEssays(*file, dir, checkSamples)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval essays: %v\n", err)
		return exitFail
	}
	es := f.Essays
	var out bytes.Buffer
	fmt.Fprintf(&out, "%s: %s; %s.\n", *file, essaysSize(es), seedWords(seed, d.Seeds()))
	fmt.Fprintf(&out, "Unmet criteria, dev and test: %s.\n", essaysTally(es))
	out.WriteString(drawnLine(d, "essay", dir))
	var words, sentences, lines, drawn, widths, heights, tokens span
	for _, x := range d.Drawn {
		if x.Read == nil {
			continue
		}
		e := x.Read.Essay
		pic, err := essay.PictureFrom(nil, x.Read.Strokes, render.Digit)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "tutor-eval essays: essay %d: %v\n", e.Number, err)
			return exitFail
		}
		words.add(len(sample.Words(e.Text)))
		sentences.add(len(sample.Sentences(e.Text)))
		// The lines an essay is written in are code's, from the seed of its sample; a person's essay
		// tells none.
		if own, err := strconv.ParseUint(x.Read.Seed, 10, 64); err == nil {
			lines.add(len(synth.EssayLinesOf(e, own)))
		}
		drawn.add(pic.Crop.Drawn)
		widths.add(pic.Crop.Width)
		heights.add(pic.Crop.Height)
		tokens.add(render.Tokens(pic.Crop.Width, pic.Crop.Height))
	}
	if words.n > 0 {
		fmt.Fprintf(&out, "Words %s; sentences %s; lines %s.\n", words.in(""), sentences.in(""), lines.in(""))
		fmt.Fprintf(&out, "Crops at %d px: digits drawn %s; width %s; height %s; tokens %s.\n", render.Digit, drawn.in(" px"), widths.in(" px"), heights.in(" px"), tokens.in(""))
	}
	if _, err := stdout.Write(out.Bytes()); err != nil {
		return exitFail
	}
	return exitOK
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
