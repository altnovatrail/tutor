package tutor

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/altnovatrail/spec/types/formatv0"
	"github.com/altnovatrail/spec/types/formatv0/open"

	"github.com/altnovatrail/tutor/internal/correct"
	"github.com/altnovatrail/tutor/provider"
)

// CorrectRequest is a call of the words after a correct answer (architecture 3.10; R112): the praise
// of a step of the solution and one question on the technique that helped, after the program's
// verdict is correct, as GetCorrectAnswerWords of spec's contract asks them.
//   - Crop is the PNG of the crop of the work; nil when the task's answer slots are only choices,
//     and the learner chose the answer with no work to send;
//   - Key and Solution are the task's — of a choice, the key is the option that is right — and
//     Techniques its ways to solve it, which may be none: the question then asks how the answer was
//     found;
//   - Language is the module's language of explanations, by its tag; Style only what of the
//     presentation style changes the words.
//
// The words are checked by api for leaks and by words.Correct for their form before they reach the
// learner; when they fail, the learner gets no words, as there is no fallback text for them (R70
// spec).
type CorrectRequest struct {
	Route      Route
	Task       *open.Task
	Crop       []byte
	Key        string
	Solution   []string
	Techniques []string
	Language   string
	Style      string
}

// CorrectAnswer is what the call of the words after a correct answer gives: the words, when Outcome
// is Answered.
type CorrectAnswer struct {
	Words   string
	Outcome Outcome
}

// asked is what the task asks for the words after a correct answer, which follow an answer slot bound
// anywhere or a choice: the text of the item of the slot bound anywhere, as the words of a hint read
// it, or else of the first item of a choice that has one, or nil.
func asked(t *open.Task) *formatv0.Text {
	if q := question(t); q != nil {
		return q
	}
	for _, item := range t.Items {
		for _, slot := range item.AnswerSlots {
			if slot.Binding == formatv0.AnswerSlotBindingChoice && item.Text != nil {
				return item.Text
			}
		}
	}
	return nil
}

// CorrectAnswerWords asks the admitted model of the words after a correct answer (CallCorrect) for
// them, with the work or without it. A task without words, a crop that is no PNG of a sheet, a
// language with no tag and no style of presentation fail before any call; an answer off the schema
// fails the call.
func (t *Tutor) CorrectAnswerWords(ctx context.Context, c CorrectRequest) CorrectAnswer {
	fail := func(err error) CorrectAnswer { return CorrectAnswer{Outcome: Outcome{Kind: Failed, Err: err}} }
	if c.Task == nil {
		return fail(errors.New("tutor: the words after a correct answer have no task"))
	}
	if strings.TrimSpace(c.Style) == "" {
		return fail(errors.New("tutor: the words after a correct answer have no style of presentation"))
	}
	statement, err := taskText(c.Task, asked(c.Task))
	if err != nil {
		return fail(err)
	}
	p := correct.Prompt{Statement: statement, Key: c.Key, Solution: c.Solution, Techniques: c.Techniques, Language: c.Language, Style: c.Style}
	if len(c.Crop) > 0 {
		cfg, err := cropSize(c.Crop)
		if err != nil {
			return fail(err)
		}
		p.PNG, p.Width, p.Height = c.Crop, cfg.Width, cfg.Height
	}
	req, err := correct.RequestOf(p)
	if err != nil {
		return fail(fmt.Errorf("tutor: %w", err))
	}
	req.MaxOutputTokens = wordsMaxOutput
	if err := checkText(req); err != nil {
		return fail(err)
	}
	calls, a, refused := t.prepare(CallCorrect, c.Route)
	if refused != nil {
		return CorrectAnswer{Outcome: *refused}
	}
	resps, out := ask(ctx, a, calls, []provider.Request{req}, kinds[CallCorrect].maxIn)
	if out.Kind != Answered {
		return CorrectAnswer{Outcome: out}
	}
	words, err := correct.Read(resps[0].JSON)
	if err != nil {
		out.Kind, out.Err = Failed, fmt.Errorf("tutor: %s: %w", calls[0].model.Name(), err)
		return CorrectAnswer{Outcome: out}
	}
	return CorrectAnswer{Words: words, Outcome: out}
}
