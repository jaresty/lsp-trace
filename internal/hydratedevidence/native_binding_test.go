package hydratedevidence

import "testing"

func TestMergeRepeatedNativeBindingUnionsSourcesAndRejectsConflict(t *testing.T) {
	base := Record{ID: "native:/nodes/0/range", ArtifactDigest: "sha256:a", Pointer: "/nodes/0/range", Kind: "NATIVE_RECORD", NativeID: "node-1", Authority: Native, Qualification: "INTEGRITY_ONLY", SourceAttribution: "SOURCE", AnchorStatus: "VALID", SourceIDs: []string{"native:b"}, RelationshipReferences: []string{}, Encoding: "utf-16"}
	next := base
	next.SourceIDs = []string{"native:a", "native:b"}
	merged, err := mergeRepeatedNativeBinding(base, next)
	if err != nil {
		t.Fatalf("ASSERT_V5_HYDRATION_REPEATED_BINDING_COALESCES: %v", err)
	}
	if len(merged.SourceIDs) != 2 || merged.SourceIDs[0] != "native:a" || merged.SourceIDs[1] != "native:b" {
		t.Fatalf("ASSERT_V5_HYDRATION_REPEATED_BINDING_SOURCE_UNION: %#v", merged.SourceIDs)
	}
	conflict := next
	conflict.AnchorStatus = "INVALID_COORDINATES"
	if _, err := mergeRepeatedNativeBinding(base, conflict); err == nil {
		t.Fatal("ASSERT_V5_HYDRATION_REPEATED_BINDING_CONFLICT_REJECTED")
	}
}
