package openview_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"slices"
	"strings"
	"testing"

	// The module of spec's contracts goes into the build information only when the test links it.
	_ "github.com/altnovatrail/spec/contracts"
	"github.com/altnovatrail/spec/types/formatv0"
	"github.com/altnovatrail/spec/types/formatv0/open"

	"github.com/altnovatrail/tutor/internal/leak"
	"github.com/altnovatrail/tutor/internal/openview"
	"github.com/altnovatrail/tutor/internal/reading"
	"github.com/altnovatrail/tutor/internal/sample"
)

// The root of the repository, the files the open views are built from, as just open-tasks builds
// them, and the file in the repository that tutor's release carries.
const (
	root      = "../.."
	committed = "eval/open-tasks.json"
)

var inputs = []string{"eval/bank.json", "eval/free-bank.json", "eval/workbook.json", "eval/essays.json"}

func readRoot(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func build(t *testing.T) *openview.File {
	t.Helper()
	var in []openview.Input
	for _, name := range inputs {
		in = append(in, openview.Input{Path: name, Data: readRoot(t, name)})
	}
	f, err := openview.Build(in)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// The file in the repository is what the files of tasks give now, byte for byte: a bank that changed
// without it fails here.
func TestFile(t *testing.T) {
	data, err := openview.Marshal(build(t))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, readRoot(t, committed)) {
		t.Fatalf("%s is not what the files of tasks give: run just open-tasks", committed)
	}
}

// The file holds the 56 tasks of the measurements, each bank by its path, the digest of its bytes
// and its language.
func TestRead(t *testing.T) {
	f, err := openview.Read(readRoot(t, committed))
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		bank, language string
		tasks          int
	}{{"eval/bank.json", "ru", 10}, {"eval/free-bank.json", "ru", 6}, {"eval/workbook.json", "ru", 20}, {"eval/essays.json", "en", 20}}
	if len(f.Banks) != len(want) {
		t.Fatalf("%d banks, want %d", len(f.Banks), len(want))
	}
	for i, w := range want {
		b := f.Banks[i]
		if b.Bank != w.bank || string(b.Language) != w.language || len(b.Tasks) != w.tasks {
			t.Errorf("bank %d: %s, %s, %d tasks; want %s, %s, %d", i, b.Bank, b.Language, len(b.Tasks), w.bank, w.language, w.tasks)
		}
	}
	if got := string(f.Banks[0].Digest); !strings.HasPrefix(got, "sha256:") || len(got) != 71 {
		t.Errorf("digest %q", got)
	}
}

// Read takes the file only as Marshal writes it: no closed field of a task, by any name the format
// gives one, nor a field in another letter case or twice, nor anything after the file.
func TestReadRefuses(t *testing.T) {
	good := string(readRoot(t, committed))
	const first = `"id": "equation-1",`
	if !strings.Contains(good, first) {
		t.Fatalf("no %s in the file", first)
	}
	cases := map[string]string{
		"another letter case": strings.Replace(good, `"statement"`, `"Statement"`, 1),
		"twice":               strings.Replace(good, `"version": 0,`, `"version": 0, "version": 0,`, 1),
		"data after":          good + "{}\n",
		"another layout":      strings.Replace(good, "\n  ", "\n ", 1),
		"no field":            strings.Replace(good, `"version": 0,`, "", 1),
	}
	for _, closed := range []string{"skills", "band", "difficulty", "age_categories", "techniques", "traps", "solution", "leak_phrases", "template", "key", "check"} {
		cases["closed field "+closed] = strings.Replace(good, first, first+` "`+closed+`": null,`, 1)
	}
	cases["check of an answer slot"] = strings.Replace(good, `"binding": "anywhere"`, `"binding": "anywhere", "check": {"kind": "number", "key": "2"}`, 1)
	cases["check of a box"] = strings.Replace(good, `"binding": "box"`, `"binding": "box", "check": {"kind": "number", "key": "5"}`, 1)
	for name, data := range cases {
		if data == good {
			t.Fatalf("%s: the case changes nothing", name)
		}
		if _, err := openview.Read([]byte(data)); err == nil {
			t.Errorf("%s: read", name)
		}
	}
}

