package sessionruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

const c09DefinitionDecodedRemarshalCap = 1048576

func c09ProgressMessage(t *testing.T, size int) lspwire.Message {
	t.Helper()
	message := lspwire.Message{JSONRPC: lspwire.Version, Method: "$/progress", Params: json.RawMessage(`{"token":"c09","value":{"padding":""}}`)}
	base, err := json.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	padding := size - len(base)
	if padding < 0 {
		t.Fatalf("ASSERT_C09_DECODED_REMARSHAL_FIXTURE target=%d base=%d", size, len(base))
	}
	message.Params = json.RawMessage(fmt.Sprintf(`{"token":"c09","value":{"padding":%q}}`, strings.Repeat("x", padding)))
	encoded, err := json.Marshal(message)
	if err != nil || len(encoded) != size {
		t.Fatalf("ASSERT_C09_DECODED_REMARSHAL_EXACT_SIZE target=%d got=%d err=%v", size, len(encoded), err)
	}
	return message
}

func c09ReleaseAccepted(t *testing.T, f *fullP1Fixture, result RoundTripResult, managerLease B4DefinitionLease) {
	t.Helper()
	selection := f.selectionBase
	selection.Key = result.Key
	if _, status := consumePrivateB4SnapshotForTest(f.m, managerLease, selection); status != PrivateB4Selected {
		t.Fatalf("ASSERT_C09_ACCEPTED_MANAGER_RELEASE status=%s", status)
	}
	requestLease, requestOK := result.PrivateB4RequestFrameLease()
	responseLease, responseOK := result.PrivateB4ResponseFrameLease()
	resultLease, resultOK := result.PrivateB4ResultLease()
	notificationLease, notificationOK := result.PrivateRetainedNotificationLease()
	if !requestOK || !responseOK || !resultOK || !notificationOK || !requestLease.Release() || !responseLease.Release() || !resultLease.Release() || !notificationLease.Release() {
		t.Fatalf("ASSERT_C09_ACCEPTED_CALLER_RELEASE request=%t response=%t result=%t notification=%t", requestOK, responseOK, resultOK, notificationOK)
	}
	if slots, charged := lifecycleCharge(f.m); slots != 0 || charged != 0 || f.owner.TargetSources[0].state.state.Load() != privateB4SourceReleased {
		t.Fatalf("ASSERT_C09_ACCEPTED_EXACT_CLEANUP slots=%d bytes=%d source=%d", slots, charged, f.owner.TargetSources[0].state.state.Load())
	}
}

func TestC09PrivateDefinitionDecodedRemarshalBoundary(t *testing.T) {
	for _, size := range []int{c09DefinitionDecodedRemarshalCap - 1, c09DefinitionDecodedRemarshalCap, c09DefinitionDecodedRemarshalCap + 1} {
		t.Run(fmt.Sprintf("size-%d", size), func(t *testing.T) {
			progress := c09ProgressMessage(t, size)
			f := newFullP1FixtureWithResponses(t, []lspwire.Message{progress})
			f.req.MaxMessages = 2
			f.req.MaxBytes = 3 << 20
			beforeObservations := len(f.m.Observations())
			result, managerLease := f.m.RoundTripPrivateB4(context.Background(), f.req, f.owner)
			if size <= c09DefinitionDecodedRemarshalCap {
				if result.Failure != "" || managerLease == (B4DefinitionLease{}) || result.Messages != 2 {
					t.Fatalf("ASSERT_C09_DECODED_REMARSHAL_ACCEPT size=%d failure=%s lease=%t messages=%d", size, result.Failure, managerLease != (B4DefinitionLease{}), result.Messages)
				}
				resultLease, ok := result.PrivateB4ResultLease()
				if !ok || resultLease.WithBytes(func(got []byte) error {
					if string(got) != string(f.expectedResult) {
						return fmt.Errorf("result mismatch")
					}
					return nil
				}) != nil {
					t.Fatal("ASSERT_C09_DECODED_REMARSHAL_ACCEPTED_RESULT")
				}
				c09ReleaseAccepted(t, f, result, managerLease)
				return
			}
			if result.Failure != session.ResourceExhausted || managerLease != (B4DefinitionLease{}) {
				t.Fatalf("ASSERT_C09_DECODED_REMARSHAL_PLUS_ONE failure=%s lease=%t", result.Failure, managerLease != (B4DefinitionLease{}))
			}
			if _, ok := result.PrivateB4ResultLease(); ok {
				t.Fatal("ASSERT_C09_PLUS_ONE_NO_ACCEPTED_RESULT")
			}
			if _, ok := result.PrivateRetainedNotificationLease(); ok {
				t.Fatal("ASSERT_C09_PLUS_ONE_NO_DOWNSTREAM_PUBLICATION")
			}
			observations := f.m.Observations()
			for _, observation := range observations[beforeObservations:] {
				if observation.Kind == "response" && observation.Failure == "" {
					t.Fatalf("ASSERT_C09_PLUS_ONE_NO_ACCEPTED_OBSERVATION observation=%+v", observation)
				}
			}
			f.m.mu.Lock()
			r := f.m.sessions[f.req.SessionID]
			requests, protocolOwned, workers := len(r.requests), r.protocolOwned, f.m.workers
			slots, charged := f.m.privateB4LeaseCountLocked(), f.m.privateB4Bytes
			f.m.mu.Unlock()
			if requests != 0 || protocolOwned || workers != 0 || slots != 0 || charged != 0 || f.owner.TargetSources[0].state.state.Load() != privateB4SourceReleased {
				t.Fatalf("ASSERT_C09_PLUS_ONE_SYNCHRONOUS_CLEANUP requests=%d protocol_owned=%t workers=%d slots=%d bytes=%d source=%d", requests, protocolOwned, workers, slots, charged, f.owner.TargetSources[0].state.state.Load())
			}
		})
	}
}
