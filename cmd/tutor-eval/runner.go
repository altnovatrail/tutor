package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/altnovatrail/tutor/internal/measure"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/sheets"
)

// The caps of the two parts, in US dollars: $3 for the development and $6 for the test
// (docs/architecture.md, 3.1, «Смета»).
var partCaps = map[string]float64{"dev": 3, "test": 6}

// runRun runs the measurement over a part of the split: every sheet of the part in every form it
// has, by the three judges, repeated (R24). It first says how many calls the run makes and what
// they are expected to cost, and starts only once the author types yes (rule 4 of RUN.md); till
// then Ctrl-C ends it as any command, and from then on it stops the run before the next call. The
// log goes next to the sheets, out of git. A run on the author's sheets, in samples, the author
// starts, and no agent reads them (R05); on sheets drawn by code, in synthetic, the executor starts
// it within the task's ceiling (R28).
func runRun(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, getenv func(string) string, now func() time.Time) int {
	flags := flag.NewFlagSet("tutor-eval run", flag.ContinueOnError)
	var f sheetFlags
	f.add(flags)
	part := flags.String("part", "", "the part of the split to run: dev or test")
	repeats := flags.Int("repeats", 3, "how many times each sheet is asked in each form by each judge")
	maxUSD := flags.Float64("max-usd", 0, "start no call that could take the run past this many US dollars; $3 for dev and $6 for test by default")
	logPath := flags.String("log", "", "the log of the run, a new file out of git: in samples or synthetic, and in samples alone for sheets there; by default in runs next to the directory of the sheets")
	stub := flags.Bool("stub", false, "ask no model: a stub answers by the markup, and the run needs no key and no network")
	if code, ok := parse(flags, args, stderr); !ok {
		return code
	}
	set := given(flags)
	if !f.usable("tutor-eval run", stderr) {
		return exitUsage
	}
	planned, ok := partCaps[*part]
	if !ok {
		_, _ = fmt.Fprintf(stderr, "tutor-eval run: -part is %q: give dev or test\n", *part)
		return exitUsage
	}
	if !set["max-usd"] {
		*maxUSD = planned
	}
	if *repeats < 1 || !(*maxUSD > 0) {
		_, _ = fmt.Fprintln(stderr, "tutor-eval run: -repeats must be 1 or more, and -max-usd above 0")
		return exitUsage
	}
	ask := measure.Stub
	if !*stub {
		key := getenv(anthropicKeyVar)
		if key == "" {
			_, _ = fmt.Fprintf(stderr, "tutor-eval run: %s is not set: the run asks Claude; with -stub it asks no one\n", anthropicKeyVar)
			return exitUsage
		}
		ask = measure.Models(key)
	}

	chosen, err := partSheets(f, *part)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval run: %v\n", err)
		return exitFail
	}
	r, err := measure.New(*part, chosen, measure.Judges(), *repeats)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tutor-eval run: %v\n", err)
		return exitFail
	}
	// The log is a new file, named to the millisecond: whatever keeps it from being written is said
	// before the author is asked.
	path := *logPath
	if path == "" {
		at := now().UTC()
		path = filepath.Join(filepath.Dir(filepath.Clean(f.dir)), "runs", fmt.Sprintf("%s-%03d-%s.jsonl", at.Format("20060102-150405"), at.Nanosecond()/int(time.Millisecond), *part))
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
	say := fmt.Sprintf("The %s part: %s, %d of them with frames, by %d judges, %s.\n%s, expected to cost $%.2f, and at most $%.2f if every call took its worst. The cap is $%.2f.\n",
		*part, count(len(chosen), "sheet"), r.Framed(), len(measure.Judges()), count(*repeats, "repeat"), count(e.Calls, "call"), e.USD, e.WorstUSD, *maxUSD)
	if *maxUSD > planned {
		say += fmt.Sprintf("That is above the $%.2f that architecture 3.1 plans for the %s part.\n", planned, *part)
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

// partSheets reads the sheets of a part of the split: each recorded, passing its check, and the
// very file the split took.
func partSheets(f sheetFlags, part string) ([]measure.Sheet, error) {
	bank, plan, err := readPlan(f.bank, f.plan)
	if err == nil {
		err = linesOnly(f.plan, plan, "the measurement runs on")
	}
	if err != nil {
		return nil, err
	}
	rec, err := sheets.Read(f.dir, plan, bank)
	if err != nil {
		return nil, err
	}
	sp, err := readSplit(f.dir, plan)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, errors.New("the split is not made yet: tutor-eval split makes it, once")
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", sheets.SplitFile, err)
	}
	list := sp.Test
	if part == "dev" {
		list = sp.Dev
	}
	if stale := sheets.Stale(rec, sp); len(stale) > 0 {
		return nil, fmt.Errorf("the split's file no longer counts for sheets %s: the run asks of the files the split took", sheets.Numbers(stale))
	}
	var out []measure.Sheet
	for _, ss := range list {
		sh := rec.Sheets[ss.Number-1]
		if sh.Sample == nil {
			return nil, fmt.Errorf("sheet %d does not pass its check: tutor-eval sheets says where", ss.Number)
		}
		i := slices.IndexFunc(bank.Tasks, func(t sample.Task) bool { return t.ID == sh.Sample.Task })
		out = append(out, measure.Sheet{Number: ss.Number, File: sh.File, Sample: sh.Sample, Task: &bank.Tasks[i]})
	}
	return out, nil
}
