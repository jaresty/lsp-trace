package v5sourcesnapshotv3

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/schema"
	"lsp-trace/internal/source"
	"lsp-trace/internal/v5sourcesnapshot"
	"lsp-trace/internal/v5sourcesnapshotv2"
)

func TestCaptureReplayOwnsAllLiveReadsAndIsWorkspaceIndependent(t *testing.T) {
	parent, workspace := fixture(t)
	var p v5sourcesnapshotv2.Artifact
	if err := json.Unmarshal(parent, &p); err != nil {
		t.Fatal(err)
	}
	var v1 v5sourcesnapshot.Artifact
	if err := json.Unmarshal(p.ParentSnapshot, &v1); err != nil {
		t.Fatal(err)
	}
	selections := make([]DisplaySelection, len(p.DisplayBindings))
	for i, b := range p.DisplayBindings {
		selections[i] = DisplaySelection{GraphSubjectID: b.GraphSubjectID, LogicalSourceID: b.LogicalSourceID, DisplayRange: b.DisplayRange}
	}
	got, err := Capture(CaptureInput{GraphV5Bytes: v1.GraphV5Bytes, Workspace: workspace, PositionEncoding: v1.PositionEncoding, Selections: selections, Limits: generousLimits()})
	if err != nil {
		t.Fatalf("ASSERT_V3_CAPTURE_VALID: %v", err)
	}
	raw1, lookup1, err := Replay(got)
	if err != nil {
		t.Fatalf("ASSERT_V3_REPLAY_VALID: %v", err)
	}
	if err := os.RemoveAll(workspace); err != nil {
		t.Fatal(err)
	}
	raw2, lookup2, err := Replay(got)
	if err != nil || !bytes.Equal(raw1, raw2) {
		t.Fatalf("ASSERT_V3_REPLAY_WORKSPACE_INDEPENDENT: %v", err)
	}
	for id := range lookup1.objects {
		a, ea := lookup1.Get(id)
		b, eb := lookup2.Get(id)
		if ea != nil || eb != nil || !bytes.Equal(a.Bytes, b.Bytes) {
			t.Fatal("ASSERT_V3_REPLAY_LOOKUP_IDENTICAL")
		}
		a.Bytes[0] ^= 1
		c, _ := lookup1.Get(id)
		if bytes.Equal(a.Bytes, c.Bytes) {
			t.Fatal("ASSERT_V3_REPLAY_DEFENSIVE_CLONE")
		}
	}
}

func TestCaptureReturnsSourceUnavailableRatherThanForgingDisplayBinding(t *testing.T) {
	parent, workspace := fixture(t)
	var p v5sourcesnapshotv2.Artifact
	_ = json.Unmarshal(parent, &p)
	var v1 v5sourcesnapshot.Artifact
	_ = json.Unmarshal(p.ParentSnapshot, &v1)
	_, err := Capture(CaptureInput{GraphV5Bytes: v1.GraphV5Bytes, Workspace: workspace, PositionEncoding: v1.PositionEncoding, Selections: []DisplaySelection{{GraphSubjectID: "absent", LogicalSourceID: "file:///absent.go"}}, Limits: generousLimits()})
	var unavailable *SourceUnavailableError
	if !errors.As(err, &unavailable) || unavailable.Endpoint != "TARGET" {
		t.Fatalf("ASSERT_V3_SOURCE_UNAVAILABLE_TYPED: %v", err)
	}
}

func TestBuildAndValidateExactOccurrencesAndAmbientIndependence(t *testing.T) {
	parent, workspace := fixture(t)
	raw, err := Build(parent, workspace, generousLimits())
	if err != nil {
		t.Fatalf("ASSERT_V3_BUILD_VALID: %v", err)
	}
	var got Artifact
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Bindings) != 4 {
		t.Fatalf("ASSERT_V3_OCCURRENCES_EXACT: got %d", len(got.Bindings))
	}
	if len(got.Receipts) != 2 {
		t.Fatalf("ASSERT_V3_ABSENT_CALLER_RETAINED: got %d", len(got.Receipts))
	}
	first := append([]byte(nil), raw...)
	if err := os.RemoveAll(workspace); err != nil {
		t.Fatal(err)
	}
	if version, err := Validate(raw, generousLimits()); err != nil || version != Version {
		t.Fatalf("ASSERT_V3_VALIDATE_AMBIENT_INDEPENDENT: version=%q err=%v", version, err)
	}
	if !bytes.Equal(first, raw) {
		t.Fatal("ASSERT_V3_DEFENSIVE_OWNERSHIP")
	}
}

