// Package discoveryfailure owns diagnostic-only prepare failure vocabulary.
// No cause is authority, proof of absence, an observed empty result, or a retry grant.
package discoveryfailure

import (
	"context"
	"encoding/json"
	"errors"
)

type Cause string

const (
	// Unknown is omitted in retained records, preserving legacy canonical bytes.
	Unknown            Cause = ""
	Canceled           Cause = "CONTEXT_CANCELED"
	Deadline           Cause = "DEADLINE_EXCEEDED"
	RPCServer          Cause = "RPC_SERVER_ERROR"
	SessionUnavailable Cause = "SESSION_UNAVAILABLE"
	SessionCrashed     Cause = "SESSION_CRASHED"
	EnclosureBudget    Cause = "ENCLOSURE_BUDGET"
	InvalidResponse    Cause = "INVALID_RESPONSE"
)

func (c Cause) Valid() bool {
	switch c {
	case Unknown, Canceled, Deadline, RPCServer, SessionUnavailable, SessionCrashed, EnclosureBudget, InvalidResponse:
		return true
	}
	return false
}
func (c Cause) MarshalJSON() ([]byte, error) {
	if !c.Valid() {
		return nil, errors.New("invalid prepare failure cause")
	}
	if c == Unknown {
		return json.Marshal("UNKNOWN")
	}
	return json.Marshal(string(c))
}
func (c *Cause) UnmarshalJSON(raw []byte) error {
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return errors.New("invalid prepare failure cause")
	}
	v := Cause(s)
	if s == "UNKNOWN" {
		v = Unknown
	} else if v == Unknown {
		return errors.New("invalid prepare failure cause")
	}
	if !v.Valid() {
		return errors.New("invalid prepare failure cause")
	}
	*c = v
	return nil
}

// FromError never formats, parses, or retains the underlying error. Providers
// may expose only the shared closed value; unrecognized or invalid values stay unknown.
func FromError(err error) Cause {
	if errors.Is(err, context.Canceled) {
		return Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return Deadline
	}
	// The concrete private prepare error is recognized on the first iteration.
	// Bound optional wrapper inspection without allocating an errors.As target;
	// an unavailable/deeper or multi-error classification remains unknown.
	for depth := 0; err != nil && depth < 32; depth++ {
		if typed, ok := err.(interface{ PrepareFailureCause() Cause }); ok {
			c := typed.PrepareFailureCause()
			if c.Valid() {
				return c
			}
			return Unknown
		}
		wrapped, ok := err.(interface{ Unwrap() error })
		if !ok {
			break
		}
		err = wrapped.Unwrap()
	}
	return Unknown
}
