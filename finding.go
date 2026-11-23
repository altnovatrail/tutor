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
// fits a photo to, the 4 784 patches of Anthropic's high-resolution tier. GPT-6 Astra takes at most
// 2 500 patches, 1.2 tokens each (R134).
const maxPhotoTokens = 4784

// TaskFinding is a call that finds the tasks on the photo of a page from the learner's paper
// (R162 spec): the photo is a JPEG or a PNG of at most 4 MiB, as app sent it. With the learner's
// request about the page come the frames on it now, in whole thousandths of the photo: those the AI
// found before, which the answer replaces, and those the learner drew, which stay (R165 spec). api
// has checked their bounds; without a request the frames are not sent.
type TaskFinding struct {
	Route   Route
	Photo   []byte
	Request string
	Found   []*observationv1.CropBox
	Drawn   []*observationv1.CropBox
}

// sameAsDrawn is the overlap, of their union, from which a frame found covers one the learner drew:
// the model repeated it, and the learner's own stays (R165 spec).
const sameAsDrawn = 0.8

// TasksFound is what the finding gives: the task frames, in whole thousandths of the photo turned by
// Turn, in the order of the page, top to bottom, when Outcome is Answered — none when the model found
// no task —; and the turn, in whole degrees clockwise from -179 to 180, by which the photo was turned
// for its page to read upright (provider.Turn), 0 with a request and with no frames (R162 spec).
type TasksFound struct {
	Frames  []*observationv1.CropBox
	Turn    int
	Outcome Outcome
}

// found is what one model of the finding gave: the frames, on the photo turned by turn.
type found struct {
	frames []*observationv1.CropBox
	turn   int
}

// FindTasks asks the admitted models of the finding (CallFinding) for the tasks on the photo of a
// page, one at a time, a later one only when the one before failed or found no task (R137). Without
// the learner's request each model is asked first which way up the page lies, and the photo is
// turned upright by its answer before the tasks are found on it; with one, the photo is app's,
// already turned (R134). The adapter fits the photo to the size its model takes unshrunk, and it
// goes as a JPEG without the metadata of its file (provider.Photo), decoded once for every model
// and turned once for each angle. A photo that is no JPEG or PNG fails before any call; an answer
// off the schema fails the attempt. A box with no area on the photo is no frame, nor one that
// covers a frame the learner drew, which the models' cascade drops once it is over: a model that
// gave only such frames found tasks, and the next one is not asked. The frames go top to bottom,
// and of two at one height the left first, the first finding.MaxTasks of them (R165 spec).
func (t *Tutor) FindTasks(ctx context.Context, f TaskFinding) TasksFound {
	calls, a, refused := t.prepare(CallFinding, f.Route)
	if refused != nil {
		return TasksFound{Outcome: *refused}
	}
	photos := provider.NewPhotos(f.Photo)
	got, out := cascade(ctx, calls, func(c call) (found, Outcome) {
		return t.findBy(ctx, a, c, f, photos)
	}, func(g found) bool { return len(g.frames) > 0 })
	frames := got.frames
	if f.Request != "" {
		frames = slices.DeleteFunc(frames, func(b *observationv1.CropBox) bool { return CoversDrawn(b, f.Drawn) })
	}
	if len(frames) > finding.MaxTasks {
		frames = frames[:finding.MaxTasks]
	}
	if len(frames) == 0 {
		got.turn = 0
	}
	return TasksFound{Frames: frames, Turn: got.turn, Outcome: out}
}

// findBy is one attempt of the finding, by one model: without a request, the orientation of the
// page, by which photos is turned; then the frames of the tasks on the photo as photos fits it. Its
// outcome carries the usage of both requests.
func (t *Tutor) findBy(ctx context.Context, a admission.Answer, c call, f TaskFinding, photos *provider.Photos) (found, Outcome) {
	var before []Usage
	fail := func(err error) (found, Outcome) {
		return found{}, Outcome{Kind: Failed, Admission: a, Usage: before, Err: err}
	}
	turn := 0
	if f.Request == "" {
		var out Outcome
		if turn, out = orient(ctx, a, c, photos); out.Kind != Answered {
			return found{}, out
		}
		before, photos = out.Usage, photos.Turned(turn)
	}
	fitted, width, height, err := photos.For(c.client)
	if err != nil {
		return fail(fmt.Errorf("tutor: %w", err))
	}
	req := finding.RequestOf(fitted, width, height, f.Request, framesOf(f, width, height))
	req.MaxOutputTokens = finding.MaxOutputTokens
	if err := checkText(req); err != nil {
		return fail(err)
	}
	resps, out := ask(ctx, a, []call{c}, []provider.Request{req}, kinds[CallFinding].maxIn)
	out.Usage = append(before, out.Usage...)
	if out.Kind != Answered {
		return found{}, out
	}
	boxes, err := finding.Read(resps[0].JSON)
	if err != nil {
		out.Kind, out.Err = Failed, fmt.Errorf("tutor: %s: %w", c.model.Name(), err)
		return found{}, out
	}
	frames := make([]*observationv1.CropBox, 0, len(boxes))
	for _, b := range boxes {
		place, ok := provider.Place(b.X1, b.Y1, b.X2, b.Y2, width, height)
		if !ok {
			continue
		}
		frames = append(frames, &observationv1.CropBox{Ymin: int32(place[0]), Xmin: int32(place[1]), Ymax: int32(place[2]), Xmax: int32(place[3])})
	}
	slices.SortStableFunc(frames, func(x, y *observationv1.CropBox) int {
		return cmp.Or(cmp.Compare(x.Ymin, y.Ymin), cmp.Compare(x.Xmin, y.Xmin))
	})
	return found{frames: frames, turn: turn}, out
}

