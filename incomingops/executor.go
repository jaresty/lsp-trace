// Package incomingops adapts exact managed session generations to the existing
// deterministic incoming traversal without owning MCP framing.
package incomingops

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/provider"
	"lsp-trace/internal/session"
	"lsp-trace/internal/traverse"
	"lsp-trace/sessionruntime"
)

const (
	OperationIncoming          operation.Name = "incoming"
	maxSymbolPrepareProbeDelta                = uint32(64)
	maxSymbolSuggestions                      = 8
	maxTargetDiagnosticCount                  = 10000
)

type Runtime interface {
	Metadata(string, uint64) (sessionruntime.SessionMetadata, session.Failure)
	RoundTrip(context.Context, sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult
	Records() []sessionruntime.Record
}

type documentRuntime interface {
	PrepareDocument(context.Context, sessionruntime.DocumentRequest) sessionruntime.DocumentResult
}

// SelectorRuntime may resolve host-owned aliases before exact runtime access.
type SelectorRuntime interface {
	ResolveSessionSelector(string, uint64) (string, uint64, session.Failure)
}

type Executor struct{ runtime Runtime }

func NewExecutor(runtime Runtime) *Executor { return &Executor{runtime: runtime} }

// RelationCollector is the minimal incoming-owned boundary to host-provisioned
// external relation providers. The provider artifact remains opaque so this
// package neither owns nor interprets framework semantics.
type RelationCollector interface {
	CollectRelations(context.Context, []string, json.RawMessage) (provider.Result, error)
}

type request struct {
	SessionID             string          `json:"session_id"`
	Generation            uint64          `json:"generation"`
	URI                   string          `json:"uri"`
	LanguageID            string          `json:"language_id,omitempty"`
	Line                  *uint32         `json:"line"`
	Character             *uint32         `json:"character"`
	Symbol                string          `json:"symbol"`
	MaxDepth              int             `json:"max_depth"`
	MaxNodes              int             `json:"max_nodes"`
	TimeoutMS             int64           `json:"timeout_ms"`
	RequestTimeoutMS      int64           `json:"request_timeout_ms"`
	Relations             *[]string       `json:"relations"`
	Languages             []string        `json:"languages"`
	Frameworks            []string        `json:"frameworks"`
	Adapters              json.RawMessage `json:"adapters"`
	Providers             []string        `json:"providers"`
	WorkspaceRevision     json.RawMessage `json:"workspace_revision"`
	FailOnUnknownRevision bool            `json:"fail_on_unknown_revision"`
}

func (e *Executor) Execute(parent context.Context, op operation.Request) (operation.Result, *operation.Failure) {
	if e == nil || e.runtime == nil || op.Name != OperationIncoming {
		return operation.Result{}, failure(operation.FailureNotImplemented, operation.ErrNotImplemented)
	}
	var input request
	if err := decodeClosed(op.Input, &input); err != nil {
		return operation.Result{}, failure(operation.FailureInvalidInput, err)
	}
	applyDefaults(&input)
	if err := validate(input); err != nil {
		return operation.Result{}, failure(operation.FailureInvalidInput, err)
	}
	resolvedID, resolvedGeneration, runtimeFailure := ResolveSession(e.runtime, input.SessionID, input.Generation)
	input.SessionID, input.Generation = resolvedID, resolvedGeneration
	if runtimeFailure != "" {
		return operation.Result{}, failure(string(runtimeFailure), nil)
	}
	metadata, runtimeFailure := e.runtime.Metadata(input.SessionID, input.Generation)
	if runtimeFailure != "" {
		return operation.Result{}, failure(string(runtimeFailure), nil)
	}
	if err := ValidateDocumentURI(SessionWorkspace(e.runtime, input.SessionID, input.Generation), input.URI); err != nil {
		return operation.Result{}, failure(operation.FailureInvalidInput, err)
	}
	if input.Relations != nil {
		return e.executeComposition(parent, op.Input, input, metadata)
	}
	if !metadata.CallHierarchySupport {
		return operation.Result{}, failure(string(graph.UnsupportedCallHierarchy), nil)
	}
	switch metadata.PositionEncoding {
	case "utf-8", "utf-16", "utf-32":
	default:
		return operation.Result{}, failure("UNSUPPORTED_POSITION_ENCODING", fmt.Errorf("retained position encoding %q is unsupported", metadata.PositionEncoding))
	}
	ctx, cancel := context.WithTimeout(parent, time.Duration(input.TimeoutMS)*time.Millisecond)
	defer cancel()
	if runtime, ok := e.runtime.(documentRuntime); ok {
		document := runtime.PrepareDocument(ctx, sessionruntime.DocumentRequest{SessionID: input.SessionID, Generation: input.Generation, URI: input.URI, LanguageID: input.LanguageID})
		if document.Failure != "" {
			return operation.Result{}, failure(string(document.Failure), nil)
		}
		input.LanguageID = document.LanguageID
	}
	client := NewSessionClient(e.runtime, input.SessionID, input.Generation, time.Duration(input.RequestTimeoutMS)*time.Millisecond)
	line, character, targetFailure := ResolveTarget(ctx, client, input.URI, input.Symbol, input.Line, input.Character)
	if targetFailure != nil {
		return operation.Result{}, targetFailure
	}
	result := traverse.Incoming(ctx, client, lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: input.URI}, Position: lsp.Position{Line: line, Character: character}}, traverse.Options{MaxDepth: input.MaxDepth, MaxNodes: input.MaxNodes, SchemaVersion: graph.SchemaVersionV3})
	result.Invocation.Target = graph.Target{URI: input.URI, Line: int(line), Column: int(character)}
	result.Invocation.Limits = graph.Limits{MaxDepth: input.MaxDepth, MaxNodes: input.MaxNodes, TimeoutMS: input.TimeoutMS}
	result.Invocation.RequestTimeoutMS = input.RequestTimeoutMS
	result.Invocation.LanguageID = input.LanguageID
	result.Capabilities.CallHierarchyProvider = metadata.CallHierarchySupport
	artifact, err := json.Marshal(result)
	if err != nil {
		return operation.Result{}, failure(operation.FailureInternal, err)
	}
	return operation.Result{Artifact: append(artifact, '\n')}, nil
}

