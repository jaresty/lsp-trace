package targetpacket

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/censusprogramc"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/sourceobject"
	"lsp-trace/internal/v5sourcesnapshot"
	"lsp-trace/internal/v5sourcesnapshotv2"
)

func digestOf(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// validV2Artifact builds a genuine serialized v5sourcesnapshotv2 artifact that
// passes Validate/Admit, plus the (nodeID, uri, digest) of its single display
// binding so the test can drive node->logical-source mapping. Mirrors
// retainedprojection's custody_fixture_test.go using only exported APIs.
func validV2Artifact(t *testing.T) (raw []byte, nodeID, uri, sourceDigest string, content []byte) {
	t.Helper()
	root := t.TempDir()
	content = []byte("package a\nfunc A() {}\n")
	if err := os.WriteFile(filepath.Join(root, "a.go"), content, 0600); err != nil {
		t.Fatal(err)
	}
	uri = (&url.URL{Scheme: "file", Path: filepath.Join(root, "a.go")}).String()
	zero := uint32(0)
	target := acquisition.Target{ID: "root", Locator: acquisition.Locator{URI: uri, Line: &zero, Character: &zero}, DownDepth: 1, UpDepth: 1}
	request := acquisition.Request{Mode: acquisition.Slice, Context: acquisition.AcquisitionContext{ID: "context", SessionID: "session", Generation: 1, PositionEncoding: "utf-16"}, Root: target, Limits: acquisition.Limits{MaxNodes: 100, MaxRequests: 10, MaxEvidenceBytes: 1 << 20, MaxPathWork: 100, Timeout: time.Second, RequestTimeout: time.Second, MaxResponseBytes: 1 << 20, MaxMessages: 16}}
	client := acquisition.NewWireClient(func(_ context.Context, wire acquisition.WireRequest) (json.RawMessage, error) {
		if wire.Method == "textDocument/prepareCallHierarchy" {
			return json.Marshal([]lsp.CallHierarchyItem{{Name: "A", Kind: 12, URI: uri, Range: lsp.Range{End: lsp.Position{Line: 1, Character: 11}}, SelectionRange: lsp.Range{End: lsp.Position{Line: 1, Character: 6}}}})
		}
		return json.RawMessage(`[]`), nil
	})
	acquired, err := acquisition.Acquire(context.Background(), client, request)
	if err != nil {
		t.Fatal(err)
	}
	v2raw, err := graphprovenance.CaptureV2(context.Background(), acquired, root)
	if err != nil {
		t.Fatal(err)
	}
	var v2 graphprovenance.EvidenceV2
	if err := json.Unmarshal(v2raw, &v2); err != nil {
		t.Fatal(err)
	}
	node := graph.NewNode(graph.Item{Name: "A", Kind: 12, URI: uri, Range: graph.Range{End: graph.Position{Line: 1, Character: 11}}, SelectionRange: graph.Range{End: graph.Position{Line: 1, Character: 6}}})
	origin := graph.NewNode(graph.Item{Name: "Origin", Kind: 12, URI: uri, Range: graph.Range{End: graph.Position{Character: 1}}, SelectionRange: graph.Range{End: graph.Position{Character: 1}}})
	seedDigest := digestOf(content)
	g := graph.Result{SchemaVersion: graph.SchemaVersionV5, Invocation: graph.Invocation{Server: graph.ServerInvocation{Command: "fake-lsp"}, Seeds: []graph.InvocationSeed{{Label: "seed", At: "a.go:1:1", ResolvedURI: uri, ContentSHA256: seedDigest, LanguageID: "go"}}, Provenance: graph.InvocationProvenance{InvocationID: "session", SourceRevision: "commit", ServerVersion: "fake@1"}, Expansion: graph.ExpansionConfig{TopmostSiblings: true}}, Nodes: []graph.Node{node}, Seeds: []graph.SeedResult{{Label: "seed", ReachedNodeIDs: []string{node.ID}}}, Summary: graph.Summary{NodeCount: 1, Complete: true}, Capabilities: graph.Capabilities{CallHierarchyProvider: true}}
	declaration := node
	g.SiblingCandidates = []graph.SiblingCandidate{{SeedURI: uri, SeedLabel: "seed", SeedLabels: []string{"seed"}, SeedIdentity: "session:seed:a.go:1:1", Origin: origin, Declaration: &declaration, Candidate: node, Direction: "SIBLING", Kind: "TOPMOST_SIBLING", ProviderEvidence: []string{"command=fake-lsp;server_version=fake@1;invocation=session"}, LSPEvidence: []string{"textDocument/documentSymbol", "textDocument/prepareCallHierarchy"}, SourceDigests: []string{"candidate=" + seedDigest, "origin=" + seedDigest}, Custody: graph.SourceCustodyEvidence{Class: graph.SourceCustodyCallerAssertedLocal, SourceContentSHA256: seedDigest, ClaimCeiling: "NO_AUTHENTICATED_ANALYZED_SOURCE_IDENTITY"}}}
	native, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	v5, err := graphprovenance.CaptureV5(native, "session", 1, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable, Records: []manageddiagnostic.Record{}}, &v2)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := v5sourcesnapshot.Build(v5, root, "utf-16")
	if err != nil {
		t.Fatal(err)
	}
	var snapshot v5sourcesnapshot.Artifact
	if err := json.Unmarshal(parent, &snapshot); err != nil {
		t.Fatal(err)
	}
	binding := snapshot.Bindings[0]
	artifact := v5sourcesnapshotv2.Artifact{SchemaVersion: v5sourcesnapshotv2.Version, Policy: v5sourcesnapshotv2.Policy, ParentSchemaVersion: v5sourcesnapshot.Version, ParentSnapshotDigest: digestOf(parent), ParentSnapshot: parent, DisplayBindings: []v5sourcesnapshotv2.DisplayBinding{{GraphSubjectID: binding.NodeID, LogicalSourceID: binding.URI, DisplayRange: binding.Range, DisplayRangePolicy: v5sourcesnapshotv2.DisplayRangePolicy, Provenance: v5sourcesnapshotv2.Provenance{Kind: v5sourcesnapshotv2.ProvenanceKind, Method: v5sourcesnapshotv2.ProvenanceMethod}, ReceiptID: binding.ReceiptIDs[0], SourceDigest: binding.SourceDigest, PositionEncoding: snapshot.PositionEncoding, Status: v5sourcesnapshotv2.Status, Custody: v5sourcesnapshotv2.Custody}}}
	raw, err = json.Marshal(artifact)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v5sourcesnapshotv2.Validate(raw); err != nil {
		t.Fatalf("fixture invalid: %v", err)
	}
	return raw, binding.NodeID, binding.URI, binding.SourceDigest, content
}

