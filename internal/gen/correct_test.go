package gen_test

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/internal/correct"
	"github.com/altnovatrail/tutor/internal/gen"
	"github.com/altnovatrail/tutor/internal/hint"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/synth"
)

// The files of the set of the words after a correct answer are what the generator makes of
// lines-gen-1 and the seed 143, byte for byte: tutor-eval gen correct makes them again (T143).
func TestCorrectFiles(t *testing.T) {
	plan, bank := linesGen1(t)
	pl, cs, err := gen.Correct(plan, bank, "correct-sheets.json", 143)
	if err != nil {
		t.Fatal(err)
	}
	for file, v := range map[string]any{"correct-sheets.json": pl, "correct-cases.json": cs} {
		want, err := sample.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if got, err := os.ReadFile(filepath.Join("..", "..", "eval", file)); err != nil || !bytes.Equal(got, want) {
			t.Errorf("eval/%s is not what the generator makes of its seed: %v", file, err)
		}
	}
}

// The set takes every sheet of the plan whose answer is right as parts of the main column, then ten
// plain sheets of the scenario correct again with a note at the side, two of each kind, one in each
// language. Each sheet drawn passes its check. The cases: two with the work and one without it on
// every sheet without a note, the languages of the choices in turn, two on every sheet with a note;
// ten sheets without a note name no technique.
func TestCorrect(t *testing.T) {
	plan, bank := linesGen1(t)
	pl, cs, err := gen.Correct(plan, bank, "p.json", 143)
	if err != nil {
		t.Fatal(err)
	}
	right := 0
	for _, sh := range plan.Sheets {
		if sh.Expected.Outcome == sample.OutcomeCorrect {
			right++
		}
	}
	if len(pl.Sheets) != right+gen.CorrectNotes || !pl.HasParts() {
		t.Fatalf("the plan has %d sheets of %d right, of parts %t", len(pl.Sheets), right, pl.HasParts())
	}
	for i, sh := range pl.Sheets {
		noted := i >= right
		if noted != (sh.Parts[len(sh.Parts)-1].Place == sample.PlaceRight) || sh.Expected.Outcome != sample.OutcomeCorrect {
			t.Errorf("sheet %d: %+v", sh.Number, sh)
		}
		if noted && (sh.Scenario != sample.ScenarioCorrect || sh.Erasure != nil) {
			t.Errorf("sheet %d with a note is %s", sh.Number, sh.Scenario)
		}
		if err := synth.Sheet(&pl.Sheets[i], 143).CheckWithBank(bank); err != nil {
			t.Errorf("sheet %d drawn: %v", sh.Number, err)
		}
	}
	bare, notes, choices := map[int]bool{}, map[hint.NoteKind]int{}, map[sample.Lang]int{}
	for _, k := range cs.Cases {
		if len(k.Techniques) == 0 {
			bare[k.Sheet] = true
		}
		switch k.Kind {
		case correct.CaseNote:
			notes[k.NoteKind]++
		case correct.CaseChoice:
			choices[k.Lang]++
		}
	}
	if len(bare) != gen.CorrectNoTechniques || slices.ContainsFunc(cs.Cases, func(k correct.Case) bool { return bare[k.Sheet] && k.Kind == correct.CaseNote }) {
		t.Errorf("%d sheets name no technique: %v", len(bare), bare)
	}
	for _, kind := range hint.NoteKinds() {
		if notes[kind] != 4 {
			t.Errorf("%d cases of the note %s", notes[kind], kind)
		}
	}
	if choices[sample.LangRussian] != right/2 || choices[sample.LangEnglish] != right-right/2 {
		t.Errorf("the choices are in %v", choices)
	}
	other, _, _ := gen.Correct(plan, bank, "p.json", 144)
	if same, _, _ := gen.Correct(plan, bank, "p.json", 143); !slices.EqualFunc(same.Sheets, pl.Sheets, func(a, b sample.PlanSheet) bool { return a.Number == b.Number && a.Task == b.Task }) ||
		slices.EqualFunc(other.Sheets[right:], pl.Sheets[right:], func(a, b sample.PlanSheet) bool {
			return a.Task == b.Task && a.Parts[len(a.Parts)-1].Text == b.Parts[len(b.Parts)-1].Text
		}) {
		t.Error("the seed does not make the set")
	}
}

// A plan of parts, a plan with too few sheets whose answer is right and a task the bank lacks are
// refused.
func TestCorrectRefused(t *testing.T) {
	plan, bank := linesGen1(t)
	v := sample.PlanParts
	parts := *plan
	parts.Version = &v
	if _, _, err := gen.Correct(&parts, bank, "p.json", 1); err == nil {
		t.Error("a plan of parts is taken")
	}
	few := *plan
	few.Sheets = plan.Sheets[:12]
	if _, _, err := gen.Correct(&few, bank, "p.json", 1); err == nil || !strings.Contains(err.Error(), "needs more") {
		t.Errorf("a plan of few sheets: %v", err)
	}
	if _, _, err := gen.Correct(plan, &sample.Bank{}, "p.json", 1); err == nil || !strings.Contains(err.Error(), "not in the bank") {
		t.Errorf("a bank without the tasks: %v", err)
	}
}
