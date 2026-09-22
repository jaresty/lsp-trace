package v4assertions

import "testing"

// TestV4PersistentAssertionMatrix is the stable inventory guard. Behavioral
// implementations of these assertion identities live in the owning packages.
func TestV4PersistentAssertionMatrix(t *testing.T) {
	assertions := []string{
		"ASSERT_V4_PARENT_V3_EXACT_BYTES",
		"ASSERT_V4_V3_OCCURRENCES_UNCHANGED",
		"ASSERT_V4_ENDPOINT_NOT_V1_BOUND",
		"ASSERT_V4_NO_ARBITRARY_ENDPOINTS",
		"ASSERT_V4_SOURCE_UNAVAILABLE_NO_SUBSTITUTE",
		"ASSERT_V4_CONSTITUENT_IDENTITY_COMPLETE",
		"ASSERT_V4_CANONICAL_ORDER",
		"ASSERT_V4_CLOSED_SCHEMA",
		"ASSERT_V4_CAPTURE_ONLY_WORKSPACE_READ",
		"ASSERT_RETAINED_V4_ONE_ARTIFACT_SET",
		"ASSERT_RETAINED_V4_ONE_PROJECT_PASS",
		"ASSERT_RETAINED_V3_OCCURRENCE_EXACT",
		"ASSERT_TARGETPACKET_V4_RECONCILIATION",
		"ASSERT_V1_V2_V3_GOLDENS_UNCHANGED",
		"ASSERT_V4_AUTHORITY_TUPLE",
		"ASSERT_PUBLICATION_V4_REGISTERED_ADDITIVELY",
	}
	seen := map[string]bool{}
	for _, assertion := range assertions {
		if assertion == "" || seen[assertion] {
			t.Fatalf("duplicate or empty persistent assertion identity: %q", assertion)
		}
		seen[assertion] = true
	}
	if len(seen) != 16 {
		t.Fatalf("persistent V4 assertion count = %d, want 16", len(seen))
	}
}
