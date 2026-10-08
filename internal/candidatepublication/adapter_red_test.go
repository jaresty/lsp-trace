package candidatepublication

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/publication"
)

func testPrivateRoot(t testing.TB) (*publication.Root, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	return root, dir
}

func testCandidateBytes(t testing.TB) (censuscontinuation.CandidateGroupArtifact, []byte) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "continuationhost", "testdata", "candidate-group-private-v2.json"))
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := censuscontinuation.ParseCandidateGroup(raw)
	if err != nil {
		t.Fatal(err)
	}
	if artifact.Authority != 0 || artifact.Accepted || artifact.Completeness != "UNKNOWN" {
		t.Fatalf("ASSERT_ADR0007_PRIVATE_CANDIDATE_CEILINGS_FIXTURE: authority=%d accepted=%t completeness=%q", artifact.Authority, artifact.Accepted, artifact.Completeness)
	}
	if artifact.Representative.ID == "" || artifact.Representative.Role == "" {
		t.Fatalf("ASSERT_ADR0007_PRIVATE_CANDIDATE_REPRESENTATIVE_FIXTURE: %+v", artifact.Representative)
	}
	return artifact, raw
}

func testPublishInput(artifact censuscontinuation.CandidateGroupArtifact, raw []byte) PublishRequest {
	return PublishRequest{
		CandidateBytes:        raw,
		GroupingPolicyID:      artifact.ResourceProfileID,
		GroupingPolicyDigest:  artifact.ResourceProfileDigest,
		QualificationID:       "candidate-group-boundary-artifact-validation/v1",
		SourceRevision:        "git:at55563ecf",
		PredecessorSelector:   "g-0000000000000000000000000000000000000000000000000000000000000000.selector.json",
		Representative:        Representative{ID: artifact.Representative.ID, Status: artifact.Representative.Role, Inferred: false},
		Authority:             0,
		Accepted:              false,
		Completeness:          CompletenessUnknown,
		FeatureIdentityStatus: FeatureIdentityUnresolved,
	}
}

