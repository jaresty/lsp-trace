package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/censuscontrolflow"
	"lsp-trace/internal/censusdiagnostic"
	"lsp-trace/internal/censussourceassembly"
	"lsp-trace/internal/censussourcedriver"
	"lsp-trace/internal/censussourceplan"
	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
)

func TestCensusFreshCatalogPoisonsHistoricalCoreRoute(t *testing.T) {
	workspace, publicationRoot := t.TempDir(), t.TempDir()
	if err := os.Chmod(publicationRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	profileConfig := filepath.Join(t.TempDir(), "profiles.toml")
	if err := os.WriteFile(profileConfig, []byte("[profiles.test]\ncommand = \"profile-server\"\nlanguage_ids = [\"go\"]\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	deps := productionCensusRunnerDependencies()
	deps.lookPath = func(string) (string, error) { return "/bin/echo", nil }
	deps.newStarter = func() (sessionruntime.Starter, error) { return sessionruntime.ManagedStarter{}, nil }
	coreCalls := 0
	deps.runCore = func(censusCLIOptions, censusCoreConfig, censusCoreDependencies) censusPublicationOutcome {
		coreCalls++
		return censusPublicationOutcome{}
	}
	deps.runCatalog = func(_ context.Context, _ censuscontinuation.Request) censuscontinuation.Result {
		t.Fatal("ASSERT_FRESH_NATIVE_CATALOG_RUN_CATALOG_POISON")
		return censuscontinuation.Result{}
	}

	var stdout, stderr bytes.Buffer
	_ = runCensusWithDependencies([]string{"--machine", "--workspace", workspace, "--source", ".", "--publication-root", publicationRoot, "--catalog", "--catalog-config", "/host.json", "--profile", "test", "--config", profileConfig}, &stdout, &stderr, deps)
	if coreCalls != 0 {
		t.Fatalf("ASSERT_FRESH_NATIVE_CATALOG_RUN_CORE_POISON calls=%d", coreCalls)
	}
}

func TestCensusNativeCatalogProcessesExactPlanWithOneRuntimeAndHonestTerminal(t *testing.T) {
	workspace := t.TempDir()
	files := make([]censusacquisition.SourceFile, censusNativeSourcePartitionSize*2+1)
	for i := range files {
		name := fmt.Sprintf("%03d.go", i)
		files[i] = censusacquisition.SourceFile{Path: name, URI: (&url.URL{Scheme: "file", Path: filepath.Join(workspace, name)}).String(), LanguageID: "go"}
	}
	objectRoot, err := publication.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	headRoot, err := publication.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = objectRoot.Close(); _ = headRoot.Close() })

	events := []string{}
	discoveredOrdinals := []int{}
	var discoveredMembers [][]censusacquisition.SourceFile
	var workflowKeys []string
	deps := censusNativeCatalogDependencies{
		enumerate: func(censusCLIOptions) ([]censusacquisition.SourceFile, error) {
			events = append(events, "enumerate")
			return append([]censusacquisition.SourceFile(nil), files...), nil
		},
		buildPlan: func(got []censusacquisition.SourceFile, gotWorkspace string, size int) (censussourceplan.Plan, error) {
			events = append(events, "build")
			if !reflect.DeepEqual(got, files) || gotWorkspace != workspace || size != censusNativeSourcePartitionSize {
				t.Fatalf("ASSERT_NATIVE_PLAN_BUILD_INPUT workspace=%q size=%d files=%d", gotWorkspace, size, len(got))
			}
			return censussourceplan.Build(got, gotWorkspace, size)
		},
		validatePlan: func(plan censussourceplan.Plan, size int) error {
			events = append(events, "validate")
			return censussourceplan.Validate(plan, size)
		},
		runSession: func(_ initializedAcquisitionRunnerConfig, callback func(context.Context, *initializedAcquisitionRuntime) int) int {
			events = append(events, "session")
			return callback(context.Background(), &initializedAcquisitionRuntime{sessionID: "one-session", generation: 7})
		},
		openRoots: func(string) (*publication.Root, *publication.Root, func() error, error) {
			events = append(events, "roots")
			return objectRoot, headRoot, func() error { return nil }, nil
		},
		discoverPartition: func(_ context.Context, _ censusCLIOptions, runtime *initializedAcquisitionRuntime, members []censusacquisition.SourceFile) (censusacquisition.Discovery, error) {
			events = append(events, "discover")
			if runtime.SessionID() != "one-session" || runtime.Generation() != 7 {
				t.Fatalf("ASSERT_NATIVE_ONE_RUNTIME session=%q generation=%d", runtime.SessionID(), runtime.Generation())
			}
			discoveredMembers = append(discoveredMembers, append([]censusacquisition.SourceFile(nil), members...))
			return censusacquisition.Discovery{Session: censusacquisition.SessionIdentity{SessionID: runtime.SessionID(), Generation: runtime.Generation()}, Workspace: workspace, Closure: censusacquisition.DiscoveryClosureSuccessful}, nil
		},
		runAssembly: func(ctx context.Context, request censussourceassembly.Request) (censussourcedriver.Result, *censussourceassembly.TerminalRecord) {
			events = append(events, "assembly")
			workflowKeys = append(workflowKeys, request.WorkflowKey)
			if request.WorkflowKey == "" || request.WorkflowKey != request.HeadKey || request.ObjectRoot != objectRoot || request.HeadRoot != headRoot || request.Objects != nil || request.Heads != nil || request.StepBound != len(request.Plan.Partitions) || request.Identity.SessionID != "one-session" || request.Identity.ManagerGeneration != 7 {
				t.Fatalf("ASSERT_NATIVE_ASSEMBLY_REQUEST request=%+v partitions=%d", request, len(request.Plan.Partitions))
			}
			for ordinal, partition := range request.Plan.Partitions {
				discoveredOrdinals = append(discoveredOrdinals, ordinal)
				if _, discoverErr := request.Discover(ctx, ordinal, partition.Members); discoverErr != nil {
					t.Fatal(discoverErr)
				}
			}
			return censussourcedriver.Result{Stop: censussourcedriver.StopComplete, State: censussourcedriver.State{Status: censussourcedriver.StatusCommitted, CommittedPrefix: len(request.Plan.Partitions), PartitionCount: len(request.Plan.Partitions), PlanIdentity: request.Plan.Identity, SessionID: request.Identity.SessionID, ManagerGeneration: request.Identity.ManagerGeneration}, Steps: len(request.Plan.Partitions), Transitions: len(request.Plan.Partitions)}, nil
		},
	}
	options := censusCLIOptions{Machine: true, Workspace: workspace, PublicationRoot: t.TempDir(), Sources: []string{"."}}
	cfg := censusCoreConfig{}
	for run := 0; run < 2; run++ {
		var stdout, stderr bytes.Buffer
		code := runCensusNativeCatalogWithDependencies(options, cfg, &stdout, &stderr, deps)
		var expected bytes.Buffer
		expectedCode := writeCensusFailure(&expected, censusStageCommitted, true, errNativeCatalogAggregateUnavailable)
		if code != expectedCode || stdout.Len() != 0 || stderr.String() != expected.String() {
			t.Fatalf("ASSERT_NATIVE_HONEST_TERMINAL run=%d code=%d/%d stdout=%q stderr=%q/%q", run, code, expectedCode, stdout.String(), stderr.String(), expected.String())
		}
	}
	wantEvents := []string{"enumerate", "build", "validate", "session", "roots", "assembly", "discover", "discover", "discover", "enumerate", "build", "validate", "session", "roots", "assembly", "discover", "discover", "discover"}
	if !reflect.DeepEqual(events, wantEvents) {
		t.Fatalf("ASSERT_NATIVE_ORDER events=%v want=%v", events, wantEvents)
	}
	if !reflect.DeepEqual(discoveredOrdinals, []int{0, 1, 2, 0, 1, 2}) {
		t.Fatalf("ASSERT_NATIVE_EXACT_ORDINAL_ORDER ordinals=%v", discoveredOrdinals)
	}
	plan, err := censussourceplan.Build(files, workspace, censusNativeSourcePartitionSize)
	if err != nil {
		t.Fatal(err)
	}
	wantMembers := [][]censusacquisition.SourceFile{plan.Partitions[0].Members, plan.Partitions[1].Members, plan.Partitions[2].Members, plan.Partitions[0].Members, plan.Partitions[1].Members, plan.Partitions[2].Members}
	if !reflect.DeepEqual(discoveredMembers, wantMembers) {
		t.Fatalf("ASSERT_NATIVE_EXACT_MEMBERS members=%v want=%v", discoveredMembers, wantMembers)
	}
	if len(workflowKeys) != 2 || workflowKeys[0] != workflowKeys[1] {
		t.Fatalf("ASSERT_NATIVE_STABLE_WORKFLOW keys=%v", workflowKeys)
	}
}

func TestCensusNativeCatalogPreservesPrivateDiagnosticsWithoutPublicChange(t *testing.T) {
	workspace := t.TempDir()
	file := censusacquisition.SourceFile{Path: "one.go", URI: (&url.URL{Scheme: "file", Path: filepath.Join(workspace, "one.go")}).String(), LanguageID: "go"}
	var baseline string
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[enabled], func(t *testing.T) {
			objectRoot, _ := publication.OpenRoot(t.TempDir())
			headRoot, _ := publication.OpenRoot(t.TempDir())
			var recorder *censusdiagnostic.Recorder
			var diagnosticPath string
			if enabled {
				diagnosticPath, recorder = newPrivateAcquisitionRecorder(t)
			}
			observer := newCensusControlFlowObserver()
			deps := censusNativeCatalogDependencies{
				enumerate: func(censusCLIOptions) ([]censusacquisition.SourceFile, error) {
					return []censusacquisition.SourceFile{file}, nil
				},
				buildPlan: censussourceplan.Build, validatePlan: censussourceplan.Validate,
				runSession: func(_ initializedAcquisitionRunnerConfig, callback func(context.Context, *initializedAcquisitionRuntime) int) int {
					return callback(context.Background(), &initializedAcquisitionRuntime{sessionID: "s", generation: 1})
				},
				openRoots: func(string) (*publication.Root, *publication.Root, func() error, error) {
					return objectRoot, headRoot, func() error { _ = objectRoot.Close(); _ = headRoot.Close(); return nil }, nil
				},
				discoverPartition: func(context.Context, censusCLIOptions, *initializedAcquisitionRuntime, []censusacquisition.SourceFile) (censusacquisition.Discovery, error) {
					return censusacquisition.Discovery{}, errors.New("private discovery failure")
				},
				runAssembly: func(ctx context.Context, request censussourceassembly.Request) (censussourcedriver.Result, *censussourceassembly.TerminalRecord) {
					_, err := request.Discover(ctx, 0, request.Plan.Partitions[0].Members)
					return censussourcedriver.Result{Stop: censussourcedriver.StopFailed, Err: err}, nil
				},
			}
			var stdout, stderr bytes.Buffer
			code := runCensusNativeCatalogWithDependencies(censusCLIOptions{Machine: true, Workspace: workspace, PublicationRoot: t.TempDir(), Sources: []string{"."}}, censusCoreConfig{diagnostic: recorder, controlFlow: observer}, &stdout, &stderr, deps)
			if code == 0 || stdout.Len() != 0 {
				t.Fatalf("ASSERT_NATIVE_PRIVATE_DIAGNOSTIC_PUBLIC code=%d stdout=%q", code, stdout.String())
			}
			if baseline == "" {
				baseline = stderr.String()
			} else if stderr.String() != baseline {
				t.Fatalf("ASSERT_NATIVE_PRIVATE_DIAGNOSTIC_PUBLIC_PARITY got=%q want=%q", stderr.String(), baseline)
			}
			if enabled {
				record, err := censusdiagnostic.ReadLast(diagnosticPath)
				if err != nil || record.Category != "ADMISSION" || record.Ordinal == nil || *record.Ordinal != 0 {
					t.Fatalf("ASSERT_NATIVE_PRIVATE_DIAGNOSTIC_EMITTED record=%+v err=%v", record, err)
				}
			} else if recorder != nil {
				t.Fatal("ASSERT_NATIVE_PRIVATE_DIAGNOSTIC_ABSENT")
			}
			control, ok := observer.record()
			if !ok || control.FirstCause != censuscontrolflow.CoreFailed || control.CorePhase != censuscontrolflow.PhaseManifest {
				t.Fatalf("ASSERT_NATIVE_CONTROL_FLOW_EMITTED record=%+v ok=%v", control, ok)
			}
		})
	}
}

