package lspwire

import (
	"encoding/json"
	"errors"
)

// ResponseFacts is a fixed, payload-free projection of original JSON, not a
// successful decode or server authentication. Shapes describe the last field
// occurrence (the last error object, not encoding/json's merged RPCError).
// Zero means no structural inspection was admitted/completed, never an
// observed absent error. A complete body may still have been acquired.
// Codes use exact JSON integer lexemes and signed 64-bit range; 1e0 and 1.0 are
// NotInteger even though mathematically integral. No server ID value survives.
type ResponseFacts struct {
	State       ResponseFactsState  `json:"state"`
	Envelope    ResponseValueKind   `json:"envelope"`
	Error       ResponseValueKind   `json:"error"`
	Result      ResponseValueKind   `json:"result"`
	ID          ResponseValueKind   `json:"id"`
	Association ResponseAssociation `json:"association"`
	CodeState   ResponseCodeState   `json:"code_state"`
	Code        int64               `json:"code"`
}
type ResponseFactsState uint8
type ResponseValueKind uint8
type ResponseAssociation uint8
type ResponseCodeState uint8

const (
	ResponseNotObserved ResponseFactsState = iota
	ResponseInvalidJSON
	ResponseValidJSON
)
const (
	ResponseAbsent ResponseValueKind = iota
	ResponseNull
	ResponseObject
	ResponseArray
	ResponseString
	ResponseNumber
	ResponseBoolean
)
const (
	ResponseUnassociated ResponseAssociation = iota
	ResponseIDMatched
	ResponseIDUnmatched
	ResponseIDInvalid
)
const (
	ResponseCodeAbsent ResponseCodeState = iota
	ResponseCodeNull
	ResponseCodeWrongType
	ResponseCodeNotInteger
	ResponseCodeOutOfRange
	ResponseCodeValid
)

func (f ResponseFacts) Valid() bool {
	if f.State > ResponseValidJSON || f.Envelope > ResponseBoolean || f.Error > ResponseBoolean || f.Result > ResponseBoolean || f.ID > ResponseBoolean || f.Association > ResponseIDInvalid || f.CodeState > ResponseCodeValid {
		return false
	}
	if f.State != ResponseValidJSON {
		return f.Envelope == 0 && f.Error == 0 && f.Result == 0 && f.ID == 0 && f.Association == 0 && f.CodeState == 0 && f.Code == 0
	}
	if f.Envelope == ResponseAbsent {
		return false
	}
	if f.Envelope != ResponseObject && (f.Error != ResponseAbsent || f.Result != ResponseAbsent || f.ID != ResponseAbsent || f.CodeState != ResponseCodeAbsent) {
		return false
	}
	if f.CodeState != ResponseCodeValid && f.Code != 0 {
		return false
	}
	if f.Error != ResponseObject && f.CodeState != ResponseCodeAbsent {
		return false
	}
	if (f.Association == ResponseIDMatched || f.Association == ResponseIDUnmatched) && f.ID != ResponseNumber {
		return false
	}
	return true
}
func (f ResponseFacts) MarshalJSON() ([]byte, error) {
	if !f.Valid() {
		return nil, errors.New("invalid private response facts")
	}
	type value ResponseFacts
	return json.Marshal(value(f))
}
func responseValueKind(c byte) ResponseValueKind {
	switch c {
	case 'n':
		return ResponseNull
	case '{':
		return ResponseObject
	case '[':
		return ResponseArray
	case '"':
		return ResponseString
	case 't', 'f':
		return ResponseBoolean
	default:
		return ResponseNumber
	}
}

// Syntax has already been validated. No conversion to string or allocation.
func responseCode(b []byte) (int64, ResponseCodeState) {
	if len(b) == 0 {
		return 0, ResponseCodeAbsent
	}
	if b[0] == 'n' {
		return 0, ResponseCodeNull
	}
	if (b[0] < '0' || b[0] > '9') && b[0] != '-' {
		return 0, ResponseCodeWrongType
	}
	for _, c := range b {
		if c == '.' || c == 'e' || c == 'E' {
			return 0, ResponseCodeNotInteger
		}
	}
	i := 0
	neg := b[0] == '-'
	if neg {
		i++
	}
	limit := uint64(1<<63 - 1)
	if neg {
		limit++
	}
	var n uint64
	for ; i < len(b); i++ {
		d := uint64(b[i] - '0')
		if n > (limit-d)/10 {
			return 0, ResponseCodeOutOfRange
		}
		n = n*10 + d
	}
	if neg {
		return -int64(n), ResponseCodeValid
	}
	return int64(n), ResponseCodeValid
}
func responseAssociation(b []byte, expected uint64) ResponseAssociation {
	if expected == 0 {
		return ResponseUnassociated
	}
	if len(b) == 0 {
		return ResponseIDInvalid
	}
	var n uint64
	for _, c := range b {
		if c < '0' || c > '9' {
			return ResponseIDInvalid
		}
		d := uint64(c - '0')
		if n > (^uint64(0)-d)/10 {
			return ResponseIDInvalid
		}
		n = n*10 + d
	}
	if n == expected {
		return ResponseIDMatched
	}
	return ResponseIDUnmatched
}
