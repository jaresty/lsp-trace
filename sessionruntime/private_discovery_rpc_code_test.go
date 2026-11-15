package sessionruntime

import (
	"context"
	"encoding/json"
	"errors"
	"lsp-trace/internal/discoveryfailure"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
	"math"
	"reflect"
	"testing"
)

func TestRPC72ActualNumericSnapshot(t *testing.T) {
	for _, code := range []int{-32700, -32600, -32601, -32602, -32603, -32002, -32001, -32800, -32801, -32802, -32803, 0, math.MinInt, math.MaxInt} {
		server := &lspwire.RPCError{Code: code, Message: "secret://message", Data: json.RawMessage(`{"secret":"data"}`)}
		e := privateDiscoveryPrepareRPCCodeFailure("", server)
		server.Code = 100
		server.Message = "changed"
		server.Data[0] = 'x'
		e = privateDiscoveryPrepareFailure(nil, e, PrivateDiscoveryRefusal{})
		cause, category := discoveryfailure.DetailsFromError(e)
		if cause != discoveryfailure.RPCServer || category != discoveryfailure.ClassifyServerCode(code) || !errors.Is(e, ErrPrivateDiscovery) {
			t.Fatal("RPC72_TYPED_NUMERIC_SNAPSHOT")
		}
		value := reflect.ValueOf(e).Elem()
		if value.NumField() != 2 || value.Field(0).Kind() != reflect.String || value.Field(1).Kind() != reflect.Uint8 {
			t.Fatal("RPC72_CLOSED_SNAPSHOT_STRUCTURE")
		}
	}
	for _, failure := range []session.Failure{"", session.RequestCancelled, session.RequestTimeout, session.SessionNotFound, session.StaleGeneration, session.SessionStopping, session.SessionPoisoned, session.SessionCrashed, session.ResourceExhausted, "opaque"} {
		for _, present := range []bool{false, true} {
			var server *lspwire.RPCError
			if present {
				server = &lspwire.RPCError{Code: -32603}
			}
			e := privateDiscoveryPrepareRPCCodeFailure(failure, server)
			cause, category := discoveryfailure.DetailsFromError(e)
			if cause != discoveryfailure.FromError(privateDiscoveryPrepareRPCFailure(failure, present)) {
				t.Fatal("RPC72_SESSION_CAUSE_UNCHANGED")
			}
			if category != discoveryfailure.ServerCodeUnknown && (failure != "" || !present) {
				t.Fatal("RPC72_NO_STALE_SESSION_CATEGORY")
			}
			if !present && category != discoveryfailure.ServerCodeUnknown {
				t.Fatal("RPC72_NIL_NOT_OTHER")
			}
		}
	}
	server := &lspwire.RPCError{Code: -32603}
	for _, ctx := range []error{context.Canceled, context.DeadlineExceeded} {
		e := privateDiscoveryPrepareFailure(ctx, privateDiscoveryPrepareRPCCodeFailure("", server), PrivateDiscoveryRefusal{})
		_, category := discoveryfailure.DetailsFromError(e)
		if category != discoveryfailure.ServerCodeUnknown {
			t.Fatal("RPC72_CONTEXT_DROPS_CATEGORY")
		}
	}
	budget := PrivateDiscoveryRefusal{Stage: PrivateDiscoveryRead, Currency: PrivateDiscoveryResponseBytes, Requested: 1, AmountsAvailable: true}
	e := privateDiscoveryPrepareFailure(context.Canceled, privateDiscoveryPrepareRPCCodeFailure("", server), budget)
	cause, category := discoveryfailure.DetailsFromError(e)
	if cause != discoveryfailure.EnclosureBudget || category != discoveryfailure.ServerCodeUnknown {
		t.Fatal("RPC72_BUDGET_PRIORITY")
	}
	if n := testing.AllocsPerRun(100, func() { _ = privateDiscoveryPrepareRPCCodeFailure("", server) }); n != 0 {
		t.Fatal("RPC72_SNAPSHOT_NO_ALLOCATION", n)
	}
}
