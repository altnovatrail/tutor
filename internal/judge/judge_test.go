package judge_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/internal/journal"
	"github.com/altnovatrail/tutor/internal/judge"
	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/synth"
	"github.com/altnovatrail/tutor/provider"
)

var update = flag.Bool("update", false, "write the golden prompts anew")

// sheet is what the forms show of a synthetic sheet, and its task.
func sheet(t *testing.T, sc sample.Scenario) (*sample.Task, *judge.Sheet) {
	t.Helper()
	s := synth.Sample(sc)
	bank := synth.Bank()
	i := slices.IndexFunc(bank.Tasks, func(task sample.Task) bool { return task.ID == s.Task })
	sh, err := judge.Prepare(s)
	if err != nil {
		t.Fatal(err)
	}
	return &bank.Tasks[i], sh
}

// Prepare draws the finished sheet at its size, its journal and the frames the journal chose.
func TestPrepare(t *testing.T) {
	s := synth.Sample(sample.ScenarioErasureFixed)
	sh, err := judge.Prepare(s)
	if err != nil {
		t.Fatal(err)
	}
	j := journal.Of(s)
	if sh.Width != 1000 || sh.Height != 600 || len(sh.Frames) != len(j.Frames) || len(sh.Frames) != 2 {
		t.Fatalf("the sheet is %d × %d with %d frames, want 1000 × 600 and the journal's 2", sh.Width, sh.Height, len(sh.Frames))
	}
	for i, f := range j.Frames {
		img, err := render.Before(s, f.Erasure)
		if err != nil {
			t.Fatal(err)
		}
		back, err := png.Decode(bytes.NewReader(sh.Frames[i]))
		if err != nil {
			t.Fatal(err)
		}
		if g, ok := back.(*image.Gray); !ok || !bytes.Equal(g.Pix, img.Pix) {
			t.Errorf("frame %d is not the sheet before erasure %d", i+1, f.Erasure)
		}
	}
	if _, err := judge.Prepare(&sample.Sample{Sheet: sample.Sheet{Width: 5000, Height: 600}}); err == nil {
		t.Error("a sheet the renderer cannot draw is prepared")
	}
}

// written puts a request in words: its instructions, its pictures by their labels and which of
// the sheet's pictures each is, and its text.
func written(r provider.Request, sh *judge.Sheet) string {
	var b strings.Builder
	fmt.Fprintf(&b, "=== instructions\n%s\n", r.Instructions)
	for _, im := range r.Images {
		which := "the finished sheet"
		for i, f := range sh.Frames {
			if string(f) == string(im.PNG) {
				which = fmt.Sprintf("frame %d", i+1)
			}
		}
		fmt.Fprintf(&b, "=== picture: %s\n%s\n", which, im.Label)
	}
	fmt.Fprintf(&b, "=== text\n%s\n", r.Text)
	return b.String()
}

