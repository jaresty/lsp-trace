package seedbinding

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"
)

func testPreparedPolicy(t *testing.T) (HostPreparedPolicy, HostCustodyReceipt, ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	zero := hex.EncodeToString(make([]byte, sha256.Size))
	p := HostPreparedPolicy{Version: 1, PolicyID: "policy-1", AssessmentID: "assessment-1", ContextID: "context-1", Prepared: true,
		Repository: "repo", SourceCommit: "commit", RequiredAdaptations: []string{"adapter-v1"}, AllowedChanges: []string{"adapter/config.json"},
		TargetPath: "seed.go", TargetUnchangedSHA256: zero, ValidFrom: time.Unix(100, 0).UTC(), ValidUntil: time.Unix(200, 0).UTC()}
	canonical, err := CanonicalHostPreparedPolicy(p)
	if err != nil {
		t.Fatal(err)
	}
	p.Signature = ed25519.Sign(priv, HostPreparedPolicySigningBytes(canonical))
	manifest := PreparedModificationManifest{FinderSHA256: zero, SourceArchiveSHA256: zero, SourceTreeSHA256: zero, PreparedTreeSHA256: zero,
		AllowedChanges: append([]string(nil), p.AllowedChanges...), TargetPath: p.TargetPath, TargetUnchangedSHA256: zero,
		SourceCommit: p.SourceCommit, AdaptationIDs: append([]string(nil), p.RequiredAdaptations...), PolicyID: p.PolicyID, AssessmentID: p.AssessmentID, ContextID: p.ContextID}
	manifestBytes, err := CanonicalPreparedManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifestDigest := sha256.Sum256(manifestBytes)
	r := HostCustodyReceipt{Authenticated: true, Prepared: true, Repository: p.Repository, SourceRevision: p.SourceCommit, TargetPath: p.TargetPath,
		TargetSourceSHA256: zero, SeedManifestSHA256: zero, PreparedAllowedChanges: append([]string(nil), p.AllowedChanges...), PreparedManifest: manifestBytes,
		PreparedManifestSHA256: hex.EncodeToString(manifestDigest[:]), PreparedManifestSignature: ed25519.Sign(priv, PreparedManifestSigningBytes(manifestBytes)),
		PolicyID: p.PolicyID, AssessmentID: p.AssessmentID, ContextID: p.ContextID}
	return p, r, pub, priv
}

func TestHostPreparedPolicyExactBindingMutationMatrix(t *testing.T) {
	policy, receipt, pub, priv := testPreparedPolicy(t)
	ctx, err := VerifyPrepared(policy, receipt, pub, time.Unix(150, 0).UTC())
	if err != nil {
		t.Fatalf("ASSERT_HOST_POLICY_EXACT_ACCEPTED: %v", err)
	}
	claim := CustodyClaim{Repository: receipt.Repository, SourceRevision: receipt.SourceRevision, TargetPath: receipt.TargetPath, TargetSourceSHA256: receipt.TargetSourceSHA256, SeedManifestSHA256: receipt.SeedManifestSHA256}
	if err := (HostReceiptAuthority{Receipt: receipt, PreparedContext: ctx}).Verify(t.Context(), claim); err != nil {
		t.Fatalf("ASSERT_VERIFIED_CONTEXT_CARRIED_OPAQUE: %v", err)
	}

	mutations := map[string]func(*HostPreparedPolicy, *HostCustodyReceipt){
		"prepared_false":  func(p *HostPreparedPolicy, _ *HostCustodyReceipt) { p.Prepared = false },
		"source":          func(_ *HostPreparedPolicy, r *HostCustodyReceipt) { r.SourceRevision = "other" },
		"allowed_changes": func(_ *HostPreparedPolicy, r *HostCustodyReceipt) { r.PreparedAllowedChanges = []string{"other"} },
		"policy":          func(_ *HostPreparedPolicy, r *HostCustodyReceipt) { r.PolicyID = "other" },
		"assessment":      func(_ *HostPreparedPolicy, r *HostCustodyReceipt) { r.AssessmentID = "other" },
		"context":         func(_ *HostPreparedPolicy, r *HostCustodyReceipt) { r.ContextID = "other" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			p, r := policy, receipt
			mutate(&p, &r)
			if _, err := VerifyPrepared(p, r, pub, time.Unix(150, 0).UTC()); err == nil {
				t.Fatalf("ASSERT_HOST_POLICY_%s_SUBSTITUTION_REJECTED", name)
			}
		})
	}
	if _, err := VerifyPrepared(policy, receipt, pub, time.Unix(250, 0).UTC()); err == nil {
		t.Fatal("ASSERT_HOST_POLICY_EXPIRED_CONTEXT_REPLAY_REJECTED")
	}
	var substituted PreparedModificationManifest
	if err := json.Unmarshal(receipt.PreparedManifest, &substituted); err != nil {
		t.Fatal(err)
	}
	substituted.AdaptationIDs = []string{"attacker-adapter"}
	canonical, err := CanonicalPreparedManifest(substituted)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(canonical)
	receipt.PreparedManifest = canonical
	receipt.PreparedManifestSHA256 = hex.EncodeToString(digest[:])
	receipt.PreparedManifestSignature = ed25519.Sign(priv, PreparedManifestSigningBytes(canonical))
	if _, err := VerifyPrepared(policy, receipt, pub, time.Unix(150, 0).UTC()); err == nil {
		t.Fatal("ASSERT_HOST_POLICY_ADAPTATION_SUBSTITUTION_REJECTED")
	}
}

func TestHostPreparedPolicyRejectsSelfSignedAndSupportsVersionedRoots(t *testing.T) {
	policy, receipt, pub, _ := testPreparedPolicy(t)
	_, attacker, _ := ed25519.GenerateKey(rand.Reader)
	canonical, _ := CanonicalHostPreparedPolicy(policy)
	policy.Signature = ed25519.Sign(attacker, HostPreparedPolicySigningBytes(canonical))
	roots := HostRootSet{Version: 1, Keys: map[string]ed25519.PublicKey{"root-v1": pub}}
	if _, err := roots.Verify("root-v1", policy, receipt, time.Unix(150, 0).UTC()); err == nil {
		t.Fatal("ASSERT_ARBITRARY_SELF_SIGNED_BUNDLE_REJECTED")
	}
	policy, receipt, pub, _ = testPreparedPolicy(t)
	roots.Keys["root-v2"] = pub
	if _, err := roots.Verify("root-v2", policy, receipt, time.Unix(150, 0).UTC()); err != nil {
		t.Fatalf("ASSERT_VERSIONED_ROOT_ROTATION_ACCEPTED: %v", err)
	}
	if _, err := roots.Verify("root-v3", policy, receipt, time.Unix(150, 0).UTC()); err == nil {
		t.Fatal("ASSERT_UNKNOWN_ROOT_VERSION_REJECTED")
	}
}
