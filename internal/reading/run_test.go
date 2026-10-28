package reading_test

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/png" // the crops a request shows
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/altnovatrail/tutor/internal/reading"
	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/runner"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/sheets"
	"github.com/altnovatrail/tutor/internal/states"
)

// chosen are sheets of a plan as a part of a run takes them, of a set by its name.
func chosen(t *testing.T, set, planPath, bankPath string, numbers ...int) []sheets.Chosen {
	t.Helper()
	all := plan(t, planPath, bankPath)
	var out []sheets.Chosen
	for _, n := range numbers {
		p := all[n-1]
		out = append(out, sheets.Chosen{Set: set, Number: n, File: fmt.Sprintf("%02d.json", n), Sample: p.sample, Task: p.task, Plan: p.plan})
	}
	return out
}

func clock() time.Time { return time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC) }

var (
	keys = reading.Reads{Keys: true}
	all  = reading.Reads{Keys: true, Heights: true, States: true}
)

// A run reads every sheet without a key and with the right key, and a correct sheet with each false
// key of its task too; Opus reads once and Haiku in every repeat, repeat by repeat, on the crop at
// 32 px. The estimate takes the tokens of the text of a reading and those of its crop.
func TestNew(t *testing.T) {
	// Sheet 1 of the plan of free writing is correct, sheet 2 has a mistake (docs/free-sheets.md).
	free := chosen(t, "free", "../../eval/free-sheets.json", "../../eval/free-bank.json", 1, 2)
	r, err := reading.New("dev", []string{"free"}, free, reading.Readers(), 2, keys)
	if err != nil {
		t.Fatal(err)
	}
	// Per repeat and Haiku: 3 conditions on sheet 1, 2 on sheet 2; Opus once.
	if want := 2*2*(3+2) + 1*(3+2); len(r.Steps) != want {
		t.Errorf("the run makes %d calls, want %d", len(r.Steps), want)
	}
	var order []string
	for _, st := range r.Steps {
		if st.WorstIn != 8000 || st.Call == nil {
			t.Errorf("a step %+v", st)
		}
		rec := st.Call.Record(runner.Outcome{NotAsked: true, Error: "no"}).(reading.CallRecord)
		order = append(order, fmt.Sprintf("%d/%d/%s/%s", rec.Repeat, rec.Sheet, rec.Reader, rec.Condition))
		req, err := st.Call.Request()
		if err != nil {
			t.Fatal(err)
		}
		img, _, err := image.DecodeConfig(bytes.NewReader(req.Images[0].PNG))
		if err != nil {
			t.Fatal(err)
		}
		in := int64(1210 + render.Tokens(img.Width, img.Height))
		if rec.Condition != reading.NoKey {
			in += 150
		}
		if st.In != in || rec.Condition != reading.NoKey && rec.Key == "" || rec.Digit != 32 || rec.Drawn != 32 || rec.State != 0 {
			t.Errorf("the call %+v expects %d tokens in, want %d", rec.Call, st.In, in)
		}
		if !strings.Contains(req.Text, fmt.Sprintf("Image 1 is %d pixels wide and %d pixels high.", img.Width, img.Height)) {
			t.Errorf("the request tells the size of its crop, %d × %d, as %q", img.Width, img.Height, req.Text)
		}
	}
	if got := strings.Join(order[:6], " "); got != "1/1/opus-high/none 1/1/opus-high/right 1/1/opus-high/false 1/1/haiku-low/none 1/1/haiku-low/right 1/1/haiku-low/false" {
		t.Errorf("the run starts %s", got)
	}
	if slices.Contains(order, "2/1/opus-high/none") {
		t.Error("Opus reads in the second repeat")
	}
	if slices.Contains(order, "1/2/haiku-low/false") {
		t.Error("a sheet with a mistake is read with a false key")
	}
	if _, err := reading.New("dev", nil, free, reading.Readers(), 0, keys); err == nil {
		t.Error("a run of no repeats is planned")
	}
	if _, err := reading.New("dev", nil, free, reading.Readers(), 1, reading.Reads{}); err == nil {
		t.Error("a run that reads nothing is planned")
	}
}