// The golden prompts of the four forms, on the synthetic sheet, are the files testdata/A.txt to
// testdata/D.txt; the author reads them there.
func TestGolden(t *testing.T) {
	task, sh := sheet(t, sample.ScenarioErasureFixed)
	for _, f := range judge.Forms {
		r, err := f.Request(task, sh)
		if err != nil {
			t.Fatal(err)
		}
		got := written(r, sh)
		path := filepath.Join("testdata", string(f)+".txt")
		if *update {
			if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if got != string(want) {
			t.Errorf("form %s: the request is\n%s\nwant, as %s,\n%s", f, got, path, want)
		}
	}
}

// The forms differ only in the block that tells their inputs, and in those inputs: the journal in
// the text of B and D, the frames after the picture in C and D.
func TestForms(t *testing.T) {
	task, sh := sheet(t, sample.ScenarioErasureFixed)
	a, err := judge.FormA.Request(task, sh)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range judge.Forms {
		r, err := f.Request(task, sh)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(r.Instructions, a.Instructions) {
			t.Errorf("form %s: the instructions do not start with those of form A", f)
		}
		block := strings.TrimPrefix(r.Instructions, a.Instructions)
		if (block == "") != (f == judge.FormA) {
			t.Errorf("form %s: its own block is %q", f, block)
		}
		if !strings.Contains(r.Instructions, "Writing that was erased is not part of the solution.") {
			t.Errorf("form %s does not say that erased writing is not part of the solution", f)
		}
		journalText := map[judge.Form]string{judge.FormB: sh.Journal.Text(false), judge.FormD: sh.Journal.Text(true)}[f]
		if want := a.Text + "\n\n" + strings.TrimRight(journalText, "\n"); journalText != "" && r.Text != want {
			t.Errorf("form %s: the text is\n%s\nwant the text of form A and the journal", f, r.Text)
		} else if journalText == "" && r.Text != a.Text {
			t.Errorf("form %s: the text is\n%s\nwant that of form A", f, r.Text)
		}
		pictures := 1
		if f.Frames() {
			pictures += len(sh.Frames)
		}
		if len(r.Images) != pictures || string(r.Images[0].PNG) != string(sh.Final) || r.Images[0].Label != a.Images[0].Label {
			t.Errorf("form %s: %d pictures, want %d, the finished sheet first", f, len(r.Images), pictures)
		}
		if r.SchemaName != "verdict" || r.Schema == nil {
			t.Errorf("form %s asks for the schema %q", f, r.SchemaName)
		}
	}
	// Form D's journal names the frames, and form B's does not.
	b, _ := judge.FormB.Request(task, sh)
	d, _ := judge.FormD.Request(task, sh)
	if strings.Contains(b.Text, "Frame 1") || !strings.Contains(d.Text, "Frame 1 shows the sheet just before this.") {
		t.Errorf("the journal of form B or D is not as it should be:\n%s\n---\n%s", b.Text, d.Text)
	}
}

func TestRequestRefusals(t *testing.T) {
	task, sh := sheet(t, sample.ScenarioErasureFixed)
	if _, err := judge.Form("E").Request(task, sh); err == nil {
		t.Error("a form E is asked")
	}
	// Frames that are not the journal's.
	cut := *sh
	cut.Frames = cut.Frames[:1]
	for _, f := range judge.Forms {
		if _, err := f.Request(task, &cut); err == nil {
			t.Errorf("form %s is asked with one frame of the journal's two", f)
		}
	}
	task, plain := sheet(t, sample.ScenarioCorrect)
	for _, f := range judge.Forms {
		if _, err := f.Request(task, plain); (err == nil) != !f.Frames() {
			t.Errorf("form %s of a sheet without frames: %v", f, err)
		}
	}
}

// The schema is valid JSON with the judge's verdicts and a box of four corners, the verdict first.
func TestSchema(t *testing.T) {
	raw, ok := judge.Schema["properties"].(json.RawMessage)
	if !ok || !json.Valid(raw) {
		t.Fatalf("the schema's properties are not valid JSON:\n%s", raw)
	}
	var props struct {
		Verdict struct {
			Enum []string `json:"enum"`
		} `json:"verdict"`
		Box struct {
			AnyOf []struct {
				Type     string          `json:"type"`
				Required []string        `json:"required"`
				Props    json.RawMessage `json:"properties"`
			} `json:"anyOf"`
		} `json:"box"`
	}
	if err := json.Unmarshal(raw, &props); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(props.Verdict.Enum, judge.Verdicts) {
		t.Errorf("the schema's verdicts = %v, want %v", props.Verdict.Enum, judge.Verdicts)
	}
	if len(props.Box.AnyOf) != 2 || props.Box.AnyOf[0].Type != "null" || !slices.Equal(props.Box.AnyOf[1].Required, []string{"x1", "y1", "x2", "y2"}) {
		t.Errorf("the box is %+v, want null or an object of x1, y1, x2 and y2", props.Box.AnyOf)
	}
	if v, b := strings.Index(string(raw), `"verdict"`), strings.Index(string(raw), `"box"`); v > b {
		t.Error("the box comes before the verdict in the schema")
	}
	if !slices.Equal(judge.Schema["required"].([]string), []string{"verdict", "box"}) {
		t.Errorf("the schema requires %v", judge.Schema["required"])
	}
}

func TestRead(t *testing.T) {
	for _, c := range []struct {
		answer string
		want   *judge.Answer
	}{
		{`{"verdict": "correct", "box": null}`, &judge.Answer{Verdict: judge.Correct}},
		{`{"verdict": "error", "box": {"x1": 60, "y1": 180, "x2": 300, "y2": 220}}`, &judge.Answer{Verdict: judge.Error, Box: &judge.Box{X1: 60, Y1: 180, X2: 300, Y2: 220}}},
		{`{"verdict": "error", "box": null}`, &judge.Answer{Verdict: judge.Error}},
		// A box with another verdict is no place of a mistake.
		{`{"verdict": "unreadable", "box": {"x1": 1, "y1": 2, "x2": 3, "y2": 4}}`, &judge.Answer{Verdict: judge.Unreadable}},
		// Corners the other way round are put in order.
		{`{"verdict": "error", "box": {"x1": 300, "y1": 220, "x2": 60, "y2": 180}}`, &judge.Answer{Verdict: judge.Error, Box: &judge.Box{X1: 60, Y1: 180, X2: 300, Y2: 220}}},
	} {
		got, err := judge.Read(c.answer)
		if err != nil {
			t.Errorf("%s: %v", c.answer, err)
			continue
		}
		if got.Verdict != c.want.Verdict || (got.Box == nil) != (c.want.Box == nil) || got.Box != nil && *got.Box != *c.want.Box {
			t.Errorf("%s = %+v, want %+v", c.answer, got, c.want)
		}
	}
	for _, answer := range []string{
		`{"verdict": "so_far_correct", "box": null}`,
		`{"box": null}`,
		`{"verdict": "correct"}`,
		`{"verdict": "error", "mark": null, "box": null}`,
		`{"verdict": "error", "box": {"x1": 1, "y1": 2, "x2": 3}}`,
		`{"verdict": "error", "box": {"x1": 1, "y1": 2, "x2": 3, "y2": 4, "shape": "box"}}`,
		`not json`,
	} {
		if _, err := judge.Read(answer); err == nil {
			t.Errorf("%s is read", answer)
		}
	}
}
