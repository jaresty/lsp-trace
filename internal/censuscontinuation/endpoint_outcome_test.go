package censuscontinuation

import (
	"encoding/json"
	"reflect"
	"testing"
)

func testEndpoint(nomination, role, node, uri string) EndpointIdentity {
	return EndpointIdentity{NominationID: nomination, Role: role, ConstituentIdentity: "constituent", ConstituentOrdinal: 2, GraphSubjectID: node, NodeID: node, LogicalSourceID: uri}
}

func TestEndpointCaptureOutcomeClosureRejectsMissingDuplicateUnknownAndPermutation(t *testing.T) {
	derived := []EndpointIdentity{
		testEndpoint("n1", EndpointRoleTarget, "target", "file:///workspace/target.go"),
		testEndpoint("n1", EndpointRoleCaller, "caller", "file:///workspace/caller.go"),
	}
	outcomes := []EndpointCaptureOutcome{
		mustEndpointOutcome(t, derived[0], EndpointStatusCaptured, ""),
		mustEndpointOutcome(t, derived[1], EndpointStatusSourceUnavailable, SourceCodeExactEndpointUnavailable),
	}
	canonical, err := ValidateEndpointCaptureClosure(derived, outcomes)
	if err != nil {
		t.Fatalf("ASSERT_ENDPOINT_CLOSURE_VALID: %v", err)
	}
	reversed, err := ValidateEndpointCaptureClosure([]EndpointIdentity{derived[1], derived[0]}, []EndpointCaptureOutcome{outcomes[1], outcomes[0]})
	if err != nil || !reflect.DeepEqual(canonical, reversed) {
		t.Fatalf("ASSERT_ENDPOINT_CLOSURE_PERMUTATION: err=%v canonical=%#v reversed=%#v", err, canonical, reversed)
	}
	attacks := map[string][]EndpointCaptureOutcome{
		"missing":   outcomes[:1],
		"duplicate": {outcomes[0], outcomes[0], outcomes[1]},
		"unknown":   {outcomes[0], mustEndpointOutcome(t, testEndpoint("n1", EndpointRoleCaller, "other", "file:///workspace/other.go"), EndpointStatusSourceUnavailable, SourceCodeExactEndpointUnavailable)},
	}
	for name, attack := range attacks {
		t.Run(name, func(t *testing.T) {
			if _, err := ValidateEndpointCaptureClosure(derived, attack); err == nil {
				t.Fatalf("ASSERT_ENDPOINT_CLOSURE_REJECT_%s", name)
			}
		})
	}
}

func TestEndpointCaptureOutcomeAuthorityAndIdentity(t *testing.T) {
	e := testEndpoint("n1", EndpointRoleTarget, "target", "file:///workspace/target.go")
	o := mustEndpointOutcome(t, e, EndpointStatusCaptured, "")
	if o.Authority != 0 || o.Accepted || o.Completeness != "UNKNOWN" || o.OutcomeID == "" || o.Endpoint != canonicalEndpoint(e) {
		t.Fatalf("ASSERT_ENDPOINT_OUTCOME_AUTHORITY_IDENTITY: %#v", o)
	}
	raw, err := json.Marshal(o)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ParseEndpointCaptureOutcome(raw); err != nil {
		t.Fatalf("ASSERT_ENDPOINT_OUTCOME_CANONICAL_PARSE: %v", err)
	}
}

func TestPreparationFailureIsSourceSafeAndResponseFree(t *testing.T) {
	e := testEndpoint("n1", EndpointRoleCaller, "caller", "file:///outside/private.go")
	f, err := NewPreparationFailure("packet-intent", e, SourceCodeExactEndpointUnavailable)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(f)
	if f.Authority != 0 || f.Accepted || f.Completeness != "UNKNOWN" || f.Role != EndpointRoleCaller || f.Code != SourceCodeExactEndpointUnavailable || f.ResponseID != "" {
		t.Fatalf("ASSERT_PREPARATION_FAILURE_SHAPE: %#v", f)
	}
	if string(raw) == "" || containsBytes(raw, []byte("private.go")) {
		t.Fatalf("ASSERT_PREPARATION_FAILURE_SOURCE_SAFE: %s", raw)
	}
}

func mustEndpointOutcome(t *testing.T, e EndpointIdentity, status, code string) EndpointCaptureOutcome {
	t.Helper()
	o, err := NewEndpointCaptureOutcome(e, status, code)
	if err != nil {
		t.Fatal(err)
	}
	return o
}

func containsBytes(haystack, needle []byte) bool {
	return len(needle) > 0 && string(haystack) != "" && json.Valid(haystack) && string(haystack) != string(needle) && containsString(string(haystack), string(needle))
}

func containsString(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
