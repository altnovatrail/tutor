package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/altnovatrail/tutor/internal/protocol"
	"github.com/altnovatrail/tutor/internal/report"
)

// runAdmit applies the rule of admission (R80) to runners' reports and prints, for each, the stage
// it admits its readers to, or why it admits them to nothing, with the numbers the rule reads and the
// SHA-256 of the file, which an entry of the list of admitted models names. It prints the IDs of no
// sample and nothing of any work, so that it may run on a report of the author's sheets.
func runAdmit(args []string, stdout, stderr io.Writer) int {
	const use = "usage: tutor-eval admit REPORT..."
	switch {
	case len(args) == 1 && (args[0] == "-h" || args[0] == "-help" || args[0] == "--help"):
		_, _ = fmt.Fprintln(stderr, use)
		return exitOK
	case len(args) == 0 || slices.ContainsFunc(args, func(a string) bool { return strings.HasPrefix(a, "-") }):
		_, _ = fmt.Fprintln(stderr, use)
		return exitUsage
	}
	for _, path := range args {
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
		writeDecision(stdout, filepath.Base(path), hex.EncodeToString(sum[:]), report.Admit(r))
	}
	return exitOK
}

// writeDecision writes a decision of the rule as Markdown.
func writeDecision(w io.Writer, file, sum string, d report.Decision) {
	stage := string(d.Stage)
	if stage == "" {
		stage = "none: " + d.Why
	}
	_, _ = fmt.Fprintf(w, "## %s\n\n- stage: %s\n- readers: %s\n- set: %s, %d repeats\n- sha256: %s\n",
		file, stage, strings.Join(d.Readers, " + "), d.Set, d.Repeats, sum)
	if d.Measures == nil {
		_, _ = fmt.Fprintln(w)
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
	_, _ = fmt.Fprintf(w, "- readings: %d; needless unreadable %d of %d; unchecked %d\n\n",
		m.Readings, m.Needless.Events, m.Needless.Of, unchecked)
}
