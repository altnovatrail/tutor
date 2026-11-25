package tutor

import (
	"context"
	"errors"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/altnovatrail/spec/types/admissionv0"

	"github.com/altnovatrail/tutor/internal/grownup"
	"github.com/altnovatrail/tutor/internal/synth"
	"github.com/altnovatrail/tutor/provider"
)

// The acts of the drawing on a work of 400 × 200 px with the strokes 3 and 4: a red line through
// stroke 3 and a line of text; one that takes off a stroke the request did not name; and one that
// writes a character the hand has not.
const (
	goodAct      = `{"say": "Struck out and solved.", "erase": [4], "marks": [{"kind": "polyline", "points": [[10, 50], [390, 52]], "color": "#ff3b30", "width": 3}, {"kind": "text", "x": 20, "y": 150, "text": "x = 2", "size": 30}]}`
	unnamedAct   = `{"say": "Erased.", "erase": [4, 99], "marks": []}`
	snowmanAct   = `{"say": "Drawn.", "marks": [{"kind": "text", "x": 20, "y": 150, "text": "☃ = 2", "size": 30}]}`
	nothingAct   = `{"say": " ", "marks": [{"kind": "polyline", "points": [[1, 1]]}]}`
	offSchemaAct = `{"say": "Done.", "marks": [{"kind": "path", "d": "M 0 0 L 10 10"}]}`
)

// drawingOf is a library whose drawing cascade answers by the tool, Haiku then Sonnet.
func drawingOf(t *testing.T, haiku, sonnet string) (*Tutor, map[string]*fake) {
	t.Helper()
	lib, m := cascadeOf(t, haiku, sonnet, `{}`)
	m["claude-haiku-5-5"].resp.Tool = grownup.ToolName
	m["claude-sonnet-5-5"].resp.Tool = grownup.ToolName
	return lib, m
}

func grownUpRequest(t *testing.T) GrownUpRequest {
	t.Helper()
	return GrownUpRequest{Route: platform, Question: "Cross out the first line and solve it", Statement: photoJPEG(t, 300, 120), Work: cropPNG(t, 400, 200),
		Strokes:  []SheetStroke{{ID: 3, Points: []float32{10, 50, 200, 51, 390, 52}}, {ID: 4, Points: []float32{10, 100, 390, 100}}},
		Turns:    []PaperTurn{{Learner: true, Text: "What is this?"}, {Text: "An equation."}},
		Language: "en"}
}

// An act Haiku does well is the answer: the words, the polyline from its ends, laid densely, the text in the hand,
// the stroke to take off; Haiku is asked at medium with the tool, the work as its PNG and the
// task's crop as a photo, and Sonnet is not asked.
func TestGrownUp(t *testing.T) {
	lib, m := drawingOf(t, goodAct, `{}`)
	got := lib.GrownUp(context.Background(), grownUpRequest(t))
	if got.Outcome.Kind != Answered || got.Words != "Struck out and solved." || !slices.Equal(got.Erase, []int32{4}) || timesAsked(m) != [3]int{1, 0, 0} || len(got.Outcome.Usage) != 1 {
		t.Fatalf("drawn %+v, asked %v", got, timesAsked(m))
	}
	if s := got.Strokes[0]; !endsAre(s.Points, 10, 50, 390, 52) || len(s.Points) <= 4 || s.Color != "#ff3b30" || s.Width != 3 {
		t.Errorf("the line is %+v", s)
	}
	text, _ := synth.Handwriting("x = 2", 20, 150, 30)
	if len(got.Strokes) != 1+len(text) || got.Strokes[1].Points[0] != float32(text[0][0].X) || got.Strokes[1].Color != "#1d1d1f" {
		t.Errorf("the text is %d strokes, want %d: %+v", len(got.Strokes)-1, len(text), got.Strokes[1:])
	}
	r := m["claude-haiku-5-5"].asked[0]
	if r.Effort != "medium" || r.MaxOutputTokens != grownup.DefaultMaxOutputTokens || len(r.Tools) != 1 || r.Tools[0].Name != grownup.ToolName ||
		r.ToolChoice != provider.ToolAuto || len(r.Images) != 2 || len(r.Images[0].PNG) == 0 || len(r.Images[1].JPEG) == 0 ||
		!strings.Contains(r.Text, "Image 1 is 400 pixels wide and 200 pixels high.") || !strings.Contains(r.Text, `{"id":3,"points":[[10,50],[200,51],[390,52]]}`) ||
		!strings.Contains(r.Text, "- Grown-up: What is this?\n- Tutor: An equation.") || !strings.Contains(r.Text, "Cross out the first line and solve it") {
		t.Errorf("the request is %+v", r)
	}
}