// SessionWorkspace returns the retained workspace identity for one exact session generation.
func SessionWorkspace(runtime Runtime, id string, generation uint64) string {
	for _, record := range runtime.Records() {
		if record.SessionID == id && record.Generation == generation {
			return record.Profile.Workspace().String()
		}
	}
	return ""
}

// ValidateDocumentURI rejects the selected workspace root where a document URI is required.
func ValidateDocumentURI(workspace, raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "file" || workspace == "" {
		return nil
	}
	if filepath.Clean(u.Path) == filepath.Clean(workspace) {
		return errors.New("uri must identify an exact document; workspace-root URI is invalid")
	}
	return nil
}

// ResolveSession preserves explicit generations and infers only one current READY generation.
func ResolveSession(runtime Runtime, id string, generation uint64) (string, uint64, session.Failure) {
	if resolver, ok := runtime.(SelectorRuntime); ok {
		return resolver.ResolveSessionSelector(id, generation)
	}
	if generation != 0 {
		return id, generation, ""
	}
	var match *sessionruntime.Record
	for _, record := range runtime.Records() {
		if record.SessionID != id {
			continue
		}
		if match != nil {
			return "", 0, session.Failure("AMBIGUOUS_SESSION_SELECTOR")
		}
		copy := record
		match = &copy
	}
	if match == nil {
		return "", 0, session.SessionNotFound
	}
	if match.State != session.Ready {
		return "", 0, session.Failure("SESSION_NOT_READY")
	}
	return match.SessionID, match.Generation, ""
}

type PreparedTarget struct {
	Line, Character uint32
	Items           []lsp.CallHierarchyItem
	ExactMatches    int
	TotalSymbols    int
	OmittedSymbols  int
	Action          string
}

