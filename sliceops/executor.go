// Package sliceops adapts an exact managed session generation to the existing
// deterministic slice and incoming traversal implementations.
package sliceops

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"sort"
	"time"

	"lsp-trace/incomingops"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/session"
	"lsp-trace/internal/slicer"
	"lsp-trace/internal/traverse"
	"lsp-trace/sessionruntime"
)

const OperationSlice operation.Name = "slice"

type Runtime interface {
	Metadata(string, uint64) (sessionruntime.SessionMetadata, session.Failure)
	RoundTrip(context.Context, sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult
	Records() []sessionruntime.Record
}

type Executor struct{ runtime Runtime }

func NewExecutor(runtime Runtime) *Executor { return &Executor{runtime: runtime} }

type request struct {
	SessionID        string  `json:"session_id"`
	Generation       uint64  `json:"generation"`
	StartMode        string  `json:"start_mode"`
	URI              string  `json:"uri"`
	Line             *uint32 `json:"line"`
	Character        *uint32 `json:"character"`
	Symbol           string  `json:"symbol"`
	DownDepth        int     `json:"down_depth"`
	UpDepth          int     `json:"up_depth"`
	MaxNodes         int     `json:"max_nodes"`
	MaxMessages      int     `json:"max_messages"`
	MaxBytes         int     `json:"max_bytes"`
	TimeoutMS        int64   `json:"timeout_ms"`
	RequestTimeoutMS int64   `json:"request_timeout_ms"`
}

func (e *Executor) Execute(parent context.Context, op operation.Request) (operation.Result, *operation.Failure) {
	if e == nil || e.runtime == nil || op.Name != OperationSlice {
		return operation.Result{}, fail(operation.FailureNotImplemented, operation.ErrNotImplemented)
	}
	var in request
	if err := decodeClosed(op.Input, &in); err != nil {
		return operation.Result{}, fail(operation.FailureInvalidInput, err)
	}
	applyDefaults(&in)
	if err := validate(in); err != nil {
		return operation.Result{}, fail(operation.FailureInvalidInput, err)
	}
	resolvedID, resolvedGeneration, runtimeFailure := incomingops.ResolveSession(e.runtime, in.SessionID, in.Generation)
	in.SessionID, in.Generation = resolvedID, resolvedGeneration
	if runtimeFailure != "" {
		return operation.Result{}, fail(string(runtimeFailure), nil)
	}
	metadata, runtimeFailure := e.runtime.Metadata(in.SessionID, in.Generation)
	if runtimeFailure != "" {
		return operation.Result{}, fail(string(runtimeFailure), nil)
	}
	if !metadata.CallHierarchySupport {
		return operation.Result{}, fail(string(graph.UnsupportedCallHierarchy), nil)
	}
	switch metadata.PositionEncoding {
	case "utf-8", "utf-16", "utf-32":
	default:
		return operation.Result{}, fail("UNSUPPORTED_POSITION_ENCODING", fmt.Errorf("retained position encoding %q is unsupported", metadata.PositionEncoding))
	}
	ctx, cancel := context.WithTimeout(parent, time.Duration(in.TimeoutMS)*time.Millisecond)
	defer cancel()
	client := incomingops.NewSessionClientWithWireLimits(e.runtime, in.SessionID, in.Generation, time.Duration(in.RequestTimeoutMS)*time.Millisecond, incomingops.WireLimits{MaxMessages: in.MaxMessages, MaxBytes: int64(in.MaxBytes)})
	line, character, targetFailure := incomingops.ResolveTarget(ctx, client, in.URI, in.Symbol, in.Line, in.Character)
	if targetFailure != nil {
		return operation.Result{}, targetFailure
	}
	in.Line, in.Character = &line, &character
	params := lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: in.URI}, Position: lsp.Position{Line: line, Character: character}}
	prepared, err := client.PrepareCallHierarchy(ctx, params)
	if err != nil {
		if code := terminalCode(err); code != "" {
			return operation.Result{}, fail(code, err)
		}
		return artifact(partialPrepare(in, metadata, err))
	}
	if len(prepared) == 0 {
		return artifact(emptyResult(in, metadata))
	}
	discovery := slicer.DiscoverPrepared(ctx, client, prepared, slicer.Options{DownDepth: in.DownDepth, MaxNodes: in.MaxNodes})
	down := graph.Result{SchemaVersion: graph.SchemaVersionV3, Nodes: discovery.Nodes, Edges: discovery.Edges, Diagnostics: discovery.Diagnostics, Summary: graph.Summary{Complete: discovery.Complete, Truncated: discovery.Truncated}}
	down.Canonicalize()
	up := graph.Result{SchemaVersion: graph.SchemaVersionV3, Summary: graph.Summary{Complete: true}}
	if len(discovery.UpwardStartItems) > 0 {
		remaining := in.MaxNodes - len(discovery.Nodes)
		if remaining < 0 {
			remaining = 0
		}
		up = traverse.IncomingPrepared(ctx, client, discovery.UpwardStartItems, traverse.Options{MaxDepth: in.UpDepth, MaxNodes: len(discovery.UpwardStartItems) + remaining, SchemaVersion: graph.SchemaVersionV3})
	}
	graph.ReconcileIncomingAliases(down, &up)
	result := graph.MergeResults(down, up)
	decorate(&result, in, metadata, prepared, discovery)
	return artifact(result)
}

