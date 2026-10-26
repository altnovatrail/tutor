package sample

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
)

// readStrict reads exactly one JSON value into v, the way the format wants it read: every field of
// every object present, spelt exactly as its JSON name, once; null only where a field may be
// absent in meaning (a pointer); no field the type does not have; and nothing after the value.
// encoding/json alone would take a missing field as its zero value, match names regardless of
// case and let a repeated key win, so a pass over the tokens checks the shape first and lists
// every problem by its path. A broken JSON text is a plain error; a wrong shape is a *FormatError.
func readStrict(r io.Reader, what string, v any) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	p := &problems{what: what}
	if err := walk(dec, reflect.TypeOf(v).Elem(), "", p); err != nil {
		return err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return errors.New("unexpected data after the JSON value")
	}
	if err := p.err(); err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// walk reads one JSON value from dec and checks it against the Go type t. It returns an error only
// when the JSON text itself is broken; what does not fit the type goes to p.
func walk(dec *json.Decoder, t reflect.Type, path string, p *problems) error {
	tok, err := dec.Token()
	if err != nil {
		return syntaxError(err)
	}
	if tok == nil {
		if t.Kind() != reflect.Pointer {
			p.add(at(path), "is null")
		}
		return nil
	}
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Struct:
		if tok != json.Delim('{') {
			p.add(at(path), "is not an object")
			return skip(dec, tok)
		}
		return walkObject(dec, t, path, p)
	case reflect.Slice:
		if tok != json.Delim('[') {
			p.add(at(path), "is not an array")
			return skip(dec, tok)
		}
		for i := 0; dec.More(); i++ {
			if err := walk(dec, t.Elem(), fmt.Sprintf("%s[%d]", path, i), p); err != nil {
				return err
			}
		}
		_, err := dec.Token() // ']'
		return syntaxError(err)
	case reflect.String:
		if _, ok := tok.(string); !ok {
			p.add(at(path), "is not a string")
			return skip(dec, tok)
		}
	case reflect.Bool:
		if _, ok := tok.(bool); !ok {
			p.add(at(path), "is not true or false")
			return skip(dec, tok)
		}
	case reflect.Int, reflect.Int64:
		n, ok := tok.(json.Number)
		if !ok {
			p.add(at(path), "is not a number")
			return skip(dec, tok)
		}
		if _, err := n.Int64(); err != nil {
			p.add(at(path), "%s is not a whole number", n)
		}
	case reflect.Float64:
		n, ok := tok.(json.Number)
		if !ok {
			p.add(at(path), "is not a number")
			return skip(dec, tok)
		}
		if _, err := n.Float64(); err != nil {
			p.add(at(path), "%s is out of range", n)
		}
	default:
		panic(fmt.Sprintf("sample: no rule for the Go type %s", t))
	}
	return nil
}

// walkObject reads the fields of an object whose '{' has been read.
func walkObject(dec *json.Decoder, t reflect.Type, path string, p *problems) error {
	fields := make(map[string]reflect.Type, t.NumField())
	var order []string
	for i := range t.NumField() {
		f := t.Field(i)
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		fields[name] = f.Type
		order = append(order, name)
	}
	seen := make(map[string]bool, len(fields))
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return syntaxError(err)
		}
		key := tok.(string) // a key of an object is always a string
		ft, known := fields[key]
		switch {
		case !known:
			p.add(at(path), "unknown field %q", key)
			if err := skipValue(dec); err != nil {
				return err
			}
			continue
		case seen[key]:
			p.add(join(path, key), "appears twice")
		}
		seen[key] = true
		if err := walk(dec, ft, join(path, key), p); err != nil {
			return err
		}
	}
	if _, err := dec.Token(); err != nil { // '}'
		return syntaxError(err)
	}
	for _, name := range order {
		if !seen[name] {
			p.add(join(path, name), "is missing")
		}
	}
	return nil
}

// skipValue reads one value and throws it away.
func skipValue(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return syntaxError(err)
	}
	return skip(dec, tok)
}

// skip throws away the rest of a value whose first token has been read.
func skip(dec *json.Decoder, tok json.Token) error {
	if tok != json.Delim('{') && tok != json.Delim('[') {
		return nil
	}
	for depth := 1; depth > 0; {
		tok, err := dec.Token()
		if err != nil {
			return syntaxError(err)
		}
		switch tok {
		case json.Delim('{'), json.Delim('['):
			depth++
		case json.Delim('}'), json.Delim(']'):
			depth--
		}
	}
	return nil
}

// syntaxError names a JSON text that ends too early in words; it passes nil and other errors on.
func syntaxError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || strings.Contains(err.Error(), "unexpected end of JSON input") {
		return errors.New("the JSON text ends too early")
	}
	return err
}

func join(path, name string) string {
	if path == "" {
		return name
	}
	return path + "." + name
}

// Root is the path of a problem with the file as a whole rather than with one of its fields, such
// as a field the format does not have at the top of the file.
const Root = "(the file)"

// at names the root of the file when the path is empty.
func at(path string) string {
	if path == "" {
		return Root
	}
	return path
}