// Check names each rule of the format that the types do not carry, by its path.
func TestCheck(t *testing.T) {
	text := func(s string) *formatv0.Text { t := formatv0.Text(s); return &t }
	cases := []struct {
		name  string
		spoil func(f *openview.File)
		path  string
	}{
		{"format", func(f *openview.File) { f.Format = "tutor-bank" }, "format"},
		{"version", func(f *openview.File) { f.Version = 1 }, "version"},
		{"module format", func(f *openview.File) { f.ModuleFormatVersion = "0.1" }, "module_format_version"},
		{"no banks", func(f *openview.File) { f.Banks = nil }, "banks"},
		{"absolute bank", func(f *openview.File) { f.Banks[0].Bank = "/eval/bank.json" }, "banks[0].bank"},
		{"bank outside", func(f *openview.File) { f.Banks[0].Bank = "../bank.json" }, "banks[0].bank"},
		{"bank not clean", func(f *openview.File) { f.Banks[0].Bank = "eval/../eval/bank.json" }, "banks[0].bank"},
		{"bank twice", func(f *openview.File) { f.Banks[1].Bank = f.Banks[0].Bank }, "banks[1].bank"},
		{"digest", func(f *openview.File) { f.Banks[0].Digest = "sha256:ABC" }, "banks[0].digest"},
		{"language", func(f *openview.File) { f.Banks[0].Language = "russian" }, "banks[0].language"},
		{"no tasks", func(f *openview.File) { f.Banks[0].Tasks = nil }, "banks[0].tasks"},
		{"ID twice", func(f *openview.File) { f.Banks[1].Tasks[0].ID = f.Banks[0].Tasks[0].ID }, "banks[1].tasks[0].id"},
		{"ID case", func(f *openview.File) { f.Banks[0].Tasks[0].ID = "Equation-1" }, "banks[0].tasks[0].id"},
		{"ID reserved", func(f *openview.File) { f.Banks[0].Tasks[0].ID = "con" }, "banks[0].tasks[0].id"},
		{"no statement", func(f *openview.File) { f.Banks[0].Tasks[0].Statement = nil }, "banks[0].tasks[0].statement"},
		{"empty paragraph", func(f *openview.File) { f.Banks[0].Tasks[0].Statement[0].Text = text("") }, "banks[0].tasks[0].statement[0]"},
		{"picture", func(f *openview.File) {
			p := formatv0.PicturePath("posts.png")
			f.Banks[0].Tasks[0].Statement[0] = open.StatementPart{Picture: &p}
		}, "banks[0].tasks[0].statement[0]"},
		{"givens", func(f *openview.File) { f.Banks[0].Tasks[0].Givens = []formatv0.Text{"7"} }, "banks[0].tasks[0].givens"},
		{"unknown", func(f *openview.File) { f.Banks[0].Tasks[0].Unknown = text("x") }, "banks[0].tasks[0].unknown"},
		{"no items", func(f *openview.File) { f.Banks[0].Tasks[0].Items = nil }, "banks[0].tasks[0].items"},
		{"canvas kind", func(f *openview.File) { f.Banks[0].Tasks[0].Canvas.Kind = "board" }, "banks[0].tasks[0].canvas.kind"},
		{"grid", func(f *openview.File) {
			g := formatv0.CanvasGrid("none")
			f.Banks[0].Tasks[0].Canvas.Grid = &g
		}, "banks[0].tasks[0].canvas.grid"},
		{"grid step", func(f *openview.File) {
			s := int64(50)
			f.Banks[0].Tasks[0].Canvas.GridStep = &s
		}, "banks[0].tasks[0].canvas.grid_step"},
		{"line on a sheet", func(f *openview.File) {
			f.Banks[0].Tasks[0].Items[0].Line = f.Banks[2].Tasks[0].Items[0].Line
		}, "banks[0].tasks[0].items[0].line"},
		{"item text", func(f *openview.File) { f.Banks[0].Tasks[0].Items[0].Text = text("a)") }, "banks[0].tasks[0].items[0].text"},
		{"no slots", func(f *openview.File) { f.Banks[0].Tasks[0].Items[0].AnswerSlots = nil }, "banks[0].tasks[0].items[0].answer_slots"},
		{"anywhere not alone", func(f *openview.File) {
			it := &f.Banks[0].Tasks[0].Items[0]
			it.AnswerSlots = append(it.AnswerSlots, open.AnswerSlot{Binding: formatv0.AnswerSlotBindingAnywhere})
		}, "banks[0].tasks[0].items[0].answer_slots[1]"},
		{"anywhere in another item", func(f *openview.File) {
			t := &f.Banks[0].Tasks[0]
			t.Items = append(t.Items, t.Items[0])
		}, "banks[0].tasks[0].items[1].answer_slots[0]"},
		{"box on a sheet", func(f *openview.File) { f.Banks[0].Tasks[0].Items[0].AnswerSlots[0].Binding = "box" }, "banks[0].tasks[0].items[0].answer_slots[0].binding"},
		{"zone", func(f *openview.File) {
			f.Banks[0].Tasks[0].Items[0].AnswerSlots[0].Binding = formatv0.AnswerSlotBindingZone
		}, "banks[0].tasks[0].items[0].answer_slots[0].binding"},
		{"zone not bound so", func(f *openview.File) {
			f.Banks[0].Tasks[0].Items[0].AnswerSlots[0].Zone = &open.SheetRectangle{Width: 100, Height: 100}
		}, "banks[0].tasks[0].items[0].answer_slots[0].zone"},
		{"choice", func(f *openview.File) {
			s := &f.Banks[0].Tasks[0].Items[0].AnswerSlots[0]
			s.Binding, s.Options = formatv0.AnswerSlotBindingChoice, []formatv0.Text{"2", "3"}
		}, "banks[0].tasks[0].items[0].answer_slots[0].binding"},
		{"options not bound so", func(f *openview.File) {
			f.Banks[0].Tasks[0].Items[0].AnswerSlots[0].Options = []formatv0.Text{"2", "3"}
		}, "banks[0].tasks[0].items[0].answer_slots[0].options"},
		{"grid of a workbook", func(f *openview.File) {
			g := formatv0.CanvasGridSquares
			f.Banks[2].Tasks[0].Canvas.Grid = &g
		}, "banks[2].tasks[0].canvas.grid"},
		{"slots on a workbook", func(f *openview.File) {
			f.Banks[2].Tasks[0].Items[0].AnswerSlots = f.Banks[0].Tasks[0].Items[0].AnswerSlots
		}, "banks[2].tasks[0].items[0]"},
		{"text and box", func(f *openview.File) {
			box := formatv0.LinePartBindingBox
			f.Banks[2].Tasks[0].Items[0].Line[0].Binding = &box
		}, "banks[2].tasks[0].items[0].line[0]"},
		{"empty piece", func(f *openview.File) { f.Banks[2].Tasks[0].Items[0].Line[0].Text = text("") }, "banks[2].tasks[0].items[0].line[0].text"},
		{"not a box", func(f *openview.File) {
			zone := formatv0.LinePartBinding("zone")
			f.Banks[2].Tasks[0].Items[0].Line[1].Binding = &zone
		}, "banks[2].tasks[0].items[0].line[1].binding"},
		{"no box", func(f *openview.File) {
			f.Banks[2].Tasks[0].Items[0].Line = []open.LinePart{{Text: text("6 × 5 = 30")}}
		}, "banks[2].tasks[0].items[0].line"},
	}
	data := readRoot(t, committed)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f, err := openview.Read(data)
			if err != nil {
				t.Fatal(err)
			}
			c.spoil(f)
			err = f.Check()
			var fe *sample.FormatError
			if !errors.As(err, &fe) {
				t.Fatalf("no format error: %v", err)
			}
			if !slices.ContainsFunc(fe.Problems, func(p sample.Problem) bool { return p.Path == c.path }) {
				t.Errorf("no problem at %s: %v", c.path, err)
			}
		})
	}
}

