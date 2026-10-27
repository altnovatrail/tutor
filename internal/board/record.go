package board

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sync"
	"time"

	"github.com/altnovatrail/tutor/internal/lines"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/sheets"
)

// The board's sheet, in its logical units: the canvas of the page, on which every sheet of the
// measurement is written (R24).
const (
	sheetWidth  = 1000
	sheetHeight = 600
)

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
	if len(plan.Sheets) > sample.MaxSheets {
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
// traps, for the author to pick the trap a mistake falls into, how many times it has been saved,
// and, on a sheet of a plan of parts, its parts, whose kind and place the page shows. A sheet of
// lines has none.
type SheetView struct {
	sample.PlanSheet
	Statement string            `json:"statement"`
	Traps     []sample.Trap     `json:"traps"`
	Saved     int               `json:"saved"`
	Parts     []sample.PlanPart `json:"parts"`
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
		view.Sheets[i] = SheetView{PlanSheet: sh, Statement: task.Statement, Traps: append([]sample.Trap{}, task.Traps...), Saved: saved[sh.Number],
			Parts: append([]sample.PlanPart{}, sh.Parts...)}
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
		n, ok := sample.ParseSheetFile(e.Name())
		if !ok || !e.Type().IsRegular() {
			continue
		}
		if n.At.After(last) {
			last = n.At
		}
		if n.Number >= 1 && n.Number <= len(rc.plan.Sheets) && n.Saves(&rc.plan.Sheets[n.Number-1]) {
			count[n.Number]++
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
// differs from its plan sheet, such as by a line more. On a sheet of a plan of parts its lines are
// the parts: each group of strokes the rule of the eval protocol finds (lines.Groups) is one, in
// the order their first strokes were written, as a device groups them (R30). A drawing drawn
// after a pause comes as two groups, and the author joins them. A plan sheet of one part, a free
// solution, takes the whole sheet.
//
// On a sheet whose plan sheet plans an erasure, Planned is the index of the erasure the board
// takes for it: of the moves of the eraser, the one that took the most strokes from the plan's
// line, the earlier of two that took as many. A stroke stood on the line its band stands on
// (lines.On). On a sheet of parts it is the move that took the most strokes from any of its parts,
// by their place on the plane (lines.PartsTaken): until the author joins the groups of a part, the
// number of a group is not yet the number of the plan's part, and slips on such a sheet are Undo's
// alone. Planned is null when none took any.
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
	if len(sh.Parts) > 0 {
		writeJSON(w, http.StatusOK, proposeParts(sh, req))
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
		taken := lines.TakenFrom(byID, req.Erasures, found)
		took := make([]int, len(taken))
		for i, t := range taken {
			took[i] = t.From[pe.Line]
		}
		at := fmt.Sprintf("line %d", pe.Line)
		p.Planned, p.Differences = plannedOf(took, req.Undone, at, at, p.Differences)
	}
	writeJSON(w, http.StatusOK, p)
}

// proposeParts proposes the parts of a sheet of a plan of parts: a part to each group of strokes,
// in the order their first strokes were written, with the texts and the answer of the plan sheet's
// parts in their order. A plan sheet of one part takes the whole sheet as that part, as the author
// writes a free solution (R28, the 2026-10-27 amendment).
func proposeParts(sh *sample.PlanSheet, req ProposeRequest) Proposal {
	found := lines.Groups(req.Strokes, req.Erasures)
	if len(sh.Parts) == 1 && len(found) > 1 {
		found = []lines.StrokeGroup{whole(found, req.Strokes)}
	}
	p := Proposal{Lines: make([]sample.Line, len(found)), Expected: sh.Expected, Differences: []Note{}}
	for i, g := range found {
		p.Lines[i] = sample.Line{Strokes: g.Strokes, Answer: i+1 == sh.Answer}
		if i < len(sh.Lines) {
			p.Lines[i].Text = sh.Lines[i]
		}
	}
	switch n, want := len(found), len(sh.Lines); {
	case n == 0:
		p.Differences = append(p.Differences, Note{"sample.lines", fmt.Sprintf("the sheet has no parts yet, and its plan sheet %d", want)})
	case n > want:
		p.Differences = append(p.Differences, Note{"sample.lines", fmt.Sprintf("the sheet has %d parts, and its plan sheet %d: a part drawn on after a pause of more than %d seconds comes as two groups, so join them, or a stray stroke stands on its own", n, want, lines.GroupPause/1000)})
	case n < want:
		p.Differences = append(p.Differences, Note{"sample.lines", fmt.Sprintf("the sheet has %d parts, and its plan sheet %d: two parts written close together without a pause come as one group, so pick the strokes of one and move them to a new part, or a part is not written", n, want)})
	}
	if pe := sh.Erasure; pe != nil {
		taken := lines.PartsTaken(&sample.Sample{Strokes: req.Strokes, Lines: p.Lines}, req.Erasures)
		took := make([]int, len(taken))
		for i, t := range taken {
			for _, k := range t.From {
				took[i] += k
			}
		}
		p.Planned, p.Differences = plannedOf(took, req.Undone, fmt.Sprintf("part %d", pe.Line), "the parts of the sheet", p.Differences)
	}
	return p
}

// whole joins groups of strokes into one, its strokes in the order they were written.
func whole(groups []lines.StrokeGroup, strokes []sample.Stroke) lines.StrokeGroup {
	order := make(map[string]int, len(strokes))
	for i, st := range strokes {
		order[st.ID] = i
	}
	var all []string
	for _, g := range groups {
		all = append(all, g.Strokes...)
	}
	slices.SortFunc(all, func(a, b string) int { return order[a] - order[b] })
	return lines.StrokeGroup{Strokes: all}
}

// plannedOf picks the erasure the board takes for the planned one of a plan sheet: of the moves of
// the eraser — Undo makes slips — the one that took the most strokes from where it counts, took
// holding how many each erasure took from there, the earlier of two that took as many. It names a
// sheet with no move of the eraser and one where more than one took strokes from there, as the
// planned erasure is one move (R25). planned names the plan's place of the erasure, such as
// "line 2", and from where the strokes count.
func plannedOf(took []int, undone []bool, planned, from string, notes []Note) (*int, []Note) {
	var picked *int
	moves, taking := 0, 0
	for i, n := range took {
		if i < len(undone) && undone[i] {
			continue
		}
		moves++
		if n == 0 {
			continue
		}
		taking++
		if picked == nil || n > took[*picked] {
			picked = &i
		}
	}
	switch {
	case moves == 0:
		notes = append(notes, Note{"sample.erasures", fmt.Sprintf("the sheet has no move of the eraser, and its plan sheet plans an erasure on %s: Undo makes slips only, so write the sheet again, erasing in one move of the eraser", planned)})
	case taking > 1:
		notes = append(notes, Note{"sample.erasures", fmt.Sprintf("%d moves of the eraser took strokes from %s, and the planned erasure is one move: if the eraser was lifted midway, write the sheet again", taking, from)})
	}
	return picked, notes
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
// erasure, and gives it whatever the markup's problems. On a sheet of a plan of parts it is the
// part, by the place of the strokes on the plane (sheets.ErasurePlace).
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
			c.ErasureLines = append(c.ErasureLines, sheets.ErasurePlace(sh, &loose, e))
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

// differences names how the markup of a sheet differs from its plan sheet (sheets.Differences), at
// the paths of the request. The board saves such a sheet all the same: what was written, the author
// knows (R25).
func differences(sh *sample.PlanSheet, s *sample.Sample) []Note {
	notes := []Note{}
	for _, d := range sheets.Differences(sh, s) {
		notes = append(notes, Note{"sample." + d.Path, d.Message})
	}
	return notes
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
		name := sample.SheetFile(sh, at)
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
