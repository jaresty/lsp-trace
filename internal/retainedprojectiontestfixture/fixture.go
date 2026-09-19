// Package retainedprojectiontestfixture provides shared test support for
// constructing a genuine, serialized v5sourcesnapshotv2 artifact that passes
// v5sourcesnapshotv2.Validate and retainedprojection.Admit. It is imported by
// tests in more than one package, so it lives outside any _test.go file.
package retainedprojectiontestfixture

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
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/v5sourcesnapshot"
	"lsp-trace/internal/v5sourcesnapshotv2"
)

// Artifact is a genuine V2 snapshot artifact plus the identities a caller needs
// to drive selection and lookup against it.
type Artifact struct {
	Raw            []byte // serialized v5sourcesnapshotv2 artifact (admits)
	NodeID         string // the single display binding's graph_subject_id
	URI            string // the single display binding's logical_source_id
	SourceDigest   string // sha256 of the source content
	Content        []byte // the source bytes the Lookup should return
	GraphV5Digest  string // parent.GraphV5Digest — the custody GraphDigest
	GraphV5ByteLen uint64 // len(parent.GraphV5Bytes) — the custody GraphByteLength
}

func digestOf(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// ValidV2Artifact builds a genuine serialized v5sourcesnapshotv2 artifact that
// passes Validate/Admit, running the full acquisition -> CaptureV2 -> CaptureV5
// -> v5sourcesnapshot.Build -> v2 pipeline with a fake LSP wire client. It uses
// only exported APIs so it is portable across packages.
func ValidV2Artifact(t testing.TB) Artifact {
	t.Helper()
	root := t.TempDir()
	content := []byte("package a\nfunc A() {}\n")
	if err := os.WriteFile(filepath.Join(root, "a.go"), content, 0600); err != nil {
		t.Fatal(err)
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.Join(root, "a.go")}).String()
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
	raw, err := json.Marshal(artifact)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v5sourcesnapshotv2.Validate(raw); err != nil {
		t.Fatalf("fixture invalid: %v", err)
	}
	return Artifact{
		Raw:            raw,
		NodeID:         binding.NodeID,
		URI:            binding.URI,
		SourceDigest:   binding.SourceDigest,
		Content:        content,
		GraphV5Digest:  snapshot.GraphV5Digest,
		GraphV5ByteLen: uint64(len(snapshot.GraphV5Bytes)),
	}
}
