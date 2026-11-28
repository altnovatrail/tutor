package tutor

import (
	"errors"
	"slices"

	"github.com/altnovatrail/tutor/admission"
)

// Debug is the debug mode, in a call's Route: the model that serves the calls of a task from
// paper in place of those of the list, and a record of each call of a model, of any kind. api puts
// it in the route of an account with the right to the mode alone, on a local run or on the
// deployment; the library takes it only when Config.Debug is on. Outside a task from paper it
// chooses nothing: the call keeps its models of the list and the admission's refusal, and the
// mode only records it.
type Debug struct {
	// First serves every call of a task from paper — CallFinding, CallStatement, CallChecking, the
	// tutor's words, CallPaperHint, and the drawing of grown-up mode, CallDrawing —, one attempt
	// with no later one; nil leaves each its models of the list.
	First *admission.Model
	// Record gets each call of a model of the call, of every kind, once the models have answered,
	// in their order, from the goroutine of the call.
	Record func(DebugCall) `json:"-"`
}

// DebugCall is one call of a model in the debug mode: the kind of the library's call; the model's
// place among the attempts of its call, 1, 2 or 3; of a reading of a sheet or a workbook page, the
// reader's place among the models it asks, 1 or 2, and 0 for any other kind; the model as it was asked, its effort and thinking with it; what the model
// returned, as it returned it; its usage when it came back with tokens; and what failed. The answer
// is the model's reading of the learner's work: it goes to the account that set the mode alone,
// never to a log.
type DebugCall struct {
	Call    Call
	Attempt int
	Reader  int
	Model   admission.Model
	Answer  string
	Usage   *Usage
	Err     error
}

// errDebugOff is a route with the debug mode to a library whose config has it off.
var errDebugOff = errors.New("tutor: the route has the debug mode, and the config has it off")

// chosen tells whether the debug mode chooses the models of a kind of call: the calls of a task
// from paper. It records every kind.
func chosen(c Call) bool {
	switch c {
	case CallFinding, CallStatement, CallChecking, CallPaperHint, CallDrawing:
		return true
	}
	return false
}

// models are the models of the debug mode for a kind of call in place of those the call asks: the
// first, or else those of the list.
func (d *Debug) models(asked []admission.Model) []admission.Model {
	if d.First != nil {
		return []admission.Model{*d.First}
	}
	return slices.Clone(asked)
}

// chose tells whether the debug mode chose the model of a call. Then the call asks it, and no
// cascade of the list.
func (d *Debug) chose() bool {
	return d != nil && d.First != nil
}

// choiceOf is the debug mode of a route where it chooses the models of a kind of call, or nil.
func choiceOf(c Call, r Route) *Debug {
	if !chosen(c) {
		return nil
	}
	return r.Debug
}
