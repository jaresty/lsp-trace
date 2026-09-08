package qualificationpolicy

import "testing"

func TestProgramACallerValuesCannotAdmit(t *testing.T) {
	if got, err := AdmitVerifiedProgramA(VerifiedProgramASubstrate{}); err == nil && got.Status == SubstrateAdmitted {
		t.Fatalf("ASSERT_PROGRAM_A_CALLER_VALUES_CANNOT_ADMIT: arbitrary caller values admitted: %#v", got)
	}
}
