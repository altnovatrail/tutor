package essay

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/provider"
)

var update = flag.Bool("update", false, "write the golden request anew")

// written writes a request as the golden file keeps it: the instructions, the picture's label and
// the text, and no pixels.
func written(r provider.Request) string {
	var b strings.Builder
	fmt.Fprintf(&b, "=== instructions\n%s\n", r.Instructions)
	for _, im := range r.Images {
		fmt.Fprintf(&b, "=== picture: the crop of the essay\n%s\n", im.Label)
	}
	fmt.Fprintf(&b, "=== text\n%s\n", r.Text)
	return b.String()
}

// The golden request is the file testdata/request.txt, on essay 1; the author reads it there. The
// right answer, read, comes to the planned truth of every criterion, reads the essay as it is
// written and keeps the rule of the feedback.
func TestGolden(t *testing.T) {
	es := readEssays(t)
	e := &es.Essays[0]
	pic, err := PictureOf(e, 1, render.Digit)
	if err != nil {
		t.Fatal(err)
	}
	r := Request(e, es.Rubric, pic)
	if r.SchemaName != "assessment" || len(r.Images) != 1 || r.Images[0].Label != label || r.Instructions != Instructions() {
		t.Errorf("the request is %+v", r)
	}
	got := written(r)
	path := filepath.Join("testdata", "request.txt")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	} else if want, err := os.ReadFile(path); err != nil || got != string(want) {
		t.Errorf("the request is\n%s\nwant, as %s,\n%s (%v)", got, path, want, err)
	}
	for i := range es.Essays {
		e := &es.Essays[i]
		right := Right(e, es.Rubric)
		marks, extra := Marks(es.Rubric, &right)
		for _, m := range marks {
			if m.Met == nil || *m.Met == slices.Contains(e.Unmet, m.ID) {
				t.Errorf("essay %d: the right answer comes to %+v on %s", e.Number, m, m.ID)
			}
		}
		if breaches, words := Feedback(right.Feedback, e); extra != 0 || breaches != nil || words != 14 {
			t.Errorf("essay %d: the right answer has %d judgements of no criterion, and its feedback of %d words breaks %v", e.Number, extra, words, breaches)
		}
		if rd := ReadingOf(e, right.Text); rd.WordErrors != 0 || rd.MisspelledKept != rd.Misspelled || rd.LowerStartsKept != rd.LowerStarts || rd.Unended && !*rd.UnendedKept {
			t.Errorf("essay %d: the right answer reads as %+v", e.Number, rd)
		}
	}
	if fp := Fingerprint(); len(fp) != 64 {
		t.Errorf("the fingerprint is %q", fp)
	}
}

// An answer is read as strictly as the schema: every field, nothing else.
func TestRead(t *testing.T) {
	good := `{"text": "A title\nThe text.", "criteria": [{"id": "title", "met": true, "evidence": "A title"}], "feedback": "Good."}`
	a, err := Read(good)
	if err != nil || a.Text != "A title\nThe text." || len(a.Criteria) != 1 || !a.Criteria[0].Met || a.Feedback != "Good." {
		t.Fatalf("Read = %+v, %v", a, err)
	}
	for _, bad := range []string{
		`{"text": "x", "criteria": [], "feedback": "y", "score": 3}`,
		`{"text": "x", "criteria": []}`,
		`{"text": "x", "criteria": [{"id": "title", "met": true}], "feedback": "y"}`,
		`not json`,
	} {
		if _, err := Read(bad); err == nil {
			t.Errorf("Read(%s) took it", bad)
		}
	}
}

// A criterion is judged once, or it is unchecked: missing, twice, or of a call that failed; a
// judgement of no criterion of the rubric is counted apart.
func TestMarks(t *testing.T) {
	rubric := []sample.Criterion{{ID: "title"}, {ID: "sentences"}, {ID: "spelling"}}
	a := &Answer{Criteria: []Judgement{{ID: "title", Met: false}, {ID: "spelling", Met: true}, {ID: "spelling", Met: false}, {ID: "neatness", Met: true}}}
	marks, extra := Marks(rubric, a)
	if extra != 1 || len(marks) != 3 {
		t.Fatalf("Marks = %+v, %d", marks, extra)
	}
	if m := marks[0]; m.Met == nil || *m.Met || m.Reason != "" {
		t.Errorf("title: %+v", m)
	}
	if m := marks[1]; m.Met != nil || m.Reason != ReasonMissing {
		t.Errorf("sentences: %+v", m)
	}
	if m := marks[2]; m.Met != nil || m.Reason != ReasonTwice {
		t.Errorf("spelling: %+v", m)
	}
	failed, extra := Marks(rubric, nil)
	for _, m := range failed {
		if m.Met != nil || m.Reason != ReasonFailed || extra != 0 {
			t.Errorf("a failed call: %+v, %d", m, extra)
		}
	}
}

