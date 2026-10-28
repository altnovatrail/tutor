package sample_test

import (
	"os"
	"strings"
	"testing"

	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/value"
)

func ptr[T any](v T) *T { return &v }

// testWorkbook is a page of two items: a number with a typical mistake, and a fraction asked in
// lowest terms, crossed out and written again.
func testWorkbook() *sample.Workbook {
	v := sample.Version
	return &sample.Workbook{Format: sample.WorkbookFormat, Version: &v, Pages: []sample.WorkbookPage{{
		Number: 1, Part: sample.PartDev, Items: []sample.WorkbookItem{
			{Text: "7 × [ ] = 56", Boxes: []sample.WorkbookBox{{
				Check:    sample.BoxCheck{Kind: sample.KindNumber, Key: "8", TrapValues: []sample.TrapValue{{Value: "49", Trap: "wrong-operation"}}},
				Scenario: sample.BoxError, Written: ptr("49"),
			}}},
			{Text: "Сократи дробь: 6/8 = [ ]", Boxes: []sample.WorkbookBox{{
				Check: sample.BoxCheck{Kind: sample.KindFraction, Key: "3/4", Form: ptr(value.Reduced),
					TrapValues: []sample.TrapValue{{Value: "6/8", Trap: "not-in-lowest-terms"}}},
				Scenario: sample.BoxRewritten, Written: ptr("3/4"), Crossed: ptr("6/8"), Fraction: ptr(sample.Stacked),
			}}},
		},
	}}}
}

func TestReadWorkbook(t *testing.T) {
	data, err := sample.Marshal(testWorkbook())
	if err != nil {
		t.Fatal(err)
	}
	wb, err := sample.ReadWorkbook(strings.NewReader(string(data)))
	if err != nil || len(wb.Pages) != 1 || *wb.Pages[0].Items[1].Boxes[0].Crossed != "6/8" {
		t.Fatalf("ReadWorkbook = %+v, %v", wb, err)
	}
	broken := strings.Replace(string(data), `"crossed": null,`, "", 1)
	_, err = sample.ReadWorkbook(strings.NewReader(broken))
	has(t, err, "pages[0].items[0].boxes[0].crossed", "is missing")
}

