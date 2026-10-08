package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/censusresult"
	"lsp-trace/internal/continuationhost"
	"lsp-trace/internal/describeworker"
	"lsp-trace/internal/liveprojection"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/retainedprojection"
	"lsp-trace/internal/sourceprojection"
	"lsp-trace/internal/v5sourcesnapshotv3"
	"lsp-trace/internal/v5sourcesnapshotv6"
	"lsp-trace/sessionruntime"
)

type continuationRuntimeProbe struct {
	prepareCalls   int
	roundTripCalls int
	workspace      string
	languageID     string
	sessionID      string
	generation     uint64
	requests       []sessionruntime.DocumentRequest
}

func (p *continuationRuntimeProbe) PrepareDocument(_ context.Context, request sessionruntime.DocumentRequest) sessionruntime.DocumentResult {
	p.prepareCalls++
	p.requests = append(p.requests, request)
	content := []byte("package fixture\nfunc Target() {}\n")
	return sessionruntime.DocumentResult{URI: request.URI, LanguageID: "go", Version: 1, Supply: &sessionruntime.DocumentSupply{SessionID: request.SessionID, Generation: request.Generation, URI: request.URI, DocumentVersion: 1, Content: content}}
}
func (p *continuationRuntimeProbe) WorkspaceRoot(sessionID string, generation uint64) (string, bool) {
	return p.workspace, p.workspace != "" && (p.sessionID == "" || p.sessionID == sessionID) && (p.generation == 0 || p.generation == generation)
}
func (p *continuationRuntimeProbe) SessionLanguageID(sessionID string, generation uint64) (string, bool) {
	return p.languageID, p.languageID != "" && (p.sessionID == "" || p.sessionID == sessionID) && (p.generation == 0 || p.generation == generation)
}
func (p *continuationRuntimeProbe) RoundTrip(context.Context, sessionruntime.RoundTripRequest) sessionruntime.RoundTripResult {
	p.roundTripCalls++
	return sessionruntime.RoundTripResult{}
}

func TestBootstrapContinuationConfigurationIsOptional(t *testing.T) {
	var config bootstrapConfig
	if err := json.Unmarshal([]byte(`{"version":1,"processes":[{"profile":{"trust_domain":"local","workspace":"/tmp/work","profile":"p","environment_reference":"e"},"execution":{"path":"/bin/true","directory":"/tmp"}}]}`), &config); err != nil {
		t.Fatal(err)
	}
	if config.Continuation != nil {
		t.Fatal("ASSERT_ORDINARY_BOOTSTRAP_HAS_NO_WORKER_REQUIREMENT")
	}
}

func TestLoadBootstrapContinuationPinsAreHostOwned(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "continuations")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "bootstrap.json")
	raw := `{"version":1,"continuation":{"publication_root":"` + root + `","max_object_bytes":1048576,"worker":{"worker":{"path":"/opt/worker","sha256":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},"model":{"path":"/opt/model","sha256":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},"library":{"path":"/opt/lib","sha256":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"},"sandbox_executable":{"path":"/usr/bin/sandbox-exec","sha256":"sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"},"sandbox_profile":{"path":"/opt/profile.sb","sha256":"sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"},"grammar":{"path":"/opt/grammar","sha256":"sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"},"runtime_identity":"runtime","adapter_identity":"adapter","model_identity":"model","limits":{"timeout_ms":1000,"max_tokens":64,"context_tokens":1024,"stdout_bytes":4096,"stderr_bytes":4096,"work_bytes":1048576,"temp_bytes":1048576}}},"processes":[{"profile":{"trust_domain":"local","workspace":"/tmp/work","profile":"p","environment_reference":"e"},"execution":{"path":"/bin/true","directory":"/tmp"}}]}`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := loadBootstrapConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if config.Continuation == nil || config.Continuation.Worker.Model.Path != "/opt/model" || config.Continuation.Worker.Limits.TimeoutMS != 1000 {
		t.Fatalf("ASSERT_HOST_PINS_DECODED %#v", config.Continuation)
	}
	worker := config.Continuation.Worker.config()
	if worker.Model.Path != "/opt/model" || worker.Model.SHA256 != "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" || worker.Limits.TimeoutMS != 1000 || worker.ResponseVersion != describeworker.ResponseVersionV2 {
		t.Fatalf("ASSERT_EXACT_TYPED_WORKER_CONFIG_V2 %#v", worker)
	}
}

