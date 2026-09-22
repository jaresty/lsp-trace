package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/continuationhost"
	"lsp-trace/internal/describerequest"
	"lsp-trace/internal/describeworker"
	"lsp-trace/internal/programccompose"
	"lsp-trace/sessionruntime"
)

func TestCensusCatalogRunnerBranchesBeforeAcquisition(t *testing.T) {
	var core, hosts, resumes, runnerBuilds int
	deps := productionCensusRunnerDependencies()
	deps.openCatalogHost = func(string, string) (*continuationhost.Bundle, error) {
		hosts++
		return &continuationhost.Bundle{}, nil
	}
	deps.resolveCatalogDescriptor = func(context.Context, *continuationhost.Bundle, string) (continuationhost.DescriptorInput, error) {
		return continuationhost.DescriptorInput{CheckpointSelector: "sha256:" + strings.Repeat("d", 64) + ":1"}, nil
	}
	deps.newCatalogWorker = func(string, string) censuscontinuation.WorkerV2 {
		return &lazyCensusCatalogWorker{build: func() (*describeworker.Runner, error) { runnerBuilds++; return &describeworker.Runner{}, nil }}
	}
	deps.resumeCatalog = func(context.Context, censuscontinuation.ResumeRequest) censuscontinuation.Result {
		resumes++
		return censuscontinuation.Result{Status: censuscontinuation.StatusComplete, CensusID: "g-" + strings.Repeat("a", 64) + ".selector.json", CheckpointID: "sha256:" + strings.Repeat("b", 64) + ":1", CatalogSelector: "sha256:" + strings.Repeat("e", 64) + ":1"}
	}
	deps.runCore = func(censusCLIOptions, censusCoreConfig, censusCoreDependencies) censusPublicationOutcome {
		core++
		return censusPublicationOutcome{}
	}
	deps.publishCatalogDescriptor = func(context.Context, *continuationhost.Bundle, continuationhost.DescriptorInput) (string, error) {
		return "g-" + strings.Repeat("c", 64) + ".selector.json", nil
	}
	var stdout, stderr bytes.Buffer
	code := runCensusWithDependencies([]string{"--machine", "--resume", "g-" + strings.Repeat("d", 64) + ".selector.json", "--publication-root", "/private", "--catalog-config", "/host.json"}, &stdout, &stderr, deps)
	if code != 0 || core != 0 || hosts != 1 || resumes != 1 || runnerBuilds != 0 || stderr.Len() != 0 {
		t.Fatalf("ASSERT_CENSUS_RESUME_CORE_ZERO code=%d core=%d hosts=%d resumes=%d builds=%d stdout=%q stderr=%q", code, core, hosts, resumes, runnerBuilds, stdout.String(), stderr.String())
	}
}

