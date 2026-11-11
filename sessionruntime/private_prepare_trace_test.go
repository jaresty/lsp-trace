package sessionruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"runtime/trace"
	"testing"
	"time"

	"lsp-trace/internal/session"
)

type privatePrepareTraceHostileError struct{ values []string }

func (privatePrepareTraceHostileError) Error() string { panic("opaque Error") }
func (privatePrepareTraceHostileError) Is(error) bool { panic("opaque Is") }
func (privatePrepareTraceHostileError) Unwrap() error { panic("opaque Unwrap") }

type privatePrepareTraceCountingContext struct {
	context.Context
	errCalls *int
}

func (c privatePrepareTraceCountingContext) Err() error {
	*c.errCalls++
	return c.Context.Err()
}

func TestPrivatePrepareTraceVocabulary(t *testing.T) {
	t.Log("PROCEDURE: independent literal failure table and hostile opaque errors")
	expected := []string{"TOOL_NOT_IMPLEMENTED", "PROCESS_CONTAINMENT_UNAVAILABLE", "LIVE_LSP_DISABLED", "SESSION_NOT_FOUND", "LIST_CURSOR_INVALID", "STALE_GENERATION", "LIFECYCLE_CONFLICT", "SESSION_REAP_INCOMPLETE", "SESSION_STOPPING", "SESSION_POISONED", "RESOURCE_EXHAUSTED", "SPAWN_FAILED", "PIPE_SETUP_FAILED", "INITIALIZATION_FAILED", "INITIALIZATION_TIMEOUT", "SESSION_CRASHED", "REQUEST_CANCELLED", "REQUEST_TIMEOUT"}
	for _, want := range expected {
		if privatePrepareTraceFailure(session.Failure(want)) != want {
			t.Fatal("ASSERT_TRACE_VOCABULARY: FAIL", want)
		}
	}
	if privatePrepareTraceFailure("") != "OK" || privatePrepareTraceFailure("secret-path-payload") != "UNKNOWN" {
		t.Fatal("ASSERT_TRACE_VOCABULARY: FAIL unknown")
	}
	for _, tc := range []struct {
		err  error
		want string
	}{{nil, "NONE"}, {context.Canceled, "CANCELED"}, {context.DeadlineExceeded, "DEADLINE"}, {privatePrepareTraceHostileError{}, "UNKNOWN"}} {
		if privatePrepareTraceError(tc.err) != tc.want {
			t.Fatal("ASSERT_TRACE_VOCABULARY: FAIL error")
		}
	}
	t.Log("ASSERT_TRACE_VOCABULARY: PASS")
}

func TestPrivatePrepareTraceEOF(t *testing.T) {
	t.Log("PROCEDURE: real roundTripManager eof with live upstream and active lease; manual transaction helper mirrors capture checkpoint, not full capture integration")
	var storage bytes.Buffer
	if err := trace.Start(&storage); err != nil {
		t.Fatal(err)
	}
	stopped := false
	defer func() {
		if !stopped {
			trace.Stop()
		}
	}()
	m, s, child := roundTripManager(t, "eof")
	defer child.Close()
	upstream, cancel := context.WithCancel(context.Background())
	defer cancel()
	var got RoundTripResult
	var sampledErr, cause error
	leaseErr := m.WithReadySessionLease(upstream, SessionLeaseRequest{SessionID: s.SessionID, Generation: s.Generation, Workspace: "/workspace"}, func(ctx context.Context) error {
		got = m.RoundTrip(ctx, RoundTripRequest{SessionID: s.SessionID, Generation: s.Generation, Method: "textDocument/documentSymbol", Params: json.RawMessage(`{}`), Deadline: time.Now().Add(time.Second), MaxMessages: 4, MaxBytes: 65536})
		sampledErr = ctx.Err()
		controlCalls, actualCalls := 0, 0
		cause = context.Cause(privatePrepareTraceCountingContext{ctx, &controlCalls})
		privatePrepareTraceTransaction(privatePrepareTraceCountingContext{ctx, &actualCalls}, sampledErr, got.Failure, got.ServerError != nil, got.ownedMethodPair != nil)
		if actualCalls != controlCalls {
			t.Fatal("ASSERT_TRACE_CAUSE_READS: FAIL")
		}
		t.Logf("ASSERT_TRACE_CAUSE_READS: PASS helper=%d standard-Cause=%d", actualCalls, controlCalls)
		return sampledErr
	})
	trace.Stop()
	stopped = true
	if got.Failure != session.SessionCrashed || sampledErr != context.Canceled || cause != errSessionLeaseConflict || leaseErr != context.Canceled || upstream.Err() != nil {
		t.Fatal("ASSERT_TRACE_EOF_FACTS: FAIL")
	}
	t.Log("ASSERT_TRACE_EOF_FACTS: PASS upstream live, failure SESSION_CRASHED, context CANCELED, cause LEASE_CONFLICT")
	for _, marker := range []string{"ABOUT_TO_CANCEL", "SESSION_CRASHED", "CANCELED", "LEASE_CONFLICT", "private.prepare.context_err", "private.prepare.context_cause", "private.prepare.server_error", "private.prepare.owned_pair", "RETURN"} {
		if !bytes.Contains(storage.Bytes(), []byte(marker)) {
			t.Fatal("ASSERT_TRACE_FAILURE_PRESERVED: FAIL", marker)
		}
	}
	if storage.Len() > 8<<20 {
		t.Fatal("ASSERT_TRACE_FAILURE_PRESERVED: FAIL trace bound")
	}
	t.Log("ASSERT_TRACE_FAILURE_PRESERVED: PASS")
	// Only an explicitly supplied test-output path retains this controlled trace.
	if path := os.Getenv("LSP_TRACE_PRIVATE_TEST_TRACE_OUTPUT"); path != "" {
		if err := os.WriteFile(path, storage.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPrivatePrepareTraceSeparateSamples(t *testing.T) {
	var storage bytes.Buffer
	if err := trace.Start(&storage); err != nil {
		t.Fatal(err)
	}
	stopped := false
	defer func() {
		if !stopped {
			trace.Stop()
		}
	}()
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(errSessionLeaseConflict)
	controlCalls, actualCalls := 0, 0
	_ = context.Cause(privatePrepareTraceCountingContext{ctx, &controlCalls})
	// Deliberately earlier nil sample and later canceled cause must not coalesce.
	privatePrepareTraceTransaction(privatePrepareTraceCountingContext{ctx, &actualCalls}, nil, session.RequestTimeout, true, true)
	if actualCalls != controlCalls {
		t.Fatal("ASSERT_TRACE_CAUSE_READS: FAIL")
	}
	t.Logf("ASSERT_TRACE_CAUSE_READS: PASS helper=%d standard-Cause=%d", actualCalls, controlCalls)
	privatePrepareTraceTransaction(context.Background(), privatePrepareTraceHostileError{}, session.Failure("secret-path-payload"), false, false)
	trace.Stop()
	stopped = true
	for _, marker := range []string{"NONE", "LEASE_CONFLICT", "REQUEST_TIMEOUT", "PRESENT", "ABSENT", "UNKNOWN"} {
		if !bytes.Contains(storage.Bytes(), []byte(marker)) {
			t.Fatal("ASSERT_TRACE_SEPARATE_SAMPLES: FAIL", marker)
		}
	}
	if bytes.Contains(storage.Bytes(), []byte("secret-path-payload")) {
		t.Fatal("ASSERT_TRACE_SEPARATE_SAMPLES: FAIL raw payload")
	}
	t.Log("ASSERT_TRACE_SEPARATE_SAMPLES: PASS")
}
