package tutor

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/altnovatrail/spec/types/formatv0"

	observationv1 "github.com/altnovatrail/spec/contracts/novatrail/observation/v1"
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

// The kinds of words of a task from paper.
const (
	// PaperOpening is the opening question, before the first wrong answer, from the task's crop as
	// printed, PaperWordsRequest.Statement.
	PaperOpening = PaperAsk(paperwords.AskOpening)
	// PaperRung is the words of a rung, from the crop of the work, PaperWordsRequest.Crop, with the
	// box of the first mistake.
	PaperRung = PaperAsk(paperwords.AskRung)
	// PaperAgain is the rung put another way, with no picture.
	PaperAgain = PaperAsk(paperwords.AskAgain)
)

// PaperWordsTask is a task of the crop as the words see it: its statement as printed, its key, or
// nil while the program has derived none, and the value read of the learner's answer, or "".
type PaperWordsTask struct {
	Statement string
	Key       *paperv1.Answer
	Read      string
}

// PaperWordsRequest is a call of the tutor's words on a task from paper:
//   - Ask is what to word;
//   - Statement is the task's crop as printed, a JPEG or a PNG of at most 4 MiB, for PaperOpening
//     alone; Crop the PNG of the crop of the work, sent again, as the server keeps none, for
//     PaperRung alone;
//   - Tasks are the tasks of the crop, in their order;
//   - Task is the number of the rung's task among them, from 1, and Rung the rung api picked, 1 to
//     3, but for PaperOpening;
//   - Language is the language of the profile's interface, by its tag; Style only what of the
//     presentation style changes the words;
//   - Asked are the questions the tutor has already asked on the task, in order.
//
// The words are checked by api for leaks and for their form (PaperBreach), before they reach the
// learner; api gives the fallback text in their place when they fail (PaperFallback).
type PaperWordsRequest struct {
	Route     Route
	Ask       PaperAsk
	Statement []byte
	Crop      []byte
	Tasks     []PaperWordsTask
	Task      int
	Rung      int
	Language  string
	Style     string
	Asked     []string
}

