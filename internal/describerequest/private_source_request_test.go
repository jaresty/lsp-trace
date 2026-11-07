package describerequest

import (
	"lsp-trace/internal/targetpacket"
	"testing"
)

// This package formats wire candidates, never producer admission.
func TestPrivateSourceRequestCandidateBounds(t *testing.T) {
	for _, v := range []struct{ deadline, bytes int }{{1000, 1 << 20}, {0, 1 << 20}, {1000, 0}, {1000, 8<<20 + 1}} {
		if _, e := FormatPrivateSourceRequestCandidate(targetpacket.PrivateSourceUnit{}, v.deadline, v.bytes); e == nil {
			t.Fatal("ASSERT_SOURCE_REQUEST_CANDIDATE_REQUIRES_UNIT")
		}
	}
	if _, e := CanonicalPrivateSourceRequests([]Record{{}}); e == nil {
		t.Fatal("ASSERT_SOURCE_REQUEST_INTRINSIC_VALIDATE_REQUIRED")
	}
	if r, e := CanonicalPrivateSourceRequests(nil); e != nil || len(r) != 0 {
		t.Fatal("ASSERT_SOURCE_REQUEST_EMPTY_NOT_ADMISSION")
	}
	if ValidateRecord(Record{}) == nil {
		t.Fatal("ASSERT_SOURCE_REQUEST_PUBLIC_VALIDATE_UNCHANGED")
	}
	t.Log("ASSERT_SOURCE_REQUEST_CANDIDATE_REQUIRES_UNIT: PASS; ASSERT_SOURCE_REQUEST_INTRINSIC_VALIDATE_REQUIRED: PASS")
}
