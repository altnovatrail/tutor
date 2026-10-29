package workbook

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/runner"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/synth"
	"github.com/altnovatrail/tutor/internal/value"
	"github.com/altnovatrail/tutor/provider"
)

// CallKind is the kind of call of the reading of the workbook, as its log tells it
// (runner.CallKind).
const CallKind = "workbook"

// Reader is a configuration of the reader: a configuration of the runner, the output tokens a call
// is expected to take, for the estimate, and whether it reads once whatever the repeats, as Opus
// does in the reduced matrix, without the other heights of digits (R08; R28).
type Reader = runner.Reader

// Readers are the readers, the configurations of the runner by their names. The cells of a page of
// ten filled boxes come to about 400 tokens; Haiku with thinking writes about 500 more, and Opus at
// high effort about 800. Opus reads each condition once, and not the other heights
// (runner.ReadersOf). The four the extended measurement adds (R50) are estimated so: Opus at medium as at high, Sonnet with thinking between Haiku and Opus, without thinking as Haiku without it, and Haiku at medium above Haiku at low.
func Readers() []Reader {
	return runner.ReadersOf("workbook", map[string]int{
		"opus-high": 1200, "haiku-low": 900, "haiku-low-no-thinking": 400,
		"opus-medium": 1200, "sonnet-low": 1000, "sonnet-low-no-thinking": 400, "haiku-medium": 1200,
	}, "opus-high", "opus-medium")
}

// The input tokens a call is expected to take, for the estimate: the instructions, the schema and
// the text, about 700 in all, and for each crop its own, ⌈w / 28⌉ × ⌈h / 28⌉ (render.Tokens), and
// about 15 of its label, 20 more with the line of its item. inputBudget is a generous count of them,
// for the worst case: a page of ten crops of 200 × 200 comes to about 1 700.
const (
	inputsOfText    = 700
	inputsOfLabel   = 15
	inputsOfCaption = 20
	inputBudget     = 8000
)

// Reads are what a run reads of its pages: the pages at 32 px, bare and captioned, captions; the
// bare pages at 32 px alone, bare, as the platform reads them, with no line of the item (the
// 2026-10-28 amendment to R53 spec; R50); and the bare pages at the other heights of digits,
// heights, read once, by the readers that do not read once (R08; R28).
type Reads struct {
	Bare, Captions, Heights bool
}

// ParseReads reads what a run reads from a list of bare, captions and heights, joined by commas.
func ParseReads(list string) (Reads, error) {
	var r Reads
	for _, name := range strings.Split(list, ",") {
		switch name {
		case "bare":
			r.Bare = true
		case "captions":
			r.Captions = true
		case "heights":
			r.Heights = true
		default:
			return r, fmt.Errorf("workbook: %q is nothing a run reads: give bare, captions or heights, joined by commas", name)
		}
	}
	return r, nil
}

// Names are what a run reads, by name, in their order.
func (r Reads) Names() []string {
	var out []string
	if r.Bare {
		out = append(out, "bare")
	}
	if r.Captions {
		out = append(out, "captions")
	}
	if r.Heights {
		out = append(out, "heights")
	}
	return out
}

// Call is one call of a run: a page by its number, a condition, a reader by its name, the repeat,
// from 1, and the height of digits of its crops.
type Call struct {
	Page      int       `json:"page"`
	Condition Condition `json:"condition"`
	Reader    string    `json:"reader"`
	Repeat    int       `json:"repeat"`
	Digit     int       `json:"digit"`
}

// step is a call of the reading of a page on the runner.
type step struct {
	Call
	page  *sample.WorkbookPage
	cells []Cell
}

// Request is the reading's request of the page's crops in the call's condition.
func (s *step) Request() (provider.Request, error) {
	return Request(s.page, s.cells, s.Condition), nil
}

// Right is the answer of the reader who is always right, by what is written in the boxes.
func (s *step) Right() (string, error) {
	data, err := json.Marshal(struct {
		Cells []Reading `json:"cells"`
	}{Right(s.cells)})
	return string(data), err
}

// Record is the call's record: the answer as the model gave it, the verdict of every box of the
// page and of every item. An answer off the schema is a failure of the reading, and every filled box
// is then unchecked.
func (s *step) Record(o runner.Outcome) any {
	rec := CallRecord{Kind: "call", Call: s.Call, NotAsked: o.NotAsked, Answer: o.Answer, Error: o.Error, Usage: o.Usage}
	var read []Reading
	if !o.NotAsked && o.Error == "" {
		var err error
		if read, err = Read(o.Answer); err != nil {
			rec.Error = err.Error()
		}
	}
	rec.Boxes = Verdicts(s.page, read)
	rec.Items = Items(s.page, rec.Boxes)
	return rec
}

