package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"lsp-trace/acquisitionops"
	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/acquisitionengine"
	"lsp-trace/internal/acquisitionorchestration"
	"lsp-trace/internal/census"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censusresult"
	"lsp-trace/internal/censussourceplan"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/seedformat"
	"lsp-trace/internal/session"
	"lsp-trace/internal/source"
	"lsp-trace/sessionruntime"
)

func censusRuntimeRequest(t *testing.T, root *publication.Root, raw []byte) operation.Request {
	t.Helper()
	return operation.Request{Name: "lsp_trace_v1_census", RequestID: "private-test", Input: raw, PublicationRoot: root}
}

func openCensusRuntimeRoot(t *testing.T) *publication.Root {
	t.Helper()
	root, err := publication.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	return root
}

func writeRuntimeSource(t *testing.T, workspace, name string) {
	t.Helper()
	path := filepath.Join(workspace, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func runtimeRaw(t *testing.T, alias string, generation uint64, overrides map[string]any) []byte {
	t.Helper()
	value := map[string]any{"session_id": alias, "generation": generation, "sources": []string{"."}, "max_nodes": 10, "timeout_ms": 1000, "request_timeout_ms": 500}
	for key, item := range overrides {
		value[key] = item
	}
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestCensusRuntimeAdmissionFailureStopsBeforeRootAndDiscovery(t *testing.T) {
	runtime, starter, started := censusRuntimeFixture(t, censusRuntimeOptions{ready: true, documentSymbolSupport: true, callHierarchySupport: true})
	shell := newCensusRuntime(runtime)
	calls := 0
	shell.admit = func(ctx context.Context, raw []byte) (censusAdmittedSession, *censusAdmissionFailure, censusFailureReason) {
		calls++
		return newCensusExecutor(runtime).execute(ctx, raw)
	}
	beforeStarts, beforeMethods, beforeTeardown, beforeClose := starter.snapshot()
	_, failure, _ := shell.execute(context.Background(), censusRuntimeRequest(t, nil, runtimeRaw(t, "missing", started.Generation, nil)))
	afterStarts, afterMethods, afterTeardown, afterClose := starter.snapshot()
	if calls != 1 || failure == nil || failure.stage != censusStageAcquisition || !reflect.DeepEqual(beforeMethods, afterMethods) || beforeStarts != afterStarts || beforeTeardown != afterTeardown || beforeClose != afterClose {
		t.Fatalf("ASSERT_CENSUS_RUNTIME_ADMISSION_ONCE_SHORT_CIRCUIT: calls=%d failure=%+v methods=%v/%v lifecycle=%d/%d/%d/%d/%d/%d", calls, failure, beforeMethods, afterMethods, beforeStarts, afterStarts, beforeTeardown, afterTeardown, beforeClose, afterClose)
	}
}

func TestCensusRuntimeRequiresHostRootBeforeDiscovery(t *testing.T) {
	runtime, starter, started := censusRuntimeFixture(t, censusRuntimeOptions{ready: true, documentSymbolSupport: true, callHierarchySupport: true})
	before := runtime.Census()
	_, beforeMethods, _, _ := starter.snapshot()
	_, failure, _ := newCensusRuntime(runtime).execute(context.Background(), censusRuntimeRequest(t, nil, runtimeRaw(t, "project", started.Generation, nil)))
	_, afterMethods, _, _ := starter.snapshot()
	if failure == nil || failure.stage != censusStageConfig || failure.code != censusCodeInvalidConfig || !reflect.DeepEqual(beforeMethods, afterMethods) || !reflect.DeepEqual(before, runtime.Census()) {
		t.Fatalf("ASSERT_CENSUS_RUNTIME_ROOT_REQUIRED_NO_DISCOVERY: failure=%+v methods=%v/%v", failure, beforeMethods, afterMethods)
	}
}

func TestCensusRuntimeIdentityOptionsDiscoveryAndDefensiveCopies(t *testing.T) {
	workspace := t.TempDir()
	writeRuntimeSource(t, workspace, "z.go")
	writeRuntimeSource(t, workspace, "generated/0.go")
	writeRuntimeSource(t, workspace, "a.go")
	runtime, starter, started := censusRuntimeFixture(t, censusRuntimeOptions{workspace: workspace, ready: true, documentSymbolSupport: true, callHierarchySupport: true})
	raw := runtimeRaw(t, "project", started.Generation, map[string]any{"sources": []string{".", "a.go"}, "includes": []string{"**/*.go"}, "excludes": []string{"generated/**"}, "max_nodes": 2})
	result, failure, _ := newCensusRuntime(runtime).execute(context.Background(), censusRuntimeRequest(t, openCensusRuntimeRoot(t), raw))
	if failure != nil {
		t.Fatalf("ASSERT_CENSUS_RUNTIME_DISCOVERY_SUCCESS: %+v", failure)
	}
	raw[0] = 'X'
	if result.admitted.sessionID != started.SessionID || result.admitted.generation != started.Generation || result.admitted.workspace != filepath.Clean(workspace) || result.admitted.positionEncoding != "utf-16" || result.options.sources[0] != "." || result.options.excludes[0] != "generated/**" {
		t.Fatalf("ASSERT_CENSUS_RUNTIME_IMMUTABLE_IDENTITY_OPTIONS: %+v %+v", result.admitted, result.options)
	}
	if result.discovery.Accounting.FileDenominator != 3 || len(result.discovery.Accounting.Files) != 3 || result.discovery.Accounting.Files[1].Disposition != census.FileExcluded && result.discovery.Accounting.Files[0].Disposition != census.FileExcluded && result.discovery.Accounting.Files[2].Disposition != census.FileExcluded {
		t.Fatalf("ASSERT_CENSUS_RUNTIME_EXCLUSION_ACCOUNTING: %+v", result.discovery.Accounting)
	}
	_, methods, _, _ := starter.snapshot()
	joined := strings.Join(methods, ",")
	if strings.Count(joined, "textDocument/didOpen") != 2 || strings.Count(joined, "textDocument/documentSymbol") != 2 {
		t.Fatalf("ASSERT_CENSUS_RUNTIME_EXCLUDED_DOES_NOT_CONSUME_MAX: %v", methods)
	}
	result.options.sources[0] = "mutated"
	result.discovery.Accounting.Files[0].Disposition = census.FileUnreadable
	again, againFailure, _ := newCensusRuntime(runtime).execute(context.Background(), censusRuntimeRequest(t, openCensusRuntimeRoot(t), runtimeRaw(t, "project", started.Generation, map[string]any{"sources": []string{"a.go"}, "max_nodes": 2})))
	if againFailure != nil || again.options.sources[0] != "a.go" || again.discovery.Accounting.Files[0].Disposition == census.FileUnreadable {
		t.Fatalf("ASSERT_CENSUS_RUNTIME_DEFENSIVE_COPY: again=%+v failure=%+v", again, againFailure)
	}
}

func TestPrivateCensusRuntimeEnumerationPrecedesDiscoveryEffects(t *testing.T) {
	workspace := t.TempDir()
	writeRuntimeSource(t, workspace, "a.go")
	runtime, starter, started := censusRuntimeFixture(t, censusRuntimeOptions{
		workspace: workspace, ready: true, documentSymbolSupport: true, callHierarchySupport: true,
		results: map[string]json.RawMessage{"textDocument/documentSymbol": json.RawMessage(`[]`)},
	})
	request := censusRuntimeRequest(t, openCensusRuntimeRoot(t), runtimeRaw(t, "project", started.Generation, map[string]any{"sources": []string{"."}, "max_nodes": 1}))
	request.PrivateCensusChunking = true
	result, failure, _ := newCensusRuntime(runtime).execute(context.Background(), request)
	_, methods, _, _ := starter.snapshot()
	joined := strings.Join(methods, ",")
	if failure != nil || strings.Contains(joined, "textDocument/didOpen") || strings.Contains(joined, "textDocument/documentSymbol") {
		t.Fatalf("ASSERT_CENSUS_ENUMERATION_PRECEDES_DISCOVERY_EFFECTS: failure=%+v methods=%v discovery=%+v", failure, methods, result.discovery)
	}
}

func TestPrivateCensusRuntimeDiscoveryHookAcceptsRetainedEnumeration(t *testing.T) {
	hook := reflect.TypeOf(censusRuntime{}.discoverEnumerated)
	fileEnumerator := reflect.TypeOf((*censusacquisition.FileEnumerator)(nil)).Elem()
	for i := 0; i < hook.NumIn(); i++ {
		if hook.In(i) == fileEnumerator {
			return
		}
	}
	t.Fatalf("ASSERT_PRIVATE_CENSUS_DISCOVERY_HOOK_RETAINS_ENUMERATION: hook=%v", hook)
}

func TestPrivateCensusRuntimeHasTypedSourcePlanCustody(t *testing.T) {
	field, ok := reflect.TypeOf(censusRuntimeResult{}).FieldByName("sourcePlan")
	if !ok || field.Type != reflect.TypeOf(censussourceplan.Plan{}) {
		t.Fatalf("ASSERT_CENSUS_SOURCE_PLAN_PRECEDES_DISCOVERY: field=%+v present=%t", field, ok)
	}
}

func TestPrivateCensusRuntimeBuildsCompleteSourcePlanBeforeDiscovery(t *testing.T) {
	workspace := t.TempDir()
	uri, err := source.FileURI(filepath.Join(workspace, "a.go"))
	if err != nil {
		t.Fatal(err)
	}
	runtime, _, started := censusRuntimeFixture(t, censusRuntimeOptions{workspace: workspace, ready: true, documentSymbolSupport: true, callHierarchySupport: true})
	shell := newCensusRuntime(runtime)
	input := []censusacquisition.SourceFile{{Path: "a.go", URI: uri, LanguageID: "go"}}
	shell.enumerate = func(context.Context, censusAdmittedSession, censusRuntimeConfig) ([]censusacquisition.SourceFile, error) {
		return input, nil
	}
	discoveryCalls := 0
	shell.discoverEnumerated = func(context.Context, *sessionruntime.Manager, censusAdmittedSession, censusRuntimeConfig, bool, censusacquisition.FileEnumerator) (censusacquisition.Discovery, error) {
		discoveryCalls++
		return censusacquisition.Discovery{}, errors.New("discovery must not run during execute")
	}
	request := censusRuntimeRequest(t, openCensusRuntimeRoot(t), runtimeRaw(t, "project", started.Generation, nil))
	request.PrivateCensusChunking = true
	result, failure, reason := shell.execute(context.Background(), request)
	want := censusacquisition.SourceFile{Path: "a.go", URI: uri, LanguageID: "go"}
	if failure != nil || reason != reasonNone || discoveryCalls != 0 || result.sourcePlan.Identity == "" || result.sourcePlan.Authority != 0 || len(result.enumeration) != 1 || len(result.sourcePlan.Files) != 1 || len(result.sourcePlan.Partitions) != 1 || result.sourcePlan.Partitions[0].Ordinal != 0 || len(result.sourcePlan.Partitions[0].Members) != 1 || result.enumeration[0] != want || result.sourcePlan.Files[0] != want || result.sourcePlan.Partitions[0].Members[0] != want {
		t.Fatalf("ASSERT_CENSUS_SOURCE_PLAN_COMPLETE_BEFORE_DISCOVERY: result=%+v failure=%+v reason=%q calls=%d", result, failure, reason, discoveryCalls)
	}
	input[0].Path = "caller.go"
	if result.enumeration[0] != want || result.sourcePlan.Files[0] != want || result.sourcePlan.Partitions[0].Members[0] != want {
		t.Fatalf("ASSERT_CENSUS_SOURCE_PLAN_CALLER_INPUT_CLONE: input=%+v result=%+v", input, result)
	}
	result.enumeration[0].Path = "enumeration.go"
	if result.sourcePlan.Files[0] != want || result.sourcePlan.Partitions[0].Members[0] != want {
		t.Fatalf("ASSERT_CENSUS_SOURCE_PLAN_ENUMERATION_CLONE: result=%+v", result)
	}
	result.sourcePlan.Files[0].Path = "files.go"
	if result.sourcePlan.Partitions[0].Members[0] != want {
		t.Fatalf("ASSERT_CENSUS_SOURCE_PLAN_FILES_PARTITION_CLONE: result=%+v", result)
	}
}

func TestPrivateCensusRuntimeRunSelectsOnlyFirstPlannedPartition(t *testing.T) {
	for _, fileCount := range []int{3, censusRuntimeSourcePartitionSize + 1} {
		t.Run(fmt.Sprintf("files-%d", fileCount), func(t *testing.T) {
			workspace := t.TempDir()
			files := make([]censusacquisition.SourceFile, fileCount)
			for i := range files {
				name := fmt.Sprintf("file-%03d.go", i)
				uri, err := source.FileURI(filepath.Join(workspace, name))
				if err != nil {
					t.Fatal(err)
				}
				files[i] = censusacquisition.SourceFile{Path: name, URI: uri, LanguageID: "go"}
			}
			runtime, _, started := censusRuntimeFixture(t, censusRuntimeOptions{workspace: workspace, ready: true, documentSymbolSupport: true, callHierarchySupport: true})
			shell := newCensusRuntime(runtime)
			events := make([]string, 0, 3)
			shell.enumerate = func(context.Context, censusAdmittedSession, censusRuntimeConfig) ([]censusacquisition.SourceFile, error) {
				events = append(events, "enumerate")
				return append([]censusacquisition.SourceFile(nil), files...), nil
			}
			var planned censussourceplan.Plan
			shell.planSourceFiles = func(got []censusacquisition.SourceFile, workspace string, partitionSize int) (censussourceplan.Plan, error) {
				events = append(events, "plan")
				var err error
				planned, err = censussourceplan.Build(got, workspace, partitionSize)
				return planned, err
			}
			discoveryCalls := 0
			var discovered []censusacquisition.SourceFile
			shell.discoverEnumerated = func(ctx context.Context, _ *sessionruntime.Manager, admitted censusAdmittedSession, _ censusRuntimeConfig, _ bool, enumerator censusacquisition.FileEnumerator) (censusacquisition.Discovery, error) {
				discoveryCalls++
				events = append(events, "discover")
				var err error
				discovered, err = enumerator.Enumerate(ctx)
				if err != nil {
					return censusacquisition.Discovery{}, err
				}
				return gatewayCompleteDiscovery(t, workspace, censusacquisition.SessionIdentity{SessionID: admitted.sessionID, Generation: admitted.generation}), nil
			}
			acquireCalls := 0
			shell.acquireOverride = func(context.Context, censusRuntimeResult) (censusacquisition.Projection, *censusRuntimeFailure, censusFailureReason) {
				acquireCalls++
				return censusacquisition.Projection{}, censusAcquisitionFailure(), reasonAcquisitionFailed
			}
			request := censusRuntimeRequest(t, openCensusRuntimeRoot(t), runtimeRaw(t, "project", started.Generation, nil))
			request.PrivateCensusChunking = true
			completion := shell.run(context.Background(), request)
			if !reflect.DeepEqual(events, []string{"enumerate", "plan", "discover"}) {
				t.Fatalf("ASSERT_PRIVATE_CENSUS_ENUMERATE_PLAN_DISCOVER_ORDER: events=%v", events)
			}
			if planned.Authority != 0 {
				t.Fatalf("ASSERT_PRIVATE_CENSUS_SELECTED_PLAN_AUTHORITY_ZERO: authority=%d", planned.Authority)
			}
			want := planned.Partitions[0].Members
			if discoveryCalls != 1 || !reflect.DeepEqual(discovered, want) {
				t.Fatalf("ASSERT_PRIVATE_CENSUS_DISCOVERY_SELECTS_PARTITION_ZERO_EXACTLY_ONCE: calls=%d got=%+v want=%+v", discoveryCalls, discovered, want)
			}
			if acquireCalls != 1 || completion.DiscoveryDiagnostic != nil || completion.Diagnostic == nil || completion.Diagnostic.Stage != censusresult.StageAcquisition {
				t.Fatalf("ASSERT_PRIVATE_CENSUS_COMPLETE_DISCOVERY_REACHES_ACQUISITION: acquire=%d completion=%+v", acquireCalls, completion)
			}
			if fileCount == censusRuntimeSourcePartitionSize+1 && (len(planned.Partitions) != 2 || len(discovered) != censusRuntimeSourcePartitionSize || reflect.DeepEqual(discovered, planned.Files)) {
				t.Fatalf("ASSERT_PRIVATE_CENSUS_DISCOVERY_EXCLUDES_LATER_AND_OUTSIDE_MEMBERS: partitions=%+v discovered=%+v", planned.Partitions, discovered)
			}
			if fileCount < censusRuntimeSourcePartitionSize && !reflect.DeepEqual(discovered, planned.Files) {
				t.Fatalf("ASSERT_PRIVATE_CENSUS_ONE_PARTITION_MEMBERSHIP_PARITY: got=%+v want=%+v", discovered, planned.Files)
			}
		})
	}
}

func TestPrivateCensusRuntimeMalformedSourcePlanStopsDiscovery(t *testing.T) {
	mutations := map[string]func(*censussourceplan.Plan){
		"empty identity": func(plan *censussourceplan.Plan) { plan.Identity = "" },
		"identity inconsistency": func(plan *censussourceplan.Plan) {
			plan.Identity = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
		},
		"no partitions":             func(plan *censussourceplan.Plan) { plan.Partitions = nil },
		"wrong first ordinal":       func(plan *censussourceplan.Plan) { plan.Partitions[0].Ordinal = 1 },
		"empty first members":       func(plan *censussourceplan.Plan) { plan.Partitions[0].Members = nil },
		"inconsistent first member": func(plan *censussourceplan.Plan) { plan.Partitions[0].Members[0].Path = "different.go" },
		"wrong later ordinal":       func(plan *censussourceplan.Plan) { plan.Partitions[1].Ordinal = 2 },
		"empty later members":       func(plan *censussourceplan.Plan) { plan.Partitions[1].Members = nil },
		"omitted coverage": func(plan *censussourceplan.Plan) {
			plan.Partitions[0].Members = plan.Partitions[0].Members[:len(plan.Partitions[0].Members)-1]
		},
		"reordered coverage": func(plan *censussourceplan.Plan) {
			plan.Partitions[0].Members[0], plan.Partitions[0].Members[1] = plan.Partitions[0].Members[1], plan.Partitions[0].Members[0]
		},
		"partition overlap": func(plan *censussourceplan.Plan) {
			plan.Partitions[1].Members[0] = plan.Partitions[0].Members[len(plan.Partitions[0].Members)-1]
		},
		"partition gap": func(plan *censussourceplan.Plan) { plan.Partitions[1].Members = plan.Partitions[1].Members[1:] },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			workspace := t.TempDir()
			files := make([]censusacquisition.SourceFile, censusRuntimeSourcePartitionSize+2)
			for i := range files {
				name := fmt.Sprintf("file-%03d.go", i)
				uri, err := source.FileURI(filepath.Join(workspace, name))
				if err != nil {
					t.Fatal(err)
				}
				files[i] = censusacquisition.SourceFile{Path: name, URI: uri, LanguageID: "go"}
			}
			runtime, _, started := censusRuntimeFixture(t, censusRuntimeOptions{workspace: workspace, ready: true, documentSymbolSupport: true, callHierarchySupport: true})
			shell := newCensusRuntime(runtime)
			shell.enumerate = func(context.Context, censusAdmittedSession, censusRuntimeConfig) ([]censusacquisition.SourceFile, error) {
				return files, nil
			}
			shell.planSourceFiles = func(got []censusacquisition.SourceFile, workspace string, partitionSize int) (censussourceplan.Plan, error) {
				plan, err := censussourceplan.Build(got, workspace, partitionSize)
				if err == nil {
					mutate(&plan)
				}
				return plan, err
			}
			discoveryCalls := 0
			shell.discoverEnumerated = func(context.Context, *sessionruntime.Manager, censusAdmittedSession, censusRuntimeConfig, bool, censusacquisition.FileEnumerator) (censusacquisition.Discovery, error) {
				discoveryCalls++
				return censusacquisition.Discovery{}, nil
			}
			request := censusRuntimeRequest(t, openCensusRuntimeRoot(t), runtimeRaw(t, "project", started.Generation, nil))
			request.PrivateCensusChunking = true
			completion := shell.run(context.Background(), request)
			if discoveryCalls != 0 || completion.Diagnostic == nil || completion.Diagnostic.Stage != censusresult.Stage(censusStagePlanning) || completion.Diagnostic.Code != censusresult.Code(censusCodePlanningInvalid) || completion.Diagnostic.Detail != string(reasonPlanningInvalid) {
				t.Fatalf("ASSERT_PRIVATE_CENSUS_MALFORMED_PLAN_EXACT_PLANNING_DIAGNOSTIC: calls=%d completion=%+v", discoveryCalls, completion)
			}
		})
	}
}

func TestCensusRuntimeNonPrivateDiscoveryRouteUnchanged(t *testing.T) {
	runtime, _, started := censusRuntimeFixture(t, censusRuntimeOptions{ready: true, documentSymbolSupport: true, callHierarchySupport: true})
	shell := newCensusRuntime(runtime)
	discoveryCalls, enumerateCalls, plannedCalls, enumeratedDiscoveryCalls := 0, 0, 0, 0
	shell.discover = func(_ context.Context, _ *sessionruntime.Manager, admitted censusAdmittedSession, _ censusRuntimeConfig, _ bool) (censusacquisition.Discovery, error) {
		discoveryCalls++
		return censusacquisition.Discovery{Session: censusacquisition.SessionIdentity{SessionID: admitted.sessionID, Generation: admitted.generation}, Complete: true}, nil
	}
	shell.enumerate = func(context.Context, censusAdmittedSession, censusRuntimeConfig) ([]censusacquisition.SourceFile, error) {
		enumerateCalls++
		return nil, nil
	}
	shell.planSourceFiles = func([]censusacquisition.SourceFile, string, int) (censussourceplan.Plan, error) {
		plannedCalls++
		return censussourceplan.Plan{}, nil
	}
	shell.discoverEnumerated = func(context.Context, *sessionruntime.Manager, censusAdmittedSession, censusRuntimeConfig, bool, censusacquisition.FileEnumerator) (censusacquisition.Discovery, error) {
		enumeratedDiscoveryCalls++
		return censusacquisition.Discovery{}, nil
	}
	_, failure, _ := shell.execute(context.Background(), censusRuntimeRequest(t, openCensusRuntimeRoot(t), runtimeRaw(t, "project", started.Generation, nil)))
	if failure != nil || discoveryCalls != 1 || enumerateCalls != 0 || plannedCalls != 0 || enumeratedDiscoveryCalls != 0 {
		t.Fatalf("ASSERT_NON_PRIVATE_CENSUS_DISCOVERY_ROUTE_UNCHANGED: failure=%+v discover=%d enumerate=%d plan=%d enumerated_discover=%d", failure, discoveryCalls, enumerateCalls, plannedCalls, enumeratedDiscoveryCalls)
	}
}

func TestPrivateCensusRuntimePlanRefusalStopsDiscovery(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files []censusacquisition.SourceFile
	}{
		{name: "missing uri", files: []censusacquisition.SourceFile{{Path: "a.go", LanguageID: "go"}}},
		{name: "reparable path", files: []censusacquisition.SourceFile{{Path: "dir/../a.go", URI: "file:///workspace/a.go", LanguageID: "go"}}},
		{name: "path uri mismatch", files: []censusacquisition.SourceFile{{Path: "a.go", URI: "file:///workspace/b.go", LanguageID: "go"}}},
		{name: "invalid disposition", files: []censusacquisition.SourceFile{{Path: "a.go", URI: "file:///workspace/a.go", LanguageID: "go", Disposition: "selected"}}},
		{name: "empty enumeration"},
		{name: "malformed", files: []censusacquisition.SourceFile{{Path: "a.go", URI: "://malformed", LanguageID: "go"}}},
		{name: "duplicate", files: []censusacquisition.SourceFile{{Path: "a.go", URI: "file:///workspace/a.go"}, {Path: "a.go", URI: "file:///workspace/a.go"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runtime, _, started := censusRuntimeFixture(t, censusRuntimeOptions{ready: true, documentSymbolSupport: true, callHierarchySupport: true})
			shell := newCensusRuntime(runtime)
			shell.enumerate = func(context.Context, censusAdmittedSession, censusRuntimeConfig) ([]censusacquisition.SourceFile, error) {
				return append([]censusacquisition.SourceFile(nil), tc.files...), nil
			}
			discoveryCalls := 0
			shell.discoverEnumerated = func(context.Context, *sessionruntime.Manager, censusAdmittedSession, censusRuntimeConfig, bool, censusacquisition.FileEnumerator) (censusacquisition.Discovery, error) {
				discoveryCalls++
				return censusacquisition.Discovery{}, nil
			}
			request := censusRuntimeRequest(t, openCensusRuntimeRoot(t), runtimeRaw(t, "project", started.Generation, nil))
			request.PrivateCensusChunking = true
			completion := shell.run(context.Background(), request)
			if discoveryCalls != 0 || completion.Diagnostic == nil {
				t.Fatalf("ASSERT_CENSUS_SOURCE_PLAN_INVALID_IDENTITY_BEFORE_DISCOVERY: calls=%d completion=%+v", discoveryCalls, completion)
			}
		})
	}
}

func TestPrivateCensusRuntimePlanFailureIsNotDiscoveryFailure(t *testing.T) {
	runtime, _, started := censusRuntimeFixture(t, censusRuntimeOptions{ready: true, documentSymbolSupport: true, callHierarchySupport: true})
	shell := newCensusRuntime(runtime)
	shell.enumerate = func(context.Context, censusAdmittedSession, censusRuntimeConfig) ([]censusacquisition.SourceFile, error) {
		return []censusacquisition.SourceFile{{Path: "a.go", URI: "file:///workspace/a.go", LanguageID: "go"}}, nil
	}
	shell.planSourceFiles = func([]censusacquisition.SourceFile, string, int) (censussourceplan.Plan, error) {
		return censussourceplan.Plan{}, censussourceplan.ErrInvalidIdentity
	}
	discoveryCalls := 0
	shell.discoverEnumerated = func(context.Context, *sessionruntime.Manager, censusAdmittedSession, censusRuntimeConfig, bool, censusacquisition.FileEnumerator) (censusacquisition.Discovery, error) {
		discoveryCalls++
		return censusacquisition.Discovery{}, nil
	}
	request := censusRuntimeRequest(t, openCensusRuntimeRoot(t), runtimeRaw(t, "project", started.Generation, nil))
	request.PrivateCensusChunking = true
	_, failure, reason := shell.execute(context.Background(), request)
	if discoveryCalls != 0 || failure == nil || failure.stage == censusStageDiscovery || failure.code == censusCodeDiscoveryFailed || reason == reasonDiscoveryFailed {
		t.Fatalf("ASSERT_CENSUS_SOURCE_PLAN_FAILURE_DISTINCT_FROM_DISCOVERY: calls=%d failure=%+v reason=%q", discoveryCalls, failure, reason)
	}
}

func TestPrivateCensusRuntimeActivatedBoundaryUsesBroadLimitsOnce(t *testing.T) {
	for _, tc := range []struct {
		name            string
		activated       bool
		wantNodes       uint64
		wantEnumerate   int
		wantDiscoveries int
	}{
		{name: "disabled", wantNodes: 10, wantDiscoveries: 1},
		{name: "activated", activated: true, wantNodes: 100000, wantEnumerate: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			workspace := t.TempDir()
			uri, err := source.FileURI(filepath.Join(workspace, "a.go"))
			if err != nil {
				t.Fatal(err)
			}
			runtime, _, started := censusRuntimeFixture(t, censusRuntimeOptions{workspace: workspace, ready: true, documentSymbolSupport: true, callHierarchySupport: true})
			shell := newCensusRuntime(runtime)
			enumerateCalls, discoveryCalls := 0, 0
			var observed censusRuntimeConfig
			shell.enumerate = func(_ context.Context, _ censusAdmittedSession, options censusRuntimeConfig) ([]censusacquisition.SourceFile, error) {
				enumerateCalls++
				observed = options
				return []censusacquisition.SourceFile{{Path: "a.go", URI: uri, LanguageID: "go"}}, nil
			}
			shell.discover = func(_ context.Context, _ *sessionruntime.Manager, admitted censusAdmittedSession, options censusRuntimeConfig, _ bool) (censusacquisition.Discovery, error) {
				discoveryCalls++
				observed = options
				return censusacquisition.Discovery{Session: censusacquisition.SessionIdentity{SessionID: admitted.sessionID, Generation: admitted.generation}, Workspace: admitted.workspace, Complete: true}, nil
			}
			request := censusRuntimeRequest(t, openCensusRuntimeRoot(t), runtimeRaw(t, "project", started.Generation, map[string]any{"max_nodes": 10}))
			request.PrivateCensusChunking = tc.activated
			result, failure, _ := shell.execute(context.Background(), request)
			if failure != nil || enumerateCalls != tc.wantEnumerate || discoveryCalls != tc.wantDiscoveries || observed.maxNodes != tc.wantNodes || result.privateAcquisition != tc.activated {
				t.Fatalf("ASSERT_PRIVATE_CENSUS_BOUNDARY_LIMITS: activated=%v retained=%v enumerate=%d/%d discover=%d/%d max_nodes=%d want=%d failure=%+v", tc.activated, result.privateAcquisition, enumerateCalls, tc.wantEnumerate, discoveryCalls, tc.wantDiscoveries, observed.maxNodes, tc.wantNodes, failure)
			}
		})
	}
}

