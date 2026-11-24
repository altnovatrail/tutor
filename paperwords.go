package tutor

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/altnovatrail/spec/types/formatv0"

	paperv1 "github.com/altnovatrail/spec/contracts/novatrail/paper/v1"

	"github.com/altnovatrail/module-kit/leak"
	"github.com/altnovatrail/module-kit/paper"

	"github.com/altnovatrail/tutor/internal/hint"
	"github.com/altnovatrail/tutor/internal/paperwords"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/provider"
	"github.com/altnovatrail/tutor/words"
)

// PaperAsk is what a call of the words of a task from paper asks for.
type PaperAsk int

// The kinds of words of a task from paper beyond the Check, which gives the tutor's words and the
// ladder of hints, each with no picture: a rung put another way ("Explain differently"), and the
// answer to a question of the learner's own about the task.
const (
	PaperAgain    PaperAsk = 1
	PaperQuestion PaperAsk = 2
)

// PaperTurn is a turn of the conversation on a task before a question: the learner's, or the
// tutor's words — a hint or an answer.
type PaperTurn struct {
	Learner bool
	Text    string
}

// PaperWordsTask is a task of the frame as the words see it: its statement as printed, as the
// reading of the statement gave it, or ""; the model's own solution and answer of the Check, which
// the learner never sees, the answer "" for a task already correct, whose answer is no leak; and the
// value read of the learner's answer, or "".
type PaperWordsTask struct {
	Statement string
	Solution  string
	Answer    string
	Read      string
}

// PaperWordsRequest is a call of the tutor's words on a task from paper:
//   - Ask is what to word, PaperAgain or PaperQuestion;
//   - Tasks are the tasks of the frame, in their order;
//   - Task is the number of the task the words are about among them, from 1, and Rung the rung of
//     the Check's ladder, 1 to 3, or for a question 0 while no rung is given;
//   - Language is the language of the profile's interface, by its tag; Style only what of the
//     presentation style changes the words;
//   - Asked are the hints the tutor has already given on the task, in order, the last the one to
//     put another way;
//   - Question is the learner's question, and Turns the conversation on the task before it, oldest
//     first: both data the model reads, never instructions, and only of PaperQuestion.
//
// The words are checked by api for leaks and for their form (PaperBreach), before they reach the
// learner; api gives the fallback text in their place when they fail (PaperFallback).
type PaperWordsRequest struct {
	Route    Route
	Ask      PaperAsk
	Tasks    []PaperWordsTask
	Task     int
	Rung     int
	Language string
	Style    string
	Asked    []string
	Question string
	Turns    []PaperTurn
}

// PaperWords asks the admitted models of the words of a task from paper (CallPaperHint), one at a
// time, a later one only when the one before failed, answered off the schema or its words broke the
// checks of PaperBreach. The answer is the tutor's question, the last model's that answered, whether
// or not its words passed. A request of another kind, with no style or no task, and a question with
// no words, fails before any call; an answer off the schema fails the call.
func (t *Tutor) PaperWords(ctx context.Context, w PaperWordsRequest) WordsAnswer {
	fail := func(err error) WordsAnswer { return WordsAnswer{Outcome: Outcome{Kind: Failed, Err: err}} }
	switch {
	case w.Ask != PaperAgain && w.Ask != PaperQuestion:
		return fail(fmt.Errorf("tutor: no kind of words of paper %d", w.Ask))
	case w.Ask == PaperQuestion && strings.TrimSpace(w.Question) == "":
		return fail(errors.New("tutor: the question has no words"))
	}
	if strings.TrimSpace(w.Style) == "" {
		return fail(errors.New("tutor: the words have no style of presentation"))
	}
	p := paperwords.Prompt{Task: w.Task, Rung: w.Rung, Language: w.Language, Style: w.Style, Asked: w.Asked}
	if w.Ask == PaperQuestion {
		p.Question = w.Question
		for _, turn := range w.Turns {
			p.Turns = append(p.Turns, paperwords.Turn{Learner: turn.Learner, Text: turn.Text})
		}
	}
	for i, task := range w.Tasks {
		p.Tasks = append(p.Tasks, paperwords.Task{Number: i + 1, Statement: task.Statement, Solution: task.Solution, Answer: task.Answer, Read: task.Read})
	}
	req, err := paperwords.RequestOf(p)
	if err != nil {
		return fail(fmt.Errorf("tutor: %w", err))
	}
	req.MaxOutputTokens = paperwords.MaxOutputTokens
	if err := checkText(req); err != nil {
		return fail(err)
	}
	calls, a, refused := t.prepare(CallPaperHint, w.Route)
	if refused != nil {
		return WordsAnswer{Outcome: *refused}
	}
	got, out := cascade(ctx, calls, func(cl call) (string, Outcome) {
		resps, out := ask(ctx, a, []call{cl}, []provider.Request{req}, kinds[CallPaperHint].maxIn)
		if out.Kind != Answered {
			return "", out
		}
		hint, err := paperwords.Read(resps[0].JSON)
		if err != nil {
			out.Kind, out.Err = Failed, fmt.Errorf("tutor: %s: %w", cl.model.Name(), err)
			return "", out
		}
		return hint, out
	}, func(hint string) bool { return PaperBreach(hint, w.Language, w.Tasks) == "" })
	if out.Kind != Answered {
		return WordsAnswer{Outcome: out}
	}
	return WordsAnswer{Question: got, Outcome: out}
}