// resolvingRequest builds a Request whose custody Raw is a genuine V2 artifact
// and whose nomination's SelectedNode maps to that artifact's display binding.
func resolvingRequest(t *testing.T) (Request, string, []byte) {
	t.Helper()
	raw, nodeID, _, digest, content := validV2Artifact(t)
	n := sampleNomination()
	n.SelectedNode = nodeID
	graphDigest, graphLen := graphV5Identity(t, raw)
	custody := SnapshotCustody{
		ConstituentIdentity: n.ConstituentIdentity,
		GraphDigest:         graphDigest,
		GraphByteLength:     graphLen,
		Raw:                 raw,
	}
	req := Request{
		CensusID:    "census-1",
		Nominations: []censusprogramc.Representative{n},
		Custody:     []SnapshotCustody{custody},
		Lookup:      contentLookup{digest: digest, body: content},
		MaxBytes:    1 << 20,
	}
	return req, digest, content
}

// graphV5Identity extracts the embedded Graph V5 digest and byte length from a
// V2 artifact's parent snapshot, matching exactly what CustodyBinding derives
// internally (Property [2]): GraphDigest = parent.GraphV5Digest and
// GraphByteLength = len(parent.GraphV5Bytes).
func graphV5Identity(t *testing.T, v2raw []byte) (string, uint64) {
	t.Helper()
	var artifact v5sourcesnapshotv2.Artifact
	if err := json.Unmarshal(v2raw, &artifact); err != nil {
		t.Fatal(err)
	}
	var parent struct {
		GraphV5Bytes  []byte `json:"graph_v5_bytes"`
		GraphV5Digest string `json:"graph_v5_digest"`
	}
	if err := json.Unmarshal(artifact.ParentSnapshot, &parent); err != nil {
		t.Fatal(err)
	}
	return parent.GraphV5Digest, uint64(len(parent.GraphV5Bytes))
}

// contentLookup is a Lookup that returns the exact body for the requested
// identity, so identity/length/SHA reverification inside Resolve is exercised.
type contentLookup struct {
	digest string
	body   []byte
}

func (c contentLookup) Get(id sourceobject.Identity) (sourceobject.Object, error) {
	return sourceobject.Object{Identity: id, Bytes: append([]byte(nil), c.body...)}, nil
}

// --- Property [3]: selected node maps to exactly one logical source ---------

func TestBuildResolvesSelectedNodeToOneLogicalSource(t *testing.T) {
	req, _, _ := resolvingRequest(t)
	res, err := Build(req)
	if err != nil || res.State != StatePrepared || len(res.Packets) != 1 {
		t.Fatalf("ASSERT_P3_PREPARED: state=%q packets=%d err=%v", res.State, len(res.Packets), err)
	}
	if res.Packets[0].LogicalSourceID == "" {
		t.Fatalf("ASSERT_P3_LOGICAL_SOURCE_MAPPED: empty logical source id")
	}
}

func TestBuildUnknownSelectedNodeFailsClosed(t *testing.T) {
	req, _, _ := resolvingRequest(t)
	req.Nominations[0].SelectedNode = "no-such-node" // zero compatible logical sources
	res, err := Build(req)
	if err == nil || res.State != StateFailed {
		t.Fatalf("ASSERT_P3_UNKNOWN_NODE_FAILS_CLOSED: state=%q err=%v", res.State, err)
	}
}

// --- Property [5]/[6]: resolve + assemble body with bounded, typed outcome ---

func TestBuildResolvesBodyThroughLookup(t *testing.T) {
	req, _, content := resolvingRequest(t)
	res, err := Build(req)
	if err != nil || len(res.Packets) != 1 {
		t.Fatalf("ASSERT_P5_PREPARED: err=%v packets=%d", err, len(res.Packets))
	}
	p := res.Packets[0]
	if p.BodyDigest != digestOf(content) {
		t.Fatalf("ASSERT_P5_BODY_DIGEST_FROM_LOOKUP: got=%q want=%q", p.BodyDigest, digestOf(content))
	}
}

func TestBuildZeroMaxBytesFailsClosed(t *testing.T) {
	req, _, _ := resolvingRequest(t)
	req.MaxBytes = 0 // resource bound violated -> typed failure
	res, err := Build(req)
	if err == nil || res.State != StateFailed {
		t.Fatalf("ASSERT_P5_ZERO_BOUND_FAILS_CLOSED: state=%q err=%v", res.State, err)
	}
}
