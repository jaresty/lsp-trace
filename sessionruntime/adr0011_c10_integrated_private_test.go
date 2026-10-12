package sessionruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

const c10ReferencesDecodedRemarshalCap = 1572864

func c10ProgressMessage(t *testing.T, size int) lspwire.Message {
	t.Helper()
	message := lspwire.Message{JSONRPC: lspwire.Version, Method: "$/progress", Params: json.RawMessage(`{"token":"c10","value":{"padding":""}}`)}
	base, err := json.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	padding := size - len(base)
	if padding < 0 {
		t.Fatalf("ASSERT_C10_DECODED_REMARSHAL_FIXTURE target=%d base=%d", size, len(base))
	}
	message.Params = json.RawMessage(fmt.Sprintf(`{"token":"c10","value":{"padding":%q}}`, strings.Repeat("x", padding)))
	encoded, err := json.Marshal(message)
	if err != nil || len(encoded) != size {
		t.Fatalf("ASSERT_C10_DECODED_REMARSHAL_EXACT_SIZE target=%d got=%d err=%v", size, len(encoded), err)
	}
	return message
}

func c10AssertExactCleanup(t *testing.T, f *fullP1Fixture, assertion string) {
	t.Helper()
	f.m.mu.Lock()
	runtimeSession := f.m.sessions[f.req.SessionID]
	if runtimeSession == nil || runtimeSession.readinessHistory == nil {
		f.m.mu.Unlock()
		t.Fatalf("%s missing runtime session/history", assertion)
	}
	requests := len(runtimeSession.requests)
	protocolOwned := runtimeSession.protocolOwned
	workers := f.m.workers
	owners := runtimeSession.readinessHistory.receiptOwners
	borrowers := runtimeSession.readinessHistory.borrowers
	leases := f.m.privateB4LeaseCountLocked()
	charged := f.m.privateB4Bytes
	f.m.mu.Unlock()
	if requests != 0 || protocolOwned || workers != 0 || owners != 0 || borrowers != 0 || leases != 0 || charged != 0 {
		t.Fatalf("%s requests=%d protocol_owned=%t workers=%d history_owners=%d history_borrowers=%d leases=%d bytes=%d", assertion, requests, protocolOwned, workers, owners, borrowers, leases, charged)
	}
}

func TestC10PrivateReferencesDecodedRemarshalBoundary(t *testing.T) {
	for _, size := range []int{c10ReferencesDecodedRemarshalCap - 1, c10ReferencesDecodedRemarshalCap, c10ReferencesDecodedRemarshalCap + 1} {
		t.Run(fmt.Sprintf("size-%d", size), func(t *testing.T) {
			progress := c10ProgressMessage(t, size)
			f := newFullP1FixtureWithResponses(t, []lspwire.Message{progress})
			f.req.Method = "textDocument/references"
			f.req.Params = json.RawMessage(`{"textDocument":{"uri":"file:///w/target-a.go"},"position":{"line":0,"character":0},"context":{"includeDeclaration":true}}`)
			f.req.ADR0011PrivateP2 = true
			f.req.MaxMessages = 2
			f.req.MaxBytes = 4 << 20
			beforeObservations := len(f.m.Observations())

			result := f.m.RoundTrip(context.Background(), f.req)
			if size <= c10ReferencesDecodedRemarshalCap {
				if result.Failure != "" || result.ServerError != nil || result.Messages != 2 {
					t.Fatalf("ASSERT_C10_DECODED_REMARSHAL_ACCEPT size=%d failure=%s server=%v messages=%d", size, result.Failure, result.ServerError, result.Messages)
				}
				if !bytes.Equal(result.Result, f.expectedResult) || len(result.Notifications) != 1 || len(result.Responses) != 0 {
					t.Fatalf("ASSERT_C10_ACCEPTED_EXACT_RESULT size=%d result_equal=%t notifications=%d unmatched=%d", size, bytes.Equal(result.Result, f.expectedResult), len(result.Notifications), len(result.Responses))
				}
				c10AssertExactCleanup(t, f, "ASSERT_C10_ACCEPTED_EXACT_CLEANUP")
				return
			}

			if result.Failure != session.ResourceExhausted {
				t.Fatalf("ASSERT_C10_DECODED_REMARSHAL_PLUS_ONE failure=%s", result.Failure)
			}
			if len(result.Result) != 0 || len(result.Notifications) != 0 || len(result.Responses) != 0 {
				t.Fatalf("ASSERT_C10_PLUS_ONE_NO_DOWNSTREAM_PUBLICATION result=%d notifications=%d unmatched=%d", len(result.Result), len(result.Notifications), len(result.Responses))
			}
			for _, observation := range f.m.Observations()[beforeObservations:] {
				if observation.Kind == "response" && observation.Failure == "" {
					t.Fatalf("ASSERT_C10_PLUS_ONE_NO_ACCEPTED_OBSERVATION observation=%+v", observation)
				}
			}
			c10AssertExactCleanup(t, f, "ASSERT_C10_PLUS_ONE_SYNCHRONOUS_CLEANUP")
		})
	}
}
