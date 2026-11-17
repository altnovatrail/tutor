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

	"github.com/altnovatrail/tutor/admission"
	"github.com/altnovatrail/tutor/internal/essay"
	"github.com/altnovatrail/tutor/internal/protocol"
	"github.com/altnovatrail/tutor/internal/report"
	"github.com/altnovatrail/tutor/internal/wordsjudge"
)

// The kinds of report admit rules on: a sheet's, by the rule of R80, a workbook's, by the rule of the
// workbook, and the log of a run of the essays, by the rules of the reading of an essay and of the
// rubric (R124).
const (
	admitSheet    = "sheet"
	admitWorkbook = "workbook"
	admitEssay    = "essay"
)

// runAdmit applies the rule of admission (R80) to runners' reports and prints, for each, the stage
// it admits its readers to, or why it admits them to nothing, with the numbers the rule reads and the
// SHA-256 of the file, which an entry of the list of admitted models names. -kind workbook applies
// the rule of the workbook (R124) and prints the mirrored and the crossed-out boxes in rows of their
// own. -kind essay applies the rules of the essay (T156) to logs of runs of the essays in two calls,
// each with the judge's log of its feedback beside it, .judge.jsonl, and the judge's log of its
// control phrases, -controls, and prints a stage or a refusal for the reading, CallEssay, and for the
// assessment by the rubric, CallRubric, each apart. It prints the IDs of no sample and nothing of any
// work, so that it may run on a report of the author's sheets.
func runAdmit(args []string, stdout, stderr io.Writer) int {
	const use = "usage: tutor-eval admit [-kind sheet|workbook] REPORT...\n       tutor-eval admit -kind essay -controls JUDGE-LOG ESSAYS-LOG..."
	flags := flag.NewFlagSet("tutor-eval admit", flag.ContinueOnError)
	kind := flags.String("kind", admitSheet, "the kind of report: sheet, of the reading of sheets (R80), workbook, of the reading of the pages of the workbook, or essay, the log of a run of the essays in two calls, for the reading of an essay and the rubric (R124)")
	controls := flags.String("controls", "", "for -kind essay, the judge's log of its control phrases")
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
	case *kind != admitSheet && *kind != admitWorkbook && *kind != admitEssay:
		_, _ = fmt.Fprintf(stderr, "tutor-eval admit: -kind is %q: give sheet, workbook or essay\n", *kind)
		return exitUsage
	case len(paths) == 0 || slices.ContainsFunc(paths, func(a string) bool { return strings.HasPrefix(a, "-") }):
		_, _ = fmt.Fprintln(stderr, use)
		return exitUsage
	case (*kind == admitEssay) != (*controls != ""):
		_, _ = fmt.Fprintln(stderr, "tutor-eval admit: -controls is for -kind essay, and -kind essay needs it: the rule of the rubric reads the judge's control phrases")
		return exitUsage
	}
	if *kind == admitEssay {
		return admitEssays(*controls, paths, stdout, stderr)
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
		file, hash := filepath.Base(path), sha256Of(data)
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

// admitEssays applies the rules of the essay to logs of runs of the essays in two calls, each with
// the judge's log of its feedback beside it, and writes a decision for the reading and one for the
// rubric of each, with the SHA-256 of every file read.
func admitEssays(controlsPath string, paths []string, stdout, stderr io.Writer) int {
	read := func(path string) ([]byte, string, error) {
		data, err := os.ReadFile(path)
		return data, sha256Of(data), err
	}
	judgeOf := func(path string) (*report.WordsJudge, string, error) {
		data, sum, err := read(path)
		if err != nil {
			return nil, "", err
		}
		j, err := wordsJudgeOf(path, data)
		return j, sum, err
	}
	controls, controlsSum, err := judgeOf(controlsPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval admit: %v\n", err)
		return exitFail
	}
	for _, path := range paths {
		data, sum, err := read(path)
		var r *report.Essays
		if err == nil {
			var l *essay.Log
			if l, err = essay.ReadLog(bytes.NewReader(data)); err == nil {
				r, err = report.NewEssays(l)
			}
		}
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "tutor-eval admit: %s: %v\n", path, err)
			return exitFail
		}
		judgePath := judgeLogOf(path)
		judge, judgeSum, err := judgeOf(judgePath)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "tutor-eval admit: the judge's log of %s: %v\n", path, err)
			return exitFail
		}
		file := filepath.Base(path)
		writeEssayDecisions(stdout, file, report.AdmitEssay(r), report.AdmitRubric(r, file, sum, controls, judge), []string{
			fmt.Sprintf("%s: sha256 %s", file, sum),
			fmt.Sprintf("%s, the judge's of its feedback: sha256 %s", filepath.Base(judgePath), judgeSum),
			fmt.Sprintf("%s, the judge's of its control phrases: sha256 %s", filepath.Base(controlsPath), controlsSum),
		})
	}
	return exitOK
}