// source is what a task of a file of tasks shows the learner, as the file writes it, and what of
// it is closed: the values that are numbers, which the leak check searches for by their values,
// and the texts.
type source struct {
	shown   []string
	numbers []leak.Key
	phrases []string
}

// sources reads the files of tasks for what each task shows and what is closed, by task ID.
func sources(t *testing.T) map[formatv0.TaskID]source {
	t.Helper()
	out := map[formatv0.TaskID]source{}
	for _, name := range inputs[:2] {
		bank, err := sample.ReadBank(bytes.NewReader(readRoot(t, name)))
		if err != nil {
			t.Fatal(err)
		}
		for _, task := range bank.Tasks {
			s := source{shown: []string{task.Statement}, numbers: []leak.Key{number(reading.KeyOf(task.Key))}}
			s.phrases = append(s.phrases, task.Key)
			s.phrases = append(s.phrases, task.Solution...)
			s.phrases = append(s.phrases, task.Leaks...)
			for _, trap := range task.Traps {
				s.phrases = append(s.phrases, trap.ID, trap.Description)
			}
			for _, fk := range task.FalseKeys {
				s.numbers = append(s.numbers, number(reading.KeyOf(fk.Key)))
				s.phrases = append(s.phrases, fk.Key)
				s.phrases = append(s.phrases, fk.Solution...)
			}
			out[formatv0.TaskID(task.ID)] = s
		}
	}
	wb, err := sample.ReadWorkbook(bytes.NewReader(readRoot(t, inputs[2])))
	if err != nil {
		t.Fatal(err)
	}
	for _, page := range wb.Pages {
		s := source{shown: []string{openview.WorkbookStatement}}
		for _, it := range page.Items {
			s.shown = append(s.shown, it.Text)
			for _, box := range it.Boxes {
				s.numbers = append(s.numbers, number(box.Check.Key))
				for _, tv := range box.Check.TrapValues {
					s.numbers = append(s.numbers, number(tv.Value))
					s.phrases = append(s.phrases, tv.Trap)
				}
				for _, v := range []*string{box.Written, box.Crossed} {
					if v != nil {
						s.numbers = append(s.numbers, number(*v))
					}
				}
			}
		}
		out[openview.Page(&page).ID] = s
	}
	es, err := sample.ReadEssays(bytes.NewReader(readRoot(t, inputs[3])))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range es.Essays {
		s := source{shown: []string{e.Topic}}
		if e.Title != nil {
			s.phrases = append(s.phrases, *e.Title)
		}
		s.phrases = append(s.phrases, sample.Sentences(e.Text)...)
		s.phrases = append(s.phrases, e.Unmet...)
		for _, c := range es.Rubric {
			s.phrases = append(s.phrases, c.ID, c.Text)
		}
		for _, m := range e.Misspelled {
			s.phrases = append(s.phrases, m.Written)
		}
		out[openview.Essay(&e).ID] = s
	}
	return out
}