// ResolveTarget resolves an exact target while preserving its historical coordinate-only API.
func ResolveTarget(ctx context.Context, client *SessionClient, uri, symbolName string, line, character *uint32) (uint32, uint32, *operation.Failure) {
	if symbolName == "" {
		return *line, *character, nil
	}
	prepared, failed := ResolvePreparedTarget(ctx, client, uri, symbolName, line, character)
	return prepared.Line, prepared.Character, failed
}

// ResolvePreparedTarget performs bounded target preflight and returns the unique prepared item.
func ResolvePreparedTarget(ctx context.Context, client *SessionClient, uri, symbolName string, line, character *uint32) (PreparedTarget, *operation.Failure) {
	if symbolName == "" {
		position := lsp.Position{Line: *line, Character: *character}
		items, err := client.PrepareCallHierarchy(ctx, lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: position})
		if err == nil && len(items) == 1 {
			if !compatiblePreparedPosition(uri, position, items[0]) {
				return PreparedTarget{ExactMatches: 1, Action: "FAIL_MISMATCH"}, failure("POSITION_PREPARE_MISMATCH", errors.New("prepared identity mismatch"))
			}
			return PreparedTarget{Line: *line, Character: *character, Items: items, Action: "DIRECT_PREPARE"}, nil
		}
		if err != nil && !retryableSymbolPrepareMiss(err) {
			return PreparedTarget{Action: "FAIL_DOCUMENT_SYMBOLS"}, classifiedPrepareFailure(err)
		}
		if err == nil && len(items) > 1 {
			return PreparedTarget{ExactMatches: len(items), Action: "FAIL_AMBIGUOUS"}, failure("POSITION_PREPARE_AMBIGUOUS", errors.New("ambiguous prepared target"))
		}
		return recoverPositionTarget(ctx, client, uri, position)
	}
	return resolveSymbolPrepared(ctx, client, uri, symbolName)
}

func resolveSymbolPrepared(ctx context.Context, client *SessionClient, uri, symbolName string) (PreparedTarget, *operation.Failure) {
	symbols, err := client.DocumentSymbols(ctx, lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}})
	if err != nil {
		if strings.Contains(err.Error(), "json-rpc error -32601") {
			return PreparedTarget{Action: "FAIL_UNSUPPORTED"}, failure("DOCUMENT_SYMBOL_UNSUPPORTED", errors.New("document symbols unsupported"))
		}
		return PreparedTarget{Action: "FAIL_DOCUMENT_SYMBOLS"}, failure("DOCUMENT_SYMBOL_FAILED", errors.New("document symbol request failed"))
	}
	flat := flattenSymbols(symbols)
	matches := make([]lsp.DocumentSymbol, 0, 1)
	for _, symbol := range flat {
		if symbol.Name == symbolName {
			matches = append(matches, symbol)
		}
	}
	exactMatches, totalSymbols, omittedSymbols := targetDiagnosticCounts(len(matches), len(flat))
	base := PreparedTarget{ExactMatches: exactMatches, TotalSymbols: totalSymbols, OmittedSymbols: omittedSymbols}
	if len(matches) == 0 {
		base.Action = "FAIL_ABSENT"
		f := failure("DOCUMENT_SYMBOL_ABSENT", errors.New("exact document symbol absent"))
		f.Diagnostics = []string{fmt.Sprintf("exact_matches=0 total_symbols=%d omitted_symbols=%d action=FAIL_ABSENT", base.TotalSymbols, base.OmittedSymbols)}
		return base, f
	}
	if len(matches) != 1 {
		base.Action = "FAIL_AMBIGUOUS"
		f := failure("DOCUMENT_SYMBOL_AMBIGUOUS", errors.New("exact document symbol ambiguous"))
		f.Diagnostics = []string{fmt.Sprintf("exact_matches=%d total_symbols=%d omitted_symbols=%d action=FAIL_AMBIGUOUS", base.ExactMatches, base.TotalSymbols, base.OmittedSymbols)}
		return base, f
	}
	return probeDocumentSymbol(ctx, client, uri, matches[0], base)
}

