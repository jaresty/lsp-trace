package mcpcontract

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

func TestFutureStructuralPolicyDocumentsExactBytesAndDigests(t *testing.T) {
	policies := []struct {
		name, document, digest, wantDocument, wantDigest string
	}{
		{"traversal", futureTraversalPolicyDocument, futureTraversalPolicyDigest, `{"policy_id":"transient-calls-traversal.v1","policy_version":"1","policy_status":"PROVISIONAL_NONCERTIFIED","relation":"CALLS","down_depth":{"minimum":0,"maximum":64},"up_depth":{"minimum":0,"maximum":64},"max_nodes":{"minimum":1,"maximum":10000}}`, "sha256:a22ece2850382f9c97efb7203dcf7346024643f8be1d58192297ab0e67cc5770"},
		{"resources", futureResourcePolicyDocument, futureResourcePolicyDigest, `{"policy_id":"transient-structural-resources.v1","policy_version":"1","policy_status":"PROVISIONAL_NONCERTIFIED","timeout_ms":{"minimum":1,"maximum":60000},"request_timeout_ms":{"minimum":1,"maximum":60000,"maximum_relation":"request_timeout_ms<=timeout_ms"},"max_messages":{"minimum":1,"maximum":4096,"default":64},"max_bytes":{"minimum":1,"maximum":16777216,"default":4194304}}`, "sha256:26f43e823046616c0ff68750516511e49d575914fa6d3b9dd71913cfab954f23"},
		{"neighborhood", futureNeighborhoodPolicyDocument, futureNeighborhoodPolicyDigest, `{"policy_id":"transient-neighborhood.v1","policy_version":"1","policy_status":"PROVISIONAL_NONCERTIFIED","kind":"NEIGHBORHOOD","relation":"CALLS","semantics":"target_rooted_admitted_nodes_and_call_edges_within_exact_independent_incoming_and_outgoing_traversal_bounds"}`, "sha256:731fff0e5725c5031ff25daad58e737acae14dc1f4d342720a8cb4f313fee8da"},
		{"impact", futureImpactPolicyDocument, futureImpactPolicyDigest, `{"policy_id":"transient-impact.v1","policy_version":"1","policy_status":"PROVISIONAL_NONCERTIFIED","kind":"IMPACT","relation":"CALLS","directions":["INCOMING","OUTGOING"],"depth":{"minimum":1,"maximum":64},"semantics":"target_rooted_admitted_directed_reachability_within_requested_direction_and_depth_without_traversal_expansion"}`, "sha256:e99184ad9674d6f16abdd9184585e0f53994268aee6ab8f2d6733d52a8ad2e65"},
	}
	for _, policy := range policies {
		t.Run(policy.name, func(t *testing.T) {
			if policy.document != policy.wantDocument || policy.digest != policy.wantDigest {
				t.Fatalf("golden policy drift: document=%q digest=%q", policy.document, policy.digest)
			}
			if policy.document == "" || policy.document[0] != '{' || policy.document[len(policy.document)-1] != '}' {
				t.Fatal("policy is not one compact JSON object")
			}
			for _, forbidden := range []string{"\n", "\r", "\t", " ", "\ufeff"} {
				if strings.Contains(policy.document, forbidden) {
					t.Fatalf("policy contains forbidden byte sequence %q", forbidden)
				}
			}
			if !json.Valid([]byte(policy.document)) {
				t.Fatal("policy is not JSON")
			}
			sum := sha256.Sum256([]byte(policy.document))
			got := "sha256:" + hex.EncodeToString(sum[:])
			if got != policy.digest {
				t.Fatalf("exact-byte digest drift: got=%s want=%s", got, policy.digest)
			}
			mutated := policy.document[:len(policy.document)-1] + " " + policy.document[len(policy.document)-1:]
			changed := sha256.Sum256([]byte(mutated))
			if "sha256:"+hex.EncodeToString(changed[:]) == policy.digest {
				t.Fatal("byte mutation did not change digest")
			}
		})
	}
}

func TestFutureStructuralResourceBoundsSchemaAndGoAlign(t *testing.T) {
	schemas := compileFutureStructuralSchemas(t)
	for _, tc := range []struct {
		field    string
		valid    int
		invalids []int
	}{
		{"max_messages", futureMaxMessages, []int{0, futureMaxMessages + 1}},
		{"max_bytes", futureMaxBytes, []int{0, futureMaxBytes + 1}},
	} {
		t.Run(tc.field, func(t *testing.T) {
			i, r := validFutureInput(), validFutureResult()
			i[tc.field], r["resource_policy"].(map[string]any)[tc.field] = tc.valid, tc.valid
			validateFuture(t, schemas[futureInputID], i, true)
			validateFuture(t, schemas[futureResultID], r, true)
			if err := ValidateFutureStructuralSemanticsV1(i, r); err != nil {
				t.Fatalf("Go validator rejected schema boundary: %v", err)
			}
			for _, invalid := range tc.invalids {
				i, r := validFutureInput(), validFutureResult()
				i[tc.field], r["resource_policy"].(map[string]any)[tc.field] = invalid, invalid
				validateFuture(t, schemas[futureInputID], i, false)
				validateFuture(t, schemas[futureResultID], r, false)
				if err := ValidateFutureStructuralSemanticsV1(i, r); err == nil {
					t.Fatalf("Go validator accepted out-of-bound %s=%d", tc.field, invalid)
				}
			}
		})
	}
}

func TestFutureStructuralSemanticPolicyAndResourceEquality(t *testing.T) {
	for name, mutate := range map[string]func(map[string]any, map[string]any){
		"arbitrary_traversal_digest": func(_ map[string]any, r map[string]any) {
			r["traversal_policy"].(map[string]any)["policy_digest"] = "sha256:" + strings.Repeat("0", 64)
		},
		"arbitrary_resource_digest": func(_ map[string]any, r map[string]any) {
			r["resource_policy"].(map[string]any)["policy_digest"] = "sha256:" + strings.Repeat("0", 64)
		},
		"arbitrary_analysis_digest": func(_ map[string]any, r map[string]any) {
			r["analysis_policy"].(map[string]any)["policy_digest"] = "sha256:" + strings.Repeat("0", 64)
		},
		"wrong_status": func(_ map[string]any, r map[string]any) {
			r["resource_policy"].(map[string]any)["policy_status"] = "CERTIFIED"
		},
		"traversal_parameter_mismatch": func(_ map[string]any, r map[string]any) { r["traversal_policy"].(map[string]any)["max_nodes"] = 101 },
		"message_parameter_mismatch":   func(_ map[string]any, r map[string]any) { r["resource_policy"].(map[string]any)["max_messages"] = 65 },
		"byte_parameter_mismatch":      func(_ map[string]any, r map[string]any) { r["resource_policy"].(map[string]any)["max_bytes"] = 4194305 },
		"request_timeout_exceeds_timeout": func(i map[string]any, r map[string]any) {
			i["request_timeout_ms"], r["resource_policy"].(map[string]any)["request_timeout_ms"] = 5001, 5001
		},
	} {
		t.Run(name, func(t *testing.T) {
			i, r := validFutureInput(), validFutureResult()
			mutate(i, r)
			if err := ValidateFutureStructuralSemanticsV1(i, r); err == nil {
				t.Fatal("semantic-v1 accepted invalid policy/resource contract")
			}
		})
	}
}
