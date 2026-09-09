package qualificationpolicy

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"lsp-trace/internal/qualificationmatrix"
)

const (
	ProgramBDecisionPolicyV2 = "lsp-trace.program-b-admission-decision.v2"
	programBEvidenceDomainV2 = "lsp-trace.program-b-program-a-evidence.v2\x00"
	programBMatrixDomainV2   = "lsp-trace.program-b-qualification-matrix.v2\x00"
)

type ProgramBAdmissionBindingV2 struct {
	BuildRevision, Operation, Scope, SubstrateID     string
	MatrixDigest, EvidenceSetDigest, DecisionPolicy  string
	ProgramAAdmissionVersion, ProgramAEvidenceDomain string
}

type ProgramBExecutionExpectationV2 struct {
	BuildRevision, Operation, Scope, SubstrateID string
	MatrixDigest, EvidenceSetDigest              string
	DecisionPolicy, ProgramAAdmissionVersion     string
}

type ProgramBAdmissionV2 struct{ binding programBAdmissionBindingV2 }
type programBAdmissionBindingV2 struct {
	buildRevision, operation, scope, substrateID     string
	matrixDigest, evidenceSetDigest, decisionPolicy  string
	programAAdmissionVersion, programAEvidenceDomain string
	authorityID, keyID, provisioningDigest           string
	assessmentID, nonce                              string
	issuanceEpoch                                    int64
	admissionPolicyID, admissionPolicyVersion        string
}

func VerifyProgramBAdmissionV2(a ProgramAAdmissionV2, p qualificationmatrix.Profile, req qualificationmatrix.AdmissionRequest, binding ProgramBAdmissionBindingV2, now time.Time) (ProgramBAdmissionV2, error) {
	if err := validateProgramAForBV2(a); err != nil {
		return ProgramBAdmissionV2{}, err
	}
	if p.SchemaVersion != qualificationmatrix.SchemaVersionV2 {
		return ProgramBAdmissionV2{}, fmt.Errorf("PROGRAM_B_ADMITTED_V2 requires qualification matrix v2")
	}
	matrix, err := qualificationmatrix.CanonicalBytes(p)
	if err != nil {
		return ProgramBAdmissionV2{}, err
	}
	matrixDigest := programBMatrixDigestV2(matrix)
	evidenceDigest := programAEvidenceSetDigestV2(a.receiptDigests)
	if binding.BuildRevision == "" || binding.Operation == "" || binding.Scope == "" || binding.SubstrateID == "" {
		return ProgramBAdmissionV2{}, fmt.Errorf("PROGRAM_B_ADMITTED_V2 requires exact revision, operation, scope, and substrate")
	}
	if binding.DecisionPolicy != ProgramBDecisionPolicyV2 || binding.ProgramAAdmissionVersion != ProgramAAdmissionVersionV2 || binding.ProgramAEvidenceDomain != programAEvidenceDomainV2 || binding.MatrixDigest != matrixDigest || binding.EvidenceSetDigest != evidenceDigest {
		return ProgramBAdmissionV2{}, fmt.Errorf("PROGRAM_B_ADMITTED_V2 version, matrix, evidence set, or decision policy mismatch")
	}
	if a.Revision != binding.BuildRevision || a.SubstrateID != binding.SubstrateID || a.operation != binding.Operation || a.evaluationScope != binding.Scope {
		return ProgramBAdmissionV2{}, fmt.Errorf("PROGRAM_B_ADMITTED_V2 Program A v2 context mismatch")
	}
	if len(req.RequestedOperations) != 1 || req.RequestedOperations[0] != binding.Operation {
		return ProgramBAdmissionV2{}, fmt.Errorf("PROGRAM_B_ADMITTED_V2 operation must be requested exactly")
	}
	for _, result := range req.Results {
		if result.Waiver != nil {
			return ProgramBAdmissionV2{}, fmt.Errorf("PROGRAM_B_ADMITTED_V2 rejects waivers")
		}
	}
	if err := qualificationmatrix.ProgramBAdmitted(p, req, now); err != nil {
		return ProgramBAdmissionV2{}, err
	}
	return ProgramBAdmissionV2{binding: programBAdmissionBindingV2{buildRevision: binding.BuildRevision, operation: binding.Operation, scope: binding.Scope, substrateID: binding.SubstrateID, matrixDigest: matrixDigest, evidenceSetDigest: evidenceDigest, decisionPolicy: binding.DecisionPolicy, programAAdmissionVersion: binding.ProgramAAdmissionVersion, programAEvidenceDomain: binding.ProgramAEvidenceDomain, authorityID: a.authorityID, keyID: a.keyID, provisioningDigest: a.provisioningReceiptDigest, assessmentID: a.assessmentID, nonce: a.nonce, issuanceEpoch: a.issuanceEpoch, admissionPolicyID: a.admissionPolicyID, admissionPolicyVersion: a.admissionPolicyVersion}}, nil
}

