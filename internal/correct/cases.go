package correct

import (
	"fmt"
	"io"
	"slices"

	"github.com/altnovatrail/tutor/internal/hint"
	"github.com/altnovatrail/tutor/internal/sample"
)

// The eval set of the words after a correct answer (T143; docs/architecture.md, 3.4, "The set of
// the words after a correct answer"): an eval set of its own form, tutor's, as the set of the words
// of the hint (§ 13 of spec's text of the contracts; R85). A case is a call of the words as api asks
// it — a sheet whose answer is right, a language and the techniques of the task, none for a task that
// names none —, with the work, or without it, as for a task whose answer slots are bound choice alone
// (attempt.proto spec), and on some sheets a note at the side that is no part of the solution and is
// not for a child, which the words must keep silent on (R54).

// CasesFormat names the file of the cases, and CasesVersion is its version.
const (
	CasesFormat  = "tutor-correct-cases"
	CasesVersion = 1
)

// CaseKind is what a case tries.
type CaseKind string

// The kinds of case.
const (
	// CaseWork is the words with the work.
	CaseWork CaseKind = "work"
	// CaseChoice is the words without the work, as for a task whose answer slots are bound choice
	// alone.
	CaseChoice CaseKind = "choice"
	// CaseNote is the words with the work, on a sheet with a note at the side.
	CaseNote CaseKind = "note"
)

// CaseKinds are the kinds of case, in their order.
func CaseKinds() []CaseKind { return []CaseKind{CaseWork, CaseChoice, CaseNote} }

// Cases is the file of the cases: the plan of the sheets they are of, by its file's name, the seed
// that made them and the cases, in the order a run asks them.
type Cases struct {
	Format  string `json:"format"`
	Version int    `json:"version"`
	Plan    string `json:"plan"`
	Seed    uint64 `json:"seed"`
	Cases   []Case `json:"cases"`
}

// Case is a call of the words: its ID and kind; the sheet of the plan by its number; the language;
// whether the work goes with it; the techniques of the task, none for a task that names none; and on
// a sheet with a note, the note's text, as the plan writes it, and its kind.
type Case struct {
	ID         string        `json:"id"`
	Kind       CaseKind      `json:"kind"`
	Sheet      int           `json:"sheet"`
	Lang       sample.Lang   `json:"lang"`
	Work       bool          `json:"work"`
	Techniques []string      `json:"techniques"`
	Note       string        `json:"note"`
	NoteKind   hint.NoteKind `json:"note_kind"`
}

// ReadCases reads a file of cases strictly: no field it does not know, no list left null, and one
// value only.
func ReadCases(r io.Reader) (*Cases, error) {
	var c Cases
	if err := hint.ReadStrict(r, &c); err != nil {
		return nil, err
	}
	if err := hint.CheckHead(c.Format, c.Version, CasesFormat, CasesVersion, len(c.Cases)); err != nil {
		return nil, err
	}
	for i, k := range c.Cases {
		if k.Techniques == nil {
			return nil, fmt.Errorf("the cases: case %d leaves techniques null, not a list", i+1)
		}
	}
	return &c, nil
}

// Check checks the cases against the plan of their sheets and its bank: every ID once; a kind and a
// language the set knows; a sheet of the plan whose answer is right, of a task of the bank; the work
// with every case but one of choice; and a note, of a kind the set knows, on a case of a note alone,
// the very text of the sheet's note at the side, its last part.
func (c *Cases) Check(plan *sample.Plan, bank *sample.Bank) error {
	seen := map[string]bool{}
	for i, k := range c.Cases {
		at := fmt.Sprintf("case %d (%s)", i+1, k.ID)
		if k.ID == "" || seen[k.ID] {
			return fmt.Errorf("%s: an ID empty or given twice", at)
		}
		seen[k.ID] = true
		switch {
		case !slices.Contains(CaseKinds(), k.Kind):
			return fmt.Errorf("%s: no kind %q", at, k.Kind)
		case !slices.Contains(hint.Langs(), k.Lang):
			return fmt.Errorf("%s: no language %q", at, k.Lang)
		case k.Sheet < 1 || k.Sheet > len(plan.Sheets):
			return fmt.Errorf("%s: no sheet %d in the plan", at, k.Sheet)
		case k.Work != (k.Kind != CaseChoice):
			return fmt.Errorf("%s: a case of the kind %s goes with the work: %t", at, k.Kind, k.Kind != CaseChoice)
		}
		sh := &plan.Sheets[k.Sheet-1]
		if sh.Expected.Outcome != sample.OutcomeCorrect {
			return fmt.Errorf("%s: sheet %d is %s, and the words after a correct answer are asked only after a right one", at, k.Sheet, sh.Expected.Outcome)
		}
		if bank.Task(sh.Task) == nil {
			return fmt.Errorf("%s: the task %s of sheet %d is not in the bank", at, sh.Task, k.Sheet)
		}
		if err := hint.CheckNote(at, k.Kind == CaseNote, string(k.Kind), k.Note, k.NoteKind, sh); err != nil {
			return err
		}
	}
	return nil
}
