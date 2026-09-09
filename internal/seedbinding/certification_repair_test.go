package seedbinding

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestPreparedCustodyManifestSignatureAndMutationBinding(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	manifest := PreparedModificationManifest{
		FinderSHA256:        hex.EncodeToString(make([]byte, 32)),
		SourceArchiveSHA256: hex.EncodeToString(make([]byte, 32)),
		SourceTreeSHA256:    hex.EncodeToString(make([]byte, 32)),
		PreparedTreeSHA256:  hex.EncodeToString(make([]byte, 32)),
		AllowedChanges:      []string{"adapter/config.json"},
		TargetPath:          "seed.go", TargetUnchangedSHA256: hex.EncodeToString(make([]byte, 32)),
		SourceCommit: "source-commit", Adaptations: "prepared-adaptations",
	}
	canonical, err := CanonicalPreparedManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(canonical)
	receipt := HostCustodyReceipt{Authenticated: true, Prepared: true, TargetSourceSHA256: hex.EncodeToString(make([]byte, 32)), PreparedManifest: canonical,
		PreparedManifestSHA256: hex.EncodeToString(digest[:]), PreparedManifestSignature: ed25519.Sign(priv, PreparedManifestSigningBytes(canonical))}
	if err := receipt.VerifyPrepared(pub, "seed.go"); err != nil {
		t.Fatalf("ASSERT_PREPARED_MANIFEST_SIGNED_EXACT_ACCEPTED: %v", err)
	}

	mutated := append([]byte(nil), canonical...)
	mutated[len(mutated)-2] ^= 1
	receipt.PreparedManifest = mutated
	if err := receipt.VerifyPrepared(pub, "seed.go"); err == nil {
		t.Fatal("ASSERT_PREPARED_MANIFEST_SWAP_REJECTED")
	}

	manifest.AllowedChanges = []string{"seed.go"}
	targetCanonical, err := CanonicalPreparedManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	targetDigest := sha256.Sum256(targetCanonical)
	receipt.PreparedManifest, receipt.PreparedManifestSHA256 = targetCanonical, hex.EncodeToString(targetDigest[:])
	receipt.PreparedManifestSignature = ed25519.Sign(priv, PreparedManifestSigningBytes(targetCanonical))
	if err := receipt.VerifyPrepared(pub, "seed.go"); err == nil {
		t.Fatal("ASSERT_PREPARED_ALLOWED_CHANGE_TARGET_REJECTED")
	}

	receipt.PreparedManifest, receipt.PreparedManifestSHA256 = canonical, hex.EncodeToString(digest[:])
	otherPub, otherPriv, _ := ed25519.GenerateKey(rand.Reader)
	_ = otherPub
	receipt.PreparedManifestSignature = ed25519.Sign(otherPriv, PreparedManifestSigningBytes(canonical))
	if err := receipt.VerifyPrepared(pub, "seed.go"); err == nil {
		t.Fatal("ASSERT_SELF_MINTED_PREPARED_KEY_REJECTED")
	}
}

func TestProviderIdentityExactMutationMatrix(t *testing.T) {
	want := ProviderIdentity{Class: "MANAGED_LSP", Authority: "host", Name: "fake-lsp-fixture", Version: "1", ExecutableSHA256: "sha256:exe", PayloadSHA256: "sha256:payload", ConfigSHA256: "sha256:config"}
	if err := VerifyProviderIdentity(want, want); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*ProviderIdentity){
		"class":      func(p *ProviderIdentity) { p.Class = "OTHER" },
		"version":    func(p *ProviderIdentity) { p.Version = "2" },
		"executable": func(p *ProviderIdentity) { p.ExecutableSHA256 = "sha256:other" },
		"payload":    func(p *ProviderIdentity) { p.PayloadSHA256 = "sha256:other" },
		"config":     func(p *ProviderIdentity) { p.ConfigSHA256 = "sha256:other" },
		"authority":  func(p *ProviderIdentity) { p.Authority = "other" },
	} {
		t.Run(name, func(t *testing.T) {
			got := want
			mutate(&got)
			if VerifyProviderIdentity(want, got) == nil {
				t.Fatalf("ASSERT_PROVIDER_%s_SUBSTITUTION_REJECTED", name)
			}
		})
	}
}
