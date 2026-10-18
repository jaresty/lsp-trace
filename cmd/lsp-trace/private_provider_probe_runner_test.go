package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

type providerProbeRunnerResult struct {
	code           int
	stdout, stderr []byte
	publication    map[string][]byte
	starterCalls   int
	managerCalls   int
	starts, stops  int
	probe          privateProviderProbeArtifact
	probeExists    bool
}

func providerProbeRunnerArgs(workspace, root, probe string, enumeration bool) []string {
	args := []string{"--machine", "--workspace", workspace, "--source", ".", "--publication-root", root, "--server", "helper", "--request-timeout", "1s"}
	if enumeration {
		args = append(args, "--private-enumeration-projection")
	}
	if probe != "" {
		args = append(args, "--private-provider-probe-path", probe)
	}
	return args
}

func providerProbePublicationFiles(t *testing.T, root string) map[string][]byte {
	t.Helper()
	got := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		got[filepath.ToSlash(rel)], err = os.ReadFile(path)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func providerProbeRunnerDeps(t *testing.T, runCore func(censusCLIOptions, censusCoreConfig, censusCoreDependencies) censusPublicationOutcome) (censusRunnerDependencies, *int, *int) {
	t.Helper()
	deps := productionCensusRunnerDependencies()
	starterCalls, managerCalls := 0, 0
	deps.lookPath = func(string) (string, error) { return os.Args[0], nil }
	deps.newStarter = func() (sessionruntime.Starter, error) {
		starterCalls++
		supervisor, err := managedprocess.NewLocalDarwinSupervisor(managedprocess.Options{StderrLimit: 4096, GracePeriod: 100 * time.Millisecond})
		if err != nil {
			return nil, err
		}
		return sessionruntime.ManagedStarter{Manager: supervisor}, nil
	}
	originalManager := deps.newManager
	deps.newManager = func(cfg sessionruntime.Config) (*sessionruntime.Manager, error) {
		managerCalls++
		return originalManager(cfg)
	}
	deps.runCore = runCore
	return deps, &starterCalls, &managerCalls
}

func runProviderProbeRunner(t *testing.T, enableProbe, enumeration bool, runCore func(censusCLIOptions, censusCoreConfig, censusCoreDependencies) censusPublicationOutcome) providerProbeRunnerResult {
	t.Helper()
	workspace, root := t.TempDir(), t.TempDir()
	for _, name := range []string{"a.go", "b.go", "c.go"} {
		if err := os.WriteFile(filepath.Join(workspace, name), []byte("package "+strings.TrimSuffix(name, ".go")+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	probe := ""
	if enableProbe {
		probe = probePath(t)
	}
	deps, starters, managers := providerProbeRunnerDeps(t, runCore)
	var stdout, stderr bytes.Buffer
	code := runCensusWithDependencies(providerProbeRunnerArgs(workspace, root, probe, enumeration), &stdout, &stderr, deps)
	result := providerProbeRunnerResult{code: code, stdout: append([]byte(nil), stdout.Bytes()...), stderr: append([]byte(nil), stderr.Bytes()...), publication: providerProbePublicationFiles(t, root), starterCalls: *starters, managerCalls: *managers}
	if probe != "" {
		raw, err := os.ReadFile(probe)
		if err == nil {
			result.probe, err = parsePrivateProviderProbe(raw)
			if err != nil {
				t.Fatal(err)
			}
			result.probeExists = true
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	return result
}

func deterministicProviderProbeOutcome(t *testing.T) censusPublicationOutcome {
	t.Helper()
	projection := censusPublicationProjection(t, 1)
	return successfulCorePublication(t, censusPublicationCapabilityFor(t, projection))
}

func TestRunCensusProviderProbeAbsentAndEnabledPreservePublicOutcomeAndPublication(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("managed process test uses Darwin supervisor")
	}
	core := func(options censusCLIOptions, cfg censusCoreConfig, _ censusCoreDependencies) censusPublicationOutcome {
		if err := os.WriteFile(filepath.Join(options.PublicationRoot, "publication.bin"), []byte("identical-publication"), 0600); err != nil {
			t.Fatal(err)
		}
		if cfg.runner.providerProbe != nil {
			cfg.runner.providerProbe.observe("DOCUMENT_SYMBOL", "DECODE_FAILURE", "DECODE_FAILURE", false)
		}
		return deterministicProviderProbeOutcome(t)
	}
	baseline := runProviderProbeRunner(t, false, true, core)
	enabled := runProviderProbeRunner(t, true, true, core)
	if baseline.code != enabled.code || !bytes.Equal(baseline.stdout, enabled.stdout) || !bytes.Equal(baseline.stderr, enabled.stderr) || !reflect.DeepEqual(baseline.publication, enabled.publication) {
		t.Fatalf("public drift baseline=%d/%q/%q/%v enabled=%d/%q/%q/%v", baseline.code, baseline.stdout, baseline.stderr, baseline.publication, enabled.code, enabled.stdout, enabled.stderr, enabled.publication)
	}
	if baseline.starterCalls != 1 || baseline.managerCalls != 1 || enabled.starterCalls != 1 || enabled.managerCalls != 1 || !enabled.probeExists || enabled.probe.Work != 1 {
		t.Fatalf("wiring baseline=%+v enabled=%+v", baseline, enabled)
	}
}

func TestRunCensusProviderProbeRealSessionSingleLifecycleAndAllFailureAccounting(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("managed process test uses Darwin supervisor")
	}
	var starts, stops int
	core := func(options censusCLIOptions, cfg censusCoreConfig, _ censusCoreDependencies) censusPublicationOutcome {
		cfg.runner.start.Process = managedprocess.Spec{Path: os.Args[0], Args: []string{"-test.run=^TestRuntimeHelperServer$"}, Dir: cfg.runner.start.Process.Dir, Env: append(os.Environ(), "LSP_TRACE_RUNTIME_HELPER=1", "LSP_TRACE_RUNTIME_HELPER_MALFORMED_DOCUMENT_SYMBOL=1")}
		cfg.runner.timeout = 3 * time.Second
		cfg.runner.requestTimeout = time.Second
		cfg.runner.observeStart = func(sessionruntime.StartResult) { starts++ }
		cfg.runner.observeStop = func(session.LifecycleResult) { stops++ }
		code := runInitializedAcquisitionSession(cfg.runner, func(ctx context.Context, initialized initializedAcquisitionSession) int {
			discovery, err := runInitializedCensusDiscovery(ctx, options, initialized.(*initializedAcquisitionRuntime), true)
			if err != nil || discovery.Accounting.FileDenominator != 3 {
				t.Fatalf("discovery denominator=%d err=%v", discovery.Accounting.FileDenominator, err)
			}
			return 0
		})
		if code != 0 {
			t.Fatalf("session code=%d", code)
		}
		return deterministicProviderProbeOutcome(t)
	}
	got := runProviderProbeRunner(t, true, false, core)
	if got.code != 0 || starts != 1 || stops != 1 || got.starterCalls != 1 || got.managerCalls != 1 {
		t.Fatalf("lifecycle code=%d starts=%d stops=%d starter=%d manager=%d stderr=%q", got.code, starts, stops, got.starterCalls, got.managerCalls, got.stderr)
	}
	wantCounts := []privateProviderProbeCount{{"DECODE_FAILURE", 3}}
	if got.probe.Work != 6 || got.probe.FileDenominator != 3 || len(got.probe.Records) != 1 || got.probe.Records[0].Sequence != 2 || !reflect.DeepEqual(got.probe.CauseCounts, wantCounts) {
		t.Fatalf("probe=%+v", got.probe)
	}
}

func TestRunCensusProviderProbeSetupAndFinalizeFailuresArePrivate(t *testing.T) {
	outcome := deterministicProviderProbeOutcome(t)
	core := func(_ censusCLIOptions, _ censusCoreConfig, _ censusCoreDependencies) censusPublicationOutcome {
		return outcome
	}
	workspace, root := t.TempDir(), t.TempDir()
	deps, starters, managers := providerProbeRunnerDeps(t, core)
	badProbe := filepath.Join(t.TempDir(), "missing", "probe.json")
	var stdout, stderr bytes.Buffer
	code := runCensusWithDependencies(providerProbeRunnerArgs(workspace, root, badProbe, false), &stdout, &stderr, deps)
	if code != 1 || stdout.Len() != 0 || *starters != 0 || *managers != 0 || strings.Contains(stderr.String(), badProbe) {
		t.Fatalf("setup code=%d starts=%d/%d stdout=%q stderr=%q", code, *starters, *managers, stdout.Bytes(), stderr.Bytes())
	}

	baseline := runProviderProbeRunner(t, false, false, core)
	probePath := probePath(t)
	deps, starters, managers = providerProbeRunnerDeps(t, func(o censusCLIOptions, c censusCoreConfig, d censusCoreDependencies) censusPublicationOutcome {
		if err := os.Chmod(filepath.Dir(probePath), 0755); err != nil {
			t.Fatal(err)
		}
		return core(o, c, d)
	})
	stdout.Reset()
	stderr.Reset()
	code = runCensusWithDependencies(providerProbeRunnerArgs(t.TempDir(), t.TempDir(), probePath, false), &stdout, &stderr, deps)
	if code != baseline.code || !bytes.Equal(stdout.Bytes(), baseline.stdout) || !bytes.Equal(stderr.Bytes(), baseline.stderr) || *starters != 1 || *managers != 1 {
		t.Fatalf("finalize drift code=%d/%d stdout=%q/%q stderr=%q/%q starts=%d/%d", code, baseline.code, stdout.Bytes(), baseline.stdout, stderr.Bytes(), baseline.stderr, *starters, *managers)
	}
}

func TestRunCensusProviderProbeRejectionsPrecedeStarterEffects(t *testing.T) {
	path := probePath(t)
	base := []string{"--machine", "--workspace", t.TempDir(), "--publication-root", t.TempDir(), "--server", "helper"}
	cases := map[string][]string{
		"duplicate":  append(append([]string{}, base...), "--private-provider-probe-path", path, "--private-provider-probe-path", path),
		"nonmachine": {"--workspace", t.TempDir(), "--publication-root", t.TempDir(), "--server", "helper", "--private-provider-probe-path", path},
		"catalog":    append(append([]string{}, base...), "--catalog", "--catalog-config", "host.json", "--private-provider-probe-path", path),
		"resume":     {"--machine", "--resume", "sha256:" + strings.Repeat("a", 64), "--catalog-config", "host.json", "--publication-root", t.TempDir(), "--private-provider-probe-path", path},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			deps := productionCensusRunnerDependencies()
			starter, manager := 0, 0
			deps.newStarter = func() (sessionruntime.Starter, error) { starter++; return nil, errors.New("unexpected") }
			deps.newManager = func(sessionruntime.Config) (*sessionruntime.Manager, error) {
				manager++
				return nil, errors.New("unexpected")
			}
			var stdout, stderr bytes.Buffer
			if code := runCensusWithDependencies(args, &stdout, &stderr, deps); code != 1 || stdout.Len() != 0 || starter != 0 || manager != 0 {
				t.Fatalf("code=%d starter=%d manager=%d stdout=%q stderr=%q", code, starter, manager, stdout.Bytes(), stderr.Bytes())
			}
		})
	}
}
