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

	"github.com/altnovatrail/tutor/internal/judge"
	"github.com/altnovatrail/tutor/internal/measure"
	"github.com/altnovatrail/tutor/internal/reading"
	"github.com/altnovatrail/tutor/internal/runner"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/sheets"
)

// The caps of the parts, in US dollars: $3 for the development and $6 for the test
// (docs/architecture.md, 3.1, «Смета»; 3.4), and $2 for all the sheets of a plan, without the
// split, which is how the author's three sheets of free writing run (3.4; T77).
var partCaps = map[string]float64{"dev": 3, "test": 6, "all": 2}

// plans are the kinds of call that run asks, by -kind, each with how it plans a run over a part of
// its sets of sheets and says what the run asks (T67): the judge, and the reading (T68). The other
// kinds of epic E4 come with their tasks.
var plans = map[string]struct {
	plan func(sets []sheetFlags, part string, repeats int) (*runner.Run, string, error)
	// several is whether the kind runs over several sets: the judge runs on one, since its
	// criterion counts the test part of one set (R28).
	several bool
}{
	"judge":   {plan: planJudge},
	"reading": {plan: planReading, several: true},
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
// A run of the reading may go over several sets, each given by -set; they lie in one folder, the
// samples or the synthetic, beside which the run logs. A run on the author's sheets, in samples, the author
// starts, and no agent reads them (R05); on sheets drawn by code, in synthetic, the executor starts
// it within the task's ceiling (R28).
func runRun(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, getenv func(string) string, now func() time.Time) int {
	flags := flag.NewFlagSet("tutor-eval run", flag.ContinueOnError)
	var f sheetFlags
	f.add(flags)
	kind := flags.String("kind", "judge", "the kind of call to run: judge or reading")
	var setValues setsFlag
	flags.Var(&setValues, "set", "a set of sheets, DIR,BANK,PLAN, in place of -dir, -bank and -plan; given once for each set, for a reading over several")
	part := flags.String("part", "", "the part of the split to run, dev or test, or all for every sheet of the plan, without the split")
	repeats := flags.Int("repeats", 3, "how many times each call is repeated: of every judge, and of every reader but Opus, which reads once")
	maxUSD := flags.Float64("max-usd", 0, "start no call that could take the run past this many US dollars; $3 for dev, $6 for test and $2 for all by default")
	logPath := flags.String("log", "", "the log of the run, a new file out of git: in samples or synthetic, and in samples alone for sheets there; by default in runs next to the directory of the sheets")
	stub := flags.Bool("stub", false, "ask no model: a stub answers by the markup, and the run needs no key and no network")
	if code, ok := parse(flags, args, stderr); !ok {
		return code
	}
	set := given(flags)
	sets, ok := setsOf(f, setValues, set, stderr)
	if !ok {
		return exitUsage
	}
	k, ok := plans[*kind]
	if !ok {
		_, _ = fmt.Fprintf(stderr, "tutor-eval run: -kind is %q: give %s\n", *kind, strings.Join(slices.Sorted(maps.Keys(plans)), " or "))
		return exitUsage
	}
	if len(sets) > 1 && !k.several {
		_, _ = fmt.Fprintf(stderr, "tutor-eval run: the %s runs on one set of sheets, and %d are given: its criterion counts the test part of one set\n", *kind, len(sets))
		return exitUsage
	}
	planned, ok := partCaps[*part]
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
	r, what, err := k.plan(sets, *part, *repeats)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval run: %v\n", err)
		return exitFail
	}
	// The log is a new file, named to the millisecond, and by the kind, the sets of sheets, each the
	// name of its directory, joined by "+", and the part, so that the logs of two sets in one runs
	// directory are told apart: whatever keeps it from being written is said before the author is
	// asked.
	path := *logPath
	if path == "" {
		at := now().UTC()
		var names []string
		for _, s := range sets {
			names = append(names, setName(s))
		}
		path = filepath.Join(filepath.Dir(filepath.Clean(sets[0].dir)), "runs", fmt.Sprintf("%s-%03d-%s-%s-%s.jsonl", at.Format("20060102-150405"), at.Nanosecond()/int(time.Millisecond), *kind, strings.Join(names, "+"), *part))
	}
	// A run on the author's sheets logs in samples alone: its log holds every answer as the model
	// gave it, and nothing of the author's sheets goes to the folder agents work with.
	folders := []string{samplesFolder, syntheticFolder}
	if sets[0].folder == samplesFolder {
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
func planJudge(sets []sheetFlags, part string, repeats int) (*runner.Run, string, error) {
	plan, chosen, err := partSheets(sets[0], setName(sets[0]), part)
	if err != nil {
		return nil, "", err
	}
	version := judge.V1
	if plan.HasParts() {
		version = judge.V2
	}
	r, err := measure.New(part, chosen, measure.Judges(), repeats, version)
	if err != nil {
		return nil, "", err
	}
	return &r.Run, fmt.Sprintf("%s: %s, %d of them with frames, by %d judges, %s, in version %d of the prompt.\n",
		partName(part), count(len(chosen), "sheet"), r.Framed(), len(measure.Judges()), count(repeats, "repeat"), version), nil
}

// planReading plans a run of the reading over a part of its sets, and says what it asks: the sheets
// of each set, the readers and their repeats, and the conditions, the false keys only on correct
// sheets (R03; T68).
func planReading(sets []sheetFlags, part string, repeats int) (*runner.Run, string, error) {
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
	readers := reading.Readers()
	r, err := reading.New(part, names, all, readers, repeats)
	if err != nil {
		return nil, "", err
	}
	var once, repeated []string
	for _, rd := range readers {
		if rd.Once {
			once = append(once, rd.Name)
		} else {
			repeated = append(repeated, rd.Name)
		}
	}
	by := strings.Join(repeated, " and ") + " in " + count(repeats, "repeat")
	if len(once) > 0 {
		by = strings.Join(once, " and ") + " once, " + by
	}
	return r, fmt.Sprintf("%s: %s of %s (%s), by %s, without a key and with the right key, and with the false keys on %s, in version %d of the reading's prompt.\n",
		partName(part), count(len(all), "sheet"), count(len(sets), "set"), strings.Join(counts, ", "), by, count(keyed, "correct sheet"), reading.Version), nil
}

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
