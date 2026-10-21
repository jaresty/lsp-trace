package censusrequest

import (
	"bytes"
	"testing"
)

func TestReceiptV1NoPruneBytesRemainExact(t *testing.T) {
	r, err := New(RefreshCoordinate{"s", 1}, SemanticFields{Sources: []string{"."}, Includes: []string{}, Excludes: []string{}, DownDepth: 1, MaxNodes: 10000, BatchTargets: 63, TimeoutMS: 60000, RequestTimeoutMS: 30000})
	if err != nil {
		t.Fatal(err)
	}
	got, err := MarshalReceipt(r)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte(`{"schema_version":"lsp-trace.census-request-receipt.v1","refresh_required":{"session_id":"s","generation":1},"semantic":{"sources":["."],"includes":[],"excludes":[],"down_depth":1,"up_depth":0,"max_nodes":10000,"batch_targets":63,"timeout_ms":60000,"request_timeout_ms":30000},"fingerprint":"sha256:46e4d6f0cbb8580a48dfef64380f590b3d23c89a44d7e0ed428df287d660623c"}`)
	if !bytes.Equal(got, want) {
		t.Fatalf("ASSERT_CENSUS_REQUEST_V1_EXACT_BYTES: got=%s want=%s", got, want)
	}
}

func TestReceiptV2PruneIdentity(t *testing.T) {
	base := SemanticFieldsV2{Sources: []string{"."}, Includes: []string{}, Excludes: []string{}, PrunedDirectories: []string{"tmp", "vendor"}, DownDepth: 1, MaxNodes: 10000, BatchTargets: 63, TimeoutMS: 60000, RequestTimeoutMS: 30000}
	a, err := NewV2(RefreshCoordinate{"s", 1}, base)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewV2(RefreshCoordinate{"s", 1}, base)
	if err != nil {
		t.Fatal(err)
	}
	if a.Fingerprint != b.Fingerprint {
		t.Fatal("ASSERT_CENSUS_REQUEST_V2_STABLE")
	}
	changed := base
	changed.PrunedDirectories = []string{"vendor"}
	c, err := NewV2(RefreshCoordinate{"s", 1}, changed)
	if err != nil {
		t.Fatal(err)
	}
	if a.Fingerprint == c.Fingerprint {
		t.Fatal("ASSERT_CENSUS_REQUEST_V2_PRUNE_SENSITIVE")
	}
}
