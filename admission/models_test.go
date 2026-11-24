package admission

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/altnovatrail/spec/types/admissionv0"

	"github.com/altnovatrail/tutor/internal/checking"
	"github.com/altnovatrail/tutor/internal/correct"
	"github.com/altnovatrail/tutor/internal/essay"
	"github.com/altnovatrail/tutor/internal/finding"
	"github.com/altnovatrail/tutor/internal/hint"
	"github.com/altnovatrail/tutor/internal/paperwords"
	"github.com/altnovatrail/tutor/internal/reading"
	"github.com/altnovatrail/tutor/internal/statement"
	"github.com/altnovatrail/tutor/internal/workbook"
	"github.com/altnovatrail/tutor/provider/claude/catalog"
)

var (
	opus  = Model{Provider: "anthropic", ID: "claude-opus-5-5", Effort: "high"}
	haiku = Model{Provider: "anthropic", ID: "claude-haiku-5-5", Effort: "medium"}
	gpt   = Model{Provider: "openai", ID: "gpt-6-astra", Effort: "high"}
	gem   = Model{Provider: "google", ID: "gemini", Effort: "high"}
)

// testList is a list of admitted models of the tests' own, not the platform's.
var testList = []Entry{
	{Capability: admissionv0.CapabilityHandwritingReading, Models: []Model{opus, gem}, Stage: StageFull},
	{Capability: admissionv0.CapabilityHandwritingReading, Models: []Model{opus, haiku}, Stage: StageAuthor},
	{Capability: admissionv0.CapabilityHint, Models: []Model{gpt}, Stage: StageFull},
	{Capability: admissionv0.CapabilitySolutionAnalysis},
}

func TestModelName(t *testing.T) {
	if opus.Name() != "claude-opus-5-5 high" {
		t.Errorf("Opus is named %q", opus.Name())
	}
	off := Model{Provider: "anthropic", ID: "claude-haiku-5-5", Effort: "low", NoThinking: true}
	if off.Name() != "claude-haiku-5-5 low no-thinking" {
		t.Errorf("Haiku without thinking is named %q", off.Name())
	}
}

