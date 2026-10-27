package sample

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

// checkPartsSheet checks a sheet of a plan of version PlanParts: the rules of a plan sheet and of
// a sample's markup on the texts of its parts, as for the lines of version 0, and then its parts,
// their figures and its planned erasure.
func checkPartsSheet(p *problems, path string, number int, sh *PlanSheet) {
	if !slices.Equal(sh.Lines, texts(sh.Parts)) {
		p.add(path, "the texts of its parts are not its Lines: a plan built in code gives a sheet its parts with SetParts")
	}
	// The rules of version 0 go over the texts of the parts, whatever the lines say.
	c := *sh
	c.Lines = texts(sh.Parts)
	checkPlanSheet(p, path, number, &c, partWords)

	sound := make([]bool, len(sh.Parts))
	for i, part := range sh.Parts {
		sound[i] = checkPart(p, fmt.Sprintf("%s.parts[%d]", path, i), part)
	}
	if n := sh.Answer; n >= 1 && n <= len(sh.Parts) && sh.Parts[n-1].Kind == PartDrawing {
		p.add(path+".answer", "part %d is a drawing: the answer is written in text", n)
	}
	if e := sh.Erasure; e != nil && e.Line >= 1 && e.Line <= len(sh.Parts) && sound[e.Line-1] {
		checkErasedFigure(p, path+".erasure", e, sh.Parts[e.Line-1])
	}
}

// checkPart checks a part and reports whether it is sound enough for the checks that rest on it:
// its kind known and its figure passing.
func checkPart(p *problems, path string, part PlanPart) bool {
	before := len(p.list)
	if part.Kind != PartText && part.Kind != PartDrawing {
		p.add(path+".kind", "%q is not %q or %q", part.Kind, PartText, PartDrawing)
		return false
	}
	if part.Place != PlaceLeft && part.Place != PlaceRight && part.Place != PlaceWide {
		p.add(path+".place", "%q is not %q, %q or %q", part.Place, PlaceLeft, PlaceRight, PlaceWide)
	}
	checkFigure(p, path+".figure", part.Figure, part.Kind)
	if c := part.Figure.Column; c != nil && part.Kind == PartText {
		if want := c.Top + " " + c.Op + " " + c.Bottom + " = " + c.Result; part.Text != want {
			p.add(path+".text", "%q is not its column written out, %q", part.Text, want)
		}
	}
	return len(p.list) == before
}

// checkFigure checks a figure for a part of a kind: which of its fields are set, and what is in
// them.
func checkFigure(p *problems, path string, f Figure, kind PartKind) {
	drawings := 0
	for _, set := range []bool{f.NumberLine != nil, f.Row != nil, f.Scheme != nil} {
		if set {
			drawings++
		}
	}
	switch {
	case kind == PartDrawing && drawings != 1:
		p.add(path, "sets %d of number_line, row and scheme: a drawing is one of them", drawings)
	case kind == PartText && drawings != 0:
		p.add(path, "sets a drawing, but the part is text: text has no figure or a column alone")
	}
	if f.Column != nil && kind == PartDrawing {
		p.add(path+".column", "is set, but the part is a drawing: a column is a calculation in text")
	}
	if nl := f.NumberLine; nl != nil {
		checkNumberLine(p, path+".number_line", *nl)
	}
	if r := f.Row; r != nil {
		checkRow(p, path+".row", *r)
	}
	if s := f.Scheme; s != nil {
		checkScheme(p, path+".scheme", *s)
	}
	if c := f.Column; c != nil && kind == PartText {
		checkColumn(p, path+".column", *c)
	}
}

