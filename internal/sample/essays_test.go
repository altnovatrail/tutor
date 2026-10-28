package sample_test

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/internal/sample"
)

// testRubric is the rubric of the measurement, its criteria in their order.
func testRubric() []sample.Criterion {
	rubric := make([]sample.Criterion, len(sample.Criteria))
	for i, id := range sample.Criteria {
		rubric[i] = sample.Criterion{ID: id, Text: "The text of " + id + "."}
	}
	return rubric
}

// testEssays are two essays: one that meets every criterion, and one with no title, four sentences,
// a small letter at the start of one and two words spelled wrong, off its topic too.
func testEssays() *sample.Essays {
	v := sample.Version
	return &sample.Essays{Format: sample.EssaysFormat, Version: &v, Rubric: testRubric(), Essays: []sample.Essay{
		{Number: 1, Part: sample.PartDev, Topic: "Imagine a robot that bakes bread. Write about its day.", Title: ptr("The Bread Robot"),
			Text: "The robot wakes up at four because the ovens need time to get hot. First it mixes flour, water and salt in a big bowl. " +
				"Then it shapes the dough into long loaves. The kitchen smells sweet and warm. The robot is tired, but it is happy.",
			Unmet: []string{}, Misspelled: []sample.Misspelling{}},
		{Number: 2, Part: sample.PartTest, Topic: "Imagine a town under the sea. Write about what you see there.",
			Text: "The bicycle race starts at nine in the morning, when the streets are closed. Hundreds of riders wait at the line, but the wind is strong. " +
				"the crowd cheers loudly as the riders climb a steep hill near the river. I can't say how fast the winnr is, he is very quik.",
			Unmet:      []string{sample.CriterionTitle, sample.CriterionSentences, sample.CriterionCapitals, sample.CriterionOnTopic, sample.CriterionSpelling},
			Misspelled: []sample.Misspelling{{Written: "winnr", Word: "winner"}, {Written: "quik", Word: "quick"}}},
	}}
}

func TestReadEssays(t *testing.T) {
	data, err := sample.Marshal(testEssays())
	if err != nil {
		t.Fatal(err)
	}
	es, err := sample.ReadEssays(strings.NewReader(string(data)))
	if err != nil || len(es.Essays) != 2 || es.Essays[1].Title != nil || es.Essays[1].Misspelled[1].Word != "quick" {
		t.Fatalf("ReadEssays = %+v, %v", es, err)
	}
	broken := strings.Replace(string(data), `"title": null,`, "", 1)
	_, err = sample.ReadEssays(strings.NewReader(broken))
	has(t, err, "essays[1].title", "is missing")
}

