package sessionruntime

import (
	"context"
	"sync/atomic"
	"testing"

	"lsp-trace/internal/session"
)

func TestPrivateGroupingCaptureObservationVocabulary(t *testing.T) {
	want := []string{"UNOBSERVED", "OK", "UNKNOWN", "PRECHECK_REFUSED", "RESOURCE_ENVELOPE_REFUSED", "ENVELOPE_CONTEXT_REFUSED", "ACTIVE_LEASE_REFUSED", "METADATA_REFUSED", "PREPARATION_UNKNOWN_FAILURE", "PREPARATION_SUPPLY_INVALID", "SOURCE_BOUND_REFUSED", "TARGET_SELECTION_REFUSED", "TARGET_BOUND_REFUSED", "FINAL_LEASE_REFUSED", "FINAL_BOUND_REFUSED", "QUERY_GUARD_REFUSED", "SYMBOL_CONTEXT_CANCELED", "SYMBOL_CONTEXT_DEADLINE", "SYMBOL_TOOL_NOT_IMPLEMENTED", "SYMBOL_PROCESS_CONTAINMENT_UNAVAILABLE", "SYMBOL_LIVE_LSP_DISABLED", "SYMBOL_SESSION_NOT_FOUND", "SYMBOL_LIST_CURSOR_INVALID", "SYMBOL_STALE_GENERATION", "SYMBOL_LIFECYCLE_CONFLICT", "SYMBOL_SESSION_REAP_INCOMPLETE", "SYMBOL_SESSION_STOPPING", "SYMBOL_SESSION_POISONED", "SYMBOL_RESOURCE_EXHAUSTED", "SYMBOL_SPAWN_FAILED", "SYMBOL_PIPE_SETUP_FAILED", "SYMBOL_INITIALIZATION_FAILED", "SYMBOL_INITIALIZATION_TIMEOUT", "SYMBOL_SESSION_CRASHED", "SYMBOL_REQUEST_CANCELLED", "SYMBOL_REQUEST_TIMEOUT", "SYMBOL_UNKNOWN_FAILURE", "SYMBOL_SERVER_ERROR", "SYMBOL_MISSING_PAIR", "SYMBOL_INVALID_PAIR", "RELATION_CONTEXT_CANCELED", "RELATION_CONTEXT_DEADLINE", "RELATION_TOOL_NOT_IMPLEMENTED", "RELATION_PROCESS_CONTAINMENT_UNAVAILABLE", "RELATION_LIVE_LSP_DISABLED", "RELATION_SESSION_NOT_FOUND", "RELATION_LIST_CURSOR_INVALID", "RELATION_STALE_GENERATION", "RELATION_LIFECYCLE_CONFLICT", "RELATION_SESSION_REAP_INCOMPLETE", "RELATION_SESSION_STOPPING", "RELATION_SESSION_POISONED", "RELATION_RESOURCE_EXHAUSTED", "RELATION_SPAWN_FAILED", "RELATION_PIPE_SETUP_FAILED", "RELATION_INITIALIZATION_FAILED", "RELATION_INITIALIZATION_TIMEOUT", "RELATION_SESSION_CRASHED", "RELATION_REQUEST_CANCELLED", "RELATION_REQUEST_TIMEOUT", "RELATION_UNKNOWN_FAILURE", "RELATION_SERVER_ERROR", "RELATION_MISSING_PAIR", "RELATION_INVALID_PAIR", "PREPARATION_TOOL_NOT_IMPLEMENTED", "PREPARATION_PROCESS_CONTAINMENT_UNAVAILABLE", "PREPARATION_LIVE_LSP_DISABLED", "PREPARATION_SESSION_NOT_FOUND", "PREPARATION_LIST_CURSOR_INVALID", "PREPARATION_STALE_GENERATION", "PREPARATION_LIFECYCLE_CONFLICT", "PREPARATION_SESSION_REAP_INCOMPLETE", "PREPARATION_SESSION_STOPPING", "PREPARATION_SESSION_POISONED", "PREPARATION_RESOURCE_EXHAUSTED", "PREPARATION_SPAWN_FAILED", "PREPARATION_PIPE_SETUP_FAILED", "PREPARATION_INITIALIZATION_FAILED", "PREPARATION_INITIALIZATION_TIMEOUT", "PREPARATION_SESSION_CRASHED", "PREPARATION_REQUEST_CANCELLED", "PREPARATION_REQUEST_TIMEOUT", "PREPARATION_DOCUMENT_SUPPLY_UNAVAILABLE", "PREPARATION_LANGUAGE_ID_UNAVAILABLE", "PREPARATION_DOCUMENT_URI_UNAVAILABLE", "PREPARATION_DOCUMENT_OUTSIDE_WORKSPACE", "PREPARATION_DOCUMENT_SOURCE_UNAVAILABLE", "PREPARATION_WINDOW_UNJOINED"}
	if len(want) != 88 {
		t.Fatal("ASSERT_CAPTURE_CLOSED: wrong count")
	}
	for i := 0; i < 256; i++ {
		expected := "UNKNOWN"
		if i < len(want) {
			expected = want[i]
		}
		if got := PrivateGroupingCaptureResult(i).String(); got != expected {
			t.Fatal("ASSERT_CAPTURE_CLOSED", i, got, expected)
		}
	}
	t.Log("ASSERT_CAPTURE_CLOSED: PASS 88 cases, 256 numeric values, maximum string 43 bytes")
}

