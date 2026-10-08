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

func testCandidateBytes(t testing.TB) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "continuationhost", "testdata", "candidate-group-private-v2.json"))
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := censuscontinuation.ParseCandidateGroup(raw)
	if err != nil {
		t.Fatal(err)
	}
	if artifact.Authority != 0 || artifact.Accepted || artifact.Completeness != "UNKNOWN" || artifact.Interpretation.Status != censuscontinuation.InterpretationUnresolved {
		t.Fatalf("ASSERT_ADR0007_PRIVATE_CANDIDATE_CEILINGS_FIXTURE: authority=%d accepted=%t completeness=%q interpretation=%q", artifact.Authority, artifact.Accepted, artifact.Completeness, artifact.Interpretation.Status)
	}
	return raw
}

func testPublishInput(raw []byte) PublishRequest {
	return PublishRequest{
		CandidateBytes:        raw,
		GroupingPolicyID:      censuscontinuation.CandidateGroupPrivateV2ProfileID,
		GroupingPolicyDigest:  censuscontinuation.CandidateGroupPrivateV2ProfileDigest,
		QualificationID:       "candidate-group-boundary-artifact-validation/v1",
		SourceRevision:        "git:at55563ecf",
		PredecessorSelector:   "g-0000000000000000000000000000000000000000000000000000000000000000.selector.json",
		Representative:        Representative{ID: "3cdff2e81db031666b6d6d4d13711ddc2749722ac804ef79b103463ebfdda93c", Status: RepresentativeStatusSelectedByGroupingPolicy},
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
	raw := testCandidateBytes(t)
	published, err := adapter.PublishCandidateGeneration(context.Background(), testPublishInput(raw))
	if err != nil {
		t.Fatal(err)
	}
	if published.Generation == "" || published.VerificationSelector == "" || published.ArtifactSelector == "" || published.ReceiptSelector == "" {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_IMMUTABLE_GENERATION_IDENTITY: %+v", published)
	}
	if published.PublicationMechanism != publication.VerifiedGenerationMechanism {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_USES_VERIFIED_GENERATION: %+v", published)
	}
	if current, err := adapter.CurrentCandidateGeneration(context.Background()); err == nil || current.Selector != "" {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_PUBLISH_DOES_NOT_ADVANCE_SELECTOR: current=%+v err=%v", current, err)
	}
	advanced, err := adapter.AdvanceCandidateGeneration(context.Background(), AdvanceRequest{Generation: published.Generation, VerificationSelector: published.VerificationSelector, PredecessorSelector: testPublishInput(raw).PredecessorSelector})
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

func TestPrivateAdapterReceiptStrictlyBindsArtifactQualificationLineageBeforeSelector(t *testing.T) {
	root, _ := testPrivateRoot(t)
	adapter, err := NewRepositoryPrivateAdapter(root, Options{MaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	raw := testCandidateBytes(t)
	input := testPublishInput(raw)
	published, err := adapter.PublishCandidateGeneration(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	receiptBytes, err := root.ReadSelector(published.ReceiptSelector, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	var receipt Receipt
	if err := DecodeReceiptStrict(receiptBytes, &receipt); err != nil {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_STRICT_RECEIPT_CANONICAL: %v", err)
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
	if receipt.Representative.Status != RepresentativeStatusSelectedByGroupingPolicy || receipt.Representative.Inferred {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_REPRESENTATIVE_SEPARATELY_TYPED_NO_INFERENCE: %+v", receipt.Representative)
	}
	ordering := adapter.DebugCommittedOrder()
	if got := strings.Join(ordering, ">"); got != "artifact>receipt>selector" {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_ARTIFACT_RECEIPT_SELECTOR_ORDERING: %s", got)
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

func TestPrivateAdapterCancellationBeforeEachIrreversibleBoundary(t *testing.T) {
	for _, boundary := range []IrreversibleBoundary{BoundaryBeforeArtifactCommit, BoundaryBeforeReceiptCommit, BoundaryBeforeSelectorCommit} {
		t.Run(string(boundary), func(t *testing.T) {
			root, _ := testPrivateRoot(t)
			adapter, err := NewRepositoryPrivateAdapter(root, Options{MaxBytes: 1 << 20, TestCancelBefore: boundary})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			published, err := adapter.PublishCandidateGeneration(ctx, testPublishInput(testCandidateBytes(t)))
			if err == nil || published.Generation != "" {
				t.Fatalf("ASSERT_PRIVATE_ADAPTER_CANCELS_BEFORE_IRREVERSIBLE_BOUNDARY: boundary=%s published=%+v err=%v", boundary, published, err)
			}
			if got := adapter.DebugCommittedOrder(); len(got) != 0 {
				t.Fatalf("ASSERT_PRIVATE_ADAPTER_CANCEL_LEAVES_NO_LATER_COMMIT: boundary=%s order=%v", boundary, got)
			}
		})
	}
}

func TestPrivateAdapterCommittedSuccessSurvivesPostcommitVerificationFailure(t *testing.T) {
	root, _ := testPrivateRoot(t)
	adapter, err := NewRepositoryPrivateAdapter(root, Options{MaxBytes: 1 << 20, TestFailPostcommitVerification: true})
	if err != nil {
		t.Fatal(err)
	}
	published, err := adapter.PublishCandidateGeneration(context.Background(), testPublishInput(testCandidateBytes(t)))
	if err != nil || !published.Committed || published.PostcommitVerificationStatus != PostcommitVerificationFailed {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_TRUTHFUL_COMMITTED_SUCCESS_ON_POSTCOMMIT_VERIFY_FAILURE: published=%+v err=%v", published, err)
	}
}

func TestPrivateAdapterTamperCollisionRestartSafeRetrievalAndIdempotence(t *testing.T) {
	root, dir := testPrivateRoot(t)
	adapter, err := NewRepositoryPrivateAdapter(root, Options{MaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	raw := testCandidateBytes(t)
	first, err := adapter.PublishCandidateGeneration(context.Background(), testPublishInput(raw))
	if err != nil {
		t.Fatal(err)
	}
	again, err := adapter.PublishCandidateGeneration(context.Background(), testPublishInput(raw))
	if err != nil || again.Generation != first.Generation || again.VerificationSelector != first.VerificationSelector {
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
	if err := os.WriteFile(filepath.Join(dir, first.Generation, "receipt.json"), []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.GetCandidateGeneration(context.Background(), first.Generation); err == nil {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_TAMPER_REJECTED")
	}
	if _, err := adapter.PublishCandidateGeneration(context.Background(), PublishRequest{CandidateBytes: append([]byte(nil), raw...), ForceGenerationForTest: first.Generation}); err == nil {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_IMMUTABLE_COLLISION_REJECTED")
	}
}

func TestPrivateAdapterRejectsPublicRootAndDoesNotExposePublicSchema(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if adapter, err := NewRepositoryPrivateAdapter(root, Options{MaxBytes: 1 << 20}); err == nil || adapter != nil {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_PRIVATE_ONLY_ROOT: adapter=%+v err=%v", adapter, err)
	}
	if schemas := PublicSchemas(); len(schemas) != 0 {
		t.Fatalf("ASSERT_PRIVATE_ADAPTER_NO_PUBLIC_SCHEMA_EXPOSURE: %v", schemas)
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
