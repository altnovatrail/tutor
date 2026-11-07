package report

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"

	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"

	"github.com/altnovatrail/tutor/internal/runner"
)

// The section of the runner's reports in the Markdown of a report for people, and their CSV: the
// measures of section 14, counted from the records of each report (R74).

// causes are the causes of an unchecked reading, in the order the section names them.
var causes = []evalv1.UncheckedCause{evalv1.UncheckedCause_UNCHECKED_CAUSE_FAILURE, evalv1.UncheckedCause_UNCHECKED_CAUSE_LIMIT,
	evalv1.UncheckedCause_UNCHECKED_CAUSE_INVALID, evalv1.UncheckedCause_UNCHECKED_CAUSE_NO_MARK}

// ReportFile is the name of the file of a runner's report beside its log: the log's name without its
// .jsonl, the report's name, and .report.json.
func ReportFile(logBase, name string) string { return logBase + "." + name + ".report.json" }

// counted is a share as the section writes it: "k of n".
func counted(c Count) string { return fmt.Sprintf("%d of %d", c.Events, c.Of) }

// causesOf are the unchecked readings by cause, as the section writes them: failure / limit /
// invalid, and no mark of an essay.
func causesOf(m map[evalv1.UncheckedCause]int, essays bool) string {
	parts := []string{itoa(m[causes[0]]), itoa(m[causes[1]]), itoa(m[causes[2]])}
	if essays {
		parts = append(parts, itoa(m[causes[3]]))
	}
	return strings.Join(parts, " / ")
}

// samplesOf is how many samples of a report are measured and refused.
func samplesOf(r *evalv1.Report) (measured, refused int) {
	for _, s := range r.GetSamples() {
		if s.GetMeasured() != nil {
			measured++
		} else {
			refused++
		}
	}
	return measured, refused
}

// readersOf names a report's models.
func readersOf(r *evalv1.Report) string {
	var out []string
	for _, m := range r.GetReaders() {
		out = append(out, m.GetModel())
	}
	return strings.Join(out, " and ")
}

// markdownProtocol writes the section of the runner's reports: none when the log tells none, and
// the refused samples that name no ID, which no report holds.
func markdownProtocol(b *strings.Builder, logName string, reports []Named, refused []runner.Refused) {
	if len(reports) == 0 {
		return
	}
	base := strings.TrimSuffix(logName, ".jsonl")
	fmt.Fprintf(b, "\n## The runner's reports of the protocol (§ 14)\n\nEach is a file beside the log, %s, the message Report of novatrail.eval.v1 that the CI of modules and the rule of admission read: a reader alone, with all its repeats, or a pair joined by module-kit, of what the platform asks. Its verdicts are module-kit's; the measures below are counted from its records, all repeats together.\n\n",
		ReportFile(base, "<reader>"))
	reading := reports[0].Report.GetCapability() == evalv1.Capability_CAPABILITY_HANDWRITING_READING
	if reading {
		b.WriteString("| Report | Readers | Repeats | Samples measured | Refused | Answer slots | Readings | False alarm | False \"correct\" | Needless \"unreadable\" | \"Unreadable\" kept | Another trap | Unchecked: failure / limit / invalid |\n|---|---|---|---|---|---|---|---|---|---|---|---|---|\n")
		for _, n := range reports {
			m := MeasuresOf(n.Report).Reading
			measured, refusedN := samplesOf(n.Report)
			for _, g := range []struct {
				name string
				of   SlotMeasures
			}{{"", m.All}, {", mirrored", m.Mirrored}, {", crossed out", m.CrossedOut}} {
				if g.name != "" && g.of.Readings == 0 {
					continue
				}
				fmt.Fprintf(b, "| %s%s | %s | %d | %d | %d | %s | %d | %s | %s | %s | %s | %s | %s |\n", n.Name, g.name, readersOf(n.Report), n.Report.GetRepeats(), measured, refusedN,
					slotsOf(n.Report, g.name), g.of.Readings, counted(g.of.FalseAlarm), counted(g.of.FalseCorrect), counted(g.of.Needless), counted(g.of.Kept), counted(g.of.OtherTrap), causesOf(g.of.Unchecked, false))
			}
		}
		b.WriteString("\nA false alarm, the main measure, is a reading \"not yet\" of an answer slot whose truth is not; a digit written mirrored, whose truth is \"unreadable\", counts in it. Each share is among the readings section 14 names; an unchecked reading is counted only by its cause.\n")
	} else {
		b.WriteString("| Report | Reader | Repeats | Samples measured | Refused | Readings | Needless \"unreadable\" | False alarm | Agreement | Marks of none | Unchecked: failure / limit / invalid / no mark |\n|---|---|---|---|---|---|---|---|---|---|---|\n")
		var crit strings.Builder
		for _, n := range reports {
			m := MeasuresOf(n.Report).Essay
			measured, refusedN := samplesOf(n.Report)
			fmt.Fprintf(b, "| %s | %s | %d | %d | %d | %d | %s | %s | %s | %d | %s |\n", n.Name, readersOf(n.Report), n.Report.GetRepeats(), measured, refusedN, m.Readings,
				counted(m.Needless), counted(m.FalseAlarm), counted(m.Agreement), m.None, causesOf(m.Unchecked, true))
			for _, c := range m.Criteria {
				fmt.Fprintf(&crit, "| %s | %d | %s | %s | %d | %s |\n", n.Name, c.Criterion, counted(c.FalseAlarm), counted(c.Agreement), c.None, kappaText(c.Kappa))
			}
		}
		b.WriteString("\nBy criterion, by its place in the rubric:\n\n| Report | Criterion | False alarm | Agreement | Marks of none | κ |\n|---|---|---|---|---|---|\n")
		b.WriteString(crit.String())
		b.WriteString("\nA false alarm, the main measure, is a criterion AI marks not met that the people marked met, among the people's marks met; the agreement is the share of AI's marks that are the people's; AI's marks of none are counted apart. κ is Cohen's, none where p_e is 1.\n")
	}
	var unnamed []string
	for _, x := range refused {
		if x.Sample == "" {
			unnamed = append(unnamed, strings.TrimPrefix(x.Set+"/"+x.File, "/"))
		}
	}
	if len(unnamed) > 0 {
		fmt.Fprintf(b, "\nRefused, and in no report, since their files name no ID that can be read: %s.\n", strings.Join(unnamed, ", "))
	}
}

