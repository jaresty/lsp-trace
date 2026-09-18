package retainedreplay

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/retainedinspection"
	"lsp-trace/internal/retainedmanifest"
	"lsp-trace/internal/retainedresolver"
	"lsp-trace/internal/sourceobject"
	"lsp-trace/internal/sourceprojection"
	"lsp-trace/internal/v5sourcesnapshot"
	"lsp-trace/internal/v5sourcesnapshotv2"
)

func TestReplayFreshInstancesHaveExactByteParityAndDeepCopies(t *testing.T) {
	packet := replayFixture(t)
	first, err := Replay(packet)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Replay(clonePacket(packet))
	if err != nil || !bytes.Equal(first, second) {
		t.Fatalf("ASSERT_R04_FRESH_INSTANCE_BYTE_PARITY: err=%v equal=%v", err, bytes.Equal(first, second))
	}
	first[0] ^= 0xff
	third, err := Replay(clonePacket(packet))
	if err != nil || bytes.Equal(first, third) || !bytes.Equal(second, third) {
		t.Fatalf("ASSERT_R04_DEEP_COPY: err=%v", err)
	}
}

func TestReplayRejectsMissingMixedTamperedAndReorderedPacketState(t *testing.T) {
	base := replayFixture(t)
	tests := map[string]func(*Packet){
		"missing object":    func(p *Packet) { p.Objects = nil },
		"tampered object":   func(p *Packet) { p.Objects[0].Bytes[0] ^= 1 },
		"reordered objects": func(p *Packet) { p.Objects[0], p.Objects[1] = p.Objects[1], p.Objects[0] },
		"tampered manifest": func(p *Packet) { p.Manifest[0] = '[' },
		"tampered snapshot": func(p *Packet) { p.SourceSnapshotV2[0] = '[' },
		"tampered request":  func(p *Packet) { p.Request[0] = '[' },
		"mixed request carrier": func(p *Packet) {
			var request map[string]any
			if err := json.Unmarshal(p.Request, &request); err != nil {
				t.Fatal(err)
			}
			evidence := request["retained_source_evidence"].(map[string]any)
			evidence["content_addressed_snapshot_v2"] = map[string]any{"id": digest([]byte("x")), "artifact_byte_length": 1, "artifact_schema_id": retainedinspection.SourceSnapshotSchemaID, "generation": "g-" + digest([]byte("x"))[7:]}
			p.Request, _ = json.Marshal(request)
		},
		"request snapshot mismatch": func(p *Packet) { p.SourceSnapshotV2 = append(p.SourceSnapshotV2, ' ') },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			packet := clonePacket(base)
			mutate(&packet)
			artifact, err := Replay(packet)
			var typed *failure
			if artifact != nil || !errors.As(err, &typed) {
				t.Fatalf("ASSERT_R04_PRIVATE_TYPED_ZERO_FAILURE: artifact=%q err=%T %v", artifact, err, err)
			}
		})
	}
}

func TestExactLookupRejectsSecondDependencyCall(t *testing.T) {
	object := object([]byte("exact"))
	lookup := &exactLookup{objects: map[sourceobject.Identity]sourceobject.Object{object.Identity: object}, calls: map[sourceobject.Identity]int{}}
	if _, err := lookup.Get(object.Identity); err != nil {
		t.Fatal(err)
	}
	if _, err := lookup.Get(object.Identity); !sourceobject.IsCode(err, sourceobject.CodePolicy) {
		t.Fatalf("ASSERT_R04_LOOKUP_EXACT_ONCE: %v", err)
	}
}

