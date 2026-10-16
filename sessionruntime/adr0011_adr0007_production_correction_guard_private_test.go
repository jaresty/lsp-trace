package sessionruntime_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sync"
	"testing"
	"time"

	"lsp-trace/incomingops"
	"lsp-trace/internal/directionalraw"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/session"
	"lsp-trace/internal/source"
	"lsp-trace/sessionruntime"
)

func assertCorrectionGuardReleasedOncePrivate(t *testing.T, root directionalraw.BidirectionalRootPrivate) {
	t.Helper()
	before, err := root.SnapshotPrivate()
	if err != nil {
		t.Fatal(err)
	}
	if !before.Released || before.ReleaseCount != 1 || before.LiveBytes != 0 {
		t.Fatalf("ASSERT_CORRECTION_GUARD_TERMINAL_RELEASE_EXACT_ONCE: %+v", before)
	}
	if err := root.Release(); err != nil {
		t.Fatal(err)
	}
	if err := root.Release(); err != nil {
		t.Fatal(err)
	}
	after, err := root.SnapshotPrivate()
	if err != nil {
		t.Fatal(err)
	}
	if after.ReleaseCount != 1 || after.LiveBytes != 0 {
		t.Fatalf("ASSERT_CORRECTION_GUARD_REDUNDANT_RELEASE_NO_DOUBLE_RELEASE: before=%+v after=%+v", before, after)
	}
}

type correctionGuardSecondPrepareFailureRuntimePrivate struct {
	*sessionruntime.Manager
	mu           sync.Mutex
	prepareCalls int
}

