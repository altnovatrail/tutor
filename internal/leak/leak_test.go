package leak

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"gopkg.in/yaml.v3"
)

// The test vectors of the leak check are copies of format/vectors/leaks of spec at v0.3.2 (commit
// 9d64b22), where they last changed in b217758, under Apache-2.0 as spec is. CI checks the
// repository out without the spec submodule, so the copies are what it tests; TestCopies holds them
// to spec's wherever the submodule is there.
const vectors = "testdata/leaks"

// vector is a vector file: a task, as much of it as the leak check reads, and the texts searched,
// each with what the check finds in it.
type vector struct {
	Task struct {
		Statement []struct {
			Text string `yaml:"text"`
			Alt  string `yaml:"alt"`
		} `yaml:"statement"`
		Items []struct {
			Text        string `yaml:"text"`
			AnswerSlots []slot `yaml:"answer_slots"`
			Line        []struct {
				Text    string `yaml:"text"`
				Binding string `yaml:"binding"`
				Check   check  `yaml:"check"`
			} `yaml:"line"`
		} `yaml:"items"`
		LeakPhrases []string `yaml:"leak_phrases"`
	} `yaml:"task"`
	Texts []struct {
		Text  string   `yaml:"text"`
		Leaks []string `yaml:"leaks"`
	} `yaml:"texts"`
}

type slot struct {
	Binding string   `yaml:"binding"`
	Options []string `yaml:"options"`
	Check   check    `yaml:"check"`
}

type check struct {
	Kind string `yaml:"kind"`
	Key  string `yaml:"key"`
}

// task is the vector's task as the leak check reads it: the statement's paragraphs and alts, and
// for each item its text, the text pieces of its line, the options of its answer slots bound choice
// and the keys of its checks.
func (v *vector) task() Task {
	var t Task
	for _, p := range v.Task.Statement {
		for _, text := range []string{p.Text, p.Alt} {
			if text != "" {
				t.Statement = append(t.Statement, text)
			}
		}
	}
	for _, it := range v.Task.Items {
		var item Item
		if it.Text != "" {
			item.Texts = append(item.Texts, it.Text)
		}
		for _, s := range it.AnswerSlots {
			if s.Binding == "choice" {
				item.Texts = append(item.Texts, s.Options...)
			}
			item.Keys = append(item.Keys, Key{Kind: s.Check.Kind, Value: s.Check.Key})
		}
		for _, p := range it.Line {
			if p.Binding == "" {
				item.Texts = append(item.Texts, p.Text)
				continue
			}
			item.Keys = append(item.Keys, Key{Kind: p.Check.Kind, Value: p.Check.Key})
		}
		t.Items = append(t.Items, item)
	}
	t.Phrases = v.Task.LeakPhrases
	return t
}

// Every text of every vector gives what the vector says the check finds in it, in any order.
func TestVectors(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join(vectors, "*.yaml"))
	if err != nil || len(paths) != 17 {
		t.Fatalf("the vectors are %v, %v", paths, err)
	}
	texts := 0
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var v vector
		if err := yaml.Unmarshal(data, &v); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		c := New(v.task())
		for _, want := range v.Texts {
			texts++
			got := c.Find(want.Text)
			if !sameSet(got, want.Leaks) {
				t.Errorf("%s, %q: found %q, want %q", filepath.Base(path), want.Text, got, want.Leaks)
			}
		}
	}
	if texts != 85 {
		t.Errorf("the vectors hold %d texts, want 85", texts)
	}
}

func sameSet(a, b []string) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}

