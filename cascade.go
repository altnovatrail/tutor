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
		if i > 0 && !timeFor(ctx) {
			break
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

// timeFor tells whether a later attempt may begin: ctx is not done, and cascadeFloor is left before
// its deadline, if it has one.
func timeFor(ctx context.Context) bool {
	if ctx.Err() != nil {
		return false
	}
	d, ok := ctx.Deadline()
	return !ok || time.Until(d) >= cascadeFloor
}
