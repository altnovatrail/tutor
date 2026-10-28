package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/altnovatrail/tutor/internal/essay"
	"github.com/altnovatrail/tutor/internal/hint"
	"github.com/altnovatrail/tutor/internal/judge"
	"github.com/altnovatrail/tutor/internal/measure"
	"github.com/altnovatrail/tutor/internal/reading"
	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/runner"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/sheets"
	"github.com/altnovatrail/tutor/internal/workbook"
)

// The caps of the parts, in US dollars: $3 for the development and $6 for the test
// (docs/architecture.md, 3.1, «Смета»; 3.4), and $2 for all the sheets of a plan, without the
// split, which is how the author's three sheets of free writing run (3.4; T77).
var partCaps = map[string]float64{"dev": 3, "test": 6, "all": 2}

// plans are the kinds of call that run asks, by -kind, each with how it plans a run over a part of
// its sets of sheets and says what the run asks (T67): the judge, the reading (T68), the hint (T74),
// the reading of the workbook (T79), which reads the pages of the workbook and no sheets, and the
// assessment by the rubric (T83), which reads the essays.
var plans = map[string]struct {
	plan func(sets []sheetFlags, part string, o options) (*runner.Run, string, error)
	// several is whether the kind runs over several sets: the judge runs on one, since its
	// criterion counts the test part of one set (R28).
	several bool
	// file names the flag of the file a kind reads in place of sets of sheets, drawing it with -seed:
	// workbook, the pages of the workbook (T79), or essays (T83). Its run logs in synthetic.
	file string
	// caps are the kind's own caps of the parts, where it plans other than partCaps: the essays,
	// $8 for the test (R45).
	caps map[string]float64
}{
	"judge":    {plan: planJudge},
	"reading":  {plan: planReading, several: true},
	"hint":     {plan: planHint, several: true},
	"workbook": {plan: planWorkbook, file: "workbook"},
	"essay":    {plan: planEssays, file: "essays", caps: map[string]float64{"dev": 3, "test": 8, "all": 2}},
}

// options are what a run is asked besides its sets and part: how many times each call is repeated,
// what a reading reads (T69), and for the workbook, the file of its pages, the seed they are drawn
// from and what its reading reads (T79); for the essays, their file (T83).
type options struct {
	readers       []string
	repeats       int
	reads         reading.Reads
	workbook      string
	essays        string
	seed          uint64
	workbookReads workbook.Reads
}

// setsFlag is the flag -set, given once for each set of sheets: its directory, bank and plan.
type setsFlag []string

func (s *setsFlag) String() string { return strings.Join(*s, " ") }

func (s *setsFlag) Set(v string) error {
	*s = append(*s, v)
	return nil
}

