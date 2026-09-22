package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/internal/acquisitionengine"
	"lsp-trace/internal/acquisitionorchestration"
	"lsp-trace/internal/captureset"
	"lsp-trace/internal/census"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censusdiagnostic"
	"lsp-trace/internal/censusrequest"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/mcp"
	"lsp-trace/internal/operation"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/seedformat"
	"lsp-trace/sessionruntime"
)

func TestCensusGatewayAcquisitionFailureNeedsPrivateDiagnosticRED(t *testing.T) {
	rootPath := t.TempDir()
	if err := os.Chmod(rootPath, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	server, manager, err := newServerRuntime(true, root)
	if err != nil {
		t.Fatal(err)
	}
	binding, ok := server.Executors[mcp.CensusExecutorFamily].(*privateCensusMCPBinding)
	if !ok {
		t.Fatal("ASSERT_GATEWAY_CENSUS_PRIVATE_BINDING")
	}
	runtime, ok := binding.runtime.(*censusRuntime)
	if !ok {
		t.Fatal("ASSERT_GATEWAY_CENSUS_RUNTIME")
	}
	workspace := t.TempDir()
	session := censusacquisition.SessionIdentity{SessionID: "gateway-session", Generation: 1}
	receipt, err := censusrequest.New(censusrequest.RefreshCoordinate{SessionID: session.SessionID, Generation: session.Generation}, censusrequest.SemanticFields{Sources: []string{"fixture.go"}, DownDepth: 1, UpDepth: 0, MaxNodes: 1, BatchTargets: 1, TimeoutMS: 1000, RequestTimeoutMS: 1000})
	if err != nil {
		t.Fatal(err)
	}
	diagnosticDir := t.TempDir()
	if err := os.Chmod(diagnosticDir, 0o700); err != nil {
		t.Fatal(err)
	}
	diagnosticPath := filepath.Join(diagnosticDir, "acquisition.ndjson")
	f, err := os.OpenFile(diagnosticPath, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	recorder, err := censusdiagnostic.NewRecorder(diagnosticPath, censusdiagnostic.DefaultMaxBytes, censusdiagnostic.DefaultMaxRecords)
	if err != nil {
		t.Fatal(err)
	}
	runtime.acquisitionDiagnostic = recorder
	runtime.runtime = &hostSelectorRuntime{Manager: manager}
	runtime.admit = func(context.Context, []byte) (censusAdmittedSession, *censusAdmissionFailure, censusFailureReason) {
		return censusAdmittedSession{sessionID: session.SessionID, generation: session.Generation, workspace: workspace, positionEncoding: "utf-16", options: censusRuntimeConfig{sources: []string{"fixture.go"}, downDepth: 1, maxNodes: 1, maxBatchTargets: 1, timeoutMS: 1000, requestTimeoutMS: 1000}, requestReceipt: receipt}, nil, reasonNone
	}
	runtime.discover = func(context.Context, *sessionruntime.Manager, censusAdmittedSession, censusRuntimeConfig) (censusacquisition.Discovery, error) {
		return gatewayCompleteDiscovery(t, workspace, session), nil
	}
	calls := 0
	runtime.batchExecute = func(context.Context, *hostSelectorRuntime, censusAdmittedSession, string, acquisitionengine.Manifest, []byte) (acquisitionorchestration.PlannedBatchResult, *operation.Failure) {
		calls++
		return acquisitionorchestration.PlannedBatchResult{}, &operation.Failure{Code: operation.FailureInternal}
	}

	arguments := map[string]any{"session_id": session.SessionID, "generation": session.Generation, "sources": []string{"fixture.go"}, "down_depth": 1, "up_depth": 0, "max_nodes": 1, "timeout_ms": 1000, "request_timeout_ms": 1000}
	request := map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "lsp_trace_v1_execute", "arguments": map[string]any{"request": map[string]any{"operation": "lsp_trace_v1_census", "arguments": arguments}}}}
	raw, _ := json.Marshal(request)
	var stdout bytes.Buffer
	if err := server.Serve(bytes.NewReader(append(raw, '\n')), &stdout); err != nil {
		t.Fatal(err)
	}
	var response map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	result, _ := response["result"].(map[string]any)
	structured, _ := result["structuredContent"].(map[string]any)
	delegated, _ := structured["delegated_envelope"].(string)
	if calls != 1 || !bytes.Contains([]byte(delegated), []byte(`"code":"ACQUISITION_FAILED"`)) {
		t.Fatalf("ASSERT_GATEWAY_GENERIC_ACQUISITION_FAILURE_AND_SINGLE_EXECUTE calls=%d delegated=%s response=%s", calls, delegated, stdout.Bytes())
	}
	ledger := diagnosticPath
	contents, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatalf("ASSERT_PRIVATE_ACQUISITION_DIAGNOSTIC_RECORDED ledger=%s err=%v", ledger, err)
	}
	var records []censusdiagnostic.Record
	for _, line := range bytes.Split(bytes.TrimSpace(contents), []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		var record censusdiagnostic.Record
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatalf("ASSERT_PRIVATE_ACQUISITION_DIAGNOSTIC_JSON err=%v contents=%s", err, contents)
		}
		records = append(records, record)
	}
	if len(records) != 1 {
		t.Fatalf("ASSERT_PRIVATE_ACQUISITION_DIAGNOSTIC_RECORDED records=%d ledger=%s", len(records), contents)
	}
	record := records[0]
	if record.Fingerprint != receipt.Fingerprint || record.Category != "ACQUISITION" || record.Ordinal == nil || *record.Ordinal != 0 || record.OperationCode != operation.FailureInternal || record.OperationCategory != "ACQUISITION" {
		t.Fatalf("ASSERT_PRIVATE_ACQUISITION_DIAGNOSTIC_FIELDS record=%+v receipt=%s", record, receipt.Fingerprint)
	}
	if bytes.Count(contents, []byte{'\n'}) != 1 || bytes.Contains(contents, []byte("CENSUS_BATCH")) {
		t.Fatalf("ASSERT_PRIVATE_ACQUISITION_DIAGNOSTIC_NO_DUPLICATE_OR_SYNTHETIC contents=%s", contents)
	}
	if bytes.Contains([]byte(delegated), []byte("censusPrivateFailure")) || !bytes.Contains([]byte(delegated), []byte(`"code":"ACQUISITION_FAILED"`)) {
		t.Fatalf("ASSERT_PUBLIC_DELEGATED_GENERIC_FAILURE_SHAPE delegated=%s", delegated)
	}
}

