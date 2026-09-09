package seedbinding

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
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
	manifest := PreparedModificationManifest{Version: p.Version, FinderSHA256: zero, SourceArchiveSHA256: zero, SourceTreeSHA256: zero, PreparedTreeSHA256: zero,
		AllowedChanges: append([]string(nil), p.AllowedChanges...), TargetPath: p.TargetPath, TargetUnchangedSHA256: zero,
		SourceCommit: p.SourceCommit, AdaptationIDs: append([]string(nil), p.RequiredAdaptations...), PolicyID: p.PolicyID, AssessmentID: p.AssessmentID, ContextID: p.ContextID}
	manifestBytes, err := CanonicalPreparedManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifestDigest := sha256.Sum256(manifestBytes)
	r := HostCustodyReceipt{Version: p.Version, Authenticated: true, Prepared: true, Repository: p.Repository, SourceRevision: p.SourceCommit, TargetPath: p.TargetPath,
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

func resignPreparedPolicy(t *testing.T, policy *HostPreparedPolicy, priv ed25519.PrivateKey) {
	t.Helper()
	unsigned := *policy
	unsigned.Signature = nil
	canonical, err := json.Marshal(unsigned)
	if err != nil {
		t.Fatal(err)
	}
	policy.Signature = ed25519.Sign(priv, HostPreparedPolicySigningBytes(canonical))
}

func setUint32Version(t *testing.T, target any, version uint32) {
	t.Helper()
	value := reflect.ValueOf(target)
	if value.Kind() != reflect.Pointer || value.Elem().Kind() != reflect.Struct {
		t.Fatal("version target must be a struct pointer")
	}
	field := value.Elem().FieldByName("Version")
	if !field.IsValid() || !field.CanSet() || field.Kind() != reflect.Uint32 {
		t.Fatal("ASSERT_PREPARED_VERSION_FIELD_REQUIRED")
	}
	field.SetUint(uint64(version))
}

func TestHostRootPolicyNumericVersionMismatchSameKeyRejected(t *testing.T) {
	policy, receipt, pub, priv := testPreparedPolicy(t)
	policy.Version = 2
	resignPreparedPolicy(t, &policy, priv)
	roots := HostRootSet{Version: 1, Keys: map[string]ed25519.PublicKey{"root": pub}}
	if _, err := roots.Verify("root", policy, receipt, time.Unix(150, 0).UTC()); err == nil {
		t.Fatal("ASSERT_HOST_ROOT_POLICY_NUMERIC_MISMATCH_SAME_KEY_REJECTED")
	}
}

func TestHostRootPolicyNumericVersionMismatchDifferentKeyRejected(t *testing.T) {
	policy, receipt, _, _ := testPreparedPolicy(t)
	pubV2, privV2, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	policy.Version = 2
	resignPreparedPolicy(t, &policy, privV2)
	receipt.PreparedManifestSignature = ed25519.Sign(privV2, PreparedManifestSigningBytes(receipt.PreparedManifest))
	roots := HostRootSet{Version: 1, Keys: map[string]ed25519.PublicKey{"root-v2": pubV2}}
	if _, err := roots.Verify("root-v2", policy, receipt, time.Unix(150, 0).UTC()); err == nil {
		t.Fatal("ASSERT_HOST_ROOT_POLICY_NUMERIC_MISMATCH_DIFFERENT_KEY_REJECTED")
	}
}

func TestHostPolicyUnsupportedNonzeroVersionRejected(t *testing.T) {
	policy, receipt, pub, priv := testPreparedPolicy(t)
	policy.Version = 99
	resignPreparedPolicy(t, &policy, priv)
	roots := HostRootSet{Version: 99, Keys: map[string]ed25519.PublicKey{"root": pub}}
	if _, err := roots.Verify("root", policy, receipt, time.Unix(150, 0).UTC()); err == nil {
		t.Fatal("ASSERT_HOST_POLICY_UNSUPPORTED_NONZERO_VERSION_REJECTED")
	}
}

func TestHostPreparedReceiptVersionMismatchRejected(t *testing.T) {
	policy, receipt, pub, _ := testPreparedPolicy(t)
	setUint32Version(t, &receipt, 2)
	roots := HostRootSet{Version: 1, Keys: map[string]ed25519.PublicKey{"root": pub}}
	if _, err := roots.Verify("root", policy, receipt, time.Unix(150, 0).UTC()); err == nil {
		t.Fatal("ASSERT_HOST_PREPARED_RECEIPT_VERSION_MISMATCH_REJECTED")
	}
}

func TestHostPreparedManifestVersionMismatchRejected(t *testing.T) {
	policy, receipt, pub, priv := testPreparedPolicy(t)
	var manifest PreparedModificationManifest
	if err := json.Unmarshal(receipt.PreparedManifest, &manifest); err != nil {
		t.Fatal(err)
	}
	setUint32Version(t, &manifest, 2)
	canonical, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(canonical)
	receipt.PreparedManifest = canonical
	receipt.PreparedManifestSHA256 = hex.EncodeToString(digest[:])
	receipt.PreparedManifestSignature = ed25519.Sign(priv, PreparedManifestSigningBytes(canonical))
	roots := HostRootSet{Version: 1, Keys: map[string]ed25519.PublicKey{"root": pub}}
	if _, err := roots.Verify("root", policy, receipt, time.Unix(150, 0).UTC()); err == nil {
		t.Fatal("ASSERT_HOST_PREPARED_MANIFEST_VERSION_MISMATCH_REJECTED")
	}
}

func TestHostPolicyDowngradeRejected(t *testing.T) {
	policy, receipt, pub, _ := testPreparedPolicy(t)
	roots := HostRootSet{Version: 2, Keys: map[string]ed25519.PublicKey{"root": pub}}
	if _, err := roots.Verify("root", policy, receipt, time.Unix(150, 0).UTC()); err == nil {
		t.Fatal("ASSERT_HOST_POLICY_DOWNGRADE_REJECTED")
	}
}

func TestHostPolicyCrossVersionReceiptReplayRejected(t *testing.T) {
	policy, receipt, pub, priv := testPreparedPolicy(t)
	policy.Version = 2
	resignPreparedPolicy(t, &policy, priv)
	roots := HostRootSet{Version: 2, Keys: map[string]ed25519.PublicKey{"root": pub}}
	if _, err := roots.Verify("root", policy, receipt, time.Unix(150, 0).UTC()); err == nil {
		t.Fatal("ASSERT_HOST_POLICY_CROSS_VERSION_RECEIPT_REPLAY_REJECTED")
	}
}

func TestHostPolicyZeroVersionRejected(t *testing.T) {
	policy, receipt, pub, _ := testPreparedPolicy(t)
	policy.Version = 0
	roots := HostRootSet{Version: HostPreparedPolicyVersionV1, Keys: map[string]ed25519.PublicKey{"root": pub}}
	if _, err := roots.Verify("root", policy, receipt, time.Unix(150, 0).UTC()); err == nil {
		t.Fatal("ASSERT_HOST_POLICY_ZERO_VERSION_REJECTED")
	}
}

func TestHostPreparedReceiptZeroVersionRejected(t *testing.T) {
	policy, receipt, pub, _ := testPreparedPolicy(t)
	receipt.Version = 0
	roots := HostRootSet{Version: HostPreparedPolicyVersionV1, Keys: map[string]ed25519.PublicKey{"root": pub}}
	if _, err := roots.Verify("root", policy, receipt, time.Unix(150, 0).UTC()); err == nil {
		t.Fatal("ASSERT_HOST_PREPARED_RECEIPT_ZERO_VERSION_REJECTED")
	}
}

func TestHostPreparedManifestZeroVersionRejected(t *testing.T) {
	_, receipt, pub, _ := testPreparedPolicy(t)
	var manifest PreparedModificationManifest
	if err := json.Unmarshal(receipt.PreparedManifest, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest.Version = 0
	canonical, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(canonical)
	receipt.PreparedManifest = canonical
	receipt.PreparedManifestSHA256 = hex.EncodeToString(digest[:])
	if err := receipt.VerifyPrepared(pub, receipt.TargetPath); err == nil {
		t.Fatal("ASSERT_HOST_PREPARED_MANIFEST_ZERO_VERSION_REJECTED")
	}
}

func TestPreparedVersionsAreInCanonicalSigningPreimages(t *testing.T) {
	policy, receipt, _, _ := testPreparedPolicy(t)
	policyV1, err := CanonicalHostPreparedPolicy(policy)
	if err != nil {
		t.Fatal(err)
	}
	unsignedPolicy := policy
	unsignedPolicy.Signature = nil
	unsignedPolicy.Version = 2
	policyV2, err := json.Marshal(unsignedPolicy)
	if err != nil {
		t.Fatal(err)
	}
	if string(HostPreparedPolicySigningBytes(policyV1)) == string(HostPreparedPolicySigningBytes(policyV2)) {
		t.Fatal("ASSERT_HOST_POLICY_VERSION_IN_SIGNING_PREIMAGE")
	}

	receiptV1, err := CustodyReceiptSigningBytes(receipt)
	if err != nil {
		t.Fatal(err)
	}
	receipt.Version = 2
	receiptV2, err := CustodyReceiptSigningBytes(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if string(receiptV1) == string(receiptV2) {
		t.Fatal("ASSERT_HOST_RECEIPT_VERSION_IN_SIGNING_PREIMAGE")
	}

	var manifest PreparedModificationManifest
	if err := json.Unmarshal(receipt.PreparedManifest, &manifest); err != nil {
		t.Fatal(err)
	}
	manifestV1 := append([]byte(nil), receipt.PreparedManifest...)
	manifest.Version = 2
	manifestV2, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if string(PreparedManifestSigningBytes(manifestV1)) == string(PreparedManifestSigningBytes(manifestV2)) {
		t.Fatal("ASSERT_HOST_PREPARED_MANIFEST_VERSION_IN_SIGNING_PREIMAGE")
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
