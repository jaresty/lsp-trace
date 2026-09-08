package qualificationpolicy

import (
	"crypto/ed25519"
	"reflect"
	"strings"
	"testing"
)

func receipt(t *testing.T, d Dimension, revision, substrate string, sequence int) VerifiedReceipt {
	t.Helper()
	contract := evaluatorContract[d]
	raw := ReceiptBytes{Dimension: d, EvaluatorID: "lsp-trace/" + contract.family + "-evaluator", SchemaVersion: ReceiptSchemaVersion, Family: contract.family, Version: contract.version, CustodyRef: "custody:" + string(d), Revision: revision, SubstrateID: substrate, Sequence: sequence, Status: SubstrateAdmitted}
	raw.Digest = receiptDigest(raw)
	seed := []byte{0x9d, 0x61, 0xb1, 0x9d, 0xef, 0xfd, 0x5a, 0x60, 0xba, 0x84, 0x4a, 0xf4, 0x92, 0xec, 0x2c, 0xc4, 0x44, 0x49, 0xc5, 0x69, 0x7b, 0x32, 0x69, 0x19, 0x70, 0x3b, 0xac, 0x03, 0x1c, 0xae, 0x7f, 0x60}
	raw.Signature = ed25519.Sign(ed25519.NewKeyFromSeed(seed), receiptPayload(raw))
	got, err := VerifyReceipt(raw)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func admittedProgramASubstrate(t *testing.T) VerifiedProgramASubstrate {
	return VerifiedProgramASubstrate{
		Custody: receipt(t, CustodyDimension, "526f658", "substrate-1", 0), EffectiveConfiguration: receipt(t, EffectiveConfigurationDimension, "526f658", "substrate-1", 0), Identity: receipt(t, IdentityDimension, "526f658", "substrate-1", 0),
		RelationNormalization: receipt(t, RelationNormalizationDimension, "526f658", "substrate-1", 1), SupportAccounting: receipt(t, SupportAccountingDimension, "526f658", "substrate-1", 2), Projection: receipt(t, ProjectionDimension, "526f658", "substrate-1", 3),
	}
}

func TestProgramAAdmissionRequiresValidatedReceipts(t *testing.T) {
	got, err := AdmitVerifiedProgramA(admittedProgramASubstrate(t))
	if err != nil || got.Status != SubstrateAdmitted || got.Revision != "526f658" || got.SubstrateID != "substrate-1" {
		t.Fatalf("ASSERT_PROGRAM_A_COMPLETE_SUBSTRATE_ADMITTED: got=%#v err=%v", got, err)
	}
	zero := admittedProgramASubstrate(t)
	zero.Custody = VerifiedReceipt{}
	got, err = AdmitVerifiedProgramA(zero)
	if err != nil || got.Status != SubstrateRejected || !reflect.DeepEqual(got.Reasons, []string{"custody: no validated receipt"}) {
		t.Fatalf("ASSERT_PROGRAM_A_NO_RECEIPT_REJECTED: %#v %v", got, err)
	}
}

func TestProgramAReceiptValidationRejectsWrongAuthority(t *testing.T) {
	base := ReceiptBytes{Dimension: CustodyDimension, EvaluatorID: "lsp-trace/custody-evaluator", SchemaVersion: ReceiptSchemaVersion, Family: "custody", Version: "v1", CustodyRef: "custody:1", Revision: "526f658", SubstrateID: "substrate-1", Status: SubstrateAdmitted}
	base.Digest = receiptDigest(base)
	seed := []byte{0x9d, 0x61, 0xb1, 0x9d, 0xef, 0xfd, 0x5a, 0x60, 0xba, 0x84, 0x4a, 0xf4, 0x92, 0xec, 0x2c, 0xc4, 0x44, 0x49, 0xc5, 0x69, 0x7b, 0x32, 0x69, 0x19, 0x70, 0x3b, 0xac, 0x03, 0x1c, 0xae, 0x7f, 0x60}
	base.Signature = ed25519.Sign(ed25519.NewKeyFromSeed(seed), receiptPayload(base))
	cases := []struct {
		name   string
		mutate func(*ReceiptBytes)
		reason string
	}{
		{"evaluator", func(r *ReceiptBytes) { r.EvaluatorID = "caller" }, "wrong evaluator"},
		{"family", func(r *ReceiptBytes) { r.Family = "other" }, "schema/family/version"},
		{"version", func(r *ReceiptBytes) { r.Version = "v2" }, "schema/family/version"},
		{"digest", func(r *ReceiptBytes) { r.Digest = "sha256:arbitrary" }, "digest mismatch"},
		{"signature", func(r *ReceiptBytes) { r.Signature = append([]byte(nil), r.Signature...); r.Signature[0] ^= 0xff }, "signature mismatch"},
		{"custody", func(r *ReceiptBytes) { r.CustodyRef = ""; r.Digest = receiptDigest(*r) }, "missing custody"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := base
			tc.mutate(&raw)
			if _, err := VerifyReceipt(raw); err == nil || !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("ASSERT_PROGRAM_A_WRONG_%s_REJECTED: %v", strings.ToUpper(tc.name), err)
			}
		})
	}
	if _, err := VerifyReceipt(ReceiptBytes{}); err == nil {
		t.Fatal("ASSERT_PROGRAM_A_ARBITRARY_VALUE_REJECTED")
	}
}

func TestProgramAAdmissionRejectsMixedAuthorityAndOrder(t *testing.T) {
	input := admittedProgramASubstrate(t)
	input.Identity = receipt(t, IdentityDimension, "other", "substrate-1", 0)
	got, _ := AdmitVerifiedProgramA(input)
	if !reflect.DeepEqual(got.Reasons, []string{"identity: revision mismatch"}) {
		t.Fatalf("ASSERT_PROGRAM_A_CROSS_REVISION_REJECTED: %#v", got)
	}
	input = admittedProgramASubstrate(t)
	input.Projection = receipt(t, ProjectionDimension, "526f658", "other", 3)
	got, _ = AdmitVerifiedProgramA(input)
	if !reflect.DeepEqual(got.Reasons, []string{"projection: substrate mismatch"}) {
		t.Fatalf("ASSERT_PROGRAM_A_CROSS_SUBSTRATE_REJECTED: %#v", got)
	}
	input = admittedProgramASubstrate(t)
	input.RelationNormalization = receipt(t, RelationNormalizationDimension, "526f658", "substrate-1", 2)
	input.SupportAccounting = receipt(t, SupportAccountingDimension, "526f658", "substrate-1", 1)
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
