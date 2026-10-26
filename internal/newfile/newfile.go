// Package newfile writes the files tutor's tools keep: each a new file, never one written over.
package newfile

import "os"

// Write writes a file that must not exist yet, with the mode perm. A file it could not finish,
// it removes, so that nothing half-written is left to read.
func Write(path string, data []byte, perm os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(path)
	}
	return err
}