// Pages are the pages of a part of the workbook: dev or test, or every page for all.
func Pages(wb *sample.Workbook, part string) []*sample.WorkbookPage {
	var out []*sample.WorkbookPage
	for i := range wb.Pages {
		if part == "all" || wb.Pages[i].Part == part {
			out = append(out, &wb.Pages[i])
		}
	}
	return out
}

// New plans a run of the reading over the pages of a part of the workbook, drawn from the seed, by
// the readers, repeated: it draws every page once, cuts its crops and puts the calls in order,
// repeat by repeat, so that a run cut short has the same repeats of every page. A page with no
// filled box goes to no model: its boxes are blank, and their verdicts need no call. Of each page,
// by each reader, as reads says:
//   - bare: the crops at 32 px, bare. A reader that reads once asks only in the first repeat;
//   - captions: the crops at 32 px, bare and captioned, so too;
//   - heights: the bare crops at 16, 24 and 48 px, in the first repeat, by the readers that do not
//     read once.
func New(part, file string, seed uint64, pages []*sample.WorkbookPage, readers []Reader, repeats int, reads Reads) (*runner.Run, error) {
	if repeats < 1 {
		return nil, fmt.Errorf("workbook: %d repeats", repeats)
	}
	if len(reads.Names()) == 0 {
		return nil, fmt.Errorf("workbook: the run reads nothing")
	}
	if len(pages) == 0 {
		return nil, fmt.Errorf("workbook: no page of the part %s", part)
	}
	heights := reads.Heights && slices.ContainsFunc(readers, func(rd Reader) bool { return !rd.Once })
	// What the run reads, as its record tells it: the other heights only where a reader reads them.
	read := reads
	read.Heights = heights
	r := &runner.Run{}
	type planned struct {
		page  *sample.WorkbookPage
		cells map[int][]Cell
	}
	var all []planned
	for _, pg := range pages {
		drawn, err := synth.PageOf(pg, seed)
		if err != nil {
			return nil, err
		}
		p := planned{page: pg, cells: map[int][]Cell{}}
		for _, digit := range render.Heights() {
			if digit != render.Digit && !heights {
				continue
			}
			if p.cells[digit], err = CellsOf(drawn, pg, digit); err != nil {
				return nil, fmt.Errorf("workbook: page %d: %w", pg.Number, err)
			}
		}
		all = append(all, p)
		r.Items = append(r.Items, pageRecord(pg, p.cells[render.Digit]))
	}
	// The run's record names the readers it asks: reading only the other heights, not the readers
	// that read once.
	var records []ReaderRecord
	for _, rd := range readers {
		if reads.Bare || reads.Captions || !rd.Once {
			records = append(records, rd.Record())
		}
	}
	r.Record = func(h runner.Head) any {
		return RunRecord{Kind: "run", CallKind: CallKind, Time: h.Time, Part: part, File: file, Seed: seed, Readers: records, Repeats: repeats, Reads: read.Names(),
			MaxUSD: h.MaxUSD, Estimate: h.Estimate, Prompt: Fingerprint(), PromptVersion: Version}
	}
	add := func(rd Reader, rep int, p planned, c Condition, digit int) {
		st := &step{Call: Call{Page: p.page.Number, Condition: c, Reader: rd.Name, Repeat: rep, Digit: digit}, page: p.page, cells: p.cells[digit]}
		in := int64(inputsOfText)
		for _, cell := range st.cells {
			in += int64(render.Tokens(cell.Crop.Width, cell.Crop.Height) + inputsOfLabel)
			if c == Captioned {
				in += inputsOfCaption
			}
		}
		r.Steps = append(r.Steps, runner.Step{Config: rd.Config, In: in, Out: int64(rd.Output), WorstIn: inputBudget, Call: st})
	}
	for rep := 1; rep <= repeats; rep++ {
		for _, p := range all {
			if len(p.cells[render.Digit]) == 0 {
				continue
			}
			for _, rd := range readers {
				if rd.Once && rep > 1 {
					continue
				}
				if reads.Bare || reads.Captions {
					add(rd, rep, p, Bare, render.Digit)
				}
				if reads.Captions {
					add(rd, rep, p, Captioned, render.Digit)
				}
				if rep == 1 && !rd.Once && heights {
					for _, digit := range render.Heights() {
						if digit != render.Digit {
							add(rd, rep, p, Bare, digit)
						}
					}
				}
			}
		}
	}
	if len(r.Steps) == 0 {
		return nil, fmt.Errorf("workbook: the run asks nothing: no page of it has a filled box, or %s are read by no reader that reads them", strings.Join(reads.Names(), " and "))
	}
	return r, nil
}

