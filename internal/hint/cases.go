package hint

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/altnovatrail/tutor/internal/sample"
)

// The eval set of the tutor's words (T118; docs/architecture.md, 3.4, "The set of words"): a capability
// whose input is no sample has an eval set of its own form, tutor's (§ 13 of spec's text of the
// contracts). A case is a call of the words as api asks it — a sheet with a mistake, a rung, a
// language, the trap the program found at the cell of the rung, the techniques of the task and the
// questions already asked —, with the work or without it, when the learner asks to have the rung
// put another way (R82 spec), and on some sheets a note at the side that is no part of the solution
// and is not for a child, which the words must keep silent on (R54).

// CasesFormat names the file of the cases, and CasesVersion is its version.
const (
	CasesFormat  = "tutor-words-cases"
	CasesVersion = 1
)

// CaseKind is what a case tries.
type CaseKind string

// The kinds of case.
const (
	// CaseRung is a hint of a rung with the work, and the questions of the rungs below asked.
	CaseRung CaseKind = "rung"
	// CaseOtherwise is the rung put another way: no work, and the question of the rung itself asked
	// too, which the words must not repeat.
	CaseOtherwise CaseKind = "otherwise"
	// CaseNote is a hint of a rung with the work, on a sheet with a note at the side.
	CaseNote CaseKind = "note"
)

// CaseKinds are the kinds of case, in their order.
func CaseKinds() []CaseKind { return []CaseKind{CaseRung, CaseOtherwise, CaseNote} }

// NoteKind is what a note at the side is: the five kinds the author chose (T118).
type NoteKind string

// The kinds of note.
const (
	NoteSad       NoteKind = "sad"
	NoteRude      NoteKind = "rude"
	NoteThreat    NoteKind = "threat"
	NotePersonal  NoteKind = "personal"
	NoteInjection NoteKind = "injection"
)

// NoteKinds are the kinds of note, in their order.
func NoteKinds() []NoteKind {
	return []NoteKind{NoteSad, NoteRude, NoteThreat, NotePersonal, NoteInjection}
}

// Cases is the file of the cases: the plan of the sheets they are of, by its file's name, the seed
// that made them and the cases, in the order a run asks them.
type Cases struct {
	Format  string `json:"format"`
	Version int    `json:"version"`
	Plan    string `json:"plan"`
	Seed    uint64 `json:"seed"`
	Cases   []Case `json:"cases"`
}

// Case is a call of the words: its ID and kind; the sheet of the plan by its number; the rung and the
// language; whether the work goes with it; the trap the program found, empty for none; the
// techniques of the task; the questions the tutor has already asked on the task, in order; and on a
// sheet with a note, the note's text, as the plan writes it, and its kind.
type Case struct {
	ID         string      `json:"id"`
	Kind       CaseKind    `json:"kind"`
	Sheet      int         `json:"sheet"`
	Rung       Rung        `json:"rung"`
	Lang       sample.Lang `json:"lang"`
	Work       bool        `json:"work"`
	Found      string      `json:"found"`
	Techniques []string    `json:"techniques"`
	Asked      []string    `json:"asked"`
	Note       string      `json:"note"`
	NoteKind   NoteKind    `json:"note_kind"`
}

// ReadCases reads a file of cases strictly: no field it does not know, no list left null, and one
// value only.
func ReadCases(r io.Reader) (*Cases, error) {
	var c Cases
	if err := ReadStrict(r, &c); err != nil {
		return nil, err
	}
	if err := CheckHead(c.Format, c.Version, CasesFormat, CasesVersion, len(c.Cases)); err != nil {
		return nil, err
	}
	for i, k := range c.Cases {
		if k.Techniques == nil || k.Asked == nil {
			return nil, fmt.Errorf("the cases: case %d leaves techniques or asked null, not a list", i+1)
		}
	}
	return &c, nil
}

// ReadStrict reads a file of cases into v strictly: no field it does not know, and one value only.
// The cases of the set of words read so, and those of the words after a correct answer
// (internal/correct).
func ReadStrict(r io.Reader, v any) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("the cases: %w", err)
	}
	if dec.More() {
		return errors.New("the cases: more than one value in the file")
	}
	return nil
}

// CheckHead holds a file of cases read to its format and version, and to some case.
func CheckHead(format string, version int, wantFormat string, wantVersion, cases int) error {
	switch {
	case format != wantFormat || version != wantVersion:
		return fmt.Errorf("the cases: format %q version %d, not %q version %d", format, version, wantFormat, wantVersion)
	case cases == 0:
		return errors.New("the cases: none")
	}
	return nil
}

// CheckNote checks the note of a case against its sheet of the plan: on a case of a note, the very
// text of the sheet's note at the side, its last part, and a kind the set knows; on another case,
// none, of a sheet without one. at names the case in the error.
func CheckNote(at string, noted bool, kind string, note string, noteKind NoteKind, sh *sample.PlanSheet) error {
	own := ""
	if n := len(sh.Parts); n > 0 && sh.Parts[n-1].Place == sample.PlaceRight {
		own = sh.Parts[n-1].Text
	}
	switch {
	case noted && (note == "" || note != own || !slices.Contains(NoteKinds(), noteKind)):
		return fmt.Errorf("%s: a case of a note gives the note of sheet %d, %q, and its kind", at, sh.Number, own)
	case !noted && (note != "" || noteKind != "" || own != ""):
		return fmt.Errorf("%s: a case of the kind %s is of a sheet without a note, and gives none", at, kind)
	}
	return nil
}

// Check checks the cases against the plan of their sheets and its bank: every ID once; a kind, rung
// and language the set knows; a sheet of the plan with a mistake; a trap found among its task's; the
// work with every case but one of the rung put another way; and a note, of a kind the set knows,
// on a case of a note alone, the very text of the sheet's note at the side, its last part.
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
		case !slices.Contains(Rungs(), k.Rung):
			return fmt.Errorf("%s: no rung %d", at, k.Rung)
		case !slices.Contains(Langs(), k.Lang):
			return fmt.Errorf("%s: no language %q", at, k.Lang)
		case k.Sheet < 1 || k.Sheet > len(plan.Sheets):
			return fmt.Errorf("%s: no sheet %d in the plan", at, k.Sheet)
		case k.Work != (k.Kind != CaseOtherwise):
			return fmt.Errorf("%s: a case of the kind %s goes with the work: %t", at, k.Kind, k.Kind != CaseOtherwise)
		}
		sh := &plan.Sheets[k.Sheet-1]
		if sh.Expected.Outcome != sample.OutcomeError {
			return fmt.Errorf("%s: sheet %d is %s, and a hint is asked only after a wrong answer", at, k.Sheet, sh.Expected.Outcome)
		}
		task := bank.Task(sh.Task)
		if task == nil {
			return fmt.Errorf("%s: the task %s of sheet %d is not in the bank", at, sh.Task, k.Sheet)
		}
		if k.Found != "" && !slices.ContainsFunc(task.Traps, func(t sample.Trap) bool { return t.ID == k.Found }) {
			return fmt.Errorf("%s: the trap found, %q, is none of the task's", at, k.Found)
		}
		if err := CheckNote(at, k.Kind == CaseNote, string(k.Kind), k.Note, k.NoteKind, sh); err != nil {
			return err
		}
		for _, q := range k.Asked {
			if strings.TrimSpace(q) == "" {
				return fmt.Errorf("%s: an empty question asked", at)
			}
		}
	}
	return nil
}
