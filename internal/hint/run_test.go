package hint_test

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/png" // the crops a request shows
	"strings"
	"testing"
	"time"

	"github.com/altnovatrail/tutor/internal/hint"
	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/runner"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/sheets"
)

func clock() time.Time { return time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC) }

// The tutors are the three configurations of the runner, each with its output for the estimate.
func TestTutors(t *testing.T) {
	var names []string
	for _, tu := range hint.Tutors() {
		names = append(names, tu.Name)
		if tu.Output < 200 {
			t.Errorf("%s is expected to write %d tokens", tu.Name, tu.Output)
		}
	}
	if strings.Join(names, " ") != "opus-high haiku-low haiku-low-no-thinking" {
		t.Errorf("the tutors are %v", names)
	}
}

// A run asks each sheet with a mistake at every rung in both languages, by every tutor, repeat by
// repeat, on the crop at 32 px; the estimate takes the tokens of the text of a hint and those of
// its crop. A run of no sheets, of no repeats or of a sheet whose answer is right is refused.
func TestNew(t *testing.T) {
	// Sheets 2 and 4 of the plan of free writing have a mistake; sheet 1 is correct.
	free := chosen(t, "free", freePlan, freeBank, 2, 4)
	r, err := hint.New("dev", []string{"free"}, free, hint.Tutors(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if want := 2 * 2 * 3 * 2 * 3; len(r.Steps) != want || len(r.Items) != 2 {
		t.Errorf("the run makes %d calls of %d sheets, want %d of 2", len(r.Steps), len(r.Items), want)
	}
	var order []string
	for _, st := range r.Steps {
		rec := st.Call.Record(runner.Outcome{NotAsked: true, Error: "no"}).(hint.CallRecord)
		order = append(order, fmt.Sprintf("%d/%d/%d/%s/%s", rec.Repeat, rec.Sheet, rec.Rung, rec.Lang, rec.Tutor))
		req, err := st.Call.Request()
		if err != nil {
			t.Fatal(err)
		}
		img, _, err := image.DecodeConfig(bytes.NewReader(req.Images[0].PNG))
		if err != nil {
			t.Fatal(err)
		}
		if in := int64(1700 + render.Tokens(img.Width, img.Height)); st.In != in || st.WorstIn != 8000 || !rec.NotAsked {
			t.Errorf("the call %+v expects %d tokens in, want %d", rec.Call, st.In, in)
		}
	}
	if got := strings.Join(order[:4], " "); got != "1/2/1/ru/opus-high 1/2/1/ru/haiku-low 1/2/1/ru/haiku-low-no-thinking 1/2/1/en/opus-high" {
		t.Errorf("the run starts %s", got)
	}
	if got := order[len(order)-1]; got != "2/4/3/en/haiku-low-no-thinking" {
		t.Errorf("the run ends %s", got)
	}
	if _, err := hint.New("dev", nil, free, hint.Tutors(), 0); err == nil {
		t.Error("a run of no repeats is planned")
	}
	if _, err := hint.New("dev", nil, nil, hint.Tutors(), 1); err == nil {
		t.Error("a run of no sheets is planned")
	}
	right := chosen(t, "free", freePlan, freeBank, 1)
	if _, err := hint.New("dev", nil, right, hint.Tutors(), 1); err == nil || !strings.Contains(err.Error(), "only after a wrong answer") {
		t.Errorf("a hint is planned on a correct sheet: %v", err)
	}
	broken := chosen(t, "free", freePlan, freeBank, 2)
	empty := *broken[0].Sample
	empty.Strokes, empty.Lines = nil, nil
	broken[0].Sample = &empty
	if _, err := hint.New("dev", nil, broken, hint.Tutors(), 1); err == nil {
		t.Error("a hint is planned on a sheet with no strokes")
	}
}

// A run on the stub goes the whole way: the log tells the run with its tutors, rungs, languages,
// style and prompt, each sheet with its first error, trap and crop, and each call with a box that
// hits the first error and the sheet's trap. A log of another kind is no log of the hint.
func TestRunOnTheStub(t *testing.T) {
	lines := chosen(t, "lines", linesPlan, linesBank, 3, 9)
	free := chosen(t, "free", freePlan, freeBank, 2, 8, 12)
	near := chosen(t, "near", nearPlan, nearBank, 12)
	all := append(append(lines, free...), near...)
	r, err := hint.New("dev", []string{"lines", "free", "near"}, all, hint.Tutors(), 1)
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	res, err := r.Do(context.Background(), runner.Stub, 10, &log, clock)
	if err != nil {
		t.Fatal(err)
	}
	if e := r.Estimate(); res.Calls != e.Calls || res.Calls != 6*6*3 || res.Stopped != "" {
		t.Errorf("the run ended %+v, want all %d calls", res, e.Calls)
	}
	if kind := runner.CallKind(log.Bytes()); kind != hint.CallKind {
		t.Errorf("the log is of the kind %q", kind)
	}
	l, err := hint.ReadLog(bytes.NewReader(log.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if l.Run.Prompt != hint.Fingerprint() || l.Run.PromptVersion != hint.Version || len(l.Run.Sets) != 3 || len(l.Run.Tutors) != 3 || len(l.Run.Rungs) != 3 ||
		len(l.Run.Langs) != 2 || l.Run.Style != hint.Style || len(l.Sheets) != 6 || l.End == nil || l.End.Stopped != "" {
		t.Errorf("the log reads back as %+v with %d sheets", l.Run, len(l.Sheets))
	}
	sheetOf := map[string]hint.SheetRecord{}
	for _, sh := range l.Sheets {
		sheetOf[fmt.Sprintf("%s/%d", sh.Set, sh.Sheet)] = sh
		if sh.Outcome != sample.OutcomeError || sh.FirstError == 0 || sh.Key == "" || sh.Digit != 32 || sh.Width < 200 || sh.Height < 200 {
			t.Errorf("the sheet %s/%d is told %+v", sh.Set, sh.Sheet, sh)
		}
	}
	if s := sheetOf["free/12"]; s.Layout != sample.LayoutNumberLine || s.FirstErrorIn != sample.RegionDrawing || s.Trap != "jump-length" {
		t.Errorf("free/12 is told %+v", s)
	}
	if s := sheetOf["lines/3"]; s.Layout != "" || s.FirstErrorIn != "" || s.Trap != "order-of-operations" {
		t.Errorf("lines/3 is told %+v", s)
	}
	for _, c := range l.Calls {
		sh := sheetOf[fmt.Sprintf("%s/%d", c.Set, c.Sheet)]
		if c.Hit != sh.FirstError || c.Place == nil || c.Box == nil || c.Trap != sh.Trap || c.Hint == "" || c.Error != "" || c.InputTokens == 0 {
			t.Errorf("the call %+v is logged %+v", c.Call, c)
		}
	}
	if len(l.Calls) != res.Calls {
		t.Errorf("%d calls read back, %d made", len(l.Calls), res.Calls)
	}
	reading := `{"kind": "run", "call_kind": "reading", "part": "test"}` + "\n"
	if _, err := hint.ReadLog(strings.NewReader(reading)); err == nil || !strings.Contains(err.Error(), "not of the hint") {
		t.Errorf("a log of the reading reads as the hint's: %v", err)
	}
	if _, err := hint.ReadLog(strings.NewReader(`{"kind": "run", "call_kind": "hint"}` + "\n" + `{"kind": "frame"}` + "\n")); err == nil {
		t.Error("a record of no kind of the hint's is read")
	}
}

// An answer off the schema is a failure of the call: the record keeps the answer and the error, and
// has no box. A box off the crop has no place and hits nothing. The stub has no answer on a sheet
// without a first error.
func TestRecord(t *testing.T) {
	free := chosen(t, "free", freePlan, freeBank, 12)
	r, err := hint.New("dev", []string{"free"}, free, hint.Tutors()[:1], 1)
	if err != nil {
		t.Fatal(err)
	}
	call := r.Steps[0].Call
	rec := call.Record(runner.Outcome{Answer: `{"verdict": "error"}`}).(hint.CallRecord)
	if rec.Answer == "" || !strings.Contains(rec.Error, "off the schema") || rec.Box != nil || rec.Hit != 0 {
		t.Errorf("an answer off the schema is logged %+v", rec)
	}
	rec = call.Record(runner.Outcome{Answer: `{"box": {"x1": -50, "y1": -50, "x2": -10, "y2": -10}, "trap": null, "hint": "?"}`}).(hint.CallRecord)
	if rec.Box == nil || rec.Place != nil || rec.Hit != 0 || rec.Error != "" || rec.Hint != "?" {
		t.Errorf("a box off the crop is logged %+v", rec)
	}
	rec = call.Record(runner.Outcome{Answer: `{"box": null, "trap": "jump-count", "hint": "?"}`}).(hint.CallRecord)
	if rec.Box != nil || rec.Trap != "jump-count" || rec.Error != "" {
		t.Errorf("an answer without a box is logged %+v", rec)
	}
	unmarked := *free[0].Sample
	unmarked.Expected.FirstError = nil
	r, err = hint.New("dev", []string{"free"}, []sheets.Chosen{{Set: "free", Number: 12, Sample: &unmarked, Task: free[0].Task, Plan: free[0].Plan}}, hint.Tutors()[:1], 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Steps[0].Call.Right(); err == nil {
		t.Error("the stub boxes a sheet without a first error")
	}
}
