package evalsample_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	mkview "github.com/altnovatrail/module-kit/openview"
	"github.com/altnovatrail/module-kit/template"
	"github.com/altnovatrail/spec/types/formatv0"
	"gopkg.in/yaml.v3"
)

// vectors is the folder of the format's vectors in the spec submodule.
const vectors = "../../spec/format/vectors"

// readVector reads a YAML file of the format's vectors into v, by way of JSON, as the types of the
// format name their fields.
func readVector(t *testing.T, path string, v any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	js, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	if err := json.Unmarshal(js, v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

// The digest of the open view by which the runner finds a sample's task (R68 spec) is the one spec's
// vectors give (R87; Д108 of spec's RUN.md): of each task of the vectors of the open view, and of
// each instance of a template that an attempt of the vectors of templates names, with its numbers
// in place.
func TestOpenViewDigestVectors(t *testing.T) {
	cases, err := filepath.Glob(filepath.Join(vectors, "open-view/task/*/digest.yaml"))
	if err != nil || len(cases) == 0 {
		t.Fatalf("no vector of the digest of an open view: %v", err)
	}
	for _, f := range cases {
		var want struct {
			Digest formatv0.Digest `json:"open_view_digest"`
		}
		readVector(t, f, &want)
		var task formatv0.Task
		readVector(t, filepath.Join(filepath.Dir(f), "part.yaml"), &task)
		if got, err := mkview.TaskDigest(task); err != nil || got != want.Digest {
			t.Errorf("%s: the digest is %s, %v; the vector gives %s", filepath.Base(filepath.Dir(f)), got, err, want.Digest)
		}
	}
	templates, err := filepath.Glob(filepath.Join(vectors, "templates/*.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	instances := 0
	for _, f := range templates {
		var vector formatv0.TemplateVector
		readVector(t, f, &vector)
		for _, a := range vector.Attempts {
			if a.OpenViewDigest == "" {
				continue
			}
			instances++
			instance, err := template.Instance(vector.Task, a.Numbers)
			if err != nil {
				t.Errorf("%s, seed %s: %v", filepath.Base(f), a.Seed, err)
				continue
			}
			if got, err := mkview.TaskDigest(instance); err != nil || got != a.OpenViewDigest {
				t.Errorf("%s, seed %s: the digest is %s, %v; the vector gives %s", filepath.Base(f), a.Seed, got, err, a.OpenViewDigest)
			}
		}
	}
	if instances == 0 {
		t.Error("no attempt of the vectors of templates names the digest of its instance")
	}
}
