package main

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/mcp"
	"lsp-trace/internal/mcpcontract"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
)

type censusTestProcess struct {
	in          *io.PipeReader
	stdin       *io.PipeWriter
	out         *io.PipeWriter
	stdout      *io.PipeReader
	metadata    string
	results     map[string]json.RawMessage
	initialized chan struct{}

	mu            sync.Mutex
	methods       []string
	teardownCalls int
	closeCalls    int
}

func newCensusTestProcess(metadata string, results map[string]json.RawMessage) *censusTestProcess {
	in, stdin := io.Pipe()
	stdout, out := io.Pipe()
	p := &censusTestProcess{in: in, stdin: stdin, out: out, stdout: stdout, metadata: metadata, results: results, initialized: make(chan struct{})}
	go p.serve()
	return p
}

func (p *censusTestProcess) serve() {
	reader := lspwire.NewReader(p.in, lspwire.DefaultLimits())
	writer := lspwire.NewWriter(p.out, lspwire.DefaultLimits())
	for {
		message, err := reader.Read()
		if err != nil {
			return
		}
		p.mu.Lock()
		p.methods = append(p.methods, message.Method)
		p.mu.Unlock()
		if message.Method == "initialized" {
			close(p.initialized)
			continue
		}
		if message.Method == "exit" {
			return
		}
		result := json.RawMessage(`null`)
		if message.Method == "initialize" {
			result = json.RawMessage(p.metadata)
		} else if configured := p.results[message.Method]; configured != nil {
			result = append(json.RawMessage(nil), configured...)
		}
		if len(message.ID) != 0 {
			if err := writer.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: message.ID, Result: result}); err != nil {
				return
			}
		}
	}
}

func (p *censusTestProcess) Stdin() io.WriteCloser { return p.stdin }
func (p *censusTestProcess) Stdout() io.ReadCloser { return p.stdout }
func (p *censusTestProcess) Teardown(context.Context) managedprocess.TeardownObservation {
	p.mu.Lock()
	p.teardownCalls++
	p.mu.Unlock()
	_ = p.stdin.Close()
	_ = p.in.Close()
	_ = p.out.Close()
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete}}}
}
func (p *censusTestProcess) Close() managedprocess.ResourceObservation {
	p.mu.Lock()
	p.closeCalls++
	p.mu.Unlock()
	_ = p.stdout.Close()
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}
func (p *censusTestProcess) snapshot() ([]string, int, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.methods...), p.teardownCalls, p.closeCalls
}

type censusTestStarter struct {
	metadata string
	results  map[string]json.RawMessage

	mu      sync.Mutex
	starts  int
	process *censusTestProcess
}

func (s *censusTestStarter) Start(context.Context, managedprocess.Spec) (sessionruntime.Child, managedprocess.StartObservation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.starts++
	s.process = newCensusTestProcess(s.metadata, s.results)
	return s.process, managedprocess.StartObservation{Kind: managedprocess.StartStarted}
}
func (s *censusTestStarter) snapshot() (int, []string, int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	methods, teardown, closeCalls := s.process.snapshot()
	return s.starts, methods, teardown, closeCalls
}

type censusRuntimeOptions struct {
	alias                 string
	workspace             string
	blankWorkspace        bool
	ready                 bool
	crashed               bool
	documentSymbolSupport bool
	callHierarchySupport  bool
	positionEncoding      string
	results               map[string]json.RawMessage
}

