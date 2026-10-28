package lines

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strconv"
	"testing"

	"github.com/altnovatrail/tutor/internal/sample"
)

// vectors is testdata/groups.json: canvas's test vectors of its rule of groups (R24 canvas), which
// the eval protocol takes as they are (Д36 in spec's RUN.md). The file is a copy of
// vectors/groups.json of altnovatrail/canvas at commit 82ac1d8, unchanged since 6e268f8, under
// Apache-2.0 as canvas is. The protocol's own vectors come in spec's releases (T35 in spec's
// RUN.md), and the test moves to them then (R30 in docs/decisions.md).
type vectors struct {
	Rule struct {
		Pause  float64 `json:"pause"`
		Across float64 `json:"across"`
		Down   float64 `json:"down"`
	} `json:"rule"`
	Cases []struct {
		Name    string `json:"name"`
		History []struct {
			Kind   string `json:"kind"`
			Stroke *struct {
				ID     int `json:"id"`
				Points []struct {
					X float64 `json:"x"`
					Y float64 `json:"y"`
					T float64 `json:"t"`
					P float64 `json:"p"`
				} `json:"points"`
			} `json:"stroke"`
		} `json:"history"`
		Visible    []int    `json:"visible"`
		Neighbours [][2]int `json:"neighbours"`
		Groups     []struct {
			Label string     `json:"label"`
			IDs   []int      `json:"ids"`
			Rect  [4]float64 `json:"rect"`
		} `json:"groups"`
	} `json:"cases"`
}

// On every case of canvas's vectors, the strokes the sheet shows have the vectors' pairs of
// neighbours and their groups: the labels, the IDs and the rectangles, exactly. The strokes of a
// case are those of its writes that it names visible, in the order written: a sample holds no
// undo, so the history needs no replay here. Times in fractions of a millisecond round half up,
// as places do.
func TestGroupsMatchCanvasVectors(t *testing.T) {
	data, err := os.ReadFile("testdata/groups.json")
	if err != nil {
		t.Fatal(err)
	}
	var v vectors
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	if v.Rule.Pause != GroupPause || v.Rule.Across != GroupAcross || v.Rule.Down != GroupDown {
		t.Fatalf("the vectors' rule is %+v, and the package's %d ms, %d × %d", v.Rule, GroupPause, GroupAcross, GroupDown)
	}
	if len(v.Cases) != 11 {
		t.Errorf("%d cases, want canvas's 11", len(v.Cases))
	}
	for _, c := range v.Cases {
		t.Run(c.Name, func(t *testing.T) {
			visible := map[int]bool{}
			for _, id := range c.Visible {
				visible[id] = true
			}
			var strokes []sample.Stroke
			for _, h := range c.History {
				if h.Kind != "write" || !visible[h.Stroke.ID] {
					continue
				}
				st := sample.Stroke{ID: strconv.Itoa(h.Stroke.ID)}
				for _, p := range h.Stroke.Points {
					st.Points = append(st.Points, sample.Point{X: p.X, Y: p.Y, TMs: roundHalfUp(p.T), Pressure: p.P})
				}
				strokes = append(strokes, st)
			}
			if len(strokes) != len(c.Visible) {
				t.Fatalf("%d strokes written of the %d visible", len(strokes), len(c.Visible))
			}

			var pairs [][2]int
			for i, a := range strokes {
				for _, b := range strokes[i+1:] {
					if Neighbours(a, b) {
						x, y := number(t, a.ID), number(t, b.ID)
						pairs = append(pairs, [2]int{min(x, y), max(x, y)})
					}
				}
			}
			slices.SortFunc(pairs, func(a, b [2]int) int { return cmpPair(a, b) })
			if want := c.Neighbours; (len(pairs) > 0 || len(want) > 0) && !reflect.DeepEqual(pairs, want) {
				t.Errorf("neighbours = %v, want %v", pairs, want)
			}

			got := map[string]string{}
			for _, g := range Groups(strokes, nil) {
				ids := make([]int, len(g.Strokes))
				for i, id := range g.Strokes {
					ids[i] = number(t, id)
				}
				slices.Sort(ids)
				got[g.Label] = fmt.Sprint(ids, [4]float64{g.Top, g.Left, g.Bottom, g.Right})
			}
			want := map[string]string{}
			for _, g := range c.Groups {
				want[g.Label] = fmt.Sprint(g.IDs, g.Rect)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("groups = %v, want %v", got, want)
			}
		})
	}
}

