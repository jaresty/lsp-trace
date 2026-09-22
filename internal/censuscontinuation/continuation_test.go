package censuscontinuation

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/captureset"
	"lsp-trace/internal/census"
	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censusprogramc"
	"lsp-trace/internal/censusresult"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/programccompose"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/seedformat"
	"lsp-trace/internal/v5sourcesnapshotv6"
)

type discoverFunc func(context.Context, censusacquisition.SessionIdentity) (censusacquisition.Discovery, error)

func (f discoverFunc) Discover(c context.Context, s censusacquisition.SessionIdentity) (censusacquisition.Discovery, error) {
	return f(c, s)
}

type acquireFunc func(context.Context, censusacquisition.BatchRequest) (censusacquisition.AcquiredV5, error)

func (f acquireFunc) AcquireV5(c context.Context, b censusacquisition.BatchRequest) (censusacquisition.AcquiredV5, error) {
	return f(c, b)
}

type fixture struct {
	input          BuildInput
	direct         censusprogramc.Result
	historical     []byte
	publicationDir string
	workspace      string
	sources        map[string][]byte
}

func fixtureGraphNodes(b censusacquisition.BatchRequest, sourceURI string) []graph.Node {
	nodes := make([]graph.Node, len(b.Targets))
	for i, target := range b.Targets {
		line := target.SelectionRange.Start.Line
		nodes[i] = graph.NewNode(graph.Item{Name: target.Name, Kind: target.Kind, URI: sourceURI, Range: graph.Range{Start: graph.Position{Line: line}, End: graph.Position{Line: line, Character: uint32(len("func " + target.Name + "() {}"))}}, SelectionRange: graph.Range{Start: graph.Position{Line: line, Character: 5}, End: graph.Position{Line: line, Character: uint32(5 + len(target.Name))}}})
	}
	if len(nodes) == 1 {
		nodes = append(nodes, graph.NewNode(graph.Item{Name: "fixtureOrigin", Kind: 12, URI: sourceURI, Range: graph.Range{Start: graph.Position{Line: 1}, End: graph.Position{Line: 1, Character: 9}}, SelectionRange: graph.Range{Start: graph.Position{Line: 1}, End: graph.Position{Line: 1, Character: 9}}}))
	}
	return nodes
}

func fixtureEndpoints(censusacquisition.BatchRequest) (caller, callee int) {
	return 0, 1
}

func fixtureGraphEdges(b censusacquisition.BatchRequest, sourceURI string) []graph.Edge {
	nodes := fixtureGraphNodes(b, sourceURI)
	caller, callee := fixtureEndpoints(b)
	edges := []graph.Edge{{CallerNodeID: nodes[caller].ID, CalleeNodeID: nodes[callee].ID, CallSites: []graph.Range{{Start: nodes[caller].SelectionRange.Start, End: nodes[caller].SelectionRange.End}}}}
	r := graph.Result{SchemaVersion: graph.SchemaVersionV5, Nodes: nodes, Edges: edges}
	r.Canonicalize()
	return r.Edges
}

func fixtureExpansion(b censusacquisition.BatchRequest) graph.ExpansionConfig {
	return graph.ExpansionConfig{TopmostSiblings: true, TopmostSiblingOutcome: graph.TopmostSiblingExactRelationsFound}
}

func fixtureSiblingCandidates(b censusacquisition.BatchRequest, sourceURI string, content []byte) []graph.SiblingCandidate {
	nodes := fixtureGraphNodes(b, sourceURI)
	origin, candidate := 1, 0
	declaration := nodes[candidate]
	digest := digestOf(content)
	seed := fixtureInvocationSeeds(b, sourceURI, content)[0]
	label := seed.Label
	return []graph.SiblingCandidate{{SeedURI: sourceURI, SeedLabel: label, SeedLabels: []string{label}, SeedIdentity: fmt.Sprintf("inv-%d:%s:%s", b.Ordinal, label, seed.At), Origin: nodes[origin], Declaration: &declaration, Candidate: nodes[candidate], Direction: "SIBLING", Kind: "TOPMOST_SIBLING", ProviderEvidence: []string{"command=gopls;server_version=v;invocation=" + fmt.Sprintf("inv-%d", b.Ordinal)}, LSPEvidence: []string{"textDocument/documentSymbol", "textDocument/prepareCallHierarchy"}, SourceDigests: []string{"candidate=" + digest, "origin=" + digest}, Custody: graph.SourceCustodyEvidence{Class: graph.SourceCustodyCallerAssertedLocal, SourceContentSHA256: digest, ClaimCeiling: "NO_AUTHENTICATED_ANALYZED_SOURCE_IDENTITY"}}}
}

func fixtureInvocationSeeds(b censusacquisition.BatchRequest, sourceURI string, content []byte) []graph.InvocationSeed {
	seeds := make([]graph.InvocationSeed, len(b.Targets))
	for i, target := range b.Targets {
		seeds[i] = graph.InvocationSeed{Label: fmt.Sprintf("census-%06d", target.CensusOrdinal), At: fmt.Sprintf("a.go:%d:6", target.SelectionRange.Start.Line+1), ResolvedURI: sourceURI, ContentSHA256: digestOf(content), LanguageID: "go"}
	}
	return seeds
}

