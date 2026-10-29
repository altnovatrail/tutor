package gen

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"

	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/synth"
)

// The traps of the generated tasks, worded as the bank of E2 words them (eval/bank.json), so that a
// generated sheet's trap means what the same trap means there.
var traps = map[string]string{
	"order-of-operations": "выполнил действия слева направо, не соблюдая порядка действий",
	"carry":               "ошибся при сложении с переходом через десяток",
	"arithmetic":          "ошибся в вычислении",
	"sign-on-transfer":    "перенёс слагаемое в другую часть уравнения, не сменив знак",
	"one-side-division":   "разделил на коэффициент только одну часть уравнения",
}

// task is a generated task: its bank entry, and the lines of what each of its sheets writes besides
// the solution, each a mistake of one of its traps.
type task struct {
	sample.Task
	// mistakes are the task's sheets with an error, one a trap, in the order of the task's traps:
	// their lines, the number of the first wrong line and the trap.
	mistakes []mistake
	// fixed is the erasure of a sheet that wrote a mistake and wrote the right line over it: the
	// line and the mistake erased. broken is that of a sheet that erased a right step and wrote a
	// wrong one: the mistake it leaves.
	fixed, broken erasure
	// other is a second correct solution: the same steps, written another way.
	other []string
	// otherAnswer is the line of other that gives the answer, when a line follows it.
	otherAnswer int
}

// mistake is a sheet with an error: its lines, its first wrong line, from 1, and its trap.
type mistake struct {
	lines []string
	first int
	trap  string
}

// erasure is a planned erasure of a generated sheet, and for a broken one the mistake it leaves.
type erasure struct {
	sample.PlanErasure
	mistake mistake
}

// kinds are the kinds of task the generator writes, in turn: a chain of calculations and two forms
// of a linear equation, as the bank of E2 has them (docs/architecture.md, section 3.1).
var kinds = []func(r *rand.Rand, id string) task{chain, equation, equationBothSides}

// Sheets is a bank of generated tasks and a plan of their sheets, the truth of each by construction:
// six sheets a task — correct, an error of each of two traps, a mistake erased and written right, a
// right step erased and written wrong, and a second correct solution, shorter for a chain and with a
// check after the answer for an equation (T93; R50). Of ten tasks, that is 14 correct sheets, 20 with
// an error, 10 of each erasure and 6 with the answer not last, against 12, 12, 7 and 2 of 40 in the
// plan of E2. The tasks are chains and equations, in turn, with
// numbers drawn from the seed: the same seed gives the same bank and plan. Their IDs begin with
// prefix.
func Sheets(tasks int, seed uint64, prefix string) (*sample.Bank, *sample.Plan, error) {
	if err := fits(tasks, 6); err != nil {
		return nil, nil, err
	}
	r := rand.New(rand.NewPCG(seed, 0x5eed5))
	bank, plan := newBank(), newPlan()
	for i := range tasks {
		t := kinds[i%len(kinds)](r, fmt.Sprintf("%s-%03d", prefix, i+1))
		bank.Tasks = append(bank.Tasks, t.Task)
		add := func(sh sample.PlanSheet) {
			sh.Number, sh.Task = len(plan.Sheets)+1, t.ID
			plan.Sheets = append(plan.Sheets, sh)
		}
		add(correctSheet(t.Solution, len(t.Solution)))
		for _, m := range t.mistakes {
			add(errorSheet(sample.ScenarioError, m, nil))
		}
		fixed := correctSheet(t.Solution, len(t.Solution))
		fixed.Scenario, fixed.Erasure = sample.ScenarioErasureFixed, &t.fixed.PlanErasure
		add(fixed)
		add(errorSheet(sample.ScenarioErasureBroken, t.broken.mistake, &t.broken.PlanErasure))
		other := correctSheet(t.other, t.otherAnswer)
		if t.otherAnswer < len(t.other) {
			other.Scenario = sample.ScenarioAnswerNotLast
		}
		add(other)
	}
	return checked(bank, plan)
}

