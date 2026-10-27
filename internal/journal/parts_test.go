package journal_test

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/internal/journal"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/synth"
)

var update = flag.Bool("update", false, "write the golden journals of parts anew")

const partsHeader = "How the sheet was written, step by step. A part is a group of strokes written close together in place and time. Parts are numbered in the order they were first written; a part's first step says where on the sheet it stands. Erased writing, numbered apart, is erased strokes that no part holds.\n"

// at adds a short level stroke 40 units long from the point x, y, from the moment start to the
// moment end.
func (s *sheet) at(id string, x, y float64, start, end int64) {
	s.strokes = append(s.strokes, sample.Stroke{ID: id, Points: []sample.Point{
		{X: x, Y: y, TMs: start, Pressure: 0.5}, {X: x + 40, Y: y + 4, TMs: end, Pressure: 0.5},
	}})
}

// The steps of a journal of parts: a part per group, named by its place on its first step, a
// return to a part, the long pauses, an erasure from a part and writing on it again after one.
func TestPartsSteps(t *testing.T) {
	var s sheet
	s.at("a", 100, 100, 0, 100)
	s.at("b", 150, 100, 300, 400)     // part 1 still: 10 units and 200 ms from a
	s.at("c", 800, 100, 1000, 1100)   // part 2, far across the sheet
	s.at("d", 200, 100, 1500, 1600)   // back on part 1: 10 units and 1.1 s from b
	s.at("e", 250, 100, 9000, 9100)   // part 3: as near d, but 7.4 s later, after a long pause
	s.at("f", 300, 104, 9500, 9600)   // part 3 still, and erased below
	s.at("g", 500, 500, 16000, 16100) // part 4, in the middle of the bottom, after a long pause
	s.erase(16500, "f")               // from part 3, which it would be a neighbour of
	s.at("h", 545, 500, 17000, 17100) // part 4 again, 0.9 s after g
	j := journal.OfParts(s.sample())
	want := partsHeader +
		"1. Wrote part 1 (top left).\n" +
		"2. Wrote part 2 (top right).\n" +
		"3. Went back to part 1 and wrote on it.\n" +
		"4. Long pause.\n" +
		"5. Wrote part 3 (top left).\n" +
		"6. Long pause.\n" +
		"7. Wrote part 4 (bottom centre).\n" +
		"8. Erased 1 stroke of part 3. Frame 1 shows the sheet just before this.\n" +
		"9. Wrote on part 4 again.\n"
	if got := j.Text(true); got != want {
		t.Errorf("the journal is\n%s\nwant\n%s", got, want)
	}
	if want := []journal.Frame{{Erasure: 0, Step: 7, Line: 3, Strokes: 1}}; !slices.Equal(j.Frames, want) {
		t.Errorf("the frames are %+v, want %+v", j.Frames, want)
	}
	if got := strokesOf(j); !slices.Equal(got, []string{"a b d", "c", "e", "g h"}) {
		t.Errorf("the parts hold %q, want a b d, c, e and g h", got)
	}
	if p := j.Parts[2]; p.Erased || !slices.Equal(p.Group.Erased, []string{"f"}) {
		t.Errorf("part 3 is %+v, want a part with the erased stroke f", p)
	}
	if len(j.Lines) != 0 {
		t.Errorf("a journal of parts has the lines %+v", j.Lines)
	}
	noTime(t, "the steps of parts", j)
	ownWords(t, "the steps of parts", j)
}

