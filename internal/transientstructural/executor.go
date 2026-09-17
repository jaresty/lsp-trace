package transientstructural

import (
	"context"
	"sort"
	"time"

	"lsp-trace/incomingops"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/session"
	"lsp-trace/internal/slicer"
	"lsp-trace/internal/traverse"
	"lsp-trace/sessionruntime"
)

// executionHooks is an unexported test seam for inducing lifecycle transitions
// at exact phase boundaries. Production callers can only use Execute, which
// always supplies the zero value and therefore cannot observe execution state.
type executionHooks struct {
	afterPhase func(Phase)
}

func (h executionHooks) after(phase Phase) {
	if h.afterPhase != nil {
		h.afterPhase(phase)
	}
}

// Execute is the sole production entry point for transient structural analysis.
// It reads one exact managed session generation and returns no retained artifact.
func Execute(parent context.Context, runtime *sessionruntime.Manager, request Request) (Result, *DomainFailure) {
	return execute(parent, runtime, request, executionHooks{})
}

func execute(parent context.Context, runtime *sessionruntime.Manager, request Request, hooks executionHooks) (Result, *DomainFailure) {
	if parent == nil {
		parent = context.Background()
	}
	if state := validateRequest(request); state != "" {
		return Result{}, fail(PhasePreflight, state, Accounting{})
	}
	if err := parent.Err(); err != nil {
		return Result{}, fail(PhasePreflight, terminalForContext(parent), Accounting{})
	}
	if runtime == nil {
		return Result{}, fail(PhasePreflight, StateInvalidServerResponse, Accounting{})
	}

	sessionID, state := resolveQualifiedSession(runtime.Records(), request.SessionID, request.Generation)
	if state != "" {
		return Result{}, fail(PhasePreflight, state, Accounting{})
	}
	metadata, failure := runtime.Metadata(sessionID, request.Generation)
	if failure != "" {
		return Result{}, fail(PhasePreflight, terminalForSessionFailure(failure), Accounting{})
	}
	if !metadata.CallHierarchySupport || (request.Target.Symbol != "" && !metadata.DocumentSymbolSupport) {
		return Result{}, fail(PhasePreflight, StateUnsupported, Accounting{})
	}
	if !supportedEncoding(metadata.PositionEncoding) {
		return Result{}, fail(PhasePreflight, StateUnsupported, Accounting{})
	}
	hooks.after(PhasePreflight)
	if state := metadataRecheck(runtime, sessionID, request.Generation, metadata); state != "" {
		return Result{}, fail(PhasePreflight, state, Accounting{})
	}

	ctx, cancel := context.WithTimeout(parent, time.Duration(request.TimeoutMS)*time.Millisecond)
	defer cancel()
	document := runtime.PrepareDocument(ctx, sessionruntime.DocumentRequest{
		SessionID: sessionID, Generation: request.Generation, URI: request.Target.URI,
		LanguageID: request.LanguageID, CaptureSupply: request.CaptureSupply,
	})
	if document.Failure != "" {
		return Result{}, fail(PhaseTraversal, terminalForSessionFailure(document.Failure), Accounting{})
	}
	if err := ctx.Err(); err != nil {
		return Result{}, fail(PhaseTraversal, terminalForContext(ctx), Accounting{})
	}

	counted := &countingRuntime{manager: runtime}
	client := incomingops.NewSessionClientWithWireLimits(counted, sessionID, request.Generation,
		time.Duration(request.RequestTimeoutMS)*time.Millisecond,
		incomingops.WireLimits{MaxMessages: request.MaxMessages, MaxBytes: request.MaxBytes})
	prepared, targetFailure := incomingops.ResolvePreparedTarget(ctx, client, request.Target.URI, request.Target.Symbol, request.Target.Line, request.Target.Character)
	if targetFailure != nil {
		accounting := counted.snapshot()
		failure := fail(PhasePreflight, targetResolutionState(ctx, targetFailure.Code, accounting, runtime, sessionID, request.Generation), accounting)
		failure.TargetDiagnostic = &TargetDiagnostic{ExactMatches: prepared.ExactMatches, TotalSymbols: prepared.TotalSymbols, OmittedSymbols: prepared.OmittedSymbols, Action: TargetAction(prepared.Action)}
		return Result{}, failure
	}
	items := prepared.Items
	root := graphNode(items[0]).ID

	down := slicer.DiscoverPrepared(ctx, client, items, slicer.Options{DownDepth: request.DownDepth, MaxNodes: request.MaxNodes})
	up := graph.Result{Nodes: []graph.Node{graphNode(items[0])}, Summary: graph.Summary{Complete: true}}
	if request.UpDepth > 0 {
		up = traverse.IncomingPrepared(ctx, client, items, traverse.Options{
			MaxDepth: request.UpDepth, MaxNodes: request.MaxNodes, IncludeTopmostSiblings: false,
		})
	}
	accounting := counted.snapshot()
	rawNodes := append(append([]graph.Node(nil), down.Nodes...), up.Nodes...)
	rawEdges := append(append([]graph.Edge(nil), down.Edges...), up.Edges...)
	if err := ctx.Err(); err != nil {
		state := terminalForContext(ctx)
		accounting = accountUnadmitted(accounting, rawNodes, rawEdges, omissionForTerminal(state))
		return Result{}, fail(PhaseTraversal, state, accounting)
	}
	upComplete := incomingCompleteWithinRequestedDepth(up)
	if !down.Complete || !down.TraversalComplete || !upComplete {
		state := traversalState(ctx, accounting, runtime, sessionID, request.Generation)
		if down.Truncated || (up.Summary.Truncated && !onlyRequestedDepthBoundary(up)) {
			state = StateTruncated
		}
		accounting = accountUnadmitted(accounting, rawNodes, rawEdges, omissionForTerminal(state))
		failure := fail(PhaseTraversal, state, accounting)
		if state == StateInvalidServerResponse {
			if !down.Complete || !down.TraversalComplete {
				failure.TraversalDiagnostic = &TraversalDiagnostic{Stage: TraversalStageOutgoing, Method: "callHierarchy/outgoingCalls", Direction: DirectionOutgoing}
			} else if !upComplete {
				failure.TraversalDiagnostic = &TraversalDiagnostic{Stage: TraversalStageIncoming, Method: "callHierarchy/incomingCalls", Direction: DirectionIncoming}
			}
		}
		return Result{}, failure
	}

	hooks.after(PhaseTraversal)
	if state := metadataRecheck(runtime, sessionID, request.Generation, metadata); state != "" {
		return Result{}, fail(PhaseTraversal, state, accounting)
	}
	bounds := bindBounds(request)
	projection, projectionErr := project(sessionID, request.Generation,
		traversalProjection{nodes: down.Nodes, edges: down.Edges, root: root},
		traversalProjection{nodes: up.Nodes, edges: up.Edges, root: root}, bounds, accounting)
	if projectionErr != nil {
		state := StateInvalidServerResponse
		if hasOmission(projection.accounting, OmissionNodeBound) {
			state = StateResourceLimit
		}
		return Result{}, fail(PhaseAdmission, state, projection.accounting)
	}
	accounting = projection.accounting
	if !reconciles(accounting) || accounting.Frontier.Unexpanded != 0 || hasNonDuplicateOmission(accounting) {
		return Result{}, fail(PhaseAdmission, StateInvalidServerResponse, accounting)
	}
	hooks.after(PhaseAdmission)
	if state := metadataRecheck(runtime, sessionID, request.Generation, metadata); state != "" {
		return Result{}, fail(PhaseAdmission, state, accounting)
	}

	analysis := analyze(request.Analysis, projection, bounds)
	hooks.after(PhaseAnalysis)
	if err := ctx.Err(); err != nil {
		return Result{}, fail(PhaseAnalysis, terminalForContext(ctx), accounting)
	}
	if state := metadataRecheck(runtime, sessionID, request.Generation, metadata); state != "" {
		return Result{}, fail(PhaseAnalysis, state, accounting)
	}

	hooks.after(PhaseDeliveryCheck)
	if state := metadataRecheck(runtime, sessionID, request.Generation, metadata); state != "" {
		return Result{}, fail(PhaseDeliveryCheck, state, accounting)
	}
	state = StateComplete
	if accounting.Occurrences.Admitted == 0 {
		state = StateEmpty
	}
	policy := frozenPolicyBinding()
	return Result{
		SchemaVersion: resultSchemaVersion, EvidenceClass: evidenceClassTransientLive, Authority: 0, SourceGraphComplete: sourceGraphCompleteUnknown,
		Retained: false, Replayable: false, PublicationEligible: false, HydrationEligible: false, ClaimCeiling: claimCeiling,
		Phase: PhaseDeliveryCheck, State: state,
		Qualification: Qualification{SessionID: sessionID, Generation: request.Generation, PositionEncoding: metadata.PositionEncoding},
		TargetID:      rootOpaque(projection), GraphDigest: graphDigest(projection, policy, bounds), Policy: policy, Bounds: bounds,
		Accounting: accounting, Analysis: analysis, SourceSupply: document.Supply,
	}, nil
}