// NearSheets is a bank of generated tasks whose keys end in a digit of a pair, each with its near
// key — the key with that digit replaced by its pair's — as its false key, and a plan of two sheets
// a task in the scenario similar-digits: the correct sheet, which a reading reads with the near key,
// and the sheet whose answer is the near key, which it reads with the right one (T70; T93). The
// answer's last digit of either leans toward its pair as far as draw is told.
func NearSheets(tasks int, seed uint64, prefix string) (*sample.Bank, *sample.Plan, error) {
	if err := fits(tasks, 2); err != nil {
		return nil, nil, err
	}
	r := rand.New(rand.NewPCG(seed, 0x5eed5))
	bank, plan := newBank(), newPlan()
	for i := 0; len(bank.Tasks) < tasks; i++ {
		t := kinds[i%len(kinds)](r, fmt.Sprintf("%s-%03d", prefix, len(bank.Tasks)+1))
		last := len(t.Solution) - 1
		near, ok := nearOf(t.Solution[last])
		if !ok {
			continue
		}
		key, _ := nearOf(t.Key)
		nearLines := append(append([]string(nil), t.Solution[:last]...), near)
		t.FalseKeys = []sample.FalseKey{{Key: key, Solution: nearLines}}
		bank.Tasks = append(bank.Tasks, t.Task)
		right := correctSheet(t.Solution, len(t.Solution))
		right.Scenario = sample.ScenarioSimilarDigits
		wrong := errorSheet(sample.ScenarioSimilarDigits, mistake{lines: nearLines, first: last + 1}, nil)
		for _, sh := range []sample.PlanSheet{right, wrong} {
			sh.Number, sh.Task = len(plan.Sheets)+1, t.ID
			plan.Sheets = append(plan.Sheets, sh)
		}
	}
	return checked(bank, plan)
}

// ErrTasks is the error of a number of tasks the generator does not make: none, or more than a plan
// holds, sample.MaxSheets, since the name of a sheet's file holds its number in two digits. More
// sheets go into several plans, each with its own seed and prefix.
var ErrTasks = errors.New("gen: a plan holds at least one task and at most sample.MaxSheets sheets")

// fits checks that tasks of perTask sheets each make a plan that draw can draw.
func fits(tasks, perTask int) error {
	if tasks < 1 || tasks*perTask > sample.MaxSheets {
		return fmt.Errorf("%w: %d tasks make %d sheets, and a plan holds %d at most: make several plans", ErrTasks, tasks, tasks*perTask, sample.MaxSheets)
	}
	return nil
}

// checked are a generated bank and plan once they pass their checks, so that no file of them fails
// what reads it; a failure is a mistake of the generator.
func checked(bank *sample.Bank, plan *sample.Plan) (*sample.Bank, *sample.Plan, error) {
	for _, err := range []error{bank.Check(), plan.Check(), plan.CheckWithBank(bank)} {
		if err != nil {
			return nil, nil, fmt.Errorf("gen: what the generator made fails its check: %w", err)
		}
	}
	return bank, plan, nil
}

// nearOf is a text with its last digit replaced by the digit of its pair, and false when that digit
// has none: the digit draw leans, by the same rule (synth.SimilarDigit), so that the near key and the
// digit drawn leaning toward it are one.
func nearOf(s string) (string, bool) {
	at, toward, ok := synth.SimilarDigit(s)
	if !ok {
		return "", false
	}
	runes := []rune(s)
	runes[at] = toward
	return string(runes), true
}

func newBank() *sample.Bank {
	v := 0
	return &sample.Bank{Format: sample.BankFormat, Version: &v}
}

func newPlan() *sample.Plan {
	v := 0
	return &sample.Plan{Format: sample.PlanFormat, Version: &v}
}

// correctSheet is a correct sheet of the lines, with its answer on the line answer.
func correctSheet(lines []string, answer int) sample.PlanSheet {
	return sample.PlanSheet{Scenario: sample.ScenarioCorrect, Lines: lines, Answer: answer, Expected: sample.Expected{Outcome: sample.OutcomeCorrect}}
}

// errorSheet is a sheet of a scenario with an error: the mistake's lines, the answer on the last.
func errorSheet(scenario sample.Scenario, m mistake, e *sample.PlanErasure) sample.PlanSheet {
	first := m.first
	return sample.PlanSheet{Scenario: scenario, Lines: m.lines, Answer: len(m.lines), Erasure: e,
		Expected: sample.Expected{Outcome: sample.OutcomeError, FirstError: &first, Trap: m.trap}}
}

