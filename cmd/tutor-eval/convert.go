package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/altnovatrail/tutor/internal/convert"
	"github.com/altnovatrail/tutor/internal/evalsample"
	"github.com/altnovatrail/tutor/internal/newfile"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/sheets"
	"github.com/altnovatrail/tutor/internal/synth"
)

// writerCodeFile is where the author's writer's code is kept by default, beside the samples it links
// and out of git, as the record mode and the converter make and read it (R75).
const writerCodeFile = samplesFolder + "/writer-code"

// pending is the prefix of a converted file before sample-check has passed it: no sheet's name
// begins so, and a run cut short leaves nothing the directory of sheets reads.
const pending = ".converting-"

// runConvert writes every sheet of tutor-sample v0 in a directory, a saved sheet of its plan, as a
// sample of spec's eval protocol in a new directory, under the same name (convert.Sheet; R75): the
// author's as a person's, with their writer's code, and the sheets code drew as tutor-eval draw
// draws them from the seed. A sheet the new directory holds already is kept as it is, so a second
// run writes nothing; the split is copied once, and it names the same files. The old files are not
// touched. Nothing of the author's goes from samples to synthetic or anywhere else (R05, R28). Each
// sample is held to the rules of sections 11 and 12 by sample-check before it gets its name, when
// the new directory is in the work tree. The summary names each sheet by its number, its writer, its
// pointer and its outcome, and nothing of the handwriting.
func runConvert(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("tutor-eval convert", flag.ContinueOnError)
	from := flags.String("from", "", "the directory of the sheets of tutor-sample v0, in samples or in synthetic")
	to := flags.String("to", "", "the new directory of their samples of the protocol, by default -from and -v1")
	bankPath := flags.String("bank", "eval/bank.json", "the bank of tasks")
	planPath := flags.String("plan", "eval/sheets.json", "the sheet plan")
	seed := flags.Uint64("seed", 1, "the number the sheets drawn by code were drawn from")
	shift := flags.Float64("shift", synth.SimilarShift, "the share of the shift the sheets of the scenario similar-digits were drawn at")
	codeFile := flags.String("writer-code", writerCodeFile, "the file of the author's writer's code, made at random when it is not there; by default "+writerCodeFile+" for a directory in samples, and beside -to otherwise")
	if code, ok := parse(flags, args, stderr); !ok {
		return code
	}
	if *to == "" {
		*to = strings.TrimRight(*from, `/`) + "-v1"
	}
	set := given(flags)
	*codeFile = writerCodeOf(*to, *codeFile, set["writer-code"])
	if err := convertDirs(*from, *to, *codeFile); err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval convert: %v\n", err)
		return exitUsage
	}
	bank, plan, err := readPlan(*bankPath, *planPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval convert: %v\n", err)
		return exitFail
	}
	code := ""
	o := convert.Options{Plan: *planPath, Seed: *seed, Shift: *shift, ID: evalsample.NewID, Code: func() (string, error) {
		if code == "" {
			c, err := evalsample.WriterCodeAt(*codeFile)
			if err != nil {
				return "", err
			}
			code = c
		}
		return code, nil
	}}
	out, failed, err := convertSheets(*from, *to, plan, bank, o)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval convert: %v\n", err)
		return exitFail
	}
	if _, err := io.WriteString(stdout, out); err != nil {
		return exitFail
	}
	if failed {
		return exitFail
	}
	return exitOK
}

// writerCodeOf is the file of the writer's code for the sheets of a directory: the one given, or by
// default the author's, samples/writer-code, for a directory in samples; none for one in synthetic,
// where no person's sheet goes; and a file beside the directory otherwise, so that the sheets of a
// test name no code of the author's and make none (R75).
func writerCodeOf(dir, file string, given bool) string {
	if given {
		return file
	}
	_, _, in, err := placeOf(dir)
	switch {
	case err == nil && in == samplesFolder:
		return writerCodeFile
	case err == nil && in == syntheticFolder:
		return ""
	}
	return filepath.Join(filepath.Dir(absolute(dir)), "writer-code")
}

// convertDirs refuses directories a conversion may not take: none named, the same twice, one in the
// work tree outside samples and synthetic, a new directory outside samples for the sheets in samples,
// which are the author's (R05), and a file of the writer's code outside samples in the work tree.
func convertDirs(from, to, codeFile string) error {
	if from == "" {
		return errors.New("-from is empty: name the directory of the sheets of tutor-sample v0")
	}
	for _, dir := range []string{from, to} {
		if err := checkIgnored(dir, samplesFolder, syntheticFolder); err != nil {
			return err
		}
	}
	if codeFile != "" {
		if err := checkIgnored(filepath.Dir(codeFile), samplesFolder); err != nil {
			return fmt.Errorf("-writer-code: %w", err)
		}
	}
	src, _, inFrom, err := placeOf(from)
	if err != nil {
		return err
	}
	dst, _, inTo, err := placeOf(to)
	if err != nil {
		return err
	}
	switch {
	case within(src, dst) || within(dst, src):
		return fmt.Errorf("-to %s and -from %s are one directory, or one is in the other: give the samples of the protocol a directory of their own", to, from)
	case inFrom == samplesFolder && inTo != samplesFolder:
		return fmt.Errorf("-from %s is in %s, the author's, and -to %s is not: nothing of the author's goes elsewhere", from, samplesFolder, to)
	}
	return nil
}