func validateRequest(request Request) TerminalState {
	if request.SessionID == "" || request.Generation == 0 || request.Target.URI == "" || request.UpDepth < 0 || request.UpDepth > 64 || request.DownDepth < 0 || request.DownDepth > 64 ||
		request.MaxNodes < 1 || request.MaxNodes > 10000 || request.TimeoutMS < 1 || request.TimeoutMS > 60000 || request.RequestTimeoutMS < 1 || request.RequestTimeoutMS > 60000 ||
		request.MaxMessages < 1 || request.MaxMessages > 4096 || request.MaxBytes < 1 || request.MaxBytes > 16777216 {
		return StateInvalidServerResponse
	}
	position := request.Target.Line != nil || request.Target.Character != nil
	if request.Target.Symbol == "" {
		if request.Target.Line == nil || request.Target.Character == nil {
			return StateInvalidServerResponse
		}
	} else if position {
		return StateInvalidServerResponse
	}
	switch request.Analysis.Kind {
	case AnalysisNeighborhood:
		if request.Analysis.Direction != "" || request.Analysis.MaxDepth != 0 {
			return StateInvalidServerResponse
		}
	case AnalysisImpact:
		if request.Analysis.MaxDepth < 0 {
			return StateInvalidServerResponse
		}
		switch request.Analysis.Direction {
		case DirectionIncoming:
			if request.Analysis.MaxDepth > request.UpDepth {
				return StateInvalidServerResponse
			}
		case DirectionOutgoing:
			if request.Analysis.MaxDepth > request.DownDepth {
				return StateInvalidServerResponse
			}
		default:
			return StateInvalidServerResponse
		}
	default:
		return StateInvalidServerResponse
	}
	return ""
}