// The causes of a fallback text of a task from paper's words that PaperBreach gives.
const (
	BreachLeak = "leak"
	BreachForm = "form"
)

// PaperBreach is why the words of a task from paper may not reach the learner, or "" when they
// may: a hidden answer of a task in them BreachLeak (leaks), and empty words or a broken form
// (words.Hint, with the statements' letters allowed) BreachForm. A text that both leaks and breaks
// its form is a leak.
func PaperBreach(text, language string, tasks []PaperWordsTask) string {
	if leaks(text, tasks) {
		return BreachLeak
	}
	var statements []string
	for _, t := range tasks {
		statements = append(statements, t.Statement)
	}
	if strings.TrimSpace(text) == "" || len(words.Hint(text, language, strings.Join(statements, " "))) > 0 {
		return BreachForm
	}
	return ""
}

// leaks tells whether a text names the hidden answer of a task, by the leak check of module-kit:
// each number the answer holds is a key of its own, which the text holds when it holds that value
// in any notation, and the statements and the values read are what the learner sees, so a number
// either holds is not searched for. An answer of no number of two letters or more, as a word, is
// searched for as a phrase, unless the learner sees it; a single letter, the label of an option, is
// in too many words to search for.
func leaks(text string, tasks []PaperWordsTask) bool {
	task := formatv0.Task{}
	var seen []string
	for _, t := range tasks {
		for _, s := range []string{t.Statement, t.Read} {
			if s != "" {
				text := formatv0.Text(s)
				task.Statement = append(task.Statement, formatv0.StatementPart{Text: &text})
				seen = append(seen, strings.ToLower(s))
			}
		}
	}
	for _, t := range tasks {
		numbers := leak.Numbers(t.Answer)
		for _, value := range numbers {
			if number := (&paperv1.Answer{Kind: &paperv1.Answer_Number{Number: &paperv1.Number{Value: value}}}); paper.Breach(number) == "" {
				task.Items = append(task.Items, paper.Items(number)...)
			}
		}
		word := strings.TrimSpace(t.Answer)
		if len(numbers) == 0 && utf8.RuneCountInString(word) > 1 && !slices.ContainsFunc(seen, func(s string) bool { return strings.Contains(s, strings.ToLower(word)) }) {
			task.LeakPhrases = append(task.LeakPhrases, formatv0.Text(word))
		}
	}
	search, err := leak.For(task)
	return err != nil || len(search.In(text)) > 0
}

// openings are the platform's fallback texts of the opening question of a task from paper, which
// has no module to give a leading question, in each language of the interface, English
// for any other. They hold no number, so that they pass the leak check of every task, and they keep
// the form of a hint.
var openings = map[sample.Lang]string{
	sample.LangEnglish: "What do you already know here, and what do you need to find?",
	sample.LangRussian: "Что здесь уже известно и что нужно найти?",
}

// PaperFallback is the fallback text of the words of a task from paper in a language of the
// interface, by its tag: the opening question at rung 0, and at rungs 1 to 3 the rung's template
// of the measurement of hints (hint.Fallback), which the measured languages have, the second
// serving the third, which has no simpler case of an author.
func PaperFallback(rung int, language string) string {
	base, _, _ := strings.Cut(strings.ToLower(language), "-")
	lang := sample.Lang(base)
	if _, ok := openings[lang]; !ok {
		lang = sample.LangEnglish
	}
	if rung < 1 {
		return openings[lang]
	}
	return hint.Fallback(hint.Rung(min(rung, 3)), lang)
}
