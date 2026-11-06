// Package openview builds the open views of the measurement's tasks: the file that tutor's release
// carries for canvas's sample page, until modules has content (T37; R64 in docs/decisions.md; the
// 2026-10-21 amendment to R25 spec). Each task in it is the open view of a task of the module format
// 0.3, spec's open.Task: what the learner's app shows — the statement, the canvas and the items with
// their answer slots — and none of the key, the solution, the traps, the leak phrases or the checks,
// which stay with tutor (R05 spec). tutor's banks are not modules: the views are built here, by the
// kind of each file's tasks, and the file names each bank by its path in the repository and the
// digest of its bytes, by which the runner at the same tag finds a task's key.
package openview

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/altnovatrail/spec/types/formatv0"
	"github.com/altnovatrail/spec/types/formatv0/open"

	"github.com/altnovatrail/tutor/internal/sample"
)

// Format is the name the file declares in its "format" field, and Version its version.
const (
	Format  = "tutor-open-tasks"
	Version = 0
)

// ModuleFormatVersion is the version of the module format whose open views of a task the file
// holds, that of spec's v0.9.3 (R65). It is written here rather than read from spec.Format: the
// committed file names it too, so a tag that changes the format changes the file's bytes, and
// TestPatterns fails until the move writes the file anew and reviews it.
const ModuleFormatVersion formatv0.ModuleFormatVersion = "0.3"

// WorkbookStatement is the statement of each page of the workbook: its file gives the items of a
// page, and no statement.
const WorkbookStatement = "Впиши ответы в окошки."

// File is the file of open views: the banks of the measurement, each with its tasks.
type File struct {
	Format              string                       `json:"format"`
	Version             int                          `json:"version"`
	ModuleFormatVersion formatv0.ModuleFormatVersion `json:"module_format_version"`
	Banks               []Bank                       `json:"banks"`
}

// Bank is a file of the measurement's tasks: its path in the repository, the digest of its bytes,
// sha256: and the SHA-256 in hexadecimal as the format writes a digest, the language of its texts,
// and the open views of its tasks in its order.
type Bank struct {
	Bank     string               `json:"bank"`
	Digest   formatv0.Digest      `json:"digest"`
	Language formatv0.LanguageTag `json:"language"`
	Tasks    []open.Task          `json:"tasks"`
}

// Input is a file of the measurement's tasks: its path in the repository, from its root, and its
// bytes.
type Input struct {
	Path string
	Data []byte
}

// Build builds the file of open views of the given files: banks of tasks, the workbook and the
// essays, each read and checked as strictly as anywhere else (R15). The file passes Check.
func Build(inputs []Input) (*File, error) {
	f := &File{Format: Format, Version: Version, ModuleFormatVersion: ModuleFormatVersion}
	for _, in := range inputs {
		b, err := bankOf(in)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", in.Path, err)
		}
		f.Banks = append(f.Banks, *b)
	}
	if err := f.Check(); err != nil {
		return nil, err
	}
	return f, nil
}

// bankOf reads a file of tasks by the format it declares, and builds the open views of its tasks.
func bankOf(in Input) (*Bank, error) {
	var head struct {
		Format string `json:"format"`
	}
	if err := json.Unmarshal(in.Data, &head); err != nil {
		return nil, fmt.Errorf("not a JSON object: %w", err)
	}
	sum := sha256.Sum256(in.Data)
	b := &Bank{
		Bank:   path.Clean(filepath.ToSlash(in.Path)),
		Digest: formatv0.Digest("sha256:" + hex.EncodeToString(sum[:])),
	}
	r := bytes.NewReader(in.Data)
	switch head.Format {
	case sample.BankFormat:
		bank, err := sample.ReadBank(r)
		if err != nil {
			return nil, err
		}
		b.Language = formatv0.LanguageTag(sample.LangRussian)
		for i := range bank.Tasks {
			b.Tasks = append(b.Tasks, Sheet(&bank.Tasks[i]))
		}
	case sample.WorkbookFormat:
		wb, err := sample.ReadWorkbook(r)
		if err != nil {
			return nil, err
		}
		b.Language = formatv0.LanguageTag(sample.LangRussian)
		for i := range wb.Pages {
			b.Tasks = append(b.Tasks, Page(&wb.Pages[i]))
		}
	case sample.EssaysFormat:
		es, err := sample.ReadEssays(r)
		if err != nil {
			return nil, err
		}
		b.Language = formatv0.LanguageTag(sample.LangEnglish)
		for i := range es.Essays {
			b.Tasks = append(b.Tasks, Essay(&es.Essays[i]))
		}
	default:
		return nil, fmt.Errorf("format %q is not %q, %q or %q", head.Format, sample.BankFormat, sample.WorkbookFormat, sample.EssaysFormat)
	}
	return b, nil
}

