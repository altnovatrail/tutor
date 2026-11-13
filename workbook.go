package tutor

import (
	"context"
	"errors"
	"fmt"

	"github.com/altnovatrail/spec/types/formatv0"
	"github.com/altnovatrail/spec/types/formatv0/open"

	observationv1 "github.com/altnovatrail/spec/contracts/novatrail/observation/v1"
	"google.golang.org/protobuf/proto"

	"github.com/altnovatrail/tutor/internal/reading"
	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/workbook"
	"github.com/altnovatrail/tutor/provider"
)

// The bounds of a reading of a workbook page, which hold its worst price (kinds): the most filled
// boxes a request shows, as many as the adapter of Claude sends pictures in one request
// (claude.MaxImages), and the longest side of the crop of a box, in pixels. A crop of a box holds a
// number or two, far smaller (internal/workbook's cells).
const (
	maxBoxes   = 20
	maxBoxSide = 512
)

// BoxCrop is the crop of a filled box of a workbook page, as app sends it: the places of its item
// and of the box among the item's boxes, from 1, and the PNG of its strokes alone, without the
// box's frame and the print of its line (R53 spec, amendment of 2026-10-27).
type BoxCrop struct {
	Item, Slot int
	Crop       []byte
}

// WorkbookReading is a workbook page to read (architecture 3.10, "reading the workbook"): the open view of
// its task, the crops of its filled boxes, and the route. A box the learner left blank has no crop
// and goes to no model (R86 spec). The model sees the crops and their places, never the line of an
// item nor the key.
type WorkbookReading struct {
	Route Route
	Task  *open.Task
	Boxes []BoxCrop
}

// WorkbookRead is what a reading of a workbook page gives: the observation of each reader, in the
// order of the pair, over every box of the page — a blank one unreadable, blank —, when Outcome is
// Answered — module-kit joins them and gives the verdict (R86 spec) —, and otherwise none.
type WorkbookRead struct {
	Observations []*observationv1.Observation
	Outcome      Outcome
}

// ReadWorkbook reads the filled boxes of a workbook page by the models admitted for reading
// handwriting with the prompt of the workbook, in parallel, each into the observation of the page
// (R86 spec; § 10 of spec's contracts text). The pair admitted for the reading of a sheet measured
// another prompt, and admits nothing of this one (R70 spec): until an entry of the list measured the
// prompt of the workbook, the reading is NotAdmitted. A page with no filled box goes to no model.
// An answer that is no observation fails the reading, as a failure of AI does.
func (t *Tutor) ReadWorkbook(ctx context.Context, r WorkbookReading) WorkbookRead {
	fail := func(err error) WorkbookRead { return WorkbookRead{Outcome: Outcome{Kind: Failed, Err: err}} }
	places, err := placesOf(r.Task, r.Boxes)
	if err != nil {
		return fail(err)
	}
	if len(r.Boxes) > maxBoxes {
		return fail(fmt.Errorf("tutor: %d filled boxes, more than the %d a reading shows", len(r.Boxes), maxBoxes))
	}
	crops := make([]workbook.Crop, len(r.Boxes))
	maxIn := int64(maxTextBytes)
	for i, b := range r.Boxes {
		cfg, err := cropSize(b.Crop)
		if err == nil && (cfg.Width > maxBoxSide || cfg.Height > maxBoxSide) {
			err = fmt.Errorf("%d × %d pixels, more than %d on a side of a box", cfg.Width, cfg.Height, maxBoxSide)
		}
		if err != nil {
			return fail(fmt.Errorf("tutor: the crop of item %d, box %d: %w", b.Item, b.Slot, err))
		}
		crops[i] = workbook.Crop{Item: b.Item, Slot: b.Slot, PNG: b.Crop}
		maxIn += int64(render.Tokens(cfg.Width, cfg.Height))
	}
	if len(crops) == 0 {
		// A page with no filled box goes to no model, and makes no client: each reader's observation
		// is its own, so that changing one changes no other.
		e, a, err := t.chooseCall(CallWorkbook, r.Route)
		switch {
		case err != nil:
			return fail(err)
		case !a.Admitted:
			return WorkbookRead{Outcome: Outcome{Kind: NotAdmitted, Admission: a}}
		}
		obs, _ := workbook.ObservationOf(places, []workbook.Reading{})
		out := make([]*observationv1.Observation, len(kinds[CallWorkbook].asked(e.Models)))
		for i := range out {
			out[i] = proto.Clone(obs).(*observationv1.Observation)
		}
		return WorkbookRead{Observations: out, Outcome: Outcome{Kind: Answered, Admission: a}}
	}
	calls, a, refused := t.prepare(CallWorkbook, r.Route)
	if refused != nil {
		return WorkbookRead{Outcome: *refused}
	}
	req := workbook.RequestOf(crops, workbook.Bare)
	req.MaxOutputTokens = reading.MaxOutputTokens
	if err := checkText(req); err != nil {
		return WorkbookRead{Outcome: Outcome{Kind: Failed, Admission: a, Err: err}}
	}
	reqs := make([]provider.Request, len(calls))
	for i := range reqs {
		reqs[i] = req
	}
	resps, out := ask(ctx, a, calls, reqs, maxIn)
	if out.Kind != Answered {
		return WorkbookRead{Outcome: out}
	}
	obs := make([]*observationv1.Observation, len(resps))
	for i, resp := range resps {
		read, err := workbook.Read(resp.JSON)
		if err == nil {
			obs[i], _ = workbook.ObservationOf(places, read)
			if obs[i] == nil {
				err = errors.New("the answer has no cell, or two, for a filled box")
			}
		}
		if err != nil {
			out.Kind, out.Err = Failed, fmt.Errorf("tutor: %s: %w", calls[i].model.Name(), err)
			return WorkbookRead{Outcome: out}
		}
	}
	return WorkbookRead{Observations: obs, Outcome: out}
}

// placesOf are the boxes of a workbook page in the task's order, each filled when a crop came for
// it. It refuses a task that is no workbook page, and a crop of a place the page has no box of, or
// of one box twice.
func placesOf(t *open.Task, boxes []BoxCrop) ([]workbook.Place, error) {
	if t == nil || t.Canvas.Kind != formatv0.CanvasKindWorkbook {
		return nil, errors.New("tutor: the task is no workbook page")
	}
	type place struct{ item, slot int }
	exists := map[place]bool{}
	for i, it := range t.Items {
		slot := 0
		for _, part := range it.Line {
			if part.Binding != nil && *part.Binding == formatv0.LinePartBindingBox {
				slot++
				exists[place{i + 1, slot}] = true
			}
		}
	}
	filled := map[place]bool{}
	for _, b := range boxes {
		p := place{b.Item, b.Slot}
		switch {
		case !exists[p]:
			return nil, fmt.Errorf("tutor: the page has no box %d of item %d", b.Slot, b.Item)
		case filled[p]:
			return nil, fmt.Errorf("tutor: two crops of item %d, box %d", b.Item, b.Slot)
		}
		filled[p] = true
	}
	var places []workbook.Place
	for i, it := range t.Items {
		slot := 0
		for _, part := range it.Line {
			if part.Binding != nil && *part.Binding == formatv0.LinePartBindingBox {
				slot++
				places = append(places, workbook.Place{Item: i + 1, Slot: slot, Filled: filled[place{i + 1, slot}]})
			}
		}
	}
	if len(places) == 0 {
		return nil, errors.New("tutor: the workbook page has no box")
	}
	return places, nil
}
