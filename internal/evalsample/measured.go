package evalsample

import (
	"encoding/json"

	"github.com/altnovatrail/module-kit/verdict"
	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"
	observationv1 "github.com/altnovatrail/spec/contracts/novatrail/observation/v1"
)

// Truth is what the runner's report takes of an answer slot of a sample that a model reads and whose
// check gives a verdict (contracts/README.md of spec, section 14): its place, module-kit's verdict
// on the truth of the markup, with its reason or trap, and the markup's notes. The log of a run
// writes it so, and the report is made of it (R74).
type Truth struct {
	Item       int    `json:"item"`
	Slot       int    `json:"slot"`
	Verdict    string `json:"verdict"`
	Reason     string `json:"reason,omitempty"`
	Trap       string `json:"trap,omitempty"`
	Mirrored   bool   `json:"mirrored,omitempty"`
	CrossedOut bool   `json:"crossed_out,omitempty"`
}

// Mark is a criterion of an essay's rubric as the people marked it in the markup: its place in the
// rubric, from 1, and whether the essay meets it.
type Mark struct {
	Criterion int  `json:"criterion"`
	Met       bool `json:"met"`
}

// truthsOf are the truths of the answer slots of a task that read keeps, in the task's order, by
// module-kit's verdicts on the truth of the markup, items, and the markup's notes. read nil keeps
// them all.
func truthsOf(items []verdict.ItemVerdict, m *evalv1.Markup, read func(item, slot int) bool) []Truth {
	type place struct{ item, slot int32 }
	notes := map[place]*evalv1.AnswerSlotNote{}
	for _, n := range m.GetNotes() {
		notes[place{n.GetAnswerSlot().GetItem(), n.GetAnswerSlot().GetSlot()}] = n
	}
	var out []Truth
	for i, it := range items {
		for k, v := range it.Slots {
			if read != nil && !read(i+1, k+1) {
				continue
			}
			t := Truth{Item: i + 1, Slot: k + 1, Verdict: string(v.Verdict), Reason: string(v.Reason)}
			if v.Trap != nil {
				t.Trap = string(v.Trap.Trap)
			}
			if n := notes[place{int32(i + 1), int32(k + 1)}]; n != nil {
				t.Mirrored, t.CrossedOut = n.GetMirrored(), n.GetCrossedOut()
			}
			out = append(out, t)
		}
	}
	return out
}

// peopleOf are the marks of an essay's markup, in their order, and the reason its truth gives no
// text, as the protocol names it, or "" when it gives the essay's text.
func peopleOf(m *evalv1.Markup) ([]Mark, string) {
	var marks []Mark
	for _, mk := range m.GetMarks().GetMarks() {
		marks = append(marks, Mark{Criterion: int(mk.GetCriterion()), Met: mk.GetMet()})
	}
	reason := ""
	if slots := m.GetTruth().GetAnswerSlots(); len(slots) == 1 {
		if u, ok := slots[0].GetRead().(*observationv1.AnswerSlotObservation_Unreadable); ok {
			reason = u.Unreadable.String()
		}
	}
	return marks, reason
}

// SampleID is the ID that a file of a sample names, read leniently, so that a sample refused because
// it breaks a rule is still named by it in the report; "" when the file names none that is an ID.
func SampleID(data []byte) string {
	var head struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(data, &head) != nil || !idPattern.MatchString(head.ID) {
		return ""
	}
	return head.ID
}
