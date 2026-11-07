package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"

	"github.com/altnovatrail/tutor/internal/essay"
	"github.com/altnovatrail/tutor/internal/evalsample"
	"github.com/altnovatrail/tutor/internal/newfile"
	"github.com/altnovatrail/tutor/internal/protocol"
	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/samplecheck"
	"github.com/altnovatrail/tutor/internal/sheets"
	"github.com/altnovatrail/tutor/internal/synth"
	"github.com/altnovatrail/tutor/internal/workbook"
)

// The pages of the workbook and the essays are samples of spec's eval protocol that tutor-eval draw
// draws, a file to a page or an essay, into a directory of their own (R71): page-NN.json and
// essay-NN.json, each of the task of its number. The summaries and the runs read them there, and
// take from the file of the workbook or of the essays what the protocol does not hold: a box's
// scenario, the values written and crossed out, how a fraction is written, the words spelled wrong
// and the part.

// drawnDir is the directory of the samples of a file of the workbook or of the essays by default:
// the folder of its name in synthetic, synthetic/workbook for eval/workbook.json.
func drawnDir(file string) string {
	return filepath.Join(syntheticFolder, strings.TrimSuffix(filepath.Base(file), filepath.Ext(file)))
}

// drawnDirOf is the directory of the samples a command is given, or by default its file's
// (drawnDir); it refuses an empty one, and one in the work tree outside synthetic: the samples are of
// code's writing, and the samples folder is for the author's. draw may write one out of the work
// tree, but the summaries and the runs read one in synthetic alone, since sample-check, which they
// hold the samples to, takes the repository's paths alone (section 11 of spec's text of the
// contracts), and says so.
func drawnDirOf(dir, file string, set bool) (string, error) {
	if !set {
		dir = drawnDir(file)
	}
	if dir == "" {
		return "", errors.New("-dir is empty: name the directory of the samples, or leave the flag out for " + drawnDir(file))
	}
	return dir, checkIgnored(dir, syntheticFolder)
}

// seedOf is the one seed the samples of a directory were drawn from, or 0 when it holds none drawn
// yet. Samples of two seeds in one directory are an error: nothing tells a run of them apart, so a
// seed draws into a directory of its own.
func seedOf(seeds []string, dir string) (uint64, error) {
	switch len(seeds) {
	case 0:
		return 0, nil
	case 1:
		return strconv.ParseUint(seeds[0], 10, 64)
	}
	return 0, fmt.Errorf("%s holds samples drawn with the seeds %s: a directory holds the samples of one seed", dir, strings.Join(seeds, ", "))
}

// readPages reads the file of the workbook and the directory of its pages' samples, held to the
// rules of sections 11 and 12 by check, and the seed they were drawn from.
func readPages(file, dir string, check samplecheck.Check) (*evalsample.Workbook, *evalsample.Dir[evalsample.Page], uint64, error) {
	w, err := evalsample.ReadWorkbook(file)
	if err != nil {
		return nil, nil, 0, err
	}
	d, err := evalsample.ReadPagesDir(dir, w, check)
	if err != nil {
		return nil, nil, 0, err
	}
	seed, err := seedOf(d.Seeds(), dir)
	return w, d, seed, err
}

// readEssays reads the file of the essays and the directory of their samples, as readPages reads
// the workbook's.
func readEssays(file, dir string, check samplecheck.Check) (*evalsample.Essays, *evalsample.Dir[evalsample.Essay], uint64, error) {
	f, err := evalsample.ReadEssays(file)
	if err != nil {
		return nil, nil, 0, err
	}
	d, err := evalsample.ReadEssaysDir(dir, f, check)
	if err != nil {
		return nil, nil, 0, err
	}
	seed, err := seedOf(d.Seeds(), dir)
	return f, d, seed, err
}

// saveDrawn saves a sample drawn and the PNG files of its crops together or not at all: the pictures
// first and the sample last, so that a draw cut short leaves no sample without them, and the
// next draws again what this one did not save.
//
// A crop is saved over one of the same name, as code draws the same crop from the same seed again,
// and only the crops that were not there before are taken back.
func saveDrawn(path string, data []byte, pngDir string, pics []picture) error {
	var written []string
	var err error
	for _, p := range pics {
		at := filepath.Join(pngDir, p.name)
		_, there := os.Lstat(at)
		if err = os.WriteFile(at, p.data, 0o600); err != nil {
			break
		}
		if there != nil {
			written = append(written, at)
		}
	}
	if err == nil {
		err = newfile.Write(path, data, 0o600)
	}
	if err != nil {
		for _, at := range written {
			_ = os.Remove(at)
		}
	}
	return err
}