// Writing erased whole is erased writing, numbered apart from the parts: written again after a
// pause, it has no neighbours on the sheet and is of no group (R30).
func TestPartsErasedWriting(t *testing.T) {
	var s sheet
	s.at("a", 100, 100, 0, 100)
	s.at("w1", 100, 300, 1000, 1100) // a line, erased whole below
	s.at("w2", 150, 300, 1300, 1400)
	s.erase(2000, "w1", "w2")
	s.at("r", 100, 300, 9000, 9100)   // the line written again in its place, after a long pause
	s.at("x", 800, 500, 10000, 10100) // a slip far off, erased at once
	s.erase(10500, "x")
	j := journal.OfParts(s.sample())
	want := partsHeader +
		"1. Wrote part 1 (top left).\n" +
		"2. Wrote erased writing 1 (middle left).\n" +
		"3. Erased 2 strokes of erased writing 1. Frame 1 shows the sheet just before this.\n" +
		"4. Long pause.\n" +
		"5. Wrote part 2 (middle left).\n" +
		"6. Wrote erased writing 2 (bottom right).\n" +
		"7. Erased 1 stroke of erased writing 2. Frame 2 shows the sheet just before this.\n"
	if got := j.Text(true); got != want {
		t.Errorf("the journal is\n%s\nwant\n%s", got, want)
	}
	if want := []journal.Frame{{Erasure: 0, Step: 2, Line: 2, Strokes: 2}, {Erasure: 1, Step: 6, Line: 4, Strokes: 1}}; !slices.Equal(j.Frames, want) {
		t.Errorf("the frames are %+v, want %+v", j.Frames, want)
	}
	var erased []bool
	for _, p := range j.Parts {
		erased = append(erased, p.Erased)
	}
	if !slices.Equal(erased, []bool{false, true, false, true}) || j.Parts[3].Number != 2 || j.Parts[2].Number != 2 {
		t.Errorf("the parts are %+v, want part 1, erased writing 1, part 2 and erased writing 2", j.Parts)
	}
	noTime(t, "erased writing", j)
	ownWords(t, "erased writing", j)

	// A line erased whole just after a stroke beside it: its first stroke would be a neighbour of
	// that stroke, so it stands on its part, and the rest of the line is erased writing.
	var beside sheet
	beside.at("a", 100, 100, 0, 100)
	beside.at("w1", 150, 100, 500, 600) // 10 units and 0.4 s from a
	beside.at("w2", 195, 100, 700, 800) // 55 units from a, 5 from w1
	beside.erase(1500, "w1", "w2")
	want = partsHeader +
		"1. Wrote part 1 (top left).\n" +
		"2. Wrote erased writing 1 (top left).\n" +
		"3. Erased 1 stroke of part 1 and 1 of erased writing 1.\n"
	if got := journal.OfParts(beside.sample()).Text(false); got != want {
		t.Errorf("a line erased beside a part: the journal is\n%s\nwant\n%s", got, want)
	}
}

// An erased stroke that would be a neighbour of two groups stands on the one written on last if
// it is one of them, or else on the earlier of the two; an erasure from two parts names both.
func TestPartsTwoGroups(t *testing.T) {
	bridge := func(s *sheet, at int64) {
		s.strokes = append(s.strokes, sample.Stroke{ID: "m", Points: []sample.Point{
			{X: 150, Y: 100, TMs: at, Pressure: 0.5}, {X: 290, Y: 104, TMs: at + 100, Pressure: 0.5},
		}})
	}
	for _, c := range []struct {
		name string
		far  bool // a third part is written between the second and the bridge
		want string
	}{
		{"after the second part", false, "1. Wrote part 1 (top left).\n" +
			"2. Wrote part 2 (top left).\n" +
			"3. Erased 1 stroke of part 2.\n"},
		{"after another part", true, "1. Wrote part 1 (top left).\n" +
			"2. Wrote part 2 (top left).\n" +
			"3. Wrote part 3 (bottom right).\n" +
			"4. Went back to part 1 and wrote on it.\n" +
			"5. Erased 1 stroke of part 1.\n"},
	} {
		var s sheet
		s.at("p", 100, 100, 0, 100)
		s.at("q", 300, 100, 500, 600) // 160 units from p: a group of its own
		if c.far {
			s.at("r", 800, 500, 700, 800)
		}
		bridge(&s, 1000) // within 10 units and 2 s of both p and q
		s.erase(1500, "m")
		if got, want := journal.OfParts(s.sample()).Text(false), partsHeader+c.want; got != want {
			t.Errorf("%s: the journal is\n%s\nwant\n%s", c.name, got, want)
		}
	}

	var s sheet
	s.at("p", 100, 100, 0, 100)
	s.at("q", 100, 300, 500, 600)
	s.at("p2", 150, 100, 700, 800) // on part 1, with the same erasure
	s.at("q2", 150, 300, 900, 1000)
	s.erase(1500, "q2", "p2")
	want := partsHeader +
		"1. Wrote part 1 (top left).\n" +
		"2. Wrote part 2 (middle left).\n" +
		"3. Went back to part 1 and wrote on it.\n" +
		"4. Went back to part 2 and wrote on it.\n" +
		"5. Erased 1 stroke of part 1 and 1 of part 2.\n"
	if got := journal.OfParts(s.sample()).Text(false); got != want {
		t.Errorf("an erasure from two parts: the journal is\n%s\nwant\n%s", got, want)
	}
}

