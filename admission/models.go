package admission

import (
	"slices"

	"github.com/altnovatrail/spec/types/admissionv0"
)

// ReasonModelNotAdmitted is the refusal that comes after the table's: the table admits the request,
// but no model of the capability is admitted for it — none passed tutor's rule of admission on the
// capability's eval set.
const ReasonModelNotAdmitted admissionv0.AnswerReason = "model_not_admitted"

// Stage is how far a model passed the rule of admission: StageFull, the target of the first
// slice — the upper bound of the share of false alarms no more than 1 % —, or StageAuthor, no false
// alarm and no false "correct" on the author's own handwriting, the rung a deployment names for
// adults: on novatrail "author", and "full" the target measured on the samples of adult volunteers.
type Stage string

// The stages of admission.
const (
	StageAuthor Stage = "author"
	StageFull   Stage = "full"
)

// rank orders the stages: full above author, and anything else below both, so that an entry of no
// stage is never chosen and a least stage of none chooses only full.
func (s Stage) rank() int {
	switch s {
	case StageAuthor:
		return 1
	case StageFull:
		return 2
	}
	return 0
}

// Model is a model as an adapter of tutor calls it: its provider, by the code of the admission
// table, its exact ID, the effort it spends and whether it answers without thinking. It names the
// model of an adapter rather than copying it: its price and its name for people stay there.
type Model struct {
	Provider   admissionv0.Provider
	ID         string
	Effort     string
	NoThinking bool
}

// Name is the model as a runner's report names it: its ID and effort, and "no-thinking" when it
// answers without thinking, such as "claude-opus-5-5 high".
func (m Model) Name() string {
	name := m.ID + " " + m.Effort
	if m.NoThinking {
		name += " no-thinking"
	}
	return name
}

// Evidence is the runner's report that admitted an entry: the name of its eval set, the name of its
// file, the SHA-256 of the file's bytes and the day of the run. A report of samples of measurement
// stays where they lie, out of git, and the entry names it so. An entry the author admitted by
// their word, while eval before admission is suspended, has no report: Word is that word, Date the
// day it was given and Until the last day it holds. Such an entry serves only the least stage
// author, whatever its own stage, and lapses with Until: by then a run on Haiku admits it by the
// rule, or the entry leaves the list. The package has no clock, so Select does not refuse it after
// Until: the entry chosen carries its evidence, and whoever holds a clock, as api, can read Until
// there.
type Evidence struct {
	Set, File, SHA256, Date string
	Word, Until             string
}

// ByWord is whether the entry was admitted by the author's word, without a report.
func (e Evidence) ByWord() bool { return e.Word != "" }

// Entry is a line of the list of admitted models: a capability, the models that serve it — a
// reading of handwriting has its pair, the strong reader first —, the stage they passed, the report
// that admitted them and the fingerprint of the prompt that report measured: another prompt is
// another reading, and admits nothing until it is measured anew.
type Entry struct {
	Capability admissionv0.Capability
	Models     []Model
	Stage      Stage
	Report     Evidence
	Prompt     string
}

