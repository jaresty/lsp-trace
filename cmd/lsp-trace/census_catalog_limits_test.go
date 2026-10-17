package main

import (
	"context"
	"net/url"
	"os"
	"testing"
	"time"

	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/liveprojection"
	"lsp-trace/sessionruntime"
)

type censusFreshCaptureRuntimeStub struct{ workspace string }

func (r censusFreshCaptureRuntimeStub) PrepareDocument(_ context.Context, request sessionruntime.DocumentRequest) sessionruntime.DocumentResult {
	u, err := url.Parse(request.URI)
	if err != nil {
		return sessionruntime.DocumentResult{Failure: sessionruntime.DocumentSupplyUnavailable}
	}
	content, err := os.ReadFile(u.Path)
	if err != nil {
		return sessionruntime.DocumentResult{Failure: sessionruntime.DocumentSupplyUnavailable}
	}
	return sessionruntime.DocumentResult{URI: request.URI, LanguageID: request.LanguageID, Version: 1, Supply: &sessionruntime.DocumentSupply{SessionID: request.SessionID, Generation: request.Generation, URI: request.URI, DocumentVersion: 1, Content: content}}
}

func (censusFreshCaptureRuntimeStub) RoundTrip(context.Context, sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult {
	return sessionruntime.RoundTripResult{}
}

func (r censusFreshCaptureRuntimeStub) WorkspaceRoot(string, uint64) (string, bool) {
	return r.workspace, r.workspace != ""
}

func (censusFreshCaptureRuntimeStub) SessionLanguageID(string, uint64) (string, bool) {
	return "go", true
}

func censusFreshCaptureTestDependencies(ctx context.Context, workspace string) *censuscontinuation.FreshCaptureDependencies {
	return censusFreshCaptureDependencies(ctx, censusFreshCaptureRuntimeStub{workspace: workspace}, time.Second)
}

func TestCensusFreshCaptureSeparatesPreparationAndResolverMessageLimits(t *testing.T) {
	deps := censusFreshCaptureDependencies(context.Background(), censusFreshCaptureRuntimeStub{}, time.Second)
	if deps == nil {
		t.Fatal("ASSERT_CLI_FRESH_CAPTURE_DEPENDENCIES_PRESENT")
	}
	if got := deps.Limits; got.MaxDocuments != 100 || got.MaxMessages != 100 || got.MaxWork != 100 {
		t.Fatalf("ASSERT_CLI_MANAGED_PREPARATION_100_URI_CAPACITY: %+v", got)
	}
	resolver, ok := deps.Resolver.(liveprojection.FullDefinitionResolver)
	if !ok || resolver.Limits.MaxMessages != 32 {
		t.Fatalf("ASSERT_CLI_RESOLVER_SEPARATE_32_MESSAGE_BUDGET: resolver=%T limits=%+v", deps.Resolver, resolver.Limits)
	}
	preparer, ok := deps.Preparer.(censuscontinuation.ManagedPreparer)
	if !ok || preparer.Workspace == nil || preparer.LanguageResolver == nil {
		t.Fatalf("ASSERT_CLI_MANAGED_PREPARATION_ROUTING_INJECTED: preparer=%T workspace=%t language_resolver=%t", deps.Preparer, preparer.Workspace != nil, preparer.LanguageResolver != nil)
	}
}
