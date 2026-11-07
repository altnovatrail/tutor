// Package task gives each task of tutor's measurements — a task of a bank, solved on a sheet; a page
// of the workbook; an essay — the task of spec's module format 0.3 it is, with its checks:
// formatv0.Task (T103; R68 in docs/decisions.md). module-kit takes a task of the format alone: its
// open view and that view's digest, by which a sample of the eval protocol names its task, and the
// verdicts on an observation, by which the runner gets the outcome and the trap of the truth of a
// sample's markup (sections 10, 11 and 13 of spec's contracts text; R95 spec). tutor's banks are not
// modules, and their files keep their own formats: the tasks are built here, by the kind of each
// file's tasks.
package task

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/altnovatrail/spec/types/formatv0"

	"github.com/altnovatrail/tutor/internal/sample"
)

// WorkbookStatement is the statement of each page of the workbook: its file gives the items of a
// page, and no statement.
const WorkbookStatement = "Впиши ответы в окошки."

// The closed fields that task.schema.json requires of every task and that a measurement has no use
// for: no engine picks a task of tutor's by its skills, band or difficulty, and no module gives
// them. A task's skill is the kind of its task — SkillWorkbook and SkillEssay on a page and an
// essay —, its band Band and its difficulty Difficulty. None of them is in the open view, so none
// changes its digest.
const (
	SkillWorkbook formatv0.SkillID = "workbook"
	SkillEssay    formatv0.SkillID = "essay"
	Band          formatv0.BandID  = "measurement"
	Difficulty                     = 1
)

// File is a file of the measurement's tasks: the language of its texts, and its tasks in its order.
type File struct {
	Language formatv0.LanguageTag
	Tasks    []formatv0.Task
}

