package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"lsp-trace/acquisitionops"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censusdiagnostic"
	"lsp-trace/internal/operation"
	"lsp-trace/sessionruntime"
)

func newPrivateAcquisitionRecorder(t *testing.T) (string, *censusdiagnostic.Recorder) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "diagnostic.ndjson")
	recorder, err := censusdiagnostic.NewRecorder(path, censusdiagnostic.DefaultMaxBytes, censusdiagnostic.DefaultMaxRecords)
	if err != nil {
		t.Fatal(err)
	}
	return path, recorder
}

func TestPrivateAcquisitionDiagnosticFlagFreshOnlyHidden(t *testing.T) {
	path := filepath.Join(t.TempDir(), "diagnostic.ndjson")
	base := []string{"census", "--workspace", "work", "--source", ".", "--publication-root", "/private", "--server", "gopls"}
	options, err := parseCensusCLIOptions(append(base, "--private-acquisition-diagnostic-path", path))
	field := reflect.ValueOf(options).FieldByName("PrivateAcquisitionDiagnosticPath")
	if err != nil || !field.IsValid() || field.Kind() != reflect.String || field.String() != path {
		t.Fatalf("ASSERT_PRIVATE_ACQUISITION_DIAGNOSTIC_FLAG_FRESH_ONLY_HIDDEN options=%+v err=%v", options, err)
	}
	resume := []string{"census", "--resume", "sha256:" + strings.Repeat("a", 64), "--catalog-config", "host.json", "--publication-root", "/private", "--private-acquisition-diagnostic-path", path}
	if _, err := parseCensusCLIOptions(resume); err == nil {
		t.Fatal("ASSERT_PRIVATE_ACQUISITION_DIAGNOSTIC_FLAG_FRESH_ONLY_HIDDEN resume accepted private diagnostic")
	}
	if strings.Contains(censusUsage, "private-acquisition-diagnostic") || strings.Contains(usageText, "private-acquisition-diagnostic") {
		t.Fatal("ASSERT_PRIVATE_ACQUISITION_DIAGNOSTIC_FLAG_FRESH_ONLY_HIDDEN leaked into usage")
	}
}

func TestCensusDiagnosticOperationCodeClosedMapping(t *testing.T) {
	allowed := []string{
		operation.FailureInvalidInput,
		censusBatchFailureInvalidInput,
		censusBatchFailureSessionDrift,
		"CANCELLED",
		"CANCELED",
		"REQUEST_CANCELLED",
		censusBatchFailureCancelled,
		operation.FailureInternal,
		censusBatchFailureAcquisition,
		censusBatchFailureAdmission,
	}
	for _, code := range allowed {
		if got := censusDiagnosticOperationCode(code); got != code {
			t.Fatalf("ASSERT_PRIVATE_ACQUISITION_DIAGNOSTIC_ALLOWED_CODE code=%q got=%q", code, got)
		}
	}
	for _, code := range []string{"", "UNKNOWN", "private://workspace/request-id/server-stderr/selector/artifact"} {
		if got := censusDiagnosticOperationCode(code); got != operation.FailureInternal {
			t.Fatalf("ASSERT_PRIVATE_ACQUISITION_DIAGNOSTIC_UNKNOWN_CODE code=%q got=%q", code, got)
		}
	}
}

func TestPrivateAcquisitionDiagnosticUnknownOperationCodeIsGeneric(t *testing.T) {
	const hostile = "private://workspace/request-id/server-stderr/selector/artifact"
	path, recorder := newPrivateAcquisitionRecorder(t)
	cause := errors.New("original acquisition cause")
	operationFailure := &operation.Failure{Code: hostile, Err: cause}
	session := &fakeBatchSession{id: "s", generation: 7, respond: func(context.Context, operation.Request) (operation.Result, *operation.Failure) {
		return operation.Result{}, operationFailure
	}}
	_, failure := newCensusBatchAcquirer(session, acquisitionops.Limits{}, recorder).acquireBatch(context.Background(), batchRequest(1, 2))
	if failure == nil || failure.Code != censusBatchFailureAcquisition || !errors.Is(failure, cause) {
		t.Fatalf("ASSERT_PRIVATE_ACQUISITION_DIAGNOSTIC_ORIGINAL_FAILURE failure=%v", failure)
	}
	if operationFailure.Code != hostile || operationFailure.Err != cause {
		t.Fatalf("ASSERT_PRIVATE_ACQUISITION_DIAGNOSTIC_OPERATION_FAILURE_MUTATED failure=%+v", operationFailure)
	}
	record, err := censusdiagnostic.ReadLast(path)
	if err != nil || record.OperationCode != operation.FailureInternal || record.OperationCategory != "ACQUISITION" {
		t.Fatalf("ASSERT_PRIVATE_ACQUISITION_DIAGNOSTIC_GENERIC_CODE record=%+v err=%v", record, err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), hostile) || !strings.Contains(string(raw), `"operation_code":"INTERNAL"`) {
		t.Fatalf("ASSERT_PRIVATE_ACQUISITION_DIAGNOSTIC_HOSTILE_CODE_CLOSED raw=%s", raw)
	}
}

