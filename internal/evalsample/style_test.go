package evalsample_test

import (
	"bytes"
	"image"
	"image/png"
	"io/fs"
	"math"
	"slices"
	"testing"

	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"

	"github.com/altnovatrail/tutor/internal/evalsample"
	"github.com/altnovatrail/tutor/internal/protocol"
	"github.com/altnovatrail/tutor/internal/protocol/protocoltest"
	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/sample"
)

// ReadSheet reads spec's sheet in styles — «123» in liner and in pen and an underline in marker,
// which an eraser that takes a part cut short —: each stroke with the style it was written in, the
// piece of the cut among them, the cut an erasure of the underline that is no slip, and the crops of
// the strokes it shows drawn in colour, an RGB PNG.
func TestReadStyledSheet(t *testing.T) {
	trees := tree(t)
	data, err := fs.ReadFile(protocoltest.Examples(t), "sheet-style.json")
	if err != nil {
		t.Fatal(err)
	}
	es, err := protocol.ReadSample(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	bankPath := es.GetTask().GetTutor().GetBank()
	es.Task.Source = &evalv1.TaskReference_File{File: &evalv1.TaskFileSource{Bank: bankPath}}
	released(t, es, trees)
	rb, _, _, err := evalsample.NewReleases(trees).Of(ref(tag, bankPath))
	if err != nil {
		t.Fatal(err)
	}
	sh, _, r, err := evalsample.ReadSheet(es, rb)
	if err != nil || r != nil {
		t.Fatalf("%v %v", r, err)
	}
	s := sh.Sample
	pens := map[string]sample.Pen{}
	for _, st := range s.Strokes {
		if st.Style == nil {
			t.Fatalf("stroke %s has no style", st.ID)
		}
		pens[st.ID] = st.Style.Pen
	}
	want := map[string]sample.Pen{"1": sample.PenLiner, "2": sample.PenPen, "3": sample.PenLiner, "4": sample.PenMarker, "5": sample.PenMarker}
	if len(pens) != len(want) {
		t.Errorf("strokes %v, want %v", pens, want)
	}
	for id, p := range want {
		if pens[id] != p {
			t.Errorf("stroke %s of the pen %v, want %v", id, pens[id], p)
		}
	}
	if len(s.Erasures) != 1 || !slices.Equal(s.Erasures[0].Strokes, []string{"4"}) {
		t.Errorf("erasures %+v, want the cut of the underline", s.Erasures)
	}
	events := protocol.ErasureEvents(es.GetHistory())
	if len(events) != 1 || !protocol.ByEraser(events[0]) {
		t.Errorf("the erasure comes of %v, want a move of the eraser", events)
	}

	shown := render.Shown(s, math.MaxInt64)
	if len(shown) != 4 || slices.ContainsFunc(shown, func(st sample.Stroke) bool { return st.ID == "4" }) {
		t.Errorf("%d strokes shown", len(shown))
	}
	set, err := evalsample.CropSet(shown, evalsample.GroupsOf(es.GetSheet().GetGroups()), []int{render.Digit})
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(set.GetCrops()[0].GetPng()))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := img.(*image.RGBA); !ok {
		t.Errorf("the crop is %T, want RGB", img)
	}
	plain := []sample.Stroke{{ID: "1", Points: shown[0].Points}}
	set, err = evalsample.CropSet(plain, nil, []int{render.Digit})
	if err != nil {
		t.Fatal(err)
	}
	if img, err := png.Decode(bytes.NewReader(set.GetCrops()[0].GetPng())); err != nil {
		t.Fatal(err)
	} else if _, ok := img.(*image.Gray); !ok {
		t.Errorf("the crop of a stroke without a style is %T, want grey", img)
	}
}