// trapsOf are the bank's traps of the IDs given.
func trapsOf(ids ...string) []sample.Trap {
	out := make([]sample.Trap, len(ids))
	for i, id := range ids {
		out[i] = sample.Trap{ID: id, Description: traps[id]}
	}
	return out
}

// n is a number as the lines write it.
func n(v int) string { return strconv.Itoa(v) }

// chain is "Вычисли a + b − c × d.": the product first, then the sum, then the difference, as task
// chain-1 of E2. Its traps: the order of operations, left to right; the carry, a sum with the ten
// lost; and a slip in the last step. The units of a and b make a carry, and every number on the way
// is positive.
func chain(r *rand.Rand, id string) task {
	for {
		a, b := between(r, 12, 69), between(r, 12, 49)
		c, d := between(r, 11, 19), between(r, 2, 4)
		s, p := a+b, c*d
		key, lost := s-p, s-10-p
		t := s - c
		// The steps of the order of operations make numbers of their own, none a number of the solution.
		if (a%10)+(b%10) < 10 || lost < 1 || t < 1 || slices.Contains([]int{p, s, key}, t) || slices.Contains([]int{p, s, key}, t*d) {
			continue
		}
		prod, sum := fmt.Sprintf("%d × %d = %d", c, d, p), fmt.Sprintf("%d + %d = %d", a, b, s)
		lostSum := fmt.Sprintf("%d + %d = %d", a, b, s-10)
		slip := key + 10
		// The slip replaces a part of its line, so the line holds it once (sample.PlanErasure).
		if slipped := fmt.Sprintf("%d − %d = %d", s, p, slip); strings.Count(slipped, n(slip)) != 1 {
			continue
		}
		solution := []string{prod, sum, fmt.Sprintf("%d − %d = %d", s, p, key)}
		carried := []string{prod, lostSum, fmt.Sprintf("%d − %d = %d", s-10, p, lost)}
		return task{
			Task: sample.Task{ID: id, Kind: sample.KindCalculation, Statement: fmt.Sprintf("Вычисли %d + %d − %d × %d.", a, b, c, d), Key: n(key), Solution: solution,
				Traps: trapsOf("order-of-operations", "carry", "arithmetic"), Leaks: []string{prod, fmt.Sprintf("%d − %d", s, p)},
				FalseKeys: []sample.FalseKey{{Key: n(lost), Solution: carried}}},
			mistakes: []mistake{
				{lines: []string{sum, fmt.Sprintf("%d − %d = %d", s, c, t), fmt.Sprintf("%d × %d = %d", t, d, t*d)}, first: 2, trap: "order-of-operations"},
				{lines: carried, first: 2, trap: "carry"},
			},
			fixed: erasure{PlanErasure: sample.PlanErasure{Line: 2, Erased: sample.ErasedMistake, Text: lostSum, Replacement: sum}},
			broken: erasure{PlanErasure: sample.PlanErasure{Line: 3, Erased: sample.ErasedCorrectStep, Text: n(key), Replacement: n(slip), PartOfLine: true},
				mistake: mistake{lines: []string{prod, sum, fmt.Sprintf("%d − %d = %d", s, p, slip)}, first: 3, trap: "arithmetic"}},
			other: []string{prod, fmt.Sprintf("%d + %d − %d = %d", a, b, p, key)}, otherAnswer: 2,
		}
	}
}