func TestPrivateAcquisitionDiagnosticCategoryFieldParityAndIdentity(t *testing.T) {
	cases := []struct {
		name, category string
		prepare        func(*fakeBatchSession, *censusacquisition.BatchRequest) context.Context
		complete       bool
	}{
		{name: "invalid-input", category: "INVALID_INPUT", complete: true, prepare: func(_ *fakeBatchSession, request *censusacquisition.BatchRequest) context.Context {
			request.Targets = nil
			return context.Background()
		}},
		{name: "session-drift", category: "SESSION_DRIFT", complete: true, prepare: func(session *fakeBatchSession, _ *censusacquisition.BatchRequest) context.Context {
			session.generation++
			return context.Background()
		}},
		{name: "cancelled", category: "CANCELLED", complete: true, prepare: func(_ *fakeBatchSession, _ *censusacquisition.BatchRequest) context.Context {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			return ctx
		}},
		{name: "acquisition", category: "ACQUISITION", complete: true, prepare: func(session *fakeBatchSession, _ *censusacquisition.BatchRequest) context.Context {
			session.respond = func(context.Context, operation.Request) (operation.Result, *operation.Failure) {
				return operation.Result{}, &operation.Failure{Code: operation.FailureInternal, Err: errors.New("private://workspace/request-id/server-stderr/selector/artifact")}
			}
			return context.Background()
		}},
		{name: "admission", category: "ADMISSION", complete: false, prepare: func(_ *fakeBatchSession, _ *censusacquisition.BatchRequest) context.Context {
			return context.Background()
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path, recorder := newPrivateAcquisitionRecorder(t)
			session := &fakeBatchSession{id: "s", generation: 7}
			session.respond = func(_ context.Context, request operation.Request) (operation.Result, *operation.Failure) {
				return successfulBatchResponse(t, request, tc.complete, false), nil
			}
			request := batchRequest(1, 4)
			ctx := tc.prepare(session, &request)
			_, failure := newCensusBatchAcquirer(session, acquisitionops.Limits{}, recorder).acquireBatch(ctx, request)
			if failure == nil {
				t.Fatal("ASSERT_PRIVATE_ACQUISITION_DIAGNOSTIC_CATEGORY_FIELD_PARITY missing failure")
			}
			record, err := censusdiagnostic.ReadLast(path)
			if err != nil || record.Category != tc.category || record.Ordinal == nil || *record.Ordinal != request.Ordinal || record.Fingerprint != rawDigest(request.CanonicalSeedsV2) {
				t.Fatalf("ASSERT_PRIVATE_ACQUISITION_DIAGNOSTIC_ORDINAL_FINGERPRINT record=%+v err=%v", record, err)
			}
			if (tc.category == "ACQUISITION") != (record.OperationCode == operation.FailureInternal && record.OperationCategory == "ACQUISITION") {
				t.Fatalf("ASSERT_PRIVATE_ACQUISITION_DIAGNOSTIC_CATEGORY_FIELD_PARITY record=%+v", record)
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, forbidden := range []string{"private://", "workspace", "request-id", "server-stderr", "selector", "artifact"} {
				if strings.Contains(string(raw), forbidden) {
					t.Fatalf("ASSERT_PRIVATE_ACQUISITION_DIAGNOSTIC_PRIVACY_FORBIDDEN_VALUE value=%q raw=%s", forbidden, raw)
				}
			}
		})
	}
}

func TestPrivateAcquisitionDiagnosticAppendFailurePreservesCauseAndNoPostReturnAppend(t *testing.T) {
	path, recorder := newPrivateAcquisitionRecorder(t)
	if err := os.Chmod(path, 0400); err != nil {
		t.Fatal(err)
	}
	session := &fakeBatchSession{id: "s", generation: 7, respond: func(context.Context, operation.Request) (operation.Result, *operation.Failure) {
		return operation.Result{}, &operation.Failure{Code: operation.FailureInternal, Err: errors.New("original acquisition cause")}
	}}
	_, failure := newCensusBatchAcquirer(session, acquisitionops.Limits{}, recorder).acquireBatch(context.Background(), batchRequest(1, 2))
	if failure == nil || failure.Code != censusBatchFailureAcquisition || !strings.Contains(failure.Error(), "original acquisition cause") {
		t.Fatalf("ASSERT_PRIVATE_ACQUISITION_DIAGNOSTIC_APPEND_DEGRADATION failure=%v", failure)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	size := info.Size()
	time.Sleep(10 * time.Millisecond)
	info, err = os.Stat(path)
	if err != nil || info.Size() != size {
		t.Fatalf("ASSERT_PRIVATE_ACQUISITION_DIAGNOSTIC_NO_POST_RETURN_APPEND before=%d after=%d err=%v", size, info.Size(), err)
	}

	cancelPath, cancelRecorder := newPrivateAcquisitionRecorder(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, cancelledFailure := newCensusBatchAcquirer(session, acquisitionops.Limits{}, cancelRecorder).acquireBatch(ctx, batchRequest(1, 3))
	if cancelledFailure == nil || cancelledFailure.Code != censusBatchFailureCancelled {
		t.Fatalf("ASSERT_PRIVATE_ACQUISITION_DIAGNOSTIC_CANCEL_NO_POST_RETURN_APPEND failure=%v", cancelledFailure)
	}
	cancelInfo, err := os.Stat(cancelPath)
	if err != nil {
		t.Fatal(err)
	}
	cancelSize := cancelInfo.Size()
	time.Sleep(10 * time.Millisecond)
	cancelInfo, err = os.Stat(cancelPath)
	if err != nil || cancelInfo.Size() != cancelSize {
		t.Fatalf("ASSERT_PRIVATE_ACQUISITION_DIAGNOSTIC_CANCEL_NO_POST_RETURN_APPEND before=%d after=%d err=%v", cancelSize, cancelInfo.Size(), err)
	}
}

func runPrivateDiagnosticCoreFailure(t *testing.T, recorder *censusdiagnostic.Recorder, operationFailure *operation.Failure) (censusPublicationOutcome, string, string, error, int, int) {
	t.Helper()
	projection := censusPublicationProjection(t, 1)
	var acquired error
	capabilities, publications := 0, 0
	deps := censusCoreDependencies{
		runSession: func(_ initializedAcquisitionRunnerConfig, callback func(context.Context, *initializedAcquisitionRuntime) int) int {
			return callback(context.Background(), &initializedAcquisitionRuntime{sessionID: "publication-session", generation: 7})
		},
		discover: func(context.Context, censusCLIOptions, *initializedAcquisitionRuntime, bool) (censusacquisition.Discovery, error) {
			return discoveryFromProjection(projection), nil
		},
		acquireDiagnostic: func(_ context.Context, _ *initializedAcquisitionRuntime, _ censusacquisition.Discoverer, _ acquisitionops.Limits, _, _, _ int, got *censusdiagnostic.Recorder, _ *censusControlFlowObserver) (censusAssembly, error) {
			if got != recorder {
				t.Fatal("ASSERT_PRIVATE_ACQUISITION_DIAGNOSTIC_CORE_RECORDER_FORWARDING")
			}
			session := &fakeBatchSession{id: "s", generation: 7, respond: func(context.Context, operation.Request) (operation.Result, *operation.Failure) {
				return operation.Result{}, operationFailure
			}}
			request := batchRequest(1, 3)
			_, failure := newCensusBatchAcquirer(session, acquisitionops.Limits{}, got).acquireBatch(context.Background(), request)
			acquired = failure
			return censusAssembly{}, &censusBatchAcquisitionFailure{ordinal: request.Ordinal, Err: failure}
		},
		capability: func(censusAssembly) (censusPublicationCapability, error) {
			capabilities++
			return censusPublicationCapability{}, errors.New("must not construct capability")
		},
		publish: func(context.Context, censusPublicationCapability, string) censusPublicationOutcome {
			publications++
			return censusPublicationOutcome{}
		},
	}
	outcome := runCensusCore(censusCLIOptions{}, censusCoreConfig{diagnostic: recorder}, deps)
	var stdout, stderr strings.Builder
	if code := writeCensusOutcome(&stdout, &stderr, true, outcome); code != 1 {
		t.Fatalf("ASSERT_PRIVATE_ACQUISITION_DIAGNOSTIC_PUBLIC_EXIT code=%d", code)
	}
	return outcome, stdout.String(), stderr.String(), acquired, capabilities, publications
}

func TestPrivateAcquisitionDiagnosticPublicFailureCoreParity(t *testing.T) {
	const want = "{\"schema_version\":\"lsp-trace.census-diagnostic.v1\",\"status\":\"FAILED\",\"stage\":\"acquisition\",\"code\":\"ACQUISITION_FAILED\",\"batch_ordinal\":3,\"retry\":true}\n"
	cause := errors.New("private ledger value")
	var baseline string
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "without-recorder", true: "with-recorder"}[enabled], func(t *testing.T) {
			var recorder *censusdiagnostic.Recorder
			if enabled {
				_, recorder = newPrivateAcquisitionRecorder(t)
			}
			outcome, stdout, stderr, acquired, capabilities, publications := runPrivateDiagnosticCoreFailure(t, recorder, &operation.Failure{Code: operation.FailureInternal, Err: cause})
			if outcome.Diagnostic == nil || outcome.Diagnostic.Retry != true || !errors.Is(acquired, cause) || capabilities != 0 || publications != 0 {
				t.Fatalf("ASSERT_PRIVATE_ACQUISITION_DIAGNOSTIC_PUBLIC_CLASSIFICATION outcome=%+v acquired=%v capability=%d publication=%d", outcome, acquired, capabilities, publications)
			}
			if stdout != "" || stderr != want || strings.Contains(stderr, cause.Error()) {
				t.Fatalf("ASSERT_PRIVATE_ACQUISITION_DIAGNOSTIC_PUBLIC_BYTES stdout=%q stderr=%q", stdout, stderr)
			}
			if !enabled {
				baseline = stderr
			} else if stderr != baseline {
				t.Fatalf("ASSERT_PRIVATE_ACQUISITION_DIAGNOSTIC_PUBLIC_PARITY enabled=%q disabled=%q", stderr, baseline)
			}
		})
	}
}

