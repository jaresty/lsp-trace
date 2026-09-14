package main

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
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
	initialized chan struct{}

	mu            sync.Mutex
	methods       []string
	teardownCalls int
	closeCalls    int
}

func newCensusTestProcess(metadata string) *censusTestProcess {
	in, stdin := io.Pipe()
	stdout, out := io.Pipe()
	p := &censusTestProcess{in: in, stdin: stdin, out: out, stdout: stdout, metadata: metadata, initialized: make(chan struct{})}
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

	mu      sync.Mutex
	starts  int
	process *censusTestProcess
}

func (s *censusTestStarter) Start(context.Context, managedprocess.Spec) (sessionruntime.Child, managedprocess.StartObservation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.starts++
	s.process = newCensusTestProcess(s.metadata)
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
	starter := &censusTestStarter{metadata: string(metadata)}
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
	path := filepath.Join("census_executor.go")
	parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	constructorOK := false
	for _, declaration := range parsed.Decls {
		switch declaration := declaration.(type) {
		case *ast.FuncDecl:
			if ast.IsExported(declaration.Name.Name) && strings.Contains(strings.ToLower(declaration.Name.Name), "census") {
				t.Fatalf("ASSERT_CENSUS_NO_EXPORTED_CONSTRUCTOR_OR_AUTHORITY: %s", declaration.Name.Name)
			}
			if declaration.Name.Name == "newCensusExecutor" && declaration.Type.Params != nil && len(declaration.Type.Params.List) == 1 {
				star, ok := declaration.Type.Params.List[0].Type.(*ast.StarExpr)
				if ok {
					identifier, named := star.X.(*ast.Ident)
					constructorOK = named && identifier.Name == "hostSelectorRuntime"
				}
			}
		case *ast.GenDecl:
			for _, specification := range declaration.Specs {
				typeSpec, ok := specification.(*ast.TypeSpec)
				if ok && ast.IsExported(typeSpec.Name.Name) && strings.Contains(strings.ToLower(typeSpec.Name.Name), "census") {
					t.Fatalf("ASSERT_CENSUS_NO_EXPORTED_RUNTIME_EXECUTOR_AUTHORITY_INTERFACE: %s", typeSpec.Name.Name)
				}
			}
		}
	}
	if !constructorOK {
		t.Fatal("ASSERT_CENSUS_PRIVATE_CONCRETE_HOST_SELECTOR_CONSTRUCTOR")
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

func TestCensusExecutorRemainsUnregistered(t *testing.T) {
	server, manager, err := newServerRuntime(false)
	if err != nil {
		t.Fatal(err)
	}
	beforeExecutors := len(server.Executors)
	_ = newCensusExecutor(newHostSelectorRuntime(manager, nil))
	if len(server.Registry.Tools()) != 33 || len(server.Executors) != beforeExecutors {
		t.Fatalf("ASSERT_CENSUS_REGISTRY_EXACT33_UNCHANGED: tools=%d executors=%d/%d", len(server.Registry.Tools()), beforeExecutors, len(server.Executors))
	}
	for _, name := range []string{mcpcontract.FutureCensusTool, "lsp_trace_v1_structural_context"} {
		if _, found := server.Registry.Resolve(name); found {
			t.Fatalf("ASSERT_CENSUS_OPERATION34_35_ABSENT: %s", name)
		}
	}
}
