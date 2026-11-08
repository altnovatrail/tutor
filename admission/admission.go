// Package admission is tutor's admission function (R57; architecture 3.9): one pure function that
// says whether the work of a learner of an age category may go to an AI provider on an AI route,
// for a capability of AI, by spec's admission table and the deployment facts an operator declares.
// api calls it before an attempt, and tutor before each call of a model that carries a learner's
// work. It does no input or output, reads no clock and keeps nothing, so that the same table, facts
// and request give the same answer anywhere.
//
// The table is spec's (R96 spec): Platform reads it from the Go module of spec at the tag go.mod
// requires, as spec.Admission, and ReadTable reads any other of the same version, such as one of
// the test vectors. Its rules are the admission text of spec, sections 4 to 7, and the function
// passes every test vector of spec, of answers and of the rules of a table (TestAnswerVectors,
// TestRuleVectors). Whether the model of a call is admitted is no part of the table: tutor checks
// it after the table admits the request.
package admission

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"slices"
	"strings"
	"sync"

	"github.com/altnovatrail/spec"
	"github.com/altnovatrail/spec/types/admissionv0"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

// Request is what the function is asked: a capability of AI, an AI route, a provider and an age
// category, by their codes in the table. It names no identifier, no country and no age in years
// (R06 spec).
type Request = admissionv0.Request

// Answer is the function's answer: admitted, with the cell that admits the request; or refused,
// with the reason, the rule or the cell that refuses it when the table has one, and for
// fact_missing the fact — and the version of the table that gave it. It holds no content and no
// identifier, so it may be written to logs (R09 spec); as JSON it is the answer of a test vector
// with the member version beside it.
type Answer struct {
	admissionv0.Answer
	// Version is the version of the table, MAJOR.MINOR, such as "0.1".
	Version string `json:"version"`
}

// Table is an admission table that follows its schema and breaks no rule of a table. It is not
// changed once read, and Admit may be called from many goroutines at once.
type Table struct {
	version   string
	providers map[admissionv0.Provider]admissionv0.AiProvider
	facts     map[admissionv0.Fact]admissionv0.DeploymentFact
	rules     map[admissionv0.CapabilityRuleKey]admissionv0.CapabilityRuleAdmission
	cells     map[admissionv0.CellKey]admissionv0.Cell
}

// Version is the version of the table, MAJOR.MINOR.
func (t *Table) Version() string { return t.version }

// RuleError is a table that follows its schema but breaks a rule of a table, which Rule names by
// its ID, such as admission-table.unique-cells (the admission text, section 7).
type RuleError struct {
	Rule   string
	Detail string
}

func (e *RuleError) Error() string {
	return "admission: the table breaks the rule " + e.Rule + ": " + e.Detail
}

// The IDs of the rules of a table, in the order of the admission text, section 7.
const (
	RuleUniqueCells        = "admission-table.unique-cells"
	RuleUniqueCapabilities = "admission-table.unique-capabilities"
	RuleKnownProviders     = "admission-table.known-providers"
	RuleKnownFacts         = "admission-table.known-facts"
)

// Platform reads the admission table of spec at the tag go.mod requires: admission-table.json of
// spec.Admission. An error is a fault of the release of spec, not of a deployment.
func Platform() (*Table, error) {
	data, err := fs.ReadFile(spec.Admission, "admission-table.json")
	if err != nil {
		return nil, fmt.Errorf("admission: %w", err)
	}
	return ReadTable(data)
}

// ReadTable reads a table, JSON or YAML, by the schemas of spec.Admission: its version is theirs,
// VERSION of spec.Admission, since a table of another version has other schemas and the package
// knows only these. It refuses a table that does not follow admission-table.schema.json, and a
// table that breaks a rule of a table with a *RuleError that names the first such rule, in the
// order of section 7.
func ReadTable(data []byte) (*Table, error) {
	s, err := schemas()
	if err != nil {
		return nil, err
	}
	var raw admissionv0.AdmissionTable
	if err := read(data, s.table, "the table", &raw); err != nil {
		return nil, err
	}
	t := &Table{
		version:   s.version,
		providers: raw.Providers,
		facts:     raw.Facts,
		rules:     map[admissionv0.CapabilityRuleKey]admissionv0.CapabilityRuleAdmission{},
		cells:     map[admissionv0.CellKey]admissionv0.Cell{},
	}
	for _, c := range raw.Cells {
		k := cellKey(c)
		if _, ok := t.cells[k]; ok {
			return nil, &RuleError{RuleUniqueCells, "two cells of " + cellName(k)}
		}
		t.cells[k] = c
	}
	for _, r := range raw.Capabilities {
		k := admissionv0.CapabilityRuleKey{Capability: r.Capability, AgeCategory: r.AgeCategory}
		if _, ok := t.rules[k]; ok {
			return nil, &RuleError{RuleUniqueCapabilities, fmt.Sprintf("two rules of %s for %s", k.Capability, k.AgeCategory)}
		}
		t.rules[k] = r.Admission
	}
	var unknownFact *RuleError
	for _, c := range raw.Cells {
		if _, ok := t.providers[c.Provider]; !ok {
			return nil, &RuleError{RuleKnownProviders, fmt.Sprintf("the provider %q of the cell of %s is not in providers", c.Provider, cellName(cellKey(c)))}
		}
		for _, f := range c.Facts {
			if _, ok := t.facts[f]; !ok && unknownFact == nil {
				unknownFact = &RuleError{RuleKnownFacts, fmt.Sprintf("the fact %q of the cell of %s is not in facts", f, cellName(cellKey(c)))}
			}
		}
	}
	if unknownFact != nil {
		return nil, unknownFact
	}
	return t, nil
}

