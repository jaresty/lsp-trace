// Package adr0011genericv5proposal contains offline candidate-only checks.
// It is neither an issuer nor a production capability verifier.
package adr0011genericv5proposal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"lsp-trace/internal/strictjson"
)

const (
	MaxIDRawTokenBytes      = 4096
	MaxIDDecodedStringBytes = 1024
	MaxIDAbsoluteExponent   = 100000
	MaxCompleteIDFrameBytes = 2097152
)

// ID keeps the exact original JSON token. A normalized decimal never replaces it.
type ID struct {
	RawToken []byte
	Kind     string // STRING or NUMBER
	String   string
	Digits   string // normalized nonzero decimal coefficient
	Power    int64  // decimal exponent of Digits, not expanded into zeroes
	Negative bool
}

func parseID(token json.RawMessage) (ID, error) {
	var id ID
	if len(token) == 0 || len(token) > MaxIDRawTokenBytes || !json.Valid(token) {
		return id, errors.New("invalid or oversized raw ID token")
	}
	id.RawToken = append([]byte(nil), token...)
	if token[0] == '"' {
		if err := json.Unmarshal(token, &id.String); err != nil {
			return ID{}, errors.New("invalid JSON string ID")
		}
		if len(id.String) > MaxIDDecodedStringBytes {
			return ID{}, errors.New("oversized decoded string ID")
		}
		id.Kind = "STRING"
		return id, nil
	}
	if token[0] != '-' && (token[0] < '0' || token[0] > '9') {
		return ID{}, errors.New("ID is not a JSON string or number")
	}
	// JSON.Valid has already checked the complete JSON-number grammar. Bound
	// every parse step by the token length; never use float64 or exponent expansion.
	s := string(token)
	if s[0] == '-' {
		id.Negative = true
		s = s[1:]
	}
	exp := int64(0)
	if p := strings.IndexAny(s, "eE"); p >= 0 {
		exponent := s[p+1:]
		s = s[:p]
		sign := int64(1)
		if strings.HasPrefix(exponent, "-") {
			sign = -1
			exponent = exponent[1:]
		} else if strings.HasPrefix(exponent, "+") {
			exponent = exponent[1:]
		}
		if len(exponent) == 0 || len(exponent) > 6 {
			return ID{}, errors.New("oversized ID exponent")
		}
		for i := 0; i < len(exponent); i++ {
			exp = exp*10 + int64(exponent[i]-'0')
		}
		exp *= sign
		if exp > MaxIDAbsoluteExponent || exp < -MaxIDAbsoluteExponent {
			return ID{}, errors.New("oversized ID exponent")
		}
	}
	if p := strings.IndexByte(s, '.'); p >= 0 {
		exp -= int64(len(s) - p - 1)
		s = s[:p] + s[p+1:]
	}
	s = strings.TrimLeft(s, "0")
	id.Kind = "NUMBER"
	if s == "" {
		id.Negative = false // JSON -0 and 0 denote the same decimal.
		id.Digits = "0"
		return id, nil
	}
	for len(s) > 1 && s[len(s)-1] == '0' {
		s = s[:len(s)-1]
		exp++
	}
	id.Digits, id.Power = s, exp
	return id, nil
}

// SameValue is typed semantic ID equality; RawToken remains available for custody.
func (id ID) SameValue(other ID) bool {
	if id.Kind == "" || id.Kind != other.Kind {
		return false
	}
	if id.Kind == "STRING" {
		return id.String == other.String
	}
	return id.Negative == other.Negative && id.Digits == other.Digits && id.Power == other.Power
}

func framedObject(frame []byte) (map[string]json.RawMessage, error) {
	if len(frame) > MaxCompleteIDFrameBytes+128 {
		return nil, errors.New("oversized complete frame")
	}
	header, body, ok := bytes.Cut(frame, []byte("\r\n\r\n"))
	if !ok || !bytes.HasPrefix(header, []byte("Content-Length: ")) || len(header) > 128 || len(body) > MaxCompleteIDFrameBytes {
		return nil, errors.New("invalid complete frame")
	}
	length, err := strconv.Atoi(string(bytes.TrimPrefix(header, []byte("Content-Length: "))))
	if err != nil || length != len(body) || strictjson.RejectDuplicates(body) != nil {
		return nil, errors.New("invalid complete frame or duplicate decoded key")
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(body, &object); err != nil || object == nil {
		return nil, errors.New("invalid JSON-RPC object")
	}
	return object, nil
}

// Pair retains exact original ID tokens while comparing typed decoded semantics.
type Pair struct {
	Request  ID
	Response ID
}

func ValidateFramedPair(requestFrame, responseFrame []byte) (Pair, error) {
	var pair Pair
	req, err := framedObject(requestFrame)
	if err != nil {
		return pair, err
	}
	resp, err := framedObject(responseFrame)
	if err != nil {
		return pair, err
	}
	if string(req["jsonrpc"]) != `"2.0"` || string(resp["jsonrpc"]) != `"2.0"` || len(req["method"]) == 0 || len(resp["method"]) != 0 || len(resp["params"]) != 0 {
		return pair, errors.New("invalid request/response JSON-RPC shape")
	}
	if _, hasResult := resp["result"]; hasResult {
		if _, hasError := resp["error"]; hasError {
			return pair, errors.New("response has both result and error")
		}
	} else if body, hasError := resp["error"]; !hasError || string(body) == "null" {
		return pair, errors.New("response lacks result or nonnull error")
	}
	pair.Request, err = parseID(req["id"])
	if err != nil {
		return Pair{}, fmt.Errorf("request ID: %w", err)
	}
	pair.Response, err = parseID(resp["id"])
	if err != nil {
		return Pair{}, fmt.Errorf("response ID: %w", err)
	}
	if !pair.Request.SameValue(pair.Response) {
		return Pair{}, errors.New("mismatched typed JSON-RPC ID")
	}
	return pair, nil
}

// CandidateState calls the state fold only after ID correspondence succeeds.
// This is an offline guard, not a private B4 implementation or runtime route.
func CandidateState(requestFrame, responseFrame []byte, state func() string) string {
	if _, err := ValidateFramedPair(requestFrame, responseFrame); err != nil {
		return "INVALID_CHRONOLOGY"
	}
	return state()
}
