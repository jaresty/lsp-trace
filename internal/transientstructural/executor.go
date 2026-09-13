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

// Execute is the sole production entry point for transient structural analysis.
// It reads one exact managed session generation and returns no retained artifact.
func Execute(parent context.Context, runtime *sessionruntime.Manager, request Request) (Result, *DomainFailure) {
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

	ctx, cancel := context.WithTimeout(parent, time.Duration(request.TimeoutMS)*time.Millisecond)
	defer cancel()
	document := runtime.PrepareDocument(ctx, sessionruntime.DocumentRequest{
		SessionID: sessionID, Generation: request.Generation, URI: request.Target.URI,
		LanguageID: request.LanguageID, CaptureSupply: false,
	})
	if document.Failure != "" {
		return Result{}, fail(PhaseAcquisition, terminalForSessionFailure(document.Failure), Accounting{})
	}
	if err := ctx.Err(); err != nil {
		return Result{}, fail(PhaseAcquisition, terminalForContext(ctx), Accounting{})
	}

	counted := &countingRuntime{manager: runtime}
	client := incomingops.NewSessionClientWithWireLimits(counted, sessionID, request.Generation,
		time.Duration(request.RequestTimeoutMS)*time.Millisecond,
		incomingops.WireLimits{MaxMessages: request.MaxMessages, MaxBytes: request.MaxBytes})
	line, character, targetFailure := incomingops.ResolveTarget(ctx, client, request.Target.URI, request.Target.Symbol, request.Target.Line, request.Target.Character)
	if targetFailure != nil {
		return Result{}, fail(PhaseAcquisition, acquisitionState(ctx, counted.snapshot(), runtime, sessionID, request.Generation), counted.snapshot())
	}

	items, prepareErr := client.PrepareCallHierarchy(ctx, lsp.PrepareCallHierarchyParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: request.Target.URI}, Position: lsp.Position{Line: line, Character: character},
	})
	if prepareErr != nil {
		accounting := counted.snapshot()
		return Result{}, fail(PhaseAcquisition, acquisitionState(ctx, accounting, runtime, sessionID, request.Generation), accounting)
	}
	if len(items) == 0 {
		accounting := counted.snapshot()
		return Result{}, fail(PhaseAcquisition, StateInvalidServerResponse, accounting)
	}
	if len(items) != 1 {
		accounting := counted.snapshot()
		return Result{}, fail(PhaseAcquisition, StateInvalidServerResponse, accounting)
	}
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
		return Result{}, fail(PhaseAcquisition, state, accounting)
	}
	upComplete := incomingCompleteWithinRequestedDepth(up)
	if !down.Complete || !down.TraversalComplete || !upComplete {
		state := acquisitionState(ctx, accounting, runtime, sessionID, request.Generation)
		if down.Truncated || (up.Summary.Truncated && !onlyRequestedDepthBoundary(up)) {
			state = StateResourceLimit
			addOmission(&accounting, OmissionNodeBound, 1)
		}
		accounting = accountUnadmitted(accounting, rawNodes, rawEdges, omissionForTerminal(state))
		return Result{}, fail(PhaseAcquisition, state, accounting)
	}

	if state := metadataRecheck(runtime, sessionID, request.Generation, metadata); state != "" {
		addOmission(&accounting, omissionForTerminal(state), 1)
		return Result{}, fail(PhaseReconciliation, state, accounting)
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
		return Result{}, fail(PhaseReconciliation, state, projection.accounting)
	}
	accounting = projection.accounting
	if !reconciles(accounting) || accounting.Frontier.Unexpanded != 0 || hasNonDuplicateOmission(accounting) {
		return Result{}, fail(PhaseReconciliation, StateInvalidServerResponse, accounting)
	}

	if state := metadataRecheck(runtime, sessionID, request.Generation, metadata); state != "" {
		addOmission(&accounting, omissionForTerminal(state), 1)
		return Result{}, fail(PhaseAnalysis, state, accounting)
	}
	analysis := analyze(request.Analysis, projection, bounds)
	if err := ctx.Err(); err != nil {
		return Result{}, fail(PhaseAnalysis, terminalForContext(ctx), accounting)
	}

	if state := metadataRecheck(runtime, sessionID, request.Generation, metadata); state != "" {
		addOmission(&accounting, omissionForTerminal(state), 1)
		return Result{}, fail(PhaseDelivery, state, accounting)
	}
	if err := ctx.Err(); err != nil {
		return Result{}, fail(PhaseDelivery, terminalForContext(ctx), accounting)
	}
	return Result{
		Phase: PhaseDelivery, State: StateComplete,
		Qualification: Qualification{SessionID: sessionID, Generation: request.Generation, PositionEncoding: metadata.PositionEncoding},
		TargetID:      rootOpaque(projection), GraphDigest: graphDigest(projection, frozenPolicy, bounds), Policy: frozenPolicy, Bounds: bounds,
		Accounting: accounting, Analysis: analysis,
		Claims: ClaimBoundary{Transient: true, Retained: false, Replayable: false, Authoritative: false, ClaimCeiling: claimCeiling},
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
	if !result.Summary.Truncated || len(result.Frontier) == 0 || len(result.Diagnostics) != 0 {
		return false
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

func acquisitionState(ctx context.Context, accounting Accounting, runtime *sessionruntime.Manager, sessionID string, generation uint64) TerminalState {
	if ctx.Err() != nil {
		return terminalForContext(ctx)
	}
	if _, failure := runtime.Metadata(sessionID, generation); failure != "" {
		return terminalForSessionFailure(failure)
	}
	for _, omission := range accounting.Omissions {
		switch omission.Reason {
		case OmissionTimeout:
			return StateTimeout
		case OmissionCancellation:
			return StateCancelled
		case OmissionGenerationChange:
			return StateGenerationChanged
		case OmissionRequestLimit, OmissionResponseLimit, OmissionNodeBound:
			return StateResourceLimit
		}
	}
	return StateInvalidServerResponse
}

func omissionForTerminal(state TerminalState) OmissionReason {
	switch state {
	case StateTimeout:
		return OmissionTimeout
	case StateCancelled:
		return OmissionCancellation
	case StateGenerationChanged:
		return OmissionGenerationChange
	case StateResourceLimit:
		return OmissionRequestLimit
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

func fail(phase Phase, state TerminalState, accounting Accounting) *DomainFailure {
	return &DomainFailure{Phase: phase, State: state, Accounting: accounting}
}
