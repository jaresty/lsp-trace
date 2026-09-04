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
	"time"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/session"
	"lsp-trace/internal/traverse"
	"lsp-trace/sessionruntime"
)

const OperationIncoming operation.Name = "incoming"

type Runtime interface {
	Metadata(string, uint64) (sessionruntime.SessionMetadata, session.Failure)
	RoundTrip(context.Context, sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult
}

type Executor struct{ runtime Runtime }

func NewExecutor(runtime Runtime) *Executor { return &Executor{runtime: runtime} }

type request struct {
	SessionID        string `json:"session_id"`
	Generation       uint64 `json:"generation"`
	URI              string `json:"uri"`
	Line             uint32 `json:"line"`
	Character        uint32 `json:"character"`
	MaxDepth         int    `json:"max_depth"`
	MaxNodes         int    `json:"max_nodes"`
	TimeoutMS        int64  `json:"timeout_ms"`
	RequestTimeoutMS int64  `json:"request_timeout_ms"`
}

func (e *Executor) Execute(parent context.Context, op operation.Request) (operation.Result, *operation.Failure) {
	if e == nil || e.runtime == nil || op.Name != OperationIncoming {
		return operation.Result{}, failure(operation.FailureNotImplemented, operation.ErrNotImplemented)
	}
	var input request
	if err := decodeClosed(op.Input, &input); err != nil {
		return operation.Result{}, failure(operation.FailureInvalidInput, err)
	}
	if err := validate(input); err != nil {
		return operation.Result{}, failure(operation.FailureInvalidInput, err)
	}
	metadata, runtimeFailure := e.runtime.Metadata(input.SessionID, input.Generation)
	if runtimeFailure != "" {
		return operation.Result{}, failure(string(runtimeFailure), nil)
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
	client := NewSessionClient(e.runtime, input.SessionID, input.Generation, time.Duration(input.RequestTimeoutMS)*time.Millisecond)
	result := traverse.Incoming(ctx, client, lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: input.URI}, Position: lsp.Position{Line: input.Line, Character: input.Character}}, traverse.Options{MaxDepth: input.MaxDepth, MaxNodes: input.MaxNodes, SchemaVersion: graph.SchemaVersionV3})
	result.Invocation.Target = graph.Target{URI: input.URI, Line: int(input.Line), Column: int(input.Character)}
	result.Invocation.Limits = graph.Limits{MaxDepth: input.MaxDepth, MaxNodes: input.MaxNodes, TimeoutMS: input.TimeoutMS}
	result.Invocation.RequestTimeoutMS = input.RequestTimeoutMS
	result.Capabilities.CallHierarchyProvider = metadata.CallHierarchySupport
	artifact, err := json.Marshal(result)
	if err != nil {
		return operation.Result{}, failure(operation.FailureInternal, err)
	}
	return operation.Result{Artifact: append(artifact, '\n')}, nil
}

func validate(r request) error {
	if r.SessionID == "" || r.Generation == 0 || r.URI == "" {
		return errors.New("session_id, generation, and uri are required")
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

func (*SessionClient) SupportsDocumentSymbols() bool { return false }
func (*SessionClient) DocumentSymbols(context.Context, lsp.DocumentSymbolParams) ([]lsp.DocumentSymbol, error) {
	return nil, errors.New("document symbols unsupported")
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