func decorate(result *graph.Result, in request, metadata sessionruntime.SessionMetadata, prepared []lsp.CallHierarchyItem, d slicer.Discovery) {
	layers := make([]graph.SliceLayer, len(d.Layers))
	for i, l := range d.Layers {
		layers[i] = graph.SliceLayer{Depth: l.Depth, NodeIDs: append([]string(nil), l.NodeIDs...)}
	}
	frontier := []string{}
	if in.DownDepth < len(layers) {
		frontier = append(frontier, layers[in.DownDepth].NodeIDs...)
	}
	outgoingRelations := make([]string, len(d.Edges))
	for i, e := range d.Edges {
		outgoingRelations[i] = e.RelationID
	}
	result.Slice = &graph.SliceEvidence{StartMode: in.StartMode, SourceURI: in.URI, DownDepth: in.DownDepth, UpDepth: in.UpDepth, StartingNodeIDs: append([]string(nil), d.StartNodeIDs...), Layers: layers, FrontierNodeIDs: frontier, OutgoingTerminalNodeIDs: itemIDs(d.OutgoingTerminalItems), UpwardStartNodeIDs: itemIDs(d.UpwardStartItems), OutgoingRelationIDs: outgoingRelations, TraversalComplete: d.TraversalComplete}
	preparedIDs := itemIDs(prepared)
	reachedNodes := make([]string, len(result.Nodes))
	for i, n := range result.Nodes {
		reachedNodes[i] = n.ID
	}
	reachedRelations := make([]string, len(result.Edges))
	for i, e := range result.Edges {
		reachedRelations[i] = e.RelationID
	}
	result.Seeds = []graph.SeedResult{{Label: "start", Requested: graph.Target{URI: in.URI, Line: int(*in.Line), Column: int(*in.Character)}, PreparedTargetIDs: preparedIDs, ReachedNodeIDs: reachedNodes, ReachedRelationIDs: reachedRelations, ReachedEdges: append([]graph.Edge(nil), result.Edges...)}}
	result.Invocation.Target = graph.Target{URI: in.URI, Line: int(*in.Line), Column: int(*in.Character)}
	result.Invocation.Seeds = []graph.InvocationSeed{{Label: "start", At: fmt.Sprintf("%s:%d:%d", in.URI, *in.Line, *in.Character), ResolvedURI: in.URI}}
	result.Invocation.Limits = graph.Limits{MaxDepth: in.UpDepth, MaxNodes: in.MaxNodes, TimeoutMS: in.TimeoutMS}
	result.Invocation.RequestTimeoutMS = in.RequestTimeoutMS
	result.Invocation.Concurrency = 1
	result.Capabilities.CallHierarchyProvider = metadata.CallHierarchySupport
	result.CapabilityQuality.Advertised = metadata.CallHierarchySupport
	result.Tool = graph.ToolIdentity{Name: "lsp-trace", Version: graph.Unknown}
	result.Canonicalize()
}