func fixtureSeedResults(b censusacquisition.BatchRequest, sourceURI string) []graph.SeedResult {
	nodes := fixtureGraphNodes(b, sourceURI)
	seeds := make([]graph.SeedResult, len(b.Targets))
	edges := fixtureGraphEdges(b, sourceURI)
	for i, target := range b.Targets {
		seeds[i] = graph.SeedResult{Label: fmt.Sprintf("census-%06d", target.CensusOrdinal), PreparedTargetIDs: []string{nodes[i].ID}, ReachedNodeIDs: []string{nodes[i].ID}}
	}
	if len(edges) > 0 {
		seeds[0].ReachedNodeIDs = []string{nodes[0].ID}
		seeds[0].ReachedRelationIDs = []string{edges[0].RelationID}
		seeds[0].ReachedEdges = append([]graph.Edge(nil), edges...)
	}
	return seeds
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	workspace := t.TempDir()
	var source strings.Builder
	source.WriteString("package fixture\n\n")
	for i := 0; i < 64; i++ {
		fmt.Fprintf(&source, "func T%d() {}\n", i)
	}
	if err := os.WriteFile(filepath.Join(workspace, "a.go"), []byte(source.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	session := censusacquisition.SessionIdentity{SessionID: "session", Generation: 7}
	d := censusacquisition.Discovery{Session: session, Workspace: workspace, Complete: true, Accounting: census.Accounting{FileDenominator: 1, Files: []census.FileEntry{{Ordinal: 0, Disposition: census.FileSelected}}}, FileLedger: captureset.Ledger{Denominator: 1, Entries: []captureset.LedgerEntry{{Ordinal: 0, Identity: "file", Disposition: "processed"}}}}
	for i := 0; i < 64; i++ {
		name := fmt.Sprintf("T%d", i)
		seed, err := seedformat.EncodeCanonical(seedformat.File{SchemaVersion: seedformat.Version, CoordinateConvention: seedformat.CoordinateConvention, Seeds: []seedformat.Seed{{Type: seedformat.PositionType, Position: &seedformat.Position{Label: fmt.Sprintf("census-%06d", i), Path: "a.go", Line: uint64(i + 3), Column: 6}}}}, workspace)
		if err != nil {
			t.Fatal(err)
		}
		target := censusacquisition.PreparedTarget{CensusOrdinal: i, CanonicalSeedV2: seed, URI: (&url.URL{Scheme: "file", Path: filepath.ToSlash(filepath.Join(workspace, "a.go"))}).String(), SelectionRange: lsp.Range{Start: lsp.Position{Line: uint32(i + 2), Character: 5}}, Name: name, Kind: 12, SymbolIdentity: fmt.Sprintf("a.go#%d:%d:%d:%s:%d", i+2, 5, 12, name, i)}
		d.Targets = append(d.Targets, target)
		d.Accounting.Symbols = append(d.Accounting.Symbols, census.SymbolEntry{Ordinal: i, Disposition: census.SymbolSelected})
		d.SymbolLedger.Entries = append(d.SymbolLedger.Entries, captureset.LedgerEntry{Ordinal: i, Identity: target.SymbolIdentity, Disposition: "prepared"})
	}
	d.Accounting.SymbolDenominator, d.SymbolLedger.Denominator = 64, 64
	projection, err := (censusacquisition.Core{Discoverer: discoverFunc(func(context.Context, censusacquisition.SessionIdentity) (censusacquisition.Discovery, error) {
		return d, nil
	}), Acquirer: acquireFunc(func(_ context.Context, b censusacquisition.BatchRequest) (censusacquisition.AcquiredV5, error) {
		sourceURI := (&url.URL{Scheme: "file", Path: filepath.ToSlash(filepath.Join(workspace, "a.go"))}).String()
		workspaceURI := (&url.URL{Scheme: "file", Path: filepath.ToSlash(workspace)}).String()
		selectedIndex := 0
		selected := b.Targets[selectedIndex]
		line, character := selected.SelectionRange.Start.Line, selected.SelectionRange.Start.Character
		request := acquisition.Request{Mode: acquisition.Slice, Context: acquisition.AcquisitionContext{ID: b.BatchID, SessionID: b.Session.SessionID, Generation: b.Session.Generation, PositionEncoding: "utf-16"}, Root: acquisition.Target{ID: "root", Locator: acquisition.Locator{URI: selected.URI, Line: &line, Character: &character}, DownDepth: 1, UpDepth: 1}, Limits: acquisition.Limits{MaxNodes: 100, MaxRequests: 10, MaxEvidenceBytes: 1 << 20, MaxPathWork: 100, Timeout: time.Second, RequestTimeout: time.Second, MaxResponseBytes: 1 << 20, MaxMessages: 16}}
		nodes := fixtureGraphNodes(b, sourceURI)
		callItem := func(i int) lsp.CallHierarchyItem {
			node, target := nodes[i], b.Targets[i]
			return lsp.CallHierarchyItem{Name: target.Name, Kind: target.Kind, URI: target.URI, Range: lsp.Range{Start: lsp.Position{Line: node.Range.Start.Line, Character: node.Range.Start.Character}, End: lsp.Position{Line: node.Range.End.Line, Character: node.Range.End.Character}}, SelectionRange: lsp.Range{Start: lsp.Position{Line: node.SelectionRange.Start.Line, Character: node.SelectionRange.Start.Character}, End: lsp.Position{Line: node.SelectionRange.End.Line, Character: node.SelectionRange.End.Character}}}
		}
		selectedNode := nodes[selectedIndex]
		client := acquisition.NewWireClient(func(_ context.Context, wire acquisition.WireRequest) (json.RawMessage, error) {
			switch wire.Method {
			case "textDocument/prepareCallHierarchy":
				return json.Marshal([]lsp.CallHierarchyItem{callItem(0)})
			case "callHierarchy/outgoingCalls":
				if len(b.Targets) > 1 {
					return json.Marshal([]lsp.CallHierarchyOutgoingCall{{To: callItem(1), FromRanges: []lsp.Range{callItem(0).SelectionRange}}})
				}
			}
			return json.RawMessage(`[]`), nil
		})
		acquired, e := acquisition.Acquire(context.Background(), client, request)
		if e != nil {
			return censusacquisition.AcquiredV5{}, e
		}
		v2raw, e := graphprovenance.CaptureV2(context.Background(), acquired, workspace)
		if e != nil {
			return censusacquisition.AcquiredV5{}, e
		}
		var v2 graphprovenance.EvidenceV2
		if e := json.Unmarshal(v2raw, &v2); e != nil {
			return censusacquisition.AcquiredV5{}, e
		}
		if len(v2.Bindings) == 0 {
			return censusacquisition.AcquiredV5{}, fmt.Errorf("ASSERT_FIXTURE_V2_SOURCE_BINDING: acquired_nodes=%d", len(acquired.Graph.Nodes))
		}
		if len(acquired.Graph.Nodes) == 0 || acquired.Graph.Nodes[0].ID != selectedNode.ID {
			return censusacquisition.AcquiredV5{}, fmt.Errorf("ASSERT_FIXTURE_V2_NODE_JOIN: acquired=%v selected=%s", acquired.Graph.Nodes, selectedNode.ID)
		}
		native, _ := json.Marshal(graph.Result{SchemaVersion: graph.SchemaVersionV5, Invocation: graph.Invocation{WorkspaceURI: workspaceURI, Server: graph.ServerInvocation{Command: "gopls"}, LanguageID: "go", Seeds: fixtureInvocationSeeds(b, sourceURI, []byte(source.String())), Expansion: fixtureExpansion(b), Provenance: graph.InvocationProvenance{InvocationID: fmt.Sprintf("inv-%d", b.Ordinal), SourceRevision: "rev", ServerVersion: "v"}}, Nodes: fixtureGraphNodes(b, sourceURI), Edges: fixtureGraphEdges(b, sourceURI), Seeds: fixtureSeedResults(b, sourceURI), SiblingCandidates: fixtureSiblingCandidates(b, sourceURI, []byte(source.String())), Summary: graph.Summary{Complete: true}, Capabilities: graph.Capabilities{CallHierarchyProvider: true}})
		raw, e := graphprovenance.CaptureV5WithSeedSpec(native, b.Session.SessionID, b.Session.Generation, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable}, b.CanonicalSeedsV2, &v2)
		return censusacquisition.AcquiredV5{Session: b.Session, Raw: raw}, e
	})}).Run(context.Background(), session)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	exact := make([][]byte, len(projection.Constituents))
	for i := range projection.Constituents {
		exact[i] = append([]byte(nil), projection.Constituents[i].Raw...)
	}
	published := captureset.NewPublisher(root).PublishCaptureSet(projection.Manifest, exact, captureset.NativeV5Authority())
	if published.Err != nil {
		t.Fatal(published.Err)
	}
	resolved := make([]censusprogramc.ResolvedConstituent, len(projection.Manifest.Constituents))
	for i, c := range projection.Manifest.Constituents {
		raw, e := captureset.NewPublisher(root).ResolveConstituent(published.Receipt.Selector, c.ImmutableSelector, captureset.NativeV5Authority())
		if e != nil {
			t.Fatal(e)
		}
		resolved[i] = censusprogramc.ResolvedConstituent{ImmutableSelector: c.ImmutableSelector, Bytes: raw}
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	metadata := programccompose.ExactMetadata{WorkspaceIdentity: "workspace", RevisionCustody: "CALLER_ASSERTED", PositionEncoding: "utf-16", AcquisitionSemantics: "managed-lsp", PrivacyPolicy: "private"}
	pub := censusprogramc.VerifiedPublication{Receipt: *published.Receipt, Manifest: projection.Manifest, Resolved: resolved}
	result, err := censusresult.Build(projection, censusresult.PublicationEvidence{Selector: published.Receipt.Selector, Digest: published.Receipt.ArtifactSHA256, ByteLength: published.Receipt.ByteLength, VerificationStatus: published.Receipt.VerificationStatus, DirectorySyncStatus: censusresult.DirectorySyncComplete, CloseStatus: censusresult.CloseComplete})
	if err != nil {
		t.Fatal(err)
	}
	historical, err := censusresult.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	input := BuildInput{Result: result, Projection: projection, Publication: pub, Metadata: metadata, Workspace: WorkspaceIdentity{URI: (&url.URL{Scheme: "file", Path: filepath.ToSlash(workspace)}).String(), Digest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}
	direct, err := censusprogramc.Compose(censusprogramc.Request{Projection: projection, Publication: pub, Metadata: metadata, Seed: 9})
	if err != nil {
		t.Fatal(err)
	}
	sourceURI := (&url.URL{Scheme: "file", Path: filepath.ToSlash(filepath.Join(workspace, "a.go"))}).String()
	return fixture{input: input, direct: direct, historical: historical, publicationDir: dir, workspace: workspace, sources: map[string][]byte{sourceURI: []byte(source.String())}}
}

type fixtureManagedPrepareFunc func(context.Context, string, string, uint64, string, []string, ManagedPreparationLimits) ([]v5sourcesnapshotv6.PreparedDocument, error)

func (f fixtureManagedPrepareFunc) PrepareManagedDocuments(ctx context.Context, workspace, sessionID string, generation uint64, encoding string, uris []string, limits ManagedPreparationLimits) ([]v5sourcesnapshotv6.PreparedDocument, error) {
	return f(ctx, workspace, sessionID, generation, encoding, uris, limits)
}

func testFreshCapture(t *testing.T, f fixture) *FreshCaptureDependencies {
	return testFreshCaptureWithResolver(t, f, false)
}

func testUnavailableFreshCapture(t *testing.T, f fixture) *FreshCaptureDependencies {
	return testFreshCaptureWithResolver(t, f, true)
}

func testManagedRequest(t *testing.T, f fixture, request Request) Request {
	t.Helper()
	request.FreshCapture = testFreshCapture(t, f)
	return request
}

func testUnavailableManagedRequest(t *testing.T, f fixture, request Request) Request {
	t.Helper()
	request.FreshCapture = testUnavailableFreshCapture(t, f)
	return request
}

func testManagedResumeRequest(t *testing.T, f fixture, request ResumeRequest) ResumeRequest {
	t.Helper()
	request.FreshCapture = testFreshCapture(t, f)
	return request
}

func testFreshCaptureWithResolver(t *testing.T, f fixture, unavailable bool) *FreshCaptureDependencies {
	t.Helper()
	session := f.input.Projection.Session
	preparer := fixtureManagedPrepareFunc(func(_ context.Context, workspace, sessionID string, generation uint64, encoding string, uris []string, _ ManagedPreparationLimits) ([]v5sourcesnapshotv6.PreparedDocument, error) {
		if workspace != f.workspace || sessionID != session.SessionID || generation != session.Generation || encoding != "utf-16" {
			return nil, fmt.Errorf("test managed custody mismatch: workspace=%q session=%q generation=%d encoding=%q", workspace, sessionID, generation, encoding)
		}
		out := make([]v5sourcesnapshotv6.PreparedDocument, 0, len(uris))
		for _, uri := range uris {
			raw, ok := f.sources[uri]
			if !ok {
				return nil, fmt.Errorf("test managed source unavailable: %s", uri)
			}
			out = append(out, v5sourcesnapshotv6.PreparedDocument{URI: uri, Bytes: append([]byte(nil), raw...), Digest: digestBytes(raw), ByteLength: uint64(len(raw)), Version: "fixture-v1", SessionID: sessionID, Generation: generation, PositionEncoding: encoding})
		}
		return out, nil
	})
	resolver := v5sourcesnapshotv6.ResolverFunc(func(_ context.Context, request v5sourcesnapshotv6.ResolveRequest) (v5sourcesnapshotv6.ResolveResult, error) {
		if unavailable {
			return v5sourcesnapshotv6.ResolveResult{}, errors.New("fixture full definition unavailable")
		}
		return v5sourcesnapshotv6.ResolveResult{DisplayRange: request.ItemRange, ItemRange: request.ItemRange, SelectionRange: request.SelectionRange, ProvenanceKind: v5sourcesnapshotv6.ProvenanceKind, Method: v5sourcesnapshotv6.ProvenanceMethod, DocumentDigest: request.DocumentDigest, DocumentVersion: request.DocumentVersion, DocumentByteLength: uint64(len(request.Bytes))}, nil
	})
	return &FreshCaptureDependencies{Context: context.Background(), Preparer: preparer, Resolver: resolver, Limits: ManagedPreparationLimits{MaxDocuments: 100, MaxMessages: 100, MaxWork: 10_000, MaxDocumentBytes: 1 << 20, MaxTotalBytes: 8 << 20}}
}

func TestBuildHandoffClassifiesMalformedProjection(t *testing.T) {
	f := newFixture(t)
	f.input.Projection.Session.SessionID = ""
	_, err := BuildHandoff(f.input)
	if err == nil {
		t.Fatal("ASSERT_MALFORMED_PROJECTION_REJECTED")
	}
	if got := ClassifyValidationCause(err); got != ValidationCauseProjection {
		t.Fatalf("ASSERT_PROJECTION_CAUSE: got %v", got)
	}
}

func TestBuildHandoffRequiresAndReconcilesCommittedEvidence(t *testing.T) {
	f := newFixture(t)
	cases := map[string]func(*BuildInput){
		"receipt-alone": func(x *BuildInput) {
			x.Projection = censusacquisition.Projection{}
			x.Publication.Manifest = captureset.Manifest{}
			x.Publication.Resolved = nil
		},
		"missing-projection":   func(x *BuildInput) { x.Projection = censusacquisition.Projection{} },
		"missing-metadata":     func(x *BuildInput) { x.Metadata = programccompose.ExactMetadata{} },
		"missing-constituents": func(x *BuildInput) { x.Publication.Resolved = nil },
		"extra-constituent": func(x *BuildInput) {
			x.Publication.Resolved = append(x.Publication.Resolved, x.Publication.Resolved[0])
		},
		"duplicate-selector": func(x *BuildInput) {
			x.Publication.Resolved[1].ImmutableSelector = x.Publication.Resolved[0].ImmutableSelector
		},
		"reordered": func(x *BuildInput) {
			x.Publication.Resolved[0], x.Publication.Resolved[1] = x.Publication.Resolved[1], x.Publication.Resolved[0]
		},
		"foreign-ordinal": func(x *BuildInput) { x.Projection.Constituents[0].Ordinal++ },
		"foreign-batch":   func(x *BuildInput) { x.Projection.Constituents[0].BatchID = x.Projection.Constituents[1].BatchID },
		"foreign-selector": func(x *BuildInput) {
			x.Projection.Constituents[0].Identity.ImmutableSelector = x.Projection.Constituents[1].Identity.ImmutableSelector
		},
		"digest": func(x *BuildInput) {
			x.Projection.Constituents[0].Identity.SHA256 = "sha256:" + strings.Repeat("0", 64)
		},
		"length":            func(x *BuildInput) { x.Projection.Constituents[0].Identity.ByteLength++ },
		"bytes":             func(x *BuildInput) { x.Publication.Resolved[0].Bytes[0] ^= 1 },
		"manifest":          func(x *BuildInput) { x.Publication.Manifest.LogicalDigest = "sha256:" + strings.Repeat("0", 64) },
		"publication":       func(x *BuildInput) { x.Result.Publication.Digest = "sha256:" + strings.Repeat("0", 64) },
		"projection-census": func(x *BuildInput) { x.Projection.CensusID += "-foreign" },
		"result-census":     func(x *BuildInput) { x.Result.CensusID += "-foreign" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			x := cloneInput(t, f.input)
			mutate(&x)
			if _, err := BuildHandoff(x); err == nil {
				t.Fatal("ASSERT_BUILD_REJECTS_MISSING_OR_MISMATCHED_EVIDENCE")
			}
		})
	}
}

func TestCanonicalRoundTripPermutationAndStrictParser(t *testing.T) {
	f := newFixture(t)
	h, err := BuildHandoff(f.input)
	if err != nil {
		t.Fatal("ASSERT_VALID_HANDOFF_BUILD:", err)
	}
	raw, err := h.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := Parse(raw)
	if err != nil {
		t.Fatal("ASSERT_EXACT_ROUNDTRIP:", err)
	}
	raw2, _ := parsed.Bytes()
	if !bytes.Equal(raw, raw2) || h.HandoffID() != parsed.HandoffID() {
		t.Fatal("ASSERT_CANONICAL_ROUNDTRIP")
	}
	permuted := cloneInput(t, f.input)
	permuted.Publication.Resolved[0], permuted.Publication.Resolved[1] = permuted.Publication.Resolved[1], permuted.Publication.Resolved[0]
	if _, err := BuildHandoff(permuted); err == nil {
		t.Fatal("ASSERT_NONCANONICAL_INPUT_ORDER_REJECTED")
	}
	bad := map[string][]byte{
		"all-whitespace": []byte(" \n\t"),
		"unknown":        bytes.Replace(raw, []byte(`{"schema_version"`), []byte(`{"unknown":1,"schema_version"`), 1),
		"duplicate":      bytes.Replace(raw, []byte(`{"schema_version"`), []byte(`{"schema_version":"`+SchemaVersion+`","schema_version"`), 1),
		"trailing":       append(append([]byte(nil), raw...), []byte("{}")...),
		"whitespace":     append([]byte(" "), raw...),
		"missing":        bytes.Replace(raw, []byte(`"position_encoding":"utf-16",`), nil, 1),
	}
	for name, b := range bad {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(b); err == nil {
				t.Fatal("ASSERT_STRICT_PARSER_REJECTION")
			}
		})
	}
}

