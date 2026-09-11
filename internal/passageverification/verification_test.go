package passageverification

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/v5sourcesnapshot"
)

func nativeFixture(t *testing.T, version string) ([]byte, Request) {
	t.Helper()
	uri := "file:///w/a.go"
	n := graph.NewNode(graph.Item{Name: "A", Kind: 12, URI: uri, Range: graph.Range{End: graph.Position{Character: 4}}, SelectionRange: graph.Range{End: graph.Position{Character: 1}}})
	seed := graph.InvocationSeed{Label: "seed", At: "a.go:1:1", ResolvedURI: uri, ContentSHA256: digest([]byte("A😀B")), LanguageID: "go"}
	g := graph.Result{SchemaVersion: version, Invocation: graph.Invocation{Server: graph.ServerInvocation{Command: "fake-lsp"}, Seeds: []graph.InvocationSeed{seed}, Provenance: graph.InvocationProvenance{InvocationID: "session", SourceRevision: "commit", ServerVersion: "fake@1"}, Expansion: graph.ExpansionConfig{TopmostSiblings: true}}, Nodes: []graph.Node{n}, Seeds: []graph.SeedResult{{Label: "seed", ReachedNodeIDs: []string{n.ID}}}, Summary: graph.Summary{NodeCount: 1, Complete: true}, Capabilities: graph.Capabilities{CallHierarchyProvider: true}}
	if version == graph.SchemaVersionV5 {
		origin := graph.NewNode(graph.Item{Name: "Origin", Kind: 12, URI: uri, Range: graph.Range{Start: graph.Position{Line: 1}, End: graph.Position{Line: 1, Character: 1}}, SelectionRange: graph.Range{Start: graph.Position{Line: 1}, End: graph.Position{Line: 1, Character: 1}}})
		g.SiblingCandidates = []graph.SiblingCandidate{{SeedURI: uri, SeedLabel: "seed", SeedLabels: []string{"seed"}, SeedIdentity: "session:seed:a.go:1:1", Origin: origin, Candidate: n, Direction: "SIBLING", Kind: "TOPMOST_SIBLING", ProviderEvidence: []string{"command=fake-lsp;server_version=fake@1;invocation=session"}, LSPEvidence: []string{"textDocument/documentSymbol", "textDocument/prepareCallHierarchy"}, SourceDigests: []string{"candidate=" + seed.ContentSHA256, "origin=" + seed.ContentSHA256}, Custody: graph.SourceCustodyEvidence{Class: graph.SourceCustodyCallerAssertedLocal, SourceContentSHA256: seed.ContentSHA256, ClaimCeiling: "NO_AUTHENTICATED_ANALYZED_SOURCE_IDENTITY"}}}
	}
	raw, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	var id struct {
		ExecutionBundleID string `json:"execution_bundle_id"`
	}
	_ = json.Unmarshal(raw, &id)
	q := Request{Artifact: Artifact{Bytes: raw}, ExpectedArtifactSHA256: digest(raw), InspectionID: id.ExecutionBundleID, SeedLabel: "seed", NodeID: n.ID, ExpectedURI: uri, RangeMode: Exact, PositionEncoding: "utf-16", PositionConvention: "LSP_ZERO_BASED_END_EXCLUSIVE", ExpectedRange: Range{End: Position{Character: 4}}, ExpectedPassageSHA256: digest([]byte("A😀B"))}
	return raw, q
}