func (a ProgramBAdmissionV2) VerifyExecution(expected ProgramBExecutionExpectationV2) error {
	b := a.binding
	if b.buildRevision == "" || b.operation == "" || b.scope == "" || b.substrateID == "" || b.matrixDigest == "" || b.evidenceSetDigest == "" || b.decisionPolicy != ProgramBDecisionPolicyV2 || b.programAAdmissionVersion != ProgramAAdmissionVersionV2 || b.programAEvidenceDomain != programAEvidenceDomainV2 || b.authorityID == "" || b.keyID == "" || b.provisioningDigest == "" || b.assessmentID == "" || b.nonce == "" || b.issuanceEpoch == 0 || b.admissionPolicyID == "" || b.admissionPolicyVersion != "v2" {
		return fmt.Errorf("PROGRAM_B_ADMITTED_V2 opaque admission is invalid")
	}
	if expected.BuildRevision != b.buildRevision || expected.Operation != b.operation || expected.Scope != b.scope || expected.SubstrateID != b.substrateID || expected.MatrixDigest != b.matrixDigest || expected.EvidenceSetDigest != b.evidenceSetDigest || expected.DecisionPolicy != b.decisionPolicy || expected.ProgramAAdmissionVersion != b.programAAdmissionVersion {
		return fmt.Errorf("PROGRAM_B_ADMITTED_V2 execution context mismatch")
	}
	return nil
}

func validateProgramAForBV2(a ProgramAAdmissionV2) error {
	if a.Status != SubstrateAdmitted || a.authorityID == "" || a.keyID == "" || a.provisioningReceiptDigest == "" || a.assessmentID == "" || a.nonce == "" || a.issuanceEpoch == 0 || a.evaluationScope == "" || a.admissionPolicyID != programAAdmissionPolicyV2 || a.admissionPolicyVersion != "v2" || a.operation == "" || a.admissionFamily != ProgramAAdmissionFamilyV2 || a.admissionVersion != ProgramAAdmissionVersionV2 || a.evidenceDomain != programAEvidenceDomainV2 {
		return fmt.Errorf("PROGRAM_B_ADMITTED_V2 requires opaque admitted Program A v2 provenance")
	}
	seen := map[string]struct{}{}
	for _, d := range a.receiptDigests {
		if strings.TrimSpace(d) == "" {
			return fmt.Errorf("PROGRAM_B_ADMITTED_V2 requires exactly seven receipt digests")
		}
		if _, ok := seen[d]; ok {
			return fmt.Errorf("PROGRAM_B_ADMITTED_V2 duplicate receipt digest")
		}
		seen[d] = struct{}{}
	}
	return nil
}

func programAEvidenceSetDigestV2(ds [7]string) string {
	b := []byte(programBEvidenceDomainV2)
	for _, d := range ds {
		var n [4]byte
		binary.BigEndian.PutUint32(n[:], uint32(len(d)))
		b = append(b, n[:]...)
		b = append(b, d...)
	}
	return digestBytes(b)
}

func programBMatrixDigestV2(matrix []byte) string {
	b := append([]byte(programBMatrixDomainV2), matrix...)
	s := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(s[:])
}
