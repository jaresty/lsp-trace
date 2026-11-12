package sessionruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"runtime/trace"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

func privateMessageFixtureBytes(t *testing.T, burst []byte, count uint64) uint64 {
	t.Helper()
	reader := lspwire.NewReader(bytes.NewReader(burst), lspwire.DefaultLimits())
	var total uint64
	for i := uint64(0); i < count; i++ {
		_, frame, err := reader.ReadWithFrame()
		if err != nil || frame.FrameBytes <= 0 {
			t.Fatal("fixture original-frame accounting", err)
		}
		total += uint64(frame.FrameBytes)
	}
	return total
}

func TestPrivateProviderMessageWindow(t *testing.T) {
	t.Log("PROCEDURE: scripted owned-window bursts; compare decoded/refused/dispatched/selected and original wire bytes against independent fixture reads")
	for _, tc := range []struct {
		name                          string
		n, unmatched, globalRemaining int
		frames, dispatched            uint64
		boundary                      uint8
		selected                      bool
	}{
		{"three", 3, 0, 0, 4, 4, 0, true},
		{"four", 4, 0, 0, 5, 5, 0, true},
		{"fifth-notification", 5, 0, 0, 5, 4, privateMessageClassRefused, false},
		{"fifth-matching-control", 0, 4, 0, 5, 4, privateMessageClassRefused, false},
		{"fifth-unmatched-control", 0, 5, 0, 5, 4, privateMessageClassRefused, false},
		{"mixed-eight", 4, 3, 0, 8, 8, 0, true},
		{"ninth-admission", 4, 4, 0, 8, 8, privateMessageFrameAdmissionRefused, false},
		{"global-admission", 3, 0, 2, 2, 2, privateMessageFrameAdmissionRefused, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, start, ctx, c, scripted := frameInterleavingSetupConfig(t, frameInterleavingConfig{notifications: tc.n, unmatched: tc.unmatched})
			st := windowTestReady(t, m, start, ctx)
			if d := m.PrepareDocument(ctx, DocumentRequest{SessionID: st.SessionID, Generation: st.Generation, URI: c.uri, LanguageID: "go"}); d.Failure != "" {
				t.Fatal(d.Failure)
			}
			w := privateProviderWindowFrom(ctx)
			w.mu.Lock()
			if tc.globalRemaining != 0 {
				w.frames = w.spec.bounds.Frames - uint64(tc.globalRemaining)
			}
			before := w.frames
			w.mu.Unlock()
			var got RoundTripResult
			var cancelled bool
			err := m.WithReadySessionLease(ctx, SessionLeaseRequest{SessionID: st.SessionID, Generation: st.Generation, Workspace: start.Profile.Workspace().String()}, func(lease context.Context) error {
				got = m.RoundTrip(lease, RoundTripRequest{SessionID: st.SessionID, Generation: st.Generation, Method: "textDocument/documentSymbol", Params: json.RawMessage(fmt.Sprintf(`{"textDocument":{"uri":%q}}`, c.uri)), Deadline: start.Deadline, MaxMessages: 4, MaxBytes: 65536, CaptureProviderMessageCompositionAllMethods: true, CaptureProviderTransactionObservationAllMethods: true})
				cancelled = lease.Err() != nil
				return nil
			})
			script := <-scripted
			e := got.privateProviderMessages
			w.mu.Lock()
			debit := w.frames - before
			w.mu.Unlock()
			if !e.Present || e.Admissions != tc.frames || e.Dispatched != tc.dispatched || e.Selected != tc.selected || e.Boundary != tc.boundary || e.Incomplete || e.Join != 0 || e.Overflow || e.Truncated || uint64(e.EventCount) != tc.frames || debit != tc.frames {
				t.Fatalf("ASSERT_MESSAGE_COVERAGE: FAIL %s evidence=%+v debit=%d", tc.name, e, debit)
			}
			wantN := uint64(tc.n)
			if wantN > tc.frames {
				wantN = tc.frames
			}
			if e.Decoded != [6]uint64{wantN, 0, 0, 0, 0, tc.frames - wantN} {
				t.Fatalf("ASSERT_MESSAGE_COVERAGE: FAIL decoded %v", e.Decoded)
			}
			wantBytes := privateMessageFixtureBytes(t, script.burst, tc.frames)
			if e.FrameBytes != wantBytes {
				t.Fatalf("ASSERT_ORIGINAL_FRAME_BYTES: FAIL got=%d want=%d", e.FrameBytes, wantBytes)
			}
			for i := uint8(0); i < e.EventCount; i++ {
				wantDisposition := uint8(1)
				if i == e.EventCount-1 {
					if tc.selected {
						wantDisposition = 2
					}
					if tc.boundary == privateMessageClassRefused {
						wantDisposition = 3
					}
				}
				if e.Events[i].Ordinal != i+1 || e.Events[i].Disposition != wantDisposition {
					t.Fatalf("ASSERT_MESSAGE_COVERAGE: FAIL order/disposition %+v", e.Events)
				}
			}
			s := privateProviderMessageSummaryValue(got)
			if !s.Composition.Present || !s.Transaction.Present || s.Transaction.Flags[0] != tc.selected || s.Transaction.Processed[0] != uint64(got.Messages) {
				t.Fatal("ASSERT_DETACHED_OBSERVATIONS: FAIL")
			}
			if tc.selected {
				if got.Failure != "" || cancelled || err != nil || len(got.Result) == 0 {
					t.Fatal("ASSERT_MESSAGE_PARITY: FAIL success")
				}
			} else if got.Failure != session.ResourceExhausted || !cancelled || len(got.Result) != 0 {
				t.Fatal("ASSERT_MESSAGE_PARITY: FAIL original refusal")
			}
			t.Logf("ASSERT_MESSAGE_COVERAGE: PASS %s decoded=%v dispatched=%d selected=%t boundary=%d", tc.name, e.Decoded, e.Dispatched, e.Selected, e.Boundary)
			t.Logf("ASSERT_ORIGINAL_FRAME_BYTES: PASS bytes=%d", wantBytes)
			t.Log("ASSERT_DETACHED_OBSERVATIONS: PASS")
			t.Log("ASSERT_MESSAGE_PARITY: PASS")
		})
	}
}

