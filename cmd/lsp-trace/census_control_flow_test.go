package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/acquisitionops"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censuscontrolflow"
	"lsp-trace/internal/censusdiagnostic"
	"lsp-trace/internal/operation"
	"lsp-trace/sessionruntime"
)

func controlFlowRecorder(t *testing.T) (string, *censuscontrolflow.Recorder) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "control.ndjson")
	r, err := censuscontrolflow.NewRecorder(path, censuscontrolflow.DefaultMaxBytes, censuscontrolflow.DefaultMaxRecords)
	if err != nil {
		t.Fatal(err)
	}
	return path, r
}

func baseControlFlowDeps(t *testing.T) censusCoreDependencies {
	t.Helper()
	projection := censusPublicationProjection(t, 1)
	return censusCoreDependencies{
		runSession: func(_ initializedAcquisitionRunnerConfig, cb func(context.Context, *initializedAcquisitionRuntime) int) int {
			return cb(context.Background(), &initializedAcquisitionRuntime{sessionID: "publication-session", generation: 7})
		},
		discover: func(context.Context, censusCLIOptions, *initializedAcquisitionRuntime, bool) (censusacquisition.Discovery, error) {
			return discoveryFromProjection(projection), nil
		},
		acquire: func(context.Context, *initializedAcquisitionRuntime, censusacquisition.Discoverer, acquisitionops.Limits, int, int, int) (censusAssembly, error) {
			return censusAssembly{}, errors.New("core failure")
		},
		capability: func(censusAssembly) (censusPublicationCapability, error) {
			return censusPublicationCapability{}, errors.New("unused")
		},
		publish: func(context.Context, censusPublicationCapability, string) censusPublicationOutcome {
			return censusPublicationOutcome{}
		},
	}
}

func TestControlFlowObserverWriteOnce(t *testing.T) {
	o := newCensusControlFlowObserver()
	o.cause(censuscontrolflow.CoreFailed, censuscontrolflow.PhasePlanning)
	o.cause(censuscontrolflow.FinalNonzeroFallback, censuscontrolflow.PhaseCompleted)
	o.batch(censuscontrolflow.BatchAccepted)
	o.batch(censuscontrolflow.BatchRejected)
	r, ok := o.record()
	if !ok || r.FirstCause != censuscontrolflow.CoreFailed || r.CorePhase != censuscontrolflow.PhasePlanning || r.BatchRecord != censuscontrolflow.BatchAccepted {
		t.Fatalf("record=%+v ok=%v", r, ok)
	}
}

