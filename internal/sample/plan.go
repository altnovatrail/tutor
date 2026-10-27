package sample

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Plan is a sheet plan: what the writer writes on each sheet, and the markup that the record mode
// of the board fills in from it (R25). A plan sheet is checked as the markup of a sample, so that
// what the board fills in passes the sample's check. A plan of Version has lines; one of PlanParts
// has parts, and in memory their texts are its lines.
type Plan struct {
	Format  string      `json:"format"`
	Version *int        `json:"version"`
	Sheets  []PlanSheet `json:"sheets"`
}

// PlanSheet is one sheet of a plan. Who writes it, the plan does not say: the author writes every
// sheet of the measurement in their own hand (R24).
type PlanSheet struct {
	// Number is the sheet's place in the plan, from 1.
	Number int `json:"number"`
	// Task is the ID of the task in the bank that the sheet solves.
	Task     string   `json:"task"`
	Scenario Scenario `json:"scenario"`
	// Lines are the texts of the lines left on the sheet when it is done, top to bottom, in the
	// notation of the bank's solutions.
	Lines []string `json:"lines"`
	// Answer is the number of the line that gives the final answer, from 1.
	Answer   int      `json:"answer"`
	Expected Expected `json:"expected"`
	// Erasure is the planned erasure, on sheets of the scenarios that plan one.
	Erasure *PlanErasure `json:"erasure"`
	// Parts are the parts of a sheet of a plan of version PlanParts, and Lines are their texts;
	// a sheet of version 0 has none. The numbers of the sheet — Answer, Expected.FirstError and
	// Erasure.Line — are then numbers of parts. Its file holds them in place of the lines.
	Parts []PlanPart `json:"-"`
}

// PlanErasure is the planned erasure of a sheet: its markup, as a sample's PlannedErasure has it,
// and what is written in place of the erased strokes.
type PlanErasure struct {
	Line   int    `json:"line"`
	Erased Erased `json:"erased"`
	Text   string `json:"text"`
	// Replacement is what is written in place of the erased strokes: the line Line, or only a part
	// of it when PartOfLine is set.
	Replacement string `json:"replacement"`
	// PartOfLine marks an erasure of a part of a line, such as one number: Text and Replacement are
	// that part, and the rest of the line stays.
	PartOfLine bool `json:"part_of_line"`
	// Figure is, in a plan of version PlanParts, the figure as it was before an erasure in a
	// drawing; nil for any other erasure. Its file holds it in the erasure.
	Figure *Figure `json:"-"`
}

// A plan sheet's lines are plain texts, and one of them is the answer, so a plan cannot hold a
// sheet whose markup needs more: these are the scenarios and outcomes of such sheets, with what
// the plan sheet lacks for them. {line} is a line, or a part in a plan of version PlanParts.
var (
	unplannedScenarios = map[Scenario]string{
		ScenarioIllegible:   "a plan sheet marks no {line} illegible or crossed out",
		ScenarioNearlyEmpty: "a plan sheet always has an answer {line}",
		ScenarioInjection:   "a plan sheet holds no injection",
	}
	unplannedOutcomes = map[Outcome]string{
		OutcomeUnreadable: "a plan sheet marks no {line} illegible",
		OutcomeNoAnswer:   "a plan sheet always has an answer {line}",
	}
)

// words are what the messages call the pieces of a plan sheet: its lines in version 0 and its
// parts in version PlanParts, where "part" alone is taken.
type words struct {
	lines, line, piece string
}

var (
	lineWords = words{lines: "lines", line: "line", piece: "part"}
	partWords = words{lines: "parts", line: "part", piece: "piece"}
)

// Check checks a plan and returns a *FormatError listing every problem, or nil. Each sheet that a
// plan can hold goes through Sample.Check as the markup of a stand-in sample, so that every rule
// of a sample's markup holds for the plan too, and a problem of the stand-in is named by the field
// of the plan sheet it comes from. A sheet of a plan of version PlanParts is checked on the texts
// of its parts, and then its parts are.
func (pl *Plan) Check() error {
	p := &problems{what: "plan"}
	checkHeader(p, pl.Format, PlanFormat, pl.Version, planVersions...)
	if len(pl.Sheets) == 0 {
		p.add("sheets", "are missing: a plan has at least one sheet")
	}
	for i := range pl.Sheets {
		path, sh := fmt.Sprintf("sheets[%d]", i), &pl.Sheets[i]
		if pl.HasParts() {
			checkPartsSheet(p, path, i+1, sh)
			continue
		}
		// What only a file of version PlanParts holds, a plan of another version would lose.
		if sh.Parts != nil {
			p.add(path, "has parts, but only a plan of version %d has parts", PlanParts)
		}
		if e := sh.Erasure; e != nil && e.Figure != nil {
			p.add(path+".erasure", "has a figure, but only a plan of version %d has figures", PlanParts)
		}
		checkPlanSheet(p, path, i+1, sh, lineWords)
	}
	return p.err()
}