// pageRecord is the record that tells a page of the run: its number and part, and each of its
// boxes as code wrote it, with the verdict of its value and the size of its crop at 32 px.
func pageRecord(pg *sample.WorkbookPage, cells []Cell) PageRecord {
	rec := PageRecord{Kind: "page", Page: pg.Number, Part: pg.Part}
	crops := map[[2]int]Cell{}
	for _, c := range cells {
		crops[[2]int{c.Item, c.Slot}] = c
	}
	for i, it := range pg.Items {
		for k, b := range it.Boxes {
			v := b.Verdict()
			br := BoxRecord{Item: i + 1, Slot: k + 1, Kind: b.Check.Kind, Key: b.Check.Key, Scenario: b.Scenario, Written: b.Written, Crossed: b.Crossed, Fraction: b.Fraction,
				Verdict: v.Verdict, Reason: v.Reason, Trap: v.Trap}
			if b.Check.Form != nil {
				br.Form = *b.Check.Form
			}
			if c, ok := crops[[2]int{i + 1, k + 1}]; ok {
				br.Width, br.Height = c.Crop.Width, c.Crop.Height
			}
			rec.Boxes = append(rec.Boxes, br)
		}
	}
	return rec
}

// The records of a run's log, a JSON object a line, each with its kind: the run first, then each
// page, each call, and the end, which is the runner's (runner.End). The run's record tells the kind
// of call, workbook, so that tutor-eval report tells its log from the others.
type (
	// RunRecord tells the run: its part, the file of the workbook and the seed its pages are drawn
	// from, its readers and repeats, what it reads, its cap and estimate, and the fingerprint and
	// the version of its prompt.
	RunRecord struct {
		Kind          string          `json:"kind"`
		CallKind      string          `json:"call_kind"`
		Time          time.Time       `json:"time"`
		Part          string          `json:"part"`
		File          string          `json:"file"`
		Seed          uint64          `json:"seed"`
		Readers       []ReaderRecord  `json:"readers"`
		Repeats       int             `json:"repeats"`
		Reads         []string        `json:"reads"`
		MaxUSD        float64         `json:"max_usd"`
		Estimate      runner.Estimate `json:"estimate"`
		Prompt        string          `json:"prompt"`
		PromptVersion int             `json:"prompt_version"`
	}
	// ReaderRecord is a reader as the run's record tells it: its model and prices, and whether it
	// reads once.
	ReaderRecord = runner.ReaderRecord
	// PageRecord tells a page: its number and part, and its boxes.
	PageRecord struct {
		Kind  string      `json:"kind"`
		Page  int         `json:"page"`
		Part  string      `json:"part"`
		Boxes []BoxRecord `json:"boxes"`
	}
	// BoxRecord tells a box as code wrote it: its place, its check — kind, form and key — its
	// scenario, what is written in it and crossed out, how a fraction is written, the verdict the
	// box's check gives the value written, with its reason or trap, and the size of its crop at
	// 32 px, none for a blank box. The verdict a right reading comes to by the prompt of the run is
	// Want of it. They are the texts of the file of the workbook, not the handwriting.
	BoxRecord struct {
		Item     int                     `json:"item"`
		Slot     int                     `json:"slot"`
		Kind     sample.CheckKind        `json:"check"`
		Form     value.Form              `json:"form,omitempty"`
		Key      string                  `json:"key"`
		Scenario sample.BoxScenario      `json:"scenario"`
		Written  *string                 `json:"written"`
		Crossed  *string                 `json:"crossed,omitempty"`
		Fraction *sample.FractionWriting `json:"fraction,omitempty"`
		Verdict  value.Verdict           `json:"verdict"`
		Reason   value.Reason            `json:"reason,omitempty"`
		Trap     string                  `json:"trap,omitempty"`
		Width    int                     `json:"width,omitempty"`
		Height   int                     `json:"height,omitempty"`
	}
	// CallRecord tells a call: the answer as the model gave it, the verdict of each box of the page
	// and of each item, the failure if it failed, and what it took. A call whose request could not
	// be built was not asked, and says so.
	CallRecord struct {
		Kind string `json:"kind"`
		Call
		NotAsked bool            `json:"not_asked,omitempty"`
		Answer   string          `json:"answer,omitempty"`
		Boxes    []Box           `json:"boxes"`
		Items    []value.Verdict `json:"items"`
		Error    string          `json:"error,omitempty"`
		runner.Usage
	}
)

// Log is a run's log read back: the run, its pages, in the field Sheets of the runner's records,
// its calls in the order they were made, and how the run ended, or nil when the log stops before the
// end.
type Log = runner.Records[RunRecord, PageRecord, CallRecord]

// ReadLog reads the log of a run of the reading of the workbook back, as runner.ReadLog reads a log.
// A log of another kind of call is refused (runner.ReadRecords).
func ReadLog(r io.Reader) (*Log, error) {
	return runner.ReadItems[RunRecord, PageRecord, CallRecord](r, CallKind, "workbook", "page")
}
