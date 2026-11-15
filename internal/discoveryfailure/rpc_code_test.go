package discoveryfailure

import (
	"context"
	"encoding/json"
	"math"
	"testing"
)

type rpc72Error struct {
	cause    Cause
	category uint8
	inner    error
}

func (rpc72Error) Error() string                          { panic("raw Error must not be called") }
func (e rpc72Error) PrepareFailureCause() Cause           { return e.cause }
func (e rpc72Error) PrepareFailureRPCCodeCategory() uint8 { return e.category }
func (e rpc72Error) Unwrap() error                        { return e.inner }
func TestRPC72ClosedMapping(t *testing.T) {
	for _, tc := range []struct {
		code  int
		want  ServerCodeCategory
		label string
	}{
		{-32700, ParseError, "PARSE_ERROR"}, {-32600, InvalidRequest, "INVALID_REQUEST"}, {-32601, MethodNotFound, "METHOD_NOT_FOUND"}, {-32602, InvalidParams, "INVALID_PARAMS"}, {-32603, InternalError, "INTERNAL_ERROR"}, {-32002, ServerNotInitialized, "SERVER_NOT_INITIALIZED"}, {-32001, UnknownErrorCode, "UNKNOWN_ERROR_CODE"}, {-32800, RequestCancelled, "REQUEST_CANCELLED"}, {-32801, ContentModified, "CONTENT_MODIFIED"}, {-32802, ServerCancelled, "SERVER_CANCELLED"}, {-32803, RequestFailed, "REQUEST_FAILED"}, {0, ServerCodeOther, "OTHER"}, {math.MinInt, ServerCodeOther, "OTHER"}, {math.MaxInt, ServerCodeOther, "OTHER"}, {-32000, ServerCodeOther, "OTHER"},
	} {
		got := ClassifyServerCode(tc.code)
		if got != tc.want {
			t.Fatal("RPC72_NUMERIC_MAPPING")
		}
		raw, e := json.Marshal(got)
		if e != nil || string(raw) != `"`+tc.label+`"` {
			t.Fatal("RPC72_CLOSED_LABEL")
		}
		var back ServerCodeCategory
		if json.Unmarshal(raw, &back) != nil || back != got {
			t.Fatal("RPC72_ENUM_ROUNDTRIP")
		}
	}
	for _, raw := range []string{`null`, `0`, `""`, `"secret://payload"`, `"FUTURE"`, `{}`} {
		var c ServerCodeCategory
		if json.Unmarshal([]byte(raw), &c) == nil {
			t.Fatal("RPC72_INVALID_ENUM_DECODE")
		}
	}
	if _, e := json.Marshal(ServerCodeCategory(255)); e == nil {
		t.Fatal("RPC72_INVALID_ENUM_ENCODE")
	}
}
func TestRPC72CauseConsistency(t *testing.T) {
	for _, cause := range []Cause{Unknown, Canceled, Deadline, RPCServer, SessionUnavailable, SessionCrashed, EnclosureBudget, InvalidResponse, "invalid"} {
		e := rpc72Error{cause, uint8(InternalError), nil}
		got, cat := DetailsFromError(e)
		want := cause
		if !want.Valid() {
			want = Unknown
		}
		if got != want || (cause == RPCServer && cat != InternalError) || (cause != RPCServer && cat != ServerCodeUnknown) {
			t.Fatal("RPC72_WINNING_CAUSE_ONLY")
		}
	}
	for _, inner := range []error{context.Canceled, context.DeadlineExceeded} {
		e := rpc72Error{RPCServer, uint8(InternalError), inner}
		got, cat := DetailsFromError(e)
		if got != FromError(e) || cat != ServerCodeUnknown {
			t.Fatal("RPC72_CONTEXT_PRIORITY")
		}
	}
	if _, cat := DetailsFromError(rpc72Error{RPCServer, 255, nil}); cat != ServerCodeUnknown {
		t.Fatal("RPC72_INVALID_DETAIL_NOT_RETAINED")
	}
	if _, cat := DetailsFromError(nil); cat != ServerCodeUnknown {
		t.Fatal("RPC72_ABSENCE")
	}
	probe := error(rpc72Error{RPCServer, uint8(InternalError), nil})
	if n := testing.AllocsPerRun(100, func() { _, _ = DetailsFromError(probe) }); n != 0 {
		t.Fatal("RPC72_DETAIL_NO_ALLOCATION", n)
	}
}