type captureObservationOpaque struct {
	data  []byte
	calls *atomic.Int32
}

func (e captureObservationOpaque) Error() string { e.calls.Add(1); panic("opaque Error") }
func (e captureObservationOpaque) Is(error) bool { e.calls.Add(1); panic("opaque Is") }
func (e captureObservationOpaque) Unwrap() error { e.calls.Add(1); panic("opaque Unwrap") }

func TestPrivateGroupingCaptureObservationTransactionBoundaries(t *testing.T) {
	// Pure typed return-boundary tests, NOT claims of induced native/provider faults.
	failures := []session.Failure{session.ToolNotImplemented, session.ProcessContainmentUnavailable, session.LiveLSPDisabled, session.SessionNotFound, session.ListCursorInvalid, session.StaleGeneration, session.LifecycleConflict, session.SessionReapIncomplete, session.SessionStopping, session.SessionPoisoned, session.ResourceExhausted, session.SpawnFailure, session.PipeSetupFailure, session.InitializationFailure, session.InitializationTimeout, session.SessionCrashed, session.RequestCancelled, session.RequestTimeout}
	for _, symbol := range []bool{true, false} {
		base := PrivateGroupingCaptureRelationContextCanceled
		if symbol {
			base = PrivateGroupingCaptureSymbolContextCanceled
		}
		for i, f := range failures {
			if got := privateGroupingCaptureTransactionResult(symbol, nil, f, true, true, true); got != base+PrivateGroupingCaptureResult(i+2) {
				t.Fatal("ASSERT_CAPTURE_TYPED_FAILURE", i, got.String())
			}
		}
		cases := []struct {
			ctx                      error
			failure                  session.Failure
			server, missing, invalid bool
			want                     PrivateGroupingCaptureResult
		}{
			{context.Canceled, session.RequestTimeout, true, true, true, base},
			{context.DeadlineExceeded, session.RequestCancelled, true, true, true, base + 1},
			{nil, session.Failure("not-a-closed-runtime-failure"), true, true, true, base + 20},
			{nil, "", true, true, true, base + 21},
			{nil, "", false, true, true, base + 22},
			{nil, "", false, false, true, base + 23},
			{nil, "", false, false, false, PrivateGroupingCaptureOK},
		}
		for _, c := range cases {
			if got := privateGroupingCaptureTransactionResult(symbol, c.ctx, c.failure, c.server, c.missing, c.invalid); got != c.want {
				t.Fatal("ASSERT_CAPTURE_BOUNDARY_PRIORITY", got.String(), c.want.String())
			}
		}
	}
	t.Log("ASSERT_CAPTURE_TYPED_FAILURE: PASS; ASSERT_CAPTURE_BOUNDARY_PRIORITY: PASS; pure typed-boundary evidence only")
}
func TestPrivateGroupingCaptureObservationOpaqueAndPreparation(t *testing.T) {
	defer func() {
		if recover() != nil {
			t.Error("ASSERT_CAPTURE_OPAQUE: FAIL method invoked")
		}
	}()
	var calls atomic.Int32
	opaque := captureObservationOpaque{[]byte("never disclose"), &calls}
	for _, symbol := range []bool{true, false} {
		if privateGroupingCaptureTransactionResult(symbol, opaque, session.RequestTimeout, true, true, true) != PrivateGroupingCaptureUnknown {
			t.Fatal("ASSERT_CAPTURE_OPAQUE")
		}
	}
	if calls.Load() != 0 {
		t.Fatal("ASSERT_CAPTURE_OPAQUE_METHOD")
	}
	// Independent literal expectations: these must distinguish typed statuses,
	// not merely agree with the implementation's generic nonempty branch.
	for _, sample := range []struct {
		failure session.Failure
		want    string
	}{
		{session.ToolNotImplemented, "PREPARATION_TOOL_NOT_IMPLEMENTED"},
		{session.ProcessContainmentUnavailable, "PREPARATION_PROCESS_CONTAINMENT_UNAVAILABLE"},
		{session.LiveLSPDisabled, "PREPARATION_LIVE_LSP_DISABLED"},
		{session.SessionNotFound, "PREPARATION_SESSION_NOT_FOUND"},
		{session.ListCursorInvalid, "PREPARATION_LIST_CURSOR_INVALID"},
		{session.StaleGeneration, "PREPARATION_STALE_GENERATION"},
		{session.LifecycleConflict, "PREPARATION_LIFECYCLE_CONFLICT"},
		{session.SessionReapIncomplete, "PREPARATION_SESSION_REAP_INCOMPLETE"},
		{session.SessionStopping, "PREPARATION_SESSION_STOPPING"},
		{session.SessionPoisoned, "PREPARATION_SESSION_POISONED"},
		{session.ResourceExhausted, "PREPARATION_RESOURCE_EXHAUSTED"},
		{session.SpawnFailure, "PREPARATION_SPAWN_FAILED"},
		{session.PipeSetupFailure, "PREPARATION_PIPE_SETUP_FAILED"},
		{session.InitializationFailure, "PREPARATION_INITIALIZATION_FAILED"},
		{session.InitializationTimeout, "PREPARATION_INITIALIZATION_TIMEOUT"},
		{session.SessionCrashed, "PREPARATION_SESSION_CRASHED"},
		{session.RequestCancelled, "PREPARATION_REQUEST_CANCELLED"},
		{session.RequestTimeout, "PREPARATION_REQUEST_TIMEOUT"},
		{DocumentSupplyUnavailable, "PREPARATION_DOCUMENT_SUPPLY_UNAVAILABLE"},
		{LanguageIDUnavailable, "PREPARATION_LANGUAGE_ID_UNAVAILABLE"},
		{DocumentURIUnavailable, "PREPARATION_DOCUMENT_URI_UNAVAILABLE"},
		{DocumentOutsideWorkspace, "PREPARATION_DOCUMENT_OUTSIDE_WORKSPACE"},
		{DocumentSourceUnavailable, "PREPARATION_DOCUMENT_SOURCE_UNAVAILABLE"},
		{privateProviderWindowUnjoined, "PREPARATION_WINDOW_UNJOINED"},
		{session.Failure("unrecognized-private-text"), "PREPARATION_UNKNOWN_FAILURE"},
		{session.Failure("request_timeout"), "PREPARATION_UNKNOWN_FAILURE"},
		{"", "PREPARATION_SUPPLY_INVALID"},
	} {
		if got := privateGroupingCapturePreparationResult(sample.failure).String(); got != sample.want {
			t.Fatal("ASSERT_CAPTURE_PREPARATION_TYPED_STATUS", got, sample.want)
		}
	}
	t.Log("ASSERT_CAPTURE_OPAQUE: PASS; ASSERT_CAPTURE_PREPARATION_TYPED_STATUS: PASS; pure typed-boundary evidence only")
}
func TestPrivateGroupingCaptureObservationLegacyGuards(t *testing.T) {
	for _, manager := range []*Manager{nil, {}} {
		for _, req := range []PrivateGroupingOwnedRequest{{}, {References: true}, {Definitions: true}, {Revision: " padded "}, {URI: "\x00"}} {
			h, e := manager.CapturePrivateGroupingOwned(nil, req)
			oh, oe, code := manager.CapturePrivateGroupingOwnedObserved(nil, req)
			if h.capture != nil || oh.capture != nil || e != ErrPrivateGroupingOwned || oe != e || code != PrivateGroupingCapturePrecheckRefused {
				t.Fatal("ASSERT_CAPTURE_LEGACY_GUARD")
			}
			h, e = manager.CapturePrivateGroupingQueryOwned(nil, req)
			oh, oe, code = manager.CapturePrivateGroupingQueryOwnedObserved(nil, req)
			want := PrivateGroupingCapturePrecheckRefused
			if req.References || req.Definitions {
				want = PrivateGroupingCaptureQueryGuardRefused
			}
			if h.capture != nil || oh.capture != nil || e != ErrPrivateGroupingOwned || oe != e || code != want {
				t.Fatal("ASSERT_CAPTURE_QUERY_GUARD")
			}
		}
	}
	t.Log("ASSERT_CAPTURE_LEGACY_GUARD: PASS; ASSERT_CAPTURE_QUERY_GUARD: PASS")
}
