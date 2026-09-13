package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"lsp-trace/acquisitionops"
	"lsp-trace/internal/captureset"
	"lsp-trace/internal/census"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/seedformat"
)

func TestCensusAssemblyCapabilityZeroAndPublicationReplay(t *testing.T) {
	if _, err := (censusAssembly{}).inspect(); err == nil {
		t.Fatal("ASSERT_ZERO_CENSUS_ASSEMBLY_REJECTED")
	}
	if err := publishCensus(context.Background(), censusPublicationCapability{}, censusPublisherFunc(func(context.Context, censusPublicationCapability) error { return nil })); err == nil {
		t.Fatal("ASSERT_ZERO_CENSUS_PUBLICATION_REJECTED")
	}

	projection := censusacquisition.Projection{ManifestBytes: []byte("product-created-test-completion")}
	_, canonical, err := cloneCensusProjection(projection)
	if err != nil {
		t.Fatal(err)
	}
	token := sha256.Sum256(append([]byte("census-assembly-capability\x00"), canonical...))
	state := &censusAssemblyState{projection: projection, token: token}
	assembly := censusAssembly{state: state, token: token}
	capability, err := assembly.publicationCapability()
	if err != nil {
		t.Fatal(err)
	}
	inspected, err := assembly.inspect()
	if err != nil {
		t.Fatal(err)
	}
	inspected.ManifestBytes[0] ^= 0xff
	if assembly.state.projection.ManifestBytes[0] == inspected.ManifestBytes[0] {
		t.Fatal("ASSERT_CENSUS_ASSEMBLY_INSPECTION_IMMUTABLE")
	}
	calls := 0
	publisher := censusPublisherFunc(func(context.Context, censusPublicationCapability) error {
		calls++
		return nil
	})
	if err := publishCensus(context.Background(), capability, publisher); err != nil || calls != 1 {
		t.Fatalf("ASSERT_PRODUCT_COMPLETION_PUBLISHES_ONCE: err=%v calls=%d", err, calls)
	}
	if err := publishCensus(context.Background(), capability, publisher); err == nil || calls != 1 {
		t.Fatalf("ASSERT_CENSUS_PUBLICATION_REPLAY_REJECTED: err=%v calls=%d", err, calls)
	}
}

func TestRunInitializedCensusAcquisitionRejectsMissingRuntime(t *testing.T) {
	if _, err := runInitializedCensusAcquisition(context.Background(), nil, nil, acquisitionops.Limits{}); err == nil {
		t.Fatal("ASSERT_INITIALIZED_RUNTIME_REQUIRED")
	}
}

type censusDiscoveryFunc func(context.Context, censusacquisition.SessionIdentity) (censusacquisition.Discovery, error)

func (f censusDiscoveryFunc) Discover(ctx context.Context, session censusacquisition.SessionIdentity) (censusacquisition.Discovery, error) {
	return f(ctx, session)
}

type censusAcquirerFunc func(context.Context, censusacquisition.BatchRequest) (censusacquisition.AcquiredV5, error)

func (f censusAcquirerFunc) AcquireV5(ctx context.Context, request censusacquisition.BatchRequest) (censusacquisition.AcquiredV5, error) {
	return f(ctx, request)
}

