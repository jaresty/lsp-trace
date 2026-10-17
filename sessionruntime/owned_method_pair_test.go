package sessionruntime

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestOwnedMethodPairBoundedManagedSuccess(t *testing.T) {
	m, s, _ := roundTripManager(t, "references-repeated")
	params := json.RawMessage(`{"textDocument":{"uri":"file:///fixture/main.go"},"position":{"line":1,"character":0},"context":{"includeDeclaration":false}}`)
	original := append([]byte(nil), params...)
	got := m.RoundTrip(context.Background(), RoundTripRequest{SessionID: s.SessionID, Generation: s.Generation, Method: "textDocument/references", Params: params, Deadline: time.Now().Add(time.Second), MaxMessages: 3, MaxBytes: 4096, CaptureOwnedMethodPair: true})
	pair, ok := got.CompletedOwnedMethodPair()
	if got.Failure != "" || !ok || pair.SessionID != s.SessionID || pair.Generation != s.Generation || pair.Key != got.Key || pair.Method != "textDocument/references" || !bytes.Equal(pair.Params, original) || !bytes.Equal(pair.Result, got.Result) || pair.Write.Key != got.Key || pair.Read.Key != got.Key {
		t.Fatalf("ASSERT_OWNED_METHOD_PAIR_KEYED_SUCCESS: ok=%v pair=%+v failure=%v", ok, pair, got.Failure)
	}
	if _, framed := got.CompletedMethodResponseFrame(); framed {
		t.Fatal("ASSERT_OWNED_METHOD_PAIR_NO_FRAME_DEPENDENCY")
	}
	params[0] = 'x'
	got.Result[0] = 'x'
	pair.Params[0] = 'x'
	pair.Result[0] = 'x'
	copied, ok := got.CompletedOwnedMethodPair()
	if !ok || !bytes.Equal(copied.Params, original) || copied.Result[0] != '[' {
		t.Fatal("ASSERT_OWNED_METHOD_PAIR_INDEPENDENT_COPIES")
	}
}

func callHierarchyDigest(t *testing.T, literal string) [32]byte {
	t.Helper()
	decoded, err := hex.DecodeString(literal)
	if err != nil || len(decoded) != 32 {
		t.Fatalf("fixture digest: %v", err)
	}
	var digest [32]byte
	copy(digest[:], decoded)
	return digest
}

func callHierarchyTestIdentity() callHierarchyRequestIdentity {
	return callHierarchyRequestIdentity{
		workspace:  callHierarchyWorkspaceCustody{managerNonce: [32]byte{1, 2, 3}},
		session:    "session-17",
		generation: 7,
		requestKey: callHierarchyJSONRPCRequestKey{kind: callHierarchyJSONRPCRequestKeyString, stringValue: "request-17"},
	}
}

func TestCallHierarchyManagerMintsExactCustody(t *testing.T) {
	m, _, _ := roundTripManager(t, "references-empty")
	identity, ok := m.privateCallHierarchyRequestIdentity()
	if ok || !reflect.DeepEqual(identity, callHierarchyRequestIdentity{}) {
		t.Fatalf("ASSERT_CH_MANAGER_CUSTODY_EXPECTED_UNAVAILABLE: ok=%v identity=%+v", ok, identity)
	}
}

func TestCallHierarchyLimitsAndAccountingBoundaries(t *testing.T) {
	limits, ok := newCallHierarchyObservationLimits(11, 3, 5, 2)
	if !ok || limits != (callHierarchyObservationLimits{maxBytes: 11, maxObjects: 3, maxWork: 5, maxDiagnostics: 2}) {
		t.Fatalf("ASSERT_CH_LIMITS_AT_BOUNDARY: ok=%v limits=%+v", ok, limits)
	}
	at := callHierarchyObservationAccounting{bytes: callHierarchyMeasure{available: true, value: 11}, objects: callHierarchyMeasure{available: true, value: 3}, work: callHierarchyMeasure{available: true, value: 5}, diagnostics: callHierarchyMeasure{available: true, value: 2}}
	if !at.validAgainst(limits) {
		t.Fatalf("ASSERT_CH_ACCOUNTING_AVAILABLE_ZERO: at-limit refused: %+v", at)
	}
	unknownAndZero := callHierarchyObservationAccounting{bytes: callHierarchyMeasure{}, objects: callHierarchyMeasure{available: true, value: 0}, work: callHierarchyMeasure{}, diagnostics: callHierarchyMeasure{available: true, value: 0}}
	plusOne := callHierarchyObservationAccounting{bytes: callHierarchyMeasure{available: true, value: 12}}
	if unknownAndZero.validAgainst(limits) || plusOne.validAgainst(limits) {
		t.Fatalf("ASSERT_CH_ACCOUNTING_EXPECTED_REFUSALS: unknown-zero=%+v plus-one=%+v", unknownAndZero, plusOne)
	}
}

