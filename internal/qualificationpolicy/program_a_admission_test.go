package qualificationpolicy

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"lsp-trace/internal/schema"
	"reflect"
	"strings"
	"testing"
)

type testReceiptSigner struct {
	authority *ProgramAReceiptAuthority
	private   ed25519.PrivateKey
}

func newTestReceiptSigner(t *testing.T) testReceiptSigner {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(public)
	policy := "sha256:" + hex.EncodeToString(sum[:])
	snapshot := "test-host-snapshot"
	receipt := []byte(fmt.Sprintf(`{"trust_provisioning_receipt_schema_version":"lsp-trace.trust-provisioning-receipt.v1","receipt_id":"test-receipt","trust_policy_id":%q,"anchor_type":"ED25519_PUBLIC_KEY","anchor_identity":%q,"source_snapshot_identity":%q,"provisioning_authority":"test-security-operations","provisioning_channel":"VERIFIER_TRUST_STORE","provisioning_event":"test-runtime","verification_method":"ED25519_SIGNATURE","verification_result":"VERIFIED"}`, policy, policy, snapshot))
	context := schema.TrustAuthenticationContext{ReceiptID: "test-receipt", TrustPolicyID: policy, AnchorType: "ED25519_PUBLIC_KEY", AnchorIdentity: policy, SourceSnapshotIdentity: snapshot, ClaimantID: "test-claimant", ProducerID: "test-evidence-producer", ProvisionedReceiptIDs: map[string]struct{}{"test-receipt": {}}}
	store, err := schema.NewHostTrustStore([]schema.HostTrustGrant{{Receipt: receipt, Context: context}})
	if err != nil {
		t.Fatal(err)
	}
	authority, err := NewProgramAReceiptAuthority(ProgramATrustProvisioning{Store: store, Request: schema.HostTrustRequest{Receipt: receipt, ClaimedSourceSnapshotIdentity: snapshot}, AuthorityID: snapshot, PolicyID: policy, PublicKey: public})
	if err != nil {
		t.Fatal(err)
	}
	return testReceiptSigner{authority: authority, private: private}
}

func rawReceipt(d Dimension, revision, substrate string, sequence int) ReceiptBytes {
	contract := evaluatorContract[d]
	return ReceiptBytes{Dimension: d, EvaluatorID: "lsp-trace/" + contract.family + "-evaluator", SchemaVersion: ReceiptSchemaVersion, Family: contract.family, Version: contract.version, CustodyRef: "custody:" + string(d), Revision: revision, SubstrateID: substrate, Sequence: sequence, Status: SubstrateAdmitted}
}

func receiptWith(t *testing.T, signer testReceiptSigner, d Dimension, revision, substrate string, sequence int) VerifiedReceipt {
	t.Helper()
	raw := rawReceipt(d, revision, substrate, sequence)
	raw.Digest = receiptDigest(raw)
	raw.Signature = ed25519.Sign(signer.private, receiptPayload(raw))
	got, err := signer.authority.VerifyReceipt(raw)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func admittedProgramASubstrate(t *testing.T) VerifiedProgramASubstrate {
	s := newTestReceiptSigner(t)
	return VerifiedProgramASubstrate{
		Custody: receiptWith(t, s, CustodyDimension, "526f658", "substrate-1", 0), EffectiveConfiguration: receiptWith(t, s, EffectiveConfigurationDimension, "526f658", "substrate-1", 0), Identity: receiptWith(t, s, IdentityDimension, "526f658", "substrate-1", 0),
		RelationNormalization: receiptWith(t, s, RelationNormalizationDimension, "526f658", "substrate-1", 1), SupportAccounting: receiptWith(t, s, SupportAccountingDimension, "526f658", "substrate-1", 2), Projection: receiptWith(t, s, ProjectionDimension, "526f658", "substrate-1", 3),
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
	if _, err := NewProgramAReceiptAuthority(ProgramATrustProvisioning{}); err == nil {
		t.Fatal("ASSERT_PROGRAM_A_UNPROVISIONED_AUTHORITY_REJECTED")
	}
	s := newTestReceiptSigner(t)
	base := rawReceipt(CustodyDimension, "526f658", "substrate-1", 0)
	base.Digest = receiptDigest(base)
	base.Signature = ed25519.Sign(s.private, receiptPayload(base))
	if _, err := s.authority.VerifyReceipt(base); err != nil {
		t.Fatalf("ASSERT_PROGRAM_A_PROVISIONED_AUTHORITY_ACCEPTS: %v", err)
	}
	other := newTestReceiptSigner(t)
	if _, err := other.authority.VerifyReceipt(base); err == nil || !strings.Contains(err.Error(), "signature mismatch") {
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
			if _, err := s.authority.VerifyReceipt(raw); err == nil {
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
	if !reflect.DeepEqual(got.Reasons, []string{"identity: revision mismatch"}) {
		t.Fatalf("ASSERT_PROGRAM_A_CROSS_REVISION_REJECTED: %#v", got)
	}
	input = admittedProgramASubstrate(t)
	s = newTestReceiptSigner(t)
	input.Projection = receiptWith(t, s, ProjectionDimension, "526f658", "other", 3)
	got, _ = AdmitVerifiedProgramA(input)
	if !reflect.DeepEqual(got.Reasons, []string{"projection: substrate mismatch"}) {
		t.Fatalf("ASSERT_PROGRAM_A_CROSS_SUBSTRATE_REJECTED: %#v", got)
	}
	input = admittedProgramASubstrate(t)
	s = newTestReceiptSigner(t)
	input.RelationNormalization = receiptWith(t, s, RelationNormalizationDimension, "526f658", "substrate-1", 2)
	input.SupportAccounting = receiptWith(t, s, SupportAccountingDimension, "526f658", "substrate-1", 1)
	got, _ = AdmitVerifiedProgramA(input)
	if !reflect.DeepEqual(got.Reasons, []string{"pipeline: relation_normalization must precede support_accounting"}) {
		t.Fatalf("ASSERT_PROGRAM_A_PIPELINE_ORDER_REJECTED: %#v", got)
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