// CheckWithBank checks what in a plan depends on the bank, as Sample.CheckWithBank does for a
// sample: that the bank has the task of every sheet, and that the trap a sheet names is one of its
// task's. It returns a *FormatError, or nil.
func (pl *Plan) CheckWithBank(b *Bank) error {
	p := &problems{what: "plan"}
	for i, sh := range pl.Sheets {
		checkInBank(p, fmt.Sprintf("sheets[%d]", i), sh.Task, sh.Expected.Trap, b)
	}
	return p.err()
}

func checkPlanSheet(p *problems, path string, number int, sh *PlanSheet, w words) {
	if sh.Number != number {
		p.add(path+".number", "is %d, not %d: the sheets are numbered in order from 1", sh.Number, number)
	}
	planned := true
	if why, ok := unplannedScenarios[sh.Scenario]; ok {
		p.add(path+".scenario", "%q cannot be planned: %s", sh.Scenario, strings.ReplaceAll(why, "{line}", w.line))
		planned = false
	}
	if why, ok := unplannedOutcomes[sh.Expected.Outcome]; ok {
		p.add(path+".expected.outcome", "%q cannot be planned: %s", sh.Expected.Outcome, strings.ReplaceAll(why, "{line}", w.line))
		planned = false
	}
	switch {
	case len(sh.Lines) == 0:
		p.add(path+"."+w.lines, "are missing: a sheet has at least one %s", w.line)
	case sh.Answer < 1 || sh.Answer > len(sh.Lines):
		p.add(path+".answer", "%d is not a %s of the sheet, 1…%d", sh.Answer, w.line, len(sh.Lines))
	}
	// A stand-in of a sheet that a plan cannot hold would only fail for what the plan lacks.
	if planned {
		checkAsSample(p, path, sh, w)
	}
	checkReplacement(p, path+".erasure", sh, w)
}

// checkAsSample checks a plan sheet as the markup of a sample: its stand-in goes through
// Sample.Check, and each problem is named by the field of the plan sheet it comes from. The lines
// of the stand-in of a sheet of parts are its parts, and its messages say so.
func checkAsSample(p *problems, path string, sh *PlanSheet, w words) {
	var fe *FormatError
	if !errors.As(sh.standIn().Check(), &fe) {
		return
	}
	for _, pr := range fe.Problems {
		to, ok := planPath(pr.Path, w)
		if !ok {
			continue
		}
		message := pr.Message
		if w == partWords {
			message = partsNotLines(message)
		}
		p.add(join(path, to), "%s", message)
	}
}

// lineWord finds the words "line" and "lines" alone: not inside a name such as part_of_line.
var lineWord = regexp.MustCompile(`\blines?\b`)

// partsNotLines says "part" for "line" in a message of the stand-in of a sheet of parts, outside
// the values it quotes, so that a quoted text keeps its words.
func partsNotLines(message string) string {
	pieces := strings.Split(message, `"`)
	for i := 0; i < len(pieces); i += 2 {
		pieces[i] = lineWord.ReplaceAllStringFunc(pieces[i], func(s string) string { return strings.Replace(s, "line", "part", 1) })
	}
	return strings.Join(pieces, `"`)
}

// standIn is a sample with the sheet's markup and the fewest strokes that markup can name: one to
// a line, in the order of the lines, and, for a planned erasure, one more, erased just before the
// line written in its place. Each stroke is one point, so no rule of time or place can fail on it.
func (sh *PlanSheet) standIn() *Sample {
	v := Version
	s := &Sample{
		Format:  SampleFormat,
		Version: &v,
		// The measurement's sheet (R24); any sheet would do, as every point is at its corner.
		Sheet:    Sheet{Width: 1000, Height: 600},
		Task:     sh.Task,
		Writer:   WriterAuthor,
		Scenario: sh.Scenario,
		Expected: sh.Expected,
	}
	var clock int64
	stroke := func() string {
		id := fmt.Sprintf("s-%d", len(s.Strokes)+1)
		s.Strokes = append(s.Strokes, Stroke{ID: id, Points: []Point{{TMs: clock, Pressure: 0.5}}})
		clock++
		return id
	}
	erase := func() {
		e := sh.Erasure
		erased := stroke()
		s.Erasures = append(s.Erasures, Erasure{
			TMs:     clock,
			Strokes: []string{erased},
			Planned: &PlannedErasure{Line: e.Line, Erased: e.Erased, Text: e.Text},
		})
		clock++
	}
	for i, text := range sh.Lines {
		if sh.Erasure != nil && sh.Erasure.Line == i+1 {
			erase()
		}
		s.Lines = append(s.Lines, Line{Text: text, Strokes: []string{stroke()}, Answer: i+1 == sh.Answer})
	}
	if e := sh.Erasure; e != nil && (e.Line < 1 || e.Line > len(sh.Lines)) {
		erase() // an erasure of no line, which the check names
	}
	return s
}