// runRun runs the calls of a kind over a part of the split, or over all the sheets of the plan. Of
// the judge, the measurement of the writing process: every sheet in every form it has, by the three
// judges, repeated (R24), in the prompt of the plan: version 1 on a plan of lines, version 2 on a
// plan of parts, of free writing (T65). It first says how many calls the run makes and what they
// are expected to cost, and starts only once the author types yes (rule 4 of RUN.md); till then
// Ctrl-C ends it as any command, and from then on it stops the run before the next call. The log
// goes next to the sheets, out of git, named by the time, the kind, the sets of sheets and the part.
// A run of the reading or the hint may go over several sets, each given by -set; they lie in one
// folder, the samples or the synthetic, beside which the run logs. A run on sheets drawn by code, in
// synthetic, the executor starts within the task's ceiling (R28); one on the author's sheets, in
// samples, goes within the task's ceiling too, and its task says who starts it (R32).
func runRun(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, getenv func(string) string, now func() time.Time) int {
	flags := flag.NewFlagSet("tutor-eval run", flag.ContinueOnError)
	var f sheetFlags
	f.add(flags)
	kind := flags.String("kind", "judge", "the kind of call to run: judge, reading, hint, workbook or essay")
	var setValues setsFlag
	flags.Var(&setValues, "set", "a set of sheets, DIR,BANK,PLAN, in place of -dir, -bank and -plan; given once for each set, for a reading or a hint over several")
	part := flags.String("part", "", "the part of the split to run, dev or test, or all for every sheet of the plan, without the split; for the workbook and the essays, their pages and essays of that part, or all")
	repeats := flags.Int("repeats", 3, "how many times each call is repeated: of every judge, every tutor of the hint and every assessor of the essays, and of every reader but Opus, which reads once")
	readsList := flags.String("reads", "keys", "what a reading reads, joined by commas: keys, the finished sheet without a key, with the right key and with the false keys; heights, at 16, 24 and 48 px; states, the sheet as it was while it was written. For the workbook, bare, its pages at 32 px without the lines of the items; captions, with them and without, by default; and heights, at 16, 24 and 48 px")
	pagesFile := flags.String("workbook", workbookFile, "for -kind workbook, the file of the pages of the workbook")
	essaysPath := flags.String("essays", essaysFile, "for -kind essay, the file of the essays")
	seed := flags.Uint64("seed", 1, "for -kind workbook and essay, the number the hands of the pages and the essays come from")
	maxUSD := flags.Float64("max-usd", 0, "start no call that could take the run past this many US dollars; $3 for dev, $6 for test ($8 for the essays) and $2 for all by default")
	logPath := flags.String("log", "", "the log of the run, a new file out of git: in samples or synthetic, and in samples alone for sheets there; by default in runs next to the directory of the sheets, and for the workbook and the essays in synthetic/runs")
	stub := flags.Bool("stub", false, "ask no model: a stub answers by the markup, and the run needs no key and no network")
	readersList := flags.String("readers", strings.Join(runner.E4Configs, ","), "the configurations to ask, joined by commas: "+strings.Join(runner.ConfigNames(), ", ")+"; the three of E4 by default")
	if code, ok := parse(flags, args, stderr); !ok {
		return code
	}
	set := given(flags)
	k, ok := plans[*kind]
	if !ok {
		_, _ = fmt.Fprintf(stderr, "tutor-eval run: -kind is %q: give %s\n", *kind, strings.Join(slices.Sorted(maps.Keys(plans)), " or "))
		return exitUsage
	}
	var names []string
	for _, name := range strings.Split(*readersList, ",") {
		names = append(names, strings.TrimSpace(name))
	}
	o := options{readers: names, repeats: *repeats, workbook: *pagesFile, essays: *essaysPath, seed: *seed}
	if _, err := runner.Chosen(runner.Configs(), func(c runner.Config) runner.Config { return c }, o.readers); err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval run: -readers is %q: %v\n", *readersList, err)
		return exitUsage
	}
	var sets []sheetFlags
	if k.file != "" {
		if !fileFlags(*kind, k.file, set, stderr) {
			return exitUsage
		}
		if k.file == "workbook" {
			if o.workbookReads, ok = workbookFlags(set, *readsList); !ok {
				_, _ = fmt.Fprintf(stderr, "tutor-eval run: -reads is %q: for the workbook, give bare, captions or heights, joined by commas\n", *readsList)
				return exitUsage
			}
		}
	} else {
		if set["workbook"] || set["essays"] || set["seed"] {
			_, _ = fmt.Fprintf(stderr, "tutor-eval run: -workbook, -essays and -seed are for -kind workbook and essay, and the %s reads sheets\n", *kind)
			return exitUsage
		}
		if sets, ok = setsOf(f, setValues, set, stderr); !ok {
			return exitUsage
		}
	}
	reads, err := reading.ParseReads(*readsList)
	switch {
	case k.file == "workbook":
	case set["reads"] && *kind != "reading":
		_, _ = fmt.Fprintf(stderr, "tutor-eval run: -reads is for -kind reading or workbook, and the %s reads nothing of it\n", *kind)
		return exitUsage
	case err != nil:
		_, _ = fmt.Fprintf(stderr, "tutor-eval run: -reads is %q: give keys, heights or states, joined by commas\n", *readsList)
		return exitUsage
	}
	o.reads = reads
	if len(sets) > 1 && !k.several {
		_, _ = fmt.Fprintf(stderr, "tutor-eval run: the %s runs on one set of sheets, and %d are given: its criterion counts the test part of one set\n", *kind, len(sets))
		return exitUsage
	}
	caps := partCaps
	if k.caps != nil {
		caps = k.caps
	}
	planned, ok := caps[*part]
	if !ok {
		_, _ = fmt.Fprintf(stderr, "tutor-eval run: -part is %q: give dev, test or all\n", *part)
		return exitUsage
	}
	if !set["max-usd"] {
		*maxUSD = planned
	}
	if *repeats < 1 || !(*maxUSD > 0) {
		_, _ = fmt.Fprintln(stderr, "tutor-eval run: -repeats must be 1 or more, and -max-usd above 0")
		return exitUsage
	}
	ask := runner.Stub
	if !*stub {
		key := getenv(anthropicKeyVar)
		if key == "" {
			_, _ = fmt.Fprintf(stderr, "tutor-eval run: %s is not set: the run asks Claude; with -stub it asks no one\n", anthropicKeyVar)
			return exitUsage
		}
		ask = runner.Models(key)
	}

	// What follows the plan is the same for every kind.
	r, what, err := k.plan(sets, *part, o)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval run: %v\n", err)
		return exitFail
	}
	// The log is a new file, named to the millisecond, and by the kind, the sets of sheets, each the
	// name of its directory, joined by "+", and the part, so that the logs of two sets in one runs
	// directory are told apart: whatever keeps it from being written is said before the author is
	// asked.
	// The pages of the workbook and the essays are code's, and their runs log in synthetic alone.
	path := *logPath
	if path == "" {
		at := now().UTC()
		own := map[string]string{"workbook": *pagesFile, "essays": *essaysPath}[k.file]
		dir, names := filepath.Join(syntheticFolder, "runs"), []string{strings.TrimSuffix(filepath.Base(own), filepath.Ext(own))}
		if k.file == "" {
			dir, names = filepath.Join(filepath.Dir(filepath.Clean(sets[0].dir)), "runs"), nil
			for _, s := range sets {
				names = append(names, setName(s))
			}
		}
		path = filepath.Join(dir, fmt.Sprintf("%s-%03d-%s-%s-%s.jsonl", at.Format("20060102-150405"), at.Nanosecond()/int(time.Millisecond), *kind, strings.Join(names, "+"), *part))
	}
	// A run on the author's sheets logs in samples alone: its log holds every answer as the model
	// gave it, and nothing of the author's sheets goes to the folder agents work with.
	folders := []string{samplesFolder, syntheticFolder}
	switch {
	case k.file != "":
		folders = folders[1:]
	case sets[0].folder == samplesFolder:
		folders = folders[:1]
	}
	if err := checkIgnored(filepath.Dir(path), folders[0], folders[1:]...); err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval run: %v\n", err)
		return exitFail
	}
	if _, err := os.Lstat(path); err == nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval run: the log %s exists: a run writes a new one\n", path)
		return exitFail
	}

	e := r.Estimate()
	say := what + fmt.Sprintf("%s, expected to cost $%.2f, and at most $%.2f if every call took its worst. The cap is $%.2f.\n", count(e.Calls, "call"), e.USD, e.WorstUSD, *maxUSD)
	if *maxUSD > planned {
		say += fmt.Sprintf("That is above the $%.2f that architecture 3.1 and 3.4 plan for -part %s.\n", planned, *part)
	}
	if e.USD > *maxUSD {
		say += "The estimate is above the cap: the run will stop short of its calls unless -max-usd gives it more.\n"
	}
	if *stub {
		say += "The stub answers them by the markup and asks no one: it costs nothing, and it bills what the estimate expects.\n"
	}
	if _, err := fmt.Fprint(stdout, say+"Type yes to start the run: "); err != nil {
		return exitFail
	}
	answer, _ := bufio.NewReader(stdin).ReadString('\n')
	if strings.TrimSpace(answer) != "yes" {
		_, _ = fmt.Fprintln(stdout, "The run is not started.")
		return exitOK
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval run: %v\n", err)
		return exitFail
	}
	log, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval run: the log: %v\n", err)
		return exitFail
	}
	res, err := r.Do(ctx, ask, *maxUSD, log, now)
	if cerr := log.Close(); err == nil {
		err = cerr
	}
	_, _ = fmt.Fprintf(stdout, "%s made, $%.4f spent. The log: %s\n", count(res.Calls, "call"), res.SpentUSD, path)
	switch {
	case err != nil:
		_, _ = fmt.Fprintf(stderr, "tutor-eval run: the log cannot be written, and the run stopped: %v\n", err)
		return exitFail
	case res.Stopped != "":
		_, _ = fmt.Fprintf(stderr, "tutor-eval run: stopped short by %s\n", res.Stopped)
		return exitFail
	}
	return exitOK
}

