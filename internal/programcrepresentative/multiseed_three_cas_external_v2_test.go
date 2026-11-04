package programcrepresentative_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/programc"
	"lsp-trace/internal/programcrepresentative"
	"lsp-trace/internal/publication"
)

type controlledCorpusV2 struct {
	mechanics    []programcrepresentative.MechanicsV2
	custody      []programcrepresentative.CustodyIdentityDTO
	constituents []programcrepresentative.ConstituentV2
	objects      map[programcrepresentative.ArtifactBindingDTO][]byte
}

func (c controlledCorpusV2) ReconstructQualifiedMechanicsV2(context.Context, programcrepresentative.QualificationSubjectV2) ([]programcrepresentative.MechanicsV2, []programcrepresentative.CustodyIdentityDTO, error) {
	return c.mechanics, c.custody, nil
}
func (c controlledCorpusV2) ReconstructCheckpointConstituentsV2(context.Context, programcrepresentative.QualificationSubjectV2) ([]programcrepresentative.ConstituentV2, map[programcrepresentative.ArtifactBindingDTO][]byte, error) {
	return c.constituents, c.objects, nil
}

func controlledCheckpointQualificationV2(t *testing.T) programcrepresentative.CheckedQualificationCalculationV2 {
	t.Helper()
	in := mechanicsV2Input(t, 2)
	expected := programcrepresentative.ControlledSubjectContextForTestsV2(t)
	expected.TerminalProgramCHead.WorkflowKey = in.WorkflowKey
	expected.TerminalProgramCHead.Selector = in.TerminalProgramCHeadSelector
	expected.ReceiptInventory[0].ReceiptSelector = in.ReceiptSelector
	expected.ReceiptInventory[0].ReceiptSHA256 = in.ReceiptSelector
	expected.ReceiptInventory[0].ReceiptOrdinal = in.ReceiptOrdinal
	custody, err := programcrepresentative.BindReconstructedCustodyV2(programcrepresentative.CustodyIdentityDTO{SchemaVersion: programcrepresentative.CustodySchemaVersion, WorkflowKey: in.WorkflowKey, TerminalProgramCHeadSelector: in.TerminalProgramCHeadSelector, ReceiptSelector: in.ReceiptSelector, ReceiptOrdinal: in.ReceiptOrdinal, ConstituentOrdinal: in.ConstituentOrdinal, PublicationSelector: in.PublicationSelector, ConstituentSelector: in.ConstituentSelector, NativeV5SHA256: in.Source.InputSHA256, NativeV5ByteLength: uint64(in.Source.InputLength), ExecutionBundleID: in.Source.ExecutionBundleID})
	if err != nil {
		t.Fatal(err)
	}
	corpus := controlledCorpusV2{custody: []programcrepresentative.CustodyIdentityDTO{custody}, objects: map[programcrepresentative.ArtifactBindingDTO][]byte{}}
	c := programcrepresentative.ConstituentV2{ConstituentOrdinal: 0, Receipt: programcrepresentative.ArtifactBindingDTO{SchemaVersion: expected.ReceiptInventory[0].ReceiptSchemaVersion, SHA256: in.ReceiptSelector, ByteLength: expected.ReceiptInventory[0].ReceiptByteLength}, PublicationSelector: in.PublicationSelector, ConstituentSelector: in.ConstituentSelector, NativeV5SHA256: in.Source.InputSHA256, NativeV5ByteLength: uint64(in.Source.InputLength), InstabilitySchemaVersion: programc.InstabilityVersionV2, ProgramCOutcome: "COMPLETE", Classification: "COMPLETE", Runs: []programcrepresentative.RunTerminalV2{}}
	for seed := uint64(1); seed <= 2; seed++ {
		for replay := uint8(1); replay <= 3; replay++ {
			outcome, failure := programc.Compute(in.Source.InputBytes(), seed)
			if failure != nil {
				t.Fatal(failure)
			}
			id := programc.InstabilityIdentity{AdmittedGraphSHA256: in.Source.InputSHA256, ProjectionSHA256: programc.ProfileDigest, ProjectionPolicyID: programc.ProfileID, AlgorithmName: outcome.Algorithm, AlgorithmVersion: in.ProgramCAlgorithmVersion, ParametersCanonicalSHA256: expected.WorkflowIdentity.QualificationPolicySHA256, ResourcePolicySHA256: expected.WorkflowIdentity.ResourcePolicySHA256}
			raw, err := programc.BuildCommunityArtifact(outcome, id)
			if err != nil {
				t.Fatal(err)
			}
			b := programcrepresentative.ArtifactBindingDTO{SchemaVersion: "lsp-trace.community.v1", SHA256: digestMechanicsV2(raw), ByteLength: uint64(len(raw))}
			corpus.objects[b] = raw
			runID := fmt.Sprintf("seed-%020d-replay-%d", seed, replay)
			c.Runs = append(c.Runs, programcrepresentative.RunTerminalV2{RunID: runID, Seed: seed, ReplayOrdinal: replay, Outcome: outcome.Outcome, CommunityArtifact: b, CommunityCount: uint64(len(outcome.Communities))})
			for ordinal, community := range outcome.Communities {
				one := in
				one.RunID = runID
				one.Seed = seed
				one.ReplayOrdinal = replay
				one.CommunityOrdinal = ordinal
				one.CommunityArtifactSHA256 = b.SHA256
				one.CommunityMembers = community.Members
				m, err := programcrepresentative.NewMechanicsV2(one)
				if err != nil {
					t.Fatal(err)
				}
				corpus.mechanics = append(corpus.mechanics, m)
			}
		}
	}
	instability, err := programc.ComputeInstabilityV2FromGraphProvenanceV5(in.Source.InputBytes(), []uint64{1, 2}, in.ProgramCAlgorithmVersion, expected.WorkflowIdentity.QualificationPolicySHA256, expected.WorkflowIdentity.ResourcePolicySHA256)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(instability)
	if err != nil {
		t.Fatal(err)
	}
	c.InstabilitySHA256 = digestMechanicsV2(raw)
	c.InstabilityByteLength = uint64(len(raw))
	corpus.constituents = []programcrepresentative.ConstituentV2{c}
	return programcrepresentative.ControlledQualificationForTestsV2(t, expected, corpus)
}