func TestCensusCatalogFreshLoadsAcquisitionProfileConfig(t *testing.T) {
	workspace, root := t.TempDir(), t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	profileConfig := filepath.Join(t.TempDir(), "profiles.toml")
	if err := os.WriteFile(profileConfig, []byte("[profiles.test]\ncommand = \"profile-server\"\nlanguage_ids = [\"go\"]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	deps := productionCensusRunnerDependencies()
	deps.lookPath = func(command string) (string, error) {
		if command != "profile-server" {
			t.Fatalf("ASSERT_CENSUS_CATALOG_PROFILE_CONFIG_COMMAND command=%q", command)
		}
		return "/bin/echo", nil
	}
	deps.newStarter = func() (sessionruntime.Starter, error) { return sessionruntime.ManagedStarter{}, nil }
	coreCalls := 0
	deps.runCore = func(_ censusCLIOptions, cfg censusCoreConfig, _ censusCoreDependencies) censusPublicationOutcome {
		coreCalls++
		if cfg.runner.start.LanguageID != "go" {
			t.Fatalf("ASSERT_CENSUS_CATALOG_PROFILE_CONFIG_LANGUAGE language=%q", cfg.runner.start.LanguageID)
		}
		return censusPublicationOutcome{}
	}
	var stdout, stderr bytes.Buffer
	code := runCensusWithDependencies([]string{"--machine", "--workspace", workspace, "--publication-root", root, "--catalog", "--catalog-config", "/host.json", "--profile", "test", "--config", profileConfig}, &stdout, &stderr, deps)
	if code == 0 || coreCalls != 1 || stdout.Len() != 0 {
		t.Fatalf("ASSERT_CENSUS_CATALOG_PROFILE_CONFIG_LOADED code=%d core=%d stdout=%q stderr=%q", code, coreCalls, stdout.String(), stderr.String())
	}
}

func TestCensusCatalogFreshRunsPipelineAndPublishesOnce(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	projection := censusPublicationProjection(t, 65)
	outcome := publishCensusCaptureSetWithContinuation(context.Background(), censusPublicationCapabilityFor(t, projection), root, programccompose.ExactMetadata{RevisionCustody: "CALLER_ASSERTED", PositionEncoding: "utf-16", AcquisitionSemantics: "managed-lsp-v1", PrivacyPolicy: "private-census-v1"})
	var contracts, runs, descriptors int
	var openedConfig, contractConfig, workerConfig string
	deps := productionCensusRunnerDependencies()
	deps.openCatalogHost = func(configPath, _ string) (*continuationhost.Bundle, error) {
		openedConfig = configPath
		return &continuationhost.Bundle{}, nil
	}
	deps.buildCatalogContract = func(configPath string) (censuscontinuation.ContinuationContract, error) {
		contracts++
		contractConfig = configPath
		return censuscontinuation.ContinuationContract{}, nil
	}
	deps.newCatalogWorker = func(configPath, _ string) censuscontinuation.WorkerV2 {
		workerConfig = configPath
		return &lazyCensusCatalogWorker{}
	}
	deps.runCatalog = func(_ context.Context, req censuscontinuation.Request) censuscontinuation.Result {
		runs++
		if req.Handoff.Validate() != nil || req.Workspace != projection.Workspace {
			t.Fatal("ASSERT_CENSUS_FRESH_HANDOFF_WORKSPACE")
		}
		if req.FreshCapture == nil || req.FreshCapture.Preparer == nil || req.FreshCapture.Resolver == nil {
			t.Fatal("ASSERT_CENSUS_FRESH_MANAGED_RUNTIME_INJECTED")
		}
		return censuscontinuation.Result{Status: censuscontinuation.StatusComplete, CensusID: outcome.Result.CensusID, CheckpointID: "sha256:" + strings.Repeat("a", 64) + ":1", CatalogSelector: "sha256:" + strings.Repeat("e", 64) + ":1"}
	}
	deps.publishCatalogDescriptor = func(context.Context, *continuationhost.Bundle, continuationhost.DescriptorInput) (string, error) {
		descriptors++
		return "g-" + strings.Repeat("c", 64) + ".selector.json", nil
	}
	var stdout, stderr bytes.Buffer
	code := runCensusCatalogFresh(censusCLIOptions{Machine: true, Workspace: projection.Workspace, ConfigPath: "/profiles.toml", CatalogConfigPath: "/host.json", PublicationRoot: root}, &stdout, &stderr, outcome, &initializedAcquisitionRuntime{}, deps)
	if code != 0 || contracts != 1 || runs != 1 || descriptors != 1 || openedConfig != "/host.json" || contractConfig != "/host.json" || workerConfig != "/host.json" || stderr.Len() != 0 || strings.Count(stdout.String(), "\n") != 1 {
		t.Fatalf("ASSERT_CENSUS_FRESH_PIPELINE_ONCE code=%d contracts=%d runs=%d descriptors=%d stdout=%q stderr=%q", code, contracts, runs, descriptors, stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), projection.Workspace) || strings.Contains(stdout.String(), "private/path") || !strings.Contains(stdout.String(), `"authority":0`) || !strings.Contains(stdout.String(), `"accepted":false`) || !strings.Contains(stdout.String(), `"completeness":"UNKNOWN"`) {
		t.Fatalf("ASSERT_CENSUS_DESCRIPTOR_SAFE_AUTHORITY %q", stdout.String())
	}
}

func TestCensusCatalogStopAfterDescribeRequestsDoesNotConstructWorker(t *testing.T) {
	projection := censusPublicationProjection(t, 65)
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	outcome := publishCensusCaptureSetWithContinuation(context.Background(), censusPublicationCapabilityFor(t, projection), root, programccompose.ExactMetadata{RevisionCustody: "CALLER_ASSERTED", PositionEncoding: "utf-16", AcquisitionSemantics: "managed-lsp-v1", PrivacyPolicy: "private-census-v1"})
	deps := productionCensusRunnerDependencies()
	deps.openCatalogHost = func(string, string) (*continuationhost.Bundle, error) { return &continuationhost.Bundle{}, nil }
	deps.buildCatalogContract = func(string) (censuscontinuation.ContinuationContract, error) {
		return censuscontinuation.ContinuationContract{}, nil
	}
	deps.newCatalogWorker = func(string, string) censuscontinuation.WorkerV2 { panic("ASSERT_STOP_AFTER_CONSTRUCTED_WORKER") }
	deps.runCatalog = func(_ context.Context, req censuscontinuation.Request) censuscontinuation.Result {
		if req.Worker != nil || req.WorkerV2 != nil || req.StopAfter != censuscontinuation.StopAfterDescribeRequests {
			t.Fatalf("ASSERT_STOP_AFTER_NO_WORKER_REQUEST: worker=%T worker_v2=%T stop=%q", req.Worker, req.WorkerV2, req.StopAfter)
		}
		return censuscontinuation.Result{Status: censuscontinuation.StatusPaused, CensusID: outcome.Result.CensusID, CheckpointID: "sha256:" + strings.Repeat("a", 64) + ":1", RequestCount: 3, PreparationCount: 2}
	}
	deps.publishCatalogDescriptor = func(context.Context, *continuationhost.Bundle, continuationhost.DescriptorInput) (string, error) {
		return "g-" + strings.Repeat("c", 64) + ".selector.json", nil
	}
	var stdout, stderr bytes.Buffer
	code := runCensusCatalogFresh(censusCLIOptions{Machine: true, Workspace: projection.Workspace, CatalogConfigPath: "/host.json", PublicationRoot: root, StopAfter: "describe-requests"}, &stdout, &stderr, outcome, &initializedAcquisitionRuntime{}, deps)
	if code != 0 || stderr.Len() != 0 || !strings.Contains(stdout.String(), `"status":"PAUSED"`) || !strings.Contains(stdout.String(), `"request_count":3`) || !strings.Contains(stdout.String(), `"preparation_count":2`) || !strings.Contains(stdout.String(), `"resume_guidance":"Resume with this selector and omit stop_after to continue exactly the remaining work once."`) {
		t.Fatalf("ASSERT_STOP_AFTER_PUBLIC_PAUSED_RESULT: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestCensusCatalogHandoffPreflightAccounting(t *testing.T) {
	projection := censusPublicationProjection(t, 65)
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	outcome := publishCensusCaptureSetWithContinuation(context.Background(), censusPublicationCapabilityFor(t, projection), root, programccompose.ExactMetadata{RevisionCustody: "CALLER_ASSERTED", PositionEncoding: "utf-16", AcquisitionSemantics: "managed-lsp-v1", PrivacyPolicy: "private-census-v1"})
	handoff, err := outcome.BuildCommittedHandoff()
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := handoff.Bytes()
	var stderr bytes.Buffer
	writePrivateHandoffPreflight(&stderr, handoff, 16<<20)
	want := fmt.Sprintf("PRIVATE_HANDOFF_PREFLIGHT canonical_bytes=%d max_object_bytes=%d\n", len(raw), 16<<20)
	if stderr.String() != want {
		t.Fatalf("ASSERT_CENSUS_HANDOFF_PREFLIGHT_ACCOUNTING got=%q want=%q", stderr.String(), want)
	}
}

func TestCensusCatalogProductionHandoffHostContractPersistsInitialCheckpoint(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	projection := censusPublicationProjection(t, 65)
	outcome := publishCensusCaptureSetWithContinuation(context.Background(), censusPublicationCapabilityFor(t, projection), root, programccompose.ExactMetadata{RevisionCustody: "CALLER_ASSERTED", PositionEncoding: "utf-16", AcquisitionSemantics: "managed-lsp-v1", PrivacyPolicy: "private-census-v1"})
	handoff, err := outcome.BuildCommittedHandoff()
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "host.json")
	raw, err := json.Marshal(map[string]any{"version": 1, "continuation": map[string]any{"publication_root": root, "max_object_bytes": 8 << 20, "worker": map[string]any{}}})
	if err != nil || os.WriteFile(configPath, raw, 0o600) != nil {
		t.Fatalf("ASSERT_CENSUS_PRODUCTION_HOST_CONFIG: %v", err)
	}
	host, err := openCensusCatalogHost(configPath, root)
	if err != nil {
		t.Fatal(err)
	}
	contract, err := censusCatalogContract(configPath)
	if err != nil {
		t.Fatal(err)
	}
	result := censuscontinuation.Run(context.Background(), censuscontinuation.Request{Handoff: handoff, Workspace: projection.Workspace, Contract: contract, WorkerV2: newLazyCensusCatalogWorker(configPath, root), Store: host.Store, FreshCapture: censusFreshCaptureTestDependencies(context.Background(), projection.Workspace)})
	if result.CheckpointID == "" {
		t.Fatalf("ASSERT_CENSUS_PRODUCTION_INITIAL_CHECKPOINT: status=%q stage=%q code=%q error=%t", result.Status, result.PrivateStage, result.PrivateCode, result.Err != nil)
	}
	if result.Status == censuscontinuation.StatusFailedCapture {
		t.Fatalf("ASSERT_CENSUS_PRODUCTION_PROGRAM_C_DEFAULT: status=%q checkpoint=%t error=%v", result.Status, result.CheckpointID != "", result.Err)
	}
}

func TestLazyCensusCatalogWorkerConcurrentConstructionFailureIsOnceAndSafe(t *testing.T) {
	var builds atomic.Int32
	worker := &lazyCensusCatalogWorker{build: func() (*describeworker.Runner, error) { builds.Add(1); return nil, errors.New("/private/model secret") }}
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := worker.Run(context.Background(), describerequest.Record{}, "attempt")
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	if builds.Load() != 1 {
		t.Fatalf("ASSERT_LAZY_WORKER_CONSTRUCTS_ONCE builds=%d", builds.Load())
	}
	for err := range errs {
		if err == nil || strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), "model") {
			t.Fatalf("ASSERT_LAZY_WORKER_FAILURE_SOURCE_SAFE %v", err)
		}
	}
}

func TestCensusCatalogCommittedDegradationIsSuccess(t *testing.T) {
	out := censusPublicationOutcome{}
	var stdout, stderr bytes.Buffer
	code := writeCensusCatalogOutcome(&stdout, &stderr, true, out, censuscontinuation.Result{Status: censuscontinuation.StatusFailedCatalog, CensusID: "census", CheckpointID: "sha256:" + strings.Repeat("a", 64) + ":1", Err: errors.New("private")}, "g-"+strings.Repeat("b", 64)+".selector.json")
	if code != 0 || stderr.Len() != 0 || !strings.Contains(stdout.String(), `"status":"DEGRADED"`) || strings.Contains(stdout.String(), "private") {
		t.Fatalf("ASSERT_CENSUS_COMMITTED_DEGRADED_SUCCESS code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}
