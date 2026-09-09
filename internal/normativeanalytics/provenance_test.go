package normativeanalytics

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
)

type syntheticAuthority struct{}

func (syntheticAuthority) VerifyProvenance(a, k, d, c, digest string, sig []byte) error {
	if a != "synthetic-test-authority" || k != "synthetic-test-key" || d != "synthetic-test-domain" || c != "synthetic-test-context" || string(sig) != "synthetic-test-signature:"+digest {
		return errors.New("synthetic authority mismatch")
	}
	return nil
}
func evidence() (RetainedEvidence, ProvenancePolicy) {
	b := []byte(`{"nodes":["a"]}`)
	s := sha256.Sum256(b)
	digest := "sha256:" + hex.EncodeToString(s[:])
	e := RetainedEvidence{Schema: "retained-graph.test.v1", Revision: "rev", CustodyRevision: "rev", Bytes: b, Digest: digest, AuthorityID: "synthetic-test-authority", KeyID: "synthetic-test-key", SignatureDomain: "synthetic-test-domain", SignatureContext: "synthetic-test-context", Signature: []byte("synthetic-test-signature:" + digest)}
	p := ProvenancePolicy{ExpectedRevision: "rev", Validators: map[string]SemanticValidator{"retained-graph.test.v1": func(raw []byte) error {
		if string(raw) != string(b) {
			return errors.New("semantic mismatch")
		}
		return nil
	}}, Authority: syntheticAuthority{}}
	return e, p
}
func TestSyntheticEphemeralAuthorityProvesMechanismOnly(t *testing.T) {
	e, p := evidence()
	if err := ValidateRetainedEvidence(e, p); err != nil {
		t.Fatal(err)
	}
}
func TestProvenanceMutationsFailClosed(t *testing.T) {
	mutations := map[string]func(*RetainedEvidence, *ProvenancePolicy){
		"bytes":  func(e *RetainedEvidence, p *ProvenancePolicy) { e.Bytes = []byte(`{"nodes":[]}`) },
		"schema": func(e *RetainedEvidence, p *ProvenancePolicy) { e.Schema = "fake" },
		"semantic": func(e *RetainedEvidence, p *ProvenancePolicy) {
			p.Validators[e.Schema] = func([]byte) error { return errors.New("fake") }
		},
		"revision":         func(e *RetainedEvidence, p *ProvenancePolicy) { e.Revision = "other" },
		"custody":          func(e *RetainedEvidence, p *ProvenancePolicy) { e.CustodyRevision = "other" },
		"policy-revision":  func(e *RetainedEvidence, p *ProvenancePolicy) { p.ExpectedRevision = "other" },
		"authority":        func(e *RetainedEvidence, p *ProvenancePolicy) { e.AuthorityID = "other" },
		"key":              func(e *RetainedEvidence, p *ProvenancePolicy) { e.KeyID = "other" },
		"domain":           func(e *RetainedEvidence, p *ProvenancePolicy) { e.SignatureDomain = "other" },
		"context":          func(e *RetainedEvidence, p *ProvenancePolicy) { e.SignatureContext = "other" },
		"digest":           func(e *RetainedEvidence, p *ProvenancePolicy) { e.Digest = "sha256:00" },
		"signature":        func(e *RetainedEvidence, p *ProvenancePolicy) { e.Signature = []byte("fake") },
		"authority-config": func(e *RetainedEvidence, p *ProvenancePolicy) { p.Authority = nil },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			e, p := evidence()
			mutate(&e, &p)
			if err := ValidateRetainedEvidence(e, p); !errors.Is(err, ErrInvalidProvenance) {
				t.Fatalf("ASSERT_PROVENANCE_MUTATION_REJECTED: %v", err)
			}
		})
	}
}
func TestUnsignedLocalEvidenceMakesNoAuthorityClaim(t *testing.T) {
	e, p := evidence()
	e.AuthorityID = ""
	e.KeyID = ""
	e.SignatureDomain = ""
	e.SignatureContext = ""
	e.Signature = nil
	p.Authority = nil
	if err := ValidateRetainedEvidence(e, p); err != nil {
		t.Fatal(err)
	}
}