func TestLoadBootstrapStopOnlyContinuationRequiresNoWorkerPins(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "continuations")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "bootstrap.json")
	raw := `{"version":1,"continuation":{"publication_root":"` + root + `","max_object_bytes":1048576,"capabilities":["STOP_AFTER_DESCRIBE_REQUESTS"]},"processes":[{"profile":{"trust_domain":"local","workspace":"/tmp/work","profile":"p","environment_reference":"e"},"execution":{"path":"/bin/true","directory":"/tmp"}}]}`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := loadBootstrapConfig(path)
	if err != nil {
		t.Fatalf("ASSERT_STOP_ONLY_BOOTSTRAP_NO_WORKER_MODEL_PIN_PREFLIGHT: %v", err)
	}
	if config.Continuation == nil {
		t.Fatal("ASSERT_STOP_ONLY_BOOTSTRAP_MINIMUM_CAPABILITY: continuation missing")
	}
}

func TestLoadBootstrapFullResumeStillRequiresExactPins(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "continuations")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "bootstrap.json")
	raw := `{"version":1,"continuation":{"publication_root":"` + root + `","max_object_bytes":1048576,"capabilities":["STOP_AFTER_DESCRIBE_REQUESTS","RESUME"]},"processes":[{"profile":{"trust_domain":"local","workspace":"/tmp/work","profile":"p","environment_reference":"e"},"execution":{"path":"/bin/true","directory":"/tmp"}}]}`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadBootstrapConfig(path); err == nil || !strings.Contains(err.Error(), "resume requires exact worker pins") {
		t.Fatalf("ASSERT_FULL_RESUME_REQUIRES_EXACT_WORKER_PINS: %v", err)
	}
}

func TestLoadBootstrapContinuationRequiresOwnerOnlyConfigAndStorage(t *testing.T) {
	for _, tc := range []struct {
		name       string
		configMode os.FileMode
		rootMode   os.FileMode
	}{
		{name: "config", configMode: 0o640, rootMode: 0o700},
		{name: "storage", configMode: 0o600, rootMode: 0o750},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			root := filepath.Join(dir, "continuations")
			if err := os.Mkdir(root, tc.rootMode); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "bootstrap.json")
			raw := `{"version":1,"continuation":{"publication_root":"` + root + `","max_object_bytes":1048576,"capabilities":["STOP_AFTER_DESCRIBE_REQUESTS"]},"processes":[{"profile":{"trust_domain":"local","workspace":"/tmp/work","profile":"p","environment_reference":"e"},"execution":{"path":"/bin/true","directory":"/tmp"}}]}`
			if err := os.WriteFile(path, []byte(raw), tc.configMode); err != nil {
				t.Fatal(err)
			}
			if _, err := loadBootstrapConfig(path); err == nil || !strings.Contains(err.Error(), "owner-only") {
				t.Fatalf("ASSERT_CONTINUATION_BOOTSTRAP_OWNER_ONLY_%s: %v", strings.ToUpper(tc.name), err)
			}
		})
	}
}