// The workbook of the measurement, eval/workbook.json, reads and passes the check: 20 pages of ten
// items and 11 boxes, the first five of the development.
func TestTheWorkbook(t *testing.T) {
	f, err := os.Open("../../eval/workbook.json")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	wb, err := sample.ReadWorkbook(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(wb.Pages) != 20 {
		t.Fatalf("%d pages", len(wb.Pages))
	}
	for _, pg := range wb.Pages {
		boxes := 0
		for _, it := range pg.Items {
			boxes += len(it.Boxes)
		}
		if want := map[bool]string{true: sample.PartDev, false: sample.PartTest}[pg.Number <= 5]; pg.Part != want || len(pg.Items) != 10 || boxes != 11 {
			t.Errorf("page %d of %s has %d items and %d boxes", pg.Number, pg.Part, len(pg.Items), boxes)
		}
	}
}

func TestWorkbookCheck(t *testing.T) {
	box := func(wb *sample.Workbook, item int) *sample.WorkbookBox { return &wb.Pages[0].Items[item].Boxes[0] }
	tests := []struct {
		name   string
		change func(wb *sample.Workbook)
		path   string
		want   string
	}{
		{"another format", func(wb *sample.Workbook) { wb.Format = "tutor-plan" }, "format", `not "tutor-workbook"`},
		{"no pages", func(wb *sample.Workbook) { wb.Pages = nil }, "pages", "at least one page"},
		{"a page out of its place", func(wb *sample.Workbook) { wb.Pages[0].Number = 2 }, "pages[0].number", "is number 1"},
		{"no part", func(wb *sample.Workbook) { wb.Pages[0].Part = "train" }, "pages[0].part", `not "dev" or "test"`},
		{"too many items", func(wb *sample.Workbook) {
			for len(wb.Pages[0].Items) <= sample.MaxItems {
				wb.Pages[0].Items = append(wb.Pages[0].Items, wb.Pages[0].Items[0])
			}
		}, "pages[0].items", "1 to 10 items"},
		{"boxes not in the text", func(wb *sample.Workbook) { wb.Pages[0].Items[0].Text = "7 × 8 = 56" }, "pages[0].items[0]", "its text shows 0"},
		{"another kind", func(wb *sample.Workbook) { box(wb, 0).Check.Kind = "choice" }, "pages[0].items[0].boxes[0].check.kind", `"choice"`},
		{"a number with a form", func(wb *sample.Workbook) { box(wb, 0).Check.Form = ptr(value.AnyForm) }, "pages[0].items[0].boxes[0].check.form", "a number has no form"},
		{"a fraction without a form", func(wb *sample.Workbook) { box(wb, 1).Check.Form = nil }, "pages[0].items[1].boxes[0].check.form", `not "any" or "reduced"`},
		{"a key not in lowest terms", func(wb *sample.Workbook) { box(wb, 1).Check.Key = "6/8" }, "pages[0].items[1].boxes[0].check.key", "no key of a fraction"},
		{"a key out of notation", func(wb *sample.Workbook) { box(wb, 0).Check.Key = "x = 8" }, "pages[0].items[0].boxes[0].check.key", "no key"},
		{"a right trap value", func(wb *sample.Workbook) { box(wb, 0).Check.TrapValues[0].Value = "8" }, "pages[0].items[0].boxes[0].check.trap_values[0].value", "is correct"},
		{"two equal trap values", func(wb *sample.Workbook) {
			box(wb, 1).Check.TrapValues = append(box(wb, 1).Check.TrapValues, sample.TrapValue{Value: "9/12", Trap: "other"})
		}, "pages[0].items[1].boxes[0].check.trap_values[1].value", "equal to the trap value"},
		{"a trap without a name", func(wb *sample.Workbook) { box(wb, 0).Check.TrapValues[0].Trap = "Wrong op" }, "pages[0].items[0].boxes[0].check.trap_values[0].trap", "no trap's ID"},
		{"an error that is no trap value", func(wb *sample.Workbook) { box(wb, 0).Written = ptr("9") }, "pages[0].items[0].boxes[0].written", "no trap value"},
		{"a correct box not right", func(wb *sample.Workbook) { box(wb, 0).Scenario = sample.BoxCorrect }, "pages[0].items[0].boxes[0].written", "holds the key's value"},
		{"a blank box with a value", func(wb *sample.Workbook) { box(wb, 0).Scenario = sample.BoxBlank }, "pages[0].items[0].boxes[0]", "holds no value"},
		{"no value outside a blank box", func(wb *sample.Workbook) { box(wb, 0).Written = nil }, "pages[0].items[0].boxes[0].written", "only a box of the scenario blank"},
		{"a value written otherwise", func(wb *sample.Workbook) { box(wb, 0).Written = ptr(" 49") }, "pages[0].items[0].boxes[0].written", "not written as the pages write"},
		{"crossed out without rewriting", func(wb *sample.Workbook) { box(wb, 0).Crossed = ptr("48") }, "pages[0].items[0].boxes[0].crossed", "rewritten alone"},
		{"rewritten without crossing", func(wb *sample.Workbook) { box(wb, 1).Crossed = nil }, "pages[0].items[1].boxes[0].crossed", "rewritten alone"},
		{"a right value crossed out", func(wb *sample.Workbook) { box(wb, 1).Crossed = ptr("3/4") }, "pages[0].items[1].boxes[0].crossed", "a wrong value"},
		{"a number written as a fraction", func(wb *sample.Workbook) { box(wb, 0).Fraction = ptr(sample.Inline) }, "pages[0].items[0].boxes[0].fraction", "a number is no fraction"},
		{"a fraction written somehow", func(wb *sample.Workbook) { box(wb, 1).Fraction = nil }, "pages[0].items[1].boxes[0].fraction", "stacked or inline"},
		{"a near key that is right", func(wb *sample.Workbook) {
			box(wb, 0).Scenario, box(wb, 0).Written = sample.BoxNearKey, ptr("8")
		}, "pages[0].items[0].boxes[0].written", "is correct"},
		{"a near key off by no pair", func(wb *sample.Workbook) {
			box(wb, 0).Scenario, box(wb, 0).Written = sample.BoxNearKey, ptr("2")
		}, "pages[0].items[0].boxes[0].written", "no digit for the scenario near-key"},
		{"no digit to mirror", func(wb *sample.Workbook) {
			box(wb, 0).Scenario, box(wb, 0).Written = sample.BoxMirrored, ptr("8")
		}, "pages[0].items[0].boxes[0].written", "none of the digits 2579"},
		{"another scenario", func(wb *sample.Workbook) { box(wb, 0).Scenario = "smudged" }, "pages[0].items[0].boxes[0].scenario", `"smudged"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wb := testWorkbook()
			tt.change(wb)
			has(t, wb.Check(), tt.path, tt.want)
		})
	}
	if err := testWorkbook().Check(); err != nil {
		t.Errorf("a good workbook: %v", err)
	}
}

// The digit a box leans: on a near key, the one digit off the key, toward the key's; on similar
// digits, the last digit of a pair; none on any other box. The digit a box mirrors is the first of
// 2, 5, 7 and 9.
func TestLeaningAndMirrored(t *testing.T) {
	near := sample.WorkbookBox{Check: sample.BoxCheck{Key: "3/4"}, Scenario: sample.BoxNearKey, Written: ptr("3/9")}
	if at, toward, ok := near.Leaning(); !ok || at != 2 || toward != '4' {
		t.Errorf("the near key 3/9 of 3/4 leans %d %c %v", at, toward, ok)
	}
	near.Written = ptr("8/9")
	if _, _, ok := near.Leaning(); ok {
		t.Error("a value two digits off the key leans")
	}
	similar := sample.WorkbookBox{Check: sample.BoxCheck{Key: "152"}, Scenario: sample.BoxSimilarDigits, Written: ptr("152")}
	if at, toward, ok := similar.Leaning(); !ok || at != 1 || toward != '6' {
		t.Errorf("152 leans %d %c %v", at, toward, ok)
	}
	similar.Scenario = sample.BoxCorrect
	if _, _, ok := similar.Leaning(); ok {
		t.Error("a correct box leans")
	}
	if at, ok := sample.Mirrored("3/5"); !ok || at != 2 {
		t.Errorf("3/5 mirrors %d %v", at, ok)
	}
	if at, ok := sample.Mirrored("−7"); !ok || at != 1 {
		t.Errorf("−7 mirrors %d %v: the place is among the characters, not the bytes", at, ok)
	}
	if _, ok := sample.Mirrored("3/4"); ok {
		t.Error("3/4 mirrors a digit")
	}
	if r, ok := sample.PairOf('8'); !ok || r != '3' {
		t.Errorf("the pair of 8 is %c %v", r, ok)
	}
	if v := (sample.WorkbookBox{Check: sample.BoxCheck{Kind: sample.KindNumber, Key: "8"}}).Verdict(); v.Verdict != value.Unreadable || v.Reason != value.Blank {
		t.Errorf("a blank box is %+v", v)
	}
}
