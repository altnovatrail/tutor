// Package evalsample is a sheet of the measurement as a sample file of spec's eval protocol, the
// message Sample of novatrail.eval.v1 (contracts/README.md of spec, sections 11 to 13; R93, R95
// spec): Of writes a sheet that internal/synth drew as one, and Read reads one back into the sample
// that tutor's runner takes, internal/sample's, with what its plan sheet says and the protocol does
// not — the scenario, the planned erasure and the trap. Read refuses a sample as section 13 has the
// runner refuse one: one that spec's check sample-check finds invalid, one without its markup, one of
// another task, one whose markup does not fit its task, and a child's (R69).
//
// A sample that the samples page of canvas did not write, a drawn one, names canvas 0.0.0 and the
// set of texts "none", the values of no release, which sample-check asks of a synthetic sample
// (section 11; the 2026-11-07 amendment to R93 spec, spec's answer to В68 of RUN.md).
package evalsample

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	mkview "github.com/altnovatrail/module-kit/openview"
	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"
	observationv1 "github.com/altnovatrail/spec/contracts/novatrail/observation/v1"
	"github.com/altnovatrail/spec/types/formatv0"

	"github.com/altnovatrail/tutor/internal/lines"
	"github.com/altnovatrail/tutor/internal/protocol"
	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/synth"
	"github.com/altnovatrail/tutor/internal/task"
)

// What a drawn sample names that only the samples page of canvas has (section 11; the 2026-11-07
// amendment to R93 spec): no version of canvas and no set of texts, since neither wrote or showed
// anything.
const (
	CanvasVersion  = "0.0.0"
	TextSetVersion = "none"
)

// Pointer is the pointer a drawn stroke names: the synthetic hand writes as a pen does.
const Pointer = "pen"

// Bank is a bank of tasks as a sample names it, as the source file of its task reference (R69), and
// the bank it holds.
type Bank struct {
	Source
	Bank *sample.Bank
}

// ReadBank reads the bank at a path, which a sample names from the root of the repository that
// holds it (ReadSource).
func ReadBank(file string) (*Bank, error) {
	at, data, err := ReadSource(file)
	if err != nil {
		return nil, err
	}
	return BankOf(at, data)
}

// BankOf is the bank of the bytes data at the path given, from the repository's root.
func BankOf(at string, data []byte) (*Bank, error) {
	b, err := sample.ReadBank(strings.NewReader(string(data)))
	if err != nil {
		return nil, err
	}
	return &Bank{Source: SourceOf(at, data), Bank: b}, nil
}

// Digest is the digest of a file as the protocol names one: "sha256:" and the SHA-256 of its bytes.
func Digest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// task is the bank's task of the ID, or nil.
func (b *Bank) task(id string) *sample.Task {
	if i := slices.IndexFunc(b.Bank.Tasks, func(t sample.Task) bool { return t.ID == id }); i >= 0 {
		return &b.Bank.Tasks[i]
	}
	return nil
}

// idPattern is the form of a sample's ID (section 11).
var idPattern = regexp.MustCompile("^[a-z0-9-]{1,64}$")

// ID is the ID of a sample drawn from the seed for the sheet of the number of a plan, by the name of
// the plan's file: synthetic-<seed>-<plan>-<NN>, and -shift-<thousandths> before the number when the
// share of the shift of a near sheet is not synth.SimilarShift. It comes from the seed, as section
// 11 has a synthetic sample's, and the plan, the shift and the sheet tell apart the samples one seed
// draws: two sets drawn with one seed, and two shares of the shift, share no ID.
func ID(plan string, seed uint64, shift float64, sheet int) (string, error) {
	name := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
			return r
		case r >= 'A' && r <= 'Z':
			return r - 'A' + 'a'
		}
		return '-'
	}, strings.TrimSuffix(filepath.Base(plan), filepath.Ext(plan)))
	id := fmt.Sprintf("synthetic-%d-%s", seed, name)
	if shift != synth.SimilarShift {
		id += fmt.Sprintf("-shift-%d", int64(math.Round(shift*1000)))
	}
	id += fmt.Sprintf("-%02d", sheet)
	if !idPattern.MatchString(id) {
		return "", fmt.Errorf("evalsample: the ID %q is not 1 to 64 lowercase Latin letters, digits and hyphens: give the plan a shorter name", id)
	}
	return id, nil
}

