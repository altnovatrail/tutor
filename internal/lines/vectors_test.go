package lines_test

import (
	"cmp"
	"encoding/json"
	"io/fs"
	"slices"
	"testing"

	"github.com/altnovatrail/spec/contracts"
	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/altnovatrail/tutor/internal/lines"
	"github.com/altnovatrail/tutor/internal/protocol"
)

// groupVectors is spec's test vectors of the rule of groups, contracts.Vectors
// eval/v1/groups.json (contracts/README.md of spec, section 12; Д96 in spec's RUN.md): canvas's
// cases (R24 canvas) in the form of the protocol, with the groups of the strokes taken off. Their
// messages are read by protojson.
type groupVectors struct {
	Rule struct {
		Pause  int64 `json:"pause"`
		Across int64 `json:"across"`
		Down   int64 `json:"down"`
	} `json:"rule"`
	Cases []struct {
		Name       string            `json:"name"`
		History    json.RawMessage   `json:"history"`
		Neighbours [][2]int32        `json:"neighbours"`
		Groups     []json.RawMessage `json:"groups"`
		GroupBoxes []json.RawMessage `json:"group_boxes"`
		Erased     []struct {
			StrokeID int32    `json:"stroke_id"`
			Labels   []string `json:"labels"`
		} `json:"erased"`
	} `json:"cases"`
}

// On every case of spec's vectors, the history folds into its strokes, and the strokes the sheet
// shows have the vectors' pairs of neighbours and their groups: the labels, the IDs and the boxes
// in tenths, exactly. Each stroke taken off is in the groups of the strokes shown that would be its
// neighbours, in the order of their labels, as the vectors have it (R30, R67 in
// docs/decisions.md).
func TestGroupsMatchSpecVectors(t *testing.T) {
	data, err := fs.ReadFile(contracts.Vectors, "eval/v1/groups.json")
	if err != nil {
		t.Fatal(err)
	}
	var v groupVectors
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	if v.Rule.Pause != lines.GroupPause || v.Rule.Across != lines.GroupAcross || v.Rule.Down != lines.GroupDown {
		t.Fatalf("the vectors' rule is %+v, and the package's %d ms, %d × %d", v.Rule, lines.GroupPause, lines.GroupAcross, lines.GroupDown)
	}
	if len(v.Cases) != 16 {
		t.Errorf("%d cases, want spec's 16", len(v.Cases))
	}
	for _, c := range v.Cases {
		t.Run(c.Name, func(t *testing.T) {
			var h evalv1.History
			if err := protojson.Unmarshal(c.History, &h); err != nil {
				t.Fatal(err)
			}
			strokes, erasures, f := protocol.Ink(&h)
			hidden := f.Hidden

			var pairs [][2]int32
			for i, a := range strokes {
				for _, b := range strokes[i+1:] {
					x, y := id(t, a.ID), id(t, b.ID)
					if !slices.Contains(hidden, x) && !slices.Contains(hidden, y) && lines.Neighbours(a, b) {
						pairs = append(pairs, [2]int32{min(x, y), max(x, y)})
					}
				}
			}
			slices.SortFunc(pairs, func(a, b [2]int32) int { return cmp.Or(cmp.Compare(a[0], b[0]), cmp.Compare(a[1], b[1])) })
			if !slices.Equal(pairs, c.Neighbours) && (len(pairs) > 0 || len(c.Neighbours) > 0) {
				t.Errorf("neighbours = %v, want %v", pairs, c.Neighbours)
			}

			groups := lines.Groups(strokes, erasures)
			slices.SortFunc(groups, func(a, b lines.StrokeGroup) int {
				return cmp.Or(cmp.Compare(len(a.Label), len(b.Label)), cmp.Compare(a.Label, b.Label))
			})
			if len(groups) != len(c.Groups) {
				t.Fatalf("%d groups, want %d", len(groups), len(c.Groups))
			}
			erasedIn := map[int32][]string{}
			for n, g := range groups {
				got := &evalv1.Group{Label: g.Label}
				for _, s := range g.Strokes {
					got.StrokeIds = append(got.StrokeIds, id(t, s))
				}
				slices.Sort(got.StrokeIds)
				var want evalv1.Group
				if err := protojson.Unmarshal(c.Groups[n], &want); err != nil {
					t.Fatal(err)
				}
				if !proto.Equal(got, &want) {
					t.Errorf("group %d = %v, want %v", n, got, &want)
				}
				box := &evalv1.TenthsBox{Ymin: tenths(g.Top), Xmin: tenths(g.Left), Ymax: tenths(g.Bottom), Xmax: tenths(g.Right)}
				var wantBox evalv1.TenthsBox
				if err := protojson.Unmarshal(c.GroupBoxes[n], &wantBox); err != nil {
					t.Fatal(err)
				}
				if !proto.Equal(box, &wantBox) {
					t.Errorf("the box of %s = %v, want %v", g.Label, box, &wantBox)
				}
				for _, s := range g.Erased {
					erasedIn[id(t, s)] = append(erasedIn[id(t, s)], g.Label)
				}
			}
			if len(c.Erased) != len(hidden) {
				t.Errorf("%d strokes taken off, and the vectors' %d", len(hidden), len(c.Erased))
			}
			for _, e := range c.Erased {
				if got := erasedIn[e.StrokeID]; !slices.Equal(got, e.Labels) && (len(got) > 0 || len(e.Labels) > 0) {
					t.Errorf("stroke %d taken off is of %v, want %v", e.StrokeID, got, e.Labels)
				}
			}
		})
	}
}

func id(t *testing.T, s string) int32 {
	t.Helper()
	n, err := protocol.StrokeNumber(s)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func tenths(v float64) int32 { return int32(lines.Tenths(v)) }