func TestPrivateCensusRuntimeDocumentSymbolNotificationPolicyAndDefaultPoisoning(t *testing.T) {
	for _, tc := range []struct {
		name       string
		private    bool
		wantSymbol int
		wantDocs   int
		wantState  session.Failure
	}{
		{name: "private-chunking", private: true, wantSymbol: 2, wantDocs: 2},
		{name: "default", wantSymbol: 1, wantDocs: 1, wantState: session.LifecycleConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			workspace := t.TempDir()
			writeRuntimeSource(t, workspace, "a.go")
			writeRuntimeSource(t, workspace, "b.go")
			runtime, starter, started := censusRuntimeFixture(t, censusRuntimeOptions{
				workspace: workspace, ready: true, documentSymbolSupport: true, callHierarchySupport: true,
				notifications: 65, results: map[string]json.RawMessage{"textDocument/documentSymbol": json.RawMessage(`[]`)},
			})
			request := censusRuntimeRequest(t, openCensusRuntimeRoot(t), runtimeRaw(t, "project", started.Generation, map[string]any{"sources": []string{"."}, "max_nodes": 2}))
			request.PrivateCensusChunking = tc.private
			shell := newCensusRuntime(runtime)
			var result censusRuntimeResult
			var failure *censusRuntimeFailure
			if tc.private {
				_ = shell.run(context.Background(), operation.Request(request))
			} else {
				result, failure, _ = shell.execute(context.Background(), request)
			}
			_, methods, _, _ := starter.snapshot()
			joined := strings.Join(methods, ",")
			if got := strings.Count(joined, "textDocument/documentSymbol"); got != tc.wantSymbol {
				t.Fatalf("ASSERT_PRIVATE_CENSUS_DOCUMENT_SYMBOL_NOTIFICATION_ROUTE: private=%t symbols=%d want=%d methods=%v failure=%+v discovery=%+v", tc.private, got, tc.wantSymbol, methods, failure, result.discovery)
			}
			if got := strings.Count(joined, "textDocument/didOpen"); got != tc.wantDocs {
				t.Fatalf("ASSERT_PRIVATE_CENSUS_NEXT_FILE_PREPARATION: private=%t documents=%d want=%d methods=%v failure=%+v", tc.private, got, tc.wantDocs, methods, failure)
			}
			_, stateFailure := runtime.Manager.Metadata(started.SessionID, started.Generation)
			if stateFailure != tc.wantState {
				t.Fatalf("ASSERT_PRIVATE_CENSUS_SESSION_STATE: private=%t got=%s want=%s failure=%+v discovery=%+v", tc.private, stateFailure, tc.wantState, failure, result.discovery)
			}
			if !tc.private {
				failures := result.discovery.ProviderFailures
				if len(failures) != 2 || failures[0].Stage != "DOCUMENT_SYMBOL" || failures[0].Code != "REQUEST_FAILED" || failures[1].Stage != "DOCUMENT_SUPPLY" || failures[1].Code != "REQUEST_FAILED" || failures[0].FileOrdinal == nil || *failures[0].FileOrdinal != 0 || failures[1].FileOrdinal == nil || *failures[1].FileOrdinal != 1 {
					t.Fatalf("ASSERT_PRIVATE_CENSUS_POISON_CASCADE_NOT_SOURCE_UNREADABLE: failures=%+v accounting=%+v", failures, result.discovery.Accounting)
				}
			}
		})
	}
}

