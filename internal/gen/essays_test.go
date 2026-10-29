package gen

import (
	"bytes"
	"math/rand/v2"
	"os"
	"slices"
	"strings"
	"testing"
	"unicode"

	"github.com/altnovatrail/tutor/internal/essay"
	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/sample"
)

// The essays of eval, eval/essays-gen.json, are the ones Essays builds from the seed 1, byte for
// byte; go test -update writes them anew.
func TestEssaysGolden(t *testing.T) {
	es, err := Essays(setSize, setSeed)
	if err != nil {
		t.Fatal(err)
	}
	golden(t, essaysFile, es)
}

// The same seed builds the same essays, and another seed others.
func TestEssaysSeed(t *testing.T) {
	a, errA := Essays(40, 7)
	b, errB := Essays(40, 7)
	c, errC := Essays(40, 8)
	if errA != nil || errB != nil || errC != nil {
		t.Fatal(errA, errB, errC)
	}
	da, _ := sample.Marshal(a)
	db, _ := sample.Marshal(b)
	dc, _ := sample.Marshal(c)
	if !bytes.Equal(da, db) || bytes.Equal(da, dc) {
		t.Error("the same seed builds other essays, or another seed the same")
	}
	if _, err := Essays(0, 1); err == nil {
		t.Error("no essays are built")
	}
}

// The rubric is E4's, eval/essays.json's, criterion by criterion.
func TestRubric(t *testing.T) {
	f, err := os.Open("../../eval/essays.json")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	e4, err := sample.ReadEssays(f)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(rubric, e4.Rubric) {
		t.Errorf("the rubric is %v, and E4's %v", rubric, e4.Rubric)
	}
}

// The 300 essays pass the check and draw; the first 20 are of the development; the topics come by
// turns, each 13 or 14 times; and in each part each criterion is unmet in a quarter of the essays, a
// fifth meet them all, and none leaves more than three unmet.
func TestEssays(t *testing.T) {
	es, err := Essays(setSize, setSeed)
	if err != nil {
		t.Fatal(err)
	}
	if err := es.Check(); err != nil {
		t.Fatal(err)
	}
	topicOf := map[string]int{}
	for i, tp := range topics {
		topicOf[tp.statement] = i
	}
	used := make([]int, len(topics))
	for i, e := range es.Essays {
		if want := i < 20; (e.Part == sample.PartDev) != want {
			t.Errorf("essay %d is of the part %s", e.Number, e.Part)
		}
		at, ok := topicOf[e.Topic]
		if !ok {
			t.Fatalf("essay %d is on no topic: %q", e.Number, e.Topic)
		}
		used[at]++
		if len(e.Unmet) > 3 {
			t.Errorf("essay %d leaves %v unmet", e.Number, e.Unmet)
		}
		if i%25 == 0 {
			if _, err := essay.PictureOf(&es.Essays[i], setSeed, render.Digit); err != nil {
				t.Errorf("essay %d: %v", e.Number, err)
			}
		}
	}
	if lo, hi := slices.Min(used), slices.Max(used); lo < setSize/len(topics) || hi > setSize/len(topics)+1 {
		t.Errorf("the topics are used %v times", used)
	}
	tally := es.Tally()
	for _, id := range sample.Criteria {
		if got := tally[id]; got != [2]int{5, 70} {
			t.Errorf("the criterion %s is unmet in %v essays, not 5 of the development and 70 of the test", id, got)
		}
	}
	if got := tally[""]; got != [2]int{4, 56} {
		t.Errorf("%v essays meet every criterion, not 4 and 56", got)
	}
}

