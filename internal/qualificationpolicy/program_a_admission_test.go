package qualificationpolicy

import (
	"reflect"
	"strings"
	"testing"
)

func admittedProgramASubstrate() ProgramASubstrate {
	return ProgramASubstrate{
		Custody:                SubstrateEvidence{Status: SubstrateAdmitted, EvidenceID: "custody:1"},
		EffectiveConfiguration: SubstrateEvidence{Status: SubstrateAdmitted, EvidenceID: "configuration:1"},
		Identity:               SubstrateEvidence{Status: SubstrateAdmitted, EvidenceID: "identity:1"},
		RelationNormalization:  SubstrateEvidence{Status: SubstrateAdmitted, EvidenceID: "normalization:1", Sequence: 1},
		SupportAccounting:      SubstrateEvidence{Status: SubstrateAdmitted, EvidenceID: "support:1", Sequence: 2},
		Projection:             SubstrateEvidence{Status: SubstrateAdmitted, EvidenceID: "projection:1", Sequence: 3},
	}
}

func TestProgramAAdmissionRequiresEveryEvidenceBackedDimension(t *testing.T) {
	input := admittedProgramASubstrate()
	got, err := AdmitProgramA(input)
	if err != nil || got.Status != SubstrateAdmitted || len(got.Reasons) != 0 {
		t.Fatalf("ASSERT_PROGRAM_A_COMPLETE_SUBSTRATE_ADMITTED: got=%#v err=%v", got, err)
	}

	input.Custody = SubstrateEvidence{Status: SubstrateMissing}
	got, err = AdmitProgramA(input)
	if err != nil || got.Status != SubstrateRejected || !reflect.DeepEqual(got.Reasons, []string{"custody: MISSING"}) {
		t.Fatalf("ASSERT_PROGRAM_A_MISSING_CUSTODY_REJECTED: got=%#v err=%v", got, err)
	}

	input = admittedProgramASubstrate()
	input.EffectiveConfiguration.Status = SubstrateRejected
	got, err = AdmitProgramA(input)
	if err != nil || got.Status != SubstrateRejected || !reflect.DeepEqual(got.Reasons, []string{"effective_configuration: REJECTED"}) {
		t.Fatalf("ASSERT_PROGRAM_A_REJECTED_CONFIGURATION_REJECTED: got=%#v err=%v", got, err)
	}
}

func TestProgramAAdmissionRejectsUnevidencedOrUnknownVerdicts(t *testing.T) {
	input := admittedProgramASubstrate()
	input.Identity.EvidenceID = ""
	got, err := AdmitProgramA(input)
	if err != nil || got.Status != SubstrateRejected || !reflect.DeepEqual(got.Reasons, []string{"identity: ADMITTED without evidence"}) {
		t.Fatalf("ASSERT_PROGRAM_A_UNEVIDENCED_IDENTITY_REJECTED: got=%#v err=%v", got, err)
	}

	input = admittedProgramASubstrate()
	input.Projection.Status = "UNKNOWN"
	got, err = AdmitProgramA(input)
	if err == nil || got.Status != SubstrateRejected || !strings.Contains(err.Error(), `projection has unknown status "UNKNOWN"`) {
		t.Fatalf("ASSERT_PROGRAM_A_UNKNOWN_STATUS_REJECTED: got=%#v err=%v", got, err)
	}
}

func TestProgramAAdmissionEnforcesPipelineOrder(t *testing.T) {
	input := admittedProgramASubstrate()
	input.RelationNormalization.Sequence = 2
	input.SupportAccounting.Sequence = 1
	got, err := AdmitProgramA(input)
	if err != nil || got.Status != SubstrateRejected || !reflect.DeepEqual(got.Reasons, []string{"pipeline: relation_normalization must precede support_accounting"}) {
		t.Fatalf("ASSERT_PROGRAM_A_NORMALIZATION_PRECEDES_SUPPORT: got=%#v err=%v", got, err)
	}

	input = admittedProgramASubstrate()
	input.SupportAccounting.Sequence = input.RelationNormalization.Sequence
	got, err = AdmitProgramA(input)
	if err != nil || got.Status != SubstrateRejected || !reflect.DeepEqual(got.Reasons, []string{"pipeline: relation_normalization must precede support_accounting"}) {
		t.Fatalf("ASSERT_PROGRAM_A_PIPELINE_EQUALITY_REJECTED: got=%#v err=%v", got, err)
	}

	input = admittedProgramASubstrate()
	input.SupportAccounting.Sequence = 3
	input.Projection.Sequence = 2
	got, err = AdmitProgramA(input)
	if err != nil || got.Status != SubstrateRejected || !reflect.DeepEqual(got.Reasons, []string{"pipeline: support_accounting must precede projection"}) {
		t.Fatalf("ASSERT_PROGRAM_A_SUPPORT_PRECEDES_PROJECTION: got=%#v err=%v", got, err)
	}
}

func TestProgramAAdmissionReasonsAreCompleteAndDeterministic(t *testing.T) {
	input := admittedProgramASubstrate()
	input.Projection = SubstrateEvidence{Status: SubstrateMissing}
	input.Custody = SubstrateEvidence{Status: SubstrateRejected, EvidenceID: "custody:reject"}
	first, err := AdmitProgramA(input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := AdmitProgramA(input)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"custody: REJECTED", "projection: MISSING"}
	if first.Status != SubstrateRejected || !reflect.DeepEqual(first.Reasons, want) || !reflect.DeepEqual(first, second) {
		t.Fatalf("ASSERT_PROGRAM_A_REASONS_COMPLETE_DETERMINISTIC: first=%#v second=%#v", first, second)
	}
}