func TestSelect(t *testing.T) {
	table, err := Platform()
	if err != nil {
		t.Fatal(err)
	}
	adultFacts, err := ReadFacts(table, []byte("anthropic:\n  data_processing_agreement: [adult]\n  minimal_retention: [adult]\n"))
	if err != nil {
		t.Fatal(err)
	}
	handwriting := admissionv0.CapabilityHandwritingReading
	adult := admissionv0.AgeCategoryAdult

	// Google closes the first pair, so the second is chosen; the answer is its first model's.
	e, a := table.selectFrom(testList, adultFacts, handwriting, admissionv0.AiRoutePlatformAi, adult, StageAuthor, nil)
	if !a.Admitted || !slices.Equal(e.Models, []Model{opus, haiku}) || e.Stage != StageAuthor || a.Cell.Provider != "anthropic" {
		t.Fatalf("chose %+v, %+v", e, a)
	}
	// The same table, facts and request choose the same, before the attempt and at the call.
	again, b := table.selectFrom(testList, adultFacts, handwriting, admissionv0.AiRoutePlatformAi, adult, StageAuthor, nil)
	if !slices.Equal(again.Models, e.Models) || b.Version != a.Version {
		t.Errorf("chose %+v the second time", again)
	}
	// The entry chosen is a copy: changing it changes no list.
	e.Models[0] = gem
	if testList[1].Models[0] != opus {
		t.Error("the choice shares its models with the list")
	}
	// The own key of an adult needs no fact.
	if e, a := table.selectFrom(testList, Facts{}, handwriting, admissionv0.AiRouteOwnKey, adult, StageAuthor, nil); !a.Admitted || e.Models[1] != haiku {
		t.Errorf("the own key chose %+v, %+v", e, a)
	}
	// With no facts the table refuses the pair: its first refusal, not the model's.
	if e, a := table.selectFrom(testList, Facts{}, handwriting, admissionv0.AiRoutePlatformAi, adult, StageAuthor, nil); a.Admitted || *a.Reason != admissionv0.AnswerReasonCellClosed && *a.Reason != admissionv0.AnswerReasonFactMissing || e.Models != nil {
		t.Errorf("no facts chose %+v, %+v", e, a)
	}
	// A child: the own key is closed whatever the list holds.
	if _, a := table.selectFrom(testList, adultFacts, handwriting, admissionv0.AiRouteOwnKey, admissionv0.AgeCategoryUnder13, StageAuthor, nil); a.Admitted || *a.Reason != admissionv0.AnswerReasonCellClosed {
		t.Errorf("a child's own key: %+v", a)
	}
	// A capability with no model, or with an entry of none, refuses as the model's, after the table.
	for _, c := range []admissionv0.Capability{admissionv0.CapabilityRubricAssessment, admissionv0.CapabilitySolutionAnalysis, "unknown"} {
		if e, a := table.selectFrom(testList, adultFacts, c, admissionv0.AiRouteOwnKey, adult, StageAuthor, nil); a.Admitted || *a.Reason != ReasonModelNotAdmitted || a.Version != table.Version() || e.Models != nil {
			t.Errorf("%s chose %+v, %+v", c, e, a)
		}
	}
	// A deployment whose least stage is full has none here for the reading on platform AI: the full
	// pair is Google's, which the table closes. Any other least stage but author is as strict.
	for _, least := range []Stage{StageFull, "", "trial"} {
		if e, a := table.selectFrom(testList, adultFacts, handwriting, admissionv0.AiRoutePlatformAi, adult, least, nil); a.Admitted || e.Models != nil {
			t.Errorf("the least stage %q chose %+v, %+v", least, e, a)
		}
	}
	// An entry of the stage full serves the least stage full: the hint on an adult's own key.
	if e, a := table.selectFrom(testList, Facts{}, admissionv0.CapabilityHint, admissionv0.AiRouteOwnKey, adult, StageFull, nil); !a.Admitted || e.Stage != StageFull || e.Models[0] != gpt {
		t.Errorf("the full hint on an adult's own key chose %+v, %+v", e, a)
	}
	// An own key of OpenAI takes no pair of Anthropic, though the table admits both: the hint of
	// GPT is chosen, and the reading has none of that provider's.
	if e, a := table.selectFrom(testList, Facts{}, admissionv0.CapabilityHint, admissionv0.AiRouteOwnKey, adult, StageAuthor, []admissionv0.Provider{"openai"}); !a.Admitted || e.Models[0] != gpt {
		t.Errorf("an OpenAI key's hint chose %+v, %+v", e, a)
	}
	if e, a := table.selectFrom(testList, Facts{}, handwriting, admissionv0.AiRouteOwnKey, adult, StageAuthor, []admissionv0.Provider{"openai"}); a.Admitted || *a.Reason != ReasonModelNotAdmitted || e.Models != nil {
		t.Errorf("an OpenAI key's reading chose %+v, %+v", e, a)
	}
	// A child's own key of OpenAI: the table refuses the cell, and that comes before the model.
	if _, a := table.selectFrom(testList, Facts{}, handwriting, admissionv0.AiRouteOwnKey, admissionv0.AgeCategoryUnder13, StageAuthor, []admissionv0.Provider{"openai"}); a.Admitted || *a.Reason != admissionv0.AnswerReasonCellClosed {
		t.Errorf("a child's OpenAI key: %+v", a)
	}
	// Platform AI with the key of Anthropic alone takes no entry of OpenAI.
	if e, a := table.selectFrom(testList, Facts{}, admissionv0.CapabilityHint, admissionv0.AiRouteOwnKey, adult, StageAuthor, []admissionv0.Provider{"anthropic"}); a.Admitted || *a.Reason != ReasonModelNotAdmitted || e.Models != nil {
		t.Errorf("the hint with Anthropic alone: %+v, %+v", e, a)
	}
	// The platform's list: the first app, where the author alone learns, reads by the author's
	// pair; a deployment for anyone else has no reader yet, as the pair passed the stage author
	// alone.
	e, a = table.Select(adultFacts, handwriting, admissionv0.AiRoutePlatformAi, adult, StageAuthor, nil)
	if !a.Admitted || e.Stage != StageAuthor || len(e.Models) != 2 || e.Models[0].Name() != "claude-opus-5-5 high" || e.Models[1].Name() != "claude-haiku-5-5 medium" {
		t.Errorf("the first app chose %+v, %+v", e, a)
	}
	if e, a := table.Select(adultFacts, handwriting, admissionv0.AiRoutePlatformAi, adult, StageFull, nil); a.Admitted || *a.Reason != ReasonModelNotAdmitted || e.Models != nil {
		t.Errorf("a deployment for others chose %+v, %+v", e, a)
	}
	if e, a := table.Select(Facts{}, handwriting, admissionv0.AiRoutePlatformAi, adult, StageAuthor, nil); a.Admitted || *a.Reason != admissionv0.AnswerReasonFactMissing || e.Models != nil {
		t.Errorf("the first app without its facts chose %+v, %+v", e, a)
	}
}