// number is a closed value that is a number or a fraction, which the leak check reads either way.
func number(v string) leak.Key { return leak.Key{Kind: leak.Number, Value: v} }

// texts is every text of an open view that the learner sees.
func texts(v *open.Task) []string {
	var out []string
	for _, s := range v.Statement {
		if s.Text != nil {
			out = append(out, string(*s.Text))
		}
	}
	for _, it := range v.Items {
		if it.Text != nil {
			out = append(out, string(*it.Text))
		}
		for _, p := range it.Line {
			if p.Text != nil {
				out = append(out, string(*p.Text))
			}
		}
		for _, s := range it.AnswerSlots {
			for _, o := range s.Options {
				out = append(out, string(o))
			}
		}
	}
	for _, g := range v.Givens {
		out = append(out, string(g))
	}
	if v.Unknown != nil {
		out = append(out, string(*v.Unknown))
	}
	return out
}

// found is what of the closed values of a task its open view shows beyond what the file of tasks
// shows: the leak check of the format (section 20), with the shown texts as the statement, so that
// a number the statement shows is not searched for, and the closed texts as leak phrases, but for
// those the statement holds.
func found(v *open.Task, s source) []string {
	shown := strings.Join(s.shown, "\n")
	var phrases []string
	for _, p := range s.phrases {
		if strings.TrimSpace(p) != "" && len(leak.New(leak.Task{Phrases: []string{p}}).Find(shown)) == 0 {
			phrases = append(phrases, p)
		}
	}
	c := leak.New(leak.Task{Statement: s.shown, Items: []leak.Item{{Keys: s.numbers}}, Phrases: phrases})
	return c.Find(strings.Join(texts(v), "\n"))
}