// The place of a part is the third of the sheet's height and of its width that the middle of its
// rectangle stands in; a middle on a border between two thirds stands in the later.
func TestPartsPlaces(t *testing.T) {
	dot := func(x, y float64) string {
		s := &sample.Sample{Sheet: sample.Sheet{Width: 1000, Height: 600}, Strokes: []sample.Stroke{
			{ID: "d", Points: []sample.Point{{X: x, Y: y, TMs: 0, Pressure: 0.5}}},
		}}
		return journal.OfParts(s).Parts[0].Place
	}
	for _, c := range []struct {
		x, y float64
		want string
	}{
		{100, 100, "top left"}, {500, 100, "top centre"}, {900, 100, "top right"},
		{100, 300, "middle left"}, {500, 300, "centre"}, {900, 300, "middle right"},
		{100, 500, "bottom left"}, {500, 500, "bottom centre"}, {900, 500, "bottom right"},
		{333, 199.9, "top left"}, {334, 200, "centre"}, {666, 399.9, "centre"}, {667, 400, "bottom right"},
	} {
		if got := dot(c.x, c.y); got != c.want {
			t.Errorf("a dot at %v, %v stands at %q, want %q", c.x, c.y, got, c.want)
		}
	}
	// A rectangle's middle, not its corner, gives the place: the axis of a number line across the
	// top of the sheet is in the middle of its width.
	var s sheet
	s.strokes = append(s.strokes, sample.Stroke{ID: "axis", Points: []sample.Point{
		{X: 60, Y: 120, TMs: 0, Pressure: 0.5}, {X: 950, Y: 120, TMs: 400, Pressure: 0.5},
	}})
	if got := journal.OfParts(s.sample()).Parts[0].Place; got != "top centre" {
		t.Errorf("the axis stands at %q, want top centre", got)
	}
}

// A long journal of parts tells its first MaxSteps steps and counts the rest, in its own words.
func TestPartsLeftOut(t *testing.T) {
	var s sheet
	for i := range 40 { // two parts far apart, written in turn, each stroke 2 s or less from the last on its part
		s.at(fmt.Sprintf("s%d", i), float64(100+700*(i%2)), 100, int64(i*100), int64(i*100+50))
	}
	s.erase(4500, "s39")
	j := journal.OfParts(s.sample())
	if len(j.Steps) != 41 || len(j.Parts) != 2 {
		t.Fatalf("%d steps on %d parts, want 41 on 2", len(j.Steps), len(j.Parts))
	}
	end := "30. Went back to part 2 and wrote on it.\n11 more steps are left out. Frame 1 shows the sheet just before one of them.\n"
	if text := j.Text(true); !strings.HasSuffix(text, end) {
		t.Errorf("the journal ends\n%s\nwant\n%s", text[len(text)-150:], end)
	}
	noTime(t, "the long journal of parts", j)
	ownWords(t, "the long journal of parts", j)
}

