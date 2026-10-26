package board

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/altnovatrail/tutor/internal/lines"
	"github.com/altnovatrail/tutor/internal/sample"
)

// The board's sheet, in its logical units: the canvas of the page, on which every sheet of the
// measurement is written (R24).
const (
	sheetWidth  = 1000
	sheetHeight = 600
)

// savedName is the name of a saved sheet's file: the number of its plan sheet in two digits, the
// time of the save in UTC to the millisecond, the task and the scenario. The start of the name is
// of fixed width, so the last file of a sheet is the last by name (R25).
var savedName = regexp.MustCompile(`^(\d{2})-(\d{8}-\d{6})-(\d{3})-([a-z0-9-]+)\.json$`)

// plainName is what a task's ID must look like to go into a file name.
var plainName = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Recorder serves the record mode of the board: the author writes the sheets of the sheet plan,
// and each sheet is saved as a file of the format tutor-sample v0, in a directory outside git, once
// it passes the format's strict reader and the check against its plan sheet and the bank (R25).
type Recorder struct {
	bank *sample.Bank
	plan *sample.Plan
	dir  string
	now  func() time.Time

	saving sync.Mutex // one save at a time
	last   time.Time  // the latest time in the name of a saved sheet
}

// NewRecorder returns the record mode of a plan whose tasks are in the bank. The sheets are saved
// in dir, which it creates.
func NewRecorder(bank *sample.Bank, plan *sample.Plan, dir string) (*Recorder, error) {
	if err := plan.Check(); err != nil {
		return nil, err
	}
	if err := plan.CheckWithBank(bank); err != nil {
		return nil, err
	}
	if len(plan.Sheets) > 99 {
		return nil, fmt.Errorf("the plan has %d sheets, and a file name holds a sheet's number in two digits", len(plan.Sheets))
	}
	for _, sh := range plan.Sheets {
		if !plainName.MatchString(sh.Task) {
			return nil, fmt.Errorf("sheet %d of the plan: the task %q goes into a file name, so it is lowercase letters and digits joined by hyphens", sh.Number, sh.Task)
		}
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	rc := &Recorder{bank: bank, plan: plan, dir: dir, now: time.Now}
	// The names go on from the latest file already in the directory, so the last save of a sheet is
	// the last by name even when the clock stands behind that file at this start.
	_, last, err := rc.saves()
	if err != nil {
		return nil, err
	}
	rc.last = last
	return rc, nil
}

// Handler routes the record mode's requests, behind the same guard as the check board's. addr is
// the address the board listens on, for the message that turns a request away.
func (rc *Recorder) Handler(addr string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", servePage("record.html"))
	mux.HandleFunc("GET /api/record/plan", rc.planHandler)
	mux.HandleFunc("POST /api/record/propose", rc.propose)
	mux.HandleFunc("POST /api/record/check", rc.check)
	mux.HandleFunc("POST /api/record/save", rc.save)
	return guard(addr, mux)
}

// PlanView is the plan as the record page shows it: the size of the sheet and every plan sheet.
type PlanView struct {
	Sheet  sample.Sheet `json:"sheet"`
	Sheets []SheetView  `json:"sheets"`
}

// SheetView is a plan sheet with what the page needs besides: the statement of its task and its
// traps, for the author to pick the trap a mistake falls into, and how many times it has been
// saved.
type SheetView struct {
	sample.PlanSheet
	Statement string        `json:"statement"`
	Traps     []sample.Trap `json:"traps"`
	Saved     int           `json:"saved"`
}

func (rc *Recorder) planHandler(w http.ResponseWriter, _ *http.Request) {
	saved, _, err := rc.saves()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "the saved sheets cannot be counted: "+err.Error())
		return
	}
	view := PlanView{Sheet: sample.Sheet{Width: sheetWidth, Height: sheetHeight}, Sheets: make([]SheetView, len(rc.plan.Sheets))}
	for i, sh := range rc.plan.Sheets {
		task := rc.bank.Task(sh.Task)
		view.Sheets[i] = SheetView{PlanSheet: sh, Statement: task.Statement, Traps: append([]sample.Trap{}, task.Traps...), Saved: saved[sh.Number]}
	}
	writeJSON(w, http.StatusOK, view)
}

