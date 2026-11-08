package admission

import (
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"path"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/altnovatrail/spec"
	"github.com/altnovatrail/spec/types/admissionv0"
	"gopkg.in/yaml.v3"
)

// specVersion is the version of the table in the release of spec that go.mod requires.
func specVersion(t *testing.T) string {
	t.Helper()
	v, err := fs.ReadFile(spec.Admission, "VERSION")
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(v))
}

// files gives the files of spec.Admission that match the pattern, failing on none: a vector that
// is not found is a vector skipped.
func files(t *testing.T, pattern string) []string {
	t.Helper()
	found, err := fs.Glob(spec.Admission, pattern)
	if err != nil || len(found) == 0 {
		t.Fatalf("no files %s in spec.Admission: %v", pattern, err)
	}
	return found
}

// asJSON gives a value read from YAML as JSON, which ReadTable and ReadFacts read as they read YAML.
func asJSON(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestAnswerVectors(t *testing.T) {
	version := specVersion(t)
	cases := 0
	for _, name := range files(t, "vectors/answers/*.yaml") {
		data, err := fs.ReadFile(spec.Admission, name)
		if err != nil {
			t.Fatal(err)
		}
		var vector struct {
			Table any `yaml:"table"`
			Cases []struct {
				Request any `yaml:"request"`
				Facts   any `yaml:"facts"`
				Answer  any `yaml:"answer"`
			} `yaml:"cases"`
		}
		if err := yaml.Unmarshal(data, &vector); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		table, err := ReadTable(asJSON(t, vector.Table))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for i, c := range vector.Cases {
			var req Request
			var want admissionv0.Answer
			if err := json.Unmarshal(asJSON(t, c.Request), &req); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(asJSON(t, c.Answer), &want); err != nil {
				t.Fatal(err)
			}
			var facts Facts
			if c.Facts != nil {
				if facts, err = ReadFacts(table, asJSON(t, c.Facts)); err != nil {
					t.Fatalf("%s, case %d: %v", name, i, err)
				}
			}
			got := table.Admit(req, facts)
			if !reflect.DeepEqual(got.Answer, want) || got.Version != version {
				t.Errorf("%s, case %d: got %s, want %s of version %s", name, i, asJSON(t, got), asJSON(t, want), version)
			}
			cases++
		}
	}
	t.Logf("%d cases", cases)
}

func TestRuleVectors(t *testing.T) {
	seen := map[string]bool{}
	for _, name := range files(t, "vectors/rules/*/*.yaml") {
		seen[path.Base(path.Dir(name))] = true
		data, err := fs.ReadFile(spec.Admission, name)
		if err != nil {
			t.Fatal(err)
		}
		_, err = ReadTable(data)
		var rule *RuleError
		if !errors.As(err, &rule) {
			t.Errorf("%s: got %v, want a breach of a rule", name, err)
			continue
		}
		if want := path.Base(path.Dir(name)); rule.Rule != want {
			t.Errorf("%s: breaks %s, want %s", name, rule.Rule, want)
		}
	}
	// Every rule of section 7 has its vectors: a rule with none would go untested.
	for _, rule := range []string{RuleUniqueCells, RuleUniqueCapabilities, RuleKnownProviders, RuleKnownFacts} {
		if !seen[rule] {
			t.Errorf("no vector of %s", rule)
		}
	}
}

func TestExamples(t *testing.T) {
	for _, name := range files(t, "examples/admission-table/valid/*.yaml") {
		data, err := fs.ReadFile(spec.Admission, name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ReadTable(data); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	for _, name := range files(t, "examples/admission-table/invalid/*.yaml") {
		data, err := fs.ReadFile(spec.Admission, name)
		if err != nil {
			t.Fatal(err)
		}
		// The first line names the error: # error: <keyword> at <location>.
		first, _, _ := strings.Cut(string(data), "\n")
		_, at, ok := strings.Cut(first, " at #")
		if !ok {
			t.Fatalf("%s: no error named on its first line", name)
		}
		_, err = ReadTable(data)
		var rule *RuleError
		if err == nil || errors.As(err, &rule) {
			t.Errorf("%s: got %v, want a table that does not follow its schema", name, err)
			continue
		}
		if !strings.Contains(err.Error(), "'"+at+"'") {
			t.Errorf("%s: the error does not name %s: %v", name, at, err)
		}
	}
}

func TestPlatform(t *testing.T) {
	table, err := Platform()
	if err != nil {
		t.Fatal(err)
	}
	if table.Version() != specVersion(t) {
		t.Errorf("version %s, want %s", table.Version(), specVersion(t))
	}
	adult := func(p admissionv0.Provider, route admissionv0.AiRoute) Request {
		return Request{Capability: admissionv0.CapabilityHandwritingReading, AiRoute: route, Provider: p, AgeCategory: admissionv0.AgeCategoryAdult}
	}
	// The own key of an adult needs no fact; platform AI needs its facts; Google is closed (R47
	// spec). These are what the first app meets (R76 spec): if the table changes them, this test
	// says so before api does.
	if a := table.Admit(adult("anthropic", admissionv0.AiRouteOwnKey), Facts{}); !a.Admitted {
		t.Errorf("anthropic, own key, adult: %s", asJSON(t, a))
	}
	if a := table.Admit(adult("anthropic", admissionv0.AiRoutePlatformAi), Facts{}); a.Admitted || *a.Reason != admissionv0.AnswerReasonFactMissing {
		t.Errorf("anthropic, platform AI, adult, no facts: %s", asJSON(t, a))
	}
	if a := table.Admit(adult("google", admissionv0.AiRouteOwnKey), Facts{}); a.Admitted || *a.Reason != admissionv0.AnswerReasonCellClosed {
		t.Errorf("google, own key, adult: %s", asJSON(t, a))
	}
	facts, err := ReadFacts(table, []byte("anthropic:\n  data_processing_agreement: [adult]\n  minimal_retention: [adult]\n"))
	if err != nil {
		t.Fatal(err)
	}
	if a := table.Admit(adult("anthropic", admissionv0.AiRoutePlatformAi), facts); !a.Admitted {
		t.Errorf("anthropic, platform AI, adult, with its facts: %s", asJSON(t, a))
	}
}

func TestClosedByDefault(t *testing.T) {
	table, err := Platform()
	if err != nil {
		t.Fatal(err)
	}
	open := Request{Capability: admissionv0.CapabilityHandwritingReading, AiRoute: admissionv0.AiRouteOwnKey, Provider: "anthropic", AgeCategory: admissionv0.AgeCategoryAdult}
	unknown := map[string]func(*Request){
		"capability": func(r *Request) { r.Capability = "Handwriting_Reading" },
		"category":   func(r *Request) { r.AgeCategory = "over_18" },
		"route":      func(r *Request) { r.AiRoute = "platform" },
		"provider":   func(r *Request) { r.Provider = "Anthropic" },
		"nothing":    func(r *Request) { *r = Request{} },
	}
	for name, change := range unknown {
		r := open
		change(&r)
		a := table.Admit(r, Facts{})
		if a.Admitted || a.Reason == nil || a.Cell != nil || a.CapabilityRule != nil || a.Fact != nil {
			t.Errorf("unknown %s: %s", name, asJSON(t, a))
		}
	}
}

func TestAnswerJSON(t *testing.T) {
	table, err := Platform()
	if err != nil {
		t.Fatal(err)
	}
	a := table.Admit(Request{Capability: admissionv0.CapabilityHint, AiRoute: admissionv0.AiRoutePlatformAi, Provider: "openai", AgeCategory: admissionv0.AgeCategoryUnder13}, Facts{})
	want := `{"admitted":false,"reason":"fact_missing","cell":{"provider":"openai","ai_route":"platform_ai","age_category":"under_13"},"fact":"data_processing_agreement","version":"` + table.Version() + `"}`
	if got := string(asJSON(t, a)); got != want {
		t.Errorf("got %s\nwant %s", got, want)
	}
}

func TestReadFacts(t *testing.T) {
	table, err := Platform()
	if err != nil {
		t.Fatal(err)
	}
	empty := []string{"", "# nothing declared yet\n", "{}", "~\n", "---\n", "---\n---\n"}
	for _, file := range empty {
		facts, err := ReadFacts(table, []byte(file))
		if err != nil || len(facts.declared) != 0 {
			t.Errorf("%q: %v, %v", file, facts, err)
		}
	}
	refused := map[string]string{
		"unknown provider":           "antropic:\n  data_processing_agreement: [adult]\n",
		"unknown fact":               "anthropic:\n  data_processing_agrement: [adult]\n",
		"unknown category":           "anthropic:\n  data_processing_agreement: [grown_up]\n",
		"no category":                "anthropic:\n  data_processing_agreement: []\n",
		"a category twice":           "anthropic:\n  data_processing_agreement: [adult, adult]\n",
		"a key twice":                "anthropic:\n  data_processing_agreement: [adult]\n  data_processing_agreement: [under_13]\n",
		"a list":                     "- anthropic\n",
		"two documents":              "anthropic:\n  minimal_retention: [adult]\n---\nopenai:\n  minimal_retention: [adult]\n",
		"no YAML":                    "anthropic: [\n",
		"a key that is not a string": "1:\n  minimal_retention: [adult]\n",
		"a merge key":                "anthropic:\n  <<: {minimal_retention: [adult]}\n  minimal_retention: [under_13]\n",
		"an alias":                   "anthropic:\n  minimal_retention: &both [adult]\n  data_processing_agreement: *both\n",
		"no YAML after the first":    "anthropic:\n  minimal_retention: [adult]\n---\nopenai: [\n",
	}
	for name, file := range refused {
		if _, err := ReadFacts(table, []byte(file)); err == nil {
			t.Errorf("%s: read", name)
		}
	}
}

func TestReadTable(t *testing.T) {
	data, err := fs.ReadFile(spec.Admission, "admission-table.json")
	if err != nil {
		t.Fatal(err)
	}
	refused := map[string]string{
		"empty":         "",
		"no YAML":       "providers: [\n",
		"a date":        "providers: {a: {name: A}}\nfacts: {f: {description: F}}\ncapabilities: [{capability: hint, age_category: adult, admission: open, basis: [R47]}]\ncells: [{provider: a, ai_route: own_key, age_category: adult, admission: open, terms: [{row: OpenAI, read: 2026-11-03}], basis: [R47]}]\n",
		"two documents": string(data) + "\n---\n{}\n",
		"a merge key":   "providers: {a: {name: A}}\nfacts: {f: {description: F}}\ncapabilities: [{<<: {capability: hint, age_category: adult}, admission: open, basis: [R47]}]\ncells: [{provider: a, ai_route: own_key, age_category: adult, admission: open, terms: [{row: OpenAI, read: \"2026-11-03\"}], basis: [R47]}]\n",
	}
	for name, file := range refused {
		if _, err := ReadTable([]byte(file)); err == nil {
			t.Errorf("%s: read", name)
		}
	}
	if err := (&RuleError{Rule: RuleKnownFacts, Detail: "x"}).Error(); !strings.Contains(err, RuleKnownFacts) {
		t.Errorf("the error %q does not name its rule", err)
	}
}

func TestCompile(t *testing.T) {
	good := fstest.MapFS{}
	for _, name := range []string{"VERSION", "schemas/admission-table.schema.json", "schemas/answer-vector.schema.json"} {
		data, err := fs.ReadFile(spec.Admission, name)
		if err != nil {
			t.Fatal(err)
		}
		good[name] = &fstest.MapFile{Data: data}
	}
	if c, err := compile(good); err != nil || c.version != specVersion(t) {
		t.Fatalf("the schemas of spec: %v, version %q", err, c.version)
	}
	broken := map[string]func(fstest.MapFS){
		"no VERSION": func(f fstest.MapFS) { delete(f, "VERSION") },
		"no schema":  func(f fstest.MapFS) { delete(f, "schemas/answer-vector.schema.json") },
		"no JSON":    func(f fstest.MapFS) { f["schemas/admission-table.schema.json"] = &fstest.MapFile{Data: []byte("{")} },
		"no $id": func(f fstest.MapFS) {
			f["schemas/admission-table.schema.json"] = &fstest.MapFile{Data: []byte(`{"type": "object"}`)}
		},
		"the same $id": func(f fstest.MapFS) {
			f["schemas/answer-vector.schema.json"] = f["schemas/admission-table.schema.json"]
		},
		"a table schema that does not compile": func(f fstest.MapFS) {
			f["schemas/admission-table.schema.json"] = &fstest.MapFile{Data: []byte(`{"$id": "https://example.org/a/admission-table.schema.json", "type": 1}`)}
		},
		"no declared_facts": func(f fstest.MapFS) {
			f["schemas/answer-vector.schema.json"] = &fstest.MapFile{Data: []byte(`{"$id": "https://example.org/a/answer-vector.schema.json"}`)}
		},
	}
	for name, breakIt := range broken {
		f := maps.Clone(good)
		breakIt(f)
		if _, err := compile(f); err == nil {
			t.Errorf("%s: compiled", name)
		}
	}
}
