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

func TestPrivateAcquisitionDiagnosticPublicFailureBytesPinnedWithAndWithoutRecorder(t *testing.T) {
	const want = "{\"schema_version\":\"lsp-trace.census-diagnostic.v1\",\"status\":\"FAILED\",\"stage\":\"acquisition\",\"code\":\"ACQUISITION_FAILED\",\"batch_ordinal\":3,\"retry\":true}\n"
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "without-recorder", true: "with-recorder"}[enabled], func(t *testing.T) {
			var recorder *censusdiagnostic.Recorder
			if enabled {
				_, recorder = newPrivateAcquisitionRecorder(t)
			}
			session := &fakeBatchSession{id: "s", generation: 7, respond: func(context.Context, operation.Request) (operation.Result, *operation.Failure) {
				return operation.Result{}, &operation.Failure{Code: operation.FailureInternal, Err: errors.New("private failure")}
			}}
			_, failure := newCensusBatchAcquirer(session, acquisitionops.Limits{}, recorder).acquireBatch(context.Background(), batchRequest(1, 2))
			if failure == nil {
				t.Fatal("ASSERT_PRIVATE_ACQUISITION_DIAGNOSTIC_PUBLIC_BYTE_PIN missing failure")
			}
			ordinal := 3
			diagnostic, err := buildCensusCLIDiagnostic(censusStageAcquisition, &ordinal)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := marshalCensusCLIDiagnostic(diagnostic)
			if err != nil || string(raw) != want {
				t.Fatalf("ASSERT_PRIVATE_ACQUISITION_DIAGNOSTIC_PUBLIC_BYTE_PIN got=%q err=%v", raw, err)
			}
		})
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