func censusRuntimeFixture(t *testing.T, options censusRuntimeOptions) (*hostSelectorRuntime, *censusTestStarter, sessionruntime.StartResult) {
	t.Helper()
	if options.alias == "" {
		options.alias = "project"
	}
	if options.workspace == "" && !options.blankWorkspace {
		options.workspace = t.TempDir()
	}
	if options.positionEncoding == "" {
		options.positionEncoding = "utf-16"
	}
	capabilities := map[string]any{
		"callHierarchyProvider":  options.callHierarchySupport,
		"documentSymbolProvider": options.documentSymbolSupport,
		"positionEncoding":       options.positionEncoding,
	}
	metadata, err := json.Marshal(map[string]any{"capabilities": capabilities})
	if err != nil {
		t.Fatal(err)
	}
	starter := &censusTestStarter{metadata: string(metadata), results: options.results}
	manager, err := sessionruntime.New(sessionruntime.Config{
		Limits:  sessionruntime.Limits{MaxSessions: 2, MaxRequests: 8, MaxChildren: 2, MaxCancels: 8, MaxTombstones: 8, MaxObservations: 128, MaxOperations: 8},
		Starter: starter,
	})
	if err != nil {
		t.Fatal(err)
	}
	var profile runtimeprofile.Profile
	if !options.blankWorkspace {
		validated, validateErr := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "census-test", Workspace: options.workspace, Profile: "fixture", EnvironmentReference: "test"})
		if validateErr != nil {
			t.Fatal(validateErr)
		}
		profile = runtimeprofile.Resolve(validated)
	}
	started := manager.Start(context.Background(), sessionruntime.StartRequest{Profile: profile, BootstrapAlias: options.alias, LanguageID: "go"})
	if started.Failure != "" {
		t.Fatalf("ASSERT_CENSUS_FIXTURE_STARTED: %+v", started)
	}
	if options.ready {
		pending := manager.BeginReadiness(context.Background(), started.SessionID, started.Generation, time.Now().Add(time.Second))
		ready, found := manager.WaitReadiness(context.Background(), pending.ID)
		if !found || ready.State != sessionruntime.ReadinessReady {
			t.Fatalf("ASSERT_CENSUS_FIXTURE_READY: %+v found=%t", ready, found)
		}
	}
	if options.ready {
		select {
		case <-starter.process.initialized:
		case <-time.After(time.Second):
			t.Fatal("ASSERT_CENSUS_FIXTURE_INITIALIZED_NOTIFICATION")
		}
	}
	if options.crashed {
		if failure := manager.ObserveCrash(started.SessionID, started.Generation); failure != "" {
			t.Fatalf("ASSERT_CENSUS_FIXTURE_CRASHED: %s", failure)
		}
	}
	runtime := newHostSelectorRuntime(manager, []bootstrapSession{{Alias: options.alias, SessionID: started.SessionID, Generation: started.Generation}})
	t.Cleanup(func() {
		if starter.process != nil {
			starter.process.Teardown(context.Background())
			starter.process.Close()
		}
		_ = manager.Shutdown(context.Background())
	})
	return runtime, starter, started
}

func censusRequest(alias string, generation uint64) []byte {
	raw, _ := json.Marshal(map[string]any{"session_id": alias, "generation": generation, "sources": []string{"."}})
	return raw
}

func assertCensusFailure(t *testing.T, failure *censusAdmissionFailure, stage censusFailureStage, code censusFailureCode) {
	t.Helper()
	if failure == nil || failure.stage != stage || failure.code != code {
		t.Fatalf("ASSERT_CENSUS_TYPED_FAILURE: got=%+v want=%s/%s", failure, stage, code)
	}
}

func TestCensusExecutorConstructionBoundary(t *testing.T) {
	var constructor func(*hostSelectorRuntime) *censusExecutor = newCensusExecutor
	if constructor == nil {
		t.Fatal("ASSERT_CENSUS_PRIVATE_CONCRETE_HOST_SELECTOR_CONSTRUCTOR")
	}
	for _, value := range []any{censusExecutor{}, censusAdmittedSession{}, censusAdmissionFailure{}} {
		typ := reflect.TypeOf(value)
		if typ.Name() == "" || typ.PkgPath() == "" || typ.NumMethod() != 0 {
			t.Fatalf("ASSERT_CENSUS_PRIVATE_NO_AUTHORITY_METHODS: %v", typ)
		}
		for i := 0; i < typ.NumField(); i++ {
			if typ.Field(i).IsExported() {
				t.Fatalf("ASSERT_CENSUS_PRIVATE_NO_EXPORTED_FIELDS: %s.%s", typ.Name(), typ.Field(i).Name)
			}
		}
	}
}

func TestCensusExecutorExactAliasGenerationSuccess(t *testing.T) {
	runtime, _, started := censusRuntimeFixture(t, censusRuntimeOptions{ready: true, documentSymbolSupport: true, callHierarchySupport: true})
	admitted, failure := newCensusExecutor(runtime).execute(context.Background(), censusRequest("project", started.Generation))
	if failure != nil || admitted.sessionID != started.SessionID || admitted.generation != started.Generation || admitted.workspace == "" || admitted.positionEncoding != "utf-16" {
		t.Fatalf("ASSERT_CENSUS_EXACT_ALIAS_GENERATION_ADMITTED: admitted=%+v failure=%+v", admitted, failure)
	}
}

