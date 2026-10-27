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

// sheet is what the forms of version 1 show of a synthetic sheet, and its task.
func sheet(t *testing.T, sc sample.Scenario) (*sample.Task, *judge.Sheet) {
	t.Helper()
	s := synth.Sample(sc)
	bank := synth.Bank()
	i := slices.IndexFunc(bank.Tasks, func(task sample.Task) bool { return task.ID == s.Task })
	sh, err := judge.Prepare(s, judge.V1)
	if err != nil {
		t.Fatal(err)
	}
	return &bank.Tasks[i], sh
}

// goldenFree is the sheet of the plan of free writing whose requests are the golden prompts of
// version 2: stamps, erasure-fixed, with a scheme of segments marked □ and △, an erasure in it and
// erased writing (docs/free-sheets.md).
const goldenFree = 14

// freeSample is sheet n of the plan of free writing, drawn with seed 1, and its task.
func freeSample(t *testing.T, n int) (*sample.Task, *sample.Sample) {
	t.Helper()
	open := func(path string) *os.File {
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = f.Close() })
		return f
	}
	plan, err := sample.ReadPlan(open("../../eval/free-sheets.json"))
	if err != nil {
		t.Fatal(err)
	}
	bank, err := sample.ReadBank(open("../../eval/free-bank.json"))
	if err != nil {
		t.Fatal(err)
	}
	s := synth.Sheet(&plan.Sheets[n-1], 1)
	i := slices.IndexFunc(bank.Tasks, func(task sample.Task) bool { return task.ID == s.Task })
	return &bank.Tasks[i], s
}

// freeSheet is what the forms of version 2 show of sheet n of the plan of free writing, and its
// task.
func freeSheet(t *testing.T, n int) (*sample.Task, *judge.Sheet) {
	t.Helper()
	task, s := freeSample(t, n)
	sh, err := judge.Prepare(s, judge.V2)
	if err != nil {
		t.Fatal(err)
	}
	return task, sh
}

// versions are the versions of the prompt, in their order, with what their tests ask about: the
// directory of their golden prompts, and their sheet — the synthetic sheet erasure-fixed for
// version 1 and the golden sheet of free writing for version 2.
var versions = []struct {
	v     judge.Version
	dir   string
	sheet func(*testing.T) (*sample.Task, *judge.Sheet)
}{
	{judge.V1, "testdata", func(t *testing.T) (*sample.Task, *judge.Sheet) { return sheet(t, sample.ScenarioErasureFixed) }},
	{judge.V2, filepath.Join("testdata", "v2"), func(t *testing.T) (*sample.Task, *judge.Sheet) { return freeSheet(t, goldenFree) }},
}

// The tests know each version of the prompt.
func TestVersions(t *testing.T) {
	var vs []judge.Version
	for _, c := range versions {
		vs = append(vs, c.v)
	}
	if !slices.Equal(vs, judge.Versions) {
		t.Errorf("the tests know the versions %v, want %v", vs, judge.Versions)
	}
}

// Prepare draws the finished sheet at its size, the journal of its version and the frames the
// journal chose.
func TestPrepare(t *testing.T) {
	s := synth.Sample(sample.ScenarioErasureFixed)
	sh, err := judge.Prepare(s, judge.V1)
	if err != nil {
		t.Fatal(err)
	}
	j := journal.Of(s)
	if sh.Version != judge.V1 || sh.Width != 1000 || sh.Height != 600 || len(sh.Frames) != len(j.Frames) || len(sh.Frames) != 2 {
		t.Fatalf("the sheet is %d × %d with %d frames, want 1000 × 600 and the journal's 2", sh.Width, sh.Height, len(sh.Frames))
	}
	if sh.Journal.Text(true) != j.Text(true) {
		t.Errorf("the journal of version 1 is not the journal of lines:\n%s", sh.Journal.Text(true))
	}
	frames(t, s, sh, j)

	_, free := freeSample(t, goldenFree)
	sh, err = judge.Prepare(free, judge.V2)
	if err != nil {
		t.Fatal(err)
	}
	parts := journal.OfParts(free)
	if sh.Version != judge.V2 || sh.Journal.Text(true) != parts.Text(true) || len(sh.Frames) != 1 {
		t.Errorf("the sheet of version 2 has %d frames and the journal\n%s\nwant 1 and the journal of parts", len(sh.Frames), sh.Journal.Text(true))
	}
	frames(t, free, sh, parts)

	if _, err := judge.Prepare(s, 3); err == nil {
		t.Error("a sheet is prepared for a version 3 of the prompt")
	}
	if _, err := judge.Prepare(&sample.Sample{Sheet: sample.Sheet{Width: 5000, Height: 600}}, judge.V1); err == nil {
		t.Error("a sheet the renderer cannot draw is prepared")
	}
}

