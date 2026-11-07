package evalsample_test

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"

	"github.com/altnovatrail/tutor/internal/evalsample"
	"github.com/altnovatrail/tutor/internal/protocol"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/synth"
)

// A person's ID is 26 lowercase letters and digits at random, which section 11 takes, and never
// the same twice.
func TestNewID(t *testing.T) {
	seen := map[string]bool{}
	for range 100 {
		id, err := evalsample.NewID()
		if err != nil {
			t.Fatal(err)
		}
		if !regexp.MustCompile(`^[a-z2-7]{26}$`).MatchString(id) || seen[id] {
			t.Fatalf("NewID = %q, want 26 lowercase letters and digits, new each time", id)
		}
		seen[id] = true
	}
}

// The writer's code is made at random the first time, in a file its owner alone reads, in a
// directory made for it, and read back the same every time after; a file that holds no code is an
// error and stays as it is.
func TestWriterCodeAt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "samples", "writer-code")
	code, err := evalsample.WriterCodeAt(path)
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[0-9A-Z]{12}$`).MatchString(code) {
		t.Errorf("the code = %q, want 12 digits and capital letters", code)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("the file of the code: %v, %v; want it its owner's alone", info, err)
	}
	if again, err := evalsample.WriterCodeAt(path); err != nil || again != code {
		t.Errorf("the code read again = %q, %v; want %q", again, err, code)
	}
	if other, err := evalsample.WriterCodeAt(filepath.Join(t.TempDir(), "writer-code")); err != nil || other == code {
		t.Errorf("another file's code = %q, %v; want a code of its own", other, err)
	}
	if err := os.WriteFile(path, []byte("abc\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := evalsample.WriterCodeAt(path); err == nil || !strings.Contains(err.Error(), "holds no writer's code") {
		t.Errorf("a file of no code: %v", err)
	}
	if data, _ := os.ReadFile(path); string(data) != "abc\n" {
		t.Errorf("the file of no code is now %q, want it as it was", data)
	}
	if _, err := evalsample.WriterCodeAt(t.TempDir()); err == nil {
		t.Error("a directory reads as a file of the code")
	}
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := evalsample.WriterCodeAt(filepath.Join(blocked, "writer-code")); err == nil {
		t.Error("a code is made under a file")
	}
}

// PersonOf writes the sheets a person wrote, and the sheets code drew are Of's.
func TestPersonOfRefusesCode(t *testing.T) {
	plan, samples := synth.Plan()
	s := *samples[0]
	s.Writer = "synthetic"
	h, ids, err := evalsample.HistoryOf(&s, "pen")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := evalsample.PersonOf(&s, &plan.Sheets[0], nil, h, ids, "id", "CODE01"); err == nil || !strings.Contains(err.Error(), "the sheets a person wrote") {
		t.Errorf("PersonOf of a sheet code drew: %v", err)
	}
}

// person writes a sheet as the record mode writes it, a person's sample by the pointer given, and
// reads it back as the runner does.
func person(t *testing.T, s *sample.Sample, sh *sample.PlanSheet, b *evalsample.Bank) (*evalv1.Sample, error) {
	t.Helper()
	h, ids, err := evalsample.HistoryOf(s, "mouse")
	if err != nil {
		t.Fatal(err)
	}
	return evalsample.PersonOf(s, sh, b, h, ids, "test-sample", "TESTCODE01")
}

// A person's sheet is a sample of measurement from a person, an adult in their own hand or as a child,
// with the ID and the code given, canvas 0.0.0 and no set of texts, its strokes by the pointer given;
// the runner reads it back as the sheet written. A stroke it shows in no line, and an inscription,
// keep it from its sample.
func TestPersonOf(t *testing.T) {
	b, plan := read(t, lines)
	sh := &plan.Sheets[0]
	s := synth.Sheet(sh, 1)
	s.Writer = sample.WriterAuthor
	es, err := person(t, s, sh, b)
	if err != nil {
		t.Fatal(err)
	}
	if es.GetPerson() == nil || es.GetWriter() != evalv1.Writer_WRITER_ADULT || es.GetId() != "test-sample" || es.GetWriterCode() != "TESTCODE01" ||
		es.GetCanvasVersion() != evalsample.CanvasVersion || es.GetTextSetVersion() != evalsample.TextSetVersion || es.GetPurpose() != evalv1.Purpose_PURPOSE_MEASUREMENT {
		t.Errorf("the sample: %v by %v, ID %q, code %q", es.GetOrigin(), es.GetWriter(), es.GetId(), es.GetWriterCode())
	}
	if p := es.GetHistory().GetEvents()[0].GetWrite().GetPointer(); p != "mouse" {
		t.Errorf("the pointer = %q, want the one given", p)
	}
	read := readBack(t, marshalOf(t, es), b, sh)
	if !read.Mouse || read.Sample.Writer != sample.WriterAuthor || len(read.Sample.Strokes) != len(s.Strokes) || len(read.Sample.Lines) != len(s.Lines) {
		t.Errorf("read back: mouse %v, %s, %d strokes, %d lines", read.Mouse, read.Sample.Writer, len(read.Sample.Strokes), len(read.Sample.Lines))
	}
	child := *s
	child.Writer = sample.WriterAuthorAsChild
	if es, err := person(t, &child, sh, b); err != nil || es.GetWriter() != evalv1.Writer_WRITER_ADULT_AS_CHILD {
		t.Errorf("as a child: %v, %v", es.GetWriter(), err)
	}

	stray := *s
	stray.Strokes = append(slices.Clone(s.Strokes), sample.Stroke{ID: "s-stray", Points: []sample.Point{{X: 500, Y: 590, TMs: 1 << 30}}})
	if id := evalsample.Loose(&stray); id != "s-stray" {
		t.Errorf("Loose = %q, want the stray stroke", id)
	}
	if id := evalsample.Loose(s); id != "" {
		t.Errorf("Loose of a sheet whose strokes are all in lines = %q", id)
	}
	if _, err := person(t, &stray, sh, b); err == nil || !strings.Contains(err.Error(), `the stroke "s-stray", which the sheet shows, is in no part`) {
		t.Errorf("a stroke in no line: %v", err)
	}
	inj := *s
	inj.Injection = &sample.Injection{Text: "x", Lang: sample.LangEnglish, Strokes: []string{s.Strokes[0].ID}}
	if id := evalsample.Loose(&inj); id != "" {
		t.Errorf("Loose of a sheet with an inscription = %q, want its strokes counted", id)
	}
	if _, err := person(t, &inj, sh, b); err == nil || !strings.Contains(err.Error(), "an inscription to steer the model") {
		t.Errorf("an inscription: %v", err)
	}
	other := *sh
	other.Task = "houses"
	if _, err := person(t, s, &other, b); err == nil || !strings.Contains(err.Error(), "is not the plan sheet's") {
		t.Errorf("another task: %v", err)
	}
}

// marshalOf writes a sample as the protocol does.
func marshalOf(t *testing.T, es *evalv1.Sample) []byte {
	t.Helper()
	data, err := protocol.MarshalSample(es)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// The truth of a free solution a person writes as one part is the key's on a sheet marked correct,
// and on one marked wrong the number after the last word «ответ» of its markup; a markup that names
// another, or none, or the key's where the sheet is wrong, gives none. Code draws no such sheet.
func TestWholeSolution(t *testing.T) {
	b, codePlan := read(t, free)
	_, messy := read(t, set{free.bank, "../../eval/free-messy.json"})
	correct, wrong := &messy.Sheets[2], &messy.Sheets[6] // houses, its key 9
	whole := func(sh *sample.PlanSheet, text string) *sample.Sample {
		t.Helper()
		for i := range codePlan.Sheets {
			code := &codePlan.Sheets[i]
			if code.Task != sh.Task || code.Erasure != nil {
				continue
			}
			s := synth.Sheet(code, 1)
			order := map[string]int{}
			for k, st := range s.Strokes {
				order[st.ID] = k
			}
			var ids []string
			for _, l := range s.Lines {
				ids = append(ids, l.Strokes...)
			}
			slices.SortFunc(ids, func(a, b string) int { return order[a] - order[b] })
			s.Writer, s.Scenario, s.Expected = sample.WriterAuthor, sh.Scenario, sh.Expected
			s.Lines = []sample.Line{{Text: text, Strokes: ids, Answer: true}}
			return s
		}
		t.Fatalf("no sheet of code of %s", sh.Task)
		return nil
	}
	for _, c := range []struct {
		name, text, value, refused string
		sh                         *sample.PlanSheet
	}{
		{"correct", "свободное решение целиком", "9", "", correct},
		{"correct, the answer named", "решение — Ответ: 9", "9", "", correct},
		{"wrong", "с ошибкой: посчитал в 2 раза больше — ответ 10", "10", "", wrong},
		{"correct, another named", "решение — ответ 12", "", "marked correct, and its markup names the answer 12", correct},
		{"wrong, none named", "с ошибкой", "", "names no number after the word «ответ»", wrong},
		{"wrong, the key named", "с ошибкой — ответ 9", "", "the answer its markup names is the key's, 9", wrong},
		{"wrong, a word of «ответ» after it", "Ответ: 10. Проверка ответа: 3 + 7", "10", "", wrong},
	} {
		es, err := person(t, whole(c.sh, c.text), c.sh, b)
		switch {
		case c.refused != "":
			if err == nil || !strings.Contains(err.Error(), c.refused) {
				t.Errorf("%s: %v, want %q", c.name, err, c.refused)
			}
		case err != nil:
			t.Errorf("%s: %v", c.name, err)
		case es.GetMarkup().GetTruth().GetAnswerSlots()[0].GetValue() != c.value:
			t.Errorf("%s: the truth %v, want %q", c.name, es.GetMarkup().GetTruth(), c.value)
		}
	}
	unmarked := whole(correct, "свободное решение целиком")
	unmarked.Expected.Outcome = sample.OutcomeUnreadable
	if _, err := person(t, unmarked, correct, b); err == nil || !strings.Contains(err.Error(), "the sheet is marked unreadable") {
		t.Errorf("a free solution marked unreadable: %v", err)
	}
	drawn := whole(correct, "свободное решение целиком")
	drawn.Writer = sample.WriterSynthetic
	if _, err := evalsample.Of(drawn, correct, b, 1, "synthetic-1-test-03"); err == nil || !strings.Contains(err.Error(), "for the author's hand to write") {
		t.Errorf("a free solution of one part drawn by code: %v", err)
	}
}
