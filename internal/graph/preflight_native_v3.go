package graph

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"
)

const (
	MaxNativeV3Bytes  = 8 << 20
	MaxNativeV3Depth  = 64
	MaxNativeV3Values = 100000
)

// PreflightNativeV3 bounds the exact embedded graph before DecodeNativeV3
// performs recursive decoding and aggregate allocation.
func PreflightNativeV3(raw []byte) error {
	if len(raw) == 0 || len(raw) > MaxNativeV3Bytes || !utf8.Valid(raw) {
		return errors.New("native graph byte/UTF-8 limit")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	depth, values := 0, 0
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		values++
		if values > MaxNativeV3Values {
			return errors.New("native graph resource limit")
		}
		if delim, ok := tok.(json.Delim); ok {
			switch delim {
			case '{', '[':
				depth++
				if depth > MaxNativeV3Depth {
					return errors.New("native graph depth limit")
				}
			case '}', ']':
				depth--
				if depth < 0 {
					return errors.New("native graph malformed nesting")
				}
			}
		}
	}
	if depth != 0 {
		return errors.New("native graph incomplete nesting")
	}
	return nil
}
