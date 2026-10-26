// Package sheets reads the directory where the record mode of the board saves the measurement's
// sheets (R25), against the sheet plan: for each plan sheet its last file, read and checked as a
// sample. From it come the summary of what is recorded, without the handwriting, and the split of
// the sheets into a development and a test part (R24).
package sheets

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/altnovatrail/tutor/internal/sample"
)

// SplitFile is the name of the split's file in the directory of sheets.
const SplitFile = "split.json"

// Sheet is a plan sheet as the directory holds it.
type Sheet struct {
	Plan *sample.PlanSheet
	// File is the name of the sheet's last file, the one that counts, or "" when it has none yet;
	// Files counts its files.
	File  string
	Files int
	// Sample is the sample in File once it passes the check with the bank and is the plan sheet's
	// task and scenario. When it is not, Problems names where it fails, by paths alone: nothing of
	// the handwriting.
	Sample   *sample.Sample
	Problems []string
}

// Recorded is the directory of sheets read against the plan.
type Recorded struct {
	Sheets []Sheet
	// Other names the files that are neither a saved sheet of the plan nor the split: other names,
	// or the names of another plan's sheets. They are not read.
	Other []string
}

// Read reads the directory against the plan and the bank. A file that does not pass is a problem
// of its sheet, not an error; the error is a directory that cannot be read.
func Read(dir string, plan *sample.Plan, bank *sample.Bank) (*Recorded, error) {
	if len(plan.Sheets) > sample.MaxSheets {
		return nil, fmt.Errorf("the plan has %d sheets, and a file's name holds a sheet's number in two digits", len(plan.Sheets))
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	r := &Recorded{Sheets: make([]Sheet, len(plan.Sheets)), Other: []string{}}
	for i := range plan.Sheets {
		r.Sheets[i].Plan = &plan.Sheets[i]
	}
	// The entries come sorted by name, so a later save of a sheet comes later.
	for _, e := range entries {
		if e.Name() == SplitFile {
			continue
		}
		n, ok := sample.ParseSheetFile(e.Name())
		if !ok || !e.Type().IsRegular() || n.Number < 1 || n.Number > len(plan.Sheets) || !n.Saves(&plan.Sheets[n.Number-1]) {
			r.Other = append(r.Other, e.Name())
			continue
		}
		sh := &r.Sheets[n.Number-1]
		sh.File = e.Name()
		sh.Files++
	}
	for i := range r.Sheets {
		if sh := &r.Sheets[i]; sh.File != "" {
			sh.Sample, sh.Problems = readSheet(filepath.Join(dir, sh.File), sh.Plan, bank)
		}
	}
	return r, nil
}

// readSheet reads a sheet's file as a sample and checks it with the bank and against its plan
// sheet's task and scenario.
func readSheet(path string, sh *sample.PlanSheet, bank *sample.Bank) (*sample.Sample, []string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, []string{"(the file cannot be read)"}
	}
	s, err := sample.ReadSample(bytes.NewReader(data))
	if err == nil {
		err = s.CheckWithBank(bank)
	}
	if err != nil {
		var fe *sample.FormatError
		if !errors.As(err, &fe) {
			return nil, []string{"(the file is not the JSON of a sample)"}
		}
		paths := make([]string, len(fe.Problems))
		for i, p := range fe.Problems {
			paths[i] = p.Path
		}
		return nil, paths
	}
	var off []string
	if s.Task != sh.Task {
		off = append(off, "task")
	}
	if s.Scenario != sh.Scenario {
		off = append(off, "scenario")
	}
	if len(off) > 0 {
		return nil, off
	}
	return s, nil
}

