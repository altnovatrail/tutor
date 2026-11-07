// Package protocoltest gives tests spec's examples of the eval protocol: the synthetic sample files
// and reports of contracts/examples/eval/v1/ (contracts/README.md of spec, sections 11 and 14). The
// module of spec's contracts carries them as files of its zip, not embedded, and CI checks tutor
// out without the spec submodule, so they are read where Go keeps the module: in the module cache,
// at the version the build takes.
package protocoltest

import (
	"go/build"
	"io/fs"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
)

// Module is the module of spec's contracts.
const Module = "github.com/altnovatrail/spec/contracts"

// Examples is the folder of the examples of novatrail.eval.v1 in the module of spec's contracts, at
// the version the test's build takes. A test that cannot find them fails: it never passes on none.
func Examples(t testing.TB) fs.FS {
	t.Helper()
	info, ok := debug.ReadBuildInfo()
	if !ok {
		t.Fatal("no build information")
	}
	version := ""
	for _, dep := range info.Deps {
		if dep.Path == Module && dep.Replace == nil {
			version = dep.Version
		}
	}
	if version == "" {
		t.Fatalf("the build takes no %s from the module cache", Module)
	}
	dir := filepath.Join(moduleCache(), filepath.FromSlash(Module)+"@"+version, "examples", "eval", "v1")
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("spec's examples of the eval protocol: %v", err)
	}
	return os.DirFS(dir)
}

// moduleCache is where Go keeps the modules it downloads, as go env GOMODCACHE says without being
// run: GOMODCACHE, or else pkg/mod in the first folder of GOPATH, or of its default, which go/build
// knows — each from the environment, or else from the file go env -w writes. The module's path
// holds no capital letter, so it needs no escaping there.
func moduleCache() string {
	if dir := goEnv("GOMODCACHE"); dir != "" {
		return dir
	}
	gopath := goEnv("GOPATH")
	if gopath == "" {
		gopath = build.Default.GOPATH
	}
	if first, _, ok := strings.Cut(gopath, string(os.PathListSeparator)); ok {
		gopath = first
	}
	return filepath.Join(gopath, "pkg", "mod")
}

// goEnv is a variable of Go as go env reads it: from the environment, or else from Go's file of
// settings, GOENV or env in Go's folder of the user's configuration, unless GOENV is off.
func goEnv(key string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	file := os.Getenv("GOENV")
	if file == "" {
		dir, err := os.UserConfigDir()
		if err != nil {
			return ""
		}
		file = filepath.Join(dir, "go", "env")
	}
	if file == "off" {
		return ""
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(line), "="); ok && strings.TrimSpace(k) == key {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