// writeEssayDecisions writes the decisions of the rules of the essay on a log as Markdown: its files,
// then the reading and the rubric, each with its stage, its reasons and the numbers it read, a block
// ended by a blank line.
func writeEssayDecisions(w io.Writer, file string, e report.EssayDecision, r report.RubricDecision, files []string) {
	p := func(format string, a ...any) { _, _ = fmt.Fprintf(w, format, a...) }
	stage := func(s admission.Stage, why []string) {
		if s == "" {
			p("- stage: none\n")
		} else {
			p("- stage: %s; the rules of the essay give the stage author alone (R124)\n", s)
		}
		for _, y := range why {
			p("- not admitted: %s\n", y)
		}
	}
	p("## %s\n\n- reader: %s; assessor: %s\n- set: %s, %d repeats\n", file, e.Reader, e.Assessor, e.Set, e.Repeats)
	for _, f := range files {
		p("- %s\n", f)
	}
	p("\n### The reading of an essay, CallEssay\n\n")
	stage(e.Stage, e.Why)
	p("- legible essays read illegible: %d of %d\n", e.Illegible.Events, e.Illegible.Of)
	p("- essays with an error of the learner's read corrected: %d of %d\n", e.Corrected.Events, e.Corrected.Of)
	p("- words read wrong: %d of %d, upper bound %.4f (at most %.2f)\n", e.Words.Events, e.Words.Of, e.Words.Upper, report.EssayWordsTarget)
	p("- readings failed otherwise: %d (none: every essay is read in every repeat)\n", e.Failed)
	p("\n### The assessment by the rubric, CallRubric\n\n")
	stage(r.Stage, r.Why)
	p("- false alarms, met criteria judged unmet: %d of %d, upper bound %.4f (at most %.2f)\n", r.Alarm.Events, r.Alarm.Of, r.Alarm.Upper, report.RubricTarget)
	p("- missed \"not met\", without a target: %d of %d, upper bound %.4f\n", r.Missed.Events, r.Missed.Of, r.Missed.Upper)
	var kappas []string
	for _, c := range r.Criteria {
		kappas = append(kappas, c.Criterion+" "+c.KappaText())
	}
	p("- assessments failed: %d (none: every essay is assessed in every repeat)\n", r.Failed)
	p("- κ by criterion, without a target: %s; judgements unchecked, without a target: %d\n", strings.Join(kappas, ", "), r.Unchecked)
	p("- the feedback in English alone (R124); the judge %s, right on %d of %d control phrases, judged %d of the %d feedbacks that came, %d without a verdict\n",
		r.Judge, r.ControlsRight, r.Controls, r.Judged, r.Feedbacks, r.JudgeFailed)
	p("- the judge: the learner's sex told %s, off the task %s, not for a child %s\n", r.Gender, r.OffTask, r.NotForAge)
	p("- feedback breaking the rule of R70 spec, without a target: %d of %d\n\n", r.Broken, r.Feedbacks)
}

// sha256Of is the SHA-256 of a file's bytes, in hex, as an entry of the list of admitted models names
// its evidence.
func sha256Of(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// wordsJudgeOf is the report of a log of the judge of the words, read from path: admit and
// admit-words read the judge's logs alike.
func wordsJudgeOf(path string, data []byte) (*report.WordsJudge, error) {
	l, err := wordsjudge.ReadLog(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	j, err := report.NewWordsJudge(l)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return j, nil
}
