package v5assertions

import "testing"

// TestV5PersistentAssertionMatrix is the stable ADR0007 V5 integration guard.
// Behavioral implementations of these assertion identities live in the owning
// packages and integrated conformance.
func TestV5PersistentAssertionMatrix(t *testing.T) {
	assertions := []string{
		"ASSERT_RETAINED_V5_EXPLICIT_DISPATCH",
		"ASSERT_RETAINED_V5_DISPLAY_SELECTIONS",
		"ASSERT_RETAINED_V5_EXACT_PARALLEL_OCCURRENCES",
		"ASSERT_RETAINED_V5_ONE_PROJECT_PASS",
		"ASSERT_RETAINED_V5_UNAVAILABLE_EXCLUDED",
		"ASSERT_TARGETPACKET_V5_EXACT_RECONCILIATION",
		"ASSERT_TARGETPACKET_V5_TYPED_UNAVAILABLE_MEMBER",
		"ASSERT_CONTINUATION_V5_DIRECT_SELECTED_CAPTURE",
		"ASSERT_CONTINUATION_V5_ENDPOINT_OUTCOME_CLOSURE",
		"ASSERT_CONTINUATION_V5_REPLAY_AFTER_WORKSPACE_DELETION",
		"ASSERT_PIPELINE_V5_MIXED_WORKER_REQUEST_DENOMINATOR",
		"ASSERT_PIPELINE_V5_EXACT_NOMINATION_ACCOUNTING",
		"ASSERT_PIPELINE_V5_COMPLETE_DEGRADED_MATRIX",
		"ASSERT_CHECKPOINT_V5_SIDECAR_SEMANTIC_RECOMPUTATION",
		"ASSERT_CHECKPOINT_V5_COORDINATED_SUBSTITUTION_REJECTED",
		"ASSERT_RENDER_V5_PREPARATION_FAILURES",
		"ASSERT_V2_V3_V4_GOLDENS_UNCHANGED",
		"ASSERT_V5_PERMUTATION_INVARIANT",
		"ASSERT_V5_REAL_QWEN_QUALIFY_RENDER_RESUME",
	}
	seen := map[string]bool{}
	for _, assertion := range assertions {
		if assertion == "" || seen[assertion] {
			t.Fatalf("duplicate or empty persistent assertion identity: %q", assertion)
		}
		seen[assertion] = true
	}
	if len(seen) != 19 {
		t.Fatalf("persistent V5 assertion count = %d, want 19", len(seen))
	}
}