func TestDefensiveOwnershipHistoricalBytesAndAuthorityCeilings(t *testing.T) {
	f := newFixture(t)
	before := append([]byte(nil), f.historical...)
	h, err := BuildHandoff(f.input)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, f.historical) {
		t.Fatal("ASSERT_HISTORICAL_BYTES_UNCHANGED_SUCCESS")
	}
	bad := cloneInput(t, f.input)
	bad.Result.CensusID = "bad"
	_, _ = BuildHandoff(bad)
	if !bytes.Equal(before, f.historical) {
		t.Fatal("ASSERT_HISTORICAL_BYTES_UNCHANGED_FAILURE")
	}
	f.input.Projection.Constituents[0].Raw[0] ^= 1
	f.input.Publication.Resolved[0].Bytes[0] ^= 1
	p, err := h.Projection()
	if err != nil {
		t.Fatal(err)
	}
	p.Constituents[0].Raw[0] ^= 1
	p2, _ := h.Projection()
	if bytes.Equal(f.input.Projection.Constituents[0].Raw, p2.Constituents[0].Raw) || bytes.Equal(p.Constituents[0].Raw, p2.Constituents[0].Raw) {
		t.Fatal("ASSERT_DEFENSIVE_CLONING")
	}
	if h.Authority() != 0 || h.Accepted() || h.Completeness() != CompletenessUnknown {
		t.Fatal("ASSERT_AUTHORITY_CEILINGS")
	}
	if h.WorkspaceIdentity() != (WorkspaceIdentity{URI: (&url.URL{Scheme: "file", Path: filepath.ToSlash(f.workspace)}).String(), Digest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}) {
		t.Fatal("ASSERT_WORKSPACE_IDENTITY_RETAINED")
	}
	raw, _ := h.Bytes()
	if bytes.Contains(raw, []byte(`"Workspace":"/workspace"`)) || bytes.Contains(raw, []byte(`"workspace":"/workspace"`)) {
		t.Fatal("ASSERT_NO_HOST_PATH_SERIALIZED")
	}
}