// Facts are the deployment facts an operator declares: for each provider, each fact with the age
// categories for which it holds (the admission text, section 5). The zero value declares none, as
// a deployment with no file of facts does: every cell open with facts is then closed, and the own
// key of an adult stays open. Admit reads only the facts its cells name, so a fact no cell names
// can never open anything.
type Facts struct {
	declared admissionv0.DeclaredFacts
}

// ReadFacts reads the file of a deployment's facts, YAML or JSON, as the table names its providers
// and facts: what the vectors of answers hold as facts. A document with nothing in it, comments
// alone, declares none. It refuses a file that does not follow the schema's declared_facts, and a
// provider or a fact the table does not name: a typo would close a route without a word.
func ReadFacts(t *Table, data []byte) (Facts, error) {
	s, err := schemas()
	if err != nil {
		return Facts{}, err
	}
	var declared admissionv0.DeclaredFacts
	if err := read(data, s.facts, "the file of facts", &declared); err != nil {
		if errors.Is(err, errEmpty) {
			return Facts{}, nil
		}
		return Facts{}, err
	}
	// The keys go in order, so that a file with two faults names the same one each time.
	for _, p := range slices.Sorted(maps.Keys(declared)) {
		if _, ok := t.providers[p]; !ok {
			return Facts{}, fmt.Errorf("admission: the file of facts names the provider %q, which the table does not", p)
		}
		for _, f := range slices.Sorted(maps.Keys(declared[p])) {
			if _, ok := t.facts[f]; !ok {
				return Facts{}, fmt.Errorf("admission: the file of facts names, for %q, the fact %q, which the table does not", p, f)
			}
		}
	}
	return Facts{declared: declared}, nil
}

// holds reports whether the fact is declared for the provider and the age category.
func (f Facts) holds(p admissionv0.Provider, fact admissionv0.Fact, c admissionv0.AgeCategory) bool {
	return slices.Contains(f.declared[p][fact], c)
}

// Admit answers the request by the table and the facts, as the admission text, section 6, says:
// the rule of the capability for the age category first, then the cell of the provider, the AI
// route and the age category, then the facts of a cell open with facts, in its order. What the
// table does not name is closed: a capability, a route, a provider or a category it does not know
// is refused, whatever its spelling.
func (t *Table) Admit(r Request, f Facts) Answer {
	a := Answer{Version: t.version}
	rule := admissionv0.CapabilityRuleKey{Capability: r.Capability, AgeCategory: r.AgeCategory}
	admission, ok := t.rules[rule]
	if !ok || admission != admissionv0.CapabilityRuleAdmissionOpen {
		a.Reason = reason(admissionv0.AnswerReasonCapabilityClosed)
		if ok {
			a.CapabilityRule = &rule
		}
		return a
	}
	key := admissionv0.CellKey{Provider: r.Provider, AiRoute: r.AiRoute, AgeCategory: r.AgeCategory}
	cell, ok := t.cells[key]
	if !ok {
		a.Reason = reason(admissionv0.AnswerReasonCellClosed)
		return a
	}
	a.Cell = &key
	switch cell.Admission {
	case admissionv0.CellAdmissionOpen:
		a.Admitted = true
	case admissionv0.CellAdmissionOpenWithFacts:
		for _, fact := range cell.Facts {
			if !f.holds(r.Provider, fact, r.AgeCategory) {
				a.Reason = reason(admissionv0.AnswerReasonFactMissing)
				a.Fact = &fact
				return a
			}
		}
		a.Admitted = true
	default:
		a.Reason = reason(admissionv0.AnswerReasonCellClosed)
	}
	return a
}