func resolveQualifiedSession(records []sessionruntime.Record, selector string, generation uint64) (string, TerminalState) {
	sort.Slice(records, func(i, j int) bool { return records[i].SessionID < records[j].SessionID })
	var exact *sessionruntime.Record
	for i := range records {
		if records[i].SessionID == selector {
			exact = &records[i]
			break
		}
	}
	if exact != nil {
		return qualifyRecord(*exact, generation)
	}
	var aliases []sessionruntime.Record
	for _, record := range records {
		if record.Routing.Alias == selector {
			aliases = append(aliases, record)
		}
	}
	if len(aliases) != 1 {
		return "", StateInvalidServerResponse
	}
	return qualifyRecord(aliases[0], generation)
}

func qualifyRecord(record sessionruntime.Record, generation uint64) (string, TerminalState) {
	if record.Generation != generation {
		return "", StateGenerationChanged
	}
	if record.State != session.Ready {
		return "", StateCancelled
	}
	return record.SessionID, ""
}

func supportedEncoding(encoding string) bool {
	return encoding == "utf-8" || encoding == "utf-16" || encoding == "utf-32"
}

func incomingCompleteWithinRequestedDepth(result graph.Result) bool {
	return result.Summary.Complete || onlyRequestedDepthBoundary(result)
}

func onlyRequestedDepthBoundary(result graph.Result) bool {
	if !result.Summary.Truncated || len(result.Frontier) == 0 {
		return false
	}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Phase != "traverse" || diagnostic.Method != "callHierarchy/incomingCalls" || diagnostic.Message != "SERVER_CALL_SITE_OUTSIDE_CALLER_RANGE" {
			return false
		}
	}
	for _, boundary := range result.Frontier {
		if boundary.Reason != graph.MaxDepth {
			return false
		}
	}
	for _, boundary := range result.Terminals {
		switch boundary.Reason {
		case graph.NoIncomingCalls, graph.ServerReportedNoIncoming, graph.PrepareReturnedNoItem, graph.IncomingReturnedNull, graph.ExternalURI:
		default:
			return false
		}
	}
	return true
}

func metadataRecheck(runtime *sessionruntime.Manager, sessionID string, generation uint64, before sessionruntime.SessionMetadata) TerminalState {
	after, failure := runtime.Metadata(sessionID, generation)
	if failure != "" {
		return terminalForSessionFailure(failure)
	}
	if after != before {
		return StateGenerationChanged
	}
	return ""
}

