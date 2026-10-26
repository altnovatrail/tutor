package report_test

import (
	"bytes"
	"math"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/internal/judge"
	"github.com/altnovatrail/tutor/internal/measure"
	"github.com/altnovatrail/tutor/internal/report"
	"github.com/altnovatrail/tutor/internal/sample"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-12 }

// Five sheets with d > 0 and the rest with d = 0 give p = 1/32: of the 32 ways to sign the five,
// one reaches their sum.
func TestSignTestFive(t *testing.T) {
	nums := []int64{1, 1, 1, 1, 1, 0, 0, 0}
	dens := []int64{1, 1, 1, 1, 1, 1, 1, 1}
	got := report.SignTest(nums, dens)
	if got.Nonzero != 5 || !near(got.Helps, 1.0/32) || !near(got.Harms, 1) || !near(got.Sum, 5) {
		t.Errorf("five sheets with d = 1: %+v, want p = 1/32", got)
	}
	if got := report.SignTest(nil, nil); got.Helps != 1 || got.Harms != 1 || got.Nonzero != 0 {
		t.Errorf("no sheets: %+v", got)
	}
}

// The counts of the test agree with every signing written out, on small sets of fractions.
func TestSignTestByHand(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	for range 200 {
		n := 1 + rng.IntN(9)
		nums, dens := make([]int64, n), make([]int64, n)
		for i := range n {
			dens[i] = int64(1 + rng.IntN(9))
			nums[i] = int64(rng.IntN(int(2*dens[i]+1))) - dens[i]
		}
		var d []float64
		obs := 0.0
		for i := range n {
			d = append(d, float64(nums[i])/float64(dens[i]))
			obs += d[len(d)-1]
		}
		var nz []float64
		for _, x := range d {
			if x != 0 {
				nz = append(nz, math.Abs(x))
			}
		}
		up, down := 0, 0
		for mask := 0; mask < 1<<len(nz); mask++ {
			s := 0.0
			for i, x := range nz {
				if mask&(1<<i) != 0 {
					s += x
				} else {
					s -= x
				}
			}
			if s >= obs-1e-9 {
				up++
			}
			if s <= obs+1e-9 {
				down++
			}
		}
		all := float64(int(1) << len(nz))
		got := report.SignTest(nums, dens)
		if !near(got.Helps, float64(up)/all) || !near(got.Harms, float64(down)/all) {
			t.Fatalf("%v / %v: %+v, want %v and %v", nums, dens, got, float64(up)/all, float64(down)/all)
		}
	}
}

// builder makes a run's log by hand: sheets, and answers right or wrong.
type builder struct {
	log measure.Log
}

func newLog(repeats int, judges ...string) *builder {
	b := &builder{}
	b.log.Run = measure.RunRecord{Kind: "run", Part: "test", Repeats: repeats, MaxUSD: 6, Prompt: strings.Repeat("ab", 32)}
	for _, j := range judges {
		b.log.Run.Judges = append(b.log.Run.Judges, measure.JudgeRecord{Name: j, Model: "claude-" + j, Effort: "high"})
	}
	return b
}

// sheet adds a sheet: correct, or with its first error on line 2.
func (b *builder) sheet(n int, outcome sample.Outcome, frames int) {
	sh := measure.SheetRecord{Kind: "sheet", Sheet: n, Scenario: sample.ScenarioCorrect, Outcome: outcome, Frames: frames}
	if outcome == sample.OutcomeError {
		sh.Scenario, sh.FirstError = sample.ScenarioError, 2
	}
	b.log.Sheets = append(b.log.Sheets, sh)
}

