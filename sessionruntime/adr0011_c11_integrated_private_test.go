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

const c11DefinitionResultTokenCap = 524288

func c11DefinitionResultToken(t *testing.T, size int) json.RawMessage {
	t.Helper()
	location := []lsp.Location{{
		URI: "file:///c11/",
		Range: &lsp.Range{
			Start: lsp.Position{Line: 11, Character: 12},
			End:   lsp.Position{Line: 13, Character: 14},
		},
	}}
	base, err := json.Marshal(location)
	if err != nil {
		t.Fatal(err)
	}
	padding := size - len(base)
	if padding < 0 {
		t.Fatalf("ASSERT_C11_RESULT_TOKEN_EXACT_SIZE target=%d base=%d", size, len(base))
	}
	location[0].URI += strings.Repeat("p", padding)
	raw, err := json.Marshal(location)
	if err != nil || len(raw) != size {
		t.Fatalf("ASSERT_C11_RESULT_TOKEN_EXACT_SIZE target=%d got=%d err=%v", size, len(raw), err)
	}
	var decoded []lsp.Location
	if err := json.Unmarshal(raw, &decoded); err != nil || len(decoded) != 1 || decoded[0].URI != location[0].URI || decoded[0].Range == nil || *decoded[0].Range != *location[0].Range {
		t.Fatalf("ASSERT_C11_RESULT_TOKEN_EXACT_SIZE size=%d decoded=%+v err=%v", size, decoded, err)
	}
	return raw
}

func c11ReleaseAccepted(t *testing.T, f *fullP1Fixture, result RoundTripResult, managerLease B4DefinitionLease) {
	t.Helper()
	selection := f.selectionBase
	selection.Key = result.Key
	if _, status := consumePrivateB4SnapshotForTest(f.m, managerLease, selection); status != PrivateB4Selected {
		t.Fatalf("ASSERT_C11_ACCEPTED_MANAGER_RELEASE status=%s", status)
	}
	requestLease, requestOK := result.PrivateB4RequestFrameLease()
	responseLease, responseOK := result.PrivateB4ResponseFrameLease()
	resultLease, resultOK := result.PrivateB4ResultLease()
	if !requestOK || !responseOK || !resultOK || !requestLease.Release() || !responseLease.Release() || !resultLease.Release() {
		t.Fatalf("ASSERT_C11_ACCEPTED_CALLER_RELEASE request=%t response=%t result=%t", requestOK, responseOK, resultOK)
	}
	if slots, charged := lifecycleCharge(f.m); slots != 0 || charged != 0 || f.owner.TargetSources[0].state.state.Load() != privateB4SourceReleased {
		t.Fatalf("ASSERT_C11_ACCEPTED_EXACT_CLEANUP slots=%d bytes=%d source=%d", slots, charged, f.owner.TargetSources[0].state.state.Load())
	}
}

