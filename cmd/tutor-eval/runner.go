package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
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
	"github.com/altnovatrail/tutor/internal/evalsample"
	"github.com/altnovatrail/tutor/internal/hint"
	"github.com/altnovatrail/tutor/internal/judge"
	"github.com/altnovatrail/tutor/internal/measure"
	"github.com/altnovatrail/tutor/internal/reading"
	"github.com/altnovatrail/tutor/internal/render"
	"github.com/altnovatrail/tutor/internal/runner"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/sheets"
	"github.com/altnovatrail/tutor/internal/workbook"
	"github.com/altnovatrail/tutor/provider/claude"
)

// The caps of the parts, in US dollars: $3 for the development and $6 for the test
// (docs/measurements.md, 1.1 and 3.1), and $2 for all the sheets of a plan, without the
// split, which is how the author's three sheets of free writing run (3.1; T77); and $8 for the
// sheets recorded so far, without the split, which is how the run of admission takes the author's
// sheets, of a plan not written whole (T126; R80).
var partCaps = map[string]float64{"dev": 3, "test": 6, "all": 2, "recorded": 8}

// plans are the kinds of call that run asks, by -kind, each with how it plans a run over a part of
// its sets of sheets and says what the run asks (T67): the judge, the reading (T68), the hint (T74),
// the reading of the workbook (T79), which reads the pages of the workbook and no sheets, and the
// assessment by the rubric (T83), which reads the essays.
var plans = map[string]struct {
	plan func(sets []sheetFlags, part string, o options) (*runner.Run, string, error)
	// several is whether the kind runs over several sets: the judge runs on one, since its
	// criterion counts the test part of one set (R28).
	several bool
	// file names the flag of the file a kind reads in place of sets of sheets, with the samples of its
	// pages or essays in -dir: workbook, the pages of the workbook (T79), or essays (T83; R71). Its
	// run logs in synthetic.
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
// what a reading reads (T69), and for the workbook, the file of its pages, the directory of their
// samples and what its reading reads (T79; R71); for the essays, their file and the directory of
// their samples (T83).
type options struct {
	two           bool
	readers       []string
	repeats       int
	reads         reading.Reads
	workbook      string
	essays        string
	dir           string
	workbookReads workbook.Reads
	// cases is the file of the cases of the set of the words, for a hint (T118), or "", and kinds
	// the kinds of case it asks, all of them when none is named (T119).
	cases string
	kinds []hint.CaseKind
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
	part := flags.String("part", "", "the part of the split to run, dev or test, or all for every sheet of the plan, without the split, or recorded for every sheet recorded so far, the rest of the plan left out; for the workbook and the essays, their pages and essays of that part, or all")
	repeats := flags.Int("repeats", 3, "how many times each call is repeated: of every judge, every tutor of the hint and every assessor of the essays, and of every reader but Opus, which reads once")
	readsList := flags.String("reads", "keys", "what a reading reads, joined by commas: keys, the finished sheet without a key, with the right key and with the false keys; heights, at 16, 24 and 48 px; states, the sheet as it was while it was written; plain, alone, the run of admission: the finished sheet without a key, by every reader in every repeat, Opus too, and the reader that reads once in a measurement, Opus, paired with each other. For the workbook, bare, its pages at 32 px without the lines of the items; captions, with them and without, by default; and heights, at 16, 24 and 48 px")
	pagesFile := flags.String("workbook", workbookFile, "for -kind workbook, the file of the pages of the workbook")
	essaysPath := flags.String("essays", essaysFile, "for -kind essay, the file of the essays")
	maxUSD := flags.Float64("max-usd", 0, "start no call that could take the run past this many US dollars; $3 for dev, $6 for test ($8 for the essays), $2 for all and $8 for recorded by default")
	logPath := flags.String("log", "", "the log of the run, a new file out of git: in samples or synthetic, and in samples alone for sheets there; by default in runs next to the directory of the sheets, and for the workbook and the essays in synthetic/runs")
	two := flags.Bool("two", false, "for -kind essay, assess in two calls: read the essay, then assess the text read (T92)")
	casesPath := flags.String("cases", "", "for -kind hint, the file of the cases of the set of the words, such as eval/words-cases.json, on the one set of its plan, asked case by case (T118)")
	kindsList := flags.String("case-kinds", "", "with -cases, the kinds of case to ask, joined by commas: rung, otherwise, note; all of them by default")
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
	if *two && *kind != "essay" {
		_, _ = fmt.Fprintf(stderr, "tutor-eval run: -two is for -kind essay, and the %s asks one call\n", *kind)
		return exitUsage
	}
	if *casesPath != "" && *kind != "hint" {
		_, _ = fmt.Fprintf(stderr, "tutor-eval run: -cases is for -kind hint, and the %s asks no case of the words\n", *kind)
		return exitUsage
	}
	o := options{two: *two, readers: names, repeats: *repeats, workbook: *pagesFile, essays: *essaysPath, cases: *casesPath}
	if *kindsList != "" {
		if *casesPath == "" {
			_, _ = fmt.Fprintln(stderr, "tutor-eval run: -case-kinds is for -cases, the cases of the set of the words")
			return exitUsage
		}
		for _, k := range strings.Split(*kindsList, ",") {
			kind := hint.CaseKind(strings.TrimSpace(k))
			if !slices.Contains(hint.CaseKinds(), kind) {
				_, _ = fmt.Fprintf(stderr, "tutor-eval run: -case-kinds is %q: give rung, otherwise or note, joined by commas\n", *kindsList)
				return exitUsage
			}
			o.kinds = append(o.kinds, kind)
		}
	}
	if _, err := runner.Chosen(runner.Configs(), func(c runner.Config) runner.Config { return c }, o.readers); err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval run: -readers is %q: %v\n", *readersList, err)
		return exitUsage
	}
	var sets []sheetFlags
	// own is the file a kind of the workbook or the essays reads, in place of sets of sheets.
	own := map[string]string{"workbook": *pagesFile, "essays": *essaysPath}[k.file]
	if k.file != "" {
		if !fileFlags(*kind, k.file, set, stderr) {
			return exitUsage
		}
		var err error
		if o.dir, err = drawnDirOf(f.dir, own, set["dir"]); err != nil {
			_, _ = fmt.Fprintf(stderr, "tutor-eval run: %v\n", err)
			return exitUsage
		}
		if k.file == "workbook" {
			if o.workbookReads, ok = workbookFlags(set, *readsList); !ok {
				_, _ = fmt.Fprintf(stderr, "tutor-eval run: -reads is %q: for the workbook, give bare, captions or heights, joined by commas\n", *readsList)
				return exitUsage
			}
		}
	} else {
		if set["workbook"] || set["essays"] {
			_, _ = fmt.Fprintf(stderr, "tutor-eval run: -workbook and -essays are for -kind workbook and essay, and the %s reads sheets\n", *kind)
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
		_, _ = fmt.Fprintf(stderr, "tutor-eval run: -reads is %q: %v\n", *readsList, err)
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
	if *part == "recorded" && *kind != "reading" && (*kind != "hint" || *casesPath == "") {
		// The sheets recorded so far are the run of admission's, of the reading alone (T126).
		ok = false
	}
	if !ok {
		_, _ = fmt.Fprintf(stderr, "tutor-eval run: -part is %q: give dev, test or all, or recorded for the reading, or the cases of a hint, of the sheets recorded so far\n", *part)
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
		ask = claudeModels(key)
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

	above := ""
	if *maxUSD > planned {
		plan := "measurements 1.1 and 3.1"
		if *part == "recorded" {
			plan = "architecture 3.9"
		}
		above = fmt.Sprintf("That is above the $%.2f that %s plan for -part %s.\n", planned, plan, *part)
	}
	return startRun(ctx, "tutor-eval run", r, what, above, path, *maxUSD, *stub, ask, stdin, stdout, stderr, now)
}

// claudeModels asks the models of a run's steps by the adapter of Claude, with the Anthropic key:
// the runner takes the adapter from here, so that no package of the library takes the SDK (R90).
func claudeModels(key string) runner.Asker { return runner.Models(claude.Dial, key) }

// startRun says what a planned run asks, how many calls and what they are expected to cost, and
// starts it only once the author types yes (rule 4 of RUN.md): till then Ctrl-C ends it as any
// command, and from then on it stops the run before the next call. It writes the log as a new file
// at path, and says what the run made and spent. above says that the cap is above the plan's.
func startRun(ctx context.Context, name string, r *runner.Run, what, above, path string, maxUSD float64, stub bool, ask runner.Asker, stdin io.Reader, stdout, stderr io.Writer, now func() time.Time) int {
	e := r.Estimate()
	say := what + fmt.Sprintf("%s, expected to cost $%.2f, and at most $%.2f if every call took its worst. The cap is $%.2f.\n", count(e.Calls, "call"), e.USD, e.WorstUSD, maxUSD) + above
	if e.USD > maxUSD {
		say += "The estimate is above the cap: the run will stop short of its calls unless -max-usd gives it more.\n"
	}
	if stub {
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
		_, _ = fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return exitFail
	}
	log, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%s: the log: %v\n", name, err)
		return exitFail
	}
	res, err := r.Do(ctx, ask, maxUSD, log, now)
	if cerr := log.Close(); err == nil {
		err = cerr
	}
	_, _ = fmt.Fprintf(stdout, "%s made, $%.4f spent. The log: %s\n", count(res.Calls, "call"), res.SpentUSD, path)
	switch {
	case err != nil:
		_, _ = fmt.Fprintf(stderr, "%s: the log cannot be written, and the run stopped: %v\n", name, err)
		return exitFail
	case res.Stopped != "":
		_, _ = fmt.Fprintf(stderr, "%s: stopped short by %s\n", name, res.Stopped)
		return exitFail
	}
	return exitOK
}

// fileFlags checks the flags of a run of a kind that reads a file of its own and no sets of sheets,
// the workbook or the essays, with the directory of their samples, and says why it refuses: no flag
// of the sheets, no file of the other kind, and -reads only for the workbook.
func fileFlags(kind, file string, set map[string]bool, stderr io.Writer) bool {
	for _, name := range []string{"set", "bank", "plan"} {
		if set[name] {
			_, _ = fmt.Fprintf(stderr, "tutor-eval run: -%s names sheets, and the %s reads its own file: give -%s and -dir\n", name, kind, file)
			return false
		}
	}
	for _, other := range []string{"workbook", "essays"} {
		if other != file && set[other] {
			_, _ = fmt.Fprintf(stderr, "tutor-eval run: -%s is not for -kind %s: give -%s and -dir\n", other, kind, file)
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
	plan, chosen, refused, err := partSheets(sets[0], setName(sets[0]), part)
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
	return &r.Run, refusedLine(refused) + fmt.Sprintf("%s: %s, %d of them with frames, by %s (%s), %s, in version %d of the prompt.\n",
		partName(part), count(len(chosen), "sheet"), r.Framed(), count(len(judges), "judge"), strings.Join(by, ", "), count(repeats, "repeat"), version), nil
}

// planReading plans a run of the reading over a part of its sets, and says what it asks: the sheets
// of each set, the readers and their repeats, and what it reads: the conditions, the false keys
// only on correct sheets (R03; T68), and the other heights and the states, by the readers that do
// not read once (T69).
func planReading(sets []sheetFlags, part string, o options) (*runner.Run, string, error) {
	repeats := o.repeats
	var names, counts []string
	var refused []refusal
	var all []sheets.Chosen
	for _, s := range sets {
		name := setName(s)
		_, chosen, off, err := partSheets(s, name, part)
		if err != nil {
			return nil, "", fmt.Errorf("%s: %w", name, err)
		}
		names, counts = append(names, name), append(counts, fmt.Sprintf("%s %d", name, len(chosen)))
		all, refused = append(all, chosen...), append(refused, off...)
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
	r.Refused = recordsOf(refused)
	planned := reading.PlannedOf(r)
	by, repeated := byReaders(readers, repeats)
	var what []string
	if o.reads.Keys {
		what = append(what, fmt.Sprintf("by %s, without a key and with the right key, and with the false keys on %s", by, count(keyed, "correct sheet")))
	}
	if o.reads.Plain {
		var readerNames []string
		for _, rd := range readers {
			readerNames = append(readerNames, rd.Name)
		}
		what = append(what, fmt.Sprintf("the run of admission: by %s in %s each, without a key at 32 px", strings.Join(readerNames, " and "), count(repeats, "repeat")))
	}
	if (o.reads.Heights || o.reads.States) && repeated != "" {
		what = append(what, fmt.Sprintf("once by %s without a key, %s at other heights of digits and %s of %s", repeated, count(planned.Heights, "call"), count(planned.StateCalls, "call"), count(planned.States, "state")))
	}
	return r, refusedLine(refused) + fmt.Sprintf("%s: %s of %s (%s), %s, in version %d of the reading's prompt.\n",
		partName(part), count(len(all), "sheet"), count(len(sets), "set"), strings.Join(counts, ", "), strings.Join(what, "; and "), reading.Version), nil
}

// planHint plans a run of the hint over the sheets with a mistake of a part of its sets, and says
// what it asks: the sheets with a mistake of each set, of how many, each at every rung of the
// ladder in both languages, by the three tutors in their repeats (T74). A part with no sheet with a
// mistake has no hint to ask, and the run is refused before the question.
func planHint(sets []sheetFlags, part string, o options) (*runner.Run, string, error) {
	var names, counts []string
	var refused []refusal
	var wrong []sheets.Chosen
	var set *hint.Set
	taken := 0
	for _, s := range sets {
		name := setName(s)
		var chosen []sheets.Chosen
		var off []refusal
		var err error
		if o.cases != "" {
			// The cases are checked against the very bank and plan the run takes its sheets of.
			var bank *evalsample.Bank
			var plan *sample.Plan
			if bank, plan, set, err = casesOf(o.cases, sets); err == nil {
				chosen, off, err = partOf(bank, plan, s, name, part)
			}
		} else {
			_, chosen, off, err = partSheets(s, name, part)
		}
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
		refused = append(refused, off...)
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
	left := ""
	if set != nil {
		var err error
		if left, err = keepCases(set, wrong, o.kinds); err != nil {
			return nil, "", err
		}
	}
	r, err := hint.New(part, names, wrong, tutors, o.repeats, set)
	if err != nil {
		return nil, "", err
	}
	if set != nil {
		return r, refusedLine(refused) + fmt.Sprintf("%s: %s of the set of the words in %s%s, on %s with a mistake of %d in %s, by %s in %s, in version %d of the hint's prompt.\n",
			partName(part), count(len(set.Cases.Cases), "case"), set.File, left, count(len(r.Items), "sheet"), taken, names[0], strings.Join(by, ", "), count(o.repeats, "repeat"), hint.Version), nil
	}
	return r, refusedLine(refused) + fmt.Sprintf("%s: %s with a mistake of %d in %s (%s), each at %d rungs in %d languages, by %s in %s, in version %d of the hint's prompt.\n",
		partName(part), count(len(wrong), "sheet"), taken, count(len(sets), "set"), strings.Join(counts, ", "), len(hint.Rungs()), len(hint.Langs()),
		strings.Join(by, ", "), count(o.repeats, "repeat"), hint.Version), nil
}

// keepCases keeps of the cases of a set those a run asks: of the kinds named, all when none is, and
// of the sheets with a mistake the part holds — not those left out of the split, refused by their
// check or, of the sheets recorded so far, not recorded yet (T119); and says what it left out, which
// the run's record counts. A run with no case left is refused.
func keepCases(set *hint.Set, wrong []sheets.Chosen, kinds []hint.CaseKind) (string, error) {
	held := map[int]bool{}
	for _, sh := range wrong {
		held[sh.Number] = true
	}
	all := *set.Cases
	all.Cases = nil
	unheld, other := 0, 0
	for _, k := range set.Cases.Cases {
		switch {
		case len(kinds) > 0 && !slices.Contains(kinds, k.Kind):
			other++
		case !held[k.Sheet]:
			unheld++
		default:
			all.Cases = append(all.Cases, k)
		}
	}
	if len(all.Cases) == 0 {
		return "", fmt.Errorf("%s: no case is left to ask", set.File)
	}
	set.Cases, set.LeftOut = &all, other+unheld
	var left []string
	if other > 0 {
		left = append(left, count(other, "case")+" of other kinds")
	}
	if unheld > 0 {
		left = append(left, count(unheld, "case")+" of sheets the run does not hold")
	}
	if len(left) == 0 {
		return "", nil
	}
	return " (" + strings.Join(left, " and ") + " left out)", nil
}

// casesOf reads the file of the cases of the set of the words and checks them against the one set of
// sheets they are of: the plan the file names, by its file's name, with its bank, which it gives
// for the run to take its sheets of (T118).
func casesOf(path string, sets []sheetFlags) (*evalsample.Bank, *sample.Plan, *hint.Set, error) {
	if len(sets) != 1 {
		return nil, nil, nil, fmt.Errorf("the cases are of the sheets of one plan, and %s are given", count(len(sets), "set"))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, nil, err
	}
	cs, err := hint.ReadCases(bytes.NewReader(data))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("%s: %w", path, err)
	}
	if cs.Plan != filepath.Base(sets[0].plan) {
		return nil, nil, nil, fmt.Errorf("%s: the cases are of the plan %s, and the set's is %s", path, cs.Plan, sets[0].plan)
	}
	bank, plan, err := readPlan(sets[0].bank, sets[0].plan)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := cs.Check(plan, bank.Bank); err != nil {
		return nil, nil, nil, fmt.Errorf("%s: %w", path, err)
	}
	return bank, plan, &hint.Set{File: filepath.Base(path), SHA256: fmt.Sprintf("%x", sha256.Sum256(data)), Cases: cs}, nil
}

// planWorkbook plans a run of the reading of the workbook over the pages of a part, as the samples
// of the directory hold them, held to the rules of sections 11 and 12 by spec's check, and says what
// it asks: the pages, the readers and repeats, what is read, and the version of the prompt (T79;
// R71). A page whose sample the runner refuses is named and not asked of. It reads no sheets.
func planWorkbook(_ []sheetFlags, part string, o options) (*runner.Run, string, error) {
	w, d, seed, err := readPages(o.workbook, o.dir, checkSamples)
	if err != nil {
		return nil, "", err
	}
	var numbers []int
	for _, pg := range workbook.Pages(w.Workbook, part) {
		numbers = append(numbers, pg.Number)
	}
	if len(numbers) == 0 {
		return nil, "", fmt.Errorf("workbook: no page of the part %s", part)
	}
	pages, refused, err := partDrawn(d, "page", numbers)
	if err != nil {
		return nil, "", err
	}
	readers, err := runner.Chosen(workbook.Readers(), readerConfig, o.readers)
	if err != nil {
		return nil, "", err
	}
	r, err := workbook.New(part, o.workbook, o.dir, seed, pages, readers, o.repeats, o.workbookReads)
	if err != nil {
		return nil, "", err
	}
	r.Refused = recordsOf(refused)
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
	return r, refusedLine(refused) + fmt.Sprintf("%s: %s of the workbook %s, drawn with the seed %d into %s, %s, a call a page, in version %d of the prompt of the workbook.\n",
		partName(part), count(len(pages), "page"), o.workbook, seed, o.dir, strings.Join(what, "; and "), workbook.Version), nil
}

// planEssays plans a run of the assessment by the rubric over the essays of a part, as the samples
// of the directory hold them, by every reader in every repeat (T83; R71). An essay whose sample the
// runner refuses is named and not asked of.
func planEssays(_ []sheetFlags, part string, o options) (*runner.Run, string, error) {
	f, d, seed, err := readEssays(o.essays, o.dir, checkSamples)
	if err != nil {
		return nil, "", err
	}
	es := f.Essays
	var numbers []int
	for _, e := range essay.Essays(es, part) {
		numbers = append(numbers, e.Number)
	}
	if len(numbers) == 0 {
		return nil, "", fmt.Errorf("essay: no essay of the part %s", part)
	}
	essays, refused, err := partDrawn(d, "essay", numbers)
	if err != nil {
		return nil, "", err
	}
	readers, err := runner.Chosen(essay.Readers(), readerConfig, o.readers)
	if err != nil {
		return nil, "", err
	}
	newRun, calls := essay.New, "a call an essay"
	version := essay.Version
	if o.two {
		newRun, calls, version = essay.NewTwo, "two calls an essay, the reading and the assessment of the text it read", essay.TwoVersion
	}
	r, err := newRun(part, o.essays, o.dir, seed, es, essays, readers, o.repeats)
	if err != nil {
		return nil, "", err
	}
	r.Refused = recordsOf(refused)
	names := make([]string, len(readers))
	for i, rd := range readers {
		names[i] = rd.Name
	}
	prompt := "the prompt of the assessment by the rubric"
	if o.two {
		prompt = "the prompts of the assessment in two calls"
	}
	return r, refusedLine(refused) + fmt.Sprintf("%s: %s of %s, drawn with the seed %d into %s, at %d px, by %s in %s, %s, in version %d of %s.\n",
		partName(part), count(len(essays), "essay"), o.essays, seed, o.dir, render.Digit, strings.Join(names, " and "), count(o.repeats, "repeat"), calls, version, prompt), nil
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
	switch part {
	case "all":
		return "All the sheets"
	case "recorded":
		return "The sheets recorded"
	}
	return fmt.Sprintf("The %s part", part)
}

// partSheets reads the plan of a set and the sheets of a part of its split, each recorded, passing
// its check, and the very file the split took; or, for the part all, every sheet of the plan,
// recorded and passing, with no split; or, for the part recorded, every sheet of the plan recorded so
// far, the rest left out. Each sheet is of the set by its name. A sheet whose sample of
// the protocol the runner refuses (section 13 of spec's text of the contracts) is not asked of, and
// nothing of it goes to AI: refused names each, with its reason.
func partSheets(f sheetFlags, set, part string) (plan *sample.Plan, chosen []sheets.Chosen, refused []refusal, err error) {
	bank, plan, err := readPlan(f.bank, f.plan)
	if err != nil {
		return nil, nil, nil, err
	}
	chosen, refused, err = partOf(bank, plan, f, set, part)
	return plan, chosen, refused, err
}

// partOf is partSheets of a bank and a plan already read.
func partOf(bank *evalsample.Bank, plan *sample.Plan, f sheetFlags, set, part string) (chosen []sheets.Chosen, refused []refusal, err error) {
	rec, err := sheets.Read(f.dir, plan, bank, checkSamples)
	if err != nil {
		return nil, nil, err
	}
	var numbers []int
	if part == "all" || part == "recorded" {
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
	for _, n := range numbers {
		sh := rec.Sheets[n-1]
		switch {
		case sh.File == "" && part == "recorded":
			continue
		case sh.File == "":
			return nil, nil, fmt.Errorf("sheet %d is not recorded: -part all runs every sheet of the plan, and -part recorded those recorded", n)
		case sh.Refused != nil:
			refused = append(refused, refusalOf(set+" sheet", n, set, sh.File, sh.Refused))
			continue
		case sh.Sample == nil:
			return nil, nil, fmt.Errorf("sheet %d does not pass its check: tutor-eval sheets says where", n)
		}
		i := slices.IndexFunc(bank.Bank.Tasks, func(t sample.Task) bool { return t.ID == sh.Sample.Task })
		chosen = append(chosen, sheets.Chosen{Set: set, Number: n, File: sh.File, Sample: sh.Sample, Task: &bank.Bank.Tasks[i], Plan: sh.Plan, ID: sh.ID, Truth: sh.Truth})
	}
	return chosen, refused, nil
}

// refusedLine names the sheets a run refuses, before what it asks: nothing of them goes to AI.
func refusedLine(refused []refusal) string {
	if len(refused) == 0 {
		return ""
	}
	lines := make([]string, len(refused))
	for i, r := range refused {
		lines[i] = r.line
	}
	return fmt.Sprintf("Refused, and nothing of them goes to AI: %s.\n", strings.Join(lines, "; "))
}
