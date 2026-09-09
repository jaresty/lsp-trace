package qualificationpolicy

import (
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/qualificationmatrix"
)

const (
	programAV1EvidenceSetGolden = "sha256:a404fa13fc730ec476227c1ef58f8c34635c9128360bdacc9ba076f9313339e7"
	programAV2EvidenceSetGolden = "sha256:d5634130ad6ff2fbc37a8ef57bf9a2eff75e1b40feb402097f5002dc9ef22048"
)

func fixedV1Digests() [6]string {
	return [6]string{"sha256:receipt-0", "sha256:receipt-1", "sha256:receipt-2", "sha256:receipt-3", "sha256:receipt-4", "sha256:receipt-5"}
}

func fixedV2Digests() [7]string {
	return [7]string{"sha256:receipt-0", "sha256:receipt-1", "sha256:receipt-2", "sha256:receipt-3", "sha256:receipt-4", "sha256:receipt-5", "sha256:receipt-6"}
}

func TestProgramAV1CanonicalReceiptBytesGoldenFrozen(t *testing.T) {
	raw := rawReceipt(CustodyDimension, "golden-rev", "golden-substrate", 0)
	payload := receiptPayload(raw)
	if len(payload) != 357 || receiptDigest(raw) != "sha256:b5fc328b3d3e2da7d57887642c838c896503490472bb3733ac8bff7589218271" {
		t.Fatalf("ASSERT_PROGRAM_A_V1_CANONICAL_RECEIPT_BYTES_FROZEN: bytes=%d digest=%s", len(payload), receiptDigest(raw))
	}
}

func TestProgramAV1CanonicalEvidenceSetGoldenFrozen(t *testing.T) {
	if got := programAEvidenceSetDigest(fixedV1Digests()); got != programAV1EvidenceSetGolden {
		t.Fatalf("ASSERT_PROGRAM_B_V1_EVIDENCE_DIGEST_FROZEN: got=%s want=%s", got, programAV1EvidenceSetGolden)
	}
	if ProgramBDecisionPolicy != "lsp-trace.program-b-admission-decision.v1" || programBEvidenceDomain != "lsp-trace.program-b-program-a-evidence.v1\x00" {
		t.Fatal("ASSERT_PROGRAM_B_V1_IDENTITIES_FROZEN")
	}
}

func TestProgramAV2CanonicalEvidenceSetGoldenDistinct(t *testing.T) {
	if got := programAEvidenceSetDigestV2(fixedV2Digests()); got != programAV2EvidenceSetGolden {
		t.Fatalf("ASSERT_PROGRAM_A_V2_EVIDENCE_DIGEST_FROZEN: got=%s want=%s", got, programAV2EvidenceSetGolden)
	}
	if ProgramAAdmissionFamilyV2 != "lsp-trace.program-a-admission" || ProgramAAdmissionVersionV1 == ProgramAAdmissionVersionV2 || programAAdmissionPolicyV2 == "program-a-admission" || programAEvidenceDomainV2 == programBEvidenceDomain || ProgramBDecisionPolicyV2 == ProgramBDecisionPolicy || programBEvidenceDomainV2 == programBEvidenceDomain {
		t.Fatal("ASSERT_PROGRAM_A_V2_IDENTITIES_DISTINCT")
	}
}

func admittedProgramAV2Fixture(t *testing.T, ctx AssessmentContext) ProgramAAdmissionV2 {
	t.Helper()
	s := newTestReceiptSigner(t)
	s.authority.policyID = ctx.AdmissionPolicyID
	got, err := AdmitVerifiedProgramAV2(admittedProgramASubstrateV2WithSigner(t, s, ctx))
	if err != nil || got.Status != SubstrateAdmitted {
		t.Fatalf("ASSERT_PROGRAM_A_V2_SEVEN_RECEIPTS_INCLUDES_QUALIFICATION: got=%#v err=%v", got, err)
	}
	return got
}

func programBV2Fixture(t *testing.T) (ProgramAAdmissionV2, qualificationmatrix.Profile, qualificationmatrix.AdmissionRequest, ProgramBAdmissionBindingV2) {
	t.Helper()
	ctx := testAssessmentContext("assessment-b-v2", "RANKING", "NORMATIVE_PROGRAM_B_V2_ONLY")
	ctx.AdmissionPolicyID = programAAdmissionPolicyV2
	ctx.AdmissionPolicyVersion = "v2"
	a := admittedProgramAV2Fixture(t, ctx)
	_, p, req, _ := programBFixture(t)
	matrix, err := qualificationmatrix.CanonicalBytes(p)
	if err != nil {
		t.Fatal(err)
	}
	binding := ProgramBAdmissionBindingV2{BuildRevision: a.Revision, Operation: a.operation, Scope: a.evaluationScope, SubstrateID: a.SubstrateID, MatrixDigest: programBMatrixDigestV2(matrix), EvidenceSetDigest: programAEvidenceSetDigestV2(a.receiptDigests), DecisionPolicy: ProgramBDecisionPolicyV2, ProgramAAdmissionVersion: ProgramAAdmissionVersionV2, ProgramAEvidenceDomain: programAEvidenceDomainV2}
	return a, p, req, binding
}