// No open view shows what is closed: no key, solution, trap, leak phrase or false key of a bank's
// task; no key, trap value, written or crossed value of a box; no title, sentence of the text,
// criterion or misspelling of an essay. Its texts are what the file of tasks shows, and a spoiled
// view of each kind is caught.
func TestLeaks(t *testing.T) {
	src := sources(t)
	f := build(t)
	n := 0
	for _, b := range f.Banks {
		for i := range b.Tasks {
			v := &b.Tasks[i]
			s, ok := src[v.ID]
			if !ok {
				t.Fatalf("%s: no source", v.ID)
			}
			n++
			if got := found(v, s); len(got) > 0 {
				t.Errorf("%s shows %q", v.ID, got)
			}
			if got, want := squeeze(texts(v)), squeeze(s.shown); got != want {
				t.Errorf("%s shows %q, and its file %q", v.ID, got, want)
			}
		}
	}
	if n != 56 {
		t.Errorf("%d tasks, want 56", n)
	}
	spoil := func(v open.Task, extra string) *open.Task {
		v.Statement = append(slices.Clone(v.Statement), open.StatementPart{Text: (*formatv0.Text)(&extra)})
		return &v
	}
	sheet, page, essay := f.Banks[0].Tasks[0], f.Banks[2].Tasks[0], f.Banks[3].Tasks[0]
	for name, v := range map[string]*open.Task{
		"key of a sheet":            spoil(sheet, "Ответ: x = 2."),
		"false key of a sheet":      spoil(sheet, "x = 8/3"),
		"trap of a sheet":           spoil(sheet, "перенёс слагаемое в другую часть уравнения, не сменив знак"),
		"key of a box":              spoil(page, "167"),
		"trap value of a box":       spoil(page, "24"),
		"title of an essay":         spoil(essay, "The Bread Robot"),
		"criterion of an essay":     spoil(essay, "It has at least five sentences."),
		"trap of a box":             spoil(page, "wrong-operation"),
		"solution line of a sheet":  spoil(sheet, "3x = 6"),
		"sentence of an essay text": spoil(essay, sample.Sentences(mustEssays(t).Essays[0].Text)[0]),
	} {
		if len(found(v, src[v.ID])) == 0 {
			t.Errorf("%s: not caught", name)
		}
	}
}

func mustEssays(t *testing.T) *sample.Essays {
	t.Helper()
	es, err := sample.ReadEssays(bytes.NewReader(readRoot(t, inputs[3])))
	if err != nil {
		t.Fatal(err)
	}
	return es
}

// squeeze joins texts without their spaces and box marks, so that a line of a workbook, its pieces
// trimmed and its boxes apart, compares with the text it comes from.
func squeeze(texts []string) string {
	all := strings.ReplaceAll(strings.Join(texts, ""), sample.BoxMark, "")
	return strings.Join(strings.Fields(all), "")
}