// The heights and the states are read once, without a key, by Haiku alone: three heights and each
// state of each sheet, in the first repeat, after the conditions of the key.
func TestNewHeightsAndStates(t *testing.T) {
	free := chosen(t, "free", "../../eval/free-sheets.json", "../../eval/free-bank.json", 1, 2)
	r, err := reading.New("dev", []string{"free"}, free, reading.Readers(), 2, all)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, sh := range free {
		n += len(states.Of(sh.Sample, true))
	}
	keyed := 2*2*(3+2) + 1*(3+2)
	if want := keyed + 2*(2*3+n); len(r.Steps) != want {
		t.Errorf("the run makes %d calls, want %d: %d states", len(r.Steps), want, n)
	}
	heights := map[int]int{}
	for _, st := range r.Steps {
		rec := st.Call.Record(runner.Outcome{NotAsked: true}).(reading.CallRecord)
		if rec.Digit == 32 && rec.State == 0 {
			continue
		}
		if rec.Reader == "opus-high" || rec.Repeat != 1 || rec.Condition != reading.NoKey || rec.Key != "" {
			t.Errorf("the call %+v at another height or of a state", rec.Call)
		}
		if rec.State == 0 {
			heights[rec.Digit]++
		} else if rec.Digit != 32 || rec.State > n {
			t.Errorf("the state %+v", rec.Call)
		}
	}
	if want := map[int]int{16: 4, 24: 4, 48: 4}; fmt.Sprint(heights) != fmt.Sprint(want) {
		t.Errorf("the heights read %v, want %v", heights, want)
	}
	if p := reading.PlannedOf(r); p != (reading.Planned{Heights: 12, States: n, StateCalls: 2 * n}) {
		t.Errorf("the run is counted %+v, with %d states", p, n)
	}
	// Only the states, and by Opus alone: nothing to read.
	r, err = reading.New("dev", []string{"free"}, free, reading.Readers()[:1], 1, reading.Reads{States: true})
	if err != nil || len(r.Steps) != 0 {
		t.Errorf("Opus reads %d states, %v", len(r.Steps), err)
	}
}

// A height whose crop is drawn at the height of one read before it, 32 px first, is not read: on a
// sheet as wide as the sheet with nothing tall on it, the crop at 24 and at 48 px is drawn as small
// as at 32, to fit, and only 16 px is read besides.
func TestHeightDrawnAlike(t *testing.T) {
	sh := chosen(t, "free", "../../eval/free-sheets.json", "../../eval/free-bank.json", 1)[0]
	s := *sh.Sample
	s.Strokes = []sample.Stroke{
		{ID: "a", Points: []sample.Point{{X: 10, Y: 100, TMs: 1000, Pressure: 0.5}}},
		{ID: "b", Points: []sample.Point{{X: 990, Y: 100, TMs: 2000, Pressure: 0.5}}},
	}
	s.Erasures = nil
	s.Lines = []sample.Line{{Text: "Ответ: в точке 10", Strokes: []string{"a", "b"}, Answer: true}}
	sh.Sample, sh.Plan = &s, nil
	r, err := reading.New("dev", []string{"free"}, []sheets.Chosen{sh}, reading.Readers()[1:2], 1, reading.Reads{Heights: true})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, st := range r.Steps {
		c := st.Call.Record(runner.Outcome{NotAsked: true}).(reading.CallRecord)
		got = append(got, fmt.Sprintf("%d/%d", c.Digit, c.Drawn))
	}
	if strings.Join(got, " ") != "16/16" {
		t.Errorf("the heights read %v, want 16/16 alone", got)
	}
}

