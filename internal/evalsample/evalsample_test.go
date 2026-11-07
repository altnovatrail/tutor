package evalsample_test

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"
	observationv1 "github.com/altnovatrail/spec/contracts/novatrail/observation/v1"

	"github.com/altnovatrail/tutor/internal/evalsample"
	"github.com/altnovatrail/tutor/internal/protocol"
	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/sheets"
	"github.com/altnovatrail/tutor/internal/synth"
)

var update = flag.Bool("update", false, "write the drawn samples of testdata anew")

// set is a set of sheets of eval/: its bank and its plan, by their paths from this package.
type set struct{ bank, plan string }

var (
	lines = set{"../../eval/bank.json", "../../eval/sheets.json"}
	free  = set{"../../eval/free-bank.json", "../../eval/free-sheets.json"}
	near  = set{"../../eval/near-bank.json", "../../eval/near-sheets.json"}
)

// read reads a set's bank, as a sample names it, and its plan.
func read(t *testing.T, s set) (*evalsample.Bank, *sample.Plan) {
	t.Helper()
	b, err := evalsample.ReadBank(s.bank)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(s.plan)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	p, err := sample.ReadPlan(f)
	if err != nil {
		t.Fatal(err)
	}
	return b, p
}

// file is a plan sheet drawn from the seed and written as a sample of the protocol, as draw writes
// a sheet of the plan of lines.
func file(t *testing.T, b *evalsample.Bank, sh *sample.PlanSheet, seed uint64) []byte {
	t.Helper()
	return fileOf(t, b, lines.plan, sh, seed)
}