func TestCloseCensusNativeCatalogRootsAttemptsBothExactlyOnce(t *testing.T) {
	first, _ := publication.OpenRoot(t.TempDir())
	second, _ := publication.OpenRoot(t.TempDir())
	firstErr, secondErr := errors.New("first close"), errors.New("second close")
	for _, tc := range []struct {
		name     string
		failures map[*publication.Root]error
	}{
		{name: "first", failures: map[*publication.Root]error{first: firstErr}},
		{name: "second", failures: map[*publication.Root]error{second: secondErr}},
		{name: "both", failures: map[*publication.Root]error{first: firstErr, second: secondErr}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := map[*publication.Root]int{}
			err := closeCensusNativeCatalogRoots([]*publication.Root{first, second}, func(root *publication.Root) error { calls[root]++; return tc.failures[root] })
			if calls[first] != 1 || calls[second] != 1 || (tc.failures[first] != nil && !errors.Is(err, firstErr)) || (tc.failures[second] != nil && !errors.Is(err, secondErr)) {
				t.Fatalf("ASSERT_NATIVE_ROOTS_CLOSE_BOTH name=%s calls=%v err=%v", tc.name, calls, err)
			}
		})
	}
	_ = first.Close()
	_ = second.Close()
}

func TestCensusNativeCatalogRootCloseErrorsSurfaceAndCloseOnce(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{name: "first", err: errors.New("first close")},
		{name: "second", err: errors.New("second close")},
		{name: "both", err: errors.Join(errors.New("first close"), errors.New("second close"))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			workspace := t.TempDir()
			file := censusacquisition.SourceFile{Path: "one.go", URI: (&url.URL{Scheme: "file", Path: filepath.Join(workspace, "one.go")}).String(), LanguageID: "go"}
			objectRoot, _ := publication.OpenRoot(t.TempDir())
			headRoot, _ := publication.OpenRoot(t.TempDir())
			closeCalls := 0
			deps := censusNativeCatalogDependencies{
				enumerate: func(censusCLIOptions) ([]censusacquisition.SourceFile, error) {
					return []censusacquisition.SourceFile{file}, nil
				},
				buildPlan: censussourceplan.Build, validatePlan: censussourceplan.Validate,
				runSession: func(_ initializedAcquisitionRunnerConfig, callback func(context.Context, *initializedAcquisitionRuntime) int) int {
					return callback(context.Background(), &initializedAcquisitionRuntime{sessionID: "s", generation: 1})
				},
				openRoots: func(string) (*publication.Root, *publication.Root, func() error, error) {
					return objectRoot, headRoot, func() error { closeCalls++; _ = objectRoot.Close(); _ = headRoot.Close(); return tc.err }, nil
				},
				discoverPartition: func(context.Context, censusCLIOptions, *initializedAcquisitionRuntime, []censusacquisition.SourceFile) (censusacquisition.Discovery, error) {
					return censusacquisition.Discovery{}, nil
				},
				runAssembly: func(context.Context, censussourceassembly.Request) (censussourcedriver.Result, *censussourceassembly.TerminalRecord) {
					return censussourcedriver.Result{Stop: censussourcedriver.StopComplete}, nil
				},
			}
			var stdout, stderr bytes.Buffer
			code := runCensusNativeCatalogWithDependencies(censusCLIOptions{Machine: true, Workspace: workspace, PublicationRoot: t.TempDir(), Sources: []string{"."}}, censusCoreConfig{}, &stdout, &stderr, deps)
			var want bytes.Buffer
			wantCode := writeCensusFailure(&want, censusStageAssembly, true, tc.err)
			if closeCalls != 1 || code != wantCode || stdout.Len() != 0 || stderr.String() != want.String() {
				t.Fatalf("ASSERT_NATIVE_ROOT_CLOSE_FAILURE name=%s closes=%d code=%d/%d stdout=%q stderr=%q/%q", tc.name, closeCalls, code, wantCode, stdout.String(), stderr.String(), want.String())
			}
		})
	}
}

