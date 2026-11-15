package sessionruntime

import (
	"context"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"lsp-trace/internal/discoveryfailure"
	"lsp-trace/internal/session"
	"os"
	"strings"
	"testing"
)

func TestPrepare66SourceRPCMapping(t *testing.T) {
	for _, tc := range []struct {
		name    string
		failure session.Failure
		server  bool
		want    discoveryfailure.Cause
	}{
		{"rpc", "", true, discoveryfailure.RPCServer},
		{"cancel", session.RequestCancelled, false, discoveryfailure.Canceled},
		{"timeout", session.RequestTimeout, false, discoveryfailure.Deadline},
		{"crash", session.SessionCrashed, false, discoveryfailure.SessionCrashed},
		{"missing", session.SessionNotFound, false, discoveryfailure.SessionUnavailable},
		{"stale", session.StaleGeneration, false, discoveryfailure.SessionUnavailable},
		{"stopping", session.SessionStopping, false, discoveryfailure.SessionUnavailable},
		{"poison", session.SessionPoisoned, false, discoveryfailure.SessionUnavailable},
		{"opaque", session.Failure("secret://source"), true, discoveryfailure.Unknown},
		{"exhausted-not-proven-budget", session.ResourceExhausted, false, discoveryfailure.Unknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := privateDiscoveryPrepareRPCFailure(tc.failure, tc.server)
			if discoveryfailure.FromError(e) != tc.want || !errors.Is(e, ErrPrivateDiscovery) || e.Error() != "private discovery enclosure refused" {
				t.Fatal("SOURCE_CLOSED_MAPPING")
			}
			if discoveryfailure.FromError(privateDiscoveryPrepareFailure(nil, e, PrivateDiscoveryRefusal{})) != tc.want {
				t.Fatal("CAUSE_SURVIVES_PRIVATE_BOUNDARY")
			}
		})
	}
}
func TestPrepare66SourceBudgetUnknownAndDecode(t *testing.T) {
	knownZero := PrivateDiscoveryRefusal{Stage: PrivateDiscoveryRead, Currency: PrivateDiscoveryResponseBytes, Requested: 1, AmountsAvailable: true}
	unknown := PrivateDiscoveryRefusal{Stage: PrivateDiscoveryUnknownStage}
	for _, tc := range []struct {
		name        string
		ctxErr, err error
		ref         PrivateDiscoveryRefusal
		want        discoveryfailure.Cause
	}{
		{"known-zero", nil, ErrPrivateDiscovery, knownZero, discoveryfailure.EnclosureBudget},
		{"unknown-amounts", nil, ErrPrivateDiscovery, unknown, discoveryfailure.Unknown},
		{"deadline", context.DeadlineExceeded, ErrPrivateDiscovery, unknown, discoveryfailure.Deadline},
		{"canceled", context.Canceled, ErrPrivateDiscovery, unknown, discoveryfailure.Canceled},
		{"opaque", nil, errors.New("secret://source"), unknown, discoveryfailure.Unknown},
		{"syntax", nil, &json.SyntaxError{}, unknown, discoveryfailure.InvalidResponse},
		{"type", nil, &json.UnmarshalTypeError{Value: "secret://source"}, unknown, discoveryfailure.InvalidResponse},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := privateDiscoveryPrepareFailure(tc.ctxErr, tc.err, tc.ref)
			if discoveryfailure.FromError(e) != tc.want || !errors.Is(e, ErrPrivateDiscovery) || strings.Contains(e.Error(), "secret") {
				t.Fatal("SOURCE_UNKNOWN_ZERO_PRIVACY")
			}
		})
	}
	// The error object is a pointer to immutable package storage, not an allocation
	// made after a failed window. Refusal accounting/cleanup state is not touched.
	if n := testing.AllocsPerRun(100, func() { _ = privateDiscoveryPrepareRPCFailure(session.RequestTimeout, false) }); n != 0 {
		t.Fatal("NO_FAILED_WINDOW_ERROR_ALLOCATION", n)
	}
}
func TestPrepare66FirstRefusalUnchanged(t *testing.T) {
	first := PrivateDiscoveryRefusal{Stage: PrivateDiscoveryRead, Currency: PrivateDiscoveryResponseBytes, Requested: 1, Remaining: 0, AmountsAvailable: true}
	w := &privateDiscoveryWindow{ctx: context.Background(), refusal: first}
	before := w.remaining
	err := w.prepareFailure(nil, ErrPrivateDiscovery)
	if discoveryfailure.FromError(err) != discoveryfailure.EnclosureBudget || w.remaining != before || w.failed {
		t.Fatal("CLASSIFICATION_NO_ACCOUNTING_EFFECT")
	}
	final := w.refusalErrorFrom(err, PrivateDiscoveryCleanup, true)
	got, ok := PrivateDiscoveryRefusalFromError(final)
	first.CleanupUncertain = true
	if !ok || got != first || !errors.Is(final, ErrPrivateDiscovery) {
		t.Fatal("FIRST_REFUSAL_CLEANUP_UNCHANGED")
	}
}

func TestPrepare66ProductionSourceCallsite(t *testing.T) {
	raw, e := os.ReadFile("private_discovery_window.go")
	if e != nil {
		t.Fatal(e)
	}
	f, e := parser.ParseFile(token.NewFileSet(), "window.go", raw, 0)
	if e != nil {
		t.Fatal(e)
	}
	mapped, deferred := false, false
	ast.Inspect(f, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if id, ok := c.Fun.(*ast.Ident); ok && id.Name == "privateDiscoveryPrepareRPCFailure" {
			mapped = true
		}
		if s, ok := c.Fun.(*ast.SelectorExpr); ok && s.Sel.Name == "prepareFailure" {
			deferred = true
		}
		return true
	})
	if !mapped || !deferred {
		t.Fatal("ACTUAL_PRIVATE_PREPARE_SOURCE_WIRED")
	}
}