// orient asks one model which way up the page lies on the photo, as photos fits it, and gives the
// turn that puts the page upright (finding.Orientation.Turn). An answer off the schema fails the
// attempt, as the finding's does, and the cascade goes on.
func orient(ctx context.Context, a admission.Answer, c call, photos *provider.Photos) (int, Outcome) {
	fitted, width, height, err := photos.For(c.client)
	if err != nil {
		return 0, Outcome{Kind: Failed, Admission: a, Err: fmt.Errorf("tutor: %w", err)}
	}
	req := finding.OrientRequestOf(fitted, width, height)
	req.MaxOutputTokens = finding.OrientMaxOutputTokens
	if err := checkText(req); err != nil {
		return 0, Outcome{Kind: Failed, Admission: a, Err: err}
	}
	resps, out := ask(ctx, a, []call{c}, []provider.Request{req}, kinds[CallFinding].maxIn)
	if out.Kind != Answered {
		return 0, out
	}
	o, err := finding.ReadOrientation(resps[0].JSON)
	if err != nil {
		out.Kind, out.Err = Failed, fmt.Errorf("tutor: %s: %w", c.model.Name(), err)
		return 0, out
	}
	return o.Turn(), out
}

// framesOf are the frames a request names, in pixels of the photo the model sees, in the order of the
// page as app numbers them — top to bottom, and of two at one height the left first —; none without
// a request.
func framesOf(f TaskFinding, width, height int) []finding.Frame {
	if f.Request == "" {
		return nil
	}
	type on struct {
		box   *observationv1.CropBox
		drawn bool
	}
	all := make([]on, 0, len(f.Found)+len(f.Drawn))
	for _, b := range f.Found {
		all = append(all, on{b, false})
	}
	for _, b := range f.Drawn {
		all = append(all, on{b, true})
	}
	slices.SortStableFunc(all, func(x, y on) int {
		return cmp.Or(cmp.Compare(x.box.Ymin, y.box.Ymin), cmp.Compare(x.box.Xmin, y.box.Xmin))
	})
	px := func(v int32, size int) int { return (int(v)*size + 500) / 1000 }
	frames := make([]finding.Frame, len(all))
	for i, o := range all {
		frames[i] = finding.Frame{Drawn: o.drawn, Box: finding.Box{
			X1: px(o.box.Xmin, width), Y1: px(o.box.Ymin, height), X2: px(o.box.Xmax, width), Y2: px(o.box.Ymax, height)}}
	}
	return frames
}

// CoversDrawn says whether a frame found covers one of the frames the learner drew, its overlap
// at least sameAsDrawn of their union: the finding drops it, and the learner's own stays.
func CoversDrawn(frame *observationv1.CropBox, drawn []*observationv1.CropBox) bool {
	return slices.ContainsFunc(drawn, func(d *observationv1.CropBox) bool { return overlap(frame, d) >= sameAsDrawn })
}

// overlap is the area two frames share, of the area of their union: 1 for the same frame, 0 for
// two apart.
func overlap(a, b *observationv1.CropBox) float64 {
	w := min(a.Xmax, b.Xmax) - max(a.Xmin, b.Xmin)
	h := min(a.Ymax, b.Ymax) - max(a.Ymin, b.Ymin)
	if w <= 0 || h <= 0 {
		return 0
	}
	area := func(c *observationv1.CropBox) float64 { return float64(c.Xmax-c.Xmin) * float64(c.Ymax-c.Ymin) }
	shared := float64(w) * float64(h)
	return shared / (area(a) + area(b) - shared)
}