func TestBuildDeterministicAcrossNativePermutations(t *testing.T) {
	parent, workspace := fixture(t)
	first, err := Build(parent, workspace, generousLimits())
	if err != nil {
		t.Fatal(err)
	}
	var p v5sourcesnapshotv2.Artifact
	if err := json.Unmarshal(parent, &p); err != nil {
		t.Fatal(err)
	}
	var v1 v5sourcesnapshot.Artifact
	if err := json.Unmarshal(p.ParentSnapshot, &v1); err != nil {
		t.Fatal(err)
	}
	var v5 graphprovenance.EvidenceV5
	if err := json.Unmarshal(v1.GraphV5Bytes, &v5); err != nil {
		t.Fatal(err)
	}
	var native graph.Result
	nativeRaw := mustDecodeGraph(t, v5.GraphV5)
	if err := json.Unmarshal(nativeRaw, &native); err != nil {
		t.Fatal(err)
	}
	original, err := occurrences(native, generousLimits())
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(native.Nodes, func(i, j int) bool { return native.Nodes[i].ID > native.Nodes[j].ID })
	sort.Slice(native.Edges, func(i, j int) bool { return native.Edges[i].CallerNodeID > native.Edges[j].CallerNodeID })
	for i := range native.Edges {
		sort.Slice(native.Edges[i].CallSites, func(a, b int) bool {
			return native.Edges[i].CallSites[a].Start.Line > native.Edges[i].CallSites[b].Start.Line
		})
	}
	var a Artifact
	json.Unmarshal(first, &a)
	permuted, err := occurrences(native, generousLimits())
	if err != nil {
		t.Fatal(err)
	}
	left, err := makeBindings(original, a.Receipts, a.ParentSnapshotDigest, a.NativeGraphDigest, generousLimits())
	if err != nil {
		t.Fatal(err)
	}
	right, err := makeBindings(permuted, a.Receipts, a.ParentSnapshotDigest, a.NativeGraphDigest, generousLimits())
	if err != nil {
		t.Fatal(err)
	}
	if !equalBindingShape(left, right) {
		t.Fatal("ASSERT_V3_DETERMINISTIC_PERMUTATIONS")
	}
}

func TestValidateRejectsExactReceiptSet(t *testing.T) {
	parent, workspace := fixture(t)
	raw, err := Build(parent, workspace, generousLimits())
	if err != nil {
		t.Fatal(err)
	}
	var base Artifact
	if err := json.Unmarshal(raw, &base); err != nil {
		t.Fatal(err)
	}
	extraURI := (&url.URL{Scheme: "file", Path: filepath.Join(workspace, "z-extra.go")}).String()
	_, retained, canonical, err := source.CanonicalizeReceipt(source.DiscoveredItem{ID: extraURI, Locator: extraURI}, source.Acquisition{Status: source.Readable, Provenance: source.Provenance{Mechanism: "bounded-workspace-file", Locator: extraURI}}, []byte("package a\n"))
	if err != nil {
		t.Fatal(err)
	}
	extra := Receipt{ID: tdigest(canonical), URI: extraURI, ContentDigest: tdigest(retained), Content: retained, CanonicalReceipt: canonical}
	cases := []struct {
		name  string
		apply func(*Artifact)
	}{
		{"extra-unbound", func(a *Artifact) { a.Receipts = append(a.Receipts, extra) }},
		{"missing-required", func(a *Artifact) { a.Receipts = a.Receipts[:len(a.Receipts)-1] }},
		{"duplicate-caller-uri", func(a *Artifact) { a.Receipts = append(a.Receipts, a.Receipts[len(a.Receipts)-1]) }},
		{"reordered", func(a *Artifact) { a.Receipts[0], a.Receipts[1] = a.Receipts[1], a.Receipts[0] }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := clone(t, base)
			tc.apply(&a)
			if _, err := Validate(mustJSON(t, a), generousLimits()); err == nil {
				t.Fatalf("ASSERT_V3_RECEIPT_SET_%s_REJECTED", tc.name)
			}
		})
	}
}