// This exact function type is a compiling structural witness: an interface or
// checked calculation is not a production store-authority parameter.
var _ func(*publication.Root, inventorysuccessor.MultiSeedOperationKindV2, string, string, inventorysuccessor.WorkflowIdentity, censuscontinuation.VerifiedQualificationV2) (*inventorysuccessor.MultiSeedCheckpointOperationV2, error) = inventorysuccessor.NewMultiSeedCheckpointOperationV2

func TestMultiSeedV2ExternalCalculationIsNotStoreAuthority(t *testing.T) {
	calculation := controlledCheckpointQualificationV2(t)
	if _, ok := any(calculation).(censuscontinuation.VerifiedQualificationV2); ok {
		t.Fatal("EXTERNAL_CALCULATION_IS_NOT_STORE_AUTHORITY")
	}
	subject, err := calculation.SubjectDTOv2()
	if err != nil {
		t.Fatal(err)
	}
	var identity inventorysuccessor.WorkflowIdentity
	raw, _ := json.Marshal(subject.WorkflowIdentity)
	if err = json.Unmarshal(raw, &identity); err != nil {
		t.Fatal(err)
	}
	op, err := inventorysuccessor.NewMultiSeedCheckpointOperationV2(nil, inventorysuccessor.MultiSeedDenominatorOperationV2, "workflow", "head", identity, censuscontinuation.VerifiedQualificationV2{})
	if !errors.Is(err, censuscontinuation.ErrQualificationUnavailableV2) || op != nil {
		t.Fatal("EXTERNAL_ZERO_CAPABILITY_REFUSES_BEFORE_STORAGE")
	}
	t.Log("EXTERNAL_CALCULATION_IS_NOT_STORE_AUTHORITY PASS: genuine COMPLETE calculation cannot convert; concrete zero capability refused before root access")
}
