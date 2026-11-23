package tutor

import (
	"errors"
	"slices"

	"github.com/altnovatrail/tutor/admission"
)

// Debug is the debug mode, in a call's Route (R163 spec; R136): the models that serve the calls of
// a task from paper in place of those of the list, and a record of each call of a model. api puts
// it in the route of an account with the right to the mode alone, on a local run or on the
// deployment; the library takes it only when Config.Debug is on, and on any other call it changes
// nothing.
type Debug struct {
	// First serves every call of a task from paper — CallFinding, CallStatement, CallSolving,
	// CallPaper and the tutor's words, CallPaperWords and CallPaperHint —; nil leaves each its
	// models of the list.
	First *admission.Model
	// Second reads the work beside the first reader, CallPaper alone, the two read as a pair is;
	// nil leaves one reader.
	Second *admission.Model
	// Record gets each call of a model of the call, once the models have answered, in their order,
	// from the goroutine of the call.
	Record func(DebugCall) `json:"-"`
}

// DebugCall is one call of a model in the debug mode: the kind of the library's call; the reader of
// the work, 1 or 2, and 0 for any other call; the model's place among the attempts of its call, 1,
// 2 or 3 (R137); the model as it was asked, its effort and thinking with it; what the model
// returned, as it returned it; its usage when it came back with tokens; and what failed. The answer
// is the model's reading of the learner's work: it goes to the account that set the mode alone,
// never to a log (R09 spec).
type DebugCall struct {
	Call    Call
	Reader  int
	Attempt int
	Model   admission.Model
	Answer  string
	Usage   *Usage
	Err     error
}

// errDebugOff is a route with the debug mode to a library whose config has it off.
var errDebugOff = errors.New("tutor: the route has the debug mode, and the config has it off")

// debugged tells whether the debug mode serves a kind of call: the calls of a task from paper.
func debugged(c Call) bool {
	switch c {
	case CallFinding, CallStatement, CallSolving, CallPaper, CallPaperWords, CallPaperHint:
		return true
	}
	return false
}

// models are the models of the debug mode for a kind of call in place of those the call asks: the
// first, or else the first of the list's, and beside the reading of the work the second.
func (d *Debug) models(c Call, asked []admission.Model) []admission.Model {
	models := slices.Clone(asked)
	if d.First != nil {
		models = []admission.Model{*d.First}
	}
	if c == CallPaper && d.Second != nil && len(models) > 0 {
		models = append(models[:1], *d.Second)
	}
	return models
}

// chose tells whether the debug mode chose the models of a kind of call: the first model, or the
// second reader of the work. Then the call asks them, and no cascade of the list (R137).
func (d *Debug) chose(c Call) bool {
	return d != nil && (d.First != nil || c == CallPaper && d.Second != nil)
}

// debugOf is the debug mode of a route for a kind of call, or nil where it serves none.
func debugOf(c Call, r Route) *Debug {
	if r.Debug == nil || !debugged(c) {
		return nil
	}
	return r.Debug
}