// saves reads the names in the directory: how many saved files each plan sheet has — those named
// by its number, its task and its scenario, so that a file of another plan does not count — and
// the latest time the name of any saved sheet holds.
func (rc *Recorder) saves() (map[int]int, time.Time, error) {
	entries, err := os.ReadDir(rc.dir)
	if err != nil {
		return nil, time.Time{}, err
	}
	count := map[int]int{}
	var last time.Time
	for _, e := range entries {
		m := savedName.FindStringSubmatch(e.Name())
		if m == nil || !e.Type().IsRegular() {
			continue
		}
		at, err := time.Parse("20060102-150405", m[2])
		if err != nil {
			continue
		}
		ms, _ := strconv.Atoi(m[3])
		if at = at.Add(time.Duration(ms) * time.Millisecond); at.After(last) {
			last = at
		}
		number, _ := strconv.Atoi(m[1])
		if number >= 1 && number <= len(rc.plan.Sheets) {
			if sh := rc.plan.Sheets[number-1]; m[4] == sh.Task+"-"+string(sh.Scenario) {
				count[number]++
			}
		}
	}
	return count, last, nil
}

// Note is a problem or a difference, at the path of its field in the request: "number", or a
// field of the sample, such as "sample.strokes[3].points[12].t_ms".
type Note struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// refused is a request the board turns away, with every problem it found at its field.
type refused struct {
	Error    string `json:"error"`
	Problems []Note `json:"problems"`
}

func refuse(w http.ResponseWriter, problems ...Note) {
	writeJSON(w, http.StatusUnprocessableEntity, refused{Error: "the sheet does not pass the check", Problems: problems})
}

// sheet returns the plan sheet of a number, or refuses the request.
func (rc *Recorder) sheet(w http.ResponseWriter, number int) *sample.PlanSheet {
	if number < 1 || number > len(rc.plan.Sheets) {
		refuse(w, Note{"number", fmt.Sprintf("%d is not a sheet of the plan, 1…%d", number, len(rc.plan.Sheets))})
		return nil
	}
	return &rc.plan.Sheets[number-1]
}

// ProposeRequest is a sheet as it stands when the author is done writing it: its number in the
// plan, its strokes and its erasures.
type ProposeRequest struct {
	Number   int              `json:"number"`
	Strokes  []sample.Stroke  `json:"strokes"`
	Erasures []sample.Erasure `json:"erasures"`
	// Undone says, for each erasure in its order, whether Undo made it rather than a move of the
	// eraser. What Undo takes away is a slip, never the planned erasure (R25).
	Undone []bool `json:"undone"`
}

// Proposal is the markup the board proposes for a sheet. Its lines are found among the strokes
// still on the sheet, by the gaps between them (lines.Group), and take the texts and the answer of
// the plan sheet's lines in their order. With them come the plan sheet's outcome and how the sheet
// differs from its plan sheet, such as by a line more.
//
// On a sheet whose plan sheet plans an erasure, Planned is the index of the erasure the board
// takes for it: of the moves of the eraser, the one that took the most strokes from the plan's
// line, the earlier of two that took as many. A stroke stood on the line its band stands on
// (lines.On). Planned is null when none took any.
type Proposal struct {
	Lines       []sample.Line   `json:"lines"`
	Expected    sample.Expected `json:"expected"`
	Planned     *int            `json:"planned"`
	Differences []Note          `json:"differences"`
}

func (rc *Recorder) propose(w http.ResponseWriter, r *http.Request) {
	var req ProposeRequest
	if !readJSON(w, r, &req) {
		return
	}
	sh := rc.sheet(w, req.Number)
	if sh == nil {
		return
	}
	byID := make(map[string]sample.Stroke, len(req.Strokes))
	for _, s := range req.Strokes {
		byID[s.ID] = s
	}
	erased := map[string]bool{}
	for _, e := range req.Erasures {
		for _, id := range e.Strokes {
			erased[id] = true
		}
	}
	var onSheet []sample.Stroke
	for _, s := range req.Strokes {
		if !erased[s.ID] {
			onSheet = append(onSheet, s)
		}
	}
	found := lines.Group(onSheet, lines.Gap)
	p := Proposal{Lines: make([]sample.Line, len(found)), Expected: sh.Expected, Differences: []Note{}}
	for i, l := range found {
		p.Lines[i] = sample.Line{Strokes: l.Strokes, Answer: i+1 == sh.Answer}
		if i < len(sh.Lines) {
			p.Lines[i].Text = sh.Lines[i]
		}
	}
	switch n, want := len(found), len(sh.Lines); {
	case n == 0:
		p.Differences = append(p.Differences, Note{"sample.lines", fmt.Sprintf("the sheet has no lines yet, and its plan sheet %d", want)})
	case n > want:
		p.Differences = append(p.Differences, Note{"sample.lines", fmt.Sprintf("the sheet has %d lines, and its plan sheet %d: a stray stroke may stand on a line of its own", n, want)})
	case n < want:
		p.Differences = append(p.Differences, Note{"sample.lines", fmt.Sprintf("the sheet has %d lines, and its plan sheet %d: a long stroke may join two lines, or a line is not written", n, want)})
	}
	if pe := sh.Erasure; pe != nil {
		p.Planned, p.Differences = plannedOf(takenFrom(byID, req.Erasures, found), req.Undone, pe.Line, p.Differences)
	}
	writeJSON(w, http.StatusOK, p)
}

