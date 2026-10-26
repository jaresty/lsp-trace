package censuscontinuation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"

	"lsp-trace/internal/programcrepresentative"
)

const programCRepresentativeImplementationSourceBytes uint64 = 135281

// ProgramCRepresentativeQualificationEvidence is immutable verified Program C
// evidence for the Program C side of a qualification subject expected context.
// It carries no predecessor workflow identity, retained Native V5 material, or
// storage handles.
type ProgramCRepresentativeQualificationEvidence struct {
	adapterContract              programcrepresentative.AdapterContractDTO
	stage6Implementation         programcrepresentative.Stage6ImplementationDTO
	stage6Policy                 programcrepresentative.Stage6PolicyDTO
	stage6PolicyBinding          programcrepresentative.ArtifactBindingDTO
	stage6Limits                 programcrepresentative.Stage6LimitsDTO
	stage6LimitsBinding          programcrepresentative.ArtifactBindingDTO
	stage6Finalization           programcrepresentative.Stage6FinalizationDTO
	stage6FinalizationBinding    programcrepresentative.ArtifactBindingDTO
	corpusIdentity               programcrepresentative.CorpusIdentityDTO
	receiptInventory             []programcrepresentative.ReceiptInventoryEntryDTO
	terminalProgramCHead         programcrepresentative.TerminalProgramCHeadDTO
	maxImplementationSourceBytes uint64
}

// QualificationSubjectExpectedContext returns a defensive copy containing only
// the verified Program C side. The authoritative workflow composes the retained
// predecessor identity before constructing a trusted subject.
func (e ProgramCRepresentativeQualificationEvidence) QualificationSubjectExpectedContext() programcrepresentative.QualificationSubjectExpectedContext {
	return programcrepresentative.QualificationSubjectExpectedContext{
		AdapterContract:              cloneQualificationAdapterContract(e.adapterContract),
		Stage6Implementation:         cloneQualificationImplementation(e.stage6Implementation),
		Stage6Policy:                 cloneQualificationPolicy(e.stage6Policy),
		Stage6PolicyBinding:          e.stage6PolicyBinding,
		Stage6Limits:                 e.stage6Limits,
		Stage6LimitsBinding:          e.stage6LimitsBinding,
		Stage6Finalization:           cloneQualificationFinalization(e.stage6Finalization),
		Stage6FinalizationBinding:    e.stage6FinalizationBinding,
		CorpusIdentity:               e.corpusIdentity,
		ReceiptInventory:             append([]programcrepresentative.ReceiptInventoryEntryDTO(nil), e.receiptInventory...),
		TerminalProgramCHead:         e.terminalProgramCHead,
		MaxImplementationSourceBytes: e.maxImplementationSourceBytes,
	}
}