func recoverPositionTarget(ctx context.Context, client *SessionClient, uri string, position lsp.Position) (PreparedTarget, *operation.Failure) {
	symbols, err := client.DocumentSymbols(ctx, lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}})
	if err != nil {
		if strings.Contains(err.Error(), "json-rpc error -32601") {
			return PreparedTarget{Action: "FAIL_UNSUPPORTED"}, failure("DOCUMENT_SYMBOL_UNSUPPORTED", errors.New("document symbols unsupported"))
		}
		return PreparedTarget{Action: "FAIL_DOCUMENT_SYMBOLS"}, failure("DOCUMENT_SYMBOL_FAILED", errors.New("document symbol request failed"))
	}
	flat := flattenSymbols(symbols)
	_, totalSymbols, omittedSymbols := targetDiagnosticCounts(0, len(flat))
	containing := make([]lsp.DocumentSymbol, 0, 1)
	for _, symbol := range flat {
		if !ValidDocumentSymbolTarget(symbol) {
			return PreparedTarget{TotalSymbols: totalSymbols, OmittedSymbols: omittedSymbols, Action: "FAIL_MALFORMED"}, failure("DOCUMENT_SYMBOL_MALFORMED_RANGE", errors.New("malformed document symbol range"))
		}
		if callableSymbolKind(symbol.Kind) && rangeContainsPosition(symbol.Range, position) {
			containing = append(containing, symbol)
		}
	}
	exactMatches, _, _ := targetDiagnosticCounts(len(containing), len(flat))
	base := PreparedTarget{ExactMatches: exactMatches, TotalSymbols: totalSymbols, OmittedSymbols: omittedSymbols}
	if len(containing) == 0 {
		base.Action = "FAIL_ABSENT"
		return base, failure("POSITION_SYMBOL_ABSENT", errors.New("no containing callable symbol"))
	}
	if len(containing) != 1 {
		base.Action = "FAIL_AMBIGUOUS"
		return base, failure("POSITION_SYMBOL_AMBIGUOUS", errors.New("multiple containing callable symbols"))
	}
	return probeDocumentSymbol(ctx, client, uri, containing[0], base)
}

func probeDocumentSymbol(ctx context.Context, client *SessionClient, uri string, symbol lsp.DocumentSymbol, base PreparedTarget) (PreparedTarget, *operation.Failure) {
	if !ValidDocumentSymbolTarget(symbol) || !callableSymbolKind(symbol.Kind) {
		base.Action = "FAIL_MALFORMED"
		return base, failure("DOCUMENT_SYMBOL_MALFORMED_RANGE", errors.New("invalid callable document symbol"))
	}
	start := symbol.SelectionRange.Start
	for delta := uint32(0); delta <= maxSymbolPrepareProbeDelta && delta <= ^uint32(0)-start.Character; delta++ {
		candidate := lsp.Position{Line: start.Line, Character: start.Character + delta}
		if !rangeContainsPosition(symbol.SelectionRange, candidate) {
			break
		}
		items, err := client.PrepareCallHierarchy(ctx, lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: candidate})
		if err == nil {
			if len(items) == 0 {
				continue
			}
			if len(items) != 1 || !compatiblePreparedMethod(uri, symbol, items[0]) {
				base.Action = "FAIL_MISMATCH"
				return base, failure("DOCUMENT_SYMBOL_PREPARE_MISMATCH", errors.New("prepared identity mismatch"))
			}
			base.Line, base.Character, base.Items, base.Action = candidate.Line, candidate.Character, items, "RECOVERED_PREPARE"
			return base, nil
		}
		if retryableSymbolPrepareMiss(err) {
			continue
		}
		base.Action = "FAIL_DOCUMENT_SYMBOLS"
		return base, classifiedPrepareFailure(err)
	}
	base.Action = "FAIL_UNPREPARABLE"
	return base, failure("DOCUMENT_SYMBOL_UNPREPARABLE", errors.New("bounded prepare probes exhausted"))
}