func number(t *testing.T, id string) int {
	t.Helper()
	n, err := strconv.Atoi(id)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func cmpPair(a, b [2]int) int {
	if a[0] != b[0] {
		return a[0] - b[0]
	}
	return a[1] - b[1]
}

// stroke is a stroke of two points, a straight line from (x0, y0) to (x1, y1), drawn from t0 to t1
// milliseconds.
func stroke(id string, x0, y0, x1, y1 float64, t0, t1 int64) sample.Stroke {
	return sample.Stroke{ID: id, Points: []sample.Point{{X: x0, Y: y0, TMs: t0, Pressure: 0.5}, {X: x1, Y: y1, TMs: t1, Pressure: 0.5}}}
}

// row writes n strokes in a row from x across, each w wide and 30 high at the top y, gap apart,
// starting at t0 and every step milliseconds; their IDs are the prefix and their place.
func row(prefix string, n int, x, y, w, gap float64, t0, step int64) []sample.Stroke {
	var out []sample.Stroke
	for i := range n {
		left := x + float64(i)*(w+gap)
		start := t0 + int64(i)*step
		out = append(out, stroke(fmt.Sprintf("%s%d", prefix, i+1), left, y, left+w, y+30, start, start+100))
	}
	return out
}

// strokesOf returns the strokes of each group.
func strokesOf(groups []StrokeGroup) [][]string {
	var out [][]string
	for _, g := range groups {
		out = append(out, g.Strokes)
	}
	return out
}

func joined(lists ...[]sample.Stroke) []sample.Stroke {
	var out []sample.Stroke
	for _, l := range lists {
		out = append(out, l...)
	}
	return out
}

func TestGroups(t *testing.T) {
	line := row("a", 3, 100, 100, 30, 20, 0, 400)
	// A column at the side: the first number, the sign and the second number under it, a bar and
	// the result under the bar, one after another without a pause to think.
	column := []sample.Stroke{
		stroke("top", 700, 100, 740, 130, 0, 300),
		stroke("sign", 690, 145, 700, 155, 500, 550),
		stroke("bottom", 710, 140, 740, 170, 700, 1000),
		stroke("bar", 680, 182, 745, 182, 1200, 1400),
	}
	result := func(t0 int64) sample.Stroke { return stroke("result", 700, 190, 740, 220, t0, t0+300) }
	// A number line: the axis, then divisions 45 apart from the left, each signed at once, and the
	// arc of a jump as soon as both its divisions are drawn.
	axis := stroke("axis", 100, 300, 820, 300, 0, 1500)
	var drawing []sample.Stroke
	clock := int64(1600)
	for k := range 4 {
		x := 100 + 45*float64(k)
		drawing = append(drawing, stroke(fmt.Sprintf("tick%d", k), x, 292, x, 308, clock, clock+60))
		drawing = append(drawing, stroke(fmt.Sprintf("label%d", k), x-6, 315, x+6, 335, clock+160, clock+360))
		clock += 460
	}
	arc := func(t0 int64) sample.Stroke { return stroke("arc", 100, 260, 235, 300, t0, t0+500) }

	tests := []struct {
		name    string
		strokes []sample.Stroke
		want    [][]string
	}{
		{"a note at the side at the height of a line, after a pause", joined(line, row("n", 2, 700, 100, 30, 20, 4000, 400)), [][]string{{"a1", "a2", "a3"}, {"n1", "n2"}}},
		{"a note at the side at once, far across", joined(line, row("n", 2, 700, 100, 30, 20, 1200, 400)), [][]string{{"a1", "a2", "a3"}, {"n1", "n2"}}},
		{"a column at the side in one go", append(slices.Clone(column), result(1600)), [][]string{{"top", "sign", "bottom", "bar", "result"}}},
		{"a column whose result comes after 2.5 s of thought", append(slices.Clone(column), result(3900)), [][]string{{"top", "sign", "bottom", "bar"}, {"result"}}},
		{"a number line with its divisions, labels and arc in one go", joined([]sample.Stroke{axis}, drawing, []sample.Stroke{arc(clock)}),
			[][]string{{"axis", "tick0", "label0", "tick1", "label1", "tick2", "label2", "tick3", "label3", "arc"}}},
		{"the arc drawn after a pause to think", joined([]sample.Stroke{axis}, drawing, []sample.Stroke{arc(clock + 3000)}),
			[][]string{{"axis", "tick0", "label0", "tick1", "label1", "tick2", "label2", "tick3", "label3"}, {"arc"}}},
		{"the answer under the drawing, after a pause", []sample.Stroke{axis, stroke("answer", 100, 345, 160, 375, 4000, 4500)}, [][]string{{"axis"}, {"answer"}}},
		{"the answer under the drawing at once", []sample.Stroke{axis, stroke("answer", 100, 345, 160, 375, 2000, 2500)}, [][]string{{"axis", "answer"}}},
		{"a stroke of one point", []sample.Stroke{line[0], {ID: "dot", Points: []sample.Point{{X: 150, Y: 120, TMs: 300}}}}, [][]string{{"a1", "dot"}}},
		{"a stroke without points", []sample.Stroke{{ID: "none"}, line[0]}, [][]string{{"a1"}}},
		{"no strokes", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := strokesOf(Groups(tt.strokes, nil)); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("groups = %q, want %q", got, tt.want)
			}
		})
	}
	// Lines by bands would take the line and the note at its height for one.
	if found := Group(joined(line, row("n", 2, 700, 100, 30, 20, 4000, 400)), Gap); len(found) != 1 {
		t.Errorf("Group finds %d lines, want the one band of the line and the note", len(found))
	}
}