// requiresRetention are the models of the adapters that need Anthropic to keep the work 30 days,
// which the list of admitted models leaves out: "Claude Fable 5.1, Claude Mythos 5.1,
// Claude Fable 5, and Claude Mythos 5: These models require 30-day data retention and are not
// available under ZDR unless expressly authorized by Anthropic" (Anthropic's «API and data
// retention»). Of them, the adapter of Claude calls Fable 5.1 alone.
var requiresRetention = []string{"claude-fable-5-1"}

var (
	sha256Form = regexp.MustCompile(`^[0-9a-f]{64}$`)
	dateForm   = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)
)

// Every entry of the platform's list names a model an adapter calls, at an effort it takes, none
// that needs 30-day retention, and the report that admitted it or, at the stage author, the author's
// word with its last day; a reading of a sheet or of a workbook page is a pair, Opus first,
// and of an essay one reader, each with the prompt the reading has now, so that a new prompt fails
// here until it is measured.
func TestAdmitted(t *testing.T) {
	for _, f := range listFaults(Admitted) {
		t.Error(f)
	}
	evidence := Evidence{Set: "author-all", File: "x.report.json", SHA256: strings.Repeat("ab", 32), Date: "2026-11-08"}
	good := Entry{Capability: admissionv0.CapabilityHandwritingReading, Models: []Model{opus, haiku}, Stage: StageAuthor, Report: evidence, Prompt: reading.Fingerprint()}
	if f := listFaults([]Entry{good}); len(f) != 0 {
		t.Errorf("a good entry has faults: %v", f)
	}
	bad := map[string]func(*Entry){
		"no models":      func(e *Entry) { e.Models = nil },
		"a stage":        func(e *Entry) { e.Stage = "trial" },
		"no report":      func(e *Entry) { e.Report = Evidence{} },
		"another prompt": func(e *Entry) { e.Prompt = "old" },
		"one reader":     func(e *Entry) { e.Models = e.Models[:1] },
		"retention": func(e *Entry) {
			e.Models = []Model{{Provider: "anthropic", ID: "claude-fable-5-1", Effort: "high"}, haiku}
		},
		"unknown model": func(e *Entry) { e.Models = []Model{opus, {Provider: "anthropic", ID: "claude-haiku-9", Effort: "low"}} },
		"unknown effort": func(e *Entry) {
			e.Models = []Model{opus, {Provider: "anthropic", ID: "claude-haiku-5-5", Effort: "turbo"}}
		},
		"other provider": func(e *Entry) { e.Models = []Model{opus, gpt} },
	}
	for name, change := range bad {
		e := good
		e.Models = slices.Clone(good.Models)
		change(&e)
		if len(listFaults([]Entry{e})) == 0 {
			t.Errorf("%s: no fault", name)
		}
	}
	// The words after a correct answer are one model's, admitted with their prompt now.
	words := Entry{Capability: admissionv0.CapabilityCorrectAnswerWords, Models: []Model{opus}, Stage: StageAuthor, Report: evidence, Prompt: correct.Fingerprint()}
	if f := listFaults([]Entry{words}); len(f) != 0 {
		t.Errorf("a good entry of the words after a correct answer has faults: %v", f)
	}
	for name, change := range map[string]func(*Entry){
		"two models":     func(e *Entry) { e.Models = []Model{opus, haiku} },
		"another prompt": func(e *Entry) { e.Prompt = hint.Fingerprint() },
	} {
		e := words
		change(&e)
		if len(listFaults([]Entry{e})) == 0 {
			t.Errorf("the words after a correct answer, %s: no fault", name)
		}
	}
	// The Check is a cascade of one to three attempts, admitted with its prompt now.
	check := Entry{Capability: admissionv0.CapabilityTaskChecking, Models: []Model{haiku, opus}, Stage: StageAuthor, Report: evidence, Prompt: checking.Fingerprint()}
	if f := listFaults([]Entry{check}); len(f) != 0 {
		t.Errorf("a good entry of the Check has faults: %v", f)
	}
	for name, change := range map[string]func(*Entry){
		"four models":    func(e *Entry) { e.Models = []Model{haiku, opus, haiku, opus} },
		"another prompt": func(e *Entry) { e.Prompt = finding.Fingerprint() },
	} {
		e := check
		change(&e)
		if len(listFaults([]Entry{e})) == 0 {
			t.Errorf("the Check, %s: no fault", name)
		}
	}
	// An entry by the author's word has no report but the word, its day and its last day, holds the
	// stage author alone, and is a fault once that day has passed.
	word := Evidence{Word: "the author's word", Date: "2026-11-17", Until: "2027-01-17"}
	page := Entry{Capability: admissionv0.CapabilityHandwritingReading, Models: []Model{opus, haiku}, Stage: StageAuthor, Report: word, Prompt: workbook.Fingerprint()}
	essayRead := Entry{Capability: admissionv0.CapabilityHandwritingReading, Models: []Model{haiku}, Stage: StageAuthor, Report: word, Prompt: essay.ReadFingerprint()}
	rubric := Entry{Capability: admissionv0.CapabilityRubricAssessment, Models: []Model{haiku}, Stage: StageAuthor, Report: word, Prompt: essay.TextFingerprint()}
	if f := listFaultsOn([]Entry{page, essayRead, rubric}, "2027-01-17"); len(f) != 0 {
		t.Errorf("good entries by the author's word have faults on their last day: %v", f)
	}
	if f := listFaultsOn([]Entry{page}, "2027-01-18"); len(f) != 1 || !strings.Contains(f[0], "Д17") {
		t.Errorf("a word past its last day: %v", f)
	}
	for name, change := range map[string]func(*Entry){
		"the stage full":     func(e *Entry) { e.Stage = StageFull },
		"a report too":       func(e *Entry) { e.Report.SHA256 = strings.Repeat("ab", 32) },
		"no last day":        func(e *Entry) { e.Report.Until = "" },
		"no day":             func(e *Entry) { e.Report.Date = "" },
		"a page of one":      func(e *Entry) { e.Models = []Model{haiku} },
		"an essay of a pair": func(e *Entry) { e.Prompt = essay.ReadFingerprint() },
		"Haiku first":        func(e *Entry) { e.Models = []Model{haiku, opus} },
	} {
		e := page
		change(&e)
		if len(listFaultsOn([]Entry{e}, "2026-11-17")) == 0 {
			t.Errorf("by the author's word, %s: no fault", name)
		}
	}
	for name, e := range map[string]Entry{
		"a rubric of two":            {Capability: rubric.Capability, Models: []Model{opus, haiku}, Stage: StageAuthor, Report: word, Prompt: rubric.Prompt},
		"a rubric of another prompt": {Capability: rubric.Capability, Models: []Model{haiku}, Stage: StageAuthor, Report: word, Prompt: essay.ReadFingerprint()},
		"a report with a last day":   {Capability: rubric.Capability, Models: []Model{haiku}, Stage: StageAuthor, Report: Evidence{Set: "s", File: "f", SHA256: strings.Repeat("ab", 32), Date: "2026-11-17", Until: "2027-01-17"}, Prompt: rubric.Prompt},
	} {
		if len(listFaultsOn([]Entry{e}, "2026-11-17")) == 0 {
			t.Errorf("%s: no fault", name)
		}
	}
}

