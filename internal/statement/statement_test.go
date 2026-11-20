package statement

import (
	"reflect"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/provider"
)

// Version 2 is frozen: the entry of the list admits this fingerprint (R135).
func TestFrozenV2(t *testing.T) {
	const frozen = "cd0747cc37281291346a44aa17de104efdf8abd825b0fa428d094947194e7e56"
	if got := Fingerprint(); got != frozen || Version != 2 {
		t.Errorf("the fingerprint of version %d is %s, not the one the list admits, %s", Version, got, frozen)
	}
}

func TestRequestOf(t *testing.T) {
	r := RequestOf(provider.Image{JPEG: []byte{1}}, 300, 80)
	if r.Instructions != Instructions() || len(r.Images) != 1 || r.Images[0].Label != "Image 1: the task as printed on the page." ||
		r.Text != "Image 1 is 300 pixels wide and 80 pixels high. List its tasks, each with its statement and its formula, or an empty one." ||
		r.SchemaName != "statement_formula" || !r.CacheInstructions {
		t.Errorf("the request is %+v", r)
	}
	if err := provider.CheckImages(r, 1); err != nil {
		t.Error(err)
	}
	// Each sum of a column is a task, the parts of a word problem are one, a word problem is no
	// formula, and the reader writes the grammar of the solver.
	for _, rule := range []string{"is four tasks", "are one task", "A word problem", "is no formula", "3x is 3*x", `"□ + 5 = 12" is x+5=12`} {
		if !strings.Contains(Instructions(), rule) {
			t.Errorf("the instructions lost %q", rule)
		}
	}
}

func TestRead(t *testing.T) {
	long := strings.Repeat("□", 70) // 210 bytes
	for answer, want := range map[string][]Task{
		`{"tasks": [{"statement": "□ + 5 = 12", "formula": "x+5=12"}]}`:      {{Statement: "□ + 5 = 12", Formula: "x+5=12"}},
		`{"tasks": [{"statement": " Ann has 3 apples. ", "formula": "  "}]}`: {{Statement: "Ann has 3 apples."}},
		`{"tasks": [{"statement": "□ − 3 = 7", "formula": "x-3=7"}, {"formula": "x+4=10", "statement": "□ + 4 = 10"}]}`: {
			{Statement: "□ − 3 = 7", Formula: "x-3=7"}, {Statement: "□ + 4 = 10", Formula: "x+4=10"},
		},
		`{"tasks": []}`: {},
		`{"tasks": [{"statement": "` + long + `", "formula": ""}]}`: {{Statement: strings.Repeat("□", 66)}},
	} {
		if got, err := Read(answer); err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("%s: read %+v, %v", answer, got, err)
		}
	}
	many := `{"tasks": [` + strings.Repeat(`{"statement": "1+1", "formula": "1+1"},`, MaxTasks) + `{"statement": "1+1", "formula": "1+1"}]}`
	for name, answer := range map[string]string{
		"not JSON":          `x+5=12`,
		"no tasks":          `{}`,
		"no formula":        `{"tasks": [{"statement": "1+1"}]}`,
		"no statement":      `{"tasks": [{"formula": "1+1"}]}`,
		"empty statement":   `{"tasks": [{"statement": " ", "formula": "1+1"}]}`,
		"not a string":      `{"tasks": [{"statement": "1+1", "formula": 12}]}`,
		"another field":     `{"tasks": [], "text": "Name: Ann"}`,
		"another of a task": `{"tasks": [{"statement": "1+1", "formula": "", "name": "Ann"}]}`,
		"more after it":     `{"tasks": []} {}`,
		"too many":          many,
	} {
		if _, err := Read(answer); err == nil {
			t.Errorf("%s: read", name)
		}
	}
}