// BuildProgramCRepresentativeQualificationEvidence performs one authoritative
// Program C completion pass, including exactly one terminal-head load, and the
// exact representative-corpus preflight before returning verified evidence.
func BuildProgramCRepresentativeQualificationEvidence(ctx context.Context, request ProgramCTransitionVerificationRequest) (ProgramCRepresentativeQualificationEvidence, error) {
	if ctx == nil || request.PlanSource == nil || request.PlanSource.VerifyPublication != nil {
		return ProgramCRepresentativeQualificationEvidence{}, ErrInvalidProgramCTransition
	}
	planRaw, err := request.PlanSource.Load(ctx)
	if err != nil {
		return ProgramCRepresentativeQualificationEvidence{}, err
	}
	runner := NewProgramCProductionRunner(request.PlanSource)
	var authority verifiedProgramCTransitionAuthority
	if _, _, err = verifyProgramCTransitionReceiptInventoryWithAuthority(ctx, request.WorkflowKey, request.HeadKey, request.Bindings, planRaw, request.Objects, request.Heads, runner, &authority); err != nil {
		return ProgramCRepresentativeQualificationEvidence{}, err
	}

	adapter, err := programcrepresentative.BuildAdapterContract(request.Bindings.AlgorithmVersion)
	if err != nil {
		return ProgramCRepresentativeQualificationEvidence{}, ErrInvalidProgramCTransition
	}
	implementation, err := programcrepresentative.BuildStage6Implementation(programCRepresentativeImplementationSourceBytes)
	if err != nil {
		return ProgramCRepresentativeQualificationEvidence{}, ErrInvalidProgramCTransition
	}
	policy, policyBinding, _, err := programcrepresentative.BuildStage6PolicyArtifact()
	if err != nil {
		return ProgramCRepresentativeQualificationEvidence{}, ErrInvalidProgramCTransition
	}
	limits, limitsBinding, _, err := programcrepresentative.BuildStage6LimitsArtifact()
	if err != nil {
		return ProgramCRepresentativeQualificationEvidence{}, ErrInvalidProgramCTransition
	}
	finalization, finalizationBinding, _, err := programcrepresentative.BuildStage6FinalizationArtifact()
	if err != nil {
		return ProgramCRepresentativeQualificationEvidence{}, ErrInvalidProgramCTransition
	}
	if err := preflightProgramCRepresentativeCorpus(request.WorkflowKey, authority.result.Head.Selector, authority.inventory, limits); err != nil {
		return ProgramCRepresentativeQualificationEvidence{}, fmt.Errorf("Program C representative corpus preflight: %w", err)
	}

	bindingsRaw, err := json.Marshal(request.Bindings)
	if err != nil {
		return ProgramCRepresentativeQualificationEvidence{}, ErrInvalidProgramCTransition
	}
	return ProgramCRepresentativeQualificationEvidence{
		adapterContract:           cloneQualificationAdapterContract(adapter),
		stage6Implementation:      cloneQualificationImplementation(implementation),
		stage6Policy:              cloneQualificationPolicy(policy),
		stage6PolicyBinding:       policyBinding,
		stage6Limits:              limits,
		stage6LimitsBinding:       limitsBinding,
		stage6Finalization:        cloneQualificationFinalization(finalization),
		stage6FinalizationBinding: finalizationBinding,
		corpusIdentity: programcrepresentative.CorpusIdentityDTO{
			FrozenCorpusSHA256:       request.Bindings.FrozenCorpusSHA256,
			CensusReceiptSHA256:      request.Bindings.CensusReceiptSHA256,
			AvailabilityLedgerSHA256: request.Bindings.LedgerSHA256,
			ProgramCPlanSHA256:       request.Bindings.PlanSHA256,
			ProgramCBindingsSHA256:   qualificationDigest(bindingsRaw),
		},
		receiptInventory: qualificationReceiptInventory(authority.inventory),
		terminalProgramCHead: programcrepresentative.TerminalProgramCHeadDTO{
			SchemaVersion:    authority.terminal.SchemaVersion,
			WorkflowKey:      authority.terminal.WorkflowKey,
			HeadKey:          authority.terminal.HeadKey,
			Generation:       authority.terminal.Generation,
			Selector:         authority.terminal.Selector,
			HeadBytesSHA256:  qualificationDigest(authority.terminalRaw),
			ByteLength:       uint64(len(authority.terminalRaw)),
			CommittedPrefix:  authority.terminal.CommittedPrefix,
			ConstituentCount: authority.terminal.ConstituentCount,
		},
		maxImplementationSourceBytes: programCRepresentativeImplementationSourceBytes,
	}, nil
}

func qualificationReceiptInventory(in []programCTransitionReceiptInventoryEntry) []programcrepresentative.ReceiptInventoryEntryDTO {
	out := make([]programcrepresentative.ReceiptInventoryEntryDTO, len(in))
	for i := range in {
		out[i] = programcrepresentative.ReceiptInventoryEntryDTO{
			PlanOrdinal: in[i].ordinal, ReceiptOrdinal: in[i].value.ReceiptOrdinal,
			ReceiptSchemaVersion: in[i].value.SchemaVersion, ReceiptSelector: in[i].selector,
			ReceiptSHA256: qualificationDigest(in[i].canonicalReceipt), ReceiptByteLength: uint64(len(in[i].canonicalReceipt)),
		}
	}
	return out
}

func cloneQualificationAdapterContract(in programcrepresentative.AdapterContractDTO) programcrepresentative.AdapterContractDTO {
	out := in
	out.Domains = append([]string(nil), in.Domains...)
	out.RunSeeds = append([]uint64(nil), in.RunSeeds...)
	out.MembershipKinds = append([]string(nil), in.MembershipKinds...)
	out.MappedMembershipKinds = append([]string(nil), in.MappedMembershipKinds...)
	return out
}

func cloneQualificationImplementation(in programcrepresentative.Stage6ImplementationDTO) programcrepresentative.Stage6ImplementationDTO {
	out := in
	out.Files = append([]programcrepresentative.ImplementationFileDTO(nil), in.Files...)
	return out
}

func cloneQualificationPolicy(in programcrepresentative.Stage6PolicyDTO) programcrepresentative.Stage6PolicyDTO {
	out := in
	out.SelectionOrder = append([]string(nil), in.SelectionOrder...)
	out.PredecessorOrder = append([]string(nil), in.PredecessorOrder...)
	return out
}

