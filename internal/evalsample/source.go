package evalsample

import (
	"os"
	"path"
	"path/filepath"

	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"
)

// Source is a file of tasks as a sample names it, the source file of its task reference (R69): its
// path in the repository, from the root, with slashes, and the digest of the file, "sha256:" and 64
// lowercase hexadecimal digits. A bank of sheets, the workbook and the essays are named alike.
type Source struct {
	Path   string
	Digest string
}

// SourceOf is the source of the bytes data at the path given, from the repository's root.
func SourceOf(at string, data []byte) Source {
	return Source{Path: path.Clean(at), Digest: Digest(data)}
}

// ReadSource reads a file of tasks and gives its path as a sample names it, from the root of the
// repository that holds it: the nearest folder above it with a go.mod, tutor's root. A file outside
// any is named by its name alone, wherever it is read from, and nothing of the folders above it goes
// into a sample; its digest tells two files of one name apart.
func ReadSource(file string) (string, []byte, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return "", nil, err
	}
	at := filepath.Base(file)
	if abs, err := filepath.Abs(file); err == nil {
		for dir := filepath.Dir(abs); ; dir = filepath.Dir(dir) {
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				if rel, err := filepath.Rel(dir, abs); err == nil {
					at = filepath.ToSlash(rel)
				}
				break
			}
			if filepath.Dir(dir) == dir {
				break
			}
		}
	}
	return at, data, nil
}

// reference is the task reference a sample of the source gives a task of it: the source file, the
// task's ID and the digest of its open view.
func (s Source) reference(task, digest string) *evalv1.TaskReference {
	return &evalv1.TaskReference{
		Source:         &evalv1.TaskReference_File{File: &evalv1.TaskFileSource{FileDigest: s.Digest, Bank: s.Path, BankDigest: s.Digest}},
		Task:           task,
		OpenViewDigest: digest,
	}
}

// refuse refuses a sample whose task is not of the source, as section 13 has it: another source
// than a file of tasks, a file of another path, or of another digest.
func (s Source) refuse(ref *evalv1.TaskReference) *Refusal {
	src := ref.GetFile()
	switch {
	case src == nil:
		return refuse(evalv1.Refusal_REFUSAL_OTHER_TASK, "its task is not of a file of tasks, and its set's is %s", s.Path)
	case src.GetBank() != s.Path:
		return refuse(evalv1.Refusal_REFUSAL_OTHER_TASK, "its task is of the file %s, and its set's is %s", src.GetBank(), s.Path)
	case src.GetBankDigest() != s.Digest || src.GetFileDigest() != s.Digest:
		return refuse(evalv1.Refusal_REFUSAL_OTHER_TASK, "its file of tasks is of another digest than %s's", s.Path)
	}
	return nil
}