// taken is what an erasure took from the lines found: how many of its strokes stood on each line,
// by the line's number, and the line most of them stood on, the higher of two that hold as many,
// or 0 when none stood on a line.
type taken struct {
	from map[int]int
	line int
}

// takenFrom finds what each erasure took from the lines found, stroke by stroke (lines.On).
func takenFrom(byID map[string]sample.Stroke, erasures []sample.Erasure, found []lines.Line) []taken {
	out := make([]taken, len(erasures))
	for i, e := range erasures {
		t := taken{from: map[int]int{}}
		for _, id := range e.Strokes {
			if top, bottom, ok := lines.Band(byID[id]); ok {
				if n := lines.On(top, bottom, found, lines.Gap) + 1; n > 0 {
					t.from[n]++
				}
			}
		}
		for n, k := range t.from {
			if k > t.from[t.line] || (k == t.from[t.line] && n < t.line) {
				t.line = n
			}
		}
		out[i] = t
	}
	return out
}

// plannedOf picks the erasure the board takes for the planned one of a plan sheet, whose line is
// line: of the moves of the eraser — Undo makes slips — the one that took the most strokes from the
// line, the earlier of two that took as many. It names a sheet with no move of the eraser and one
// where more than one took strokes from the line, as the planned erasure is one move (R25).
func plannedOf(erasures []taken, undone []bool, line int, notes []Note) (*int, []Note) {
	var planned *int
	moves, from := 0, 0
	for i, t := range erasures {
		if i < len(undone) && undone[i] {
			continue
		}
		moves++
		if t.from[line] == 0 {
			continue
		}
		from++
		if planned == nil || t.from[line] > erasures[*planned].from[line] {
			planned = &i
		}
	}
	switch {
	case moves == 0:
		notes = append(notes, Note{"sample.erasures", fmt.Sprintf("the sheet has no move of the eraser, and its plan sheet plans an erasure on line %d: Undo makes slips only, so write the sheet again, erasing in one move of the eraser", line)})
	case from > 1:
		notes = append(notes, Note{"sample.erasures", fmt.Sprintf("%d moves of the eraser took strokes from line %d, and the planned erasure is one move: if the eraser was lifted midway, write the sheet again", from, line)})
	}
	return planned, notes
}

// SaveRequest is a sheet to save: the number of its plan sheet and the sample document, which the
// board reads with the format's strict reader.
type SaveRequest struct {
	Number int             `json:"number"`
	Sample json.RawMessage `json:"sample"`
}

// Saved is what a save did: the name of the new file, how many files the sheet has now, and how its
// markup differs from its plan sheet, which the board names but does not refuse (R25).
type Saved struct {
	File        string `json:"file"`
	Saved       int    `json:"saved"`
	Differences []Note `json:"differences"`
}

// Checked is what a check of a sheet found: the problems that would keep it from being saved, and
// how its markup differs from its plan sheet; a sheet with problems has no differences named.
// ErasureLines says, for each erasure in its order, on which line of the markup most of its
// strokes stood, the higher of two that hold as many, or 0 when none did: the page shows it by the
// erasure, and gives it whatever the markup's problems.
type Checked struct {
	Problems     []Note `json:"problems"`
	Differences  []Note `json:"differences"`
	ErasureLines []int  `json:"erasure_lines"`
}

// check reads a sheet's sample as a save would, and answers what keeps it from being saved and how
// it differs from its plan sheet, without saving it. The page asks after every change of the
// markup, so that each rule of the format comes in words before the save.
func (rc *Recorder) check(w http.ResponseWriter, r *http.Request) {
	var req SaveRequest
	if !readJSON(w, r, &req) {
		return
	}
	sh := rc.sheet(w, req.Number)
	if sh == nil {
		return
	}
	c := Checked{Problems: []Note{}, Differences: []Note{}, ErasureLines: []int{}}
	if s, problems := rc.examine(sh, req.Sample); len(problems) > 0 {
		c.Problems = problems
	} else {
		c.Differences = differences(sh, s)
	}
	// The lines of the erasures are geometry, not rules: they come from the markup as it is, even
	// one the format refuses.
	var loose sample.Sample
	if json.Unmarshal(req.Sample, &loose) == nil {
		for _, e := range loose.Erasures {
			c.ErasureLines = append(c.ErasureLines, fromLine(&loose, e))
		}
	}
	writeJSON(w, http.StatusOK, c)
}