func TestRunCensusCoreControlFlowBranchMatrix(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*censusCoreDependencies)
		cause  censuscontrolflow.FirstCause
		phase  censuscontrolflow.CorePhase
	}{
		{"callback-not-entered", func(d *censusCoreDependencies) {
			d.runSession = func(initializedAcquisitionRunnerConfig, func(context.Context, *initializedAcquisitionRuntime) int) int {
				return 1
			}
		}, censuscontrolflow.CallbackNotEntered, censuscontrolflow.PhaseNotEntered},
		{"callback-runtime-invalid", func(d *censusCoreDependencies) {
			d.runSession = func(_ initializedAcquisitionRunnerConfig, cb func(context.Context, *initializedAcquisitionRuntime) int) int {
				return cb(context.Background(), nil)
			}
		}, censuscontrolflow.CallbackRuntimeInvalid, censuscontrolflow.PhaseInput},
		{"core-failed", func(*censusCoreDependencies) {}, censuscontrolflow.CoreFailed, censuscontrolflow.PhaseBatchAcquire},
		{"post-core-runtime-invalid", func(d *censusCoreDependencies) {
			d.acquire = func(_ context.Context, runtime *initializedAcquisitionRuntime, _ censusacquisition.Discoverer, _ acquisitionops.Limits, _, _, _ int) (censusAssembly, error) {
				runtime.generation = 8
				return censusAssembly{}, nil
			}
		}, censuscontrolflow.PostCoreRuntimeInvalid, censuscontrolflow.PhaseCompleted},
		{"post-capability-runtime-invalid", func(d *censusCoreDependencies) {
			projection := censusPublicationProjection(t, 1)
			ctx, cancel := context.WithCancel(context.Background())
			d.runSession = func(_ initializedAcquisitionRunnerConfig, cb func(context.Context, *initializedAcquisitionRuntime) int) int {
				return cb(ctx, &initializedAcquisitionRuntime{sessionID: "publication-session", generation: 7})
			}
			d.acquire = func(context.Context, *initializedAcquisitionRuntime, censusacquisition.Discoverer, acquisitionops.Limits, int, int, int) (censusAssembly, error) {
				capability := censusPublicationCapabilityFor(t, projection)
				return censusAssembly{state: capability.state, token: capability.token}, nil
			}
			d.capability = func(a censusAssembly) (censusPublicationCapability, error) {
				capability, err := a.publicationCapability()
				cancel()
				return capability, err
			}
		}, censuscontrolflow.PostCoreRuntimeInvalid, censuscontrolflow.PhaseCompleted},
		{"final-nonzero-fallback", func(d *censusCoreDependencies) {
			projection := censusPublicationProjection(t, 1)
			d.acquire = func(context.Context, *initializedAcquisitionRuntime, censusacquisition.Discoverer, acquisitionops.Limits, int, int, int) (censusAssembly, error) {
				capability := censusPublicationCapabilityFor(t, projection)
				return censusAssembly{state: capability.state, token: capability.token}, nil
			}
			d.capability = func(a censusAssembly) (censusPublicationCapability, error) { return a.publicationCapability() }
			d.publish = func(context.Context, censusPublicationCapability, string) censusPublicationOutcome {
				return successfulCorePublication(t, censusPublicationCapabilityFor(t, projection))
			}
			d.runSession = func(_ initializedAcquisitionRunnerConfig, cb func(context.Context, *initializedAcquisitionRuntime) int) int {
				_ = cb(context.Background(), &initializedAcquisitionRuntime{sessionID: "publication-session", generation: 7})
				return 1
			}
		}, censuscontrolflow.FinalNonzeroFallback, censuscontrolflow.PhaseCompleted},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := baseControlFlowDeps(t)
			tc.mutate(&d)
			out := runCensusCore(censusCLIOptions{}, censusCoreConfig{}, d)
			if out.controlFlow == nil || out.controlFlow.FirstCause != tc.cause || out.controlFlow.CorePhase != tc.phase || out.controlFlow.BatchRecord != censuscontrolflow.BatchNotAttempted {
				t.Fatalf("out=%+v control=%+v", out, out.controlFlow)
			}
		})
	}
}

func TestControlFlowFlagHiddenFreshOnlyAndSetupBeforeManager(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.ndjson")
	base := []string{"census", "--workspace", "work", "--source", ".", "--publication-root", "/private", "--server", "gopls", "--private-control-flow-diagnostic-path", path}
	o, err := parseCensusCLIOptions(base)
	if err != nil || o.PrivateControlFlowDiagnosticPath != path {
		t.Fatalf("options=%+v err=%v", o, err)
	}
	if strings.Contains(censusUsage, "control-flow") || strings.Contains(usageText, "control-flow") {
		t.Fatal("hidden flag leaked")
	}
	resume := []string{"census", "--resume", "sha256:" + strings.Repeat("a", 64), "--catalog-config", "host.json", "--publication-root", "/private", "--private-control-flow-diagnostic-path", path}
	if _, err := parseCensusCLIOptions(resume); err == nil {
		t.Fatal("resume accepted")
	}

	pub := t.TempDir()
	_ = os.Chmod(pub, 0700)
	workspace := t.TempDir()
	bad := t.TempDir()
	_ = os.Chmod(bad, 0755)
	deps := productionCensusRunnerDependencies()
	deps.lookPath = func(string) (string, error) { return "/bin/echo", nil }
	deps.abs = func(p string) (string, error) { return p, nil }
	starter, manager, core := 0, 0, 0
	deps.newStarter = func() (sessionruntime.Starter, error) { starter++; return nil, nil }
	deps.newManager = func(sessionruntime.Config) (*sessionruntime.Manager, error) { manager++; return nil, errors.New("no") }
	deps.runCore = func(censusCLIOptions, censusCoreConfig, censusCoreDependencies) censusPublicationOutcome {
		core++
		return censusPublicationOutcome{}
	}
	var stdout, stderr strings.Builder
	code := runCensusWithDependencies([]string{"--machine", "--workspace", workspace, "--source", ".", "--publication-root", pub, "--server", "gopls", "--private-control-flow-diagnostic-path", filepath.Join(bad, "x")}, &stdout, &stderr, deps)
	if code != 1 || starter != 0 || manager != 0 || core != 0 || stdout.Len() != 0 {
		t.Fatalf("code=%d calls=%d/%d/%d stdout=%q stderr=%q", code, starter, manager, core, stdout.String(), stderr.String())
	}
}

