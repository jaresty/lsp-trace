package captureset

import (
	"errors"
	"fmt"

	"lsp-trace/internal/publication"
)

// ExactConstituent is one exact V5 byte string resolved from a verified
// capture-set. Its selector is checked independently of its position.
type ExactConstituent struct {
	ImmutableSelector string
	Bytes             []byte
}

// VerifiedPublicationEvidence is a canonical, cloned, transport-neutral
// binding of a verified private publication receipt to its exact bundle input.
// It establishes byte consistency only; it does not attest to a producer.
type VerifiedPublicationEvidence struct {
	Receipt      PublicationReceipt
	Manifest     Manifest
	Constituents []ExactConstituent
}

// VerifyPublicationEvidence reconstructs the canonical private bundle from
// exact admitted V5 constituents and requires its aggregate identity to equal
// the committed verified receipt. It performs no filesystem access.
func VerifyPublicationEvidence(receipt PublicationReceipt, manifest Manifest, exact []ExactConstituent) (VerifiedPublicationEvidence, error) {
	if receipt.Disclosure != "PRIVATE" || receipt.VerificationStatus != "VERIFIED" || receipt.Mechanism != publication.BoundFileMechanism || !receipt.NamespaceAtomic {
		return VerifiedPublicationEvidence{}, errors.New("verified private bound-file receipt required")
	}
	if err := ValidatePublicationSelector(receipt.Selector); err != nil || receipt.Selector != CaptureSetPublicationSelector(manifest) {
		if err != nil {
			return VerifiedPublicationEvidence{}, err
		}
		return VerifiedPublicationEvidence{}, errors.New("receipt selector does not bind manifest")
	}
	if receipt.ConstituentCount != len(manifest.Constituents) || len(exact) != len(manifest.Constituents) {
		return VerifiedPublicationEvidence{}, errors.New("receipt constituent count mismatch")
	}
	manifestRaw, err := EncodeCanonical(manifest)
	if err != nil {
		return VerifiedPublicationEvidence{}, err
	}
	bySelector := make(map[string][]byte, len(exact))
	for _, item := range exact {
		if item.ImmutableSelector == "" {
			return VerifiedPublicationEvidence{}, errors.New("empty exact constituent selector")
		}
		if _, duplicate := bySelector[item.ImmutableSelector]; duplicate {
			return VerifiedPublicationEvidence{}, errors.New("duplicate exact constituent selector")
		}
		bySelector[item.ImmutableSelector] = append([]byte(nil), item.Bytes...)
	}
	canonical := make([]ExactConstituent, len(manifest.Constituents))
	authority := NativeV5Authority()
	for i, constituent := range manifest.Constituents {
		raw, ok := bySelector[constituent.ImmutableSelector]
		if !ok {
			return VerifiedPublicationEvidence{}, errors.New("missing exact manifest constituent")
		}
		if err := authority.VerifyConstituent(constituent, raw); err != nil {
			return VerifiedPublicationEvidence{}, fmt.Errorf("exact manifest constituent: %w", err)
		}
		canonical[i] = ExactConstituent{ImmutableSelector: constituent.ImmutableSelector, Bytes: append([]byte(nil), raw...)}
	}
	bundle, err := encodePrivateBundle(manifest, manifestRaw, bySelector)
	if err != nil {
		return VerifiedPublicationEvidence{}, err
	}
	if receipt.ArtifactSHA256 != rawDigest(bundle) || receipt.ByteLength != uint64(len(bundle)) {
		return VerifiedPublicationEvidence{}, errors.New("receipt aggregate digest or byte length mismatch")
	}
	return VerifiedPublicationEvidence{Receipt: receipt, Manifest: cloneManifest(manifest), Constituents: canonical}, nil
}