func (r *correctionGuardSecondPrepareFailureRuntimePrivate) RoundTrip(ctx context.Context, req sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult {
	if req.Method == "textDocument/prepareCallHierarchy" {
		r.mu.Lock()
		r.prepareCalls++
		call := r.prepareCalls
		r.mu.Unlock()
		if call == 2 {
			return sessionruntime.RoundTripResult{Failure: session.Failure("forced-outgoing-target-resolution-failure")}
		}
	}
	return r.Manager.RoundTrip(ctx, req)
}

func TestADR0011ADR0007ProductionCorrectionGuardPrivate(t *testing.T) {
	readyManagerPrivate := func(t *testing.T, child *scriptedTransportPrivate) (*sessionruntime.Manager, sessionruntime.StartResult, string) {
		t.Helper()
		workspace := fixedWorkspacePathPrivate
		if err := os.RemoveAll(workspace); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(workspace, 0o700); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(workspace) })
		path := filepath.Join(workspace, "calls.go")
		if err := os.WriteFile(path, []byte("package calls\nfunc Target() {}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		uri, err := source.FileURI(path)
		if err != nil || uri != fixedWorkspaceURIPrivate {
			t.Fatalf("ASSERT_CORRECTION_GUARD_FIXED_WORKSPACE: uri=%q err=%v", uri, err)
		}
		validated, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: workspace, Profile: "go", EnvironmentReference: "local"})
		if err != nil {
			t.Fatal(err)
		}
		manager, err := sessionruntime.New(sessionruntime.Config{Wire: lspwire.Limits{MaxBodyBytes: 64 << 20, MaxHeaderBytes: 65536}, Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 16, MaxChildren: 2, MaxCancels: 16, MaxTombstones: 32, MaxObservations: 256, MaxOperations: 16}, Starter: starterPrivate{child: child}})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { manager.Shutdown(context.Background()) })
		started := manager.Start(context.Background(), sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(validated), LanguageID: "go"})
		pending := manager.BeginReadiness(context.Background(), started.SessionID, started.Generation, time.Now().Add(2*time.Second))
		ready, found := manager.WaitReadiness(context.Background(), pending.ID)
		if !found || ready.State != sessionruntime.ReadinessReady || ready.Failure != "" {
			t.Fatalf("ASSERT_CORRECTION_GUARD_READY: found=%t ready=%+v", found, ready)
		}
		deadline := time.Now().Add(time.Second)
		for len(child.snapshot()) < 3 {
			if time.Now().After(deadline) {
				t.Fatal("ASSERT_CORRECTION_GUARD_STARTUP_HANDOFF")
			}
			runtime.Gosched()
		}
		startup := child.snapshot()
		if len(startup) != 3 || startup[2].Method != "initialized" || startup[2].Kind != "request" {
			t.Fatalf("ASSERT_CORRECTION_GUARD_STARTUP_SEQUENCE: %+v", startup)
		}
		return manager, started, uri
	}
	t.Run("success-propagates-facets-authentic-wire-and-directional-results", func(t *testing.T) {
		child := newScriptedTransportPrivate("nonempty", "nonempty", false)
		manager, started, uri := readyManagerPrivate(t, child)
		req := privateRequestPrivate(t, started, uri, directionalraw.BidirectionalPlanPrivate{IncomingBytes: 4096, OutgoingBytes: 4096}, false)
		root := req.Coordinate.Root
		result := executePrivate(t, context.Background(), manager, req)
		if result.FailurePrivate() != nil || !result.CoordinationPrivate().AcceptedPrivate() {
			t.Fatalf("ASSERT_CORRECTION_GUARD_AUTHENTIC_EXECUTE_PRIVATE_SUCCESS: %+v", result)
		}
		for _, facet := range req.Coordinate.Facets {
			sealed := result.SealedArtifactPrivate()
			if !bytes.Contains(sealed, []byte(facet.Facet)) || !bytes.Contains(sealed, []byte(facet.ObservationState)) {
				t.Fatalf("ASSERT_CORRECTION_GUARD_RETURNED_FACET_DISPOSITION[%s/%s]: sealed=%s", facet.Facet, facet.ObservationState, sealed)
			}
		}

		receipts, err := root.ReceiptsPrivate()
		if err != nil || len(receipts) != 2 {
			t.Fatalf("ASSERT_CORRECTION_GUARD_DIRECTION_RECEIPT_CARDINALITY: receipts=%+v err=%v", receipts, err)
		}
		for i, direction := range []string{"incoming", "outgoing"} {
			method := "callHierarchy/" + direction + "Calls"
			var request, response []byte
			for _, record := range child.snapshot() {
				if record.Method != method {
					continue
				}
				if record.Kind == "request" {
					request = record.Raw
				} else if record.Kind == "response" {
					response = record.Raw
				}
			}
			got := receipts[i]
			if got.Direction != direction || got.RequestBytes != uint64(len(request)) || got.ResponseBytes != uint64(len(response)) || got.RequestSHA256 != sha256.Sum256(request) || got.ResponseSHA256 != sha256.Sum256(response) {
				t.Fatalf("ASSERT_CORRECTION_GUARD_RECORD_WIRE_PRIVATE_AUTHENTIC_BYTES[%s]: receipt=%+v request=%d response=%d", direction, got, len(request), len(response))
			}
		}
		if reflect.DeepEqual(result.TraversalPrivate(), operation.Result{}) || len(result.OutgoingPrivate()) == 0 {
			t.Fatalf("ASSERT_CORRECTION_GUARD_RETURNED_TYPED_DIRECTIONAL_RESULTS: traversal=%+v outgoing=%+v", result.TraversalPrivate(), result.OutgoingPrivate())
		}
		assertCorrectionGuardReleasedOncePrivate(t, root)
	})

	cases := []struct {
		name     string
		incoming string
		outgoing string
		plan     directionalraw.BidirectionalPlanPrivate
		mutate   func(*incomingops.ProductionPrivateRequest)
		context  func(*scriptedTransportPrivate) context.Context
	}{
		{name: "cancel-before", incoming: "nonempty", outgoing: "nonempty", plan: directionalraw.BidirectionalPlanPrivate{IncomingBytes: 4096, OutgoingBytes: 4096}, context: func(_ *scriptedTransportPrivate) context.Context {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			return ctx
		}},
		{name: "incoming-preflight", incoming: "nonempty", outgoing: "nonempty", plan: directionalraw.BidirectionalPlanPrivate{IncomingBytes: directionalraw.DirectionLimitPrivate + 1, OutgoingBytes: 1}},
		{name: "invalid-input", incoming: "nonempty", outgoing: "nonempty", plan: directionalraw.BidirectionalPlanPrivate{IncomingBytes: 4096, OutgoingBytes: 4096}, mutate: func(req *incomingops.ProductionPrivateRequest) { req.Operation.Input = []byte(`{"session_id":`) }},
		{name: "unknown-session", incoming: "nonempty", outgoing: "nonempty", plan: directionalraw.BidirectionalPlanPrivate{IncomingBytes: 4096, OutgoingBytes: 4096}, mutate: func(req *incomingops.ProductionPrivateRequest) {
			req.Operation.Input = []byte(`{"session_id":"missing","generation":1,"uri":"file:///missing.go","symbol":"Target"}`)
		}},
		{name: "incoming-runtime-error", incoming: "error", outgoing: "nonempty", plan: directionalraw.BidirectionalPlanPrivate{IncomingBytes: 4096, OutgoingBytes: 4096}},
		{name: "cancel-between", incoming: "nonempty", outgoing: "nonempty", plan: directionalraw.BidirectionalPlanPrivate{IncomingBytes: 4096, OutgoingBytes: 4096}, context: func(child *scriptedTransportPrivate) context.Context {
			ctx, cancel := context.WithCancel(context.Background())
			child.cancelAfterIncoming = cancel
			return ctx
		}},
		{name: "outgoing-preflight", incoming: "nonempty", outgoing: "nonempty", plan: directionalraw.BidirectionalPlanPrivate{IncomingBytes: 4096, OutgoingBytes: directionalraw.DirectionLimitPrivate + 1}},
		{name: "outgoing-runtime-error", incoming: "nonempty", outgoing: "error", plan: directionalraw.BidirectionalPlanPrivate{IncomingBytes: 4096, OutgoingBytes: 4096}},
		{name: "coordinator-raw-mismatch", incoming: "nonempty", outgoing: "nonempty", plan: directionalraw.BidirectionalPlanPrivate{IncomingBytes: 4096, OutgoingBytes: 4096}, mutate: func(req *incomingops.ProductionPrivateRequest) { req.Coordinate.RawAttempt = []byte(`{}`) }},
	}
	for _, tc := range cases {
		t.Run("release/"+tc.name, func(t *testing.T) {
			child := newScriptedTransportPrivate(tc.incoming, tc.outgoing, false)
			manager, started, uri := readyManagerPrivate(t, child)
			req := privateRequestPrivate(t, started, uri, tc.plan, false)
			root := req.Coordinate.Root
			if tc.mutate != nil {
				tc.mutate(&req)
			}
			ctx := context.Background()
			if tc.context != nil {
				ctx = tc.context(child)
			}
			entry, err := incomingops.NewADR0011ADR0007ProductionPrivate(incomingops.NewExecutor(manager))
			if err != nil {
				t.Fatal(err)
			}
			_, _ = entry.ExecutePrivate(ctx, req)
			assertCorrectionGuardReleasedOncePrivate(t, root)
		})
	}

	t.Run("release/prepared-document-failure", func(t *testing.T) {
		child := newScriptedTransportPrivate("nonempty", "nonempty", false)
		manager, started, uri := readyManagerPrivate(t, child)
		req := privateRequestPrivate(t, started, uri, directionalraw.BidirectionalPlanPrivate{IncomingBytes: 4096, OutgoingBytes: 4096}, false)
		req.Operation.Input = bytes.ReplaceAll(req.Operation.Input, []byte(uri), []byte("file:///private/tmp/adr0011-fixed-workspace/missing.go"))
		root := req.Coordinate.Root
		entry, err := incomingops.NewADR0011ADR0007ProductionPrivate(incomingops.NewExecutor(manager))
		if err != nil {
			t.Fatal(err)
		}
		result, execErr := entry.ExecutePrivate(context.Background(), req)
		if execErr != nil || result.FailurePrivate() == nil || result.FailurePrivate().Code != string(sessionruntime.DocumentSourceUnavailable) || result.CoordinationPrivate().SuppressionPrivate() == nil || result.CoordinationPrivate().SuppressionPrivate().Code != string(sessionruntime.DocumentSourceUnavailable) || result.CoordinationPrivate().SuppressionPrivate().Stage != "document_preparation" || result.CoordinationPrivate().SuppressionPrivate().Direction != "incoming" {
			t.Fatalf("ASSERT_CORRECTION_GUARD_PREPARED_DOCUMENT_EXACT_FAILURE: result=%+v err=%v", result, execErr)
		}
		if child.countFor("textDocument/documentSymbol", "request") != 0 || child.countFor("textDocument/prepareCallHierarchy", "request") != 0 || child.countFor("callHierarchy/incomingCalls", "request") != 0 || child.countFor("callHierarchy/outgoingCalls", "request") != 0 {
			t.Fatalf("ASSERT_CORRECTION_GUARD_PREPARED_DOCUMENT_ZERO_DOWNSTREAM_CALLS: %+v", child.snapshot())
		}
		assertCorrectionGuardReleasedOncePrivate(t, root)
	})

	t.Run("release/incoming-target-resolution-failure", func(t *testing.T) {
		child := newScriptedTransportPrivate("nonempty", "nonempty", false)
		manager, started, uri := readyManagerPrivate(t, child)
		req := privateRequestPrivate(t, started, uri, directionalraw.BidirectionalPlanPrivate{IncomingBytes: 4096, OutgoingBytes: 4096}, false)
		req.Operation.Input = bytes.ReplaceAll(req.Operation.Input, []byte(`"symbol":"Target"`), []byte(`"symbol":"Missing"`))
		root := req.Coordinate.Root
		entry, err := incomingops.NewADR0011ADR0007ProductionPrivate(incomingops.NewExecutor(manager))
		if err != nil {
			t.Fatal(err)
		}
		result, execErr := entry.ExecutePrivate(context.Background(), req)
		if execErr != nil || result.FailurePrivate() == nil || result.FailurePrivate().Code != "DIRECTIONAL_RELEASE_FAILED" || result.CoordinationPrivate().SuppressionPrivate() == nil || result.CoordinationPrivate().SuppressionPrivate().Code != "DIRECTIONAL_RELEASE_FAILED" || result.CoordinationPrivate().SuppressionPrivate().Stage != "document_preparation" || result.CoordinationPrivate().SuppressionPrivate().Direction != "incoming" {
			t.Fatalf("ASSERT_CORRECTION_GUARD_UNATTACHED_MANAGER_RELEASE_FAILURE_TRUTH: result=%+v err=%v", result, execErr)
		}
		if child.countFor("textDocument/documentSymbol", "request") != 1 || child.countFor("textDocument/prepareCallHierarchy", "request") != 0 || child.countFor("callHierarchy/incomingCalls", "request") != 0 || child.countFor("callHierarchy/outgoingCalls", "request") != 0 {
			t.Fatalf("ASSERT_CORRECTION_GUARD_INCOMING_TARGET_CAUSAL_CALLS: %+v", child.snapshot())
		}
		assertCorrectionGuardReleasedOncePrivate(t, root)
	})

	t.Run("single-manager-preparation-reused-exactly", func(t *testing.T) {
		child := newScriptedTransportPrivate("nonempty", "nonempty", false)
		manager, started, uri := readyManagerPrivate(t, child)
		req := privateRequestPrivate(t, started, uri, directionalraw.BidirectionalPlanPrivate{IncomingBytes: 4096, OutgoingBytes: 4096}, false)
		root := req.Coordinate.Root
		result := executePrivate(t, context.Background(), manager, req)
		if result.FailurePrivate() != nil || !result.CoordinationPrivate().AcceptedPrivate() {
			t.Fatalf("ASSERT_MANAGER_OWNED_SINGLE_PREPARE_SUCCESS: %+v", result)
		}
		if child.countFor("textDocument/documentSymbol", "request") != 1 || child.countFor("textDocument/prepareCallHierarchy", "request") != 1 || child.countFor("callHierarchy/incomingCalls", "request") != 1 || child.countFor("callHierarchy/outgoingCalls", "request") != 1 {
			t.Fatalf("ASSERT_MANAGER_OWNED_SINGLE_PREPARE_COUNTS: %+v", child.snapshot())
		}
		var incomingItem, outgoingItem json.RawMessage
		for _, record := range child.snapshot() {
			if record.Kind != "request" || (record.Method != "callHierarchy/incomingCalls" && record.Method != "callHierarchy/outgoingCalls") {
				continue
			}
			body := record.Raw
			if split := bytes.Index(body, []byte("\r\n\r\n")); split >= 0 {
				body = body[split+4:]
			}
			var envelope struct {
				Params struct {
					Item json.RawMessage `json:"item"`
				} `json:"params"`
			}
			if err := json.Unmarshal(body, &envelope); err != nil {
				t.Fatal(err)
			}
			if record.Method == "callHierarchy/incomingCalls" {
				incomingItem = envelope.Params.Item
			} else {
				outgoingItem = envelope.Params.Item
			}
		}
		if len(incomingItem) == 0 || !bytes.Equal(incomingItem, outgoingItem) {
			t.Fatalf("ASSERT_MANAGER_OWNED_EXACT_PREPARED_IDENTITY_REUSED: incoming=%s outgoing=%s", incomingItem, outgoingItem)
		}
		assertCorrectionGuardReleasedOncePrivate(t, root)
	})
}
