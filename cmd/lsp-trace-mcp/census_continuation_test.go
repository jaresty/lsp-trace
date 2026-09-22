package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"lsp-trace/internal/captureset"
	"lsp-trace/internal/census"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censusresult"
	"lsp-trace/internal/describeworker"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/programccompose"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/seedformat"
)

type mcpContinuationDiscoverer struct{ discovery censusacquisition.Discovery }

func (d mcpContinuationDiscoverer) Discover(context.Context, censusacquisition.SessionIdentity) (censusacquisition.Discovery, error) {
	return d.discovery, nil
}

type mcpContinuationAcquirer struct{}

func (mcpContinuationAcquirer) AcquireV5(_ context.Context, request censusacquisition.BatchRequest) (censusacquisition.AcquiredV5, error) {
	g := graph.Result{SchemaVersion: graph.SchemaVersionV5, Invocation: graph.Invocation{WorkspaceURI: "file:///private/work", LanguageID: "go", Server: graph.ServerInvocation{Command: "fake"}, Seeds: []graph.InvocationSeed{}, Provenance: graph.InvocationProvenance{InvocationID: "i", SourceRevision: "r", ServerVersion: "v"}, Expansion: graph.ExpansionConfig{TopmostSiblings: true}}, Seeds: []graph.SeedResult{}, Summary: graph.Summary{Complete: true}, Capabilities: graph.Capabilities{CallHierarchyProvider: true}}
	native, _ := json.Marshal(g)
	raw, err := graphprovenance.CaptureV5WithSeedSpec(native, request.Session.SessionID, request.Session.Generation, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable, Records: []manageddiagnostic.Record{}}, request.CanonicalSeedsV2)
	return censusacquisition.AcquiredV5{Session: request.Session, Raw: raw}, err
}

