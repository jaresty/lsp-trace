package qualificationpolicy

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

type testReceiptSigner struct {
	authority *verifiedProgramAAuthority
	private   ed25519.PrivateKey
}

func newTestReceiptSigner(t *testing.T) testReceiptSigner {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keySum := sha256.Sum256(public)
	keyID := "sha256:" + hex.EncodeToString(keySum[:])
	receipt := []byte(fmt.Sprintf(`{"receipt_id":"test-receipt","authority_id":"test-host-authority","key_id":%q}`, keyID))
	receiptSum := sha256.Sum256(receipt)
	authority, err := provisionVerifiedProgramAAuthority(hostProgramATrustProvisioning{authorityID: "test-host-authority", keyID: keyID, policyID: "program-a-admission", provisioningReceipt: receipt, pinnedProvisioningReceiptDigest: "sha256:" + hex.EncodeToString(receiptSum[:]), publicKey: public})
	if err != nil {
		t.Fatal(err)
	}
	return testReceiptSigner{authority: authority, private: private}
}

func testAssessmentContext(id, operation, scope string) AssessmentContext {
	return AssessmentContext{AssessmentID: id, Nonce: "nonce-1", IssuanceEpoch: 1788825600, EvaluationScope: scope, AdmissionPolicyID: "program-a-admission", AdmissionPolicyVersion: "v1", Operation: operation}
}

func rawReceipt(d Dimension, revision, substrate string, sequence int) ReceiptBytes {
	return rawReceiptWithContext(d, revision, substrate, sequence, testAssessmentContext("assessment-1", "RANKING", "NORMATIVE_PROGRAM_A"))
}

func rawReceiptWithContext(d Dimension, revision, substrate string, sequence int, c AssessmentContext) ReceiptBytes {
	contract := evaluatorContract[d]
	return ReceiptBytes{Dimension: d, EvaluatorID: "lsp-trace/" + contract.family + "-evaluator", SchemaVersion: ReceiptSchemaVersion, Family: contract.family, Version: contract.version, CustodyRef: "custody:" + string(d), Revision: revision, SubstrateID: substrate, Sequence: sequence, Status: SubstrateAdmitted, AssessmentID: c.AssessmentID, Nonce: c.Nonce, IssuanceEpoch: c.IssuanceEpoch, EvaluationScope: c.EvaluationScope, AdmissionPolicyID: c.AdmissionPolicyID, AdmissionPolicyVersion: c.AdmissionPolicyVersion, Operation: c.Operation, EvidenceDigest: "sha256:evidence-" + string(d)}
}

func receiptWith(t *testing.T, signer testReceiptSigner, d Dimension, revision, substrate string, sequence int) VerifiedReceipt {
	return receiptWithContext(t, signer, d, revision, substrate, sequence, testAssessmentContext("assessment-1", "RANKING", "NORMATIVE_PROGRAM_A"))
}