// WriterCode is the writer's code of the sheets drawn from the seed: one hand, one code, as the
// samples page gives a person one at random. It is the first 8 hexadecimal digits of the SHA-256 of
// "tutor-synthetic:" and the seed, in capitals, which section 11's 6 to 16 digits and capital Latin
// letters take.
func WriterCode(seed uint64) string {
	sum := sha256.Sum256([]byte("tutor-synthetic:" + strconv.FormatUint(seed, 10)))
	return strings.ToUpper(hex.EncodeToString(sum[:4]))
}

// Of is a sheet that internal/synth drew from the seed for its plan sheet, as a sample of
// measurement of the protocol, from an adult, with its history of writing (section 11): the strokes
// numbered from 1 in the order they were written, their points to a tenth of a unit, each stroke
// written at its last point and each erasure at its time; the sheet's groups and crops, which the
// rules of section 12 give the strokes the history shows; the task of the bank by its open view's
// digest; and the markup that the plan sheet drew: the truth of its one answer slot, the parts in
// the order they read and the first error.
//
// A sheet with an inscription to steer the model, which the markup has no part for, is no sheet of a
// plan of the measurement, and Of refuses it.
func Of(s *sample.Sample, sh *sample.PlanSheet, b *Bank, seed uint64, id string) (*evalv1.Sample, error) {
	if s.Writer != sample.WriterSynthetic {
		return nil, fmt.Errorf("evalsample: the sheet is the %s's, and Of writes the sheets code drew", s.Writer)
	}
	h, ids, err := HistoryOf(s, Pointer)
	if err != nil {
		return nil, err
	}
	ref, sheet, markup, err := sheetParts(s, sh, b, h, ids)
	if err != nil {
		return nil, err
	}
	es := drawnSample(seed, id, ref, h, markup)
	es.Surface = &evalv1.Sample_Sheet{Sheet: sheet}
	return es, nil
}

// PersonOf is a sheet that a person wrote for its plan sheet, as a sample of measurement of the
// protocol, from an adult in their own hand or imitating a child's, with the history of writing h,
// whose strokes ids numbers (section 11): a person's origin, the sample's ID id, which they give at
// random (NewID), and their writer's code, code; and the sheet's groups and crops, its task and its
// markup as Of has them. A sheet the record mode of the board wrote, or one converted from a file of
// tutor-sample v0, names canvas 0.0.0 and the set of texts "none", as the samples page of canvas
// wrote neither (R75).
func PersonOf(s *sample.Sample, sh *sample.PlanSheet, b *Bank, h *evalv1.History, ids map[string]int32, id, code string) (*evalv1.Sample, error) {
	writer := evalv1.Writer_WRITER_ADULT
	switch s.Writer {
	case sample.WriterAuthor:
	case sample.WriterAuthorAsChild:
		writer = evalv1.Writer_WRITER_ADULT_AS_CHILD
	default:
		return nil, fmt.Errorf("evalsample: the sheet is the %s's, and PersonOf writes the sheets a person wrote", s.Writer)
	}
	ref, sheet, markup, err := sheetParts(s, sh, b, h, ids)
	if err != nil {
		return nil, err
	}
	return &evalv1.Sample{
		Protocol:       protocol.Protocol,
		Id:             id,
		Purpose:        evalv1.Purpose_PURPOSE_MEASUREMENT,
		Writer:         writer,
		Origin:         &evalv1.Sample_Person{Person: &evalv1.Person{}},
		WriterCode:     new(code),
		TextSetVersion: TextSetVersion,
		CanvasVersion:  CanvasVersion,
		Task:           ref,
		Ink:            &evalv1.Sample_History{History: h},
		Surface:        &evalv1.Sample_Sheet{Sheet: sheet},
		Markup:         markup,
	}, nil
}

