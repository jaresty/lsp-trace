package v5sourcesnapshotv2

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/schema"
	"lsp-trace/internal/v5sourcesnapshot"
)

func TestValidateAcceptsRetainedFullDefinitionBindings(t *testing.T) {
	raw := validArtifact(t)
	if got, err := Validate(raw); err != nil || got != Version {
		t.Fatalf("ASSERT_V2_BASELINE_ACCEPTED: version=%q err=%v", got, err)
	}
	var artifact Artifact
	if err := json.Unmarshal(raw, &artifact); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(mustJSON(t, artifact.DisplayBindings), []byte(`"content"`)) {
		t.Fatal("ASSERT_V2_NO_MUTABLE_SOURCE_BYTES_DUPLICATED")
	}
}

func TestValidateRejectsStrictAdmissionMutations(t *testing.T) {
	base := validArtifactValue(t)
	mutations := []struct {
		name, want string
		apply      func(*Artifact)
	}{
		{"parent-schema", "V2_PARENT_IDENTITY_MISMATCH", func(a *Artifact) { a.ParentSchemaVersion = "lsp-trace.graph-v5-source-snapshot.v0" }},
		{"parent-digest-noncanonical", "V2_PARENT_DIGEST_INVALID", func(a *Artifact) { a.ParentSnapshotDigest = "SHA256:" + strings.Repeat("0", 64) }},
		{"parent-digest-mismatch", "V2_PARENT_DIGEST_MISMATCH", func(a *Artifact) { a.ParentSnapshotDigest = "sha256:" + strings.Repeat("0", 64) }},
		{"range-reversed", "V2_DISPLAY_RANGE_INVALID", func(a *Artifact) {
			a.DisplayBindings[0].DisplayRange = graph.Range{Start: graph.Position{Line: 2}, End: graph.Position{Line: 1}}
		}},
		{"encoding", "V2_POSITION_ENCODING_INVALID", func(a *Artifact) { a.DisplayBindings[0].PositionEncoding = "utf-7" }},
		{"missing-receipt", "V2_RECEIPT_BINDING_REQUIRED", func(a *Artifact) { a.DisplayBindings[0].ReceiptID = "" }},
		{"missing-source", "V2_SOURCE_BINDING_REQUIRED", func(a *Artifact) { a.DisplayBindings[0].SourceDigest = "" }},
		{"receipt-mismatch", "V2_RECEIPT_BINDING_MISMATCH", func(a *Artifact) { a.DisplayBindings[0].ReceiptID = "sha256:" + strings.Repeat("1", 64) }},
		{"source-mismatch", "V2_SOURCE_BINDING_MISMATCH", func(a *Artifact) { a.DisplayBindings[0].SourceDigest = "sha256:" + strings.Repeat("1", 64) }},
		{"policy", "V2_POLICY_MISMATCH", func(a *Artifact) { a.Policy = "SUBSTITUTED" }},
		{"display-policy", "V2_DISPLAY_POLICY_MISMATCH", func(a *Artifact) { a.DisplayBindings[0].DisplayRangePolicy = "SELECTION_RANGE" }},
		{"provenance-kind", "V2_PROVENANCE_MISMATCH", func(a *Artifact) { a.DisplayBindings[0].Provenance.Kind = "CALL_HIERARCHY_ITEM" }},
		{"provenance-method", "V2_PROVENANCE_MISMATCH", func(a *Artifact) { a.DisplayBindings[0].Provenance.Method = "callHierarchy" }},
		{"status", "V2_STATUS_MISMATCH", func(a *Artifact) { a.DisplayBindings[0].Status = "ACQUIRED" }},
		{"custody", "V2_CUSTODY_MISMATCH", func(a *Artifact) { a.DisplayBindings[0].Custody = "LIVE" }},
		{"duplicate-key", "V2_AMBIGUOUS_DISPLAY_BINDING", func(a *Artifact) { a.DisplayBindings = append(a.DisplayBindings, a.DisplayBindings[0]) }},
		{"ordering", "V2_BINDING_ORDER_INVALID", func(a *Artifact) {
			a.DisplayBindings[0], a.DisplayBindings[1] = a.DisplayBindings[1], a.DisplayBindings[0]
		}},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			candidate := cloneArtifact(t, base)
			mutation.apply(&candidate)
			assertRejected(t, mustJSON(t, candidate), mutation.want)
		})
	}
}

func TestValidateRejectsJSONBoundaryMutations(t *testing.T) {
	base := validArtifact(t)
	var object map[string]any
	if err := json.Unmarshal(base, &object); err != nil {
		t.Fatal(err)
	}
	object["unknown"] = true
	assertRejected(t, mustJSON(t, object), "V2_JSON_INVALID")
	duplicate := bytes.Replace(base, []byte(`{"schema_version":`), []byte(`{"schema_version":"duplicate","schema_version":`), 1)
	assertRejected(t, duplicate, "V2_JSON_DUPLICATE_MEMBER")
	assertRejected(t, append(append([]byte{}, base...), []byte(`{}`)...), "V2_JSON_TRAILING_VALUE")
}

func TestSchemaV2RegisteredInternally(t *testing.T) {
	raw, err := schema.BytesFor(schema.FamilyGraphV5SourceSnapshot, "v2")
	if err != nil {
		t.Fatalf("ASSERT_V2_SCHEMA_REGISTERED: %v", err)
	}
	if !bytes.Contains(raw, []byte(`"const":"`+Version+`"`)) {
		t.Fatalf("ASSERT_V2_SCHEMA_IDENTITY: %s", raw)
	}
}

func assertRejected(t *testing.T, raw []byte, want string) {
	t.Helper()
	if _, err := Validate(raw); err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("%s: err=%v", want, err)
	}
}

func validArtifact(t *testing.T) []byte { return mustJSON(t, validArtifactValue(t)) }

func validArtifactValue(t *testing.T) Artifact {
	t.Helper()
	parent := retainedV1(t)
	var snapshot v5sourcesnapshot.Artifact
	if err := json.Unmarshal(parent, &snapshot); err != nil {
		t.Fatal(err)
	}
	byNode := map[string]v5sourcesnapshot.Binding{}
	for _, binding := range snapshot.Bindings {
		if _, ok := byNode[binding.NodeID]; !ok {
			byNode[binding.NodeID] = binding
		}
	}
	ids := make([]string, 0, len(byNode))
	for id := range byNode {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	bindings := make([]DisplayBinding, 0, len(ids))
	for _, id := range ids {
		binding := byNode[id]
		bindings = append(bindings, DisplayBinding{GraphSubjectID: id, LogicalSourceID: binding.URI, DisplayRange: binding.Range, DisplayRangePolicy: DisplayRangePolicy, Provenance: Provenance{Kind: ProvenanceKind, Method: ProvenanceMethod}, ReceiptID: binding.ReceiptIDs[0], SourceDigest: binding.SourceDigest, PositionEncoding: snapshot.PositionEncoding, Status: Status, Custody: Custody})
	}
	return Artifact{SchemaVersion: Version, Policy: Policy, ParentSchemaVersion: v5sourcesnapshot.Version, ParentSnapshotDigest: testDigest(parent), ParentSnapshot: parent, DisplayBindings: bindings}
}

func retainedV1(t *testing.T) []byte {
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
	seedDigest := testDigest(content)
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
	return parent
}

func cloneArtifact(t *testing.T, in Artifact) Artifact {
	t.Helper()
	var out Artifact
	if err := json.Unmarshal(mustJSON(t, in), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func testDigest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}
