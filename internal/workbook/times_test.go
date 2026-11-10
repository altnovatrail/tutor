package workbook_test

import (
	"context"
	"encoding/json"
	"io/fs"
	"testing"

	"github.com/altnovatrail/module-kit/observation"
	"github.com/altnovatrail/spec/contracts"
	observationv1 "github.com/altnovatrail/spec/contracts/novatrail/observation/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/workbook"
)

// timesVector is the vector of the observation of boxes checked by a time of day,
// observation/v1/times.json of spec's contracts (Д135 RUN.md spec): its items, each a line of boxes
// with their checks, and its cases, two readers' observations, the one they join into and the
// verdict on each item of that one.
type timesVector struct {
	Items []struct {
		Line []struct {
			Check *struct {
				Kind       sample.CheckKind   `json:"kind"`
				Key        string             `json:"key"`
				TrapValues []sample.TrapValue `json:"trap_values"`
			} `json:"check"`
		} `json:"line"`
	} `json:"items"`
	Cases []struct {
		Description  string            `json:"description"`
		Observations []json.RawMessage `json:"observations"`
		Joined       json.RawMessage   `json:"joined"`
		Verdicts     []struct {
			Verdict     string `json:"verdict"`
			AnswerSlots []struct {
				Verdict string `json:"verdict"`
				Reason  string `json:"reason"`
				Trap    string `json:"trap"`
			} `json:"answer_slots"`
		} `json:"verdicts"`
	} `json:"cases"`
}

// The boxes of a time, as a page's record tells them, are joined and judged by module-kit as the
// vector says (R111 spec): the report takes the items of a page by workbook.ItemsOf, so a box of a
// time reaches module-kit with its kind, its key and its trap values.
func TestTimesVector(t *testing.T) {
	data, err := fs.ReadFile(contracts.Vectors, "observation/v1/times.json")
	if err != nil {
		t.Fatal(err)
	}
	var v timesVector
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	pr := &workbook.PageRecord{}
	for i, it := range v.Items {
		for k, p := range it.Line {
			if p.Check != nil {
				pr.Boxes = append(pr.Boxes, workbook.BoxRecord{Item: i + 1, Slot: k, Kind: p.Check.Kind, Key: p.Check.Key, TrapValues: p.Check.TrapValues})
			}
		}
	}
	if len(pr.Boxes) == 0 || len(v.Cases) == 0 {
		t.Fatalf("the vector holds %d boxes and %d cases", len(pr.Boxes), len(v.Cases))
	}
	items := workbook.ItemsOf(pr)
	read := func(raw json.RawMessage) *observationv1.Observation {
		o := &observationv1.Observation{}
		if err := protojson.Unmarshal(raw, o); err != nil {
			t.Fatal(err)
		}
		return o
	}
	for _, c := range v.Cases {
		first, second := read(c.Observations[0]), read(c.Observations[1])
		joined, err := observation.Join(items, first, second)
		if err != nil {
			t.Fatalf("%s: %v", c.Description, err)
		}
		if want := read(c.Joined); !proto.Equal(joined, want) {
			t.Errorf("%s: joined into %v, want %v", c.Description, joined, want)
		}
		got, err := observation.Verdicts(context.Background(), items, joined, nil)
		if err != nil || len(got) != len(c.Verdicts) {
			t.Fatalf("%s: %d verdicts, %v; want %d", c.Description, len(got), err, len(c.Verdicts))
		}
		for i, want := range c.Verdicts {
			if string(got[i].Verdict) != want.Verdict || len(got[i].Slots) != len(want.AnswerSlots) {
				t.Errorf("%s, item %d: %+v, want %+v", c.Description, i+1, got[i], want)
				continue
			}
			for k, s := range got[i].Slots {
				w := want.AnswerSlots[k]
				trap := ""
				if s.Trap != nil {
					trap = string(s.Trap.Trap)
				}
				if string(s.Verdict) != w.Verdict || string(s.Reason) != w.Reason || trap != w.Trap {
					t.Errorf("%s, item %d, box %d: %+v, want %+v", c.Description, i+1, k+1, s, w)
				}
			}
		}
	}
}
