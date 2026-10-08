//go:build linux || darwin

package candidatepublication

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	secondReq := firstReq
	secondReq.QualificationID = firstReq.QualificationID + "/alternate"
	secondReq.SourceRevision = firstReq.SourceRevision + "-alternate"
	second, err := adapter.PublishCandidateGeneration(context.Background(), secondReq)
	if err != nil {
		t.Fatal(err)
	}
	secondManifestBytes, err := root.ReadSelector(second.ManifestSelector, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	var tampered Manifest
	if err := decodeStrict(secondManifestBytes, &tampered); err != nil {
		t.Fatal(err)
	}
	tampered.CandidateGeneration = first.Generation
	tampered.CandidateVerificationSelector = first.VerificationSelector
	tampered.CandidateSelector = first.ArtifactSelector
	tampered.CandidateDigest = Digest(raw)
	tampered.CandidateByteLength = uint64(len(raw))
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