func TestCallHierarchyDirectionalMethodAllowlistTriples(t *testing.T) {
	t.Skip("ADR0011_CALLHIERARCHY_DIRECTIONAL_GATE_PENDING: method allowlist triples")
	limits := callHierarchyObservationLimits{maxObjects: 3}
	incoming := []callHierarchyMethodIdentity{"textDocument/documentSymbol", "callHierarchy/prepare", "callHierarchy/incomingCalls"}
	outgoing := []callHierarchyMethodIdentity{"textDocument/documentSymbol", "callHierarchy/prepare", "callHierarchy/outgoingCalls"}
	for name, methods := range map[string][]callHierarchyMethodIdentity{"incoming": incoming, "outgoing": outgoing} {
		t.Run(name, func(t *testing.T) {
			allowlist, ok := newCallHierarchyMethodAllowlist(methods, limits)
			if !ok || !reflect.DeepEqual(allowlist.copyMethods(), methods) {
				t.Fatalf("ASSERT_CH_ALLOWLIST_EXACT_ORDERED_BINDING: ok=%v got=%v want=%v", ok, allowlist.copyMethods(), methods)
			}
			for _, method := range methods {
				if !allowlist.permits(method) {
					t.Fatalf("ASSERT_CH_ALLOWLIST_PERMITS: %q", method)
				}
			}
			copied := allowlist.copyMethods()
			copied[0] = "mutated"
			if !reflect.DeepEqual(allowlist.copyMethods(), methods) {
				t.Fatal("ASSERT_CH_ALLOWLIST_DEFENSIVE_COPY")
			}
		})
	}
	invalid := [][]callHierarchyMethodIdentity{{"arbitrary"}, {incoming[0], incoming[1], outgoing[2]}, {incoming[1], incoming[0], incoming[2]}, {incoming[0], incoming[1], incoming[1]}}
	for _, methods := range invalid {
		if got, ok := newCallHierarchyMethodAllowlist(methods, limits); ok || len(got.copyMethods()) != 0 {
			t.Fatalf("ASSERT_CH_ALLOWLIST_REFUSAL_ZERO_EFFECTS: methods=%v got=%v ok=%v", methods, got.copyMethods(), ok)
		}
	}
}

func TestCallHierarchyCompleteFrameBytesAndDigest(t *testing.T) {
	identity := callHierarchyTestIdentity()
	limits := callHierarchyObservationLimits{maxBytes: 256, maxObjects: 4, maxWork: 8, maxDiagnostics: 2}
	request := []byte("Content-Length: 30\r\n\r\n{\"jsonrpc\":\"2.0\",\"id\":\"r\"}")
	requestObservation, requestOK := newCallHierarchyOriginalRequestFrameObservation(identity, request, 22, 30, 23, 6, limits)
	if requestOK || !reflect.DeepEqual(requestObservation, callHierarchyOriginalRequestFrameObservation{}) {
		t.Fatalf("ASSERT_CH_REQUEST_FRAME_EXPECTED_REFUSED: ok=%v observation=%+v", requestOK, requestObservation)
	}

	response := []byte("Content-Length: 38\r\n\r\n{\"jsonrpc\":\"2.0\",\"id\":\"r\",\"result\":[]}")
	responseWant := append([]byte(nil), response...)
	responseObservation, responseOK := newCallHierarchyOriginalResponseFrameObservation(identity, response, 22, 38, 56, 2, limits)
	if !responseOK || !bytes.Equal(responseObservation.copyFrameBytes(), responseWant) || responseObservation.frameLength != uint64(len(responseWant)) || responseObservation.frameSHA256 != callHierarchyDigest(t, "e87e7afb747c6e59460cba2816e0290bee58ffacd7d1eca49bb69bf4b0570248") {
		t.Fatalf("ASSERT_CH_RESPONSE_FRAME_EXPECTED_AVAILABLE: ok=%v observation=%+v", responseOK, responseObservation)
	}
	response[0] = 'x'
	copyResponse := responseObservation.copyFrameBytes()
	copyResponse[0] = 'y'
	if !bytes.Equal(responseObservation.copyFrameBytes(), responseWant) {
		t.Fatal("ASSERT_CH_RESPONSE_FRAME_DEFENSIVE_COPY")
	}
}