func TestPrivateAcquisitionDiagnosticAppendFailurePublicPathPreservesOriginal(t *testing.T) {
	path, recorder := newPrivateAcquisitionRecorder(t)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	cause := errors.New("original acquisition public cause")
	outcome, stdout, stderr, acquired, capabilities, publications := runPrivateDiagnosticCoreFailure(t, recorder, &operation.Failure{Code: operation.FailureInternal, Err: cause})
	const want = "{\"schema_version\":\"lsp-trace.census-diagnostic.v1\",\"status\":\"FAILED\",\"stage\":\"acquisition\",\"code\":\"ACQUISITION_FAILED\",\"batch_ordinal\":3,\"retry\":true}\n"
	if outcome.Diagnostic == nil || !errors.Is(acquired, cause) || stdout != "" || stderr != want || capabilities != 0 || publications != 0 {
		t.Fatalf("ASSERT_PRIVATE_ACQUISITION_DIAGNOSTIC_APPEND_PUBLIC_PATH outcome=%+v acquired=%v stdout=%q stderr=%q capability=%d publication=%d", outcome, acquired, stdout, stderr, capabilities, publications)
	}
}

func TestPrivateAcquisitionDiagnosticLateBatchFailureCoreLeavesNoCapabilityResultOrPublication(t *testing.T) {
	projection := censusPublicationProjection(t, 64)
	capabilities, publications, afterCommits := 0, 0, 0
	deps := censusCoreDependencies{
		runSession: func(_ initializedAcquisitionRunnerConfig, callback func(context.Context, *initializedAcquisitionRuntime) int) int {
			return callback(context.Background(), &initializedAcquisitionRuntime{sessionID: "publication-session", generation: 7})
		},
		discover: func(context.Context, censusCLIOptions, *initializedAcquisitionRuntime, bool) (censusacquisition.Discovery, error) {
			return discoveryFromProjection(projection), nil
		},
		acquireDiagnostic: func(_ context.Context, _ *initializedAcquisitionRuntime, _ censusacquisition.Discoverer, _ acquisitionops.Limits, _, _, _ int, recorder *censusdiagnostic.Recorder, _ *censusControlFlowObserver) (censusAssembly, error) {
			calls := 0
			session := &fakeBatchSession{id: "s", generation: 7, respond: func(_ context.Context, request operation.Request) (operation.Result, *operation.Failure) {
				calls++
				if calls == 2 {
					return operation.Result{}, &operation.Failure{Code: operation.FailureInternal, Err: errors.New("late private batch failure")}
				}
				return successfulBatchResponse(t, request, true, false), nil
			}}
			acquirer := newCensusBatchAcquirer(session, acquisitionops.Limits{}, recorder)
			if first, failure := acquirer.acquireBatch(context.Background(), batchRequest(63, 0)); failure != nil || len(first.Raw) == 0 {
				t.Fatalf("ASSERT_PRIVATE_ACQUISITION_DIAGNOSTIC_FIRST_BATCH first=%+v failure=%v", first, failure)
			}
			request := batchRequest(1, 1)
			late, failure := acquirer.acquireBatch(context.Background(), request)
			if failure == nil || len(late.Raw) != 0 {
				t.Fatalf("ASSERT_PRIVATE_ACQUISITION_DIAGNOSTIC_LATE_BATCH late=%+v failure=%v", late, failure)
			}
			return censusAssembly{}, &censusBatchAcquisitionFailure{ordinal: request.Ordinal, Err: failure}
		},
		capability: func(censusAssembly) (censusPublicationCapability, error) {
			capabilities++
			return censusPublicationCapability{}, nil
		},
		publish: func(context.Context, censusPublicationCapability, string) censusPublicationOutcome {
			publications++
			return censusPublicationOutcome{}
		},
	}
	cfg := censusCoreConfig{afterCommit: func(context.Context, *initializedAcquisitionRuntime, censusPublicationOutcome) int {
		afterCommits++
		return 0
	}}
	outcome := runCensusCore(censusCLIOptions{}, cfg, deps)
	var stdout, stderr strings.Builder
	code := writeCensusOutcome(&stdout, &stderr, true, outcome)
	if code != 1 || outcome.Result != nil || outcome.DiscoveryDiagnostic != nil || outcome.RequestReceipt != nil || outcome.Diagnostic == nil || capabilities != 0 || publications != 0 || afterCommits != 0 || stdout.Len() != 0 || strings.Contains(stderr.String(), "late private batch failure") {
		t.Fatalf("ASSERT_PRIVATE_ACQUISITION_DIAGNOSTIC_LATE_BATCH_CORE outcome=%+v code=%d capability=%d publication=%d after=%d stdout=%q stderr=%q", outcome, code, capabilities, publications, afterCommits, stdout.String(), stderr.String())
	}
}