// drawFlags are what draw is given for the pages of the workbook or the essays: the file, the
// directory of the samples, the seed, and the directory of the crops, or "" for none.
type drawFlags struct {
	file, dir string
	seed      uint64
	pngDir    string
}

// drawPages draws, with internal/synth, every page of the workbook whose sample the directory does
// not hold yet, from the seed, and saves it as page-NN.json, a sample of spec's eval protocol drawn
// by code (evalsample.OfPage), each read back as the runner reads it. With a directory of crops it
// saves the crop of every filled box of each page drawn at 32 px, what a reading sees, as
// page-NN-item-NN-box-N.png.
func drawPages(f drawFlags, stdout, stderr io.Writer) int {
	w, d, code := drawInto(f, stderr, evalsample.ReadWorkbook, func(dir string, w *evalsample.Workbook) (*evalsample.Dir[evalsample.Page], error) {
		return evalsample.ReadPagesDir(dir, w, nil)
	})
	if code != exitOK {
		return code
	}
	var drawn []int
	saved := 0
	for i := range w.Workbook.Pages {
		pg := &w.Workbook.Pages[i]
		if d.Drawn[i].File != "" {
			continue
		}
		data, read, err := drawPage(pg, w, f)
		var pics []picture
		if err == nil && f.pngDir != "" {
			pics, err = pagePictures(read)
		}
		if err == nil {
			err = saveDrawn(filepath.Join(f.dir, evalsample.PageFile(pg.Number)), data, f.pngDir, pics)
		}
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "tutor-eval draw: page %d is not saved, and nothing more is: %v\n", pg.Number, err)
			return exitFail
		}
		saved += len(pics)
		drawn = append(drawn, pg.Number)
	}
	return drawnSaid(stdout, "page", drawn, d, f, saved, "Crops of the filled boxes")
}

// drawInto reads the file draw draws, makes the directory of its samples when it is not there, and
// reads what the directory holds, without the check: draw reads it only to see which samples it
// holds. It refuses a seed other than the one the samples there were drawn with, since nothing in a
// run would tell the samples of two seeds apart, and only then makes the directory of the crops.
// Nothing is made for a file that cannot be read.
func drawInto[F, T any](f drawFlags, stderr io.Writer, readFile func(string) (F, error), readDir func(string, F) (*evalsample.Dir[T], error)) (F, *evalsample.Dir[T], int) {
	file, err := readFile(f.file)
	var d *evalsample.Dir[T]
	if err == nil {
		err = os.MkdirAll(f.dir, 0o700)
	}
	if err == nil {
		d, err = readDir(f.dir, file)
	}
	var seed uint64
	if err == nil {
		seed, err = seedOf(d.Seeds(), f.dir)
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval draw: %v\n", err)
		return file, nil, exitFail
	}
	if len(d.Seeds()) > 0 && seed != f.seed {
		_, _ = fmt.Fprintf(stderr, "tutor-eval draw: %s holds samples drawn with the seed %d: draw -seed %d into a directory of its own\n", f.dir, seed, f.seed)
		return file, nil, exitUsage
	}
	if f.pngDir != "" {
		if err := os.MkdirAll(f.pngDir, 0o700); err != nil {
			_, _ = fmt.Fprintf(stderr, "tutor-eval draw: %v\n", err)
			return file, nil, exitFail
		}
	}
	return file, d, exitOK
}

// drawPage draws a page from the seed and writes it as a sample of the protocol, its ID of the file's
// name: the file's bytes, and the page as the runner reads it back from them (drawnBack).
func drawPage(pg *sample.WorkbookPage, w *evalsample.Workbook, f drawFlags) ([]byte, *evalsample.Page, error) {
	ink, err := synth.PageOf(pg, f.seed)
	if err != nil {
		return nil, nil, err
	}
	id, err := evalsample.ID(f.file, f.seed, synth.SimilarShift, pg.Number)
	if err != nil {
		return nil, nil, err
	}
	es, err := evalsample.OfPage(ink, pg, w, f.seed, id)
	if err != nil {
		return nil, nil, err
	}
	return drawnBack(es,
		func(back *evalv1.Sample) (*evalsample.Page, *evalsample.Refusal, error) {
			return evalsample.ReadPage(back, w)
		},
		func(back *evalv1.Sample) []string { return evalsample.PageDifferences(back, pg) })
}

