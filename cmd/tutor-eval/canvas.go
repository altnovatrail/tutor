package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"

	"github.com/altnovatrail/tutor/internal/evalsample"
	"github.com/altnovatrail/tutor/internal/newfile"
	"github.com/altnovatrail/tutor/internal/protocol"
	"github.com/altnovatrail/tutor/internal/reading"
	"github.com/altnovatrail/tutor/internal/report"
	"github.com/altnovatrail/tutor/internal/runner"
	"github.com/altnovatrail/tutor/internal/sheets"
	"github.com/altnovatrail/tutor/internal/workbook"
)

// The kinds of a sample of canvas, by the file of tasks its task is of.
const (
	kindSheet = "sheet"
	kindPage  = "page"
)

// setNamePattern is the form of the name of a set of canvas's, which names its logs and reports, and
// which finds them again: no character a pattern of filepath.Glob reads.
var setNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// canvasSample is a file of a directory of canvas's samples as the check read it: its name, the
// sample's ID, its kind, and why the runner refuses it, or what keeps the runner from reading it.
type canvasSample struct {
	file, id, kind string
	what           string
	refused        *evalsample.Refusal
	problem        string
}

// runCanvas checks that tutor's runner reads canvas's synthetic samples, as CI does with those of a
// pinned release of canvas (R38 spec; R76): every JSON file of -dir is held to the rules of sections
// 11 and 12 by spec's sample-check, read by the task it names, at the tag of tutor it names, from the
// trees of those tags under -at, and read on the stub, which asks no model and costs nothing: the
// sheets in a run of the reading, the pages in a run of the workbook, each with its log in -runs
// and its runner's reports beside it. The check fails when a sample breaks a rule, is refused for
// any reason but a child's (П-3), cannot be read, or a report does not measure a sample read; and
// says so, each sample by its file, its ID, its kind and task, and nothing of the handwriting.
func runCanvas(ctx context.Context, args []string, stdout, stderr io.Writer, now func() time.Time) int {
	flags := flag.NewFlagSet("tutor-eval canvas", flag.ContinueOnError)
	dir := flags.String("dir", "", "the directory of canvas's samples, a file of JSON each, in synthetic")
	at := flags.String("at", "", "the trees of tutor's tags the samples name, <at>/<tag>/eval/…, as just canvas-fetch writes them with git archive")
	name := flags.String("name", "", "the name of the set in the logs and the reports; the directory's name by default")
	runs := flags.String("runs", syntheticFolder+"/runs", "the directory of the logs of the runs, in synthetic")
	repeats := flags.Int("repeats", 1, "how many times each call is repeated, on the stub")
	if code, ok := parse(flags, args, stderr); !ok {
		return code
	}
	if *dir == "" || *at == "" || *repeats < 1 {
		_, _ = fmt.Fprintln(stderr, "tutor-eval canvas: -dir and -at are needed, and -repeats 1 or more")
		return exitUsage
	}
	for _, d := range []string{*dir, *runs} {
		if err := checkIgnored(d, syntheticFolder); err != nil {
			_, _ = fmt.Fprintf(stderr, "tutor-eval canvas: %v\n", err)
			return exitUsage
		}
	}
	if *name == "" {
		*name = filepath.Base(absolute(*dir))
	}
	if !setNamePattern.MatchString(*name) {
		_, _ = fmt.Fprintf(stderr, "tutor-eval canvas: the name of the set is %q: give -name of letters, digits, dots, dashes and underscores\n", *name)
		return exitUsage
	}
	fail := func(err error) int {
		_, _ = fmt.Fprintf(stderr, "tutor-eval canvas: %v\n", err)
		return exitFail
	}

	read, chosen, pages, sources, err := readCanvas(*dir, *at, *name)
	if err != nil {
		return fail(err)
	}
	var b strings.Builder
	failed := 0
	var refusals = map[string][]refusal{}
	for i, s := range read {
		switch {
		case s.problem != "":
			failed++
			fmt.Fprintf(&b, "%s: %s, not read: %s.\n", s.file, s.what, s.problem)
		case s.refused != nil:
			if s.refused.Reason != evalv1.Refusal_REFUSAL_CHILD {
				failed++
			}
			kind := s.kind
			if kind == "" {
				kind = kindSheet
			}
			refusals[kind] = append(refusals[kind], refusalOf("sample", i+1, *name, s.file, s.refused))
			fmt.Fprintf(&b, "%s: %s, refused as %s: %s.\n", s.file, s.what, evalsample.Reason(s.refused.Reason), s.refused.Detail)
		default:
			fmt.Fprintf(&b, "%s: %s: read.\n", s.file, s.what)
		}
	}
	fmt.Fprintf(&b, "Samples: %d; read %s and %s; refused %d; not read or refused for more than a child's, %d.\n",
		len(read), count(len(chosen), "sheet"), count(len(pages), "page"), len(refusals[kindSheet])+len(refusals[kindPage]), failed)
	_, _ = io.WriteString(stdout, b.String())

	// A refusal goes to the run of its kind, or to the other when its kind has none.
	if len(chosen) == 0 {
		refusals[kindPage] = append(refusals[kindPage], refusals[kindSheet]...)
		refusals[kindSheet] = nil
	} else if len(pages) == 0 {
		refusals[kindSheet] = append(refusals[kindSheet], refusals[kindPage]...)
		refusals[kindPage] = nil
	}
	stamp := now().UTC()
	prefix := filepath.Join(*runs, fmt.Sprintf("%s-%03d-%s", stamp.Format("20060102-150405"), stamp.Nanosecond()/int(time.Millisecond), *name))
	type planned struct {
		kind    string
		plan    func() (*runner.Run, error)
		refused []refusal
	}
	var plans []planned
	if len(chosen) > 0 {
		plans = append(plans, planned{"reading", func() (*runner.Run, error) {
			return reading.New("all", []string{*name}, chosen, runner.E4(reading.Readers(), readerConfig), *repeats, reading.Reads{Keys: true})
		}, refusals[kindSheet]})
	}
	if len(pages) > 0 {
		plans = append(plans, planned{"workbook", func() (*runner.Run, error) {
			r, err := workbook.New("all", strings.Join(sources, ", "), *dir, 0, pages, runner.E4(workbook.Readers(), readerConfig), *repeats, workbook.Reads{Bare: true, Captions: true})
			if err != nil {
				return nil, err
			}
			record := r.Record
			r.Record = func(h runner.Head) any {
				rec := record(h)
				if wr, ok := rec.(workbook.RunRecord); ok {
					wr.Set = *name + "-workbook"
					return wr
				}
				return rec
			}
			return r, nil
		}, refusals[kindPage]})
	}
	if len(plans) == 0 {
		return fail(errors.New("no sample of the directory is read, and the runner measures nothing"))
	}
	var logs []string
	for _, p := range plans {
		r, err := p.plan()
		if err != nil {
			return fail(err)
		}
		r.Refused = recordsOf(p.refused)
		path := prefix + "-" + p.kind + ".jsonl"
		if err := stubRun(ctx, r, path, now); err != nil {
			return fail(err)
		}
		if code := runReport([]string{path}, stdout, stderr); code != exitOK {
			return code
		}
		logs = append(logs, path)
	}
	joined, err := joinReports(prefix, *name, logs)
	if err != nil {
		return fail(err)
	}
	_, _ = fmt.Fprintf(stdout, "The runner's reports of the whole set, the sheets and the pages in one: %s\n", strings.Join(joined, ", "))
	if err := checkReports(joined, read); err != nil {
		return fail(err)
	}
	if failed > 0 {
		_, _ = fmt.Fprintf(stdout, "The canvas check fails: %s of %s cannot be read, or the runner refuses them for more than a child's.\n", count(failed, "sample"), *dir)
		return exitFail
	}
	_, _ = fmt.Fprintf(stdout, "The canvas check passes: the runner reads every sample of %s, or refuses it as a child's, and its reports measure each it reads.\n", *dir)
	return exitOK
}

