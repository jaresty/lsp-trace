package incomingops

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	compositionruntime "lsp-trace/internal/adr0011adr0007compositionruntime"
	"lsp-trace/internal/directionalraw"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/operation"
	"lsp-trace/sessionruntime"
)

var ErrBehaviorNotImplementedPrivate = errors.New("adr0011/adr0007 behavior not implemented")

type ADR0011ADR0007ProductionPrivate struct {
	executor    *Executor
	coordinator *compositionruntime.CoordinatorPrivate
}
type SelectorFactsPrivate struct{ ProviderRelations, GenericRelations uint64 }
type SelectorClassificationPrivate struct{ Family, Code string }
type RelationSelectorPrivate struct{ Name string }
type ProductionPrivateRequest struct {
	Operation  operation.Request
	Coordinate compositionruntime.CoordinateRequestPrivate
}
type ProductionPrivateResult struct {
	traversal      operation.Result
	outgoing       []lsp.CallHierarchyOutgoingCall
	coordination   compositionruntime.CoordinateResultPrivate
	sealedArtifact []byte
	failure        *operation.Failure
}

type productionPrivateFailureCause string

func (e productionPrivateFailureCause) Error() string { return string(e) }

func retainProductionPrivateFailure(failure *operation.Failure) *operation.Failure {
	normalized := operation.NormalizeFailure(failure)
	if normalized == nil || normalized.Err == nil {
		return normalized
	}
	normalized.Err = productionPrivateFailureCause(normalized.Err.Error())
	return normalized
}

func newProductionPrivateFailureResult(failure *operation.Failure) ProductionPrivateResult {
	return ProductionPrivateResult{failure: retainProductionPrivateFailure(failure)}
}

func cloneOutgoingPrivate(values []lsp.CallHierarchyOutgoingCall) []lsp.CallHierarchyOutgoingCall {
	out := make([]lsp.CallHierarchyOutgoingCall, len(values))
	for i := range values {
		out[i] = values[i]
		out[i].To.Tags = append([]int(nil), values[i].To.Tags...)
		out[i].To.Data = append([]byte(nil), values[i].To.Data...)
		out[i].FromRanges = append([]lsp.Range(nil), values[i].FromRanges...)
	}
	return out
}

func cloneIncomingPrivate(values []lsp.CallHierarchyIncomingCall) []lsp.CallHierarchyIncomingCall {
	out := make([]lsp.CallHierarchyIncomingCall, len(values))
	for i := range values {
		out[i] = values[i]
		out[i].From.Tags = append([]int(nil), values[i].From.Tags...)
		out[i].From.Data = append([]byte(nil), values[i].From.Data...)
		out[i].FromRanges = append([]lsp.Range(nil), values[i].FromRanges...)
	}
	return out
}
func (r ProductionPrivateResult) TraversalPrivate() operation.Result {
	out := r.traversal
	out.Artifact = append([]byte(nil), r.traversal.Artifact...)
	if incoming, ok := r.traversal.Value.([]lsp.CallHierarchyIncomingCall); ok {
		out.Value = cloneIncomingPrivate(incoming)
	}
	return out
}
func (r ProductionPrivateResult) OutgoingPrivate() []lsp.CallHierarchyOutgoingCall {
	return cloneOutgoingPrivate(r.outgoing)
}
func (r ProductionPrivateResult) CoordinationPrivate() compositionruntime.CoordinateResultPrivate {
	return r.coordination
}
func (r ProductionPrivateResult) SealedArtifactPrivate() []byte {
	return append([]byte(nil), r.sealedArtifact...)
}
func (r ProductionPrivateResult) FailurePrivate() *operation.Failure {
	return retainProductionPrivateFailure(r.failure)
}

