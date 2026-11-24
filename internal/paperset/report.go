package paperset

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
)

// Report is the report of a run of a call of a task from paper: for each configuration and each
// kind of case, how many calls were made, how many came back on the schema, and how many of those
// were right by the case's truth; and what the run cost.
type Report struct {
	log  *Log
	rows []Row
}

// Row is a line of the report: a configuration, a kind of case or "all", and its counts.
type Row struct {
	Config, Kind           string
	Calls, Answered, Right int
	USD                    float64
}

// NewReport counts a log read back by its configurations and the kinds of its cases. A call of a
// case or a configuration the log does not tell is an error.
func NewReport(l *Log) (*Report, error) {
	if len(l.Run.Configs) == 0 || len(l.Sheets) == 0 {
		return nil, errors.New("paperset: the log has no configurations or no cases")
	}
	kinds := map[int]string{}
	for _, c := range l.Sheets {
		kinds[c.Case.Number] = c.Case.Kind
	}
	type at struct{ config, kind string }
	counts := map[at]*Row{}
	add := func(config, kind string, c *CallRecord) {
		r := counts[at{config, kind}]
		if r == nil {
			r = &Row{Config: config, Kind: kind}
			counts[at{config, kind}] = r
		}
		if c.NotAsked {
			return
		}
		r.Calls++
		r.USD += c.CostUSD
		if c.Error == "" {
			r.Answered++
		}
		if c.Right {
			r.Right++
		}
	}
	names := []string{}
	for _, c := range l.Run.Configs {
		names = append(names, c.Name)
	}
	for i := range l.Calls {
		c := &l.Calls[i]
		kind, ok := kinds[c.Case]
		if !ok || !slices.Contains(names, c.Config) {
			return nil, fmt.Errorf("paperset: a call of case %d by %q, which the log does not tell", c.Case, c.Config)
		}
		add(c.Config, kind, c)
		add(c.Config, "all", c)
	}
	r := &Report{log: l}
	for _, name := range names {
		for _, kind := range append(slices.Clone(caseKinds), "all") {
			if row := counts[at{name, kind}]; row != nil {
				r.rows = append(r.rows, *row)
			}
		}
	}
	return r, nil
}

// Rows are the lines of the report, configuration by configuration, the kinds of case in their order
// and all of them last.
func (r *Report) Rows() []Row { return r.rows }

func share(n, of int) string {
	if of == 0 {
		return "—"
	}
	return fmt.Sprintf("%.1f%%", 100*float64(n)/float64(of))
}

// Summary is what the terminal says: each configuration's share right over all its cases.
func (r *Report) Summary() string {
	s := ""
	for _, row := range r.rows {
		if row.Kind == "all" {
			s += fmt.Sprintf("%s, %s: %d of %d answered right, %s, $%.4f.\n", r.log.Run.CallKind, row.Config, row.Right, row.Calls, share(row.Right, row.Calls), row.USD)
		}
	}
	return s
}

// Markdown writes the report for the author: the run and a table of its rows.
func (r *Report) Markdown(w io.Writer, logName string) error {
	run := r.log.Run
	_, err := fmt.Fprintf(w, "# The %s of a task from paper\n\nThe log %s: the part %s of %s, seed %d, prompt version %d (%s), %d repeats.\n\n| Configuration | Kind | Calls | Answered | Right | Share right | USD |\n|---|---|---|---|---|---|---|\n",
		run.CallKind, logName, run.Part, run.File, run.Seed, run.PromptVersion, run.Prompt, run.Repeats)
	for _, row := range r.rows {
		if err == nil {
			_, err = fmt.Fprintf(w, "| %s | %s | %d | %d | %d | %s | %.4f |\n", row.Config, row.Kind, row.Calls, row.Answered, row.Right, share(row.Right, row.Calls), row.USD)
		}
	}
	return err
}

// CSV writes the rows of the report, a header first.
func (r *Report) CSV(w io.Writer) error {
	out := csv.NewWriter(w)
	_ = out.Write([]string{"call_kind", "config", "kind", "calls", "answered", "right", "usd"})
	for _, row := range r.rows {
		_ = out.Write([]string{r.log.Run.CallKind, row.Config, row.Kind, strconv.Itoa(row.Calls), strconv.Itoa(row.Answered), strconv.Itoa(row.Right), strconv.FormatFloat(row.USD, 'f', 6, 64)})
	}
	out.Flush()
	return out.Error()
}
