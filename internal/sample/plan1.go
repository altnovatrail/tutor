package sample

import "encoding/json"

// A sheet of a plan of version PlanParts is made of parts: lines of text, drawings and notes at the
// side, as a free-form solution is written (T56). In memory such a sheet keeps its parts in Parts
// and their texts in Lines, so that whatever reads the lines of a plan sheet — the check of its
// markup, sheets.Differences, the record mode's proposal — reads the parts in their order.

// PartKind is what a part of a sheet is.
type PartKind string

// The kinds of a part.
const (
	// PartText is text: a line, or a calculation in a column.
	PartText PartKind = "text"
	// PartDrawing is a drawing: a number line, a row of objects or a scheme of segments.
	PartDrawing PartKind = "drawing"
)

// Place is where a part stands on the sheet.
type Place string

// The places of a part.
const (
	// PlaceLeft is the main column of the solution.
	PlaceLeft Place = "left"
	// PlaceRight is a note at the side.
	PlaceRight Place = "right"
	// PlaceWide is across the sheet.
	PlaceWide Place = "wide"
)

// PlanPart is one part of a sheet, in the order the solution is read.
type PlanPart struct {
	Kind  PartKind `json:"kind"`
	Place Place    `json:"place"`
	// Text is the text of a part of text and the description in words of a drawing: the text of
	// the part's line in the sample's markup.
	Text string `json:"text"`
	// Figure is what code draws the part by: one of the three drawings for a drawing, and for
	// text nothing, or the column it is calculated in.
	Figure Figure `json:"figure"`
}

// Figure is the figure of a part. Of its four fields, a drawing sets one of the first three and
// text sets none or Column alone.
type Figure struct {
	NumberLine *NumberLine `json:"number_line"`
	Row        *Row        `json:"row"`
	Scheme     *Scheme     `json:"scheme"`
	Column     *Column     `json:"column"`
}

// NumberLine is a number line: divisions from From to To by Step, the divisions signed with their
// values, and the jumps drawn over it as arcs, in the order they are drawn.
type NumberLine struct {
	From   int    `json:"from"`
	To     int    `json:"to"`
	Step   int    `json:"step"`
	Labels []int  `json:"labels"`
	Jumps  []Jump `json:"jumps"`
}

// Jump is an arc of a number line from one division to another.
type Jump struct {
	From int `json:"from"`
	To   int `json:"to"`
}

// Row is a row of objects with one of them marked: Left objects to its left and Right to its
// right, and, with Numbers, each object numbered under it from the left.
type Row struct {
	Object  string `json:"object"`
	Left    int    `json:"left"`
	Right   int    `json:"right"`
	Numbers bool   `json:"numbers"`
}

// ObjectHouse is the one object a row holds so far: a house, as in the task of the row of houses.
const ObjectHouse = "house"

// Scheme is a scheme of segments, one under another from the top, and the label of the brace that
// joins them, empty for none.
type Scheme struct {
	Segments []Segment `json:"segments"`
	Total    string    `json:"total"`
}

// Segment is a segment of a scheme: its label, written before it, and its pieces from the left.
type Segment struct {
	Label  string  `json:"label"`
	Pieces []Piece `json:"pieces"`
}

// Piece is a piece of a segment: its length in the scheme's units and its label, written over it.
type Piece struct {
	Length int    `json:"length"`
	Label  string `json:"label"`
}

// Column is a calculation in a column: Top, under it Op and Bottom, a bar and Result under the bar.
// The part's text is the column written in a line: "14 × 3 = 42".
type Column struct {
	Top    string `json:"top"`
	Op     string `json:"op"`
	Bottom string `json:"bottom"`
	Result string `json:"result"`
}

// ColumnOps are the signs a column takes, as the bank writes them.
var ColumnOps = []string{"+", "−", "×"}

// How big a figure is at most: about as much as a sheet of 1000 × 600 holds, legibly, beside the
// rest of the solution.
const (
	// MaxDivisions is how many divisions a number line has at most, with their labels.
	MaxDivisions = 31
	// MaxObjects is how many objects a row holds at most, the marked one with them.
	MaxObjects = 15
	// MaxSegments is how many segments a scheme has at most, and MaxLength how many units long a
	// segment is at most, its pieces together.
	MaxSegments = 6
	MaxLength   = 40
)