// sheetParts are what a sample of a sheet holds whoever wrote it: the reference of its task, by the
// digest of its open view; its surface, which the rules of section 12 give the strokes of its
// history h; and its markup, whose strokes ids numbers. A sheet with an inscription to steer the
// model, which the markup has no part for, is no sheet of a plan of the measurement.
func sheetParts(s *sample.Sample, sh *sample.PlanSheet, b *Bank, h *evalv1.History, ids map[string]int32) (*evalv1.TaskReference, *evalv1.Sheet, *evalv1.Markup, error) {
	if s.Injection != nil {
		return nil, nil, nil, errors.New("evalsample: a sheet with an inscription to steer the model is no sheet of a plan of the measurement")
	}
	t := b.task(sh.Task)
	if t == nil || s.Task != sh.Task {
		return nil, nil, nil, fmt.Errorf("evalsample: the sheet's task %s is not the plan sheet's %s of %s", s.Task, sh.Task, b.Path)
	}
	_, digest, err := sheetTask(t)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("evalsample: the open view of %s: %w", t.ID, err)
	}
	sheet, err := sheetOf(h, s.Sheet)
	if err != nil {
		return nil, nil, nil, err
	}
	markup, err := markupOf(s, sh, t, ids)
	if err != nil {
		return nil, nil, nil, err
	}
	return b.reference(t.ID, digest), sheet, markup, nil
}

// drawnSample is a sample of measurement that code drew from the seed, by an adult, with its history
// of writing, its task and its markup, and no surface yet: what every drawn sample holds alike
// (section 11).
func drawnSample(seed uint64, id string, task *evalv1.TaskReference, h *evalv1.History, markup *evalv1.Markup) *evalv1.Sample {
	return &evalv1.Sample{
		Protocol:       protocol.Protocol,
		Id:             id,
		Purpose:        evalv1.Purpose_PURPOSE_MEASUREMENT,
		Writer:         evalv1.Writer_WRITER_ADULT,
		Origin:         &evalv1.Sample_Synthetic{Synthetic: &evalv1.Synthetic{Seed: strconv.FormatUint(seed, 10)}},
		WriterCode:     new(WriterCode(seed)),
		TextSetVersion: TextSetVersion,
		CanvasVersion:  CanvasVersion,
		Task:           task,
		Ink:            &evalv1.Sample_History{History: h},
		Markup:         markup,
	}
}

// event is an event of the history before it is written: its time, and the stroke written or the
// erasure, by their places in the sample.
type event struct {
	t       int64
	erasure bool
	at      int
}

// HistoryOf is the sheet's strokes and erasures as a history of writing, each stroke written by the
// pointer given: the strokes numbered from 1 in the order they were written, which ids gives by
// their IDs in the sheet; a stroke written at its last point, and an erasure at its time; at one
// millisecond the strokes come before the erasures, so that an erasure takes off a stroke that ended
// as it touched.
func HistoryOf(s *sample.Sample, pointer string) (*evalv1.History, map[string]int32, error) {
	ids := map[string]int32{}
	for i, st := range s.Strokes {
		ids[st.ID] = int32(i + 1)
	}
	h, err := historyOf(s, ids, pointer)
	return h, ids, err
}