// examine reads the sample sent for a plan sheet with the format's strict reader and checks it on
// its own, against the plan sheet and against the bank. It returns the sample, or every problem
// it found at its path in the request.
func (rc *Recorder) examine(sh *sample.PlanSheet, raw json.RawMessage) (*sample.Sample, []Note) {
	if len(raw) == 0 {
		return nil, []Note{{"sample", "is missing: the sample document of the sheet"}}
	}
	s, err := sample.ReadSample(bytes.NewReader(raw))
	if err != nil {
		return nil, notesOf(err)
	}
	if problems := rc.against(sh, s); len(problems) > 0 {
		return nil, problems
	}
	return s, nil
}

func (rc *Recorder) save(w http.ResponseWriter, r *http.Request) {
	var req SaveRequest
	if !readJSON(w, r, &req) {
		return
	}
	sh := rc.sheet(w, req.Number)
	if sh == nil {
		return
	}
	s, problems := rc.examine(sh, req.Sample)
	if len(problems) > 0 {
		refuse(w, problems...)
		return
	}

	rc.saving.Lock()
	defer rc.saving.Unlock()
	name, err := rc.write(sh, s)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "the sheet is not saved: "+err.Error())
		return
	}
	saved, _, err := rc.saves()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "the sheet is saved as "+name+", but the saved sheets cannot be counted: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, Saved{File: name, Saved: saved[sh.Number], Differences: differences(sh, s)})
}

// notesOf names the problems of a sample the format refused by their paths in the request.
func notesOf(err error) []Note {
	var fe *sample.FormatError
	if !errors.As(err, &fe) {
		return []Note{{"sample", err.Error()}}
	}
	notes := make([]Note, len(fe.Problems))
	for i, p := range fe.Problems {
		path := "sample"
		if p.Path != sample.Root {
			path += "." + p.Path
		}
		notes[i] = Note{path, p.Message}
	}
	return notes
}

// against checks a sample the format took against its plan sheet and the bank: it solves the plan
// sheet's task, in its scenario, the author writes it in their own hand, on the board's sheet, and
// the trap it names is one of its task's.
func (rc *Recorder) against(sh *sample.PlanSheet, s *sample.Sample) []Note {
	var notes []Note
	if s.Task != sh.Task {
		notes = append(notes, Note{"sample.task", fmt.Sprintf("is %q, but sheet %d of the plan solves %q", s.Task, sh.Number, sh.Task)})
	}
	if s.Scenario != sh.Scenario {
		notes = append(notes, Note{"sample.scenario", fmt.Sprintf("is %q, but sheet %d of the plan is of the scenario %q", s.Scenario, sh.Number, sh.Scenario)})
	}
	if s.Writer != sample.WriterAuthor {
		notes = append(notes, Note{"sample.writer", fmt.Sprintf("is %q, but the author writes every sheet of the plan in their own hand, %q", s.Writer, sample.WriterAuthor)})
	}
	if s.Sheet != (sample.Sheet{Width: sheetWidth, Height: sheetHeight}) {
		notes = append(notes, Note{"sample.sheet", fmt.Sprintf("is %v × %v, not the board's %d × %d", s.Sheet.Width, s.Sheet.Height, sheetWidth, sheetHeight)})
	}
	if err := s.CheckWithBank(rc.bank); err != nil {
		notes = append(notes, notesOf(err)...)
	}
	return notes
}