// An erased stroke is of the groups of the strokes on the sheet that would be its neighbours: of
// both when they are in two groups, of none when there are none. It joins no groups and changes
// neither their rectangles nor their labels.
func TestGroupsOfErasedStrokes(t *testing.T) {
	strokes := []sample.Stroke{
		stroke("a", 100, 100, 150, 130, 0, 500),
		stroke("bridge", 160, 100, 290, 130, 1000, 1500),
		stroke("b", 300, 100, 350, 130, 2000, 2500),
		stroke("alone", 600, 500, 620, 520, 9000, 9100),
	}
	erasures := []sample.Erasure{{TMs: 9500, Strokes: []string{"bridge", "alone"}}}
	got := Groups(strokes, erasures)
	want := []StrokeGroup{
		{Label: "A", Strokes: []string{"a"}, Erased: []string{"bridge"}, Top: 100, Left: 100, Bottom: 130, Right: 150},
		{Label: "B", Strokes: []string{"b"}, Erased: []string{"bridge"}, Top: 100, Left: 300, Bottom: 130, Right: 350},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("groups = %+v, want %+v", got, want)
	}
}

// A fix written in place after a pause is a group of its own, and what it replaced is of the group
// it was written with: «75» and «66», then «66» erased and «65» written after 2.4 s.
func TestGroupsOfAFixAfterAPause(t *testing.T) {
	strokes := []sample.Stroke{
		stroke("75", 100, 100, 150, 130, 0, 500),
		stroke("66", 160, 100, 210, 130, 700, 1100),
		stroke("65", 160, 100, 210, 130, 3500, 3900),
	}
	got := Groups(strokes, []sample.Erasure{{TMs: 3000, Strokes: []string{"66"}}})
	if len(got) != 2 || !reflect.DeepEqual(got[0].Strokes, []string{"75"}) || !reflect.DeepEqual(got[0].Erased, []string{"66"}) ||
		!reflect.DeepEqual(got[1].Strokes, []string{"65"}) || got[1].Erased != nil {
		t.Errorf("groups = %+v, want «75» with the erased «66», then «65» alone", got)
	}
}

