package newfile_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/altnovatrail/tutor/internal/newfile"
)

func TestWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.json")
	if err := newfile.Write(path, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("the file: %v, %v; want it with the mode given", info, err)
	}
	if err := newfile.Write(path, []byte("two"), 0o600); !errors.Is(err, fs.ErrExist) {
		t.Errorf("a second write = %v, want it refused as existing", err)
	}
	if data, _ := os.ReadFile(path); string(data) != "one" {
		t.Errorf("the file holds %q after a refused write, want the first", data)
	}
	if err := newfile.Write(filepath.Join(t.TempDir(), "no", "a.json"), nil, 0o600); err == nil {
		t.Error("a write into a directory that is not there succeeded")
	}
}