func TestCensusRuntimeNonEmptyDiscoveryResponsesSurviveAndAreUsed(t *testing.T) {
	workspace := t.TempDir()
	writeRuntimeSource(t, workspace, "a.go")
	uri, err := source.FileURI(filepath.Join(workspace, "a.go"))
	if err != nil {
		t.Fatal(err)
	}
	rng := lsp.Range{Start: lsp.Position{Line: 2, Character: 1}, End: lsp.Position{Line: 2, Character: 8}}
	symbols, _ := json.Marshal([]lsp.DocumentSymbol{{Name: "Target", Kind: 12, Range: rng, SelectionRange: rng}})
	items, _ := json.Marshal([]lsp.CallHierarchyItem{{Name: "Target", Kind: 12, URI: uri, Range: rng, SelectionRange: rng}})
	runtime, starter, started := censusRuntimeFixture(t, censusRuntimeOptions{
		workspace: workspace, ready: true, documentSymbolSupport: true, callHierarchySupport: true,
		results: map[string]json.RawMessage{"textDocument/documentSymbol": symbols, "textDocument/prepareCallHierarchy": items},
	})
	result, failure, _ := newCensusRuntime(runtime).execute(context.Background(), censusRuntimeRequest(t, openCensusRuntimeRoot(t), runtimeRaw(t, "project", started.Generation, map[string]any{"sources": []string{"a.go"}})))
	if failure != nil {
		t.Fatalf("ASSERT_CENSUS_RUNTIME_NONEMPTY_RESPONSES_SUCCESS: %+v", failure)
	}
	if result.discovery.Session.SessionID != started.SessionID || result.discovery.Session.Generation != started.Generation || result.discovery.Accounting.SymbolDenominator != 1 || len(result.discovery.Accounting.Symbols) != 1 || len(result.discovery.Targets) != 1 || result.discovery.Targets[0].Name != "Target" || result.discovery.Targets[0].URI != uri {
		t.Fatalf("ASSERT_CENSUS_RUNTIME_DOCUMENT_SYMBOLS_AND_PREPARED_ITEMS_SURVIVE: discovery=%+v started=%+v", result.discovery, started)
	}
	_, methods, _, _ := starter.snapshot()
	if strings.Count(strings.Join(methods, ","), "textDocument/documentSymbol") != 1 || strings.Count(strings.Join(methods, ","), "textDocument/prepareCallHierarchy") != 1 {
		t.Fatalf("ASSERT_CENSUS_RUNTIME_EXACT_DISCOVERY_METHODS: %v", methods)
	}
}

