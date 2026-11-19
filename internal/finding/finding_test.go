package finding

import (
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/provider"
)

// Version 1 is frozen: the entry of the list admits this fingerprint (R134).
func TestFrozenV1(t *testing.T) {
	const frozen = "311d729683e8bf38e169990d123005f5ce93ea6571294ee11a85a1461edf023f"
	if got := Fingerprint(); got != frozen || Version != 1 {
		t.Errorf("the fingerprint of version %d is %s, not the one the list admits, %s", Version, got, frozen)
	}
}

func TestRequestOf(t *testing.T) {
	r := RequestOf(provider.Image{JPEG: []byte{1}}, 1200, 1600)
	if r.Instructions != Instructions() || len(r.Images) != 1 || r.Images[0].Label != "Image 1: the photo of the page." ||
		r.Text != "Image 1 is 1200 pixels wide and 1600 pixels high. Find the tasks on it." ||
		r.SchemaName != "task_frames" || r.NoPicture || !r.CacheInstructions {
		t.Errorf("the request is %+v", r)
	}
	if err := provider.CheckImages(r, 1); err != nil {
		t.Error(err)
	}
	// The model is told not to copy the page, and of how many tasks it may give.
	if !strings.Contains(Instructions(), "the answer holds the boxes alone") || !strings.Contains(Instructions(), "at most 20 tasks") {
		t.Error("the instructions lost a rule")
	}
}

func TestRead(t *testing.T) {
	boxes, err := Read(`{"tasks": [{"x1": 300, "y1": 40, "x2": 10, "y2": 200}, {"x1": 0, "y1": 0, "x2": 5, "y2": 5}]}`)
	if err != nil || len(boxes) != 2 || boxes[0] != (Box{X1: 10, Y1: 40, X2: 300, Y2: 200}) {
		t.Fatalf("read %v, %v", boxes, err)
	}
	if boxes, err := Read(`{"tasks": []}`); err != nil || len(boxes) != 0 {
		t.Errorf("no task: %v, %v", boxes, err)
	}
	twentyOne := `{"tasks": [` + strings.Repeat(`{"x1": 0, "y1": 0, "x2": 5, "y2": 5},`, MaxTasks) + `{"x1": 0, "y1": 0, "x2": 5, "y2": 5}]}`
	for name, answer := range map[string]string{
		"not JSON":        `tasks`,
		"no tasks":        `{}`,
		"another field":   `{"tasks": [], "text": "Name: Ann"}`,
		"a missed corner": `{"tasks": [{"x1": 0, "y1": 0, "x2": 5}]}`,
		"a field of a box": `{"tasks": [{"x1": 0, "y1": 0, "x2": 5, "y2": 5, "label": "1"}]}`,
		"more after it":   `{"tasks": []} {}`,
		"too many":        twentyOne,
	} {
		if _, err := Read(answer); err == nil {
			t.Errorf("%s: read", name)
		}
	}
}