func TestCensusNativeWorkflowKeyCanonicalOrderingAndMaterialDifferences(t *testing.T) {
	base := censusCLIOptions{
		Workspace: "/workspace",
		Sources:   []string{"b", "a"}, Includes: []string{"*.go", "cmd/**"},
		Excludes: []string{"vendor/**", "tmp/**"}, PrunedDirectories: []string{".git", "vendor"},
		ServerArgs: []string{"--mode", "strict"},
	}
	key, err := censusNativeWorkflowKey(base, "sha256:plan")
	if err != nil {
		t.Fatal(err)
	}
	reordered := base
	reordered.Sources = []string{"a", "b"}
	reordered.Includes = []string{"cmd/**", "*.go"}
	reordered.Excludes = []string{"tmp/**", "vendor/**"}
	reordered.PrunedDirectories = []string{"vendor", ".git"}
	got, err := censusNativeWorkflowKey(reordered, "sha256:plan")
	if err != nil || got != key {
		t.Fatalf("ASSERT_NATIVE_WORKFLOW_REORDER_STABLE key=%q got=%q err=%v", key, got, err)
	}
	cases := map[string]censusCLIOptions{
		"multiplicity": func() censusCLIOptions { v := base; v.Sources = []string{"b", "a", "a"}; return v }(),
		"server-order": func() censusCLIOptions { v := base; v.ServerArgs = []string{"strict", "--mode"}; return v }(),
		"option":       func() censusCLIOptions { v := base; v.Excludes = []string{"other/**"}; return v }(),
	}
	for name, options := range cases {
		t.Run(name, func(t *testing.T) {
			changed, err := censusNativeWorkflowKey(options, "sha256:plan")
			if err != nil || changed == key {
				t.Fatalf("ASSERT_NATIVE_WORKFLOW_MATERIAL_DIFFERENCE key=%q changed=%q err=%v", key, changed, err)
			}
		})
	}
	changedPlan, err := censusNativeWorkflowKey(base, "sha256:other-plan")
	if err != nil || changedPlan == key {
		t.Fatalf("ASSERT_NATIVE_WORKFLOW_PLAN_PRIMARY key=%q changed=%q err=%v", key, changedPlan, err)
	}
}

