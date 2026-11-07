package protocol_test

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"slices"
	"testing"

	"github.com/altnovatrail/spec/contracts"
	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"
	observationv1 "github.com/altnovatrail/spec/contracts/novatrail/observation/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/altnovatrail/tutor/internal/protocol"
	"github.com/altnovatrail/tutor/internal/sample"
)

// vectorFile is a file of spec's test vectors of section 12, contracts.Vectors eval/v1/<rule>.json:
// the rule's numbers, and its cases, each a JSON object whose messages are read by protojson.
type vectorFile struct {
	Rule  map[string]json.RawMessage   `json:"rule"`
	Cases []map[string]json.RawMessage `json:"cases"`
}

func readVectors(t *testing.T, name string) vectorFile {
	t.Helper()
	data, err := fs.ReadFile(contracts.Vectors, "eval/v1/"+name)
	if err != nil {
		t.Fatal(err)
	}
	var v vectorFile
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// message reads a value of a case as a message of the protocol.
func message[M proto.Message](t *testing.T, raw json.RawMessage, m M) M {
	t.Helper()
	if err := protojson.Unmarshal(raw, m); err != nil {
		t.Fatal(err)
	}
	return m
}

// list reads a list of a case, each value as a message made by m.
func list[M proto.Message](t *testing.T, raw json.RawMessage, m func() M) []M {
	t.Helper()
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatal(err)
	}
	out := make([]M, len(items))
	for i, item := range items {
		out[i] = message(t, item, m())
	}
	return out
}

func name(t *testing.T, c map[string]json.RawMessage) string {
	t.Helper()
	var n string
	if err := json.Unmarshal(c["name"], &n); err != nil {
		t.Fatal(err)
	}
	return n
}

func ruleIs(t *testing.T, v vectorFile, want map[string]any) {
	t.Helper()
	for k, w := range want {
		data, _ := json.Marshal(w)
		var given bytes.Buffer
		if err := json.Compact(&given, v.Rule[k]); err != nil || given.String() != string(data) {
			t.Errorf("the vectors' %s is %s, and the package's %s", k, v.Rule[k], data)
		}
	}
	if len(v.Rule) != len(want) {
		t.Errorf("the vectors' rule has %d numbers, and the package %d", len(v.Rule), len(want))
	}
}

// On every case of spec's vectors of boxes, each stroke fills the box the vectors name, or none,
// and the table "box → stroke IDs" is theirs (section 12, "The boxes of a workbook").
func TestBoxesVectors(t *testing.T) {
	v := readVectors(t, "boxes.json")
	ruleIs(t, v, map[string]any{"margin": protocol.BoxMargin, "limit": protocol.BoxLimit})
	if len(v.Cases) != 20 {
		t.Errorf("%d cases, want spec's 20", len(v.Cases))
	}
	for _, c := range v.Cases {
		t.Run(name(t, c), func(t *testing.T) {
			boxes := list(t, c["boxes"], func() *evalv1.PageBox { return &evalv1.PageBox{} })
			strokes := list(t, c["strokes"], func() *evalv1.FinalStroke { return &evalv1.FinalStroke{} })
			var verdicts []struct {
				StrokeID   int32           `json:"stroke_id"`
				AnswerSlot json.RawMessage `json:"answer_slot"`
			}
			if err := json.Unmarshal(c["verdicts"], &verdicts); err != nil {
				t.Fatal(err)
			}
			byID := map[int32]*evalv1.FinalStroke{}
			for _, st := range strokes {
				byID[st.GetId()] = st
			}
			for _, want := range verdicts {
				i := protocol.BoxOf(byID[want.StrokeID].GetPoints(), boxes)
				if string(want.AnswerSlot) == "null" {
					if i >= 0 {
						t.Errorf("stroke %d fills box %d, and the vectors' is the draft", want.StrokeID, i)
					}
					continue
				}
				place := message(t, want.AnswerSlot, &observationv1.AnswerSlotPlace{})
				if i < 0 || !proto.Equal(boxes[i].GetAnswerSlot(), place) {
					t.Errorf("stroke %d fills box %d, and the vectors' is %v", want.StrokeID, i, place)
				}
			}
			want := list(t, c["filled_boxes"], func() *evalv1.FilledBox { return &evalv1.FilledBox{} })
			got := protocol.Table(strokes, boxes)
			if !slices.EqualFunc(got, want, func(a, b *evalv1.FilledBox) bool { return proto.Equal(a, b) }) {
				t.Errorf("the table is %v, and the vectors' %v", got, want)
			}
		})
	}
}

