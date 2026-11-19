package statement

import (
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/provider"
)

// Version 1 is frozen: the entry of the list admits this fingerprint (R135).
func TestFrozenV1(t *testing.T) {
	const frozen = "1e101d0e1e475651f88c2fa4bc0c5910c2421c6a64fe15a4c623e6b38fd98723"
	if got := Fingerprint(); got != frozen || Version != 1 {
		t.Errorf("the fingerprint of version %d is %s, not the one the list admits, %s", Version, got, frozen)
	}
}

func TestRequestOf(t *testing.T) {
	r := RequestOf(provider.Image{JPEG: []byte{1}}, 300, 80)
	if r.Instructions != Instructions() || len(r.Images) != 1 || r.Images[0].Label != "Image 1: the task as printed on the page." ||
		r.Text != "Image 1 is 300 pixels wide and 80 pixels high. Write its statement as a formula, or an empty one." ||
		r.SchemaName != "statement_formula" || !r.CacheInstructions {
		t.Errorf("the request is %+v", r)
	}
	if err := provider.CheckImages(r, 1); err != nil {
		t.Error(err)
	}
	// A word problem is no formula, and the reader writes the grammar of the solver.
	for _, rule := range []string{"A word problem", "is no formula", "3x is 3*x", `"□ + 5 = 12" is x+5=12`} {
		if !strings.Contains(Instructions(), rule) {
			t.Errorf("the instructions lost %q", rule)
		}
	}
}

func TestRead(t *testing.T) {
	for answer, want := range map[string]string{`{"formula": "x+5=12"}`: "x+5=12", `{"formula": "  "}`: "", `{"formula": ""}`: ""} {
		if got, err := Read(answer); err != nil || got != want {
			t.Errorf("%s: read %q, %v", answer, got, err)
		}
	}
	for name, answer := range map[string]string{
		"not JSON":      `x+5=12`,
		"no formula":    `{}`,
		"not a string":  `{"formula": 12}`,
		"another field": `{"formula": "", "text": "Name: Ann"}`,
		"more after it": `{"formula": ""} {}`,
	} {
		if _, err := Read(answer); err == nil {
			t.Errorf("%s: read", name)
		}
	}
}
