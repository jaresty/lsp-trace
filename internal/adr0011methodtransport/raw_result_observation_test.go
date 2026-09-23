package adr0011methodtransport

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

func TestSuccessRawResultPresenceDigestAndCopy(t *testing.T) {
	var twoByteDigests []string
	for _, tc := range []struct {
		name string
		raw  json.RawMessage
		want RawResultDisposition
	}{
		{"nil", nil, RawResultAbsent},
		{"present empty", json.RawMessage{}, RawResultRetained},
		{"null", json.RawMessage(`null`), RawResultRetained},
		{"empty array", json.RawMessage(`[]`), RawResultRetained},
		{"empty object", json.RawMessage(`{}`), RawResultRetained},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := tc.raw
			if tc.name == "present empty" {
				raw = make(json.RawMessage, 0)
			}
			original := append([]byte(nil), raw...)
			f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{DefinitionSupport: true},
				result: sessionruntime.RoundTripResult{Result: raw, Messages: 1, Bytes: int64(len(raw))}}
			got := New(f).Execute(context.Background(), request(MethodDefinition))
			observation, present := got.Observation()
			if got.Outcome() != OutcomeTransportSuccess || !present || observation.RawResultDisposition != tc.want ||
				(got.Raw() == nil) != (raw == nil) || string(got.Raw()) != string(original) {
				t.Fatalf("ASSERT_RAW_PRESENCE: name=%s outcome=%s disposition=%s presence=%t rawNil=%t wantNil=%t", tc.name, got.Outcome(), observation.RawResultDisposition, present, got.Raw() == nil, raw == nil)
			}
			if raw == nil {
				if observation.RawResultBytes != 0 || observation.RawResultSHA256 != "" {
					t.Fatalf("ASSERT_RAW_DIGEST: absent has metadata: %+v", observation)
				}
			} else {
				want := fmt.Sprintf("sha256:%x", sha256.Sum256(raw))
				if observation.RawResultBytes != len(raw) || observation.RawResultSHA256 != want {
					t.Fatalf("ASSERT_RAW_DIGEST: name=%s bytes=%d digest=%q want=%q", tc.name, observation.RawResultBytes, observation.RawResultSHA256, want)
				}
			}
			if len(raw) == 2 {
				twoByteDigests = append(twoByteDigests, observation.RawResultSHA256)
			}
			if len(raw) > 0 {
				f.result.Result[0] = '!'
				copyOut := got.Raw()
				copyOut[0] = '?'
				if string(got.Raw()) != string(original) || observation.RawResultSHA256 != fmt.Sprintf("sha256:%x", sha256.Sum256(original)) {
					t.Fatalf("ASSERT_RAW_COPY_ISOLATION: name=%s raw=%q", tc.name, got.Raw())
				}
			}
			t.Log("ASSERT_RAW_PRESENCE: PASS")
			t.Log("ASSERT_RAW_DIGEST: PASS")
			t.Log("ASSERT_RAW_COPY_ISOLATION: PASS")
		})
	}
	if len(twoByteDigests) != 2 || twoByteDigests[0] == twoByteDigests[1] {
		t.Fatalf("ASSERT_RAW_EQUAL_LENGTH_DISTINCT: digests=%v", twoByteDigests)
	}
	t.Log("ASSERT_RAW_EQUAL_LENGTH_DISTINCT: PASS")
}

func TestFailureRawResultIsWithheldFromObservation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result sessionruntime.RoundTripResult
		want   Outcome
	}{
		{"server error", sessionruntime.RoundTripResult{Result: json.RawMessage(`{"secret":"marker"}`), ServerError: &lspwire.RPCError{Code: -32601, Message: "bounded"}}, OutcomeServerError},
		{"runtime failure", sessionruntime.RoundTripResult{Result: json.RawMessage(`{"secret":"marker"}`), Failure: session.ResourceExhausted}, OutcomeTransportFailure},
		{"timeout", sessionruntime.RoundTripResult{Result: json.RawMessage(`{"secret":"marker"}`), Failure: session.RequestTimeout}, OutcomeTimeout},
		{"overlimit", sessionruntime.RoundTripResult{Result: json.RawMessage(`{"secret":"marker"}`), Bytes: 8193}, OutcomeTransportFailure},
		{"oversized raw server error", sessionruntime.RoundTripResult{Result: json.RawMessage(strings.Repeat("x", 8193)), Bytes: 1, ServerError: &lspwire.RPCError{Code: -32601, Message: "bounded"}}, OutcomeTransportFailure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{DefinitionSupport: true}, result: tc.result}
			got := New(f).Execute(context.Background(), request(MethodDefinition))
			observation, present := got.Observation()
			if !present || f.calls != 1 || got.Outcome() != tc.want || observation.RawResultDisposition != RawResultWithheld ||
				observation.RawResultBytes != 0 || observation.RawResultSHA256 != "" || len(got.Raw()) != 0 {
				t.Fatalf("ASSERT_RAW_FAILURE_WITHHELD: name=%s outcome=%s disposition=%s bytes=%d digest=%q raw=%q", tc.name, got.Outcome(), observation.RawResultDisposition, observation.RawResultBytes, observation.RawResultSHA256, got.Raw())
			}
			if tc.name == "oversized raw server error" {
				if _, present := got.ServerErrorCode(); present {
					t.Fatal("ASSERT_RAW_FAILURE_WITHHELD: oversized raw exposed server-error code")
				}
			}
			if encoded, err := json.Marshal(observation); err != nil || strings.Contains(string(encoded), "marker") {
				t.Fatalf("ASSERT_RAW_FAILURE_WITHHELD: serialized observation leaked raw: err=%v", err)
			}
			t.Log("ASSERT_RAW_FAILURE_WITHHELD: PASS")
		})
	}
	f := &fakeRuntime{metadataFailure: session.StaleGeneration}
	got := New(f).Execute(context.Background(), request(MethodDefinition))
	observation, present := got.Observation()
	if !present || f.calls != 0 || observation.RawResultDisposition != RawResultNotInvoked || observation.RawResultBytes != 0 || observation.RawResultSHA256 != "" {
		t.Fatalf("ASSERT_RAW_NOT_INVOKED: present=%t calls=%d observation=%+v", present, f.calls, observation)
	}
	t.Log("ASSERT_RAW_NOT_INVOKED: PASS")
	f = &fakeRuntime{metadata: sessionruntime.SessionMetadata{DefinitionSupport: false}}
	got = New(f).Execute(context.Background(), request(MethodDefinition))
	observation, present = got.Observation()
	if !present || got.Outcome() != OutcomeUnsupportedCapability || f.calls != 0 || observation.RawResultDisposition != RawResultNotInvoked || observation.RawResultBytes != 0 || observation.RawResultSHA256 != "" {
		t.Fatalf("ASSERT_RAW_NOT_INVOKED: unsupported present=%t calls=%d observation=%+v", present, f.calls, observation)
	}
	t.Log("ASSERT_RAW_NOT_INVOKED: PASS capability absent")
}