func TestPrivateAcquisitionDiagnosticSetupFailsBeforeManagerWithoutLeak(t *testing.T) {
	publicationRoot := t.TempDir()
	if err := os.Chmod(publicationRoot, 0700); err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	badParent := t.TempDir()
	if err := os.Chmod(badParent, 0755); err != nil {
		t.Fatal(err)
	}
	privatePath := filepath.Join(badParent, "secret-diagnostic.ndjson")
	deps := productionCensusRunnerDependencies()
	deps.lookPath = func(string) (string, error) { return "/bin/echo", nil }
	deps.abs = func(path string) (string, error) { return path, nil }
	starterCalls, managerCalls, coreCalls := 0, 0, 0
	deps.newStarter = func() (sessionruntime.Starter, error) { starterCalls++; return sessionruntime.ManagedStarter{}, nil }
	deps.newManager = func(sessionruntime.Config) (*sessionruntime.Manager, error) {
		managerCalls++
		return nil, errors.New("manager must not start")
	}
	deps.runCore = func(censusCLIOptions, censusCoreConfig, censusCoreDependencies) censusPublicationOutcome {
		coreCalls++
		return censusPublicationOutcome{}
	}
	args := []string{"--machine", "--workspace", workspace, "--source", ".", "--publication-root", publicationRoot, "--server", "gopls", "--private-acquisition-diagnostic-path", privatePath}
	var stdout, stderr strings.Builder
	code := runCensusWithDependencies(args, &stdout, &stderr, deps)
	const want = "{\"schema_version\":\"lsp-trace.census-diagnostic.v1\",\"status\":\"FAILED\",\"stage\":\"config\",\"code\":\"INVALID_CONFIG\",\"retry\":true}\n"
	if code != 1 || stdout.Len() != 0 || stderr.String() != want || starterCalls != 0 || managerCalls != 0 || coreCalls != 0 || strings.Contains(stderr.String(), privatePath) || strings.Contains(stderr.String(), "acquisition diagnostic") {
		t.Fatalf("ASSERT_PRIVATE_ACQUISITION_DIAGNOSTIC_PREFLIGHT_GENERIC code=%d starter=%d manager=%d core=%d stdout=%q stderr=%q", code, starterCalls, managerCalls, coreCalls, stdout.String(), stderr.String())
	}
}
