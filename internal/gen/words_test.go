package gen_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode"

	"github.com/altnovatrail/tutor/internal/gen"
	"github.com/altnovatrail/tutor/internal/hint"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/synth"
)

// linesGen1 is the plan of lines the set of the words is made of, and its bank.
func linesGen1(t *testing.T) (*sample.Plan, *sample.Bank) {
	t.Helper()
	bank, plan, err := gen.Sheets(10, 1, "lines-gen-1")
	if err != nil {
		t.Fatal(err)
	}
	return plan, bank
}

// The files of the set of the words are what the generator makes of lines-gen-1 and the seed 118,
// byte for byte: tutor-eval gen words makes them again (T118).
func TestWordsFiles(t *testing.T) {
	plan, bank := linesGen1(t)
	pl, cs, err := gen.Words(plan, bank, "words-sheets.json", 118)
	if err != nil {
		t.Fatal(err)
	}
	for file, v := range map[string]any{"words-sheets.json": pl, "words-cases.json": cs} {
		want, err := sample.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if got, err := os.ReadFile(filepath.Join("..", "..", "eval", file)); err != nil || !bytes.Equal(got, want) {
			t.Errorf("eval/%s is not what the generator makes of its seed: %v", file, err)
		}
	}
}

// The set takes every sheet with a mistake of the plan as parts of the main column, then ten of the
// sheets of the scenario error again with a note at the side after the last line, two of each kind,
// one in each language; its first errors and answers stay. The trap found is the planned one but on
// ten sheets without a note; the questions asked climb the ladder, and a case of the rung put
// another way asks its own rung's question too. Each sheet drawn passes its check.
func TestWords(t *testing.T) {
	plan, bank := linesGen1(t)
	pl, cs, err := gen.Words(plan, bank, "p.json", 118)
	if err != nil {
		t.Fatal(err)
	}
	if len(pl.Sheets) != 30+gen.WordsNotes || !pl.HasParts() {
		t.Fatalf("the plan has %d sheets, of parts %t", len(pl.Sheets), pl.HasParts())
	}
	kinds := map[hint.NoteKind][]sample.Lang{}
	for i, sh := range pl.Sheets {
		last := sh.Parts[len(sh.Parts)-1]
		noted := i >= 30
		if noted != (last.Place == sample.PlaceRight) || sh.Expected.Outcome != sample.OutcomeError || sh.Answer > len(sh.Parts) || *sh.Expected.FirstError > sh.Answer {
			t.Errorf("sheet %d: %+v", sh.Number, sh)
		}
		if noted {
			if sh.Scenario != sample.ScenarioError || sh.Erasure != nil || sh.Answer != len(sh.Parts)-1 {
				t.Errorf("sheet %d with a note is %s, answer %d of %d parts", sh.Number, sh.Scenario, sh.Answer, len(sh.Parts))
			}
		}
		s := synth.Sheet(&pl.Sheets[i], 118)
		if err := s.CheckWithBank(bank); err != nil {
			t.Errorf("sheet %d drawn: %v", sh.Number, err)
		}
	}
	noFound := map[int]bool{}
	for _, k := range cs.Cases {
		if k.Found == "" {
			noFound[k.Sheet] = true
		}
		if k.Kind == hint.CaseNote && k.Rung == 1 && k.Lang == sample.LangRussian {
			lang := sample.LangEnglish
			if strings.IndexFunc(k.Note, func(r rune) bool { return unicode.Is(unicode.Cyrillic, r) }) >= 0 {
				lang = sample.LangRussian
			}
			kinds[k.NoteKind] = append(kinds[k.NoteKind], lang)
		}
		want := int(k.Rung) - 1
		if k.Kind == hint.CaseOtherwise {
			want++
		}
		if len(k.Asked) != want || len(k.Techniques) != 2 {
			t.Errorf("case %s asks %d questions, with %d techniques", k.ID, len(k.Asked), len(k.Techniques))
		}
	}
	if len(noFound) != gen.WordsNoFound {
		t.Errorf("%d sheets have no trap found", len(noFound))
	}
	for _, kind := range hint.NoteKinds() {
		slices.Sort(kinds[kind])
		if !slices.Equal(kinds[kind], []sample.Lang{"en", "ru"}) {
			t.Errorf("the notes %s are in %v", kind, kinds[kind])
		}
	}
	// A case of a rung asks the questions of the cases below it on the sheet in its language.
	asked := map[string][]string{}
	for _, k := range cs.Cases {
		if k.Kind == hint.CaseRung && k.Rung == 3 {
			asked[fmt.Sprintf("%s/%d", k.Lang, k.Sheet)] = k.Asked
		}
	}
	for _, k := range cs.Cases {
		if k.Kind == hint.CaseOtherwise && k.Rung == 3 {
			if below := asked[fmt.Sprintf("%s/%d", k.Lang, k.Sheet)]; !slices.Equal(k.Asked[:2], below) {
				t.Errorf("case %s asks %q, and the rung 3 of its sheet %q", k.ID, k.Asked, below)
			}
		}
	}
	// The same seed makes the same set, and another another.
	_, again, _ := gen.Words(plan, bank, "p.json", 118)
	_, other, _ := gen.Words(plan, bank, "p.json", 119)
	if !sameJSON(t, cs, again) || sameJSON(t, cs, other) {
		t.Error("the set is not made of its seed alone")
	}
}

// Whatever the seed, the set of every plan of lines passes its checks.
func TestWordsSeeds(t *testing.T) {
	plan, bank := linesGen1(t)
	for seed := uint64(1); seed <= 50; seed++ {
		if _, _, err := gen.Words(plan, bank, "p.json", seed); err != nil {
			t.Fatalf("the seed %d: %v", seed, err)
		}
	}
}

// A plan of parts, a plan with too few sheets with a mistake and a task the bank lacks are refused.
func TestWordsRefused(t *testing.T) {
	plan, bank := linesGen1(t)
	v := sample.PlanParts
	parts := *plan
	parts.Version = &v
	if _, _, err := gen.Words(&parts, bank, "p.json", 1); err == nil {
		t.Error("a plan of parts is taken")
	}
	few := *plan
	few.Sheets = plan.Sheets[:12]
	if _, _, err := gen.Words(&few, bank, "p.json", 1); err == nil || !strings.Contains(err.Error(), "needs more") {
		t.Errorf("a plan of few sheets: %v", err)
	}
	if _, _, err := gen.Words(plan, &sample.Bank{}, "p.json", 1); err == nil || !strings.Contains(err.Error(), "not in the bank") {
		t.Errorf("a bank without the tasks: %v", err)
	}
}