// The essays of the measurement, eval/essays.json, read and pass the check: 20 essays, the first
// five of the development. Each criterion is unmet in one essay of the development and in four of the
// test, and one essay of the development and three of the test meet them all (T82).
func TestTheEssays(t *testing.T) {
	f, err := os.Open("../../eval/essays.json")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	es, err := sample.ReadEssays(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(es.Essays) != 20 {
		t.Fatalf("%d essays", len(es.Essays))
	}
	unmet := map[string]map[string]int{sample.PartDev: {}, sample.PartTest: {}}
	for _, e := range es.Essays {
		if want := map[bool]string{true: sample.PartDev, false: sample.PartTest}[e.Number <= 5]; e.Part != want {
			t.Errorf("essay %d is of %s", e.Number, e.Part)
		}
		for _, id := range e.Unmet {
			unmet[e.Part][id]++
		}
		if len(e.Unmet) == 0 {
			unmet[e.Part]["none"]++
		}
	}
	for part, want := range map[string]int{sample.PartDev: 1, sample.PartTest: 4} {
		for _, id := range append(slices.Clone(sample.Criteria), "none") {
			if id == "none" {
				want = map[string]int{sample.PartDev: 1, sample.PartTest: 3}[part]
			}
			if unmet[part][id] != want {
				t.Errorf("%s: %d essays of %s leave it unmet, want %d", id, unmet[part][id], part, want)
			}
		}
	}
}

func TestEssaysCheck(t *testing.T) {
	first := func(es *sample.Essays) *sample.Essay { return &es.Essays[0] }
	second := func(es *sample.Essays) *sample.Essay { return &es.Essays[1] }
	tests := []struct {
		name   string
		change func(es *sample.Essays)
		path   string
		want   string
	}{
		{"another format", func(es *sample.Essays) { es.Format = "tutor-workbook" }, "format", `not "tutor-essays"`},
		{"a criterion out of order", func(es *sample.Essays) { es.Rubric[0], es.Rubric[1] = es.Rubric[1], es.Rubric[0] }, "rubric", "in this order"},
		{"a criterion without text", func(es *sample.Essays) { es.Rubric[2].Text = " " }, "rubric[2].text", "is empty"},
		{"no essays", func(es *sample.Essays) { es.Essays = nil }, "essays", "at least one essay"},
		{"an essay out of its place", func(es *sample.Essays) { first(es).Number = 3 }, "essays[0].number", "is number 1"},
		{"no part", func(es *sample.Essays) { first(es).Part = "train" }, "essays[0].part", `not "dev" or "test"`},
		{"development after the test", func(es *sample.Essays) { first(es).Part, second(es).Part = sample.PartTest, sample.PartDev }, "essays[1].part", "come first"},
		{"a topic without a stop", func(es *sample.Essays) { first(es).Topic = "Imagine a robot" }, "essays[0].topic", "ending with a full stop"},
		{"a name in the topic", func(es *sample.Essays) { first(es).Topic = "Imagine a robot called Robo. Write about its day." }, "essays[0].topic", `capital letter inside, in "Robo"`},
		{"a title too long", func(es *sample.Essays) { first(es).Title = ptr("The Robot That Bakes Bread Every Day") }, "essays[0].title", "1 to 6 words"},
		{"a title with a stop", func(es *sample.Essays) { first(es).Title = ptr("The Bread Robot.") }, "essays[0].title", "no mark at the end"},
		{"a title from a small letter", func(es *sample.Essays) { first(es).Title = ptr("the Bread Robot") }, "essays[0].title", "from a capital letter"},
		{"a sign the essays do not write", func(es *sample.Essays) { first(es).Text += " It is \"happy\"." }, "essays[0].text", "not written as the essays are"},
		{"two spaces", func(es *sample.Essays) { first(es).Text = strings.Replace(first(es).Text, " ", "  ", 1) }, "essays[0].text", "one space between"},
		{"too few words", func(es *sample.Essays) {
			first(es).Text = "The robot wakes up. First it mixes flour. Then it bakes. It is warm. The robot is happy, but tired."
		}, "essays[0].text", "has 20 words"},
		{"a name", func(es *sample.Essays) {
			first(es).Text = strings.Replace(first(es).Text, "The kitchen", "The kitchen of Anna", 1)
		}, "essays[0].text", `capital letter inside, in "Anna"`},
		{"a criterion not of the rubric", func(es *sample.Essays) { first(es).Unmet = []string{"neatness"} }, "essays[0].unmet[0]", "no criterion"},
		{"criteria out of order", func(es *sample.Essays) {
			second(es).Unmet[0], second(es).Unmet[1] = second(es).Unmet[1], second(es).Unmet[0]
		}, "essays[1].unmet[1]", "out of the order"},
		{"a title listed unmet", func(es *sample.Essays) { first(es).Unmet = []string{sample.CriterionTitle} }, "essays[0].unmet", "lists the criterion title, but the essay as written meets it"},
		{"few sentences not listed", func(es *sample.Essays) {
			second(es).Unmet = slices.DeleteFunc(second(es).Unmet, func(id string) bool { return id == sample.CriterionSentences })
		}, "essays[1].unmet", "does not list the criterion sentences, but the essay as written does not meet it"},
		{"too few sentences", func(es *sample.Essays) {
			first(es).Text = strings.ReplaceAll(strings.ReplaceAll(first(es).Text, ". First", ", first"), ". Then", ", then")
			first(es).Text = strings.ReplaceAll(first(es).Text, ". The kitchen", ", the kitchen")
		}, "essays[0].text", "has 2 sentences"},
		{"a small letter like its capital", func(es *sample.Essays) {
			second(es).Text = strings.Replace(second(es).Text, "the crowd", "crowds", 1)
		}, "essays[1].text", `begins with 'c'`},
		{"a stop missing at the end", func(es *sample.Essays) { first(es).Text = strings.TrimSuffix(first(es).Text, ".") }, "essays[0].unmet", "the criterion capitals-and-stops"},
		{"one linking word", func(es *sample.Essays) {
			first(es).Text = strings.NewReplacer("because", "as", "First it", "It", "Then it", "It").Replace(first(es).Text)
		}, "essays[0].text", "has the linking words [but]"},
		{"a joining word with no linking word", func(es *sample.Essays) {
			first(es).Text = strings.NewReplacer("because", "as", "First it", "It", "Then it", "It", ", but", ". It").Replace(first(es).Text)
		}, "essays[0].text", "the joining words [and]"},
		{"one word spelled wrong", func(es *sample.Essays) { second(es).Misspelled = second(es).Misspelled[:1] }, "essays[1].misspelled", "are 1"},
		{"a misspelling not in the text", func(es *sample.Essays) { second(es).Misspelled[0].Written = "qwerty" }, "essays[1].misspelled[0].written", "not a word of the text"},
		{"a misspelling that is the word", func(es *sample.Essays) { second(es).Misspelled[0].Written = "winner" }, "essays[1].misspelled[0]", "the word itself"},
		{"a misspelling with a capital", func(es *sample.Essays) { second(es).Misspelled[0].Word = "Winner" }, "essays[1].misspelled[0]", "are small letters"},
		{"a linking word spelled wrong", func(es *sample.Essays) {
			second(es).Text = strings.Replace(second(es).Text, "but the wind", "becaus the wind", 1)
			second(es).Misspelled[0] = sample.Misspelling{Written: "becaus", Word: "because"}
		}, "essays[1].misspelled[0].word", "a linking or joining word"},
		{"spelling listed without misspellings", func(es *sample.Essays) { second(es).Misspelled = nil }, "essays[1].unmet", "the criterion spelling, but the essay as written meets it"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			es := testEssays()
			tt.change(es)
			has(t, es.Check(), tt.path, tt.want)
		})
	}
	if err := testEssays().Check(); err != nil {
		t.Errorf("good essays: %v", err)
	}
	// An essay of no part known is no essay of the test: the development may still follow it.
	es := testEssays()
	es.Essays[0].Part, es.Essays[1].Part = "train", sample.PartDev
	if err := es.Check(); err == nil || strings.Contains(err.Error(), "come first") {
		t.Errorf("an essay of the development after one of no part: %v", err)
	}
}

