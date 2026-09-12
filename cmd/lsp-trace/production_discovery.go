package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"lsp-trace/acquisitionops"
	"lsp-trace/internal/lsp"
	"lsp-trace/sessionruntime"
)

func defaultDiscoveryLimits() acquisitionops.Limits {
	integer := func(value int) *int { return &value }
	return acquisitionops.Limits{
		MaxNodes: integer(100), MaxRequests: integer(1000), MaxEvidenceBytes: integer(4 << 20), MaxPathWork: integer(100000),
		TimeoutMS: integer(60000), RequestTimeoutMS: integer(30000), MaxResponseBytes: integer(4 << 20), MaxMessages: integer(64),
	}
}

type productionDiscoveryClient struct {
	runtime    *privateAcquisitionRuntime
	sessionID  string
	generation uint64
	timeout    time.Duration
}

func (c productionDiscoveryClient) SupportsDocumentSymbols() bool { return true }

func (c productionDiscoveryClient) call(ctx context.Context, method string, params any, out any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	result := c.runtime.RoundTrip(ctx, sessionruntime.RoundTripRequest{
		SessionID: c.sessionID, Generation: c.generation, Method: method, Params: raw,
		Deadline: time.Now().Add(c.timeout), MaxMessages: 64, MaxBytes: 4 << 20,
	})
	if result.Failure != "" {
		return fmt.Errorf("%s: %s", method, result.Failure)
	}
	if result.ServerError != nil {
		return fmt.Errorf("%s: server error %d: %s", method, result.ServerError.Code, result.ServerError.Message)
	}
	if len(result.Result) == 0 || string(result.Result) == "null" {
		return nil
	}
	return json.Unmarshal(result.Result, out)
}

func (c productionDiscoveryClient) DocumentSymbols(ctx context.Context, params lsp.DocumentSymbolParams) ([]lsp.DocumentSymbol, error) {
	var symbols []lsp.DocumentSymbol
	return symbols, c.call(ctx, "textDocument/documentSymbol", params, &symbols)
}

func (c productionDiscoveryClient) PrepareCallHierarchy(ctx context.Context, params lsp.PrepareCallHierarchyParams) ([]lsp.CallHierarchyItem, error) {
	var items []lsp.CallHierarchyItem
	return items, c.call(ctx, "textDocument/prepareCallHierarchy", params, &items)
}

func (c productionDiscoveryClient) OutgoingCalls(ctx context.Context, item lsp.CallHierarchyItem) ([]lsp.CallHierarchyOutgoingCall, bool, error) {
	var calls []lsp.CallHierarchyOutgoingCall
	err := c.call(ctx, "callHierarchy/outgoingCalls", lsp.CallHierarchyOutgoingCallsParams{Item: item}, &calls)
	return calls, true, err
}
