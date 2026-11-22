package tutor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"math"
	"strings"
	"testing"

	observationv1 "github.com/altnovatrail/spec/contracts/novatrail/observation/v1"
	"github.com/altnovatrail/spec/types/admissionv0"

	"github.com/altnovatrail/tutor/admission"
	"github.com/altnovatrail/tutor/internal/finding"
	"github.com/altnovatrail/tutor/provider"
)

// photoJPEG is a grey photo of w × h px.
func photoJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, image.NewGray(image.Rect(0, 0, w, h)), nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// fitHalf is a fake model whose adapter fits a photo to half its size.
type fitHalf struct{ *fake }

func (f fitHalf) Fit(w, h int) (int, int) { return w / 2, h / 2 }

// The frames come in thousandths of the photo as the model saw it, top to bottom and of one height
// the left first; a box with no area is none. Haiku at high, the first attempt, finds them, and no
// later model is asked.
func TestFindTasks(t *testing.T) {
	haiku := &fake{resp: provider.Response{JSON: `{"tasks": [
		{"x1": 20, "y1": 300, "x2": 380, "y2": 400},
		{"x1": 210, "y1": 20, "x2": 380, "y2": 120},
		{"x1": 20, "y1": 20, "x2": 190, "y2": 120},
		{"x1": 500, "y1": 500, "x2": 600, "y2": 600}
	]}`, Model: "claude-haiku-5-5", InputTokens: 3000, OutputTokens: 200}}
	opus := &fake{}
	fs := &fakes{models: map[string]*fake{"claude-haiku-5-5": haiku, "claude-sonnet-5-5": {}, "claude-opus-5-5": opus}}
	lib, err := New(Config{Table: library(t, adultFacts, fs).table, Facts: library(t, adultFacts, fs).facts, Least: admission.StageAuthor,
		Keys: map[admissionv0.Provider]string{"anthropic": "platform-key"},
		Adapters: map[admissionv0.Provider]provider.Dial{"anthropic": func(id, key string) (provider.Model, error) {
			m, err := fs.dial(id, key)
			if err != nil {
				return nil, err
			}
			return fitHalf{m.(*fake)}, nil
		}}})
	if err != nil {
		t.Fatal(err)
	}
	got := lib.FindTasks(context.Background(), TaskFinding{Route: platform, Photo: photoJPEG(t, 800, 1000)})
	if got.Outcome.Kind != Answered || len(got.Outcome.Usage) != 2 || len(got.Frames) != 3 || got.Turn != 0 {
		t.Fatalf("found %+v", got)
	}
	// The model saw 400 × 500 px.
	f := got.Frames
	if f[0].Ymin != 40 || f[0].Xmin != 50 || f[0].Ymax != 240 || f[0].Xmax != 475 || f[1].Xmin != 525 || f[2].Ymin != 600 || f[2].Ymax != 800 {
		t.Errorf("the frames are %v", f)
	}
	if len(opus.asked) != 0 {
		t.Errorf("Opus was asked %d times", len(opus.asked))
	}
	if o := haiku.asked[0]; o.SchemaName != finding.OrientSchemaName || o.MaxOutputTokens != finding.OrientMaxOutputTokens || o.Effort != "high" ||
		len(o.Images) != 1 || !bytes.Equal(o.Images[0].JPEG, haiku.asked[1].Images[0].JPEG) {
		t.Errorf("the orientation is %+v", o)
	}
	r := haiku.asked[1]
	if len(r.Images) != 1 || len(r.Images[0].JPEG) == 0 || r.Effort != "high" || r.MaxOutputTokens != finding.MaxOutputTokens ||
		!strings.Contains(r.Text, "400 pixels wide and 500 pixels high") {
		t.Errorf("the request is %+v", r)
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(r.Images[0].JPEG))
	if err != nil || cfg.Width != 400 || cfg.Height != 500 {
		t.Errorf("the photo sent is %+v, %v", cfg, err)
	}
}

