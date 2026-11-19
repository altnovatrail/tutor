package tutor

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	observationv1 "github.com/altnovatrail/spec/contracts/novatrail/observation/v1"

	"github.com/altnovatrail/tutor/admission"
	"github.com/altnovatrail/tutor/internal/finding"
	"github.com/altnovatrail/tutor/provider"
)

// maxPhotoTokens are the most tokens the photo of a page takes: the largest a provider adapter
// fits a photo to, Anthropic's high-resolution tier (sources 1.19). GPT-6 Astra takes at most 2 500
// patches, 1.2 tokens each (sources 1.8).
const maxPhotoTokens = 4784

// TaskFinding is a call that finds the tasks on the photo of a page from the learner's paper
// (R162 spec): the photo is a JPEG or a PNG of at most 4 MiB, as app sent it.
type TaskFinding struct {
	Route Route
	Photo []byte
}

// TasksFound is what the finding gives: the task frames, in whole thousandths of the photo, in the
// order of the page, top to bottom, when Outcome is Answered — none when the model found no task.
type TasksFound struct {
	Frames  []*observationv1.CropBox
	Outcome Outcome
}

// FindTasks asks the admitted models of the finding (CallFinding) for the tasks on the photo of a
// page, one at a time, a later one only when the one before failed or found no task (R137). The
// adapter fits the photo to the size its model takes unshrunk, and it goes as a JPEG without the
// metadata of its file (provider.Photo). A photo that is no JPEG or PNG fails before any call; an
// answer off the schema fails the call. A box with no area on the photo is no frame. The frames go
// top to bottom, and of two at one height the left first.
func (t *Tutor) FindTasks(ctx context.Context, f TaskFinding) TasksFound {
	calls, a, refused := t.prepare(CallFinding, f.Route)
	if refused != nil {
		return TasksFound{Outcome: *refused}
	}
	frames, out := cascade(ctx, calls, func(c call) ([]*observationv1.CropBox, Outcome) {
		return t.findBy(ctx, a, c, f.Photo)
	}, func(frames []*observationv1.CropBox) bool { return len(frames) > 0 })
	return TasksFound{Frames: frames, Outcome: out}
}

// findBy asks one model for the frames of the tasks on the photo.
func (t *Tutor) findBy(ctx context.Context, a admission.Answer, c call, photo []byte) ([]*observationv1.CropBox, Outcome) {
	fail := func(err error) ([]*observationv1.CropBox, Outcome) {
		return nil, Outcome{Kind: Failed, Admission: a, Err: err}
	}
	fitted, width, height, err := provider.Photo(photo, c.client)
	if err != nil {
		return fail(fmt.Errorf("tutor: %w", err))
	}
	req := finding.RequestOf(fitted, width, height)
	req.MaxOutputTokens = finding.MaxOutputTokens
	if err := checkText(req); err != nil {
		return fail(err)
	}
	resps, out := ask(ctx, a, []call{c}, []provider.Request{req}, kinds[CallFinding].maxIn)
	if out.Kind != Answered {
		return nil, out
	}
	boxes, err := finding.Read(resps[0].JSON)
	if err != nil {
		out.Kind, out.Err = Failed, fmt.Errorf("tutor: %s: %w", c.model.Name(), err)
		return nil, out
	}
	frames := make([]*observationv1.CropBox, 0, len(boxes))
	for _, b := range boxes {
		if place, ok := provider.Place(b.X1, b.Y1, b.X2, b.Y2, width, height); ok {
			frames = append(frames, &observationv1.CropBox{Ymin: int32(place[0]), Xmin: int32(place[1]), Ymax: int32(place[2]), Xmax: int32(place[3])})
		}
	}
	slices.SortStableFunc(frames, func(x, y *observationv1.CropBox) int {
		return cmp.Or(cmp.Compare(x.Ymin, y.Ymin), cmp.Compare(x.Xmin, y.Xmin))
	})
	return frames, out
}