func TestReconstructDeterministicEquivalentAndNoPublicationIO(t *testing.T) {
	f := newFixture(t)
	h, err := BuildHandoff(f.input)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(f.publicationDir); err != nil {
		t.Fatal(err)
	}
	one, err := ReconstructProgramC(h, 9)
	if err != nil {
		t.Fatal("ASSERT_RECONSTRUCT_NO_IO:", err)
	}
	two, err := ReconstructProgramC(h, 9)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(one, two) || !reflect.DeepEqual(one, f.direct) {
		t.Fatal("ASSERT_RECONSTRUCT_DETERMINISTIC_DIRECT_EQUIVALENCE")
	}
}

func TestCoordinatedSubstitutionRejectedAfterIDRecompute(t *testing.T) {
	f := newFixture(t)
	h, err := BuildHandoff(f.input)
	if err != nil {
		t.Fatal(err)
	}
	forged, err := cloneHandoff(h)
	if err != nil {
		t.Fatal(err)
	}
	forged.wire.Constituents[0].Bytes[len(forged.wire.Constituents[0].Bytes)/2] ^= 1
	forged.wire.Projection.Constituents[0].Raw = append([]byte(nil), forged.wire.Constituents[0].Bytes...)
	forged.wire.HandoffID, err = identity(forged.wire)
	if err != nil {
		t.Fatal(err)
	}
	if err := forged.Validate(); err == nil {
		t.Fatal("ASSERT_COORDINATED_SUBSTITUTION_REJECTED")
	}
}

type partitionFixture struct {
	projection    censusacquisition.Projection
	publication   censusprogramc.VerifiedPublication
	result        censusprogramc.Result
	crossRelation string
}

type normalizedOccurrence struct {
	ID, RelationID, CallerID, CalleeID string
	CallSite                           graph.Range
}

type normalizedSeed struct {
	Label, At, URI, Digest string
}

type normalizedSource struct {
	NodeID, URI           string
	Range, SelectionRange graph.Range
}

type normalizedGraph struct {
	NodeIDs     []string
	Occurrences []normalizedOccurrence
	Seeds       []normalizedSeed
	Sources     []normalizedSource
}

type normalizedRepresentative struct {
	Status, ClaimCeiling, SelectionState, CommunityIdentity, SelectedNode, SourceGraphComplete string
	Members, SCCMembers                                                                        []string
	Distance, Authority                                                                        int
	AllTraversalComplete, AnyTruncated                                                         bool
	IncomingPredecessors                                                                       []censusprogramc.RepresentativePredecessor
}

type normalizedProgramC struct {
	Communities             [][]string
	Candidates              []censusprogramc.Candidate
	RepresentativeState     string
	Nominations, Unresolved []normalizedRepresentative
}

