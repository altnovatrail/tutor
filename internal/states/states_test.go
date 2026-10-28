package states_test

import (
	"bytes"
	"math"
	"os"
	"reflect"
	"slices"
	"testing"

	"github.com/altnovatrail/tutor/internal/journal"
	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/states"
	"github.com/altnovatrail/tutor/internal/synth"
)

// stroke is an upright stroke 30 units high at (x, y), written from t0 to t1 ms.
func stroke(id string, x, y float64, t0, t1 int64) sample.Stroke {
	return sample.Stroke{ID: id, Points: []sample.Point{{X: x, Y: y, TMs: t0, Pressure: 0.5}, {X: x, Y: y + 30, TMs: t1, Pressure: 0.5}}}
}

// sheet is a sample of the strokes, the erasures and the lines, each line its text and strokes.
func sheet(strokes []sample.Stroke, erasures []sample.Erasure, lines ...sample.Line) *sample.Sample {
	return &sample.Sample{Sheet: sample.Sheet{Width: 1000, Height: 600}, Strokes: strokes, Erasures: erasures, Lines: lines}
}

// ids are the IDs of the strokes a state shows.
func ids(st states.State) []string {
	var out []string
	for _, s := range st.Shown {
		out = append(out, s.ID)
	}
	return out
}

// A line of two strokes with a pause of 0.6 s between them, and below it the answer line of three
// with pauses of 0.45 and 0.3 s: a state at the pause of each line, and one after the first line,
// where the writer goes on to write on the next. The pauses a policy reads at are every pause of
// 0.4, 2 and 6 s or more between one event and the next.
func TestOf(t *testing.T) {
	s := sheet([]sample.Stroke{
		stroke("a", 100, 100, 1000, 1200), stroke("b", 120, 100, 1800, 2000),
		stroke("c", 100, 300, 4500, 4700), stroke("d", 120, 300, 5150, 5300), stroke("e", 140, 300, 5600, 5800),
	}, nil,
		sample.Line{Text: "1 1", Strokes: []string{"a", "b"}},
		sample.Line{Text: "1 1 1", Strokes: []string{"c", "d", "e"}, Answer: true},
	)
	got := states.Of(s, false)
	want := []states.State{
		{Event: 1, TMs: 1200, Pause: 1, PauseMs: 600, Parts: []states.Progress{states.Begun, states.None}},
		{Event: 2, TMs: 2000, Line: 1, Parts: []states.Progress{states.Done, states.None}},
		{Event: 3, TMs: 4700, Pause: 2, PauseMs: 450, Parts: []states.Progress{states.Done, states.Begun}},
	}
	for i := range got {
		got[i].Shown = nil
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("states %+v, want %+v", got, want)
	}
	if p := states.PoliciesOf(s, states.Of(s, false)); p != (states.Policies{AfterLine: 1, Pause400: 3, Pause2000: 1}) {
		t.Errorf("policies %+v", p)
	}
	if got := ids(states.Of(s, false)[2]); !slices.Equal(got, []string{"a", "b", "c"}) {
		t.Errorf("the pause in the answer line shows %v", got)
	}
}

// Before a run of erasures with no stroke between them, one state, which shows the strokes they
// take away and counts them as of no part; an erasure goes before a stroke begun at its very
// moment, unless it takes that stroke away, as the journal and the frames take it.
func TestOfErasures(t *testing.T) {
	s := sheet([]sample.Stroke{
		stroke("a", 100, 100, 1000, 1100), stroke("x", 120, 100, 1300, 1400), stroke("y", 140, 100, 2000, 2100),
		stroke("b", 120, 100, 3000, 3100), stroke("z", 160, 100, 4000, 4100), stroke("c", 160, 100, 5000, 5100),
	}, []sample.Erasure{
		{TMs: 2500, Strokes: []string{"x"}}, {TMs: 2600, Strokes: []string{"y"}},
		{TMs: 5000, Strokes: []string{"z"}},
	}, sample.Line{Text: "1 1 1", Strokes: []string{"a", "b", "c"}, Answer: true})
	got := states.Of(s, false)
	if len(got) != 2 {
		t.Fatalf("%d states, want 2: %+v", len(got), got)
	}
	if st := got[0]; st.Erasure != 1 || st.Event != 3 || st.Erased != 2 || !slices.Equal(ids(st), []string{"a", "x", "y"}) {
		t.Errorf("before the first run: %+v, showing %v", st, ids(st))
	}
	// z is taken away at 5000, the moment c begins: the erasure goes first.
	if st := got[1]; st.Erasure != 3 || st.Erased != 1 || !slices.Equal(ids(st), []string{"a", "b", "z"}) {
		t.Errorf("before the second: %+v, showing %v", st, ids(st))
	}
	for i, st := range got {
		f, err := render.Before(s, st.Erasure-1)
		if err != nil {
			t.Fatal(err)
		}
		g, err := render.Final(sheet(st.Shown, nil))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(f.Pix, g.Pix) {
			t.Errorf("state %d is not the frame before its erasure", i)
		}
	}
	// A line finished, a slip erased, and the next line begun: the state before the erasure is after
	// the line too.
	s = sheet([]sample.Stroke{stroke("a", 100, 100, 1000, 1100), stroke("x", 120, 100, 1300, 1400), stroke("b", 100, 400, 3000, 3100)},
		[]sample.Erasure{{TMs: 2000, Strokes: []string{"x"}}},
		sample.Line{Text: "1", Strokes: []string{"a"}}, sample.Line{Text: "1", Strokes: []string{"b"}, Answer: true})
	if got := states.Of(s, false); len(got) != 1 || got[0].Erasure != 1 || got[0].Line != 1 || states.PoliciesOf(s, got).AfterLine != 1 {
		t.Errorf("a line finished before an erasure: %+v", got)
	}
	// An erasure that takes away a stroke begun at its own moment goes after it.
	s = sheet([]sample.Stroke{stroke("a", 100, 100, 1000, 1100), stroke("x", 120, 100, 2000, 2000)},
		[]sample.Erasure{{TMs: 2000, Strokes: []string{"x"}}}, sample.Line{Text: "1", Strokes: []string{"a"}, Answer: true})
	if got := states.Of(s, false); len(got) != 1 || got[0].Erasure != 1 || !slices.Equal(ids(got[0]), []string{"a", "x"}) {
		t.Errorf("an erasure of a stroke of its moment: %+v", got)
	}
}

