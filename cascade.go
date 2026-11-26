package tutor

import (
	"context"
	"time"
)

// cascadeFloor is the least time a later attempt of a cascade needs before the deadline of its
// call: with less left, it is not begun, since its model would rarely answer in time and the call
// would be billed for nothing.
const cascadeFloor = 10 * time.Second

// cascade asks the models of a call one at a time, in the order of the list, until one does: the
// attempts of a call of a task from paper, the cheapest model first. try asks one model and
// gives what it read, did tells whether that will do — a program's judgement, never the model's.
// A later model is asked only while cascadeFloor is left before the deadline of ctx. A
// provider's refusal for a limit or of the key ends the cascade as its outcome: the next model is of
// the same provider and key. Otherwise the result is that of the last model that answered, though
// it did not do, or, when none answered, the last failure; its usage is that of every attempt, since
// each is billed. A call of one model is one attempt.
func cascade[T any](ctx context.Context, calls []call, try func(call) (T, Outcome), did func(T) bool) (T, Outcome) {
	var (
		zero, best T
		answered   bool
		bestOut    Outcome
		last       Outcome
		usage      []Usage
	)
	for i, c := range calls {
		if i > 0 {
			if !timeFor(ctx) {
				break
			}
			NextModel(ctx, i, len(calls))
		}
		got, out := try(c)
		usage = append(usage, out.Usage...)
		switch out.Kind {
		case LimitReached, KeyRefused:
			out.Usage = usage
			return zero, out
		case Answered:
			best, bestOut, answered = got, out, true
			if did(got) {
				bestOut.Usage = usage
				return best, bestOut
			}
		default:
			last = out
		}
	}
	if !answered {
		last.Usage = usage
		return zero, last
	}
	bestOut.Usage = usage
	return best, bestOut
}

type nextModelKey struct{}

// WithNextModel gives ctx a hook that a cascade of a call made with it calls before it asks a later
// model: model is the place of that model in the call's list, from 0, and models the list's length.
// The hook runs on the call's own goroutine and holds the cascade until it returns, so it should be
// quick and must not fail the call.
func WithNextModel(ctx context.Context, next func(model, models int)) context.Context {
	return context.WithValue(ctx, nextModelKey{}, next)
}

// NextModel tells the hook WithNextModel gave ctx, if any, that a call goes on to the model at
// place model of a list of models: what a cascade does, and a stand-in for tutor, as a test's or
// the stub's, may do too.
func NextModel(ctx context.Context, model, models int) {
	if next, ok := ctx.Value(nextModelKey{}).(func(int, int)); ok {
		next(model, models)
	}
}

// timeFor tells whether a later attempt may begin: ctx is not done, and cascadeFloor is left before
// its deadline, if it has one.
func timeFor(ctx context.Context) bool {
	if ctx.Err() != nil {
		return false
	}
	d, ok := ctx.Deadline()
	return !ok || time.Until(d) >= cascadeFloor
}