func TestPrivateProviderMessageFixedValue(t *testing.T) {
	t.Log("PROCEDURE: direct fixed-value boundary, closed bucket and hostile payload canaries; no provider")
	var absent privateProviderMessageEvidence
	zero := privateProviderMessageEvidence{enabled: true}
	zero.stop(privateMessageFrameAdmissionRefused)
	if absent.Present || !zero.Present || zero.Admissions != 0 || zero.Decoded != [6]uint64{} {
		t.Fatal("ASSERT_MESSAGE_AVAILABILITY: FAIL absent/observed-zero")
	}
	if reflect.TypeOf(absent.Events).Kind() != reflect.Array || len(absent.Events) != 8 {
		t.Fatal("ASSERT_FIXED_CAPS: FAIL array")
	}
	e := privateProviderMessageEvidence{enabled: true}
	canary := "PRIVATE_CANARY:/workspace/secret.go:method:params:rpc-error"
	methods := []string{"$/progress", "textDocument/publishDiagnostics", "window/logMessage", "window/showMessage", "telemetry/event", canary}
	for _, method := range methods {
		e.admitted()
		e.decoded(lspwire.Message{JSONRPC: lspwire.Version, Method: method, Params: json.RawMessage(`{"secret":"` + canary + `"}`)}, 137, false)
		e.dispatched(false)
	}
	e.admitted()
	e.decoded(lspwire.Message{JSONRPC: lspwire.Version, Method: canary, ID: json.RawMessage(`77`), Params: json.RawMessage(`{}`)}, 31, false)
	e.stop(privateMessageRequestRefused)
	e.admitted()
	e.decoded(lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(`88`), Error: &lspwire.RPCError{Code: -1, Message: canary}}, 41, false)
	e.stop(privateMessageClassRefused)
	if e.Decoded != [6]uint64{1, 1, 3, 1, 1, 1} || e.EventCount != 8 || e.Truncated || e.Events[6].Disposition != 3 || e.Events[7].Disposition != 3 || e.Selected {
		t.Fatal("ASSERT_CLOSED_BUCKETS: FAIL", e)
	}
	e.admitted()
	e.decoded(lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(`99`), Result: json.RawMessage(`null`)}, 2, false)
	e.dispatched(false)
	if e.EventCount != 8 || !e.Truncated || e.Events[7].Ordinal != 8 || e.Events[7].Disposition != 3 {
		t.Fatal("ASSERT_FIXED_CAPS: FAIL truncation")
	}
	r := RoundTripResult{privateProviderMessages: e, Result: json.RawMessage(`{"secret":"` + canary + `"}`), ServerError: &lspwire.RPCError{Message: canary}, Failure: session.Failure(canary), providerMessageCompositionPresent: true, providerMessageComposition: ProviderMessageComposition{Outcome: ProviderMessageOutcome(canary), InvalidProtocolCategory: ProviderInvalidProtocolCategory(canary)}}
	text := privateProviderMessageSummaryJSON(r)
	if strings.Contains(text, canary) || strings.Contains(text, "$/progress") || strings.Contains(text, "publishDiagnostics") || strings.Contains(text, "workspace") {
		t.Fatal("ASSERT_MESSAGE_PRIVACY: FAIL", text)
	}
	var s privateProviderMessageSummary
	if json.Unmarshal([]byte(text), &s) != nil || s.Composition.Outcome != 0 || s.Composition.Invalid != 0 || !s.Evidence.Truncated {
		t.Fatal("ASSERT_MESSAGE_PRIVACY: FAIL schema")
	}
	// Direct arithmetic boundaries are reachable without unbounded physical reads.
	over := privateProviderMessageEvidence{enabled: true, Admissions: math.MaxUint64, FrameBytes: math.MaxUint64 - 1, Dispatched: math.MaxUint64, Decoded: [6]uint64{math.MaxUint64}}
	over.admitted()
	over.decoded(lspwire.Message{JSONRPC: lspwire.Version, Method: "$/progress"}, 2, false)
	over.dispatched(false)
	if !over.Overflow || over.Admissions != math.MaxUint64 || over.FrameBytes != math.MaxUint64 || over.Dispatched != math.MaxUint64 || over.Decoded[0] != math.MaxUint64 {
		t.Fatal("ASSERT_FIXED_CAPS: FAIL saturation")
	}
	unknown := privateProviderMessageEvidence{enabled: true}
	unknown.admitted()
	unknown.stop(privateMessageReadFailed)
	if !unknown.Present || !unknown.Incomplete || unknown.FrameBytes != 0 || unknown.Boundary != privateMessageReadFailed {
		t.Fatal("ASSERT_READ_BOUNDARY: FAIL")
	}
	unknown.joined(lspwire.Message{}, 0, true)
	if unknown.Join != 2 || unknown.Selected || unknown.Dispatched != 0 || !unknown.Incomplete {
		t.Fatal("ASSERT_JOIN_COVERAGE: FAIL unclassified")
	}
	for _, boundary := range []uint8{privateMessageRequestRefused, privateMessageWorkRefused, privateMessageByteRefused, privateMessageClassRefused, privateMessageProtocolRefused, privateMessageRuntimeRefused} {
		x := privateProviderMessageEvidence{enabled: true}
		x.admitted()
		x.decoded(lspwire.Message{JSONRPC: lspwire.Version, Method: "$/progress"}, 1, false)
		x.stop(boundary)
		if x.Boundary != boundary || x.Events[0].Disposition != 3 || x.Dispatched != 0 {
			t.Fatal("ASSERT_READ_BOUNDARY: FAIL refused", boundary)
		}
	}
	// Disabled instrumentation is a fixed zero value and adds no diagnostic allocation.
	msg := lspwire.Message{JSONRPC: lspwire.Version, Method: "$/progress"}
	if allocations := testing.AllocsPerRun(100, func() {
		absent.admitted()
		absent.decoded(msg, 10, false)
		absent.stop(privateMessageReadFailed)
		absent.dispatched(true)
		absent.joined(msg, 10, false)
	}); allocations != 0 || absent != (privateProviderMessageEvidence{}) {
		t.Fatal("ASSERT_MESSAGE_DISABLED: FAIL", allocations)
	}
	for _, assertion := range []string{"ASSERT_MESSAGE_AVAILABILITY", "ASSERT_CLOSED_BUCKETS", "ASSERT_MESSAGE_PRIVACY", "ASSERT_FIXED_CAPS", "ASSERT_READ_BOUNDARY", "ASSERT_JOIN_COVERAGE", "ASSERT_MESSAGE_DISABLED"} {
		t.Log(assertion + ": PASS")
	}
}

