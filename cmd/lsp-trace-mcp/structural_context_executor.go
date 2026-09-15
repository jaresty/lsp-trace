package main

import (
	"context"
	"encoding/json"
	"time"

	"lsp-trace/incomingops"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/transientstructural"
	"lsp-trace/internal/transientstructuralresult"
)

const structuralContextOperation operation.Name = "structural_context"

type structuralContextInput struct {
	SessionID        string  `json:"session_id"`
	Generation       uint64  `json:"generation"`
	URI              string  `json:"uri"`
	Symbol           string  `json:"symbol,omitempty"`
	Line             *uint32 `json:"line,omitempty"`
	Character        *uint32 `json:"character,omitempty"`
	DownDepth        int     `json:"down_depth"`
	UpDepth          int     `json:"up_depth"`
	MaxNodes         int     `json:"max_nodes"`
	TimeoutMS        int64   `json:"timeout_ms"`
	RequestTimeoutMS int64   `json:"request_timeout_ms"`
	MaxMessages      int     `json:"max_messages,omitempty"`
	MaxBytes         int64   `json:"max_bytes,omitempty"`
	Analysis         struct {
		Kind      string `json:"kind"`
		Direction string `json:"direction,omitempty"`
		Depth     int    `json:"depth,omitempty"`
	} `json:"analysis"`
}
type structuralContextExecutor struct{ runtime *hostSelectorRuntime }

func newStructuralContextExecutor(r *hostSelectorRuntime) *structuralContextExecutor {
	return &structuralContextExecutor{runtime: r}
}
func (e *structuralContextExecutor) Execute(parent context.Context, op operation.Request) (operation.Result, *operation.Failure) {
	if e == nil || e.runtime == nil || (op.Name != structuralContextOperation && op.Name != operation.Name("structural_context_v2")) {
		return fail(operation.FailureNotImplemented, operation.ErrNotImplemented)
	}
	var in structuralContextInput
	if err := decode(op.Input, &in); err != nil {
		return fail(operation.FailureInvalidInput, err)
	}
	if in.MaxMessages == 0 {
		in.MaxMessages = int(transientstructuralresult.DefaultMaxMessages)
	}
	if in.MaxBytes == 0 {
		in.MaxBytes = int64(transientstructuralresult.DefaultMaxBytes)
	}
	id, generation, sf := incomingops.ResolveSession(e.runtime, in.SessionID, in.Generation)
	if sf != "" {
		return fail(string(sf), nil)
	}
	ctx, cancel := context.WithTimeout(parent, time.Duration(in.TimeoutMS)*time.Millisecond)
	defer cancel()
	q := transientstructural.Request{SessionID: id, Generation: generation, Target: transientstructural.Target{URI: in.URI, Symbol: in.Symbol, Line: in.Line, Character: in.Character}, DownDepth: in.DownDepth, UpDepth: in.UpDepth, MaxNodes: in.MaxNodes, TimeoutMS: in.TimeoutMS, RequestTimeoutMS: in.RequestTimeoutMS, MaxMessages: in.MaxMessages, MaxBytes: in.MaxBytes, Analysis: transientstructural.AnalysisRequest{Kind: transientstructural.AnalysisKind(in.Analysis.Kind), Direction: transientstructural.Direction(in.Analysis.Direction), MaxDepth: in.Analysis.Depth}}
	got, domain := transientstructural.Execute(ctx, e.runtime.Manager, q)
	if domain != nil {
		return fail(string(domain.State), domain)
	}
	managerID, sf := e.runtime.TransientSessionIdentity(id, generation)
	if sf != "" {
		return fail(string(sf), nil)
	}
	var projected any
	var err error
	if op.Name == operation.Name("structural_context_v2") {
		root, ok := e.runtime.Manager.WorkspaceRoot(id, generation)
		if !ok {
			return fail("INVALID_SERVER_RESPONSE", nil)
		}
		projected, err = transientstructuralresult.ProjectV2(got, q, managerID, root)
	} else {
		projected, err = transientstructuralresult.Project(got, q, managerID)
	}
	if err != nil {
		return fail("INVALID_SERVER_RESPONSE", err)
	}
	raw, err := json.Marshal(projected)
	if err != nil {
		return fail("INVALID_SERVER_RESPONSE", err)
	}
	return operation.Result{Artifact: raw}, nil
}