// fileFlags checks the flags of a run of a kind that reads a file of its own and no sets of sheets,
// the workbook or the essays, and says why it refuses: no flag of the sheets, no file of the other
// kind, and -reads only for the workbook.
func fileFlags(kind, file string, set map[string]bool, stderr io.Writer) bool {
	for _, name := range []string{"set", "dir", "bank", "plan"} {
		if set[name] {
			_, _ = fmt.Fprintf(stderr, "tutor-eval run: -%s names sheets, and the %s reads its own file: give -%s and -seed\n", name, kind, file)
			return false
		}
	}
	for _, other := range []string{"workbook", "essays"} {
		if other != file && set[other] {
			_, _ = fmt.Fprintf(stderr, "tutor-eval run: -%s is not for -kind %s: give -%s and -seed\n", other, kind, file)
			return false
		}
	}
	if file != "workbook" && set["reads"] {
		_, _ = fmt.Fprintf(stderr, "tutor-eval run: -reads is for -kind reading or workbook, and the %s reads nothing of it\n", kind)
		return false
	}
	return true
}

// workbookFlags is what a run of the reading of the workbook reads: captions by default.
func workbookFlags(set map[string]bool, list string) (workbook.Reads, bool) {
	if !set["reads"] {
		list = "captions"
	}
	reads, err := workbook.ParseReads(list)
	return reads, err == nil
}

