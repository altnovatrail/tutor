package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/workbook"
)

// workbookFile is the file of the pages of the workbook (T78).
const workbookFile = "eval/workbook.json"

// runWorkbook checks the pages of the workbook, reads the samples of its pages that tutor-eval draw
// drew, held to the rules of sections 11 and 12 and refused as the runner refuses them (R71), and
// prints their summary: the pages, the items and the boxes by scenario, kind and writing of a
// fraction, the samples read, refused and off their file, and the crops of the filled boxes of the
// pages read at each height of eval, cut from their samples' strokes.
func runWorkbook(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("tutor-eval workbook", flag.ContinueOnError)
	file := flags.String("file", workbookFile, "the pages of the workbook")
	dirFlag := flags.String("dir", "", "the directory of the samples of the pages, which tutor-eval draw -workbook draws: in synthetic, since sample-check reads the repository's paths alone; by default synthetic/ and the file's name")
	if code, ok := parse(flags, args, stderr); !ok {
		return code
	}
	dir, err := drawnDirOf(*dirFlag, *file, given(flags)["dir"])
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval workbook: %v\n", err)
		return exitUsage
	}
	w, d, seed, err := readPages(*file, dir, checkSamples)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval workbook: %v\n", err)
		return exitFail
	}
	var out bytes.Buffer
	fmt.Fprintf(&out, "%s: %s; %s.\n", *file, workbookSize(w.Workbook), seedWords(seed, d.Seeds()))
	fmt.Fprintf(&out, "Boxes by scenario: %s.\n", workbookTally(w.Workbook))
	out.WriteString(drawnLine(d, "page", dir))
	tallies := make(map[int]*cropTally)
	for _, x := range d.Drawn {
		if x.Read == nil {
			continue
		}
		for _, digit := range render.Heights() {
			cells, err := workbook.CellsOf(x.Read.Ink, x.Read.Page, digit)
			if err != nil {
				_, _ = fmt.Fprintf(stderr, "tutor-eval workbook: page %d: %v\n", x.Number, err)
				return exitFail
			}
			if tallies[digit] == nil {
				tallies[digit] = &cropTally{}
			}
			tallies[digit].add(cells)
		}
	}
	for _, digit := range render.Heights() {
		if tallies[digit] != nil {
			fmt.Fprintf(&out, "Crops at %d px: %s.\n", digit, tallies[digit])
		}
	}
	if _, err := stdout.Write(out.Bytes()); err != nil {
		return exitFail
	}
	return exitOK
}

// seedWords says which seed the samples of a directory were drawn with, or that none is drawn.
func seedWords(seed uint64, seeds []string) string {
	if len(seeds) == 0 {
		return "no sample drawn"
	}
	return fmt.Sprintf("seed %d", seed)
}

// cropsDir refuses a directory for the crops of the pages or of the essays that draw draws, that is
// not named, or that lies in the work tree outside synthetic: the crops are of code's writing, and
// the samples folder is for the author's.
func cropsDir(dir string) error {
	if dir == "" {
		return errors.New("-png is empty: name the directory of the crops")
	}
	return checkIgnored(dir, syntheticFolder)
}

// workbookSize says how large the workbook is: its pages, by part, its items and its boxes.
func workbookSize(wb *sample.Workbook) string {
	dev, items, boxes := 0, 0, 0
	for _, pg := range wb.Pages {
		if pg.Part == sample.PartDev {
			dev++
		}
		items += len(pg.Items)
		for _, it := range pg.Items {
			boxes += len(it.Boxes)
		}
	}
	return fmt.Sprintf("%s, %d of them %s and %d %s; %s, %s",
		count(len(wb.Pages), "page"), dev, sample.PartDev, len(wb.Pages)-dev, sample.PartTest, count(items, "item"), boxesCount(boxes))
}

// boxesCount writes a number of boxes, in the singular for one.
func boxesCount(n int) string {
	if n == 1 {
		return "1 box"
	}
	return fmt.Sprintf("%d boxes", n)
}

// workbookTally counts the boxes of each scenario, in the order of the scenarios, each with its
// boxes of a number and of a fraction, and of the fractions written, those stacked and those inline.
func workbookTally(wb *sample.Workbook) string {
	type tally struct{ all, number, fraction, stacked, inline int }
	by := map[sample.BoxScenario]*tally{}
	for _, pg := range wb.Pages {
		for _, it := range pg.Items {
			for _, b := range it.Boxes {
				t := by[b.Scenario]
				if t == nil {
					t = &tally{}
					by[b.Scenario] = t
				}
				t.all++
				if b.Check.Kind == sample.KindNumber {
					t.number++
				} else {
					t.fraction++
				}
				switch {
				case b.Fraction == nil:
				case *b.Fraction == sample.Stacked:
					t.stacked++
				default:
					t.inline++
				}
			}
		}
	}
	var parts []string
	for _, s := range sample.BoxScenarios {
		if t := by[s]; t != nil {
			parts = append(parts, fmt.Sprintf("%s %d (number %d, fraction %d: stacked %d, inline %d)", s, t.all, t.number, t.fraction, t.stacked, t.inline))
		}
	}
	return strings.Join(parts, "; ")
}

// cropTally is what the crops of a height come to: how many, how many are larger than the least
// crop, render.CropSide square, their widest and tallest sides, and the tokens of a page's crops.
type cropTally struct {
	crops, larger, width, height int
	pages, tokens, mostTokens    int
}

func (c *cropTally) add(cells []workbook.Cell) {
	tokens := 0
	for _, cell := range cells {
		c.crops++
		if cell.Crop.Width > render.CropSide || cell.Crop.Height > render.CropSide {
			c.larger++
		}
		c.width, c.height = max(c.width, cell.Crop.Width), max(c.height, cell.Crop.Height)
		tokens += render.Tokens(cell.Crop.Width, cell.Crop.Height)
	}
	c.pages++
	c.tokens += tokens
	c.mostTokens = max(c.mostTokens, tokens)
}

func (c *cropTally) String() string {
	mean := 0.0
	if c.pages > 0 {
		mean = float64(c.tokens) / float64(c.pages)
	}
	return fmt.Sprintf("%s, %d of them larger than %d × %d, at most %d px wide and %d px high; %.0f tokens of crops a page, at most %d",
		count(c.crops, "crop"), c.larger, render.CropSide, render.CropSide, c.width, c.height, mean, c.mostTokens)
}
