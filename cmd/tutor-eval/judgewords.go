package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/altnovatrail/tutor/internal/correct"
	"github.com/altnovatrail/tutor/internal/essay"
	"github.com/altnovatrail/tutor/internal/hint"
	"github.com/altnovatrail/tutor/internal/runner"
	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/internal/wordsjudge"
)

// judgeCap is the cap of a run of the judge of the words by default, in US dollars: the hints of a
// run of the set on one configuration, 330, at less than a cent each by Sonnet at low, the judge
// that passed its control phrases (T119; R86).
const judgeCap = 3.0

// runJudgeWords runs the judge of the tutor's words (T118): on the hints of a log of the set of the
// words, -log, into a log beside it under its name and .judge.jsonl, or on the control phrases of
// -controls, into a log in synthetic/runs, each by the judge -judge, Sonnet at low by default. As run
// does, it first says what it asks and what it is expected to cost, and starts once the author types
// yes; with -stub it asks no one. tutor-eval report reports on the log.
func runJudgeWords(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, getenv func(string) string, now func() time.Time) int {
	const name = "tutor-eval judge-words"
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	logOf := flags.String("log", "", "the log of a run of the hint on the set of the words, whose hints the judge reads, of the words after a correct answer on their set, or of the essays in two calls, whose feedback it reads")
	controls := flags.String("controls", "", "the file of the judge's control phrases, such as eval/words-judge.json, in place of -log")
	judgeName := flags.String("judge", "sonnet-low", "the configuration that judges: "+strings.Join(runner.ConfigNames(), ", "))
	maxUSD := flags.Float64("max-usd", judgeCap, "start no call that could take the run past this many US dollars")
	out := flags.String("out", "", "the log of the judge, a new file: beside the log of the hint by default, or in synthetic/runs for the controls")
	stub := flags.Bool("stub", false, "ask no model: the judge that is always right answers, by the code's measures of a hint or by a control's answers")
	if code, ok := parse(flags, args, stderr); !ok {
		return code
	}
	if (*logOf == "") == (*controls == "") {
		_, _ = fmt.Fprintf(stderr, "%s: give -log or -controls, one of them\n", name)
		return exitUsage
	}
	if !(*maxUSD > 0) {
		_, _ = fmt.Fprintf(stderr, "%s: -max-usd must be above 0\n", name)
		return exitUsage
	}
	i := slices.IndexFunc(runner.Configs(), func(c runner.Config) bool { return c.Name == *judgeName })
	if i < 0 {
		_, _ = fmt.Fprintf(stderr, "%s: -judge is %q: give one of %s\n", name, *judgeName, strings.Join(runner.ConfigNames(), ", "))
		return exitUsage
	}
	judge := runner.Configs()[i]
	ask := runner.Stub
	if !*stub {
		key := getenv(anthropicKeyVar)
		if key == "" {
			_, _ = fmt.Fprintf(stderr, "%s: %s is not set: the judge asks Claude; with -stub it asks no one\n", name, anthropicKeyVar)
			return exitUsage
		}
		ask = claudeModels(key)
	}

	file := *logOf + *controls
	data, err := os.ReadFile(file)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return exitFail
	}
	sum := fmt.Sprintf("%x", sha256.Sum256(data))
	var r *runner.Run
	var what string
	path := *out
	// The judge's log goes where the log it reads lies: the hints of the author's sheets stay in
	// samples.
	folders := []string{syntheticFolder}
	if *logOf != "" {
		var err error
		switch runner.CallKind(data) {
		case essay.CallKind:
			r, err = feedbackRun(data, filepath.Base(file), sum, judge)
			what = fmt.Sprintf("The feedback of %s that came, each judged once by %s, in version %d of the judge's prompt.\n", file, judge.Name, wordsjudge.Version)
		case correct.CallKind:
			what = fmt.Sprintf("The words after a correct answer of %s that came, each judged once by %s, in version %d of the judge's prompt.\n", file, judge.Name, wordsjudge.Version)
			var l *correct.Log
			if l, err = correct.ReadLog(bytes.NewReader(data)); err == nil {
				r, err = wordsjudge.OfCorrect(l, filepath.Base(file), sum, judge)
			}
		default:
			what = fmt.Sprintf("The hints of %s that came, each judged once by %s, in version %d of the judge's prompt.\n", file, judge.Name, wordsjudge.Version)
			var l *hint.Log
			if l, err = hint.ReadLog(bytes.NewReader(data)); err == nil {
				r, err = wordsjudge.Of(l, filepath.Base(file), sum, judge)
			}
		}
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "%s: %s: %v\n", name, file, err)
			return exitFail
		}
		if path == "" {
			path = judgeLogOf(file)
		}
		if _, _, in, err := placeOf(filepath.Dir(file)); err == nil && in == samplesFolder {
			folders = []string{samplesFolder}
		}
	} else {
		c, err := wordsjudge.ReadControls(bytes.NewReader(data))
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "%s: %s: %v\n", name, file, err)
			return exitFail
		}
		r = wordsjudge.Check(c, filepath.Base(file), sum, judge)
		what = fmt.Sprintf("The %s of %s, each judged once by %s, in version %d of the judge's prompt.\n", count(len(c.Controls), "control phrase"), file, judge.Name, wordsjudge.Version)
		if path == "" {
			at := now().UTC()
			path = filepath.Join(syntheticFolder, "runs", fmt.Sprintf("%s-%03d-words-judge-controls.jsonl", at.Format("20060102-150405"), at.Nanosecond()/int(time.Millisecond)))
		}
	}
	if err := checkIgnored(filepath.Dir(path), folders[0]); err != nil {
		_, _ = fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return exitFail
	}
	if _, err := os.Lstat(path); err == nil {
		_, _ = fmt.Fprintf(stderr, "%s: the log %s exists: a run writes a new one\n", name, path)
		return exitFail
	}
	return startRun(ctx, name, r, what, "", path, *maxUSD, *stub, ask, stdin, stdout, stderr, now)
}

// judgeLogOf is where the judge's log of a log of the hint lies by default: beside it, under its
// name and .judge.jsonl, where admit-words finds it.
func judgeLogOf(hintLog string) string {
	return strings.TrimSuffix(hintLog, filepath.Ext(hintLog)) + ".judge.jsonl"
}

// feedbackRun plans the judging of the feedback of a log of the essays in two calls, with the topics
// of its essays from the file of essays the log names, as the run named it, from where it ran: each
// essay of the log must be the file's of its number, by its part and its unmet criteria, or the
// topics would be another file's.
func feedbackRun(data []byte, file, sum string, judge wordsjudge.Judge) (*runner.Run, error) {
	l, err := essay.ReadLog(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	f, err := os.Open(l.Run.File)
	if err != nil {
		return nil, fmt.Errorf("the essays of the log: %w", err)
	}
	defer func() { _ = f.Close() }()
	es, err := sample.ReadEssays(f)
	if err != nil {
		return nil, err
	}
	topics := map[int]string{}
	byNumber := map[int]*sample.Essay{}
	for i, e := range es.Essays {
		topics[e.Number], byNumber[e.Number] = e.Topic, &es.Essays[i]
	}
	for _, s := range l.Sheets {
		if e := byNumber[s.Essay]; e == nil || e.Part != s.Part || !slices.Equal(e.Unmet, s.Unmet) {
			return nil, fmt.Errorf("essay %d of the log is not the one of %s", s.Essay, l.Run.File)
		}
	}
	return wordsjudge.OfFeedback(l, topics, file, sum, judge)
}
