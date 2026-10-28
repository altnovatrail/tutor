package runner

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
)

// ReadLog reads a run's log back: every line a record of its kind, the run's first and the end,
// if there is one, last. A last line cut off half written is a run that broke off as it wrote it,
// and the log is read without it. The kind of the run reads its records with read, which is given
// every record but the end, by its kind, in order; ReadLog reads the end itself and gives it, or
// nil when the log stops before the end: a run that broke off.
func ReadLog(r io.Reader, read func(kind string, line []byte) error) (*End, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	var lines [][]byte
	for sc.Scan() {
		lines = append(lines, slices.Clone(sc.Bytes()))
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, errors.New("the log is empty")
	}
	if n := len(lines); n > 1 && !json.Valid(lines[n-1]) {
		lines = lines[:n-1]
	}
	var end *End
	for i, line := range lines {
		n := i + 1
		var kind struct{ Kind string }
		if err := json.Unmarshal(line, &kind); err != nil {
			return nil, fmt.Errorf("line %d of the log is not a record: %w", n, err)
		}
		if (n == 1) != (kind.Kind == "run") || end != nil {
			return nil, fmt.Errorf("line %d of the log is a %q record out of its place", n, kind.Kind)
		}
		var err error
		if kind.Kind == "end" {
			end = &End{}
			err = json.Unmarshal(line, end)
		} else {
			err = read(kind.Kind, line)
		}
		if err != nil {
			return nil, fmt.Errorf("line %d of the log: %w", n, err)
		}
	}
	return end, nil
}

// CallKind is the kind of call a run's log is of, as the record of the run, its first line, tells
// it: "" for a log that tells none, which is the judge's, as the logs of E2 and T66 are. A first
// line that is no record tells none either, and the reader of the judge's log says what is wrong
// with it.
func CallKind(log []byte) string {
	first, _, _ := bytes.Cut(log, []byte("\n"))
	var run struct {
		CallKind string `json:"call_kind"`
	}
	if json.Unmarshal(first, &run) != nil {
		return ""
	}
	return run.CallKind
}

// Records are the log of a run of a kind of call read back: the run, its items, which are the
// sheets of the run or the pages of the workbook (ReadItems), and its calls in the order they were
// made, each as the kind's own record, and how the run ended, or nil when the log stops before the
// end.
type Records[R, S, C any] struct {
	Run    R
	Sheets []S
	Calls  []C
	End    *End
}

// ReadRecords reads the log of a run of a kind of call over sheets back, as ReadLog reads a log, each
// record into the kind's own type: the run's, which must tell that kind of call, named so in what it
// says, and the sheets' and the calls'. A log of another kind of call is refused, and so is a record
// of a kind the log has no place for.
func ReadRecords[R, S, C any](r io.Reader, callKind, name string) (*Records[R, S, C], error) {
	return ReadItems[R, S, C](r, callKind, name, "sheet")
}

// ReadItems is ReadRecords for a kind whose items are records of another kind than "sheet", as the
// pages of the workbook are, "page" (T79): a record of the kind of item goes to the Sheets of the
// log, and one of the other kind has no place in it.
func ReadItems[R, S, C any](r io.Reader, callKind, name, item string) (*Records[R, S, C], error) {
	var l Records[R, S, C]
	end, err := ReadLog(r, func(kind string, line []byte) error {
		switch kind {
		case "run":
			var head struct {
				CallKind string `json:"call_kind"`
			}
			if err := json.Unmarshal(line, &head); err != nil {
				return err
			}
			if head.CallKind != callKind {
				return fmt.Errorf("a log of the kind of call %q, not of the %s", head.CallKind, name)
			}
			return json.Unmarshal(line, &l.Run)
		case item:
			var rec S
			err := json.Unmarshal(line, &rec)
			l.Sheets = append(l.Sheets, rec)
			return err
		case "call":
			var rec C
			err := json.Unmarshal(line, &rec)
			l.Calls = append(l.Calls, rec)
			return err
		}
		return fmt.Errorf("no record of the kind %q", kind)
	})
	if err != nil {
		return nil, err
	}
	l.End = end
	return &l, nil
}
