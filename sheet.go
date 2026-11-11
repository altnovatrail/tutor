package tutor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"strings"

	"github.com/altnovatrail/spec/types/admissionv0"
	"github.com/altnovatrail/spec/types/formatv0"
	"github.com/altnovatrail/spec/types/formatv0/open"

	observationv1 "github.com/altnovatrail/spec/contracts/novatrail/observation/v1"

	"github.com/altnovatrail/tutor/internal/reading"
	"github.com/altnovatrail/tutor/provider"
)

// maxCropSide is the longest side of a crop the reading takes, in pixels: a crop cut by the rule of
// the eval protocol is far smaller (render.CropOf), and a larger one is no crop of a sheet but a
// picture that would cost both readers a large input for nothing.
const maxCropSide = 4096

// SheetReading is a handwriting sheet to read (architecture 3.10, «чтение листа»): the PNG of the
// crop of its visible strokes, as app sends it (the contract between app and api, SheetWork), the
// open view of its task, and the route. The model sees the crop and the statement, never the key
// (R67 spec).
type SheetReading struct {
	Route Route
	Task  *open.Task
	Crop  []byte
}

// SheetRead is what a reading of a sheet gives: the observation of each reader, in the order of the
// pair, the strong reader first (R68 spec), when Outcome is Answered — module-kit joins them and
// gives the verdict (R86 spec) —, and otherwise none: tutor gives both or the outcome, never one.
type SheetRead struct {
	Observations []*observationv1.Observation
	Outcome      Outcome
}

// ReadSheet reads a handwriting sheet by the models admitted for reading handwriting, in parallel,
// each into the observation of the task's one answer slot bound anywhere (R86 spec; § 10 of spec's
// contracts text). The first version reads that slot alone: the sheets of the first modules have no
// other that AI reads (T115). A slot bound choice is no reading of AI: api gives the option tapped
// to module-kit. An answer that is no observation fails the reading, as a failure of AI does: there
// is no observation of a pair then (R86 spec).
func (t *Tutor) ReadSheet(ctx context.Context, r SheetReading) SheetRead {
	place, statement, err := sheetTask(r.Task)
	if err != nil {
		return SheetRead{Outcome: Outcome{Kind: Failed, Err: err}}
	}
	cfg, err := cropSize(r.Crop)
	if err != nil {
		return SheetRead{Outcome: Outcome{Kind: Failed, Err: err}}
	}
	calls, a, refused := t.prepare(admissionv0.CapabilityHandwritingReading, r.Route)
	if refused != nil {
		return SheetRead{Outcome: *refused}
	}
	req := reading.RequestOf(statement, r.Crop, cfg.Width, cfg.Height, nil)
	req.MaxOutputTokens = reading.MaxOutputTokens
	if err := checkText(req); err != nil {
		return SheetRead{Outcome: Outcome{Kind: Failed, Admission: a, Err: err}}
	}
	reqs := make([]provider.Request, len(calls))
	for i := range reqs {
		reqs[i] = req
	}
	resps, out := ask(ctx, a, calls, reqs, kinds[admissionv0.CapabilityHandwritingReading].maxIn)
	if out.Kind != Answered {
		return SheetRead{Outcome: out}
	}
	obs := make([]*observationv1.Observation, len(resps))
	for i, resp := range resps {
		o, err := reading.Read(resp.JSON)
		if err != nil {
			out.Kind, out.Err = Failed, fmt.Errorf("tutor: %s: %w", calls[i].model.Name(), err)
			return SheetRead{Outcome: out}
		}
		obs[i], _ = reading.ObservationOf(o)
		// Each observation has a place of its own, so that changing one changes no other.
		obs[i].AnswerSlots[0].AnswerSlot = &observationv1.AnswerSlotPlace{Item: place.Item, Slot: place.Slot}
	}
	return SheetRead{Observations: obs, Outcome: out}
}

// sheetTask is the place of a task's one answer slot that AI reads, bound anywhere, and the task as
// the learner reads it, each piece on its line, in the task's own words and none of tutor's: the
// paragraphs of the statement, the alt of each of its pictures, since the reader sees no picture of
// the statement, and the text of the item of that slot, what it asks. It refuses a task that is no
// handwriting sheet, one with a slot bound to a zone or the whole sheet, which this version does not
// read, one with no slot bound anywhere or more than one, and one with no words to read.
func sheetTask(t *open.Task) (*observationv1.AnswerSlotPlace, string, error) {
	if t == nil {
		return nil, "", errors.New("tutor: the reading has no task")
	}
	if t.Canvas.Kind != formatv0.CanvasKindHandwritingSheet {
		return nil, "", fmt.Errorf("tutor: the task %s is on a %s, not a handwriting sheet", t.ID, t.Canvas.Kind)
	}
	var place *observationv1.AnswerSlotPlace
	var asks *formatv0.Text
	for i, item := range t.Items {
		for j, slot := range item.AnswerSlots {
			switch slot.Binding {
			case formatv0.AnswerSlotBindingAnywhere:
				if place != nil {
					return nil, "", fmt.Errorf("tutor: the task %s has more than one answer slot bound anywhere", t.ID)
				}
				place = &observationv1.AnswerSlotPlace{Item: int32(i + 1), Slot: int32(j + 1)}
				asks = item.Text
			case formatv0.AnswerSlotBindingChoice:
			default:
				return nil, "", fmt.Errorf("tutor: the task %s has an answer slot bound %s, which this reading does not read", t.ID, slot.Binding)
			}
		}
	}
	if place == nil {
		return nil, "", fmt.Errorf("tutor: the task %s has no answer slot bound anywhere: AI reads nothing of it", t.ID)
	}
	statement, err := taskText(t, asks)
	if err != nil {
		return nil, "", err
	}
	return place, statement, nil
}

// taskText is a task as the learner reads it, each piece on its line, in the task's own words and
// none of tutor's: the paragraphs of the statement, the alt of each of its pictures, since a model
// sees no picture of the statement, and the text of an item, what it asks, when there is one. A task
// with no words is refused.
func taskText(t *open.Task, asks *formatv0.Text) (string, error) {
	var lines []string
	add := func(s string) {
		if s = strings.TrimSpace(s); s != "" {
			lines = append(lines, s)
		}
	}
	for _, p := range t.Statement {
		if p.Text != nil {
			add(string(*p.Text))
		}
		if p.Alt != nil {
			add(*p.Alt)
		}
	}
	if asks != nil {
		add(string(*asks))
	}
	if len(lines) == 0 {
		return "", fmt.Errorf("tutor: the task %s has no words to read: no statement, alt or question", t.ID)
	}
	return strings.Join(lines, "\n"), nil
}

// cropSize is the size of a crop, which must be a whole PNG of no more than maxCropSide on a side:
// the whole picture decodes, not its header alone, so that a broken crop goes to no model.
func cropSize(crop []byte) (image.Config, error) {
	cfg, err := png.DecodeConfig(bytes.NewReader(crop))
	if err == nil && (cfg.Width > maxCropSide || cfg.Height > maxCropSide) {
		err = fmt.Errorf("%d × %d pixels, more than %d on a side", cfg.Width, cfg.Height, maxCropSide)
	}
	if err == nil {
		_, err = png.Decode(bytes.NewReader(crop))
	}
	if err != nil {
		return cfg, fmt.Errorf("tutor: the crop is no PNG of a sheet: %w", err)
	}
	return cfg, nil
}