func TestCensusExecutorAdmissionMatrix(t *testing.T) {
	t.Run("strict decode", func(t *testing.T) {
		runtime, _, _ := censusRuntimeFixture(t, censusRuntimeOptions{ready: true, documentSymbolSupport: true, callHierarchySupport: true})
		for name, raw := range map[string][]byte{
			"unknown":         []byte(`{"session_id":"project","generation":1,"sources":["."],"unknown":true}`),
			"duplicate":       []byte(`{"session_id":"project","session_id":"project","generation":1,"sources":["."]}`),
			"missing":         []byte(`{"session_id":"project","sources":["."]}`),
			"zero generation": []byte(`{"session_id":"project","generation":0,"sources":["."]}`),
		} {
			t.Run(name, func(t *testing.T) {
				_, failure := newCensusExecutor(runtime).execute(context.Background(), raw)
				assertCensusFailure(t, failure, censusStageConfig, censusCodeInvalidConfig)
			})
		}
	})

	for _, test := range []struct {
		name       string
		options    censusRuntimeOptions
		alias      string
		generation func(sessionruntime.StartResult) uint64
		stage      censusFailureStage
		code       censusFailureCode
	}{
		{name: "stale generation", options: censusRuntimeOptions{ready: true, documentSymbolSupport: true, callHierarchySupport: true}, alias: "project", generation: func(s sessionruntime.StartResult) uint64 { return s.Generation + 1 }, stage: censusStageAcquisition, code: censusCodeAcquisitionFailed},
		{name: "missing alias", options: censusRuntimeOptions{ready: true, documentSymbolSupport: true, callHierarchySupport: true}, alias: "missing", generation: func(s sessionruntime.StartResult) uint64 { return s.Generation }, stage: censusStageAcquisition, code: censusCodeAcquisitionFailed},
		{name: "non ready", options: censusRuntimeOptions{documentSymbolSupport: true, callHierarchySupport: true}, alias: "project", generation: func(s sessionruntime.StartResult) uint64 { return s.Generation }, stage: censusStageAcquisition, code: censusCodeAcquisitionFailed},
		{name: "crashed", options: censusRuntimeOptions{ready: true, crashed: true, documentSymbolSupport: true, callHierarchySupport: true}, alias: "project", generation: func(s sessionruntime.StartResult) uint64 { return s.Generation }, stage: censusStageAcquisition, code: censusCodeAcquisitionFailed},
		{name: "blank workspace", options: censusRuntimeOptions{blankWorkspace: true, ready: true, documentSymbolSupport: true, callHierarchySupport: true}, alias: "project", generation: func(s sessionruntime.StartResult) uint64 { return s.Generation }, stage: censusStageConfig, code: censusCodeInvalidConfig},
		{name: "document symbols unsupported", options: censusRuntimeOptions{ready: true, callHierarchySupport: true}, alias: "project", generation: func(s sessionruntime.StartResult) uint64 { return s.Generation }, stage: censusStageConfig, code: censusCodeInvalidConfig},
		{name: "call hierarchy unsupported", options: censusRuntimeOptions{ready: true, documentSymbolSupport: true}, alias: "project", generation: func(s sessionruntime.StartResult) uint64 { return s.Generation }, stage: censusStageConfig, code: censusCodeInvalidConfig},
		{name: "encoding unsupported", options: censusRuntimeOptions{ready: true, documentSymbolSupport: true, callHierarchySupport: true, positionEncoding: "utf-7"}, alias: "project", generation: func(s sessionruntime.StartResult) uint64 { return s.Generation }, stage: censusStageConfig, code: censusCodeInvalidConfig},
	} {
		t.Run(test.name, func(t *testing.T) {
			runtime, _, started := censusRuntimeFixture(t, test.options)
			_, failure := newCensusExecutor(runtime).execute(context.Background(), censusRequest(test.alias, test.generation(started)))
			assertCensusFailure(t, failure, test.stage, test.code)
		})
	}

	t.Run("generation conversion boundary", func(t *testing.T) {
		runtime, _, _ := censusRuntimeFixture(t, censusRuntimeOptions{ready: true, documentSymbolSupport: true, callHierarchySupport: true})
		_, maximumFailure := newCensusExecutor(runtime).execute(context.Background(), []byte(`{"session_id":"project","generation":9223372036854775807,"sources":["."]}`))
		assertCensusFailure(t, maximumFailure, censusStageAcquisition, censusCodeAcquisitionFailed)
		_, overflowFailure := newCensusExecutor(runtime).execute(context.Background(), []byte(`{"session_id":"project","generation":9223372036854775808,"sources":["."]}`))
		assertCensusFailure(t, overflowFailure, censusStageConfig, censusCodeInvalidConfig)
		if math.MaxInt64 <= 0 {
			t.Fatal("ASSERT_CENSUS_GENERATION_MAX_INT64_PLATFORM")
		}
	})

	t.Run("canonical absolute workspace", func(t *testing.T) {
		base := t.TempDir()
		requested := filepath.Join(base, "parent", "..", "workspace")
		runtime, _, started := censusRuntimeFixture(t, censusRuntimeOptions{workspace: requested, ready: true, documentSymbolSupport: true, callHierarchySupport: true})
		admitted, failure := newCensusExecutor(runtime).execute(context.Background(), censusRequest("project", started.Generation))
		if failure != nil || admitted.workspace != filepath.Clean(requested) || !filepath.IsAbs(admitted.workspace) || strings.TrimSpace(admitted.workspace) == "" {
			t.Fatalf("ASSERT_CENSUS_CANONICAL_ABSOLUTE_WORKSPACE: requested=%q admitted=%+v failure=%+v", requested, admitted, failure)
		}
	})

	t.Run("cancelled", func(t *testing.T) {
		runtime, _, started := censusRuntimeFixture(t, censusRuntimeOptions{ready: true, documentSymbolSupport: true, callHierarchySupport: true})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, failure := newCensusExecutor(runtime).execute(ctx, censusRequest("project", started.Generation))
		assertCensusFailure(t, failure, censusStageAcquisition, censusCodeAcquisitionFailed)
	})
}

