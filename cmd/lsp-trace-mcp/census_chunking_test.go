package main

import (
	"context"
	"errors"
	"net/url"
	"path/filepath"
	"testing"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censuschunking"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
)

func TestPrivateCensusChunkRouteExactCompatibilityBoundary(t *testing.T) {
	for _, tc := range []struct {
		name      string
		activated bool
		count     int
		want      bool
	}{
		{name: "disabled-10001", count: 10001},
		{name: "activated-10000", activated: true, count: 10000},
		{name: "activated-10001", activated: true, count: 10001, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			discovery := censusacquisition.Discovery{Complete: true, Targets: make([]censusacquisition.PreparedTarget, tc.count)}
			got, err := privateCensusChunkRoute(tc.activated, discovery)
			if err != nil || got != tc.want {
				t.Fatalf("ASSERT_PRIVATE_CENSUS_ROUTE_10000_10001: activated=%v count=%d got=%v want=%v err=%v", tc.activated, tc.count, got, tc.want, err)
			}
		})
	}
}

func TestPrivateCensusChunkFailuresCollapseToAcquisitionWithoutCustody(t *testing.T) {
	for _, source := range []string{"coordinator", "store", "terminal-read", "lease"} {
		t.Run(source, func(t *testing.T) {
			runtime, _, started := censusRuntimeFixture(t, censusRuntimeOptions{ready: true, documentSymbolSupport: true, callHierarchySupport: true})
			shell := newCensusRuntime(runtime)
			retained := []censusacquisition.SourceFile{{Path: "retained.go"}}
			enumerateCalls, discoveryCalls, chunkCalls := 0, 0, 0
			shell.enumerate = func(_ context.Context, admitted censusAdmittedSession, _ censusRuntimeConfig) ([]censusacquisition.SourceFile, error) {
				enumerateCalls++
				retained[0].URI = (&url.URL{Scheme: "file", Path: filepath.Join(admitted.workspace, retained[0].Path)}).String()
				return append([]censusacquisition.SourceFile(nil), retained...), nil
			}
			shell.discoverEnumerated = func(ctx context.Context, _ *sessionruntime.Manager, admitted censusAdmittedSession, _ censusRuntimeConfig, _ bool, files censusacquisition.FileEnumerator) (censusacquisition.Discovery, error) {
				discoveryCalls++
				got, err := files.Enumerate(ctx)
				if err != nil || len(got) != 1 || got[0].Path != retained[0].Path {
					t.Fatalf("ASSERT_PRIVATE_CENSUS_CHUNK_DISCOVERY_REUSES_ENUMERATION: got=%+v err=%v", got, err)
				}
				return censusacquisition.Discovery{Session: censusacquisition.SessionIdentity{SessionID: admitted.sessionID, Generation: admitted.generation}, Workspace: admitted.workspace, Complete: true, Targets: make([]censusacquisition.PreparedTarget, 10001)}, nil
			}
			shell.chunk = func(context.Context, censusRuntimeResult, *publication.Root) (*privateCensusChunkCompletion, error) {
				chunkCalls++
				return nil, errors.New(source + " sentinel")
			}
			request := censusRuntimeRequest(t, openCensusRuntimeRoot(t), runtimeRaw(t, "project", started.Generation, nil))
			request.PrivateCensusChunking = true
			completion := shell.run(context.Background(), operation.Request(request))
			if enumerateCalls != 1 {
				t.Fatalf("ASSERT_PRIVATE_CENSUS_CHUNK_ENUMERATES_ONCE: calls=%d", enumerateCalls)
			}
			if discoveryCalls != 1 || chunkCalls != 1 {
				t.Fatalf("ASSERT_PRIVATE_CENSUS_CHUNK_FAILURE_PATH_REACHED: source=%s discovery=%d chunk=%d", source, discoveryCalls, chunkCalls)
			}
			if completion.Diagnostic == nil || completion.privateChunk != nil || completion.Result != nil {
				t.Fatalf("ASSERT_PRIVATE_CENSUS_CHUNK_FAILURE_ACQUISITION_NO_CUSTODY: source=%s completion=%+v", source, completion)
			}
			projected, err := projectPrivateCensusMCP("chunk-failure", completion)
			if err != nil || !projected.IsError {
				t.Fatalf("ASSERT_PRIVATE_CENSUS_CHUNK_FAILURE_PUBLIC_BOUNDARY: source=%s projected=%+v err=%v", source, projected, err)
			}
		})
	}
}

