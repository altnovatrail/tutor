package paperset

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	_ "image/png" // the crop of the work is a PNG
	"os"
	"path/filepath"
	"slices"
	"strings"

	v1 "github.com/altnovatrail/spec/contracts/novatrail/api/v1"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/altnovatrail/tutor/internal/sample"
	"github.com/altnovatrail/tutor/provider"
)

// outcomes are the verdicts of a case of the debug mode as the set names them.
var outcomes = map[v1.DebugOutcome]string{
	v1.DebugOutcome_DEBUG_OUTCOME_CORRECT:    OutcomeCorrect,
	v1.DebugOutcome_DEBUG_OUTCOME_NOT_YET:    OutcomeNotYet,
	v1.DebugOutcome_DEBUG_OUTCOME_UNREADABLE: OutcomeUnreadable,
	v1.DebugOutcome_DEBUG_OUTCOME_UNCHECKED:  OutcomeUnchecked,
}

// exported reads a folder of the cases the debug mode of a local run exports, a file of the JSON of
// DebugCase each, in the order of their names (R136): a case of the part dev, its pictures those
// the Check sent and its truth the developer's. Its kind follows from the truth: a formula with "="
// an equation, any other an expression; no key open; a key of a choice a choice; else a word
// problem.
func exported(dir string) (*Set, error) {
	names, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	s := &Set{Description: "The cases the debug mode of a local run exported from " + dir + "."}
	for i, name := range names {
		data, err := os.ReadFile(name)
		if err != nil {
			return nil, err
		}
		var dc v1.DebugCase
		if err := protojson.Unmarshal(data, &dc); err != nil {
			return nil, fmt.Errorf("%s: not a case of the debug mode: %w", name, err)
		}
		c, err := caseOf(i+1, &dc)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		s.Cases = append(s.Cases, c)
	}
	return s, nil
}

func caseOf(number int, dc *v1.DebugCase) (Case, error) {
	if n := len(dc.GetTasks()); n == 0 || n > maxTasks {
		return Case{}, fmt.Errorf("%d tasks, not 1 to %d", n, maxTasks)
	}
	var tasks []Task
	for i, t := range dc.GetTasks() {
		outcome, ok := outcomes[t.GetOutcome()]
		if !ok {
			return Case{}, fmt.Errorf("task %d: the outcome %v is none of the set's", i+1, t.GetOutcome())
		}
		if t.GetStatement() == "" {
			return Case{}, fmt.Errorf("task %d has no statement", i+1)
		}
		tasks = append(tasks, Task{Statement: t.GetStatement(), Formula: t.GetFormula(), Key: keyOf(t.GetKey()),
			Work: t.GetRead(), Read: t.GetRead(), Outcome: outcome})
	}
	statement, width, height, err := provider.Photo(dc.GetWork().GetStatement(), nil)
	if err != nil {
		return Case{}, fmt.Errorf("the task's crop: %w", err)
	}
	crop := dc.GetWork().GetCrop()
	cfg, format, err := image.DecodeConfig(bytes.NewReader(crop))
	if err != nil || format != "png" {
		return Case{}, fmt.Errorf("the crop of the work is no PNG: %v", err)
	}
	c := Case{Number: number, Part: sample.PartDev, Tasks: tasks,
		pictures: &Pictures{
			Statement: picture{JPEG: statement.JPEG, Width: width, Height: height},
			Work:      picture{PNG: crop, Width: cfg.Width, Height: cfg.Height},
		}}
	one := tasks[0]
	switch {
	case len(tasks) > 1:
		c.Kind = KindSeveral
	case strings.Contains(one.Formula, "="):
		c.Kind = KindEquation
	case one.Formula != "":
		c.Kind = KindExpression
	case one.Key == nil:
		c.Kind = KindOpen
	case one.Key.Choice != "":
		c.Kind = KindChoice
	default:
		c.Kind = KindWord
	}
	return c, nil
}

// Digest is the SHA-256 of a set as a run's log names it: of its file, or of a folder of exported
// cases, each file's name and bytes in the order the set reads them.
func Digest(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		data, err := os.ReadFile(path)
		return fmt.Sprintf("%x", sha256.Sum256(data)), err
	}
	names, err := filepath.Glob(filepath.Join(path, "*.json"))
	if err != nil {
		return "", err
	}
	slices.Sort(names)
	h := sha256.New()
	for _, name := range names {
		data, err := os.ReadFile(name)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%s\x00%d\x00", filepath.Base(name), len(data))
		h.Write(data)
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