func TestC11PrivateDefinitionExactOriginalResultTokenBoundary(t *testing.T) {
	for _, size := range []int{c11DefinitionResultTokenCap - 1, c11DefinitionResultTokenCap, c11DefinitionResultTokenCap + 1} {
		t.Run(fmt.Sprintf("size-%d", size), func(t *testing.T) {
			raw := c11DefinitionResultToken(t, size)
			f := newFullP1Fixture(t)
			f.child.result = append(json.RawMessage(nil), raw...)
			f.expectedResult = append([]byte(nil), raw...)
			f.req.MaxBytes = 2 << 20
			f.req.CaptureDefinitionResponseFrameMaxBytes = 1 << 20
			beforeObservations := len(f.m.Observations())

			result, managerLease := f.m.RoundTripPrivateB4(context.Background(), f.req, f.owner)
			if size <= c11DefinitionResultTokenCap {
				if result.Failure != "" || result.ServerError != nil || result.Messages != 1 || managerLease == (B4DefinitionLease{}) {
					t.Fatalf("ASSERT_C11_RESULT_TOKEN_ACCEPT size=%d failure=%s server=%v messages=%d manager_lease=%t", size, result.Failure, result.ServerError, result.Messages, managerLease != (B4DefinitionLease{}))
				}
				resultLease, ok := result.PrivateB4ResultLease()
				if !ok || resultLease.WithBytes(func(got []byte) error {
					if !bytes.Equal(got, raw) {
						return fmt.Errorf("raw token mismatch")
					}
					var decoded []lsp.Location
					if err := json.Unmarshal(got, &decoded); err != nil || len(decoded) != 1 || decoded[0].Range == nil || decoded[0].Range.Start.Line != 11 || decoded[0].Range.Start.Character != 12 || decoded[0].Range.End.Line != 13 || decoded[0].Range.End.Character != 14 {
						return fmt.Errorf("semantic mismatch: decoded=%+v err=%v", decoded, err)
					}
					return nil
				}) != nil {
					t.Fatal("ASSERT_C11_ACCEPTED_EXACT_RAW_RESULT")
				}
				if result.Key.Generation != f.req.Generation || result.Key.ID == 0 {
					t.Fatalf("ASSERT_C11_ACCEPTED_SELECTED_RESPONSE key=%+v", result.Key)
				}
				c11ReleaseAccepted(t, f, result, managerLease)
				return
			}

			if result.Failure != session.ResourceExhausted || managerLease != (B4DefinitionLease{}) {
				t.Fatalf("ASSERT_C11_RESULT_TOKEN_PLUS_ONE failure=%s manager_lease=%t", result.Failure, managerLease != (B4DefinitionLease{}))
			}
			if len(result.Result) != 0 {
				t.Fatalf("ASSERT_C11_PLUS_ONE_NO_DOWNSTREAM_PUBLICATION result_bytes=%d", len(result.Result))
			}
			if _, ok := result.PrivateB4ResultLease(); ok {
				t.Fatal("ASSERT_C11_PLUS_ONE_NO_DOWNSTREAM_PUBLICATION result lease")
			}
			if _, ok := result.PrivateB4RequestFrameLease(); ok {
				t.Fatal("ASSERT_C11_PLUS_ONE_NO_DOWNSTREAM_PUBLICATION request lease")
			}
			if _, ok := result.PrivateB4ResponseFrameLease(); ok {
				t.Fatal("ASSERT_C11_PLUS_ONE_NO_DOWNSTREAM_PUBLICATION response lease")
			}
			if _, ok := result.PrivateRetainedNotificationLease(); ok {
				t.Fatal("ASSERT_C11_PLUS_ONE_NO_DOWNSTREAM_PUBLICATION notification lease")
			}
			if _, ok := result.PrivateRetainedResponseLease(); ok {
				t.Fatal("ASSERT_C11_PLUS_ONE_NO_DOWNSTREAM_PUBLICATION retained response lease")
			}
			for _, observation := range f.m.Observations()[beforeObservations:] {
				if observation.Kind == "response" && observation.Failure == "" {
					t.Fatalf("ASSERT_C11_PLUS_ONE_NO_SUCCESSFUL_RESPONSE_OBSERVATION observation=%+v", observation)
				}
			}
			f.m.mu.Lock()
			runtimeSession := f.m.sessions[f.req.SessionID]
			requests, protocolOwned, workers := len(runtimeSession.requests), runtimeSession.protocolOwned, f.m.workers
			slots, charged := f.m.privateB4LeaseCountLocked(), f.m.privateB4Bytes
			f.m.mu.Unlock()
			if requests != 0 || protocolOwned || workers != 0 || slots != 0 || charged != 0 || f.owner.TargetSources[0].state.state.Load() != privateB4SourceReleased {
				t.Fatalf("ASSERT_C11_PLUS_ONE_SYNCHRONOUS_CLEANUP requests=%d protocol_owned=%t workers=%d slots=%d bytes=%d source=%d", requests, protocolOwned, workers, slots, charged, f.owner.TargetSources[0].state.state.Load())
			}
		})
	}
}
