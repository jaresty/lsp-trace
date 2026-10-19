package main

import (
	"bytes"
	"context"
	"os"
	"runtime"
	"testing"
	"time"

	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

func TestRunCensusProviderProbeV5RealManagerMalformedDocumentSymbolPoisonTuple(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("managed process test uses Darwin supervisor")
	}
	var starts, stops int
	core := func(options censusCLIOptions, cfg censusCoreConfig, _ censusCoreDependencies) censusPublicationOutcome {
		cfg.runner.start.Process = managedprocess.Spec{
			Path: os.Args[0], Args: []string{"-test.run=^TestRuntimeHelperServer$"}, Dir: cfg.runner.start.Process.Dir,
			Env: append(os.Environ(), "LSP_TRACE_RUNTIME_HELPER=1", "LSP_TRACE_RUNTIME_HELPER_MALFORMED_DOCUMENT_SYMBOL_FRAME=1"),
		}
		cfg.runner.timeout = 3 * time.Second
		cfg.runner.requestTimeout = time.Second
		cfg.runner.observeStart = func(sessionruntime.StartResult) { starts++ }
		cfg.runner.observeStop = func(session.LifecycleResult) { stops++ }
		code := runInitializedAcquisitionSession(cfg.runner, func(ctx context.Context, initialized initializedAcquisitionSession) int {
			_, _ = runInitializedCensusDiscovery(ctx, options, initialized.(*initializedAcquisitionRuntime), true)
			return 0
		})
		if code != 0 {
			t.Fatalf("session code=%d", code)
		}
		return deterministicProviderProbeOutcome(t)
	}

	baseline := runProviderProbeRunnerVersion(t, "v4", false, false, core)
	got := runProviderProbeRunnerVersion(t, "v5", true, false, core)
	if got.code != baseline.code || !bytes.Equal(got.stdout, baseline.stdout) || !bytes.Equal(got.stderr, baseline.stderr) {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V5_PUBLIC_PARITY baseline=%d/%q/%q got=%d/%q/%q", baseline.code, baseline.stdout, baseline.stderr, got.code, got.stdout, got.stderr)
	}
	if !got.probeExists || got.probeErr != nil || got.probeV5.Schema != privateProviderProbeSchemaV5 || got.probeV5.Transaction == nil {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V5_PRODUCTION_MANAGER_ROUTING exists=%t err=%v probe=%+v", got.probeExists, got.probeErr, got.probeV5)
	}
	tuple := got.probeV5.Transaction
	if tuple.SelectedResponseObserved || !tuple.RequestWriteCompleted || tuple.ProviderMessagesProcessed != 0 || tuple.ProviderResponseWriteCompleted || !tuple.FailurePostEffect || !tuple.TerminalPoisonObserved || tuple.TerminalLifecycle != sessionruntime.ProviderTransactionLifecyclePoisoned {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V5_POISON_TUPLE tuple=%+v", tuple)
	}
	if starts != 2 || stops != 2 || got.starterCalls != 1 || got.managerCalls != 1 {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V5_SINGLE_LIFECYCLE starts=%d stops=%d starter=%d manager=%d", starts, stops, got.starterCalls, got.managerCalls)
	}
}