// equation is "Реши уравнение ax + b = c.", as task equation-1 of E2. Its traps: the term carried
// over without its sign changed, which ends in x + 2b/a, a fraction unless a divides 2b, and the
// coefficient divided on one side alone; the two never end in the same answer. A second correct solution checks the root after the answer.
func equation(r *rand.Rand, id string) task {
	for {
		a, x, b := between(r, 2, 9), between(r, 2, 12), between(r, 1, 20)
		c := a*x + b
		m, wrongM := c-b, c+b
		if m == x || fraction(wrongM, a) == n(m) {
			continue
		}
		head := fmt.Sprintf("%dx + %d = %d", a, b, c)
		right, carried := fmt.Sprintf("%dx = %d", a, m), fmt.Sprintf("%dx = %d", a, wrongM)
		wrongKey := "x = " + fraction(wrongM, a)
		solution := []string{head, right, fmt.Sprintf("x = %d", x)}
		signed := []string{head, carried, wrongKey}
		return task{
			Task: sample.Task{ID: id, Kind: sample.KindLinearEquation, Statement: fmt.Sprintf("Реши уравнение %s.", head), Key: fmt.Sprintf("x = %d", x), Solution: solution,
				Traps: trapsOf("sign-on-transfer", "one-side-division"), Leaks: []string{right, fmt.Sprintf("%d : %d", m, a)},
				FalseKeys: []sample.FalseKey{{Key: wrongKey, Solution: signed}}},
			mistakes: []mistake{
				{lines: signed, first: 2, trap: "sign-on-transfer"},
				{lines: []string{head, right, fmt.Sprintf("x = %d", m)}, first: 3, trap: "one-side-division"},
			},
			fixed: erasure{PlanErasure: sample.PlanErasure{Line: 2, Erased: sample.ErasedMistake, Text: carried, Replacement: right}},
			broken: erasure{PlanErasure: sample.PlanErasure{Line: 3, Erased: sample.ErasedCorrectStep, Text: n(x), Replacement: n(m), PartOfLine: true},
				mistake: mistake{lines: []string{head, right, fmt.Sprintf("x = %d", m)}, first: 3, trap: "one-side-division"}},
			other:       append(append([]string(nil), solution...), fmt.Sprintf("Проверка: %d × %d + %d = %d", a, x, b, c)),
			otherAnswer: 3,
		}
	}
}

// equationBothSides is "Реши уравнение ax − b = cx + d.", as task equation-4 of E2: the terms
// carried over, collected, and divided. Its traps are those of equation, and the term carried over
// with its sign unchanged leaves a positive number.
func equationBothSides(r *rand.Rand, id string) task {
	for {
		c, e, x, b := between(r, 2, 5), between(r, 2, 5), between(r, 2, 9), between(r, 1, 9)
		a, d := c+e, e*x-b
		m, wrongM := d+b, d-b
		if d < 1 || wrongM < 1 || m == x {
			continue
		}
		head := fmt.Sprintf("%dx − %d = %dx + %d", a, b, c, d)
		collected, carried := fmt.Sprintf("%dx − %dx = %d + %d", a, c, d, b), fmt.Sprintf("%dx − %dx = %d − %d", a, c, d, b)
		wrongKey := "x = " + fraction(wrongM, e)
		solution := []string{head, collected, fmt.Sprintf("%dx = %d", e, m), fmt.Sprintf("x = %d", x)}
		signed := []string{head, carried, fmt.Sprintf("%dx = %d", e, wrongM), wrongKey}
		return task{
			Task: sample.Task{ID: id, Kind: sample.KindLinearEquation, Statement: fmt.Sprintf("Реши уравнение %s.", head), Key: fmt.Sprintf("x = %d", x), Solution: solution,
				Traps: trapsOf("sign-on-transfer", "one-side-division"), Leaks: []string{fmt.Sprintf("%dx = %d", e, m), fmt.Sprintf("%d : %d", m, e)},
				FalseKeys: []sample.FalseKey{{Key: wrongKey, Solution: signed}}},
			mistakes: []mistake{
				{lines: signed, first: 2, trap: "sign-on-transfer"},
				{lines: []string{head, collected, fmt.Sprintf("%dx = %d", e, m), fmt.Sprintf("x = %d", m)}, first: 4, trap: "one-side-division"},
			},
			fixed: erasure{PlanErasure: sample.PlanErasure{Line: 2, Erased: sample.ErasedMistake, Text: carried, Replacement: collected}},
			broken: erasure{PlanErasure: sample.PlanErasure{Line: 2, Erased: sample.ErasedCorrectStep, Text: collected, Replacement: carried},
				mistake: mistake{lines: signed, first: 2, trap: "sign-on-transfer"}},
			other:       append(append([]string(nil), solution...), fmt.Sprintf("Проверка: %d × %d − %d = %d × %d + %d", a, x, b, c, x, d)),
			otherAnswer: 4,
		}
	}
}

// fraction is p/q in lowest terms, or a whole number when q divides p, as the lines write it.
func fraction(p, q int) string {
	g := gcd(p, q)
	if q/g == 1 {
		return n(p / g)
	}
	return fmt.Sprintf("%d/%d", p/g, q/g)
}
