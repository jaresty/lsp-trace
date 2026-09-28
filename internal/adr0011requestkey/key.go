// Package adr0011requestkey provides an inert V1 identity codec. It does not issue requests.
package adr0011requestkey

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"lsp-trace/internal/lspwire"
)

const prefix = "lsp-trace.request-key.v1:g="

// Encode returns the canonical textual identity of a structured runtime request key.
func Encode(key lspwire.RequestKey) string {
	return prefix + strconv.FormatUint(key.Generation, 10) + ";id=" + strconv.FormatUint(key.ID, 10)
}

// Parse accepts only the exact V1 grammar and round-trip canonical representation.
func Parse(text string) (lspwire.RequestKey, error) {
	var key lspwire.RequestKey
	if !strings.HasPrefix(text, prefix) {
		return key, fmt.Errorf("request key: unsupported version or prefix")
	}
	fields := strings.Split(strings.TrimPrefix(text, prefix), ";id=")
	if len(fields) != 2 {
		return key, fmt.Errorf("request key: invalid fields")
	}
	var err error
	key.Generation, err = strconv.ParseUint(fields[0], 10, 64)
	if err != nil {
		return lspwire.RequestKey{}, fmt.Errorf("request key generation: %w", err)
	}
	key.ID, err = strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return lspwire.RequestKey{}, fmt.Errorf("request key id: %w", err)
	}
	if text != Encode(key) {
		return lspwire.RequestKey{}, fmt.Errorf("request key: noncanonical encoding")
	}
	return key, nil
}

// Identity compares an encoded key to independently supplied runtime and raw
// JSON-RPC ID tokens. A string ID must never be coerced to a numeric V1 ID.
// Invocation identity is separately compared, not inferred from the key.
func Identity(text string, runtime lspwire.RequestKey, generation, ownerWireID uint64, requestID, responseID json.RawMessage, invocation, expectedInvocation string) error {
	parsed, err := Parse(text)
	if err != nil {
		return err
	}
	wireToken := strconv.FormatUint(parsed.ID, 10)
	if parsed != runtime || parsed.Generation != generation || parsed.ID != ownerWireID ||
		string(requestID) != wireToken || string(responseID) != wireToken ||
		invocation == "" || invocation != expectedInvocation {
		return fmt.Errorf("request key: identity mismatch")
	}
	return nil
}
