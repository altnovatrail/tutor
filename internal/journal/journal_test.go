package journal_test

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/internal/journal"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/synth"
)

const header = "How the sheet was written, step by step. Lines are numbered from the top of the sheet down, erased writing included.\n"

// The journals of the synthetic sheets, which have no long pause: the order of their lines, and
// on the erasure-* sheets the erasures, each with the frame just before it.
func TestSynthetic(t *testing.T) {
	wrote := func(n int) string {
		var b strings.Builder
		for i := 1; i <= n; i++ {
			fmt.Fprintf(&b, "%d. Wrote line %d.\n", i, i)
		}
		return b.String()
	}
	for _, c := range []struct {
		scenario sample.Scenario
		text     string
		frames   []journal.Frame
		agree    journal.Agreement
	}{
		{sample.ScenarioCorrect, wrote(3), nil, journal.Agreement{Marked: 3, Same: 3}},
		{sample.ScenarioError, wrote(4), nil, journal.Agreement{Marked: 4, Same: 4}},
		{sample.ScenarioSimilarDigits, wrote(2), nil, journal.Agreement{Marked: 2, Same: 2}},
		{sample.ScenarioIllegible, wrote(3), nil, journal.Agreement{Marked: 3, Same: 3}},
		{sample.ScenarioAnswerNotLast, wrote(4), nil, journal.Agreement{Marked: 4, Same: 4}},
		{sample.ScenarioNearlyEmpty, wrote(1), nil, journal.Agreement{Marked: 1, Same: 1}},
		{sample.ScenarioMirroredDigits, wrote(2), nil, journal.Agreement{Marked: 2, Same: 2}},
		// The slip between lines 1 and 2 stands on line 1; the planned erasure takes all of line 2.
		{sample.ScenarioErasureFixed, "1. Wrote line 1.\n" +
			"2. Erased 1 stroke on line 1. Frame 1 shows the sheet just before this.\n" +
			"3. Wrote line 2.\n" +
			"4. Erased 9 strokes on line 2. Frame 2 shows the sheet just before this.\n" +
			"5. Wrote on line 2 again.\n" +
			"6. Wrote line 3.\n",
			[]journal.Frame{{Erasure: 0, Step: 1, Line: 1, Strokes: 1}, {Erasure: 1, Step: 3, Line: 2, Strokes: 9}}, journal.Agreement{Marked: 3, Same: 3}},
		{sample.ScenarioErasureBroken, "1. Wrote line 1.\n" +
			"2. Wrote line 2.\n" +
			"3. Erased 6 strokes on line 2. Frame 1 shows the sheet just before this.\n" +
			"4. Wrote on line 2 again.\n" +
			"5. Wrote line 3.\n" +
			"6. Wrote line 4.\n",
			[]journal.Frame{{Erasure: 0, Step: 2, Line: 2, Strokes: 6}}, journal.Agreement{Marked: 4, Same: 4}},
		// The inscription in the corner is a line of its own, which the markup does not hold.
		{sample.ScenarioInjection, wrote(4), nil, journal.Agreement{Marked: 3, Same: 3}},
	} {
		s := synth.Sample(c.scenario)
		j := journal.Of(s)
		if got := j.Text(true); got != header+c.text {
			t.Errorf("%s: the journal is\n%s\nwant\n%s%s", c.scenario, got, header, c.text)
		}
		if !slices.Equal(j.Frames, c.frames) {
			t.Errorf("%s: the frames are %+v, want %+v", c.scenario, j.Frames, c.frames)
		}
		if got := journal.Agree(s, j); got != c.agree {
			t.Errorf("%s: the agreement is %+v, want %+v", c.scenario, got, c.agree)
		}
		noTime(t, string(c.scenario), j)
	}
}

// noTime checks that the text of a journal names no moment and no point of the sheet: each
// number in it counts steps, lines or parts, strokes or frames, all of them small.
func noTime(t *testing.T, name string, j *journal.Journal) {
	t.Helper()
	most := max(len(j.Steps), len(j.Lines), len(j.Parts), journal.MaxFrames)
	for _, st := range j.Steps {
		for _, k := range st.Taken {
			most = max(most, k)
		}
	}
	for _, text := range []string{j.Text(false), j.Text(true)} {
		for _, n := range regexp.MustCompile(`\d+`).FindAllString(text, -1) {
			if v, _ := strconv.Atoi(n); v > most {
				t.Errorf("%s: the journal says %s, more than any count in it, %d", name, n, most)
			}
		}
	}
}

// sheet is a sample of strokes and erasures on the board's sheet, with the markup's lines.
type sheet struct {
	strokes  []sample.Stroke
	erasures []sample.Erasure
}