// Read reads a file of tasks by the format it declares — a bank, the workbook or the essays —, as
// strictly as anywhere else (R15), and builds its tasks. A bank whose key gives no number is
// refused: the check of its sheet would have no key.
func Read(data []byte) (*File, error) {
	var head struct {
		Format string `json:"format"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return nil, fmt.Errorf("not a JSON object: %w", err)
	}
	r := bytes.NewReader(data)
	f := &File{}
	switch head.Format {
	case sample.BankFormat:
		bank, err := sample.ReadBank(r)
		if err != nil {
			return nil, err
		}
		f.Language = formatv0.LanguageTag(sample.LangRussian)
		for i := range bank.Tasks {
			if sample.KeyOf(bank.Tasks[i].Key) == "" {
				return nil, fmt.Errorf("task %s: the key %q gives no number", bank.Tasks[i].ID, bank.Tasks[i].Key)
			}
			f.Tasks = append(f.Tasks, Sheet(&bank.Tasks[i]))
		}
	case sample.WorkbookFormat:
		wb, err := sample.ReadWorkbook(r)
		if err != nil {
			return nil, err
		}
		f.Language = formatv0.LanguageTag(sample.LangRussian)
		for i := range wb.Pages {
			f.Tasks = append(f.Tasks, Page(&wb.Pages[i]))
		}
	case sample.EssaysFormat:
		es, err := sample.ReadEssays(r)
		if err != nil {
			return nil, err
		}
		f.Language = formatv0.LanguageTag(sample.LangEnglish)
		for i := range es.Essays {
			f.Tasks = append(f.Tasks, Essay(es.Rubric, &es.Essays[i]))
		}
	default:
		return nil, fmt.Errorf("format %q is not %q, %q or %q", head.Format, sample.BankFormat, sample.WorkbookFormat, sample.EssaysFormat)
	}
	return f, nil
}

// Sheet is the task of a bank, solved on a sheet: its statement; a handwriting sheet, of the
// format's default grid, as the format has no sheet without one; and one item whose one answer slot
// is anywhere on the sheet, where AI finds the final answer, as the measurement's judge and reading
// find it. The slot's check is number, with the value of the bank's key, sample.KeyOf, as the
// reading's verdict takes it. The bank's traps are the task's traps, those that AI may name from the
// kind of mistake in the work: a bank gives no values of them, so the check has no trap values and
// a wrong value is not_yet with no trap. The solution is a paragraph to a line, and the leaks are the
// leak phrases.
func Sheet(t *sample.Task) formatv0.Task {
	key := sample.KeyOf(t.Key)
	task := formatv0.Task{
		ID:        formatv0.TaskID(t.ID),
		Statement: paragraphs(t.Statement),
		Canvas:    formatv0.Canvas{Kind: formatv0.CanvasKindHandwritingSheet},
		Items: []formatv0.Item{{AnswerSlots: []formatv0.AnswerSlot{{
			Binding: formatv0.AnswerSlotBindingAnywhere,
			Check:   formatv0.Check{Kind: formatv0.CheckKindNumber, Key: &key},
		}}}},
		Solution: paragraphs(t.Solution...),
	}
	for _, trap := range t.Traps {
		task.Traps = append(task.Traps, formatv0.TrapID(trap.ID))
	}
	for _, leak := range t.Leaks {
		task.LeakPhrases = append(task.LeakPhrases, formatv0.Text(leak))
	}
	return closed(task, formatv0.SkillID(t.Kind))
}

// Page is a page of the workbook, workbook-01 for the first: WorkbookStatement, a workbook, and an
// item to each of the page's, a line of its text with a box for each "[ ]", its pieces of text
// trimmed, as the format's vectors write a line. Each box has its check: number, or fraction with
// its form, and the trap values. The format wants a solution of a task whose items have verdicts,
// and the workbook's file gives none: the solution is each item's line with the keys in its boxes.
// The page is one as sample.ReadWorkbook takes it, with a box to each "[ ]" of an item's text: Page
// panics on an item whose boxes do not match its text.
func Page(p *sample.WorkbookPage) formatv0.Task {
	items := make([]formatv0.Item, len(p.Items))
	solution := make([]string, len(p.Items))
	for i, it := range p.Items {
		if n := strings.Count(it.Text, sample.BoxMark); n != len(it.Boxes) {
			panic(fmt.Sprintf("task: page %d, item %d has %d boxes, but its text shows %d", p.Number, i+1, len(it.Boxes), n))
		}
		var line []formatv0.LinePart
		line, solution[i] = lineOf(it)
		items[i] = formatv0.Item{Line: line}
	}
	return closed(formatv0.Task{
		ID:        formatv0.TaskID(fmt.Sprintf("workbook-%02d", p.Number)),
		Statement: paragraphs(WorkbookStatement),
		Canvas:    formatv0.Canvas{Kind: formatv0.CanvasKindWorkbook},
		Items:     items,
		Solution:  paragraphs(solution...),
	}, SkillWorkbook)
}

// lineOf is the line of an item of the workbook — pieces of its text, and a box for each
// sample.BoxMark with the check of the item's box in its place — and the text of the line with the
// key of each box in its place.
func lineOf(it sample.WorkbookItem) ([]formatv0.LinePart, string) {
	var parts []formatv0.LinePart
	var solved strings.Builder
	for i, piece := range strings.Split(it.Text, sample.BoxMark) {
		if i > 0 {
			box := formatv0.LinePartBindingBox
			c := boxCheck(it.Boxes[i-1].Check)
			parts = append(parts, formatv0.LinePart{Binding: &box, Check: &c})
			solved.WriteString(it.Boxes[i-1].Check.Key)
		}
		solved.WriteString(piece)
		if piece = strings.TrimSpace(piece); piece != "" {
			text := formatv0.Text(piece)
			parts = append(parts, formatv0.LinePart{Text: &text})
		}
	}
	return parts, solved.String()
}

// boxCheck is the check of a box, as the format writes it: the workbook's check is the format's
// already (sample.BoxCheck), and a number has no form.
func boxCheck(c sample.BoxCheck) formatv0.Check {
	key := c.Key
	out := formatv0.Check{Kind: formatv0.CheckKind(c.Kind), Key: &key}
	if c.Form != nil {
		form := formatv0.CheckForm(*c.Form)
		out.Form = &form
	}
	for _, tv := range c.TrapValues {
		out.TrapValues = append(out.TrapValues, formatv0.TrapValue{Value: tv.Value, Trap: formatv0.TrapID(tv.Trap)})
	}
	return out
}

// Essay is an essay's task, essay-01 for the first: its topic, a handwriting sheet with lines, and
// one answer slot, the whole sheet, a work as a whole, whose check is the rubric: the texts of its
// criteria in its order. The format gives a criterion no ID, and the eval protocol names a criterion
// by its place in the rubric, from 1: sample.Criteria in that order. What code wrote for the essay,
// its title and text, is no part of the task.
func Essay(rubric []sample.Criterion, e *sample.Essay) formatv0.Task {
	lines := formatv0.CanvasGridLines
	check := formatv0.Check{Kind: formatv0.CheckKindRubric}
	for _, c := range rubric {
		check.Criteria = append(check.Criteria, formatv0.Criterion{Text: formatv0.Text(c.Text)})
	}
	return closed(formatv0.Task{
		ID:        formatv0.TaskID(fmt.Sprintf("essay-%02d", e.Number)),
		Statement: paragraphs(e.Topic),
		Canvas:    formatv0.Canvas{Kind: formatv0.CanvasKindHandwritingSheet, Grid: &lines},
		Items:     []formatv0.Item{{AnswerSlots: []formatv0.AnswerSlot{{Binding: formatv0.AnswerSlotBindingSheet, Check: check}}}},
	}, SkillEssay)
}

// closed gives a task the closed fields that every task has: its skill, Band and Difficulty.
func closed(t formatv0.Task, skill formatv0.SkillID) formatv0.Task {
	t.Skills = formatv0.Skills{Primary: []formatv0.SkillID{skill}}
	t.Band = Band
	t.Difficulty = Difficulty
	return t
}

// paragraphs is a paragraph to each text, or nil for none.
func paragraphs(texts ...string) []formatv0.StatementPart {
	var out []formatv0.StatementPart
	for _, text := range texts {
		t := formatv0.Text(text)
		out = append(out, formatv0.StatementPart{Text: &t})
	}
	return out
}