// Of more boxes than finding.MaxTasks the first of the page are kept, an answer and no failure:
// the model that gave them found tasks, and the next is not asked.
func TestFindTasksKeepsTheFirst(t *testing.T) {
	var boxes []string
	for i := finding.MaxTasks + 4; i >= 1; i-- {
		boxes = append(boxes, fmt.Sprintf(`{"x1": 0, "y1": %d, "x2": 30, "y2": %d}`, i*10, i*10+5))
	}
	lib, m := cascadeOf(t, `{"tasks": [`+strings.Join(boxes, ",")+`]}`, `{}`, `{}`)
	got := lib.FindTasks(context.Background(), TaskFinding{Route: platform, Photo: photoJPEG(t, 40, 300)})
	if got.Outcome.Kind != Answered || len(got.Frames) != finding.MaxTasks || len(m["claude-sonnet-5-5"].asked) != 0 {
		t.Fatalf("found %d frames, %+v", len(got.Frames), got.Outcome)
	}
	if first, last := got.Frames[0], got.Frames[finding.MaxTasks-1]; first.Ymin != 33 || last.Ymin != 666 {
		t.Errorf("the frames from %v to %v", first, last)
	}
}

// No task on the page, by every model, is an answer of no frames, not a failure: the learner frames
// them by hand.
func TestFindTasksNone(t *testing.T) {
	none := `{"tasks": []}`
	lib, m := cascadeOf(t, none, none, none)
	if got := lib.FindTasks(context.Background(), TaskFinding{Route: platform, Photo: photoJPEG(t, 40, 30)}); got.Outcome.Kind != Answered || len(got.Frames) != 0 || len(m["claude-opus-5-5"].asked) != 2 {
		t.Errorf("found %+v", got)
	}
}

func TestFindTasksFails(t *testing.T) {
	// An answer off the schema of every model fails the call, with what each took.
	off := `{"tasks": [{"x1": 1}]}`
	lib, m := cascadeOf(t, off, off, off)
	haiku := m["claude-haiku-5-5"]
	if got := lib.FindTasks(context.Background(), TaskFinding{Route: platform, Photo: photoJPEG(t, 40, 30)}); got.Outcome.Kind != Failed || len(got.Outcome.Usage) != 6 {
		t.Errorf("off the schema: %+v", got)
	}
	// A photo that is no picture fails before any call.
	if got := lib.FindTasks(context.Background(), TaskFinding{Route: platform, Photo: []byte("no photo")}); got.Outcome.Kind != Failed || got.Outcome.Err == nil {
		t.Errorf("no photo: %+v", got)
	}
	if len(haiku.asked) != 2 {
		t.Errorf("%d calls asked", len(haiku.asked))
	}
	// A provider's limit is the outcome LimitReached.
	haiku.err = &provider.LimitError{Limit: provider.LimitOwner}
	if got := lib.FindTasks(context.Background(), TaskFinding{Route: platform, Photo: photoJPEG(t, 40, 30)}); got.Outcome.Kind != LimitReached {
		t.Errorf("a limit: %+v", got)
	}
	haiku.err = errors.New("down")
	if got := lib.FindTasks(context.Background(), TaskFinding{Route: platform, Photo: photoJPEG(t, 40, 30)}); got.Outcome.Kind != Failed {
		t.Errorf("a failure: %+v", got)
	}
}

// Without the entry of the finding nothing is asked.
func TestFindTasksNotAdmitted(t *testing.T) {
	withoutWord(t)
	lib, opus := tutorOf(t, `{"tasks": []}`)
	if got := lib.FindTasks(context.Background(), TaskFinding{Route: platform, Photo: photoJPEG(t, 40, 30)}); got.Outcome.Kind != NotAdmitted {
		t.Errorf("found %+v", got)
	}
	if len(opus.asked) != 0 {
		t.Errorf("%d calls asked though not admitted", len(opus.asked))
	}
}

