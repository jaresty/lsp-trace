package adr0011generic

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"unicode/utf8"
)

const maxPrivateMetadata = 1048576 // checkpoint A test-only bound; not a selected generic policy limit
const maxSafeInteger = 9007199254740991

// parseMetadata rejects duplicate decoded object member names at every depth,
// including escaped aliases, before canonical encoding or schema validation.
func parseMetadata(raw []byte) (any, error) {
	if len(raw) == 0 || len(raw) > maxPrivateMetadata || !utf8.Valid(raw) {
		return nil, errors.New("metadata byte/UTF-8 bound")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	work := 0
	var parse func(int) (any, error)
	parse = func(depth int) (any, error) {
		work++
		if depth > 64 || work > maxPrivateMetadata {
			return nil, errors.New("metadata parse work bound")
		}
		tok, err := d.Token()
		if err != nil {
			return nil, err
		}
		delim, ok := tok.(json.Delim)
		if !ok {
			return tok, nil
		}
		switch delim {
		case '{':
			obj := map[string]any{}
			for d.More() {
				k, e := d.Token()
				if e != nil {
					return nil, e
				}
				key, ok := k.(string)
				if !ok || !utf8.ValidString(key) {
					return nil, errors.New("invalid metadata member")
				}
				if _, present := obj[key]; present {
					return nil, errors.New("duplicate decoded metadata key")
				}
				value, e := parse(depth + 1)
				if e != nil {
					return nil, e
				}
				obj[key] = value
			}
			end, e := d.Token()
			if e != nil || end != json.Delim('}') {
				return nil, errors.New("invalid object end")
			}
			return obj, nil
		case '[':
			a := []any{}
			for d.More() {
				v, e := parse(depth + 1)
				if e != nil {
					return nil, e
				}
				a = append(a, v)
			}
			end, e := d.Token()
			if e != nil || end != json.Delim(']') {
				return nil, errors.New("invalid array end")
			}
			return a, nil
		default:
			return nil, errors.New("unexpected metadata delimiter")
		}
	}
	v, err := parse(0)
	if err != nil {
		return nil, err
	}
	if _, err = d.Token(); err != io.EOF {
		return nil, errors.New("trailing metadata content")
	}
	return v, nil
}

func writeString(out *bytes.Buffer, s string) error {
	if !utf8.ValidString(s) {
		return errors.New("invalid string UTF-8")
	}
	out.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			out.WriteString(`\"`)
		case '\\':
			out.WriteString(`\\`)
		case '\b':
			out.WriteString(`\b`)
		case '\f':
			out.WriteString(`\f`)
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		case '\t':
			out.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(out, "\\u%04x", r)
			} else {
				out.WriteRune(r)
			}
		}
	}
	out.WriteByte('"')
	return nil
}
func writeCanonical(out *bytes.Buffer, v any, depth int) error {
	if depth > 64 || out.Len() > maxPrivateMetadata {
		return errors.New("canonical metadata bound")
	}
	switch x := v.(type) {
	case nil:
		out.WriteString("null")
	case bool:
		if x {
			out.WriteString("true")
		} else {
			out.WriteString("false")
		}
	case string:
		return writeString(out, x)
	case json.Number:
		n, e := strconv.ParseInt(string(x), 10, 64)
		if e != nil || n > maxSafeInteger || n < -maxSafeInteger {
			return errors.New("unsupported non-integer or imprecise metadata number")
		}
		out.WriteString(strconv.FormatInt(n, 10))
	case int:
		if x > maxSafeInteger || x < -maxSafeInteger {
			return errors.New("imprecise metadata integer")
		}
		out.WriteString(strconv.Itoa(x))
	case int64:
		if x > maxSafeInteger || x < -maxSafeInteger {
			return errors.New("imprecise metadata integer")
		}
		out.WriteString(strconv.FormatInt(x, 10))
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return errors.New("invalid metadata number")
		}
		return errors.New("floating metadata number not selected")
	case []any:
		out.WriteByte('[')
		for i, item := range x {
			if i > 0 {
				out.WriteByte(',')
			}
			if err := writeCanonical(out, item, depth+1); err != nil {
				return err
			}
		}
		out.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			if !utf8.ValidString(k) {
				return errors.New("invalid metadata key")
			}
			keys = append(keys, k)
		}
		// UTF-8 lexical ordering of valid scalar sequences agrees with Unicode scalar ordering.
		sort.Strings(keys)
		out.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				out.WriteByte(',')
			}
			if err := writeString(out, k); err != nil {
				return err
			}
			out.WriteByte(':')
			if err := writeCanonical(out, x[k], depth+1); err != nil {
				return err
			}
		}
		out.WriteByte('}')
	default:
		return fmt.Errorf("unsupported metadata type %T", v)
	}
	if out.Len() > maxPrivateMetadata {
		return errors.New("canonical metadata byte bound")
	}
	return nil
}

// canonicalMetadata encodes a JSON metadata value; it never re-encodes binary
// source, request, response or framed-wire originals.
func canonicalMetadata(v any) ([]byte, error) {
	if _, ok := v.(map[string]any); !ok {
		return nil, errors.New("metadata root must be an object")
	}
	var out bytes.Buffer
	if err := writeCanonical(&out, v, 0); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
func readCanonicalMetadata(raw []byte) (any, error) {
	v, err := parseMetadata(raw)
	if err != nil {
		return nil, err
	}
	encoded, err := canonicalMetadata(v)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(raw, encoded) {
		return nil, errors.New("noncanonical metadata bytes")
	}
	return v, nil
}
