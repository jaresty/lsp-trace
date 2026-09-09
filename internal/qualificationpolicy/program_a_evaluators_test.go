package qualificationpolicy

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/qualificationmatrix"
)

func evaluatorAuthorityForTest(t *testing.T) *programAEvaluatorAuthority {
	t.Helper()
	signer := newTestReceiptSigner(t)
	return &programAEvaluatorAuthority{verified: signer.authority, private: signer.private}
}

func retained(dimension Dimension, revision, substrate string, sequence int, data []byte) retainedProgramAEvidence {
	return retainedProgramAEvidence{CustodyRef: "retained:" + string(dimension), Revision: revision, SubstrateID: substrate, Sequence: sequence, Bytes: data}
}

func TestProgramAEvaluatorAuthorityFailsClosedWithoutAuthenticatedConfiguration(t *testing.T) {
	result := (*programAEvaluatorAuthority)(nil).evaluateProgramA(programAEvaluation{})
	if result.Admission.Status != SubstrateRejected || result.FailedAxes[CustodyDimension] != "authenticated evaluator authority is unavailable" {
		t.Fatalf("ASSERT_PROGRAM_A_EVALUATOR_DORMANT_WITHOUT_AUTHENTICATED_CONFIG: %#v", result)
	}
}

func TestProgramAEvaluatorHonestlyRejectsUnqualifiedRetainedIdentityAndTamper(t *testing.T) {
	graphBytes, err := os.ReadFile("../../qualification/retained/typescript/graph.json")
	if err != nil {
		t.Fatal(err)
	}
	input := programAEvaluation{Context: testAssessmentContext("assessment-evaluator", "RANKING", "NORMATIVE_PROGRAM_A")}
	input.Identity = retained(IdentityDimension, "c47032f", "typescript-retained", 0, graphBytes)
	result := evaluatorAuthorityForTest(t).evaluateProgramA(input)
	if result.Admission.Status != SubstrateRejected || !strings.Contains(result.FailedAxes[IdentityDimension], "lsp-trace.graph.v3") {
		t.Fatalf("ASSERT_PROGRAM_A_RETAINED_V2_NOT_UPGRADED_TO_V3: %#v", result)
	}
	input.Identity.Bytes = append([]byte(nil), graphBytes...)
	input.Identity.Bytes[len(input.Identity.Bytes)/2] ^= 0xff
	result = evaluatorAuthorityForTest(t).evaluateProgramA(input)
	if result.Admission.Status != SubstrateRejected || result.FailedAxes[IdentityDimension] == "" {
		t.Fatalf("ASSERT_PROGRAM_A_TAMPERED_RETAINED_IDENTITY_REJECTED: %#v", result)
	}
}

func TestProgramAA4RejectsRetainedB05MatrixAsNormativeProfile(t *testing.T) {
	raw, err := os.ReadFile("../../qualification/retained/b05/qualification-matrix.v2.json")
	if err != nil {
		t.Fatal(err)
	}
	var profile qualificationmatrix.Profile
	if err := json.Unmarshal(raw, &profile); err != nil {
		t.Fatal(err)
	}
	if err := qualificationmatrix.ValidateProfile(profile); err == nil || !strings.Contains(err.Error(), "unsupported schema_version") {
		t.Fatalf("ASSERT_PROGRAM_A_A4_B05_MATRIX_NOT_UPGRADED_TO_NORMATIVE_PROFILE: %v", err)
	}
	found := false
	for _, axis := range profile.Axes {
		if axis.Name == "provider_version" && len(axis.Members) != 0 {
			found = true
		}
	}
	if found {
		t.Fatal("ASSERT_PROGRAM_A_A4_B05_MATRIX_UNEXPECTEDLY_HAS_PROVIDER_EXACT_VERSION_AXIS")
	}
}

func TestProgramAEvaluatorReportsEveryMissingAxis(t *testing.T) {
	result := evaluatorAuthorityForTest(t).evaluateProgramA(programAEvaluation{Context: testAssessmentContext("assessment-missing", "RANKING", "NORMATIVE_PROGRAM_A")})
	want := []Dimension{CustodyDimension, EffectiveConfigurationDimension, IdentityDimension, RelationNormalizationDimension, SupportAccountingDimension, ProjectionDimension}
	if result.Admission.Status != SubstrateRejected || !reflect.DeepEqual(result.MissingAxes, want) {
		t.Fatalf("ASSERT_PROGRAM_A_EXPLICIT_MISSING_AXES: got=%#v want=%#v", result, want)
	}
	if result.FailedAxes[Dimension("qualification_matrix")] == "" {
		t.Fatalf("ASSERT_PROGRAM_A_EXPLICIT_FAILED_A4_AXIS: %#v", result)
	}
}

