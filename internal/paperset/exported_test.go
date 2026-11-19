package paperset_test

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
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
// files' names: each case's kind follows from its truth, its pictures are those the Check sent —
// the task's crop as the library sends a photo, a JPEG —, and the stub, which is right by that
// truth, is right on each. A case with no outcome is refused.
func TestExportedCases(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	write := func(name string, c *v1.DebugCase) {
		t.Helper()
		c.Work = &v1.PaperWork{Statement: pngOf(t, 300, 80), Crop: pngOf(t, 320, 160)}
		data, err := protojson.Marshal(c)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("b.json", &v1.DebugCase{Key: number("28"), Read: "27", Outcome: v1.DebugOutcome_DEBUG_OUTCOME_NOT_YET})
	write("a.json", &v1.DebugCase{Formula: "37+48", Key: number("85"), Read: "85", Outcome: v1.DebugOutcome_DEBUG_OUTCOME_CORRECT,
		Note: "read as 35", Action: &v1.DebugAction{Kind: v1.DebugActionKind_DEBUG_ACTION_KIND_CHECK}})

	s, err := paperset.Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Cases) != 2 || s.Cases[0].Kind != paperset.KindExpression || s.Cases[0].Read != "85" ||
		s.Cases[1].Kind != paperset.KindWord || s.Cases[1].Outcome != paperset.OutcomeNotYet || s.Cases[1].Part != "dev" {
		t.Fatalf("the set = %+v", s.Cases)
	}
	digest, err := paperset.Digest(dir)
	if again, _ := paperset.Digest(dir); err != nil || len(digest) != 64 || again != digest {
		t.Errorf("the digest = %q, %v; again %q", digest, err, again)
	}
	f := &paperset.File{Name: dir, SHA256: digest, Set: s}
	for _, kind := range paperset.CallKinds {
		r, err := paperset.New(kind, "dev", f, haiku(t), 1)
		if err != nil {
			t.Fatal(err)
		}
		for _, st := range r.Steps {
			req, err := st.Call.Request()
			if err != nil || len(req.Images) != 1 {
				t.Fatalf("%s: the request %+v, %v", kind, req, err)
			}
			media, _ := req.Images[0].MediaType()
			if want := map[bool]string{true: "image/png", false: "image/jpeg"}[kind == paperset.CallPaper]; media != want {
				t.Errorf("%s: the picture is %s, want %s", kind, media, want)
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

	write("c.json", &v1.DebugCase{Key: number("1"), Read: "1"})
	if _, err := paperset.Read(dir); err == nil {
		t.Error("a case with no outcome was read")
	}
}
