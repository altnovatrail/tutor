package evalsample

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"

	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"

	"github.com/altnovatrail/tutor/internal/sample"
)

// OpenTasks is the file of open tasks in tutor's tree, which a release of tutor carries as
// tutor_<version>_open-tasks.json, byte for byte (R64): a sample that names a task of a release
// names it by that file's digest.
const OpenTasks = "eval/open-tasks.json"

// ErrOtherFile is the error of a sample whose task's file at its tag is neither a bank of sheets nor
// the workbook: the essays, or a file of no tasks, whose samples the check of canvas's synthetic
// samples does not read.
var ErrOtherFile = errors.New("evalsample: the file of its task is neither a bank of sheets nor the workbook, and the runner reads samples of tutor's releases of only those")

// tagPattern is the form of the tag of a release of tutor: a version of three numbers, which names
// a folder of the trees and nothing above it.
var tagPattern = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)

// Releases are the files of tasks of tutor's releases, as the samples of a set of a capability name
// them: each tag's tree of tutor under Dir, Dir/<tag>/eval/…, as git archive writes it. A sample of
// a release is read with the bank it names at the tag it names, not with the bank of the work tree,
// so a bank changed since that release takes nothing from the samples made on it (R76).
type Releases struct {
	Dir   string
	files map[[2]string]any
	open  map[string]string
}

// NewReleases are the trees of tutor's tags under dir.
func NewReleases(dir string) *Releases {
	return &Releases{Dir: dir, files: map[[2]string]any{}, open: map[string]string{}}
}

// Of is the file of tasks that a sample's task reference names, at its tag: a bank of sheets or the
// workbook, whose Source holds the tag and the digest of the tree's file of open tasks, so that the
// file refuses a sample of another release or digest (Source.refuse). A reference to no release of
// tutor, a tag of no version, or a file the tree does not hold, is refused as another task; a file
// of another kind is ErrOtherFile. The error is a tree that is not there, which the recipe that
// fetches the samples writes, or a file of the tree that cannot be read.
func (rs *Releases) Of(ref *evalv1.TaskReference) (*Bank, *Workbook, *Refusal, error) {
	src := ref.GetTutor()
	switch {
	case src == nil:
		return nil, nil, refuse(evalv1.Refusal_REFUSAL_OTHER_TASK, "its task is of no release of tutor"), nil
	case !tagPattern.MatchString(src.GetTag()):
		return nil, nil, refuse(evalv1.Refusal_REFUSAL_OTHER_TASK, "its task is of the tag %q, which is no release of tutor", src.GetTag()), nil
	case !fs.ValidPath(src.GetBank()) || src.GetBank() == ".":
		return nil, nil, refuse(evalv1.Refusal_REFUSAL_OTHER_TASK, "its task is of %q, which is no file of tutor's tree", src.GetBank()), nil
	}
	tag, at := src.GetTag(), src.GetBank()
	open, err := rs.openDigest(tag)
	if err != nil {
		return nil, nil, nil, err
	}
	key := [2]string{tag, at}
	f, ok := rs.files[key]
	if !ok {
		data, err := rs.read(tag, at)
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil, refuse(evalv1.Refusal_REFUSAL_OTHER_TASK, "tutor %s has no file %s", tag, at), nil
		}
		if err != nil {
			return nil, nil, nil, err
		}
		if f, err = fileOf(at, data); err != nil {
			return nil, nil, nil, fmt.Errorf("evalsample: %s of tutor %s: %w", at, tag, err)
		}
		switch f := f.(type) {
		case *Bank:
			f.Tag, f.Open = tag, open
		case *Workbook:
			f.Tag, f.Open = tag, open
		}
		rs.files[key] = f
	}
	switch f := f.(type) {
	case *Bank:
		return f, nil, nil, nil
	case *Workbook:
		return nil, f, nil, nil
	}
	return nil, nil, nil, ErrOtherFile
}

// openDigest is the digest of the file of open tasks of a tag's tree, which must be there.
func (rs *Releases) openDigest(tag string) (string, error) {
	if d, ok := rs.open[tag]; ok {
		return d, nil
	}
	data, err := rs.read(tag, OpenTasks)
	if err != nil {
		return "", fmt.Errorf("evalsample: tutor's tree at %s is not in %s, or its %s cannot be read: just canvas-fetch writes it with git archive: %w", tag, rs.Dir, OpenTasks, err)
	}
	rs.open[tag] = Digest(data)
	return rs.open[tag], nil
}

// read reads a file of a tag's tree, within the tree: a link out of it is not followed.
func (rs *Releases) read(tag, at string) ([]byte, error) {
	root, err := os.OpenRoot(filepath.Join(rs.Dir, tag))
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	return root.ReadFile(at)
}

// fileOf is the file of tasks at the path given, by the format it declares: a *Bank or a *Workbook,
// or nil for a file of another format.
func fileOf(at string, data []byte) (any, error) {
	var head struct {
		Format string `json:"format"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return nil, err
	}
	switch head.Format {
	case sample.BankFormat:
		return BankOf(at, data)
	case sample.WorkbookFormat:
		return WorkbookOf(at, data)
	}
	return nil, nil
}