// A line of the workbook keeps the order of its text and boxes.
func TestLine(t *testing.T) {
	page := sample.WorkbookPage{Number: 7, Items: []sample.WorkbookItem{{Text: "88 : 9 = [ ] (ост. [ ])"}, {Text: "[ ] + 22 = 55"}}}
	v := openview.Page(&page)
	if v.ID != "workbook-07" {
		t.Errorf("ID %s", v.ID)
	}
	got, err := json.Marshal(v.Items)
	if err != nil {
		t.Fatal(err)
	}
	want := `[{"line":[{"text":"88 : 9 ="},{"binding":"box"},{"text":"(ост."},{"binding":"box"},{"text":")"}]},` +
		`{"line":[{"binding":"box"},{"text":"+ 22 = 55"}]}]`
	if string(got) != want {
		t.Errorf("items %s\nwant  %s", got, want)
	}
}

// Build refuses a file that is no file of tasks, and names it.
func TestBuildRefuses(t *testing.T) {
	for name, in := range map[string]openview.Input{
		"plan":     {Path: "eval/sheets.json", Data: readRoot(t, "eval/sheets.json")},
		"not JSON": {Path: "x.json", Data: []byte("[")},
		"broken":   {Path: "eval/bank.json", Data: []byte(`{"format": "tutor-bank", "version": 0, "tasks": []}`)},
	} {
		if _, err := openview.Build([]openview.Input{in}); err == nil || !strings.Contains(err.Error(), in.Path) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// The version of spec's Go module that the build takes is the tag of spec that CLAUDE.md names, and,
// where the spec submodule is checked out, the submodule's tag: the module and the submodule move
// together (Д46 in spec's RUN.md; R64 in docs/decisions.md). From spec's line 0.7 its release tags
// the same commit contracts/<tag> too, so the submodule's tag is the one that starts with v (the
// 2026-11-02 amendment to R83 spec). That release tags the module of spec's contracts with the same
// number, and the build takes it at the same version as spec's module (Д92 in spec's RUN.md; R67 in
// docs/decisions.md). CI checks the repository out without the submodule, and there CLAUDE.md alone
// holds the modules to the tag.
func TestSpecVersion(t *testing.T) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		t.Fatal("no build information")
	}
	versions := map[string]string{}
	for _, dep := range info.Deps {
		if dep.Path == "github.com/altnovatrail/spec" || dep.Path == "github.com/altnovatrail/spec/contracts" {
			if dep.Replace != nil {
				t.Fatalf("%s is replaced by %s %s", dep.Path, dep.Replace.Path, dep.Replace.Version)
			}
			versions[dep.Path] = dep.Version
		}
	}
	version := versions["github.com/altnovatrail/spec"]
	if contracts := versions["github.com/altnovatrail/spec/contracts"]; contracts != version {
		t.Errorf("the build takes spec's module %q and the module of its contracts %q: move them together", version, contracts)
	}
	named := regexp.MustCompile("spec at `(v[0-9]+\\.[0-9]+\\.[0-9]+)`").FindSubmatch(readRoot(t, "CLAUDE.md"))
	if named == nil {
		t.Fatal("CLAUDE.md names no tag of spec")
	}
	if version != string(named[1]) {
		t.Errorf("the build takes spec's module %q, and CLAUDE.md names spec at %s: move them together", version, named[1])
	}
	sub := filepath.Join(root, "spec")
	if _, err := os.Stat(filepath.Join(sub, ".git")); err != nil {
		t.Skipf("no spec submodule: %v", err)
	}
	out, err := exec.Command(gitProgram(), "-C", sub, "describe", "--tags", "--exact-match", "--match", "v*").Output()
	if err != nil {
		t.Fatalf("the spec submodule is at no v* tag: %v", err)
	}
	if tag := strings.TrimSpace(string(out)); version != tag {
		t.Errorf("the build takes spec's module %q, and the submodule is at %s: move them together", version, tag)
	}
}

// gitProgram is git by its absolute path, so that no directory on PATH decides which program runs,
// as spec's tools find it.
func gitProgram() string {
	for _, p := range []string{"/usr/local/bin/git", "/usr/bin/git", "/bin/git"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "/usr/bin/git"
}