func TestValidateRejectsMutationTable(t *testing.T) {
	parent, workspace := fixture(t)
	raw, err := Build(parent, workspace, generousLimits())
	if err != nil {
		t.Fatal(err)
	}
	var base Artifact
	json.Unmarshal(raw, &base)
	mutations := []struct {
		name  string
		apply func(*Artifact)
	}{
		{"schema", func(a *Artifact) { a.SchemaVersion = "bad" }}, {"policy", func(a *Artifact) { a.Policy = "bad" }},
		{"parent", func(a *Artifact) { a.ParentSnapshotDigest = tdigest([]byte("bad")) }}, {"native", func(a *Artifact) { a.NativeGraphDigest = tdigest([]byte("bad")) }},
		{"relation", func(a *Artifact) { a.Bindings[0].RelationID = "bad" }}, {"direction", func(a *Artifact) { a.Bindings[0].Direction = "CALLEE_TO_CALLER" }},
		{"range", func(a *Artifact) { a.Bindings[0].Range.End.Line = a.Bindings[0].Range.Start.Line - 1 }}, {"occurrence", func(a *Artifact) { a.Bindings[0].OccurrenceID = tdigest([]byte("bad")) }},
		{"uri", func(a *Artifact) { a.Bindings[0].CallerURI = "file:///bad" }}, {"source", func(a *Artifact) { a.Bindings[0].SourceDigest = tdigest([]byte("bad")) }},
		{"provenance", func(a *Artifact) { a.Bindings[0].Provenance = "TEXT" }}, {"status", func(a *Artifact) { a.Bindings[0].Status = "LIVE" }},
		{"custody", func(a *Artifact) { a.Bindings[0].Custody = "LIVE" }}, {"authority", func(a *Artifact) { a.Bindings[0].Authority = 1 }},
		{"accepted", func(a *Artifact) { a.Bindings[0].Accepted = true }}, {"completeness", func(a *Artifact) { a.Bindings[0].Completeness = "COMPLETE" }},
		{"receipt", func(a *Artifact) { a.Receipts[0].Content[0] ^= 1 }}, {"missing-binding", func(a *Artifact) { a.Bindings = a.Bindings[1:] }},
		{"extra-binding", func(a *Artifact) { a.Bindings = append(a.Bindings, a.Bindings[0]) }}, {"reorder", func(a *Artifact) { a.Bindings[0], a.Bindings[1] = a.Bindings[1], a.Bindings[0] }},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			c := clone(t, base)
			tc.apply(&c)
			if _, err := Validate(mustJSON(t, c), generousLimits()); err == nil {
				t.Fatalf("ASSERT_V3_MUTATION_REJECTED_%s", tc.name)
			}
		})
	}
	unknown := bytes.Replace(raw, []byte(`{"schema_version":`), []byte(`{"unknown":true,"schema_version":`), 1)
	if _, err := Validate(unknown, generousLimits()); err == nil {
		t.Fatal("ASSERT_V3_UNKNOWN_REJECTED")
	}
	dup := bytes.Replace(raw, []byte(`{"schema_version":`), []byte(`{"schema_version":"x","schema_version":`), 1)
	if _, err := Validate(dup, generousLimits()); err == nil {
		t.Fatal("ASSERT_V3_DUPLICATE_REJECTED")
	}
	if _, err := Validate(append(raw, []byte(`{}`)...), generousLimits()); err == nil {
		t.Fatal("ASSERT_V3_TRAILING_REJECTED")
	}
}

