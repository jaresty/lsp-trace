package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/census"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censuschunking"
	"lsp-trace/internal/censusrequest"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/seedformat"
)

func privateChunkDiscovery(t *testing.T, count int) censusacquisition.Discovery {
	t.Helper()
	workspace := t.TempDir()
	d := censusacquisition.Discovery{Session: censusacquisition.SessionIdentity{SessionID: "session", Generation: 7}, Workspace: workspace, Complete: true}
	d.FileLedger = captureset.Ledger{Denominator: 1, Entries: []captureset.LedgerEntry{{Ordinal: 0, Identity: "a.go", Disposition: "selected"}}}
	d.EnumerationSymbols = make([]censusacquisition.EnumerationSymbol, count)
	d.Targets = make([]censusacquisition.PreparedTarget, count)
	for i := 0; i < count; i++ {
		position := lsp.Position{Line: uint32(i), Character: 1}
		name := fmt.Sprintf("symbol-%06d", i)
		seed, err := seedformat.EncodeCanonical(seedformat.File{SchemaVersion: seedformat.Version, CoordinateConvention: seedformat.CoordinateConvention, Seeds: []seedformat.Seed{{Type: seedformat.PositionType, Position: &seedformat.Position{Label: fmt.Sprintf("census-%06d", i), Path: "a.go", Line: uint64(i) + 1, Column: 2}}}}, workspace)
		if err != nil {
			t.Fatal(err)
		}
		d.EnumerationSymbols[i] = censusacquisition.EnumerationSymbol{Ordinal: i, FileOrdinal: 0, SelectionStart: position, Kind: 12, Name: name, Disposition: census.SymbolSelected}
		d.Targets[i] = censusacquisition.PreparedTarget{CensusOrdinal: i, CanonicalSeedV2: seed, URI: "file:///workspace/a.go", LanguageID: "go", SelectionRange: lsp.Range{Start: position, End: position}, Name: name, Kind: 12, Range: lsp.Range{Start: position, End: position}}
	}
	return d
}

func privateChunkOptions() censusCLIOptions {
	return censusCLIOptions{PrivateEnumerationProjection: true, Machine: true, Sources: []string{"."}, DownDepth: 1, UpDepth: 1, MaxNodes: 10000, MaxBatchTargets: 63, Timeout: time.Minute, RequestTimeout: 30 * time.Second}
}

func TestPrivateCensusChunkingNoOptInDoesNothing(t *testing.T) {
	calls := 0
	handled, terminal, err := runPrivateCensusChunking(context.Background(), censusCLIOptions{}, censusCoreConfig{}, nil, censusacquisition.Discovery{}, newCensusControlFlowObserver(), privateCensusChunkDependencies{coordinate: func(context.Context, censuschunking.CoordinatorRequest) (censuschunking.CoordinatorResult, error) {
		calls++
		return censuschunking.CoordinatorResult{}, nil
	}})
	if err != nil || handled || terminal != nil || calls != 0 {
		t.Fatalf("ASSERT_PRIVATE_CHUNK_NO_OPT_IN handled=%v terminal=%+v calls=%d err=%v", handled, terminal, calls, err)
	}
}

func TestPrivateCensusChunkingCompatibilityDoesNotLeaseCoordinateOrStore(t *testing.T) {
	for _, count := range []int{1, censuschunking.ChunkSize} {
		t.Run(fmt.Sprintf("targets-%d", count), func(t *testing.T) {
			discovery := privateChunkDiscovery(t, count)
			discovery.EnumerationSymbols = nil // Complete legacy discovery may not retain global enumeration fields.
			calls := 0
			handled, terminal, err := runPrivateCensusChunking(context.Background(), privateChunkOptions(), censusCoreConfig{}, nil, discovery, newCensusControlFlowObserver(), privateCensusChunkDependencies{coordinate: func(context.Context, censuschunking.CoordinatorRequest) (censuschunking.CoordinatorResult, error) {
				calls++
				return censuschunking.CoordinatorResult{}, nil
			}})
			if err != nil || handled || terminal != nil || calls != 0 {
				t.Fatalf("ASSERT_PRIVATE_CHUNK_COMPATIBILITY_EXACT count=%d handled=%v terminal=%+v calls=%d err=%v", count, handled, terminal, calls, err)
			}
		})
	}
}

func TestPrivateCensusChunkingAboveCompatibilityRequiresGlobalEnumeration(t *testing.T) {
	discovery := privateChunkDiscovery(t, censuschunking.ChunkSize+1)
	discovery.EnumerationSymbols = nil
	calls := 0
	handled, terminal, err := runPrivateCensusChunking(context.Background(), privateChunkOptions(), censusCoreConfig{}, nil, discovery, newCensusControlFlowObserver(), privateCensusChunkDependencies{coordinate: func(context.Context, censuschunking.CoordinatorRequest) (censuschunking.CoordinatorResult, error) {
		calls++
		return censuschunking.CoordinatorResult{}, nil
	}})
	if err == nil || !handled || terminal != nil || calls != 0 || err.Error() != "private census chunk target enumeration mismatch" {
		t.Fatalf("ASSERT_PRIVATE_CHUNK_GLOBAL_ENUMERATION_REQUIRED handled=%v terminal=%+v calls=%d err=%v", handled, terminal, calls, err)
	}
}