// Every topic keeps to the rules of the essays: a statement and a title the check takes; sentences
// that end with a full stop, hold no capital inside but I, and link their ideas or hold no linking
// or joining word at all, the first of them of the second kind; enough of each kind; and long
// sentences of 14 to 20 words that link nothing. No sentence holds a misspelling, and no word of
// the misspellings is a linking or joining word.
func TestTopics(t *testing.T) {
	if len(topics) < 20 {
		t.Errorf("%d topics, and there are 20 at least", len(topics))
	}
	sentence := func(s string) bool {
		words := strings.Fields(s)
		for j, w := range words[1:] {
			if w != "I" && !strings.HasPrefix(w, "I'") && strings.IndexFunc(w, unicode.IsUpper) >= 0 {
				return false
			}
			if j < len(words)-2 && strings.ContainsAny(w, ".!?") {
				return false
			}
		}
		return strings.HasSuffix(s, ".") && len(sample.Sentences(s)) == 1 && unicode.IsUpper([]rune(s)[0])
	}
	seen := map[string]bool{}
	for _, tp := range topics {
		if seen[tp.statement] || seen[tp.title] {
			t.Errorf("%q or %q comes twice", tp.statement, tp.title)
		}
		seen[tp.statement], seen[tp.title] = true, true
		var plainCount int
		var links []string
		for i, s := range tp.body {
			link := sample.LinkingWords(s)
			switch {
			case !sentence(s):
				t.Errorf("%s: %q is no sentence of an essay", tp.title, s)
			case plain(s):
				plainCount++
			case len(link) == 0:
				t.Errorf("%s: %q holds a joining word but no linking word", tp.title, s)
			case i == 0:
				t.Errorf("%s: the first sentence %q holds a linking word", tp.title, s)
			case strings.HasPrefix(s, "However"):
				t.Errorf("%s: %q begins with however, which merge cannot join", tp.title, s)
			}
			for _, w := range link {
				if !slices.Contains(links, w) {
					links = append(links, w)
				}
			}
		}
		if plainCount < 7 || len(tp.body)-plainCount < 4 || len(links) < 3 {
			t.Errorf("%s: %d sentences link nothing and %d link with %v", tp.title, plainCount, len(tp.body)-plainCount, links)
		}
		if len(tp.long) < 4 {
			t.Errorf("%s: %d long sentences", tp.title, len(tp.long))
		}
		for _, s := range tp.long {
			if n := len(sample.Words(s)); !sentence(s) || !plain(s) || n < 14 || n > 20 {
				t.Errorf("%s: %q is no long sentence that links nothing, of 14 to 20 words", tp.title, s)
			}
		}
		for _, s := range slices.Concat(tp.body, tp.long, []string{tp.title}) {
			for _, w := range sample.Words(s) {
				for _, wrong := range misspellings {
					if strings.EqualFold(w, wrong) {
						t.Errorf("%s: %q holds the misspelling %q", tp.title, s, wrong)
					}
				}
			}
		}
	}
	for word, wrong := range misspellings {
		if wrong == word || !plain(word) || !plain(wrong) || strings.ToLower(wrong) != wrong {
			t.Errorf("the misspelling %q of %q", wrong, word)
		}
	}
}

// An essay off its topic takes the title and the sentences of another topic, and an essay on it its
// own; the text of every essay begins with the first sentence of its body, with its misspellings
// read right.
func TestOffTopic(t *testing.T) {
	es, err := Essays(setSize, setSeed)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range es.Essays {
		text := e.Text
		for _, m := range e.Misspelled {
			text = strings.ReplaceAll(text, m.Written, m.Word)
			text = strings.ReplaceAll(text, strings.ToUpper(m.Written[:1])+m.Written[1:], strings.ToUpper(m.Word[:1])+m.Word[1:])
		}
		from := -1
		for i, tp := range topics {
			if strings.HasPrefix(text, strings.TrimSuffix(tp.body[0], ".")) || strings.HasPrefix(text, strings.TrimSuffix(tp.long[0], ".")) {
				from = i
			}
		}
		if from < 0 {
			t.Fatalf("essay %d begins with no first sentence of a topic: %q", e.Number, e.Text)
		}
		off := slices.Contains(e.Unmet, sample.CriterionOnTopic)
		if (topics[from].statement != e.Topic) != off || e.Title != nil && *e.Title != topics[from].title {
			t.Errorf("essay %d, on %q, has the title %v and the text of %q", e.Number, e.Topic, e.Title, topics[from].title)
		}
	}
}

