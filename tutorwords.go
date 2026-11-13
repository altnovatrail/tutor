package tutor

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/altnovatrail/spec/types/formatv0"
	"github.com/altnovatrail/spec/types/formatv0/open"

	observationv1 "github.com/altnovatrail/spec/contracts/novatrail/observation/v1"

	"github.com/altnovatrail/tutor/internal/hint"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/provider"
)

// wordsMaxOutput is the most the call of words may write, thinking included: the bound of the
// runner's calls, as for the reading, so that what the platform asks is what eval measures (R70
// spec). A tighter bound would make the library ask what eval does not measure (R94).
const wordsMaxOutput = 16000

// Trap is a trap of a task that a model may name, by its ID, with its description from the
// catalogue of traps, which api takes through module-kit (R57 spec).
type Trap struct {
	ID, Description string
}

// WordsRequest is a call of the tutor's words (architecture 3.10, "analysis and hint"): the analysis
// of the steps and the hint of a rung, in one call, as architecture 4.1 of spec draws it.
//   - Crop is the PNG of the crop of the work, sent again, as the server keeps none (R09 spec); nil
//     when the learner asks to have the rung put another way, which sends no work (R82 spec);
//   - Key and Solution are the task's, which the model sees here alone (R67 spec);
//   - Traps are the task's traps that AI may name, with their descriptions, and Found the trap of
//     the cell of the rung, which the program found by its value, or empty (R82 spec);
//   - Techniques are the task's ways to solve it;
//   - Rung is the rung api picked, 1 to 3; Language the module's language of explanations, by its
//     tag; Style only what of the presentation style changes the words (Д74 RUN.md spec);
//   - Asked are the questions the tutor has already asked on the task, in order (privacy 4 spec).
//
// The words are checked by api for leaks and by words.Hint for their form, before they reach the
// learner; api gives the fallback text in their place when they fail (R70 spec).
type WordsRequest struct {
	Route      Route
	Task       *open.Task
	Crop       []byte
	Key        string
	Solution   []string
	Traps      []Trap
	Found      string
	Techniques []string
	Rung       int
	Language   string
	Style      string
	Asked      []string
}

// question is the text of the item of a task's one answer slot bound anywhere, what it asks, or nil.
func question(t *open.Task) *formatv0.Text {
	for _, item := range t.Items {
		for _, slot := range item.AnswerSlots {
			if slot.Binding == formatv0.AnswerSlotBindingAnywhere {
				return item.Text
			}
		}
	}
	return nil
}

// WordsAnswer is what the call of words gives, when Outcome is Answered: the box round the first
// mistake, in thousandths of the crop, the same message as an observation's (R86 spec), or nil when
// the model found none or saw no work; the trap of the request's that the mistake falls into, or
// empty; and the tutor's question.
type WordsAnswer struct {
	Box      *observationv1.CropBox
	Trap     string
	Question string
	Outcome  Outcome
}

// Words asks the admitted model of the words for the analysis and the hint of a rung. With the work
// it serves the analysis of the steps and the hint, so both must be admitted, and the model is the
// analysis's (CallWords); without it, the hint alone (CallHint). A task without words, a crop that is no PNG of a sheet, a
// rung the ladder has not, a language with no tag and no style of presentation fail before any
// call; an answer off the schema, or naming a trap the request does not hold, fails the call.
func (t *Tutor) Words(ctx context.Context, w WordsRequest) WordsAnswer {
	fail := func(err error) WordsAnswer { return WordsAnswer{Outcome: Outcome{Kind: Failed, Err: err}} }
	if w.Task == nil {
		return fail(errors.New("tutor: the words have no task"))
	}
	if strings.TrimSpace(w.Style) == "" {
		// The model words the hint for the learner the style describes, and api always knows one.
		return fail(errors.New("tutor: the words have no style of presentation"))
	}
	statement, err := taskText(w.Task, question(w.Task))
	if err != nil {
		return fail(err)
	}
	traps := make([]sample.Trap, 0, len(w.Traps))
	for _, tr := range w.Traps {
		traps = append(traps, sample.Trap{ID: tr.ID, Description: tr.Description})
	}
	if w.Found != "" && !slices.ContainsFunc(traps, func(tr sample.Trap) bool { return tr.ID == w.Found }) {
		// The trap found must be one the model reads described, and one its answer may name.
		return fail(fmt.Errorf("tutor: the trap found, %q, is none of the traps of the request", w.Found))
	}
	p := hint.Prompt{Statement: statement, Key: w.Key, Solution: w.Solution, Traps: traps, Found: w.Found,
		Techniques: w.Techniques, Rung: hint.Rung(w.Rung), Language: w.Language, Style: w.Style, Asked: w.Asked}
	c := CallHint
	if len(w.Crop) > 0 {
		cfg, err := cropSize(w.Crop)
		if err != nil {
			return fail(err)
		}
		p.PNG, p.Width, p.Height = w.Crop, cfg.Width, cfg.Height
		c = CallWords
	}
	req, err := hint.RequestOf(p)
	if err != nil {
		return fail(fmt.Errorf("tutor: %w", err))
	}
	req.MaxOutputTokens = wordsMaxOutput
	if err := checkText(req); err != nil {
		return fail(err)
	}
	// With the work, the one call words the hint too: the hint must be admitted on the analysis's
	// model (R84), which prepare holds, as ChooseCall, Price and CheckKey do.
	calls, a, refused := t.prepare(c, w.Route)
	if refused != nil {
		return WordsAnswer{Outcome: *refused}
	}
	// The words are one model's: an entry of more asks its first.
	resps, out := ask(ctx, a, calls, []provider.Request{req}, kinds[c].maxIn)
	if out.Kind != Answered {
		return WordsAnswer{Outcome: out}
	}
	answer, err := hint.Read(resps[0].JSON, traps)
	if err != nil {
		out.Kind, out.Err = Failed, fmt.Errorf("tutor: %s: %w", calls[0].model.Name(), err)
		return WordsAnswer{Outcome: out}
	}
	got := WordsAnswer{Trap: answer.Trap, Question: answer.Hint, Outcome: out}
	if answer.Box != nil && len(p.PNG) > 0 {
		if place, ok := provider.Place(answer.Box.X1, answer.Box.Y1, answer.Box.X2, answer.Box.Y2, p.Width, p.Height); ok {
			got.Box = &observationv1.CropBox{Ymin: int32(place[0]), Xmin: int32(place[1]), Ymax: int32(place[2]), Xmax: int32(place[3])}
		}
	}
	return got
}