func targetResolutionState(ctx context.Context, code string, accounting Accounting, runtime *sessionruntime.Manager, sessionID string, generation uint64) TerminalState {
	switch code {
	case "DOCUMENT_SYMBOL_ABSENT":
		return StateTargetNotFound
	case "DOCUMENT_SYMBOL_AMBIGUOUS", "DOCUMENT_SYMBOL_PREPARE_MISMATCH":
		return StateAmbiguousTarget
	case "DOCUMENT_SYMBOL_UNSUPPORTED":
		return StateUnsupported
	default:
		state := traversalState(ctx, accounting, runtime, sessionID, generation)
		switch state {
		case StateResourceLimit, StateTimeout, StateCancelled, StateGenerationChanged:
			return state
		default:
			return StateInvalidServerResponse
		}
	}
}

func traversalState(ctx context.Context, accounting Accounting, runtime *sessionruntime.Manager, sessionID string, generation uint64) TerminalState {
	if ctx.Err() != nil {
		return terminalForContext(ctx)
	}
	if _, failure := runtime.Metadata(sessionID, generation); failure != "" {
		return terminalForSessionFailure(failure)
	}
	if accounting.Requests.Succeeded > 0 && (accounting.Requests.Failed > 0 || accounting.Requests.Cancelled > 0) {
		return StatePartial
	}
	for _, omission := range accounting.Omissions {
		switch omission.Reason {
		case OmissionTimeout:
			return StateTimeout
		case OmissionCancellation:
			return StateCancelled
		case OmissionRequestBound, OmissionNodeBound:
			return StateResourceLimit
		}
	}
	return StateInvalidServerResponse
}

func omissionForTerminal(state TerminalState) OmissionReason {
	switch state {
	case StateTimeout:
		return OmissionTimeout
	case StateCancelled, StateGenerationChanged:
		return OmissionCancellation
	case StateResourceLimit:
		return OmissionRequestBound
	case StateTruncated:
		return OmissionNodeBound
	case StateUnsupported:
		return OmissionUnsupported
	default:
		return OmissionInvalidResponse
	}
}

func hasOmission(accounting Accounting, reason OmissionReason) bool {
	for _, omission := range accounting.Omissions {
		if omission.Reason == reason && omission.Count > 0 {
			return true
		}
	}
	return false
}

func bindBounds(request Request) BoundsBinding {
	return BoundsBinding{
		UpDepth: request.UpDepth, DownDepth: request.DownDepth, MaxNodes: request.MaxNodes,
		TimeoutMS: request.TimeoutMS, RequestTimeoutMS: request.RequestTimeoutMS, MaxMessages: request.MaxMessages, MaxBytes: request.MaxBytes,
		AnalysisKind: request.Analysis.Kind, Direction: request.Analysis.Direction, AnalysisMaxDepth: request.Analysis.MaxDepth,
	}
}

func graphNode(item lsp.CallHierarchyItem) graph.Node {
	convertPosition := func(position lsp.Position) graph.Position {
		return graph.Position{Line: position.Line, Character: position.Character}
	}
	convertRange := func(r lsp.Range) graph.Range {
		return graph.Range{Start: convertPosition(r.Start), End: convertPosition(r.End)}
	}
	return graph.NewNode(graph.Item{Name: item.Name, Kind: item.Kind, Detail: item.Detail, URI: item.URI, Range: convertRange(item.Range), SelectionRange: convertRange(item.SelectionRange), Data: item.Data})
}

func rootOpaque(projection admittedProjection) string { return projection.targetID }

func legalTerminalPair(phase Phase, state TerminalState) bool {
	switch phase {
	case PhasePreflight:
		return state == StateUnsupported || state == StateAmbiguousTarget || state == StateTargetNotFound || state == StateResourceLimit || state == StateTimeout || state == StateCancelled || state == StateGenerationChanged || state == StateInvalidServerResponse
	case PhaseTraversal:
		return state == StatePartial || state == StateTruncated || state == StateResourceLimit || state == StateTimeout || state == StateCancelled || state == StateGenerationChanged || state == StateInvalidServerResponse
	case PhaseAdmission:
		return state == StateResourceLimit || state == StateCancelled || state == StateGenerationChanged || state == StateInvalidServerResponse
	case PhaseAnalysis:
		return state == StateResourceLimit || state == StateTimeout || state == StateCancelled || state == StateGenerationChanged || state == StateAnalysisFailed
	case PhaseDeliveryCheck:
		return state == StateComplete || state == StateEmpty || state == StateCancelled || state == StateGenerationChanged
	default:
		return false
	}
}

func fail(phase Phase, state TerminalState, accounting Accounting) *DomainFailure {
	if state == StateComplete || state == StateEmpty || !legalTerminalPair(phase, state) {
		panic("illegal transient structural phase/state pair")
	}
	return &DomainFailure{Phase: phase, State: state, Accounting: accounting}
}