// historyOf is HistoryOf with the strokes numbered by ids.
func historyOf(s *sample.Sample, ids map[string]int32, pointer string) (*evalv1.History, error) {
	var events []event
	for i, st := range s.Strokes {
		if len(st.Points) == 0 {
			return nil, fmt.Errorf("evalsample: the stroke %q has no points", st.ID)
		}
		events = append(events, event{t: st.Points[len(st.Points)-1].TMs, at: i})
	}
	for i, e := range s.Erasures {
		events = append(events, event{t: e.TMs, erasure: true, at: i})
	}
	slices.SortStableFunc(events, func(a, b event) int {
		return cmp.Or(cmp.Compare(a.t, b.t), cmp.Compare(boolInt(a.erasure), boolInt(b.erasure)))
	})
	h := &evalv1.History{}
	for _, e := range events {
		if e.t < 0 || e.t > math.MaxInt32 {
			return nil, fmt.Errorf("evalsample: a time of %d ms is not one a history holds", e.t)
		}
		he := &evalv1.HistoryEvent{TMs: int32(e.t)}
		if e.erasure {
			var gone []int32
			for _, id := range s.Erasures[e.at].Strokes {
				n, ok := ids[id]
				if !ok {
					return nil, fmt.Errorf("evalsample: an erasure takes the stroke %q, which the sheet has not", id)
				}
				gone = append(gone, n)
			}
			he.Event = &evalv1.HistoryEvent_Erase{Erase: &evalv1.StrokeIds{Ids: gone}}
		} else {
			st := s.Strokes[e.at]
			w := &evalv1.Stroke{Id: ids[st.ID], Pointer: pointer}
			for _, p := range st.Points {
				w.Points = append(w.Points, &evalv1.InkPoint{X: tenth(p.X), Y: tenth(p.Y), TMs: int32(p.TMs)})
			}
			he.Event = &evalv1.HistoryEvent_Write{Write: w}
		}
		h.Events = append(h.Events, he)
	}
	return h, nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// tenth is a place to a tenth of a unit, rounded half up as the rules of section 12 count it.
func tenth(v float64) float64 { return float64(lines.Tenths(v)) / 10 }

// sheetOf is the surface of a history on a sheet of the size: its height of a digit, render.Digit,
// the one the reading reads at; the groups of the strokes it shows, by the rule of groups; and their
// crops at each height of section 11, by the rule of the crop. The strokes are those of the history
// as a reader folds it (protocol.Ink), to the tenth they are written at, so that sample-check, which
// computes them again, finds the same.
func sheetOf(h *evalv1.History, size sample.Sheet) (*evalv1.Sheet, error) {
	if size.Width != math.Trunc(size.Width) || size.Height != math.Trunc(size.Height) || size.Width < 1 || size.Height < 1 || size.Width > math.MaxInt32 || size.Height > math.MaxInt32 {
		return nil, fmt.Errorf("evalsample: a sheet of %g × %g units, and the protocol's is whole units", size.Width, size.Height)
	}
	strokes, erasures, f := protocol.Ink(h)
	sheet := &evalv1.Sheet{Width: int32(size.Width), Height: int32(size.Height), DigitHeight: render.Digit}
	shown := Shown(strokes, f)
	groups, err := Groups(strokes, erasures)
	if err != nil {
		return nil, err
	}
	sheet.Groups = groups
	if len(shown) == 0 {
		return sheet, nil
	}
	crops, err := CropSet(shown, GroupsOf(sheet.GetGroups()), Heights(render.Digit))
	if err != nil {
		return nil, err
	}
	sheet.Crops = crops
	return sheet, nil
}

// Groups are the groups of a sheet's strokes as the protocol writes them, by the rule of groups
// (lines.Groups): in the order of their labels, A to Z and then AA, each with the IDs of the strokes
// the sheet shows in it, ascending.
func Groups(strokes []sample.Stroke, erasures []sample.Erasure) ([]*evalv1.Group, error) {
	groups := lines.Groups(strokes, erasures)
	slices.SortFunc(groups, func(a, b lines.StrokeGroup) int {
		return cmp.Or(cmp.Compare(len(a.Label), len(b.Label)), cmp.Compare(a.Label, b.Label))
	})
	var out []*evalv1.Group
	for _, g := range groups {
		ids, err := numbers(g.Strokes)
		if err != nil {
			return nil, err
		}
		slices.Sort(ids)
		out = append(out, &evalv1.Group{Label: g.Label, StrokeIds: ids})
	}
	return out, nil
}

// Shown are the strokes of a history that the sheet shows at the end, in the order they were
// written.
func Shown(strokes []sample.Stroke, f protocol.Folded) []sample.Stroke {
	var shown []sample.Stroke
	for k, st := range strokes {
		if _, ok := slices.BinarySearch(f.Visible, f.Written[k].GetId()); ok {
			shown = append(shown, st)
		}
	}
	return shown
}

// Heights are the heights of a digit a set of crops is drawn at: those of eval, render.Heights, and
// the module's, ascending and each once (section 11).
func Heights(module int) []int {
	hs := render.Heights()
	if !slices.Contains(hs, module) {
		hs = append(hs, module)
		slices.Sort(hs)
	}
	return hs
}

// CropSet is the set of crops of strokes, with the frames of their groups (none on a workbook), at
// each of the heights: the box of their points and h_med in whole tenths, and each crop's numbers
// and PNG, as the rule of the crop gives them (section 12).
func CropSet(strokes []sample.Stroke, groups []lines.StrokeGroup, heights []int) (*evalv1.CropSet, error) {
	set := &evalv1.CropSet{}
	for _, h := range heights {
		c, err := render.CropOf(strokes, groups, h)
		if err != nil {
			return nil, err
		}
		png, err := c.PNG(strokes)
		if err != nil {
			return nil, err
		}
		if set.InkBox == nil {
			b := c.InkBox()
			set.InkBox = &evalv1.TenthsBox{Ymin: int32(b[0]), Xmin: int32(b[1]), Ymax: int32(b[2]), Xmax: int32(b[3])}
			set.BundleMedian = int32(c.MedianTenths())
		}
		crop := &evalv1.Crop{
			DigitHeight: int32(c.Digit), DrawnHeight: int32(c.Drawn), Width: int32(c.Width), Height: int32(c.Height),
			MarginTop: int32(c.Margins[0]), MarginLeft: int32(c.Margins[1]), Scale: c.Scale, LineWidth: c.Line,
			SheetBox: &evalv1.SheetBox{Ymin: c.SheetBox[0], Xmin: c.SheetBox[1], Ymax: c.SheetBox[2], Xmax: c.SheetBox[3]},
			Png:      png,
		}
		for _, f := range c.Frames {
			crop.Frames = append(crop.Frames, &evalv1.Frame{Label: f.Label, Box: &observationv1.CropBox{
				Ymin: int32(f.Box[0]), Xmin: int32(f.Box[1]), Ymax: int32(f.Box[2]), Xmax: int32(f.Box[3]),
			}})
		}
		set.Crops = append(set.Crops, crop)
	}
	return set, nil
}

// GroupsOf are a sheet's groups as the crop takes them: each label with its strokes, as tutor's code
// names them (protocol.StrokeID).
func GroupsOf(groups []*evalv1.Group) []lines.StrokeGroup {
	out := make([]lines.StrokeGroup, len(groups))
	for i, g := range groups {
		out[i] = lines.StrokeGroup{Label: g.GetLabel()}
		for _, id := range g.GetStrokeIds() {
			out[i].Strokes = append(out[i].Strokes, protocol.StrokeID(id))
		}
	}
	return out
}

// numbers are strokes of tutor's code by their IDs of the protocol.
func numbers(ids []string) ([]int32, error) {
	out := make([]int32, len(ids))
	for i, id := range ids {
		n, err := protocol.StrokeNumber(id)
		if err != nil {
			return nil, err
		}
		out[i] = n
	}
	return out, nil
}

// slot is the place of a sheet's one answer slot, bound anywhere: item 1, slot 1.
func slot() *observationv1.AnswerSlotPlace { return &observationv1.AnswerSlotPlace{Item: 1, Slot: 1} }

// markupOf is the markup a sheet was drawn with, as the protocol writes it: its lines as parts, each
// a text, or a drawing where its part of the plan is one, with its strokes ascending; the first
// error; and the truth of its answer slot, which a reader that reads right gives (section 11):
//   - a sheet that shows no stroke is blank;
//   - with no part holding the final answer, no part crossed out among them, it has no final answer,
//     and with two or more, several;
//   - an illegible answer is illegible, and an answer that gives no number is no answer the
//     measurement draws;
//   - else the value of the answer, as the reading reads one (sample.ValueOf).
//
// The answer of a plan sheet of one part, a whole solution a person writes freely, is that part: on
// a sheet a person wrote, its value is the key's on a sheet they mark correct, and on one they mark
// wrong the number after the last word «ответ» of its text, as their markup names it (wholeValue).
// Code draws no such sheet: its truth is for the author's hand to write.
//
// A sheet with a part crossed out has the note crossed_out on its slot: something there is crossed
// out and written again. Every stroke a sheet shows is in a part (section 11), and a sheet with one
// in none has no markup the protocol takes.
func markupOf(s *sample.Sample, sh *sample.PlanSheet, t *sample.Task, ids map[string]int32) (*evalv1.Markup, error) {
	if id := Loose(s); id != "" {
		return nil, fmt.Errorf("evalsample: the stroke %q, which the sheet shows, is in no part, and every stroke a sheet shows is in one", id)
	}
	parts := &evalv1.WritingParts{}
	if len(sh.Parts) > 0 && len(sh.Parts) != len(s.Lines) {
		return nil, fmt.Errorf("evalsample: the sheet has %d lines, and its plan sheet %d parts: the kind of each part is its plan's", len(s.Lines), len(sh.Parts))
	}
	byPlan := len(sh.Parts) > 0
	var answers []sample.Line
	crossed := false
	for i, l := range s.Lines {
		p := &evalv1.WritingPart{Kind: evalv1.PartKind_PART_KIND_TEXT, Text: l.Text, CrossedOut: l.CrossedOut, Illegible: l.Illegible, FinalAnswer: l.Answer && !l.CrossedOut}
		if byPlan && sh.Parts[i].Kind == sample.PartDrawing {
			p.Kind = evalv1.PartKind_PART_KIND_DRAWING
		}
		for _, id := range l.Strokes {
			n, ok := ids[id]
			if !ok {
				return nil, fmt.Errorf("evalsample: the line %d has the stroke %q, which the sheet has not", i+1, id)
			}
			p.StrokeIds = append(p.StrokeIds, n)
		}
		slices.Sort(p.StrokeIds)
		if p.FinalAnswer {
			answers = append(answers, l)
		}
		crossed = crossed || l.CrossedOut
		parts.Parts = append(parts.Parts, p)
	}
	if fe := s.Expected.FirstError; fe != nil {
		parts.FirstError = new(int32(*fe))
	}

	read := &observationv1.AnswerSlotObservation{AnswerSlot: slot()}
	switch {
	case len(render.Shown(s, math.MaxInt64)) == 0:
		read.Read = unreadable(observationv1.Unreadable_UNREADABLE_BLANK)
	case len(answers) == 0:
		read.Read = unreadable(observationv1.Unreadable_UNREADABLE_NO_FINAL_ANSWER)
	case len(answers) > 1:
		read.Read = unreadable(observationv1.Unreadable_UNREADABLE_SEVERAL_FINAL_ANSWERS)
	case answers[0].Illegible:
		read.Read = unreadable(observationv1.Unreadable_UNREADABLE_ILLEGIBLE)
	case len(sh.Parts) == 1 && s.Writer != sample.WriterSynthetic:
		v, err := wholeValue(s, t, answers[0].Text)
		if err != nil {
			return nil, err
		}
		read.Read = &observationv1.AnswerSlotObservation_Value{Value: v}
	default:
		v := sample.ValueOf(answers[0].Text)
		if v == "" {
			return nil, errors.New("evalsample: the answer gives no number, and the truth of a sample is the value of its answer: a plan sheet that marks a whole solution as its answer, as the author's plan does, is for the author's hand to write")
		}
		read.Read = &observationv1.AnswerSlotObservation_Value{Value: v}
	}
	m := &evalv1.Markup{
		Truth:   &observationv1.Observation{AnswerSlots: []*observationv1.AnswerSlotObservation{read}},
		Notes:   []*evalv1.AnswerSlotNote{},
		Writing: &evalv1.Markup_Parts{Parts: parts},
	}
	if crossed {
		m.Notes = append(m.Notes, &evalv1.AnswerSlotNote{AnswerSlot: slot(), CrossedOut: true})
	}
	return m, nil
}

// Loose is the first stroke a sheet shows, in the order they were written, that is in no line and no
// inscription, or "" when there is none.
func Loose(s *sample.Sample) string {
	in := map[string]bool{}
	for _, l := range s.Lines {
		for _, id := range l.Strokes {
			in[id] = true
		}
	}
	if s.Injection != nil {
		for _, id := range s.Injection.Strokes {
			in[id] = true
		}
	}
	for _, st := range render.Shown(s, math.MaxInt64) {
		if !in[st.ID] {
			return st.ID
		}
	}
	return ""
}

// answerWord is the word after which the author's markup of a whole solution names its answer: «ответ»
// as a word of its own, in any case, and not inside «ответа» or «ответы».
var answerWord = regexp.MustCompile(`(?i)(?:^|[^\p{L}])(ответ)(?:[^\p{L}]|$)`)

// wholeValue is the value of the answer of a whole solution the author wrote, its markup's text: the
// key's on a sheet marked correct, and on one marked wrong the first number after the last word
// «ответ» of the text. A sheet marked correct whose text names another number there, and one marked
// wrong whose text names none or the key's, has no truth the author's markup gives (R75).
func wholeValue(s *sample.Sample, t *sample.Task, text string) (string, error) {
	key := sample.KeyOf(t.Key)
	named := ""
	if at := answerWord.FindAllStringSubmatchIndex(text, -1); len(at) > 0 {
		if nums := sample.Numbers(text[at[len(at)-1][3]:]); len(nums) > 0 {
			named = nums[0]
		}
	}
	switch {
	case s.Expected.Outcome != sample.OutcomeCorrect && s.Expected.Outcome != sample.OutcomeError:
		return "", fmt.Errorf("evalsample: the sheet is marked %s, and the truth of a whole solution is the key's or the answer its markup names, on a sheet marked correct or wrong: mark its part illegible, or the sheet correct or wrong", s.Expected.Outcome)
	case s.Expected.Outcome == sample.OutcomeCorrect && named != "" && named != key:
		return "", fmt.Errorf("evalsample: the sheet is marked correct, and its markup names the answer %s, not the key's %s", named, key)
	case s.Expected.Outcome == sample.OutcomeCorrect:
		return key, nil
	case named == "":
		return "", errors.New("evalsample: the sheet is marked wrong, and its markup names no number after the word «ответ»: say there what the answer written is")
	case named == key:
		return "", fmt.Errorf("evalsample: the sheet is marked wrong, and the answer its markup names is the key's, %s", key)
	}
	return named, nil
}

func unreadable(r observationv1.Unreadable) *observationv1.AnswerSlotObservation_Unreadable {
	return &observationv1.AnswerSlotObservation_Unreadable{Unreadable: r}
}

// sheetTask is the task of the format of a bank's task on a sheet (task.Sheet), with the digest of
// its open view.
func sheetTask(t *sample.Task) (formatv0.Task, string, error) {
	ft := task.Sheet(t)
	d, err := mkview.TaskDigest(ft)
	return ft, string(d), err
}

// ctx is the context of a verdict on a truth: a check of a sheet's number runs no predicate that
// could take long.
var ctx = context.Background()