func targetDiagnosticCounts(matches, total int) (int, int, int) {
	return min(matches, maxTargetDiagnosticCount), min(total, maxTargetDiagnosticCount), min(max(0, total-maxSymbolSuggestions), maxTargetDiagnosticCount)
}

func flattenSymbols(symbols []lsp.DocumentSymbol) []lsp.DocumentSymbol {
	var flat []lsp.DocumentSymbol
	var walk func([]lsp.DocumentSymbol)
	walk = func(items []lsp.DocumentSymbol) {
		for _, symbol := range items {
			flat = append(flat, symbol)
			walk(symbol.Children)
		}
	}
	walk(symbols)
	return flat
}

func classifiedPrepareFailure(err error) *operation.Failure {
	if errors.Is(err, context.Canceled) {
		return failure("CANCELLED", context.Canceled)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return failure("REQUEST_TIMEOUT", context.DeadlineExceeded)
	}
	return failure("DOCUMENT_SYMBOL_PREPARE_FAILED", errors.New("prepare request failed"))
}

var supportedRelations = map[string]bool{
	"CALLS": true, "BINDS_ARGUMENT": true, "PASSES_CALLBACK": true,
	"INVOKES_TASK": true, "TRIGGERS_RELOAD": true, "UPDATES_STATE": true,
	"RENDERS_FROM": true,
}

type compositionArtifact struct {
	SchemaVersion string            `json:"schema_version"`
	Complete      bool              `json:"complete"`
	Calls         json.RawMessage   `json:"calls,omitempty"`
	Providers     []provider.Result `json:"providers,omitempty"`
}

func (e *Executor) executeComposition(parent context.Context, raw json.RawMessage, input request, metadata sessionruntime.SessionMetadata) (operation.Result, *operation.Failure) {
	selected := append([]string(nil), (*input.Relations)...)
	seen := make(map[string]bool, len(selected))
	external := make([]string, 0, len(selected))
	wantCalls := false
	for _, kind := range selected {
		if !supportedRelations[kind] || seen[kind] {
			return operation.Result{}, failure(operation.FailureInvalidInput, fmt.Errorf("unknown or duplicate relation %q", kind))
		}
		seen[kind] = true
		if kind == "CALLS" {
			wantCalls = true
		} else {
			external = append(external, kind)
		}
	}
	artifact := compositionArtifact{SchemaVersion: "lsp-trace.incoming-composition.v1", Complete: true}
	if wantCalls {
		if !metadata.CallHierarchySupport {
			return operation.Result{}, failure(string(graph.UnsupportedCallHierarchy), nil)
		}
		legacyInput := input
		legacyInput.Relations = nil
		encoded, _ := json.Marshal(legacyInput)
		legacy, failed := e.Execute(parent, operation.Request{Name: OperationIncoming, Input: encoded})
		if failed != nil {
			return operation.Result{}, failed
		}
		artifact.Calls = bytes.TrimSpace(legacy.Artifact)
		var calls graph.Result
		if json.Unmarshal(legacy.Artifact, &calls) != nil || !calls.Summary.Complete {
			artifact.Complete = false
		}
	}
	if len(external) > 0 {
		collector, ok := e.runtime.(RelationCollector)
		if !ok {
			return operation.Result{}, failure("ADAPTER_NOT_AVAILABLE", nil)
		}
		providerResult, err := collector.CollectRelations(parent, external, raw)
		if err != nil {
			if provider.IsRevisionCustodyError(err) {
				return operation.Result{}, failure("RELATION_CUSTODY_FAILED", err)
			}
			return operation.Result{}, failure("RELATION_PROVIDER_FAILED", err)
		}
		if providerResult.ProviderID == "" || providerResult.Terminal == "" || providerResult.GraphV4.SchemaVersion != graph.NormalizedRelationsSchemaVersion {
			return operation.Result{}, failure("RELATION_PROVIDER_MALFORMED", errors.New("provider result omitted required identity, terminal, or graph-v4"))
		}
		for _, relation := range providerResult.GraphV4.Relations {
			if !seen[relation.Kind] || relation.Kind == graph.RelationCalls {
				return operation.Result{}, failure("RELATION_PROVIDER_KIND_MISMATCH", fmt.Errorf("provider returned unselected relation %q", relation.Kind))
			}
		}
		if !wantCalls {
			encoded, err := json.Marshal(providerResult.GraphV4)
			if err != nil || len(encoded) == 0 {
				return operation.Result{}, failure("RELATION_PROVIDER_MALFORMED", err)
			}
			return operation.Result{Artifact: append(encoded, '\n'), LogicalDigest: providerResult.LogicalDigest}, nil
		}
		artifact.Providers = []provider.Result{providerResult}
		if !providerResult.Complete || providerResult.Truncated || providerResult.Terminal != "COMPLETE_WITHIN_BOUNDS" {
			artifact.Complete = false
		}
	}
	encoded, err := json.Marshal(artifact)
	if err != nil {
		return operation.Result{}, failure(operation.FailureInternal, err)
	}
	return operation.Result{Artifact: append(encoded, '\n')}, nil
}