func TestPrivateProviderMessageSummaryBound(t *testing.T) {
	t.Log("PROCEDURE: maximum-width numeric fields, longer false booleans, fixed eight records; marshal and strict round-trip")
	r := RoundTripResult{privateProviderMessages: privateProviderMessageEvidence{Present: true, Admissions: math.MaxUint64, FrameBytes: math.MaxUint64, Dispatched: math.MaxUint64, Boundary: 255, Join: 255, EventCount: 255}, providerMessageCompositionPresent: true}
	for i := range r.privateProviderMessages.Decoded {
		r.privateProviderMessages.Decoded[i] = math.MaxUint64
	}
	for i := range r.privateProviderMessages.Events {
		r.privateProviderMessages.Events[i] = privateProviderMessageEvent{255, 255, 255}
	}
	r.providerMessageComposition = ProviderMessageComposition{ProgressNotifications: math.MaxUint64, LogShowTelemetryNotifications: math.MaxUint64, PublishDiagnosticsNotifications: math.MaxUint64, OtherNotifications: math.MaxUint64, ServerRequests: math.MaxUint64, UnmatchedLateInvalidResponses: math.MaxUint64, AcceptedTerminalResponses: math.MaxUint64, InvalidProtocolMessages: math.MaxUint64, Outcome: ProviderMessageOutcomeOtherRuntimeFailure, InvalidProtocolCategory: ProviderInvalidProtocolNonemptyHeaderMissingLength}
	r.providerTransaction.present = true
	r.providerTransaction.observation = ProviderTransactionObservation{CompletionSequence: math.MaxUint64, ProviderMessagesProcessed: math.MaxUint64, NotificationMessagesProcessed: math.MaxUint64, ControlMessagesProcessed: math.MaxUint64, ConfiguredControlCeiling: math.MaxUint64, ConfiguredMaxBytes: math.MaxUint64, NotificationCeilingPolicy: ProviderNotificationCeilingConfiguredCount, TerminalLifecycle: ProviderTransactionLifecyclePoisoned}
	text := privateProviderMessageSummaryJSON(r)
	var s privateProviderMessageSummary
	if len(text) > 2048 || json.Unmarshal([]byte(text), &s) != nil || s.Version != 1 || s.Transaction.Completion != math.MaxUint64 || s.Evidence.FrameBytes != math.MaxUint64 {
		t.Fatal("ASSERT_SUMMARY_BOUND: FAIL", len(text), text)
	}
	t.Logf("ASSERT_SUMMARY_BOUND: PASS max-width bytes=%d <=2048", len(text))
}