func checkNumberLine(p *problems, path string, nl NumberLine) {
	switch {
	case nl.Step < 1:
		p.add(path+".step", "%d is below 1", nl.Step)
		return
	case nl.To <= nl.From:
		p.add(path+".to", "%d is not above from, %d", nl.To, nl.From)
		return
	}
	// A span below 0 is one too wide for an int.
	span := nl.To - nl.From
	if span < 0 || span/nl.Step >= MaxDivisions {
		p.add(path, "has more than %d divisions, from %d to %d by %d: a sheet holds %d at most", MaxDivisions, nl.From, nl.To, nl.Step, MaxDivisions)
	}
	if span >= 0 && span%nl.Step != 0 {
		p.add(path+".to", "%d is not a division: the number line goes from %d by %d", nl.To, nl.From, nl.Step)
		return
	}
	// The range comes first: Go's % keeps the sign of a value below from.
	on := func(v int) bool { return v >= nl.From && v <= nl.To && (v-nl.From)%nl.Step == 0 }
	division := func(at string, v int) bool {
		if !on(v) {
			p.add(at, "%d is not a division of the number line, from %d to %d by %d", v, nl.From, nl.To, nl.Step)
			return false
		}
		return true
	}
	signed := map[int]bool{}
	for i, v := range nl.Labels {
		at := fmt.Sprintf("%s.labels[%d]", path, i)
		if division(at, v) && signed[v] {
			p.add(at, "%d is signed twice", v)
		}
		signed[v] = true
	}
	for i, j := range nl.Jumps {
		at := fmt.Sprintf("%s.jumps[%d]", path, i)
		from, to := division(at+".from", j.From), division(at+".to", j.To)
		if from && to && j.From == j.To {
			p.add(at, "starts and ends at %d: a jump goes from one division to another", j.From)
		}
	}
}

func checkRow(p *problems, path string, r Row) {
	if r.Object != ObjectHouse {
		p.add(path+".object", "%q is not %q, the one object a row holds so far", r.Object, ObjectHouse)
	}
	if r.Left < 0 {
		p.add(path+".left", "%d is below 0", r.Left)
	}
	if r.Right < 0 {
		p.add(path+".right", "%d is below 0", r.Right)
	}
	// Each side is below the limit first, so that their sum cannot leave an int.
	if r.Left >= MaxObjects || r.Right >= MaxObjects || r.Left+r.Right+1 > MaxObjects {
		p.add(path, "holds more than %d objects, %d and %d beside the marked one: a sheet holds %d at most", MaxObjects, r.Left, r.Right, MaxObjects)
	}
}

func checkScheme(p *problems, path string, s Scheme) {
	switch {
	case len(s.Segments) == 0:
		p.add(path+".segments", "are missing: a scheme has at least one segment")
	case len(s.Segments) > MaxSegments:
		p.add(path+".segments", "are %d: a sheet holds %d at most", len(s.Segments), MaxSegments)
	}
	for i, seg := range s.Segments {
		at := fmt.Sprintf("%s.segments[%d]", path, i)
		checkLabel(p, at+".label", seg.Label)
		if len(seg.Pieces) == 0 {
			p.add(at+".pieces", "are missing: a segment has at least one piece")
		}
		length := 0
		for j, pc := range seg.Pieces {
			pat := fmt.Sprintf("%s.pieces[%d]", at, j)
			switch {
			case pc.Length < 1:
				p.add(pat+".length", "%d is below 1", pc.Length)
			case pc.Length > MaxLength:
				p.add(pat+".length", "%d is above %d: a segment is %d units long at most", pc.Length, MaxLength, MaxLength)
			default:
				length += pc.Length
			}
			checkLabel(p, pat+".label", pc.Label)
		}
		if length > MaxLength {
			p.add(at, "is %d units long: a segment is %d units long at most", length, MaxLength)
		}
	}
	checkLabel(p, path+".total", s.Total)
}

// checkLabel checks a label of a scheme: a number, □ or △ for an unknown, as the author wrote in
// the pilot (T54), or nothing.
func checkLabel(p *problems, path, label string) {
	if label != "" && label != "□" && label != "△" && !digits(label) {
		p.add(path, "%q is not a number, □ or △: a label of a scheme is one of them, or empty", label)
	}
}

func checkColumn(p *problems, path string, c Column) {
	for _, n := range []struct{ name, value string }{{"top", c.Top}, {"bottom", c.Bottom}, {"result", c.Result}} {
		if !digits(n.value) {
			p.add(path+"."+n.name, "%q is not a number written in digits", n.value)
		}
	}
	if !slices.Contains(ColumnOps, c.Op) {
		signs := make([]string, len(ColumnOps))
		for i, op := range ColumnOps {
			signs[i] = sign(op)
		}
		p.add(path+".op", "%s is not %s or %s: the signs a column takes", sign(c.Op), strings.Join(signs[:len(signs)-1], ", "), signs[len(signs)-1])
	}
}

