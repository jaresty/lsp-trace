package qualificationpolicy_test

import (
	"reflect"
	"testing"
	"time"

	"lsp-trace/internal/qualificationmatrix"
	"lsp-trace/internal/qualificationpolicy"
)

func TestExternalCallerCannotInspectProvenanceOrMintAdmission(t *testing.T) {
	programAType := reflect.TypeOf(qualificationpolicy.ProgramAAdmission{})
	for _, forbidden := range []string{"AuthorityID", "KeyID", "ProvisioningReceiptDigest", "AssessmentID", "Nonce", "IssuanceEpoch", "EvaluationScope", "AdmissionPolicyID", "AdmissionPolicyVersion", "Operation", "ReceiptDigests"} {
		if _, ok := programAType.FieldByName(forbidden); ok {
			t.Fatalf("ASSERT_EXTERNAL_CALLER_CANNOT_INSPECT_PROGRAM_A: exported %s", forbidden)
		}
	}
	programBType := reflect.TypeOf(qualificationpolicy.ProgramBAdmission{})
	if programBType.NumField() != 1 || programBType.Field(0).IsExported() {
		t.Fatal("ASSERT_EXTERNAL_CALLER_CANNOT_INSPECT_PROGRAM_B")
	}
	_, err := qualificationpolicy.VerifyProgramBAdmission(qualificationpolicy.ProgramAAdmission{}, qualificationmatrix.Profile{}, qualificationmatrix.AdmissionRequest{}, qualificationpolicy.ProgramBAdmissionBinding{}, time.Time{})
	if err == nil {
		t.Fatal("ASSERT_EXTERNAL_CALLER_CANNOT_MINT_PROGRAM_B")
	}
}
