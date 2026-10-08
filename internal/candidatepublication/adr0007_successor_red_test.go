//go:build linux || darwin

package candidatepublication

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/publication"
)

func TestADR0007SameCandidateDifferentQualificationOrdinaryPredecessorIsImmutableAliasCollision(t *testing.T) {
	root, _ := testPrivateRoot(t)
	adapter, err := NewRepositoryPrivateAdapter(root, Options{MaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	artifact, raw := testCandidateBytes(t)
	firstReq := testPublishInput(artifact, raw)
	first, err := adapter.PublishCandidateGeneration(context.Background(), firstReq)
	if err != nil {
		t.Fatal(err)
	}
	same, err := adapter.PublishCandidateGeneration(context.Background(), firstReq)
	if err != nil || same.Generation != first.Generation || same.ManifestDigest != first.ManifestDigest {
		t.Fatalf("ASSERT_ADR0007_CANDIDATE_ALIAS_EXACT_CANONICAL_BYTES_IDEMPOTENT: first=%+v same=%+v err=%v", first, same, err)
	}
	secondReq := firstReq
	secondReq.QualificationID = firstReq.QualificationID + "/ordinary-alternate"
	secondReq.SourceRevision = firstReq.SourceRevision + "-ordinary-alternate"
	second, err := adapter.PublishCandidateGeneration(context.Background(), secondReq)
	if err == nil {
		t.Fatalf("ASSERT_ADR0007_CANDIDATE_ALIAS_SAME_CANDIDATE_DIFFERENT_QUALIFICATION_ORDINARY_PREDECESSOR_COLLIDES: first=%+v second=%+v", first, second)
	}
}

func TestADR0007AdvanceReturnsCommittedResultOnPostcommitCASError(t *testing.T) {
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
	original := adapter.compareAndReplaceBoundFile
	postcommitErr := errors.New("synthetic committed postrename verification failure")
	adapter.compareAndReplaceBoundFile = func(ctx context.Context, root *publication.Root, selector string, predecessor publication.BoundFilePredecessor, raw []byte, verify func([]byte) error) (*publication.CompareAndReplaceReceipt, error) {
		receipt, err := original(ctx, root, selector, predecessor, raw, verify)
		if err != nil {
			return receipt, err
		}
		receipt.Committed = true
		receipt.Outcome = publication.CASOutcomeCommittedVerificationFailed
		receipt.VerificationStatus = publication.CASOutcomeCommittedVerificationFailed
		return receipt, postcommitErr
	}
	advanced, err := adapter.AdvanceCandidateGeneration(context.Background(), AdvanceRequest{Generation: published.Generation, VerificationSelector: published.VerificationSelector, PredecessorSelector: input.PredecessorSelector})
	if err == nil || !advanced.Committed || advanced.Selector != currentSelector || advanced.Generation != published.Generation || advanced.PostcommitVerificationStatus != publication.CASOutcomeCommittedVerificationFailed {
		t.Fatalf("ASSERT_ADR0007_ADVANCE_POSTCOMMIT_CAS_ERROR_RETURNS_COMMITTED_RESULT_NO_BLIND_RETRY: advanced=%+v err=%v", advanced, err)
	}
}

func TestADR0007PublishCandidateGenerationExactAliasCollisionRequiresExactBytes(t *testing.T) {
	root, _ := testPrivateRoot(t)
	adapter, err := NewRepositoryPrivateAdapter(root, Options{MaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	artifact, raw := testCandidateBytes(t)
	firstReq := testPublishInput(artifact, raw)
	firstReq.QualificationID = "candidate-group-boundary-artifact-validation/v1-left"
	first, err := adapter.PublishCandidateGeneration(context.Background(), firstReq)
	if err != nil {
		t.Fatal(err)
	}
	secondReq := firstReq
	secondReq.QualificationID = "candidate-group-boundary-artifact-validation/v1-right"
	secondReq.SourceRevision = firstReq.SourceRevision + "-right"
	secondReq.PredecessorSelector = first.Generation + ".selector.json"
	second, err := adapter.PublishCandidateGeneration(context.Background(), secondReq)
	if err == nil {
		t.Fatalf("ASSERT_ADR0007_CANDIDATE_ALIAS_COLLISION_DIFFERENT_BYTES_FAILS_CLOSED: first=%+v second=%+v", first, second)
	}
}

func TestADR0007CandidateReceiptSeparatesSourceLineageFromPublicationPredecessorAndCeilings(t *testing.T) {
	root, _ := testPrivateRoot(t)
	adapter, err := NewRepositoryPrivateAdapter(root, Options{MaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	artifact, raw := testCandidateBytes(t)
	input := testPublishInput(artifact, raw)
	input.SourceRevision = "git:frozen-design-443c4054"
	input.PredecessorSelector = "candidate-publication/current.selector.json"
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
		t.Fatal(err)
	}
	if receipt.SourceRevision != input.SourceRevision || receipt.PredecessorSelector != input.PredecessorSelector || receipt.SourceRevision == receipt.PredecessorSelector {
		t.Fatalf("ASSERT_ADR0007_CANDIDATE_SOURCE_LINEAGE_AND_PUBLICATION_PREDECESSOR_SEPARATE: receipt=%+v input=%+v", receipt, input)
	}
	if receipt.Authority != 0 || receipt.Accepted || receipt.Completeness != CompletenessUnknown || receipt.FeatureIdentityStatus != FeatureIdentityUnresolved || receipt.Representative.Inferred {
		t.Fatalf("ASSERT_ADR0007_CANDIDATE_AUTHORITY_CEILINGS_PRESERVED: receipt=%+v", receipt)
	}
}

func TestADR0007DefaultManifestAliasMustBeBackedByVerifiedSelectorReceipt(t *testing.T) {
	root, dir := testPrivateRoot(t)
	adapter, err := NewRepositoryPrivateAdapter(root, Options{MaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	artifact, raw := testCandidateBytes(t)
	firstReq := testPublishInput(artifact, raw)
	first, err := adapter.PublishCandidateGeneration(context.Background(), firstReq)
	if err != nil {
		t.Fatal(err)
	}
	secondManifestBytes, err := root.ReadSelector(first.ManifestSelector, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	var tampered Manifest
	if err := decodeStrict(secondManifestBytes, &tampered); err != nil {
		t.Fatal(err)
	}
	tampered.QualificationReceiptDigest = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	aliasBytes, err := json.Marshal(tampered)
	if err != nil {
		t.Fatal(err)
	}
	aliasBytes = append(aliasBytes, '\n')
	aliasPath := filepath.Join(dir, filepath.FromSlash(manifestSelectorForGeneration(first.Generation)))
	if err := os.WriteFile(aliasPath, aliasBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := adapter.GetCandidateGeneration(context.Background(), first.Generation)
	if err == nil {
		t.Fatalf("ASSERT_ADR0007_DEFAULT_ALIAS_TAMPER_WITHOUT_VERIFIED_SELECTOR_RECEIPT_REJECTED: got=%+v", got.Published)
	}
}

func TestADR0007DefaultManifestAliasPortableCustodyPolicy(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*testing.T, string)
	}{
		{"symlink", func(t *testing.T, p string) {
			_ = os.Remove(p)
			if err := os.Symlink(filepath.Base(p)+".target", p); err != nil {
				t.Fatal(err)
			}
		}},
		{"directory", func(t *testing.T, p string) {
			_ = os.Remove(p)
			if err := os.MkdirAll(p, 0o700); err != nil {
				t.Fatal(err)
			}
		}},
		{"hardlink", func(t *testing.T, p string) {
			if err := os.Link(p, p+".link"); err != nil {
				t.Skipf("hardlink not supported: %v", err)
			}
		}},
		{"group-writable", func(t *testing.T, p string) {
			if err := os.Chmod(p, 0o620); err != nil {
				t.Fatal(err)
			}
		}},
		{"world-writable", func(t *testing.T, p string) {
			if err := os.Chmod(p, 0o602); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, dir := testPrivateRoot(t)
			adapter, err := NewRepositoryPrivateAdapter(root, Options{MaxBytes: 1 << 20})
			if err != nil {
				t.Fatal(err)
			}
			artifact, raw := testCandidateBytes(t)
			published, err := adapter.PublishCandidateGeneration(context.Background(), testPublishInput(artifact, raw))
			if err != nil {
				t.Fatal(err)
			}
			aliasPath := filepath.Join(dir, filepath.FromSlash(manifestSelectorForGeneration(published.Generation)))
			if tc.name == "symlink" {
				original, err := os.ReadFile(aliasPath)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(aliasPath+".target", original, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			tc.setup(t, aliasPath)
			if got, err := adapter.GetCandidateGeneration(context.Background(), published.Generation); err == nil {
				t.Fatalf("ASSERT_ADR0007_DEFAULT_ALIAS_POLICY_REJECTS_%s: got=%+v", strings.ToUpper(strings.ReplaceAll(tc.name, "-", "_")), got.Published)
			}
		})
	}
}