// The rule of the feedback: at most three sentences and 60 words, a quoted span of four words or
// more only as the essay writes it, and Latin letters alone.
func TestFeedback(t *testing.T) {
	title := "The Bread Robot"
	e := &sample.Essay{Title: &title, Text: "The robot wakes up at four. It is quik tired, but happy."}
	for _, c := range []struct {
		feedback string
		want     []string
	}{
		{`Your title "The Bread Robot" is good. Check the word "quik".`, nil},
		{`You wrote "It is quik tired, but happy." Well done.`, nil},
		{`You quote “the robot wakes up at four” well.`, nil},
		{`Try «The robot wakes up very early» instead.`, []string{BreachQuote}},
		{`You could write "The robot is sleepy at dawn." Good work.`, []string{BreachQuote}},
		{"One. Two. Three. Four.", []string{BreachSentences}},
		{strings.Repeat("word ", 61), []string{BreachWords}},
		{"", []string{BreachWords}},
		{"Хорошо. Good.", []string{BreachScript}},
		{"Try ‘The robot sleeps until noon’ next time.", []string{BreachQuote}},
		{"You don't need to say it's a robot that wakes up early.", nil},
		{`One. Two. Three. Four "with words not in it" too.`, []string{BreachSentences, BreachQuote}},
	} {
		if got, _ := Feedback(c.feedback, e); !slices.Equal(got, c.want) {
			t.Errorf("Feedback(%q) = %v, want %v", c.feedback, got, c.want)
		}
	}
}

// The reading of an essay: the word errors are the edit distance in words, in small letters and
// without marks; a misspelling, a small letter at the start of a sentence and a last sentence
// without its mark are kept only when the reading keeps them as they are written.
func TestReadingOf(t *testing.T) {
	title := "My Day"
	e := &sample.Essay{Title: &title, Text: "I walk to the sea. the sand is warm and my frend is there. We swim",
		Misspelled: []sample.Misspelling{{Written: "frend", Word: "friend"}}}
	kept := ReadingOf(e, "My Day\nI walk to the sea. the sand is warm\nand my frend is there. We swim")
	if kept.Words != 18 || kept.WordErrors != 0 || kept.MisspelledKept != 1 || kept.LowerStartsKept != 1 || !*kept.UnendedKept {
		t.Errorf("a reading as written: %+v", kept)
	}
	fixed := ReadingOf(e, "My Day. I walk to a sea. The sand is warm and my friend is there. We swim.")
	if fixed.WordErrors != 2 || fixed.MisspelledKept != 0 || fixed.LowerStartsKept != 0 || *fixed.UnendedKept {
		t.Errorf("a reading that corrects: %+v", fixed)
	}
	// The first sentence begins with a small letter, a comma follows its first word, and the reading
	// ends with a closing quote after a mark it added; a misspelled form read in another word's place
	// is no misspelling kept.
	opens := &sample.Essay{Text: "then, we swim. I am a frend and a frend is good. We swim",
		Misspelled: []sample.Misspelling{{Written: "frend", Word: "friend"}}}
	if rd := ReadingOf(opens, "then, we swim. I am a friend and a frend is good. We swim.”"); rd.LowerStartsKept != 1 || rd.MisspelledKept != 0 || *rd.UnendedKept {
		t.Errorf("a reading of the first sentence: %+v", rd)
	}
	if rd := ReadingOf(opens, "Then we swim. I am a frend and a frend is good. We swim…"); rd.LowerStartsKept != 0 || rd.MisspelledKept != 1 || *rd.UnendedKept {
		t.Errorf("a reading that capitalizes the first sentence: %+v", rd)
	}
	if d, at := align([]string{"a", "b", "c"}, []string{"b", "c", "d", "e"}); d != 3 || !slices.Equal(at, []int{-1, 0, 1}) {
		t.Errorf("align = %d, %v", d, at)
	}
	if d, at := align([]string{"x", "y"}, []string{"y", "x"}); d != 2 || at[0] < 0 && at[1] < 0 {
		t.Errorf("align of a swap = %d, %v", d, at)
	}
	if got := quoted(`a "b c" and «d» and “e” and "unclosed`); !slices.Equal(got, []string{"b c", "d", "e"}) {
		t.Errorf("quoted = %q", got)
	}
}

// Version 1 of the prompt is frozen after the dev rounds of T84, before its test part runs: any change
// to a word a model reads changes the fingerprint, and this test fails.
func TestFrozenV1(t *testing.T) {
	const frozen = "e395e9fcefeea413b28c2c63f3df9713ec23b3a0ec0f6cffe11d7c471e15accd"
	if got := Fingerprint(); got != frozen || Version != 1 {
		t.Errorf("the fingerprint of version %d is %s, not the one frozen after the dev rounds of T84, %s", Version, got, frozen)
	}
}
