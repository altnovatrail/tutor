package protocol_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"reflect"
	"strings"
	"testing"

	evalv1 "github.com/altnovatrail/spec/contracts/novatrail/eval/v1"
	"google.golang.org/protobuf/proto"

	"github.com/altnovatrail/tutor/internal/protocol"
	"github.com/altnovatrail/tutor/internal/protocol/protocoltest"
	"github.com/altnovatrail/tutor/internal/sample"
)

// Every example of spec — five sample files and two reports — is read, written again and read
// back: what is written holds the same value as the example, as spec's check compares them, and
// reads back into the same message (contracts/README.md, sections 11 and 14).
func TestExamplesReadBack(t *testing.T) {
	examples := protocoltest.Examples(t)
	names, err := fs.Glob(examples, "*.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 7 {
		t.Errorf("%d examples, want spec's 7: %v", len(names), names)
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			data, err := fs.ReadFile(examples, name)
			if err != nil {
				t.Fatal(err)
			}
			var read, again proto.Message
			var written []byte
			if strings.HasPrefix(name, "report-") {
				r, err := protocol.ReadReport(bytes.NewReader(data))
				if err != nil {
					t.Fatal(err)
				}
				if written, err = protocol.MarshalReport(r); err != nil {
					t.Fatal(err)
				}
				read = r
				again, err = protocol.ReadReport(bytes.NewReader(written))
				if err != nil {
					t.Fatal(err)
				}
			} else {
				s, err := protocol.ReadSample(bytes.NewReader(data))
				if err != nil {
					t.Fatal(err)
				}
				if written, err = protocol.MarshalSample(s); err != nil {
					t.Fatal(err)
				}
				read = s
				again, err = protocol.ReadSample(bytes.NewReader(written))
				if err != nil {
					t.Fatal(err)
				}
			}
			if !reflect.DeepEqual(valueOf(t, data), valueOf(t, written)) {
				t.Errorf("what is written holds another value than the example:\n%s", written)
			}
			if !proto.Equal(read, again) {
				t.Error("what is written reads back into another message")
			}
		})
	}
}

func valueOf(t *testing.T, data []byte) any {
	t.Helper()
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// A file is written indented by two spaces and ending with a newline, as spec's examples are, and
// the same message gives the same bytes: protojson's own spaces, which change from build to build,
// are gone.
func TestMarshalIsIndented(t *testing.T) {
	s := &evalv1.Sample{Protocol: protocol.Protocol, Id: "synthetic-7", Markup: &evalv1.Markup{}}
	data, err := protocol.MarshalSample(s)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "{\n  \"protocol\": \"novatrail.eval.v1\",\n  \"id\": \"synthetic-7\",") || !strings.HasSuffix(string(data), "\n}\n") {
		t.Errorf("written:\n%s", data)
	}
	var b bytes.Buffer
	if err := json.Indent(&b, bytes.TrimSpace(data), "", "  "); err != nil || b.String()+"\n" != string(data) {
		t.Errorf("not indented as json.Indent indents: %v\n%s", err, data)
	}
	if !strings.Contains(string(data), `"canvas_version": ""`) || !strings.Contains(string(data), `"notes": []`) {
		t.Errorf("the zero values are not written:\n%s", data)
	}
	for _, m := range []func() ([]byte, error){
		func() ([]byte, error) { return protocol.MarshalSample(&evalv1.Sample{Protocol: "novatrail.eval.v2"}) },
		func() ([]byte, error) { return protocol.MarshalReport(&evalv1.Report{}) },
	} {
		if _, err := m(); !isProtocolError(err) {
			t.Errorf("a message of another protocol is written: %v", err)
		}
	}
}

// A reader reads only a file of the protocol, strictly: one with no protocol, of another protocol
// or with a field the message does not have is refused, and so is a file that is no JSON object.
func TestReadRefuses(t *testing.T) {
	for _, c := range []struct {
		name, file string
		protocol   bool // the refusal is the protocol's, at the path "protocol"
	}{
		{"no protocol", `{"id": "a"}`, true},
		{"another protocol", `{"protocol": "novatrail.eval.v2", "id": "a"}`, true},
		{"protocol not a string", `{"protocol": 1}`, true},
		{"protocol twice", `{"protocol": "novatrail.eval.v2", "protocol": "novatrail.eval.v1"}`, true},
		{"unknown field", `{"protocol": "novatrail.eval.v1", "writer_name": "Ann"}`, false},
		{"field twice", `{"protocol": "novatrail.eval.v1", "id": "a", "id": "b"}`, false},
		{"not an object", `["novatrail.eval.v1"]`, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, errSample := protocol.ReadSample(strings.NewReader(c.file))
			_, errReport := protocol.ReadReport(strings.NewReader(strings.ReplaceAll(c.file, `"id"`, `"runner"`)))
			for _, err := range []error{errSample, errReport} {
				if err == nil {
					t.Fatal("read")
				}
				if isProtocolError(err) != c.protocol {
					t.Errorf("the refusal is %v", err)
				}
			}
		})
	}
	if _, err := protocol.ReadSample(strings.NewReader(`{"protocol": "novatrail.eval.v1", "writer_name": "Ann"}`)); err == nil || !strings.Contains(err.Error(), "writer_name") {
		t.Errorf("the refusal does not name the field: %v", err)
	}
}

func isProtocolError(err error) bool {
	var fe *sample.FormatError
	return errors.As(err, &fe) && len(fe.Problems) == 1 && fe.Problems[0].Path == "protocol"
}
