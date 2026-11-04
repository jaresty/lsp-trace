package censuscontinuation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"lsp-trace/internal/censussourcecontinuation"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/programcrepresentative"
)

// ProgramCMultiSeedQualificationEvidenceV2 carries no public mutable inventory,
// retained Native bodies, caller-selected seeds, or writable store handle.
type ProgramCMultiSeedQualificationEvidenceV2 struct {
	transition  ProgramCTransitionResult
	terminalRaw []byte
	inventory   []programCTransitionReceiptInventoryEntry
	preflight   programCV2PreflightResult
	corpus      programcrepresentative.CorpusIdentityDTO
}

// BuildProgramCMultiSeedQualificationEvidenceV2 is the production-owned complete
// inventory reconstruction boundary; caller-supplied runner/seed/count overrides
// are deliberately absent. Controlled fixtures use the private sibling below.
func BuildProgramCMultiSeedQualificationEvidenceV2(ctx context.Context, request ProgramCTransitionVerificationRequest) (ProgramCMultiSeedQualificationEvidenceV2, error) {
	if ctx == nil || request.PlanSource == nil || request.PlanSource.VerifyPublication != nil {
		return ProgramCMultiSeedQualificationEvidenceV2{}, ErrInvalidProgramCTransition
	}
	planRaw, err := request.PlanSource.Load(ctx)
	if err != nil {
		return ProgramCMultiSeedQualificationEvidenceV2{}, err
	}
	return buildProgramCMultiSeedQualificationEvidenceV2WithRunner(ctx, request.WorkflowKey, request.HeadKey, request.Bindings, planRaw, request.Objects, request.Heads, NewProgramCProductionRunner(request.PlanSource), programCV2ExpectedAccounting{})
}

func (e ProgramCMultiSeedQualificationEvidenceV2) QualificationSubjectExpectedContextV2() (programcrepresentative.QualificationSubjectExpectedContextV2, error) {
	zero := programcrepresentative.QualificationSubjectExpectedContextV2{}
	if len(e.inventory) == 0 || len(e.terminalRaw) == 0 || e.transition.Status != ProgramCTransitionComplete {
		return zero, ErrInvalidProgramCTransition
	}
	var head programCTransitionHead
	if transitionStrict(e.terminalRaw, &head) != nil {
		return zero, ErrInvalidProgramCTransition
	}
	adapter, err := programcrepresentative.BuildAdapterContractV2(e.inventory[0].value.Bindings.AlgorithmVersion)
	if err != nil {
		return zero, err
	}
	return programcrepresentative.QualificationSubjectExpectedContextV2{AdapterContract: adapter, CorpusIdentity: e.corpus, ReceiptInventory: qualificationReceiptInventory(e.inventory), TerminalProgramCHead: programcrepresentative.TerminalProgramCHeadDTO{SchemaVersion: head.SchemaVersion, WorkflowKey: head.WorkflowKey, HeadKey: head.HeadKey, Generation: head.Generation, Selector: head.Selector, HeadBytesSHA256: qualificationDigest(e.terminalRaw), ByteLength: uint64(len(e.terminalRaw)), CommittedPrefix: head.CommittedPrefix, ConstituentCount: head.ConstituentCount}}, nil
}

func deriveProgramCMultiSeedExpectedV2(inventory []programCTransitionReceiptInventoryEntry) (programCV2ExpectedAccounting, error) {
	expected := programCV2ExpectedAccounting{}
	for _, entry := range inventory {
		if entry.value.ProgramCOutcome != "COMPLETE" || entry.value.Disposition != "" {
			continue
		}
		if len(entry.outcomes) != 6 {
			return programCV2ExpectedAccounting{}, ErrInvalidProgramCTransition
		}
		view, err := graph.NewNativeV5RepresentativeView(entry.outcomes[0].Source.GraphV5Bytes())
		if err != nil {
			return programCV2ExpectedAccounting{}, err
		}
		groups := uint64(len(view.InvocationSeeds()))
		for _, outcome := range entry.outcomes {
			d := uint64(len(outcome.Communities))
			expected.eligibleRunCommunityResultsD, err = programCV2CheckedAdd(expected.eligibleRunCommunityResultsD, d)
			if err != nil {
				return programCV2ExpectedAccounting{}, err
			}
			g, err := programCV2CheckedMultiply(d, groups)
			if err != nil {
				return programCV2ExpectedAccounting{}, err
			}
			expected.seedGroupResultsG, err = programCV2CheckedAdd(expected.seedGroupResultsG, g)
			if err != nil {
				return programCV2ExpectedAccounting{}, err
			}
		}
	}
	return expected, nil
}

