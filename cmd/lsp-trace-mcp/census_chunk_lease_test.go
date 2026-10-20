package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/census"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censuschunking"
	"lsp-trace/internal/censusrequest"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/seedformat"
	"lsp-trace/sessionruntime"
)

func chunkLeaseDiscoveryFixture(t *testing.T, workspace string, started sessionruntime.StartResult) (censusacquisition.Discovery, censusrequest.Receipt) {
	t.Helper()
	positionRange := func(p lsp.Position) lsp.Range { return lsp.Range{Start: p, End: p} }
	discovery := censusacquisition.Discovery{
		Session: censusacquisition.SessionIdentity{SessionID: started.SessionID, Generation: started.Generation}, Workspace: workspace, Complete: true,
		FileLedger:         captureset.Ledger{Denominator: 1, Entries: []captureset.LedgerEntry{{Ordinal: 0, Identity: "a.go", Disposition: "selected"}}},
		EnumerationSymbols: make([]censusacquisition.EnumerationSymbol, censuschunking.ChunkSize+1),
		Targets:            make([]censusacquisition.PreparedTarget, censuschunking.ChunkSize+1),
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.Join(workspace, "a.go")}).String()
	for i := range discovery.Targets {
		position := lsp.Position{Line: uint32(i), Character: 1}
		seed, err := seedformat.EncodeCanonical(seedformat.File{SchemaVersion: seedformat.Version, CoordinateConvention: seedformat.CoordinateConvention, Seeds: []seedformat.Seed{{Type: seedformat.PositionType, Position: &seedformat.Position{Label: fmt.Sprintf("census-%06d", i), Path: "a.go", Line: uint64(i) + 1, Column: 2}}}}, workspace)
		if err != nil {
			t.Fatal(err)
		}
		name := fmt.Sprintf("Target%05d", i)
		discovery.EnumerationSymbols[i] = censusacquisition.EnumerationSymbol{Ordinal: i, FileOrdinal: 0, SelectionStart: position, Kind: 12, Name: name, Disposition: census.SymbolSelected}
		discovery.Targets[i] = censusacquisition.PreparedTarget{CensusOrdinal: i, CanonicalSeedV2: seed, URI: uri, LanguageID: "go", SelectionRange: positionRange(position), Name: name, Kind: 12, Range: positionRange(position)}
	}
	receipt, err := censusrequest.New(censusrequest.RefreshCoordinate{SessionID: started.SessionID, Generation: started.Generation}, censusrequest.SemanticFields{Sources: []string{"."}, DownDepth: 1, UpDepth: 1, MaxNodes: 10000, BatchTargets: 63, TimeoutMS: 60000, RequestTimeoutMS: 30000})
	if err != nil {
		t.Fatal(err)
	}
	return discovery, receipt
}

func TestPrivateCensusChunkOuterLeaseSequence(t *testing.T) {
	workspace := filepath.Clean(t.TempDir())
	runtime, _, started := censusRuntimeFixture(t, censusRuntimeOptions{workspace: workspace, ready: true, documentSymbolSupport: true, callHierarchySupport: true})
	shell := newCensusRuntime(runtime)
	discovery, receipt := chunkLeaseDiscoveryFixture(t, workspace, started)
	terminal := realCoordinatorTerminalFixture(t)
	events := []string{"discovery"}
	leaseCalls, nestedAccepted := 0, 0
	shell.chunkLease = func(ctx context.Context, request sessionruntime.SessionLeaseRequest, run func(context.Context) error) error {
		leaseCalls++
		events = append(events, "lease-enter")
		err := runtime.Manager.WithReadySessionLease(ctx, request, run)
		events = append(events, "lease-exit")
		return err
	}
	shell.chunkCoordinate = func(ctx context.Context, request censuschunking.CoordinatorRequest) (censuschunking.CoordinatorResult, error) {
		events = append(events, "coordinator")
		nested := runtime.Manager.WithReadySessionLease(ctx, sessionruntime.SessionLeaseRequest{SessionID: started.SessionID, Generation: started.Generation, Workspace: workspace}, func(context.Context) error { nestedAccepted++; return nil })
		if nested == nil {
			return censuschunking.CoordinatorResult{}, errors.New("nested lease unexpectedly accepted")
		}
		err := request.Sessions.WithReadySession(ctx, censuschunking.ExpectedSession{SessionID: started.SessionID, Generation: started.Generation, WorkspaceURI: (&url.URL{Scheme: "file", Path: workspace}).String()}, func(censusacquisition.Core) error { events = append(events, "core"); return nil })
		return censuschunking.CoordinatorResult{TerminalSelector: "terminal-content-selector"}, err
	}
	shell.chunkReadTerminal = func(_ *publication.Root, selector string, _ int64) ([]byte, error) {
		events = append(events, "terminal-read")
		if selector != "terminal-content-selector" {
			return nil, errors.New("terminal selector changed")
		}
		return append([]byte(nil), terminal.terminalIndex...), nil
	}
	prepared := censusRuntimeResult{admitted: censusAdmittedSession{sessionID: started.SessionID, generation: started.Generation, workspace: workspace, requestReceipt: receipt}, options: censusRuntimeConfig{downDepth: 1, upDepth: 1, maxNodes: 10000, maxBatchTargets: 63, timeoutMS: 60000, requestTimeoutMS: 30000}, discovery: discovery}
	rootPath := t.TempDir()
	if err := os.Chmod(rootPath, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	completion, err := shell.runPrivateCensusChunking(context.Background(), prepared, root)
	if err != nil {
		t.Fatal(err)
	}
	events = append(events, "completion-projection")
	if _, err := projectPrivateCensusMCP("lease-sequence", censusCompletion{privateChunk: completion}); err != nil {
		t.Fatal(err)
	}
	want := []string{"discovery", "lease-enter", "coordinator", "core", "lease-exit", "terminal-read", "completion-projection"}
	if !reflect.DeepEqual(events, want) || leaseCalls != 1 || nestedAccepted != 0 {
		t.Fatalf("ASSERT_PRIVATE_CENSUS_SINGLE_OUTER_LEASE_SEQUENCE: events=%v lease=%d nested=%d", events, leaseCalls, nestedAccepted)
	}
}