func TestCensusRuntimeMalformedDocumentSymbolsRemainClosedAndPrivacySafe(t *testing.T) {
	workspace := t.TempDir()
	writeRuntimeSource(t, workspace, "a.go")
	const privatePayload = `{"private":"SECRET_PAYLOAD"`
	runtime, starter, started := censusRuntimeFixture(t, censusRuntimeOptions{
		workspace: workspace, ready: true, documentSymbolSupport: true, callHierarchySupport: true,
		results: map[string]json.RawMessage{"textDocument/documentSymbol": json.RawMessage(privatePayload)},
	})
	result, failure, _ := newCensusRuntime(runtime).execute(context.Background(), censusRuntimeRequest(t, openCensusRuntimeRoot(t), runtimeRaw(t, "project", started.Generation, map[string]any{"sources": []string{"a.go"}})))
	if failure != nil {
		t.Fatalf("ASSERT_CENSUS_RUNTIME_MALFORMED_SYMBOLS_ACCOUNTED_NOT_FAILED: %+v", failure)
	}
	accounting := result.discovery.Accounting
	if result.discovery.Session.SessionID != started.SessionID || result.discovery.Session.Generation != started.Generation || result.options.timeoutMS != 1000 || result.options.requestTimeoutMS != 500 {
		t.Fatalf("ASSERT_CENSUS_RUNTIME_MALFORMED_EXACT_IDENTITY_DEADLINES: result=%+v started=%+v", result, started)
	}
	if result.discovery.Complete || accounting.FileDenominator != 1 || len(accounting.Files) != 1 || accounting.Files[0].Ordinal != 0 || accounting.Files[0].Disposition != census.FileDocumentSymbolFailed || accounting.SymbolDenominator != 0 || len(accounting.Symbols) != 0 || len(result.discovery.Targets) != 0 {
		t.Fatalf("ASSERT_CENSUS_RUNTIME_MALFORMED_EXACT_CLOSED_ACCOUNTING: %+v", result.discovery)
	}
	if err := accounting.Validate(); err != nil {
		t.Fatalf("ASSERT_CENSUS_RUNTIME_MALFORMED_ACCOUNTING_VALID: %v", err)
	}
	if result.discovery.FileLedger.Denominator != 1 || len(result.discovery.FileLedger.Entries) != 1 || result.discovery.FileLedger.Entries[0].Ordinal != 0 || result.discovery.FileLedger.Entries[0].Disposition != string(census.FileDocumentSymbolFailed) || result.discovery.SymbolLedger.Denominator != 0 || len(result.discovery.SymbolLedger.Entries) != 0 {
		t.Fatalf("ASSERT_CENSUS_RUNTIME_MALFORMED_EXACT_CLOSED_LEDGERS: file=%+v symbol=%+v", result.discovery.FileLedger, result.discovery.SymbolLedger)
	}
	_, methods, _, _ := starter.snapshot()
	joined := strings.Join(methods, ",")
	if strings.Count(joined, "textDocument/didOpen") != 1 || strings.Count(joined, "textDocument/documentSymbol") != 1 || strings.Contains(joined, "textDocument/prepareCallHierarchy") {
		t.Fatalf("ASSERT_CENSUS_RUNTIME_MALFORMED_EXACT_METHODS_NO_PREPARE: %v", methods)
	}
	resultSurface, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	failureSurface, err := json.Marshal(censusRuntimeFailure{stage: censusStageAcquisition, code: censusCodeDiscoveryFailed})
	if err != nil {
		t.Fatal(err)
	}
	for name, surface := range map[string][]byte{"result": resultSurface, "failure": failureSurface} {
		if string(surface) != `{}` || strings.Contains(string(surface), privatePayload) || strings.Contains(string(surface), workspace) || strings.Contains(string(surface), "a.go") {
			t.Fatalf("ASSERT_CENSUS_RUNTIME_MALFORMED_PRIVATE_%s_SURFACE: %s", name, surface)
		}
	}
}

