package paperset_test

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"testing"

	v1 "github.com/altnovatrail/spec/contracts/novatrail/api/v1"
	paperv1 "github.com/altnovatrail/spec/contracts/novatrail/paper/v1"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/altnovatrail/tutor/internal/paperset"
)

func pngOf(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewGray(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func number(v string) *paperv1.Answer {
	return &paperv1.Answer{Kind: &paperv1.Answer_Number{Number: &paperv1.Number{Value: v}}}
}

// A folder of the cases the debug mode exported is a set of the part dev, read in the order of the
// files' names: each case's kind follows from its truth — of more than one task, several; of a
// Check, which has no key, exported —, its pictures are those the Check sent — the task's crop and
// the instruction's as the library sends a photo, a JPEG, and the work as the PNG it is —, and the
// stub, which is right by that truth, is right on each. The finding, judged by the lines code
// draws, runs on none of them. A case with no outcome or with no task is refused.
func TestExportedCases(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, c *v1.DebugCase) {
		t.Helper()
		instruction := c.GetWork().GetInstruction()
		c.Work = &v1.PaperWork{Statement: pngOf(t, 300, 80), Crop: pngOf(t, 320, 160), Instruction: instruction}
		data, err := protojson.Marshal(c)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	task := func(t *v1.DebugTask) []*v1.DebugTask { return []*v1.DebugTask{t} }
	write("b.json", &v1.DebugCase{Tasks: task(&v1.DebugTask{Statement: "Sam has 14 stamps and gets 14 more.", Key: number("28"), Read: "27", Outcome: v1.DebugOutcome_DEBUG_OUTCOME_NOT_YET})})
	write("a.json", &v1.DebugCase{Tasks: task(&v1.DebugTask{Statement: "37 + 48 =", Formula: "37+48", Key: number("85"), Read: "85", Outcome: v1.DebugOutcome_DEBUG_OUTCOME_CORRECT}),
		Note: "read as 35", Action: &v1.DebugAction{Kind: v1.DebugActionKind_DEBUG_ACTION_KIND_CHECK}})
	write("ab.json", &v1.DebugCase{Tasks: []*v1.DebugTask{
		{Statement: "□ + 5 = 12", Formula: "x+5=12", Key: number("7"), Read: "7", Outcome: v1.DebugOutcome_DEBUG_OUTCOME_CORRECT},
		{Statement: "□ + 3 = 10", Formula: "x+3=10", Key: number("7"), Outcome: v1.DebugOutcome_DEBUG_OUTCOME_UNREADABLE},
		{Statement: "Draw a line.", Outcome: v1.DebugOutcome_DEBUG_OUTCOME_UNCHECKED},
	}})
	write("c.json", &v1.DebugCase{Work: &v1.PaperWork{Instruction: pngOf(t, 300, 30)}, Tasks: []*v1.DebugTask{
		{Read: "12", Outcome: v1.DebugOutcome_DEBUG_OUTCOME_CORRECT, CheckVerdict: "correct", CheckRead: "12"},
		{Outcome: v1.DebugOutcome_DEBUG_OUTCOME_SO_FAR_CORRECT, CheckVerdict: "incorrect"},
		{Outcome: v1.DebugOutcome_DEBUG_OUTCOME_UNCHECKED, CheckVerdict: "no_answer"},
	}})
	write("d.json", &v1.DebugCase{Tasks: task(&v1.DebugTask{Statement: "Underline the noun.", Read: "cat", Outcome: v1.DebugOutcome_DEBUG_OUTCOME_NOT_YET})})

	s, err := paperset.Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Cases) != 5 || s.Cases[3].Kind != paperset.KindSeveral || s.Cases[3].Tasks[1].Outcome != paperset.OutcomeSoFarCorrect || s.Cases[4].Kind != paperset.KindExported || s.Cases[0].Kind != paperset.KindExpression || s.Cases[0].Tasks[0].Read != "85" ||
		s.Cases[1].Kind != paperset.KindSeveral || len(s.Cases[1].Tasks) != 3 || s.Cases[1].Tasks[2].Key != nil || s.Cases[1].Tasks[1].Formula != "x+3=10" ||
		s.Cases[2].Kind != paperset.KindWord || s.Cases[2].Tasks[0].Outcome != paperset.OutcomeNotYet || s.Cases[2].Part != "dev" {
		t.Fatalf("the set = %+v", s.Cases)
	}
	digest, err := paperset.Digest(dir)
	if again, _ := paperset.Digest(dir); err != nil || len(digest) != 64 || again != digest {
		t.Errorf("the digest = %q, %v; again %q", digest, err, again)
	}
	f := &paperset.File{Name: dir, SHA256: digest, Set: s}
	instructions := 0
	for _, kind := range paperset.CallKinds {
		r, err := paperset.New(kind, "dev", f, haiku(t), 1)
		if kind == paperset.CallFinding {
			if err == nil {
				t.Errorf("the finding runs on exported cases: %d steps", len(r.Steps))
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		for _, st := range r.Steps {
			req, err := st.Call.Request()
			want := []string{"image/jpeg"}
			if kind == paperset.CallCheck {
				want = append(want, "image/png")
				if len(req.Images) == 3 {
					want = append(want, "image/jpeg")
					instructions++
				}
			}
			if err != nil || len(req.Images) != len(want) {
				t.Fatalf("%s: the request %+v, %v", kind, req, err)
			}
			for i, w := range want {
				if media, _ := req.Images[i].MediaType(); media != w {
					t.Errorf("%s: picture %d is %s, want %s", kind, i+1, media, w)
				}
			}

			right, err := st.Call.Right()
			if err != nil {
				t.Fatal(err)
			}
			if rec := recordOf(st, right); !rec.Right || rec.Error != "" {
				t.Errorf("%s: the stub's %s is %+v", kind, right, rec)
			}
		}
	}

	if instructions != 1 {
		t.Errorf("%d Checks sent the instruction, not 1", instructions)
	}

	// Beside a case of several tasks that code draws, the finding asks that one alone.
	drawn := setOf(t, 30).Set
	for _, c := range drawn.Cases {
		if c.Kind == paperset.KindSeveral {
			c.Number, c.Part = len(s.Cases)+1, "dev"
			mixed := &paperset.File{Name: dir, SHA256: digest, Set: &paperset.Set{Seed: drawn.Seed, Cases: append(slices.Clone(s.Cases), c)}}
			r, err := paperset.New(paperset.CallFinding, "dev", mixed, haiku(t), 1)
			if err != nil || len(r.Items) != 1 || r.Items[0].(paperset.CaseRecord).Case.Number != c.Number {
				t.Fatalf("the finding beside exported cases: %v, %v", r, err)
			}
			break
		}
	}

	for name, c := range map[string]*v1.DebugCase{
		"no outcome": {Tasks: task(&v1.DebugTask{Statement: "1 =", Key: number("1"), Read: "1"})},
		"no task":    {},
	} {
		write("e.json", c)
		if _, err := paperset.Read(dir); err == nil {
			t.Errorf("a case with %s was read", name)
		}
	}
}