// fileOf is a sheet of a plan drawn from the seed as draw writes it, its ID of the plan's file.
func fileOf(t *testing.T, b *evalsample.Bank, plan string, sh *sample.PlanSheet, seed uint64) []byte {
	t.Helper()
	id, err := evalsample.ID(plan, seed, synth.SimilarShift, sh.Number)
	if err != nil {
		t.Fatal(err)
	}
	es, err := evalsample.Of(synth.Sheet(sh, seed), sh, b, seed, id)
	if err != nil {
		t.Fatalf("sheet %d: %v", sh.Number, err)
	}
	data, err := protocol.MarshalSample(es)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// readBack reads a file as the runner does: no refusal, and the sheet of its plan.
func readBack(t *testing.T, data []byte, b *evalsample.Bank, sh *sample.PlanSheet) *evalsample.Sheet {
	t.Helper()
	es, err := protocol.ReadSample(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	read, refused, err := evalsample.Read(es, sh, b)
	if err != nil || refused != nil {
		t.Fatalf("sheet %d: %v %v", sh.Number, refused, err)
	}
	return read
}

// The bank of a set is named by its path from the root of the repository and the digest of its
// file, wherever it is read from.
func TestReadBank(t *testing.T) {
	b, err := evalsample.ReadBank("../../eval/bank.json")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile("../../eval/bank.json")
	if b.Path != "eval/bank.json" || b.Digest != evalsample.Digest(data) || len(b.Bank.Tasks) != 10 {
		t.Errorf("the bank is %s %s with %d tasks", b.Path, b.Digest, len(b.Bank.Tasks))
	}
	if !regexp.MustCompile(`^sha256:[0-9a-f]{64}$`).MatchString(b.Digest) {
		t.Errorf("the digest %s", b.Digest)
	}
	// A bank outside any repository goes by its file's name alone.
	out := filepath.Join(t.TempDir(), "bank.json")
	if err := os.WriteFile(out, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if b, err := evalsample.ReadBank(out); err != nil || b.Path != "bank.json" {
		t.Errorf("a bank outside: %v %v", b, err)
	}
	if _, err := evalsample.ReadBank(filepath.Join(t.TempDir(), "none.json")); err == nil {
		t.Error("no bank is read")
	}
	if _, err := evalsample.BankOf("x.json", []byte("{}")); err == nil {
		t.Error("a file that is no bank is a bank")
	}
}

// What a drawn sample names of itself: an ID and a writer's code from its seed, in the forms of
// section 11, and canvas 0.0.0 and the set of texts "none" where only canvas's samples page has one
// (the 2026-11-07 amendment to R93 spec).
func TestHeader(t *testing.T) {
	for _, c := range []struct {
		plan  string
		seed  uint64
		shift float64
		want  string
	}{
		{"eval/sheets.json", 1, synth.SimilarShift, "synthetic-1-sheets-07"},
		{"../../eval/Near_Gen.json", 18446744073709551615, synth.SimilarShift, "synthetic-18446744073709551615-near-gen-07"},
		{"near.json", 1, 0.3, "synthetic-1-near-shift-300-07"},
	} {
		if id, err := evalsample.ID(c.plan, c.seed, c.shift, 7); err != nil || id != c.want {
			t.Errorf("the ID of %s: %q %v, want %q", c.plan, id, err, c.want)
		}
	}
	if id, err := evalsample.ID(strings.Repeat("p", 60)+".json", 1, synth.SimilarShift, 7); err == nil {
		t.Errorf("an ID of %d characters: %s", len(id), id)
	}
	codes := map[string]bool{}
	for seed := range uint64(50) {
		c := evalsample.WriterCode(seed)
		if !regexp.MustCompile(`^[0-9A-Z]{6,16}$`).MatchString(c) {
			t.Errorf("the writer's code %s", c)
		}
		codes[c] = true
	}
	if len(codes) != 50 {
		t.Errorf("%d codes of 50 seeds", len(codes))
	}
	b, p := read(t, lines)
	es, err := protocol.ReadSample(bytes.NewReader(file(t, b, &p.Sheets[0], 3)))
	if err != nil {
		t.Fatal(err)
	}
	if es.GetId() != "synthetic-3-sheets-01" || es.GetSynthetic().GetSeed() != "3" || es.GetWriterCode() != evalsample.WriterCode(3) ||
		es.GetCanvasVersion() != "0.0.0" || es.GetTextSetVersion() != "none" || es.License != nil ||
		es.GetPurpose() != evalv1.Purpose_PURPOSE_MEASUREMENT || es.GetWriter() != evalv1.Writer_WRITER_ADULT {
		t.Errorf("the header of a drawn sample: %v", es)
	}
	ref := es.GetTask()
	if ref.GetFile().GetBank() != "eval/bank.json" || ref.GetFile().GetBankDigest() != b.Digest || ref.GetFile().GetFileDigest() != b.Digest ||
		ref.GetTask() != p.Sheets[0].Task || !strings.HasPrefix(ref.GetOpenViewDigest(), "sha256:") {
		t.Errorf("the task: %v", ref)
	}
	if es.GetSheet().GetDigitHeight() != render.Digit || len(es.GetSheet().GetCrops().GetCrops()) != 4 {
		t.Errorf("the sheet: digit %d, %d crops", es.GetSheet().GetDigitHeight(), len(es.GetSheet().GetCrops().GetCrops()))
	}
	for _, e := range es.GetHistory().GetEvents() {
		if w := e.GetWrite(); w != nil && w.GetPointer() != evalsample.Pointer {
			t.Errorf("the stroke %d is of the pointer %q", w.GetId(), w.GetPointer())
		}
	}
}

// Every sheet of the plans that draw draws comes back from its file as the plan sheet it was drawn
// as, with no difference from the plan: the strokes to a tenth, the lines its parts, the outcome
// module-kit's verdict on its truth, the first error, the trap and the planned erasure. The same
// seed writes the same bytes.
func TestReadBack(t *testing.T) {
	for _, s := range []set{lines, free, near} {
		b, p := read(t, s)
		for i := range p.Sheets {
			sh := &p.Sheets[i]
			data := fileOf(t, b, s.plan, sh, 1)
			if !bytes.Equal(data, fileOf(t, b, s.plan, sh, 1)) {
				t.Errorf("%s sheet %d: one seed writes two files", s.plan, sh.Number)
			}
			got := readBack(t, data, b, sh).Sample
			if err := got.Check(); err != nil {
				t.Errorf("%s sheet %d: %v", s.plan, sh.Number, err)
			}
			if diffs := sheets.Differences(sh, got); len(diffs) > 0 {
				t.Errorf("%s sheet %d: off the plan at %v", s.plan, sh.Number, diffs)
			}
			drawn := synth.Sheet(sh, 1)
			if len(got.Strokes) != len(drawn.Strokes) || len(got.Erasures) != len(drawn.Erasures) || got.Writer != sample.WriterSynthetic {
				t.Errorf("%s sheet %d: %d strokes and %d erasures, drawn %d and %d", s.plan, sh.Number, len(got.Strokes), len(got.Erasures), len(drawn.Strokes), len(drawn.Erasures))
			}
		}
	}
}

// The markup of a sheet, and the truth of its one answer slot, by what was drawn: a value, or why a
// reader gives none.
func TestMarkup(t *testing.T) {
	b, p := read(t, lines)
	sh := &p.Sheets[0]
	drawn := synth.Sheet(sh, 1)
	truthOf := func(s *sample.Sample) (*evalv1.Markup, error) {
		es, err := evalsample.Of(s, sh, b, 1, "synthetic-1-sheets-01")
		if err != nil {
			return nil, err
		}
		return es.GetMarkup(), nil
	}
	m, err := truthOf(drawn)
	if err != nil {
		t.Fatal(err)
	}
	answer := drawn.Lines[sh.Answer-1]
	if v := m.GetTruth().GetAnswerSlots()[0].GetValue(); v != sample.ValueOf(answer.Text) || len(m.GetNotes()) != 0 {
		t.Errorf("the truth %v and the notes %v", m.GetTruth(), m.GetNotes())
	}
	for i, part := range m.GetParts().GetParts() {
		if part.GetKind() != evalv1.PartKind_PART_KIND_TEXT || part.GetText() != drawn.Lines[i].Text || part.GetFinalAnswer() != (i == sh.Answer-1) || !slices.IsSorted(part.GetStrokeIds()) {
			t.Errorf("part %d: %v", i+1, part)
		}
	}

	reason := func(change func(s *sample.Sample)) (observationv1.Unreadable, []*evalv1.AnswerSlotNote) {
		t.Helper()
		s := synth.Sheet(sh, 1)
		change(s)
		m, err := truthOf(s)
		if err != nil {
			t.Fatal(err)
		}
		return m.GetTruth().GetAnswerSlots()[0].GetUnreadable(), m.GetNotes()
	}
	for _, c := range []struct {
		name   string
		change func(s *sample.Sample)
		want   observationv1.Unreadable
	}{
		{"no answer", func(s *sample.Sample) { s.Lines[sh.Answer-1].Answer = false }, observationv1.Unreadable_UNREADABLE_NO_FINAL_ANSWER},
		{"two answers", func(s *sample.Sample) { s.Lines[0].Answer, s.Lines[sh.Answer-1].Answer = true, true }, observationv1.Unreadable_UNREADABLE_SEVERAL_FINAL_ANSWERS},
		{"an illegible answer", func(s *sample.Sample) { s.Lines[sh.Answer-1].Illegible = true }, observationv1.Unreadable_UNREADABLE_ILLEGIBLE},
		{"a blank sheet", func(s *sample.Sample) {
			var gone []string
			for _, st := range s.Strokes {
				gone = append(gone, st.ID)
			}
			last := s.Strokes[len(s.Strokes)-1].Points
			s.Erasures = []sample.Erasure{{TMs: last[len(last)-1].TMs + 1, Strokes: gone}}
		}, observationv1.Unreadable_UNREADABLE_BLANK},
	} {
		if got, _ := reason(c.change); got != c.want {
			t.Errorf("%s: %s, want %s", c.name, got, c.want)
		}
	}
	if _, notes := reason(func(s *sample.Sample) { s.Lines[0].CrossedOut = true }); len(notes) != 1 || !notes[0].GetCrossedOut() || notes[0].GetMirrored() {
		t.Errorf("a part crossed out: %v", notes)
	}

	// What Of does not write: a sheet of the author's, an inscription, a task of another bank, an
	// answer that gives no number.
	for _, c := range []struct {
		name   string
		change func(s *sample.Sample)
		says   string
	}{
		{"the author's sheet", func(s *sample.Sample) { s.Writer = sample.WriterAuthor }, "the sheets code drew"},
		{"an inscription", func(s *sample.Sample) { s.Injection = &sample.Injection{} }, "an inscription"},
		{"another task", func(s *sample.Sample) { s.Task = "no-such-task" }, "is not the plan sheet's"},
		{"an answer without a number", func(s *sample.Sample) { s.Lines[sh.Answer-1].Text = "ответ" }, "gives no number"},
		{"a line's stroke that is not there", func(s *sample.Sample) { s.Lines[0].Strokes = append(s.Lines[0].Strokes, "s-none") }, "which the sheet has not"},
		{"an erasure of a stroke that is not there", func(s *sample.Sample) { s.Erasures = []sample.Erasure{{TMs: 1, Strokes: []string{"s-none"}}} }, "which the sheet has not"},
		{"a stroke without points", func(s *sample.Sample) { s.Strokes[0].Points = nil }, "has no points"},
		{"a time past a history's", func(s *sample.Sample) { s.Erasures = []sample.Erasure{{TMs: 1 << 40}} }, "is not one a history holds"},
		{"a sheet of a fraction of a unit", func(s *sample.Sample) { s.Sheet.Width = 999.5 }, "whole units"},
	} {
		s := synth.Sheet(sh, 1)
		c.change(s)
		if _, err := truthOf(s); err == nil || !strings.Contains(err.Error(), c.says) {
			t.Errorf("%s: %v, want %q", c.name, err, c.says)
		}
	}
}

// The heights of a set of crops are eval's and the module's, ascending and each once.
func TestHeights(t *testing.T) {
	if got := evalsample.Heights(render.Digit); !slices.Equal(got, []int{16, 24, 32, 48}) {
		t.Errorf("at %d: %v", render.Digit, got)
	}
	if got := evalsample.Heights(40); !slices.Equal(got, []int{16, 24, 32, 40, 48}) {
		t.Errorf("at 40: %v", got)
	}
}

// A sample is refused as section 13 has it, for each of its reasons, and a sample that is no sheet
// of measurement with its history is no sheet the runner reads.
func TestRefusals(t *testing.T) {
	b, p := read(t, lines)
	sh := &p.Sheets[1]
	data := file(t, b, sh, 1)
	other, _ := read(t, free)
	for _, c := range []struct {
		name   string
		change func(es *evalv1.Sample)
		bank   *evalsample.Bank
		plan   *sample.PlanSheet
		want   evalv1.Refusal
	}{
		{"no markup", func(es *evalv1.Sample) { es.Markup = nil }, b, sh, evalv1.Refusal_REFUSAL_NO_MARKUP},
		{"a task of a release", func(es *evalv1.Sample) {
			es.Task.Source = &evalv1.TaskReference_Tutor{Tutor: &evalv1.TutorSource{Tag: "v0.1.4", Bank: b.Path}}
		}, b, sh, evalv1.Refusal_REFUSAL_OTHER_TASK},
		{"another bank", nil, other, sh, evalv1.Refusal_REFUSAL_OTHER_TASK},
		{"a bank of another digest", func(es *evalv1.Sample) { es.Task.GetFile().BankDigest = "sha256:" + strings.Repeat("0", 64) }, b, sh, evalv1.Refusal_REFUSAL_OTHER_TASK},
		{"no such task", func(es *evalv1.Sample) { es.Task.Task = "no-such-task" }, b, sh, evalv1.Refusal_REFUSAL_OTHER_TASK},
		{"another plan sheet's task", nil, b, &p.Sheets[0], evalv1.Refusal_REFUSAL_OTHER_TASK},
		{"an open view of another digest", func(es *evalv1.Sample) { es.Task.OpenViewDigest = "sha256:" + strings.Repeat("0", 64) }, b, sh, evalv1.Refusal_REFUSAL_OTHER_TASK},
		{"a truth of no valid observation", func(es *evalv1.Sample) { es.Markup.Truth.AnswerSlots = nil }, b, sh, evalv1.Refusal_REFUSAL_INVALID},
		{"marks of criteria", func(es *evalv1.Sample) {
			es.Markup.Writing = &evalv1.Markup_Marks{Marks: &evalv1.CriterionMarks{Marks: []*evalv1.CriterionMark{{Criterion: 1, Met: true}}}}
		}, b, sh, evalv1.Refusal_REFUSAL_INVALID},
		{"a child's", func(es *evalv1.Sample) { es.Writer = evalv1.Writer_WRITER_CHILD }, b, sh, evalv1.Refusal_REFUSAL_CHILD},
	} {
		t.Run(c.name, func(t *testing.T) {
			es, err := protocol.ReadSample(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if c.change != nil {
				c.change(es)
			}
			read, refused, err := evalsample.Read(es, c.plan, c.bank)
			if err != nil || read != nil || refused == nil || refused.Reason != c.want {
				t.Fatalf("%v %v %v, want %s", read, refused, err, c.want)
			}
			if !strings.HasPrefix(refused.Error(), evalsample.Reason(c.want)+": ") {
				t.Errorf("the refusal says %q", refused.Error())
			}
		})
	}
	for _, r := range []evalv1.Refusal{evalv1.Refusal_REFUSAL_INVALID, evalv1.Refusal_REFUSAL_NO_MARKUP, evalv1.Refusal_REFUSAL_OTHER_TASK, evalv1.Refusal_REFUSAL_CHILD, evalv1.Refusal_REFUSAL_UNSPECIFIED} {
		if evalsample.Reason(r) == "" {
			t.Errorf("%s has no reason", r)
		}
	}

	es, _ := protocol.ReadSample(bytes.NewReader(data))
	es.Ink = &evalv1.Sample_Geometry{Geometry: &evalv1.Geometry{}}
	if _, refused, err := evalsample.Read(es, sh, b); err == nil || refused != nil {
		t.Errorf("a sample of the final geometry: %v %v", refused, err)
	}
}

// The outcome of a sheet is module-kit's verdict on its truth, by the task's key: a value right or
// not, an answer illegible or ambiguous, and no answer, several or none on a blank sheet.
func TestOutcome(t *testing.T) {
	b, p := read(t, lines)
	sh := &p.Sheets[0]
	data := file(t, b, sh, 1)
	key := sample.KeyOf(b.Bank.Tasks[slices.IndexFunc(b.Bank.Tasks, func(tk sample.Task) bool { return tk.ID == sh.Task })].Key)
	value := func(v string) *observationv1.AnswerSlotObservation_Value {
		return &observationv1.AnswerSlotObservation_Value{Value: v}
	}
	reason := func(r observationv1.Unreadable) *observationv1.AnswerSlotObservation_Unreadable {
		return &observationv1.AnswerSlotObservation_Unreadable{Unreadable: r}
	}
	for _, c := range []struct {
		read any
		want sample.Outcome
	}{
		{value(key), sample.OutcomeCorrect},
		{value(key + "1"), sample.OutcomeError},
		{value("x"), sample.OutcomeUnreadable},
		{reason(observationv1.Unreadable_UNREADABLE_ILLEGIBLE), sample.OutcomeUnreadable},
		{reason(observationv1.Unreadable_UNREADABLE_AMBIGUOUS), sample.OutcomeUnreadable},
		{reason(observationv1.Unreadable_UNREADABLE_NO_FINAL_ANSWER), sample.OutcomeNoAnswer},
		{reason(observationv1.Unreadable_UNREADABLE_SEVERAL_FINAL_ANSWERS), sample.OutcomeNoAnswer},
		{reason(observationv1.Unreadable_UNREADABLE_BLANK), sample.OutcomeNoAnswer},
	} {
		es, _ := protocol.ReadSample(bytes.NewReader(data))
		slot := es.GetMarkup().GetTruth().GetAnswerSlots()[0]
		switch r := c.read.(type) {
		case *observationv1.AnswerSlotObservation_Value:
			slot.Read = r
		case *observationv1.AnswerSlotObservation_Unreadable:
			slot.Read = r
		}
		read, refused, err := evalsample.Read(es, sh, b)
		if err != nil || refused != nil || read.Sample.Expected.Outcome != c.want {
			t.Errorf("%v: %v %v %v, want %s", c.read, read, refused, err, c.want)
			continue
		}
		want := ""
		if c.want == sample.OutcomeError {
			want = sh.Expected.Trap
		}
		if trap := read.Sample.Expected.Trap; trap != want {
			t.Errorf("%v: the trap %q", c.read, trap)
		}
	}
}

// Who wrote a sample, as tutor's code names a writer, and the mouse by the pointer.
func TestWriter(t *testing.T) {
	b, p := read(t, lines)
	sh := &p.Sheets[0]
	data := file(t, b, sh, 1)
	for _, c := range []struct {
		change func(es *evalv1.Sample)
		want   sample.Writer
		mouse  bool
	}{
		{func(*evalv1.Sample) {}, sample.WriterSynthetic, false},
		{func(es *evalv1.Sample) { es.Origin = &evalv1.Sample_Person{Person: &evalv1.Person{}} }, sample.WriterAuthor, false},
		{func(es *evalv1.Sample) {
			es.Origin = &evalv1.Sample_Person{Person: &evalv1.Person{}}
			es.Writer = evalv1.Writer_WRITER_ADULT_AS_CHILD
			es.GetHistory().GetEvents()[0].GetWrite().Pointer = "mouse"
		}, sample.WriterAuthorAsChild, true},
	} {
		es, _ := protocol.ReadSample(bytes.NewReader(data))
		c.change(es)
		read, refused, err := evalsample.Read(es, sh, b)
		if err != nil || refused != nil || read.Sample.Writer != c.want || read.Mouse != c.mouse {
			t.Errorf("%v %v: %v, want %s and the mouse %v", refused, err, read, c.want, c.mouse)
		}
	}
}

// drawn are the plan sheets whose samples testdata holds: one of each scenario of the plan of lines,
// a free sheet with a drawing and a planned erasure, and a sheet of the near key.
func drawn(t *testing.T) []struct {
	name string
	set  set
	sh   *sample.PlanSheet
} {
	t.Helper()
	var out []struct {
		name string
		set  set
		sh   *sample.PlanSheet
	}
	add := func(name string, s set, sh *sample.PlanSheet) {
		out = append(out, struct {
			name string
			set  set
			sh   *sample.PlanSheet
		}{fmt.Sprintf("%s-%02d-%s.json", name, sh.Number, sh.Scenario), s, sh})
	}
	_, p := read(t, lines)
	seen := map[sample.Scenario]bool{}
	for i := range p.Sheets {
		if sh := &p.Sheets[i]; !seen[sh.Scenario] {
			seen[sh.Scenario] = true
			add("lines", lines, sh)
		}
	}
	_, p = read(t, free)
	for i := range p.Sheets {
		sh := &p.Sheets[i]
		if sh.Erasure != nil && slices.ContainsFunc(sh.Parts, func(pt sample.PlanPart) bool { return pt.Kind == sample.PartDrawing }) {
			add("free", free, sh)
			break
		}
	}
	_, p = read(t, near)
	add("near", near, &p.Sheets[0])
	return out
}

// testdata holds a drawn sample of each kind that tutor-eval draw draws, with the seed 1 — of the
// sheets, a page of the workbook with every scenario of a box and both writings of a fraction, and an
// essay (R71) —, and they are what draw writes, to the byte: CI holds them to the rules of sections 11 and 12 with spec's
// sample-check (just ci-sample-check), so that a drawn file that breaks a rule fails a pull request.
// go test -update writes them anew.
func TestDrawn(t *testing.T) {
	want := map[string]bool{}
	for _, d := range drawn(t) {
		want[d.name] = true
		b, _ := read(t, d.set)
		data := fileOf(t, b, d.set.plan, d.sh, 1)
		path := filepath.Join("testdata", d.name)
		if *update {
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			continue
		}
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, data) {
			t.Errorf("%s is not what draw writes, %v: run go test ./internal/evalsample -update to write it anew", path, err)
		}
		readBack(t, got, b, d.sh)
	}
	w, f := readWorkbook(t), readEssays(t)
	for name, data := range map[string][]byte{
		"workbook-05.json": pageFile(t, w, &w.Workbook.Pages[4], 1),
		"essays-16.json":   essayFile(t, f, &f.Essays.Essays[15], 1),
	} {
		want[name] = true
		path := filepath.Join("testdata", name)
		if *update {
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, data) {
			t.Errorf("%s is not what draw writes, %v: run go test ./internal/evalsample -update to write it anew", path, err)
		}
	}
	if len(want) != 9 {
		t.Errorf("%d drawn samples, want 9: %v", len(want), want)
	}
	files, _ := filepath.Glob(filepath.Join("testdata", "*.json"))
	for _, f := range files {
		if !want[filepath.Base(f)] {
			t.Errorf("%s is no drawn sample of the test", f)
		}
	}
}

// The groups of a sheet, as the crop takes them, name the strokes as tutor's code does.
func TestGroupsOf(t *testing.T) {
	got := evalsample.GroupsOf([]*evalv1.Group{{Label: "A", StrokeIds: []int32{1, 3}}, {Label: "B", StrokeIds: []int32{2}}})
	if len(got) != 2 || got[0].Label != "A" || !slices.Equal(got[0].Strokes, []string{"1", "3"}) || !slices.Equal(got[1].Strokes, []string{"2"}) {
		t.Errorf("%+v", got)
	}
	if _, err := evalsample.CropSet(nil, nil, []int{32}); err == nil {
		t.Error("a set of crops of no strokes")
	}
}