// drawnBack writes a drawn sample and reads its bytes back as the runner will, refusals and all,
// and holds what it reads to its file: a sample the runner would refuse, or that is off its file, is a
// fault of the drawing, never saved. The rules of sections 11 and 12 are sample-check's, which the
// tests and CI run on drawn files.
func drawnBack[T any](es *evalv1.Sample, read func(*evalv1.Sample) (*T, *evalsample.Refusal, error), off func(*evalv1.Sample) []string) ([]byte, *T, error) {
	data, err := protocol.MarshalSample(es)
	if err != nil {
		return nil, nil, err
	}
	back, err := protocol.ReadSample(bytes.NewReader(data))
	if err != nil {
		return nil, nil, err
	}
	got, refused, err := read(back)
	switch {
	case err != nil:
		return nil, nil, err
	case refused != nil:
		return nil, nil, fmt.Errorf("the runner would refuse it: %w", refused)
	}
	if at := off(back); len(at) > 0 {
		return nil, nil, fmt.Errorf("read back, it is off its file at %s", strings.Join(at, ", "))
	}
	return data, got, nil
}

// pagePictures are the crops of the filled boxes of a page read back, at 32 px.
func pagePictures(p *evalsample.Page) ([]picture, error) {
	cells, err := workbook.CellsOf(p.Ink, p.Page, render.Digit)
	if err != nil {
		return nil, err
	}
	pics := make([]picture, len(cells))
	for i, c := range cells {
		pics[i] = picture{fmt.Sprintf("page-%02d-item-%02d-box-%d.png", p.Page.Number, c.Item, c.Slot), c.PNG}
	}
	return pics, nil
}

// drawEssays draws, with internal/synth, every essay whose sample the directory does not hold yet,
// from the seed, and saves it as essay-NN.json (evalsample.OfEssay), each read back as the runner
// reads it. With a directory of crops it saves the crop of each essay drawn at 32 px, what a model
// sees, as essay-NN.png.
func drawEssays(f drawFlags, stdout, stderr io.Writer) int {
	es, d, code := drawInto(f, stderr, evalsample.ReadEssays, func(dir string, es *evalsample.Essays) (*evalsample.Dir[evalsample.Essay], error) {
		return evalsample.ReadEssaysDir(dir, es, nil)
	})
	if code != exitOK {
		return code
	}
	var drawn []int
	saved := 0
	for i := range es.Essays.Essays {
		e := &es.Essays.Essays[i]
		if d.Drawn[i].File != "" {
			continue
		}
		data, read, err := drawEssay(e, es, f)
		var pics []picture
		if err == nil && f.pngDir != "" {
			var pic *essay.Picture
			if pic, err = essay.PictureFrom(synth.EssayLinesOf(e, f.seed), read.Strokes, render.Digit); err == nil {
				pics = []picture{{fmt.Sprintf("essay-%02d.png", e.Number), pic.PNG}}
			}
		}
		if err == nil {
			err = saveDrawn(filepath.Join(f.dir, evalsample.EssayFile(e.Number)), data, f.pngDir, pics)
		}
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "tutor-eval draw: essay %d is not saved, and nothing more is: %v\n", e.Number, err)
			return exitFail
		}
		saved += len(pics)
		drawn = append(drawn, e.Number)
	}
	return drawnSaid(stdout, "essay", drawn, d, f, saved, "Crops of the essays")
}

// drawEssay draws an essay from the seed and writes it as a sample of the protocol, as drawPage
// draws a page.
func drawEssay(e *sample.Essay, es *evalsample.Essays, f drawFlags) ([]byte, *evalsample.Essay, error) {
	id, err := evalsample.ID(f.file, f.seed, synth.SimilarShift, e.Number)
	if err != nil {
		return nil, nil, err
	}
	s, err := evalsample.OfEssay(e, es, f.seed, id)
	if err != nil {
		return nil, nil, err
	}
	return drawnBack(s,
		func(back *evalv1.Sample) (*evalsample.Essay, *evalsample.Refusal, error) {
			return evalsample.ReadEssay(back, es)
		},
		func(back *evalv1.Sample) []string { return evalsample.EssayDifferences(back, es, e) })
}