func TestLimitsAndAcquisitionFailures(t *testing.T) {
	parent, workspace := fixture(t)
	raw, err := Build(parent, workspace, generousLimits())
	if err != nil {
		t.Fatal(err)
	}
	var a Artifact
	json.Unmarshal(raw, &a)
	equalities := Limits{MaxArtifactBytes: len(raw), MaxParentBytes: len(parent), MaxReceipts: len(a.Receipts), MaxSourceBytes: maxSource(a.Receipts), MaxTotalSourceBytes: totalSource(a.Receipts), MaxBindings: len(a.Bindings), MaxWork: len(a.Bindings) + 2*len(a.Receipts)}
	if _, err := Validate(raw, equalities); err != nil {
		t.Fatalf("ASSERT_V3_LIMIT_EQUALITY: %v", err)
	}
	checks := []func(*Limits){func(l *Limits) { l.MaxArtifactBytes-- }, func(l *Limits) { l.MaxParentBytes-- }, func(l *Limits) { l.MaxReceipts-- }, func(l *Limits) { l.MaxSourceBytes-- }, func(l *Limits) { l.MaxTotalSourceBytes-- }, func(l *Limits) { l.MaxBindings-- }, func(l *Limits) { l.MaxWork-- }}
	for i, shrink := range checks {
		l := equalities
		shrink(&l)
		if _, err := Validate(raw, l); err == nil {
			t.Fatalf("ASSERT_V3_LIMIT_PLUS_ONE_%d", i)
		}
	}
	zero := generousLimits()
	zero.MaxWork = 0
	if _, err := Validate(raw, zero); err == nil {
		t.Fatal("ASSERT_V3_LIMIT_POSITIVE")
	}
	if err := os.Remove(filepath.Join(workspace, "caller2.go")); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(parent, workspace, generousLimits()); err == nil {
		t.Fatal("ASSERT_V3_UNREADABLE_REJECTED")
	}
	if _, err := Build(parent, filepath.Join(workspace, "."), generousLimits()); err == nil {
		t.Fatal("ASSERT_V3_NONCANONICAL_WORKSPACE_REJECTED")
	}
}

func TestSchemaV3Registered(t *testing.T) {
	raw, err := schema.BytesFor(schema.FamilyGraphV5SourceSnapshot, "v3")
	if err != nil || !bytes.Contains(raw, []byte(Version)) {
		t.Fatalf("ASSERT_V3_SCHEMA_REGISTERED: %v", err)
	}
}

func generousLimits() Limits { return Limits{1 << 26, 1 << 25, 100, 1 << 20, 1 << 22, 100, 1000} }
func maxSource(r []Receipt) int {
	n := 0
	for _, x := range r {
		if len(x.Content) > n {
			n = len(x.Content)
		}
	}
	return n
}
func totalSource(r []Receipt) int {
	n := 0
	for _, x := range r {
		n += len(x.Content)
	}
	return n
}
func clone(t *testing.T, a Artifact) Artifact {
	var b Artifact
	json.Unmarshal(mustJSON(t, a), &b)
	return b
}
func equalBindingShape(a, b []Binding) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].OccurrenceID != b[i].OccurrenceID {
			return false
		}
	}
	return true
}

