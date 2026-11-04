package censuscontinuation

import (
	"context"
	"fmt"
	"reflect"

	"lsp-trace/internal/programc"
	"lsp-trace/internal/programcrepresentative"
)

func (e ProgramCMultiSeedQualificationEvidenceV2) checkQualificationSubjectV2(s programcrepresentative.QualificationSubjectV2) error {
	expected, err := e.QualificationSubjectExpectedContextV2()
	if err != nil {
		return err
	}
	if s.CorpusIdentity != expected.CorpusIdentity || s.TerminalProgramCHead != expected.TerminalProgramCHead || !reflect.DeepEqual(s.ReceiptInventory, expected.ReceiptInventory) || !reflect.DeepEqual(s.AdapterContract, expected.AdapterContract) {
		return ErrInvalidProgramCTransition
	}
	return nil
}
func (e ProgramCMultiSeedQualificationEvidenceV2) ReconstructQualifiedMechanicsV2(ctx context.Context, s programcrepresentative.QualificationSubjectV2) ([]programcrepresentative.MechanicsV2, []programcrepresentative.CustodyIdentityDTO, error) {
	if ctx == nil {
		return nil, nil, ErrInvalidProgramCTransition
	}
	if err := e.checkQualificationSubjectV2(s); err != nil {
		return nil, nil, err
	}
	mechanics := []programcrepresentative.MechanicsV2{}
	custodies := []programcrepresentative.CustodyIdentityDTO{}
	for ordinal, adapted := range e.preflight.adaptations {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		if adapted.classification != programCStage6Complete {
			continue
		}
		entry := e.inventory[ordinal]
		source := entry.outcomes[0].Source
		c, err := programcrepresentative.BindReconstructedCustodyV2(programcrepresentative.CustodyIdentityDTO{SchemaVersion: programcrepresentative.CustodySchemaVersion, WorkflowKey: entry.value.WorkflowKey, TerminalProgramCHeadSelector: e.transition.Head.Selector, ReceiptSelector: entry.selector, ReceiptOrdinal: entry.value.ReceiptOrdinal, ConstituentOrdinal: entry.ordinal, PublicationSelector: entry.value.PublicationSelector, ConstituentSelector: entry.value.ConstituentSelector, NativeV5SHA256: source.InputSHA256, NativeV5ByteLength: uint64(source.InputLength), ExecutionBundleID: source.ExecutionBundleID})
		if err != nil {
			return nil, nil, err
		}
		for _, m := range adapted.mechanics {
			if err := programcrepresentative.AdmitMechanicsArtifactV2(m); err != nil {
				return nil, nil, err
			}
			dto := m.DTO()
			if dto.CustodyIdentity != c.Identity {
				return nil, nil, ErrInvalidProgramCTransition
			}
			mechanics = append(mechanics, m)
		}
		custodies = append(custodies, c)
	}
	return mechanics, custodies, nil
}

func (e ProgramCMultiSeedQualificationEvidenceV2) ReconstructCheckpointConstituentsV2(ctx context.Context, s programcrepresentative.QualificationSubjectV2) ([]programcrepresentative.ConstituentV2, map[programcrepresentative.ArtifactBindingDTO][]byte, error) {
	if ctx == nil {
		return nil, nil, ErrInvalidProgramCTransition
	}
	if err := e.checkQualificationSubjectV2(s); err != nil {
		return nil, nil, err
	}
	constituents := make([]programcrepresentative.ConstituentV2, 0, len(e.inventory))
	artifacts := map[programcrepresentative.ArtifactBindingDTO][]byte{}
	for i, entry := range e.inventory {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		source := entry.outcomes[0].Source
		c := programcrepresentative.ConstituentV2{ConstituentOrdinal: entry.ordinal, Receipt: programcrepresentative.ArtifactBindingDTO{SchemaVersion: entry.value.SchemaVersion, SHA256: entry.selector, ByteLength: uint64(len(entry.canonicalReceipt))}, PublicationSelector: entry.value.PublicationSelector, ConstituentSelector: entry.value.ConstituentSelector, NativeV5SHA256: source.InputSHA256, NativeV5ByteLength: uint64(source.InputLength), InstabilitySHA256: entry.value.ResultSHA256, InstabilityByteLength: uint64(entry.value.ResultByteLength), InstabilitySchemaVersion: entry.value.InstabilitySchema, ProgramCOutcome: entry.value.ProgramCOutcome, Disposition: entry.value.Disposition, Classification: string(e.preflight.adaptations[i].classification), Runs: []programcrepresentative.RunTerminalV2{}}
		for j, outcome := range entry.outcomes {
			run := programcrepresentative.RunTerminalV2{RunID: fmt.Sprintf("seed-%020d-replay-%d", outcome.Seed, j%3+1), Seed: outcome.Seed, ReplayOrdinal: uint8(j%3 + 1), Outcome: outcome.Outcome, CommunityCount: uint64(len(outcome.Communities))}
			if outcome.Outcome != "EMPTY" {
				id := programc.InstabilityIdentity{AdmittedGraphSHA256: outcome.Source.InputSHA256, ProjectionSHA256: programc.ProfileDigest, ProjectionPolicyID: programc.ProfileID, AlgorithmName: outcome.Algorithm, AlgorithmVersion: entry.value.Bindings.AlgorithmVersion, ParametersCanonicalSHA256: entry.value.Bindings.ParametersSHA256, ResourcePolicySHA256: entry.value.Bindings.ResourcePolicySHA256}
				size, err := programcrepresentative.CanonicalRebuiltCommunitySizeV2(outcome, id)
				if err != nil {
					return nil, nil, err
				}
				raw, err := programc.BuildCommunityArtifact(outcome, id)
				if err != nil {
					return nil, nil, err
				}
				if uint64(len(raw)) != size {
					return nil, nil, ErrInvalidProgramCTransition
				}
				run.CommunityArtifact = programcrepresentative.ArtifactBindingDTO{SchemaVersion: "lsp-trace.community.v1", SHA256: qualificationDigest(raw), ByteLength: uint64(len(raw))}
				artifacts[run.CommunityArtifact] = raw
			}
			c.Runs = append(c.Runs, run)
		}
		if _, err := programcrepresentative.CanonicalSizeConstituentV2(c); err != nil {
			return nil, nil, err
		}
		constituents = append(constituents, c)
	}
	return constituents, artifacts, nil
}