func gatewayCompleteDiscovery(t *testing.T, workspace string, session censusacquisition.SessionIdentity) censusacquisition.Discovery {
	t.Helper()
	path := "fixture.go"
	seed, err := seedformat.EncodeCanonical(seedformat.File{SchemaVersion: seedformat.Version, CoordinateConvention: seedformat.CoordinateConvention, Seeds: []seedformat.Seed{{Type: seedformat.PositionType, Position: &seedformat.Position{Label: "census-000000", Path: path, Line: 1, Column: 1}}}}, workspace)
	if err != nil {
		t.Fatal(err)
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.Join(workspace, path)}).String()
	target := censusacquisition.PreparedTarget{CensusOrdinal: 0, CanonicalSeedV2: seed, URI: uri, SelectionRange: lsp.Range{Start: lsp.Position{}}, Range: lsp.Range{Start: lsp.Position{}, End: lsp.Position{Character: 1}}, Name: "Target", Kind: 12, SymbolIdentity: fmt.Sprintf("%s#0:0:12:Target:0", path)}
	return censusacquisition.Discovery{Session: session, Workspace: workspace, Complete: true, Targets: []censusacquisition.PreparedTarget{target}, Accounting: census.Accounting{FileDenominator: 1, SymbolDenominator: 1, Files: []census.FileEntry{{Ordinal: 0, Disposition: census.FileSelected}}, Symbols: []census.SymbolEntry{{Ordinal: 0, Disposition: census.SymbolSelected}}}, FileLedger: captureset.Ledger{Denominator: 1, Entries: []captureset.LedgerEntry{{Ordinal: 0, Identity: path, Disposition: censusacquisition.FileProcessed}}}, SymbolLedger: captureset.Ledger{Denominator: 1, Entries: []captureset.LedgerEntry{{Ordinal: 0, Identity: target.SymbolIdentity, Disposition: censusacquisition.SymbolPrepared}}}}
}
