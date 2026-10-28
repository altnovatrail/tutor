package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/synth"
	"github.com/altnovatrail/tutor/internal/workbook"
)

// workbookFile is the file of the pages of the workbook (T78).
const workbookFile = "eval/workbook.json"

// runWorkbook checks the pages of the workbook, draws them in memory and prints their summary: the
// pages, the items and the boxes by scenario, kind and writing of a fraction, and the crops of the
// filled boxes at each height of eval. With -png it saves the crop of every filled box at 32 px,
// what a reading sees, in a directory of synthetic or out of the work tree.
func runWorkbook(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("tutor-eval workbook", flag.ContinueOnError)
	file := flags.String("file", workbookFile, "the pages of the workbook")
	seed := flags.Uint64("seed", 1, "the number the hands of the pages come from")
	pngDir := flags.String("png", "", "also save the crop of every filled box at 32 px, as PNG, in this directory: in synthetic, or out of the work tree")
	if code, ok := parse(flags, args, stderr); !ok {
		return code
	}
	pictured := given(flags)["png"]
	if pictured {
		if err := workbookPicturesDir(*pngDir); err != nil {
			_, _ = fmt.Fprintf(stderr, "tutor-eval workbook: %v\n", err)
			return exitUsage
		}
	}
	wb, err := readFile(*file, sample.ReadWorkbook)
	if err == nil {
		err = synth.CheckLayout(wb)
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval workbook: %v\n", err)
		return exitFail
	}
	var out bytes.Buffer
	fmt.Fprintf(&out, "%s: %s; seed %d.\n", *file, workbookSize(wb), *seed)
	fmt.Fprintf(&out, "Boxes by scenario: %s.\n", workbookTally(wb))
	saved := 0
	tallies := make(map[int]*cropTally)
	for i := range wb.Pages {
		pg := &wb.Pages[i]
		drawn, err := synth.PageOf(pg, *seed)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "tutor-eval workbook: %v\n", err)
			return exitFail
		}
		for _, digit := range render.Heights() {
			cells, err := workbook.CellsOf(drawn, pg, digit)
			if err == nil && pictured && digit == render.Digit {
				var n int
				n, err = saveCells(*pngDir, pg.Number, cells)
				saved += n
			}
			if err != nil {
				_, _ = fmt.Fprintf(stderr, "tutor-eval workbook: page %d: %v\n", pg.Number, err)
				return exitFail
			}
			if tallies[digit] == nil {
				tallies[digit] = &cropTally{}
			}
			tallies[digit].add(cells)
		}
	}
	for _, digit := range render.Heights() {
		fmt.Fprintf(&out, "Crops at %d px: %s.\n", digit, tallies[digit])
	}
	if pictured {
		fmt.Fprintf(&out, "Crops of the filled boxes at %d px: %s, into %s.\n", render.Digit, count(saved, "file"), *pngDir)
	}
	if _, err := stdout.Write(out.Bytes()); err != nil {
		return exitFail
	}
	return exitOK
}

// workbookPicturesDir refuses a directory for the crops that is not named, or that lies in the work
// tree outside synthetic: the crops are of code's writing, and the samples folder is for the
// author's.
func workbookPicturesDir(dir string) error {
	if dir == "" {
		return errors.New("-png is empty: name the directory of the crops")
	}
	return checkIgnored(dir, syntheticFolder)
}

// saveCells saves the crop of each filled box of a page as page-NN-item-NN-box-N.png, over a crop
// saved before, and returns how many it saved: the crops are drawn by code, the same from the same
// seed, so a second run saves them again.
func saveCells(dir string, page int, cells []workbook.Cell) (int, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return 0, err
	}
	for i, c := range cells {
		name := fmt.Sprintf("page-%02d-item-%02d-box-%d.png", page, c.Item, c.Slot)
		if err := os.WriteFile(filepath.Join(dir, name), c.PNG, 0o600); err != nil {
			return i, err
		}
	}
	return len(cells), nil
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