func TestCensusRuntimeDeadlineCancellationAndConstructionSurface(t *testing.T) {
	runtime, starter, started := censusRuntimeFixture(t, censusRuntimeOptions{ready: true, documentSymbolSupport: true, callHierarchySupport: true})
	before := runtime.Census()
	beforeStarts, beforeMethods, beforeTeardown, beforeClose := starter.snapshot()
	shell := newCensusRuntime(runtime)
	if shell == nil || !reflect.DeepEqual(before, runtime.Census()) {
		t.Fatal("ASSERT_CENSUS_RUNTIME_CONSTRUCTION_NO_SURFACE_MUTATION")
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Millisecond))
	defer cancel()
	_, failure, _ := shell.execute(ctx, censusRuntimeRequest(t, openCensusRuntimeRoot(t), runtimeRaw(t, "project", started.Generation, nil)))
	afterStarts, afterMethods, afterTeardown, afterClose := starter.snapshot()
	if failure == nil || failure.stage != censusStageAcquisition || beforeStarts != afterStarts || !reflect.DeepEqual(beforeMethods, afterMethods) || beforeTeardown != afterTeardown || beforeClose != afterClose {
		t.Fatalf("ASSERT_CENSUS_RUNTIME_CANCELLED_NOT_COMPLETE_NO_LIFECYCLE: failure=%+v methods=%v/%v", failure, beforeMethods, afterMethods)
	}
}

func TestPlannedBatchUsesAdmittedHostManagerWithoutLifecycleDelta(t *testing.T) {
	workspace := t.TempDir()
	runtime, starter, started := censusRuntimeFixture(t, censusRuntimeOptions{workspace: workspace, ready: true, documentSymbolSupport: true, callHierarchySupport: true})
	admitted, failure, _ := newCensusExecutor(runtime).execute(context.Background(), runtimeRaw(t, "project", started.Generation, nil))
	if failure != nil {
		t.Fatalf("ASSERT_PLANNED_BATCH_MCP_ADMITTED: %+v", failure)
	}
	zero, coordinate := 0, uint32(0)
	uri, err := source.FileURI(filepath.Join(workspace, "a.go"))
	if err != nil {
		t.Fatal(err)
	}
	seeds, err := seedformat.EncodeCanonical(seedformat.File{SchemaVersion: seedformat.Version, CoordinateConvention: seedformat.CoordinateConvention, Defaults: seedformat.Defaults{DownDepth: &zero, UpDepth: &zero}, Seeds: []seedformat.Seed{{Type: seedformat.PositionType, Position: &seedformat.Position{Label: "root", Path: "a.go", Line: 1, Column: 1}}}}, workspace)
	if err != nil {
		t.Fatal(err)
	}
	manifest := acquisitionengine.Manifest{SchemaVersion: acquisitionengine.ManifestVersion, CoordinateConvention: "zero-based-session", Root: acquisitionengine.Target{ID: "root", Locator: acquisition.Locator{URI: uri, Line: &coordinate, Character: &coordinate}, DownDepth: &zero, UpDepth: &zero}, RequiredTargets: []acquisitionengine.Target{}}
	beforeStarts, beforeMethods, beforeTeardown, beforeClose := starter.snapshot()
	result, operationFailure := executeCensusPlannedBatch(context.Background(), runtime, admitted, "census:c:000000:b", manifest, seeds)
	afterStarts, afterMethods, afterTeardown, afterClose := starter.snapshot()
	if operationFailure != nil || len(result.RawV5) == 0 || result.SessionID != started.SessionID || result.Generation != started.Generation {
		t.Fatalf("ASSERT_PLANNED_BATCH_MCP_RAW_V5: result=%+v failure=%+v", result, operationFailure)
	}
	if beforeStarts != afterStarts || beforeTeardown != afterTeardown || beforeClose != afterClose || len(beforeMethods) != len(afterMethods) {
		t.Fatalf("ASSERT_PLANNED_BATCH_MCP_NO_LIFECYCLE_DELTA: methods=%v/%v lifecycle=%d/%d %d/%d %d/%d", beforeMethods, afterMethods, beforeStarts, afterStarts, beforeTeardown, afterTeardown, beforeClose, afterClose)
	}
}

func TestCensusRuntimeAcquisitionLimitsAreFixed(t *testing.T) {
	limits, err := censusRuntimeAcquisitionLimits(censusRuntimeConfig{maxNodes: 321, timeoutMS: 987, requestTimeoutMS: 654}, false)
	if err != nil {
		t.Fatal(err)
	}
	if limits.MaxNodes == nil || *limits.MaxNodes != 321 || limits.MaxRequests == nil || *limits.MaxRequests != 1000 || limits.MaxEvidenceBytes == nil || *limits.MaxEvidenceBytes != 4<<20 || limits.MaxPathWork == nil || *limits.MaxPathWork != 100000 || limits.TimeoutMS == nil || *limits.TimeoutMS != 987 || limits.RequestTimeoutMS == nil || *limits.RequestTimeoutMS != 654 || limits.MaxResponseBytes == nil || *limits.MaxResponseBytes != 4<<20 || limits.MaxMessages == nil || *limits.MaxMessages != 64 {
		t.Fatalf("ASSERT_MCP_CENSUS_FIXED_ACQUISITION_LIMITS: %+v", limits)
	}
}

