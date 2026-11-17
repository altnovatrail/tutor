package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/altnovatrail/tutor/internal/protocol"
	"github.com/altnovatrail/tutor/internal/report"
)

// The kinds of report admit rules on: a sheet's, by the rule of R80, and a workbook's, by the rule of
// the workbook (R124).
const (
	admitSheet    = "sheet"
	admitWorkbook = "workbook"
)

// runAdmit applies the rule of admission (R80) to runners' reports and prints, for each, the stage
// it admits its readers to, or why it admits them to nothing, with the numbers the rule reads and the
// SHA-256 of the file, which an entry of the list of admitted models names. -kind workbook applies
// the rule of the workbook (R124) and prints the mirrored and the crossed-out boxes in rows of their
// own. It prints the IDs of no sample and nothing of any work, so that it may run on a report of the
// author's sheets.
func runAdmit(args []string, stdout, stderr io.Writer) int {
	const use = "usage: tutor-eval admit [-kind sheet|workbook] REPORT..."
	flags := flag.NewFlagSet("tutor-eval admit", flag.ContinueOnError)
	kind := flags.String("kind", admitSheet, "the kind of report: sheet, of the reading of sheets (R80), or workbook, of the reading of the pages of the workbook (R124)")
	flags.SetOutput(stderr)
	flags.Usage = func() {
		_, _ = fmt.Fprintln(stderr, use)
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage
	}
	paths := flags.Args()
	switch {
	case *kind != admitSheet && *kind != admitWorkbook:
		_, _ = fmt.Fprintf(stderr, "tutor-eval admit: -kind is %q: give sheet or workbook\n", *kind)
		return exitUsage
	case len(paths) == 0 || slices.ContainsFunc(paths, func(a string) bool { return strings.HasPrefix(a, "-") }):
		_, _ = fmt.Fprintln(stderr, use)
		return exitUsage
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "tutor-eval admit: %v\n", err)
			return exitFail
		}
		r, err := protocol.ReadReport(bytes.NewReader(data))
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "tutor-eval admit: %s: %v\n", path, err)
			return exitFail
		}
		sum := sha256.Sum256(data)
		file, hash := filepath.Base(path), hex.EncodeToString(sum[:])
		if *kind == admitWorkbook {
			writeWorkbookDecision(stdout, file, hash, report.AdmitWorkbook(r))
			continue
		}
		writeDecision(stdout, file, hash, report.Admit(r))
	}
	return exitOK
}

// writeDecision writes a decision of the rule as Markdown, a block ended by a blank line.
func writeDecision(w io.Writer, file, sum string, d report.Decision) {
	writeRows(w, file, sum, d)
	_, _ = fmt.Fprintln(w)
}

// writeRows writes the rows of a decision of the rule as Markdown: its head and, where it counted
// any, its shares and the measures of its readings.
func writeRows(w io.Writer, file, sum string, d report.Decision) {
	stage := string(d.Stage)
	if stage == "" {
		stage = "none: " + d.Why
	}
	_, _ = fmt.Fprintf(w, "## %s\n\n- stage: %s\n- readers: %s\n- set: %s, %d repeats\n- sha256: %s\n",
		file, stage, strings.Join(d.Readers, " + "), d.Set, d.Repeats, sum)
	if d.Measures == nil {
		return
	}
	share := func(name string, s report.SlotShare) {
		_, _ = fmt.Fprintf(w, "- %s: %d of %d answer slots, upper bound %.4f\n", name, s.Events, s.Of, s.Upper)
	}
	share("false alarms", d.Alarm)
	share("false correct", d.Correct)
	m := d.Measures.All
	unchecked := 0
	for _, n := range m.Unchecked {
		unchecked += n
	}
	_, _ = fmt.Fprintf(w, "- readings: %d; needless unreadable %d of %d; unchecked %d\n",
		m.Readings, m.Needless.Events, m.Needless.Of, unchecked)
}

// writeWorkbookDecision writes a decision of the rule of the workbook as Markdown: the decision on
// the boxes an adult writes, as writeDecision writes it, and the mirrored and the crossed-out boxes,
// which have no target.
func writeWorkbookDecision(w io.Writer, file, sum string, d report.WorkbookDecision) {
	writeRows(w, file, sum, d.Decision)
	if d.Measures == nil {
		_, _ = fmt.Fprintln(w)
		return
	}
	_, _ = fmt.Fprintln(w, "- the false alarms and false correct above are of the boxes an adult writes, the readings of every box; the rule gives the stage author alone (R124)")
	for _, a := range []struct {
		name string
		of   report.Apart
	}{{"mirrored boxes", d.Mirrored}, {"crossed-out boxes", d.CrossedOut}} {
		_, _ = fmt.Fprintf(w, "- %s, without a target: false alarms %d of %d, false correct %d of %d\n",
			a.name, a.of.Alarm.Events, a.of.Alarm.Of, a.of.Correct.Events, a.of.Correct.Of)
	}
	_, _ = fmt.Fprintln(w)
}