func receiptWithContext(t *testing.T, signer testReceiptSigner, d Dimension, revision, substrate string, sequence int, c AssessmentContext) VerifiedReceipt {
	t.Helper()
	raw := rawReceiptWithContext(d, revision, substrate, sequence, c)
	raw.AuthorityID = signer.authority.authorityID
	raw.KeyID = signer.authority.keyID
	raw.ProvisioningReceiptDigest = signer.authority.provisioningReceiptDigest
	raw.Digest = receiptDigest(raw)
	raw.Signature = ed25519.Sign(signer.private, receiptPayload(raw))
	got, err := signer.authority.VerifyReceipt(raw, c)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func admittedProgramASubstrate(t *testing.T) VerifiedProgramASubstrate {
	return admittedProgramASubstrateWithSigner(t, newTestReceiptSigner(t), testAssessmentContext("assessment-1", "RANKING", "NORMATIVE_PROGRAM_A"))
}

func admittedProgramASubstrateWithSigner(t *testing.T, s testReceiptSigner, c AssessmentContext) VerifiedProgramASubstrate {
	return VerifiedProgramASubstrate{
		Custody: receiptWithContext(t, s, CustodyDimension, "526f658", "substrate-1", 0, c), EffectiveConfiguration: receiptWithContext(t, s, EffectiveConfigurationDimension, "526f658", "substrate-1", 0, c), Identity: receiptWithContext(t, s, IdentityDimension, "526f658", "substrate-1", 0, c),
		RelationNormalization: receiptWithContext(t, s, RelationNormalizationDimension, "526f658", "substrate-1", 1, c), SupportAccounting: receiptWithContext(t, s, SupportAccountingDimension, "526f658", "substrate-1", 2, c), Projection: receiptWithContext(t, s, ProjectionDimension, "526f658", "substrate-1", 3, c), Qualification: receiptWithContext(t, s, QualificationDimension, "526f658", "substrate-1", 4, c),
	}
}

func TestProgramAAdmissionRequiresValidatedReceipts(t *testing.T) {
	got, err := AdmitVerifiedProgramA(admittedProgramASubstrate(t))
	if err != nil || got.Status != SubstrateAdmitted {
		t.Fatalf("ASSERT_PROGRAM_A_COMPLETE_SUBSTRATE_ADMITTED: got=%#v err=%v", got, err)
	}
	zero := admittedProgramASubstrate(t)
	zero.Custody = VerifiedReceipt{}
	got, err = AdmitVerifiedProgramA(zero)
	if err != nil || !reflect.DeepEqual(got.Reasons, []string{"custody: no validated receipt"}) {
		t.Fatalf("ASSERT_PROGRAM_A_NO_RECEIPT_REJECTED: %#v %v", got, err)
	}
}

func TestProgramATrustProvisioningAndCanonicalSignature(t *testing.T) {
	if _, err := provisionVerifiedProgramAAuthority(hostProgramATrustProvisioning{}); err == nil {
		t.Fatal("ASSERT_PROGRAM_A_UNPROVISIONED_AUTHORITY_REJECTED")
	}
	s := newTestReceiptSigner(t)
	base := rawReceipt(CustodyDimension, "526f658", "substrate-1", 0)
	base.Digest = receiptDigest(base)
	base.Signature = ed25519.Sign(s.private, receiptPayload(base))
	base.AuthorityID, base.KeyID, base.ProvisioningReceiptDigest = s.authority.authorityID, s.authority.keyID, s.authority.provisioningReceiptDigest
	base.Digest = receiptDigest(base)
	base.Signature = ed25519.Sign(s.private, receiptPayload(base))
	if _, err := s.authority.VerifyReceipt(base, testAssessmentContext("assessment-1", "RANKING", "NORMATIVE_PROGRAM_A")); err != nil {
		t.Fatalf("ASSERT_PROGRAM_A_PROVISIONED_AUTHORITY_ACCEPTS: %v", err)
	}
	other := newTestReceiptSigner(t)
	if _, err := other.authority.VerifyReceipt(base, testAssessmentContext("assessment-1", "RANKING", "NORMATIVE_PROGRAM_A")); err == nil || !strings.Contains(err.Error(), "authority provisioning mismatch") {
		t.Fatalf("ASSERT_PROGRAM_A_WRONG_TRUST_AUTHORITY_REJECTED: %v", err)
	}
	if !strings.HasPrefix(string(receiptPayload(base)), receiptSignatureDomain) {
		t.Fatal("ASSERT_PROGRAM_A_SIGNATURE_DOMAIN_BOUND")
	}
	for name, mutate := range map[string]func(*ReceiptBytes){
		"evaluator": func(r *ReceiptBytes) { r.EvaluatorID = "caller" }, "family": func(r *ReceiptBytes) { r.Family = "other" }, "version": func(r *ReceiptBytes) { r.Version = "v2" },
		"digest": func(r *ReceiptBytes) { r.Digest = "sha256:arbitrary" }, "signature": func(r *ReceiptBytes) { r.Signature = append([]byte(nil), r.Signature...); r.Signature[0] ^= 0xff },
		"custody": func(r *ReceiptBytes) { r.CustodyRef = ""; r.Digest = receiptDigest(*r) }, "nul": func(r *ReceiptBytes) {
			r.Revision = "a\x00b"
			r.Digest = receiptDigest(*r)
			r.Signature = ed25519.Sign(s.private, receiptPayload(*r))
		},
	} {
		t.Run(name, func(t *testing.T) {
			raw := base
			mutate(&raw)
			if _, err := s.authority.VerifyReceipt(raw, testAssessmentContext("assessment-1", "RANKING", "NORMATIVE_PROGRAM_A")); err == nil {
				t.Fatalf("ASSERT_PROGRAM_A_MUTATION_%s_REJECTED", name)
			}
		})
	}
	a := rawReceipt(CustodyDimension, "a", "b\x00c", 0)
	b := rawReceipt(CustodyDimension, "a\x00b", "c", 0)
	if string(receiptPayload(a)) == string(receiptPayload(b)) {
		t.Fatal("ASSERT_PROGRAM_A_FIELD_BOUNDARY_UNAMBIGUOUS")
	}
}

func TestProgramAAdmissionRejectsMixedAuthorityAndOrder(t *testing.T) {
	input := admittedProgramASubstrate(t)
	s := newTestReceiptSigner(t)
	input.Identity = receiptWith(t, s, IdentityDimension, "other", "substrate-1", 0)
	got, _ := AdmitVerifiedProgramA(input)
	if !containsReason(got.Reasons, "identity: revision mismatch") {
		t.Fatalf("ASSERT_PROGRAM_A_CROSS_REVISION_REJECTED: %#v", got)
	}
	input = admittedProgramASubstrate(t)
	s = newTestReceiptSigner(t)
	input.Projection = receiptWith(t, s, ProjectionDimension, "526f658", "other", 3)
	got, _ = AdmitVerifiedProgramA(input)
	if !containsReason(got.Reasons, "projection: substrate mismatch") {
		t.Fatalf("ASSERT_PROGRAM_A_CROSS_SUBSTRATE_REJECTED: %#v", got)
	}
	s = newTestReceiptSigner(t)
	input = admittedProgramASubstrateWithSigner(t, s, testAssessmentContext("assessment-1", "RANKING", "NORMATIVE_PROGRAM_A"))
	input.RelationNormalization = receiptWith(t, s, RelationNormalizationDimension, "526f658", "substrate-1", 2)
	input.SupportAccounting = receiptWith(t, s, SupportAccountingDimension, "526f658", "substrate-1", 1)
	got, _ = AdmitVerifiedProgramA(input)
	if !reflect.DeepEqual(got.Reasons, []string{"pipeline: relation_normalization must precede support_accounting"}) {
		t.Fatalf("ASSERT_PROGRAM_A_PIPELINE_ORDER_REJECTED: %#v", got)
	}
}

func containsReason(reasons []string, want string) bool {
	for _, reason := range reasons {
		if reason == want {
			return true
		}
	}
	return false
}

func TestProgramAAuthorityAndAssessmentReplayMutations(t *testing.T) {
	a := newTestReceiptSigner(t)
	b := newTestReceiptSigner(t)
	input := admittedProgramASubstrateWithSigner(t, a, testAssessmentContext("assessment-1", "RANKING", "NORMATIVE_PROGRAM_A"))
	input.Identity = receiptWithContext(t, b, IdentityDimension, "526f658", "substrate-1", 0, testAssessmentContext("assessment-1", "RANKING", "NORMATIVE_PROGRAM_A"))
	got, _ := AdmitVerifiedProgramA(input)
	if got.Status != SubstrateRejected || !containsReason(got.Reasons, "identity: authority mismatch") {
		t.Fatalf("ASSERT_PROGRAM_A_MIXED_AUTHORITY_REJECTED: %#v", got)
	}

	baseContext := testAssessmentContext("assessment-1", "RANKING", "NORMATIVE_PROGRAM_A")
	raw := rawReceiptWithContext(CustodyDimension, "526f658", "substrate-1", 0, baseContext)
	raw.AuthorityID, raw.KeyID, raw.ProvisioningReceiptDigest = a.authority.authorityID, a.authority.keyID, a.authority.provisioningReceiptDigest
	raw.Digest = receiptDigest(raw)
	raw.Signature = ed25519.Sign(a.private, receiptPayload(raw))
	if _, err := a.authority.VerifyReceipt(raw, baseContext); err != nil {
		t.Fatalf("ASSERT_PROGRAM_A_SAME_CONTEXT_REVERIFIABLE: first verify: %v", err)
	}
	if _, err := a.authority.VerifyReceipt(raw, baseContext); err != nil {
		t.Fatalf("ASSERT_PROGRAM_A_SAME_CONTEXT_REVERIFIABLE: second verify: %v", err)
	}
	for name, changed := range map[string]AssessmentContext{
		"assessment": testAssessmentContext("assessment-2", "RANKING", "NORMATIVE_PROGRAM_A"),
		"operation":  testAssessmentContext("assessment-1", "ANALYSIS", "NORMATIVE_PROGRAM_A"),
		"scope":      testAssessmentContext("assessment-1", "RANKING", "OTHER_SCOPE"),
		"policy":     func() AssessmentContext { c := baseContext; c.AdmissionPolicyVersion = "v2"; return c }(),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := a.authority.VerifyReceipt(raw, changed); err == nil {
				t.Fatalf("ASSERT_PROGRAM_A_CONTEXT_MUTATION_REJECTED_%s", name)
			}
		})
	}

	for name, changed := range map[string]AssessmentContext{
		"mixed_assessment": testAssessmentContext("assessment-2", "RANKING", "NORMATIVE_PROGRAM_A"),
		"mixed_scope":      testAssessmentContext("assessment-1", "RANKING", "OTHER_SCOPE"),
	} {
		t.Run(name, func(t *testing.T) {
			mixed := admittedProgramASubstrateWithSigner(t, a, baseContext)
			mixed.Projection = receiptWithContext(t, a, ProjectionDimension, "526f658", "substrate-1", 3, changed)
			got, _ := AdmitVerifiedProgramA(mixed)
			if got.Status != SubstrateRejected {
				t.Fatalf("ASSERT_PROGRAM_A_MIXED_ASSESSMENT_SCOPE_REJECTED_%s: %#v", name, got)
			}
		})
	}

	substituted := raw
	substituted.EvidenceDigest = "sha256:substituted-evidence"
	if _, err := a.authority.VerifyReceipt(substituted, baseContext); err == nil {
		t.Fatal("ASSERT_PROGRAM_A_RECEIPT_SUBSTITUTION_REJECTED")
	}
}

func TestProgramAAdmissionReasonsAreDeterministic(t *testing.T) {
	input := admittedProgramASubstrate(t)
	input.Custody = VerifiedReceipt{}
	input.Projection = VerifiedReceipt{}
	first, _ := AdmitVerifiedProgramA(input)
	second, _ := AdmitVerifiedProgramA(input)
	if !reflect.DeepEqual(first, second) || first.Status != SubstrateRejected {
		t.Fatalf("ASSERT_PROGRAM_A_REASONS_COMPLETE_DETERMINISTIC: %#v %#v", first, second)
	}
}