// ValidDocumentSymbolTarget reports whether a document symbol has ordered ranges
// and a selection range wholly contained by its enclosing range.
func ValidDocumentSymbolTarget(symbol lsp.DocumentSymbol) bool {
	return validRange(symbol.Range) && validRange(symbol.SelectionRange) && rangeContains(symbol.Range, symbol.SelectionRange)
}

func validRange(r lsp.Range) bool {
	return !positionLess(r.End, r.Start)
}

func rangeContains(outer, inner lsp.Range) bool {
	return !positionLess(inner.Start, outer.Start) && !positionLess(outer.End, inner.End)
}

func rangeContainsPosition(r lsp.Range, position lsp.Position) bool {
	return !positionLess(position, r.Start) && positionLess(position, r.End)
}

func retryableSymbolPrepareMiss(err error) bool {
	message, ok := strings.CutPrefix(err.Error(), "json-rpc error 0: ")
	return ok && (message == "identifier not found" || message == "column is beyond end of line" || strings.HasSuffix(message, " is not a function"))
}

func compatiblePreparedPosition(uri string, position lsp.Position, item lsp.CallHierarchyItem) bool {
	return item.URI == uri && canonicalDocumentURI(item.URI) && callableSymbolKind(item.Kind) &&
		validRange(item.Range) && validRange(item.SelectionRange) && rangeContains(item.Range, item.SelectionRange) &&
		rangeContainsPosition(item.Range, position)
}

func compatiblePreparedMethod(uri string, symbol lsp.DocumentSymbol, item lsp.CallHierarchyItem) bool {
	return item.URI == uri && canonicalDocumentURI(item.URI) && callableSymbolKind(item.Kind) &&
		validRange(item.Range) && validRange(item.SelectionRange) && rangeContains(item.Range, item.SelectionRange) &&
		rangeContains(symbol.Range, item.Range) && rangeContains(symbol.Range, item.SelectionRange)
}

func callableSymbolKind(kind int) bool {
	return kind == 6 || kind == 9 || kind == 12
}

func canonicalDocumentURI(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.IsAbs() && u.Fragment == "" && u.RawQuery == "" && u.Opaque == "" &&
		u.Path == path.Clean(u.Path) && u.String() == raw
}

func positionLess(a, b lsp.Position) bool {
	return a.Line < b.Line || (a.Line == b.Line && a.Character < b.Character)
}

func applyDefaults(r *request) {
	if r.MaxDepth == 0 {
		r.MaxDepth = 4
	}
	if r.MaxNodes == 0 {
		r.MaxNodes = 100
	}
	if r.TimeoutMS == 0 {
		r.TimeoutMS = 5000
	}
	if r.RequestTimeoutMS == 0 {
		r.RequestTimeoutMS = 1000
	}
}

