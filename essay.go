package tutor

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/altnovatrail/spec/types/formatv0"
	"github.com/altnovatrail/spec/types/formatv0/open"

	observationv1 "github.com/altnovatrail/spec/contracts/novatrail/observation/v1"

	"github.com/altnovatrail/tutor/internal/essay"
	"github.com/altnovatrail/tutor/provider"
)

// EssayReading is an essay to read (architecture 3.10, "reading an essay"), the first of the two
// calls of a task with a rubric (architecture 3.5; R75 spec): the PNG of the crop of the sheet, the
// open view of its task and the route. The model sees the crop alone, without the topic and the
// rubric.
type EssayReading struct {
	Route Route
	Task  *open.Task
	Crop  []byte
}

// EssayRead is what a reading of an essay gives: the observation of its answer slot bound to the
// sheet — the text as it is written, or unreadable when the essay as a whole cannot be read, which
// does not reach the assessment (R75 spec; R86 spec) — when Outcome is Answered, and otherwise none.
type EssayRead struct {
	Observation *observationv1.Observation
	Outcome     Outcome
}

// ReadEssay reads an essay by the one model admitted for reading handwriting with the prompt of the
// reading of an essay (R68 spec, second amendment of 2026-11-01; R53). The pair of the reading of a
// sheet measured another prompt and admits nothing of this one (R70 spec): until an entry of the list
// measured it, the reading is NotAdmitted. A word that cannot be made out has no field of its own
// until the measurement on the essays of volunteers (Т-37; П-16).
func (t *Tutor) ReadEssay(ctx context.Context, r EssayReading) EssayRead {
	fail := func(err error) EssayRead { return EssayRead{Outcome: Outcome{Kind: Failed, Err: err}} }
	place, err := essayPlace(r.Task)
	if err != nil {
		return fail(err)
	}
	cfg, err := cropSize(r.Crop)
	if err != nil {
		return fail(err)
	}
	k := kinds[CallEssay]
	calls, a, refused := t.prepare(CallEssay, r.Route)
	if refused != nil {
		return EssayRead{Outcome: *refused}
	}
	req := essay.ReadRequestOf(r.Crop, cfg.Width, cfg.Height)
	req.MaxOutputTokens = k.maxOut
	if err := checkText(req); err != nil {
		return EssayRead{Outcome: Outcome{Kind: Failed, Admission: a, Err: err}}
	}
	resps, out := ask(ctx, a, calls, []provider.Request{req}, k.maxIn)
	if out.Kind != Answered {
		return EssayRead{Outcome: out}
	}
	slot := &observationv1.AnswerSlotObservation{AnswerSlot: place}
	text, err := essay.ReadReading(resps[0].JSON)
	switch {
	case errors.Is(err, essay.ErrIllegible):
		slot.Read = &observationv1.AnswerSlotObservation_Unreadable{Unreadable: observationv1.Unreadable_UNREADABLE_ILLEGIBLE}
	case err != nil:
		out.Kind, out.Err = Failed, fmt.Errorf("tutor: %s: %w", calls[0].model.Name(), err)
		return EssayRead{Outcome: out}
	case strings.TrimSpace(text) == "":
		slot.Read = &observationv1.AnswerSlotObservation_Unreadable{Unreadable: observationv1.Unreadable_UNREADABLE_BLANK}
	default:
		slot.Read = &observationv1.AnswerSlotObservation_Value{Value: text}
	}
	return EssayRead{Observation: &observationv1.Observation{AnswerSlots: []*observationv1.AnswerSlotObservation{slot}}, Outcome: out}
}

// essayPlace is the place of the one answer slot of a task bound to the whole sheet, an essay: it
// refuses a task that is no handwriting sheet, has none or more than one such slot, or has a slot
// bound otherwise, which the reading of an essay does not read.
func essayPlace(t *open.Task) (*observationv1.AnswerSlotPlace, error) {
	if t == nil || t.Canvas.Kind != formatv0.CanvasKindHandwritingSheet {
		return nil, errors.New("tutor: the task of an essay is no handwriting sheet")
	}
	var place *observationv1.AnswerSlotPlace
	for i, it := range t.Items {
		for s, slot := range it.AnswerSlots {
			switch {
			case slot.Binding != formatv0.AnswerSlotBindingSheet:
				return nil, fmt.Errorf("tutor: item %d, slot %d of the essay is bound %q, which the reading of an essay does not read", i+1, s+1, slot.Binding)
			case place != nil:
				return nil, errors.New("tutor: the task has more than one answer slot bound to the sheet")
			}
			place = &observationv1.AnswerSlotPlace{Item: int32(i + 1), Slot: int32(s + 1)}
		}
	}
	if place == nil {
		return nil, errors.New("tutor: the task has no answer slot bound to the sheet")
	}
	return place, nil
}