// setsOf are the sets of sheets of a run: those -set gives, each DIR,BANK,PLAN, or else the one of
// -dir, -bank and -plan. Each directory must be one a command of the sheets takes (usable), no two
// sets may share a name, the name of the directory, and all of them lie in one folder, the samples
// or the synthetic, beside which the run logs: a run does not mix the author's sheets with those of
// code. It says why it refuses.
func setsOf(f sheetFlags, values []string, set map[string]bool, stderr io.Writer) ([]sheetFlags, bool) {
	if len(values) == 0 {
		ok := f.usable("tutor-eval run", stderr)
		return []sheetFlags{f}, ok
	}
	if set["dir"] || set["bank"] || set["plan"] {
		_, _ = fmt.Fprintln(stderr, "tutor-eval run: -set names the directory, the bank and the plan of its set: give it in place of -dir, -bank and -plan")
		return nil, false
	}
	var sets []sheetFlags
	names := map[string]bool{}
	for _, v := range values {
		fields := strings.Split(v, ",")
		if len(fields) != 3 || slices.Contains(fields, "") {
			_, _ = fmt.Fprintf(stderr, "tutor-eval run: -set is %q: give DIR,BANK,PLAN\n", v)
			return nil, false
		}
		s := sheetFlags{dir: fields[0], bank: fields[1], plan: fields[2]}
		if !s.usable("tutor-eval run", stderr) {
			return nil, false
		}
		name := setName(s)
		switch {
		case names[name]:
			_, _ = fmt.Fprintf(stderr, "tutor-eval run: two sets are named %s: the name of a set is that of its directory, and the log tells the sheets by it\n", name)
			return nil, false
		case len(sets) > 0 && (filepath.Dir(absolute(s.dir)) != filepath.Dir(absolute(sets[0].dir)) || s.folder != sets[0].folder):
			_, _ = fmt.Fprintf(stderr, "tutor-eval run: the sets of a run lie in one folder, beside which it logs, and %s is not beside %s\n", s.dir, sets[0].dir)
			return nil, false
		}
		names[name] = true
		sets = append(sets, s)
	}
	return sets, true
}