// A run on the stub over two sets goes the whole way: its log tells the kind of call, the sets, what
// it reads and the prompt, each sheet with its set, key, markup, h_med, states and readings of each
// policy, and each call with the verdict of its sheet's outcome and nothing read under the key; a
// state whose answer is not written yet has no result. A log of the judge is no log of the reading.
func TestRunOnTheStub(t *testing.T) {
	lines := chosen(t, "lines", "../../eval/sheets.json", "../../eval/bank.json", 1, 7, 21)
	free := chosen(t, "free", "../../eval/free-sheets.json", "../../eval/free-bank.json", 1, 2, 14)
	r, err := reading.New("dev", []string{"lines", "free"}, append(lines, free...), reading.Readers(), 1, all)
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	res, err := r.Do(context.Background(), runner.Stub, 10, &log, clock)
	if err != nil {
		t.Fatal(err)
	}
	if e := r.Estimate(); res.Calls != e.Calls || res.Stopped != "" {
		t.Errorf("the run ended %+v, want all %d calls", res, e.Calls)
	}
	if kind := runner.CallKind(log.Bytes()); kind != reading.CallKind {
		t.Errorf("the log is of the kind %q", kind)
	}
	l, err := reading.ReadLog(bytes.NewReader(log.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if l.Run.Prompt != reading.Fingerprint() || len(l.Run.Sets) != 2 || len(l.Run.Readers) != 3 || len(l.Sheets) != 6 || l.End == nil ||
		strings.Join(l.Run.Reads, ",") != "keys,heights,states" {
		t.Errorf("the log reads back as %+v with %d sheets", l.Run, len(l.Sheets))
	}
	sheetOf := map[string]reading.SheetRecord{}
	for _, sh := range l.Sheets {
		sheetOf[fmt.Sprintf("%s/%d", sh.Set, sh.Sheet)] = sh
		if sh.Key == "" || len(sh.FalseKeys) == 0 || len(sh.Markup.Parts) == 0 || sh.Markup.Value == "" || sh.Median < 15 || len(sh.States) == 0 || sh.Policies.Pause400 == 0 {
			t.Errorf("the sheet %s/%d is told %+v", sh.Set, sh.Sheet, sh)
		}
	}
	kinds := map[string]int{}
	for _, c := range l.Calls {
		sh := sheetOf[fmt.Sprintf("%s/%d", c.Set, c.Sheet)]
		want := truth[sh.Outcome]
		kind := "keys"
		switch {
		case c.State > 0:
			kind = "states"
			st := sh.States[c.State-1]
			answer := slices.IndexFunc(sh.Markup.Parts, func(p reading.MarkedPart) bool { return p.Answer })
			if st.Parts[answer] != states.Done {
				want = "unreadable"
			}
		case c.Digit != 32:
			kind = "heights"
		}
		kinds[kind]++
		if c.Verdict != string(want) || c.Error != "" || c.UnderKey.Result || len(c.UnderKey.Parts) > 0 || c.InputTokens == 0 {
			t.Errorf("the call %+v is logged %s (%s), under %+v, error %q; want %s", c.Call, c.Verdict, c.Reason, c.UnderKey, c.Error, want)
		}
	}
	if len(l.Calls) != res.Calls || kinds["heights"] == 0 || kinds["states"] == 0 {
		t.Errorf("%d calls read back, %d made: %v", len(l.Calls), res.Calls, kinds)
	}
	judge := `{"kind": "run", "part": "test"}` + "\n"
	if _, err := reading.ReadLog(strings.NewReader(judge)); err == nil || !strings.Contains(err.Error(), "not of the reading") {
		t.Errorf("a log of the judge reads as the reading's: %v", err)
	}
}

// What a run reads is named by a list of keys, heights and states, and nothing else.
func TestParseReads(t *testing.T) {
	r, err := reading.ParseReads("keys,states")
	if err != nil || r != (reading.Reads{Keys: true, States: true}) || strings.Join(r.Names(), ",") != "keys,states" {
		t.Errorf("keys,states reads %+v, %v", r, err)
	}
	if _, err := reading.ParseReads("keys,frames"); err == nil {
		t.Error("frames are read")
	}
}

// An answer off the schema is a failure of the reading: its verdict is unchecked, and the record
// keeps the answer and the error.
func TestOffTheSchema(t *testing.T) {
	free := chosen(t, "free", "../../eval/free-sheets.json", "../../eval/free-bank.json", 1)
	r, err := reading.New("dev", []string{"free"}, free, reading.Readers()[:1], 1, keys)
	if err != nil {
		t.Fatal(err)
	}
	rec := r.Steps[0].Call.Record(runner.Outcome{Answer: `{"verdict": "correct"}`}).(reading.CallRecord)
	if rec.Verdict != "unchecked" || rec.Answer == "" || !strings.Contains(rec.Error, "not an observation") {
		t.Errorf("an answer off the schema is logged %+v", rec)
	}
}