func reason(r admissionv0.AnswerReason) *admissionv0.AnswerReason { return &r }

func cellKey(c admissionv0.Cell) admissionv0.CellKey {
	return admissionv0.CellKey{Provider: c.Provider, AiRoute: c.AiRoute, AgeCategory: c.AgeCategory}
}

func cellName(k admissionv0.CellKey) string {
	return fmt.Sprintf("%s, %s, %s", k.Provider, k.AiRoute, k.AgeCategory)
}

// errEmpty is a document with no value in it.
var errEmpty = errors.New("admission: the document is empty")

// read reads one YAML document — JSON is YAML too — checks it against the schema and decodes it
// into v. YAML refuses a key given twice in one mapping, which JSON would let pass. read refuses
// also what would hide a key given twice: an anchor's alias, and the merge key <<, through which a
// key named again would replace the merged one without a word. A document after the first is
// refused unless it is empty, as a file that ends in --- has one.
func read(data []byte, schema *jsonschema.Schema, what string, v any) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return errEmpty
		}
		return fmt.Errorf("admission: %s: %w", what, err)
	}
	for {
		var more yaml.Node
		err := dec.Decode(&more)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("admission: %s: %w", what, err)
		}
		if !empty(&more) {
			return fmt.Errorf("admission: %s: more than one YAML document", what)
		}
	}
	if empty(&doc) {
		return errEmpty
	}
	if err := plain(&doc); err != nil {
		return fmt.Errorf("admission: %s: %w", what, err)
	}
	var value any
	if err := doc.Decode(&value); err != nil {
		return fmt.Errorf("admission: %s: %w", what, err)
	}
	// A value of YAML goes to the schema as JSON gives it, so that a timestamp or a key that is no
	// string fails here rather than passing as something JSON cannot hold.
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("admission: %s: %w", what, err)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("admission: %s: %w", what, err)
	}
	if err := schema.Validate(inst); err != nil {
		return fmt.Errorf("admission: %s does not follow its schema: %w", what, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("admission: %s: %w", what, err)
	}
	return nil
}

// empty reports whether a document holds no value: nothing, comments alone, or a bare null.
func empty(doc *yaml.Node) bool {
	if len(doc.Content) == 0 {
		return true
	}
	n := doc.Content[0]
	return n.Kind == yaml.ScalarNode && n.Tag == "!!null"
}

// plain refuses an alias of an anchor and the merge key << anywhere in a node.
func plain(n *yaml.Node) error {
	if n.Kind == yaml.AliasNode {
		return fmt.Errorf("line %d: an alias of an anchor", n.Line)
	}
	if n.Tag == "!!merge" {
		return fmt.Errorf("line %d: the merge key <<", n.Line)
	}
	for _, c := range n.Content {
		if err := plain(c); err != nil {
			return err
		}
	}
	return nil
}

// compiled is the schemas of spec.Admission that the package reads by, and their version.
type compiled struct {
	version string
	table   *jsonschema.Schema
	facts   *jsonschema.Schema
}

// schemas is the schemas of spec.Admission, compiled once. A schema that is missing or does not
// compile is a fault of the release of spec, given as an error to every call.
var schemas = sync.OnceValues(func() (compiled, error) { return compile(spec.Admission) })

// compile compiles the schemas of the admission folder of a release of spec, each added by its $id,
// so that a reference between them resolves without loading anything.
func compile(fsys fs.FS) (compiled, error) {
	fail := func(err error) (compiled, error) {
		return compiled{}, fmt.Errorf("admission: the schemas of spec: %w", err)
	}
	version, err := fs.ReadFile(fsys, "VERSION")
	if err != nil {
		return fail(err)
	}
	c := jsonschema.NewCompiler()
	ids := map[string]string{}
	for _, name := range []string{"admission-table.schema.json", "answer-vector.schema.json"} {
		data, err := fs.ReadFile(fsys, "schemas/"+name)
		if err != nil {
			return fail(err)
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
		if err != nil {
			return fail(fmt.Errorf("%s: %w", name, err))
		}
		root, _ := doc.(map[string]any)
		id, _ := root["$id"].(string)
		if id == "" {
			return fail(fmt.Errorf("%s has no $id", name))
		}
		if err := c.AddResource(id, doc); err != nil {
			return fail(fmt.Errorf("%s: %w", name, err))
		}
		ids[name] = id
	}
	table, err := c.Compile(ids["admission-table.schema.json"])
	if err != nil {
		return fail(err)
	}
	facts, err := c.Compile(ids["answer-vector.schema.json"] + "#/$defs/declared_facts")
	if err != nil {
		return fail(err)
	}
	return compiled{version: strings.TrimSpace(string(version)), table: table, facts: facts}, nil
}
