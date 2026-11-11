package tutor

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"

	"github.com/altnovatrail/tutor/internal/essay"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/provider"
	"github.com/altnovatrail/tutor/words"
)

// RubricAssessment is an essay to assess by the criteria of its task's rubric (architecture 3.10,
// «оценка по рубрике»), the second of the two calls of a task with a rubric (architecture 3.5; R75
// spec): the text the reading of the essay wrote down, the statement of the task as its topic, the
// criteria of the rubric in their order, in the module's explanation language, the presentation
// style — only what of it changes the words (Д74 RUN.md spec) —, the language of the feedback by its
// tag, and the route. The model sees the text in the block <essay> and the topic and the criteria in
// the block of the task's data, and no picture.
type RubricAssessment struct {
	Route    Route
	Text     string
	Topic    string
	Criteria []string
	Style    string
	Language string
}

// CriterionMark is the assessment of a criterion: what in the essay the model judged it by, and
// whether the essay meets it, nil when it has no assessment — a criterion the answer has no
// judgement of, or two (architecture 3.5).
type CriterionMark struct {
	Evidence string
	Met      *bool
}

// RubricAssessed is what an assessment by the rubric gives: a mark for each criterion, in the
// rubric's order, and one feedback, which api checks by the rule of the feedback (words.Feedback; R70
// spec) before it reaches the learner. When the call failed, every criterion is without an
// assessment, and there is no feedback.
type RubricAssessed struct {
	Criteria []CriterionMark
	Feedback string
	Outcome  Outcome
}

// AssessEssay assesses the text of an essay by the criteria of the rubric, by the one model admitted
// for the assessment by a rubric with the prompt of the assessment of a text (R70 spec; R97). The
// criteria go to the model by their numbers, from 1. Until an entry of the list measured the prompt,
// the assessment is NotAdmitted: its target comes from the measurement on the essays of volunteers
// (R71 spec; П-16). An answer off the schema fails the call; a criterion it judges twice, or not at
// all, is without an assessment, as is every criterion of a failed call.
func (t *Tutor) AssessEssay(ctx context.Context, r RubricAssessment) RubricAssessed {
	none := func(out Outcome) RubricAssessed {
		return RubricAssessed{Criteria: make([]CriterionMark, len(r.Criteria)), Outcome: out}
	}
	fail := func(err error) RubricAssessed { return none(Outcome{Kind: Failed, Err: err}) }
	switch {
	case strings.TrimSpace(r.Text) == "":
		return fail(errors.New("tutor: the essay has no text to assess"))
	case len(r.Criteria) == 0:
		return fail(errors.New("tutor: the rubric has no criterion"))
	case strings.TrimSpace(r.Topic) == "":
		return fail(errors.New("tutor: the essay has no topic"))
	case strings.TrimSpace(r.Style) == "":
		return fail(errors.New("tutor: the request names no presentation style"))
	}
	for i, c := range r.Criteria {
		if strings.TrimSpace(c) == "" {
			return fail(fmt.Errorf("tutor: criterion %d of the rubric is blank", i+1))
		}
	}
	name, err := languageName(r.Language)
	if err != nil {
		return fail(err)
	}
	rubric := make([]sample.Criterion, len(r.Criteria))
	for i, c := range r.Criteria {
		rubric[i] = sample.Criterion{ID: strconv.Itoa(i + 1), Text: c}
	}
	k := kinds[CallRubric]
	calls, a, refused := t.prepare(k.capability, r.Route, k.models, k.prompt)
	if refused != nil {
		return none(*refused)
	}
	req := essay.TextRequestOf(r.Topic, rubric, r.Text, r.Style, name)
	req.MaxOutputTokens = k.maxOut
	if err := checkText(req); err != nil {
		return none(Outcome{Kind: Failed, Admission: a, Err: err})
	}
	resps, out := ask(ctx, a, calls, []provider.Request{req}, k.maxIn)
	if out.Kind != Answered {
		return none(out)
	}
	answer, err := essay.ReadText(resps[0].JSON, r.Text)
	if err != nil {
		out.Kind, out.Err = Failed, fmt.Errorf("tutor: %s: %w", calls[0].model.Name(), err)
		return none(out)
	}
	marks, _ := essay.Marks(rubric, answer)
	got := RubricAssessed{Criteria: make([]CriterionMark, len(marks)), Feedback: answer.Feedback, Outcome: out}
	for i, m := range marks {
		if m.Met == nil {
			continue
		}
		got.Criteria[i].Met = m.Met
		for _, j := range answer.Criteria {
			if j.ID == m.ID {
				got.Criteria[i].Evidence = j.Evidence
			}
		}
	}
	return got
}

// languageName is the name in English of the language of a tag, which a request of the feedback
// names, as hint.RequestOf takes it for a hint: a tag that cannot be read, that names no language, or
// whose language words.Feedback cannot check, is refused, since a check that fails closed on its
// language would send every paid answer to the fallback.
func languageName(tag string) (string, error) {
	t, err := language.Parse(tag)
	name := display.English.Tags().Name(t)
	if err != nil || !words.Readable(tag) || name == "" || t.IsRoot() {
		return "", fmt.Errorf("tutor: %q is no language of the feedback", tag)
	}
	return name, nil
}