// With the learner's request the model gets it and the frames on the page, numbered in its order —
// top to bottom, and of one height the left first — in pixels of the photo it sees; a frame it gives
// that covers one the learner drew is dropped, the learner's own staying; a model that gave only
// such frames found tasks, and the next is not asked. Without a request no frame is sent and none is
// dropped.
func TestFindTasksByARequest(t *testing.T) {
	answer := `{"tasks": [{"x1": 0, "y1": 200, "x2": 400, "y2": 300}, {"x1": 0, "y1": 50, "x2": 200, "y2": 150}, {"x1": 200, "y1": 50, "x2": 400, "y2": 150}]}`
	lib, m := cascadeOf(t, answer, `{}`, `{}`)
	f := TaskFinding{Route: platform, Photo: photoJPEG(t, 1000, 500), Request: "split frame 1",
		Found: []*observationv1.CropBox{{Ymin: 100, Xmin: 500, Ymax: 300, Xmax: 900}, {Ymin: 100, Xmin: 0, Ymax: 300, Xmax: 400}},
		Drawn: []*observationv1.CropBox{{Ymin: 400, Xmin: 0, Ymax: 600, Xmax: 400}}}
	got := lib.FindTasks(context.Background(), f)
	if got.Outcome.Kind != Answered || len(got.Frames) != 2 || got.Frames[0].Xmax != 200 || got.Frames[1].Xmin != 200 || got.Frames[0].Ymin != 100 {
		t.Fatalf("found %+v", got)
	}
	if q := m["claude-haiku-5-5"].asked; len(q) != 1 || q[0].SchemaName != "task_frames" || got.Turn != 0 {
		t.Errorf("with a request, asked %d times, turned %d", len(q), got.Turn)
	}
	want := "<frames>\n1. found x1=0 y1=50 x2=400 y2=150\n2. found x1=500 y1=50 x2=900 y2=150\n3. drawn x1=0 y1=200 x2=400 y2=300\n</frames>"
	if text := m["claude-haiku-5-5"].asked[0].Text; !strings.Contains(text, "\nsplit frame 1\n</request>") || !strings.HasSuffix(text, want) {
		t.Errorf("the text is %q", text)
	}
	f.Request = ""
	got = lib.FindTasks(context.Background(), f)
	if text := m["claude-haiku-5-5"].asked[2].Text; len(got.Frames) != 3 || strings.Contains(text, "<request>") || strings.Contains(text, "<frames>") {
		t.Errorf("without a request: %+v, %q", got, text)
	}

	repeats, r := cascadeOf(t, `{"tasks": [{"x1": 0, "y1": 200, "x2": 400, "y2": 300}]}`, `{}`, `{}`)
	f.Request = "only the drawn one"
	if got := repeats.FindTasks(context.Background(), f); got.Outcome.Kind != Answered || len(got.Frames) != 0 || len(r["claude-sonnet-5-5"].asked) != 0 {
		t.Errorf("only a drawn frame repeated: %+v, Sonnet asked %d times", got, len(r["claude-sonnet-5-5"].asked))
	}
}

// Without a request each attempt asks first which way up the page lies, on the photo as it came,
// and then finds the tasks on the photo turned upright: the frames are in thousandths of the turned
// photo, and the turn is the attempt's whose frames came.
func TestFindTasksTurnsThePage(t *testing.T) {
	lib, m := cascadeOf(t, noTasks, twoTasks, twoTasks)
	haiku, sonnet := m["claude-haiku-5-5"], m["claude-sonnet-5-5"]
	haiku.orient, sonnet.orient = `{"top": "down", "tilt": 0}`, `{"top": "left", "tilt": 1}`
	got := lib.FindTasks(context.Background(), TaskFinding{Route: platform, Photo: photoJPEG(t, 40, 30)})
	if got.Outcome.Kind != Answered || got.Turn != 90 || len(got.Frames) != 2 || len(got.Outcome.Usage) != 4 || timesAsked(m) != [3]int{2, 2, 0} {
		t.Fatalf("found %+v, asked %v", got, timesAsked(m))
	}
	// The top at the left: a quarter clockwise, 40 × 30 px turned to 30 × 40.
	sizes := func(q []provider.Request) []string {
		var s []string
		for _, r := range q {
			cfg, err := jpeg.DecodeConfig(bytes.NewReader(r.Images[0].JPEG))
			if err != nil {
				t.Fatal(err)
			}
			s = append(s, fmt.Sprintf("%s %d×%d", r.SchemaName, cfg.Width, cfg.Height))
		}
		return s
	}
	if s := sizes(sonnet.asked); s[0] != "page_orientation 40×30" || s[1] != "task_frames 30×40" || !strings.Contains(sonnet.asked[1].Text, "30 pixels wide and 40 pixels high") {
		t.Errorf("Sonnet was asked %v", s)
	}
	if s := sizes(haiku.asked); s[1] != "task_frames 40×30" {
		t.Errorf("Haiku was asked %v", s)
	}
	if f := got.Frames[0]; f.Xmin != 66 || f.Ymin != 50 || f.Xmax != 600 || f.Ymax != 250 {
		t.Errorf("the first frame is %v, in thousandths of 30 × 40", f)
	}
}

