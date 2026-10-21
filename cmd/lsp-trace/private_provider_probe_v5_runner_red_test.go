package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/census"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

func TestRunCensusAcquisitionBudgetAuthorizationUsesPrivateEnumerationOnly(t *testing.T) {
	tests := []struct {
		name                      string
		privateEnumeration, probe bool
		maxNodes, want            int
	}{
		{name: "ordinary-default", maxNodes: 405, want: 1000},
		{name: "probe-only-does-not-authorize", probe: true, maxNodes: 405, want: 1000},
		{name: "private-crosses-legacy-1000", privateEnumeration: true, maxNodes: 405, want: 1001},
		{name: "private-maximum", privateEnumeration: true, maxNodes: 10000, want: 20191},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			workspace, root := t.TempDir(), t.TempDir()
			if err := os.Chmod(root, 0700); err != nil {
				t.Fatal(err)
			}
			probePath := ""
			if tc.probe {
				probePath = filepath.Join(root, "provider-probe.json")
			}
			coreCalls := 0
			core := func(options censusCLIOptions, cfg censusCoreConfig, _ censusCoreDependencies) censusPublicationOutcome {
				coreCalls++
				if options.PrivateEnumerationProjection != tc.privateEnumeration || options.PrivateProviderProbeByteTimeNotifications != tc.probe {
					t.Fatalf("ASSERT_CLI_CENSUS_BUDGET_OPTION_BINDING: options=%+v", options)
				}
				got := -1
				if cfg.limits.MaxRequests != nil {
					got = *cfg.limits.MaxRequests
				}
				if got != tc.want {
					t.Fatalf("ASSERT_CLI_CENSUS_ACQUISITION_REQUEST_BUDGET: private=%t probe=%t max_nodes=%d max_requests=%d want=%d", tc.privateEnumeration, tc.probe, tc.maxNodes, got, tc.want)
				}
				return deterministicProviderProbeOutcome(t)
			}
			deps, starterCalls, managerCalls := providerProbeRunnerDeps(t, core)
			args := append(providerProbeRunnerArgsVersion(workspace, root, probePath, "v6", tc.privateEnumeration), "--max-nodes", strconv.Itoa(tc.maxNodes))
			var stdout, stderr bytes.Buffer
			if code := runCensusWithDependencies(args, &stdout, &stderr, deps); code != 0 {
				t.Fatalf("ASSERT_CLI_CENSUS_BUDGET_RUNNER_ACCEPTED: code=%d core_calls=%d starter_calls=%d manager_calls=%d args=%q stderr=%q", code, coreCalls, *starterCalls, *managerCalls, args, stderr.Bytes())
			}
			if coreCalls != 1 {
				t.Fatalf("ASSERT_CLI_CENSUS_BUDGET_RUNNER_CORE_INVOKED_ONCE: core_calls=%d starter_calls=%d manager_calls=%d args=%q", coreCalls, *starterCalls, *managerCalls, args)
			}
		})
	}
}