// count writes a number with a noun, in the singular for one: "1 sheet", "40 sheets".
func count(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// Numbers lists sheets by their numbers, or says there are none.
func Numbers(list []int) string {
	if len(list) == 0 {
		return "none"
	}
	parts := make([]string, len(list))
	for i, n := range list {
		parts[i] = fmt.Sprint(n)
	}
	return strings.Join(parts, ", ")
}

// tally is how many sheets of a kind are recorded and pass, against how many the plan has.
type tally struct {
	name              string
	recorded, planned int
}

func tallies(list []tally) string {
	parts := make([]string, len(list))
	for i, t := range list {
		parts[i] = fmt.Sprintf("%s %d of %d", t.name, t.recorded, t.planned)
	}
	return strings.Join(parts, ", ")
}

// Summary says what the directory holds against the plan, in numbers and names only: no point, no
// stroke and no text of a line, nothing of the handwriting. The counts by task, scenario and
// outcome, the erasures, the frames, the sheets drawn by code, the mouse and the markup off the plan, named by the paths of
// its fields as the board names them (Differences), are of the sheets that pass the check.
func Summary(r *Recorded) string {
	var b strings.Builder
	line := func(format string, args ...any) { _, _ = fmt.Fprintf(&b, format+"\n", args...) }

	var missing, failing []int
	var more []string
	recorded, files := 0, 0
	var passing []*Sheet
	for i := range r.Sheets {
		sh := &r.Sheets[i]
		switch {
		case sh.File == "":
			missing = append(missing, sh.Plan.Number)
			continue
		case sh.Sample == nil:
			failing = append(failing, sh.Plan.Number)
		default:
			passing = append(passing, sh)
		}
		recorded++
		files += sh.Files
		if sh.Files > 1 {
			more = append(more, fmt.Sprintf("%d (%d files)", sh.Plan.Number, sh.Files))
		}
	}
	line("Sheets: %d of the plan's %d recorded, %d of them passing the check.", recorded, len(r.Sheets), len(passing))
	if len(missing) > 0 {
		line("Not recorded yet: %s.", Numbers(missing))
	}
	line("Files: %d for these sheets, and the last of each counts.", files)
	if len(more) > 0 {
		line("Saved more than once: %s.", strings.Join(more, ", "))
	}
	if len(r.Other) > 0 {
		line("Not read: %s, no saved sheet of this plan.", count(len(r.Other), "other file"))
	}
	for _, n := range failing {
		sh := r.Sheets[n-1]
		line("Sheet %d does not pass the check, %s, at %s.", n, sh.File, strings.Join(sh.Problems, ", "))
	}

	line("By task: %s.", tallies(byTask(r, passing)))
	line("By scenario: %s.", tallies(byScenario(r, passing)))
	line("By outcome as marked: %s.", tallies(byOutcome(r, passing)))

	planned, slips, slipped, framed := 0, 0, 0, 0
	var mouse, drawn []int
	var off []string
	for _, sh := range passing {
		s := sh.Sample
		own := 0
		for _, e := range s.Erasures {
			if e.Planned != nil {
				planned++
			} else {
				own++
			}
		}
		slips += own
		if own > 0 {
			slipped++
		}
		if len(s.Erasures) > 0 {
			framed++
		}
		if s.Writer == sample.WriterSynthetic {
			drawn = append(drawn, sh.Plan.Number)
		}
		if byMouse(s) {
			mouse = append(mouse, sh.Plan.Number)
		}
		if where := offPaths(Differences(sh.Plan, s)); len(where) > 0 {
			off = append(off, fmt.Sprintf("%d (%s)", sh.Plan.Number, strings.Join(where, ", ")))
		}
	}
	line("Planned erasures: %d. Slips: %s on %s.", planned, count(slips, "erasure"), count(slipped, "sheet"))
	line("With frames, that is with an erasure: %s.", count(framed, "sheet"))
	line("Drawn by code, the writer synthetic: %s.", Numbers(drawn))
	line("Written with a mouse, pressure 0.5 at every point: %s.", Numbers(mouse))
	if len(off) == 0 {
		line("Marked off the plan: none.")
	} else {
		line("Marked off the plan: %s.", strings.Join(off, ", "))
	}
	return b.String()
}

func byTask(r *Recorded, passing []*Sheet) []tally {
	var names []string
	at := map[string]*tally{}
	for _, sh := range r.Sheets {
		if at[sh.Plan.Task] == nil {
			names = append(names, sh.Plan.Task)
			at[sh.Plan.Task] = &tally{name: sh.Plan.Task}
		}
		at[sh.Plan.Task].planned++
	}
	for _, sh := range passing {
		at[sh.Plan.Task].recorded++
	}
	slices.Sort(names)
	out := make([]tally, len(names))
	for i, n := range names {
		out[i] = *at[n]
	}
	return out
}

func byScenario(r *Recorded, passing []*Sheet) []tally {
	var out []tally
	for _, sc := range sample.Scenarios {
		t := tally{name: string(sc)}
		for _, sh := range r.Sheets {
			if sh.Plan.Scenario == sc {
				t.planned++
			}
		}
		for _, sh := range passing {
			if sh.Plan.Scenario == sc {
				t.recorded++
			}
		}
		if t.planned > 0 || t.recorded > 0 {
			out = append(out, t)
		}
	}
	return out
}

func byOutcome(r *Recorded, passing []*Sheet) []tally {
	var out []tally
	for _, o := range []sample.Outcome{sample.OutcomeCorrect, sample.OutcomeError, sample.OutcomeUnreadable, sample.OutcomeNoAnswer} {
		t := tally{name: string(o)}
		for _, sh := range r.Sheets {
			if sh.Plan.Expected.Outcome == o {
				t.planned++
			}
		}
		for _, sh := range passing {
			if sh.Sample.Expected.Outcome == o {
				t.recorded++
			}
		}
		if t.planned > 0 || t.recorded > 0 {
			out = append(out, t)
		}
	}
	return out
}

// byMouse reports whether every point of a sheet has the pressure 0.5, which a device without
// pressure, a mouse, reports while its button is down (architecture 4).
func byMouse(s *sample.Sample) bool {
	points := 0
	for _, st := range s.Strokes {
		for _, p := range st.Points {
			if p.Pressure != 0.5 {
				return false
			}
			points++
		}
	}
	return points > 0
}

// offPaths are the paths of a sheet's differences from its plan sheet, each once, in their order.
func offPaths(diffs []sample.Problem) []string {
	var paths []string
	for _, d := range diffs {
		if !slices.Contains(paths, d.Path) {
			paths = append(paths, d.Path)
		}
	}
	return paths
}

// Split divides the sheets into a development part of dev sheets and a test part (R24). Each
// scenario gives the development part its share of dev, as near its share of the plan as whole
// sheets allow: the shares left over go to the scenarios with the largest remainders, and of two
// as large, to the one that comes first among sample.Scenarios. Which of a scenario's sheets go,
// the seed decides: they stand in the order of the SHA-256 of the seed and the file's name. Every
// sheet of the plan must be recorded and pass the check.
func Split(r *Recorded, seed int64, dev int) (*sample.Split, error) {
	var missing []int
	for _, sh := range r.Sheets {
		if sh.Sample == nil {
			missing = append(missing, sh.Plan.Number)
		}
	}
	if len(missing) > 0 {
		which := "these are"
		if len(missing) == 1 {
			which = "this one is"
		}
		return nil, fmt.Errorf("every sheet of the plan is recorded and passes the check before the split, and %s not: %s", which, Numbers(missing))
	}
	total := len(r.Sheets)
	if dev < 1 || dev >= total {
		return nil, fmt.Errorf("the development part of %d sheets leaves no part: it takes 1 to %d of the plan's %d sheets", dev, total-1, total)
	}
	type stratum struct {
		sheets      []Sheet
		share, rest int
		order       int
	}
	var strata []*stratum
	for i, sc := range sample.Scenarios {
		st := &stratum{order: i}
		for _, sh := range r.Sheets {
			if sh.Plan.Scenario == sc {
				st.sheets = append(st.sheets, sh)
			}
		}
		if len(st.sheets) > 0 {
			st.share, st.rest = dev*len(st.sheets)/total, dev*len(st.sheets)%total
			strata = append(strata, st)
		}
	}
	left := dev
	for _, st := range strata {
		left -= st.share
	}
	byRest := slices.Clone(strata)
	slices.SortStableFunc(byRest, func(a, b *stratum) int { return b.rest - a.rest })
	for _, st := range byRest[:left] {
		st.share++
	}

	v := sample.Version
	sp := &sample.Split{Format: sample.SplitFormat, Version: &v, Seed: seed, Dev: []sample.SplitSheet{}, Test: []sample.SplitSheet{}}
	for _, st := range strata {
		slices.SortFunc(st.sheets, func(a, b Sheet) int { return bytes.Compare(key(seed, a.File), key(seed, b.File)) })
		for i, sh := range st.sheets {
			one := sample.SplitSheet{Number: sh.Plan.Number, File: sh.File}
			if i < st.share {
				sp.Dev = append(sp.Dev, one)
			} else {
				sp.Test = append(sp.Test, one)
			}
		}
	}
	byNumber := func(a, b sample.SplitSheet) int { return a.Number - b.Number }
	slices.SortFunc(sp.Dev, byNumber)
	slices.SortFunc(sp.Test, byNumber)
	return sp, nil
}

// key is where a file stands in its scenario for the split of a seed.
func key(seed int64, file string) []byte {
	sum := sha256.Sum256(fmt.Appendf(nil, "%d:%s", seed, file))
	return sum[:]
}

// Lopsided names the scenarios a split leaves out of a part: those with no sheet in the development
// part, and those with none in the test part. Each comes in the order of sample.Scenarios.
func Lopsided(plan *sample.Plan, sp *sample.Split) (noDev, noTest []sample.Scenario) {
	in := func(part []sample.SplitSheet, sc sample.Scenario) bool {
		return slices.ContainsFunc(part, func(s sample.SplitSheet) bool {
			return s.Number >= 1 && s.Number <= len(plan.Sheets) && plan.Sheets[s.Number-1].Scenario == sc
		})
	}
	for _, sc := range sample.Scenarios {
		planned := slices.ContainsFunc(plan.Sheets, func(sh sample.PlanSheet) bool { return sh.Scenario == sc })
		switch {
		case !planned:
		case !in(sp.Dev, sc):
			noDev = append(noDev, sc)
		case !in(sp.Test, sc):
			noTest = append(noTest, sc)
		}
	}
	return noDev, noTest
}

// Stale names the sheets of a split whose file no longer counts for them in the directory: one
// saved again since the split, or one whose file is gone.
func Stale(r *Recorded, sp *sample.Split) []int {
	var out []int
	for _, part := range [][]sample.SplitSheet{sp.Dev, sp.Test} {
		for _, s := range part {
			if s.Number < 1 || s.Number > len(r.Sheets) || r.Sheets[s.Number-1].File != s.File {
				out = append(out, s.Number)
			}
		}
	}
	slices.Sort(out)
	return out
}
