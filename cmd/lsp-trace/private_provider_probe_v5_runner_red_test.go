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

func TestRunCensusProviderProbeV6RealManagerNotificationBurstReachesTerminal(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("managed process test uses Darwin supervisor")
	}
	core := func(options censusCLIOptions, cfg censusCoreConfig, _ censusCoreDependencies) censusPublicationOutcome {
		wantRequests := 1000
		if options.PrivateProviderProbeByteTimeNotifications {
			wantRequests = 20191
		}
		if cfg.limits.MaxRequests == nil || *cfg.limits.MaxRequests != wantRequests {
			t.Fatalf("ASSERT_CLI_CENSUS_ACQUISITION_REQUEST_BUDGET: private=%t max_requests=%v want=%d", options.PrivateProviderProbeByteTimeNotifications, cfg.limits.MaxRequests, wantRequests)
		}
		cfg.runner.start.Process = managedprocess.Spec{
			Path: os.Args[0], Args: []string{"-test.run=^TestRuntimeHelperServer$"}, Dir: cfg.runner.start.Process.Dir,
			Env: append(os.Environ(), "LSP_TRACE_RUNTIME_HELPER=1", "LSP_TRACE_RUNTIME_HELPER_DOCUMENT_SYMBOL_NOTIFICATION_BURST=1"),
		}
		cfg.runner.timeout = 3 * time.Second
		cfg.runner.requestTimeout = time.Second
		code := runInitializedAcquisitionSession(cfg.runner, func(ctx context.Context, initialized initializedAcquisitionSession) int {
			_, _ = runInitializedCensusDiscovery(ctx, options, initialized.(*initializedAcquisitionRuntime), true)
			return 0
		})
		if code != 0 {
			t.Fatalf("session code=%d", code)
		}
		return deterministicProviderProbeOutcome(t)
	}

	baseline := runProviderProbeRunnerVersion(t, "v5", false, false, core)
	ordinary := runProviderProbeRunnerVersion(t, "v5", true, false, core)
	got := runProviderProbeRunnerVersion(t, "v6", true, false, core)
	if got.code != baseline.code || !bytes.Equal(got.stdout, baseline.stdout) || !bytes.Equal(got.stderr, baseline.stderr) {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V6_PUBLIC_PARITY baseline=%d/%q/%q got=%d/%q/%q", baseline.code, baseline.stdout, baseline.stderr, got.code, got.stdout, got.stderr)
	}
	if !ordinary.probeExists || ordinary.probeErr == nil || ordinary.probeV5.Composition == nil || ordinary.probeV5.Composition.Outcome != string(sessionruntime.ProviderMessageOutcomeMessageCeiling) || len(ordinary.probeV5.CauseCounts) == 0 || ordinary.probeV5.CauseCounts[len(ordinary.probeV5.CauseCounts)-1].Cause != string(session.ResourceExhausted) {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V6_ORDINARY_COUNT_CEILING err=%v probe=%+v", ordinary.probeErr, ordinary.probeV5)
	}
	if !got.probeExists || got.probeErr != nil || got.probeV6.Transaction == nil || got.probeV6.Composition == nil {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V6_PRODUCTION_MANAGER_ROUTING err=%v probe=%+v", got.probeErr, got.probeV6)
	}
	if got.probeV6.Composition.Outcome != string(sessionruntime.ProviderMessageOutcomeAcceptedTerminalResponse) || got.probeV6.Transaction.ObservedNotificationCount != 65 || got.probeV6.Transaction.ObservedControlCount != 1 || !got.probeV6.Transaction.TerminalResponseObserved || !got.probeV6.Transaction.ReadCompleted || got.probeV6.Transaction.TerminalLifecycle != sessionruntime.ProviderTransactionLifecycleReady {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V6_NOTIFICATION_BURST_TERMINAL composition=%+v transaction=%+v", got.probeV6.Composition, got.probeV6.Transaction)
	}
}

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
	if tuple.SelectedResponseObserved || !tuple.RequestWriteCompleted || tuple.ProviderMessagesProcessed != 0 || tuple.SelectedResponseReadCompleted || !tuple.FailurePostEffect || !tuple.TerminalPoisonObserved || tuple.TerminalLifecycle != sessionruntime.ProviderTransactionLifecyclePoisoned {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V5_POISON_TUPLE tuple=%+v", tuple)
	}
	if starts != 2 || stops != 2 || got.starterCalls != 1 || got.managerCalls != 1 {
		t.Fatalf("ASSERT_PROVIDER_PROBE_V5_SINGLE_LIFECYCLE starts=%d stops=%d starter=%d manager=%d", starts, stops, got.starterCalls, got.managerCalls)
	}
}
