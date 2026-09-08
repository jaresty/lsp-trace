package retainedcalls

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"lsp-trace/internal/graphprovenance"
)

// Scan known carriers iteratively before any recursive schema/semantic decoding.
// Only the named byte carriers are decoded. Content and opaque data stay opaque.
func preflightExportV2(raw []byte, max int) error {
	return scanExportV2(raw, max, true, &envelopeAdmissionV2{})
}
func scanExportV2(raw []byte, max int, carriers bool, admission *envelopeAdmissionV2) error {
	if len(raw) > max {
		return &LimitErrorV2{"carrier bytes", max}
	}
	if len(raw) == 0 || !utf8.Valid(raw) {
		return errors.New("V2 invalid JSON UTF-8")
	}
	type frame struct {
		object, key bool
		keys        map[string]bool
	}
	stack := []frame{}
	roots := 0
	var authoritativeInput []byte
	nestedInput := false
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	for {
		token, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if len(stack) == 0 {
			roots++
		}
		if n, ok := token.(json.Number); ok {
			text := n.String()
			if len(text) > 64 {
				return errors.New("V2 numeric token limit")
			}
			if i := strings.IndexAny(text, "eE"); i >= 0 {
				x, err := strconv.ParseInt(text[i+1:], 10, 32)
				if err != nil || x < -1024 || x > 1024 {
					return errors.New("V2 numeric exponent limit")
				}
			}
		}
		if len(stack) > 0 {
			top := &stack[len(stack)-1]
			if top.object && top.key {
				if key, ok := token.(string); ok {
					if top.keys[key] {
						return errors.New("V2 duplicate member")
					}
					top.keys[key] = true
					top.key = false
					if key == "data" {
						var opaque json.RawMessage
						if err := d.Decode(&opaque); err != nil {
							return err
						}
						top.key = true
					}
					limit := 0
					switch key {
					case "input_bytes":
						limit = graphprovenance.MaxEnvelopeBytesV2
					case "graph_bytes":
						limit = graphprovenance.MaxGraphBytesV2
					case "canonical_receipt":
						limit = 4 << 20
					}
					if limit > 0 && carriers {
						v, err := d.Token()
						if err != nil {
							return err
						}
						s, ok := v.(string)
						if !ok || len(s) > base64.StdEncoding.EncodedLen(limit) {
							return errors.New("V2 known byte carrier type/limit")
						}
						decoded, err := base64.StdEncoding.DecodeString(s)
						if err != nil {
							return err
						}
						// Structurally scan every decoded carrier before semantic recursion.
						// Only input_bytes directly owned by the outer root object is
						// authoritative; identically named nested members remain unknown
						// typed fields and must never trigger full semantic admission.
						if key == "input_bytes" && len(stack) == 1 {
							authoritativeInput = append(authoritativeInput[:0], decoded...)
						} else {
							if err = scanLeafV2(decoded, limit); err != nil {
								return err
							}
							if key == "input_bytes" {
								nestedInput = true
							}
						}
						top.key = true
					}
					continue
				}
			} else if top.object {
				top.key = true
			}
		}
		if delim, ok := token.(json.Delim); ok {
			switch delim {
			case '{', '[':
				stack = append(stack, frame{delim == '{', delim == '{', map[string]bool{}})
				if len(stack) > 64 {
					return errors.New("V2 known-carrier depth limit")
				}
			case '}', ']':
				stack = stack[:len(stack)-1]
			}
		}
	}
	if roots != 1 || len(stack) != 0 {
		return errors.New("V2 requires one JSON value")
	}
	if carriers && !nestedInput && len(authoritativeInput) > 0 {
		if err := admission.admit(authoritativeInput); err != nil {
			return err
		}
	}
	return nil
}

// Decoded native graphs and canonical source receipts contain no additional
// encoded JSON carriers. Their typed validators reject unknown members; string
// values are never interpreted as member names or recursively decoded.
func scanLeafV2(raw []byte, max int) error {
	return scanExportV2(raw, max, false, nil)
}

// envelopeAdmissionV2 belongs to one validation invocation. input is an owned,
// immutable copy of the most recently admitted bytes, never a digest, borrowed
// slice, or mutable typed graph. Empty input cannot be admitted. Keeping one
// value bounds retention even for invalid exports with extra named carriers.
// The observer is a private test seam, not admission authority.
type envelopeAdmissionV2 struct {
	input   string
	observe func()
}

func (a *envelopeAdmissionV2) admit(raw []byte) error {
	if a.input != "" && a.input == string(raw) {
		return nil
	}
	if a.observe != nil {
		a.observe()
	}
	// Graph-provenance owns the envelope's complete preflight (including its
	// graph_bytes) and exact admission. No recursive export validator is invoked.
	if _, err := graphprovenance.ValidateFor(raw, graphprovenance.Family, "v2"); err != nil {
		return err
	}
	a.input = string(raw)
	return nil
}