// differences names how the markup of a sheet differs from its plan sheet: in its lines, its
// answer, its outcome, its first error, its trap and its planned erasure. The board saves such a
// sheet all the same: what was written, the author knows (R25).
func differences(sh *sample.PlanSheet, s *sample.Sample) []Note {
	notes := []Note{}
	add := func(path, format string, args ...any) {
		notes = append(notes, Note{path, fmt.Sprintf(format, args...)})
	}
	if len(s.Lines) != len(sh.Lines) {
		add("sample.lines", "are %d, and the plan sheet's %d", len(s.Lines), len(sh.Lines))
	}
	answer := 0
	for i, l := range s.Lines {
		path := fmt.Sprintf("sample.lines[%d]", i)
		if i < len(sh.Lines) && l.Text != sh.Lines[i] {
			add(path+".text", "is %q, and the plan sheet's line %d is %q", l.Text, i+1, sh.Lines[i])
		}
		if l.Illegible {
			add(path+".illegible", "is set, and the plan sheet marks no line illegible")
		}
		if l.CrossedOut {
			add(path+".crossed_out", "is set, and the plan sheet marks no line crossed out")
		}
		if l.Answer {
			answer = i + 1
		}
	}
	if answer != sh.Answer {
		add("sample.lines", "give the answer on %s, and the plan sheet on line %d", lineOrNone(answer), sh.Answer)
	}
	e, plan := s.Expected, sh.Expected
	if e.Outcome != plan.Outcome {
		add("sample.expected.outcome", "is %q, and the plan sheet's %q", e.Outcome, plan.Outcome)
	}
	if first, planned := lineNumber(e.FirstError), lineNumber(plan.FirstError); first != planned {
		add("sample.expected.first_error", "is on %s, and the plan sheet's on %s", lineOrNone(first), lineOrNone(planned))
	}
	if e.Trap != plan.Trap {
		add("sample.expected.trap", "is %q, and the plan sheet's %q", e.Trap, plan.Trap)
	}
	if pe := sh.Erasure; pe != nil {
		for i, er := range s.Erasures {
			if er.Planned == nil {
				continue
			}
			path := fmt.Sprintf("sample.erasures[%d].planned", i)
			// What the planned erasure took away, the scenario sets, so it cannot differ from the plan's.
			if er.Planned.Line != pe.Line {
				add(path+".line", "is %d, and the plan sheet's %d", er.Planned.Line, pe.Line)
			}
			if er.Planned.Text != pe.Text {
				add(path+".text", "is %q, and the plan sheet's %q", er.Planned.Text, pe.Text)
			}
			if from := fromLine(s, er); from != er.Planned.Line {
				add(path+".line", "is %d, but the erasure took its strokes from %s", er.Planned.Line, lineOrNone(from))
			}
		}
	}
	return notes
}

// fromLine is the number of the sample's line that most of an erasure's strokes stood on, by the
// bands of the lines (lines.On), the higher on the sheet of two that hold as many, or 0 when none
// of them stood on a line. The sample's lines need not stand in order down the sheet.
func fromLine(s *sample.Sample, e sample.Erasure) int {
	byID := make(map[string]sample.Stroke, len(s.Strokes))
	for _, st := range s.Strokes {
		byID[st.ID] = st
	}
	// found holds the lines with a band, top to bottom; number gives each its number in the sample.
	var found []lines.Line
	var number []int
	for i, l := range s.Lines {
		var strokes []sample.Stroke
		for _, id := range l.Strokes {
			strokes = append(strokes, byID[id])
		}
		if top, bottom, ok := lines.Band(strokes...); ok {
			found = append(found, lines.Line{Top: top, Bottom: bottom})
			number = append(number, i+1)
		}
	}
	order := make([]int, len(found))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int { return cmp.Compare(found[a].Top, found[b].Top) })
	sorted := make([]lines.Line, len(found))
	for i, at := range order {
		sorted[i] = found[at]
	}
	if n := takenFrom(byID, []sample.Erasure{e}, sorted)[0].line; n > 0 {
		return number[order[n-1]]
	}
	return 0
}

// lineNumber is the number a markup field names, or 0 for none.
func lineNumber(n *int) int {
	if n == nil {
		return 0
	}
	return *n
}

// lineOrNone names a line by its number, or says there is none.
func lineOrNone(n int) string {
	if n == 0 {
		return "no line"
	}
	return "line " + strconv.Itoa(n)
}

// write saves a sheet in a new file, named by the plan sheet's number, the time of the save in UTC
// to the millisecond, the task and the scenario. Saves go one at a time. When the clock has not
// moved on since the last save, the time in the name does by a millisecond, and so it does when
// another board writing to the directory has taken the name: no two saves share a name, and the
// last save of a sheet is the last by name.
func (rc *Recorder) write(sh *sample.PlanSheet, s *sample.Sample) (string, error) {
	data, err := sample.Marshal(s)
	if err != nil {
		return "", err
	}
	at := rc.now().UTC().Truncate(time.Millisecond)
	if !at.After(rc.last) {
		at = rc.last.Add(time.Millisecond)
	}
	for range 100 {
		name := fmt.Sprintf("%02d-%s-%03d-%s-%s.json", sh.Number, at.Format("20060102-150405"), at.Nanosecond()/int(time.Millisecond), sh.Task, sh.Scenario)
		err = createNew(filepath.Join(rc.dir, name), data)
		if errors.Is(err, fs.ErrExist) {
			at = at.Add(time.Millisecond)
			continue
		}
		if err != nil {
			return "", err
		}
		rc.last = at
		return name, nil
	}
	return "", err
}