func TestPrivateProviderMessageCancellationJoin(t *testing.T) {
	for _, fault := range []string{"", "malformed"} {
		t.Run("fault="+fault, func(t *testing.T) { privateProviderMessageCancellationJoin(t, fault) })
	}
}

func privateProviderMessageCancellationJoin(t *testing.T, fault string) {
	t.Helper()
	t.Log("PROCEDURE: block decode hook after complete frame, cancel while read is held, wait owned transport closure, release and join; never dispatch/accept joined matching response")
	m, start, ctx, c, scripted := frameInterleavingSetupConfig(t, frameInterleavingConfig{fault: fault})
	st := windowTestReady(t, m, start, ctx)
	if d := m.PrepareDocument(ctx, DocumentRequest{SessionID: st.SessionID, Generation: st.Generation, URI: c.uri, LanguageID: "go"}); d.Failure != "" {
		t.Fatal(d.Failure)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	m.cDecodeEntry = func() { close(entered); <-release }
	// Keep the existing owned wrapper's parent live. Cancel only the local
	// RoundTrip request; canceling the outer wrapper tests FAILEDUNJOINED,
	// not this local joined-read coverage seam.
	cancelReady := make(chan context.CancelFunc, 1)
	result := make(chan RoundTripResult, 1)
	go func() {
		result <- privateProviderWindowRun(privateProviderWindowFrom(ctx), ctx, RoundTripResult{Failure: privateProviderWindowUnjoined}, func(inner context.Context) RoundTripResult {
			requestCtx, cancel := context.WithCancel(inner)
			defer cancel()
			cancelReady <- cancel
			return m.RoundTrip(requestCtx, RoundTripRequest{SessionID: st.SessionID, Generation: st.Generation, Method: "textDocument/documentSymbol", Deadline: start.Deadline, MaxMessages: 4, MaxBytes: 65536})
		})
	}()
	cancel := <-cancelReady
	defer cancel()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("decode not entered")
	}
	cancel()
	select {
	case <-c.done:
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("transport not retired before releasing decode")
	}
	close(release)
	var got RoundTripResult
	select {
	case got = <-result:
	case <-time.After(2 * time.Second):
		t.Fatal("reader not joined")
	}
	<-scripted
	e := got.privateProviderMessages
	if got.Failure != session.RequestCancelled || len(got.Result) != 0 || got.providerTransaction.selectedResponseObserved || !e.Present || e.Admissions != 1 || e.Selected || e.Dispatched != 0 || e.Boundary != privateMessageCancelledJoin {
		t.Fatalf("ASSERT_JOIN_COVERAGE: FAIL failure=%s evidence=%+v", got.Failure, e)
	}
	if fault == "malformed" {
		if e.Join != 2 || !e.Incomplete || e.Decoded != [6]uint64{} || e.EventCount != 0 || e.FrameBytes != 0 {
			t.Fatalf("ASSERT_JOIN_COVERAGE: FAIL unclassified evidence=%+v", e)
		}
		t.Log("ASSERT_JOIN_COVERAGE: PASS joined unclassified read explicitly incomplete; REQUEST_CANCELLED preserved")
	} else {
		if e.Join != 1 || e.Decoded[5] != 1 || e.EventCount != 1 || e.Events[0].Disposition != 4 || e.Incomplete || e.FrameBytes == 0 {
			t.Fatalf("ASSERT_JOIN_COVERAGE: FAIL decoded evidence=%+v", e)
		}
		t.Log("ASSERT_JOIN_COVERAGE: PASS joined decoded response remains unselected and nondispatched; REQUEST_CANCELLED preserved")
	}
}