// TestProgramCBatchPartitionInvariance is the Program C partition oracle. The
// one-batch capture is acquisition evidence only: it is never admitted to
// Program C, whose composite policy requires 2..16 constituents.
func TestProgramCBatchPartitionInvariance(t *testing.T) {
	const targetCount = 23
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "a.go"), partitionSource(targetCount), 0o600); err != nil {
		t.Fatal(err)
	}
	oracle := buildPartitionFixture(t, workspace, targetCount, targetCount, false, false)
	if len(oracle.projection.Constituents) != 1 {
		t.Fatalf("ASSERT_PARTITION_ORACLE_ONE_BATCH_ONLY: constituents=%d", len(oracle.projection.Constituents))
	}
	wantGraph := normalizeExactProjection(t, oracle.projection)
	if got := len(wantGraph.Occurrences); got != targetCount { // 22 chain sites plus one parallel site.
		t.Fatalf("ASSERT_PARTITION_ORACLE_OCCURRENCE_COUNT: got=%d want=%d", got, targetCount)
	}

	variants := []struct {
		name                             string
		max                              int
		reverseDiscovery, reverseMembers bool
	}{
		{"16+7", 16, false, false},
		{"8+8+7", 8, false, false},
		{"16+7-reordered-input-and-members", 16, true, true},
		{"8+8+7-reordered-input-and-members", 8, true, true},
	}
	var wantOutput normalizedProgramC
	for i, variant := range variants {
		t.Run(variant.name, func(t *testing.T) {
			got := buildPartitionFixture(t, workspace, targetCount, variant.max, variant.reverseDiscovery, variant.reverseMembers)
			if variant.max == 16 && (len(got.projection.Constituents) != 2 || len(got.projection.Batches[0].Targets) != 16 || len(got.projection.Batches[1].Targets) != 7) {
				t.Fatalf("ASSERT_PARTITION_16_PLUS_7: batches=%d sizes=%v", len(got.projection.Batches), batchSizes(got.projection))
			}
			graphProjection := normalizeComposite(got.result)
			if !reflect.DeepEqual(wantGraph, graphProjection) {
				t.Fatalf("ASSERT_PARTITION_ORACLE_GRAPH_EQUAL: excluded_custody_fields=%v\nwant=%#v\ngot=%#v", partitionCustodyExclusions(), wantGraph, graphProjection)
			}
			output := normalizeProgramC(got.result)
			if i == 0 {
				wantOutput = output
			} else if !reflect.DeepEqual(wantOutput, output) {
				t.Fatalf("ASSERT_PARTITION_PROGRAM_C_OUTPUT_EQUAL: excluded_custody_fields=%v communities_equal=%t candidates_equal=%t state_equal=%t nominations_equal=%t unresolved_equal=%t\nwant=%#v\ngot=%#v", partitionCustodyExclusions(), reflect.DeepEqual(wantOutput.Communities, output.Communities), reflect.DeepEqual(wantOutput.Candidates, output.Candidates), wantOutput.RepresentativeState == output.RepresentativeState, reflect.DeepEqual(wantOutput.Nominations, output.Nominations), reflect.DeepEqual(wantOutput.Unresolved, output.Unresolved), wantOutput, output)
			}
			cross := occurrencesForRelation(graphProjection.Occurrences, got.crossRelation)
			if len(cross) != 2 {
				t.Fatalf("ASSERT_PARALLEL_OCCURRENCES_PRESERVED: got=%d occurrences=%#v", len(cross), cross)
			}
		})
	}

	mutated := buildPartitionFixture(t, workspace, targetCount, 16, false, false)
	removeCrossPartitionOccurrence(t, &mutated, mutated.crossRelation)
	mutatedResult, err := censusprogramc.Compose(censusprogramc.Request{Projection: mutated.projection, Publication: mutated.publication, Metadata: partitionMetadata(), Seed: 9})
	if err == nil && reflect.DeepEqual(wantGraph, normalizeComposite(mutatedResult)) {
		t.Fatal("ASSERT_CROSS_PARTITION_OCCURRENCE_MUTATION_DETECTED: mutation silently equaled oracle")
	}
}

func buildPartitionFixture(t *testing.T, workspace string, targetCount, maxBatchTargets int, reverseDiscovery, reverseMembers bool) partitionFixture {
	return buildPartitionFixtureWithURISelector(t, workspace, targetCount, maxBatchTargets, reverseDiscovery, reverseMembers, nil)
}

func buildPartitionFixtureWithURISelector(t *testing.T, workspace string, targetCount, maxBatchTargets int, reverseDiscovery, reverseMembers bool, selectURI func(graph.Item) string) partitionFixture {
	t.Helper()
	content := partitionSource(targetCount)
	session := censusacquisition.SessionIdentity{SessionID: "partition-session", Generation: 11}
	discovery := partitionDiscovery(t, workspace, session, targetCount, reverseDiscovery)
	sourceURI := (&url.URL{Scheme: "file", Path: filepath.ToSlash(filepath.Join(workspace, "a.go"))}).String()
	workspaceURI := (&url.URL{Scheme: "file", Path: filepath.ToSlash(workspace)}).String()
	allTargets := append([]censusacquisition.PreparedTarget(nil), discovery.Targets...)
	sort.Slice(allTargets, func(i, j int) bool { return allTargets[i].CensusOrdinal < allTargets[j].CensusOrdinal })
	var sourceV2 *graphprovenance.EvidenceV2
	core := censusacquisition.Core{
		Planning: &censusacquisition.PlanningConfig{DownDepth: 1, UpDepth: 1, MaxBatchTargets: maxBatchTargets},
		Discoverer: discoverFunc(func(context.Context, censusacquisition.SessionIdentity) (censusacquisition.Discovery, error) {
			return discovery, nil
		}),
		Acquirer: acquireFunc(func(_ context.Context, b censusacquisition.BatchRequest) (censusacquisition.AcquiredV5, error) {
			nodes := partitionNodesWithURISelector(allTargets, sourceURI, selectURI)
			edges := partitionEdges(t, b, nodes)
			if reverseMembers {
				reverseNodes(nodes)
				reverseEdges(edges)
			}
			g := graph.Result{SchemaVersion: graph.SchemaVersionV5, Invocation: graph.Invocation{WorkspaceURI: workspaceURI, Server: graph.ServerInvocation{Command: "gopls"}, LanguageID: "go", Seeds: fixtureInvocationSeeds(b, sourceURI, content), Expansion: graph.ExpansionConfig{TopmostSiblingOutcome: graph.TopmostSiblingNotRequested}, Provenance: graph.InvocationProvenance{InvocationID: fmt.Sprintf("partition-inv-%02d", b.Ordinal), SourceRevision: "partition-rev", ServerVersion: "v"}}, Nodes: nodes, Edges: edges, Seeds: partitionSeedResults(b, nodes, edges), Summary: graph.Summary{Complete: true}, Capabilities: graph.Capabilities{CallHierarchyProvider: true}}
			g.Canonicalize()
			native, err := json.Marshal(g)
			if err != nil {
				return censusacquisition.AcquiredV5{}, err
			}
			if sourceV2 == nil {
				line, character := b.Targets[0].SelectionRange.Start.Line, b.Targets[0].SelectionRange.Start.Character
				request := acquisition.Request{Mode: acquisition.Slice, Context: acquisition.AcquisitionContext{ID: "partition-source", SessionID: session.SessionID, Generation: session.Generation, PositionEncoding: "utf-16"}, Root: acquisition.Target{ID: "root", Locator: acquisition.Locator{URI: sourceURI, Line: &line, Character: &character}, DownDepth: 1, UpDepth: 1}, Limits: acquisition.Limits{MaxNodes: 100, MaxRequests: 10, MaxEvidenceBytes: 1 << 20, MaxPathWork: 100, Timeout: time.Second, RequestTimeout: time.Second, MaxResponseBytes: 1 << 20, MaxMessages: 16}}
				item := partitionCallItem(allTargets[0], nodes[0])
				acquired, acquireErr := acquisition.Acquire(context.Background(), acquisition.NewWireClient(func(_ context.Context, wire acquisition.WireRequest) (json.RawMessage, error) {
					if wire.Method == "textDocument/prepareCallHierarchy" {
						return json.Marshal([]lsp.CallHierarchyItem{item})
					}
					return json.RawMessage(`[]`), nil
				}), request)
				if acquireErr != nil {
					return censusacquisition.AcquiredV5{}, acquireErr
				}
				rawV2, captureErr := graphprovenance.CaptureV2(context.Background(), acquired, workspace)
				if captureErr != nil {
					return censusacquisition.AcquiredV5{}, captureErr
				}
				var captured graphprovenance.EvidenceV2
				if err := json.Unmarshal(rawV2, &captured); err != nil {
					return censusacquisition.AcquiredV5{}, err
				}
				sourceV2 = &captured
			}
			raw, err := graphprovenance.CaptureV5WithSeedSpec(native, session.SessionID, session.Generation, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable}, b.CanonicalSeedsV2, sourceV2)
			return censusacquisition.AcquiredV5{Session: session, Raw: raw}, err
		}),
	}
	projection, err := core.Run(context.Background(), session)
	if err != nil {
		t.Fatal(err)
	}
	publication := publishPartitionProjection(t, projection)
	result := censusprogramc.Result{}
	if len(projection.Constituents) >= 2 {
		committedResult, buildErr := censusresult.Build(projection, censusresult.PublicationEvidence{Selector: publication.Receipt.Selector, Digest: publication.Receipt.ArtifactSHA256, ByteLength: publication.Receipt.ByteLength, VerificationStatus: publication.Receipt.VerificationStatus, DirectorySyncStatus: censusresult.DirectorySyncComplete, CloseStatus: censusresult.CloseComplete})
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		handoff, handoffErr := BuildHandoff(BuildInput{Result: committedResult, Projection: projection, Publication: publication, Metadata: partitionMetadata(), Workspace: WorkspaceIdentity{URI: workspaceURI, Digest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}})
		if handoffErr != nil {
			t.Fatal(handoffErr)
		}
		result, err = ReconstructProgramC(handoff, 9)
		if err != nil {
			t.Fatal(err)
		}
	}
	nodes := partitionNodes(allTargets, sourceURI)
	cross := partitionRelation(nodes[15], nodes[16], []graph.Range{{Start: nodes[15].SelectionRange.Start, End: nodes[15].SelectionRange.End}, {Start: graph.Position{Line: nodes[15].SelectionRange.Start.Line, Character: 1}, End: graph.Position{Line: nodes[15].SelectionRange.Start.Line, Character: 3}}})
	return partitionFixture{projection: projection, publication: publication, result: result, crossRelation: cross}
}

