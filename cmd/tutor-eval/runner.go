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
	"github.com/altnovatrail/tutor/internal/runner"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/sheets"
)

// The caps of the parts, in US dollars: $3 for the development and $6 for the test
// (docs/architecture.md, 3.1, «Смета»; 3.4), and $2 for all the sheets of a plan, without the
// split, which is how the author's three sheets of free writing run (3.4; T77).
var partCaps = map[string]float64{"dev": 3, "test": 6, "all": 2}

// plans are the kinds of call that run asks, by -kind, each with how it plans a run over a part and
// says what the run asks (T67): so far the judge alone. The kinds of epic E4 come with their tasks,
// reading first (T68).
var plans = map[string]func(f sheetFlags, part string, repeats int) (*runner.Run, string, error){
	"judge": planJudge,
}

// runRun runs the calls of a kind over a part of the split, or over all the sheets of the plan. Of
// the judge, the measurement of the writing process: every sheet in every form it has, by the three
// judges, repeated (R24), in the prompt of the plan: version 1 on a plan of lines, version 2 on a
// plan of parts, of free writing (T65). It first says how many calls the run makes and what they
// are expected to cost, and starts only once the author types yes (rule 4 of RUN.md); till then
// Ctrl-C ends it as any command, and from then on it stops the run before the next call. The log
// goes next to the sheets, out of git, named by the time, the kind, the set of sheets and the part. A run on the author's sheets, in samples, the author
// starts, and no agent reads them (R05); on sheets drawn by code, in synthetic, the executor starts
// it within the task's ceiling (R28).
func runRun(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, getenv func(string) string, now func() time.Time) int {
	flags := flag.NewFlagSet("tutor-eval run", flag.ContinueOnError)
	var f sheetFlags
	f.add(flags)
	kind := flags.String("kind", "judge", "the kind of call to run: judge")
	part := flags.String("part", "", "the part of the split to run, dev or test, or all for every sheet of the plan, without the split")
	repeats := flags.Int("repeats", 3, "how many times each sheet is asked in each form by each judge")
	maxUSD := flags.Float64("max-usd", 0, "start no call that could take the run past this many US dollars; $3 for dev, $6 for test and $2 for all by default")
	logPath := flags.String("log", "", "the log of the run, a new file out of git: in samples or synthetic, and in samples alone for sheets there; by default in runs next to the directory of the sheets")
	stub := flags.Bool("stub", false, "ask no model: a stub answers by the markup, and the run needs no key and no network")
	if code, ok := parse(flags, args, stderr); !ok {
		return code
	}
	set := given(flags)
	if !f.usable("tutor-eval run", stderr) {
		return exitUsage
	}
	plan, ok := plans[*kind]
	if !ok {
		_, _ = fmt.Fprintf(stderr, "tutor-eval run: -kind is %q: give %s\n", *kind, strings.Join(slices.Sorted(maps.Keys(plans)), " or "))
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
	r, what, err := plan(f, *part, *repeats)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval run: %v\n", err)
		return exitFail
	}
	// The log is a new file, named to the millisecond, and by the kind, the set of sheets, which is
	// the name of their directory, and the part, so that the logs of two sets in one runs directory
	// are told apart: whatever keeps it from being written is said before the author is asked.
	path := *logPath
	if path == "" {
		at := now().UTC()
		dir := filepath.Clean(f.dir)
		abs, err := filepath.Abs(dir)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "tutor-eval run: %v\n", err)
			return exitFail
		}
		path = filepath.Join(filepath.Dir(dir), "runs", fmt.Sprintf("%s-%03d-%s-%s-%s.jsonl", at.Format("20060102-150405"), at.Nanosecond()/int(time.Millisecond), *kind, filepath.Base(abs), *part))
	}
	// A run on the author's sheets logs in samples alone: its log holds every answer as the model
	// gave it, and nothing of the author's sheets goes to the folder agents work with.
	folders := []string{samplesFolder, syntheticFolder}
	if f.folder == samplesFolder {
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

// planJudge plans a run of the judge over a part, and says what it asks: its sheets, how many of
// them have frames, the judges, the repeats and the version of the prompt.
func planJudge(f sheetFlags, part string, repeats int) (*runner.Run, string, error) {
	chosen, version, err := partSheets(f, part)
	if err != nil {
		return nil, "", err
	}
	r, err := measure.New(part, chosen, measure.Judges(), repeats, version)
	if err != nil {
		return nil, "", err
	}
	what := fmt.Sprintf("The %s part", part)
	if part == "all" {
		what = "All the sheets"
	}
	return &r.Run, fmt.Sprintf("%s: %s, %d of them with frames, by %d judges, %s, in version %d of the prompt.\n",
		what, count(len(chosen), "sheet"), r.Framed(), len(measure.Judges()), count(repeats, "repeat"), version), nil
}

// partSheets reads the sheets of a part of the split, each recorded, passing its check, and the
// very file the split took; or, for the part all, every sheet of the plan, recorded and passing,
// with no split. It gives the version of the prompt the plan takes: version 1 for a plan of lines
// and version 2 for a plan of parts.
func partSheets(f sheetFlags, part string) ([]measure.Sheet, judge.Version, error) {
	bank, plan, err := readPlan(f.bank, f.plan)
	if err != nil {
		return nil, 0, err
	}
	version := judge.V1
	if plan.HasParts() {
		version = judge.V2
	}
	rec, err := sheets.Read(f.dir, plan, bank)
	if err != nil {
		return nil, 0, err
	}
	var numbers []int
	if part == "all" {
		for i := range rec.Sheets {
			numbers = append(numbers, i+1)
		}
	} else {
		sp, err := readSplit(f.dir, plan)
		if errors.Is(err, fs.ErrNotExist) {
			return nil, 0, errors.New("the split is not made yet: tutor-eval split makes it, once, or -part all runs every sheet without it")
		}
		if err != nil {
			return nil, 0, fmt.Errorf("%s: %w", sheets.SplitFile, err)
		}
		list := sp.Test
		if part == "dev" {
			list = sp.Dev
		}
		if stale := sheets.Stale(rec, sp); len(stale) > 0 {
			return nil, 0, fmt.Errorf("the split's file no longer counts for sheets %s: the run asks of the files the split took", sheets.Numbers(stale))
		}
		for _, ss := range list {
			numbers = append(numbers, ss.Number)
		}
	}
	var out []measure.Sheet
	for _, n := range numbers {
		sh := rec.Sheets[n-1]
		switch {
		case sh.File == "":
			return nil, 0, fmt.Errorf("sheet %d is not recorded: -part all runs every sheet of the plan", n)
		case sh.Sample == nil:
			return nil, 0, fmt.Errorf("sheet %d does not pass its check: tutor-eval sheets says where", n)
		}
		i := slices.IndexFunc(bank.Tasks, func(t sample.Task) bool { return t.ID == sh.Sample.Task })
		out = append(out, measure.Sheet{Number: n, File: sh.File, Sample: sh.Sample, Task: &bank.Tasks[i], Plan: sh.Plan})
	}
	return out, version, nil
}