func mcpContinuationProjection(t *testing.T) censusacquisition.Projection {
	t.Helper()
	session := censusacquisition.SessionIdentity{SessionID: "publication-session", Generation: 7}
	d := censusacquisition.Discovery{Session: session, Workspace: "/private/work", Complete: true}
	d.FileLedger = captureset.Ledger{Denominator: 1, Entries: []captureset.LedgerEntry{{Ordinal: 0, Identity: "private/file.go", Disposition: censusacquisition.FileProcessed}}}
	d.Accounting.Files = []census.FileEntry{{Ordinal: 0, Disposition: census.FileSelected}}
	d.Accounting.FileDenominator = 1
	for i := 0; i < 65; i++ {
		seed, err := seedformat.EncodeCanonical(seedformat.File{SchemaVersion: seedformat.Version, CoordinateConvention: seedformat.CoordinateConvention, Seeds: []seedformat.Seed{{Type: seedformat.PositionType, Position: &seedformat.Position{Label: fmt.Sprintf("census-%06d", i), Path: fmt.Sprintf("f%03d.go", i), Line: uint64(i + 11), Column: uint64(i + 4)}}}}, d.Workspace)
		if err != nil {
			t.Fatal(err)
		}
		name := fmt.Sprintf("Target%d", i)
		d.Targets = append(d.Targets, censusacquisition.PreparedTarget{CensusOrdinal: i, CanonicalSeedV2: seed, URI: fmt.Sprintf("file:///private/work/f%03d.go", i), SelectionRange: lsp.Range{Start: lsp.Position{Line: uint32(i + 10), Character: uint32(i + 3)}}, Name: name, Kind: 12, SymbolIdentity: fmt.Sprintf("f%03d.go#%d:%d:12:%s:%d", i, i+10, i+3, name, i)})
		d.SymbolLedger.Entries = append(d.SymbolLedger.Entries, captureset.LedgerEntry{Ordinal: i, Identity: d.Targets[i].SymbolIdentity, Disposition: censusacquisition.SymbolPrepared})
		d.Accounting.Symbols = append(d.Accounting.Symbols, census.SymbolEntry{Ordinal: i, Disposition: census.SymbolSelected})
	}
	d.SymbolLedger.Denominator, d.Accounting.SymbolDenominator = 65, 65
	p, err := (censusacquisition.Core{Discoverer: mcpContinuationDiscoverer{d}, Acquirer: mcpContinuationAcquirer{}}).Run(context.Background(), session)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestProductionHostAcceptsCommittedCompletionForStopOnly(t *testing.T) {
	projection := mcpContinuationProjection(t)
	publicationDir := t.TempDir()
	continuationDir := t.TempDir()
	if err := os.Chmod(publicationDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(continuationDir, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(publicationDir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	publisher := captureset.NewPublisher(root)
	metadata := programccompose.ExactMetadata{RevisionCustody: "CALLER_ASSERTED", PositionEncoding: "utf-16", AcquisitionSemantics: "managed-lsp-v1", PrivacyPolicy: "private-census-v1"}
	completion := completeCensusProjectionWithContinuation(context.Background(), projection, publisher, publisher, metadata)
	host, err := newProductionCensusContinuationHost(bootstrapContinuationConfig{PublicationRoot: continuationDir, MaxObjectBytes: 8 << 20}, &continuationRuntimeProbe{workspace: projection.Workspace, languageID: "go"})
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()
	lazy := host.worker.(*lazyMCPContinuationWorker)
	builds := 0
	originalBuild := lazy.build
	lazy.build = func() (*describeworker.Runner, error) {
		builds++
		return originalBuild()
	}
	result, err := host.Fresh(context.Background(), completion, "DESCRIBE_REQUESTS")
	if err != nil || result.Descriptor == "" {
		t.Fatalf("ASSERT_PRODUCTION_STOP_ONLY_ACCEPTS_COMMITTED_COMPLETION: result=%+v err=%v", result, err)
	}
	replayed, err := host.Resume(context.Background(), result.Descriptor, "DESCRIBE_REQUESTS")
	if err != nil || replayed.Status != "PAUSED" {
		t.Fatalf("ASSERT_PRODUCTION_STOP_ONLY_SAME_STORE_REPLAY: result=%+v err=%v", replayed, err)
	}
	if replayed.Descriptor != result.Descriptor || replayed.RequestCount != result.RequestCount || replayed.PreparationCount != result.PreparationCount {
		t.Fatalf("ASSERT_PRODUCTION_STOP_ONLY_REPLAY_IDENTITY: fresh=%+v replayed=%+v", result, replayed)
	}
	if builds != 0 {
		t.Fatalf("ASSERT_PRODUCTION_STOP_ONLY_ZERO_WORKER_CONSTRUCTION: builds=%d", builds)
	}
}

func TestMCPCommittedContinuationCustody(t *testing.T) {
	projection := mcpContinuationProjection(t)
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	publisher := captureset.NewPublisher(root)
	metadata := programccompose.ExactMetadata{RevisionCustody: "CALLER_ASSERTED", PositionEncoding: "utf-16", AcquisitionSemantics: "managed-lsp-v1", PrivacyPolicy: "private-census-v1"}
	completion := completeCensusProjectionWithContinuation(context.Background(), projection, publisher, publisher, metadata)
	if completion.Result == nil || completion.Diagnostic != nil {
		t.Fatalf("ASSERT_MCP_COMMITTED_HANDOFF_BUILDS: %+v", completion)
	}
	before, _ := censusresult.Marshal(*completion.Result)
	handoff, err := completion.BuildCommittedHandoff()
	if err != nil || handoff.Validate() != nil {
		t.Fatalf("ASSERT_MCP_COMMITTED_HANDOFF_VALIDATES: %v", err)
	}
	after, _ := censusresult.Marshal(*completion.Result)
	if !bytes.Equal(before, after) || bytes.Contains(after, []byte("workspace_identity")) || bytes.Contains(after, []byte(projection.Workspace)) {
		t.Fatalf("ASSERT_MCP_HISTORICAL_BYTES_IDENTICAL: %s", after)
	}
	projection.Constituents[0].Raw[0] ^= 1
	p, err := handoff.Projection()
	if err != nil || bytes.Equal(p.Constituents[0].Raw, projection.Constituents[0].Raw) {
		t.Fatalf("ASSERT_MCP_HANDOFF_DEFENSIVE_CLONE: %v", err)
	}
	for name, mutate := range map[string]func(*censusCompletion){
		"missing":  func(c *censusCompletion) { c.continuation = nil },
		"receipt":  func(c *censusCompletion) { c.continuation.publication.Receipt = captureset.PublicationReceipt{} },
		"manifest": func(c *censusCompletion) { c.continuation.publication.Manifest = captureset.Manifest{} },
		"resolved": func(c *censusCompletion) { c.continuation.publication.Resolved = nil },
		"metadata": func(c *censusCompletion) { c.continuation.metadata = programccompose.ExactMetadata{} },
	} {
		t.Run(name, func(t *testing.T) {
			copy := completion
			custody, _ := cloneCensusCompletionCustody(*completion.continuation)
			copy.continuation = &custody
			mutate(&copy)
			if _, err := copy.BuildCommittedHandoff(); err == nil {
				t.Fatalf("ASSERT_MCP_INCOMPLETE_CUSTODY_REJECTED_%s", strings.ToUpper(name))
			}
		})
	}
	diagnostic, _ := censusresult.NewDiagnostic(censusresult.StageCommitted, nil)
	degraded := completion
	degraded.Diagnostic = &diagnostic
	if _, err := degraded.BuildCommittedHandoff(); err == nil {
		t.Fatal("ASSERT_MCP_DEGRADED_HANDOFF_REJECTED")
	}
}