// Labels go by place in whole tenths, not by the raw numbers: tops of 100.01 and 100.04 are both
// 1000 tenths, so the group farther left is A. Rectangles alike go by the earlier start, and only
// then by the order of the strokes given.
func TestGroupLabels(t *testing.T) {
	right := stroke("right", 500, 100.01, 540, 130, 0, 100)
	left := stroke("left", 100, 100.04, 140, 130, 5000, 5100)
	if got := Groups([]sample.Stroke{right, left}, nil); got[0].Label != "B" || got[1].Label != "A" {
		t.Errorf("labels = %s, %s; want B for the group on the right, A on the left", got[0].Label, got[1].Label)
	}
	late := stroke("late", 100, 100, 140, 130, 9000, 9100)
	early := stroke("early", 100, 100, 140, 130, 0, 100)
	if got := Groups([]sample.Stroke{late, early}, nil); got[0].Strokes[0] != "late" || got[0].Label != "B" || got[1].Label != "A" {
		t.Errorf("groups = %+v; want the late one B, though given first", got)
	}
	for n, want := range map[int]string{0: "A", 25: "Z", 26: "AA", 51: "AZ", 52: "BA", 701: "ZZ", 702: "AAA"} {
		if got := labelOf(n); got != want {
			t.Errorf("labelOf(%d) = %s, want %s", n, got, want)
		}
	}
}

// Places round half up, as JavaScript's Math.round, for every number: 100.05 units are 1001
// tenths, and 0.049999999999999996 units, 0.49999999999999994 tenths, are 0.
func TestTenths(t *testing.T) {
	for v, want := range map[float64]int64{100.05: 1001, 50: 500, 50.02: 500, 50.06: 501, 0.049999999999999996: 0, 0.05: 1, -0.05: 0, -0.051: -1} {
		if got := Tenths(v); got != want {
			t.Errorf("Tenths(%v) = %d, want %d", v, got, want)
		}
	}
}