func cloneQualificationFinalization(in programcrepresentative.Stage6FinalizationDTO) programcrepresentative.Stage6FinalizationDTO {
	out := in
	out.CompositionMappings = append([]string(nil), in.CompositionMappings...)
	out.EvidenceMappings = append([]string(nil), in.EvidenceMappings...)
	out.GroupingMappings = append([]string(nil), in.GroupingMappings...)
	out.ContextMappings = append([]string(nil), in.ContextMappings...)
	out.QualificationMappings = append([]string(nil), in.QualificationMappings...)
	out.FinalizationOrder = append([]string(nil), in.FinalizationOrder...)
	return out
}

func preflightProgramCRepresentativeCorpus(workflowKey, terminalHeadSelector string, inventory []programCTransitionReceiptInventoryEntry, limits programcrepresentative.Stage6LimitsDTO) error {
	adaptations := make([]programCStage6Adaptation, len(inventory))
	for i := range inventory {
		adapted, err := adaptProgramCStage6InventoryEntry(workflowKey, terminalHeadSelector, inventory[i])
		if err != nil {
			return ErrInvalidProgramCTransition
		}
		adaptations[i] = adapted
	}
	return measureProgramCRepresentativeCorpus(adaptations, limits)
}

func measureProgramCRepresentativeCorpus(adaptations []programCStage6Adaptation, limits programcrepresentative.Stage6LimitsDTO) error {
	type runMeasurement struct {
		communities uint64
		members     uint64
		occurrences uint64
	}
	var maxCommunities, maxMembers, maxCandidates, maxOccurrences, maxStringBytes, maxWork uint64
	for _, adapted := range adaptations {
		switch adapted.classification {
		case programCStage6Empty, programCStage6ResourceIncomplete:
			if len(adapted.mechanics) != 0 {
				return ErrInvalidProgramCTransition
			}
			continue
		case programCStage6Complete:
			if len(adapted.mechanics) == 0 {
				return ErrInvalidProgramCTransition
			}
		default:
			return ErrInvalidProgramCTransition
		}

		runs := map[string]*runMeasurement{}
		for _, mechanics := range adapted.mechanics {
			dto := mechanics.DTO()
			run := runs[dto.RunID]
			if run == nil {
				run = &runMeasurement{occurrences: uint64(len(dto.Evidence))}
				runs[dto.RunID] = run
			} else if run.occurrences != uint64(len(dto.Evidence)) {
				return ErrInvalidProgramCTransition
			}
			run.communities++
			run.members += uint64(len(dto.CommunityMembers))
			maxCandidates = maximumObserved(maxCandidates, uint64(len(dto.CommunityMembers)))
			maxStringBytes = maximumObserved(maxStringBytes, maximumStringBytes(reflect.ValueOf(dto)))
		}
		for _, run := range runs {
			maxCommunities = maximumObserved(maxCommunities, run.communities)
			maxMembers = maximumObserved(maxMembers, run.members)
			maxOccurrences = maximumObserved(maxOccurrences, run.occurrences)
			maxWork = maximumObserved(maxWork, run.occurrences+run.members)
		}
	}
	if exceedsIntLimit(maxCommunities, limits.MaxCommunities) || exceedsIntLimit(maxMembers, limits.MaxMembers) || exceedsIntLimit(maxCandidates, limits.MaxCandidates) || exceedsIntLimit(maxOccurrences, limits.MaxOccurrences) || exceedsIntLimit(maxStringBytes, limits.MaxStringBytes) || exceedsIntLimit(maxWork, limits.MaxWork) {
		return fmt.Errorf("%w: Program C corpus exceeds Stage 6 limits", ErrInvalidProgramCTransition)
	}
	return nil
}

func maximumObserved(left, right uint64) uint64 {
	if right > left {
		return right
	}
	return left
}

func exceedsIntLimit(observed uint64, limit int) bool {
	return limit < 0 || observed > uint64(limit)
}

func maximumStringBytes(v reflect.Value) uint64 {
	if !v.IsValid() {
		return 0
	}
	switch v.Kind() {
	case reflect.Interface, reflect.Pointer:
		if v.IsNil() {
			return 0
		}
		return maximumStringBytes(v.Elem())
	case reflect.String:
		return uint64(len(v.String()))
	case reflect.Struct:
		var max uint64
		for i := 0; i < v.NumField(); i++ {
			if got := maximumStringBytes(v.Field(i)); got > max {
				max = got
			}
		}
		return max
	case reflect.Array, reflect.Slice:
		var max uint64
		for i := 0; i < v.Len(); i++ {
			if got := maximumStringBytes(v.Index(i)); got > max {
				max = got
			}
		}
		return max
	default:
		return 0
	}
}

func qualificationDigest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}