func validate(r request) error {
	if r.SessionID == "" || r.URI == "" {
		return errors.New("session_id and uri are required")
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
	if r.MaxDepth < 1 || r.MaxDepth > 64 || r.MaxNodes < 1 || r.MaxNodes > 10000 || r.TimeoutMS < 1 || r.TimeoutMS > 60000 || r.RequestTimeoutMS < 1 || r.RequestTimeoutMS > 60000 {
		return errors.New("limits and deadlines are outside supported bounds")
	}
	return nil
}

func decodeClosed(raw json.RawMessage, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("one JSON value required")
	}
	return nil
}

func failure(code string, err error) *operation.Failure {
	return &operation.Failure{Code: code, Err: err}
}

const (
	defaultMaxMessages = 64
	defaultMaxBytes    = 4 << 20
)

// WireLimits bounds each individual LSP request made by SessionClient.
type WireLimits struct {
	MaxMessages int
	MaxBytes    int64
}

// SessionClient adapts standard call-hierarchy requests to one retained exact generation.
type SessionClient struct {
	runtime        Runtime
	sessionID      string
	generation     uint64
	requestTimeout time.Duration
	wireLimits     WireLimits
}

// NewSessionClient preserves the incoming operation's fixed safe per-wire-request limits.
func NewSessionClient(runtime Runtime, sessionID string, generation uint64, requestTimeout time.Duration) *SessionClient {
	return NewSessionClientWithWireLimits(runtime, sessionID, generation, requestTimeout, WireLimits{MaxMessages: defaultMaxMessages, MaxBytes: defaultMaxBytes})
}

// NewSessionClientWithWireLimits applies explicit limits to every individual LSP request.
func NewSessionClientWithWireLimits(runtime Runtime, sessionID string, generation uint64, requestTimeout time.Duration, limits WireLimits) *SessionClient {
	return &SessionClient{runtime: runtime, sessionID: sessionID, generation: generation, requestTimeout: requestTimeout, wireLimits: limits}
}

func (c *SessionClient) WorkspaceSymbols(ctx context.Context, params lsp.WorkspaceSymbolParams) ([]lsp.WorkspaceSymbol, error) {
	var symbols []lsp.WorkspaceSymbol
	wasNull, err := c.call(ctx, "workspace/symbol", params, &symbols)
	if err != nil {
		return nil, err
	}
	if wasNull {
		return nil, nil
	}
	return symbols, nil
}

func (c *SessionClient) PrepareCallHierarchy(ctx context.Context, params lsp.PrepareCallHierarchyParams) ([]lsp.CallHierarchyItem, error) {
	var items []lsp.CallHierarchyItem
	wasNull, err := c.call(ctx, "textDocument/prepareCallHierarchy", params, &items)
	if err != nil {
		return nil, err
	}
	if wasNull {
		return nil, nil
	}
	return items, nil
}

func (c *SessionClient) IncomingCalls(ctx context.Context, item lsp.CallHierarchyItem) ([]lsp.CallHierarchyIncomingCall, bool, error) {
	var calls []lsp.CallHierarchyIncomingCall
	wasNull, err := c.call(ctx, "callHierarchy/incomingCalls", lsp.CallHierarchyIncomingCallsParams{Item: item}, &calls)
	return calls, wasNull, err
}

func (c *SessionClient) OutgoingCalls(ctx context.Context, item lsp.CallHierarchyItem) ([]lsp.CallHierarchyOutgoingCall, bool, error) {
	var calls []lsp.CallHierarchyOutgoingCall
	wasNull, err := c.call(ctx, "callHierarchy/outgoingCalls", lsp.CallHierarchyOutgoingCallsParams{Item: item}, &calls)
	return calls, wasNull, err
}

type documentSymbolWire struct {
	Name           string               `json:"name"`
	Detail         string               `json:"detail,omitempty"`
	Kind           int                  `json:"kind"`
	Tags           []int                `json:"tags,omitempty"`
	Deprecated     bool                 `json:"deprecated,omitempty"`
	Range          lsp.Range            `json:"range"`
	SelectionRange lsp.Range            `json:"selectionRange"`
	Children       []documentSymbolWire `json:"children,omitempty"`
}

