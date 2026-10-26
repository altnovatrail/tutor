package sample

import (
	"fmt"
	"io"
)

// SplitFormat is the name a split's file declares in its "format" field.
const SplitFormat = "tutor-split"

// Split divides the recorded sheets of the measurement into a development part, on which the
// prompt, the journal and the rule of frames are tuned, and a test part, which is run once they are
// frozen (R24). It is made once, before the first change of the prompt, and is never rewritten:
// each sheet in it is the file that counted then.
type Split struct {
	Format  string `json:"format"`
	Version *int   `json:"version"`
	// Seed is the number that decided which sheets of each scenario went to the development part.
	Seed int64        `json:"seed"`
	Dev  []SplitSheet `json:"dev"`
	Test []SplitSheet `json:"test"`
}

// SplitSheet is a sheet of a part: the number of its plan sheet and the name of its file.
type SplitSheet struct {
	Number int    `json:"number"`
	File   string `json:"file"`
}

// ReadSplit reads a split from JSON and checks it on its own; CheckWithPlan checks it against the
// sheet plan. An error from a split that fails the check is a *FormatError.
func ReadSplit(r io.Reader) (*Split, error) {
	var sp Split
	if err := readStrict(r, "split", &sp); err != nil {
		return nil, readError("split", err)
	}
	if err := sp.Check(); err != nil {
		return nil, err
	}
	return &sp, nil
}

// Check checks a split on its own: its header, that each part holds sheets, and that each sheet
// is in one part once, by the name of a saved file of its number.
func (sp *Split) Check() error {
	p := &problems{what: "split"}
	checkHeader(p, sp.Format, SplitFormat, sp.Version)
	where := map[int]string{}
	for _, part := range []struct {
		name   string
		sheets []SplitSheet
	}{{"dev", sp.Dev}, {"test", sp.Test}} {
		if len(part.sheets) == 0 {
			p.add(part.name, "is empty: each part holds sheets")
		}
		for i, sh := range part.sheets {
			path := fmt.Sprintf("%s[%d]", part.name, i)
			if other, seen := where[sh.Number]; seen {
				p.add(path+".number", "%d is already in %s", sh.Number, other)
			} else {
				where[sh.Number] = path
			}
			if n, ok := ParseSheetFile(sh.File); !ok || n.Number != sh.Number {
				p.add(path+".file", "%q is not the name of a saved file of sheet %d", sh.File, sh.Number)
			}
		}
	}
	return p.err()
}

// CheckWithPlan checks a split against the sheet plan: every sheet of the plan is in it, and each
// file is a saved file of its plan sheet.
func (sp *Split) CheckWithPlan(plan *Plan) error {
	p := &problems{what: "split"}
	in := map[int]bool{}
	for _, part := range []struct {
		name   string
		sheets []SplitSheet
	}{{"dev", sp.Dev}, {"test", sp.Test}} {
		for i, sh := range part.sheets {
			path := fmt.Sprintf("%s[%d]", part.name, i)
			if sh.Number < 1 || sh.Number > len(plan.Sheets) {
				p.add(path+".number", "%d is not a sheet of the plan, 1…%d", sh.Number, len(plan.Sheets))
				continue
			}
			in[sh.Number] = true
			if n, ok := ParseSheetFile(sh.File); ok && !n.Saves(&plan.Sheets[sh.Number-1]) {
				p.add(path+".file", "%q is not a file of sheet %d of the plan, whose task and scenario are %s-%s", sh.File, sh.Number, plan.Sheets[sh.Number-1].Task, plan.Sheets[sh.Number-1].Scenario)
			}
		}
	}
	for _, sh := range plan.Sheets {
		if !in[sh.Number] {
			p.add(Root, "sheet %d of the plan is in neither part", sh.Number)
		}
	}
	return p.err()
}