// answer adds a call, right or wrong: on a correct sheet a wrong answer is a false alarm, and on a
// sheet with a mistake a false "correct".
func (b *builder) answer(sheet int, f judge.Form, j string, rep int, right bool) {
	var sh measure.SheetRecord
	for _, s := range b.log.Sheets {
		if s.Sheet == sheet {
			sh = s
		}
	}
	c := measure.CallRecord{Kind: "call", Call: measure.Call{Sheet: sheet, Form: f, Judge: j, Repeat: rep}, LatencyMs: 1000, CostUSD: 0.01}
	switch {
	case sh.Outcome == sample.OutcomeCorrect && right, sh.Outcome == sample.OutcomeError && !right:
		c.Verdict = judge.Correct
	default:
		c.Verdict, c.Hit = judge.Error, 2
	}
	b.log.Calls = append(b.log.Calls, c)
}

func (b *builder) same(sheet int, j string, rep int) {
	b.log.Calls = append(b.log.Calls, measure.CallRecord{Kind: "call", Call: measure.Call{Sheet: sheet, Form: judge.FormD, Judge: j, Repeat: rep}, SameAs: judge.FormB})
}

func (b *builder) report(t *testing.T) *report.Report {
	t.Helper()
	r, err := report.New(&b.log)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// helpOn builds a log of ten correct sheets with frames where form D is right on all and form A
// wrong on the first `on`, by one judge, the same in every repeat.
func helpOn(on, repeats int) *builder {
	b := newLog(repeats, "opus")
	for n := 1; n <= 10; n++ {
		b.sheet(n, sample.OutcomeCorrect, 1)
		for rep := 1; rep <= repeats; rep++ {
			for _, f := range judge.Forms {
				b.answer(n, f, "opus", rep, f != judge.FormA || n > on)
			}
		}
	}
	return b
}

// The criterion: five sheets where D is right and A is not, the rest alike, give p = 1/32, and
// with no more false alarms in D the judge helps. Three repeats alike give the same p as one.
func TestCriterion(t *testing.T) {
	for _, repeats := range []int{1, 3} {
		r := helpOn(5, repeats).report(t)
		c := r.Main[0]
		if c.Nonzero != 5 || !near(c.Helps, 1.0/32) || c.Outcome != "helps" || c.FalseAlarmA != 5*repeats || c.FalseAlarmD != 0 {
			t.Errorf("%d repeats: %+v, want p = 1/32 and helps", repeats, c)
		}
		// One judge: Holm's correction is for one, and p = 1/32 < 0.05 passes it.
		if !c.Holm || !r.Helps {
			t.Errorf("%d repeats: Holm %v, helps %v", repeats, c.Holm, r.Helps)
		}
	}
	// Four sheets cannot pass.
	if c := helpOn(4, 1).report(t).Main[0]; c.Outcome != "not shown" || !near(c.Helps, 1.0/16) {
		t.Errorf("four sheets: %+v", c)
	}
}

// With three judges, Holm's correction asks p < 0.05/3 of the smallest: five sheets, p = 1/32, do
// not pass it, and six, p = 1/64, do.
func TestHolm(t *testing.T) {
	for _, c := range []struct {
		on    int
		helps bool
	}{{5, false}, {6, true}} {
		b := newLog(1, "opus", "haiku", "haiku-fast")
		for n := 1; n <= 10; n++ {
			b.sheet(n, sample.OutcomeCorrect, 1)
			for _, j := range []string{"opus", "haiku", "haiku-fast"} {
				for _, f := range judge.Forms {
					b.answer(n, f, j, 1, f != judge.FormA || n > c.on || j != "opus")
				}
			}
		}
		r := b.report(t)
		if r.Main[0].Outcome != "helps" || r.Helps != c.helps || r.Main[0].Holm != c.helps || r.Main[1].Outcome != "not shown" {
			t.Errorf("%d sheets: helps %v, the judges %+v", c.on, r.Helps, r.Main)
		}
	}
}

// D right where A is wrong, but with more false "correct" in D than in A: not "helps". And the
// other way round, "harms".
func TestConditions(t *testing.T) {
	b := newLog(1, "opus")
	for n := 1; n <= 10; n++ {
		b.sheet(n, sample.OutcomeCorrect, 1)
		for _, f := range judge.Forms {
			b.answer(n, f, "opus", 1, f != judge.FormA || n > 8)
		}
	}
	b.sheet(11, sample.OutcomeError, 1)
	for _, f := range judge.Forms {
		b.answer(11, f, "opus", 1, f != judge.FormD) // a false "correct" in D alone
	}
	if c := b.report(t).Main[0]; c.Outcome != "not shown" || c.FalseCorrectD != 1 || c.FalseCorrectA != 0 || !(c.Helps < 0.05) {
		t.Errorf("D with a false correct: %+v", c)
	}

	h := newLog(1, "opus")
	for n := 1; n <= 8; n++ {
		h.sheet(n, sample.OutcomeCorrect, 1)
		for _, f := range judge.Forms {
			h.answer(n, f, "opus", 1, f != judge.FormD || n > 6)
		}
	}
	if c := h.report(t).Main[0]; c.Outcome != "harms" || !near(c.Harms, 1.0/64) {
		t.Errorf("D wrong on six: %+v", c)
	}
}

// On a sheet without frames form D is form B; an answer to a sheet with a mistake is right only
// with its box on the line of the first error; a failed call is wrong.
func TestAnswers(t *testing.T) {
	b := newLog(1, "opus")
	b.sheet(1, sample.OutcomeCorrect, 0)
	b.answer(1, judge.FormA, "opus", 1, false)
	b.answer(1, judge.FormB, "opus", 1, true)
	b.same(1, "opus", 1)
	b.sheet(2, sample.OutcomeError, 0)
	b.answer(2, judge.FormA, "opus", 1, true)
	b.log.Calls[len(b.log.Calls)-1].Hit = 3 // "error", but on the wrong line
	b.answer(2, judge.FormB, "opus", 1, true)
	b.same(2, "opus", 1)
	b.sheet(3, sample.OutcomeCorrect, 0)
	b.answer(3, judge.FormA, "opus", 1, true)
	b.answer(3, judge.FormB, "opus", 1, true)
	b.log.Calls[len(b.log.Calls)-1].Error = "the call failed"
	b.same(3, "opus", 1)
	r := b.report(t)
	if c := r.Main[0]; !near(c.Sum, 1+1-1) || c.Nonzero != 3 {
		t.Errorf("D against A: %+v, want d = +1, +1 and −1", c)
	}
	cells := map[judge.Form]report.Cell{}
	for _, c := range r.Cells {
		cells[c.Form] = c
	}
	if a, d := cells[judge.FormA], cells[judge.FormD]; a.Right != 1 || a.FalseAlarm != 1 || d.Right != 2 || d.Failed != 1 || d.TimeP50 != 0 || cells[judge.FormB].TimeP50 != 1000 {
		t.Errorf("the cells are A %+v, D %+v", a, d)
	}
}

// The cells: repeats that disagree, the erasure sheets, time and cost; and the agreement of a judge
// with the first in "correct".
func TestCells(t *testing.T) {
	b := newLog(3, "opus", "haiku")
	b.sheet(1, sample.OutcomeCorrect, 1)
	b.log.Sheets[0].Scenario = sample.ScenarioErasureFixed
	b.sheet(2, sample.OutcomeError, 1)
	b.log.Sheets[1].Scenario = sample.ScenarioErasureBroken
	for rep := 1; rep <= 3; rep++ {
		for _, f := range judge.Forms {
			b.answer(1, f, "opus", rep, rep != 2) // the second repeat a false alarm
			b.answer(2, f, "opus", rep, true)
			b.answer(1, f, "haiku", rep, true)
			b.answer(2, f, "haiku", rep, false) // a false "correct"
		}
	}
	for i := range b.log.Calls {
		b.log.Calls[i].LatencyMs = int64(100 * (i%10 + 1))
	}
	r := b.report(t)
	c := r.Cells[0]
	if c.Judge != "opus" || c.Form != judge.FormA || c.Answers != 6 || c.Right != 5 || c.FalseAlarm != 1 || c.Split != 1 || c.Repeated != 2 ||
		c.FixedAlarms != 1 || c.FixedAnswers != 3 || c.BrokenRight != 3 || c.BrokenAnswers != 3 || !near(c.MeanUSD, 0.01) {
		t.Errorf("opus in form A: %+v", c)
	}
	// Haiku says "correct" in all its 24 answers; Opus says "correct" with it on sheet 1 but in
	// the second repeat, 8 times, and on sheet 2 never.
	if a := r.Agreements[0]; a.Judge != "haiku" || a.With != "opus" || a.Correct != 24 || a.Both != 8 || a.Answers != 24 {
		t.Errorf("the agreement is %+v", a)
	}
}

func TestWritten(t *testing.T) {
	r := helpOn(6, 1).report(t)
	var md bytes.Buffer
	if err := r.Markdown(&md, "run.jsonl"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"# The measurement of the writing process: the test part",
		"The log run.jsonl, of the prompt abababababab.",
		"| opus | +6.00 | 6 | 0.01562 | 1 | 0 → 0 | 6 → 0 | helps | yes |",
		"**The writing process helps Claude:**",
		"| opus | C against A | +6.00 | 6 | 0.01562 | 1 |",
		"**The log has no end: the run broke off",
	} {
		if !strings.Contains(md.String(), want) {
			t.Errorf("the report lacks %q:\n%s", want, md.String())
		}
	}
	var csv bytes.Buffer
	if err := r.CSV(&csv); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(csv.String()), "\n")
	if len(lines) != 5 || !strings.HasPrefix(lines[0], "judge,form,answers,right,") || !strings.HasPrefix(lines[1], "opus,A,10,4,0,6,0,0,") {
		t.Errorf("the CSV is\n%s", csv.String())
	}
}