// absolute is a path made absolute, or the path itself when the working directory cannot be read.
func absolute(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return filepath.Clean(path)
}

// setName is the name of a set of sheets: the name of its directory.
func setName(f sheetFlags) string { return filepath.Base(absolute(f.dir)) }

// planJudge plans a run of the judge over a part of its one set, and says what it asks: its
// sheets, how many of them have frames, the judges, the repeats and the version of the prompt,
// which is that of the plan.
func planJudge(sets []sheetFlags, part string, o options) (*runner.Run, string, error) {
	repeats := o.repeats
	plan, chosen, err := partSheets(sets[0], setName(sets[0]), part)
	if err != nil {
		return nil, "", err
	}
	version := judge.V1
	if plan.HasParts() {
		version = judge.V2
	}
	judges, err := runner.Chosen(measure.Judges(), func(j measure.Judge) runner.Config { return j.Config }, o.readers)
	if err != nil {
		return nil, "", err
	}
	r, err := measure.New(part, chosen, judges, repeats, version)
	if err != nil {
		return nil, "", err
	}
	var by []string
	for _, j := range judges {
		by = append(by, j.Name)
	}
	return &r.Run, fmt.Sprintf("%s: %s, %d of them with frames, by %s (%s), %s, in version %d of the prompt.\n",
		partName(part), count(len(chosen), "sheet"), r.Framed(), count(len(judges), "judge"), strings.Join(by, ", "), count(repeats, "repeat"), version), nil
}

// planReading plans a run of the reading over a part of its sets, and says what it asks: the sheets
// of each set, the readers and their repeats, and what it reads: the conditions, the false keys
// only on correct sheets (R03; T68), and the other heights and the states, by the readers that do
// not read once (T69).
func planReading(sets []sheetFlags, part string, o options) (*runner.Run, string, error) {
	repeats := o.repeats
	var names, counts []string
	var all []sheets.Chosen
	for _, s := range sets {
		name := setName(s)
		_, chosen, err := partSheets(s, name, part)
		if err != nil {
			return nil, "", fmt.Errorf("%s: %w", name, err)
		}
		names, counts = append(names, name), append(counts, fmt.Sprintf("%s %d", name, len(chosen)))
		all = append(all, chosen...)
	}
	keyed := 0
	for _, sh := range all {
		if reading.FalseKeyed(sh) {
			keyed++
		}
	}
	readers, err := runner.Chosen(reading.Readers(), readerConfig, o.readers)
	if err != nil {
		return nil, "", err
	}
	r, err := reading.New(part, names, all, readers, repeats, o.reads)
	if err != nil {
		return nil, "", err
	}
	planned := reading.PlannedOf(r)
	by, repeated := byReaders(readers, repeats)
	var what []string
	if o.reads.Keys {
		what = append(what, fmt.Sprintf("by %s, without a key and with the right key, and with the false keys on %s", by, count(keyed, "correct sheet")))
	}
	if (o.reads.Heights || o.reads.States) && repeated != "" {
		what = append(what, fmt.Sprintf("once by %s without a key, %s at other heights of digits and %s of %s", repeated, count(planned.Heights, "call"), count(planned.StateCalls, "call"), count(planned.States, "state")))
	}
	return r, fmt.Sprintf("%s: %s of %s (%s), %s, in version %d of the reading's prompt.\n",
		partName(part), count(len(all), "sheet"), count(len(sets), "set"), strings.Join(counts, ", "), strings.Join(what, "; and "), reading.Version), nil
}