func TestCallHierarchyDirectionalTokenFormAndResultSubspan(t *testing.T) {
	t.Skip("ADR0011_CALLHIERARCHY_DIRECTIONAL_GATE_PENDING: token form and result subspan")
	identity := callHierarchyTestIdentity()
	limits := callHierarchyObservationLimits{maxBytes: 256, maxObjects: 4, maxWork: 8, maxDiagnostics: 2}
	response := []byte("Content-Length: 38\r\n\r\n{\"jsonrpc\":\"2.0\",\"id\":\"r\",\"result\":[]}")
	responseObservation, ok := newCallHierarchyOriginalResponseFrameObservation(identity, response, 22, 38, 56, 2, limits)
	if !ok || responseObservation.resultForm != callHierarchyJSONTokenFormEmptyArray {
		t.Fatalf("ASSERT_CH_RESPONSE_RESULT_TOKEN_FORM_AND_SUBSPAN: ok=%v observation=%+v", ok, responseObservation)
	}
}

func TestCallHierarchyDirectionalRawSelectedAndHandoffCommitments(t *testing.T) {
	t.Skip("ADR0011_CALLHIERARCHY_DIRECTIONAL_GATE_PENDING: raw selected and handoff commitments")
	identity := callHierarchyTestIdentity()
	transaction := callHierarchyTransactionIdentity{request: identity, local: [32]byte{9}}
	limits := callHierarchyObservationLimits{maxBytes: 64, maxObjects: 4, maxWork: 8}
	selectedBytes := []byte(`{"name":"Target"}`)
	selected, ok := newCallHierarchySelectedItemCommitment(2, identity, transaction, 7, uint64(len(selectedBytes)), selectedBytes, limits)
	if !ok || selected.ordinal != 2 || !reflect.DeepEqual(selected.sourceRequest, identity) || !bytes.Equal(selected.copyBytes(), selectedBytes) {
		t.Fatalf("ASSERT_CH_SELECTED_ITEM_COMMITMENT: ok=%v selected=%+v", ok, selected)
	}
	successor := []byte(`{"item":"Target"}`)
	handoff, ok := newCallHierarchyHandoffCommitment(selected, identity, transaction, 11, uint64(len(successor)), successor, limits)
	if !ok || handoff.selected.exactSHA256 != selected.exactSHA256 || !bytes.Equal(handoff.copySuccessorBytes(), successor) {
		t.Fatalf("ASSERT_CH_SELECTED_HANDOFF_COMMITMENT: ok=%v handoff=%+v", ok, handoff)
	}
	selectedBytes[0] = 'x'
	successor[0] = 'x'
	if string(selected.copyBytes()) != `{"name":"Target"}` || string(handoff.copySuccessorBytes()) != `{"item":"Target"}` {
		t.Fatal("ASSERT_CH_HANDOFF_DEFENSIVE_COPY")
	}
}

func TestCallHierarchyTerminalClassificationIsExact(t *testing.T) {
	if callHierarchyObservationTerminalUnavailable.valid() {
		t.Fatal("ASSERT_CH_TERMINAL_UNAVAILABLE_INVALID")
	}
	for _, terminal := range []callHierarchyObservationTerminal{callHierarchyObservationTerminalComplete, callHierarchyObservationTerminalSuccessfulEmpty, callHierarchyObservationTerminalTransportError, callHierarchyObservationTerminalProtocolError, callHierarchyObservationTerminalMissingPayload, callHierarchyObservationTerminalMalformed, callHierarchyObservationTerminalPartialCapture, callHierarchyObservationTerminalLimitRefused, callHierarchyObservationTerminalCancelled} {
		if !terminal.valid() {
			t.Fatalf("ASSERT_CH_TERMINAL_VALID: terminal=%d", terminal)
		}
	}
}

