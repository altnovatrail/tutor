package tutor

import (
	"slices"
	"testing"

	"github.com/altnovatrail/spec/types/admissionv0"

	"github.com/altnovatrail/tutor/admission"
	"github.com/altnovatrail/tutor/provider"
)

// withoutWord leaves out of the list the entries the author admitted by their word, for the test
// alone: the kinds they admit are "not admitted" again.
func withoutWord(t *testing.T) {
	t.Helper()
	saved := admission.Admitted
	t.Cleanup(func() { admission.Admitted = saved })
	admission.Admitted = slices.DeleteFunc(slices.Clone(saved), func(e admission.Entry) bool { return e.Report.ByWord() })
}

// The kinds the author admitted by their word, without eval, and the models each asks (R126).
var byWord = []struct {
	call   Call
	models []string
}{
	{CallWorkbook, []string{"claude-opus-5-5 high", "claude-haiku-5-5 medium"}},
	{CallEssay, []string{"claude-haiku-5-5 medium"}},
	{CallRubric, []string{"claude-haiku-5-5 medium"}},
	{CallFinding, []string{"claude-haiku-5-5 high", "claude-sonnet-5-5 medium", "claude-opus-5-5 medium"}},
	{CallStatement, []string{"claude-haiku-5-5 high", "claude-sonnet-5-5 medium", "claude-opus-5-5 medium"}},
	{CallSolving, []string{"claude-haiku-5-5 high", "claude-sonnet-5-5 medium", "claude-opus-5-5 medium"}},
	{CallPaper, []string{"claude-haiku-5-5 high", "claude-sonnet-5-5 medium", "claude-opus-5-5 medium"}},
	{CallPaperWords, []string{"claude-haiku-5-5 high", "claude-sonnet-5-5 medium", "claude-opus-5-5 medium"}},
	{CallPaperHint, []string{"claude-haiku-5-5 high", "claude-sonnet-5-5 medium", "claude-opus-5-5 medium"}},
}

func names(e admission.Entry) []string {
	var out []string
	for _, m := range e.Models {
		out = append(out, m.Name())
	}
	return out
}

// The list's own entries admit the workbook, the essay and the rubric at the least stage author, on
// platform AI and on an adult's own key of Anthropic, and none of them at the least stage full, on
// an own key of OpenAI or for a child whose cells the deployment's facts do not open.
func TestByWord(t *testing.T) {
	fs := &fakes{models: map[string]*fake{"claude-opus-5-5": {}, "claude-haiku-5-5": {}, "claude-sonnet-5-5": {}}}
	lib := library(t, adultFacts, fs)
	full, err := New(Config{Table: lib.table, Facts: lib.facts, Least: admission.StageFull, Keys: lib.keys,
		Adapters: map[admissionv0.Provider]provider.Dial{"anthropic": fs.dial}})
	if err != nil {
		t.Fatal(err)
	}
	gpt, err := New(Config{Table: lib.table, Facts: lib.facts, Least: admission.StageAuthor,
		Adapters: map[admissionv0.Provider]provider.Dial{"anthropic": fs.dial, "openai": fs.dial}})
	if err != nil {
		t.Fatal(err)
	}
	openAI := Route{AI: admissionv0.AiRouteOwnKey, Category: admissionv0.AgeCategoryAdult, Own: NewOwnKey("openai", testKey)}
	child := Route{AI: admissionv0.AiRoutePlatformAi, Category: admissionv0.AgeCategoryUnder13}
	for _, k := range byWord {
		for name, r := range map[string]Route{"platform AI": platform, "an own key": ownRoute} {
			e, a, err := lib.ChooseCall(k.call, r)
			if err != nil || !a.Admitted || e.Stage != admission.StageAuthor || !e.Report.ByWord() || !slices.Equal(names(e), k.models) {
				t.Errorf("%s on %s: %+v, %+v, %v", k.call, name, e, a, err)
			}
		}
		if e, a, err := full.ChooseCall(k.call, platform); err != nil || a.Admitted || *a.Reason != admission.ReasonModelNotAdmitted || e.Models != nil {
			t.Errorf("%s at the least stage full: %+v, %+v, %v", k.call, e, a, err)
		}
		if e, a, err := gpt.ChooseCall(k.call, openAI); err != nil || a.Admitted || *a.Reason != admission.ReasonModelNotAdmitted || e.Models != nil {
			t.Errorf("%s on an own key of OpenAI: %+v, %+v, %v", k.call, e, a, err)
		}
		if _, a, err := lib.ChooseCall(k.call, child); err != nil || a.Admitted || a.Reason == nil || *a.Reason == admission.ReasonModelNotAdmitted {
			t.Errorf("%s for a child without the facts for minors: %+v, %v", k.call, a, err)
		}
	}
	// A child's workbook is read only where the deployment declares the facts for minors, as the
	// synthetic child of api's dev-app does.
	minors := library(t, "anthropic:\n  data_processing_agreement: [adult, under_13]\n  minimal_retention: [adult, under_13]\n  minor_safeguards: [under_13]\n", fs)
	if e, a, err := minors.ChooseCall(CallWorkbook, child); err != nil || !a.Admitted || len(e.Models) != 2 {
		t.Errorf("a child's workbook with the facts for minors: %+v, %+v, %v", e, a, err)
	}
	// Without the word's entries the three kinds are "not admitted" again.
	withoutWord(t)
	for _, k := range byWord {
		if _, a, err := lib.ChooseCall(k.call, platform); err != nil || a.Admitted || *a.Reason != admission.ReasonModelNotAdmitted {
			t.Errorf("%s without its entry: %+v, %v", k.call, a, err)
		}
	}
}

// The price of each kind admitted by the word is by the tokens of the runs that tried its prompt:
// the workbook over both readers of the pair, or Haiku's alone in the mode of one reader, and the
// essay and the rubric Haiku's.
func TestByWordPrice(t *testing.T) {
	lib, _, _ := pricedPair(t)
	page := float64(24000 + 20*19*19)
	for _, c := range []struct {
		call            Call
		reading         Reading
		expected, worst float64
	}{
		{CallWorkbook, ReadingPair, (1849*4+1090*20)/1e6 + (1849*0.1+1568*0.5)/1e6, (page*5+16000*20)/1e6 + (page*0.1+16000*0.5)/1e6},
		{CallWorkbook, ReadingSingle, (1849*0.1 + 1568*0.5) / 1e6, (page*0.1 + 16000*0.5) / 1e6},
		{CallEssay, ReadingPair, (2067*0.1 + 127*0.5) / 1e6, (float64(24000+147*147)*0.1 + 16000*0.5) / 1e6},
		{CallRubric, ReadingSingle, (1398*0.1 + 962*0.5) / 1e6, (24000*0.1 + 16000*0.5) / 1e6},
	} {
		lib.reading = c.reading
		p, a, err := lib.Price(c.call, platform)
		if err != nil || !a.Admitted || !p.Expected || !p.Worst || !near(p.ExpectedUSD, c.expected) || !near(p.WorstUSD, c.worst) {
			t.Errorf("the price of %s in the mode %s: %+v, %+v, %v; want expected %v and worst %v", c.call, c.reading, p, a, err, c.expected, c.worst)
		}
	}
}
