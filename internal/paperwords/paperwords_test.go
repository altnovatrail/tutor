package paperwords

import (
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/provider"
)

func TestFrozenV1(t *testing.T) {
	const frozen = "db9d46040adf1135e94064091ccf8c94d9232607f68338e39eb684228ac5f719"
	if got := Fingerprint(); got != frozen || Version != 1 {
		t.Errorf("the fingerprint of version %d is %s, not the one the list admits, %s", Version, got, frozen)
	}
}

var tasks = []Task{{Number: 1, Statement: "37 + 48 =", Key: "85", Read: "75"}, {Number: 2, Statement: "□ − 3 = 7"}}

func TestRequestOf(t *testing.T) {
	r, err := RequestOf(Prompt{Ask: AskRung, Image: provider.Image{PNG: []byte{1}}, Width: 300, Height: 80, Tasks: tasks, Task: 1, Rung: 2,
		Language: "en", Style: "an adult", Asked: []string{"Where is it?"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Image 1 is 300 pixels wide and 80 pixels high.", "Write: rung 2, a question on the rule, about task 1.",
		"Language: English.", "Words: no more than 35.", "1. Statement: 37 + 48 =\n   Key: 85\n   Learner's answer read: 75",
		"2. Statement: □ − 3 = 7\n   Key: none\n   Learner's answer read: none", "<asked>", "- Where is it?"} {
		if !strings.Contains(r.Text, want) {
			t.Errorf("the text lacks %q:\n%s", want, r.Text)
		}
	}
	if len(r.Images) != 1 || r.Images[0].Label != workLabel || r.NoPicture || r.Instructions != Instructions() || !r.CacheInstructions {
		t.Errorf("the request is %+v", r)
	}
	r, err = RequestOf(Prompt{Ask: AskOpening, Image: provider.Image{JPEG: []byte{1}}, Width: 30, Height: 8, Tasks: tasks, Language: "ru", Style: "a child"})
	if err != nil || r.Images[0].Label != printedLabel || !strings.Contains(r.Text, "Write: the opening question.") {
		t.Errorf("the opening question is %+v, %v", r, err)
	}
	// Before the program has read the tasks the opening question goes with none listed.
	r, err = RequestOf(Prompt{Ask: AskOpening, Image: provider.Image{PNG: []byte{1}}, Width: 30, Height: 8, Language: "en", Style: "an adult"})
	if err != nil || !strings.Contains(r.Text, noTasks) {
		t.Errorf("the opening question with no tasks is %+v, %v", r, err)
	}
	r, err = RequestOf(Prompt{Ask: AskAgain, Tasks: tasks, Task: 2, Rung: 3, Language: "en", Style: "an adult"})
	if err != nil || len(r.Images) != 0 || !r.NoPicture || !strings.Contains(r.Text, noImage) || !strings.Contains(r.Text, "Write: again, rung 3") {
		t.Errorf("the rung again is %+v, %v", r, err)
	}
}

func TestRequestOfRefuses(t *testing.T) {
	png := provider.Image{PNG: []byte{1}}
	for name, p := range map[string]Prompt{
		"no language":      {Ask: AskRung, Image: png, Tasks: tasks, Task: 1, Rung: 1, Language: "", Style: "s"},
		"no tasks":         {Ask: AskAgain, Task: 1, Rung: 1, Language: "en", Style: "s"},
		"no rung":          {Ask: AskRung, Image: png, Tasks: tasks, Task: 1, Rung: 4, Language: "en", Style: "s"},
		"no such task":     {Ask: AskRung, Image: png, Tasks: tasks, Task: 3, Rung: 1, Language: "en", Style: "s"},
		"a rung, no work":  {Ask: AskRung, Tasks: tasks, Task: 1, Rung: 1, Language: "en", Style: "s"},
		"again with work":  {Ask: AskAgain, Image: png, Tasks: tasks, Task: 1, Rung: 1, Language: "en", Style: "s"},
		"opening, no crop": {Ask: AskOpening, Tasks: tasks, Language: "en", Style: "s"},
		"no kind of words": {Tasks: tasks, Language: "en", Style: "s"},
	} {
		if _, err := RequestOf(p); err == nil {
			t.Errorf("%s: no refusal", name)
		}
	}
}

func TestRead(t *testing.T) {
	a, err := Read(`{"box": {"x1": 30, "y1": 40, "x2": 10, "y2": 20}, "answers": [], "hint": "Where?"}`)
	if err != nil || a.Box == nil || a.Box.X1 != 10 || a.Box.Y2 != 40 || a.Hint != "Where?" || len(a.Answers) != 0 {
		t.Errorf("the answer is %+v, %v", a, err)
	}
	a, err = Read(`{"box": null, "answers": ["85"], "hint": "What first?"}`)
	if err != nil || a.Box != nil || a.Answers[0] != "85" {
		t.Errorf("the answer is %+v, %v", a, err)
	}
	for _, bad := range []string{`{"box": null, "hint": "x"}`, `{"box": null, "answers": [], "hint": "x", "more": 1}`, `{"box": null, "answers": [], "hint": "x"} {}`, `[]`} {
		if _, err := Read(bad); err == nil {
			t.Errorf("%s: read", bad)
		}
	}
}