func TestProgramAVersionedReceiptCountsAndDowngradesReject(t *testing.T) {
	v1, err := AdmitVerifiedProgramA(admittedProgramASubstrate(t))
	if err != nil || v1.Status != SubstrateAdmitted || len(v1.receiptDigests) != 6 {
		t.Fatalf("ASSERT_PROGRAM_A_V1_SIX_RECEIPTS_ACCEPTED: %#v %v", v1, err)
	}
	ctx := testAssessmentContext("assessment-a-v2", "RANKING", "NORMATIVE_PROGRAM_A_V2")
	ctx.AdmissionPolicyID, ctx.AdmissionPolicyVersion = programAAdmissionPolicyV2, "v2"
	v2 := admittedProgramAV2Fixture(t, ctx)
	if len(v2.receiptDigests) != 7 || v2.receiptDigests[6] == "" {
		t.Fatal("ASSERT_PROGRAM_A_VERSION_RECEIPT_COUNTS_REJECT_CROSS_ARITY")
	}
	bad := ctx
	bad.AdmissionPolicyVersion = "v1"
	s := newTestReceiptSigner(t)
	s.authority.policyID = bad.AdmissionPolicyID
	downgraded, _ := AdmitVerifiedProgramAV2(admittedProgramASubstrateV2WithSigner(t, s, bad))
	if downgraded.Status != SubstrateRejected || !strings.Contains(strings.Join(downgraded.Reasons, " "), "requires policy identity and version v2") {
		t.Fatalf("ASSERT_PROGRAM_ADMISSION_DOWNGRADE_REJECTED: %#v", downgraded)
	}
}

func TestProgramBV2AcceptsOnlySevenReceiptV2Context(t *testing.T) {
	a, p, req, binding := programBV2Fixture(t)
	now := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	admission, err := VerifyProgramBAdmissionV2(a, p, req, binding, now)
	if err != nil {
		t.Fatalf("ASSERT_PROGRAM_B_V2_CONSUMES_ONLY_A2_AND_MATRIX_V2: %v", err)
	}
	expected := ProgramBExecutionExpectationV2{BuildRevision: binding.BuildRevision, Operation: binding.Operation, Scope: binding.Scope, SubstrateID: binding.SubstrateID, MatrixDigest: binding.MatrixDigest, EvidenceSetDigest: binding.EvidenceSetDigest, DecisionPolicy: binding.DecisionPolicy, ProgramAAdmissionVersion: binding.ProgramAAdmissionVersion}
	if err := admission.VerifyExecution(expected); err != nil {
		t.Fatal(err)
	}
	mutations := map[string]func(*ProgramAAdmissionV2, *ProgramBAdmissionBindingV2){
		"omit-a4": func(a *ProgramAAdmissionV2, _ *ProgramBAdmissionBindingV2) { a.receiptDigests[6] = "" },
		"reorder-a4": func(a *ProgramAAdmissionV2, _ *ProgramBAdmissionBindingV2) {
			a.receiptDigests[5], a.receiptDigests[6] = a.receiptDigests[6], a.receiptDigests[5]
		},
		"substitute-a4": func(a *ProgramAAdmissionV2, _ *ProgramBAdmissionBindingV2) {
			a.receiptDigests[6] = "sha256:substituted-a4"
		},
		"v1-policy": func(_ *ProgramAAdmissionV2, b *ProgramBAdmissionBindingV2) { b.DecisionPolicy = ProgramBDecisionPolicy },
		"v1-domain": func(_ *ProgramAAdmissionV2, b *ProgramBAdmissionBindingV2) {
			b.ProgramAEvidenceDomain = programBEvidenceDomain
		},
		"v1-version": func(_ *ProgramAAdmissionV2, b *ProgramBAdmissionBindingV2) {
			b.ProgramAAdmissionVersion = ProgramAAdmissionVersionV1
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			changedA, changedBinding := a, binding
			mutate(&changedA, &changedBinding)
			if _, err := VerifyProgramBAdmissionV2(changedA, p, req, changedBinding, now); err == nil {
				t.Fatalf("ASSERT_PROGRAM_B_V2_CROSS_VERSION_MUTATION_REJECTED: %s", name)
			}
		})
	}
}
