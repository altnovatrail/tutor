package openview

import (
	"encoding/json"
	"io/fs"
	"slices"
	"strings"
	"testing"

	"github.com/altnovatrail/spec"
)

// schema reads a schema of the format from spec's Go module, at the version go.mod requires.
func schema(t *testing.T, name string) map[string]any {
	t.Helper()
	data, err := fs.ReadFile(spec.Format, "schemas/"+name)
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// at is the value at a path of keys in a schema.
func at(t *testing.T, v any, path ...string) any {
	t.Helper()
	for _, key := range path {
		m, ok := v.(map[string]any)
		if !ok {
			t.Fatalf("no %s in the schema", strings.Join(path, "."))
		}
		v = m[key]
	}
	return v
}

func enum(t *testing.T, v any) []string {
	t.Helper()
	list, ok := v.([]any)
	if !ok {
		t.Fatalf("no enum: %v", v)
	}
	var out []string
	for _, s := range list {
		out = append(out, s.(string))
	}
	return out
}

// What Check copies of spec's format is what the format of spec's module says: its version, the
// patterns of a local ID, a language tag and a digest, and the kinds and bindings Check knows. A
// move to a tag of spec that changes one of them fails here (R64 in docs/decisions.md).
func TestPatterns(t *testing.T) {
	version, err := fs.ReadFile(spec.Format, "VERSION")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(version)); got != string(ModuleFormatVersion) {
		t.Errorf("spec's format is %s, and the file names %s", got, ModuleFormatVersion)
	}
	common, task, inventory := schema(t, "common.schema.json"), schema(t, "task.schema.json"), schema(t, "inventory.schema.json")
	for _, c := range []struct {
		name string
		got  string
		want any
	}{
		{"local ID", localID.String(), at(t, common, "$defs", "local_id", "pattern")},
		{"reserved ID", reservedID.String(), at(t, common, "$defs", "local_id", "not", "pattern")},
		{"language tag", languageTag.String(), at(t, common, "$defs", "language_tag", "pattern")},
		{"digest", digest.String(), at(t, inventory, "$defs", "digest", "pattern")},
	} {
		if c.got != c.want {
			t.Errorf("%s: %s here, %v in spec's format", c.name, c.got, c.want)
		}
	}
	if got := at(t, common, "$defs", "local_id", "maxLength"); got != float64(maxID) {
		t.Errorf("a local ID is %v characters at most in spec's format, and %d here", got, maxID)
	}
	for _, c := range []struct {
		name string
		got  []string
		want any
	}{
		{"canvas kinds", []string{"handwriting_sheet", "workbook"}, at(t, common, "$defs", "canvas", "properties", "kind", "enum")},
		{"grids", []string{"squares", "lines", "dots"}, at(t, common, "$defs", "canvas", "properties", "grid", "enum")},
		{"line bindings", []string{"box"}, at(t, task, "$defs", "line_part", "properties", "binding", "enum")},
	} {
		if want := enum(t, c.want); !slices.Equal(c.got, want) {
			t.Errorf("%s: %v here, %v in spec's format", c.name, c.got, want)
		}
	}
	bindings := enum(t, at(t, task, "$defs", "answer_slot", "properties", "binding", "enum"))
	for _, b := range []string{"anywhere", "sheet"} {
		if !slices.Contains(bindings, b) {
			t.Errorf("binding %s is not among spec's, %v", b, bindings)
		}
	}
}