// The copies are spec's vectors as they are, file for file, where the spec submodule is checked
// out; in CI, which has no submodule, the test is skipped.
func TestCopies(t *testing.T) {
	spec := filepath.Join("..", "..", "spec", "format", "vectors", "leaks")
	theirs, err := os.ReadDir(spec)
	if err != nil {
		t.Skipf("no spec submodule: %v", err)
	}
	ours, err := os.ReadDir(vectors)
	if err != nil {
		t.Fatal(err)
	}
	names := func(entries []os.DirEntry) []string {
		var out []string
		for _, e := range entries {
			out = append(out, e.Name())
		}
		return out
	}
	if !slices.Equal(names(ours), names(theirs)) {
		t.Fatalf("the copies are %v, and spec has %v", names(ours), names(theirs))
	}
	for _, name := range names(ours) {
		a, _ := os.ReadFile(filepath.Join(vectors, name))
		b, _ := os.ReadFile(filepath.Join(spec, name))
		if !bytes.Equal(a, b) {
			t.Errorf("%s is not spec's: copy it again, and say which tag in the comment of the vectors", name)
		}
	}
}

// numbers are the numbers of a text, as numerator/denominator in lowest terms.
func numbers(text string) []string {
	var out []string
	for _, r := range numbersIn(form(text)) {
		out = append(out, r.RatString())
	}
	return out
}

// The edges of the numbers of a text, as spec's implementation tests them
// (tools/format-check/test/leaks.test.mjs): a number begins at a digit with no character of a word
// before it and none after the longest run, and the search goes on after the whole run.
func TestNumbers(t *testing.T) {
	for _, c := range []struct {
		text string
		want []string
	}{
		{"7 rungs, (7), 07:30", []string{"7", "7", "7", "30"}},
		{"17 x7 7x 7th 7rungs", []string{"17"}},
		{"-7 and 5-7", []string{"7", "5", "7"}},
		{"x77", nil},
		{"x7.5 v1/2 A4.7", nil},
		{"a5-3", []string{"3"}},
		{"1/2, 2 / 4 and 0.50", []string{"1/2", "1/2", "1/2"}},
		{"12/4.5", []string{"3", "5"}},
		{"1/0 and 1/00", []string{"1", "0", "1", "0"}},
		{"1 1/2", []string{"1", "1/2"}},
		{"7. And .5", []string{"7", "5"}},
		{"1/2x", nil},
		{"1/x", []string{"1"}},
		{"0009007199254740992", []string{"9007199254740992"}},
		{"010/2", []string{"5"}},
		{"½ and 1½", []string{"1/2", "1"}},
		{"\U0001D7D5", []string{"7"}},
	} {
		if got := numbers(c.text); !slices.Equal(got, c.want) {
			t.Errorf("%q: %q, want %q", c.text, got, c.want)
		}
	}
}

// A phrase is found whole where its ends are characters of a word, and anywhere where they are
// signs; a phrase whose normal form is empty is found nowhere; every place a phrase stands is tried,
// not only the first.
func TestPhrases(t *testing.T) {
	for _, c := range []struct {
		text, phrase string
		want         bool
	}{
		{"It is odd.", "odd", true},
		{"even/odd", "odd", true},
		{"the odds", "odd", false},
		{"oddly", "odd", false},
		{"oddly odd", "odd", true},
		{"56<65", "<", true},
		{"56 ≤ 65", "<", false},
		{"＜", "<", true},
		{"किताब", "कित", false},
		{"Café", "cafe", false},
		{"7 RUNGS", "7 rungs", true},
		{"anything", " ", false},
		{"anything", "", false},
		{"на 2  ноги больше", "на 2 ноги больше", true},
	} {
		if got := holdsForm(form(c.text), form(c.phrase)); got != c.want {
			t.Errorf("%q in %q: %v", c.phrase, c.text, got)
		}
	}
}