func TestPrivateCensusAcquisitionRequestBudgetExceedsLegacy1000(t *testing.T) {
	limits, err := censusRuntimeAcquisitionLimits(censusRuntimeConfig{maxNodes: 10000, maxBatchTargets: 63, timeoutMS: 987, requestTimeoutMS: 654}, true)
	if err != nil {
		t.Fatal(err)
	}
	if limits.MaxRequests == nil || *limits.MaxRequests != 20191 {
		t.Fatalf("ASSERT_PRIVATE_CENSUS_ACQUISITION_REQUEST_BUDGET: max_requests=%v want=20191", limits.MaxRequests)
	}
}

func TestEffectiveCensusDeadlineNeverExtendsParent(t *testing.T) {
	configured := time.Now().Add(time.Hour)
	parentDeadline := time.Now().Add(time.Minute)
	parent, cancel := context.WithDeadline(context.Background(), parentDeadline)
	defer cancel()
	if got := effectiveCensusDeadline(parent, configured); !got.Equal(parentDeadline) {
		t.Fatalf("ASSERT_MCP_CENSUS_EFFECTIVE_PARENT_DEADLINE: got=%v want=%v", got, parentDeadline)
	}
	if got := effectiveCensusDeadline(context.Background(), configured); !got.Equal(configured) {
		t.Fatalf("ASSERT_MCP_CENSUS_CONFIGURED_DEADLINE: got=%v want=%v", got, configured)
	}
}

func TestCensusAcquisitionBranchTraceFiniteTagWhitelist(t *testing.T) {
	for _, tag := range []string{"BEFORE_ACQUIRE", "UNMAPPED_OPERATION_CODE", "RECORDER_ABSENT", "RECORDER_REJECTED", "RECORD_ACCEPTED", "CORE_ERROR_INPUT", "CORE_ERROR_DISCOVERY", "CORE_ERROR_PLANNING", "CORE_ERROR_BATCH_ACQUIRE", "CORE_ERROR_BATCH_ADMISSION", "CORE_ERROR_MANIFEST", "BATCH_RESULT_INCOMPLETE", "BATCH_RESULT_IDENTITY_DRIFT"} {
		if !validCensusAcquisitionBranchTag(tag) {
			t.Fatalf("ASSERT_FINITE_BRANCH_TAG_ALLOWED: %q", tag)
		}
	}
	for _, tag := range []string{"", "OTHER", "ACQUISITION_FAILED", "raw-error", "file:///private/path"} {
		if validCensusAcquisitionBranchTag(tag) {
			t.Fatalf("ASSERT_FINITE_BRANCH_TAG_REJECTED: %q", tag)
		}
	}
}

func TestCensusAcquisitionBranchTraceRequiresOptIn(t *testing.T) {
	t.Setenv(runtimeTracePathEnv, "")
	runtimeTraceState.Lock()
	before := len(runtimeTraceState.markers)
	runtimeTraceState.Unlock()
	censusAcquisitionBranchTrace(context.Background(), "BEFORE_ACQUIRE")
	runtimeTraceState.Lock()
	after := len(runtimeTraceState.markers)
	runtimeTraceState.Unlock()
	if after != before {
		t.Fatalf("ASSERT_PRIVATE_BRANCH_TRACE_DISABLED_WITHOUT_OPT_IN: before=%d after=%d", before, after)
	}
}

func TestCensusRuntimeBatchAcquirerMapsPrivateFailureTaxonomyAndOrdinal(t *testing.T) {
	cases := []struct{ code, category string }{
		{"CENSUS_BATCH_INVALID_INPUT", "INVALID_INPUT"},
		{"CENSUS_BATCH_SESSION_DRIFT", "SESSION_DRIFT"},
		{"CENSUS_BATCH_CANCELLED", "CANCELLED"},
		{"CENSUS_BATCH_ACQUISITION_FAILED", "ACQUISITION"},
		{"CENSUS_BATCH_ADMISSION_FAILED", "ADMISSION"},
	}
	for _, tc := range cases {
		t.Run(tc.category, func(t *testing.T) {
			a := censusRuntimeBatchAcquirer{admitted: censusAdmittedSession{sessionID: "s", generation: 1}, execute: func(context.Context, *hostSelectorRuntime, censusAdmittedSession, string, acquisitionengine.Manifest, []byte) (acquisitionorchestration.PlannedBatchResult, *operation.Failure) {
				return acquisitionorchestration.PlannedBatchResult{}, &operation.Failure{Code: tc.code}
			}}
			_, err := a.AcquireV5(context.Background(), censusacquisition.BatchRequest{Session: censusacquisition.SessionIdentity{SessionID: "s", Generation: 1}, CensusID: "c", BatchID: "b", Ordinal: 7, CanonicalSeedsV2: []byte("seed"), Targets: []censusacquisition.PreparedTarget{{URI: "file:///w/a.go", SelectionRange: lsp.Range{Start: lsp.Position{}}, Name: "Target", Kind: 12, SymbolIdentity: "a.go#0:0:12:Target:0"}}})
			var typed *censusPrivateFailure
			if !errors.As(err, &typed) || typed.category != tc.category || typed.ordinal == nil || *typed.ordinal != 7 {
				t.Fatalf("err=%T %+v", err, err)
			}
			if tc.category == "ACQUISITION" && (typed.operationCode == "" || typed.operationCategory != "ACQUISITION") {
				t.Fatalf("operation evidence=%+v", typed)
			}
			if tc.category != "ACQUISITION" && (typed.operationCode != "" || typed.operationCategory != "") {
				t.Fatalf("unexpected operation evidence=%+v", typed)
			}
		})
	}
	t.Run("canonical input diagnostic", func(t *testing.T) {
		_, _, validationErr := acquisitionengine.CanonicalInput([]byte(`{"unexpected":"private"}`))
		a := censusRuntimeBatchAcquirer{admitted: censusAdmittedSession{sessionID: "s", generation: 1}, execute: func(context.Context, *hostSelectorRuntime, censusAdmittedSession, string, acquisitionengine.Manifest, []byte) (acquisitionorchestration.PlannedBatchResult, *operation.Failure) {
			return acquisitionorchestration.PlannedBatchResult{}, &operation.Failure{Code: "CENSUS_BATCH_INVALID_INPUT", Err: validationErr}
		}}
		_, err := a.AcquireV5(context.Background(), censusacquisition.BatchRequest{Session: censusacquisition.SessionIdentity{SessionID: "s", Generation: 1}, CensusID: "c", BatchID: "b", Ordinal: 6, CanonicalSeedsV2: []byte("seed"), Targets: []censusacquisition.PreparedTarget{{URI: "file:///w/a.go", SelectionRange: lsp.Range{Start: lsp.Position{}}, Name: "Target", Kind: 12, SymbolIdentity: "a.go#0:0:12:Target:0"}}})
		var typed *censusPrivateFailure
		if !errors.As(err, &typed) || typed.category != "INVALID_INPUT" || typed.ordinal == nil || *typed.ordinal != 6 || typed.failedField != "envelope" || typed.failedInvariant != "canonical_input_decode" {
			t.Fatalf("private failure=%+v err=%v", typed, err)
		}
	})
	t.Run("unknown", func(t *testing.T) {
		a := censusRuntimeBatchAcquirer{admitted: censusAdmittedSession{sessionID: "s", generation: 1}, execute: func(context.Context, *hostSelectorRuntime, censusAdmittedSession, string, acquisitionengine.Manifest, []byte) (acquisitionorchestration.PlannedBatchResult, *operation.Failure) {
			return acquisitionorchestration.PlannedBatchResult{}, &operation.Failure{Code: "UNKNOWN"}
		}}
		if _, err := a.AcquireV5(context.Background(), censusacquisition.BatchRequest{Session: censusacquisition.SessionIdentity{SessionID: "s", Generation: 1}, CensusID: "c", BatchID: "b", CanonicalSeedsV2: []byte("seed"), Targets: []censusacquisition.PreparedTarget{{URI: "file:///w/a.go", SelectionRange: lsp.Range{Start: lsp.Position{}}, Name: "Target", Kind: 12, SymbolIdentity: "a.go#0:0:12:Target:0"}}}); err == nil {
			t.Fatal("unknown failure code accepted")
		}
	})
}