// readCanvas reads the samples of a directory of canvas's: each JSON file held to the rules by
// sample-check, then read by the task it names at its tag of tutor (evalsample.Releases), a sheet
// of a bank (evalsample.ReadSheet) or a page of the workbook (evalsample.ReadMarkedPage). It gives
// each file as it was read, in the order of their names, and the sheets and pages the runner
// asks of, with the files of the workbook at their tags that its pages are of, in the trees. The
// error is a directory with no sample, a check that cannot run, or a tree of a tag that is not
// there.
func readCanvas(dir, at, name string) ([]canvasSample, []sheets.Chosen, []*evalsample.Page, []string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	var paths []string
	for _, e := range entries {
		if e.Type().IsRegular() && strings.HasSuffix(e.Name(), ".json") {
			paths = append(paths, filepath.Join(dir, e.Name()))
		}
	}
	if len(paths) == 0 {
		return nil, nil, nil, nil, fmt.Errorf("%s holds no sample, a file of JSON", dir)
	}
	broken, err := checkSamples(paths)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	rs := evalsample.NewReleases(at)
	var out []canvasSample
	var chosen []sheets.Chosen
	var pages []*evalsample.Page
	numbers := map[int]string{}
	sources := map[string]bool{}
	for _, path := range paths {
		s := canvasSample{file: filepath.Base(path), what: "a sample"}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		es, refused := evalsample.ReadChecked(data, broken[path])
		if refused != nil {
			s.refused, s.id = refused, refused.Sample
			out = append(out, s)
			continue
		}
		s.id = es.GetId()
		ref := es.GetTask()
		s.what = fmt.Sprintf("%s, of the task %s", s.id, ref.GetTask())
		if src := ref.GetTutor(); src != nil {
			s.what += fmt.Sprintf(" in %s of tutor %s", src.GetBank(), src.GetTag())
		}
		bank, wb, r, err := rs.Of(ref)
		switch {
		case errors.Is(err, evalsample.ErrOtherFile):
			s.problem = err.Error()
		case err != nil:
			return nil, nil, nil, nil, err
		case r != nil:
			s.refused = r
		case bank != nil:
			s.kind = kindSheet
			sh, plan, r, err := evalsample.ReadSheet(es, bank)
			switch {
			case err != nil:
				s.problem = err.Error()
			case r != nil:
				s.refused = r
			default:
				plan.Number = len(chosen) + 1
				t := bank.Bank.Task(sh.Sample.Task)
				chosen = append(chosen, sheets.Chosen{Set: name, Number: plan.Number, File: s.file, Sample: sh.Sample, Task: t, Plan: plan, ID: sh.ID, Truth: sh.Truth})
			}
		default:
			s.kind = kindPage
			pg, r, err := evalsample.ReadMarkedPage(es, wb)
			switch {
			case err != nil:
				s.problem = err.Error()
			case r != nil:
				s.refused = r
			case numbers[pg.Page.Number] != "":
				s.problem = fmt.Sprintf("its page %d is %s's too, and a run of the workbook reads a page once", pg.Page.Number, numbers[pg.Page.Number])
			default:
				numbers[pg.Page.Number] = s.file
				sources[filepath.Join(at, wb.Tag, wb.Path)] = true
				pages = append(pages, pg)
			}
		}
		if s.refused != nil {
			s.refused.Sample = s.id
		}
		out = append(out, s)
	}
	return out, chosen, pages, slices.Sorted(maps.Keys(sources)), nil
}