// stroke adds a short level stroke at height y, from the moment start to the moment end.
func (s *sheet) stroke(id string, y float64, start, end int64) {
	s.strokes = append(s.strokes, sample.Stroke{ID: id, Points: []sample.Point{
		{X: 100, Y: y, TMs: start, Pressure: 0.5}, {X: 140, Y: y + 4, TMs: end, Pressure: 0.5},
	}})
}

func (s *sheet) erase(t int64, ids ...string) {
	s.erasures = append(s.erasures, sample.Erasure{TMs: t, Strokes: ids})
}

func (s *sheet) sample(lines ...[]string) *sample.Sample {
	out := &sample.Sample{Sheet: sample.Sheet{Width: 1000, Height: 600}, Strokes: s.strokes, Erasures: s.erasures}
	for _, l := range lines {
		out.Lines = append(out.Lines, sample.Line{Strokes: l})
	}
	return out
}

func TestSteps(t *testing.T) {
	var s sheet
	s.stroke("a", 100, 9000, 9100)   // line 1, after a long wait before the first stroke
	s.stroke("b", 100, 9500, 9600)   // line 1 still
	s.stroke("c", 200, 15600, 15700) // line 2, after a pause just long
	s.stroke("d", 200, 21699, 21799) // a pause one millisecond short of long
	s.erase(23000, "d")
	s.erase(23500, "c")              // a second erasure of the same line: the same step
	s.erase(24000, "b")              // an erasure of another line: a step of its own
	s.stroke("e", 100, 24000, 24100) // begun at the erasure's moment, so after it; line 2 was written on last
	s.stroke("f", 200, 25000, 25100) // line 2, coming from line 1
	s.erase(32000, "e", "f")         // after a long pause, from two lines at once
	s.stroke("g", 300, 32000, 32100) // a new line, begun at the erasure's moment
	s.stroke("h", 300, 40000, 40100) // the same line after a long pause
	j := journal.Of(s.sample())
	want := header +
		"1. Wrote line 1.\n" +
		"2. Long pause.\n" +
		"3. Wrote line 2.\n" +
		"4. Erased 2 strokes on line 2. Frame 1 shows the sheet just before this.\n" +
		"5. Erased 1 stroke on line 1. Frame 2 shows the sheet just before this.\n" +
		"6. Went back to line 1 and wrote on it.\n" +
		"7. Went back to line 2 and wrote on it.\n" +
		"8. Long pause.\n" +
		"9. Erased 1 stroke on line 1 and 1 on line 2.\n" +
		"10. Wrote line 3.\n" +
		"11. Long pause.\n" +
		"12. Wrote on line 3 again.\n"
	if got := j.Text(true); got != want {
		t.Errorf("the journal is\n%s\nwant\n%s", got, want)
	}
	// Line 2 lost 3 strokes and line 1 lost 2: the frames are before the first erasure of each,
	// in the order of the writing, and the later erasure of both lines gives none.
	if wantFrames := []journal.Frame{{Erasure: 0, Step: 3, Line: 2, Strokes: 3}, {Erasure: 2, Step: 4, Line: 1, Strokes: 2}}; !slices.Equal(j.Frames, wantFrames) {
		t.Errorf("the frames are %+v, want %+v", j.Frames, wantFrames)
	}
	if got := j.Steps[3].Erasures; !slices.Equal(got, []int{0, 1}) {
		t.Errorf("step 4 tells the erasures %v, want [0 1]", got)
	}
	noTime(t, "the steps", j)
	// Without marks, the text is the same but for them.
	if got := j.Text(false); strings.Contains(got, "Frame") {
		t.Errorf("the journal without marks names a frame:\n%s", got)
	}
}

// A stroke an erasure takes away at its very moment came before it.
func TestTheErasuresMoment(t *testing.T) {
	var s sheet
	s.stroke("a", 100, 0, 100)
	s.stroke("b", 100, 500, 500) // a dot, taken away in the same millisecond
	s.erase(500, "b")
	want := header + "1. Wrote line 1.\n2. Erased 1 stroke on line 1. Frame 1 shows the sheet just before this.\n"
	if got := journal.Of(s.sample()).Text(true); got != want {
		t.Errorf("the journal is\n%s\nwant\n%s", got, want)
	}
}