func TestRefuses(t *testing.T) {
	if _, err := report.New(&measure.Log{}); err == nil {
		t.Error("a log without judges or sheets is reported")
	}
	for name, add := range map[string]func(*builder){
		"a call of a sheet the log does not tell":  func(b *builder) { b.answer(2, judge.FormA, "opus", 1, true) },
		"a call by a judge the run does not have":  func(b *builder) { b.answer(1, judge.FormA, "sonnet", 1, true) },
		"a call of a repeat the run does not have": func(b *builder) { b.answer(1, judge.FormA, "opus", 2, true) },
		"two calls at one place": func(b *builder) {
			b.answer(1, judge.FormA, "opus", 1, true)
			b.answer(1, judge.FormA, "opus", 1, false)
		},
	} {
		b := newLog(1, "opus")
		b.sheet(1, sample.OutcomeCorrect, 0)
		add(b)
		if _, err := report.New(&b.log); err == nil {
			t.Errorf("%s: reported", name)
		}
	}
}

// A judge with fewer than five sheets with d ≠ 0 is named, with its smallest p; and a call that
// was not asked is not a call made, and has no time or cost.
func TestFewAndUnasked(t *testing.T) {
	b := helpOn(3, 1)
	b.log.Calls[0].NotAsked, b.log.Calls[0].Error, b.log.Calls[0].LatencyMs, b.log.Calls[0].CostUSD = true, "judge: the request", 0, 0
	r := b.report(t)
	var md bytes.Buffer
	if err := r.Markdown(&md, "run.jsonl"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"With fewer than 5 sheets with d ≠ 0 a judge cannot pass the test: opus has 3, and its smallest p is 1/8.",
		"Calls: 39 made, 0 of them failed",
	} {
		if !strings.Contains(md.String(), want) {
			t.Errorf("the report lacks %q:\n%s", want, md.String())
		}
	}
	if c := r.Cells[0]; c.TimeP50 != 1000 || !near(c.MeanUSD, 0.01) {
		t.Errorf("form A's time and cost take the call not asked: %+v", c)
	}
}