func TestV3AttributionOnly(t *testing.T) {
	_, q := nativeFixture(t, graph.SchemaVersionV3)
	got := Verify(q)
	if got.Checks.Range != Verified || got.Checks.PassageBytes != SourceBytesUnavailable || !got.ReacquisitionNeeded || got.Checks.BodyCompleteness != NotEvaluated {
		t.Fatalf("%+v", got)
	}
}
func TestMutationsAndRanges(t *testing.T) {
	_, q := nativeFixture(t, graph.SchemaVersionV3)
	tests := []struct {
		name string
		mut  func(*Request)
		want Status
	}{
		{"digest", func(x *Request) { x.ExpectedArtifactSHA256 = digest([]byte("wrong")) }, ArtifactDigestMismatch},
		{"node", func(x *Request) { x.NodeID = "missing" }, NodeAbsent},
		{"uri", func(x *Request) { x.ExpectedURI = "file:///other" }, PathMismatch},
		{"exact", func(x *Request) { x.ExpectedRange.End.Character = 3 }, RangeMismatch},
		{"intersects", func(x *Request) {
			x.RangeMode = Intersects
			x.ExpectedRange = Range{Start: Position{Character: 1}, End: Position{Character: 2}}
		}, SourceBytesUnavailable},
		{"invalid", func(x *Request) { x.ExpectedRange.Start.Line = -1 }, InvalidInput},
		{"reversed", func(x *Request) { x.ExpectedRange.Start.Character = 5 }, InvalidInput},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x := q
			tt.mut(&x)
			if got := Verify(x); got.Overall != tt.want {
				t.Fatalf("got %s: %+v", got.Overall, got)
			}
		})
	}
}
func TestSeedAmbiguousAndUnknownFields(t *testing.T) {
	raw, q := nativeFixture(t, graph.SchemaVersionV3)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	ss := m["seeds"].([]any)
	m["seeds"] = append(ss, ss[0])
	bad, _ := json.Marshal(m)
	q.Artifact.Bytes = bad
	q.ExpectedArtifactSHA256 = digest(bad)
	if got := Verify(q); got.Overall != ArtifactInvalid || got.Checks.Seed != SeedAmbiguous {
		t.Fatalf("semantic admission should reject and identify duplicate seed: %+v", got)
	}
	if _, err := MarshalStrictBatch([]byte(`{"records":[],"unknown":1}`)); err == nil {
		t.Fatal("unknown field admitted")
	}
	if _, err := MarshalStrictBatch([]byte(`{"records":[],"records":[]}`)); err == nil {
		t.Fatal("duplicate field admitted")
	}
}