func TestProductionContinuationHostConstructionAttribution(t *testing.T) {
	private := errors.New("/private/root worker_pin raw_config descriptor")
	cases := []struct {
		name   string
		stage  continuationConstructionStage
		reason continuationConstructionReason
		mutate func(*productionCensusContinuationFactory)
	}{
		{name: "valid"},
		{name: "root_open", stage: constructionStageRootOpen, reason: constructionReasonRootOpenFailed, mutate: func(f *productionCensusContinuationFactory) {
			f.openRoot = func(string) (*publication.Root, error) { return nil, private }
		}},
		{name: "store_private_validation", stage: constructionStageStorePrivateValidation, reason: constructionReasonStorePrivateValidationFailed, mutate: func(f *productionCensusContinuationFactory) {
			f.newStore = func(*publication.Root, int64) (*continuationhost.Store, error) { return nil, private }
		}},
		{name: "contract_construction", stage: constructionStageContractConstruction, reason: constructionReasonContractConstructionFailed, mutate: func(f *productionCensusContinuationFactory) {
			f.newContract = func(censuscontinuation.ContractInput) (censuscontinuation.ContinuationContract, error) {
				return censuscontinuation.ContinuationContract{}, private
			}
		}},
		{name: "runtime_arity", stage: constructionStageRuntimeArity, reason: constructionReasonRuntimeArityInvalid, mutate: func(f *productionCensusContinuationFactory) { f.runtimeCount = 2 }},
		{name: "worker_assembly", stage: constructionStageWorkerAssembly, reason: constructionReasonWorkerAssemblyFailed, mutate: func(f *productionCensusContinuationFactory) {
			f.newWorker = func(func() (*describeworker.Runner, error)) (censuscontinuation.Worker, censuscontinuation.WorkerV2, error) {
				return nil, nil, private
			}
		}},
		{name: "host_assembly", stage: constructionStageHostAssembly, reason: constructionReasonHostAssemblyFailed, mutate: func(f *productionCensusContinuationFactory) {
			f.newHost = func(*publication.Root, *continuationhost.Store, censuscontinuation.Worker, censuscontinuation.WorkerV2, censuscontinuation.ContinuationContract, *censuscontinuation.FreshCaptureDependencies) (*productionCensusContinuationHost, error) {
				return nil, private
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Chmod(root, 0o700); err != nil {
				t.Fatal(err)
			}
			factory := defaultProductionCensusContinuationFactory()
			if tc.mutate != nil {
				tc.mutate(&factory)
			}
			host, err := newProductionCensusContinuationHostWithFactory(bootstrapContinuationConfig{PublicationRoot: root, MaxObjectBytes: 1 << 20}, factory)
			if tc.stage == "" {
				if err != nil {
					t.Fatal(err)
				}
				defer host.Close()
				return
			}
			var constructionErr *continuationConstructionError
			if !errors.As(err, &constructionErr) || constructionErr.Stage != tc.stage || constructionErr.Reason != tc.reason || (tc.stage != constructionStageRuntimeArity && !errors.Is(err, private)) {
				t.Fatalf("ASSERT_CONSTRUCTION_ATTRIBUTION got=%#v err=%v", constructionErr, err)
			}
			diagnostic, buildErr := censusresult.NewContinuationDiagnostic(censusresult.ContinuationHostConstructionFailed, censusresult.ContinuationDiagnosticContext{ConstructionStage: string(constructionErr.Stage), ConstructionReason: string(constructionErr.Reason)})
			if buildErr != nil {
				t.Fatal(buildErr)
			}
			raw, _ := censusresult.MarshalContinuationDiagnostic(diagnostic)
			if bytes.Contains(raw, []byte("/private/")) || bytes.Contains(raw, []byte("worker_pin")) || bytes.Contains(raw, []byte("raw_config")) || bytes.Contains(raw, []byte("descriptor")) {
				t.Fatalf("ASSERT_CONSTRUCTION_DIAGNOSTIC_PRIVATE_SAFE %s", raw)
			}
		})
	}
}

func TestCurrentPrivateBootstrapConstructsContinuationHost(t *testing.T) {
	path := os.Getenv("LSP_TRACE_PRIVATE_BOOTSTRAP_TEST_CONFIG")
	if path == "" {
		t.Skip("set LSP_TRACE_PRIVATE_BOOTSTRAP_TEST_CONFIG to run the developer-private bootstrap smoke test")
	}
	config, err := loadBootstrapConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if config.Continuation == nil {
		t.Fatal("ASSERT_CURRENT_BOOTSTRAP_CONTINUATION_PRESENT")
	}
	host, err := newProductionCensusContinuationHost(*config.Continuation)
	if err != nil {
		var constructionErr *continuationConstructionError
		if errors.As(err, &constructionErr) {
			t.Fatalf("ASSERT_CURRENT_BOOTSTRAP_CONSTRUCTION stage=%s reason=%s", constructionErr.Stage, constructionErr.Reason)
		}
		t.Fatal(err)
	}
	defer host.Close()
}

func TestProductionContinuationManagedPreparationDiagnosticConfiguration(t *testing.T) {
	root := t.TempDir()
	parent := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(parent, "managed-preparation.ndjson")
	probe := &continuationRuntimeProbe{}
	host, err := newProductionCensusContinuationHost(bootstrapContinuationConfig{PublicationRoot: root, MaxObjectBytes: 1 << 20, ManagedPreparationDiagnosticPath: path}, probe)
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()
	managed, ok := host.freshCapture.Preparer.(censuscontinuation.ManagedPreparer)
	if !ok || managed.Recorder == nil {
		t.Fatalf("ASSERT_MANAGED_PREPARATION_DIAGNOSTIC_INJECTED: %#v", host.freshCapture.Preparer)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("ASSERT_MANAGED_PREPARATION_DIAGNOSTIC_READY: info=%v err=%v", info, err)
	}
}

func TestProductionContinuationManagedPreparationDiagnosticAbsentAndUnavailable(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	probe := &continuationRuntimeProbe{}
	host, err := newProductionCensusContinuationHost(bootstrapContinuationConfig{PublicationRoot: root, MaxObjectBytes: 1 << 20}, probe)
	if err != nil {
		t.Fatal(err)
	}
	managed := host.freshCapture.Preparer.(censuscontinuation.ManagedPreparer)
	if managed.Recorder != nil {
		t.Fatal("ASSERT_MANAGED_PREPARATION_DIAGNOSTIC_ABSENT")
	}
	_ = host.Close()
	_, err = newProductionCensusContinuationHost(bootstrapContinuationConfig{PublicationRoot: root, MaxObjectBytes: 1 << 20, ManagedPreparationDiagnosticPath: filepath.Join(t.TempDir(), "missing", "diagnostic.ndjson")}, probe)
	var constructionErr *continuationConstructionError
	if !errors.As(err, &constructionErr) || constructionErr.Stage != constructionStagePreparationDiagnostic || constructionErr.Reason != constructionReasonPreparationDiagnosticFailed {
		t.Fatalf("ASSERT_MANAGED_PREPARATION_DIAGNOSTIC_FAIL_CLOSED: %#v %v", constructionErr, err)
	}
}

func TestProductionContinuationUsesCoherentBoundedCaptureDefaults(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	host, err := newProductionCensusContinuationHost(bootstrapContinuationConfig{PublicationRoot: root, MaxObjectBytes: 64 << 20})
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()
	limits := host.contract.CaptureLimits()
	if limits.MaxArtifactBytes != 64<<20 || limits.MaxParentBytes != 64<<20 {
		t.Fatalf("ASSERT_PRODUCTION_CAPTURE_TIER_64M: artifact=%d parent=%d", limits.MaxArtifactBytes, limits.MaxParentBytes)
	}
	if limits.MaxSourceBytes != 1<<20 || limits.MaxTotalSourceBytes != 8<<20 {
		t.Fatalf("ASSERT_PRODUCTION_CAPTURE_TIER_DOES_NOT_INFLATE_SOURCE: source=%d total=%d", limits.MaxSourceBytes, limits.MaxTotalSourceBytes)
	}
	historical, err := censuscontinuation.NewContinuationContract(censuscontinuation.ContractInput{ContinuationID: continuationDigest("lsp-trace-mcp-census-catalog-v1"), ProfileID: continuationDigest("lsp-trace-mcp-census-catalog-profile-v1"), WorkerPinID: continuationDigest(describeworker.Config{ResponseVersion: describeworker.ResponseVersionV2}), ProgramCSeed: 9, ResponseVersion: describeworker.ResponseVersionV2, CaptureLimits: v5sourcesnapshotv3.Limits{MaxArtifactBytes: 8 << 20, MaxParentBytes: 4 << 20, MaxReceipts: 100, MaxSourceBytes: 1 << 20, MaxTotalSourceBytes: 8 << 20, MaxBindings: 100, MaxWork: 10000}, PacketSourcePolicy: sourceprojection.Policy{PolicyID: "private-census-catalog-v1", BodyRequested: true, MaxBytes: 1 << 20, MaxRanges: 100, MaxObjects: 100, MaxWork: 1000, EnforceLimits: true}, PacketResolveLimits: retainedprojection.ResolveLimits{MaxDistinctObjects: 100, MaxUniqueSourceBytes: 8 << 20, MaxLogicalSelections: 100}, PacketMaxResponseBytes: 1 << 20, DescribeDeadlineMS: 90_000, RetryPolicy: censuscontinuation.RetryPolicy{MaxAttemptsPerRequest: 2}})
	if err != nil || historical.ID() == host.contract.ID() {
		t.Fatalf("ASSERT_CAPTURE_LIMITS_FINGERPRINT_CONTRACT: err=%v historical=%s current=%s", err, historical.ID(), host.contract.ID())
	}
}

func TestProductionContinuationUsesQualifiedBoundedDescribeDeadline(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	host, err := newProductionCensusContinuationHost(bootstrapContinuationConfig{
		PublicationRoot: root,
		MaxObjectBytes:  1 << 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()
	const qualifiedDeadlineMS = 90_000
	if got := host.contract.DescribeDeadlineMS(); got != qualifiedDeadlineMS {
		t.Fatalf("ASSERT_HOST_DESCRIBE_DEADLINE_QUALIFIED_BOUNDED: got=%d want=%d", got, qualifiedDeadlineMS)
	}
}

func TestProductionContinuationSeparatesPreparationAndResolverMessageLimits(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	host, err := newProductionCensusContinuationHost(bootstrapContinuationConfig{PublicationRoot: root, MaxObjectBytes: 1 << 20}, &continuationRuntimeProbe{})
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()
	if got := host.freshCapture.Limits; got.MaxDocuments != 100 || got.MaxMessages != 100 || got.MaxWork != 100 {
		t.Fatalf("ASSERT_MCP_MANAGED_PREPARATION_100_URI_CAPACITY: %+v", got)
	}
	resolver, ok := host.freshCapture.Resolver.(liveprojection.FullDefinitionResolver)
	if !ok || resolver.Limits.MaxMessages != 32 {
		t.Fatalf("ASSERT_MCP_RESOLVER_SEPARATE_32_MESSAGE_BUDGET: resolver=%T limits=%+v", host.freshCapture.Resolver, resolver.Limits)
	}
}

func TestProductionContinuationManagedPreparationUsesExactCommittedWorkspace(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	committedWorkspace := t.TempDir()
	hostDefaultWorkspace := t.TempDir()
	probe := &continuationRuntimeProbe{workspace: hostDefaultWorkspace, languageID: "go", sessionID: "committed-session", generation: 17}
	host, err := newProductionCensusContinuationHost(bootstrapContinuationConfig{PublicationRoot: root, MaxObjectBytes: 1 << 20}, probe)
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()
	uri := (&url.URL{Scheme: "file", Path: filepath.Join(committedWorkspace, "target.go")}).String()
	_, err = host.freshCapture.Preparer.PrepareManagedDocuments(context.Background(), committedWorkspace, "committed-session", 17, "utf-16", []string{uri}, host.freshCapture.Limits)
	if err == nil {
		t.Fatal("ASSERT_PRODUCTION_MANAGED_PREPARATION_REJECTS_HOST_DEFAULT_WORKSPACE")
	}
	if probe.prepareCalls != 0 {
		t.Fatalf("ASSERT_PRODUCTION_MANAGED_PREPARATION_MISMATCH_ZERO_RUNTIME_CALLS calls=%d", probe.prepareCalls)
	}

	probe.workspace = committedWorkspace
	documents, err := host.freshCapture.Preparer.PrepareManagedDocuments(context.Background(), committedWorkspace, "committed-session", 17, "utf-16", []string{uri}, host.freshCapture.Limits)
	if err != nil || len(documents) != 1 || len(probe.requests) != 1 {
		t.Fatalf("ASSERT_MANAGED_PREPARATION_EXACT_COMMITTED_ROUTE documents=%d requests=%d err=%v", len(documents), len(probe.requests), err)
	}
	request := probe.requests[0]
	if request.SessionID != "committed-session" || request.Generation != 17 || request.URI != uri || request.LanguageID != "go" || !request.CaptureSupply {
		t.Fatalf("ASSERT_MANAGED_PREPARATION_EXACT_COMMITTED_ROUTE request=%+v", request)
	}
	baselineCalls := probe.prepareCalls
	for _, tc := range []struct {
		name       string
		sessionID  string
		generation uint64
		languageID string
	}{
		{name: "unknown-session", sessionID: "other-session", generation: 17, languageID: "go"},
		{name: "stale-generation", sessionID: "committed-session", generation: 18, languageID: "go"},
		{name: "empty-language", sessionID: "committed-session", generation: 17},
	} {
		probe.languageID = tc.languageID
		got, gotErr := host.freshCapture.Preparer.PrepareManagedDocuments(context.Background(), committedWorkspace, tc.sessionID, tc.generation, "utf-16", []string{uri}, host.freshCapture.Limits)
		var managed *censuscontinuation.ManagedPreparationError
		if len(got) != 0 || !errors.As(gotErr, &managed) || managed.Kind != censuscontinuation.ManagedPreparationAvailability || probe.prepareCalls != baselineCalls {
			t.Fatalf("ASSERT_MANAGED_PREPARATION_LANGUAGE_ROUTE_FAILS_CLOSED_%s documents=%d calls=%d err=%v", tc.name, len(got), probe.prepareCalls, gotErr)
		}
	}
	_, _ = host.freshCapture.Resolver.ResolveFullDefinition(context.Background(), v5sourcesnapshotv6.ResolveRequest{SessionID: request.SessionID, Generation: request.Generation, URI: uri, Bytes: documents[0].Bytes})
	if probe.roundTripCalls == 0 {
		t.Fatal("ASSERT_MANAGED_PREPARATION_REACHES_RESOLVER")
	}
}

func TestProductionContinuationInjectsManagedRuntimeWithoutCallingIt(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	probe := &continuationRuntimeProbe{}
	host, err := newProductionCensusContinuationHost(bootstrapContinuationConfig{PublicationRoot: root, MaxObjectBytes: 1 << 20}, probe)
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()
	if host.freshCapture == nil || host.freshCapture.Preparer == nil || host.freshCapture.Resolver == nil {
		t.Fatal("ASSERT_MCP_FRESH_MANAGED_RUNTIME_INJECTED")
	}
	if probe.prepareCalls != 0 || probe.roundTripCalls != 0 {
		t.Fatalf("ASSERT_MCP_COMPOSITION_ZERO_EAGER_CALLS prepare=%d resolve=%d", probe.prepareCalls, probe.roundTripCalls)
	}
}

func TestBootstrapContinuationRejectsRelativePaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bootstrap.json")
	raw := `{"version":1,"continuation":{"publication_root":"relative","max_object_bytes":1},"processes":[{"profile":{"trust_domain":"local","workspace":"/tmp/work","profile":"p","environment_reference":"e"},"execution":{"path":"/bin/true","directory":"/tmp"}}]}`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadBootstrapConfig(path); err == nil {
		t.Fatal("ASSERT_CONTINUATION_CANONICAL_ABSOLUTE_PATHS")
	}
}