func TestControlFlowV1AppendDispositionAndOriginalCause(t *testing.T) {
	for _, tc := range []struct {
		name     string
		recorder func(*testing.T) *censusdiagnostic.Recorder
		want     censuscontrolflow.BatchRecord
	}{
		{"not-attempted", func(*testing.T) *censusdiagnostic.Recorder { return nil }, censuscontrolflow.BatchNotAttempted},
		{"accepted", func(t *testing.T) *censusdiagnostic.Recorder { _, r := newPrivateAcquisitionRecorder(t); return r }, censuscontrolflow.BatchAccepted},
		{"rejected", func(t *testing.T) *censusdiagnostic.Recorder {
			path, r := newPrivateAcquisitionRecorder(t)
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(path, 0700); err != nil {
				t.Fatal(err)
			}
			return r
		}, censuscontrolflow.BatchRejected},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cause := errors.New("original cause")
			session := &fakeBatchSession{id: "s", generation: 7, respond: func(context.Context, operation.Request) (operation.Result, *operation.Failure) {
				return operation.Result{}, &operation.Failure{Code: operation.FailureInternal, Err: cause}
			}}
			observer := newCensusControlFlowObserver()
			observer.cause(censuscontrolflow.CoreFailed, censuscontrolflow.PhaseBatchAcquire)
			a := newCensusBatchAcquirer(session, acquisitionops.Limits{}, tc.recorder(t))
			a.controlFlow = observer
			_, failure := a.acquireBatch(context.Background(), batchRequest(1, 2))
			record, _ := observer.record()
			if failure == nil || !errors.Is(failure, cause) || record.BatchRecord != tc.want {
				t.Fatalf("failure=%v record=%+v", failure, record)
			}
		})
	}
}

func TestControlFlowPublicParityAndRecorderFailureIgnored(t *testing.T) {
	d := baseControlFlowDeps(t)
	out := runCensusCore(censusCLIOptions{}, censusCoreConfig{}, d)
	var aout, aerr, bout, berr strings.Builder
	acode := writeCensusOutcome(&aout, &aerr, true, out)
	path, r := controlFlowRecorder(t)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	bcode := writeCensusOutcome(&bout, &berr, true, out)
	if out.controlFlow != nil {
		_ = r.Record(*out.controlFlow)
	}
	if acode != bcode || aout.String() != bout.String() || aerr.String() != berr.String() {
		t.Fatalf("baseline=%d/%q/%q enabled=%d/%q/%q", acode, aout.String(), aerr.String(), bcode, bout.String(), berr.String())
	}
}

func runnerControlFlowDeps(t *testing.T, runCore func(censusCLIOptions, censusCoreConfig, censusCoreDependencies) censusPublicationOutcome) censusRunnerDependencies {
	t.Helper()
	deps := productionCensusRunnerDependencies()
	deps.lookPath = func(string) (string, error) { return "/bin/echo", nil }
	deps.abs = func(path string) (string, error) { return path, nil }
	deps.newStarter = func() (sessionruntime.Starter, error) { return nil, nil }
	deps.newManager = func(sessionruntime.Config) (*sessionruntime.Manager, error) { return nil, nil }
	deps.runCore = runCore
	return deps
}

func runnerControlFlowArgs(t *testing.T, publicationRoot, controlFlowPath string) []string {
	t.Helper()
	args := []string{"--machine", "--workspace", t.TempDir(), "--source", ".", "--publication-root", publicationRoot, "--server", "gopls"}
	if controlFlowPath != "" {
		args = append(args, "--private-control-flow-diagnostic-path", controlFlowPath)
	}
	return args
}

