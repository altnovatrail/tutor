package gen

import (
	_ "embed"
	"encoding/json"
)

// topic is a topic of the essays and the sentences an essay on it is put together from: the
// statement of the task; the title; the sentences of the body, in the order an essay keeps them,
// the first naming the subject; and long sentences that join their ideas with commas alone, for an
// essay of three or four sentences that links nothing. A sentence of the body links its ideas with a
// linking word of the criterion, or holds neither a linking word nor a joining word, such as and
// or if (sample.LinkingWords, sample.JoiningWords): the first kind makes an essay meet the criterion
// of linking words, and the second alone one that does not. The topics are invented and general,
// never the learner's own life, and no sentence names a person or a place (П-20).
type topic struct {
	statement string
	title     string
	body      []string
	long      []string
}

// banks is the file of the sentence banks: the topics and the misspellings. They are data, kept in a
// file of their own beside the code that puts the essays together.
//
//go:embed topics.json
var banks []byte

// bankFile is the file of the sentence banks as it is written.
type bankFile struct {
	Topics []struct {
		Statement string   `json:"statement"`
		Title     string   `json:"title"`
		Body      []string `json:"body"`
		Long      []string `json:"long"`
	} `json:"topics"`
	Misspellings map[string]string `json:"misspellings"`
}

// topics are the topics of the essays. The tests hold every sentence to the rules of the format
// and every topic to enough sentences of each kind.
var topics []topic

// misspellings are the common words an essay may spell wrong, each with the way it is written
// wrong: a slip a child makes, and no other English word, so that the truth of the criterion of
// spelling is the list's. No linking or joining word is among them, as the format wants.
var misspellings map[string]string

// The banks are read once, when the package starts; a file that does not read is a mistake of the
// code, and the package panics on it.
func init() {
	var f bankFile
	if err := json.Unmarshal(banks, &f); err != nil {
		panic("gen: the sentence banks do not read: " + err.Error())
	}
	for _, t := range f.Topics {
		topics = append(topics, topic{statement: t.Statement, title: t.Title, body: t.Body, long: t.Long})
	}
	misspellings = f.Misspellings
}