// Sheet is the open view of a task of a bank, solved on a sheet: its statement; a handwriting sheet,
// of the format's default grid, as the format has no sheet without one; and one item whose one answer
// slot is anywhere on the sheet, where AI finds the final answer, as the measurement's judge and
// reading find it.
func Sheet(t *sample.Task) open.Task {
	return open.Task{
		ID:        formatv0.TaskID(t.ID),
		Statement: paragraph(t.Statement),
		Canvas:    open.Canvas{Kind: formatv0.CanvasKindHandwritingSheet},
		Items:     []open.Item{{AnswerSlots: []open.AnswerSlot{{Binding: formatv0.AnswerSlotBindingAnywhere}}}},
	}
}

// Page is the open view of a page of the workbook, workbook-01 for the first: WorkbookStatement, a
// workbook, and an item to each of the page's, a line of its text with a box for each "[ ]", its
// pieces of text trimmed, as the format's vectors write a line.
func Page(p *sample.WorkbookPage) open.Task {
	items := make([]open.Item, len(p.Items))
	for i, it := range p.Items {
		items[i] = open.Item{Line: line(it.Text)}
	}
	return open.Task{
		ID:        formatv0.TaskID(fmt.Sprintf("workbook-%02d", p.Number)),
		Statement: paragraph(WorkbookStatement),
		Canvas:    open.Canvas{Kind: formatv0.CanvasKindWorkbook},
		Items:     items,
	}
}

// line is the line of an item of the workbook: pieces of its text and a box for each sample.BoxMark.
func line(text string) []open.LinePart {
	var parts []open.LinePart
	for i, piece := range strings.Split(text, sample.BoxMark) {
		if i > 0 {
			box := formatv0.LinePartBindingBox
			parts = append(parts, open.LinePart{Binding: &box})
		}
		if piece = strings.TrimSpace(piece); piece != "" {
			t := formatv0.Text(piece)
			parts = append(parts, open.LinePart{Text: &t})
		}
	}
	return parts
}

// Essay is the open view of an essay's task, essay-01 for the first: its topic, a handwriting sheet
// with lines, and one answer slot, the whole sheet, a work as a whole. What code wrote for the essay,
// its title and text, is no part of the task, and the rubric is the check, which is closed.
func Essay(e *sample.Essay) open.Task {
	lines := formatv0.CanvasGridLines
	return open.Task{
		ID:        formatv0.TaskID(fmt.Sprintf("essay-%02d", e.Number)),
		Statement: paragraph(e.Topic),
		Canvas:    open.Canvas{Kind: formatv0.CanvasKindHandwritingSheet, Grid: &lines},
		Items:     []open.Item{{AnswerSlots: []open.AnswerSlot{{Binding: formatv0.AnswerSlotBindingSheet}}}},
	}
}

// paragraph is a statement of one paragraph.
func paragraph(text string) []open.StatementPart {
	t := formatv0.Text(text)
	return []open.StatementPart{{Text: &t}}
}

// Marshal writes the file as indented JSON, ending with a newline, as tutor's other files are.
func Marshal(f *File) ([]byte, error) { return sample.Marshal(f) }

// Read reads a file of open views and checks it. The file is to be as Marshal writes it, byte for
// byte: so no field is in it that the types do not have — a closed field of a task among them — nor a
// field in another letter case or twice, which encoding/json alone would take.
func Read(data []byte) (*File, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var f File
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("read the open tasks: %w", err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, errors.New("read the open tasks: data after the file")
	}
	if err := f.Check(); err != nil {
		return nil, err
	}
	again, err := Marshal(&f)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(again, data) {
		return nil, errors.New("read the open tasks: the file is not as tutor-eval open writes it: a field in another letter case or twice, or another layout")
	}
	return &f, nil
}