// A key is searched for unless the statement or the texts of its own item show it, and a leak
// phrase always; a key out of its kind's notation, and a kind with no key, are searched for nowhere.
func TestSearched(t *testing.T) {
	item := func(texts []string, keys ...Key) Item { return Item{Texts: texts, Keys: keys} }
	for _, c := range []struct {
		name string
		task Task
		want []string
	}{
		{"in the statement", Task{Statement: []string{"A chicken has 2 legs."}, Items: []Item{item(nil, Key{Number, "2"})}, Phrases: []string{"2 rabbits"}}, []string{"2 rabbits"}},
		{"in an alt", Task{Statement: []string{"Rows of 7 dots"}, Items: []Item{item(nil, Key{Number, "7"})}}, nil},
		{"own item only", Task{Items: []Item{item([]string{"3 +", "= 6"}, Key{Number, "3"}), item([]string{"- 3 = 3"}, Key{Number, "6"})}}, []string{"6"}},
		{"tapped", Task{Items: []Item{item([]string{"A cat", "A dog"}, Key{Choice, "A dog"})}}, nil},
		{"by hand", Task{Items: []Item{item(nil, Key{Choice, "odd"})}}, []string{"odd"}},
		{"predicate", Task{Items: []Item{item(nil, Key{"predicate", ""})}}, nil},
		{"placeholder", Task{Items: []Item{item([]string{"and"}, Key{Number, "{x}"}, Key{Number, "42"})}}, []string{"42"}},
		{"a fraction that is no fraction", Task{Items: []Item{item(nil, Key{Fraction, "0.5"}, Key{Fraction, "1/2"})}}, []string{"1/2"}},
	} {
		ch := New(c.task)
		if got := append(ch.Keys(), ch.Phrases()...); !slices.Equal(got, c.want) {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

// What a text gives away: each key by value and magnitude, each phrase as written, each once.
func TestFind(t *testing.T) {
	anywhere := func(keys ...Key) Task {
		var items []Item
		for _, k := range keys {
			items = append(items, Item{Keys: []Key{k}})
		}
		return Task{Statement: []string{"A task."}, Items: items}
	}
	half := anywhere(Key{Number, "0.5"}, Key{Fraction, "1/2"})
	half.Phrases = []string{"a half"}
	for _, c := range []struct {
		task Task
		text string
		want []string
	}{
		{half, "2/4, and 1/2: A HALF.", []string{"0.5", "1/2", "a half"}},
		{half, "0,5 and .5", nil},
		{anywhere(Key{Number, "-3"}), "5-3", []string{"-3"}},
		{anywhere(Key{Number, "8"}, Key{Number, "8"}), "Two keys of 8: 8.", []string{"8"}},
		{anywhere(Key{Choice, "odd"}), "It is odd.", []string{"odd"}},
	} {
		if got := New(c.task).Find(c.text); !slices.Equal(got, c.want) {
			t.Errorf("%q: %q, want %q", c.text, got, c.want)
		}
	}
	// A leak phrase written as a key is a phrase, and Find gives it once.
	five := Task{Statement: []string{"Every 5 m."}, Items: []Item{{Keys: []Key{{Number, "5"}}}}, Phrases: []string{"5"}}
	c := New(five)
	keys, phrases := c.Split("It is 5.")
	if c.Keys() != nil || !slices.Equal(c.Phrases(), []string{"5"}) || keys != nil || !slices.Equal(phrases, []string{"5"}) || !slices.Equal(c.Find("5"), []string{"5"}) {
		t.Errorf("the key 5 that the statement shows and the phrase 5: %v %v, %v %v", c.Keys(), c.Phrases(), keys, phrases)
	}
	both := New(Task{Items: []Item{{Keys: []Key{{Number, "8"}}}}, Phrases: []string{"8"}})
	if got := both.Find("8"); !slices.Equal(got, []string{"8"}) {
		t.Errorf("the key 8 and the phrase 8 give %v", got)
	}
}

func Example() {
	c := New(Task{
		Statement: []string{"Во дворе гуляют куры и кролики. У них вместе 5 голов и 14 ног. Сколько во дворе кроликов?"},
		Items:     []Item{{Keys: []Key{{Number, "2"}}}},
		Phrases:   []string{"на 2 ноги больше", "two more legs"},
	})
	fmt.Println(c.Find("Кроликов 2."))
	fmt.Println(c.Find("У кролика на 2 ноги больше, чем у курицы?"))
	fmt.Println(c.Find("Сколько ног у курицы?"))
	// Output:
	// [2]
	// [2 на 2 ноги больше]
	// []
}