func partitionDiscovery(t *testing.T, workspace string, session censusacquisition.SessionIdentity, count int, reverse bool) censusacquisition.Discovery {
	t.Helper()
	d := censusacquisition.Discovery{Session: session, Workspace: workspace, Complete: true, Accounting: census.Accounting{FileDenominator: 1, Files: []census.FileEntry{{Ordinal: 0, Disposition: census.FileSelected}}}, FileLedger: captureset.Ledger{Denominator: 1, Entries: []captureset.LedgerEntry{{Ordinal: 0, Identity: "file", Disposition: "processed"}}}}
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("T%d", i)
		seed, err := seedformat.EncodeCanonical(seedformat.File{SchemaVersion: seedformat.Version, CoordinateConvention: seedformat.CoordinateConvention, Seeds: []seedformat.Seed{{Type: seedformat.PositionType, Position: &seedformat.Position{Label: fmt.Sprintf("census-%06d", i), Path: "a.go", Line: uint64(i + 3), Column: 6}}}}, workspace)
		if err != nil {
			t.Fatal(err)
		}
		target := censusacquisition.PreparedTarget{CensusOrdinal: i, CanonicalSeedV2: seed, URI: (&url.URL{Scheme: "file", Path: filepath.ToSlash(filepath.Join(workspace, "a.go"))}).String(), SelectionRange: lsp.Range{Start: lsp.Position{Line: uint32(i + 2), Character: 5}}, Name: name, Kind: 12, SymbolIdentity: fmt.Sprintf("a.go#%d:%d:%d:%s:%d", i+2, 5, 12, name, i)}
		d.Targets = append(d.Targets, target)
		d.Accounting.Symbols = append(d.Accounting.Symbols, census.SymbolEntry{Ordinal: i, Disposition: census.SymbolSelected})
		d.SymbolLedger.Entries = append(d.SymbolLedger.Entries, captureset.LedgerEntry{Ordinal: i, Identity: target.SymbolIdentity, Disposition: "prepared"})
	}
	if reverse {
		reverseTargets(d.Targets)
	}
	d.Accounting.SymbolDenominator, d.SymbolLedger.Denominator = count, count
	return d
}

func partitionSource(count int) []byte {
	var source strings.Builder
	source.WriteString("package fixture\n\n")
	for i := 0; i < count; i++ {
		fmt.Fprintf(&source, "func T%d() {}\n", i)
	}
	return []byte(source.String())
}

func partitionNodes(targets []censusacquisition.PreparedTarget, sourceURI string) []graph.Node {
	return partitionNodesWithURISelector(targets, sourceURI, nil)
}

func partitionNodesWithURISelector(targets []censusacquisition.PreparedTarget, sourceURI string, selectURI func(graph.Item) string) []graph.Node {
	nodes := make([]graph.Node, len(targets))
	for i, target := range targets {
		line := target.SelectionRange.Start.Line
		item := graph.Item{Name: target.Name, Kind: target.Kind, URI: sourceURI, Range: graph.Range{Start: graph.Position{Line: line}, End: graph.Position{Line: line, Character: uint32(len("func " + target.Name + "() {}"))}}, SelectionRange: graph.Range{Start: graph.Position{Line: line, Character: 5}, End: graph.Position{Line: line, Character: uint32(5 + len(target.Name))}}}
		if selectURI != nil {
			item.URI = selectURI(item)
		}
		nodes[i] = graph.NewNode(item)
	}
	return nodes
}

func partitionEdges(t *testing.T, b censusacquisition.BatchRequest, nodes []graph.Node) []graph.Edge {
	t.Helper()
	owned := map[int]bool{}
	for _, target := range b.Targets {
		owned[target.CensusOrdinal] = true
	}
	edges := make([]graph.Edge, 0, len(b.Targets))
	for caller := 0; caller+1 < len(nodes); caller++ {
		if !owned[caller] {
			continue
		}
		sites := []graph.Range{{Start: nodes[caller].SelectionRange.Start, End: nodes[caller].SelectionRange.End}}
		if caller == 15 {
			sites = append(sites, graph.Range{Start: graph.Position{Line: nodes[caller].SelectionRange.Start.Line, Character: 1}, End: graph.Position{Line: nodes[caller].SelectionRange.Start.Line, Character: 3}})
		}
		r := graph.Result{SchemaVersion: graph.SchemaVersionV5, Nodes: nodes, Edges: []graph.Edge{{CallerNodeID: nodes[caller].ID, CalleeNodeID: nodes[caller+1].ID, CallSites: sites}}}
		r.Canonicalize()
		edges = append(edges, r.Edges[0])
	}
	sort.Slice(edges, func(i, j int) bool { return edges[i].RelationID < edges[j].RelationID })
	return edges
}

func partitionSeedResults(b censusacquisition.BatchRequest, nodes []graph.Node, edges []graph.Edge) []graph.SeedResult {
	byOrdinal := make(map[int]graph.Node, len(nodes))
	for i, node := range nodes {
		byOrdinal[i] = node
	}
	out := make([]graph.SeedResult, len(b.Targets))
	for i, target := range b.Targets {
		node := byOrdinal[target.CensusOrdinal]
		out[i] = graph.SeedResult{Label: fmt.Sprintf("census-%06d", target.CensusOrdinal), PreparedTargetIDs: []string{node.ID}, ReachedNodeIDs: []string{node.ID}}
		for _, edge := range edges {
			if edge.CallerNodeID == node.ID {
				out[i].ReachedRelationIDs = append(out[i].ReachedRelationIDs, edge.RelationID)
				out[i].ReachedEdges = append(out[i].ReachedEdges, edge)
			}
		}
	}
	return out
}