// The patterns of spec's format: a local ID and a language tag (common.schema.json), and a digest
// (inventory.schema.json).
var (
	localID     = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)
	reservedID  = regexp.MustCompile(`^(con|prn|aux|nul|com[1-9]|lpt[1-9])$`)
	languageTag = regexp.MustCompile(`^[a-z]{2,3}(-[A-Z][a-z]{3})?(-([A-Z]{2}|[0-9]{3}))?$`)
	digest      = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

// maxID is the longest local ID the format takes.
const maxID = 64

// Check checks the file and returns a *sample.FormatError listing every problem, or nil: its
// format, version and module format version; each bank named by a relative path in the repository,
// once, with its digest, its language and its tasks; each task's ID once in the file; and the rules
// of spec's format for the open view of a task that its Go types do not carry (task.schema.json and
// common.schema.json; the format text, section 14), for what the file carries: a statement of
// paragraphs, a canvas, and items of a slot bound anywhere or sheet, or of a line with boxes.
func (f *File) Check() error {
	p := &problems{}
	if f.Format != Format {
		p.add("format", "is %q, not %q", f.Format, Format)
	}
	if f.Version != Version {
		p.add("version", "%d is not supported; this reader knows %d", f.Version, Version)
	}
	if f.ModuleFormatVersion != ModuleFormatVersion {
		p.add("module_format_version", "is %q, not %q", f.ModuleFormatVersion, ModuleFormatVersion)
	}
	if len(f.Banks) == 0 {
		p.add("banks", "is empty")
	}
	banks := map[string]bool{}
	ids := map[formatv0.TaskID]string{}
	for i := range f.Banks {
		b := &f.Banks[i]
		at := fmt.Sprintf("banks[%d]", i)
		switch name := b.Bank; {
		case name == "" || path.IsAbs(name) || path.Clean(name) != name || name == ".." || strings.HasPrefix(name, "../"):
			p.add(at+".bank", "%q is not a relative path in the repository", name)
		case banks[name]:
			p.add(at+".bank", "%q is named twice", name)
		default:
			banks[name] = true
		}
		if !digest.MatchString(string(b.Digest)) {
			p.add(at+".digest", "%q is not sha256: and 64 lowercase hexadecimal digits", b.Digest)
		}
		if !languageTag.MatchString(string(b.Language)) {
			p.add(at+".language", "%q is not a language tag", b.Language)
		}
		if len(b.Tasks) == 0 {
			p.add(at+".tasks", "is empty")
		}
		for j := range b.Tasks {
			t := &b.Tasks[j]
			tat := fmt.Sprintf("%s.tasks[%d]", at, j)
			if first, ok := ids[t.ID]; ok {
				p.add(tat+".id", "%q is the ID of %s too", t.ID, first)
			} else {
				ids[t.ID] = tat
			}
			checkTask(p, tat, t)
		}
	}
	return p.err()
}

// checkTask checks the open view of a task by the rules of the format that its type does not carry,
// and that it holds only what Build writes: the measurement's tasks have no picture, givens, unknown,
// grid step, item text, zone or choice, so the file carries none of them, and Check need not repeat
// the rules of the format for them.
func checkTask(p *problems, at string, t *open.Task) {
	if id := string(t.ID); !localID.MatchString(id) || reservedID.MatchString(id) || len(id) > maxID {
		p.add(at+".id", "%q is not a local ID: kebab-case from a letter, at most %d characters, and no name Windows reserves", id, maxID)
	}
	if len(t.Statement) == 0 {
		p.add(at+".statement", "is empty")
	}
	for i, s := range t.Statement {
		sat := fmt.Sprintf("%s.statement[%d]", at, i)
		switch {
		case s.Picture != nil || s.Alt != nil || s.Width != nil || s.Height != nil:
			p.add(sat, "is a picture, and the file carries no media")
		case s.Text == nil || *s.Text == "":
			p.add(sat, "has no text")
		}
	}
	if t.Givens != nil {
		p.add(at+".givens", "the file carries no givens")
	}
	if t.Unknown != nil {
		p.add(at+".unknown", "the file carries no unknown")
	}
	if len(t.Items) == 0 {
		p.add(at+".items", "is empty")
	}
	if t.Canvas.GridStep != nil {
		p.add(at+".canvas.grid_step", "the file sets no grid step")
	}
	switch t.Canvas.Kind {
	case formatv0.CanvasKindHandwritingSheet:
		checkSheet(p, at, t)
	case formatv0.CanvasKindWorkbook:
		if t.Canvas.Grid != nil {
			p.add(at+".canvas.grid", "a workbook has no grid")
		}
		checkWorkbook(p, at, t)
	default:
		p.add(at+".canvas.kind", "%q is not %q or %q", t.Canvas.Kind, formatv0.CanvasKindHandwritingSheet, formatv0.CanvasKindWorkbook)
	}
}