func fixture(t *testing.T) ([]byte, string) {
	t.Helper()
	root := t.TempDir()
	files := map[string][]byte{"callee.go": []byte("package a\nfunc C(){}\n"), "caller1.go": []byte("package a\nfunc A(){C();C()}\n"), "caller2.go": []byte("package a\nfunc B(){C()}\n")}
	for n, b := range files {
		if err := os.WriteFile(filepath.Join(root, n), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	uri := func(n string) string { return (&url.URL{Scheme: "file", Path: filepath.Join(root, n)}).String() }
	callee := graph.NewNode(graph.Item{Name: "C", Kind: 12, URI: uri("callee.go"), Range: graph.Range{End: graph.Position{Line: 1, Character: 10}}, SelectionRange: graph.Range{End: graph.Position{Line: 1, Character: 6}}})
	c1 := graph.NewNode(graph.Item{Name: "A", Kind: 12, URI: uri("caller1.go"), Range: graph.Range{End: graph.Position{Line: 1, Character: 17}}, SelectionRange: graph.Range{End: graph.Position{Line: 1, Character: 6}}})
	c2 := graph.NewNode(graph.Item{Name: "B", Kind: 12, URI: uri("caller2.go"), Range: graph.Range{End: graph.Position{Line: 1, Character: 14}}, SelectionRange: graph.Range{End: graph.Position{Line: 1, Character: 6}}})
	r := func(line, start, end uint32) graph.Range {
		return graph.Range{Start: graph.Position{Line: line, Character: start}, End: graph.Position{Line: line, Character: end}}
	}
	native := graph.Result{SchemaVersion: graph.SchemaVersionV5, Invocation: graph.Invocation{Server: graph.ServerInvocation{Command: "fake"}, Seeds: []graph.InvocationSeed{{Label: "seed", At: "callee.go:1:1", ResolvedURI: callee.URI, ContentSHA256: tdigest(files["callee.go"]), LanguageID: "go"}}, Provenance: graph.InvocationProvenance{InvocationID: "session", SourceRevision: "commit", ServerVersion: "fake@1"}}, Nodes: []graph.Node{callee, c1, c2}, Seeds: []graph.SeedResult{{Label: "seed", ReachedNodeIDs: []string{callee.ID}}}, Edges: []graph.Edge{{RelationID: "r1", CallerNodeID: c1.ID, CalleeNodeID: callee.ID, CallSites: []graph.Range{r(1, 9, 12), r(1, 13, 16)}}, {RelationID: "r2", CallerNodeID: c2.ID, CalleeNodeID: callee.ID, CallSites: []graph.Range{r(1, 9, 12)}}, {RelationID: "r3", CallerNodeID: c1.ID, CalleeNodeID: c1.ID, CallSites: []graph.Range{r(1, 2, 3)}}}, Summary: graph.Summary{NodeCount: 3, Complete: true}, Capabilities: graph.Capabilities{CallHierarchyProvider: true}}
	decl := callee
	origin := graph.NewNode(graph.Item{Name: "Origin", Kind: 12, URI: callee.URI, Range: graph.Range{End: graph.Position{Character: 1}}, SelectionRange: graph.Range{End: graph.Position{Character: 1}}})
	native.SiblingCandidates = []graph.SiblingCandidate{{SeedURI: callee.URI, SeedLabel: "seed", SeedLabels: []string{"seed"}, SeedIdentity: "session:seed:callee.go:1:1", Origin: origin, Declaration: &decl, Candidate: callee, Direction: "SIBLING", Kind: "TOPMOST_SIBLING", ProviderEvidence: []string{"command=fake;server_version=fake@1;invocation=session"}, LSPEvidence: []string{"textDocument/documentSymbol", "textDocument/prepareCallHierarchy"}, SourceDigests: []string{"candidate=" + tdigest(files["callee.go"]), "origin=" + tdigest(files["callee.go"])}, Custody: graph.SourceCustodyEvidence{Class: graph.SourceCustodyCallerAssertedLocal, SourceContentSHA256: tdigest(files["callee.go"]), ClaimCeiling: "NO_AUTHENTICATED_ANALYZED_SOURCE_IDENTITY"}}}
	nativeRaw := mustJSON(t, native)
	v5raw, err := graphprovenance.CaptureV5(nativeRaw, "session", 1, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable, Records: []manageddiagnostic.Record{}})
	if err != nil {
		t.Fatal(err)
	}
	// Minimal V1 Build needs no acquisition fixture beyond the native V5 and sibling source.
	return wrapV2(t, v5raw, root), root
}

func wrapV2(t *testing.T, v5raw []byte, root string) []byte {
	t.Helper()
	v1raw, err := v5sourcesnapshot.Build(v5raw, root, "utf-16")
	if err != nil {
		t.Fatal(err)
	}
	var v1 v5sourcesnapshot.Artifact
	if err := json.Unmarshal(v1raw, &v1); err != nil {
		t.Fatal(err)
	}
	b := v1.Bindings[0]
	display := v5sourcesnapshotv2.DisplayBinding{GraphSubjectID: b.NodeID, LogicalSourceID: b.URI, DisplayRange: b.Range, DisplayRangePolicy: v5sourcesnapshotv2.DisplayRangePolicy, Provenance: v5sourcesnapshotv2.Provenance{Kind: v5sourcesnapshotv2.ProvenanceKind, Method: v5sourcesnapshotv2.ProvenanceMethod}, ReceiptID: b.ReceiptIDs[0], SourceDigest: b.SourceDigest, PositionEncoding: v1.PositionEncoding, Status: v5sourcesnapshotv2.Status, Custody: v5sourcesnapshotv2.Custody}
	p := v5sourcesnapshotv2.Artifact{SchemaVersion: v5sourcesnapshotv2.Version, Policy: v5sourcesnapshotv2.Policy, ParentSchemaVersion: v5sourcesnapshot.Version, ParentSnapshotDigest: tdigest(v1raw), ParentSnapshot: v1raw, DisplayBindings: []v5sourcesnapshotv2.DisplayBinding{display}}
	parent := mustJSON(t, p)
	if _, err := v5sourcesnapshotv2.Validate(parent); err != nil {
		t.Fatal(err)
	}
	return parent
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func tdigest(b []byte) string  { s := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(s[:]) }
func encode64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }
func mustDecodeGraph(t *testing.T, s string) []byte {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
