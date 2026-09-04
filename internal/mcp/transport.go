package mcp

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/publication"
)

const (
	protocolVersion          = "2025-06-18"
	resultEnvelopeSchemaID   = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-result.v1.schema.json"
	artifactEnvelopeSchemaID = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-artifact.v1.schema.json"
	domainEnvelopeSchemaID   = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-domain-error.v1.schema.json"
	disabledEnvelopeSchemaID = "https://jaresty.github.io/lsp-trace/mcp/schemas/envelope-not-implemented.v1.schema.json"
)

type Executor interface {
	Execute(context.Context, operation.Request) (operation.Result, *operation.Failure)
}

type Server struct {
	Registry        *Registry
	Executor        Executor
	Executors       map[ExecutorFamily]Executor
	PublicationRoot *publication.Root
	Publisher       *publication.Publisher
	requestSequence atomic.Uint64
	lifecycleMu     sync.Mutex
	serveGeneration uint64
	serveCancel     context.CancelFunc
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type response struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id,omitempty"`
	Result  any       `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}

type callParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type envelope struct {
	EnvelopeVersion         string               `json:"envelope_version"`
	EnvelopeSchemaID        string               `json:"envelope_schema_id"`
	Tool                    string               `json:"tool"`
	RequestID               string               `json:"request_id"`
	Outcome                 string               `json:"outcome"`
	OperationStatus         string               `json:"operation_status"`
	IsError                 bool                 `json:"isError"`
	Code                    string               `json:"code,omitempty"`
	Diagnostics             []string             `json:"diagnostics,omitempty"`
	Result                  any                  `json:"result,omitempty"`
	Content                 *string              `json:"content,omitempty"`
	PublicationReceipt      *publication.Receipt `json:"publication_receipt,omitempty"`
	ArtifactSchemaID        string               `json:"artifact_schema_id,omitempty"`
	ArtifactDigest          string               `json:"artifact_digest,omitempty"`
	ArtifactByteLength      *uint64              `json:"artifact_byte_length,omitempty"`
	RetainedFailureMetadata *publication.Failure `json:"retained_failure_metadata,omitempty"`
}

type callResult struct {
	Content           []any    `json:"content"`
	StructuredContent envelope `json:"structuredContent"`
	IsError           bool     `json:"isError"`
}

func (s *Server) Serve(stdin io.Reader, stdout io.Writer) error {
	return s.ServeContext(context.Background(), stdin, stdout)
}

func (s *Server) ServeContext(parent context.Context, stdin io.Reader, stdout io.Writer) error {
	ctx, cancel := context.WithCancel(parent)
	s.lifecycleMu.Lock()
	if s.serveCancel != nil {
		s.serveCancel()
	}
	s.serveGeneration++
	generation := s.serveGeneration
	s.serveCancel = cancel
	s.lifecycleMu.Unlock()
	defer func() {
		cancel()
		s.lifecycleMu.Lock()
		if s.serveGeneration == generation {
			s.serveCancel = nil
		}
		s.lifecycleMu.Unlock()
	}()
	if s.Registry == nil {
		s.Registry = NewRegistryWithPublication(false, s.PublicationRoot != nil)
	}
	scanner := bufio.NewScanner(stdin)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	encoder := json.NewEncoder(stdout)
	encoder.SetEscapeHTML(false)
	for scanner.Scan() {
		var req request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			if err := encoder.Encode(response{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "Parse error"}}); err != nil {
				return err
			}
			continue
		}
		if req.ID == nil { // MCP notifications have no response.
			continue
		}
		resp := s.handleContext(ctx, req)
		if err := encoder.Encode(resp); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// Shutdown cancels the active serving context. Closing or interrupting the
// transport reader remains the caller's responsibility.
func (s *Server) Shutdown() {
	s.lifecycleMu.Lock()
	cancel := s.serveCancel
	s.lifecycleMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *Server) handleContext(ctx context.Context, req request) response {
	base := response{JSONRPC: "2.0", ID: req.ID}
	if req.JSONRPC != "2.0" {
		base.Error = &rpcError{Code: -32600, Message: "Invalid Request"}
		return base
	}
	switch req.Method {
	case "initialize":
		base.Result = map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": "lsp-trace-mcp", "version": "1"},
		}
	case "ping":
		base.Result = map[string]any{}
	case "tools/list":
		tools := make([]map[string]any, 0, len(s.Registry.Advertised()))
		for _, tool := range s.Registry.Advertised() {
			tools = append(tools, map[string]any{"name": tool.Name, "description": tool.Description, "inputSchema": tool.InputSchema})
		}
		base.Result = map[string]any{"tools": tools}
	case "tools/call":
		return s.callContext(ctx, base, req.Params)
	default:
		base.Error = &rpcError{Code: -32601, Message: "Method not found"}
	}
	return base
}

func (s *Server) callContext(ctx context.Context, base response, raw json.RawMessage) response {
	var params callParams
	if err := decodeClosed(raw, &params, "name", "arguments"); err != nil || params.Name == "" || params.Arguments == nil {
		base.Error = &rpcError{Code: -32602, Message: "Invalid params"}
		return base
	}
	tool, ok := s.Registry.Resolve(params.Name)
	if !ok {
		base.Error = &rpcError{Code: -32602, Message: "Unknown tool: " + params.Name}
		return base
	}
	if err := validateArguments(tool, params.Arguments); err != nil {
		base.Error = &rpcError{Code: -32602, Message: "Invalid tool arguments: " + err.Error()}
		return base
	}
	if tool.semanticValidator != nil {
		if err := tool.semanticValidator(ctx, tool, cloneMap(params.Arguments)); err != nil {
			base.Error = &rpcError{Code: -32602, Message: "Invalid tool arguments: " + err.Error()}
			return base
		}
	}
	requestID := s.nextRequestID()
	if tool.Availability != Enabled {
		env := envelope{
			EnvelopeVersion: "1", EnvelopeSchemaID: disabledEnvelopeSchemaID, Tool: tool.Name, RequestID: requestID,
			Outcome: "DOMAIN_ERROR", OperationStatus: "FAILED", IsError: true, Code: "TOOL_NOT_IMPLEMENTED",
		}
		return bindEnvelope(base, tool, env)
	}
	selector, publicationRequested := params.Arguments["output_selector"].(string)
	if publicationRequested && s.PublicationRoot == nil {
		env := domainErrorEnvelope(tool.Name, requestID, "OUTPUT_SELECTOR_UNSAFE", []string{"selector publication is disabled"})
		return bindEnvelope(base, tool, env)
	}
	executor := s.Executor
	if routed, ok := s.Executors[tool.ExecutorFamily]; ok {
		executor = routed
	}
	if executor == nil {
		env := domainErrorEnvelope(tool.Name, requestID, "OUTPUT_VALIDATION_FAILED", []string{"operation executor unavailable for family " + string(tool.ExecutorFamily)})
		return bindEnvelope(base, tool, env)
	}
	operationArguments := params.Arguments
	if publicationRequested {
		operationArguments = make(map[string]any, len(params.Arguments)-1)
		for key, value := range params.Arguments {
			if key != "output_selector" {
				operationArguments[key] = value
			}
		}
	}
	opInput, err := json.Marshal(operationArguments)
	if err != nil {
		env := domainErrorEnvelope(tool.Name, requestID, "OUTPUT_VALIDATION_FAILED", []string{err.Error()})
		return bindEnvelope(base, tool, env)
	}
	opResult, failure := executor.Execute(ctx, operation.Request{
		Name: operationName(tool.Name), RequestID: requestID, Input: opInput,
	})
	if failure != nil {
		code, diagnostics := normalizeDomainFailure(failure)
		if tool.ExecutorFamily == LifecycleExecutorFamily {
			code = failure.Code
			return bindLifecycleEnvelope(base, domainErrorEnvelope(tool.Name, requestID, code, diagnostics))
		}
		if tool.ExecutorFamily == IncomingExecutorFamily {
			code = failure.Code
			if code == operation.FailureInvalidInput {
				code = "INPUT_INVALID"
			}
		}
		return bindEnvelope(base, tool, domainErrorEnvelope(tool.Name, requestID, code, diagnostics))
	}
	if tool.ExecutorFamily == LifecycleExecutorFamily {
		return bindLifecycleEnvelope(base, envelope{
			EnvelopeVersion: "1", EnvelopeSchemaID: resultEnvelopeSchemaID, Tool: tool.Name, RequestID: requestID,
			Outcome: "COMPLETE", OperationStatus: "SUCCEEDED", Result: opResult.Value,
		})
	}
	if tool.Name == "lsp_trace_v1_capabilities" {
		env := envelope{
			EnvelopeVersion: "1", EnvelopeSchemaID: resultEnvelopeSchemaID, Tool: tool.Name, RequestID: requestID,
			Outcome: "COMPLETE", OperationStatus: "SUCCEEDED", Result: opResult.Value,
		}
		return bindEnvelope(base, tool, env)
	}
	artifactID := artifactSchemaID(opResult.Artifact)
	if artifactID == "" || !containsSchemaID(tool.ArtifactSchemaIDs, artifactID) {
		env := domainErrorEnvelope(tool.Name, requestID, "OUTPUT_VALIDATION_FAILED", []string{"artifact schema identity is not permitted"})
		return bindEnvelope(base, tool, env)
	}
	if !publicationRequested && len(opResult.Artifact) > inlineByteLimit {
		env := domainErrorEnvelope(tool.Name, requestID, "OUTPUT_REQUIRES_SELECTOR", nil)
		return bindEnvelope(base, tool, env)
	}
	if publicationRequested {
		sum := sha256.Sum256(opResult.Artifact)
		digest := "sha256:" + hex.EncodeToString(sum[:])
		byteLength := uint64(len(opResult.Artifact))
		successEnvelope := envelope{
			EnvelopeVersion: "1", EnvelopeSchemaID: publicationEnvelopeSchemaID, Tool: tool.Name, RequestID: requestID,
			Outcome: "COMPLETE", OperationStatus: "SUCCEEDED", ArtifactSchemaID: artifactID,
		}
		failureEnvelope := envelope{
			EnvelopeVersion: "1", EnvelopeSchemaID: publicationErrorEnvelopeSchemaID, Tool: tool.Name, RequestID: requestID,
			Outcome: "PUBLICATION_ERROR", OperationStatus: "FAILED", IsError: true, Code: publication.CodePublicationFailed,
			ArtifactSchemaID: artifactID, ArtifactDigest: digest, ArtifactByteLength: &byteLength,
		}
		publisher := s.Publisher
		if publisher == nil {
			publisher = publication.NewPublisher()
		}
		publicationResult := publication.NewOperation(publisher, publication.Request{
			Root: s.PublicationRoot, Selector: selector, Bytes: opResult.Artifact, ArtifactSchemaID: artifactID,
		}).Publish()
		if publicationResult.Failure != nil {
			if publicationResult.Failure.Code == publication.CodeOutputSelectorUnsafe {
				env := domainErrorEnvelope(tool.Name, requestID, publication.CodeOutputSelectorUnsafe, []string{"output selector is unsafe"})
				return bindEnvelope(base, tool, env)
			}
			failureEnvelope.RetainedFailureMetadata = publicationResult.Failure
			return bindEnvelope(base, tool, failureEnvelope)
		}
		successEnvelope.PublicationReceipt = publicationResult.Receipt
		return bindEnvelope(base, tool, successEnvelope)
	}
	content := string(opResult.Artifact)
	env := envelope{
		EnvelopeVersion: "1", EnvelopeSchemaID: artifactEnvelopeSchemaID, Tool: tool.Name, RequestID: requestID,
		Outcome: "COMPLETE", OperationStatus: "SUCCEEDED", Content: &content, ArtifactSchemaID: artifactID,
	}
	return bindEnvelope(base, tool, env)
}

func (s *Server) nextRequestID() string {
	return fmt.Sprintf("offline-%d", s.requestSequence.Add(1))
}

func bindLifecycleEnvelope(base response, env envelope) response {
	base.Result = callResult{Content: []any{}, StructuredContent: env, IsError: env.IsError}
	return base
}

func bindEnvelope(base response, tool Tool, env envelope) response {
	raw, err := json.Marshal(env)
	if err == nil {
		err = validateEmittedEnvelope(tool, env, raw)
	}
	if err != nil {
		base.Error = &rpcError{Code: -32603, Message: "Internal error: invalid operation envelope"}
		return base
	}
	base.Result = callResult{Content: []any{}, StructuredContent: env, IsError: env.IsError}
	return base
}

func validateEmittedEnvelope(tool Tool, env envelope, raw []byte) error {
	if !containsSchemaID(tool.EnvelopeSchemaIDs, env.EnvelopeSchemaID) {
		return fmt.Errorf("envelope schema %q is not permitted for %s", env.EnvelopeSchemaID, tool.Name)
	}
	hasArtifactMetadata := env.Content != nil || env.PublicationReceipt != nil || env.ArtifactDigest != "" || env.ArtifactByteLength != nil
	if hasArtifactMetadata {
		if env.ArtifactSchemaID == "" {
			return fmt.Errorf("artifact schema identity is required for %s", tool.Name)
		}
		if !containsSchemaID(tool.ArtifactSchemaIDs, env.ArtifactSchemaID) {
			return fmt.Errorf("artifact schema %q is not permitted for %s", env.ArtifactSchemaID, tool.Name)
		}
	} else if env.ArtifactSchemaID != "" {
		return fmt.Errorf("artifact schema identity without artifact delivery metadata for %s", tool.Name)
	}
	if env.Content != nil && env.PublicationReceipt != nil {
		return fmt.Errorf("inline content and publication receipt are mutually exclusive for %s", tool.Name)
	}
	return mcpcontract.ValidateEnvelopeExclusive(raw)
}

func containsSchemaID(ids []string, wanted string) bool {
	for _, id := range ids {
		if id == wanted {
			return true
		}
	}
	return false
}

func domainErrorEnvelope(tool, requestID, code string, diagnostics []string) envelope {
	return envelope{
		EnvelopeVersion: "1", EnvelopeSchemaID: domainEnvelopeSchemaID, Tool: tool, RequestID: requestID,
		Outcome: "DOMAIN_ERROR", OperationStatus: "FAILED", IsError: true, Code: code,
		Diagnostics: diagnostics,
	}
}

func normalizeDomainFailure(failure *operation.Failure) (string, []string) {
	code := failure.Code
	switch code {
	case "INPUT_INVALID", "INPUT_FAMILY_MISMATCH":
	case operation.FailureInternal, operation.FailureNotImplemented:
		code = "OUTPUT_VALIDATION_FAILED"
	default:
		code = "INPUT_INVALID"
	}
	diagnostics := append([]string(nil), failure.Diagnostics...)
	if len(diagnostics) == 0 && failure.Err != nil {
		diagnostics = []string{failure.Err.Error()}
	}
	return code, diagnostics
}

func artifactSchemaID(artifact []byte) string {
	var identity struct {
		ID                      string `json:"$id"`
		SchemaVersion           string `json:"schema_version"`
		InspectionSchemaVersion string `json:"inspection_schema_version"`
		FilterSchemaVersion     string `json:"filter_schema_version"`
	}
	if json.Unmarshal(artifact, &identity) != nil {
		return ""
	}
	if identity.ID != "" {
		return identity.ID
	}
	version := identity.SchemaVersion
	if version == "" {
		version = identity.InspectionSchemaVersion
	}
	if version == "" {
		version = identity.FilterSchemaVersion
	}
	if version == "" {
		return ""
	}
	return "https://jaresty.github.io/lsp-trace/schemas/" + version + ".schema.json"
}

func decodeClosed(raw json.RawMessage, dst any, allowed ...string) error {
	var members map[string]json.RawMessage
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	if err := json.Unmarshal(raw, &members); err != nil {
		return err
	}
	set := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		set[key] = struct{}{}
	}
	for key := range members {
		if _, ok := set[key]; !ok {
			return fmt.Errorf("unknown member %q", key)
		}
	}
	return json.Unmarshal(raw, dst)
}

func validateArguments(tool Tool, arguments map[string]any) error {
	if (tool.ExecutorFamily == LifecycleExecutorFamily || tool.ExecutorFamily == IncomingExecutorFamily || tool.ExecutorFamily == SliceExecutorFamily) && tool.Availability == Enabled {
		return nil // family executor owns closed semantic decoding for the local contract
	}
	raw, err := json.Marshal(arguments)
	if err != nil {
		return err
	}
	if tool.Availability == Enabled {
		return mcpcontract.ValidateJSON(tool.InputSchemaID, raw)
	}
	// Stage 1 reserves later-stage names but compiles no later-stage input
	// variants. The reserved Stage 1 call shape is therefore exactly {}.
	if len(arguments) != 0 {
		return errors.New("reserved Stage 1 input must be exactly an empty object")
	}
	return nil
}

func operationName(canonical string) operation.Name {
	switch canonical {
	case "lsp_trace_v1_schema_get":
		return operation.SchemaGet
	case "lsp_trace_v1_validate":
		return operation.Validate
	case "lsp_trace_v1_verify":
		return operation.Verify
	case "lsp_trace_v1_inspect":
		return operation.Inspect
	case "lsp_trace_v1_filter":
		return operation.Filter
	case "lsp_trace_v1_incoming":
		return operation.Name("incoming")
	case "lsp_trace_v1_slice":
		return operation.Name("slice")
	case "lsp_session_v1_list":
		return operation.Name("session_list")
	case "lsp_session_v1_status":
		return operation.Name("session_status")
	case "lsp_session_v1_stop":
		return operation.Name("session_stop")
	case "lsp_session_v1_restart":
		return operation.Name("session_restart")
	default:
		return operation.Capabilities
	}
}