func TestPrivateCensusChunkingUsesOneRealManagerLease(t *testing.T) {
	cfg := newInitializedRunnerTestConfig(t, 10*time.Second)
	observed := runnerObservations{}
	instrumentRunner(&cfg, &observed)
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	coordinateErr := errors.New("coordinate observed real manager lease")
	var handled, sawLease bool
	var runErr error
	code := runInitializedAcquisitionSession(cfg, func(ctx context.Context, session initializedAcquisitionSession) int {
		runtime, ok := session.(*initializedAcquisitionRuntime)
		if !ok {
			t.Fatal("initialized runtime type changed")
		}
		discovery := privateChunkDiscovery(t, censuschunking.ChunkSize+1)
		workspace, ok := runtime.WorkspaceRoot(runtime.SessionID(), runtime.Generation())
		if !ok {
			t.Fatal("ready runtime workspace unavailable")
		}
		discovery.Workspace = workspace
		discovery.Session = censusacquisition.SessionIdentity{SessionID: runtime.SessionID(), Generation: runtime.Generation()}
		options := privateChunkOptions()
		options.PublicationRoot = root
		handled, _, runErr = runPrivateCensusChunking(ctx, options, censusCoreConfig{}, runtime, discovery, newCensusControlFlowObserver(), privateCensusChunkDependencies{coordinate: func(context.Context, censuschunking.CoordinatorRequest) (censuschunking.CoordinatorResult, error) {
			sawLease = runtime.manager.Census().Workers == 1
			return censuschunking.CoordinatorResult{}, coordinateErr
		}})
		return 0
	})
	if code != 0 || !handled || !errors.Is(runErr, coordinateErr) || !sawLease {
		t.Fatalf("ASSERT_PRIVATE_CHUNK_ONE_REAL_MANAGER_LEASE code=%d handled=%v saw_lease=%v err=%v", code, handled, sawLease, runErr)
	}
	assertInitializedRunnerCleaned(t, observed)
}

func TestPrivateCensusChunkInputPreservesGlobalFields(t *testing.T) {
	discovery := privateChunkDiscovery(t, 2)
	request, err := censusRequestForChunkTest(discovery)
	if err != nil {
		t.Fatal(err)
	}
	input, _, err := privateCensusChunkInput(discovery, request)
	if err != nil {
		t.Fatal(err)
	}
	for i, got := range input.Targets {
		want := discovery.Targets[i]
		if got.GlobalOrdinal != i || got.Name != want.Name || got.URI != want.URI || got.LanguageID != want.LanguageID || got.CanonicalSeedV2 != string(want.CanonicalSeedV2) || got.Coordinate.Line != uint32(i) {
			t.Fatalf("ASSERT_PRIVATE_CHUNK_GLOBAL_FIELDS ordinal=%d got=%+v want=%+v", i, got, want)
		}
	}
}

func censusRequestForChunkTest(discovery censusacquisition.Discovery) (censusrequest.Receipt, error) {
	o := privateChunkOptions()
	return censusrequest.New(censusrequest.RefreshCoordinate{SessionID: discovery.Session.SessionID, Generation: discovery.Session.Generation}, censusrequest.SemanticFields{Sources: o.Sources, DownDepth: uint64(o.DownDepth), UpDepth: uint64(o.UpDepth), MaxNodes: uint64(o.MaxNodes), BatchTargets: uint64(o.MaxBatchTargets), TimeoutMS: uint64(o.Timeout.Milliseconds()), RequestTimeoutMS: uint64(o.RequestTimeout.Milliseconds())})
}

func TestPrivateCensusChunkTerminalMachineOnlyPrivacyCeiling(t *testing.T) {
	terminal := privateCensusChunkTerminal{SchemaVersion: privateCensusChunkTerminalVersion, TerminalSelector: "sha256:terminal", SourceGraphComplete: "UNKNOWN", CrossChunkCalls: "NOT_COMPUTED", AggregateCapture: "NOT_MATERIALIZED"}
	raw, err := marshalPrivateCensusChunkTerminal(terminal)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 7 || fields["authority"] != float64(0) || fields["accepted"] != false {
		t.Fatalf("ASSERT_PRIVATE_CHUNK_TERMINAL_FIELDS %s", raw)
	}
	for _, forbidden := range []string{"result", "catalog", "feature", "completeness"} {
		if bytes.Contains(raw, []byte(forbidden)) {
			t.Fatalf("ASSERT_PRIVATE_CHUNK_TERMINAL_NO_PUBLIC_%s %s", forbidden, raw)
		}
	}
	var stdout, stderr bytes.Buffer
	if code := writeCensusOutcome(&stdout, &stderr, true, censusPublicationOutcome{PrivateChunkTerminal: &terminal}); code != 0 || !bytes.Equal(stdout.Bytes(), append(raw, '\n')) || stderr.Len() != 0 {
		t.Fatalf("ASSERT_PRIVATE_CHUNK_TERMINAL_MACHINE code=%d stdout=%q stderr=%q", code, stdout.Bytes(), stderr.Bytes())
	}
}