func TestCensusRuntimeBatchAcquirerDistinguishesSuccessfulBatchResultPredicates(t *testing.T) {
	for _, tc := range []struct {
		name    string
		result  acquisitionorchestration.PlannedBatchResult
		wantTag string
	}{
		{name: "incomplete", result: acquisitionorchestration.PlannedBatchResult{SessionID: "s", Generation: 7}, wantTag: "BATCH_RESULT_INCOMPLETE"},
		{name: "identity drift", result: acquisitionorchestration.PlannedBatchResult{SessionID: "other", Generation: 7, BoundedTraversalComplete: true}, wantTag: "BATCH_RESULT_IDENTITY_DRIFT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Chmod(dir, 0700); err != nil {
				t.Fatal(err)
			}
			tracePath := filepath.Join(dir, "runtime.trace")
			t.Setenv(runtimeTracePathEnv, tracePath)
			stop, err := startRuntimeFlightRecorder()
			if err != nil {
				t.Fatal(err)
			}
			a := censusRuntimeBatchAcquirer{admitted: censusAdmittedSession{sessionID: "s", generation: 7}, execute: func(context.Context, *hostSelectorRuntime, censusAdmittedSession, string, acquisitionengine.Manifest, []byte) (acquisitionorchestration.PlannedBatchResult, *operation.Failure) {
				return tc.result, nil
			}}
			request := censusacquisition.BatchRequest{Session: censusacquisition.SessionIdentity{SessionID: "s", Generation: 7}, CensusID: "c", BatchID: "b", Ordinal: 6, CanonicalSeedsV2: []byte("seed"), Targets: []censusacquisition.PreparedTarget{{URI: "file:///w/a.go", SelectionRange: lsp.Range{Start: lsp.Position{}}, Name: "Target", Kind: 12, SymbolIdentity: "a.go#0:0:12:Target:0"}}}
			_, err = a.AcquireV5(context.Background(), request)
			if err == nil {
				stop()
				t.Fatal("ASSERT_BATCH_RESULT_ERROR_NON_NIL")
			}
			var typed *censusPrivateFailure
			if tc.name == "incomplete" {
				if !errors.As(err, &typed) || typed.category != "ACQUISITION" || typed.ordinal == nil || *typed.ordinal != 6 || typed.operationCode != "CENSUS_BATCH_INCOMPLETE" || typed.operationCategory != "ACQUISITION" {
					stop()
					t.Fatalf("ASSERT_INCOMPLETE_TYPED_PRIVATE_FAILURE: failure=%+v err=%v", typed, err)
				}
			} else if err.Error() != "census batch execution incomplete or identity drifted" {
				stop()
				t.Fatalf("ASSERT_IDENTITY_DRIFT_GENERIC_ERROR: err=%v", err)
			} else if errors.As(err, &typed) && typed.operationCode == "CENSUS_BATCH_INCOMPLETE" {
				stop()
				t.Fatalf("ASSERT_IDENTITY_DRIFT_NOT_INCOMPLETE: %+v", typed)
			}
			stop()
			traceBytes, readErr := os.ReadFile(tracePath)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if !strings.Contains(string(traceBytes), "CENSUS_ACQUISITION_BRANCH tag="+tc.wantTag) {
				t.Fatalf("ASSERT_BATCH_RESULT_PRIVATE_TAG: expected=%s", tc.wantTag)
			}
		})
	}
}

func TestCensusRuntimeBatchAcquirerDeterministicRequestAndReceipt(t *testing.T) {
	line, character := uint32(4), uint32(2)
	down, up := 3, 1
	request := censusacquisition.BatchRequest{
		Session: censusacquisition.SessionIdentity{SessionID: "s", Generation: 7}, CensusID: "c", BatchID: "b", Ordinal: 2,
		DownDepth: down, UpDepth: up, CanonicalSeedsV2: []byte("seeds"),
		Targets: []censusacquisition.PreparedTarget{{CensusOrdinal: 0, URI: "file:///w/a.go", SelectionRange: lsp.Range{Start: lsp.Position{Line: line, Character: character}}, Name: "Target", Kind: 12, SymbolIdentity: "a.go#4:2:12:Target:0"}},
	}
	maxNodes := 10
	acquirer := censusRuntimeBatchAcquirer{
		admitted: censusAdmittedSession{sessionID: "s", generation: 7}, limits: acquisitionops.Limits{MaxNodes: &maxNodes},
		execute: func(_ context.Context, _ *hostSelectorRuntime, _ censusAdmittedSession, requestID string, manifest acquisitionengine.Manifest, seeds []byte) (acquisitionorchestration.PlannedBatchResult, *operation.Failure) {
			if requestID != "census:c:000002:b" || *manifest.Root.DownDepth != down || *manifest.Root.UpDepth != up || string(seeds) != "seeds" {
				t.Fatalf("ASSERT_MCP_CENSUS_BATCH_EXACT_REQUEST: id=%s manifest=%+v seeds=%q", requestID, manifest, seeds)
			}
			seeds[0] = 'X'
			return acquisitionorchestration.PlannedBatchResult{SessionID: "s", Generation: 7, RawV5: []byte("raw"), BoundedTraversalComplete: true}, nil
		},
	}
	got, err := acquirer.AcquireV5(context.Background(), request)
	if err != nil || got.Session != request.Session || string(got.Raw) != "raw" || string(request.CanonicalSeedsV2) != "seeds" {
		t.Fatalf("ASSERT_MCP_CENSUS_BATCH_RESULT_AND_COPY: got=%+v err=%v seeds=%q", got, err, request.CanonicalSeedsV2)
	}
	acquirer.execute = func(context.Context, *hostSelectorRuntime, censusAdmittedSession, string, acquisitionengine.Manifest, []byte) (acquisitionorchestration.PlannedBatchResult, *operation.Failure) {
		return acquisitionorchestration.PlannedBatchResult{SessionID: "s", Generation: 7, RawV5: []byte("raw")}, nil
	}
	if _, err := acquirer.AcquireV5(context.Background(), request); err == nil {
		t.Fatal("ASSERT_MCP_CENSUS_BATCH_FALSE_RECEIPT_REJECTED")
	}
}

func TestCensusRuntimeAcquireRejectsIncompleteDiscoveryBeforeBatch(t *testing.T) {
	runtime, _, started := censusRuntimeFixture(t, censusRuntimeOptions{ready: true, documentSymbolSupport: true, callHierarchySupport: true})
	result := censusRuntimeResult{
		admitted:  censusAdmittedSession{sessionID: started.SessionID, generation: started.Generation},
		options:   censusRuntimeConfig{downDepth: 1, maxNodes: 10, timeoutMS: 1000, requestTimeoutMS: 500},
		discovery: censusacquisition.Discovery{Session: censusacquisition.SessionIdentity{SessionID: started.SessionID, Generation: started.Generation}, Complete: false},
		deadline:  time.Now().Add(time.Second),
	}
	projection, failure, _ := newCensusRuntime(runtime).acquire(context.Background(), result)
	if failure == nil || failure.stage != censusStageDiscovery || failure.code != censusCodeDiscoveryFailed || len(projection.Constituents) != 0 {
		t.Fatalf("ASSERT_MCP_CENSUS_INCOMPLETE_DISCOVERY_CLASSIFIED_NO_PROJECTION: projection=%+v failure=%+v", projection, failure)
	}
}

func TestCensusRuntimePrivateTypedSurface(t *testing.T) {
	for _, value := range []any{censusRuntime{}, censusRuntimeConfig{}, censusRuntimeResult{}, censusRuntimeFailure{}} {
		typ := reflect.TypeOf(value)
		if typ.NumMethod() != 0 {
			t.Fatalf("ASSERT_CENSUS_RUNTIME_PRIVATE_METHODS: %v", typ)
		}
		for i := 0; i < typ.NumField(); i++ {
			if typ.Field(i).IsExported() {
				t.Fatalf("ASSERT_CENSUS_RUNTIME_PRIVATE_FIELD: %s", typ.Field(i).Name)
			}
		}
	}
	var _ censusacquisition.Discovery
}

type censusRecordingPreparer struct {
	result   sessionruntime.DocumentResult
	calls    int
	requests []sessionruntime.DocumentRequest
	before   func()
}

func (p *censusRecordingPreparer) PrepareDocument(_ context.Context, request sessionruntime.DocumentRequest) sessionruntime.DocumentResult {
	p.calls++
	p.requests = append(p.requests, request)
	if p.before != nil {
		p.before()
	}
	return p.result
}