func censusPublicationProjection(t *testing.T, targetCount int) censusacquisition.Projection {
	t.Helper()
	session := censusacquisition.SessionIdentity{SessionID: "publication-session", Generation: 7}
	discovery := censusacquisition.Discovery{Session: session, Workspace: "/private/work", Complete: true}
	discovery.FileLedger = captureset.Ledger{Denominator: 1, Entries: []captureset.LedgerEntry{{Ordinal: 0, Identity: "private/file.go", Disposition: censusacquisition.FileProcessed}}}
	discovery.Accounting.Files = []census.FileEntry{{Ordinal: 0, Disposition: census.FileSelected}}
	discovery.Accounting.FileDenominator = 1
	for i := 0; i < targetCount; i++ {
		seed, err := seedformat.EncodeCanonical(seedformat.File{SchemaVersion: seedformat.Version, CoordinateConvention: seedformat.CoordinateConvention, Seeds: []seedformat.Seed{{Type: seedformat.PositionType, Position: &seedformat.Position{Label: fmt.Sprintf("census-%06d", i), Path: fmt.Sprintf("f%03d.go", i), Line: uint64(i + 11), Column: uint64(i + 4)}}}}, "/private/work")
		if err != nil {
			t.Fatal(err)
		}
		name := fmt.Sprintf("Target%d", i)
		discovery.Targets = append(discovery.Targets, censusacquisition.PreparedTarget{CensusOrdinal: i, CanonicalSeedV2: seed, URI: fmt.Sprintf("file:///private/work/f%03d.go", i), SelectionRange: lsp.Range{Start: lsp.Position{Line: uint32(i + 10), Character: uint32(i + 3)}}, Name: name, Kind: 12, SymbolIdentity: fmt.Sprintf("f%03d.go#%d:%d:12:%s:%d", i, i+10, i+3, name, i)})
		discovery.SymbolLedger.Entries = append(discovery.SymbolLedger.Entries, captureset.LedgerEntry{Ordinal: i, Identity: discovery.Targets[i].SymbolIdentity, Disposition: censusacquisition.SymbolPrepared})
		discovery.Accounting.Symbols = append(discovery.Accounting.Symbols, census.SymbolEntry{Ordinal: i, Disposition: census.SymbolSelected})
	}
	discovery.SymbolLedger.Denominator = targetCount
	discovery.Accounting.SymbolDenominator = targetCount
	core := censusacquisition.Core{
		Discoverer: censusDiscoveryFunc(func(context.Context, censusacquisition.SessionIdentity) (censusacquisition.Discovery, error) {
			return discovery, nil
		}),
		Acquirer: censusAcquirerFunc(func(_ context.Context, request censusacquisition.BatchRequest) (censusacquisition.AcquiredV5, error) {
			g := graph.Result{SchemaVersion: graph.SchemaVersionV5, Invocation: graph.Invocation{Server: graph.ServerInvocation{Command: "fake"}, Seeds: []graph.InvocationSeed{}, Provenance: graph.InvocationProvenance{InvocationID: "i", SourceRevision: "r", ServerVersion: "v"}, Expansion: graph.ExpansionConfig{TopmostSiblings: true}}, Seeds: []graph.SeedResult{}, Summary: graph.Summary{Complete: true}, Capabilities: graph.Capabilities{CallHierarchyProvider: true}}
			native, _ := json.Marshal(g)
			raw, err := graphprovenance.CaptureV5WithSeedSpec(native, request.Session.SessionID, request.Session.Generation, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable, Records: []manageddiagnostic.Record{}}, request.CanonicalSeedsV2)
			return censusacquisition.AcquiredV5{Session: request.Session, Raw: raw}, err
		}),
	}
	projection, err := core.Run(context.Background(), session)
	if err != nil {
		t.Fatal(err)
	}
	return projection
}

func censusPublicationCapabilityFor(t *testing.T, projection censusacquisition.Projection) censusPublicationCapability {
	t.Helper()
	clone, canonical, err := cloneCensusProjection(projection)
	if err != nil {
		t.Fatal(err)
	}
	token := sha256.Sum256(append([]byte("census-assembly-capability\x00"), canonical...))
	return censusPublicationCapability{state: &censusAssemblyState{projection: clone, token: token}, token: token}
}

func TestCensusCaptureSetPublicationOneAndMultipleConstituents(t *testing.T) {
	for _, targetCount := range []int{1, 64} {
		t.Run(fmt.Sprint(targetCount), func(t *testing.T) {
			root := t.TempDir()
			if err := os.Chmod(root, 0o700); err != nil {
				t.Fatal(err)
			}
			projection := censusPublicationProjection(t, targetCount)
			outcome := publishCensusCaptureSet(context.Background(), censusPublicationCapabilityFor(t, projection), root)
			if outcome.Result == nil || outcome.Diagnostic != nil || outcome.Result.Status != "SUCCEEDED" || outcome.Result.Publication.VerificationStatus != "VERIFIED" || outcome.Result.BatchCount != len(projection.Constituents) {
				t.Fatalf("ASSERT_CENSUS_CAPTURE_SET_PUBLISHED: %+v", outcome)
			}
			if strings.Contains(fmt.Sprintf("%+v", outcome), root) {
				t.Fatal("ASSERT_CENSUS_RECEIPT_PATH_FREE")
			}
		})
	}
}

type censusCaptureSetPublisherProbe struct {
	censusCaptureSetPublisher
	resolveCalls int
	verifyErr    error
}

func (p *censusCaptureSetPublisherProbe) Verify(selector string, authority captureset.ExactBytesAuthority) (captureset.Manifest, error) {
	manifest, err := p.censusCaptureSetPublisher.Verify(selector, authority)
	if err == nil && p.verifyErr != nil {
		return captureset.Manifest{}, p.verifyErr
	}
	return manifest, err
}

