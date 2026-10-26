package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/altnovatrail/tutor/internal/measure"
	"github.com/altnovatrail/tutor/internal/newfile"
	"github.com/altnovatrail/tutor/internal/report"
)

// runReport turns the log of a run into its report, asking no model (R24): the criterion and the
// numbers of architecture 3.1 in Markdown, and the answers by judge and form in CSV, next to the
// log under its name, as new files. A report holds no handwriting: numbers, sheets by their
// numbers and scenarios, and the names of judges and forms.
func runReport(args []string, stdout, stderr io.Writer) int {
	const use = "usage: tutor-eval report LOG"
	switch {
	case len(args) == 1 && (args[0] == "-h" || args[0] == "-help" || args[0] == "--help"):
		_, _ = fmt.Fprintln(stderr, use)
		return exitOK
	case len(args) != 1 || strings.HasPrefix(args[0], "-"):
		_, _ = fmt.Fprintln(stderr, use)
		return exitUsage
	}
	path := args[0]
	f, err := os.Open(path)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval report: %v\n", err)
		return exitFail
	}
	l, err := measure.ReadLog(f)
	_ = f.Close()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval report: %s: %v\n", path, err)
		return exitFail
	}
	r, err := report.New(l)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval report: %s: %v\n", path, err)
		return exitFail
	}
	var md, table bytes.Buffer
	if err := r.Markdown(&md, filepath.Base(path)); err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval report: %v\n", err)
		return exitFail
	}
	if err := r.CSV(&table); err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval report: %v\n", err)
		return exitFail
	}
	base := strings.TrimSuffix(path, filepath.Ext(path))
	outputs := []struct {
		path string
		data []byte
	}{{base + ".md", md.Bytes()}, {base + ".csv", table.Bytes()}}
	for _, o := range outputs {
		if _, err := os.Lstat(o.path); err == nil {
			_, _ = fmt.Fprintf(stderr, "tutor-eval report: %s exists: a report is written once, beside its log\n", o.path)
			return exitFail
		}
	}
	for i, o := range outputs {
		if err := newfile.Write(o.path, o.data, 0o644); err != nil {
			// A report is the two files or none: the next try finds no half of it.
			for _, done := range outputs[:i] {
				_ = os.Remove(done.path)
			}
			_, _ = fmt.Fprintf(stderr, "tutor-eval report: %v\n", err)
			return exitFail
		}
	}
	var b strings.Builder
	for _, c := range r.Main {
		fmt.Fprintf(&b, "%s: %s; D against A, Σd %+.2f over %d sheets with d ≠ 0, p %.4g.\n", c.Judge, c.Outcome, c.Sum, c.Nonzero, c.Helps)
	}
	if r.Helps {
		b.WriteString("The writing process helps Claude.\n")
	} else {
		b.WriteString("That the writing process helps Claude is not shown.\n")
	}
	fmt.Fprintf(&b, "The report: %s and %s\n", outputs[0].path, outputs[1].path)
	if _, err := io.WriteString(stdout, b.String()); err != nil {
		return exitFail
	}
	return exitOK
}
