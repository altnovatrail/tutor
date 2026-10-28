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
	"github.com/altnovatrail/tutor/internal/reading"
	"github.com/altnovatrail/tutor/internal/report"
	"github.com/altnovatrail/tutor/internal/runner"
)

// runReport turns the log of a run into its report, asking no model, by the kind of call the log
// is of (T67). Of the judge, whose log tells no kind (R24): the criterion and the numbers of
// architecture 3.1 in Markdown, and the answers by judge and form in CSV. Of the reading (T71): the
// threshold of R03 a reader each, the readings by reader and condition and the rule of two readers
// in Markdown, and the readings in CSV. Both go next to the log under its name, as new files. A
// report holds no handwriting: numbers, sheets by their sets, numbers and scenarios, and the names
// of judges, forms, readers and conditions.
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
	data, err := os.ReadFile(path)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval report: %v\n", err)
		return exitFail
	}
	r, err := reportOf(data)
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
	b.WriteString(r.Summary())
	fmt.Fprintf(&b, "The report: %s and %s\n", outputs[0].path, outputs[1].path)
	if _, err := io.WriteString(stdout, b.String()); err != nil {
		return exitFail
	}
	return exitOK
}

// written is a report as the command writes it: its Markdown, its CSV and what it says on the
// terminal.
type written interface {
	Markdown(w io.Writer, logName string) error
	CSV(w io.Writer) error
	Summary() string
}

// reportOf builds the report of a log by the kind of call it is of: the judge's, whose log tells no
// kind, or the reading's (T71).
func reportOf(data []byte) (written, error) {
	switch kind := runner.CallKind(data); kind {
	case "":
		l, err := measure.ReadLog(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		r, err := report.New(l)
		if err != nil {
			return nil, err
		}
		return judgeReport{r}, nil
	case reading.CallKind:
		l, err := reading.ReadLog(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		return report.NewReading(l)
	default:
		return nil, fmt.Errorf("a log of the kind of call %q, and the report knows the judge and the reading", kind)
	}
}

// judgeReport is the judge's report with what it says on the terminal: each judge's criterion and
// whether the writing process helps.
type judgeReport struct{ *report.Report }

func (r judgeReport) Summary() string {
	var b strings.Builder
	for _, c := range r.Main {
		fmt.Fprintf(&b, "%s: %s; D against A, Σd %+.2f over %d sheets with d ≠ 0, p %.4g.\n", c.Judge, c.Outcome, c.Sum, c.Nonzero, c.Helps)
	}
	if r.Helps {
		b.WriteString("The writing process helps Claude.\n")
	} else {
		b.WriteString("That the writing process helps Claude is not shown.\n")
	}
	return b.String()
}
