package sample

import (
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

// EssaysFormat is the name a file of the essays declares in its "format" field.
const EssaysFormat = "tutor-essays"

// Essays is the synthetic essays of the measurement of the assessment by a rubric (T82;
// docs/architecture.md, 3.4, «Сочинения»): one rubric, the criteria a module's task with a rubric
// would give (spec's format, section 15), and short essays in English, each with its topic, its
// title and its text, and which criteria of the rubric it does not meet, as planned. Code draws each
// essay in memory from a seed (synth.Essay): no file of a page is written.
type Essays struct {
	Format  string      `json:"format"`
	Version *int        `json:"version"`
	Rubric  []Criterion `json:"rubric"`
	Essays  []Essay     `json:"essays"`
}

// Criterion is a criterion of the rubric: its ID, and its text, as the module gives it to a model.
type Criterion struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// The criteria of the rubric, by ID. The program counts whether an essay meets each of them but
// CriterionOnTopic, whose truth is the plan's and the author's reading of it.
const (
	CriterionTitle     = "title"
	CriterionSentences = "sentences"
	CriterionCapitals  = "capitals-and-stops"
	CriterionLinking   = "linking-words"
	CriterionOnTopic   = "on-topic"
	CriterionSpelling  = "spelling"
)

// Criteria are the IDs of the rubric's criteria, in its order.
var Criteria = []string{CriterionTitle, CriterionSentences, CriterionCapitals, CriterionLinking, CriterionOnTopic, CriterionSpelling}

// Essay is one essay: its place among the essays, from 1; the part of the measurement it belongs to,
// PartDev or PartTest; the topic of its task, as the task's statement gives it; its title, nil when
// it has none; its text, in one line, which code breaks into lines; the criteria it does not meet,
// in the order of the rubric; and the common words it spells wrong.
type Essay struct {
	Number     int           `json:"number"`
	Part       string        `json:"part"`
	Topic      string        `json:"topic"`
	Title      *string       `json:"title"`
	Text       string        `json:"text"`
	Unmet      []string      `json:"unmet"`
	Misspelled []Misspelling `json:"misspelled"`
}

// Misspelling is a common word spelled wrong: as it is written, and the word it is.
type Misspelling struct {
	Written string `json:"written"`
	Word    string `json:"word"`
}

// The bounds of an essay, so that whether it meets a criterion is no matter of opinion: its words,
// its title's, and its sentences — at least LeastSentences when it meets the criterion of sentences,
// and fewer, but not much fewer, when it does not.
const (
	MinWords       = 40
	MaxWords       = 80
	MaxTitleWords  = 6
	LeastSentences = 5
	MinSentences   = 3
	MaxSentences   = 9
	MinMisspelled  = 2
	MaxMisspelled  = 3
)

// linking are the linking words of the criterion of linking words: an essay meets it with two of
// them at least. joining are more words that join ideas: an essay that does not meet the criterion
// has none of either, and an essay between the two is refused, since an adult might judge it
// either way.
var (
	linking = []string{"because", "but", "so", "then", "first", "next", "finally", "also", "however", "later", "when"}
	joining = []string{"and", "or", "after", "before", "while", "until", "if", "although", "since"}
)

// distinctLower are the small letters whose capital a hand writes in another shape, not only
// taller: a sentence that begins with a small letter, against the criterion of capitals and stops,
// begins with one of them, so that a reader can tell it.
const distinctLower = "abdefghimnqrty"

// The words of the essays: letters, with an apostrophe inside, as "don't", and a comma or an end
// mark after a word of a text.
var (
	textWord  = regexp.MustCompile(`^[A-Za-z]+('[A-Za-z]+)?[,.!?]?$`)
	titleWord = regexp.MustCompile(`^[A-Za-z]+('[A-Za-z]+)?$`)
	lowerWord = regexp.MustCompile(`^[a-z]+$`)
)

// Sentences are the sentences of a text: each ends at a full stop, a question mark or an
// exclamation mark that a space or the end of the text follows, and what follows the last such mark
// is a sentence too.
func Sentences(text string) []string {
	var out []string
	rs := []rune(text)
	start := 0
	for i, r := range rs {
		if strings.ContainsRune(".!?", r) && (i+1 == len(rs) || rs[i+1] == ' ') {
			out = append(out, strings.TrimSpace(string(rs[start:i+1])))
			start = i + 1
		}
	}
	if rest := strings.TrimSpace(string(rs[start:])); rest != "" {
		out = append(out, rest)
	}
	return out
}

// Words are the words of a text, as written, without the commas and end marks after them.
func Words(text string) []string {
	var out []string
	for _, f := range strings.Fields(text) {
		if w := strings.TrimRight(f, ",.!?"); w != "" {
			out = append(out, w)
		}
	}
	return out
}

// LinkingWords are the distinct linking words of the criterion that a text holds, in small letters,
// in the order they first come.
func LinkingWords(text string) []string { return holds(text, linking) }

// holds are the distinct words of a list that a text holds, in small letters, in the order they
// first come.
func holds(text string, list []string) []string {
	var out []string
	for _, w := range Words(text) {
		if w = strings.ToLower(w); slices.Contains(list, w) && !slices.Contains(out, w) {
			out = append(out, w)
		}
	}
	return out
}

// LowerStarts are the sentences of a text, by their place from 1, that begin with a small letter.
func LowerStarts(text string) []int {
	var out []int
	for i, s := range Sentences(text) {
		if r := []rune(s)[0]; unicode.IsLower(r) {
			out = append(out, i+1)
		}
	}
	return out
}

// Unended is whether the last sentence of a text ends without a full stop, a question mark or an
// exclamation mark: the only sentence that can, since a mark missing inside a text joins two
// sentences into one.
func Unended(text string) bool {
	ss := Sentences(text)
	return len(ss) > 0 && !strings.ContainsAny(ss[len(ss)-1][len(ss[len(ss)-1])-1:], ".!?")
}

// Met is whether an essay meets each criterion the program counts, by ID: CriterionOnTopic is not
// among them. It counts the essay as it stands; Check refuses an essay whose count an adult might
// read otherwise.
func (e *Essay) Met() map[string]bool {
	return map[string]bool{
		CriterionTitle:     e.Title != nil,
		CriterionSentences: len(Sentences(e.Text)) >= LeastSentences,
		CriterionCapitals:  len(LowerStarts(e.Text)) == 0 && !Unended(e.Text),
		CriterionLinking:   len(LinkingWords(e.Text)) >= 2,
		CriterionSpelling:  len(e.Misspelled) == 0,
	}
}

// Tally counts, for each criterion of the rubric by ID, the essays of the development and of the test
// that do not meet it, and under "" the essays that meet them all.
func (es *Essays) Tally() map[string][2]int {
	n := map[string][2]int{}
	for _, e := range es.Essays {
		part := 0
		if e.Part == PartTest {
			part = 1
		}
		ids := e.Unmet
		if len(ids) == 0 {
			ids = []string{""}
		}
		for _, id := range ids {
			c := n[id]
			c[part]++
			n[id] = c
		}
	}
	return n
}

// ReadEssays reads the essays from JSON and checks them. An error from a file that fails the check
// is a *FormatError.
func ReadEssays(r io.Reader) (*Essays, error) {
	var es Essays
	if err := readStrict(r, "essays", &es); err != nil {
		return nil, readError("essays", err)
	}
	if err := es.Check(); err != nil {
		return nil, err
	}
	return &es, nil
}

// Check checks the essays and returns a *FormatError listing every problem, or nil: the rubric, the
// criteria of Criteria in their order; the essays numbered from 1, those of the development first;
// each essay written as the essays are, within their bounds, without a name; and every criterion
// the program counts unmet as the essay says, and clearly so.
func (es *Essays) Check() error {
	p := &problems{what: "essays"}
	checkHeader(p, es.Format, EssaysFormat, es.Version)
	ids := make([]string, len(es.Rubric))
	for i, c := range es.Rubric {
		ids[i] = c.ID
		if strings.TrimSpace(c.Text) == "" {
			p.add(fmt.Sprintf("rubric[%d].text", i), "is empty")
		}
	}
	if !slices.Equal(ids, Criteria) {
		p.add("rubric", "names the criteria %v, and the program knows %v, in this order", ids, Criteria)
	}
	if len(es.Essays) == 0 {
		p.add("essays", "are missing: there is at least one essay")
	}
	dev := true
	for i, e := range es.Essays {
		path := fmt.Sprintf("essays[%d]", i)
		if e.Number != i+1 {
			p.add(path+".number", "is %d, but the essay is number %d", e.Number, i+1)
		}
		switch {
		case e.Part != PartDev && e.Part != PartTest:
			p.add(path+".part", "is %q, not %q or %q", e.Part, PartDev, PartTest)
		case e.Part == PartDev && !dev:
			p.add(path+".part", "is %q after an essay of %q: the essays of the development come first", e.Part, PartTest)
		}
		if e.Part == PartTest {
			dev = false
		}
		checkEssay(p, path, &e)
	}
	return p.err()
}

func checkEssay(p *problems, path string, e *Essay) {
	if !strings.HasSuffix(e.Topic, ".") || !wordsMatch(e.Topic, textWord) {
		p.add(path+".topic", "%q is not written as the essays are: words of letters, ending with a full stop", e.Topic)
	} else {
		checkNames(p, path+".topic", e.Topic)
	}
	if e.Title != nil {
		n := len(strings.Fields(*e.Title))
		if !wordsMatch(*e.Title, titleWord) || n < 1 || n > MaxTitleWords || !unicode.IsUpper([]rune(*e.Title)[0]) {
			p.add(path+".title", "%q is no title: 1 to %d words of letters, from a capital letter, with no mark at the end", *e.Title, MaxTitleWords)
		}
	}
	if !wordsMatch(e.Text, textWord) {
		p.add(path+".text", "is not written as the essays are: words of letters, an apostrophe inside, a comma or an end mark after, one space between")
		return
	}
	if n := len(Words(e.Text)); n < MinWords || n > MaxWords {
		p.add(path+".text", "has %d words, and an essay %d to %d", n, MinWords, MaxWords)
	}
	checkNames(p, path+".text", e.Text)
	unmet := checkUnmet(p, path, e.Unmet)
	met := e.Met()
	for _, id := range Criteria {
		if id == CriterionOnTopic {
			continue
		}
		if met[id] == unmet[id] {
			p.add(path+".unmet", "%s the criterion %s, but the essay as written %s it", map[bool]string{true: "lists", false: "does not list"}[unmet[id]], id, map[bool]string{true: "meets", false: "does not meet"}[met[id]])
		}
	}
	checkClear(p, path, e)
	checkMisspelled(p, path, e)
}

// wordsMatch is whether a text is words each of which matches the pattern, one space between them.
func wordsMatch(text string, word *regexp.Regexp) bool {
	fields := strings.Fields(text)
	if len(fields) == 0 || strings.Join(fields, " ") != text {
		return false
	}
	for _, f := range fields {
		if !word.MatchString(f) {
			return false
		}
	}
	return true
}

// checkNames refuses a capital letter inside a sentence but that of I, so that no text holds a name
// (privacy П-20): a capital starts a sentence, or is I. A title is written with capitals, and a name
// in it only the author's reading finds.
func checkNames(p *problems, path, text string) {
	for i, s := range Sentences(text) {
		for j, w := range Words(s) {
			first := j == 0 || w == "I" || strings.HasPrefix(w, "I'")
			for k, r := range []rune(w) {
				if unicode.IsUpper(r) && (k > 0 || !first) {
					p.add(path, "sentence %d has a capital letter inside, in %q: an essay names no one, and a capital starts a sentence or is I", i+1, w)
					break
				}
			}
		}
	}
}

// checkUnmet checks the criteria an essay lists unmet: each of the rubric, once, in its order. It
// returns them as a set.
func checkUnmet(p *problems, path string, ids []string) map[string]bool {
	set := map[string]bool{}
	last := -1
	for i, id := range ids {
		at := slices.Index(Criteria, id)
		switch {
		case at < 0:
			p.add(fmt.Sprintf("%s.unmet[%d]", path, i), "%q is no criterion of the rubric", id)
		case at <= last:
			p.add(fmt.Sprintf("%s.unmet[%d]", path, i), "%q is out of the order of the rubric, or twice", id)
		default:
			last = at
		}
		set[id] = true
	}
	return set
}

// checkClear refuses an essay whose count of a criterion an adult might read otherwise: sentences
// fewer than MinSentences or more than MaxSentences, or, met, fewer than LeastSentences; a sentence
// that begins with a small letter a hand writes like its capital; and linking words neither two at
// least nor none, with no joining word either.
func checkClear(p *problems, path string, e *Essay) {
	if n := len(Sentences(e.Text)); n < MinSentences || n > MaxSentences {
		p.add(path+".text", "has %d sentences, and an essay %d to %d", n, MinSentences, MaxSentences)
	}
	ss := Sentences(e.Text)
	for _, i := range LowerStarts(e.Text) {
		if r := []rune(ss[i-1])[0]; !strings.ContainsRune(distinctLower, r) {
			p.add(path+".text", "sentence %d begins with %q, which a hand writes like its capital: a small letter that begins a sentence is one of %s", i, r, distinctLower)
		}
	}
	link, join := LinkingWords(e.Text), holds(e.Text, joining)
	if len(link) == 1 || len(link) == 0 && len(join) > 0 {
		p.add(path+".text", "has the linking words %v and the joining words %v: an essay has two linking words at least, or none of either", link, join)
	}
}

// checkMisspelled checks the words an essay spells wrong: MinMisspelled to MaxMisspelled of them
// when it does not meet the criterion of spelling, each a word of small letters other than the word
// it is, written so in the text, and no linking or joining word.
func checkMisspelled(p *problems, path string, e *Essay) {
	if n := len(e.Misspelled); n > 0 && (n < MinMisspelled || n > MaxMisspelled) {
		p.add(path+".misspelled", "are %d, and an essay spells %d to %d words wrong, or none", n, MinMisspelled, MaxMisspelled)
	}
	words := map[string]bool{}
	for _, w := range Words(e.Text) {
		words[strings.ToLower(w)] = true
	}
	for i, m := range e.Misspelled {
		mp := fmt.Sprintf("%s.misspelled[%d]", path, i)
		switch {
		case !lowerWord.MatchString(m.Written) || !lowerWord.MatchString(m.Word):
			p.add(mp, "%q for %q: a misspelling and its word are small letters", m.Written, m.Word)
		case m.Written == m.Word:
			p.add(mp, "%q is the word itself", m.Written)
		case !words[m.Written]:
			p.add(mp+".written", "%q is not a word of the text", m.Written)
		case slices.Contains(linking, m.Word) || slices.Contains(joining, m.Word):
			p.add(mp+".word", "%q is a linking or joining word, and those are spelled right", m.Word)
		}
	}
}