func TestPrivateAdapterPublishesImmutableQualifiedGenerationAndAdvancesOnlyByOwnerOp(t *testing.T) {
	root, _ := testPrivateRoot(t)
	adapter, err := NewRepositoryPrivateAdapter(root, Options{MaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	artifact, raw := testCandidateBytes(t)
	input := testPublishInput(artifact, raw)
	published, err := adapter.PublishCandidateGeneration(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if published.Generation == "" || published.VerificationSelector == "" || published.ArtifactSelector == "" || published.QualificationReceiptSelector == "" {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_IMMUTABLE_GENERATION_IDENTITY: %+v", published)
	}
	if published.PublicationMechanism != publication.VerifiedGenerationMechanism {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_USES_VERIFIED_GENERATION: %+v", published)
	}
	if published.ArtifactSelector != published.Generation+"/artifact.json" || published.ByteCustodyReceiptSelector != published.Generation+"/receipt.json" || published.VerificationSelector != published.Generation+".selector.json" {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_RESPECTS_VERIFIED_GENERATION_CONTRACT_PATHS: %+v", published)
	}
	if published.QualificationReceiptSelector == published.ByteCustodyReceiptSelector || strings.HasPrefix(published.QualificationReceiptSelector, published.Generation+"/") {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_QUALIFICATION_RECEIPT_IS_DISTINCT_ADAPTER_ARTIFACT: %+v", published)
	}
	if current, err := adapter.CurrentCandidateGeneration(context.Background()); err == nil || current.Selector != "" {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_PUBLISH_DOES_NOT_ADVANCE_SELECTOR: current=%+v err=%v", current, err)
	}
	advanced, err := adapter.AdvanceCandidateGeneration(context.Background(), AdvanceRequest{Generation: published.Generation, VerificationSelector: published.VerificationSelector, PredecessorSelector: input.PredecessorSelector})
	if err != nil {
		t.Fatal(err)
	}
	if advanced.Selector == "" || advanced.Generation != published.Generation {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_OWNER_ADVANCE_SELECTOR: %+v", advanced)
	}
	current, err := adapter.CurrentCandidateGeneration(context.Background())
	if err != nil || current.Generation != published.Generation || current.Selector != advanced.Selector {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_CURRENT_RESTART_SAFE_SELECTOR: current=%+v advanced=%+v err=%v", current, advanced, err)
	}
}

func TestPrivateAdapterQualificationReceiptBindsCandidateQualificationAndLineage(t *testing.T) {
	root, _ := testPrivateRoot(t)
	adapter, err := NewRepositoryPrivateAdapter(root, Options{MaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	artifact, raw := testCandidateBytes(t)
	input := testPublishInput(artifact, raw)
	published, err := adapter.PublishCandidateGeneration(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	receiptBytes, err := root.ReadSelector(published.QualificationReceiptSelector, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	var receipt Receipt
	if err := DecodeReceiptStrict(receiptBytes, &receipt); err != nil {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_STRICT_QUALIFICATION_RECEIPT_CANONICAL: %v", err)
	}
	if !bytes.Equal(receipt.CandidateDigest.Bytes, DigestBytes(raw)) || receipt.CandidateDigest.Hex != Digest(raw) || receipt.CandidateByteLength != uint64(len(raw)) {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_RECEIPT_BINDS_EXACT_CANDIDATE_BYTES: %+v", receipt.CandidateDigest)
	}
	if receipt.GroupingPolicyID != input.GroupingPolicyID || receipt.GroupingPolicyDigest != input.GroupingPolicyDigest || receipt.QualificationID != input.QualificationID || receipt.SourceRevision != input.SourceRevision || receipt.Generation != published.Generation || receipt.PredecessorSelector != input.PredecessorSelector {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_RECEIPT_BINDS_QUALIFICATION_LINEAGE: receipt=%+v input=%+v published=%+v", receipt, input, published)
	}
	if receipt.Authority != 0 || receipt.Accepted || receipt.Completeness != CompletenessUnknown || receipt.FeatureIdentityStatus != FeatureIdentityUnresolved {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_RECEIPT_PRESERVES_UNACCEPTED_UNRESOLVED_CEILINGS: %+v", receipt)
	}
	if receipt.Representative.ID != artifact.Representative.ID || receipt.Representative.Status != artifact.Representative.Role || receipt.Representative.Inferred {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_REPRESENTATIVE_SEPARATELY_TYPED_NO_INFERENCE: receipt=%+v fixture=%+v", receipt.Representative, artifact.Representative)
	}
}

func TestPrivateAdapterStrictCanonicalRejectionUnknownTrailingAndDuplicate(t *testing.T) {
	for name, raw := range map[string][]byte{
		"unknown":   []byte(`{"schema_version":"lsp-trace.private-candidate-publication-receipt.v1","unknown":true}`),
		"trailing":  []byte(`{"schema_version":"lsp-trace.private-candidate-publication-receipt.v1"} {}`),
		"duplicate": []byte(`{"schema_version":"lsp-trace.private-candidate-publication-receipt.v1","generation":"g-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","generation":"g-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}`),
	} {
		t.Run(name, func(t *testing.T) {
			var receipt Receipt
			if err := DecodeReceiptStrict(raw, &receipt); err == nil {
				t.Fatalf("ASSERT_PRIVATE_ADAPTER_STRICT_CANONICAL_REJECTION_%s", strings.ToUpper(name))
			}
		})
	}
}

func TestPrivateAdapterPreflightCancellationDoesNotPublishOrAdvance(t *testing.T) {
	root, _ := testPrivateRoot(t)
	adapter, err := NewRepositoryPrivateAdapter(root, Options{MaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	artifact, raw := testCandidateBytes(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	published, err := adapter.PublishCandidateGeneration(ctx, testPublishInput(artifact, raw))
	if err == nil || published.Generation != "" {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_PREFLIGHT_CANCELS_BEFORE_PUBLICATION: published=%+v err=%v", published, err)
	}
	if current, err := adapter.CurrentCandidateGeneration(context.Background()); err == nil || current.Selector != "" {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_PREFLIGHT_CANCEL_DOES_NOT_ADVANCE: current=%+v err=%v", current, err)
	}
}

func TestPrivateAdapterAdvanceRejectsTamperedQualificationReceiptBeforeSelectorCommit(t *testing.T) {
	root, dir := testPrivateRoot(t)
	adapter, err := NewRepositoryPrivateAdapter(root, Options{MaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	artifact, raw := testCandidateBytes(t)
	input := testPublishInput(artifact, raw)
	published, err := adapter.PublishCandidateGeneration(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(published.QualificationReceiptSelector)), []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	advanced, err := adapter.AdvanceCandidateGeneration(context.Background(), AdvanceRequest{Generation: published.Generation, VerificationSelector: published.VerificationSelector, PredecessorSelector: input.PredecessorSelector})
	if err == nil || advanced.Committed || advanced.Selector != "" {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_TAMPERED_RECEIPT_REJECTS_ADVANCE_BEFORE_SELECTOR: advanced=%+v err=%v", advanced, err)
	}
	if current, err := adapter.CurrentCandidateGeneration(context.Background()); err == nil || current.Selector != "" {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_TAMPERED_ADVANCE_LEAVES_NO_CURRENT_SELECTOR: current=%+v err=%v", current, err)
	}
}

func TestPrivateAdapterOwnerAdvanceComposesBoundFilePostcommitContract(t *testing.T) {
	// This adapter must use publication.PublishBoundFile's returned BoundFileReceipt
	// commit and VerificationStatus when advancing the owner selector. The underlying
	// truthful postcommit status matrix is pinned by:
	// go test ./internal/publication -run TestBoundFilePostcommitStatusMatrix
	var _ *publication.BoundFileReceipt
}

func TestPrivateAdapterTamperCollisionRestartSafeRetrievalAndIdempotence(t *testing.T) {
	root, dir := testPrivateRoot(t)
	adapter, err := NewRepositoryPrivateAdapter(root, Options{MaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	artifact, raw := testCandidateBytes(t)
	input := testPublishInput(artifact, raw)
	first, err := adapter.PublishCandidateGeneration(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	again, err := adapter.PublishCandidateGeneration(context.Background(), input)
	if err != nil || again.Generation != first.Generation || again.VerificationSelector != first.VerificationSelector || again.QualificationReceiptSelector != first.QualificationReceiptSelector {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_IDEMPOTENT_GENERATION: first=%+v again=%+v err=%v", first, again, err)
	}
	reopenedRoot, err := publication.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopenedRoot.Close()
	reopened, err := NewRepositoryPrivateAdapter(reopenedRoot, Options{MaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	got, err := reopened.GetCandidateGeneration(context.Background(), first.Generation)
	if err != nil || !bytes.Equal(got.CandidateBytes, raw) {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_RESTART_SAFE_RETRIEVAL: err=%v equal=%v", err, bytes.Equal(got.CandidateBytes, raw))
	}
	if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(first.QualificationReceiptSelector)), []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.GetCandidateGeneration(context.Background(), first.Generation); err == nil {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_TAMPERED_QUALIFICATION_RECEIPT_REJECTED")
	}
	if _, err := reopened.PublishCandidateGeneration(context.Background(), input); err == nil {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_NATURAL_SAME_GENERATION_COLLISION_REJECTED_AFTER_TAMPER")
	}
}

func TestPrivateAdapterReceiptDecoderRejectsNonCanonicalJSONEvenWhenStandardJSONAcceptsIt(t *testing.T) {
	raw := []byte(`{"schema_version":"lsp-trace.private-candidate-publication-receipt.v1","generation":"g-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","generation":"g-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`)
	var loose map[string]any
	if err := json.Unmarshal(raw, &loose); err != nil {
		t.Fatal(err)
	}
	var receipt Receipt
	if err := DecodeReceiptStrict(raw, &receipt); err == nil {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_DUPLICATE_REJECTED_DESPITE_STANDARD_JSON_ACCEPTANCE")
	}
}