// sign names a sign with its code point, since "-" and "−" look alike.
func sign(s string) string {
	if r, n := utf8.DecodeRuneInString(s); n == len(s) && n > 0 {
		return fmt.Sprintf("%q (U+%04X)", s, r)
	}
	return fmt.Sprintf("%q", s)
}

func digits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// checkErasedFigure checks the planned erasure of a sound part against it. An erasure in a drawing
// takes some of its elements and draws new ones: it has the figure as it was before, of the same
// drawing on the same frame and not the same, and it names the erased and the drawn element in
// words, a piece of the part's description. An erasure in text has no figure.
func checkErasedFigure(p *problems, path string, e *PlanErasure, part PlanPart) {
	switch {
	case part.Kind == PartText && e.Figure != nil:
		p.add(path+".figure", "is set, but part %d is text: only an erasure in a drawing has the figure as it was before", e.Line)
		return
	case part.Kind == PartText:
		return
	case e.Figure == nil:
		p.add(path+".figure", "is missing: an erasure in the drawing of part %d has the figure as it was before the erasure", e.Line)
		return
	}
	if !e.PartOfLine {
		p.add(path+".part_of_line", "is false, but an erasure in a drawing takes some of its elements: text and replacement name the erased and the drawn one, a piece of the description")
	}
	before := len(p.list)
	checkFigure(p, path+".figure", *e.Figure, PartDrawing)
	if len(p.list) > before {
		return
	}
	switch b, f := e.Figure.Drawing(), part.Figure.Drawing(); {
	case b != f:
		p.add(path+".figure", "is a %s, and part %d a %s: an erasure in a drawing leaves the same drawing", b, e.Line, f)
	case !e.Figure.sameFrame(part.Figure):
		p.add(path+".figure", "is on another frame than part %d: an erasure takes elements of a drawing, and %s", e.Line, frameOf(f))
	case e.Figure.equal(part.Figure):
		p.add(path+".figure", "is the figure of part %d: an erasure in a drawing changes it", e.Line)
	}
}

// Drawing names the drawing a figure sets, as the plan's field does, or is empty for text.
func (f Figure) Drawing() string {
	switch {
	case f.NumberLine != nil:
		return "number_line"
	case f.Row != nil:
		return "row"
	case f.Scheme != nil:
		return "scheme"
	}
	return ""
}

// frameOf names what of a drawing no erasure takes away.
func frameOf(drawing string) string {
	switch drawing {
	case "number_line":
		return "its divisions stay: from, to and step"
	case "row":
		return "its object stays"
	}
	return "the number of its segments stays"
}

// sameFrame reports whether two figures of one drawing stand on the same frame: a number line on
// the same divisions, a row of the same objects, a scheme of as many segments.
func (f Figure) sameFrame(g Figure) bool {
	switch {
	case f.NumberLine != nil && g.NumberLine != nil:
		a, b := f.NumberLine, g.NumberLine
		return a.From == b.From && a.To == b.To && a.Step == b.Step
	case f.Row != nil && g.Row != nil:
		return f.Row.Object == g.Row.Object
	case f.Scheme != nil && g.Scheme != nil:
		return len(f.Scheme.Segments) == len(g.Scheme.Segments)
	}
	return false
}

// equal reports whether two figures are the same, an empty list and none alike.
func (f Figure) equal(g Figure) bool {
	return same(f.NumberLine, g.NumberLine, func(a, b NumberLine) bool {
		return a.From == b.From && a.To == b.To && a.Step == b.Step && slices.Equal(a.Labels, b.Labels) && slices.Equal(a.Jumps, b.Jumps)
	}) && same(f.Row, g.Row, func(a, b Row) bool { return a == b }) &&
		same(f.Scheme, g.Scheme, func(a, b Scheme) bool {
			return a.Total == b.Total && slices.EqualFunc(a.Segments, b.Segments, func(x, y Segment) bool {
				return x.Label == y.Label && slices.Equal(x.Pieces, y.Pieces)
			})
		}) && same(f.Column, g.Column, func(a, b Column) bool { return a == b })
}

func same[T any](a, b *T, eq func(T, T) bool) bool {
	if a == nil || b == nil {
		return a == b
	}
	return eq(*a, *b)
}