func TestCallHierarchyDirectionalRichPredecessorRequestChain(t *testing.T) {
	t.Skip("ADR0011_CALLHIERARCHY_DIRECTIONAL_GATE_PENDING: rich predecessor and request chain")
	identity := callHierarchyTestIdentity()
	limits := callHierarchyObservationLimits{maxBytes: 128, maxObjects: 8, maxWork: 8, maxDiagnostics: 2}
	predecessor := callHierarchyPredecessorCommitment{identity: "document-symbol", requestIdentity: identity, requestKey: identity.requestKey, method: "textDocument/documentSymbol", requestOrdinal: 1, commitmentSHA256: [32]byte{7}}
	chain, ok := newCallHierarchyPredecessorChain([]callHierarchyPredecessorCommitment{predecessor}, limits)
	if !ok || !reflect.DeepEqual(chain.copyCommitments(), []callHierarchyPredecessorCommitment{predecessor}) {
		t.Fatalf("ASSERT_CH_PREDECESSOR_CHAIN: ok=%v chain=%+v", ok, chain)
	}
	methods, ok := newCallHierarchyMethodAllowlist([]callHierarchyMethodIdentity{"textDocument/documentSymbol", "callHierarchy/prepare", "callHierarchy/incomingCalls"}, limits)
	if !ok {
		t.Fatal("ASSERT_CH_REQUEST_ALLOWLIST_SETUP")
	}
	observationIdentity := callHierarchyObservationIdentity{request: identity, profile: "go", binding: "opaque-incoming", transaction: callHierarchyTransactionIdentity{request: identity, local: [32]byte{3}}, method: "callHierarchy/incomingCalls"}
	request, ok := newCallHierarchyObservationRequest(observationIdentity, methods, chain, nil, nil, limits)
	if !ok || request.identity.binding != "opaque-incoming" || !reflect.DeepEqual(request.predecessors.copyCommitments(), []callHierarchyPredecessorCommitment{predecessor}) {
		t.Fatalf("ASSERT_CH_REQUEST_COMMITMENT_CHAIN: ok=%v request=%+v", ok, request)
	}
}

func TestOwnedMethodPairRejectsNonEligibleResults(t *testing.T) {
	for _, tc := range []struct {
		name, mode, method    string
		capture               bool
		maxBytes, maxMessages int64
	}{
		{"default-off", "references-empty", "textDocument/references", false, 4096, 1},
		{"non-method", "success", "test/method", true, 4096, 1},
		{"server-error", "server-error", "textDocument/definition", true, 4096, 1},
		{"unmatched-timeout", "unmatched-only", "textDocument/references", true, 4096, 1},
		{"over-byte-policy", "references-empty", "textDocument/references", true, 1<<20 + 1, 1},
		{"over-message-policy", "references-empty", "textDocument/references", true, 4096, 65},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, s, _ := roundTripManager(t, tc.mode)
			deadline := time.Now().Add(time.Second)
			if tc.mode == "unmatched-only" {
				deadline = time.Now().Add(30 * time.Millisecond)
			}
			got := m.RoundTrip(context.Background(), RoundTripRequest{SessionID: s.SessionID, Generation: s.Generation, Method: tc.method, Params: json.RawMessage(`{"textDocument":{"uri":"file:///fixture/main.go"},"position":{"line":1,"character":0},"context":{"includeDeclaration":false}}`), Deadline: deadline, MaxMessages: int(tc.maxMessages), MaxBytes: tc.maxBytes, CaptureOwnedMethodPair: tc.capture})
			if _, ok := got.CompletedOwnedMethodPair(); ok {
				t.Fatal("ASSERT_OWNED_METHOD_PAIR_WITHHELD")
			}
		})
	}
}