// An entry admitted by the author's word serves the least stage author alone, whatever stage it
// names; the platform's list admits the workbook, the essay and the rubric so at the stage author
// and none of them at full.
func TestSelectByWord(t *testing.T) {
	table, err := Platform()
	if err != nil {
		t.Fatal(err)
	}
	f, err := ReadFacts(table, []byte("anthropic:\n  data_processing_agreement: [adult]\n  minimal_retention: [adult]\n"))
	if err != nil {
		t.Fatal(err)
	}
	route, adult := admissionv0.AiRoutePlatformAi, admissionv0.AgeCategoryAdult
	full := Entry{Capability: admissionv0.CapabilityHint, Models: []Model{opus}, Stage: StageFull, Report: Evidence{Word: "the author's word", Date: "2026-11-17", Until: "2027-01-17"}}
	if e, a := table.selectFrom([]Entry{full}, f, admissionv0.CapabilityHint, route, adult, StageFull, nil); a.Admitted || *a.Reason != ReasonModelNotAdmitted || e.Models != nil {
		t.Errorf("a word of the stage full at the least stage full: %+v, %+v", e, a)
	}
	if e, a := table.selectFrom([]Entry{full}, f, admissionv0.CapabilityHint, route, adult, StageAuthor, nil); !a.Admitted || len(e.Models) != 1 {
		t.Errorf("a word at the least stage author: %+v, %+v", e, a)
	}
	for _, c := range []struct {
		capability admissionv0.Capability
		prompt     string
		models     []Model
	}{
		{admissionv0.CapabilityHandwritingReading, workbook.Fingerprint(), []Model{opus, haiku}},
		{admissionv0.CapabilityHandwritingReading, essay.ReadFingerprint(), []Model{haiku}},
		{admissionv0.CapabilityRubricAssessment, essay.TextFingerprint(), []Model{haiku}},
	} {
		e, a := table.SelectPrompt(f, c.capability, route, adult, StageAuthor, nil, c.prompt)
		if !a.Admitted || !slices.Equal(e.Models, c.models) || !e.Report.ByWord() || e.Report.Until != "2027-01-17" {
			t.Errorf("%s at the stage author: %+v, %+v", c.prompt, e, a)
		}
		if e, a := table.SelectPrompt(f, c.capability, route, adult, StageFull, nil, c.prompt); a.Admitted || *a.Reason != ReasonModelNotAdmitted || e.Models != nil {
			t.Errorf("%s at the stage full: %+v, %+v", c.prompt, e, a)
		}
	}
	// The first entry of the reading of handwriting is still the sheet's pair, admitted by its run.
	if e, a := table.Select(f, admissionv0.CapabilityHandwritingReading, route, adult, StageAuthor, nil); !a.Admitted || e.Prompt != reading.Fingerprint() || e.Report.ByWord() {
		t.Errorf("the first reading: %+v, %+v", e, a)
	}
}