func partitionCallItem(target censusacquisition.PreparedTarget, node graph.Node) lsp.CallHierarchyItem {
	return lsp.CallHierarchyItem{Name: target.Name, Kind: target.Kind, URI: target.URI, Range: lsp.Range{Start: lsp.Position{Line: node.Range.Start.Line, Character: node.Range.Start.Character}, End: lsp.Position{Line: node.Range.End.Line, Character: node.Range.End.Character}}, SelectionRange: lsp.Range{Start: lsp.Position{Line: node.SelectionRange.Start.Line, Character: node.SelectionRange.Start.Character}, End: lsp.Position{Line: node.SelectionRange.End.Line, Character: node.SelectionRange.End.Character}}}
}

func publishPartitionProjection(t *testing.T, p censusacquisition.Projection) censusprogramc.VerifiedPublication {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	exact := make([][]byte, len(p.Constituents))
	for i := range p.Constituents {
		exact[i] = append([]byte(nil), p.Constituents[i].Raw...)
	}
	published := captureset.NewPublisher(root).PublishCaptureSet(p.Manifest, exact, captureset.NativeV5Authority())
	if published.Err != nil {
		t.Fatal(published.Err)
	}
	resolved := make([]censusprogramc.ResolvedConstituent, len(p.Manifest.Constituents))
	for i, c := range p.Manifest.Constituents {
		raw, resolveErr := captureset.NewPublisher(root).ResolveConstituent(published.Receipt.Selector, c.ImmutableSelector, captureset.NativeV5Authority())
		if resolveErr != nil {
			t.Fatal(resolveErr)
		}
		resolved[i] = censusprogramc.ResolvedConstituent{ImmutableSelector: c.ImmutableSelector, Bytes: raw}
	}
	if err := root.Close(); err != nil {
		t.Fatal(err)
	}
	return censusprogramc.VerifiedPublication{Receipt: *published.Receipt, Manifest: p.Manifest, Resolved: resolved}
}

func normalizeExactProjection(t *testing.T, p censusacquisition.Projection) normalizedGraph {
	t.Helper()
	if len(p.Constituents) != 1 {
		t.Fatalf("oracle requires exactly one acquisition constituent")
	}
	return normalizeV5Constituents(t, [][]byte{p.Constituents[0].Raw})
}

func normalizeComposite(r censusprogramc.Result) normalizedGraph {
	g := normalizedGraph{}
	for _, node := range r.Composite.Artifact.Nodes {
		g.NodeIDs = append(g.NodeIDs, node.ID)
		g.Sources = append(g.Sources, normalizedSource{NodeID: node.ID, URI: node.URI, Range: node.Range, SelectionRange: node.SelectionRange})
	}
	for _, edge := range r.Composite.Artifact.Edges {
		for _, site := range edge.CallSites {
			g.Occurrences = append(g.Occurrences, normalizedOccurrence{ID: occurrenceSemanticID(edge.RelationID, site), RelationID: edge.RelationID, CallerID: edge.CallerNodeID, CalleeID: edge.CalleeNodeID, CallSite: site})
		}
	}
	for _, constituent := range r.Composite.Artifact.Constituents {
		raw, _ := base64.StdEncoding.DecodeString(constituent.BytesBase64)
		part := normalizeV5ConstituentsNoGraph(raw)
		g.Seeds = append(g.Seeds, part...)
	}
	canonicalNormalizedGraph(&g)
	return g
}

func normalizeV5Constituents(t *testing.T, raws [][]byte) normalizedGraph {
	t.Helper()
	g := normalizedGraph{}
	for _, raw := range raws {
		var envelope graphprovenance.EvidenceV5
		if err := json.Unmarshal(raw, &envelope); err != nil {
			t.Fatal(err)
		}
		native, err := base64.StdEncoding.DecodeString(envelope.GraphV5)
		if err != nil {
			t.Fatal(err)
		}
		var result graph.Result
		if err := json.Unmarshal(native, &result); err != nil {
			t.Fatal(err)
		}
		for _, node := range result.Nodes {
			g.NodeIDs = append(g.NodeIDs, node.ID)
			g.Sources = append(g.Sources, normalizedSource{NodeID: node.ID, URI: node.URI, Range: node.Range, SelectionRange: node.SelectionRange})
		}
		for _, edge := range result.Edges {
			for _, site := range edge.CallSites {
				g.Occurrences = append(g.Occurrences, normalizedOccurrence{ID: occurrenceSemanticID(edge.RelationID, site), RelationID: edge.RelationID, CallerID: edge.CallerNodeID, CalleeID: edge.CalleeNodeID, CallSite: site})
			}
		}
		for _, seed := range result.Invocation.Seeds {
			g.Seeds = append(g.Seeds, normalizedSeed{Label: seed.Label, At: seed.At, URI: seed.ResolvedURI, Digest: seed.ContentSHA256})
		}
	}
	canonicalNormalizedGraph(&g)
	return g
}

func normalizeV5ConstituentsNoGraph(raw []byte) []normalizedSeed {
	var envelope graphprovenance.EvidenceV5
	_ = json.Unmarshal(raw, &envelope)
	native, _ := base64.StdEncoding.DecodeString(envelope.GraphV5)
	var result graph.Result
	_ = json.Unmarshal(native, &result)
	out := make([]normalizedSeed, len(result.Invocation.Seeds))
	for i, seed := range result.Invocation.Seeds {
		out[i] = normalizedSeed{Label: seed.Label, At: seed.At, URI: seed.ResolvedURI, Digest: seed.ContentSHA256}
	}
	return out
}

func canonicalNormalizedGraph(g *normalizedGraph) {
	g.NodeIDs = uniqueSorted(g.NodeIDs)
	sort.Slice(g.Occurrences, func(i, j int) bool { return g.Occurrences[i].ID < g.Occurrences[j].ID })
	sort.Slice(g.Seeds, func(i, j int) bool {
		if g.Seeds[i].Label != g.Seeds[j].Label {
			return g.Seeds[i].Label < g.Seeds[j].Label
		}
		return g.Seeds[i].At < g.Seeds[j].At
	})
	g.Seeds = uniqueSeeds(g.Seeds)
	sort.Slice(g.Sources, func(i, j int) bool { return g.Sources[i].NodeID < g.Sources[j].NodeID })
	g.Sources = uniqueSources(g.Sources)
}

func normalizeProgramC(r censusprogramc.Result) normalizedProgramC {
	out := normalizedProgramC{Candidates: append([]censusprogramc.Candidate(nil), r.Candidates...)}
	for _, community := range r.Outcome.Communities {
		out.Communities = append(out.Communities, append([]string(nil), community.Members...))
	}
	out.RepresentativeState = r.Representatives.State
	for _, representative := range r.Representatives.Nominations {
		out.Nominations = append(out.Nominations, normalizeRepresentative(representative))
	}
	for _, representative := range r.Representatives.Unresolved {
		out.Unresolved = append(out.Unresolved, normalizeRepresentative(representative))
	}
	out.Nominations = canonicalRepresentatives(out.Nominations)
	out.Unresolved = canonicalRepresentatives(out.Unresolved)
	return out
}

func canonicalRepresentatives(in []normalizedRepresentative) []normalizedRepresentative {
	sort.Slice(in, func(i, j int) bool {
		a, _ := json.Marshal(in[i])
		b, _ := json.Marshal(in[j])
		return bytes.Compare(a, b) < 0
	})
	out := in[:0]
	for _, representative := range in {
		if len(out) == 0 || !reflect.DeepEqual(out[len(out)-1], representative) {
			out = append(out, representative)
		}
	}
	return out
}