func TestCensusRuntimeSupplierFailureMappingClosure(t *testing.T) {
	file := censusacquisition.SourceFile{Path: "private/path.go", URI: "file:///private/path.go", LanguageID: "go"}
	lifecycle := []struct {
		manager sessionruntime.LifecycleSubcause
		census  censusacquisition.LifecycleSubcause
	}{
		{sessionruntime.LifecycleSubcauseManagerClosed, censusacquisition.LifecycleSubcauseManagerClosed},
		{sessionruntime.LifecycleSubcauseSessionNotReady, censusacquisition.LifecycleSubcauseSessionNotReady},
		{sessionruntime.LifecycleSubcauseProtocolOwned, censusacquisition.LifecycleSubcauseProtocolOwned},
		{sessionruntime.LifecycleSubcauseLeaseClosing, censusacquisition.LifecycleSubcauseLeaseClosing},
		{sessionruntime.LifecycleSubcauseLeaseTokenMismatch, censusacquisition.LifecycleSubcauseLeaseTokenMismatch},
		{sessionruntime.LifecycleSubcauseLanguageIDConflict, censusacquisition.LifecycleSubcauseLanguageIDConflict},
	}
	produced := sessionruntime.ProducedLifecycleSubcauses()
	if len(produced) != len(lifecycle) {
		t.Fatalf("ASSERT_CENSUS_SUPPLIER_LIFECYCLE_MAPPING_CLOSED got=%v want=%d", produced, len(lifecycle))
	}
	for _, supported := range produced {
		found := false
		for _, pair := range lifecycle {
			found = found || pair.manager == supported
		}
		if !found {
			t.Fatalf("ASSERT_CENSUS_SUPPLIER_LIFECYCLE_MAPPING_CLOSED unsupported=%q produced=%v", supported, produced)
		}
	}
	cases := []struct {
		name      string
		result    sessionruntime.DocumentResult
		want      error
		subcause  censusacquisition.LifecycleSubcause
		configure func(*censusRecordingPreparer, context.Context) context.Context
	}{
		{name: "poisoned", result: sessionruntime.DocumentResult{Failure: session.SessionPoisoned}, want: censusacquisition.ErrSessionPoisoned},
		{name: "session-not-found", result: sessionruntime.DocumentResult{Failure: session.SessionNotFound}, want: censusacquisition.ErrSessionUnavailable},
		{name: "supply-unavailable", result: sessionruntime.DocumentResult{Failure: sessionruntime.DocumentSupplyUnavailable}, want: censusacquisition.ErrSessionUnavailable},
		{name: "stale-generation-generic", result: sessionruntime.DocumentResult{Failure: session.StaleGeneration}},
		{name: "request-timeout-generic", result: sessionruntime.DocumentResult{Failure: session.RequestTimeout}},
		{name: "provider-failure-generic", result: sessionruntime.DocumentResult{Failure: sessionruntime.DocumentSourceUnavailable}},
		{name: "unknown-lifecycle-subcause-generic", result: sessionruntime.DocumentResult{Failure: session.LifecycleConflict, LifecycleSubcause: sessionruntime.LifecycleSubcause("PRIVATE_UNKNOWN")}},
		{name: "identity-mismatch-generic", result: sessionruntime.DocumentResult{URI: "file:///other.go", LanguageID: file.LanguageID}},
		{name: "post-attempt-canceled", result: sessionruntime.DocumentResult{Failure: session.RequestCancelled}, want: context.Canceled, configure: func(p *censusRecordingPreparer, _ context.Context) context.Context {
			ctx, cancel := context.WithCancel(context.Background())
			p.before = cancel
			return ctx
		}},
		{name: "post-attempt-deadline", result: sessionruntime.DocumentResult{Failure: session.RequestTimeout}, want: context.DeadlineExceeded, configure: func(p *censusRecordingPreparer, _ context.Context) context.Context {
			ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
			p.before = func() { <-ctx.Done(); cancel() }
			return ctx
		}},
	}
	for _, pair := range lifecycle {
		cases = append(cases, struct {
			name      string
			result    sessionruntime.DocumentResult
			want      error
			subcause  censusacquisition.LifecycleSubcause
			configure func(*censusRecordingPreparer, context.Context) context.Context
		}{name: "lifecycle-" + string(pair.manager), result: sessionruntime.DocumentResult{Failure: session.LifecycleConflict, LifecycleSubcause: pair.manager}, want: censusacquisition.ErrLifecycleConflict, subcause: pair.census})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			preparer := &censusRecordingPreparer{result: tc.result}
			ctx := context.Background()
			if tc.configure != nil {
				ctx = tc.configure(preparer, ctx)
			}
			err := (censusRuntimeSupplier{manager: preparer, sessionID: "private-session", generation: 7}).Supply(ctx, file)
			if preparer.calls != 1 || len(preparer.requests) != 1 || preparer.requests[0].SessionID != "private-session" || preparer.requests[0].Generation != 7 || preparer.requests[0].URI != file.URI || preparer.requests[0].LanguageID != file.LanguageID || !preparer.requests[0].CaptureSupply {
				t.Fatalf("ASSERT_CENSUS_SUPPLIER_EXACTLY_ONE_MANAGER_ATTEMPT calls=%d requests=%+v", preparer.calls, preparer.requests)
			}
			if tc.want != nil {
				if !errors.Is(err, tc.want) {
					t.Fatalf("ASSERT_CENSUS_SUPPLIER_TYPED_FAILURE name=%s err=%v want=%v", tc.name, err, tc.want)
				}
			} else if err == nil || errors.Is(err, censusacquisition.ErrSessionPoisoned) || errors.Is(err, censusacquisition.ErrLifecycleConflict) || errors.Is(err, censusacquisition.ErrSessionUnavailable) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("ASSERT_CENSUS_SUPPLIER_ORDINARY_FAILURE_REMAINS_GENERIC name=%s err=%v", tc.name, err)
			}
			var typed *censusacquisition.DocumentSupplyFailure
			if tc.subcause != "" {
				if !errors.As(err, &typed) || typed.LifecycleSubcause != tc.subcause {
					t.Fatalf("ASSERT_CENSUS_SUPPLIER_EXACT_LIFECYCLE_SUBCAUSE name=%s err=%T %v typed=%+v", tc.name, err, err, typed)
				}
			} else if errors.As(err, &typed) {
				t.Fatalf("ASSERT_CENSUS_SUPPLIER_NON_LIFECYCLE_NOT_WRAPPED name=%s typed=%+v", tc.name, typed)
			}
			if text := fmt.Sprintf("%v", err); strings.Contains(text, "private-session") || strings.Contains(text, file.Path) || strings.Contains(text, file.URI) {
				t.Fatalf("ASSERT_CENSUS_SUPPLIER_ERROR_PRIVACY name=%s err=%q", tc.name, text)
			}
		})
	}
}

func TestCensusRuntimeSupplierSessionNotFoundUsesProductionAdapterCustody(t *testing.T) {
	workspace := t.TempDir()
	writeRuntimeSource(t, workspace, "a.go")
	uri, err := source.FileURI(filepath.Join(workspace, "a.go"))
	if err != nil {
		t.Fatal(err)
	}
	runtime, starter, _ := censusRuntimeFixture(t, censusRuntimeOptions{workspace: workspace, ready: true, documentSymbolSupport: true, callHierarchySupport: true})
	beforeStarts, beforeMethods, beforeTeardown, beforeClose := starter.snapshot()
	discovery, err := censusRuntimeDiscover(
		context.Background(),
		runtime.Manager,
		censusAdmittedSession{sessionID: "missing-private-session", generation: 1, workspace: workspace},
		censusRuntimeConfig{sources: []string{"a.go"}, maxNodes: 1, requestTimeoutMS: 500},
		false,
	)
	afterStarts, afterMethods, afterTeardown, afterClose := starter.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(discovery.IncompleteCauses) != 1 || discovery.IncompleteCauses[0].Code != "SESSION_UNAVAILABLE" || discovery.IncompleteCauses[0].LifecycleSubcause != "" || len(discovery.ProviderFailures) != 1 || discovery.ProviderFailures[0].Code != "UNAVAILABLE" {
		t.Fatalf("ASSERT_CENSUS_SUPPLIER_SESSION_NOT_FOUND_TYPED_AND_PRIVATE_PUBLIC_COLLAPSE causes=%+v providers=%+v", discovery.IncompleteCauses, discovery.ProviderFailures)
	}
	if beforeStarts != afterStarts || beforeTeardown != afterTeardown || beforeClose != afterClose || !reflect.DeepEqual(beforeMethods, afterMethods) {
		t.Fatalf("ASSERT_CENSUS_SUPPLIER_FAILURE_NO_RETRY_RESTART_OR_SUBSTITUTION starts=%d/%d methods=%v/%v teardown=%d/%d close=%d/%d", beforeStarts, afterStarts, beforeMethods, afterMethods, beforeTeardown, afterTeardown, beforeClose, afterClose)
	}
	if strings.Contains(fmt.Sprintf("%+v", discovery.ProviderFailures), "missing-private-session") || strings.Contains(fmt.Sprintf("%+v", discovery.ProviderFailures), workspace) || strings.Contains(fmt.Sprintf("%+v", discovery.ProviderFailures), uri) {
		t.Fatalf("ASSERT_CENSUS_SUPPLIER_PUBLIC_FAILURE_PRIVACY providers=%+v", discovery.ProviderFailures)
	}
}