// Admitted is the list of admitted models, by capability in the order they are chosen: a model is in
// it only once it passed the rule of admission, or by the author's word while eval before admission
// is suspended, until the day its evidence names. A model that needs its provider to keep the work,
// such as one that needs 30-day retention at Anthropic, is not in it.
var Admitted = []Entry{
	// The pair of the first app, on the author's own handwriting, with no false alarm and no false
	// "correct", on version 2 of the reading's prompt. In the library's mode of one reader, Haiku at
	// medium reads alone (tutor.ReadingSingle), found here by its provider, ID and effort.
	{
		Capability: admissionv0.CapabilityHandwritingReading,
		Models: []Model{
			{Provider: "anthropic", ID: "claude-opus-5-5", Effort: "high"},
			{Provider: "anthropic", ID: "claude-haiku-5-5", Effort: "medium"},
		},
		Stage: StageAuthor,
		Report: Evidence{
			Set:    "sheets-v1+free-v1+free-messy-v1+author-v1-recorded",
			File:   "samples/runs/t115-admission-2.opus-high+haiku-medium.report.json",
			SHA256: "284d8eacaa7337210ecb589449f0315bbb65ee20cb78fe7ae6fe0b9452e7bb2b",
			Date:   "2026-11-08",
		},
		Prompt: "c693bfa87760abec24fcf04073f8c786c682ba13d02569c9ec480da1f7e64d9d",
	},
	// The words of the tutor — the analysis of the steps and the hint, one call of one model —, on
	// the author's sheets with a mistake and the set of the words: version 3 of the hint's prompt,
	// judged by Opus 5.5 at high on version 3 of the judge's prompt, which passed all its control
	// phrases.
	{
		Capability: admissionv0.CapabilitySolutionAnalysis,
		Models:     []Model{{Provider: "anthropic", ID: "claude-opus-5-5", Effort: "high"}},
		Stage:      StageAuthor,
		Report:     wordsEvidence,
		Prompt:     wordsPrompt,
	},
	{
		Capability: admissionv0.CapabilityHint,
		Models:     []Model{{Provider: "anthropic", ID: "claude-opus-5-5", Effort: "high"}},
		Stage:      StageAuthor,
		Report:     wordsEvidence,
		Prompt:     wordsPrompt,
	},
	// The words after a correct answer — the praise of a step and a question on the technique —, on
	// their synthetic set, with the work, without it and with a note, of version 1 of their prompt,
	// judged by Opus 5.5 at high on version 4 of the judge's prompt, which passed all its control
	// phrases.
	{
		Capability: admissionv0.CapabilityCorrectAnswerWords,
		Models:     []Model{{Provider: "anthropic", ID: "claude-opus-5-5", Effort: "high"}},
		Stage:      StageAuthor,
		Report: Evidence{
			Set:    "correct-v1",
			File:   "synthetic/t144/correct.admit.md",
			SHA256: "0998a69466f62bc6c167d1cf71da6c737ee3e96a94e02d6fd13fd1b57e6b969b",
			Date:   "2026-11-14",
		},
		Prompt: "ee01e82ada7c347c45f56828d86342897569ec7517fc52265963a76f31def1d9",
	},
	// The reading of a workbook page, by the sheet's pair on version 3 of the workbook's prompt,
	// and in the mode of one reader by Haiku at medium alone, as a sheet; the reading of an essay, one
	// reader, Haiku at medium, on version 2 of its prompt; and the assessment of the text read by the
	// rubric, Haiku at medium, on version 3 of its prompt. All three by the author's word, without a
	// run, until the day authorsWord names. They follow the sheet's pair, so that the first entry of
	// the reading of handwriting, Choose's, stays the sheet's.
	{
		Capability: admissionv0.CapabilityHandwritingReading,
		Models: []Model{
			{Provider: "anthropic", ID: "claude-opus-5-5", Effort: "high"},
			{Provider: "anthropic", ID: "claude-haiku-5-5", Effort: "medium"},
		},
		Stage:  StageAuthor,
		Report: authorsWord,
		Prompt: "26925af5e000d1c9cb30571f1e4f1b845852dedbb4da056b3b360a9eae3a5789",
	},
	{
		Capability: admissionv0.CapabilityHandwritingReading,
		Models:     []Model{{Provider: "anthropic", ID: "claude-haiku-5-5", Effort: "medium"}},
		Stage:      StageAuthor,
		Report:     authorsWord,
		Prompt:     "49cc90421a42ef4d9b3e85e753288a871b0b23f7e94b6aa422928249f7222440",
	},
	{
		Capability: admissionv0.CapabilityRubricAssessment,
		Models:     []Model{{Provider: "anthropic", ID: "claude-haiku-5-5", Effort: "medium"}},
		Stage:      StageAuthor,
		Report:     authorsWord,
		Prompt:     "a9bf54404fbb8215ac2def2ec16b0a053846ffb9d88ced72ca7d2560b0246cba",
	},
	// The three calls of a task from paper, by the author's word, without a run, until the day
	// paperWord names, each a cascade of three attempts, the cheapest first: Haiku 5.5 at high, then
	// Sonnet 5.5 at medium, then Opus 5.5 at medium, all with thinking. The finding of the tasks on
	// the photo of a page, on version 5 of its prompt, which first asks which way up the page is and
	// gives the box of each task's instruction, with the learner's request about the page, ran on
	// the stub alone, the cases of several tasks of the set of a task from paper, and has no set of
	// whole pages, Д21.
	{
		Capability: admissionv0.CapabilityTaskFinding,
		Models:     paperCascade,
		Stage:      StageAuthor,
		Report:     paperWord,
		Prompt:     "f53df0bb4a622cb83a633e8a4450244a066b42a500dc2d5ed958215d97195dd5",
	},
	// The statement read into its tasks and their formulas for Recognise, on version 3 of its prompt,
	// which writes a fraction and a power LaTeX's way and leaves out a task's label and separator;
	// its set ran on the stub alone, and the paid run is Д22.
	{
		Capability: admissionv0.CapabilityStatementReading,
		Models:     paperCascade,
		Stage:      StageAuthor,
		Report:     paperWord,
		Prompt:     "6454ba0b86393043e8c7bad38335e51cd6098a3bb9932d7b27c3ea75fa259c10",
	},
	// The Check, the work judged with no key in one call — each task's verdict, the place of each
	// mistake, the tutor's words and the ladder of hints —, on version 1 of its prompt. Its set ran
	// on the stub alone, and the paid run is Д22.
	{
		Capability: admissionv0.CapabilityTaskChecking,
		Models:     paperCascade,
		Stage:      StageAuthor,
		Report:     paperWord,
		Prompt:     "d18478adbd9389bbead7230df9e757ea930493ae2274fbc7ae24e93e5ca24d59",
	},
	// The tutor's words of a rung of the Check's ladder put another way, without the work, on
	// version 2 of their prompt. By the author's word, without a set and without a run, which are Д24.
	{
		Capability: admissionv0.CapabilityHint,
		Models:     paperCascade,
		Stage:      StageAuthor,
		Report:     paperWordsWord,
		Prompt:     paperWordsPrompt,
	},
}

