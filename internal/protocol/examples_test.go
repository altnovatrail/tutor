package protocol_test

import (
	"bytes"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"testing"

	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"
	"google.golang.org/protobuf/proto"

	"github.com/altnovatrail/tutor/internal/evalsample"
	"github.com/altnovatrail/tutor/internal/lines"
	"github.com/altnovatrail/tutor/internal/protocol"
	"github.com/altnovatrail/tutor/internal/protocol/protocoltest"
	"github.com/altnovatrail/tutor/internal/sample"
)

// spec's examples keep the rules of section 12 by the code of tutor too, the code that writes the
// samples tutor-eval draw draws (internal/evalsample): the groups of a sheet with a history are those
// lines.Groups finds among the strokes it shows, the table of a workbook's boxes is Table's, and
// every set of crops is what evalsample.CropSet gives — the box of the points, h_med, and each crop's
// height drawn, size, margins, scale, line, place on the sheet and frames. The pixels are not
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
					got, err := evalsample.Groups(strokes, erasures)
					if err != nil {
						t.Fatal(err)
					}
					if !slices.EqualFunc(got, sh.GetGroups(), func(a, b *evalv1.Group) bool { return proto.Equal(a, b) }) {
						t.Errorf("groups %v, and the example's %v", got, sh.GetGroups())
					}
				}
			} else {
				shown = protocol.Final(final)
			}

			if sh := s.GetSheet(); sh != nil {
				cropsAre(t, "the sheet", shown, evalsample.GroupsOf(sh.GetGroups()), sh.GetCrops())
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

// cropsAre checks that a set of crops is what evalsample.CropSet gives the strokes at its heights,
// but for the pixels of each PNG.
func cropsAre(t *testing.T, what string, strokes []sample.Stroke, groups []lines.StrokeGroup, set *evalv1.CropSet) {
	t.Helper()
	if len(set.GetCrops()) == 0 {
		t.Errorf("%s has no crops", what)
	}
	var heights []int
	for _, c := range set.GetCrops() {
		heights = append(heights, int(c.GetDigitHeight()))
	}
	got, err := evalsample.CropSet(strokes, groups, heights)
	if err != nil {
		t.Fatal(err)
	}
	want := proto.CloneOf(set)
	for _, c := range append(got.GetCrops(), want.GetCrops()...) {
		c.Png = nil
	}
	if !proto.Equal(got, want) {
		t.Errorf("%s: the crops %v, and the example's %v", what, got, want)
	}
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
