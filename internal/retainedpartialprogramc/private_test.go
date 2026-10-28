package retainedpartialprogramc

import (
	"crypto/sha256"
	"fmt"
	"testing"

	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/programc"
)

func TestPrivateProfileAndClosedEntry(t *testing.T) {
	if fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(ProfileDefinition))) != ProfileDigest || ProfileID == programc.ProfileID || ProfileDigest == programc.ProfileDigest {
		t.Fatal("ASSERT_DISTINCT_PINNED_PROFILE")
	}
	if _, f := Project(pilot.DerivedRetainedPartialNativePrivate{}); f == nil || f.Code != programc.CodeInvalidProvenance {
		t.Fatal("ASSERT_CONCRETE_PROOF_REQUIRED")
	}
	if _, f := ComputeProjection(Projection{}, 42); f == nil || f.Code != programc.CodeInvalidProvenance {
		t.Fatal("ASSERT_OPAQUE_PROJECTION_REQUIRED")
	}
}