// ErasurePart finds the part of the markup an erasure stood on by the place of its strokes on the
// plane, without the rule's time.
func TestErasurePart(t *testing.T) {
	line := row("a", 3, 100, 100, 30, 20, 0, 400) // the main column, left, at the top
	column := []sample.Stroke{                    // a column at the side, at the same height
		stroke("top", 700, 100, 740, 130, 2000, 2300),
		stroke("sign", 690, 145, 700, 155, 2500, 2550),
		stroke("bottom", 710, 140, 740, 170, 2700, 3000),
		stroke("bar", 680, 182, 745, 182, 3200, 3400),
		stroke("result", 700, 190, 740, 220, 3600, 3900),
	}
	axis := stroke("axis", 100, 400, 820, 400, 5000, 6000)
	tick := stroke("tick", 235, 392, 235, 408, 6100, 6160)
	sheet := &sample.Sample{
		Strokes: joined(line, column, []sample.Stroke{axis, tick, stroke("stray", 900, 300, 905, 305, 7000, 7050)}),
		Lines: []sample.Line{
			{Strokes: []string{"a1", "a2", "a3"}},
			{Strokes: []string{"top", "sign", "bottom", "bar", "result"}},
			{Strokes: []string{"axis", "tick"}},
		},
	}
	erased := func(st ...sample.Stroke) (*sample.Sample, sample.Erasure) {
		s := *sheet
		s.Strokes = joined(sheet.Strokes, st)
		e := sample.Erasure{TMs: 99999}
		for _, x := range st {
			e.Strokes = append(e.Strokes, x.ID)
		}
		return &s, e
	}
	tests := []struct {
		name    string
		strokes []sample.Stroke
		want    int
	}{
		{"an arc of the drawing, above its axis", []sample.Stroke{stroke("arc", 100, 360, 235, 400, 8000, 8500)}, 3},
		{"the first number of the column, at the height of the main column's line", []sample.Stroke{stroke("old", 700, 100, 740, 130, 1000, 1300)}, 2},
		{"a line written again in its place", []sample.Stroke{stroke("x1", 100, 100, 130, 130, 50, 60), stroke("x2", 150, 100, 180, 130, 70, 80)}, 1},
		{"nothing near", []sample.Stroke{stroke("far", 400, 550, 420, 570, 8000, 8100)}, 0},
		{"as near two parts, the earlier", []sample.Stroke{stroke("both", 220, 120, 700, 125, 8000, 8100)}, 1},
		{"most strokes, on the later part", []sample.Stroke{stroke("c1", 700, 100, 740, 130, 8000, 8100), stroke("c2", 710, 140, 740, 170, 8200, 8300), stroke("l1", 100, 100, 130, 130, 8400, 8500)}, 2},
		{"a stroke of no part is not near", []sample.Stroke{stroke("by-stray", 880, 300, 895, 305, 8000, 8100)}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, e := erased(tt.strokes...)
			if got := ErasurePart(s, e); got != tt.want {
				t.Errorf("ErasurePart = %d, want %d", got, tt.want)
			}
		})
	}
	// By bands, the first number of the column stands as much on the main column's line, and the
	// higher of the two takes it.
	s, e := erased(stroke("old", 700, 100, 740, 130, 1000, 1300))
	if got := ErasureLine(s, e); got != 1 {
		t.Errorf("ErasureLine = %d, want 1, the line the bands give", got)
	}
}

// PartsTaken counts, for each erasure, the strokes it took from each part by the place on the plane,
// and names the part most of them stood on, the earlier of two that hold as many.
func TestPartsTaken(t *testing.T) {
	line := row("a", 3, 100, 100, 30, 20, 0, 400)
	axis := stroke("axis", 100, 400, 820, 400, 5000, 6000)
	s := &sample.Sample{
		Strokes: joined(line, []sample.Stroke{axis,
			stroke("arc", 100, 360, 235, 400, 8000, 8500), stroke("arc2", 300, 360, 435, 400, 8600, 9000),
			stroke("x1", 100, 100, 130, 130, 9100, 9200), stroke("far", 400, 550, 420, 570, 9300, 9400)}),
		Lines: []sample.Line{{Strokes: []string{"a1", "a2", "a3"}}, {Strokes: []string{"axis"}}},
	}
	got := PartsTaken(s, []sample.Erasure{
		{TMs: 9500, Strokes: []string{"arc", "arc2", "x1"}},
		{TMs: 9600, Strokes: []string{"x1", "arc"}},
		{TMs: 9700, Strokes: []string{"far", "gone"}},
	})
	want := []Taken{{From: map[int]int{1: 1, 2: 2}, Line: 2}, {From: map[int]int{1: 1, 2: 1}, Line: 1}, {From: map[int]int{}, Line: 0}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("PartsTaken = %+v, want %+v", got, want)
	}
	for i, e := range []sample.Erasure{{Strokes: []string{"arc", "arc2", "x1"}}, {Strokes: []string{"x1", "arc"}}} {
		if n := ErasurePart(s, e); n != want[i].Line {
			t.Errorf("ErasurePart of erasure %d = %d, want PartsTaken's %d", i, n, want[i].Line)
		}
	}
}