// frames checks that the frames of a sheet are those its journal chose.
func frames(t *testing.T, s *sample.Sample, sh *judge.Sheet, j *journal.Journal) {
	t.Helper()
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

// The golden prompts of the four forms are the files testdata/A.txt to testdata/D.txt for version
// 1, on the synthetic sheet, and testdata/v2/A.txt to testdata/v2/D.txt for version 2, on a sheet of
// free writing; the author reads them there.
func TestGolden(t *testing.T) {
	for _, c := range versions {
		task, sh := c.sheet(t)
		golden(t, c.dir, task, sh)
	}
}

func golden(t *testing.T, dir string, task *sample.Task, sh *judge.Sheet) {
	t.Helper()
	for _, f := range judge.Forms {
		r, err := f.Request(task, sh)
		if err != nil {
			t.Fatal(err)
		}
		got := written(r, sh)
		path := filepath.Join(dir, string(f)+".txt")
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

// In each version, the forms differ only in the block that tells their inputs, and in those
// inputs: the journal in the text of B and D, the frames after the picture in C and D.
func TestForms(t *testing.T) {
	for _, c := range versions {
		task, sh := c.sheet(t)
		forms(t, task, sh)
	}
}

func forms(t *testing.T, task *sample.Task, sh *judge.Sheet) {
	t.Helper()
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
		t.Errorf("version %d: the journal of form B or D is not as it should be:\n%s\n---\n%s", sh.Version, b.Text, d.Text)
	}
}

// Version 2 tells a free solution: drawings, writing at the side, sums in a column and signs for
// unknowns are part of it, the first mistake is first in the order of its steps, and the journal
// is of parts. Version 1 says none of it.
func TestFreeWriting(t *testing.T) {
	for _, phrase := range []string{
		"a number line with jumps, a row of things or a scheme of segments",
		"writing at the side",
		"sums worked in a column",
		"□ or △",
		"a mistake in a drawing is a mistake",
		"The reference solution tells its drawings in words.",
		"the writing or the drawing where the solution first goes wrong",
		"in the order of the solution's steps, wherever they stand on the sheet",
	} {
		for _, f := range judge.Forms {
			if !strings.Contains(f.Instructions(judge.V2), phrase) {
				t.Errorf("form %s of version 2 does not say %q", f, phrase)
			}
			if strings.Contains(f.Instructions(judge.V1), phrase) {
				t.Errorf("form %s of version 1 says %q", f, phrase)
			}
		}
	}
	for _, f := range []judge.Form{judge.FormB, judge.FormD} {
		if !strings.Contains(f.Instructions(judge.V2), "Erased writing in it is not on Image 1") {
			t.Errorf("the journal block of form %s of version 2 does not tell erased writing", f)
		}
	}
	task, sh := freeSheet(t, goldenFree)
	r, err := judge.FormB.Request(task, sh)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Text, "1. Wrote part 1 (") || strings.Contains(r.Text, "line 1") {
		t.Errorf("the journal of version 2 is not of parts:\n%s", r.Text)
	}
	for _, f := range judge.Forms {
		if got := f.Instructions(3); got != "" {
			t.Errorf("form %s of a version 3 of the prompt has the instructions %q", f, got)
		}
	}
}

func TestRequestRefusals(t *testing.T) {
	task, sh := sheet(t, sample.ScenarioErasureFixed)
	if _, err := judge.Form("E").Request(task, sh); err == nil {
		t.Error("a form E is asked")
	}
	// A sheet not drawn by Prepare has no version, and one whose journal is not of its version's
	// kind mixes two prompts.
	bare := *sh
	bare.Version = 0
	if _, err := judge.FormA.Request(task, &bare); err == nil {
		t.Error("a sheet without a version of the prompt is asked")
	}
	mixed := *sh
	mixed.Version = judge.V2
	for _, f := range judge.Forms {
		if _, err := f.Request(task, &mixed); err == nil {
			t.Errorf("form %s is asked in version 2 with a journal of lines", f)
		}
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

// The fingerprint of each version of the prompt is one SHA-256, the same at every call, and the
// two versions have two; a version that does not exist has none.
func TestFingerprint(t *testing.T) {
	for _, v := range judge.Versions {
		f := judge.Fingerprint(v)
		if len(f) != 64 || strings.Trim(f, "0123456789abcdef") != "" || judge.Fingerprint(v) != f {
			t.Errorf("the fingerprint of version %d is %q", v, f)
		}
	}
	if judge.Fingerprint(judge.V1) == judge.Fingerprint(judge.V2) {
		t.Error("the two versions of the prompt have one fingerprint")
	}
	if judge.Fingerprint(3) != "" {
		t.Error("a version 3 of the prompt has a fingerprint")
	}
}

// The box a judge that is always right gives round a line holds every point of the line, and the
// line is the one it hits.
func TestAround(t *testing.T) {
	s := synth.Sample(sample.ScenarioError)
	b := judge.Around(s, *s.Expected.FirstError)
	if b == nil || judge.Hit(s, *b) != *s.Expected.FirstError {
		t.Fatalf("the box round line %d is %+v, and it hits line %d", *s.Expected.FirstError, b, judge.Hit(s, *b))
	}
	if judge.Hit(s, judge.Box{X1: 900, Y1: 500, X2: 950, Y2: 550}) != 0 {
		t.Error("a box away from the writing hits a line")
	}
}

// The prompt is frozen after the dev rounds of T27, before the test part runs (R24): any change to
// a word a judge reads changes the fingerprint, and this test fails.
func TestFrozen(t *testing.T) {
	const frozen = "6a98139b3dbd6748f2552db5ec4ee91c72136724b0ffb7aebd461f6108e76b41"
	if got := judge.Fingerprint(judge.V1); got != frozen {
		t.Errorf("the prompt's fingerprint is %s, not the one frozen after T27, %s", got, frozen)
	}
}
