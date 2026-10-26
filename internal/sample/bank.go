package sample

import (
	"fmt"
	"slices"
)

// Bank is the bank of tasks the sheets solve, as a module's tasks are (concepts 6 and 7 in spec).
type Bank struct {
	Format  string `json:"format"`
	Version *int   `json:"version"`
	Tasks   []Task `json:"tasks"`
}

// Task is one task of the bank.
type Task struct {
	// ID is unique within the bank, such as "linear-01"; a sample names its task by it.
	ID   string   `json:"id"`
	Kind TaskKind `json:"kind"`
	// Statement is the condition the writer solves, in Russian.
	Statement string `json:"statement"`
	// Key is the answer, in the notation of the solution's last line.
	Key string `json:"key"`
	// Solution is a correct solution, line by line.
	Solution []string `json:"solution"`
	Traps    []Trap   `json:"traps"`
	// Leaks are phrases that give the answer away, besides the key itself in any notation: the
	// leak check looks for them in every hint.
	Leaks []string `json:"leaks"`
	// FalseKeys are wrong keys with their solutions: the same steps with another number. The
	// measurement reads correct sheets with them, to see whether the model reads what it is given
	// rather than what is written (R03).
	FalseKeys []FalseKey `json:"false_keys"`
}

// TaskKind is the kind of a task, one of the measurement's three (docs/architecture.md, section
// 3.1). No code checks the lines of any kind: the line check went with R19, and the markup says
// where the first error is, whatever the kind (R24).
type TaskKind string

// The kinds of task.
const (
	// KindLinearEquation: a linear equation in one unknown.
	KindLinearEquation TaskKind = "linear-equation"
	// KindCalculation: a chain of calculations, grades 1–6.
	KindCalculation TaskKind = "calculation"
	// KindFreeSolution: a free solution of an olympiad task.
	KindFreeSolution TaskKind = "free-solution"
)

// TaskKinds lists every kind of task.
var TaskKinds = []TaskKind{KindLinearEquation, KindCalculation, KindFreeSolution}

// Trap is a typical mistake the task catches (concepts 7 in spec).
type Trap struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

// FalseKey is a wrong key with the solution that leads to it.
type FalseKey struct {
	Key      string   `json:"key"`
	Solution []string `json:"solution"`
}

// Task returns the task with the given ID, or nil.
func (b *Bank) Task(id string) *Task {
	for i := range b.Tasks {
		if b.Tasks[i].ID == id {
			return &b.Tasks[i]
		}
	}
	return nil
}

// Check checks a bank and returns a *FormatError listing every problem, or nil.
func (b *Bank) Check() error {
	p := &problems{what: "bank"}
	checkHeader(p, b.Format, BankFormat, b.Version)
	if len(b.Tasks) == 0 {
		p.add("tasks", "are missing: a bank has at least one task")
	}
	ids := make(map[string]bool)
	for i, t := range b.Tasks {
		path := fmt.Sprintf("tasks[%d]", i)
		switch {
		case t.ID == "":
			p.add(path+".id", "is missing")
		case ids[t.ID]:
			p.add(path+".id", "%q is used by an earlier task", t.ID)
		}
		ids[t.ID] = true
		checkTask(p, path, t)
	}
	return p.err()
}

func checkTask(p *problems, path string, t Task) {
	if !slices.Contains(TaskKinds, t.Kind) {
		p.add(path+".kind", "%q is not one of %q", t.Kind, TaskKinds)
	}
	if t.Statement == "" {
		p.add(path+".statement", "is missing")
	}
	if t.Key == "" {
		p.add(path+".key", "is missing")
	}
	checkSolution(p, path+".solution", t.Solution)

	traps := make(map[string]bool)
	for j, tr := range t.Traps {
		tpath := fmt.Sprintf("%s.traps[%d]", path, j)
		switch {
		case tr.ID == "":
			p.add(tpath+".id", "is missing")
		case traps[tr.ID]:
			p.add(tpath+".id", "%q is used by an earlier trap of the task", tr.ID)
		}
		traps[tr.ID] = true
		if tr.Description == "" {
			p.add(tpath+".description", "is missing")
		}
	}
	for j, l := range t.Leaks {
		if l == "" {
			p.add(fmt.Sprintf("%s.leaks[%d]", path, j), "is empty")
		}
	}

	if len(t.FalseKeys) == 0 {
		p.add(path+".false_keys", "are missing: every task has a false key, for the condition with a false key (R03)")
	}
	keys := map[string]bool{t.Key: true}
	for j, fk := range t.FalseKeys {
		fpath := fmt.Sprintf("%s.false_keys[%d]", path, j)
		switch {
		case fk.Key == "":
			p.add(fpath+".key", "is missing")
		case fk.Key == t.Key:
			p.add(fpath+".key", "%q is the task's key", fk.Key)
		case keys[fk.Key]:
			p.add(fpath+".key", "%q is used by an earlier false key", fk.Key)
		}
		keys[fk.Key] = true
		checkSolution(p, fpath+".solution", fk.Solution)
		if len(fk.Solution) != len(t.Solution) {
			p.add(fpath+".solution", "has %d lines, the task's solution %d: a false key takes the same steps", len(fk.Solution), len(t.Solution))
		} else if slices.Equal(fk.Solution, t.Solution) {
			p.add(fpath+".solution", "is the task's solution")
		}
	}
}

func checkSolution(p *problems, path string, lines []string) {
	if len(lines) == 0 {
		p.add(path, "is missing: a solution has at least one line")
	}
	for j, l := range lines {
		if l == "" {
			p.add(fmt.Sprintf("%s[%d]", path, j), "is empty")
		}
	}
}