// planPath names the field of a plan sheet that a problem of its stand-in comes from: its planned
// erasure comes from the erasure, the text of its lines from the lines — or the text of the parts
// of a sheet of parts — and a problem of its answer line from the answer. The task, the scenario
// and the expected outcome are the plan sheet's own, under the same names. A problem of its
// strokes or of how many lines it marks as the answer comes only from a sheet without lines or
// with an answer of no line, which the plan's own check names, and planPath drops it.
func planPath(path string, w words) (string, bool) {
	switch {
	case path == "strokes", path == "lines":
		return "", false
	case path == "erasures":
		return "erasure", true
	case strings.HasPrefix(path, "erasures[0].planned."):
		return "erasure." + strings.TrimPrefix(path, "erasures[0].planned."), true
	case strings.HasPrefix(path, "lines[") && strings.HasSuffix(path, "].text") && w == partWords:
		return "parts" + strings.TrimPrefix(path, "lines"), true
	case strings.HasPrefix(path, "lines[") && strings.HasSuffix(path, "].text"):
		return strings.TrimSuffix(path, ".text"), true
	case strings.HasPrefix(path, "lines[") && !strings.Contains(path, "."):
		return "answer", true
	}
	return path, true
}

// checkReplacement checks that what a planned erasure writes in place of the erased strokes
// stands in its line: it is the line, or a piece of it that the line holds once, and not the
// erased text again. In a column, the piece is one of its numbers, so that the drawing knows which
// one is written again.
func checkReplacement(p *problems, path string, sh *PlanSheet, w words) {
	e := sh.Erasure
	if e == nil || e.Line < 1 || e.Line > len(sh.Lines) {
		return // no erasure, or one of no line, which the stand-in's check names
	}
	line := sh.Lines[e.Line-1]
	var column *Column
	if e.Line <= len(sh.Parts) {
		column = sh.Parts[e.Line-1].Figure.Column
	}
	switch {
	case e.Replacement == "":
		p.add(path+".replacement", "is missing: what is written in place of the erased strokes")
	case e.Replacement == e.Text:
		p.add(path+".replacement", "is the erased text: a planned erasure writes something else in its place")
	case column != nil:
		checkColumnErasure(p, path, e, *column, w)
	case !e.PartOfLine && e.Replacement != line:
		p.add(path+".replacement", "%q is not %s %d, %q: the %s is what is written in place of the erased one", e.Replacement, w.line, e.Line, line, w.line)
	case e.PartOfLine && (e.Replacement == line || strings.Count(line, e.Replacement) != 1):
		p.add(path+".replacement", "%q is not a %s of %s %d, %q, that the %s holds once: only that %s replaces the erased one, and the rest of the %s stays", e.Replacement, w.piece, w.line, e.Line, line, w.line, w.piece, w.line)
	}
}

// checkColumnErasure checks a planned erasure in a column: it takes one of the column's numbers,
// a number written before it, and writes that number of the column again, one the column holds
// once, so that the drawing knows which number to write again.
func checkColumnErasure(p *problems, path string, e *PlanErasure, c Column, w words) {
	if !e.PartOfLine {
		p.add(path+".part_of_line", "is false, but an erasure in a column takes one of its numbers: text and replacement are that number before and after")
	}
	if !digits(e.Text) {
		p.add(path+".text", "%q is not a number: an erasure in a column takes one of its numbers", e.Text)
	}
	numbers := []string{c.Top, c.Bottom, c.Result}
	held := 0
	for _, n := range numbers {
		if n == e.Replacement {
			held++
		}
	}
	if held != 1 {
		p.add(path+".replacement", "%q is not a number that the column of %s %d holds once, among %q: only that number is written again", e.Replacement, w.line, e.Line, numbers)
	}
}
