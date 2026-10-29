package gen

import (
	"cmp"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/altnovatrail/tutor/internal/sample"
)

// rubric is the rubric of E4's essays, its criteria and their texts as eval/essays.json gives them
// (R44), so that a run on the essays built here measures the same criteria.
var rubric = []sample.Criterion{
	{ID: sample.CriterionTitle, Text: "It has a title on its own first line."},
	{ID: sample.CriterionSentences, Text: "It has at least five sentences."},
	{ID: sample.CriterionCapitals, Text: "Every sentence begins with a capital letter and ends with a full stop, a question mark or an exclamation mark."},
	{ID: sample.CriterionLinking, Text: "It links its ideas with words such as because, but, so, then or first."},
	{ID: sample.CriterionOnTopic, Text: "It keeps to the topic of the task."},
	{ID: sample.CriterionSpelling, Text: "Common words are spelled correctly."},
}

// The shares of the essays by how many criteria each leaves unmet, as in E4 (R44): a fifth meet
// them all, a fifth leave three unmet, and the rest one or two, half and half. Each criterion is
// then unmet in a quarter of the essays.
const (
	shareAllMet = 0.2
	shareThree  = 0.2
)

// tries is how many times an essay is put together again from other random choices before Essays
// gives up on it.
const tries = 200

// Essays builds essays on the rubric of E4, from the seed: the first devCount of them for the
// development, and the rest for the test. In each part the essays leave unmet one criterion in a
// quarter of the essays each, a fifth of them none and the rest one to three (unmetSets), and the
// topics come by turns, each once before any comes again. An essay is put together from the
// sentences of its topic, and changed for each criterion it does not meet (essayOf). The essays pass
// their check, or Essays returns an error.
func Essays(essays int, seed uint64) (*sample.Essays, error) {
	if essays < 1 {
		return nil, fmt.Errorf("gen: there is at least one essay, not %d", essays)
	}
	rng := newRand(seed, essaysStream)
	dev := devCount(essays)
	sets := append(unmetSets(rng, dev), unmetSets(rng, essays-dev)...)
	v := sample.Version
	es := &sample.Essays{Format: sample.EssaysFormat, Version: &v, Rubric: slices.Clone(rubric), Essays: make([]sample.Essay, essays)}
	var order []int
	for i, unmet := range sets {
		if len(order) == 0 {
			order = rng.Perm(len(topics))
		}
		e, err := essayOf(rng, order[0], unmet)
		if err != nil {
			return nil, fmt.Errorf("gen: essay %d: %w", i+1, err)
		}
		order = order[1:]
		e.Number, e.Part = i+1, partOf(i, essays)
		es.Essays[i] = e
	}
	if err := es.Check(); err != nil {
		return nil, err
	}
	return es, nil
}

// unmetSets are the criteria each of n essays leaves unmet, in the order of the rubric, in a random
// order of the essays: a fifth of them none, a fifth three, and the rest one or two. The criteria go
// out as evenly as they can, a quarter of the essays each: the essays of three take theirs first,
// and each essay the criteria left most often, the ties broken at random, so the counts of any two
// never differ by more than one.
func unmetSets(rng *rand.Rand, n int) [][]string {
	three := int(float64(n)*shareThree + 0.5)
	none := int(float64(n)*shareAllMet + 0.5)
	rest := n - three - none
	two := rest / 2
	one := rest - two
	sizes := slices.Concat(slices.Repeat([]int{3}, three), slices.Repeat([]int{2}, two), slices.Repeat([]int{1}, one), slices.Repeat([]int{0}, none))
	total := 3*three + 2*two + one
	left := make(map[string]int, len(sample.Criteria))
	for i, at := range rng.Perm(len(sample.Criteria)) {
		left[sample.Criteria[at]] = total / len(sample.Criteria)
		if i < total%len(sample.Criteria) {
			left[sample.Criteria[at]]++
		}
	}
	sets := make([][]string, n)
	for i, size := range sizes {
		ids := slices.Clone(sample.Criteria)
		rng.Shuffle(len(ids), func(a, b int) { ids[a], ids[b] = ids[b], ids[a] })
		slices.SortStableFunc(ids, func(a, b string) int { return cmp.Compare(left[b], left[a]) })
		chosen := ids[:size]
		for _, id := range chosen {
			left[id]--
		}
		sets[i] = make([]string, 0, size)
		for _, id := range sample.Criteria {
			if slices.Contains(chosen, id) {
				sets[i] = append(sets[i], id)
			}
		}
	}
	rng.Shuffle(n, func(a, b int) { sets[a], sets[b] = sets[b], sets[a] })
	return sets
}