func NewADR0011ADR0007ProductionPrivate(e *Executor) (*ADR0011ADR0007ProductionPrivate, error) {
	if e == nil {
		return nil, errors.New("nil executor")
	}
	c, err := compositionruntime.NewCoordinatorPrivate()
	if err != nil {
		return nil, err
	}
	return &ADR0011ADR0007ProductionPrivate{e, c}, nil
}
func (p *ADR0011ADR0007ProductionPrivate) ExecutePrivate(ctx context.Context, req ProductionPrivateRequest) (result ProductionPrivateResult, err error) {
	root := req.Coordinate.Root
	releaseDelegated := false
	defer func() {
		s, _ := root.SnapshotPrivate()
		if !s.Released && !releaseDelegated {
			if releaseErr := root.Release(); releaseErr != nil {
				result.failure = retainProductionPrivateFailure(failure("DIRECTIONAL_RELEASE_FAILED", releaseErr))
				err = errors.Join(err, releaseErr)
			}
			s, _ = root.SnapshotPrivate()
		}
		result.coordination.SetLeaseStatePrivate(compositionruntime.LeaseStatePrivate{Released: s.Released, RootReleaseCount: s.ReleaseCount, IncomingLive: s.IncomingLiveBytes, OutgoingLive: s.OutgoingLiveBytes, RootLive: s.LiveBytes, IncomingAdmitted: s.IncomingAdmittedBytes, OutgoingAdmitted: s.OutgoingAdmittedBytes, RootAdmitted: s.RootAdmittedBytes, IncomingReleased: s.IncomingReleasedBytes, OutgoingReleased: s.OutgoingReleasedBytes, RootReleased: s.RootReleasedBytes})
	}()
	fail := func(code, stage, direction string) ProductionPrivateResult {
		coord := compositionruntime.NewSuppressedCoordinateResultPrivate(compositionruntime.CandidateSuppressionPrivate{Code: code, Stage: stage, Direction: direction, Family: "GENERIC"})
		outer := code
		if code == "DIRECTION_LIMIT_EXCEEDED" {
			if direction == "incoming" {
				outer = "INCOMING_DIRECTION_LIMIT_EXCEEDED"
			} else {
				outer = "OUTGOING_DIRECTION_LIMIT_EXCEEDED"
			}
		}
		if code == "OUTGOING_RUNTIME_FAILED" {
			outer = "OUTGOING_CALLS_FAILED"
		}
		return ProductionPrivateResult{coordination: coord, failure: retainProductionPrivateFailure(failure(outer, errors.New(outer)))}
	}
	if err := ctx.Err(); err != nil {
		return fail("CANCELLED_BEFORE_DIRECTION", "incoming_preflight", "incoming"), nil
	}
	if err := root.PreflightDirectionPrivate("incoming"); err != nil {
		return fail("DIRECTION_LIMIT_EXCEEDED", "incoming_preflight", "incoming"), nil
	}
	if err := root.PreflightDirectionPrivate("outgoing"); err != nil {
		return fail("DIRECTION_LIMIT_EXCEEDED", "outgoing_preflight", "outgoing"), nil
	}
	var input request
	if err := decodeClosed(req.Operation.Input, &input); err != nil {
		return newProductionPrivateFailureResult(failure(operation.FailureInvalidInput, err)), nil
	}
	applyDefaults(&input)
	resolvedID, resolvedGeneration, rf := ResolveSession(p.executor.runtime, input.SessionID, input.Generation)
	if rf != "" {
		return newProductionPrivateFailureResult(failure(string(rf), nil)), nil
	}
	runtime, ok := p.executor.runtime.(interface {
		ExecuteCallHierarchyDirectionsPrivate(context.Context, sessionruntime.DirectionalCallHierarchyRequestPrivate) (sessionruntime.DirectionalCallHierarchyResultPrivate, error)
	})
	if !ok {
		return fail("INCOMING_CALLS_FAILED", "manager_directional_api", "incoming"), nil
	}
	wireCtx, err := root.Context(ctx)
	if err != nil {
		return ProductionPrivateResult{}, err
	}
	deadline := time.Now().Add(time.Duration(input.RequestTimeoutMS) * time.Millisecond)
	if callerDeadline, hasDeadline := ctx.Deadline(); hasDeadline && callerDeadline.Before(deadline) {
		deadline = callerDeadline
	}
	transfer := func(direction string, receipt sessionruntime.DirectionalWireReceiptPrivate, required bool) error {
		requestLength := receipt.RequestLengthPrivate()
		responseLength := receipt.ResponseLengthPrivate()
		wireBytes := requestLength + responseLength
		if wireBytes == 0 {
			if required {
				return errors.New("directional wire receipt unavailable")
			}
			return nil
		}
		child, childErr := root.ChildPrivate(direction)
		if childErr != nil {
			return childErr
		}
		if reserveErr := child.ReservePrivate(wireBytes); reserveErr != nil {
			return reserveErr
		}
		return child.RecordWireMetadataPrivate(requestLength, responseLength, receipt.RequestSHA256Private(), receipt.ResponseSHA256Private())
	}
	directional, directionalErr := runtime.ExecuteCallHierarchyDirectionsPrivate(wireCtx, sessionruntime.DirectionalCallHierarchyRequestPrivate{
		SessionID: resolvedID, Generation: resolvedGeneration, URI: input.URI, LanguageID: input.LanguageID,
		Symbol: input.Symbol, Line: input.Line, Character: input.Character, Deadline: deadline,
		MaxMessages: defaultMaxMessages, MaxBytes: int64(directionalraw.DirectionLimitPrivate),
	})
	if directionalErr != nil {
		if directional.Failure != "" {
			return fail(string(directional.Failure), "document_preparation", "incoming"), nil
		}
		if err := transfer("incoming", directional.IncomingWire, false); err != nil {
			return fail("INCOMING_CALLS_FAILED", "incoming_wire", "incoming"), nil
		}
		if err := transfer("outgoing", directional.OutgoingWire, false); err != nil {
			return fail("OUTGOING_RUNTIME_FAILED", "outgoing_wire", "outgoing"), nil
		}
		if errors.Is(directionalErr, context.Canceled) {
			if directional.IncomingWire.RequestLengthPrivate() == 0 {
				return fail("CANCELLED_BEFORE_DIRECTION", "incoming_preflight", "incoming"), nil
			}
			return fail("CANCELLED_BETWEEN_DIRECTIONS", "outgoing_preflight", "outgoing"), nil
		}
		if directional.IncomingWire.RequestLengthPrivate() == 0 {
			return fail("INCOMING_CALLS_FAILED", "incoming_traversal", "incoming"), nil
		}
		return fail("OUTGOING_RUNTIME_FAILED", "outgoing_traversal", "outgoing"), nil
	}
	if err := transfer("incoming", directional.IncomingWire, true); err != nil {
		return fail("INCOMING_CALLS_FAILED", "incoming_wire", "incoming"), nil
	}
	if err := transfer("outgoing", directional.OutgoingWire, true); err != nil {
		return fail("OUTGOING_RUNTIME_FAILED", "outgoing_wire", "outgoing"), nil
	}
	releaseDelegated = true
	coord, err := p.coordinator.CoordinatePrivate(ctx, req.Coordinate)
	if err != nil {
		out := ProductionPrivateResult{coordination: coord}
		if errors.Is(err, directionalraw.ErrTerminalReleasePrivate) {
			out.failure = retainProductionPrivateFailure(failure("DIRECTIONAL_RELEASE_FAILED", err))
		}
		return out, err
	}
	sealed, marshalErr := json.Marshal(coord)
	if marshalErr != nil {
		return ProductionPrivateResult{coordination: coord}, marshalErr
	}
	return ProductionPrivateResult{traversal: operation.Result{Value: cloneIncomingPrivate(directional.Incoming)}, outgoing: cloneOutgoingPrivate(directional.Outgoing), coordination: coord, sealedArtifact: append([]byte(nil), sealed...)}, nil
}
func NormalizeCallsOnlyPrivate(req operation.Request) ([]RelationSelectorPrivate, error) {
	var x struct {
		Relations []string `json:"relations"`
	}
	if err := json.Unmarshal(req.Input, &x); err != nil {
		return nil, err
	}
	if len(x.Relations) == 0 {
		return []RelationSelectorPrivate{{"CALLS"}}, nil
	}
	if len(x.Relations) != 1 || x.Relations[0] != "CALLS" {
		return nil, errors.New("CALLS-only selector required")
	}
	return []RelationSelectorPrivate{{"CALLS"}}, nil
}
func ClassifySelectorPrivate(f SelectorFactsPrivate) (SelectorClassificationPrivate, error) {
	switch {
	case f.ProviderRelations > 0 && f.GenericRelations > 0:
		return SelectorClassificationPrivate{"MIXED_PROVIDER_GENERIC", "MIXED_PROVIDER_GENERIC_SELECTOR"}, nil
	case f.ProviderRelations > 0:
		return SelectorClassificationPrivate{"PROVIDER_ONLY", "PROVIDER_ONLY_SELECTOR"}, nil
	case f.GenericRelations > 0:
		return SelectorClassificationPrivate{"GENERIC", "GENERIC_SELECTOR"}, nil
	default:
		return SelectorClassificationPrivate{}, errors.New("empty selector facts")
	}
}