// stubRun runs a run on the stub, which asks no model, into a new log at path, with a cap above its
// worst estimate, so that no call is left out.
func stubRun(ctx context.Context, r *runner.Run, path string, now func() time.Time) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	log, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("the log: %w", err)
	}
	res, err := r.Do(ctx, runner.Stub, r.Estimate().WorstUSD+1, log, now)
	if cerr := log.Close(); err == nil {
		err = cerr
	}
	if err == nil && res.Stopped != "" {
		err = fmt.Errorf("it stopped short by %s", res.Stopped)
	}
	if err != nil {
		return fmt.Errorf("the run of %s: %w", path, err)
	}
	return nil
}

// joinReports joins the runner's reports that the report of each log wrote beside it, of one reader
// or one pair each, into one report of the set for each of them, as one run of the set would make
// it (report.Joined): prefix.<reader>.report.json, a new file. It gives their paths.
func joinReports(prefix, set string, logs []string) ([]string, error) {
	byName := map[string][]*evalv1.Report{}
	var names []string
	for _, l := range logs {
		base := strings.TrimSuffix(l, filepath.Ext(l))
		found, err := filepath.Glob(base + ".*.report.json")
		if err != nil {
			return nil, err
		}
		for _, f := range found {
			data, err := os.ReadFile(f)
			if err != nil {
				return nil, err
			}
			rep, err := protocol.ReadReport(bytes.NewReader(data))
			if err != nil {
				return nil, fmt.Errorf("%s: %w", f, err)
			}
			name := strings.TrimSuffix(strings.TrimPrefix(f, base+"."), ".report.json")
			if byName[name] == nil {
				names = append(names, name)
			}
			byName[name] = append(byName[name], rep)
		}
	}
	if len(names) == 0 {
		return nil, errors.New("the runs wrote no runner's report")
	}
	var out []string
	for _, name := range names {
		rep, err := report.Joined(set, byName[name]...)
		if err != nil {
			return nil, fmt.Errorf("the reports of %s: %w", name, err)
		}
		data, err := protocol.MarshalReport(rep)
		if err != nil {
			return nil, err
		}
		path := report.ReportFile(prefix, name)
		if err := newfile.Write(path, data, 0o600); err != nil {
			return nil, err
		}
		out = append(out, path)
	}
	return out, nil
}