func TestRunCensusControlFlowAcquisitionFailurePublicParityAndSingleRecord(t *testing.T) {
	outcome := censusPublicationOutcome{
		Diagnostic: mustDiagnostic(t, censusStageAcquisition),
		controlFlow: &censuscontrolflow.Record{
			SchemaVersion: censuscontrolflow.SchemaVersion,
			FirstCause:    censuscontrolflow.CoreFailed,
			CorePhase:     censuscontrolflow.PhaseBatchAcquire,
			BatchRecord:   censuscontrolflow.BatchRejected,
		},
	}
	deps := runnerControlFlowDeps(t, func(censusCLIOptions, censusCoreConfig, censusCoreDependencies) censusPublicationOutcome {
		return outcome
	})

	baselineRoot := t.TempDir()
	var baselineOut, baselineErr bytes.Buffer
	baselineCode := runCensusWithDependencies(runnerControlFlowArgs(t, baselineRoot, ""), &baselineOut, &baselineErr, deps)

	enabledRoot := t.TempDir()
	controlPath := privatePathForRunner(t)
	var enabledOut, enabledErr bytes.Buffer
	enabledCode := runCensusWithDependencies(runnerControlFlowArgs(t, enabledRoot, controlPath), &enabledOut, &enabledErr, deps)
	if baselineCode != enabledCode || !bytes.Equal(baselineOut.Bytes(), enabledOut.Bytes()) || !bytes.Equal(baselineErr.Bytes(), enabledErr.Bytes()) {
		t.Fatalf("baseline=%d/%q/%q enabled=%d/%q/%q", baselineCode, baselineOut.Bytes(), baselineErr.Bytes(), enabledCode, enabledOut.Bytes(), enabledErr.Bytes())
	}
	if baselineCode != 1 || baselineOut.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", baselineCode, baselineOut.Bytes(), baselineErr.Bytes())
	}
	for name, root := range map[string]string{"baseline": baselineRoot, "enabled": enabledRoot} {
		entries, err := os.ReadDir(root)
		if err != nil || len(entries) != 0 {
			t.Fatalf("%s publication entries=%v err=%v", name, entries, err)
		}
	}
	want, err := censuscontrolflow.Marshal(*outcome.controlFlow)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(controlPath)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("ledger=%q want=%q err=%v", got, want, err)
	}
}

func TestRunCensusControlFlowAppendFailurePreservesPublicBytesAndDoesNotRecurse(t *testing.T) {
	outcome := censusPublicationOutcome{
		Diagnostic:  mustDiagnostic(t, censusStageAcquisition),
		controlFlow: &censuscontrolflow.Record{SchemaVersion: censuscontrolflow.SchemaVersion, FirstCause: censuscontrolflow.CoreFailed, CorePhase: censuscontrolflow.PhaseBatchAcquire, BatchRecord: censuscontrolflow.BatchNotAttempted},
	}
	baselineDeps := runnerControlFlowDeps(t, func(censusCLIOptions, censusCoreConfig, censusCoreDependencies) censusPublicationOutcome {
		return outcome
	})
	var baselineOut, baselineErr bytes.Buffer
	baselineCode := runCensusWithDependencies(runnerControlFlowArgs(t, t.TempDir(), ""), &baselineOut, &baselineErr, baselineDeps)

	controlPath := privatePathForRunner(t)
	calls := 0
	failingDeps := runnerControlFlowDeps(t, func(censusCLIOptions, censusCoreConfig, censusCoreDependencies) censusPublicationOutcome {
		calls++
		if err := os.Remove(controlPath); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(controlPath, 0700); err != nil {
			t.Fatal(err)
		}
		return outcome
	})
	var gotOut, gotErr bytes.Buffer
	gotCode := runCensusWithDependencies(runnerControlFlowArgs(t, t.TempDir(), controlPath), &gotOut, &gotErr, failingDeps)
	if calls != 1 || gotCode != baselineCode || !bytes.Equal(gotOut.Bytes(), baselineOut.Bytes()) || !bytes.Equal(gotErr.Bytes(), baselineErr.Bytes()) {
		t.Fatalf("calls=%d baseline=%d/%q/%q got=%d/%q/%q", calls, baselineCode, baselineOut.Bytes(), baselineErr.Bytes(), gotCode, gotOut.Bytes(), gotErr.Bytes())
	}
	entries, err := os.ReadDir(controlPath)
	if err != nil || len(entries) != 0 {
		t.Fatalf("recursive ledger entries=%v err=%v", entries, err)
	}
}