// A later model is asked when the act before took off a stroke the request did not name, wrote a
// character the hand has not, did nothing, was off the schema, or no tool came; Sonnet at medium,
// since its own default is high.
func TestGrownUpCascade(t *testing.T) {
	for name, first := range map[string]string{"an ID not named": unnamedAct, "a snowman": snowmanAct, "nothing": nothingAct, "off the schema": offSchemaAct} {
		lib, m := drawingOf(t, first, goodAct)
		got := lib.GrownUp(context.Background(), grownUpRequest(t))
		if got.Outcome.Kind != Answered || got.Words != "Struck out and solved." || timesAsked(m) != [3]int{1, 1, 0} || len(got.Outcome.Usage) != 2 {
			t.Errorf("%s: drawn %+v, asked %v", name, got, timesAsked(m))
		}
		if q := m["claude-sonnet-5-5"].asked; len(q) != 1 || q[0].Effort != "medium" {
			t.Errorf("%s: Sonnet was asked %+v", name, q)
		}
	}
	lib, m := drawingOf(t, goodAct, goodAct)
	m["claude-haiku-5-5"].err = provider.ErrNoAnswer
	if got := lib.GrownUp(context.Background(), grownUpRequest(t)); got.Outcome.Kind != Answered || timesAsked(m) != [3]int{1, 1, 0} {
		t.Errorf("no tool from Haiku: %+v", got)
	}
}

// When no act will do, the last that answered stands, put right: the IDs not named left out and
// the characters not written left out; one that does nothing at all fails, and so does a cascade
// whose every act is off the schema.
func TestGrownUpLastStands(t *testing.T) {
	lib, _ := drawingOf(t, snowmanAct, unnamedAct)
	got := lib.GrownUp(context.Background(), grownUpRequest(t))
	if got.Outcome.Kind != Answered || got.Words != "Erased." || !slices.Equal(got.Erase, []int32{4}) || len(got.Strokes) != 0 {
		t.Errorf("an act put right: %+v", got)
	}
	lib, _ = drawingOf(t, unnamedAct, snowmanAct)
	if got := lib.GrownUp(context.Background(), grownUpRequest(t)); got.Outcome.Kind != Answered || len(got.Strokes) == 0 || got.Erase != nil {
		t.Errorf("a text without its snowman: %+v", got)
	}
	lib, _ = drawingOf(t, nothingAct, nothingAct)
	if got := lib.GrownUp(context.Background(), grownUpRequest(t)); got.Outcome.Kind != Failed || got.Outcome.Err == nil || len(got.Outcome.Usage) != 2 {
		t.Errorf("nothing done: %+v", got)
	}
	lib, _ = drawingOf(t, offSchemaAct, offSchemaAct)
	if got := lib.GrownUp(context.Background(), grownUpRequest(t)); got.Outcome.Kind != Failed || !strings.Contains(got.Outcome.Err.Error(), "off the schema") {
		t.Errorf("off the schema: %+v", got)
	}
}

// endsAre tells whether a stroke's points begin at x0, y0 and end at x1, y1.
func endsAre(points []float32, x0, y0, x1, y1 float32) bool {
	n := len(points)
	return n >= 4 && slices.Equal(points[:2], []float32{x0, y0}) && slices.Equal(points[n-2:], []float32{x1, y1})
}

// A point beyond the work is moved onto its edge, and a model whose act had one gives way to the
// next.
func TestGrownUpOffTheWork(t *testing.T) {
	far := `{"say": "Done.", "marks": [{"kind": "polyline", "points": [[10, 50], [900, 52]]}]}`
	near := `{"say": "Done.", "marks": [{"kind": "polyline", "points": [[-5, 50], [404, 205]]}]}`
	lib, m := drawingOf(t, far, near)
	got := lib.GrownUp(context.Background(), grownUpRequest(t))
	if got.Outcome.Kind != Answered || timesAsked(m) != [3]int{1, 1, 0} || !endsAre(got.Strokes[0].Points, 0, 50, 400, 200) {
		t.Errorf("drawn %+v, asked %v", got, timesAsked(m))
	}
}