// slotsOf is how many answer slots of a report the measures of a group count: all, or those noted.
func slotsOf(r *evalv1.Report, group string) string {
	n := 0
	for _, s := range r.GetSamples() {
		for _, slot := range s.GetMeasured().GetAnswerSlots() {
			switch {
			case group == "", group == ", mirrored" && slot.GetMirrored(), group == ", crossed out" && slot.GetCrossedOut():
				n++
			}
		}
	}
	return itoa(n)
}

// kappaText is κ to two places, or a dash where there is none.
func kappaText(k *float64) string {
	if k == nil {
		return "—"
	}
	return strconv.FormatFloat(*k, 'f', 2, 64)
}

// ProtocolCSV writes the measures of the runner's reports as CSV, a row for each group of each
// report: all its answer slots, the mirrored and the crossed out, or in a run of assessment all its
// criteria and each.
func protocolCSV(w io.Writer, reports []Named) error {
	cw := csv.NewWriter(w)
	rows := [][]string{{"report", "readers", "repeats", "measured", "refused", "group", "readings", "false_alarm", "false_alarm_of", "false_correct", "false_correct_of",
		"needless_unreadable", "needless_unreadable_of", "unreadable_kept", "unreadable_kept_of", "another_trap", "another_trap_of", "agreement", "agreement_of", "none", "kappa",
		"unchecked_failure", "unchecked_limit", "unchecked_invalid", "unchecked_no_mark"}}
	empty := Count{}
	row := func(n Named, group string, readings int, fa, fc, nu, uk, at, ag Count, none int, k string, un map[evalv1.UncheckedCause]int) {
		measured, refused := samplesOf(n.Report)
		r := []string{n.Name, readersOf(n.Report), itoa(int(n.Report.GetRepeats())), itoa(measured), itoa(refused), group, itoa(readings)}
		for _, c := range []Count{fa, fc, nu, uk, at, ag} {
			r = append(r, itoa(c.Events), itoa(c.Of))
		}
		r = append(r, itoa(none), k)
		for _, c := range causes {
			r = append(r, itoa(un[c]))
		}
		rows = append(rows, r)
	}
	for _, n := range reports {
		m := MeasuresOf(n.Report)
		if m.Reading != nil {
			for _, g := range []struct {
				name string
				of   SlotMeasures
			}{{"all", m.Reading.All}, {"mirrored", m.Reading.Mirrored}, {"crossed_out", m.Reading.CrossedOut}} {
				row(n, g.name, g.of.Readings, g.of.FalseAlarm, g.of.FalseCorrect, g.of.Needless, g.of.Kept, g.of.OtherTrap, empty, 0, "", g.of.Unchecked)
			}
			continue
		}
		e := m.Essay
		row(n, "all", e.Readings, e.FalseAlarm, empty, e.Needless, empty, empty, e.Agreement, e.None, "", e.Unchecked)
		for _, c := range e.Criteria {
			k := ""
			if c.Kappa != nil {
				k = strconv.FormatFloat(*c.Kappa, 'f', 4, 64)
			}
			row(n, "criterion "+itoa(c.Criterion), 0, c.FalseAlarm, empty, empty, empty, empty, c.Agreement, c.None, k, nil)
		}
	}
	if err := cw.WriteAll(rows); err != nil {
		return err
	}
	return cw.Error()
}