func retainedV5(t *testing.T, encoding string) ([]byte, []byte, Request, string) {
	t.Helper()
	root := t.TempDir()
	content := []byte("A😀B")
	end := map[string]uint32{"utf-8": 6, "utf-16": 4, "utf-32": 3}[encoding]
	if err := os.WriteFile(filepath.Join(root, "a.go"), content, 0600); err != nil {
		t.Fatal(err)
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.Join(root, "a.go")}).String()
	zero := uint32(0)
	target := acquisition.Target{ID: "root", Locator: acquisition.Locator{URI: uri, Line: &zero, Character: &zero}, DownDepth: 1, UpDepth: 1}
	ar := acquisition.Request{Mode: acquisition.Slice, Context: acquisition.AcquisitionContext{ID: "context", SessionID: "session", Generation: 1, PositionEncoding: encoding}, Root: target, Limits: acquisition.Limits{MaxNodes: 10000, MaxRequests: 100, MaxEvidenceBytes: 1 << 20, MaxPathWork: 1000, Timeout: time.Second, RequestTimeout: time.Second, MaxResponseBytes: 1 << 20, MaxMessages: 64}}
	client := acquisition.NewWireClient(func(_ context.Context, w acquisition.WireRequest) (json.RawMessage, error) {
		if w.Method == "textDocument/prepareCallHierarchy" {
			return json.Marshal([]lsp.CallHierarchyItem{{Name: "A", Kind: 12, URI: uri, Range: lsp.Range{End: lsp.Position{Character: end}}, SelectionRange: lsp.Range{End: lsp.Position{Character: 1}}}})
		}
		return json.RawMessage(`[]`), nil
	})
	acq, err := acquisition.Acquire(context.Background(), client, ar)
	if err != nil {
		t.Fatal(err)
	}
	v2raw, err := graphprovenance.CaptureV2(context.Background(), acq, root)
	if err != nil {
		t.Fatal(err)
	}
	var v2 graphprovenance.EvidenceV2
	if err = json.Unmarshal(v2raw, &v2); err != nil {
		t.Fatal(err)
	}
	n := graph.NewNode(graph.Item{Name: "A", Kind: 12, URI: uri, Range: graph.Range{End: graph.Position{Character: end}}, SelectionRange: graph.Range{End: graph.Position{Character: 1}}})
	unbound := graph.NewNode(graph.Item{Name: "Unbound", Kind: 12, URI: uri, Range: graph.Range{End: graph.Position{Character: end}}, SelectionRange: graph.Range{End: graph.Position{Character: 1}}})
	seed := graph.InvocationSeed{Label: "seed", At: "a.go:1:1", ResolvedURI: uri, ContentSHA256: digest(content), LanguageID: "go"}
	g := graph.Result{SchemaVersion: graph.SchemaVersionV5, Invocation: graph.Invocation{Server: graph.ServerInvocation{Command: "fake-lsp"}, Seeds: []graph.InvocationSeed{seed}, Provenance: graph.InvocationProvenance{InvocationID: "session", SourceRevision: "commit", ServerVersion: "fake@1"}, Expansion: graph.ExpansionConfig{TopmostSiblings: true}}, Nodes: []graph.Node{n, unbound}, Seeds: []graph.SeedResult{{Label: "seed", ReachedNodeIDs: []string{n.ID}}}, Summary: graph.Summary{NodeCount: 2, Complete: true}, Capabilities: graph.Capabilities{CallHierarchyProvider: true}}
	origin := graph.NewNode(graph.Item{Name: "Origin", Kind: 12, URI: uri, Range: graph.Range{Start: graph.Position{Line: 1}, End: graph.Position{Line: 1, Character: 1}}, SelectionRange: graph.Range{Start: graph.Position{Line: 1}, End: graph.Position{Line: 1, Character: 1}}})
	declaration := n
	g.SiblingCandidates = []graph.SiblingCandidate{{SeedURI: uri, SeedLabel: "seed", SeedLabels: []string{"seed"}, SeedIdentity: "session:seed:a.go:1:1", Origin: origin, Declaration: &declaration, Candidate: n, Direction: "SIBLING", Kind: "TOPMOST_SIBLING", ProviderEvidence: []string{"command=fake-lsp;server_version=fake@1;invocation=session"}, LSPEvidence: []string{"textDocument/documentSymbol", "textDocument/prepareCallHierarchy"}, SourceDigests: []string{"candidate=" + seed.ContentSHA256, "origin=" + seed.ContentSHA256}, Custody: graph.SourceCustodyEvidence{Class: graph.SourceCustodyCallerAssertedLocal, SourceContentSHA256: seed.ContentSHA256, ClaimCeiling: "NO_AUTHENTICATED_ANALYZED_SOURCE_IDENTITY"}}}
	native, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := graphprovenance.CaptureV5(native, "session", 1, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable, Records: []manageddiagnostic.Record{}}, &v2)
	if err != nil {
		t.Fatal(err)
	}
	var id struct {
		GraphV5 string `json:"graph_v5"`
	}
	_ = json.Unmarshal(artifact, &id)
	var gn struct {
		ExecutionBundleID string `json:"execution_bundle_id"`
	}
	_ = json.Unmarshal(native, &gn)
	q := Request{Artifact: Artifact{Bytes: artifact, Descriptor: &ArtifactDescriptor{AdmissionStatus: "CUSTODY_ADMITTED", ExactSHA256: digest(artifact), InspectionID: gn.ExecutionBundleID}}, ExpectedArtifactSHA256: digest(artifact), InspectionID: gn.ExecutionBundleID, SeedLabel: "seed", NodeID: n.ID, ExpectedURI: uri, RangeMode: Exact, PositionEncoding: encoding, PositionConvention: "LSP_ZERO_BASED_END_EXCLUSIVE", ExpectedRange: Range{End: Position{Character: int(end)}}, ExpectedPassageSHA256: digest(content)}
	snapshot, err := v5sourcesnapshot.Build(artifact, root, encoding)
	if err != nil {
		t.Fatal(err)
	}
	return artifact, snapshot, q, unbound.ID
}
func TestV5RetainedPassageEncodingsAndCarriers(t *testing.T) {
	for _, encoding := range []string{"utf-8", "utf-16", "utf-32"} {
		t.Run(encoding, func(t *testing.T) {
			artifact, snapshot, q, unboundID := retainedV5(t, encoding)
			for _, carrier := range []struct {
				name       string
				bytes      []byte
				descriptor *ArtifactDescriptor
			}{{"graph-provenance-v5", artifact, q.Artifact.Descriptor}, {"source-snapshot", snapshot, nil}} {
				t.Run(carrier.name, func(t *testing.T) {
					x := q
					x.Artifact = Artifact{Bytes: carrier.bytes, Descriptor: carrier.descriptor}
					x.ExpectedArtifactSHA256 = digest(carrier.bytes)
					got := Verify(x)
					if got.Overall != Verified || got.Checks.SourceReceipt != Verified || got.Checks.SourceDigest != Verified || got.Checks.PassageDigest != Verified || got.Checks.BodyCompleteness != NotEvaluated {
						t.Fatalf("%+v", got)
					}
					if carrier.descriptor == nil && got.Checks.SelectorCustody != SelectorCustodyUnavailable {
						t.Fatalf("direct artifact claimed selector custody: %+v", got)
					}
					missing := x
					missing.NodeID = "absent-despite-supply"
					if got = Verify(missing); got.Overall != NodeAbsent || got.Checks.NodeIdentity != NodeAbsent {
						t.Fatalf("absent node despite supply: %+v", got)
					}
					unbound := x
					unbound.NodeID = unboundID
					if got = Verify(unbound); got.Overall != NodeNotInSeed || got.Checks.Membership != NodeNotInSeed || got.Checks.PassageBytes != NotEvaluated {
						t.Fatalf("unbound node selected retained bytes: %+v", got)
					}
					x.ExpectedPassageSHA256 = digest([]byte("bad"))
					if got = Verify(x); got.Overall != PassageDigestMismatch {
						t.Fatalf("%+v", got)
					}
				})
			}
		})
	}
}
func TestRetainedCarrierMutationsFailClosed(t *testing.T) {
	_, snapshot, q, _ := retainedV5(t, "utf-16")
	var base map[string]any
	if err := json.Unmarshal(snapshot, &base); err != nil {
		t.Fatal(err)
	}
	mutations := []struct {
		name string
		edit func(map[string]any)
	}{
		{"parent-digest", func(m map[string]any) { m["graph_v5_digest"] = "sha256:forged" }},
		{"receipt-content-digest", func(m map[string]any) {
			m["receipts"].([]any)[0].(map[string]any)["content_digest"] = "sha256:forged"
		}},
		{"binding-node", func(m map[string]any) {
			m["bindings"].([]any)[0].(map[string]any)["node_id"] = "sha256:forged"
		}},
	}
	for _, tt := range mutations {
		t.Run(tt.name, func(t *testing.T) {
			var m map[string]any
			raw, _ := json.Marshal(base)
			_ = json.Unmarshal(raw, &m)
			tt.edit(m)
			mutated, _ := json.Marshal(m)
			x := q
			x.Artifact = Artifact{Bytes: mutated}
			x.ExpectedArtifactSHA256 = digest(mutated)
			got := Verify(x)
			if got.Overall != ArtifactInvalid || got.Checks.ArtifactAdmission != ArtifactInvalid || got.Checks.BodyCompleteness != NotEvaluated {
				t.Fatalf("mutation admitted: %+v", got)
			}
			if got.Diagnostic != "ARTIFACT_ADMISSION" {
				t.Fatalf("non-stable diagnostic: %q", got.Diagnostic)
			}
		})
	}

	direct := q
	direct.Artifact.Descriptor = nil
	if got := Verify(direct); got.Overall != Verified || got.Checks.SelectorCustody != SelectorCustodyUnavailable {
		t.Fatalf("direct bytes confused artifact and selector custody: %+v", got)
	}
	mismatched := q
	mismatched.Artifact.Descriptor = &ArtifactDescriptor{AdmissionStatus: "CUSTODY_ADMITTED", ExactSHA256: "sha256:forged", InspectionID: q.InspectionID}
	if got := Verify(mismatched); got.Overall != Verified || got.Checks.SelectorCustody != SelectorCustodyUnavailable {
		t.Fatalf("mismatched descriptor established custody: %+v", got)
	}
}

func TestDeterministicBatchAndBounds(t *testing.T) {
	_, q := nativeFixture(t, graph.SchemaVersionV3)
	a := VerifyBatch(BatchRequest{Records: []Request{q, q}})
	b := VerifyBatch(BatchRequest{Records: []Request{q, q}})
	if !reflect.DeepEqual(a, b) || a.Results[0].Index != 0 || a.Results[1].Index != 1 {
		t.Fatal("nondeterministic")
	}
	many := make([]Request, MaxBatchRecords+1)
	got := VerifyBatch(BatchRequest{Records: many})
	if len(got.Results) != len(many) || got.Results[0].Overall != LimitExceeded {
		t.Fatal("silent truncation")
	}
}