// checkReports holds the runner's reports of the set to the samples the check read: every sample
// read is measured in each, and every one refused is refused for its reason; and no reading of the
// stub, which reads by the markup, is unchecked, since one would be tutor's runner failing on a
// sample of canvas.
func checkReports(files []string, read []canvasSample) error {
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		rep, err := protocol.ReadReport(bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
		results := map[string]*evalv1.SampleResult{}
		for _, sr := range rep.GetSamples() {
			results[sr.GetSample()] = sr
			for _, slot := range sr.GetMeasured().GetAnswerSlots() {
				for _, rd := range slot.GetReadings() {
					if rd.GetUnchecked() != evalv1.UncheckedCause_UNCHECKED_CAUSE_UNSPECIFIED {
						return fmt.Errorf("%s: a reading of the sample %s is unchecked, %s: the stub reads by the markup, so the runner failed on it", filepath.Base(f), sr.GetSample(), rd.GetUnchecked())
					}
				}
			}
		}
		for _, s := range read {
			if s.problem != "" || s.id == "" {
				continue
			}
			sr := results[s.id]
			switch {
			case sr == nil:
				return fmt.Errorf("%s does not name the sample %s of %s", filepath.Base(f), s.id, s.file)
			case s.refused != nil && sr.GetRefused() != s.refused.Reason:
				return fmt.Errorf("%s does not refuse the sample %s as %s", filepath.Base(f), s.id, evalsample.Reason(s.refused.Reason))
			case s.refused == nil && sr.GetMeasured() == nil:
				return fmt.Errorf("%s does not measure the sample %s, which the runner read", filepath.Base(f), s.id)
			}
		}
	}
	return nil
}
