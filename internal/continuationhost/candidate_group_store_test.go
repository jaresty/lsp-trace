package continuationhost

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/programc"
	"lsp-trace/internal/programctestfixture"
)

func candidateGroupBytes(t testing.TB) (censuscontinuation.CandidateGroupArtifact, []byte) {
	return candidateGroupBytesWithProfile(t, censuscontinuation.FrozenCandidateGroupProfile())
}

func candidateGroupBytesWithProfile(t testing.TB, profile censuscontinuation.CandidateGroupProfile) (censuscontinuation.CandidateGroupArtifact, []byte) {
	t.Helper()
	o, failure := programc.Compute(programctestfixture.ValidV5(t), 19)
	if failure != nil {
		t.Fatal(failure)
	}
	b, err := programc.ComputeBoundary(o, programc.BoundaryRequest{PageRankTopK: 2, HubTopK: 2})
	if err != nil {
		t.Fatal(err)
	}
	a, err := censuscontinuation.BuildCandidateGroup(censuscontinuation.CandidateGroupInput{
		Outcome: o, Boundary: b,
		CommunityMembers: []string{"3cdff2e81db031666b6d6d4d13711ddc2749722ac804ef79b103463ebfdda93c", "4a4c524d563487befdff3720d59fb28740155453ebe6625b782f7612ef9e445f"},
		RepresentativeID: "3cdff2e81db031666b6d6d4d13711ddc2749722ac804ef79b103463ebfdda93c",
		Bounds:           profile.Bounds,
		ResourceProfile:  profile,
		Interpretation:   censuscontinuation.HostInterpretation{Status: censuscontinuation.InterpretationUnresolved, Attempts: 1, HostID: "caller-host", ModelID: "caller-model", ContextID: "caller-context"},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := a.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	return a, raw
}

func TestCandidateGroupStorePublicationFailureLeavesNoObject(t *testing.T) {
	root, dir := testRoot(t)
	store, err := NewStore(root, 65536)
	if err != nil {
		t.Fatal(err)
	}
	a, raw := candidateGroupBytes(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if id, err := store.PutCandidateGroup(ctx, raw); err == nil || id != "" {
		t.Fatalf("ASSERT_CANDIDATE_PUBLICATION_FAILURE: id=%q err=%v", id, err)
	}
	path, _ := candidateGroupPath(candidateGroupArtifactNamespace, a.ID())
	if _, err := os.Lstat(filepath.Join(dir, filepath.FromSlash(path))); !os.IsNotExist(err) {
		t.Fatalf("ASSERT_CANDIDATE_PUBLICATION_FAILURE_NO_OBJECT: %v", err)
	}
}

func TestCandidateGroupStoreRejectsImmutableCollision(t *testing.T) {
	root, dir := testRoot(t)
	store, err := NewStore(root, 65536)
	if err != nil {
		t.Fatal(err)
	}
	a, raw := candidateGroupBytes(t)
	path, _ := candidateGroupPath(candidateGroupArtifactNamespace, a.ID())
	full := filepath.Join(dir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("different"), 0o600); err != nil {
		t.Fatal(err)
	}
	if id, err := store.PutCandidateGroup(context.Background(), raw); err == nil || id != "" || !strings.Contains(err.Error(), "immutable collision") {
		t.Fatalf("ASSERT_CANDIDATE_IMMUTABLE_COLLISION: id=%q err=%v", id, err)
	}
}

func TestCandidateGroupStoreRetainedPrivateV2ExactPublicationRetrievalAndReadback(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "candidate-group-private-v2.json"))
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := censuscontinuation.ParseCandidateGroup(raw)
	if err != nil {
		t.Fatal(err)
	}
	const wantID = "sha256:c38ca11da73c48e901de6e9d03a6deb441ac4379af4589b29a7a6292b89a4b6f"
	const wantDigest = "sha256:89d57f2c5328f2f6d40df7e519bf6d835ede7dc0f8106bf87726f066b7021cd7"
	if artifact.ID() != wantID || artifact.ResourceProfileID != censuscontinuation.CandidateGroupPrivateV2ProfileID || artifact.ResourceProfileDigest != censuscontinuation.CandidateGroupPrivateV2ProfileDigest || Digest(raw) != wantDigest || len(raw) != 39381 {
		t.Fatalf("ASSERT_RETAINED_V2_FROZEN_IDENTITY: artifact=%+v digest=%s length=%d", artifact, Digest(raw), len(raw))
	}
	root, _ := testRoot(t)
	store, err := NewStore(root, 65536)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := store.PutCandidateGroupWithReceipt(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	const wantSelector = "continuations/candidate-groups/artifacts/c38ca11da73c48e901de6e9d03a6deb441ac4379af4589b29a7a6292b89a4b6f"
	if receipt != (CandidateGroupPublicationReceipt{ArtifactID: wantID, Selector: wantSelector, Digest: wantDigest, ByteLength: 39381}) {
		t.Fatalf("ASSERT_RETAINED_V2_PRIVATE_RECEIPT: %+v", receipt)
	}
	got, err := store.GetCandidateGroup(context.Background(), wantID)
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatalf("ASSERT_RETAINED_V2_EXACT_GET: err=%v equal=%v", err, bytes.Equal(got, raw))
	}
	if _, err := censuscontinuation.ParseCandidateGroup(got); err != nil {
		t.Fatalf("ASSERT_RETAINED_V2_READBACK_PARSE: %v", err)
	}
	again, err := store.PutCandidateGroupWithReceipt(context.Background(), raw)
	if err != nil || again != receipt {
		t.Fatalf("ASSERT_RETAINED_V2_IDEMPOTENT_PUT: receipt=%+v err=%v", again, err)
	}
}

func TestCandidateGroupStorePrivateV2ReceiptExactRetrievalAndIdempotence(t *testing.T) {
	root, _ := testRoot(t)
	store, err := NewStore(root, 65536)
	if err != nil {
		t.Fatal(err)
	}
	a, raw := candidateGroupBytesWithProfile(t, censuscontinuation.CandidateGroupPrivateV2Profile())
	receipt, err := store.PutCandidateGroupWithReceipt(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	wantSelector, _ := candidateGroupPath(candidateGroupArtifactNamespace, a.ID())
	if receipt.ArtifactID != a.ID() || receipt.Selector != wantSelector || receipt.Digest != Digest(raw) || receipt.ByteLength != uint64(len(raw)) {
		t.Fatalf("ASSERT_CANDIDATE_V2_PRIVATE_RECEIPT: %+v", receipt)
	}
	got, err := store.GetCandidateGroup(context.Background(), receipt.ArtifactID)
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatalf("ASSERT_CANDIDATE_V2_EXACT_RETRIEVAL: err=%v equal=%v", err, bytes.Equal(got, raw))
	}
	again, err := store.PutCandidateGroupWithReceipt(context.Background(), raw)
	if err != nil || again != receipt {
		t.Fatalf("ASSERT_CANDIDATE_V2_IDEMPOTENT_PUT: receipt=%+v err=%v", again, err)
	}
	foreign := append([]byte(nil), raw...)
	foreign = bytes.Replace(foreign, []byte(censuscontinuation.CandidateGroupPrivateV2ProfileID), []byte(censuscontinuation.CandidateGroupPrivateV1ProfileID), 1)
	if bad, err := store.PutCandidateGroupWithReceipt(context.Background(), foreign); err == nil || bad != (CandidateGroupPublicationReceipt{}) {
		t.Fatalf("ASSERT_CANDIDATE_V2_FOREIGN_PROFILE_REJECTED: receipt=%+v err=%v", bad, err)
	}
}

func TestCandidateGroupStoreSinglePutRetrievesExactBytesByArtifactID(t *testing.T) {
	root, _ := testRoot(t)
	store, err := NewStore(root, 65536)
	if err != nil {
		t.Fatal(err)
	}
	a, raw := candidateGroupBytes(t)
	artifactID, err := store.PutCandidateGroup(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	if artifactID != a.ID() {
		t.Fatalf("identity mismatch: %s", artifactID)
	}
	byArtifact, err := store.GetCandidateGroup(context.Background(), artifactID)
	if err != nil || !bytes.Equal(byArtifact, raw) {
		t.Fatalf("artifact retrieval mismatch: %v", err)
	}
	again, err := store.PutCandidateGroup(context.Background(), raw)
	if err != nil || again != artifactID {
		t.Fatalf("idempotent put mismatch: id=%s err=%v", again, err)
	}
	if _, err := store.GetCandidateGroup(context.Background(), a.CommunityID); err == nil {
		t.Fatal("community id accepted as artifact id")
	}
}