// Of three lines with erasures, the frames are those of the two that lost the most strokes, of
// two that lost as many the one erased first; and an erasure that is the first of two lines gives
// one frame, so the next line gives the second.
func TestFrames(t *testing.T) {
	var s sheet
	s.stroke("a1", 100, 0, 10)
	s.stroke("a2", 100, 20, 30)
	s.stroke("a3", 100, 40, 50)
	s.stroke("b1", 200, 60, 70)
	s.stroke("b2", 200, 80, 90)
	s.stroke("b3", 200, 100, 110)
	s.stroke("c1", 300, 120, 130)
	s.stroke("c2", 300, 140, 150)
	s.stroke("c3", 300, 160, 170)
	s.erase(200, "c1", "c2")
	s.erase(300, "a1", "a2")
	s.erase(400, "b1", "b2", "b3")
	j := journal.Of(s.sample())
	if want := []journal.Frame{{Erasure: 0, Step: 3, Line: 3, Strokes: 2}, {Erasure: 2, Step: 5, Line: 2, Strokes: 3}}; !slices.Equal(j.Frames, want) {
		t.Errorf("the frames are %+v, want %+v", j.Frames, want)
	}

	var two sheet
	two.stroke("a", 100, 0, 10)
	two.stroke("b", 200, 20, 30)
	two.stroke("c", 300, 40, 50)
	two.erase(100, "a", "b")
	two.erase(200, "c")
	j = journal.Of(two.sample())
	if want := []journal.Frame{{Erasure: 0, Step: 3, Line: 1, Strokes: 1}, {Erasure: 1, Step: 4, Line: 3, Strokes: 1}}; !slices.Equal(j.Frames, want) {
		t.Errorf("an erasure of two lines: the frames are %+v, want %+v", j.Frames, want)
	}
	if j := journal.Of(synth.Sample(sample.ScenarioCorrect)); len(j.Frames) != 0 {
		t.Errorf("a sheet without erasures has the frames %+v", j.Frames)
	}
}

// A long journal tells its first MaxSteps steps and counts the rest, and with marks it names a
// frame among the steps it leaves out.
func TestLeftOut(t *testing.T) {
	// Strokes on lines 1 and 2 in turn: a step each.
	turns := func(n int) *sheet {
		var s sheet
		for i := range n {
			s.stroke(fmt.Sprintf("s%d", i), float64(100+100*(i%2)), int64(i*100), int64(i*100+50))
		}
		return &s
	}
	j := journal.Of(turns(40).sample())
	if len(j.Steps) != 40 {
		t.Fatalf("%d steps, want 40", len(j.Steps))
	}
	if text := j.Text(false); !strings.HasSuffix(text, "30. Went back to line 2 and wrote on it.\n10 more steps are left out.\n") {
		t.Errorf("the journal ends\n%s", text[len(text)-120:])
	}
	noTime(t, "the long journal", j)

	s := turns(30)
	s.erase(3500, "s29")
	j = journal.Of(s.sample())
	for marks, end := range map[bool]string{
		true:  "30. Went back to line 2 and wrote on it.\n1 more step is left out. Frame 1 shows the sheet just before it.\n",
		false: "30. Went back to line 2 and wrote on it.\n1 more step is left out.\n",
	} {
		if text := j.Text(marks); !strings.HasSuffix(text, end) {
			t.Errorf("with marks %v, the journal ends\n%s\nwant\n%s", marks, text[len(text)-120:], end)
		}
	}
}

// Two lines of the markup written at one height are one line of the journal: the agreement counts
// neither of them.
func TestAgreement(t *testing.T) {
	var s sheet
	s.stroke("a", 100, 0, 10)
	s.stroke("b", 104, 20, 30)
	s.stroke("c", 300, 60, 70)
	smp := s.sample([]string{"a"}, []string{"b"}, []string{"c"})
	j := journal.Of(smp)
	if len(j.Lines) != 2 {
		t.Fatalf("the journal finds %d lines, want 2: the first two lines of the markup stand at one height", len(j.Lines))
	}
	if got, want := journal.Agree(smp, j), (journal.Agreement{Marked: 3, Same: 1}); got != want {
		t.Errorf("the agreement is %+v, want %+v", got, want)
	}

	// A stroke no line of the markup holds, at the height of a line, does not count against it.
	var aside sheet
	aside.stroke("a", 100, 0, 10)
	aside.strokes = append(aside.strokes, sample.Stroke{ID: "note", Points: []sample.Point{
		{X: 800, Y: 98, TMs: 20, Pressure: 0.5}, {X: 900, Y: 108, TMs: 30, Pressure: 0.5},
	}})
	smp = aside.sample([]string{"a"})
	j = journal.Of(smp)
	if len(j.Lines) != 1 {
		t.Fatalf("the journal finds %d lines, want 1: the note stands on the line", len(j.Lines))
	}
	if got, want := journal.Agree(smp, j), (journal.Agreement{Marked: 1, Same: 1}); got != want {
		t.Errorf("with a note on the line, the agreement is %+v, want %+v", got, want)
	}
}
