package workbook_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/altnovatrail/tutor/internal/runner"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/value"
	"github.com/altnovatrail/tutor/internal/workbook"
)

// A run reads each page of its part, a call a page: at 32 px bare and captioned by every reader,
// Opus in the first repeat alone; and with heights, bare at 16, 24 and 48 px by the two Haiku in the
// first repeat. The pages are those of the part.
func TestNew(t *testing.T) {
	wb := theWorkbook(t)
	readers := e4Readers()
	for _, c := range []struct {
		part  string
		reads workbook.Reads
		pages int
		calls int
	}{
		{"dev", workbook.Reads{Captions: true}, 5, 5 * (2 + 2*2*3)},
		{"test", workbook.Reads{Captions: true, Heights: true}, 15, 15 * (2 + 2*2*3 + 2*3)},
		{"all", workbook.Reads{Heights: true}, 20, 20 * 2 * 3},
	} {
		pages := workbook.Pages(wb, c.part)
		r, err := workbook.New(c.part, "eval/workbook.json", 1, pages, readers, 3, c.reads)
		if err != nil {
			t.Fatal(err)
		}
		if len(pages) != c.pages || len(r.Items) != c.pages || len(r.Steps) != c.calls {
			t.Errorf("%s, %v: %d pages, %d items and %d calls, want %d and %d", c.part, c.reads.Names(), len(pages), len(r.Items), len(r.Steps), c.pages, c.calls)
		}
	}
	if _, err := workbook.New("dev", "", 1, workbook.Pages(wb, "dev"), readers, 3, workbook.Reads{}); err == nil {
		t.Error("a run that reads nothing is planned")
	}
	if _, err := workbook.New("dev", "", 1, nil, readers, 3, workbook.Reads{Captions: true}); err == nil {
		t.Error("a run of no page is planned")
	}
	if _, err := workbook.New("dev", "", 1, workbook.Pages(wb, "dev"), readers, 0, workbook.Reads{Captions: true}); err == nil {
		t.Error("a run of no repeat is planned")
	}
	if _, err := workbook.ParseReads("captions,states"); err == nil {
		t.Error("states are read of the workbook")
	}
	if r, err := workbook.ParseReads("heights,captions"); err != nil || !r.Captions || !r.Heights || len(r.Names()) != 2 {
		t.Errorf("ParseReads = %+v, %v", r, err)
	}
}

// On the stub, a run goes the whole way: its log reads back, of the kind workbook, with the run,
// the pages and every call, and every box of every call comes to the verdict a right reading comes
// to.
func TestRunOnTheStub(t *testing.T) {
	wb := theWorkbook(t)
	r, err := workbook.New("dev", "eval/workbook.json", 1, workbook.Pages(wb, "dev"), e4Readers(), 1, workbook.Reads{Captions: true, Heights: true})
	if err != nil {
		t.Fatal(err)
	}
	clock := func() time.Time { return time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC) }
	var log bytes.Buffer
	res, err := r.Do(context.Background(), runner.Stub, 10, &log, clock)
	if err != nil {
		t.Fatal(err)
	}
	if e := r.Estimate(); res.Calls != e.Calls || res.Stopped != "" {
		t.Errorf("the run ended %+v, want all %d calls", res, e.Calls)
	}
	if kind := runner.CallKind(log.Bytes()); kind != workbook.CallKind {
		t.Errorf("the log is of the kind %q", kind)
	}
	l, err := workbook.ReadLog(&log)
	if err != nil {
		t.Fatal(err)
	}
	if l.Run.Prompt != workbook.Fingerprint() || l.Run.Seed != 1 || len(l.Sheets) != 5 || len(l.Calls) != res.Calls || l.End == nil {
		t.Fatalf("the log holds %+v, %d pages and %d calls", l.Run, len(l.Sheets), len(l.Calls))
	}
	truth := map[int][]workbook.BoxRecord{}
	for _, p := range l.Sheets {
		truth[p.Page] = p.Boxes
	}
	for _, c := range l.Calls {
		if c.Error != "" || len(c.Boxes) != 11 || len(c.Items) != 10 {
			t.Fatalf("page %d, %s, %s: %+v", c.Page, c.Condition, c.Reader, c)
		}
		for i, b := range c.Boxes {
			want := truth[c.Page][i]
			if b.Verdict != want.Verdict || b.Trap != want.Trap || (b.Verdict == value.Unreadable) != (want.Scenario == "blank") {
				t.Errorf("page %d, box %d.%d, %s at %d px: %s %s, want %s %s", c.Page, b.Item, b.Slot, c.Condition, c.Digit, b.Verdict, b.Trap, want.Verdict, want.Trap)
			}
		}
	}
	if _, err := workbook.ReadLog(bytes.NewReader([]byte(`{"kind":"run","call_kind":"reading"}` + "\n"))); err == nil {
		t.Error("a log of the reading reads as the workbook's")
	}
}

// A page with no filled box goes to no model, and a run of such pages alone asks nothing and is not
// planned. A run of heights alone names only the readers that read them. A log of the workbook
// holds pages, and a record of a sheet has no place in it.
func TestNewAsksNothingIdle(t *testing.T) {
	wb := theWorkbook(t)
	empty := *workbook.Pages(wb, "dev")[0]
	empty.Items = append([]sample.WorkbookItem(nil), empty.Items...)
	for i := range empty.Items {
		boxes := append([]sample.WorkbookBox(nil), empty.Items[i].Boxes...)
		for k := range boxes {
			boxes[k].Scenario, boxes[k].Written, boxes[k].Crossed, boxes[k].Fraction = sample.BoxBlank, nil, nil, nil
		}
		empty.Items[i].Boxes = boxes
	}
	readers := e4Readers()
	if _, err := workbook.New("dev", "", 1, []*sample.WorkbookPage{&empty}, readers, 3, workbook.Reads{Captions: true}); err == nil || !strings.Contains(err.Error(), "asks nothing") {
		t.Errorf("a run of a page with no filled box: %v", err)
	}
	pages := append([]*sample.WorkbookPage{&empty}, workbook.Pages(wb, "dev")[1:]...)
	r, err := workbook.New("dev", "", 1, pages, readers, 1, workbook.Reads{Heights: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Items) != 5 || len(r.Steps) != 4*2*3 {
		t.Errorf("%d pages and %d calls, want 5 and %d", len(r.Items), len(r.Steps), 4*2*3)
	}
	var log bytes.Buffer
	if _, err := r.Do(context.Background(), runner.Stub, 10, &log, time.Now); err != nil {
		t.Fatal(err)
	}
	l, err := workbook.ReadLog(bytes.NewReader(log.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	for _, rd := range l.Run.Readers {
		if rd.Once {
			t.Errorf("a run of heights alone names %s, which reads none of them", rd.Name)
		}
	}
	if _, err := workbook.ReadLog(strings.NewReader(strings.Replace(log.String(), `"kind":"page"`, `"kind":"sheet"`, 1))); err == nil {
		t.Error("a record of a sheet reads in a log of the workbook")
	}
	if _, err := workbook.New("dev", "", 1, workbook.Pages(wb, "dev"), readers[:1], 1, workbook.Reads{Heights: true}); err == nil {
		t.Error("heights by Opus alone, who reads none of them, are planned")
	}
}

// e4Readers are the readers of the three configurations of E4, which these tests were written for.
func e4Readers() []workbook.Reader {
	return runner.E4(workbook.Readers(), func(rd workbook.Reader) runner.Config { return rd.Config })
}
