package censuscontinuation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"

	"lsp-trace/internal/censussourcecontinuation"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/programc"
)

type verifiedProgramCQualificationAuthority struct {
	result      ProgramCTransitionResult
	terminal    programCTransitionHead
	terminalRaw []byte
	inventory   []programCTransitionReceiptInventoryEntry
}

// verifyProgramCQualificationAuthority duplicates the accepted immutable
// completion reconstruction for qualification only. It adapts each uniquely
// selected receipt before advancing the reconstructed head, retains no partial
// authority on failure, and publishes authority only after terminal byte
// equality succeeds.
func verifyProgramCQualificationAuthority(ctx context.Context, workflowKey, headKey string, bindings ProgramCTransitionBindings, planRaw []byte, objects censussourcecontinuation.ImmutableStore, heads censussourcecontinuation.HeadStore, runner ProgramCTransitionRunner) (verifiedProgramCQualificationAuthority, []programCStage6Adaptation, error) {
	if ctx == nil || workflowKey == "" || headKey == "" || objects == nil || heads == nil || runner == nil {
		return verifiedProgramCQualificationAuthority{}, nil, ErrInvalidProgramCTransition
	}
	plan, err := decodeProgramCTransitionPlan(planRaw)
	if err != nil || !transitionValidBindings(bindings, plan, planRaw) || len(plan.Records) == 0 {
		return verifiedProgramCQualificationAuthority{}, nil, ErrInvalidProgramCTransition
	}
	terminalRaw, err := heads.Load(ctx, headKey)
	if err != nil {
		return verifiedProgramCQualificationAuthority{}, nil, err
	}
	var terminal programCTransitionHead
	bound := ProgramCTransitionRequest{WorkflowKey: workflowKey, HeadKey: headKey, Bindings: bindings, Objects: objects, Runner: runner}
	if transitionStrict(terminalRaw, &terminal) != nil || terminal.SchemaVersion != programCTransitionHeadVersion || transitionValidateHead(terminal, bound, len(plan.Records)) != nil || terminal.State != "COMMITTED" || terminal.CommittedPrefix != len(plan.Records) {
		return verifiedProgramCQualificationAuthority{}, nil, ErrInvalidProgramCTransition
	}
	terminalCopy := terminal
	terminalCanonical, err := transitionSelector(&terminalCopy)
	if err != nil || !bytes.Equal(terminalCanonical, terminalRaw) {
		return verifiedProgramCQualificationAuthority{}, nil, ErrInvalidProgramCTransition
	}

	reconstructed := programCTransitionHead{SchemaVersion: programCTransitionHeadVersion, WorkflowKey: workflowKey, HeadKey: headKey, Authority: 0, State: "ABSENT", Bindings: bindings, ConstituentCount: len(plan.Records), ClaimOrdinal: -1}
	inventory := make([]programCTransitionReceiptInventoryEntry, 0, len(plan.Records))
	adaptations := make([]programCStage6Adaptation, 0, len(plan.Records))
	var reconstructedRaw []byte
	for _, record := range plan.Records {
		if err := ctx.Err(); err != nil {
			return verifiedProgramCQualificationAuthority{}, nil, err
		}
		current := censussourcecontinuation.HeadRevision{Generation: reconstructed.Generation, Selector: reconstructed.Selector}
		claim := reconstructed
		claim.Generation++
		claim.State = "CLAIMED"
		claim.ClaimOrdinal = record.Ordinal
		claim.ClaimGeneration = claim.Generation
		claim.ClaimSelector = current.Selector
		claim.ClaimPublicationSelector = record.PublicationSelector
		claim.ClaimConstituentSelector = record.ConstituentSelector
		claim.ReceiptSelector = ""
		if _, err = transitionSelector(&claim); err != nil {
			return verifiedProgramCQualificationAuthority{}, nil, ErrInvalidProgramCTransition
		}
		computed, computeErr := transitionComputeReceipt(ctx, bound, claim, record)
		if contextErr := ctx.Err(); contextErr != nil {
			return verifiedProgramCQualificationAuthority{}, nil, contextErr
		}
		if computeErr != nil {
			if errors.Is(computeErr, context.Canceled) || errors.Is(computeErr, context.DeadlineExceeded) {
				return verifiedProgramCQualificationAuthority{}, nil, computeErr
			}
			return verifiedProgramCQualificationAuthority{}, nil, ErrInvalidProgramCTransition
		}
		if computed.receipt.Outcome != "SUCCEEDED" {
			return verifiedProgramCQualificationAuthority{}, nil, ErrInvalidProgramCTransition
		}
		candidates := []programCTransitionReceipt{computed.receipt}
		if computed.receipt.InstabilitySchema == programc.InstabilityVersionV2 {
			historical := computed.receipt
			historical.SchemaVersion = programCTransitionReceiptVersionV3
			candidates = append([]programCTransitionReceipt{historical}, candidates...)
		}
		matches := 0
		var selected programCTransitionReceiptInventoryEntry
		var linked programCTransitionHead
		for _, candidate := range candidates {
			candidateRaw, marshalErr := json.Marshal(candidate)
			if marshalErr != nil {
				return verifiedProgramCQualificationAuthority{}, nil, ErrInvalidProgramCTransition
			}
			selector := transitionSHA(candidateRaw)
			stored, getErr := objects.Get(ctx, selector)
			if getErr != nil {
				if errors.Is(getErr, context.Canceled) || errors.Is(getErr, context.DeadlineExceeded) {
					return verifiedProgramCQualificationAuthority{}, nil, getErr
				}
				if contextErr := ctx.Err(); contextErr != nil {
					return verifiedProgramCQualificationAuthority{}, nil, contextErr
				}
				if errors.Is(getErr, os.ErrNotExist) {
					continue
				}
				return verifiedProgramCQualificationAuthority{}, nil, getErr
			}
			if contextErr := ctx.Err(); contextErr != nil {
				return verifiedProgramCQualificationAuthority{}, nil, contextErr
			}
			candidateLinked := claim
			candidateLinked.Generation++
			candidateLinked.ReceiptSelector = selector
			if _, err = transitionSelector(&candidateLinked); err != nil {
				return verifiedProgramCQualificationAuthority{}, nil, ErrInvalidProgramCTransition
			}
			value, loadErr := transitionDecodeReceipt(stored, selector, bound, candidateLinked, plan)
			if loadErr != nil || !bytes.Equal(stored, candidateRaw) {
				return verifiedProgramCQualificationAuthority{}, nil, ErrInvalidProgramCTransition
			}
			matches++
			selected = programCTransitionReceiptInventoryEntry{ordinal: record.Ordinal, selector: selector, canonicalReceipt: bytes.Clone(stored), value: value, nativeV5: bytes.Clone(computed.nativeV5), outcomes: cloneProgramCOutcomes(computed.outcomes)}
			linked = candidateLinked
		}
		if matches != 1 {
			return verifiedProgramCQualificationAuthority{}, nil, ErrInvalidProgramCTransition
		}
		adapted, adaptErr := adaptProgramCQualificationStage6Entry(workflowKey, terminal.Selector, selected)
		if contextErr := ctx.Err(); contextErr != nil {
			return verifiedProgramCQualificationAuthority{}, nil, contextErr
		}
		if adaptErr != nil {
			return verifiedProgramCQualificationAuthority{}, nil, fmt.Errorf("Program C representative corpus preflight: Stage 6 adaptation ordinal=%d: %w", selected.ordinal, adaptErr)
		}
		committed := linked
		committed.Generation++
		committed.State = "COMMITTED"
		committed.CommittedPrefix = record.Ordinal + 1
		reconstructedRaw, err = transitionSelector(&committed)
		if err != nil {
			return verifiedProgramCQualificationAuthority{}, nil, ErrInvalidProgramCTransition
		}
		inventory = append(inventory, selected)
		adaptations = append(adaptations, adapted)
		reconstructed = committed
	}
	if !bytes.Equal(reconstructedRaw, terminalRaw) {
		return verifiedProgramCQualificationAuthority{}, nil, ErrInvalidProgramCTransition
	}
	if err := ctx.Err(); err != nil {
		return verifiedProgramCQualificationAuthority{}, nil, err
	}
	result := transitionResult(terminal, ProgramCTransitionComplete, inventory[len(inventory)-1].value)
	return verifiedProgramCQualificationAuthority{
		result:      result,
		terminal:    terminal,
		terminalRaw: bytes.Clone(terminalRaw),
		inventory:   cloneProgramCQualificationReceiptInventory(inventory),
	}, append([]programCStage6Adaptation(nil), adaptations...), nil
}

