package targetpacket

import (
	"testing"
	"time"

	rg "lsp-trace/internal/referencegrouping"
	"lsp-trace/internal/sourceobject"
	"lsp-trace/sessionruntime"
)

// Positive actual-Manager/producer fixtures and field substitutions live in the
// owned inventorysuccessor test, which exercises these APIs without a test-only
// admission issuer. These local tests deliberately cannot mint a positive token.
func TestPrivateSourceAdmissionZeroAndFiniteContract(t *testing.T) {
	valid := PrivateSourceLimits{MaxDocumentBytes: 1 << 20, MaxTotalBytes: 8 << 20, MaxObjects: 1000, MaxWork: 10000}
	if !valid.Valid() {
		t.Fatal("ASSERT_SOURCE_FINITE_LIMITS_POSITIVE")
	}
	for _, l := range []PrivateSourceLimits{{}, {MaxDocumentBytes: 1<<20 + 1, MaxTotalBytes: 8 << 20, MaxObjects: 1000, MaxWork: 10000}, {MaxDocumentBytes: 1 << 20, MaxTotalBytes: 8<<20 + 1, MaxObjects: 1000, MaxWork: 10000}, {MaxDocumentBytes: 1 << 20, MaxTotalBytes: 8 << 20, MaxObjects: 1001, MaxWork: 10000}, {MaxDocumentBytes: 1 << 20, MaxTotalBytes: 8 << 20, MaxObjects: 1000, MaxWork: 10001}} {
		if l.Valid() {
			t.Fatal("ASSERT_SOURCE_NO_CEILING_INCREASE")
		}
	}
	if a, _ := CalculatePrivateEndpointSource(PrivateSourceIntent{}, CandidateOriginals{}, nil, nil, valid); a.Valid() {
		t.Fatal("ASSERT_SOURCE_ZERO_ORIGINALS_REFUSED")
	}
	if _, e := RetainPrivateSourceCandidate(sessionruntime.PrivateGroupingSourceEvidence{}, rg.AdmittedInput{}, nil, []byte("caller query"), time.Now(), "caller workspace"); e == nil {
		t.Fatal("ASSERT_SOURCE_DTO_NOT_ISSUANCE")
	}
	if (PrivateSourceCandidate{}).Matches(PrivateSourceIntent{}, CandidateOriginals{}) {
		t.Fatal("ASSERT_SOURCE_ZERO_CANDIDATE_MATCH")
	}
	if _, e := PreparePrivateSourceCandidate(CandidateOriginals{}, valid); e == nil {
		t.Fatal("ASSERT_SOURCE_ZERO_PREPARE")
	}
	if _, e := (privateSourceObjects{}).Get(sourceobject.Identity{}); e == nil {
		t.Fatal("ASSERT_SOURCE_ABSENT_OBJECT_NOT_EMPTY")
	}
	t.Log("ASSERT_SOURCE_DTO_NOT_ISSUANCE: PASS; ASSERT_SOURCE_ZERO_ORIGINALS_REFUSED: PASS; ASSERT_SOURCE_NO_CEILING_INCREASE: PASS")
}
