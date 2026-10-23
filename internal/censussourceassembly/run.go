package censussourceassembly

import (
	"context"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censussourcecontinuation"
	"lsp-trace/internal/censussourcedriver"
	"lsp-trace/internal/censussourceplan"
	"lsp-trace/internal/publication"
)

// Discover returns neutral discovery custody for one exact source partition.
type Discover func(context.Context, int, []censusacquisition.SourceFile) (censusacquisition.Discovery, error)

// Identity optionally binds a run to one opaque manager session generation.
type Identity struct {
	SessionID         string
	ManagerGeneration uint64
}

// Request contains the stable caller-owned inputs for one bounded assembly run.
type Request struct {
	Plan          censussourceplan.Plan
	PartitionSize int
	WorkflowKey   string
	HeadKey       string
	Objects       censussourcecontinuation.ImmutableStore
	Heads         censussourcecontinuation.HeadStore
	ObjectRoot    *publication.Root
	HeadRoot      *publication.Root
	StepBound     int
	Identity      Identity
	Discover      Discover

	// CoordinateForTest transparently preserves the driver's internal integration
	// seam. Production callers leave it nil.
	CoordinateForTest func(context.Context, censussourcecontinuation.Request) (censussourcecontinuation.Result, error)
}

// Run performs bounded sequential source-partition transitions and returns only
// the last terminal record accepted by an advancing committed transition.
func Run(ctx context.Context, request Request) (censussourcedriver.Result, *TerminalRecord) {
	partitionCount := len(request.Plan.Partitions)
	initial := censussourcedriver.State{
		Status:            censussourcedriver.StatusCommitted,
		CommittedPrefix:   0,
		PartitionCount:    partitionCount,
		PlanIdentity:      request.Plan.Identity,
		SessionID:         request.Identity.SessionID,
		ManagerGeneration: request.Identity.ManagerGeneration,
	}
	identityValid := request.Identity.SessionID == "" && request.Identity.ManagerGeneration == 0 || request.Identity.SessionID != "" && request.Identity.ManagerGeneration > 0
	if ctx == nil || !identityValid || request.StepBound <= 0 || request.Discover == nil || request.WorkflowKey == "" || request.HeadKey == "" || censussourceplan.Validate(request.Plan, request.PartitionSize) != nil {
		return censussourcedriver.Result{Stop: censussourcedriver.StopFailed, State: initial, Err: censussourcedriver.ErrInvalid}, nil
	}

	objects := request.Objects
	heads := request.Heads
	injected := objects != nil && heads != nil && request.ObjectRoot == nil && request.HeadRoot == nil
	rooted := objects == nil && heads == nil && request.ObjectRoot != nil && request.HeadRoot != nil && request.ObjectRoot != request.HeadRoot
	if injected == rooted {
		return censussourcedriver.Result{Stop: censussourcedriver.StopFailed, State: initial, Err: censussourcedriver.ErrInvalid}, nil
	}
	if rooted {
		var err error
		objects, err = NewObjects(request.ObjectRoot)
		if err != nil {
			return censussourcedriver.Result{Stop: censussourcedriver.StopFailed, State: initial, Err: censussourcedriver.ErrInvalid}, nil
		}
		heads, err = NewHeads(request.HeadRoot)
		if err != nil {
			return censussourcedriver.Result{Stop: censussourcedriver.StopFailed, State: initial, Err: censussourcedriver.ErrInvalid}, nil
		}
	}

	rawHead, err := heads.Load(context.WithoutCancel(ctx), request.HeadKey)
	if err != nil {
		return censussourcedriver.Result{Stop: censussourcedriver.StopFailed, State: initial, Err: censussourcedriver.ErrInvalid}, nil
	}
	inspection, err := censussourcecontinuation.InspectHead(rawHead, request.WorkflowKey, request.Plan.Identity, partitionCount)
	if err != nil {
		return censussourcedriver.Result{Stop: censussourcedriver.StopFailed, State: initial, Err: censussourcedriver.ErrInvalid}, nil
	}
	initial.CommittedPrefix = inspection.CommittedPrefix
	if inspection.State == censussourcecontinuation.HeadStateClaimed && inspection.ReceiptSelector == "" {
		initial.Status = censussourcedriver.StatusIndeterminateExecution
		return censussourcedriver.Result{Stop: censussourcedriver.StopIndeterminateExecution, State: initial}, nil
	}

	var last *TerminalRecord
	result := censussourcedriver.Repeat(ctx, censussourcedriver.Request{
		Initial: initial,
		Bound:   request.StepBound,
		Step: func(stepCtx context.Context, current censussourcedriver.State) (censussourcedriver.State, error) {
			var candidate *TerminalRecord
			var callbackErr error
			stepResult, stepErr := censussourcedriver.Step(stepCtx, censussourcedriver.StepRequest{
				Plan:              request.Plan,
				PartitionSize:     request.PartitionSize,
				WorkflowKey:       request.WorkflowKey,
				HeadKey:           request.HeadKey,
				Objects:           objects,
				Heads:             heads,
				CoordinateForTest: request.CoordinateForTest,
				Discover: func(discoverCtx context.Context, ordinal int, members []censusacquisition.SourceFile) (censussourcecontinuation.Terminal, error) {
					discovery, err := request.Discover(discoverCtx, ordinal, cloneSlice(members))
					if err != nil {
						callbackErr = err
						return censussourcecontinuation.Terminal{}, err
					}
					record, err := NewTerminalRecord(request.Plan, request.PartitionSize, ordinal, discovery)
					if err != nil {
						callbackErr = err
						return censussourcecontinuation.Terminal{}, err
					}
					terminal, err := AdaptTerminal(request.Plan, request.PartitionSize, record)
					if err != nil {
						callbackErr = err
						return censussourcecontinuation.Terminal{}, err
					}
					candidate = record
					return terminal, nil
				},
			})

			if callbackErr != nil {
				return current, callbackErr
			}
			continuation := stepResult.Continuation
			next := censussourcedriver.State{
				CommittedPrefix:   continuation.CommittedPrefix,
				PartitionCount:    partitionCount,
				PlanIdentity:      request.Plan.Identity,
				SessionID:         current.SessionID,
				ManagerGeneration: current.ManagerGeneration,
			}
			switch continuation.Status {
			case censussourcecontinuation.StatusCommitted:
				next.Status = censussourcedriver.StatusCommitted
				if stepErr != nil {
					// A receipt-linked resume can advance authoritative custody without
					// invoking discovery in this process.
					if stepResult.SelectedOrdinal >= 0 || next.CommittedPrefix <= current.CommittedPrefix {
						return current, stepErr
					}
				} else if candidate != nil {
					last = candidate
				}
				return next, nil
			case censussourcecontinuation.StatusConflict:
				next.Status = censussourcedriver.StatusConflict
				return next, nil
			case censussourcecontinuation.StatusRetained:
				next.Status = censussourcedriver.StatusRetained
				return next, nil
			case censussourcecontinuation.StatusIndeterminateExecution:
				next.Status = censussourcedriver.StatusIndeterminateExecution
				return next, nil
			default:
				if stepErr != nil {
					return current, stepErr
				}
				return current, censussourcedriver.ErrInvalid
			}
		},
	})
	if result.Stop == censussourcedriver.StopFailed {
		return result, nil
	}
	return result, last
}
