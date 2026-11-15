package discoveryfailure

import (
	"context"
	"encoding/json"
	"errors"
)

// ServerCodeCategory is a diagnostic-only closed snapshot, never a raw code.
// Zero means absent/unavailable, not the numeric RPC error code zero.
type ServerCodeCategory uint8

const (
	ServerCodeUnknown ServerCodeCategory = iota
	ParseError
	InvalidRequest
	MethodNotFound
	InvalidParams
	InternalError
	ServerNotInitialized
	UnknownErrorCode
	RequestCancelled
	ContentModified
	ServerCancelled
	RequestFailed
	ServerCodeOther
)

var serverCodeLabels = [...]string{"UNKNOWN", "PARSE_ERROR", "INVALID_REQUEST", "METHOD_NOT_FOUND", "INVALID_PARAMS", "INTERNAL_ERROR", "SERVER_NOT_INITIALIZED", "UNKNOWN_ERROR_CODE", "REQUEST_CANCELLED", "CONTENT_MODIFIED", "SERVER_CANCELLED", "REQUEST_FAILED", "OTHER"}

func (c ServerCodeCategory) Valid() bool { return c <= ServerCodeOther }
func (c ServerCodeCategory) MarshalJSON() ([]byte, error) {
	if !c.Valid() {
		return nil, errors.New("invalid server code category")
	}
	return json.Marshal(serverCodeLabels[c])
}
func (c *ServerCodeCategory) UnmarshalJSON(raw []byte) error {
	var label string
	if json.Unmarshal(raw, &label) != nil {
		return errors.New("invalid server code category")
	}
	for i, s := range serverCodeLabels {
		if label == s {
			*c = ServerCodeCategory(i)
			return nil
		}
	}
	return errors.New("invalid server code category")
}

// ClassifyServerCode accepts only an actual typed server-error Code. Callers
// must handle absence separately. Unknown/custom codes, including zero, are OTHER.
// Values implement the JSON-RPC 2.0 and LSP error-code whitelist in the request.
func ClassifyServerCode(code int) ServerCodeCategory {
	switch code {
	case -32700:
		return ParseError
	case -32600:
		return InvalidRequest
	case -32601:
		return MethodNotFound
	case -32602:
		return InvalidParams
	case -32603:
		return InternalError
	case -32002:
		return ServerNotInitialized
	case -32001:
		return UnknownErrorCode
	case -32800:
		return RequestCancelled
	case -32801:
		return ContentModified
	case -32802:
		return ServerCancelled
	case -32803:
		return RequestFailed
	default:
		return ServerCodeOther
	}
}

// DetailsFromError uses the same bounded, first-typed-cause policy as FromError.
// Only the winning cause's own optional closed detail is consulted; no formatting,
// native payload retention, multi-error traversal, or deeper-detail borrowing.
func DetailsFromError(err error) (Cause, ServerCodeCategory) {
	if errors.Is(err, context.Canceled) {
		return Canceled, ServerCodeUnknown
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return Deadline, ServerCodeUnknown
	}
	for depth := 0; err != nil && depth < 32; depth++ {
		if typed, ok := err.(interface{ PrepareFailureCause() Cause }); ok {
			c := typed.PrepareFailureCause()
			if !c.Valid() {
				return Unknown, ServerCodeUnknown
			}
			if c == RPCServer {
				if detail, ok := err.(interface{ PrepareFailureRPCCodeCategory() uint8 }); ok {
					category := ServerCodeCategory(detail.PrepareFailureRPCCodeCategory())
					if category.Valid() {
						return c, category
					}
				}
			}
			return c, ServerCodeUnknown
		}
		wrapped, ok := err.(interface{ Unwrap() error })
		if !ok {
			break
		}
		err = wrapped.Unwrap()
	}
	return Unknown, ServerCodeUnknown
}