type multiSeedQualificationHeadSnapshotV2 struct {
	censussourcecontinuation.HeadStore
	key    string
	raw    []byte
	loaded bool
}

func (s *multiSeedQualificationHeadSnapshotV2) Load(ctx context.Context, key string) ([]byte, error) {
	if s.loaded || key != s.key {
		return nil, fmt.Errorf("multiseed qualification requires one terminal snapshot")
	}
	s.loaded = true
	raw, err := s.HeadStore.Load(ctx, key)
	if err != nil {
		return nil, err
	}
	if uint64(len(raw)) > programcrepresentative.MaxArtifactCanonicalBytesV2 {
		return nil, fmt.Errorf("multiseed terminal snapshot exceeds artifact cap")
	}
	s.raw = bytes.Clone(raw)
	return raw, nil
}

func buildProgramCMultiSeedQualificationEvidenceV2WithRunner(ctx context.Context, workflowKey, headKey string, bindings ProgramCTransitionBindings, planRaw []byte, objects censussourcecontinuation.ImmutableStore, heads censussourcecontinuation.HeadStore, runner ProgramCTransitionRunner, expected programCV2ExpectedAccounting) (ProgramCMultiSeedQualificationEvidenceV2, error) {
	zero := ProgramCMultiSeedQualificationEvidenceV2{}
	if ctx == nil || heads == nil || objects == nil || runner == nil {
		return zero, ErrInvalidProgramCTransition
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	snapshot := &multiSeedQualificationHeadSnapshotV2{HeadStore: heads, key: headKey}
	transition, inventory, err := verifyProgramCTransitionReceiptInventory(ctx, workflowKey, headKey, bindings, planRaw, objects, snapshot, runner)
	if err != nil {
		return zero, err
	}
	if !snapshot.loaded || len(snapshot.raw) == 0 {
		return zero, ErrInvalidProgramCTransition
	}
	reconstructedExpected, err := deriveProgramCMultiSeedExpectedV2(inventory)
	if err != nil {
		return zero, err
	}
	// The optional private equality expectation exists for controlled negative
	// fixtures only; the public path supplies none and owns reconstruction.
	if expected != (programCV2ExpectedAccounting{}) && expected != reconstructedExpected {
		return zero, ErrInvalidProgramCTransition
	}
	preflight, err := finalizeProgramCMultiSeedV2Preflight(ctx, transition, inventory, reconstructedExpected)
	if err != nil {
		return zero, err
	}
	for _, adapted := range preflight.adaptations {
		for _, m := range adapted.mechanics {
			dto := m.DTO()
			if _, err := programcrepresentative.CanonicalSizeMechanicsIdentityDTOv2(dto); err != nil {
				return zero, err
			}
		}
	}
	var terminal programCTransitionHead
	if transitionStrict(snapshot.raw, &terminal) != nil || terminal.Selector != transition.Head.Selector || terminal.Generation != transition.Head.Generation || terminal.CommittedPrefix != len(inventory) {
		return zero, ErrInvalidProgramCTransition
	}
	bindingsRaw, err := json.Marshal(bindings)
	if err != nil {
		return zero, err
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return ProgramCMultiSeedQualificationEvidenceV2{transition: transition, terminalRaw: bytes.Clone(snapshot.raw), inventory: cloneProgramCQualificationReceiptInventory(inventory), preflight: preflight, corpus: programcrepresentative.CorpusIdentityDTO{FrozenCorpusSHA256: bindings.FrozenCorpusSHA256, CensusReceiptSHA256: bindings.CensusReceiptSHA256, AvailabilityLedgerSHA256: bindings.LedgerSHA256, ProgramCPlanSHA256: bindings.PlanSHA256, ProgramCBindingsSHA256: qualificationDigest(bindingsRaw)}}, nil
}