// planHint plans a run of the hint over the sheets with a mistake of a part of its sets, and says
// what it asks: the sheets with a mistake of each set, of how many, each at every rung of the
// ladder in both languages, by the three tutors in their repeats (T74). A part with no sheet with a
// mistake has no hint to ask, and the run is refused before the question.
func planHint(sets []sheetFlags, part string, o options) (*runner.Run, string, error) {
	var names, counts []string
	var wrong []sheets.Chosen
	taken := 0
	for _, s := range sets {
		name := setName(s)
		_, chosen, err := partSheets(s, name, part)
		if err != nil {
			return nil, "", fmt.Errorf("%s: %w", name, err)
		}
		n := 0
		for _, sh := range chosen {
			if hint.Wrong(sh) {
				wrong = append(wrong, sh)
				n++
			}
		}
		taken += len(chosen)
		names, counts = append(names, name), append(counts, fmt.Sprintf("%s %d of %d", name, n, len(chosen)))
	}
	if len(wrong) == 0 {
		return nil, "", fmt.Errorf("%s: no sheet with a mistake (%s), and a hint is asked only after a wrong answer", partName(part), strings.Join(counts, ", "))
	}
	tutors, err := runner.Chosen(hint.Tutors(), func(tu hint.Tutor) runner.Config { return tu.Config }, o.readers)
	if err != nil {
		return nil, "", err
	}
	var by []string
	for _, tu := range tutors {
		by = append(by, tu.Name)
	}
	r, err := hint.New(part, names, wrong, tutors, o.repeats)
	if err != nil {
		return nil, "", err
	}
	return r, fmt.Sprintf("%s: %s with a mistake of %d in %s (%s), each at %d rungs in %d languages, by %s in %s, in version %d of the hint's prompt.\n",
		partName(part), count(len(wrong), "sheet"), taken, count(len(sets), "set"), strings.Join(counts, ", "), len(hint.Rungs()), len(hint.Langs()),
		strings.Join(by, ", "), count(o.repeats, "repeat"), hint.Version), nil
}

// planWorkbook plans a run of the reading of the workbook over the pages of a part, drawn from the
// seed, and says what it asks: the pages, the readers and repeats, what is read, and the version of
// the prompt (T79). It reads no sheets.
func planWorkbook(_ []sheetFlags, part string, o options) (*runner.Run, string, error) {
	wb, err := readWorkbook(o.workbook)
	if err != nil {
		return nil, "", err
	}
	pages := workbook.Pages(wb, part)
	readers, err := runner.Chosen(workbook.Readers(), readerConfig, o.readers)
	if err != nil {
		return nil, "", err
	}
	r, err := workbook.New(part, o.workbook, o.seed, pages, readers, o.repeats, o.workbookReads)
	if err != nil {
		return nil, "", err
	}
	by, repeated := byReaders(readers, o.repeats)
	var what []string
	switch {
	case o.workbookReads.Captions:
		what = append(what, fmt.Sprintf("bare and with the lines of the items at %d px, by %s", render.Digit, by))
	case o.workbookReads.Bare:
		what = append(what, fmt.Sprintf("bare at %d px, by %s", render.Digit, by))
	}
	if o.workbookReads.Heights && repeated != "" {
		what = append(what, fmt.Sprintf("bare at 16, 24 and 48 px, once by %s", repeated))
	}
	return r, fmt.Sprintf("%s: %s of the workbook %s, drawn with the seed %d, %s, a call a page, in version %d of the prompt of the workbook.\n",
		partName(part), count(len(pages), "page"), o.workbook, o.seed, strings.Join(what, "; and "), workbook.Version), nil
}