// Of a journal of parts, the agreement counts the parts of the markup the journal found with just
// their strokes: a part of the markup that is two groups is not found.
func TestPartsAgreement(t *testing.T) {
	var s sheet
	s.at("a", 100, 100, 0, 100)
	s.at("b", 150, 100, 300, 400)
	s.at("c", 100, 300, 1000, 1100)
	s.at("d", 150, 300, 5000, 5100) // the end of the markup's second part, after a pause: a group of its own
	smp := s.sample([]string{"a", "b"}, []string{"c", "d"})
	if got, want := journal.Agree(smp, journal.OfParts(smp)), (journal.Agreement{Marked: 2, Same: 1}); got != want {
		t.Errorf("the agreement is %+v, want %+v", got, want)
	}
}

// tokensPerPiece is how many input tokens a piece of a journal's text took in the measurement of
// E2: a word, a number, a sign or a line break. On the 30 sheets of its test, the input of form B
// grew over that of form A by 111 tokens and 1.575 a piece of the journal, the same for each judge
// (architecture 3.1, «Журнал хода и кадры»). The input tokens of form B are counted again by T66.
const tokensPerPiece = 1.575

var piece = regexp.MustCompile(`[A-Za-z]+|[0-9]+|\n|[^ A-Za-z0-9\n]`)

// The journals of parts of the sheets of code (T59, T60): the golden texts of seed 1, and on seeds
// 1, 2 and 77, no coordinates and about 300 tokens at most.
func TestPartsCodeSheets(t *testing.T) {
	f, err := os.Open("../../eval/free-sheets.json")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	plan, err := sample.ReadPlan(f)
	if err != nil {
		t.Fatal(err)
	}
	var golden strings.Builder
	golden.WriteString("# The journals of parts of the sheets of eval/free-sheets.json, drawn with seed 1, with the marks\n" +
		"# of the frames (internal/journal, TestPartsCodeSheets). After each sheet's number, task and\n" +
		"# scenario: how many parts of its markup the journal finds with just their strokes.\n")
	most := 0
	for _, seed := range []uint64{1, 2, 77} {
		for i, sh := range plan.Sheets {
			s := synth.Sheet(&sh, seed)
			j := journal.OfParts(s)
			name := fmt.Sprintf("sheet %d of seed %d", i+1, seed)
			noTime(t, name, j)
			ownWords(t, name, j)
			text := j.Text(true)
			tokens := int(tokensPerPiece*float64(len(piece.FindAllString(text, -1))) + 0.5)
			most = max(most, tokens)
			if tokens > 300 {
				t.Errorf("%s: the journal is about %d tokens, more than 300:\n%s", name, tokens, text)
			}
			if seed == 1 {
				a := journal.Agree(s, j)
				fmt.Fprintf(&golden, "\n## %02d %s %s: %d of %d parts\n%s", i+1, sh.Task, sh.Scenario, a.Same, a.Marked, text)
			}
		}
	}
	t.Logf("the journals of parts are about %d tokens at most", most)
	const path = "testdata/parts.txt"
	if *update {
		if err := os.WriteFile(path, []byte(golden.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if golden.String() != string(want) {
		t.Errorf("the journals of parts are not those of %s; with -update the test writes them anew", path)
	}
}

// ownWords checks that each word of a journal of parts' text is among its words, journal.PartsWords,
// which the fingerprint of a prompt holds: a phrase told and not listed there would change what a
// judge reads and leave the fingerprint as it was.
func ownWords(t *testing.T, name string, j *journal.Journal) {
	t.Helper()
	known := map[string]bool{}
	letters := regexp.MustCompile(`[A-Za-z]+`)
	for _, w := range journal.PartsWords() {
		for _, word := range letters.FindAllString(w, -1) {
			known[word] = true
		}
	}
	for _, text := range []string{j.Text(false), j.Text(true)} {
		for _, word := range letters.FindAllString(text, -1) {
			if !known[word] {
				t.Errorf("%s: the journal says %q, which is not among journal.PartsWords", name, word)
			}
		}
	}
}

// strokesOf lists the strokes each part of a journal holds on the sheet.
func strokesOf(j *journal.Journal) []string {
	var out []string
	for _, p := range j.Parts {
		out = append(out, strings.Join(p.Group.Strokes, " "))
	}
	return out
}
