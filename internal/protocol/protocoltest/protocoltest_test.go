package protocoltest

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	// The module of spec's contracts goes into the build information only when the test links it.
	_ "github.com/altnovatrail/spec/contracts"
)

// Examples finds spec's seven examples of the eval protocol where Go keeps the module: in the
// folder GOMODCACHE names, or else under the first folder of GOPATH, or of its default.
func TestExamples(t *testing.T) {
	cache := moduleCache()
	gopath := filepath.Dir(filepath.Dir(cache))
	for _, c := range []struct{ name, modcache, gopath string }{
		{"GOMODCACHE", cache, ""},
		{"GOPATH", "", gopath + string(os.PathListSeparator) + t.TempDir()},
		{"default", "", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("GOMODCACHE", c.modcache)
			t.Setenv("GOPATH", c.gopath)
			if moduleCache() != cache {
				t.Skipf("the module cache %s is not under %s", cache, c.name)
			}
			names, err := fs.Glob(Examples(t), "*.json")
			if err != nil {
				t.Fatal(err)
			}
			if len(names) != 7 {
				t.Errorf("%d examples, want spec's 7: %v", len(names), names)
			}
		})
	}
}

// A variable that go env -w set is read from Go's file of settings when the environment has none,
// and GOENV=off reads no file.
func TestGoEnv(t *testing.T) {
	file := filepath.Join(t.TempDir(), "env")
	if err := os.WriteFile(file, []byte("GOFLAGS=-mod=mod\nGOMODCACHE=/data/gomod\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOMODCACHE", "")
	t.Setenv("GOPATH", "")
	t.Setenv("GOENV", file)
	if got := goEnv("GOMODCACHE"); got != "/data/gomod" {
		t.Errorf("GOMODCACHE from the file = %q", got)
	}
	if got := goEnv("GOPATH"); got != "" {
		t.Errorf("GOPATH from the file = %q", got)
	}
	t.Setenv("GOMODCACHE", "/env/gomod")
	if got := goEnv("GOMODCACHE"); got != "/env/gomod" {
		t.Errorf("GOMODCACHE from the environment = %q", got)
	}
	t.Setenv("GOMODCACHE", "")
	t.Setenv("GOENV", "off")
	if got := goEnv("GOMODCACHE"); got != "" {
		t.Errorf("GOMODCACHE with GOENV=off = %q", got)
	}
	t.Setenv("GOENV", filepath.Join(t.TempDir(), "none"))
	if got := goEnv("GOMODCACHE"); got != "" {
		t.Errorf("GOMODCACHE from no file = %q", got)
	}
}