// PaperWords asks the admitted models of the words of a task from paper, one at a time, a later
// one only when the one before failed or its words broke the checks of PaperBreach: with the
// work, CallPaperWords, which serves the analysis of the steps and the hint, so both are admitted
// on its models; without it, CallPaperHint. The answer is the box round the first mistake, in
// thousandths of the crop of the work, and the tutor's question: the last model's that answered,
// whether or not its words passed. A request a kind does not take fails before any call; an answer
// off the schema fails the call.
func (t *Tutor) PaperWords(ctx context.Context, w PaperWordsRequest) WordsAnswer {
	fail := func(err error) WordsAnswer { return WordsAnswer{Outcome: Outcome{Kind: Failed, Err: err}} }
	if strings.TrimSpace(w.Style) == "" {
		return fail(errors.New("tutor: the words have no style of presentation"))
	}
	p := paperwords.Prompt{Ask: paperwords.Ask(w.Ask), Task: w.Task, Rung: w.Rung, Language: w.Language, Style: w.Style, Asked: w.Asked}
	for i, task := range w.Tasks {
		p.Tasks = append(p.Tasks, paperwords.Task{Number: i + 1, Statement: task.Statement, Key: keyWritten(task.Key), Read: task.Read})
	}
	c := CallPaperHint
	switch w.Ask {
	case PaperRung:
		cfg, err := cropSize(w.Crop)
		if err != nil {
			return fail(err)
		}
		p.Image, p.Width, p.Height = provider.Image{PNG: w.Crop}, cfg.Width, cfg.Height
		c = CallPaperWords
	case PaperOpening:
		if len(w.Statement) == 0 {
			return fail(errors.New("tutor: the opening question has no crop of the task"))
		}
	}
	calls, a, refused := t.prepare(c, w.Route)
	if refused != nil {
		return WordsAnswer{Outcome: *refused}
	}
	type said struct {
		answer *paperwords.Answer
		p      paperwords.Prompt
	}
	statements := provider.NewPhotos(w.Statement)
	got, out := cascade(ctx, calls, func(cl call) (said, Outcome) {
		failed := func(err error) (said, Outcome) { return said{}, Outcome{Kind: Failed, Admission: a, Err: err} }
		p := p
		if w.Ask == PaperOpening {
			crop, width, height, err := statements.For(cl.client)
			if err != nil {
				return failed(fmt.Errorf("tutor: the task's crop: %w", err))
			}
			p.Image, p.Width, p.Height = crop, width, height
		}
		req, err := paperwords.RequestOf(p)
		if err != nil {
			return failed(fmt.Errorf("tutor: %w", err))
		}
		req.MaxOutputTokens = paperwords.MaxOutputTokens
		if err := checkText(req); err != nil {
			return failed(err)
		}
		resps, out := ask(ctx, a, []call{cl}, []provider.Request{req}, kinds[c].maxIn)
		if out.Kind != Answered {
			return said{}, out
		}
		answer, err := paperwords.Read(resps[0].JSON)
		if err != nil {
			out.Kind, out.Err = Failed, fmt.Errorf("tutor: %s: %w", cl.model.Name(), err)
			return said{}, out
		}
		return said{answer: answer, p: p}, out
	}, func(s said) bool {
		return PaperBreach(s.answer.Hint, w.Language, w.Tasks, s.answer.Answers) == ""
	})
	if out.Kind != Answered {
		return WordsAnswer{Outcome: out}
	}
	answer := WordsAnswer{Question: got.answer.Hint, Answers: got.answer.Answers, Outcome: out}
	if got.answer.Box != nil && w.Ask == PaperRung {
		b := got.answer.Box
		if place, ok := provider.Place(b.X1, b.Y1, b.X2, b.Y2, got.p.Width, got.p.Height); ok {
			answer.Box = &observationv1.CropBox{Ymin: int32(place[0]), Xmin: int32(place[1]), Ymax: int32(place[2]), Xmax: int32(place[3])}
		}
	}
	return answer
}

// keyWritten is a key as the learner would write it: the number, or the label of the option; ""
// for none.
func keyWritten(a *paperv1.Answer) string {
	if n := a.GetNumber(); n != nil {
		return n.GetValue()
	}
	return a.GetChoice().GetKey()
}

// The causes of a fallback text of a task from paper's words that PaperBreach gives.
const (
	BreachLeak = "leak"
	BreachForm = "form"
)

// PaperBreach is why the words of a task from paper may not reach the learner, or "" when they
// may: empty words or a broken form (words.Hint, with the statements' letters allowed) BreachForm,
// and a key of a task of the crop in them, or a number of own — the model's own answers of the
// opening question, which nothing else keeps — BreachLeak. The leak check is module-kit's, with
// the keys as checks of their kind and the statements as the text the learner sees with them: a
// key the statement prints is not searched for. A text that both leaks and breaks its
// form is a leak.
func PaperBreach(text, language string, tasks []PaperWordsTask, own []string) string {
	var statements []string
	task := formatv0.Task{}
	for _, t := range tasks {
		statements = append(statements, t.Statement)
		text := formatv0.Text(t.Statement)
		task.Statement = append(task.Statement, formatv0.StatementPart{Text: &text})
		if t.Key != nil {
			task.Items = append(task.Items, paper.Items(t.Key)...)
		}
	}
	for _, value := range own {
		// A label of an option of one's own is not searched for: a single letter is in too many
		// words, and the key's own label, once there is one, is.
		if number := (&paperv1.Answer{Kind: &paperv1.Answer_Number{Number: &paperv1.Number{Value: value}}}); paper.Breach(number) == "" {
			task.Items = append(task.Items, paper.Items(number)...)
		}
	}
	if search, err := leak.For(task); err != nil || len(search.In(text)) > 0 {
		return BreachLeak
	}
	if strings.TrimSpace(text) == "" || len(words.Hint(text, language, strings.Join(statements, " "))) > 0 {
		return BreachForm
	}
	return ""
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
