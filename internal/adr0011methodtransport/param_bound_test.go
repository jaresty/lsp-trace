package adr0011methodtransport

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"lsp-trace/sessionruntime"
)

func exactParameterBytes(method string) json.RawMessage {
	prefix := `{"textDocument":{"uri":"file:///w/`
	suffix := `"},"position":{"line":2,"character":3}`
	if method == MethodReferences {
		suffix += `,"context":{"includeDeclaration":true}`
	}
	suffix += `}`
	return json.RawMessage(prefix + strings.Repeat("a", maxParamsBytes-len(prefix)-len(suffix)) + suffix)
}

func TestRawParamsOverLimitBeforeValidationAndRuntime(t *testing.T) {
	for _, method := range []string{MethodDefinition, MethodReferences} {
		for _, tc := range []struct {
			name string
			raw  json.RawMessage
		}{
			{"valid plus whitespace", append(exactParameterBytes(method), ' ')},
			{"malformed", json.RawMessage(strings.Repeat("secret-marker", maxParamsBytes/13+1))},
		} {
			t.Run(method+"/"+tc.name, func(t *testing.T) {
				if len(tc.raw) <= maxParamsBytes {
					t.Fatalf("invalid fixture length: %d", len(tc.raw))
				}
				f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{DefinitionSupport: true, ReferencesSupport: true},
					result: sessionruntime.RoundTripResult{Result: json.RawMessage(`[]`), Messages: 1, Bytes: 2}}
				req := request(method)
				req.Params, req.MaxBytes = tc.raw, maxBytes
				got := New(f).Execute(context.Background(), req)
				_, present := got.Observation()
				if got.Outcome() != OutcomePreflightFailure || got.FailureText() != "params exceed private byte limit" || present ||
					f.metadataCalls != 0 || f.calls != 0 || len(got.Raw()) != 0 || strings.Contains(got.FailureText(), "secret-marker") {
					t.Fatalf("ASSERT_PARAM_PRECOPY_LIMIT: method=%s outcome=%s failure=%q observation=%t metadata=%d wire=%d", method, got.Outcome(), got.FailureText(), present, f.metadataCalls, f.calls)
				}
				t.Log("ASSERT_PARAM_PRECOPY_LIMIT: PASS")
			})
		}
	}
}

func TestRawParamsExactLimitPreservesCopiedWireAndObservation(t *testing.T) {
	for _, method := range []string{MethodDefinition, MethodReferences} {
		t.Run(method, func(t *testing.T) {
			raw := exactParameterBytes(method)
			if len(raw) != maxParamsBytes || !json.Valid(raw) {
				t.Fatalf("invalid exact fixture: len=%d valid=%t", len(raw), json.Valid(raw))
			}
			original := append([]byte(nil), raw...)
			f := &fakeRuntime{metadata: sessionruntime.SessionMetadata{DefinitionSupport: true, ReferencesSupport: true},
				result: sessionruntime.RoundTripResult{Result: json.RawMessage(`[]`), Messages: 1, Bytes: 2}}
			req := request(method)
			req.Params, req.MaxBytes = raw, maxBytes
			got := New(f).Execute(context.Background(), req)
			observation, present := got.Observation()
			if got.Outcome() != OutcomeTransportSuccess || !present || f.metadataCalls != 1 || f.calls != 1 ||
				string(f.wire.Params) != string(original) || observation.ParamsBytes != maxParamsBytes ||
				observation.ParamsSHA256 != fmt.Sprintf("sha256:%x", sha256.Sum256(original)) {
				t.Fatalf("ASSERT_PARAM_EXACT_COPY: method=%s outcome=%s failure=%q observation=%t metadata=%d wire=%d length=%d", method, got.Outcome(), got.FailureText(), present, f.metadataCalls, f.calls, observation.ParamsBytes)
			}
			raw[0] = '!'
			if string(f.wire.Params) != string(original) {
				t.Fatal("ASSERT_PARAM_EXACT_COPY: caller mutation changed copied wire")
			}
			t.Log("ASSERT_PARAM_EXACT_COPY: PASS")
		})
	}
}
