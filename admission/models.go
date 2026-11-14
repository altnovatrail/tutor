package admission

import (
	"slices"

	"github.com/altnovatrail/spec/types/admissionv0"
)

// ReasonModelNotAdmitted is the refusal that comes after the table's (R96 spec; Д98 RUN.md spec):
// the table admits the request, but no model of the capability is admitted for it — none passed
// tutor's rule of admission on the capability's eval set (R07 spec; R80).
const ReasonModelNotAdmitted admissionv0.AnswerReason = "model_not_admitted"

// Stage is how far a model passed the rule of admission (R80): StageFull, the target of the first
// slice — the upper bound of the share of false alarms no more than 1 % (product 10 spec) —, or
// StageAuthor, no false alarm and no false "correct" on the author's own handwriting, enough for
// the first app, where the author alone learns (R76 spec), and for nothing more.
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
// stays where they lie, out of git (privacy 9 spec, П-10 and П-18), and the entry names it so.
type Evidence struct {
	Set, File, SHA256, Date string
}

// Entry is a line of the list of admitted models: a capability, the models that serve it — a
// reading of handwriting has its pair, the strong reader first (R68 spec) —, the stage they passed,
// the report that admitted them and the fingerprint of the prompt that report measured: another
// prompt is another reading, and admits nothing until it is measured anew (R70 spec).
type Entry struct {
	Capability admissionv0.Capability
	Models     []Model
	Stage      Stage
	Report     Evidence
	Prompt     string
}

// Admitted is the list of admitted models, by capability in the order they are chosen (R07 spec):
// a model is in it only once it passed the rule of admission. A model that needs its provider to
// keep the work, such as one that needs 30-day retention at Anthropic, is not in it, since the
// cells of platform AI rest on minimal retention (Д98 RUN.md spec).
var Admitted = []Entry{
	// The pair of the first app, on the author's own handwriting: 60 sheets, 3 repeats, no false
	// alarm on 35 answer slots and no false "correct" on 25 (T126; R80 and its amendment of
	// 2026-11-08), measured again with version 2 of the reading's prompt (T115; R82). The report
	// lies in samples/, out of git.
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
	// The words of the tutor — the analysis of the steps and the hint, one call of one model —, on the
	// author's sheets with a mistake and the set of the words of T118: 300 hints of version 3 of the
	// hint's prompt, judged by Opus 5.5 at high on version 3 of the judge's prompt, which passed all its
	// control phrases (T127; R86, R100). The output of the rule lies in samples/, out of git.
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
	// their synthetic set: 110 cases on 40 sheets whose answer is right, with the work, without it and
	// with a note, of version 1 of their prompt, judged by Opus 5.5 at high on version 4 of the judge's
	// prompt, which passed all its 68 control phrases (T144; R112, R113). The output of the rule lies
	// in synthetic/, out of git.
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
}

// wordsEvidence is the output of the rule of admission of the words that admitted them (T127), and
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
// The least stage is the deployment's: StageAuthor only where the author alone learns, as in the
// first app (R76 spec), and StageFull anywhere else; any other value chooses only StageFull.
// Providers, when not nil, are the only providers whose models may serve: an own key is one
// provider's, and on platform AI a provider without the platform's key cannot be called (T114).
// When no entry of them is left, the answer is the table's for the first of them that it refuses,
// as the table's reasons come before the model's. api calls it before an attempt, and tutor before
// the call: with the same table, facts, request, least stage and providers it chooses the same.
func (t *Table) Select(f Facts, c admissionv0.Capability, route admissionv0.AiRoute, category admissionv0.AgeCategory, least Stage, providers []admissionv0.Provider) (Entry, Answer) {
	return t.selectFrom(Admitted, f, c, route, category, least, providers)
}

// SelectPrompt is Select of the entries whose prompt is the one named: a call that asks another
// prompt than the one an entry measured is another reading, and that entry admits nothing of it (R70
// spec), such as the reading of a workbook page beside the reading of a sheet, of one capability.
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
		if e.Capability != c || len(e.Models) == 0 || e.Stage.rank() < floor {
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
				// One reader of a pair refused refuses the pair: neither call goes (R68 spec).
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
