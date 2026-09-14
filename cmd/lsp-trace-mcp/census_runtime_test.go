package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/acquisitionengine"
	"lsp-trace/internal/census"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/seedformat"
	"lsp-trace/internal/source"
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
	shell.admit = func(ctx context.Context, raw []byte) (censusAdmittedSession, *censusAdmissionFailure) {
		calls++
		return newCensusExecutor(runtime).execute(ctx, raw)
	}
	beforeStarts, beforeMethods, beforeTeardown, beforeClose := starter.snapshot()
	_, failure := shell.execute(context.Background(), censusRuntimeRequest(t, nil, runtimeRaw(t, "missing", started.Generation, nil)))
	afterStarts, afterMethods, afterTeardown, afterClose := starter.snapshot()
	if calls != 1 || failure == nil || failure.stage != censusStageAcquisition || !reflect.DeepEqual(beforeMethods, afterMethods) || beforeStarts != afterStarts || beforeTeardown != afterTeardown || beforeClose != afterClose {
		t.Fatalf("ASSERT_CENSUS_RUNTIME_ADMISSION_ONCE_SHORT_CIRCUIT: calls=%d failure=%+v methods=%v/%v lifecycle=%d/%d/%d/%d/%d/%d", calls, failure, beforeMethods, afterMethods, beforeStarts, afterStarts, beforeTeardown, afterTeardown, beforeClose, afterClose)
	}
}

func TestCensusRuntimeRequiresHostRootBeforeDiscovery(t *testing.T) {
	runtime, starter, started := censusRuntimeFixture(t, censusRuntimeOptions{ready: true, documentSymbolSupport: true, callHierarchySupport: true})
	before := runtime.Census()
	_, beforeMethods, _, _ := starter.snapshot()
	_, failure := newCensusRuntime(runtime).execute(context.Background(), censusRuntimeRequest(t, nil, runtimeRaw(t, "project", started.Generation, nil)))
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
	result, failure := newCensusRuntime(runtime).execute(context.Background(), censusRuntimeRequest(t, openCensusRuntimeRoot(t), raw))
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
	again, againFailure := newCensusRuntime(runtime).execute(context.Background(), censusRuntimeRequest(t, openCensusRuntimeRoot(t), runtimeRaw(t, "project", started.Generation, map[string]any{"sources": []string{"a.go"}, "max_nodes": 2})))
	if againFailure != nil || again.options.sources[0] != "a.go" || again.discovery.Accounting.Files[0].Disposition == census.FileUnreadable {
		t.Fatalf("ASSERT_CENSUS_RUNTIME_DEFENSIVE_COPY: again=%+v failure=%+v", again, againFailure)
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
	result, failure := newCensusRuntime(runtime).execute(context.Background(), censusRuntimeRequest(t, openCensusRuntimeRoot(t), runtimeRaw(t, "project", started.Generation, map[string]any{"sources": []string{"a.go"}})))
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
	result, failure := newCensusRuntime(runtime).execute(context.Background(), censusRuntimeRequest(t, openCensusRuntimeRoot(t), runtimeRaw(t, "project", started.Generation, map[string]any{"sources": []string{"a.go"}})))
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
	_, failure := shell.execute(ctx, censusRuntimeRequest(t, openCensusRuntimeRoot(t), runtimeRaw(t, "project", started.Generation, nil)))
	afterStarts, afterMethods, afterTeardown, afterClose := starter.snapshot()
	if failure == nil || failure.stage != censusStageAcquisition || beforeStarts != afterStarts || !reflect.DeepEqual(beforeMethods, afterMethods) || beforeTeardown != afterTeardown || beforeClose != afterClose {
		t.Fatalf("ASSERT_CENSUS_RUNTIME_CANCELLED_NOT_COMPLETE_NO_LIFECYCLE: failure=%+v methods=%v/%v", failure, beforeMethods, afterMethods)
	}
}

func TestPlannedBatchUsesAdmittedHostManagerWithoutLifecycleDelta(t *testing.T) {
	workspace := t.TempDir()
	runtime, starter, started := censusRuntimeFixture(t, censusRuntimeOptions{workspace: workspace, ready: true, documentSymbolSupport: true, callHierarchySupport: true})
	admitted, failure := newCensusExecutor(runtime).execute(context.Background(), runtimeRaw(t, "project", started.Generation, nil))
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