func TestProgramAEvaluatorReceiptBindsExactAuthorityEvidenceAndPipelineFields(t *testing.T) {
	authority := evaluatorAuthorityForTest(t)
	ctx := testAssessmentContext("assessment-receipt", "RANKING", "NORMATIVE_PROGRAM_A")
	evidence := retained(RelationNormalizationDimension, "c47032f", "substrate-real", 7, []byte("retained-evidence"))
	receipt, err := authority.issue(RelationNormalizationDimension, evidence, ctx)
	if err != nil {
		t.Fatal(err)
	}
	raw := receipt.receipt.raw
	if raw.Revision != evidence.Revision || raw.SubstrateID != evidence.SubstrateID || raw.Sequence != evidence.Sequence || raw.EvaluatorID != "lsp-trace/relation-normalization-evaluator" || raw.Family != "relation-normalization" || raw.Version != "v1" || raw.CustodyRef != evidence.CustodyRef || !strings.HasPrefix(raw.EvidenceDigest, "sha256:") {
		t.Fatalf("ASSERT_PROGRAM_A_RECEIPT_EXACT_PROVENANCE: %#v", raw)
	}
	tampered := raw
	tampered.EvidenceDigest = "sha256:tampered"
	if _, err := authority.verified.VerifyReceipt(tampered, ctx); err == nil {
		t.Fatal("ASSERT_PROGRAM_A_RECEIPT_TAMPER_REJECTED")
	}
}

func TestProgramAEvaluatorCompositionRejectsMixedRevisionSubstrateAndOrder(t *testing.T) {
	authority := evaluatorAuthorityForTest(t)
	ctx := testAssessmentContext("assessment-compose", "RANKING", "NORMATIVE_PROGRAM_A")
	makeInput := func() VerifiedProgramASubstrate {
		var out VerifiedProgramASubstrate
		for i, dimension := range []Dimension{CustodyDimension, EffectiveConfigurationDimension, IdentityDimension, RelationNormalizationDimension, SupportAccountingDimension, ProjectionDimension} {
			sequence := 0
			if i >= 3 {
				sequence = i - 3
			}
			receipt, err := authority.issue(dimension, retained(dimension, "c47032f", "substrate-real", sequence, []byte("evidence-"+string(dimension))), ctx)
			if err != nil {
				t.Fatal(err)
			}
			setProgramAReceipt(&out, dimension, receipt)
		}
		return out
	}

	mixedRevision := makeInput()
	mixedRevision.Identity, _ = authority.issue(IdentityDimension, retained(IdentityDimension, "other", "substrate-real", 0, []byte("other-revision")), ctx)
	if got, _ := AdmitVerifiedProgramA(mixedRevision); got.Status != SubstrateRejected || !containsReason(got.Reasons, "identity: revision mismatch") {
		t.Fatalf("ASSERT_PROGRAM_A_EVALUATOR_MIXED_REVISION_REJECTED: %#v", got)
	}
	mixedSubstrate := makeInput()
	mixedSubstrate.Projection, _ = authority.issue(ProjectionDimension, retained(ProjectionDimension, "c47032f", "other", 2, []byte("other-substrate")), ctx)
	if got, _ := AdmitVerifiedProgramA(mixedSubstrate); got.Status != SubstrateRejected || !containsReason(got.Reasons, "projection: substrate mismatch") {
		t.Fatalf("ASSERT_PROGRAM_A_EVALUATOR_MIXED_SUBSTRATE_REJECTED: %#v", got)
	}
	wrongOrder := makeInput()
	wrongOrder.RelationNormalization, _ = authority.issue(RelationNormalizationDimension, retained(RelationNormalizationDimension, "c47032f", "substrate-real", 2, []byte("normalization-order")), ctx)
	if got, _ := AdmitVerifiedProgramA(wrongOrder); got.Status != SubstrateRejected || !containsReason(got.Reasons, "pipeline: relation_normalization must precede support_accounting") {
		t.Fatalf("ASSERT_PROGRAM_A_EVALUATOR_PIPELINE_ORDER_REJECTED: %#v", got)
	}
}

func containsDimension(values []Dimension, want Dimension) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
