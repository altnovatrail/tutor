package protocol

import (
	"fmt"
	"slices"
	"strconv"

	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"

	"github.com/altnovatrail/tutor/internal/sample"
)

// Folded is a history of writing folded into the strokes the sheet shows at the end (section 12,
// "The history of writing"): every stroke written, in the order of its event, and the IDs of those
// shown at the end and of those written and not shown, each ascending.
type Folded struct {
	Written []*evalv1.Stroke
	Visible []int32
	Hidden  []int32
}

// Fold folds a history by adding and taking off IDs alone, with no rule of the stack of undo: write
// adds its stroke, erase and clear take off the strokes they name, and undo takes off the strokes it
// removed and adds back those it restored. Whether a history keeps the rules of section 11 —
// strokes written once, taken off only while shown — is sample-check's to say; Fold folds any.
func Fold(h *evalv1.History) Folded {
	var f Folded
	shown := map[int32]bool{}
	written := map[int32]bool{}
	for _, e := range h.GetEvents() {
		if w := e.GetWrite(); w != nil {
			if !written[w.GetId()] {
				f.Written = append(f.Written, w)
			}
			written[w.GetId()] = true
			shown[w.GetId()] = true
		}
		for _, id := range takenOff(e) {
			delete(shown, id)
		}
		for _, id := range e.GetUndo().GetRestored() {
			shown[id] = true
		}
	}
	for id := range written {
		if shown[id] {
			f.Visible = append(f.Visible, id)
		} else {
			f.Hidden = append(f.Hidden, id)
		}
	}
	slices.Sort(f.Visible)
	slices.Sort(f.Hidden)
	return f
}

// takenOff are the IDs an event takes off the sheet: an erase's, a clear's, or those an undo removed.
func takenOff(e *evalv1.HistoryEvent) []int32 {
	switch {
	case e.GetErase() != nil:
		return e.GetErase().GetIds()
	case e.GetClear() != nil:
		return e.GetClear().GetIds()
	default:
		return e.GetUndo().GetRemoved()
	}
}

// StrokeID is a stroke's ID of the protocol as tutor's code names a stroke: its number in decimal
// digits, so that the strokes of a sample and of a group keep the order of their IDs when they are
// compared as numbers.
func StrokeID(id int32) string { return strconv.FormatInt(int64(id), 10) }

// StrokeNumber is the stroke's ID of the protocol that StrokeID spells: decimal digits alone, a
// whole number from 1 that an int32 holds.
func StrokeNumber(id string) (int32, error) {
	n, err := strconv.ParseInt(id, 10, 32)
	if err != nil || n < 1 || StrokeID(int32(n)) != id {
		return 0, fmt.Errorf("protocol: %q is no stroke's ID of the protocol", id)
	}
	return int32(n), nil
}

// Ink is a history as tutor's code takes the strokes of a sheet (internal/sample): every stroke
// written, in the order of its event, with its points and their times, and the erasures — at each
// event that took off a stroke the sheet does not show at the end for the last time, those strokes,
// in the event's order. So a stroke the eraser took off and an undo put back is in no erasure, and
// an undo that took off a stroke just written is an erasure of it: the strokes shown and those not
// shown are the fold's, and the moment each went is the one the sheet last lost it. A point has no
// pressure: the protocol holds none (R93 spec).
//
// It gives the fold too, whose IDs say which strokes the sheet shows at the end.
//
// What Ink gives is no sample of tutor-sample v0, and its check does not hold it: a stroke of a
// history may end while another action lands (section 12).
func Ink(h *evalv1.History) ([]sample.Stroke, []sample.Erasure, Folded) {
	f := Fold(h)
	strokes := make([]sample.Stroke, len(f.Written))
	for i, w := range f.Written {
		st := sample.Stroke{ID: StrokeID(w.GetId()), Points: make([]sample.Point, len(w.GetPoints()))}
		for k, p := range w.GetPoints() {
			st.Points[k] = sample.Point{X: p.GetX(), Y: p.GetY(), TMs: int64(p.GetTMs())}
		}
		strokes[i] = st
	}

	// The last event that took each hidden stroke off.
	events := h.GetEvents()
	last := map[int32]int{}
	for n, e := range events {
		for _, id := range takenOff(e) {
			if _, ok := slices.BinarySearch(f.Hidden, id); ok {
				last[id] = n
			}
		}
	}
	var erasures []sample.Erasure
	for n, e := range events {
		var ids []string
		for _, id := range takenOff(e) {
			if at, ok := last[id]; ok && at == n && !slices.Contains(ids, StrokeID(id)) {
				ids = append(ids, StrokeID(id))
			}
		}
		if len(ids) > 0 {
			erasures = append(erasures, sample.Erasure{TMs: int64(e.GetTMs()), Strokes: ids})
		}
	}
	return strokes, erasures, f
}

// Final are the strokes of a final geometry as tutor's code takes the strokes of a sheet: their
// points along each, with no time.
func Final(strokes []*evalv1.FinalStroke) []sample.Stroke {
	out := make([]sample.Stroke, len(strokes))
	for i, st := range strokes {
		s := sample.Stroke{ID: StrokeID(st.GetId()), Points: make([]sample.Point, len(st.GetPoints()))}
		for k, p := range st.GetPoints() {
			s.Points[k] = sample.Point{X: p.GetX(), Y: p.GetY()}
		}
		out[i] = s
	}
	return out
}
