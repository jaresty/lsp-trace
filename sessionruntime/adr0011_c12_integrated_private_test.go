package sessionruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"lsp-trace/internal/lsp"
	"lsp-trace/internal/session"
)

const c12ReferencesResultTokenCap = 524288

func c12ReferencesFixture(t *testing.T, size int) (*fullP1Fixture, json.RawMessage, lsp.Location) {
	t.Helper()
	location := lsp.Location{
		URI: "file:///c12/",
		Range: &lsp.Range{
			Start: lsp.Position{Line: 12, Character: 13},
			End:   lsp.Position{Line: 14, Character: 15},
		},
	}
	base, err := json.Marshal([]lsp.Location{location})
	if err != nil {
		t.Fatal(err)
	}
	padding := size - len(base)
	if padding < 0 {
		t.Fatalf("ASSERT_C12_RESULT_TOKEN_EXACT_SIZE target=%d base=%d", size, len(base))
	}
	location.URI += strings.Repeat("r", padding)
	raw, err := json.Marshal([]lsp.Location{location})
	if err != nil || len(raw) != size {
		t.Fatalf("ASSERT_C12_RESULT_TOKEN_EXACT_SIZE target=%d got=%d err=%v", size, len(raw), err)
	}
	var decoded []lsp.Location
	if err := json.Unmarshal(raw, &decoded); err != nil || len(decoded) != 1 || decoded[0].URI != location.URI || decoded[0].Range == nil || *decoded[0].Range != *location.Range {
		t.Fatalf("ASSERT_C12_RESULT_TOKEN_EXACT_SIZE size=%d decoded=%+v err=%v", size, decoded, err)
	}

	f := newFullP1Fixture(t)
	f.child.result = append(json.RawMessage(nil), raw...)
	f.expectedResult = append([]byte(nil), raw...)
	f.req.Method = "textDocument/references"
	f.req.Params = json.RawMessage(`{"textDocument":{"uri":"file:///w/target-a.go"},"position":{"line":0,"character":0},"context":{"includeDeclaration":true}}`)
	f.req.ADR0011PrivateP2 = true
	f.req.MaxMessages = 1
	f.req.MaxBytes = 4 << 20
	return f, raw, location
}

func c12AssertNoPrivateLeases(t *testing.T, result RoundTripResult, assertion string) {
	t.Helper()
	if _, ok := result.PrivateB4RequestFrameLease(); ok {
		t.Fatalf("%s request", assertion)
	}
	if _, ok := result.PrivateB4ResponseFrameLease(); ok {
		t.Fatalf("%s response", assertion)
	}
	if _, ok := result.PrivateB4ResultLease(); ok {
		t.Fatalf("%s result", assertion)
	}
	if _, ok := result.PrivateRetainedNotificationLease(); ok {
		t.Fatalf("%s notification", assertion)
	}
	if _, ok := result.PrivateRetainedResponseLease(); ok {
		t.Fatalf("%s retained_response", assertion)
	}
}

func c12AssertExactCleanup(t *testing.T, f *fullP1Fixture, assertion string) {
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
	slots := f.m.privateB4LeaseCountLocked()
	charged := f.m.privateB4Bytes
	f.m.mu.Unlock()
	if requests != 0 || protocolOwned || workers != 0 || owners != 0 || borrowers != 0 || slots != 0 || charged != 0 {
		t.Fatalf("%s requests=%d protocol_owned=%t workers=%d history_owners=%d history_borrowers=%d private_slots=%d bytes=%d", assertion, requests, protocolOwned, workers, owners, borrowers, slots, charged)
	}
}

func TestC12PrivateReferencesExactOriginalResultTokenBoundary(t *testing.T) {
	for _, size := range []int{c12ReferencesResultTokenCap - 1, c12ReferencesResultTokenCap, c12ReferencesResultTokenCap + 1} {
		t.Run(fmt.Sprintf("size-%d", size), func(t *testing.T) {
			f, raw, location := c12ReferencesFixture(t, size)
			beforeObservations := len(f.m.Observations())

			result := f.m.RoundTrip(context.Background(), f.req)
			observations := f.m.Observations()[beforeObservations:]
			if size <= c12ReferencesResultTokenCap {
				if result.Failure != "" || result.ServerError != nil || result.Messages != 1 {
					t.Fatalf("ASSERT_C12_RESULT_TOKEN_ACCEPT size=%d failure=%s server=%v messages=%d", size, result.Failure, result.ServerError, result.Messages)
				}
				var decoded []lsp.Location
				if err := json.Unmarshal(result.Result, &decoded); !bytes.Equal(result.Result, raw) || err != nil || len(decoded) != 1 || decoded[0].URI != location.URI || decoded[0].Range == nil || *decoded[0].Range != *location.Range {
					t.Fatalf("ASSERT_C12_ACCEPTED_EXACT_RAW_RESULT size=%d equal=%t decoded=%+v err=%v", size, bytes.Equal(result.Result, raw), decoded, err)
				}
				if result.Key.Generation != f.req.Generation || result.Key.ID == 0 || len(result.Notifications) != 0 || len(result.Responses) != 0 {
					t.Fatalf("ASSERT_C12_ACCEPTED_SELECTED_RESPONSE key=%+v notifications=%d unmatched=%d", result.Key, len(result.Notifications), len(result.Responses))
				}
				successful := 0
				for _, observation := range observations {
					if observation.Kind == "response" && observation.Failure == "" {
						successful++
						if observation.SessionID != f.req.SessionID || observation.Generation != result.Key.Generation {
							t.Fatalf("ASSERT_C12_ACCEPTED_ONE_SUCCESSFUL_RESPONSE_OBSERVATION session=%s generation=%d want_session=%s want_generation=%d", observation.SessionID, observation.Generation, f.req.SessionID, result.Key.Generation)
						}
					}
				}
				if successful != 1 {
					t.Fatalf("ASSERT_C12_ACCEPTED_ONE_SUCCESSFUL_RESPONSE_OBSERVATION count=%d observations=%+v", successful, observations)
				}
				c12AssertNoPrivateLeases(t, result, "ASSERT_C12_ACCEPTED_NO_PRIVATE_LEASES")
				c12AssertExactCleanup(t, f, "ASSERT_C12_ACCEPTED_EXACT_CLEANUP")
				return
			}

			if result.Failure != session.ResourceExhausted {
				t.Fatalf("ASSERT_C12_RESULT_TOKEN_PLUS_ONE failure=%s", result.Failure)
			}
			if len(result.Result) != 0 || len(result.Notifications) != 0 || len(result.Responses) != 0 {
				t.Fatalf("ASSERT_C12_PLUS_ONE_NO_DOWNSTREAM_PUBLICATION result=%d notifications=%d unmatched=%d", len(result.Result), len(result.Notifications), len(result.Responses))
			}
			c12AssertNoPrivateLeases(t, result, "ASSERT_C12_PLUS_ONE_NO_PRIVATE_LEASES")
			for _, observation := range observations {
				if observation.Kind == "response" && observation.Failure == "" {
					t.Fatalf("ASSERT_C12_PLUS_ONE_NO_SUCCESSFUL_RESPONSE_OBSERVATION observation=%+v", observation)
				}
			}
			c12AssertExactCleanup(t, f, "ASSERT_C12_PLUS_ONE_SYNCHRONOUS_CLEANUP")
		})
	}
}
