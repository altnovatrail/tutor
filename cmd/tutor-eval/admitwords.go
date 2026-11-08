package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/altnovatrail/tutor/internal/hint"
	"github.com/altnovatrail/tutor/internal/report"
	"github.com/altnovatrail/tutor/internal/wordsjudge"
)

// runAdmitWords applies the rule of admission of the words (T119; R86) to the judge's log of its
// control phrases, -controls, and to logs of the hint on cases of the words, each with the judge's
// log beside it, .judge.jsonl, and prints in Markdown the stage it admits the tutor to, or every
// reason why none, with the counts the targets read and the SHA-256 of every file. It prints no
// hint and nothing of any work, so that it may run on the logs of the author's sheets.
func runAdmitWords(args []string, stdout, stderr io.Writer) int {
	const name = "tutor-eval admit-words"
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	controls := flags.String("controls", "", "the judge's log of its control phrases")
	flags.SetOutput(stderr)
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage
	}
	logs := flags.Args()
	if *controls == "" || len(logs) == 0 {
		_, _ = fmt.Fprintf(stderr, "usage: %s -controls LOG HINT-LOG...\n", name)
		return exitUsage
	}
	var files []string
	sums := map[string]string{}
	read := func(path string) ([]byte, error) {
		data, err := os.ReadFile(path)
		if err == nil {
			sum := sha256.Sum256(data)
			sums[path] = hex.EncodeToString(sum[:])
			files = append(files, fmt.Sprintf("- %s: sha256 %s", filepath.Base(path), sums[path]))
		}
		return data, err
	}
	judgeOf := func(path string) (*report.WordsJudge, error) {
		data, err := read(path)
		if err != nil {
			return nil, err
		}
		l, err := wordsjudge.ReadLog(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		return report.NewWordsJudge(l)
	}
	c, err := judgeOf(*controls)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return exitFail
	}
	var sets []report.WordsSet
	for _, path := range logs {
		data, err := read(path)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "%s: %v\n", name, err)
			return exitFail
		}
		l, err := hint.ReadLog(bytes.NewReader(data))
		var h *report.Hint
		if err == nil {
			h, err = report.NewHint(l)
		}
		var j *report.WordsJudge
		if err == nil {
			j, err = judgeOf(judgeLogOf(path))
		}
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "%s: %s: %v\n", name, path, err)
			return exitFail
		}
		sets = append(sets, report.WordsSet{Name: filepath.Base(path), SHA256: sums[path], Hint: h, Judge: j})
	}
	d := report.AdmitWords(c, sets)
	stage := string(d.Stage)
	if stage == "" {
		stage = "none"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# The admission of the words\n\n- stage: %s\n- tutor: %s, the prompt of the hint %s, version %d\n- judge: %s, right on %d of %d control phrases\n",
		stage, d.Tutor, hint.Fingerprint(), hint.Version, d.Judge, d.ControlsRight, d.Controls)
	fmt.Fprintf(&b, "- calls: %d, hints that came %d, failed %d; hints the judge did not judge: %d\n", d.Calls, d.Came, d.Failed, d.JudgeFailed)
	fmt.Fprintf(&b, "- the judge: the learner's sex told %s, off the task %s, not for a child %s, the note taken up %s\n", d.Gender, d.OffTask, d.NotForAge, d.Note)
	fmt.Fprintf(&b, "- the code: the note echoed %s, a question asked repeated %s (at most %.0f %%)\n", d.Echo, d.Repeated, report.WordsRepeated*100)
	fmt.Fprintf(&b, "- leaked before the check %s (at most %.3f), the form of R70 spec broken %s (at most %.3f)\n", d.Leaked, report.V1Leaked, d.Broken, report.V1Broken)
	for _, w := range d.Why {
		fmt.Fprintf(&b, "- not admitted: %s\n", w)
	}
	b.WriteString("\n## Files\n\n" + strings.Join(files, "\n") + "\n")
	if _, err := io.WriteString(stdout, b.String()); err != nil {
		return exitFail
	}
	return exitOK
}