func (p *censusCaptureSetPublisherProbe) ResolveConstituent(selector, constituent string, authority captureset.ExactBytesAuthority) ([]byte, error) {
	p.resolveCalls++
	return p.censusCaptureSetPublisher.ResolveConstituent(selector, constituent, authority)
}

func TestCensusCaptureSetPublicationResolvesEveryConstituentAndKeepsCommittedDegradation(t *testing.T) {
	old := newCensusCaptureSetPublisher
	t.Cleanup(func() { newCensusCaptureSetPublisher = old })
	var probe *censusCaptureSetPublisherProbe
	newCensusCaptureSetPublisher = func(root *publication.Root) censusCaptureSetPublisher {
		probe = &censusCaptureSetPublisherProbe{censusCaptureSetPublisher: captureset.NewPublisher(root)}
		return probe
	}
	root := t.TempDir()
	_ = os.Chmod(root, 0o700)
	projection := censusPublicationProjection(t, 64)
	outcome := publishCensusCaptureSet(context.Background(), censusPublicationCapabilityFor(t, projection), root)
	if outcome.Result == nil || outcome.Diagnostic != nil || probe.resolveCalls != len(projection.Constituents) {
		t.Fatalf("ASSERT_EVERY_CONSTITUENT_RESOLVED: calls=%d outcome=%+v", probe.resolveCalls, outcome)
	}

	root = t.TempDir()
	_ = os.Chmod(root, 0o700)
	newCensusCaptureSetPublisher = func(root *publication.Root) censusCaptureSetPublisher {
		return &censusCaptureSetPublisherProbe{censusCaptureSetPublisher: captureset.NewPublisher(root), verifyErr: fmt.Errorf("private path %s", root.Path())}
	}
	outcome = publishCensusCaptureSet(context.Background(), censusPublicationCapabilityFor(t, censusPublicationProjection(t, 1)), root)
	if outcome.Result == nil || outcome.Result.Publication.VerificationStatus != "COMMITTED_VERIFICATION_FAILED" || outcome.Diagnostic == nil || outcome.Diagnostic.Status != "SUCCEEDED_DEGRADED" || outcome.Diagnostic.Retry {
		t.Fatalf("ASSERT_COMMITTED_DEGRADATION_SUCCESS: %+v", outcome)
	}
	if strings.Contains(fmt.Sprintf("%+v", outcome), root) {
		t.Fatal("ASSERT_COMMITTED_DEGRADATION_PATH_FREE")
	}
}

func TestCensusCaptureSetPublicationRejectsReplayMutationAndPrivateRootFailures(t *testing.T) {
	projection := censusPublicationProjection(t, 1)
	root := t.TempDir()
	_ = os.Chmod(root, 0o700)
	capability := censusPublicationCapabilityFor(t, projection)
	if got := publishCensusCaptureSet(context.Background(), capability, root); got.Result == nil {
		t.Fatalf("first=%+v", got)
	}
	if got := publishCensusCaptureSet(context.Background(), capability, root); got.Result != nil || got.Diagnostic == nil || got.Diagnostic.Stage != censusStagePublication {
		t.Fatalf("replay=%+v", got)
	}
	if got := publishCensusCaptureSet(context.Background(), censusPublicationCapabilityFor(t, projection), root); got.Result != nil || got.Diagnostic == nil || got.Diagnostic.Stage != censusStagePublication {
		t.Fatalf("competitor=%+v", got)
	}
	mutated := censusPublicationCapabilityFor(t, projection)
	mutated.state.projection.CensusID = "mutated"
	if got := publishCensusCaptureSet(context.Background(), mutated, t.TempDir()); got.Result != nil || got.Diagnostic == nil {
		t.Fatalf("mutation=%+v", got)
	}
	public := t.TempDir()
	_ = os.Chmod(public, 0o755)
	if got := publishCensusCaptureSet(context.Background(), censusPublicationCapabilityFor(t, projection), public); got.Result != nil || got.Diagnostic == nil || got.Diagnostic.Retry != true {
		t.Fatalf("public root=%+v", got)
	}
	parent := t.TempDir()
	private := t.TempDir()
	_ = os.Chmod(private, 0o700)
	symlink := parent + "/root"
	if err := os.Symlink(private, symlink); err != nil {
		t.Fatal(err)
	}
	if got := publishCensusCaptureSet(context.Background(), censusPublicationCapabilityFor(t, projection), symlink); got.Result != nil || got.Diagnostic == nil {
		t.Fatalf("symlink root=%+v", got)
	}
}