// convertSheets converts the sheets of from into to and says what it did, and whether a sheet was
// refused. A sheet a person wrote it never writes into synthetic, wherever it comes from (R28). The
// error is a directory that cannot be read or written, or a check that cannot run.
func convertSheets(from, to string, plan *sample.Plan, bank *evalsample.Bank, o convert.Options) (string, bool, error) {
	entries, err := os.ReadDir(from)
	if err != nil {
		return "", false, err
	}
	_, _, in, err := placeOf(to)
	if err != nil {
		return "", false, err
	}
	if in == syntheticFolder {
		o.Code = func() (string, error) {
			return "", fmt.Errorf("a person wrote it, and nothing of a person's goes into %s", syntheticFolder)
		}
	}
	if err := os.MkdirAll(to, 0o700); err != nil {
		return "", false, err
	}
	type sheet struct {
		name   string
		number int
		c      *convert.Converted
	}
	var made []sheet
	var kept, refused []string
	other := 0
	for _, e := range entries {
		n, ok := sample.ParseSheetFile(e.Name())
		if !ok || !e.Type().IsRegular() || n.Number < 1 || n.Number > len(plan.Sheets) || !n.Saves(&plan.Sheets[n.Number-1]) {
			if e.Name() != sheets.SplitFile {
				other++
			}
			continue
		}
		if _, err := os.Lstat(filepath.Join(to, e.Name())); err == nil {
			kept = append(kept, e.Name())
			continue
		}
		data, err := os.ReadFile(filepath.Join(from, e.Name()))
		if err != nil {
			return "", false, err
		}
		c, err := convert.Sheet(data, &plan.Sheets[n.Number-1], bank, o)
		if err != nil {
			refused = append(refused, fmt.Sprintf("sheet %d, %s: %v", n.Number, e.Name(), err))
			continue
		}
		made = append(made, sheet{e.Name(), n.Number, c})
	}

	// Each sample waits under a name no sheet has until sample-check has passed them all.
	var paths []string
	defer func() {
		for _, p := range paths {
			_ = os.Remove(p)
		}
	}()
	for _, s := range made {
		// A file pending from a run cut short is this command's own, and goes.
		p := filepath.Join(to, pending+s.name)
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return "", false, err
		}
		if err := newfile.Write(p, s.c.Data, 0o600); err != nil {
			return "", false, err
		}
		paths = append(paths, p)
	}
	checked := checkable(to)
	broken := map[string][]int{}
	if checked && len(paths) > 0 {
		if broken, err = checkSamples(paths); err != nil {
			return "", false, err
		}
	}
	var lines []string
	written := 0
	for i, s := range made {
		if at := broken[paths[i]]; len(at) > 0 {
			refused = append(refused, fmt.Sprintf("sheet %d, %s: sample-check finds its sample breaks a rule of sections 11 and 12, at lines %v", s.number, s.name, at))
			continue
		}
		if err := os.Link(paths[i], filepath.Join(to, s.name)); err != nil {
			return "", false, err
		}
		written++
		who := "written by the " + string(s.c.Writer)
		if s.c.Writer == sample.WriterSynthetic {
			who = "drawn by code"
		}
		lines = append(lines, fmt.Sprintf("Sheet %d: %s, by %s; %s by module-kit's verdict on its truth.", s.number, who, s.c.Pointer, s.c.Outcome))
	}

	split := ""
	if _, err := os.Lstat(filepath.Join(to, sheets.SplitFile)); errors.Is(err, fs.ErrNotExist) {
		data, err := os.ReadFile(filepath.Join(from, sheets.SplitFile))
		switch {
		case err == nil:
			if err := newfile.Write(filepath.Join(to, sheets.SplitFile), data, 0o600); err != nil {
				return "", false, err
			}
			split = "The split is copied: it names the same files.\n"
		case !errors.Is(err, fs.ErrNotExist):
			return "", false, err
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Converted: %s, into %s.\n", count(written, "sheet"), to)
	for _, l := range lines {
		b.WriteString(l + "\n")
	}
	if len(kept) > 0 {
		fmt.Fprintf(&b, "Kept as they were, already in %s: %s.\n", to, count(len(kept), "sheet"))
	}
	if other > 0 {
		fmt.Fprintf(&b, "Not read: %s, no saved sheet of this plan.\n", count(other, "other file"))
	}
	b.WriteString(split)
	if !checked && written > 0 {
		fmt.Fprintf(&b, "Not held to the rules by sample-check: %s is out of the work tree, and the check takes the repository's paths.\n", to)
	}
	for _, r := range refused {
		fmt.Fprintf(&b, "Refused, and not written: %s.\n", r)
	}
	return b.String(), len(refused) > 0, nil
}

// checkable reports whether sample-check can hold the files of a directory: whether it lies in the
// work tree, whose paths the check takes.
func checkable(dir string) bool {
	_, root, _, err := placeOf(dir)
	return err == nil && root != ""
}
