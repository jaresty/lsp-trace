package schema

import (
	"fmt"
	"strings"
	"testing"
)

const validSourceDenominatorV1 = `{
  "denominator_schema_version":"lsp-trace.source-denominator.v1",
  "scope":{"class":"SOURCE_SNAPSHOT_SCOPE","canonical_member_selector":"manifest:src@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","inputs":["manifest:src"],"source_manifest_digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","independently_bounded":true},
  "scope_policy":{"id":"source-snapshot","version":"1"},
  "derivation_policy":{"id":"manifest-universe","version":"1","basis":"ADMITTED_SOURCE_MANIFEST"},
  "members":[{"member_id":"a.go"},{"member_id":"b.go"}],
  "exclusions":[{"member_id":"b.go","reason":"POLICY_EXCLUDED"}],
  "covered_members":[{"member_id":"a.go","evidence_ids":["relation:1"]}],
  "counts":{"universe":2,"members":2,"excluded":1,"covered":1},
  "logical_digest":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
  "semantic_validator":{"id":"lsp-trace","version":"1","result":"PASS"},
  "extractor_qualification":{"extractor_id":"gopls","extractor_version":"v1","scope_policy_id":"source-snapshot","scope_policy_version":"1","operation":"DENOMINATOR_DERIVATION","status":"PASS"}
}`

func TestSourceDenominatorV1AcceptsCompleteSnapshotScope(t *testing.T) {
	got, err := ValidateFor([]byte(validSourceDenominatorV1), FamilySourceDenominator, "v1")
	if err != nil || got != SourceDenominatorVersionV1 {
		t.Fatalf("ASSERT_SOURCE_DENOMINATOR_VALID: version=%q err=%v", got, err)
	}
}

func TestSourceDenominatorV1RejectsInvalidScopeAndDenominatorProofs(t *testing.T) {
	tests := []struct {
		name       string
		old, new   string
		old2, new2 string
		want       string
	}{
		{"unbounded scope", `"independently_bounded":true`, `"independently_bounded":false`, "", "", "scope universe must be independently bounded"},
		{"provider-derived universe", `"basis":"ADMITTED_SOURCE_MANIFEST"`, `"basis":"PROVIDER_RETURNS"`, "", "", "schema validation"},
		{"scope policy mismatch", `"scope_policy_version":"1"`, `"scope_policy_version":"2"`, "", "", "qualification scope policy must exactly match"},
		{"qualification failure", `"status":"PASS"`, `"status":"FAIL"`, "", "", "schema validation"},
		{"non-exhaustive universe", `"universe":2`, `"universe":3`, "", "", "universe count"},
		{"unknown exclusion", `"member_id":"b.go","reason":"POLICY_EXCLUDED"`, `"member_id":"missing.go","reason":"POLICY_EXCLUDED"`, "", "", "exclusion member"},
		{"invalid exclusion reason", `"reason":"POLICY_EXCLUDED"`, `"reason":"UNREVIEWED"`, "", "", "schema validation"},
		{"overlap covered and excluded", `"member_id":"a.go","evidence_ids"`, `"member_id":"b.go","evidence_ids"`, "", "", "both covered and excluded"},
		{"unaccounted member", `"exclusions":[{"member_id":"b.go","reason":"POLICY_EXCLUDED"}]`, `"exclusions":[]`, `"excluded":1`, `"excluded":0`, "members must be exhausted"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mutated := strings.Replace(validSourceDenominatorV1, tc.old, tc.new, 1)
			if tc.old2 != "" {
				mutated = strings.Replace(mutated, tc.old2, tc.new2, 1)
			}
			if mutated == validSourceDenominatorV1 {
				t.Fatalf("bad fixture mutation: %s", tc.name)
			}
			_, err := ValidateFor([]byte(mutated), FamilySourceDenominator, "v1")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ASSERT_SOURCE_DENOMINATOR_REJECT_%s: %v", strings.ToUpper(strings.ReplaceAll(tc.name, " ", "_")), err)
			}
		})
	}
}

func TestSourceDenominatorV1AcceptsVerifiedMembershipCommitment(t *testing.T) {
	committed := strings.Replace(validSourceDenominatorV1,
		`"members":[{"member_id":"a.go"},{"member_id":"b.go"}]`,
		`"membership_commitment":{"digest":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc","member_ids":["a.go","b.go"],"equality_verified":true}`,
		1)
	if _, err := ValidateFor([]byte(committed), FamilySourceDenominator, "v1"); err != nil {
		t.Fatalf("ASSERT_SOURCE_DENOMINATOR_VERIFIED_COMMITMENT_VALID: %v", err)
	}
	unopened := strings.Replace(committed, `"equality_verified":true`, `"equality_verified":false`, 1)
	if _, err := ValidateFor([]byte(unopened), FamilySourceDenominator, "v1"); err == nil {
		t.Fatal("ASSERT_SOURCE_DENOMINATOR_UNOPENED_COMMITMENT_REJECTED: accepted")
	}
}

func TestSourceDenominatorV1RejectsDuplicateIdentitiesAndBadCounts(t *testing.T) {
	for name, replacement := range map[string]string{
		"duplicate member": `{"member_id":"a.go"},{"member_id":"a.go"}`,
		"members count":    `{"member_id":"a.go"},{"member_id":"b.go"},{"member_id":"c.go"}`,
	} {
		t.Run(name, func(t *testing.T) {
			mutated := strings.Replace(validSourceDenominatorV1, `{"member_id":"a.go"},{"member_id":"b.go"}`, replacement, 1)
			_, err := ValidateFor([]byte(mutated), FamilySourceDenominator, "v1")
			if err == nil {
				t.Fatalf("ASSERT_SOURCE_DENOMINATOR_IDENTITY_OR_COUNT_%s: accepted", fmt.Sprint(name))
			}
		})
	}
}