// HasParts reports whether a plan is of version PlanParts, its sheets made of parts.
func (pl *Plan) HasParts() bool {
	return pl != nil && pl.Version != nil && *pl.Version == PlanParts
}

// SetParts gives a sheet of a plan of version PlanParts its parts, and their texts as its lines.
func (sh *PlanSheet) SetParts(parts []PlanPart) {
	sh.Parts = parts
	sh.Lines = texts(parts)
}

func texts(parts []PlanPart) []string {
	out := make([]string, len(parts))
	for i, p := range parts {
		out[i] = p.Text
	}
	return out
}

// MarshalJSON writes a plan in the shape of its version: a plan of version PlanParts with parts,
// any other with lines, as encoding/json writes it.
func (pl Plan) MarshalJSON() ([]byte, error) {
	if pl.HasParts() {
		return json.Marshal(pl.file1())
	}
	type plan0 Plan // the same fields without this method
	return json.Marshal(plan0(pl))
}

// plan1, planSheet1 and planErasure1 are the file of a plan of version PlanParts: a plan sheet has
// parts where version 0 has lines, and the planned erasure of a drawing the figure as it was
// before the erasure.
type plan1 struct {
	Format  string       `json:"format"`
	Version *int         `json:"version"`
	Sheets  []planSheet1 `json:"sheets"`
}

type planSheet1 struct {
	Number   int           `json:"number"`
	Task     string        `json:"task"`
	Scenario Scenario      `json:"scenario"`
	Parts    []PlanPart    `json:"parts"`
	Answer   int           `json:"answer"`
	Expected Expected      `json:"expected"`
	Erasure  *planErasure1 `json:"erasure"`
}

type planErasure1 struct {
	Line        int     `json:"line"`
	Erased      Erased  `json:"erased"`
	Text        string  `json:"text"`
	Replacement string  `json:"replacement"`
	PartOfLine  bool    `json:"part_of_line"`
	Figure      *Figure `json:"figure"`
}

// plan is the plan in memory: the parts of each sheet, and their texts as its lines.
func (f *plan1) plan() *Plan {
	pl := &Plan{Format: f.Format, Version: f.Version, Sheets: make([]PlanSheet, len(f.Sheets))}
	for i, s := range f.Sheets {
		sh := PlanSheet{Number: s.Number, Task: s.Task, Scenario: s.Scenario, Answer: s.Answer, Expected: s.Expected}
		sh.SetParts(s.Parts)
		if e := s.Erasure; e != nil {
			sh.Erasure = &PlanErasure{Line: e.Line, Erased: e.Erased, Text: e.Text, Replacement: e.Replacement, PartOfLine: e.PartOfLine, Figure: e.Figure}
		}
		pl.Sheets[i] = sh
	}
	return pl
}

// file1 is the file of a plan of version PlanParts. A slice the plan leaves nil is written empty,
// for the reader takes null for no list.
func (pl *Plan) file1() plan1 {
	f := plan1{Format: pl.Format, Version: pl.Version, Sheets: make([]planSheet1, len(pl.Sheets))}
	for i, sh := range pl.Sheets {
		s := planSheet1{Number: sh.Number, Task: sh.Task, Scenario: sh.Scenario, Parts: make([]PlanPart, len(sh.Parts)), Answer: sh.Answer, Expected: sh.Expected}
		for j, p := range sh.Parts {
			p.Figure = p.Figure.filled()
			s.Parts[j] = p
		}
		if e := sh.Erasure; e != nil {
			s.Erasure = &planErasure1{Line: e.Line, Erased: e.Erased, Text: e.Text, Replacement: e.Replacement, PartOfLine: e.PartOfLine}
			if e.Figure != nil {
				before := e.Figure.filled()
				s.Erasure.Figure = &before
			}
		}
		f.Sheets[i] = s
	}
	return f
}

// filled is a copy of the figure with every list it leaves nil made empty.
func (f Figure) filled() Figure {
	if nl := f.NumberLine; nl != nil {
		c := *nl
		c.Labels, c.Jumps = orEmpty(c.Labels), orEmpty(c.Jumps)
		f.NumberLine = &c
	}
	if s := f.Scheme; s != nil {
		c := *s
		c.Segments = make([]Segment, len(s.Segments))
		for i, seg := range s.Segments {
			seg.Pieces = orEmpty(seg.Pieces)
			c.Segments[i] = seg
		}
		f.Scheme = &c
	}
	return f
}

func orEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
