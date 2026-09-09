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

const ProgramBDecisionPolicy = "lsp-trace.program-b-admission-decision.v1"
const programBEvidenceDomain = "lsp-trace.program-b-program-a-evidence.v1\x00"

type ProgramBAdmissionBinding struct {
	BuildRevision, Operation, Scope, SubstrateID    string
	MatrixDigest, EvidenceSetDigest, DecisionPolicy string
}

type ProgramBExecutionExpectation struct {
	BuildRevision, Operation, Scope, SubstrateID string
	MatrixDigest, EvidenceSetDigest              string
}

// ProgramBAdmission is opaque. Only VerifyProgramBAdmission can produce a valid value.
type ProgramBAdmission struct{ binding programBAdmissionBinding }
type programBAdmissionBinding struct {
	buildRevision, operation, scope, substrateID    string
	matrixDigest, evidenceSetDigest, decisionPolicy string
	authorityID, keyID, provisioningDigest          string
	assessmentID, nonce                             string
	issuanceEpoch                                   int64
	admissionPolicyID, admissionPolicyVersion       string
}

func VerifyProgramBAdmission(a ProgramAAdmission, p qualificationmatrix.Profile, req qualificationmatrix.AdmissionRequest, binding ProgramBAdmissionBinding, now time.Time) (ProgramBAdmission, error) {
	if err := validateProgramAForB(a); err != nil {
		return ProgramBAdmission{}, err
	}
	matrix, err := qualificationmatrix.CanonicalBytes(p)
	if err != nil {
		return ProgramBAdmission{}, err
	}
	matrixDigest := digestBytes(matrix)
	evidenceDigest := programAEvidenceSetDigest(a.receiptDigests)
	if p.SchemaVersion != qualificationmatrix.SchemaVersionV2 || binding.BuildRevision == "" || binding.Operation == "" || binding.Scope == "" || binding.SubstrateID == "" {
		return ProgramBAdmission{}, fmt.Errorf("PROGRAM_B_ADMITTED requires exact v2 revision, operation, scope, and substrate")
	}
	if binding.DecisionPolicy != ProgramBDecisionPolicy || binding.MatrixDigest != matrixDigest || binding.EvidenceSetDigest != evidenceDigest {
		return ProgramBAdmission{}, fmt.Errorf("PROGRAM_B_ADMITTED matrix, evidence set, or decision policy mismatch")
	}
	if a.Revision != binding.BuildRevision || a.SubstrateID != binding.SubstrateID || a.operation != binding.Operation || a.evaluationScope != binding.Scope {
		return ProgramBAdmission{}, fmt.Errorf("PROGRAM_B_ADMITTED Program A context mismatch")
	}
	if len(req.RequestedOperations) != 1 || req.RequestedOperations[0] != binding.Operation {
		return ProgramBAdmission{}, fmt.Errorf("PROGRAM_B_ADMITTED operation must be requested exactly")
	}
	for _, result := range req.Results {
		if result.Waiver != nil {
			return ProgramBAdmission{}, fmt.Errorf("PROGRAM_B_ADMITTED canonical admission rejects waivers")
		}
	}
	if err := qualificationmatrix.ProgramBAdmitted(p, req, now); err != nil {
		return ProgramBAdmission{}, err
	}
	return ProgramBAdmission{binding: programBAdmissionBinding{buildRevision: binding.BuildRevision, operation: binding.Operation, scope: binding.Scope, substrateID: binding.SubstrateID, matrixDigest: matrixDigest, evidenceSetDigest: evidenceDigest, decisionPolicy: binding.DecisionPolicy, authorityID: a.authorityID, keyID: a.keyID, provisioningDigest: a.provisioningReceiptDigest, assessmentID: a.assessmentID, nonce: a.nonce, issuanceEpoch: a.issuanceEpoch, admissionPolicyID: a.admissionPolicyID, admissionPolicyVersion: a.admissionPolicyVersion}}, nil
}

func (a ProgramBAdmission) VerifyExecution(expected ProgramBExecutionExpectation) error {
	b := a.binding
	if b.buildRevision == "" || b.operation == "" || b.scope == "" || b.substrateID == "" || b.matrixDigest == "" || b.evidenceSetDigest == "" || b.decisionPolicy != ProgramBDecisionPolicy || b.authorityID == "" || b.keyID == "" || b.provisioningDigest == "" || b.assessmentID == "" || b.nonce == "" || b.issuanceEpoch == 0 || b.admissionPolicyID == "" || b.admissionPolicyVersion == "" {
		return fmt.Errorf("PROGRAM_B_ADMITTED opaque admission is invalid")
	}
	if expected.BuildRevision != b.buildRevision || expected.Operation != b.operation || expected.Scope != b.scope || expected.SubstrateID != b.substrateID || expected.MatrixDigest != b.matrixDigest || expected.EvidenceSetDigest != b.evidenceSetDigest {
		return fmt.Errorf("PROGRAM_B_ADMITTED execution context mismatch")
	}
	return nil
}

func validateProgramAForB(a ProgramAAdmission) error {
	if a.Status != SubstrateAdmitted || a.authorityID == "" || a.keyID == "" || a.provisioningReceiptDigest == "" || a.assessmentID == "" || a.nonce == "" || a.issuanceEpoch == 0 || a.evaluationScope == "" || a.admissionPolicyID == "" || a.admissionPolicyVersion == "" || a.operation == "" {
		return fmt.Errorf("PROGRAM_B_ADMITTED requires opaque admitted Program A provenance")
	}
	seen := map[string]struct{}{}
	for _, d := range a.receiptDigests {
		if strings.TrimSpace(d) == "" {
			return fmt.Errorf("PROGRAM_B_ADMITTED requires exactly seven receipt digests")
		}
		if _, ok := seen[d]; ok {
			return fmt.Errorf("PROGRAM_B_ADMITTED duplicate receipt digest")
		}
		seen[d] = struct{}{}
	}
	return nil
}

func programAEvidenceSetDigest(ds [7]string) string {
	b := []byte(programBEvidenceDomain)
	for _, d := range ds {
		var n [4]byte
		binary.BigEndian.PutUint32(n[:], uint32(len(d)))
		b = append(b, n[:]...)
		b = append(b, d...)
	}
	return digestBytes(b)
}
func digestBytes(b []byte) string { s := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(s[:]) }