func replayFixture(t *testing.T) Packet {
	t.Helper()
	root := t.TempDir()
	content := []byte("package a\nfunc A() {}\n")
	path := filepath.Join(root, "a.go")
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	uri := (&url.URL{Scheme: "file", Path: path}).String()
	zero := uint32(0)
	node := graph.NewNode(graph.Item{Name: "A", Kind: 12, URI: uri, Range: graph.Range{End: graph.Position{Line: 1, Character: 11}}, SelectionRange: graph.Range{End: graph.Position{Line: 1, Character: 6}}})
	origin := graph.NewNode(graph.Item{Name: "Origin", Kind: 12, URI: uri, Range: graph.Range{End: graph.Position{Character: 1}}, SelectionRange: graph.Range{End: graph.Position{Character: 1}}})
	target := acquisition.Target{ID: "root", Locator: acquisition.Locator{URI: uri, Line: &zero, Character: &zero}, DownDepth: 1, UpDepth: 1}
	req := acquisition.Request{Mode: acquisition.Slice, Context: acquisition.AcquisitionContext{ID: "context", SessionID: "session", Generation: 1, PositionEncoding: "utf-16"}, Root: target, Limits: acquisition.Limits{MaxNodes: 10, MaxRequests: 10, MaxEvidenceBytes: 1 << 20, MaxPathWork: 100, Timeout: time.Second, RequestTimeout: time.Second, MaxResponseBytes: 1 << 20, MaxMessages: 16}}
	client := acquisition.NewWireClient(func(_ context.Context, wire acquisition.WireRequest) (json.RawMessage, error) {
		if wire.Method == "textDocument/prepareCallHierarchy" {
			return json.Marshal([]lsp.CallHierarchyItem{{Name: "A", Kind: 12, URI: uri, Range: lsp.Range{End: lsp.Position{Line: 1, Character: 11}}, SelectionRange: lsp.Range{End: lsp.Position{Line: 1, Character: 6}}}})
		}
		return json.RawMessage(`[]`), nil
	})
	acquired, err := acquisition.Acquire(context.Background(), client, req)
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
	g := graph.Result{SchemaVersion: graph.SchemaVersionV5, Invocation: graph.Invocation{Server: graph.ServerInvocation{Command: "fake-lsp"}, Seeds: []graph.InvocationSeed{{Label: "seed", At: "a.go:1:1", ResolvedURI: uri, ContentSHA256: digest(content), LanguageID: "go"}}, Provenance: graph.InvocationProvenance{InvocationID: "session", SourceRevision: "commit", ServerVersion: "fake@1"}, Expansion: graph.ExpansionConfig{TopmostSiblings: true}}, Nodes: []graph.Node{node}, Seeds: []graph.SeedResult{{Label: "seed", ReachedNodeIDs: []string{node.ID}}}, Summary: graph.Summary{NodeCount: 1, Complete: true}, Capabilities: graph.Capabilities{CallHierarchyProvider: true}, SiblingCandidates: []graph.SiblingCandidate{{SeedURI: uri, SeedLabel: "seed", SeedLabels: []string{"seed"}, SeedIdentity: "session:seed:a.go:1:1", Origin: origin, Declaration: &node, Candidate: node, Direction: "SIBLING", Kind: "TOPMOST_SIBLING", ProviderEvidence: []string{"command=fake-lsp;server_version=fake@1;invocation=session"}, LSPEvidence: []string{"textDocument/documentSymbol", "textDocument/prepareCallHierarchy"}, SourceDigests: []string{"candidate=" + digest(content), "origin=" + digest(content)}, Custody: graph.SourceCustodyEvidence{Class: graph.SourceCustodyCallerAssertedLocal, SourceContentSHA256: digest(content), ClaimCeiling: "NO_AUTHENTICATED_ANALYZED_SOURCE_IDENTITY"}}}}
	graphRaw, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	capture, err := graphprovenance.CaptureV5(graphRaw, "session", 1, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable, Records: []manageddiagnostic.Record{}}, &v2)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := v5sourcesnapshot.Build(capture, root, "utf-16")
	if err != nil {
		t.Fatal(err)
	}
	var parentArtifact v5sourcesnapshot.Artifact
	if err := json.Unmarshal(parent, &parentArtifact); err != nil {
		t.Fatal(err)
	}
	binding := parentArtifact.Bindings[0]
	display := v5sourcesnapshotv2.DisplayBinding{GraphSubjectID: binding.NodeID, LogicalSourceID: binding.URI, DisplayRange: binding.Range, DisplayRangePolicy: v5sourcesnapshotv2.DisplayRangePolicy, Provenance: v5sourcesnapshotv2.Provenance{Kind: v5sourcesnapshotv2.ProvenanceKind, Method: v5sourcesnapshotv2.ProvenanceMethod}, ReceiptID: binding.ReceiptIDs[0], SourceDigest: binding.SourceDigest, PositionEncoding: parentArtifact.PositionEncoding, Status: v5sourcesnapshotv2.Status, Custody: v5sourcesnapshotv2.Custody}
	snapshot, _ := json.Marshal(v5sourcesnapshotv2.Artifact{SchemaVersion: v5sourcesnapshotv2.Version, Policy: v5sourcesnapshotv2.Policy, ParentSchemaVersion: v5sourcesnapshot.Version, ParentSnapshotDigest: digest(parent), ParentSnapshot: parent, DisplayBindings: []v5sourcesnapshotv2.DisplayBinding{display}})
	policyID := digest([]byte("policy"))
	primary := object(content)
	extra := object([]byte("authorized-extra"))
	objects := []sourceobject.Object{primary, extra}
	sort.Slice(objects, func(i, j int) bool { return identityLess(objects[i].Identity, objects[j].Identity) })
	entries := make([]retainedmanifest.EntryInput, 0, 2)
	for _, candidate := range objects {
		entries = append(entries, retainedmanifest.EntryInput{Source: candidate.Identity, StorageClass: "CONTENT_ADDRESS", Qualification: "QUALIFIED", PrivacyClassification: "PUBLIC", Availability: "AVAILABLE", Role: "ENDPOINT", GraphSubjectID: display.GraphSubjectID, LogicalSourceID: display.LogicalSourceID, Range: sourceprojection.Range{Start: sourceprojection.Position{Line: display.DisplayRange.Start.Line, Character: display.DisplayRange.Start.Character}, End: sourceprojection.Position{Line: display.DisplayRange.End.Line, Character: display.DisplayRange.End.Character}}, PositionEncoding: "utf-16", PolicyID: policyID, CustodyIdentity: retainedresolver.ContentCustodyIdentity("CONTENT_ADDRESS", candidate.Identity)})
	}
	manifest, _, err := retainedmanifest.Build(graphRaw, capture, entries)
	if err != nil {
		t.Fatal(err)
	}
	key := retainedinspection.Key{GraphSubjectID: display.GraphSubjectID, LogicalSourceID: display.LogicalSourceID}
	request, err := json.Marshal(retainedinspection.Request{Mode: retainedinspection.Mode, RetainedSourceEvidence: retainedinspection.Evidence{InlineSnapshotV2: string(snapshot)}, Selection: retainedinspection.Selection{Target: key, Selections: []retainedinspection.Key{key}}, Projection: retainedinspection.Projection{Body: "INCLUDE", PrivacyPolicyID: policyID, Limits: retainedinspection.ProjectionLimits{MaxSourceBytes: 1 << 20, MaxRanges: 100, MaxObjects: 100, MaxWork: 1 << 20, MaxResponseBytes: 1 << 20}}, ResolveLimits: retainedinspection.ResolveLimits{MaxDistinctObjects: 10, MaxUniqueSourceBytes: 1 << 20, MaxLogicalSelections: 10}})
	if err != nil {
		t.Fatal(err)
	}
	return Packet{Manifest: manifest, Graph: graphRaw, GraphProvenanceV5: capture, SourceSnapshotV2: snapshot, Request: request, Objects: objects}
}

func object(raw []byte) sourceobject.Object {
	return sourceobject.Object{Identity: sourceobject.Identity{Digest: digest(raw), ByteLength: uint64(len(raw))}, Bytes: append([]byte(nil), raw...)}
}
func clonePacket(p Packet) Packet {
	clone := Packet{Manifest: append([]byte(nil), p.Manifest...), Graph: append([]byte(nil), p.Graph...), GraphProvenanceV5: append([]byte(nil), p.GraphProvenanceV5...), SourceSnapshotV2: append([]byte(nil), p.SourceSnapshotV2...), Request: append([]byte(nil), p.Request...), Objects: make([]sourceobject.Object, len(p.Objects))}
	for i, object := range p.Objects {
		clone.Objects[i] = sourceobject.Object{Identity: object.Identity, Bytes: append([]byte(nil), object.Bytes...)}
	}
	return clone
}