func (s documentSymbolWire) normalized() lsp.DocumentSymbol {
	children := make([]lsp.DocumentSymbol, len(s.Children))
	for i := range s.Children {
		children[i] = s.Children[i].normalized()
	}
	return lsp.DocumentSymbol{Name: s.Name, Detail: s.Detail, Kind: s.Kind, Range: s.Range, SelectionRange: s.SelectionRange, Children: children}
}

func (*SessionClient) SupportsDocumentSymbols() bool { return true }
func (c *SessionClient) DocumentSymbols(ctx context.Context, params lsp.DocumentSymbolParams) ([]lsp.DocumentSymbol, error) {
	var rawSymbols []json.RawMessage
	wasNull, err := c.call(ctx, "textDocument/documentSymbol", params, &rawSymbols)
	if err != nil {
		return nil, err
	}
	if wasNull {
		return nil, nil
	}
	symbols := make([]lsp.DocumentSymbol, 0, len(rawSymbols))
	for _, raw := range rawSymbols {
		var discriminator struct {
			Location json.RawMessage `json:"location"`
		}
		if err := json.Unmarshal(raw, &discriminator); err != nil {
			return nil, fmt.Errorf("malformed textDocument/documentSymbol result: %w", err)
		}
		if len(discriminator.Location) == 0 {
			var symbol documentSymbolWire
			if err := decodeStrict(raw, &symbol); err != nil {
				return nil, fmt.Errorf("malformed textDocument/documentSymbol result: %w", err)
			}
			symbols = append(symbols, symbol.normalized())
			continue
		}
		var symbol struct {
			Name       string `json:"name"`
			Kind       int    `json:"kind"`
			Tags       []int  `json:"tags,omitempty"`
			Deprecated bool   `json:"deprecated,omitempty"`
			Location   struct {
				URI   string    `json:"uri"`
				Range lsp.Range `json:"range"`
			} `json:"location"`
			ContainerName string `json:"containerName,omitempty"`
		}
		if err := decodeStrict(raw, &symbol); err != nil {
			return nil, fmt.Errorf("malformed textDocument/documentSymbol result: %w", err)
		}
		symbols = append(symbols, lsp.DocumentSymbol{Name: symbol.Name, Kind: symbol.Kind, Range: symbol.Location.Range, SelectionRange: symbol.Location.Range})
	}
	return symbols, nil
}

func decodeStrict(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func (c *SessionClient) call(parent context.Context, method string, params, target any) (bool, error) {
	raw, err := json.Marshal(params)
	if err != nil {
		return false, err
	}
	deadline := time.Now().Add(c.requestTimeout)
	if outer, ok := parent.Deadline(); ok && outer.Before(deadline) {
		deadline = outer
	}
	observed := c.runtime.RoundTrip(parent, sessionruntime.RoundTripRequest{SessionID: c.sessionID, Generation: c.generation, Method: method, Params: raw, Deadline: deadline, MaxMessages: c.wireLimits.MaxMessages, MaxBytes: c.wireLimits.MaxBytes})
	if observed.Failure != "" {
		switch observed.Failure {
		case session.RequestCancelled:
			return false, context.Canceled
		case session.RequestTimeout:
			return false, context.DeadlineExceeded
		default:
			return false, fmt.Errorf("%s", observed.Failure)
		}
	}
	if observed.ServerError != nil {
		return false, fmt.Errorf("json-rpc error %d: %s", observed.ServerError.Code, observed.ServerError.Message)
	}
	if bytes.Equal(bytes.TrimSpace(observed.Result), []byte("null")) {
		return true, nil
	}
	if len(observed.Result) == 0 || !json.Valid(observed.Result) {
		return false, errors.New("malformed JSON-RPC result")
	}
	decoder := json.NewDecoder(bytes.NewReader(observed.Result))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return false, fmt.Errorf("malformed %s result: %w", method, err)
	}
	return false, nil
}