func TestRunCensusControlFlowSuccessAndNonAcquisitionFailuresDoNotRecord(t *testing.T) {
	projection := censusPublicationProjection(t, 1)
	cases := []struct {
		name    string
		outcome censusPublicationOutcome
	}{
		{"success", successfulCorePublication(t, censusPublicationCapabilityFor(t, projection))},
		{"discovery", censusPublicationOutcome{Diagnostic: mustDiagnostic(t, censusStageDiscovery)}},
		{"post-core-publication", censusPublicationOutcome{Diagnostic: mustDiagnostic(t, censusStagePublication)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := privatePathForRunner(t)
			deps := runnerControlFlowDeps(t, func(censusCLIOptions, censusCoreConfig, censusCoreDependencies) censusPublicationOutcome {
				return tc.outcome
			})
			var stdout, stderr bytes.Buffer
			_ = runCensusWithDependencies(runnerControlFlowArgs(t, t.TempDir(), path), &stdout, &stderr, deps)
			raw, err := os.ReadFile(path)
			if err != nil || len(raw) != 0 {
				t.Fatalf("ledger=%q err=%v stdout=%q stderr=%q", raw, err, stdout.Bytes(), stderr.Bytes())
			}
		})
	}

	path := privatePathForRunner(t)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	deps := runnerControlFlowDeps(t, func(censusCLIOptions, censusCoreConfig, censusCoreDependencies) censusPublicationOutcome {
		t.Fatal("core called after config failure")
		return censusPublicationOutcome{}
	})
	deps.abs = func(string) (string, error) { return "", errors.New("config failure") }
	var stdout, stderr bytes.Buffer
	if code := runCensusWithDependencies(runnerControlFlowArgs(t, t.TempDir(), path), &stdout, &stderr, deps); code != 1 || stdout.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.Bytes(), stderr.Bytes())
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("config failure created ledger: %v", err)
	}
}

func TestRunCensusControlFlowOneAcquisitionInvocationProducesAtMostOneRecord(t *testing.T) {
	path := privatePathForRunner(t)
	acquisitions := 0
	deps := runnerControlFlowDeps(t, func(options censusCLIOptions, cfg censusCoreConfig, _ censusCoreDependencies) censusPublicationOutcome {
		core := baseControlFlowDeps(t)
		baseAcquire := core.acquire
		core.acquire = func(ctx context.Context, runtime *initializedAcquisitionRuntime, discoverer censusacquisition.Discoverer, limits acquisitionops.Limits, down, up, batches int) (censusAssembly, error) {
			acquisitions++
			return baseAcquire(ctx, runtime, discoverer, limits, down, up, batches)
		}
		core.runSession = func(config initializedAcquisitionRunnerConfig, callback func(context.Context, *initializedAcquisitionRuntime) int) int {
			_ = callback(context.Background(), &initializedAcquisitionRuntime{sessionID: "publication-session", generation: 7})
			return 1
		}
		return runCensusCore(options, cfg, core)
	})
	var stdout, stderr bytes.Buffer
	if code := runCensusWithDependencies(runnerControlFlowArgs(t, t.TempDir(), path), &stdout, &stderr, deps); code != 1 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.Bytes(), stderr.Bytes())
	}
	if acquisitions != 1 {
		t.Fatalf("acquisitions=%d", acquisitions)
	}
	raw, err := os.ReadFile(path)
	if err != nil || bytes.Count(raw, []byte{'\n'}) != 1 {
		t.Fatalf("ledger=%q err=%v", raw, err)
	}
	record, err := censuscontrolflow.Parse(bytes.TrimSuffix(raw, []byte{'\n'}))
	if err != nil || record.FirstCause != censuscontrolflow.CoreFailed || record.CorePhase != censuscontrolflow.PhaseBatchAcquire {
		t.Fatalf("record=%+v err=%v", record, err)
	}
}

func privatePathForRunner(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "control.ndjson")
}

func TestControlFlowParserCrossRejectsV1(t *testing.T) {
	v1 := censusdiagnostic.Record{Fingerprint: "sha256:abc", Category: "INVALID_INPUT"}
	dir := t.TempDir()
	_ = os.Chmod(dir, 0700)
	path := filepath.Join(dir, "v1")
	r, err := censusdiagnostic.NewRecorder(path, 4096, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.Record(v1); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = censuscontrolflow.Parse(raw[:len(raw)-1]); err == nil {
		t.Fatal("control-flow parser accepted v1")
	}
	cf, _ := censuscontrolflow.Marshal(censuscontrolflow.Record{SchemaVersion: censuscontrolflow.SchemaVersion, FirstCause: censuscontrolflow.CoreFailed, CorePhase: censuscontrolflow.PhaseInput, BatchRecord: censuscontrolflow.BatchNotAttempted})
	if _, err = censusdiagnostic.Parse(cf[:len(cf)-1]); err == nil {
		t.Fatal("v1 parser accepted control-flow")
	}
}