// paperWordsWord is the author's word that admitted the words of a task from paper, under the same
// suspension as paperWord, to the same last day, and paperWordsPrompt the fingerprint of their
// prompt, version 2.
var (
	paperWordsWord = Evidence{
		Word:  "the author's word, while eval before admission is suspended",
		Date:  "2026-11-20",
		Until: "2027-01-17",
	}
	paperWordsPrompt = "90f888684b3a0fa3356de15c8a897a4f0a62de153b1d20108fcbd8df526e6282"
)

// authorsWord is the evidence of the entries the author admitted without eval, while eval before
// admission is suspended. The suspension ends sooner, with the first code of api invite to another
// adult or the first child, which no clock here sees.
var authorsWord = Evidence{
	Word:  "the author's word, while eval before admission is suspended",
	Date:  "2026-11-17",
	Until: "2027-01-17",
}

// paperCascade are the attempts of each call of a task from paper, in their order.
var paperCascade = []Model{
	{Provider: "anthropic", ID: "claude-haiku-5-5", Effort: "high"},
	{Provider: "anthropic", ID: "claude-sonnet-5-5", Effort: "medium"},
	{Provider: "anthropic", ID: "claude-opus-5-5", Effort: "medium"},
}

// paperWord is the author's word that admitted the cascade of a task from paper, under the same
// suspension as authorsWord, to the same last day.
var paperWord = Evidence{
	Word:  "the author's word, while eval before admission is suspended",
	Date:  "2026-11-19",
	Until: "2027-01-17",
}