// On every case of spec's vectors of snapping, each box of a model on the crop at its height takes
// the strokes the vectors name, and their box, or none (section 12, "Snapping").
func TestSnapsVectors(t *testing.T) {
	v := readVectors(t, "snaps.json")
	ruleIs(t, v, map[string]any{"margin": protocol.SnapMargin, "share": []int{protocol.SnapAbove, protocol.SnapBelow}})
	if len(v.Cases) != 14 {
		t.Errorf("%d cases, want spec's 14", len(v.Cases))
	}
	for _, c := range v.Cases {
		t.Run(name(t, c), func(t *testing.T) {
			strokes := list(t, c["strokes"], func() *evalv1.FinalStroke { return &evalv1.FinalStroke{} })
			set := message(t, c["crops"], &evalv1.CropSet{})
			var snaps []struct {
				DigitHeight int32           `json:"digit_height"`
				Box         json.RawMessage `json:"box"`
				StrokeIDs   []int32         `json:"stroke_ids"`
				Snapped     json.RawMessage `json:"snapped"`
			}
			if err := json.Unmarshal(c["snaps"], &snaps); err != nil {
				t.Fatal(err)
			}
			if len(snaps) == 0 {
				t.Fatal("no snaps")
			}
			for _, s := range snaps {
				i := slices.IndexFunc(set.GetCrops(), func(cr *evalv1.Crop) bool { return cr.GetDigitHeight() == s.DigitHeight })
				if i < 0 {
					t.Fatalf("no crop at %d px", s.DigitHeight)
				}
				box := message(t, s.Box, &observationv1.CropBox{})
				ids, snapped := protocol.Snap(set, set.GetCrops()[i], box, strokes)
				if !slices.Equal(ids, s.StrokeIDs) && (len(ids) > 0 || len(s.StrokeIDs) > 0) {
					t.Errorf("at %d px, %v takes %v, and the vectors' %v", s.DigitHeight, box, ids, s.StrokeIDs)
				}
				if string(s.Snapped) == "null" {
					if snapped != nil {
						t.Errorf("at %d px, %v snaps to %v, and the vectors' to none", s.DigitHeight, box, snapped)
					}
				} else if want := message(t, s.Snapped, &evalv1.TenthsBox{}); !proto.Equal(snapped, want) {
					t.Errorf("at %d px, %v snaps to %v, and the vectors' to %v", s.DigitHeight, box, snapped, want)
				}
			}
		})
	}
}

// On every case of spec's vectors of groups, the history folds into the strokes the vectors name
// visible, and the others written are hidden (section 12, "The history of writing").
func TestFoldVectors(t *testing.T) {
	v := readVectors(t, "groups.json")
	for _, c := range v.Cases {
		t.Run(name(t, c), func(t *testing.T) {
			f := protocol.Fold(message(t, c["history"], &evalv1.History{}))
			var visible []int32
			var erased []struct {
				StrokeID int32 `json:"stroke_id"`
			}
			if err := json.Unmarshal(c["visible"], &visible); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(c["erased"], &erased); err != nil {
				t.Fatal(err)
			}
			hidden := []int32{}
			for _, e := range erased {
				hidden = append(hidden, e.StrokeID)
			}
			if !slices.Equal(f.Visible, visible) || !slices.Equal(append([]int32{}, f.Hidden...), hidden) {
				t.Errorf("visible %v and hidden %v, want %v and %v", f.Visible, f.Hidden, visible, hidden)
			}
			if len(f.Written) != len(visible)+len(hidden) {
				t.Errorf("%d strokes written, and %d visible and hidden", len(f.Written), len(visible)+len(hidden))
			}
		})
	}
}

// Ink gives every stroke written and an erasure at the event that took a hidden stroke off for the
// last time: an erasure put back by undo is none, an undo of a write is one, and a clear is one.
func TestInk(t *testing.T) {
	v := readVectors(t, "groups.json")
	want := map[string][]sample.Erasure{
		"undone":       nil,
		"undone-write": {{TMs: 1200, Strokes: []string{"2"}}},
		"cleared":      {{TMs: 1200, Strokes: []string{"1", "2"}}},
	}
	seen := 0
	for _, c := range v.Cases {
		n := name(t, c)
		w, ok := want[n]
		if !ok {
			continue
		}
		seen++
		h := message(t, c["history"], &evalv1.History{})
		strokes, erasures, f := protocol.Ink(h)
		if len(strokes) != len(f.Written) {
			t.Errorf("%s: %d strokes, and %d written", n, len(strokes), len(f.Written))
		}
		if !slices.EqualFunc(erasures, w, func(a, b sample.Erasure) bool {
			return a.TMs == b.TMs && slices.Equal(a.Strokes, b.Strokes) && a.Planned == nil
		}) {
			t.Errorf("%s: erasures %+v, want %+v", n, erasures, w)
		}
	}
	if seen != len(want) {
		t.Errorf("%d of the cases %v are in the vectors", seen, want)
	}
}