// essayOf puts together an essay on a topic that leaves the criteria unmet, from random choices,
// and checks it alone; choices that give no essay within the bounds of the format are made again.
func essayOf(rng *rand.Rand, t int, unmet []string) (sample.Essay, error) {
	v := sample.Version
	for range tries {
		e, ok := compose(rng, t, unmet)
		if !ok {
			continue
		}
		alone := e
		alone.Number, alone.Part = 1, sample.PartDev
		one := sample.Essays{Format: sample.EssaysFormat, Version: &v, Rubric: rubric, Essays: []sample.Essay{alone}}
		if one.Check() == nil {
			return e, nil
		}
	}
	return sample.Essay{}, fmt.Errorf("no essay on %q leaves unmet %v alone", topics[t].title, unmet)
}

// alikeTopics are pairs of words of the statements of topics so near that an essay on one could be
// judged to keep to the other: the hedgehog's post office and the owl that delivers letters both carry
// letters through the woods, and the ship on the clouds and the river that flows uphill are both
// trips on a boat. Neither of a pair is the other's off-topic body.
var alikeTopics = [][2]string{{"hedgehog", "owl"}, {"ship", "river"}}

// alike is whether two statements are of a pair of alikeTopics.
func alike(a, b string) bool {
	for _, p := range alikeTopics {
		if strings.Contains(a, p[0]) && strings.Contains(b, p[1]) || strings.Contains(a, p[1]) && strings.Contains(b, p[0]) {
			return true
		}
	}
	return false
}

// compose puts together an essay on a topic that leaves the criteria unmet:
//   - on-topic: the title and the body are another topic's, and the statement this one's;
//   - sentences: three or four sentences, pairs of the body merged into one (merge), or long
//     sentences of the topic when linking words are unmet too;
//   - linking-words: sentences of the body with no linking or joining word alone;
//   - capitals-and-stops: one or two sentences but the first begin with a small letter, or the last
//     ends without its stop (capitals);
//   - spelling: two or three common words of the text spelled wrong (misspell);
//   - title: no title.
//
// It reports false when its choices give no essay of MinWords to MaxWords words, or one that lacks
// what a change needs.
func compose(rng *rand.Rand, t int, unmet []string) (sample.Essay, bool) {
	miss := func(id string) bool { return slices.Contains(unmet, id) }
	body := t
	if miss(sample.CriterionOnTopic) {
		var others []int
		for i := range topics {
			if i != t && !alike(topics[i].statement, topics[t].statement) {
				others = append(others, i)
			}
		}
		body = others[rng.IntN(len(others))]
	}
	tp := topics[body]
	pool := tp.body
	if miss(sample.CriterionLinking) {
		pool = nil
		for _, s := range tp.body {
			if plain(s) {
				pool = append(pool, s)
			}
		}
	}
	var ss []string
	switch {
	case miss(sample.CriterionSentences) && miss(sample.CriterionLinking):
		ss = pick(rng, tp.long, between(rng, 3, 4))
	case miss(sample.CriterionSentences):
		groups := between(rng, 3, 4)
		ss = merge(rng, pick(rng, pool, between(rng, groups+2, 2*groups)), groups)
	default:
		ss = pick(rng, pool, between(rng, sample.LeastSentences, 8))
	}
	if ss == nil {
		return sample.Essay{}, false
	}
	text := strings.Join(ss, " ")
	if n := len(sample.Words(text)); n < sample.MinWords || n > sample.MaxWords {
		return sample.Essay{}, false
	}
	if !miss(sample.CriterionLinking) && len(sample.LinkingWords(text)) < 2 {
		return sample.Essay{}, false
	}
	if miss(sample.CriterionCapitals) {
		ss = capitals(rng, ss)
		text = strings.Join(ss, " ")
	}
	e := sample.Essay{Topic: topics[t].statement, Text: text, Unmet: slices.Clone(unmet), Misspelled: []sample.Misspelling{}}
	if miss(sample.CriterionSpelling) {
		var ok bool
		if e.Text, e.Misspelled, ok = misspell(rng, text); !ok {
			return sample.Essay{}, false
		}
	}
	if !miss(sample.CriterionTitle) {
		title := tp.title
		e.Title = &title
	}
	return e, true
}

// plain is whether a sentence holds no linking word and no joining word.
func plain(s string) bool {
	return len(sample.LinkingWords(s)) == 0 && len(sample.JoiningWords(s)) == 0
}