// Sentences merged become so many, the second of a pair joined by ", and" or, after but, so or
// because, by a comma; and too few or too many to merge are none.
func TestMerge(t *testing.T) {
	ss := []string{"The sun is up.", "But the sea is cold.", "I swim.", "Its waves are high.", "We go home."}
	got := merge(rand.New(rand.NewPCG(1, 1)), ss, 3)
	if len(got) != 3 || strings.Join(got, " ") == strings.Join(ss, " ") {
		t.Fatalf("merge = %q", got)
	}
	for _, s := range got {
		if len(sample.Sentences(s)) != 1 {
			t.Errorf("merge gives %q", s)
		}
	}
	if got := joined("The sun is up.", "But the sea is cold."); got != "The sun is up, but the sea is cold." {
		t.Errorf("joined = %q", got)
	}
	if got := joined("The sun is up.", "I swim."); got != "The sun is up, and I swim." {
		t.Errorf("joined = %q", got)
	}
	if merge(rand.New(rand.NewPCG(1, 1)), ss[:2], 3) != nil || merge(rand.New(rand.NewPCG(1, 1)), ss, 2) != nil {
		t.Error("merge merges sentences it cannot")
	}
}

// Capitals leave a sentence but the first beginning with a small letter a hand writes in another
// shape, or the last without its stop; never I, and never a letter like its capital.
func TestCapitals(t *testing.T) {
	ss := []string{"The sun is up.", "Our boat is red.", "I swim.", "Its waves are high.", "Every fish hides."}
	lowered, unended := 0, 0
	for seed := range uint64(50) {
		got := capitals(rand.New(rand.NewPCG(seed, 1)), ss)
		text := strings.Join(got, " ")
		starts := sample.LowerStarts(text)
		if sample.Unended(text) {
			unended++
		} else if len(starts) == 1 || len(starts) == 2 {
			lowered++
		} else {
			t.Fatalf("capitals = %q", got)
		}
		for _, i := range starts {
			if i == 1 || i == 2 || i == 3 {
				t.Errorf("capitals lowers sentence %d: %q", i, got)
			}
		}
	}
	if lowered == 0 || unended == 0 {
		t.Errorf("capitals lowered %d and left %d unended", lowered, unended)
	}
	if got := capitals(rand.New(rand.NewPCG(1, 1)), []string{"I swim.", "Our boat is red."}); !sample.Unended(strings.Join(got, " ")) {
		t.Errorf("capitals with no sentence to lower gives %q", got)
	}
	if got := lowerFirst("I'm here."); got != "I'm here." {
		t.Errorf("lowerFirst = %q", got)
	}
}

// Misspell spells two or three words of the list wrong, each once in the text, keeping its capital
// and the mark after it, and lists them in the order they come; a text of fewer than two such words
// it leaves.
func TestMisspell(t *testing.T) {
	text := "Every morning the little robot is happy, but the kitchen is quiet. The robot is happy."
	for seed := range uint64(20) {
		got, wrong, ok := misspell(rand.New(rand.NewPCG(seed, 1)), text)
		if !ok || len(wrong) < 2 || len(wrong) > 3 || len(strings.Fields(got)) != len(strings.Fields(text)) {
			t.Fatalf("misspell = %q, %v, %v", got, wrong, ok)
		}
		at := -1
		for _, m := range wrong {
			if m.Word == "happy" || misspellings[m.Word] != m.Written {
				t.Errorf("misspell gives %v", m)
			}
			i := strings.Index(strings.ToLower(got), m.Written)
			if i < at {
				t.Errorf("misspell lists %v out of order", wrong)
			}
			at = i
		}
		if strings.Contains(got, "evry") || !strings.HasSuffix(got, "happy.") {
			t.Errorf("misspell = %q", got)
		}
	}
	if _, _, ok := misspell(rand.New(rand.NewPCG(1, 1)), "The robot is happy. The robot is happy."); ok {
		t.Error("misspell spells wrong a word the text holds twice")
	}
}