// The counts the check and the report share: sentences end at a mark a space or the end follows, the
// rest of the text is one more; words are without their commas and marks; linking words are distinct,
// in small letters, in their order.
func TestEssayCounts(t *testing.T) {
	text := "Then it rains. it's cold, so we stay in! Why? Because. then we sleep"
	if got := sample.Sentences(text); !slices.Equal(got, []string{"Then it rains.", "it's cold, so we stay in!", "Why?", "Because.", "then we sleep"}) {
		t.Errorf("Sentences = %q", got)
	}
	if got := sample.Words(text); len(got) != 14 || got[3] != "it's" || got[4] != "cold" {
		t.Errorf("Words = %q", got)
	}
	if got := sample.LinkingWords(text); !slices.Equal(got, []string{"then", "so", "because"}) {
		t.Errorf("LinkingWords = %q", got)
	}
	if got := sample.LowerStarts(text); !slices.Equal(got, []int{2, 5}) {
		t.Errorf("LowerStarts = %v", got)
	}
	if !sample.Unended(text) || sample.Unended("It rains.") || sample.Unended("") {
		t.Error("Unended is wrong")
	}
	met := testEssays().Essays[1].Met()
	if met[sample.CriterionTitle] || met[sample.CriterionSentences] || met[sample.CriterionCapitals] || !met[sample.CriterionLinking] || met[sample.CriterionSpelling] {
		t.Errorf("Met = %v", met)
	}
}
