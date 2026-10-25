package inventorysuccessor

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
)

func retainedWaitingHead(t *testing.T) (*memoryHeads, GapLedgerRequest) {
	t.Helper()
	heads := &memoryHeads{}
	successorRequest := validRequest(heads)
	if _, err := coordinateTest(context.Background(), successorRequest); err != nil {
		t.Fatal(err)
	}
	return heads, GapLedgerRequest{WorkflowKey: successorRequest.WorkflowKey, HeadKey: successorRequest.HeadKey, Identity: successorRequest.Identity, Heads: heads}
}

func TestBuildDescribePreparationGapLedgerFromRetainedHead(t *testing.T) {
	_, request := retainedWaitingHead(t)
	first, err := BuildDescribePreparationGapLedger(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildDescribePreparationGapLedger(context.Background(), request)
	if err != nil || !bytes.Equal(first, second) {
		t.Fatalf("ASSERT_GAP_LEDGER_DETERMINISTIC err=%v equal=%t", err, bytes.Equal(first, second))
	}
	ledger, err := ValidateDescribePreparationGapLedger(first)
	if err != nil {
		t.Fatal(err)
	}
	if ledger.Outcome != OutcomeWaitingEvidence || ledger.CurrentStage != StageDescribePreparation || ledger.LastCompletedStage != StageProgramC || ledger.SuccessorHeadSelector == "" || ledger.ProgramCPredecessorHead == "" {
		t.Fatalf("ASSERT_GAP_LEDGER_LINEAGE ledger=%+v", ledger)
	}
	want := []EvidenceGap{GapRepresentativeNominationDenominator, GapCommunityRepresentativeLineage, GapRetainedSourceClosure, GapFullDefinitionBindings, GapCallerOccurrenceV3Custody}
	if len(ledger.Requirements) != len(want) {
		t.Fatalf("ASSERT_GAP_LEDGER_REQUIREMENT_COUNT got=%d", len(ledger.Requirements))
	}
	for index, requirement := range ledger.Requirements {
		if requirement.ID != want[index] || requirement.Status != GapStatusNotRetained || requirement.EvidenceSelector != nil || requirement.EvidenceByteLength != nil {
			t.Fatalf("ASSERT_GAP_LEDGER_REQUIREMENT_%d got=%+v", index, requirement)
		}
	}
	if ledger.RepresentativeNomination.Status != ComputationNotComputed || ledger.RepresentativeNomination.Count != nil || ledger.CandidatePreparation.Status != ComputationNotComputed || ledger.CandidatePreparation.Count != nil {
		t.Fatalf("ASSERT_GAP_LEDGER_NULL_COUNTS ledger=%+v", ledger)
	}
	if ledger.Authority != 0 || ledger.Accepted || ledger.FeatureIdentity != "UNRESOLVED" || ledger.Completeness != "UNKNOWN" || ledger.Effects != "NONE" || ledger.ProductionAdmission != "REFUSED_MISSING_EVIDENCE" || ledger.AbsenceInference != "PROHIBITED" {
		t.Fatalf("ASSERT_GAP_LEDGER_AUTHORITY ledger=%+v", ledger)
	}
}

func TestDescribePreparationGapLedgerRejectsHeadAndLedgerMutation(t *testing.T) {
	heads, request := retainedWaitingHead(t)
	retained := bytes.Clone(heads.raw)
	heads.raw[len(heads.raw)-2] ^= 1
	if _, err := BuildDescribePreparationGapLedger(context.Background(), request); err == nil {
		t.Fatal("ASSERT_GAP_LEDGER_REJECTS_MUTATED_RETAINED_HEAD")
	}
	heads.raw = retained
	wrongIdentity := request
	wrongIdentity.Identity.SourcePinSHA256 = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if _, err := BuildDescribePreparationGapLedger(context.Background(), wrongIdentity); err == nil {
		t.Fatal("ASSERT_GAP_LEDGER_REJECTS_WORKFLOW_IDENTITY_SUBSTITUTION")
	}
	raw, err := BuildDescribePreparationGapLedger(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	var ledger DescribePreparationGapLedger
	if err = json.Unmarshal(raw, &ledger); err != nil {
		t.Fatal(err)
	}
	ledger.Requirements[0].Status = GapStatusRetained
	mutated, _ := json.Marshal(ledger)
	if _, err = ValidateDescribePreparationGapLedger(mutated); err == nil {
		t.Fatal("ASSERT_GAP_LEDGER_REJECTS_FORGED_PRESENT_EVIDENCE")
	}
}