// listFaults are what makes a list of admitted models wrong, entry by entry.
func listFaults(list []Entry) []string {
	return listFaultsOn(list, time.Now().UTC().Format(time.DateOnly))
}

// listFaultsOn is listFaults on the day named: an entry admitted by the author's word is a fault
// once the last day of its word has passed, so that the list fails here until a run admits it by
// the rule or it leaves the list.
func listFaultsOn(list []Entry, today string) []string {
	var out []string
	for i, e := range list {
		if len(e.Models) == 0 || (e.Stage != StageAuthor && e.Stage != StageFull) || !dateForm.MatchString(e.Report.Date) ||
			!e.Report.ByWord() && (e.Report.Set == "" || e.Report.File == "" || !sha256Form.MatchString(e.Report.SHA256) || e.Report.Until != "") {
			out = append(out, fmt.Sprintf("entry %d is %+v", i, e))
		}
		if e.Report.ByWord() {
			switch {
			case e.Stage != StageAuthor || e.Report.Set != "" || e.Report.File != "" || e.Report.SHA256 != "" || !dateForm.MatchString(e.Report.Until):
				out = append(out, fmt.Sprintf("entry %d, by the author's word, is %+v", i, e))
			case e.Report.Until < today:
				out = append(out, fmt.Sprintf("entry %d: the author's word held until %s; admit it by a run on Haiku or take it off the list (Д17)", i, e.Report.Until))
			}
		}
		for _, m := range e.Models {
			if m.Provider != "anthropic" {
				out = append(out, fmt.Sprintf("entry %d: %s of %s, whose adapter the list does not check", i, m.ID, m.Provider))
				continue
			}
			known := slices.ContainsFunc(catalog.Models, func(c catalog.Model) bool { return c.ID == m.ID })
			if !known || !slices.Contains(catalog.Efforts, m.Effort) || slices.Contains(requiresRetention, m.ID) {
				out = append(out, fmt.Sprintf("entry %d: %s is no model the list may hold", i, m.Name()))
			}
		}
		// The words of a task of a module are one model's, and those of a task from paper — the hint
		// put another way, without the work — a cascade of one to three attempts, each with its prompt
		// now.
		words := e.Capability == admissionv0.CapabilitySolutionAnalysis || e.Capability == admissionv0.CapabilityHint
		ofModule := e.Prompt == hint.Fingerprint() && len(e.Models) == 1
		ofPaper := e.Capability == admissionv0.CapabilityHint && e.Prompt == paperwords.Fingerprint() && len(e.Models) >= 1 && len(e.Models) <= 3
		if words && !ofModule && !ofPaper {
			out = append(out, fmt.Sprintf("entry %d of the words: %d models, prompt %s, the hint's now %s and the words of paper's %s", i, len(e.Models), e.Prompt, hint.Fingerprint(), paperwords.Fingerprint()))
		}
		if e.Capability == admissionv0.CapabilityCorrectAnswerWords && (len(e.Models) != 1 || e.Prompt != correct.Fingerprint()) {
			out = append(out, fmt.Sprintf("entry %d of the words after a correct answer: %d models, prompt %s, theirs now %s", i, len(e.Models), e.Prompt, correct.Fingerprint()))
		}
		// A reading is of a sheet or of a workbook page, by a pair, or of an essay, by one reader, each
		// with its prompt now; the assessment by a rubric is one model's. A call of a task from paper
		// is a cascade of one to three attempts.
		cascade := len(e.Models) >= 1 && len(e.Models) <= 3
		if e.Capability == admissionv0.CapabilityHandwritingReading {
			pair := e.Prompt == reading.Fingerprint() || e.Prompt == workbook.Fingerprint()
			one := e.Prompt == essay.ReadFingerprint()
			strongFirst := len(e.Models) == 2 && e.Models[0].ID == "claude-opus-5-5"
			if pair && !strongFirst || one && len(e.Models) != 1 || !pair && !one {
				out = append(out, fmt.Sprintf("entry %d of reading: %d models, prompt %s, the readings' now %s, %s and %s", i, len(e.Models), e.Prompt,
					reading.Fingerprint(), workbook.Fingerprint(), essay.ReadFingerprint()))
			}
		}
		if e.Capability == admissionv0.CapabilityTaskFinding && (!cascade || e.Prompt != finding.Fingerprint()) {
			out = append(out, fmt.Sprintf("entry %d of the finding: %d models, prompt %s, theirs now %s", i, len(e.Models), e.Prompt, finding.Fingerprint()))
		}
		if e.Capability == admissionv0.CapabilityStatementReading && (!cascade || e.Prompt != statement.Fingerprint()) {
			out = append(out, fmt.Sprintf("entry %d of the statement: %d models, prompt %s, theirs now %s", i, len(e.Models), e.Prompt, statement.Fingerprint()))
		}
		if e.Capability == admissionv0.CapabilityTaskChecking && (!cascade || e.Prompt != checking.Fingerprint()) {
			out = append(out, fmt.Sprintf("entry %d of the Check: %d models, prompt %s, theirs now %s", i, len(e.Models), e.Prompt, checking.Fingerprint()))
		}
		if e.Capability == admissionv0.CapabilityRubricAssessment && (len(e.Models) != 1 || e.Prompt != essay.TextFingerprint()) {
			out = append(out, fmt.Sprintf("entry %d of the rubric: %d models, prompt %s, the assessment's now %s", i, len(e.Models), e.Prompt, essay.TextFingerprint()))
		}
	}
	return out
}

// SelectPrompt chooses only among the entries that measured the prompt named: the pair of the
// reading of a sheet admits nothing of another prompt.
func TestSelectPrompt(t *testing.T) {
	table, err := Platform()
	if err != nil {
		t.Fatal(err)
	}
	f, err := ReadFacts(table, []byte("anthropic:\n  data_processing_agreement: [adult]\n  minimal_retention: [adult]\n"))
	if err != nil {
		t.Fatal(err)
	}
	c, route, cat := admissionv0.CapabilityHandwritingReading, admissionv0.AiRoutePlatformAi, admissionv0.AgeCategoryAdult
	if e, a := table.SelectPrompt(f, c, route, cat, StageAuthor, nil, reading.Fingerprint()); !a.Admitted || len(e.Models) != 2 {
		t.Errorf("the prompt of the reading of a sheet: %+v, %+v", e, a)
	}
	if _, a := table.SelectPrompt(f, c, route, cat, StageAuthor, nil, "another prompt"); a.Admitted {
		t.Errorf("another prompt is admitted: %+v", a)
	}
}