// What the contract refuses fails before any call, and so do a work that is no PNG and a request
// with no crop of the task; a refusal of the admission is "not admitted", with no call either.
func TestGrownUpRefuses(t *testing.T) {
	lib, m := drawingOf(t, goodAct, goodAct)
	for name, change := range map[string]func(*GrownUpRequest){
		"no words":        func(g *GrownUpRequest) { g.Question = "  " },
		"a long question": func(g *GrownUpRequest) { g.Question = strings.Repeat("a", 501) },
		"too many turns":  func(g *GrownUpRequest) { g.Turns = make([]PaperTurn, 9) },
		"a long stroke":   func(g *GrownUpRequest) { g.Strokes = []SheetStroke{{ID: 1, Points: make([]float32, 1026)}} },
		"no PNG":          func(g *GrownUpRequest) { g.Work = photoJPEG(t, 40, 40) },
		"no crop":         func(g *GrownUpRequest) { g.Statement = nil },
		"no language":     func(g *GrownUpRequest) { g.Language = "" },
		"no picture":      func(g *GrownUpRequest) { g.Statement = []byte("not a picture") },
	} {
		g := grownUpRequest(t)
		change(&g)
		if got := lib.GrownUp(context.Background(), g); got.Outcome.Kind != Failed || got.Outcome.Err == nil || len(got.Strokes) != 0 {
			t.Errorf("%s: %+v", name, got)
		}
	}
	if timesAsked(m) != [3]int{0, 0, 0} {
		t.Errorf("a model was asked: %v", timesAsked(m))
	}
	g := grownUpRequest(t)
	g.Route = Route{AI: admissionv0.AiRoutePlatformAi, Category: admissionv0.AgeCategoryUnder13}
	if got := lib.GrownUp(context.Background(), g); got.Outcome.Kind != NotAdmitted || got.Outcome.Admission.Admitted || len(got.Outcome.Usage) != 0 || timesAsked(m) != [3]int{0, 0, 0} {
		t.Errorf("a child without the facts for minors: %+v", got)
	}
	withoutWord(t)
	if got := lib.GrownUp(context.Background(), grownUpRequest(t)); got.Outcome.Kind != NotAdmitted {
		t.Errorf("without its entry: %+v", got)
	}
}

// A limit of the provider ends the cascade as its outcome.
func TestGrownUpLimit(t *testing.T) {
	lib, m := drawingOf(t, goodAct, goodAct)
	m["claude-haiku-5-5"].err = &provider.LimitError{Limit: provider.LimitBalance}
	if got := lib.GrownUp(context.Background(), grownUpRequest(t)); got.Outcome.Kind != LimitReached || timesAsked(m) != [3]int{1, 0, 0} {
		t.Errorf("a limit: %+v", got)
	}
	m["claude-haiku-5-5"].err = errors.New("down")
	m["claude-sonnet-5-5"].err = errors.New("down")
	if got := lib.GrownUp(context.Background(), grownUpRequest(t)); got.Outcome.Kind != Failed {
		t.Errorf("both down: %+v", got)
	}
}

// The debug mode pins the model of the drawing: one attempt, recorded with the tool's input.
func TestGrownUpDebug(t *testing.T) {
	lib, m := debugLibrary(t, unnamedAct)
	var rec []DebugCall
	g := grownUpRequest(t)
	g.Route.Debug = &Debug{First: &sonnetLow, Record: func(c DebugCall) { rec = append(rec, c) }}
	got := lib.GrownUp(context.Background(), g)
	if got.Outcome.Kind != Answered || len(m["claude-haiku-5-5"].asked) != 0 || len(m["claude-sonnet-5-5"].asked) != 1 {
		t.Fatalf("drawn %+v", got)
	}
	if len(rec) != 1 || rec[0].Call != CallDrawing || rec[0].Model != sonnetLow || rec[0].Answer != unnamedAct {
		t.Errorf("recorded %+v", rec)
	}
}

// The price of the drawing is that of both attempts, each by its expected tokens, and its worst
// holds the text of the strokes and the work, both pictures, and 64 000 tokens out of each.
func TestGrownUpPrice(t *testing.T) {
	_, m := drawingOf(t, goodAct, goodAct)
	m["claude-haiku-5-5"].prices = provider.Prices{Input: 0.1, Output: 0.5, Read: "2026-11-11"}
	m["claude-sonnet-5-5"].prices = provider.Prices{Input: 2, Output: 10, Read: "2026-11-11"}
	lib := library(t, adultFacts, &fakes{models: m})
	p, a, err := lib.Price(CallDrawing, platform)
	if err != nil || !a.Admitted || !p.Expected || !p.Worst {
		t.Fatalf("%+v %+v %v", p, a, err)
	}
	expected := (6500*0.1+25000*0.5)/1e6 + (6500*2+5500*10)/1e6
	in := float64(kinds[CallDrawing].maxIn)
	worst := (in*0.1+64000*0.5)/1e6 + (in*2+64000*10)/1e6
	if math.Abs(p.ExpectedUSD-expected) > 1e-12 || math.Abs(p.WorstUSD-worst) > 1e-9 || kinds[CallDrawing].maxIn < grownup.MaxTextBytes {
		t.Errorf("expected %v, worst %v; want %v and %v", p.ExpectedUSD, p.WorstUSD, expected, worst)
	}
}
