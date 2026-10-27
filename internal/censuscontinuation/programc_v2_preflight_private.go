package censuscontinuation

import (
	"context"
	"fmt"

	"lsp-trace/internal/censussourcecontinuation"
)

const (
	programCV2FrozenEligibleRunCommunityResultsD uint64 = 15537
	programCV2FrozenSeedGroupResultsG            uint64 = 530232
)

type programCV2ExpectedAccounting struct {
	eligibleRunCommunityResultsD uint64
	seedGroupResultsG            uint64
}

type programCV2PreflightAccounting struct {
	completeConstituents           uint64
	emptyConstituents              uint64
	resourceIncompleteConstituents uint64
	eligibleRunCommunityResultsD   uint64
	seedGroupResultsG              uint64
}

type programCV2PreflightResult struct {
	transition  ProgramCTransitionResult
	adaptations []programCStage6AdaptationV2
	accounting  programCV2PreflightAccounting
}

func preflightFrozenProgramCMultiSeedV2(ctx context.Context, request ProgramCTransitionVerificationRequest) (programCV2PreflightResult, error) {
	if ctx == nil || request.PlanSource == nil || request.PlanSource.VerifyPublication != nil {
		return programCV2PreflightResult{}, ErrInvalidProgramCTransition
	}
	planRaw, err := request.PlanSource.Load(ctx)
	if err != nil {
		return programCV2PreflightResult{}, err
	}
	runner := NewProgramCProductionRunner(request.PlanSource)
	return preflightProgramCMultiSeedV2WithRunner(ctx, request.WorkflowKey, request.HeadKey, request.Bindings, planRaw, request.Objects, request.Heads, runner, programCV2ExpectedAccounting{
		eligibleRunCommunityResultsD: programCV2FrozenEligibleRunCommunityResultsD,
		seedGroupResultsG:            programCV2FrozenSeedGroupResultsG,
	})
}

func preflightProgramCMultiSeedV2WithRunner(ctx context.Context, workflowKey, headKey string, bindings ProgramCTransitionBindings, planRaw []byte, objects censussourcecontinuation.ImmutableStore, heads censussourcecontinuation.HeadStore, runner ProgramCTransitionRunner, expected programCV2ExpectedAccounting) (programCV2PreflightResult, error) {
	transition, inventory, err := verifyProgramCTransitionReceiptInventory(ctx, workflowKey, headKey, bindings, planRaw, objects, heads, runner)
	if err != nil {
		return programCV2PreflightResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return programCV2PreflightResult{}, err
	}
	return finalizeProgramCMultiSeedV2Preflight(ctx, transition, inventory, expected)
}

func finalizeProgramCMultiSeedV2Preflight(ctx context.Context, transition ProgramCTransitionResult, inventory []programCTransitionReceiptInventoryEntry, expected programCV2ExpectedAccounting) (programCV2PreflightResult, error) {
	if ctx == nil || transition.Status != ProgramCTransitionComplete || transition.Head.Selector == "" || len(inventory) == 0 {
		return programCV2PreflightResult{}, ErrInvalidProgramCTransition
	}
	adaptations := make([]programCStage6AdaptationV2, 0, len(inventory))
	accounting := programCV2PreflightAccounting{}
	for _, entry := range inventory {
		if err := ctx.Err(); err != nil {
			return programCV2PreflightResult{}, err
		}
		adapted, err := adaptProgramCStage6InventoryEntryV2(entry.value.WorkflowKey, transition.Head.Selector, entry)
		if err != nil {
			return programCV2PreflightResult{}, fmt.Errorf("Program C multi-seed v2 preflight: Stage 6 adaptation ordinal=%d: %w", entry.ordinal, err)
		}
		switch adapted.classification {
		case programCStage6Complete:
			accounting.completeConstituents, err = programCV2CheckedAdd(accounting.completeConstituents, 1)
			if err != nil {
				return programCV2PreflightResult{}, err
			}
			accounting.eligibleRunCommunityResultsD, err = programCV2CheckedAdd(accounting.eligibleRunCommunityResultsD, uint64(len(adapted.mechanics)))
			if err != nil {
				return programCV2PreflightResult{}, err
			}
			for _, mechanics := range adapted.mechanics {
				if !mechanics.Valid() || !mechanics.Trusted() {
					return programCV2PreflightResult{}, ErrInvalidProgramCTransition
				}
				accounting.seedGroupResultsG, err = programCV2CheckedAdd(accounting.seedGroupResultsG, uint64(len(mechanics.SelectionContext().SeedGroups)))
				if err != nil {
					return programCV2PreflightResult{}, err
				}
			}
		case programCStage6Empty:
			if len(adapted.mechanics) != 0 {
				return programCV2PreflightResult{}, ErrInvalidProgramCTransition
			}
			accounting.emptyConstituents, err = programCV2CheckedAdd(accounting.emptyConstituents, 1)
			if err != nil {
				return programCV2PreflightResult{}, err
			}
		case programCStage6ResourceIncomplete:
			if len(adapted.mechanics) != 0 {
				return programCV2PreflightResult{}, ErrInvalidProgramCTransition
			}
			accounting.resourceIncompleteConstituents, err = programCV2CheckedAdd(accounting.resourceIncompleteConstituents, 1)
			if err != nil {
				return programCV2PreflightResult{}, err
			}
		default:
			return programCV2PreflightResult{}, ErrInvalidProgramCTransition
		}
		adaptations = append(adaptations, adapted)
	}
	if err := ctx.Err(); err != nil {
		return programCV2PreflightResult{}, err
	}
	constituents, err := programCV2CheckedSum(accounting.completeConstituents, accounting.emptyConstituents, accounting.resourceIncompleteConstituents)
	if err != nil || constituents != uint64(len(inventory)) || accounting.eligibleRunCommunityResultsD != expected.eligibleRunCommunityResultsD || accounting.seedGroupResultsG != expected.seedGroupResultsG {
		return programCV2PreflightResult{}, ErrInvalidProgramCTransition
	}
	return programCV2PreflightResult{transition: transition, adaptations: adaptations, accounting: accounting}, nil
}
