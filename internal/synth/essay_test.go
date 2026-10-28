package synth

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/altnovatrail/tutor/internal/sample"
)

// The lines of an essay: its title first, when it has one, and then its text broken between words,
// each line within the width, but for a word longer than the width, which takes a line of its own.
func TestEssayLines(t *testing.T) {
	title := "The Bread Robot"
	e := &sample.Essay{Title: &title, Text: "The robot wakes up at four. It bakes extraordinarily good bread."}
	got := EssayLines(e, 12)
	want := []string{"The Bread Robot", "The robot", "wakes up at", "four. It", "bakes", "extraordinarily", "good bread."}
	if !slices.Equal(got, want) {
		t.Errorf("EssayLines = %q, want %q", got, want)
	}
	e.Title = nil
	if got := EssayLines(e, 80); !slices.Equal(got, []string{e.Text}) {
		t.Errorf("EssayLines of no title = %q", got)
	}
}

// Every essay of the measurement, drawn with seed 1, is what it was: the hash of all their strokes
// holds them on every machine. The same seed draws the same essay, and another seed another one.
// Each essay is written in lines of 28 to 40 characters, which give back its title and its text, and
// every point lies inside the page of 1000 × 1400.
func TestEssays(t *testing.T) {
	f, err := os.Open("../../eval/essays.json")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	es, err := sample.ReadEssays(f)
	if err != nil {
		t.Fatal(err)
	}
	var all [][]sample.Stroke
	for i := range es.Essays {
		e := &es.Essays[i]
		lines, strokes := Essay(e, 1)
		if again, same := Essay(e, 1); !slices.Equal(again, lines) || !reflect.DeepEqual(same, strokes) {
			t.Errorf("essay %d: the same seed draws another essay", e.Number)
		}
		if _, other := Essay(e, 2); reflect.DeepEqual(other, strokes) {
			t.Errorf("essay %d: another seed draws the same strokes", e.Number)
		}
		body := lines
		if e.Title != nil {
			if lines[0] != *e.Title {
				t.Errorf("essay %d begins with %q, not its title", e.Number, lines[0])
			}
			body = lines[1:]
		}
		if strings.Join(body, " ") != e.Text {
			t.Errorf("essay %d: the lines %q do not give back its text", e.Number, body)
		}
		for _, l := range body {
			if utf8.RuneCountInString(l) > essayMaxLine {
				t.Errorf("essay %d: the line %q is longer than %d", e.Number, l, essayMaxLine)
			}
		}
		for _, s := range strokes {
			for _, p := range s.Points {
				if p.X <= 0 || p.X >= EssayWidth || p.Y <= 0 || p.Y >= EssayHeight {
					t.Fatalf("essay %d: the stroke %s reaches (%v, %v), off the page", e.Number, s.ID, p.X, p.Y)
				}
			}
		}
		all = append(all, strokes)
	}
	data, err := json.Marshal(all)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := fmt.Sprintf("%x", sha256.Sum256(data)), "a396239b81ffa9369696bef72564f7a5cedd98d2140087d0db9cce9dc121e0c5"; got != want {
		t.Errorf("the strokes of the essays hash to %s, want %s", got, want)
	}
}