// pick is k sentences of a list in their order, the first always among them, since it names the
// subject, and the others at random; nil when the list has fewer than k.
func pick(rng *rand.Rand, list []string, k int) []string {
	if k < 1 || k > len(list) {
		return nil
	}
	at := rng.Perm(len(list) - 1)[:k-1]
	slices.Sort(at)
	out := []string{list[0]}
	for _, i := range at {
		out = append(out, list[i+1])
	}
	return out
}

// merge merges sentences into so many: pairs that follow each other, chosen at random, become one
// sentence, the second joined to the first by ", and", or by a comma alone when it begins with
// but, so or because.
func merge(rng *rand.Rand, ss []string, groups int) []string {
	pairs := len(ss) - groups
	if ss == nil || pairs < 0 || pairs > groups {
		return nil
	}
	paired := make([]bool, groups)
	for _, g := range rng.Perm(groups)[:pairs] {
		paired[g] = true
	}
	out := make([]string, 0, groups)
	i := 0
	for g := range groups {
		s := ss[i]
		i++
		if paired[g] {
			s = joined(s, ss[i])
			i++
		}
		out = append(out, s)
	}
	return out
}

// joined is two sentences made one.
func joined(a, b string) string {
	first, _, _ := strings.Cut(b, " ")
	join := ", and "
	if slices.Contains([]string{"But", "So", "Because"}, first) {
		join = ", "
	}
	return strings.TrimSuffix(a, ".") + join + lowerFirst(b)
}

// lowerFirst is a sentence that begins with a small letter, but for I, which stays.
func lowerFirst(s string) string {
	first, _, _ := strings.Cut(s, " ")
	if first == "I" || strings.HasPrefix(first, "I'") {
		return s
	}
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToLower(r)) + s[n:]
}

// capitals breaks the criterion of capitals and stops: half the time one sentence but the first
// begins with a small letter, a fifth of the time two, and otherwise, or when the sentences have
// too few that can, the last ends without its stop. A sentence that can begin with a small letter
// begins with a capital whose small letter a hand writes in another shape, sample.DistinctLower,
// and not with I.
func capitals(rng *rand.Rand, ss []string) []string {
	var can []int
	for i, s := range ss[1:] {
		first, _, _ := strings.Cut(s, " ")
		r, _ := utf8.DecodeRuneInString(s)
		if first != "I" && !strings.HasPrefix(first, "I'") && unicode.IsUpper(r) && strings.ContainsRune(sample.DistinctLower, unicode.ToLower(r)) {
			can = append(can, i+1)
		}
	}
	out := slices.Clone(ss)
	lower := 0
	switch x := rng.Float64(); {
	case x < 0.5:
		lower = 1
	case x < 0.7:
		lower = 2
	}
	if lower == 0 || len(can) < lower {
		out[len(out)-1] = strings.TrimSuffix(out[len(out)-1], ".")
		return out
	}
	rng.Shuffle(len(can), func(a, b int) { can[a], can[b] = can[b], can[a] })
	for _, i := range can[:lower] {
		out[i] = lowerFirst(out[i])
	}
	return out
}

// misspell spells two or three common words of a text wrong, each a word of misspellings that the
// text holds once, chosen at random: the text with them so written, and the words in the order they
// come. It reports false when the text holds fewer than two such words.
func misspell(rng *rand.Rand, text string) (string, []sample.Misspelling, bool) {
	fields := strings.Fields(text)
	count := map[string]int{}
	for _, f := range fields {
		count[strings.ToLower(strings.TrimRight(f, ",.!?"))]++
	}
	var can []int
	for i, f := range fields {
		w := strings.ToLower(strings.TrimRight(f, ",.!?"))
		if _, ok := misspellings[w]; ok && count[w] == 1 {
			can = append(can, i)
		}
	}
	if len(can) < sample.MinMisspelled {
		return "", nil, false
	}
	n := min(between(rng, sample.MinMisspelled, sample.MaxMisspelled), len(can))
	rng.Shuffle(len(can), func(a, b int) { can[a], can[b] = can[b], can[a] })
	at := can[:n]
	slices.Sort(at)
	var out []sample.Misspelling
	for _, i := range at {
		f := fields[i]
		w := strings.TrimRight(f, ",.!?")
		word := strings.ToLower(w)
		wrong := misspellings[word]
		written := wrong
		if r, _ := utf8.DecodeRuneInString(w); unicode.IsUpper(r) {
			written = strings.ToUpper(wrong[:1]) + wrong[1:]
		}
		fields[i] = written + f[len(w):]
		out = append(out, sample.Misspelling{Written: wrong, Word: word})
	}
	return strings.Join(fields, " "), out, true
}