// An orientation off the schema fails its attempt, with no finding asked of its model, and the
// cascade goes on; with no frames there is no turn; with a request no orientation is asked.
func TestFindTasksOrientation(t *testing.T) {
	lib, m := cascadeOf(t, twoTasks, twoTasks, twoTasks)
	m["claude-haiku-5-5"].orient = `{"top": "sideways", "tilt": 0}`
	m["claude-sonnet-5-5"].orient = `{"top": "right", "tilt": -30}`
	got := lib.FindTasks(context.Background(), TaskFinding{Route: platform, Photo: photoJPEG(t, 40, 30)})
	if got.Outcome.Kind != Answered || got.Turn != -120 || len(got.Frames) != 2 || len(got.Outcome.Usage) != 3 || timesAsked(m) != [3]int{1, 2, 0} {
		t.Errorf("off the schema first: %+v, asked %v", got, timesAsked(m))
	}
	lib, m = cascadeOf(t, noTasks, noTasks, noTasks)
	for _, f := range m {
		f.orient = `{"top": "left", "tilt": 0}`
	}
	if got := lib.FindTasks(context.Background(), TaskFinding{Route: platform, Photo: photoJPEG(t, 40, 30)}); got.Outcome.Kind != Answered || len(got.Frames) != 0 || got.Turn != 0 {
		t.Errorf("no frames: %+v", got)
	}
	lib, m = cascadeOf(t, twoTasks, twoTasks, twoTasks)
	m["claude-haiku-5-5"].orient = `{"top": "left", "tilt": 0}`
	if got := lib.FindTasks(context.Background(), TaskFinding{Route: platform, Photo: photoJPEG(t, 40, 30), Request: "each apart"}); got.Turn != 0 || len(got.Frames) != 2 || timesAsked(m) != [3]int{1, 0, 0} || m["claude-haiku-5-5"].asked[0].SchemaName != "task_frames" {
		t.Errorf("with a request: %+v, asked %v", got, timesAsked(m))
	}
}

// The debug mode records both requests of an attempt, the orientation first, each with its answer
// and usage, at the attempt's place.
func TestDebugRecordsTheOrientation(t *testing.T) {
	lib, m := debugLibrary(t, twoTasks)
	m["claude-sonnet-5-5"].orient = `{"top": "up", "tilt": 5}`
	var rec []DebugCall
	r := platform
	r.Debug = &Debug{First: &sonnetLow, Record: func(c DebugCall) { rec = append(rec, c) }}
	got := lib.FindTasks(context.Background(), TaskFinding{Route: r, Photo: photoJPEG(t, 40, 30)})
	if got.Turn != 5 || len(rec) != 2 || rec[0].Answer != `{"top": "up", "tilt": 5}` || rec[1].Answer != twoTasks ||
		rec[0].Call != CallFinding || rec[1].Call != CallFinding || rec[0].Attempt != 1 || rec[1].Attempt != 1 || rec[0].Usage == nil || rec[1].Usage == nil {
		t.Errorf("found %+v, recorded %+v", got, rec)
	}
}

// The worst price of the finding holds both requests of each attempt: the most input twice, and
// the most output of each.
func TestFindingPrice(t *testing.T) {
	_, m := cascadeOf(t, noTasks, noTasks, noTasks)
	for _, f := range m {
		f.prices = provider.Prices{Input: 1, Output: 5, Read: "2026-11-22"}
	}
	lib := library(t, adultFacts, &fakes{models: m})
	p, _, err := lib.Price(CallFinding, platform)
	in := float64(maxTextBytes + maxPhotoTokens)
	want := 3 * (2*in*1 + float64(finding.MaxOutputTokens+finding.OrientMaxOutputTokens)*5) / 1e6
	if err != nil || !p.Worst || math.Abs(p.WorstUSD-want) > 1e-12 {
		t.Errorf("the worst is %v, want %v (%v)", p.WorstUSD, want, err)
	}
}
