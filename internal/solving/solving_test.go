package solving

import (
	"strings"
	"testing"

	paperv1 "github.com/altnovatrail/spec/contracts/novatrail/paper/v1"
	"google.golang.org/protobuf/proto"

	"github.com/altnovatrail/tutor/provider"
)

// Version 1 is frozen: the entry of the list admits this fingerprint (R135).
func TestFrozenV1(t *testing.T) {
	const frozen = "0bac79715447d6f2e028700328fb1089bdcb7891c770a03f9ce1cbaeee43f823"
	if got := Fingerprint(); got != frozen || Version != 1 {
		t.Errorf("the fingerprint of version %d is %s, not the one the list admits, %s", Version, got, frozen)
	}
}

func TestRequestOf(t *testing.T) {
	r := RequestOf(provider.Image{JPEG: []byte{1}}, 300, 80)
	if r.Instructions != Instructions() || len(r.Images) != 1 || r.Images[0].Label != "Image 1: the task as printed on the page." ||
		r.Text != "Image 1 is 300 pixels wide and 80 pixels high. Solve the task on it." || r.SchemaName != "task_answer" || !r.CacheInstructions {
		t.Errorf("the request is %+v", r)
	}
	if err := provider.CheckImages(r, 1); err != nil {
		t.Error(err)
	}
	if !strings.Contains(Instructions(), `"none" when the task has no one answer`) || !strings.Contains(Instructions(), "The learner never sees your answer") {
		t.Error("the instructions lost a rule")
	}
}

// Read gives the answer of each kind, and Answered writes it back as Read takes it.
func TestReadAndAnswered(t *testing.T) {
	number := &paperv1.Answer{Kind: &paperv1.Answer_Number{Number: &paperv1.Number{Value: "-3", Units: []string{}, Unknown: "x"}}}
	choice := &paperv1.Answer{Kind: &paperv1.Answer_Choice{Choice: &paperv1.Choice{Options: []string{"A", "B"}, Key: "B"}}}
	for _, want := range []*paperv1.Answer{number, choice, nil} {
		text, err := Answered(want)
		if err != nil {
			t.Fatal(err)
		}
		got, err := Read(text)
		if err != nil || !proto.Equal(got, want) {
			t.Errorf("%s: read %v, %v", text, got, err)
		}
	}
	// The fields of the other kind are not read.
	got, err := Read(`{"kind": "choice", "value": "7", "units": ["cm"], "unknown": "x", "options": ["A", "B"], "key": "A"}`)
	if err != nil || got.GetChoice().GetKey() != "A" || got.GetNumber() != nil {
		t.Errorf("read %v, %v", got, err)
	}
	for name, answer := range map[string]string{
		"not JSON":      `12`,
		"a field less":  `{"kind": "none", "value": "", "units": [], "unknown": "", "options": []}`,
		"another field": `{"kind": "none", "value": "", "units": [], "unknown": "", "options": [], "key": "", "working": "12"}`,
		"another kind":  `{"kind": "text", "value": "", "units": [], "unknown": "", "options": [], "key": ""}`,
		"more after it": `{"kind": "none", "value": "", "units": [], "unknown": "", "options": [], "key": ""} {}`,
	} {
		if _, err := Read(answer); err == nil {
			t.Errorf("%s: read", name)
		}
	}
}
