package evalsample

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"

	"github.com/altnovatrail/tutor/internal/protocol"
	"github.com/altnovatrail/tutor/internal/samplecheck"
)

// PageFile is the name of the sample of a page of the workbook in its directory, page-01.json for the
// first: by the page's number in its file, which its task names too (R71).
func PageFile(number int) string { return fmt.Sprintf("page-%02d.json", number) }

// EssayFile is the name of the sample of an essay in its directory, essay-01.json for the first.
func EssayFile(number int) string { return fmt.Sprintf("essay-%02d.json", number) }

// drawnFile is the form of the name of a sample in a directory of pages or essays.
var drawnFile = regexp.MustCompile(`^(page|essay)-([0-9]{2,})\.json$`)

// Drawn is a page or an essay of the file as a directory of samples holds it: its number, and the
// name of its sample's file, or "" when the directory holds none. Read is the sample read, once it
// is refused for nothing and fits the file; Refused is why the runner refuses it, as section 13 has
// it; Problems are where it is off its file, by paths alone, nothing of the handwriting; and Seed is
// the seed it was drawn from, or "" for a person's.
type Drawn[T any] struct {
	Number   int
	File     string
	Read     *T
	Refused  *Refusal
	Problems []string
	Seed     string
}

// Dir is a directory of samples of pages or essays read against their file: each page or essay of
// the file in its order, and the names of the other files of the directory, which are not read.
type Dir[T any] struct {
	Drawn []Drawn[T]
	Other []string
}

// Seeds are the seeds the samples read were drawn from, each once, ascending as numbers: one, when
// one seed drew the directory.
func (d *Dir[T]) Seeds() []string {
	var out []string
	for _, x := range d.Drawn {
		if x.Seed != "" && !slices.Contains(out, x.Seed) {
			out = append(out, x.Seed)
		}
	}
	slices.SortFunc(out, func(a, b string) int {
		if len(a) != len(b) {
			return len(a) - len(b)
		}
		return strings.Compare(a, b)
	})
	return out
}

// ReadPagesDir reads a directory of samples of the pages of a workbook: the sample of each page is
// page-NN.json, its number's (PageFile), read as ReadPage reads it and held to its file
// (PageDifferences). A sample of the protocol is held to the rules of sections 11 and 12 by check,
// spec's code (samplecheck.Run), and refused when it breaks one; with no check, as tutor-eval draw
// reads the directory only to see which pages it holds, it is not. The error is a directory that
// cannot be read, or a check that cannot run.
func ReadPagesDir(dir string, w *Workbook, check samplecheck.Check) (*Dir[Page], error) {
	numbers := make([]int, len(w.Workbook.Pages))
	for i, pg := range w.Workbook.Pages {
		numbers[i] = pg.Number
	}
	return readDir(dir, "page", numbers, check, func(i int, es *evalv1.Sample) (*Page, *Refusal, []string, error) {
		p, r, err := ReadPage(es, w)
		switch {
		case errors.Is(err, ErrNotPage):
			return nil, nil, []string{"(the sample is no page of a workbook with its history)"}, nil
		case err != nil || r != nil:
			return nil, r, nil, err
		case p.Page.Number != numbers[i]:
			return nil, nil, []string{"task"}, nil
		}
		if off := PageDifferences(es, p.Page); len(off) > 0 {
			return nil, nil, off, nil
		}
		return p, nil, nil, nil
	})
}

// ReadEssaysDir reads a directory of samples of the essays of a file, as ReadPagesDir reads one of pages:
// the sample of each essay is essay-NN.json (EssayFile), read as ReadEssay reads it and held to its
// file (EssayDifferences).
func ReadEssaysDir(dir string, f *Essays, check samplecheck.Check) (*Dir[Essay], error) {
	numbers := make([]int, len(f.Essays.Essays))
	for i, e := range f.Essays.Essays {
		numbers[i] = e.Number
	}
	return readDir(dir, "essay", numbers, check, func(i int, es *evalv1.Sample) (*Essay, *Refusal, []string, error) {
		e, r, err := ReadEssay(es, f)
		switch {
		case errors.Is(err, ErrNotEssay):
			return nil, nil, []string{"(the sample is no essay on a sheet with its history)"}, nil
		case err != nil || r != nil:
			return nil, r, nil, err
		case e.Essay.Number != numbers[i]:
			return nil, nil, []string{"task"}, nil
		}
		if off := EssayDifferences(es, f, e.Essay); len(off) > 0 {
			return nil, nil, off, nil
		}
		return e, nil, nil, nil
	})
}

// readDir reads a directory of samples whose files are named by kind and by the numbers given, in
// their order, and reads each as read does, by its place among them.
func readDir[T any](dir, kind string, numbers []int, check samplecheck.Check, read func(int, *evalv1.Sample) (*T, *Refusal, []string, error)) (*Dir[T], error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	d := &Dir[T]{Drawn: make([]Drawn[T], len(numbers)), Other: []string{}}
	at := map[int]int{}
	for i, n := range numbers {
		d.Drawn[i].Number = n
		at[n] = i
	}
	for _, e := range entries {
		m := drawnFile.FindStringSubmatch(e.Name())
		n := -1
		if m != nil && m[1] == kind {
			n, _ = strconv.Atoi(m[2])
		}
		i, ok := at[n]
		if !ok || !e.Type().IsRegular() || e.Name() != fmt.Sprintf("%s-%02d.json", kind, n) {
			d.Other = append(d.Other, e.Name())
			continue
		}
		d.Drawn[i].File = e.Name()
	}
	var files []string
	for _, x := range d.Drawn {
		if x.File != "" {
			files = append(files, filepath.Join(dir, x.File))
		}
	}
	broken := map[string][]int{}
	if check != nil && len(files) > 0 {
		if broken, err = check(files); err != nil {
			return nil, err
		}
	}
	for i := range d.Drawn {
		x := &d.Drawn[i]
		if x.File == "" {
			continue
		}
		path := filepath.Join(dir, x.File)
		if lines := broken[path]; len(lines) > 0 {
			x.Refused = &Refusal{Reason: evalv1.Refusal_REFUSAL_INVALID, Detail: fmt.Sprintf("sample-check finds it breaks a rule of sections 11 and 12, at %s", lineList(lines))}
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			x.Problems = []string{"(the file cannot be read)"}
			continue
		}
		es, err := protocol.ReadSample(bytes.NewReader(data))
		if err != nil {
			x.Refused = &Refusal{Reason: evalv1.Refusal_REFUSAL_INVALID, Detail: "it cannot be read strictly as a sample of the protocol"}
			continue
		}
		x.Seed = es.GetSynthetic().GetSeed()
		if x.Read, x.Refused, x.Problems, err = read(i, es); err != nil {
			return nil, err
		}
	}
	return d, nil
}

// lineList names lines of a file.
func lineList(at []int) string {
	parts := make([]string, len(at))
	for i, n := range at {
		parts[i] = strconv.Itoa(n)
	}
	if len(at) == 1 {
		return "line " + parts[0]
	}
	return "lines " + strings.Join(parts, ", ")
}