func normalizeRepresentative(r censusprogramc.Representative) normalizedRepresentative {
	predecessors := append([]censusprogramc.RepresentativePredecessor(nil), r.IncomingPredecessors...)
	for i := range predecessors {
		predecessors[i].OccurrenceID = occurrenceSemanticID(predecessors[i].RelationID, predecessors[i].CallSite)
	}
	return normalizedRepresentative{Status: r.Status, ClaimCeiling: r.ClaimCeiling, SelectionState: r.SelectionState, CommunityIdentity: r.CommunityIdentity, SelectedNode: r.SelectedNode, SourceGraphComplete: r.SourceGraphComplete, Members: append([]string(nil), r.Members...), SCCMembers: append([]string(nil), r.SCCMembers...), Distance: r.Distance, Authority: r.Authority, AllTraversalComplete: r.AllTraversalComplete, AnyTruncated: r.AnyTruncated, IncomingPredecessors: predecessors}
}

func removeCrossPartitionOccurrence(t *testing.T, fixture *partitionFixture, relationID string) {
	t.Helper()
	removed := 0
	for i := range fixture.projection.Constituents {
		var envelope graphprovenance.EvidenceV5
		if err := json.Unmarshal(fixture.projection.Constituents[i].Raw, &envelope); err != nil {
			t.Fatal(err)
		}
		native, err := base64.StdEncoding.DecodeString(envelope.GraphV5)
		if err != nil {
			t.Fatal(err)
		}
		var result graph.Result
		if err := json.Unmarshal(native, &result); err != nil {
			t.Fatal(err)
		}
		mutatedEdge := -1
		for edgeIndex := range result.Edges {
			if result.Edges[edgeIndex].RelationID == relationID && len(result.Edges[edgeIndex].CallSites) == 2 {
				result.Edges[edgeIndex].CallSites = result.Edges[edgeIndex].CallSites[1:]
				mutatedEdge = edgeIndex
				removed++
			}
		}
		if removed == 0 {
			continue
		}
		result.Canonicalize()
		_ = result.Edges[mutatedEdge]
		result.Seeds = partitionSeedResults(fixture.projection.Batches[i], result.Nodes, result.Edges)
		result.Canonicalize()
		newNative, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		var source graphprovenance.EvidenceV2
		hasSource := envelope.WorkspaceURI != ""
		if hasSource {
			source = graphprovenance.EvidenceV2{SchemaVersion: graphprovenance.VersionV2, Policy: graphprovenance.PolicyV2, WorkspaceURI: envelope.WorkspaceURI, AnalyzedVersion: envelope.AnalyzedVersion, DependencyCompleteness: envelope.DependencyCompleteness, CaptureBudget: envelope.CaptureBudget, Supplies: envelope.Supplies, Captures: envelope.Captures, Bindings: envelope.Bindings}
		}
		var newRaw []byte
		var captureErr error
		if hasSource {
			newRaw, captureErr = graphprovenance.CaptureV5WithSeedSpec(newNative, fixture.projection.Session.SessionID, fixture.projection.Session.Generation, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable}, fixture.projection.Batches[i].CanonicalSeedsV2, &source)
		} else {
			newRaw, captureErr = graphprovenance.CaptureV5WithSeedSpec(newNative, fixture.projection.Session.SessionID, fixture.projection.Session.Generation, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable}, fixture.projection.Batches[i].CanonicalSeedsV2)
		}
		if captureErr != nil {
			t.Fatal(captureErr)
		}
		fixture.projection.Constituents[i].Raw = newRaw
		fixture.publication.Resolved[i].Bytes = append([]byte(nil), newRaw...)
		break
	}
	if removed != 1 {
		t.Fatalf("ASSERT_MUTATION_REMOVES_EXACTLY_ONE_OCCURRENCE: removed=%d", removed)
	}
}

func partitionRelation(caller, callee graph.Node, sites []graph.Range) string {
	r := graph.Result{SchemaVersion: graph.SchemaVersionV5, Nodes: []graph.Node{caller, callee}, Edges: []graph.Edge{{CallerNodeID: caller.ID, CalleeNodeID: callee.ID, CallSites: sites}}}
	r.Canonicalize()
	return r.Edges[0].RelationID
}

func occurrenceSemanticID(relation string, site graph.Range) string {
	return fmt.Sprintf("%s@%d:%d-%d:%d", relation, site.Start.Line, site.Start.Character, site.End.Line, site.End.Character)
}
func occurrencesForRelation(in []normalizedOccurrence, relation string) []normalizedOccurrence {
	var out []normalizedOccurrence
	for _, occurrence := range in {
		if occurrence.RelationID == relation {
			out = append(out, occurrence)
		}
	}
	return out
}
func partitionMetadata() programccompose.ExactMetadata {
	return programccompose.ExactMetadata{WorkspaceIdentity: "workspace", RevisionCustody: "CALLER_ASSERTED", PositionEncoding: "utf-16", AcquisitionSemantics: "managed-lsp", PrivacyPolicy: "private"}
}
func batchSizes(p censusacquisition.Projection) []int {
	out := make([]int, len(p.Batches))
	for i := range p.Batches {
		out[i] = len(p.Batches[i].Targets)
	}
	return out
}
func partitionCustodyExclusions() []string {
	return []string{"census_id", "batch_id", "constituent immutable_selector/sha256/byte_length", "publication selector/digest/byte_length", "invocation_id/execution_bundle_id", "composite_id/output_sha256", "admission_id", "representative constituent_identity/constituent_ordinal/prepared_targets/seed_label/seed_at/composite-derived predecessor occurrence_id"}
}
func reverseTargets(in []censusacquisition.PreparedTarget) {
	for i, j := 0, len(in)-1; i < j; i, j = i+1, j-1 {
		in[i], in[j] = in[j], in[i]
	}
}
func reverseNodes(in []graph.Node) {
	for i, j := 0, len(in)-1; i < j; i, j = i+1, j-1 {
		in[i], in[j] = in[j], in[i]
	}
}
func reverseEdges(in []graph.Edge) {
	for i, j := 0, len(in)-1; i < j; i, j = i+1, j-1 {
		in[i], in[j] = in[j], in[i]
	}
}
func uniqueSorted(in []string) []string {
	sort.Strings(in)
	out := in[:0]
	for _, value := range in {
		if len(out) == 0 || out[len(out)-1] != value {
			out = append(out, value)
		}
	}
	return out
}
func uniqueSeeds(in []normalizedSeed) []normalizedSeed {
	out := in[:0]
	for _, value := range in {
		if len(out) == 0 || out[len(out)-1] != value {
			out = append(out, value)
		}
	}
	return out
}
func uniqueSources(in []normalizedSource) []normalizedSource {
	out := in[:0]
	for _, value := range in {
		if len(out) == 0 || out[len(out)-1] != value {
			out = append(out, value)
		}
	}
	return out
}

func cloneInput(t *testing.T, in BuildInput) BuildInput {
	t.Helper()
	p, err := censusacquisition.CloneProjection(in.Projection)
	if err != nil {
		t.Fatal(err)
	}
	out := in
	out.Projection = p
	out.Publication.Resolved = append([]censusprogramc.ResolvedConstituent(nil), in.Publication.Resolved...)
	for i := range out.Publication.Resolved {
		out.Publication.Resolved[i].Bytes = append([]byte(nil), in.Publication.Resolved[i].Bytes...)
	}
	b, _ := json.Marshal(in.Publication.Manifest)
	_ = json.Unmarshal(b, &out.Publication.Manifest)
	return out
}
