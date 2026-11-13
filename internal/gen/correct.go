package gen

import (
	"errors"
	"fmt"
	"math/rand/v2"

	"github.com/altnovatrail/tutor/internal/correct"
	"github.com/altnovatrail/tutor/internal/hint"
	"github.com/altnovatrail/tutor/internal/sample"
)

// The set of the words after a correct answer (T143; docs/architecture.md, 3.4, "The set of the
// words after a correct answer"): the sheets whose answer is right of a plan of lines, written again
// as a plan of parts, ten of them once more with a note at the side, as in the set of the words of
// the hint (Words), and the cases on them.

// CorrectNotes is how many sheets of the set carry a note at the side: two of each kind of note, one
// in Russian and one in English. CorrectNoTechniques is how many of the other sheets are of a task
// that names no technique: a task of a module may name none (the module format of spec, section
// "Techniques").
const (
	CorrectNotes        = 10
	CorrectNoTechniques = 10
)

// Correct is the set of the words after a correct answer of a plan of lines and its bank, from the
// seed: a plan of parts of every sheet of the plan whose answer is right, its lines the parts of the
// main column, in their order, and then CorrectNotes of its sheets of the scenario correct, with no
// erasure, once more, each with a note at the side after its last line, two of each kind, one in
// Russian and one in English; and the cases on it, named for the plan's file:
//   - a case in each language with the work on every sheet without a note;
//   - a case without the work, as for a task whose answer slots are bound choice alone, on every
//     sheet without a note, in Russian and English in turn;
//   - a case in each language on every sheet with a note.
//
// The techniques are those of the task's kind (techniques), but none on CorrectNoTechniques of the
// sheets without a note. The same seed gives the same set.
func Correct(plan *sample.Plan, bank *sample.Bank, planFile string, seed uint64) (*sample.Plan, *correct.Cases, error) {
	if plan.HasParts() {
		return nil, nil, errors.New("gen: the set of the words after a correct answer is made of a plan of lines")
	}
	var right, plain []int
	for i, sh := range plan.Sheets {
		if sh.Expected.Outcome != sample.OutcomeCorrect {
			continue
		}
		if bank.Task(sh.Task) == nil {
			return nil, nil, fmt.Errorf("gen: the task %s of sheet %d is not in the bank", sh.Task, sh.Number)
		}
		right = append(right, i)
		if sh.Scenario == sample.ScenarioCorrect && sh.Erasure == nil {
			plain = append(plain, i)
		}
	}
	if len(right) <= CorrectNoTechniques || len(plain) < CorrectNotes {
		return nil, nil, fmt.Errorf("gen: %d sheets whose answer is right, %d of them of the scenario correct: the set needs more than %d and %d", len(right), len(plain), CorrectNoTechniques, CorrectNotes)
	}
	r := rand.New(rand.NewPCG(seed, 0xc0cc))
	out := partsPlan()
	for _, i := range right {
		addParts(out, plan.Sheets[i], "")
	}
	type noted struct {
		kind hint.NoteKind
		lang sample.Lang
	}
	var marked []noted
	for i, pick := range r.Perm(len(plain))[:CorrectNotes] {
		n := noted{hint.NoteKinds()[i/2], hint.Langs()[i%2]}
		addParts(out, plan.Sheets[plain[pick]], notes[n.kind][n.lang])
		marked = append(marked, n)
	}
	bare := map[int]bool{}
	for _, i := range r.Perm(len(right))[:CorrectNoTechniques] {
		bare[i+1] = true
	}
	cs := &correct.Cases{Format: correct.CasesFormat, Version: correct.CasesVersion, Plan: planFile, Seed: seed}
	kase := func(kind correct.CaseKind, sh *sample.PlanSheet, lang sample.Lang) correct.Case {
		k := correct.Case{ID: fmt.Sprintf("c%03d", len(cs.Cases)+1), Kind: kind, Sheet: sh.Number, Lang: lang, Work: kind != correct.CaseChoice, Techniques: []string{}}
		if !bare[sh.Number] {
			k.Techniques = append(k.Techniques, techniques[bank.Task(sh.Task).Kind]...)
		}
		return k
	}
	base := out.Sheets[:len(right)]
	for i := range base {
		for _, lang := range hint.Langs() {
			cs.Cases = append(cs.Cases, kase(correct.CaseWork, &base[i], lang))
		}
	}
	for i := range base {
		cs.Cases = append(cs.Cases, kase(correct.CaseChoice, &base[i], hint.Langs()[i%2]))
	}
	for i, n := range marked {
		sh := &out.Sheets[len(right)+i]
		for _, lang := range hint.Langs() {
			k := kase(correct.CaseNote, sh, lang)
			k.Note, k.NoteKind = notes[n.kind][n.lang], n.kind
			cs.Cases = append(cs.Cases, k)
		}
	}
	if err := out.CheckWithBank(bank); err != nil {
		return nil, nil, fmt.Errorf("gen: the plan of the set of the words after a correct answer: %w", err)
	}
	if err := cs.Check(out, bank); err != nil {
		return nil, nil, fmt.Errorf("gen: the cases of the set of the words after a correct answer: %w", err)
	}
	return out, cs, nil
}