// drawnSaid says what draw drew and kept, of the directory it read before, and the crops it saved.
// A sample kept that the runner refuses, or finds off its file — one of a file since changed, whose
// digest is another now —, draw does not draw again, and names it.
func drawnSaid[T any](stdout io.Writer, noun string, drawn []int, d *evalsample.Dir[T], f drawFlags, saved int, crops string) int {
	var kept, unread []int
	for _, x := range d.Drawn {
		if x.File != "" {
			kept = append(kept, x.Number)
			if x.Read == nil {
				unread = append(unread, x.Number)
			}
		}
	}
	out := fmt.Sprintf("Drawn by code with the seed %d: %s, into %s.\nKept as they were: %s.\n", f.seed, listOf(noun, drawn), f.dir, listOf(noun, kept))
	if len(unread) > 0 {
		out += fmt.Sprintf("Of them the runner refuses, or finds off their file: %s. Remove them, and draw draws them anew; tutor-eval %s says why.\n", listOf(noun, unread), map[string]string{"page": "workbook", "essay": "essays"}[noun])
	}
	if f.pngDir != "" {
		out += fmt.Sprintf("%s at %d px: %s, into %s.\n", crops, render.Digit, count(saved, "file"), f.pngDir)
	}
	if _, err := io.WriteString(stdout, out); err != nil {
		return exitFail
	}
	return exitOK
}

// drawnLine says what a directory of samples holds against its file, without the handwriting: how
// many are drawn, and which pages or essays are not drawn, are refused, with the reason, or are off
// their file, by paths.
func drawnLine[T any](d *evalsample.Dir[T], noun, dir string) string {
	var missing []int
	var refused, off []string
	read := 0
	for _, x := range d.Drawn {
		switch {
		case x.File == "":
			missing = append(missing, x.Number)
		case x.Refused != nil:
			refused = append(refused, refusedOf(noun, x.Number, x.Refused))
		case x.Read == nil:
			off = append(off, fmt.Sprintf("%s %d (%s)", noun, x.Number, strings.Join(x.Problems, ", ")))
		default:
			read++
		}
	}
	none := func(list []string) string {
		if len(list) == 0 {
			return "none"
		}
		return strings.Join(list, "; ")
	}
	line := fmt.Sprintf("Samples in %s: %d of %d read; not drawn: %s; refused, and nothing of them goes to AI: %s; off their file: %s.\n",
		dir, read, len(d.Drawn), sheets.Numbers(missing), none(refused), none(off))
	if len(d.Other) > 0 {
		line += fmt.Sprintf("Not read, no sample of the file: %s.\n", strings.Join(d.Other, ", "))
	}
	return line
}

// refusedOf names a sample refused, by what it is, with its reason and in words what of it is
// refused: nothing of the handwriting.
func refusedOf(what string, n int, r *evalsample.Refusal) string {
	return fmt.Sprintf("%s %d (%s: %s)", what, n, evalsample.Reason(r.Reason), r.Detail)
}

// partDrawn are the samples read of the pages or essays of a part, by their numbers: a run asks of
// each that is drawn and fits its file. One refused is named in refused, with its reason, and not
// asked of, and nothing of it goes to AI; one not drawn, or off its file, stops the run before it
// asks, as a sheet not recorded or failing its check does.
func partDrawn[T any](d *evalsample.Dir[T], noun string, numbers []int) (chosen []*T, refused []string, err error) {
	at := map[int]int{}
	for i, x := range d.Drawn {
		at[x.Number] = i
	}
	for _, n := range numbers {
		x := d.Drawn[at[n]]
		switch {
		case x.File == "":
			return nil, nil, fmt.Errorf("%s %d is not drawn: tutor-eval draw draws it", noun, n)
		case x.Refused != nil:
			refused = append(refused, refusedOf(noun, n, x.Refused))
		case x.Read == nil:
			return nil, nil, fmt.Errorf("the sample of %s %d is off its file at %s", noun, n, strings.Join(x.Problems, ", "))
		default:
			chosen = append(chosen, x.Read)
		}
	}
	if len(chosen) == 0 {
		return nil, refused, fmt.Errorf("no %s of the part has a sample the runner reads", noun)
	}
	return chosen, refused, nil
}