func TestPrivateProviderMessageGenericParity(t *testing.T) {
	t.Log("PROCEDURE: generic controlled response with legacy all-method observations enabled; no native-window evidence")
	m, s, child := roundTripManager(t, "eof")
	defer child.Close()
	got := m.RoundTrip(context.Background(), RoundTripRequest{SessionID: s.SessionID, Generation: s.Generation, Method: "textDocument/documentSymbol", Deadline: time.Now().Add(time.Second), MaxMessages: 4, MaxBytes: 65536, CaptureProviderMessageCompositionAllMethods: true, CaptureProviderTransactionObservationAllMethods: true})
	if got.Failure != session.SessionCrashed || got.privateProviderMessages != (privateProviderMessageEvidence{}) {
		t.Fatal("ASSERT_MESSAGE_DISABLED: FAIL generic")
	}
	summary := privateProviderMessageSummaryValue(got)
	if summary.Evidence.Present || !summary.Composition.Present || !summary.Transaction.Present || summary.Composition.Outcome != 6 {
		t.Fatal("ASSERT_MESSAGE_AVAILABILITY: FAIL generic detached observations")
	}
	t.Log("ASSERT_MESSAGE_DISABLED: PASS generic original failure and zero evidence")
}

func TestPrivateProviderMessageFailureTrace(t *testing.T) {
	t.Log("PROCEDURE: scripted fifth notification under active lease; manual checkpoint, not full capture integration; preserve failure/cancellation and emit fixed summary")
	m, start, ctx, c, scripted := frameInterleavingSetup(t, 5)
	st := windowTestReady(t, m, start, ctx)
	if d := m.PrepareDocument(ctx, DocumentRequest{SessionID: st.SessionID, Generation: st.Generation, URI: c.uri, LanguageID: "go"}); d.Failure != "" {
		t.Fatal(d.Failure)
	}
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
	var got RoundTripResult
	var sampled error
	_ = m.WithReadySessionLease(ctx, SessionLeaseRequest{SessionID: st.SessionID, Generation: st.Generation, Workspace: start.Profile.Workspace().String()}, func(lease context.Context) error {
		got = m.RoundTrip(lease, RoundTripRequest{SessionID: st.SessionID, Generation: st.Generation, Method: "textDocument/documentSymbol", Deadline: start.Deadline, MaxMessages: 4, MaxBytes: 65536, CaptureProviderMessageCompositionAllMethods: true, CaptureProviderTransactionObservationAllMethods: true})
		sampled = lease.Err()
		privatePrepareTraceTransaction(lease, sampled, got.Failure, got.ServerError != nil, got.ownedMethodPair != nil)
		privatePrepareTraceMessages(lease, got)
		return sampled
	})
	<-scripted
	trace.Stop()
	stopped = true
	if got.Failure != session.ResourceExhausted || sampled != context.Canceled || got.privateProviderMessages.Decoded[0] != 5 || got.privateProviderMessages.Dispatched != 4 || got.privateProviderMessages.Selected || !bytes.Contains(storage.Bytes(), []byte("private.prepare.messages")) || storage.Len() > 8<<20 {
		t.Fatal("ASSERT_FAILURE_SUMMARY: FAIL")
	}
	if path := os.Getenv("LSP_TRACE_PRIVATE_MESSAGE_TEST_TRACE_OUTPUT"); path != "" {
		if err := os.WriteFile(path, storage.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Log("ASSERT_FAILURE_SUMMARY: PASS RESOURCE_EXHAUSTED / CANCELED, decoded=5 dispatched=4 selected=false; manual checkpoint")
}

func TestPrivateProviderMessageReadBoundaries(t *testing.T) {
	t.Log("PROCEDURE: actual malformed and unsupported server-request reads in scripted native window")
	for _, fault := range []string{"malformed", "request"} {
		t.Run(fault, func(t *testing.T) {
			m, start, ctx, c, scripted := frameInterleavingSetupConfig(t, frameInterleavingConfig{fault: fault})
			st := windowTestReady(t, m, start, ctx)
			if d := m.PrepareDocument(ctx, DocumentRequest{SessionID: st.SessionID, Generation: st.Generation, URI: c.uri, LanguageID: "go"}); d.Failure != "" {
				t.Fatal(d.Failure)
			}
			var got RoundTripResult
			_ = m.WithReadySessionLease(ctx, SessionLeaseRequest{SessionID: st.SessionID, Generation: st.Generation, Workspace: start.Profile.Workspace().String()}, func(lease context.Context) error {
				got = m.RoundTrip(lease, RoundTripRequest{SessionID: st.SessionID, Generation: st.Generation, Method: "textDocument/documentSymbol", Deadline: start.Deadline, MaxMessages: 4, MaxBytes: 65536})
				return nil
			})
			script := <-scripted
			e := got.privateProviderMessages
			if !e.Present || e.Admissions != 1 || e.Selected || e.Dispatched != 0 {
				t.Fatal("ASSERT_READ_BOUNDARY: FAIL availability/dispatch", e)
			}
			if fault == "malformed" {
				if got.Failure != session.SessionPoisoned || e.Boundary != privateMessageReadFailed || !e.Incomplete || e.FrameBytes != 0 || e.EventCount != 0 {
					t.Fatal("ASSERT_READ_BOUNDARY: FAIL malformed", e)
				}
			} else if got.Failure != session.ToolNotImplemented || e.Boundary != privateMessageRequestRefused || e.Incomplete || e.Decoded[4] != 1 || e.Events[0].Disposition != 3 || e.FrameBytes != uint64(len(script.burst)) {
				t.Fatal("ASSERT_READ_BOUNDARY: FAIL server request", e)
			}
			t.Log("ASSERT_READ_BOUNDARY: PASS", fault)
		})
	}
}

func TestPrivateProviderMessageLogicalBudget(t *testing.T) {
	t.Log("PROCEDURE: count serialized leaves; derive seven plans from constructor only; check conservative global upper bounds, not exact traffic counts, and pure 80/81 sensitivity; no window entry or provider execution")
	var value any
	if err := json.Unmarshal([]byte(privateProviderMessageSummaryJSON(RoundTripResult{})), &value); err != nil {
		t.Fatal(err)
	}
	var leaves func(any) int
	leaves = func(v any) int {
		switch v := v.(type) {
		case map[string]any:
			n := 0
			for _, x := range v {
				n += leaves(x)
			}
			return n
		case []any:
			n := 0
			for _, x := range v {
				n += leaves(x)
			}
			return n
		default:
			return 1
		}
	}
	// Keep the 67-leaf guard linked to four closed enum switches and nine
	// wrapper/guard units. Each summarized transaction has at most eight
	// frames at nine units each, this format allowance, and one boundary.
	const formatCost = uint64(80)
	scalars := leaves(value)
	perTransaction := 8*9 + formatCost + 1
	if scalars != 67 || scalars+4+9 > int(formatCost) || perTransaction > 256 || 18+2 > 32 || 24+3 > 32 {
		t.Fatal("ASSERT_LOGICAL_BUDGET: FAIL retained guards", scalars, perTransaction)
	}
	// This adapter is used only for constructor admission, never Start.
	// Do not use windowTestSetup: this test must not serve a transport.
	m, err := New(Config{Limits: Limits{MaxSessions: 2, MaxRequests: 4, MaxChildren: 16, MaxCancels: 8, MaxTombstones: 16, MaxObservations: 64, MaxOperations: 8}, Starter: &windowPipeStarter{}})
	if err != nil {
		t.Fatal("ASSERT_GLOBAL_BUDGET_CONSTRUCTOR: FAIL", err)
	}
	start := StartRequest{Deadline: time.Now().Add(time.Minute)}
	cases := [...]struct {
		name string
		plan PrivateProviderWindowPlan
		want [4]uint64 // S, X, F, conservative full cost
	}{
		{"C", PrivateProviderWindowPlan{Calls: true}, [4]uint64{1, 6, 24, 222}},
		{"R", PrivateProviderWindowPlan{References: true}, [4]uint64{2, 4, 16, 340}},
		{"D", PrivateProviderWindowPlan{Definitions: true}, [4]uint64{2, 4, 16, 340}},
		{"CR", PrivateProviderWindowPlan{Calls: true, References: true}, [4]uint64{2, 7, 28, 367}},
		{"CD", PrivateProviderWindowPlan{Calls: true, Definitions: true}, [4]uint64{2, 7, 28, 367}},
		{"RD", PrivateProviderWindowPlan{References: true, Definitions: true}, [4]uint64{3, 5, 20, 493}},
		{"CRD", PrivateProviderWindowPlan{Calls: true, References: true, Definitions: true}, [4]uint64{3, 8, 32, 512}},
	}
	var counts [7][3]uint64
	var maxSummaries, maxExchanges, maxFrames uint64
	// Validate all constructor counts before using their finite maxima as
	// arithmetic limits. This calls no Start and serves no transport.
	for i, tc := range cases {
		envelope, err := m.PrivateProviderWindowEnvelope(start, tc.plan)
		if err != nil {
			t.Fatalf("ASSERT_GLOBAL_BUDGET_CONSTRUCTOR: FAIL plan=%s error=%v", tc.name, err)
		}
		bounds := envelope.Bounds()
		// RequestBytes=(X+preparations+2)*(frame+1). Invert funding
		// independently of Frames=4*X, guarding each unsigned operation.
		if bounds.FrameBytes == ^uint64(0) || bounds.SourcePreparations > ^uint64(0)-2 {
			t.Fatalf("ASSERT_GLOBAL_BUDGET_CONSTRUCTOR: FAIL plan=%s funding inversion overflow", tc.name)
		}
		frameUnit := bounds.FrameBytes + 1
		base := bounds.SourcePreparations + 2
		if bounds.RequestBytes%frameUnit != 0 || bounds.RequestBytes/frameUnit < base {
			t.Fatalf("ASSERT_GLOBAL_BUDGET_CONSTRUCTOR: FAIL plan=%s invalid funding inversion", tc.name)
		}
		exchanges := bounds.RequestBytes/frameUnit - base
		summaries := uint64(1)
		if tc.plan.References {
			summaries++
		}
		if tc.plan.Definitions {
			summaries++
		}
		if summaries != tc.want[0] || exchanges != tc.want[1] || bounds.Frames != tc.want[2] {
			t.Fatalf("ASSERT_GLOBAL_BUDGET_CONSTRUCTOR: FAIL plan=%s S=%d X=%d F=%d", tc.name, summaries, exchanges, bounds.Frames)
		}
		// The checked table bounds X by eight before multiplying it.
		if bounds.Frames != 4*exchanges || bounds.FramesPerExchange != 8 {
			t.Fatalf("ASSERT_GLOBAL_BUDGET_CONSTRUCTOR: FAIL plan=%s frames=%d aggregate=%d", tc.name, bounds.Frames, bounds.FramesPerExchange)
		}
		counts[i] = [3]uint64{summaries, exchanges, bounds.Frames}
		if summaries > maxSummaries {
			maxSummaries = summaries
		}
		if exchanges > maxExchanges {
			maxExchanges = exchanges
		}
		if bounds.Frames > maxFrames {
			maxFrames = bounds.Frames
		}
	}
	cost := func(summaries, exchanges, frames, formatting uint64) (uint64, uint64, bool) {
		// Constructor-checked maxima are S=3, X=8, F=32; K<=81
		// includes only the pure sensitivity probe beyond K=80.
		// Check these bounds before multiplication or subtraction:
		// even summing independent term maxima is at most 566.
		if summaries > maxSummaries || exchanges > maxExchanges || frames > maxFrames || formatting > formatCost+1 || exchanges < summaries {
			return 0, 0, false
		}
		// This is a conservative allowance, not an exact traffic count.
		// If F<8*S (RD:20<24), summarized frame allowances deliberately
		// overcount capacity. Floor unsummarized allowance at zero:
		// no negative credit, unsigned wrap, or F>=8*S admission rule.
		var remaining uint64
		if frames > 8*summaries {
			remaining = frames - 8*summaries
		}
		return summaries*(8*9+formatting+1) + 32 + 2*remaining + (exchanges - summaries), remaining, true
	}
	withinGlobal := func(total uint64) bool { return total <= 512 }
	var rd, crd [3]uint64
	for i, tc := range cases {
		summaries, exchanges, frames := counts[i][0], counts[i][1], counts[i][2]
		total, remaining, valid := cost(summaries, exchanges, frames, formatCost)
		if !valid || total != tc.want[3] || !withinGlobal(total) {
			t.Fatalf("ASSERT_GLOBAL_BUDGET_MODEL: FAIL plan=%s total=%d valid=%t want=%d", tc.name, total, valid, tc.want[3])
		}
		if tc.name == "RD" {
			rd = counts[i]
		}
		if tc.name == "CRD" {
			crd = counts[i]
		}
		t.Logf("ASSERT_GLOBAL_BUDGET_MODEL: PASS plan=%s S=%d X=%d F=%d full=%d/512 residual=%d", tc.name, summaries, exchanges, frames, total, remaining)
	}
	rdCost, rdRemaining, rdValid := cost(rd[0], rd[1], rd[2], formatCost)
	if !rdValid || rd != [3]uint64{3, 5, 20} || rdRemaining != 0 || rdCost != 493 {
		t.Fatalf("ASSERT_GLOBAL_BUDGET_RD_DOMAIN: FAIL counts=%v residual=%d cost=%d valid=%t", rd, rdRemaining, rdCost, rdValid)
	}
	t.Log("ASSERT_GLOBAL_BUDGET_RD_DOMAIN: PASS S=3 X=5 F=20 residual=0 full=493")
	probeCost, probeRemaining, probeValid := cost(3, 8, 23, formatCost)
	if !probeValid || probeRemaining != 0 || probeCost != 496 {
		t.Fatalf("ASSERT_GLOBAL_BUDGET_DOMAIN: FAIL 23-frame probe residual=%d cost=%d valid=%t", probeRemaining, probeCost, probeValid)
	}
	t.Log("ASSERT_GLOBAL_BUDGET_DOMAIN: PASS S=3 X=8 F=23 residual=0 full=496")
	if _, _, valid := cost(3, 2, 24, formatCost); valid {
		t.Fatal("ASSERT_GLOBAL_BUDGET_DOMAIN: FAIL insufficient exchanges")
	}
	t.Log("ASSERT_GLOBAL_BUDGET_DOMAIN: PASS insufficient exchanges rejected S=3 X=2 F=24")
	baseline, _, baselineValid := cost(crd[0], crd[1], crd[2], formatCost)
	perturbed, _, perturbedValid := cost(crd[0], crd[1], crd[2], formatCost+1)
	if !baselineValid || !perturbedValid || baseline != 512 || !withinGlobal(baseline) || perturbed != 515 || withinGlobal(perturbed) {
		t.Fatalf("ASSERT_GLOBAL_BUDGET_SENSITIVITY: FAIL baseline=%d perturbed=%d", baseline, perturbed)
	}
	t.Log("ASSERT_GLOBAL_BUDGET_LIMIT: PASS cost=512")
	t.Log("ASSERT_GLOBAL_BUDGET_LIMIT: FAIL cost=515 (expected sensitivity rejection)")
	t.Log("ASSERT_GLOBAL_BUDGET_SENSITIVITY: PASS baseline=512 accepted=true perturbed=515 accepted=false")
	t.Logf("ASSERT_LOGICAL_BUDGET: PASS scalars=%d perTransaction=%d allSevenConservativeCostsBounded=true CRD=512/512 R_annotations=20/32 maxCapture=27/32", scalars, perTransaction)
}
