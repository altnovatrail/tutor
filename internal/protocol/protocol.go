// Package protocol is spec's eval protocol in tutor's code: the package novatrail.eval.v1, whose
// messages Sample and Report are the sample file and the runner's report (contracts/README.md of
// spec, sections 11 and 14; R93, R95 spec). It reads and writes their files, folds a history of
// writing into the strokes a sheet shows, and holds the rules of a workbook's boxes and of snapping
// that the device and the runner share (section 12; R94 spec). The groups of strokes and the crop
// are internal/lines' and internal/render's, and their tests pass spec's vectors too.
//
// The messages are the generated code of the module github.com/altnovatrail/spec/contracts, at the
// tag of the spec submodule: code on Go takes a contract from it and generates none of its own
// (section 8). Whether a file keeps the rules of sections 11 and 12 is spec's check sample-check to
// say, and tutor writes no check of its own (R93 spec): this package reads a file strictly, as the
// text asks, and no more.
package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/altnovatrail/tutor/internal/sample"
)

// Protocol is what a file of the protocol names in its field "protocol": the package, whose last
// component is the version of the contract (section 2).
const Protocol = "novatrail.eval.v1"

// ReadSample reads a sample file: its protocol first, and a file of any other refused, and then the
// message strictly, a field the message does not have refused (section 11, "Writing and reading").
// An error of the protocol is a *sample.FormatError at the path "protocol".
func ReadSample(r io.Reader) (*evalv1.Sample, error) {
	var s evalv1.Sample
	if err := read(r, "sample file", &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// ReadReport reads a runner's report as ReadSample reads a sample file (section 14).
func ReadReport(r io.Reader) (*evalv1.Report, error) {
	var rep evalv1.Report
	if err := read(r, "report", &rep); err != nil {
		return nil, err
	}
	return &rep, nil
}

// read reads a file of the protocol into m. The protocol is read first, loosely, so that a file of
// another protocol is named as such and not by the first field this one lacks.
func read(r io.Reader, what string, m proto.Message) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read %s: %w", what, err)
	}
	given, err := protocolsOf(data)
	if err != nil {
		return fmt.Errorf("read %s: %w", what, err)
	}
	switch {
	case len(given) == 0:
		return protocolError(what, "is missing, and this reader reads only %q", Protocol)
	case len(given) > 1:
		return protocolError(what, "is given %d times, and a file names its protocol once", len(given))
	}
	var protocol string
	if json.Unmarshal(given[0], &protocol) != nil || protocol != Protocol {
		return protocolError(what, "is %s, and this reader reads only %q", given[0], Protocol)
	}
	if err := protojson.Unmarshal(data, m); err != nil {
		return fmt.Errorf("read %s: %w", what, err)
	}
	return nil
}

// protocolsOf are the values of every key "protocol" of a JSON object, at its top level: each one,
// since a decoder into a map would keep only the last.
func protocolsOf(data []byte) ([]json.RawMessage, error) {
	d := json.NewDecoder(bytes.NewReader(data))
	t, err := d.Token()
	if err != nil {
		return nil, err
	}
	if t != json.Delim('{') {
		return nil, errors.New("a file of the protocol is a JSON object")
	}
	var out []json.RawMessage
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return nil, err
		}
		var value json.RawMessage
		if err := d.Decode(&value); err != nil {
			return nil, err
		}
		if key == "protocol" {
			out = append(out, value)
		}
	}
	return out, nil
}

func protocolError(what, format string, args ...any) error {
	return &sample.FormatError{What: what, Problems: []sample.Problem{{Path: "protocol", Message: fmt.Sprintf(format, args...)}}}
}

// MarshalSample writes a sample file as section 6 has it: the protobuf names, and every field, its
// zero too. It is indented by two spaces and ends with a newline, as spec's examples are, and the
// same message gives the same bytes on every build. spec's check compares the values a file holds,
// not its bytes.
func MarshalSample(s *evalv1.Sample) ([]byte, error) {
	if s.GetProtocol() != Protocol {
		return nil, protocolError("sample file", "is %q, and a file of the protocol names %q", s.GetProtocol(), Protocol)
	}
	return marshal(s)
}

// MarshalReport writes a runner's report as MarshalSample writes a sample file.
func MarshalReport(r *evalv1.Report) ([]byte, error) {
	if r.GetProtocol() != Protocol {
		return nil, protocolError("report", "is %q, and a file of the protocol names %q", r.GetProtocol(), Protocol)
	}
	return marshal(r)
}

// marshal writes m with protojson and indents it again: protojson puts spaces at random on purpose,
// so that nothing holds to its bytes, and json.Indent drops them.
func marshal(m proto.Message) ([]byte, error) {
	data, err := protojson.MarshalOptions{UseProtoNames: true, EmitDefaultValues: true}.Marshal(m)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	if err := json.Indent(&b, bytes.TrimSpace(data), "", "  "); err != nil {
		return nil, err
	}
	b.WriteByte('\n')
	return b.Bytes(), nil
}