func TestCensusExecutorHasNoRuntimeSideEffects(t *testing.T) {
	runtime, starter, started := censusRuntimeFixture(t, censusRuntimeOptions{ready: true, documentSymbolSupport: true, callHierarchySupport: true})
	beforeCensus := runtime.Census()
	beforeStarts, beforeMethods, beforeTeardown, beforeClose := starter.snapshot()
	if _, failure := newCensusExecutor(runtime).execute(context.Background(), censusRequest("project", started.Generation)); failure != nil {
		t.Fatalf("ASSERT_CENSUS_SIDE_EFFECT_FIXTURE_ADMITTED: %+v", failure)
	}
	afterCensus := runtime.Census()
	afterStarts, afterMethods, afterTeardown, afterClose := starter.snapshot()
	if !reflect.DeepEqual(beforeCensus, afterCensus) || beforeStarts != afterStarts || !reflect.DeepEqual(beforeMethods, afterMethods) || beforeTeardown != afterTeardown || beforeClose != afterClose {
		t.Fatalf("ASSERT_CENSUS_NO_LSP_OR_PROCESS_LIFECYCLE: census=%+v/%+v starts=%d/%d methods=%v/%v teardown=%d/%d close=%d/%d", beforeCensus, afterCensus, beforeStarts, afterStarts, beforeMethods, afterMethods, beforeTeardown, afterTeardown, beforeClose, afterClose)
	}
}

func TestCensusExecutorFailuresAreTypedAndPrivacySafe(t *testing.T) {
	workspace := t.TempDir()
	runtime, _, _ := censusRuntimeFixture(t, censusRuntimeOptions{workspace: workspace, ready: true, documentSymbolSupport: true, callHierarchySupport: true})
	failures := []*censusAdmissionFailure{}
	for _, raw := range [][]byte{
		[]byte(`{"session_id":"missing","generation":1,"sources":["."]}`),
		[]byte(`{"session_id":"project","generation":0,"sources":["."]}`),
	} {
		_, failure := newCensusExecutor(runtime).execute(context.Background(), raw)
		failures = append(failures, failure)
	}
	assertCensusFailure(t, failures[0], censusStageAcquisition, censusCodeAcquisitionFailed)
	assertCensusFailure(t, failures[1], censusStageConfig, censusCodeInvalidConfig)
	typeOfFailure := reflect.TypeOf(*failures[0])
	if typeOfFailure.NumField() != 2 || typeOfFailure.Field(0).Name != "stage" || typeOfFailure.Field(0).Type.Name() != "censusFailureStage" || typeOfFailure.Field(1).Name != "code" || typeOfFailure.Field(1).Type.Name() != "censusFailureCode" {
		t.Fatalf("ASSERT_CENSUS_FAILURE_CLOSED_TYPED_FIELDS: %+v", typeOfFailure)
	}
	for _, failure := range failures {
		serialized, err := json.Marshal(failure)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(serialized), workspace) || strings.Contains(string(serialized), "missing") || len(serialized) != 2 {
			t.Fatalf("ASSERT_CENSUS_FAILURE_NO_PATH_OR_ERROR_TEXT: %s", serialized)
		}
	}
}

func TestCensusExecutorRegisteredAsOperation34Only(t *testing.T) {
	server, _, err := newServerRuntime(false)
	if err != nil {
		t.Fatal(err)
	}
	tool, found := server.Registry.ResolveCanonical(mcpcontract.CensusTool)
	_, executorFound := server.Executors[mcp.CensusExecutorFamily]
	if !found || tool.ExecutorFamily != mcp.CensusExecutorFamily || !executorFound || len(server.Registry.Tools()) != 38 {
		t.Fatalf("ASSERT_CENSUS_OPERATION34_REGISTERED: found=%t tool=%+v executor=%t tools=%d", found, tool, executorFound, len(server.Registry.Tools()))
	}
	if tool35, found := server.Registry.Resolve(mcpcontract.StructuralContextTool); !found || tool35.ExecutorFamily != mcp.StructuralContextExecutorFamily {
		t.Fatal("ASSERT_CENSUS_OPERATION35_APPEND_ONLY_PRESENT")
	}
}