// No pause of 0.4 s or more, no state at a pause; a line crossed out or written illegibly is no
// other line to pause in, and of two other lines the one whose pause is longest is taken; a line of
// one stroke has no pause. A state chosen for two reasons — a line of the journal ends where the
// writer paused within a line of the markup — is one state with both.
func TestOfPauses(t *testing.T) {
	s := sheet([]sample.Stroke{
		stroke("a", 100, 100, 1000, 1100), stroke("b", 120, 100, 1300, 1400),
		stroke("c", 100, 300, 3000, 3100), stroke("d", 120, 300, 4000, 4100),
		stroke("e", 100, 500, 6000, 6100), stroke("f", 120, 500, 6800, 6900),
		stroke("g", 100, 400, 8000, 8100), stroke("h", 120, 400, 8500, 8600),
		stroke("i", 300, 400, 9000, 9100),
	}, nil,
		sample.Line{Text: "1 1", Strokes: []string{"a", "b"}, Answer: true},
		sample.Line{Text: "1 1", Strokes: []string{"c", "d"}, CrossedOut: true},
		sample.Line{Text: "1 1", Strokes: []string{"e", "f"}, Illegible: true},
		sample.Line{Text: "1 1", Strokes: []string{"g", "h"}},
		sample.Line{Text: "1", Strokes: []string{"i"}},
	)
	var pauses []int
	for _, st := range states.Of(s, false) {
		if st.Pause > 0 {
			pauses = append(pauses, st.Pause)
		}
	}
	if !slices.Equal(pauses, []int{4}) {
		t.Errorf("pauses in the lines %v, want only in line 4: none of 0.4 s in the answer line", pauses)
	}
	// One line of the markup over two lines of the journal, with a pause where it goes down.
	s = sheet([]sample.Stroke{stroke("a", 100, 100, 1000, 1100), stroke("b", 100, 400, 1700, 1800), stroke("c", 120, 400, 2000, 2100)}, nil,
		sample.Line{Text: "1 1 1", Strokes: []string{"a", "b", "c"}, Answer: true})
	got := states.Of(s, false)
	if len(got) != 1 || got[0].Line != 1 || got[0].Pause != 1 || got[0].PauseMs != 600 {
		t.Errorf("a line's end and a pause at one event: %+v", got)
	}
}

// plans are the sheets of both plans the measurement draws, drawn with the seed of their sets,
// each with whether it is of parts.
func plans(t *testing.T) (out []*sample.Sample, parts []bool) {
	t.Helper()
	for _, path := range []string{"../../eval/sheets.json", "../../eval/free-sheets.json"} {
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		plan, err := sample.ReadPlan(f)
		_ = f.Close()
		if err != nil {
			t.Fatal(err)
		}
		for i := range plan.Sheets {
			sh := &plan.Sheets[i]
			out = append(out, synth.Sheet(sh, 1))
			parts = append(parts, len(sh.Parts) > 0)
		}
	}
	return out, parts
}

// On every sheet of both plans: no state is the finished sheet; a state before an erasure is the
// frame render.Before draws; the policy of pauses of 6 s reads as often as the journal tells a long
// pause; and the sheets of E2 and of free writing have the states the measurement expects of them.
func TestOfPlans(t *testing.T) {
	sheets, parts := plans(t)
	count := 0
	for n, s := range sheets {
		final := render.Shown(s, math.MaxInt64)
		got := states.Of(s, parts[n])
		count += len(got)
		for _, st := range got {
			if reflect.DeepEqual(ids(st), ids(states.State{Shown: final})) {
				t.Errorf("sheet %d: the state after event %d is the finished sheet", n+1, st.Event)
			}
			if st.Erasure > 0 {
				f, err := render.Before(s, st.Erasure-1)
				if err != nil {
					t.Fatal(err)
				}
				g, err := render.Final(sheet(st.Shown, nil))
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(f.Pix, g.Pix) {
					t.Errorf("sheet %d: the state before erasure %d is not its frame", n+1, st.Erasure)
				}
			}
		}
		j := journal.Of(s)
		if parts[n] {
			j = journal.OfParts(s)
		}
		long := 0
		for _, step := range j.Steps {
			if step.Kind == journal.Paused {
				long++
			}
		}
		if p := states.PoliciesOf(s, got); p.Pause6000 != long {
			t.Errorf("sheet %d: %d pauses of 6 s, and the journal tells %d", n+1, p.Pause6000, long)
		}
	}
	t.Logf("%d states on %d sheets", count, len(sheets))
}