func emptyResult(in request, m sessionruntime.SessionMetadata) graph.Result {
	r := graph.Result{SchemaVersion: graph.SchemaVersionV3, Summary: graph.Summary{Complete: true}}
	decorate(&r, in, m, nil, slicer.Discovery{SourceURI: in.URI, Complete: true, TraversalComplete: true})
	return r
}
func partialPrepare(in request, m sessionruntime.SessionMetadata, err error) graph.Result {
	r := emptyResult(in, m)
	r.Summary.Complete = false
	r.Diagnostics = []graph.Diagnostic{{Phase: "slice-prepare", Method: "textDocument/prepareCallHierarchy", Message: err.Error()}}
	r.Canonicalize()
	return r
}
func artifact(result graph.Result) (operation.Result, *operation.Failure) {
	raw, err := json.Marshal(result)
	if err != nil {
		return operation.Result{}, fail(operation.FailureInternal, err)
	}
	return operation.Result{Artifact: append(raw, '\n')}, nil
}
func itemIDs(items []lsp.CallHierarchyItem) []string {
	ids := make([]string, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		n := graph.NewNode(graph.Item{Name: item.Name, Kind: item.Kind, Detail: item.Detail, URI: item.URI, Range: toRange(item.Range), SelectionRange: toRange(item.SelectionRange), Data: item.Data})
		if !seen[n.ID] {
			seen[n.ID] = true
			ids = append(ids, n.ID)
		}
	}
	sort.Strings(ids)
	return ids
}
func toRange(r lsp.Range) graph.Range {
	return graph.Range{Start: graph.Position{Line: r.Start.Line, Character: r.Start.Character}, End: graph.Position{Line: r.End.Line, Character: r.End.Character}}
}
func terminalCode(err error) string {
	if errors.Is(err, context.Canceled) {
		return string(session.RequestCancelled)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return string(session.RequestTimeout)
	}
	return ""
}
func applyDefaults(r *request) {
	if r.DownDepth == 0 {
		r.DownDepth = 2
	}
	if r.UpDepth == 0 {
		r.UpDepth = 2
	}
	if r.MaxNodes == 0 {
		r.MaxNodes = 100
	}
	if r.MaxMessages == 0 {
		r.MaxMessages = 64
	}
	if r.MaxBytes == 0 {
		r.MaxBytes = 4 << 20
	}
	if r.TimeoutMS == 0 {
		r.TimeoutMS = 5000
	}
	if r.RequestTimeoutMS == 0 {
		r.RequestTimeoutMS = 1000
	}
}

func validate(r request) error {
	if r.SessionID == "" || r.URI == "" || r.StartMode != "at" {
		return errors.New("session_id, start_mode=at, and uri are required")
	}
	position := r.Line != nil && r.Character != nil
	partialPosition := (r.Line == nil) != (r.Character == nil)
	if partialPosition || position == (r.Symbol != "") {
		return errors.New("exactly one complete target selector is required: line+character or symbol")
	}
	u, err := url.Parse(r.URI)
	if err != nil || !u.IsAbs() {
		return errors.New("uri must be absolute")
	}
	if r.DownDepth < 1 || r.DownDepth > 64 || r.UpDepth < 1 || r.UpDepth > 64 || r.MaxNodes < 1 || r.MaxNodes > 10000 || r.MaxMessages < 1 || r.MaxMessages > 4096 || r.MaxBytes < 1 || r.MaxBytes > 16<<20 || r.TimeoutMS < 1 || r.TimeoutMS > 60000 || r.RequestTimeoutMS < 1 || r.RequestTimeoutMS > 60000 {
		return errors.New("limits and deadlines are outside supported bounds")
	}
	return nil
}
func decodeClosed(raw json.RawMessage, target any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return errors.New("one JSON value required")
	}
	return nil
}
func fail(code string, err error) *operation.Failure { return &operation.Failure{Code: code, Err: err} }