// wordsEvidence is the output of the rule of admission of the words that admitted them, and
// wordsPrompt the fingerprint of the hint's prompt it measured, version 3.
var (
	wordsEvidence = Evidence{
		Set:    "sheets-v1+free-messy-v1+author-v1-recorded+words-v1",
		File:   "samples/runs/t127-words.admit.md",
		SHA256: "1e49fa51007867292f868869ce4888cfbfac6e6d3b45dbee86632998a72bd68a",
		Date:   "2026-11-11",
	}
	wordsPrompt = "7d598021bf716157a54b4e60ac2c5583e0ea8429c43e9e0a30805ef99afdeeeb"
)

// Select chooses the models of a call: the first entry of the capability in Admitted, of the least
// stage or above, whose every model the table admits for the route and the age category, with the
// table's answer for its first model. Without such an entry it answers the first refusal of the
// table for an entry of the capability, or, when the table refused none, ReasonModelNotAdmitted.
// The least stage is the deployment's: StageAuthor where its operator names it, as novatrail does
// for adults, and StageFull otherwise; any other value chooses only StageFull. An entry
// admitted by the author's word serves StageAuthor alone. Providers, when not nil, are the only
// providers whose models may serve: an own key is one provider's, and on platform AI a provider
// without the platform's key cannot be called. When no entry of them is left, the answer is the
// table's for the first of them that it refuses, as the table's reasons come before the model's.
// api calls it before an attempt, and tutor before the call: with the same table, facts, request,
// least stage and providers it chooses the same.
func (t *Table) Select(f Facts, c admissionv0.Capability, route admissionv0.AiRoute, category admissionv0.AgeCategory, least Stage, providers []admissionv0.Provider) (Entry, Answer) {
	return t.selectFrom(Admitted, f, c, route, category, least, providers)
}

// SelectPrompt is Select of the entries whose prompt is the one named: a call that asks another
// prompt than the one an entry measured is another reading, and that entry admits nothing of it, such
// as the reading of a workbook page beside the reading of a sheet, of one capability.
func (t *Table) SelectPrompt(f Facts, c admissionv0.Capability, route admissionv0.AiRoute, category admissionv0.AgeCategory, least Stage, providers []admissionv0.Provider, prompt string) (Entry, Answer) {
	var list []Entry
	for _, e := range Admitted {
		if e.Prompt == prompt {
			list = append(list, e)
		}
	}
	return t.selectFrom(list, f, c, route, category, least, providers)
}

// selectFrom is Select over a list of its own.
func (t *Table) selectFrom(list []Entry, f Facts, c admissionv0.Capability, route admissionv0.AiRoute, category admissionv0.AgeCategory, least Stage, providers []admissionv0.Provider) (Entry, Answer) {
	floor := StageFull.rank()
	if least == StageAuthor {
		floor = StageAuthor.rank()
	}
	var refused *Answer
	for _, e := range list {
		if e.Capability != c || len(e.Models) == 0 || e.Stage.rank() < floor || e.Report.ByWord() && least != StageAuthor {
			continue
		}
		if providers != nil && slices.ContainsFunc(e.Models, func(m Model) bool { return !slices.Contains(providers, m.Provider) }) {
			continue
		}
		var first Answer
		admitted := true
		for i, m := range e.Models {
			a := t.Admit(Request{Capability: c, AiRoute: route, Provider: m.Provider, AgeCategory: category}, f)
			if i == 0 {
				first = a
			}
			if !a.Admitted {
				// One reader of a pair refused refuses the pair: neither call goes.
				admitted = false
				if refused == nil {
					refused = &a
				}
				break
			}
		}
		if admitted {
			e.Models = slices.Clone(e.Models)
			return e, first
		}
	}
	if refused != nil {
		return Entry{}, *refused
	}
	for _, p := range providers {
		if a := t.Admit(Request{Capability: c, AiRoute: route, Provider: p, AgeCategory: category}, f); !a.Admitted {
			return Entry{}, a
		}
	}
	return Entry{}, Answer{Answer: admissionv0.Answer{Reason: reason(ReasonModelNotAdmitted)}, Version: t.version}
}