func TestCensusNativeCatalogStopsOnceForEveryAssemblyTerminal(t *testing.T) {
	stops := []struct {
		stop     censussourcedriver.Stop
		humanErr string
	}{
		{stop: censussourcedriver.StopComplete, humanErr: "native census catalog handoff unavailable"},
		{stop: censussourcedriver.StopBoundReached, humanErr: "native census catalog assembly bound reached"},
		{stop: censussourcedriver.StopDeadline, humanErr: "native census catalog assembly deadline reached"},
		{stop: censussourcedriver.StopConflict, humanErr: "native census catalog assembly conflict"},
		{stop: censussourcedriver.StopRetained, humanErr: "native census catalog assembly retained"},
		{stop: censussourcedriver.StopIndeterminateExecution, humanErr: "native census catalog assembly indeterminate execution"},
		{stop: censussourcedriver.StopFailed, humanErr: "native census catalog assembly failed"},
	}
	for _, tc := range stops {
		t.Run(string(tc.stop), func(t *testing.T) {
			workspace := t.TempDir()
			file := censusacquisition.SourceFile{Path: "one.go", URI: (&url.URL{Scheme: "file", Path: filepath.Join(workspace, "one.go")}).String(), LanguageID: "go"}
			objectRoot, err := publication.OpenRoot(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			headRoot, err := publication.OpenRoot(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = objectRoot.Close(); _ = headRoot.Close() })
			assemblyCalls := 0
			deps := censusNativeCatalogDependencies{
				enumerate: func(censusCLIOptions) ([]censusacquisition.SourceFile, error) {
					return []censusacquisition.SourceFile{file}, nil
				},
				buildPlan:    censussourceplan.Build,
				validatePlan: censussourceplan.Validate,
				runSession: func(_ initializedAcquisitionRunnerConfig, callback func(context.Context, *initializedAcquisitionRuntime) int) int {
					return callback(context.Background(), &initializedAcquisitionRuntime{sessionID: "one-session", generation: 1})
				},
				openRoots: func(string) (*publication.Root, *publication.Root, func() error, error) {
					return objectRoot, headRoot, func() error { return nil }, nil
				},
				discoverPartition: func(context.Context, censusCLIOptions, *initializedAcquisitionRuntime, []censusacquisition.SourceFile) (censusacquisition.Discovery, error) {
					t.Fatal("ASSERT_NATIVE_STOP_DISCOVERY_OWNED_BY_ASSEMBLY")
					return censusacquisition.Discovery{}, nil
				},
				runAssembly: func(context.Context, censussourceassembly.Request) (censussourcedriver.Result, *censussourceassembly.TerminalRecord) {
					assemblyCalls++
					return censussourcedriver.Result{Stop: tc.stop}, nil
				},
			}
			for _, machine := range []bool{false, true} {
				t.Run(map[bool]string{false: "human", true: "machine"}[machine], func(t *testing.T) {
					assemblyCalls = 0
					var stdout, stderr bytes.Buffer
					code := runCensusNativeCatalogWithDependencies(censusCLIOptions{Machine: machine, Workspace: workspace, PublicationRoot: t.TempDir(), Sources: []string{"."}}, censusCoreConfig{}, &stdout, &stderr, deps)
					if assemblyCalls != 1 {
						t.Fatalf("ASSERT_NATIVE_STOP_NO_RETRY stop=%s calls=%d", tc.stop, assemblyCalls)
					}
					if !machine {
						want := tc.humanErr + "\n"
						if code != 1 || stdout.Len() != 0 || stderr.String() != want || stderr.String() == "<nil>\n" {
							t.Fatalf("ASSERT_NATIVE_STOP_HUMAN_TERMINAL stop=%s code=%d stdout=%q stderr=%q want=%q", tc.stop, code, stdout.String(), stderr.String(), want)
						}
						return
					}
					stage := censusStageAssembly
					if tc.stop == censussourcedriver.StopComplete {
						stage = censusStageCommitted
					} else if tc.stop == censussourcedriver.StopDeadline {
						stage = censusStageAcquisition
					}
					var expected bytes.Buffer
					expectedCode := writeCensusFailure(&expected, stage, true, errors.New("private"))
					if code != expectedCode || stdout.Len() != 0 || stderr.String() != expected.String() {
						t.Fatalf("ASSERT_NATIVE_STOP_EXACT_TERMINAL stop=%s code=%d/%d stdout=%q stderr=%q/%q", tc.stop, code, expectedCode, stdout.String(), stderr.String(), expected.String())
					}
					var diagnostic censusCLIDiagnostic
					if err := json.Unmarshal(stderr.Bytes(), &diagnostic); err != nil {
						t.Fatal(err)
					}
					if tc.stop != censussourcedriver.StopComplete && (diagnostic.Status != "FAILED" || !diagnostic.Retry || diagnostic.Code == censusCodeCommittedDegraded) {
						t.Fatalf("ASSERT_NATIVE_STOP_NOT_COMMITTED_DEGRADED stop=%s diagnostic=%+v", tc.stop, diagnostic)
					}
				})
			}
		})
	}
}

func TestCensusNativeCatalogAssemblyErrorPrecedence(t *testing.T) {
	assemblyErr := errors.New("assembly detail")
	for _, stop := range []censussourcedriver.Stop{
		censussourcedriver.StopBoundReached,
		censussourcedriver.StopDeadline,
		censussourcedriver.StopConflict,
		censussourcedriver.StopRetained,
		censussourcedriver.StopIndeterminateExecution,
		censussourcedriver.StopFailed,
	} {
		t.Run(string(stop), func(t *testing.T) {
			if got := censusNativeCatalogTerminalError(stop, assemblyErr); !errors.Is(got, assemblyErr) {
				t.Fatalf("ASSERT_NATIVE_STOP_ERROR_PRECEDENCE stop=%s got=%v want=%v", stop, got, assemblyErr)
			}
		})
	}
	if got := censusNativeCatalogTerminalError(censussourcedriver.StopComplete, assemblyErr); !errors.Is(got, errNativeCatalogAggregateUnavailable) {
		t.Fatalf("ASSERT_NATIVE_COMPLETE_ERROR_PRECEDENCE got=%v want=%v", got, errNativeCatalogAggregateUnavailable)
	}
}
