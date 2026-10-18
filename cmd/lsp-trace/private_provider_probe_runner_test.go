package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censusenumerationprojection"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

type providerProbeRunnerResult struct {
	code                       int
	stdout, stderr             []byte
	discoveryLine, receiptLine []byte
	enumerationArtifact        []byte
	enumerationReceipt         censusenumerationprojection.Receipt
	publication                map[string][]byte
	starterCalls, managerCalls int
	starts, stops              int
	probe                      privateProviderProbeArtifact
	probeExists                bool
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
	return runProviderProbeRunnerInWorkspace(t, t.TempDir(), enableProbe, enumeration, runCore)
}

func runProviderProbeRunnerInWorkspace(t *testing.T, workspace string, enableProbe, enumeration bool, runCore func(censusCLIOptions, censusCoreConfig, censusCoreDependencies) censusPublicationOutcome) providerProbeRunnerResult {
	t.Helper()
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.go", "b.go", "c.go"} {
		if err := os.WriteFile(filepath.Join(workspace, name), []byte("package "+strings.TrimSuffix(name, ".go")+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	probe := ""
	if enableProbe {
		probe = filepath.Join(root, "provider-probe.json")
	}
	deps, starters, managers := providerProbeRunnerDeps(t, runCore)
	var stdout, stderr bytes.Buffer
	code := runCensusWithDependencies(providerProbeRunnerArgs(workspace, root, probe, enumeration), &stdout, &stderr, deps)
	result := providerProbeRunnerResult{code: code, stdout: append([]byte(nil), stdout.Bytes()...), stderr: append([]byte(nil), stderr.Bytes()...), publication: providerProbePublicationFiles(t, root), starterCalls: *starters, managerCalls: *managers}
	if enumeration {
		lines := bytes.Split(bytes.TrimSuffix(result.stderr, []byte{'\n'}), []byte{'\n'})
		if len(lines) != 2 {
			t.Fatalf("enumeration stderr lines=%d stderr=%q", len(lines), result.stderr)
		}
		result.discoveryLine = append([]byte(nil), lines[0]...)
		result.receiptLine = append([]byte(nil), lines[1]...)
		decoder := json.NewDecoder(bytes.NewReader(lines[1]))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&result.enumerationReceipt); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
			t.Fatalf("enumeration receipt=%q err=%v", lines[1], err)
		}
		canonicalReceipt, err := censusenumerationprojection.MarshalReceipt(result.enumerationReceipt)
		if err != nil || !bytes.Equal(canonicalReceipt, append(append([]byte(nil), lines[1]...), '\n')) {
			t.Fatalf("noncanonical enumeration receipt=%q err=%v", lines[1], err)
		}
		if result.enumerationReceipt.Status != "PUBLISHED" || result.enumerationReceipt.Selector == "" {
			t.Fatalf("enumeration not published: %+v", result.enumerationReceipt)
		}
		result.enumerationArtifact, err = os.ReadFile(filepath.Join(root, result.enumerationReceipt.Selector))
		if err != nil {
			t.Fatalf("resolve enumeration selector=%q: %v", result.enumerationReceipt.Selector, err)
		}
		projection, err := censusenumerationprojection.Decode(result.enumerationArtifact)
		if err != nil {
			t.Fatal(err)
		}
		canonicalArtifact, err := censusenumerationprojection.EncodeCanonical(projection)
		if err != nil || !bytes.Equal(canonicalArtifact, result.enumerationArtifact) {
			t.Fatalf("noncanonical enumeration artifact selector=%q err=%v", result.enumerationReceipt.Selector, err)
		}
	}
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

func TestRunCensusProviderProbeAbsentAndEnabledPreserveEnumerationPublication(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("managed process test uses Darwin supervisor")
	}
	core := func(options censusCLIOptions, cfg censusCoreConfig, deps censusCoreDependencies) censusPublicationOutcome {
		cfg.runner.start.Process = managedprocess.Spec{Path: os.Args[0], Args: []string{"-test.run=^TestRuntimeHelperServer$"}, Dir: cfg.runner.start.Process.Dir, Env: append(os.Environ(), "LSP_TRACE_RUNTIME_HELPER=1", "LSP_TRACE_RUNTIME_HELPER_MALFORMED_DOCUMENT_SYMBOL=1")}
		cfg.runner.timeout = 3 * time.Second
		cfg.runner.requestTimeout = time.Second
		runSession, discover := deps.runSession, deps.discover
		var actual *initializedAcquisitionRuntime
		deps.runSession = func(runner initializedAcquisitionRunnerConfig, callback func(context.Context, *initializedAcquisitionRuntime) int) int {
			return runSession(runner, func(ctx context.Context, runtime *initializedAcquisitionRuntime) int {
				actual = runtime
				fixed := *runtime
				fixed.sessionID, fixed.generation = "provider-probe-parity", 1
				return callback(ctx, &fixed)
			})
		}
		deps.discover = func(ctx context.Context, options censusCLIOptions, _ *initializedAcquisitionRuntime, callHierarchy bool) (censusacquisition.Discovery, error) {
			discovery, err := discover(ctx, options, actual, callHierarchy)
			discovery.Session = censusacquisition.SessionIdentity{SessionID: "provider-probe-parity", Generation: 1}
			return discovery, err
		}
		return runCensusCore(options, cfg, deps)
	}
	workspace := t.TempDir()
	baseline := runProviderProbeRunnerInWorkspace(t, workspace, false, true, core)
	enabled := runProviderProbeRunnerInWorkspace(t, workspace, true, true, core)
	if baseline.code != 1 || enabled.code != baseline.code || len(baseline.stdout) != 0 || !bytes.Equal(baseline.stdout, enabled.stdout) {
		t.Fatalf("exit/stdout drift baseline=%d/%q enabled=%d/%q", baseline.code, baseline.stdout, enabled.code, enabled.stdout)
	}
	if !bytes.Equal(baseline.discoveryLine, enabled.discoveryLine) || !bytes.Equal(baseline.receiptLine, enabled.receiptLine) || !bytes.Equal(baseline.stderr, enabled.stderr) {
		t.Fatalf("diagnostic/receipt drift baseline=%q enabled=%q", baseline.stderr, enabled.stderr)
	}
	if baseline.enumerationReceipt.Status != "PUBLISHED" || enabled.enumerationReceipt.Status != "PUBLISHED" || !reflect.DeepEqual(baseline.enumerationReceipt, enabled.enumerationReceipt) || len(baseline.enumerationArtifact) == 0 || !bytes.Equal(baseline.enumerationArtifact, enabled.enumerationArtifact) {
		t.Fatalf("enumeration drift baseline=%+v/%d enabled=%+v/%d", baseline.enumerationReceipt, len(baseline.enumerationArtifact), enabled.enumerationReceipt, len(enabled.enumerationArtifact))
	}
	sum := sha256.Sum256(baseline.enumerationArtifact)
	wantDigest := "sha256:" + hex.EncodeToString(sum[:])
	if baseline.enumerationReceipt.Selector != censusenumerationprojection.Selector(baseline.enumerationArtifact) || baseline.enumerationReceipt.Digest != wantDigest || baseline.enumerationReceipt.ByteLength != uint64(len(baseline.enumerationArtifact)) {
		t.Fatalf("enumeration identity receipt=%+v digest=%s length=%d", baseline.enumerationReceipt, wantDigest, len(baseline.enumerationArtifact))
	}
	probeBytes, probeAdded := enabled.publication["provider-probe.json"]
	delete(enabled.publication, "provider-probe.json")
	if !probeAdded || len(probeBytes) == 0 || !reflect.DeepEqual(baseline.publication, enabled.publication) {
		t.Fatalf("publication drift baseline=%v enabled=%v probe_added=%t", baseline.publication, enabled.publication, probeAdded)
	}
	if baseline.starterCalls != 1 || baseline.managerCalls != 1 || enabled.starterCalls != 1 || enabled.managerCalls != 1 || !enabled.probeExists || enabled.probe.Work == 0 {
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