func TestRunCensusNotificationBurstAuthorizationUsesPrivateEnumerationOnly(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("managed process test uses Darwin supervisor")
	}
	core := func(wantPrivate bool) func(censusCLIOptions, censusCoreConfig, censusCoreDependencies) censusPublicationOutcome {
		return func(options censusCLIOptions, cfg censusCoreConfig, _ censusCoreDependencies) censusPublicationOutcome {
			if options.PrivateEnumerationProjection != wantPrivate {
				t.Fatalf("ASSERT_CLI_PRIVATE_ENUMERATION_OPTION_BINDING: got=%t want=%t", options.PrivateEnumerationProjection, wantPrivate)
			}
			cfg.runner.start.Process = managedprocess.Spec{
				Path: os.Args[0], Args: []string{"-test.run=^TestRuntimeHelperServer$"}, Dir: cfg.runner.start.Process.Dir,
				Env: append(os.Environ(), "LSP_TRACE_RUNTIME_HELPER=1", "LSP_TRACE_RUNTIME_HELPER_DOCUMENT_SYMBOL_NOTIFICATION_BURST=1"),
			}
			cfg.runner.timeout = 3 * time.Second
			cfg.runner.requestTimeout = time.Second
			code := runInitializedAcquisitionSession(cfg.runner, func(ctx context.Context, initialized initializedAcquisitionSession) int {
				discovery, err := runInitializedCensusDiscovery(ctx, options, initialized.(*initializedAcquisitionRuntime), true)
				if err != nil || discovery.Accounting.FileDenominator != 3 || len(discovery.Accounting.Files) != 3 {
					t.Fatalf("ASSERT_CLI_CENSUS_NOTIFICATION_DISCOVERY_CLOSED: private=%t discovery=%+v err=%v", wantPrivate, discovery, err)
				}
				if wantPrivate {
					for _, file := range discovery.Accounting.Files {
						if file.Disposition != census.FileSelected {
							t.Fatalf("ASSERT_CLI_PRIVATE_ENUMERATION_NOTIFICATION_ROUTE: files=%+v", discovery.Accounting.Files)
						}
					}
					return 0
				}
				if discovery.Accounting.Files[0].Disposition != census.FileDocumentSymbolFailed || discovery.Accounting.Files[1].Disposition != census.FileIncomplete || discovery.Accounting.Files[2].Disposition != census.FileIncomplete {
					t.Fatalf("ASSERT_CLI_ORDINARY_NOTIFICATION_POISON_TAXONOMY: files=%+v", discovery.Accounting.Files)
				}
				return 0
			})
			if code != 0 {
				t.Fatalf("session code=%d", code)
			}
			return deterministicProviderProbeOutcome(t)
		}
	}

	ordinary := runProviderProbeRunnerVersion(t, "v5", false, false, core(false))
	probeOnly := runProviderProbeRunnerVersion(t, "v6", true, false, core(false))
	privateEnumeration := runPrivateEnumerationNotificationRunner(t, core(true))
	if probeOnly.code != ordinary.code || !bytes.Equal(probeOnly.stdout, ordinary.stdout) || !bytes.Equal(probeOnly.stderr, ordinary.stderr) {
		t.Fatalf("ASSERT_CLI_PROVIDER_PROBE_DOES_NOT_AUTHORIZE_NOTIFICATIONS ordinary=%d/%q/%q probe=%d/%q/%q", ordinary.code, ordinary.stdout, ordinary.stderr, probeOnly.code, probeOnly.stdout, probeOnly.stderr)
	}
	if !probeOnly.probeExists || probeOnly.probeErr == nil || probeOnly.probeV6.Composition == nil || probeOnly.probeV6.Composition.Outcome != string(sessionruntime.ProviderMessageOutcomeMessageCeiling) || len(probeOnly.probeV6.CauseCounts) == 0 || probeOnly.probeV6.CauseCounts[len(probeOnly.probeV6.CauseCounts)-1].Cause != string(session.ResourceExhausted) {
		t.Fatalf("ASSERT_CLI_PROVIDER_PROBE_COUNT_CEILING err=%v probe=%+v", probeOnly.probeErr, probeOnly.probeV6)
	}
	if privateEnumeration.code != 0 || privateEnumeration.probeExists {
		t.Fatalf("ASSERT_CLI_PRIVATE_ENUMERATION_WITHOUT_PROBE: code=%d probe=%t stderr=%q", privateEnumeration.code, privateEnumeration.probeExists, privateEnumeration.stderr)
	}
}

func runPrivateEnumerationNotificationRunner(t *testing.T, runCore func(censusCLIOptions, censusCoreConfig, censusCoreDependencies) censusPublicationOutcome) providerProbeRunnerResult {
	t.Helper()
	workspace, root := t.TempDir(), t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.go", "b.go", "c.go"} {
		if err := os.WriteFile(filepath.Join(workspace, name), []byte("package "+strings.TrimSuffix(name, ".go")+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	deps, starters, managers := providerProbeIntegrationRunnerDeps(t, runCore)
	var stdout, stderr bytes.Buffer
	code := runCensusWithDependencies(providerProbeRunnerArgsVersion(workspace, root, "", "v5", true), &stdout, &stderr, deps)
	return providerProbeRunnerResult{code: code, stdout: stdout.Bytes(), stderr: stderr.Bytes(), starterCalls: *starters, managerCalls: *managers}
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