// checkSheet checks a task on a handwriting sheet: its grid, and items of answer slots alone, each
// bound anywhere or sheet, the task's only slot.
func checkSheet(p *problems, at string, t *open.Task) {
	if g := t.Canvas.Grid; g != nil {
		switch *g {
		case formatv0.CanvasGridSquares, formatv0.CanvasGridLines, formatv0.CanvasGridDots:
		default:
			p.add(at+".canvas.grid", "%q is not squares, lines or dots", *g)
		}
	}
	slots := 0
	for i, it := range t.Items {
		iat := fmt.Sprintf("%s.items[%d]", at, i)
		if it.Line != nil {
			p.add(iat+".line", "an item of a handwriting sheet has answer slots, not a line")
		}
		if it.Text != nil {
			p.add(iat+".text", "the file carries no text of an item")
		}
		if len(it.AnswerSlots) == 0 {
			p.add(iat+".answer_slots", "is empty")
		}
		for j, s := range it.AnswerSlots {
			sat := fmt.Sprintf("%s.answer_slots[%d]", iat, j)
			if slots++; slots == 2 {
				p.add(sat, "a slot bound anywhere or sheet is the task's only answer slot")
			}
			if s.Binding != formatv0.AnswerSlotBindingAnywhere && s.Binding != formatv0.AnswerSlotBindingSheet {
				p.add(sat+".binding", "%q is not anywhere or sheet, the bindings of the measurement's tasks", s.Binding)
			}
			if s.Zone != nil {
				p.add(sat+".zone", "the file carries no zone")
			}
			if s.Options != nil {
				p.add(sat+".options", "the file carries no options")
			}
		}
	}
}

// checkWorkbook checks a task on a workbook: items of a line alone, each piece of it a text or a box,
// and a box in each line.
func checkWorkbook(p *problems, at string, t *open.Task) {
	for i, it := range t.Items {
		iat := fmt.Sprintf("%s.items[%d]", at, i)
		if it.Text != nil || it.AnswerSlots != nil {
			p.add(iat, "an item of a workbook is a line alone")
		}
		boxes := 0
		for j, part := range it.Line {
			pat := fmt.Sprintf("%s.line[%d]", iat, j)
			switch {
			case part.Text != nil && part.Binding != nil, part.Text == nil && part.Binding == nil:
				p.add(pat, "is a piece of text or a box")
			case part.Binding != nil && *part.Binding != formatv0.LinePartBindingBox:
				p.add(pat+".binding", "%q is not box", *part.Binding)
			case part.Text != nil && *part.Text == "":
				p.add(pat+".text", "is empty")
			case part.Binding != nil:
				boxes++
			}
		}
		if boxes == 0 {
			p.add(iat+".line", "has no box")
		}
	}
}

// problems collects what a check finds, as sample's checks do.
type problems struct {
	list []sample.Problem
}

func (p *problems) add(path, format string, args ...any) {
	p.list = append(p.list, sample.Problem{Path: path, Message: fmt.Sprintf(format, args...)})
}

func (p *problems) err() error {
	if len(p.list) == 0 {
		return nil
	}
	return &sample.FormatError{What: "file of open tasks", Problems: p.list}
}
