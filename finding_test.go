package tutor

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	"strings"
	"testing"

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
	if got.Outcome.Kind != Answered || len(got.Outcome.Usage) != 1 || len(got.Frames) != 3 {
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
	r := haiku.asked[0]
	if len(r.Images) != 1 || len(r.Images[0].JPEG) == 0 || r.Effort != "high" || r.MaxOutputTokens != finding.MaxOutputTokens ||
		!strings.Contains(r.Text, "400 pixels wide and 500 pixels high") {
		t.Errorf("the request is %+v", r)
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(r.Images[0].JPEG))
	if err != nil || cfg.Width != 400 || cfg.Height != 500 {
		t.Errorf("the photo sent is %+v, %v", cfg, err)
	}
}

// No task on the page, by every model, is an answer of no frames, not a failure: the learner frames
// them by hand.
func TestFindTasksNone(t *testing.T) {
	none := `{"tasks": []}`
	lib, m := cascadeOf(t, none, none, none)
	if got := lib.FindTasks(context.Background(), TaskFinding{Route: platform, Photo: photoJPEG(t, 40, 30)}); got.Outcome.Kind != Answered || len(got.Frames) != 0 || len(m["claude-opus-5-5"].asked) != 1 {
		t.Errorf("found %+v", got)
	}
}

func TestFindTasksFails(t *testing.T) {
	// An answer off the schema of every model fails the call, with what each took.
	off := `{"tasks": [{"x1": 1}]}`
	lib, m := cascadeOf(t, off, off, off)
	haiku := m["claude-haiku-5-5"]
	if got := lib.FindTasks(context.Background(), TaskFinding{Route: platform, Photo: photoJPEG(t, 40, 30)}); got.Outcome.Kind != Failed || len(got.Outcome.Usage) != 3 {
		t.Errorf("off the schema: %+v", got)
	}
	// A photo that is no picture fails before any call.
	if got := lib.FindTasks(context.Background(), TaskFinding{Route: platform, Photo: []byte("no photo")}); got.Outcome.Kind != Failed || got.Outcome.Err == nil {
		t.Errorf("no photo: %+v", got)
	}
	if len(haiku.asked) != 1 {
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
