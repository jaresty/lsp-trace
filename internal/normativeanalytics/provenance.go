package normativeanalytics

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

var ErrInvalidProvenance = errors.New("normative analytics v2: invalid retained-evidence provenance")

type SemanticValidator func([]byte) error

type AuthorityVerifier interface {
	VerifyProvenance(authorityID, keyID, domain, context, digest string, signature []byte) error
}

type RetainedEvidence struct {
	Schema, Revision, CustodyRevision                     string
	Bytes                                                 []byte
	Digest                                                string
	AuthorityID, KeyID, SignatureDomain, SignatureContext string
	Signature                                             []byte
}

type ProvenancePolicy struct {
	ExpectedRevision string
	Validators       map[string]SemanticValidator
	Authority        AuthorityVerifier // nil means no authority claim is made or accepted.
}

// ValidateRetainedEvidence validates provenance claims independently of execution.
// Public analytics can execute without calling it; callers that publish provenance
// must supply policy-owned schema validators and, when claimed, authority config.
func ValidateRetainedEvidence(e RetainedEvidence, p ProvenancePolicy) error {
	validator, ok := p.Validators[e.Schema]
	if !ok || validator == nil || p.ExpectedRevision == "" || e.Revision != p.ExpectedRevision || e.CustodyRevision != p.ExpectedRevision {
		return ErrInvalidProvenance
	}
	sum := sha256.Sum256(e.Bytes)
	if e.Digest != "sha256:"+hex.EncodeToString(sum[:]) || validator(e.Bytes) != nil {
		return ErrInvalidProvenance
	}
	claimsAuthority := e.AuthorityID != "" || e.KeyID != "" || e.SignatureDomain != "" || e.SignatureContext != "" || len(e.Signature) != 0
	if !claimsAuthority {
		return nil
	}
	if p.Authority == nil || e.AuthorityID == "" || e.KeyID == "" || e.SignatureDomain == "" || e.SignatureContext == "" || len(e.Signature) == 0 {
		return ErrInvalidProvenance
	}
	if err := p.Authority.VerifyProvenance(e.AuthorityID, e.KeyID, e.SignatureDomain, e.SignatureContext, e.Digest, e.Signature); err != nil {
		return ErrInvalidProvenance
	}
	return nil
}
