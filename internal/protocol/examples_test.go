package protocol_test

import (
	"bytes"
	"cmp"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"testing"

	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"
	"google.golang.org/protobuf/proto"

	"github.com/altnovatrail/tutor/internal/lines"
	"github.com/altnovatrail/tutor/internal/protocol"
	"github.com/altnovatrail/tutor/internal/protocol/protocoltest"
	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/sample"
)

// spec's examples keep the rules of section 12 by the code of tutor too: the groups of a sheet
// with a history are those lines.Groups finds among the strokes it shows, the table of a workbook's
// boxes is Table's, and every crop's numbers are render.CropOf's — the height drawn, the size, the
// margins, the scale, the line, where it lies on the sheet and the frames. The pixels are not
// compared: two implementations may draw a crop a level of grey apart.
func TestExamplesFollowTheRules(t *testing.T) {
	examples := protocoltest.Examples(t)
	names, err := fs.Glob(examples, "*.json")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, name := range names {
		if strings.HasPrefix(name, "report-") {
			continue
		}
		checked++
		t.Run(name, func(t *testing.T) {
			data, err := fs.ReadFile(examples, name)
			if err != nil {
				t.Fatal(err)
			}
			s, err := protocol.ReadSample(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			var shown []sample.Stroke
			final := s.GetGeometry().GetStrokes()
			if h := s.GetHistory(); h != nil {
				strokes, erasures, f := protocol.Ink(h)
				for k, st := range strokes {
					if _, ok := slices.BinarySearch(f.Visible, f.Written[k].GetId()); ok {
						shown = append(shown, st)
						final = append(final, &evalv1.FinalStroke{Id: f.Written[k].GetId(), Points: pointsOf(f.Written[k])})
					}
				}
				if sh := s.GetSheet(); sh != nil {
					groups := lines.Groups(strokes, erasures)
					slices.SortFunc(groups, func(a, b lines.StrokeGroup) int {
						return cmp.Or(cmp.Compare(len(a.Label), len(b.Label)), cmp.Compare(a.Label, b.Label))
					})
					if got := groupsOf(t, groups); !slices.EqualFunc(got, sh.GetGroups(), func(a, b *evalv1.Group) bool { return proto.Equal(a, b) }) {
						t.Errorf("groups %v, and the example's %v", got, sh.GetGroups())
					}
				}
			} else {
				shown = protocol.Final(final)
			}

			if sh := s.GetSheet(); sh != nil {
				cropsAre(t, "the sheet", shown, groupsFrom(sh.GetGroups()), sh.GetCrops())
				return
			}
			wb := s.GetWorkbook()
			table := protocol.Table(final, wb.GetBoxes())
			if len(table) != len(wb.GetFilledBoxes()) {
				t.Fatalf("%d boxes filled, and the example's %d", len(table), len(wb.GetFilledBoxes()))
			}
			for i, want := range wb.GetFilledBoxes() {
				if !proto.Equal(table[i].GetAnswerSlot(), want.GetAnswerSlot()) || !slices.Equal(table[i].GetStrokeIds(), want.GetStrokeIds()) {
					t.Errorf("filled box %d is %v, and the example's %v %v", i, table[i], want.GetAnswerSlot(), want.GetStrokeIds())
				}
				var in []sample.Stroke
				for _, st := range shown {
					if slices.Contains(want.GetStrokeIds(), id(t, st.ID)) {
						in = append(in, st)
					}
				}
				cropsAre(t, fmt.Sprintf("box %d", i), in, nil, want.GetCrops())
			}
		})
	}
	if checked != 5 {
		t.Errorf("%d sample files of spec's examples, want 5", checked)
	}
}

// cropsAre checks that every crop of a set has the numbers render.CropOf gives the strokes.
func cropsAre(t *testing.T, what string, strokes []sample.Stroke, groups []lines.StrokeGroup, set *evalv1.CropSet) {
	t.Helper()
	if len(set.GetCrops()) == 0 {
		t.Errorf("%s has no crops", what)
	}
	for _, want := range set.GetCrops() {
		got, err := render.CropOf(strokes, groups, int(want.GetDigitHeight()))
		if err != nil {
			t.Fatal(err)
		}
		sb := want.GetSheetBox()
		if got.Drawn != int(want.GetDrawnHeight()) || got.Width != int(want.GetWidth()) || got.Height != int(want.GetHeight()) ||
			got.Margins != [2]int{int(want.GetMarginTop()), int(want.GetMarginLeft())} ||
			got.Scale != want.GetScale() || got.Line != want.GetLineWidth() ||
			got.SheetBox != [4]float64{sb.GetYmin(), sb.GetXmin(), sb.GetYmax(), sb.GetXmax()} {
			t.Errorf("%s at %d px: %+v, and the example's %v", what, want.GetDigitHeight(), got, want)
		}
		var frames []render.Frame
		for _, f := range want.GetFrames() {
			b := f.GetBox()
			frames = append(frames, render.Frame{Label: f.GetLabel(), Box: [4]int64{int64(b.GetYmin()), int64(b.GetXmin()), int64(b.GetYmax()), int64(b.GetXmax())}})
		}
		if !slices.EqualFunc(got.Frames, frames, func(a, b render.Frame) bool { return a == b }) {
			t.Errorf("%s at %d px: frames %v, and the example's %v", what, want.GetDigitHeight(), got.Frames, frames)
		}
	}
}

// groupsOf are groups as the protocol writes them: their strokes' IDs ascending.
func groupsOf(t *testing.T, groups []lines.StrokeGroup) []*evalv1.Group {
	t.Helper()
	var out []*evalv1.Group
	for _, g := range groups {
		m := &evalv1.Group{Label: g.Label}
		for _, s := range g.Strokes {
			m.StrokeIds = append(m.StrokeIds, id(t, s))
		}
		slices.Sort(m.StrokeIds)
		out = append(out, m)
	}
	return out
}

// groupsFrom are a sample's groups as the renderer takes them.
func groupsFrom(groups []*evalv1.Group) []lines.StrokeGroup {
	var out []lines.StrokeGroup
	for _, g := range groups {
		sg := lines.StrokeGroup{Label: g.GetLabel()}
		for _, id := range g.GetStrokeIds() {
			sg.Strokes = append(sg.Strokes, protocol.StrokeID(id))
		}
		out = append(out, sg)
	}
	return out
}

// pointsOf are a written stroke's points as a final geometry holds them, without their times.
func pointsOf(st *evalv1.Stroke) []*evalv1.FinalPoint {
	out := make([]*evalv1.FinalPoint, len(st.GetPoints()))
	for i, p := range st.GetPoints() {
		out[i] = &evalv1.FinalPoint{X: p.GetX(), Y: p.GetY()}
	}
	return out
}

func id(t *testing.T, s string) int32 {
	t.Helper()
	n, err := protocol.StrokeNumber(s)
	if err != nil {
		t.Fatal(err)
	}
	return n
}