func TestPrivateCensusRuntimeRunRoutesOnlyAboveCompatibility(t *testing.T) {
	for _, tc := range []struct {
		count, wantChunkCalls, wantAcquireCalls int
	}{{10000, 0, 1}, {10001, 1, 0}} {
		runtime, _, started := censusRuntimeFixture(t, censusRuntimeOptions{ready: true, documentSymbolSupport: true, callHierarchySupport: true})
		shell := newCensusRuntime(runtime)
		retained := []censusacquisition.SourceFile{{Path: "retained.go"}}
		enumerateCalls := 0
		shell.enumerate = func(_ context.Context, admitted censusAdmittedSession, _ censusRuntimeConfig) ([]censusacquisition.SourceFile, error) {
			enumerateCalls++
			retained[0].URI = (&url.URL{Scheme: "file", Path: filepath.Join(admitted.workspace, retained[0].Path)}).String()
			return append([]censusacquisition.SourceFile(nil), retained...), nil
		}
		shell.discoverEnumerated = func(ctx context.Context, _ *sessionruntime.Manager, admitted censusAdmittedSession, _ censusRuntimeConfig, _ bool, files censusacquisition.FileEnumerator) (censusacquisition.Discovery, error) {
			got, err := files.Enumerate(ctx)
			if err != nil || len(got) != 1 || got[0].Path != retained[0].Path {
				t.Fatalf("ASSERT_PRIVATE_CENSUS_ROUTE_REUSES_ENUMERATION: got=%+v err=%v", got, err)
			}
			return censusacquisition.Discovery{Session: censusacquisition.SessionIdentity{SessionID: admitted.sessionID, Generation: admitted.generation}, Workspace: admitted.workspace, Complete: true, Targets: make([]censusacquisition.PreparedTarget, tc.count)}, nil
		}
		chunkCalls, acquireCalls, leaseCalls, coordinateCalls, readCalls := 0, 0, 0, 0, 0
		shell.chunk = func(context.Context, censusRuntimeResult, *publication.Root) (*privateCensusChunkCompletion, error) {
			chunkCalls++
			return realCoordinatorTerminalFixture(t), nil
		}
		shell.acquireOverride = func(context.Context, censusRuntimeResult) (censusacquisition.Projection, *censusRuntimeFailure, censusFailureReason) {
			acquireCalls++
			return censusacquisition.Projection{}, censusAcquisitionFailure(), reasonAcquisitionFailed
		}
		shell.chunkLease = func(context.Context, sessionruntime.SessionLeaseRequest, func(context.Context) error) error {
			leaseCalls++
			return nil
		}
		shell.chunkCoordinate = func(context.Context, censuschunking.CoordinatorRequest) (censuschunking.CoordinatorResult, error) {
			coordinateCalls++
			return censuschunking.CoordinatorResult{}, nil
		}
		shell.chunkReadTerminal = func(*publication.Root, string, int64) ([]byte, error) { readCalls++; return nil, nil }
		request := censusRuntimeRequest(t, openCensusRuntimeRoot(t), runtimeRaw(t, "project", started.Generation, nil))
		request.PrivateCensusChunking = true
		_ = shell.run(context.Background(), operation.Request(request))
		if enumerateCalls != 1 {
			t.Fatalf("ASSERT_PRIVATE_CENSUS_ROUTE_ENUMERATES_ONCE: calls=%d", enumerateCalls)
		}
		if chunkCalls != tc.wantChunkCalls || acquireCalls != tc.wantAcquireCalls {
			t.Fatalf("ASSERT_PRIVATE_CENSUS_POST_DISCOVERY_ROUTE: count=%d chunk=%d/%d acquire=%d/%d", tc.count, chunkCalls, tc.wantChunkCalls, acquireCalls, tc.wantAcquireCalls)
		}
		if tc.count <= 10000 && (leaseCalls != 0 || coordinateCalls != 0 || readCalls != 0) {
			t.Fatalf("ASSERT_PRIVATE_CENSUS_COMPATIBILITY_ZERO_CHUNK_EFFECTS: lease=%d coordinate=%d read=%d", leaseCalls, coordinateCalls, readCalls)
		}
	}
}