// planEssays plans a run of the assessment by the rubric over the essays of a part, drawn with the
// seed, by every reader in every repeat (T83).
func planEssays(_ []sheetFlags, part string, o options) (*runner.Run, string, error) {
	es, err := readFile(o.essays, sample.ReadEssays)
	if err != nil {
		return nil, "", err
	}
	essays := essay.Essays(es, part)
	readers, err := runner.Chosen(essay.Readers(), readerConfig, o.readers)
	if err != nil {
		return nil, "", err
	}
	r, err := essay.New(part, o.essays, o.seed, es, essays, readers, o.repeats)
	if err != nil {
		return nil, "", err
	}
	names := make([]string, len(readers))
	for i, rd := range readers {
		names[i] = rd.Name
	}
	return r, fmt.Sprintf("%s: %s of %s, drawn with the seed %d, at %d px, by %s in %s, a call an essay, in version %d of the prompt of the assessment by the rubric.\n",
		partName(part), count(len(essays), "essay"), o.essays, o.seed, render.Digit, strings.Join(names, " and "), count(o.repeats, "repeat"), essay.Version), nil
}

// byReaders says who reads in a run: those that read once, and the others in their repeats; and
// names the others alone, or none, for what only they read.
func byReaders(readers []runner.Reader, repeats int) (by, repeated string) {
	var once, rest []string
	for _, rd := range readers {
		if rd.Once {
			once = append(once, rd.Name)
		} else {
			rest = append(rest, rd.Name)
		}
	}
	repeated = strings.Join(rest, " and ")
	var parts []string
	if len(once) > 0 {
		parts = append(parts, strings.Join(once, " and ")+" once")
	}
	if len(rest) > 0 {
		parts = append(parts, repeated+" in "+count(repeats, "repeat"))
	}
	return strings.Join(parts, ", "), repeated
}

// readerConfig is the configuration of a reader, by which runner.Chosen takes the readers a run names.
func readerConfig(rd runner.Reader) runner.Config { return rd.Config }

// partName is what the run says of its part.
func partName(part string) string {
	if part == "all" {
		return "All the sheets"
	}
	return fmt.Sprintf("The %s part", part)
}

// partSheets reads the plan of a set and the sheets of a part of its split, each recorded, passing
// its check, and the very file the split took; or, for the part all, every sheet of the plan,
// recorded and passing, with no split. Each sheet is of the set by its name.
func partSheets(f sheetFlags, set, part string) (*sample.Plan, []sheets.Chosen, error) {
	bank, plan, err := readPlan(f.bank, f.plan)
	if err != nil {
		return nil, nil, err
	}
	rec, err := sheets.Read(f.dir, plan, bank)
	if err != nil {
		return nil, nil, err
	}
	var numbers []int
	if part == "all" {
		for i := range rec.Sheets {
			numbers = append(numbers, i+1)
		}
	} else {
		sp, err := readSplit(f.dir, plan)
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil, errors.New("the split is not made yet: tutor-eval split makes it, once, or -part all runs every sheet without it")
		}
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", sheets.SplitFile, err)
		}
		list := sp.Test
		if part == "dev" {
			list = sp.Dev
		}
		if stale := sheets.Stale(rec, sp); len(stale) > 0 {
			return nil, nil, fmt.Errorf("the split's file no longer counts for sheets %s: the run asks of the files the split took", sheets.Numbers(stale))
		}
		for _, ss := range list {
			numbers = append(numbers, ss.Number)
		}
	}
	var out []sheets.Chosen
	for _, n := range numbers {
		sh := rec.Sheets[n-1]
		switch {
		case sh.File == "":
			return nil, nil, fmt.Errorf("sheet %d is not recorded: -part all runs every sheet of the plan", n)
		case sh.Sample == nil:
			return nil, nil, fmt.Errorf("sheet %d does not pass its check: tutor-eval sheets says where", n)
		}
		i := slices.IndexFunc(bank.Tasks, func(t sample.Task) bool { return t.ID == sh.Sample.Task })
		out = append(out, sheets.Chosen{Set: set, Number: n, File: sh.File, Sample: sh.Sample, Task: &bank.Tasks[i], Plan: sh.Plan})
	}
	return plan, out, nil
}