func adaptProgramCQualificationStage6Entry(workflowKey, terminalHeadSelector string, entry programCTransitionReceiptInventoryEntry) (programCStage6Adaptation, error) {
	adapted, err := adaptProgramCStage6InventoryEntry(workflowKey, terminalHeadSelector, entry)
	if err == nil {
		return adapted, nil
	}
	if diagnostic := programCQualificationInvocationSeedCountError(entry); diagnostic != nil {
		return programCStage6Adaptation{}, diagnostic
	}
	return programCStage6Adaptation{}, err
}

func programCQualificationInvocationSeedCountError(entry programCTransitionReceiptInventoryEntry) error {
	if len(entry.outcomes) == 0 {
		return nil
	}
	source := entry.outcomes[0].Source
	sourceBytes := source.InputBytes()
	graphV5Bytes := source.GraphV5Bytes()
	if !bytes.Equal(entry.nativeV5, sourceBytes) || source.InputLength != len(sourceBytes) || source.InputSHA256 != transitionSHA(sourceBytes) || len(graphV5Bytes) == 0 || source.GraphV5Length != len(graphV5Bytes) || source.GraphV5SHA256 != transitionSHA(graphV5Bytes) {
		return nil
	}
	for _, outcome := range entry.outcomes {
		if !reflect.DeepEqual(outcome.Source, source) || !reflect.DeepEqual(outcome.Projection.Source, source) {
			return nil
		}
	}
	view, err := graph.NewNativeV5RepresentativeView(graphV5Bytes)
	if err != nil {
		return nil
	}
	if count := len(view.InvocationSeeds()); count != 1 {
		return fmt.Errorf("%w: stage6 category=count invariant=invocation-seed-count observed=%d expected=1", ErrInvalidProgramCTransition, count)
	}
	return nil
}

func cloneProgramCQualificationReceiptInventory(in []programCTransitionReceiptInventoryEntry) []programCTransitionReceiptInventoryEntry {
	out := make([]programCTransitionReceiptInventoryEntry, len(in))
	for i := range in {
		out[i] = in[i]
		out[i].canonicalReceipt = bytes.Clone(in[i].canonicalReceipt)
		out[i].nativeV5 = bytes.Clone(in[i].nativeV5)
		out[i].outcomes = cloneProgramCOutcomes(in[i].outcomes)
	}
	return out
}
