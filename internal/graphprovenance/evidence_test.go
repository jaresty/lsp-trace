package graphprovenance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/schema"
	"lsp-trace/sessionruntime"
)

func fixture(t *testing.T, extra ...string) (string, []byte, string) {
	t.Helper()
	root := t.TempDir()
	uris := []string{}
	for i := 0; i < 6; i++ {
		file := filepath.Join(root, fmt.Sprintf("f%d.go", i))
		if err := os.WriteFile(file, []byte(fmt.Sprintf("package p\nfunc F%d() {}\n", i)), 0600); err != nil {
			t.Fatal(err)
		}
		uris = append(uris, (&url.URL{Scheme: "file", Path: filepath.ToSlash(file)}).String())
	}
	for _, uri := range extra {
		if strings.HasPrefix(uri, "relative:") {
			file := filepath.Join(root, strings.TrimPrefix(uri, "relative:"))
			if err := os.WriteFile(file, []byte("package p\n"), 0600); err != nil {
				t.Fatal(err)
			}
			uri = (&url.URL{Scheme: "file", Path: filepath.ToSlash(file)}).String()
		}
		uris = append(uris, uri)
	}
	r := graph.Result{SchemaVersion: graph.SchemaVersionV3, Summary: graph.Summary{Complete: true}, Capabilities: graph.Capabilities{CallHierarchyProvider: true}}
	for i, uri := range uris {
		n := graph.NewNode(graph.Item{Name: fmt.Sprintf("F%d", i), Kind: 12, URI: uri, Range: graph.Range{End: graph.Position{Character: 1}}, SelectionRange: graph.Range{End: graph.Position{Character: 1}}})
		n.Data = json.RawMessage(`{"uri":"file:///opaque/not-a-source.go","node_id":"not-a-node"}`)
		r.Nodes = append(r.Nodes, n)
		if i > 0 {
			r.Edges = graph.MergeEdge(r.Edges, graph.Edge{CallerNodeID: r.Nodes[0].ID, CalleeNodeID: n.ID, CallSites: []graph.Range{{End: graph.Position{Character: 1}}}})
		}
	}
	ids := []string{}
	for _, n := range r.Nodes {
		ids = append(ids, n.ID)
	}
	r.Invocation.Target = graph.Target{URI: uris[0]}
	r.Invocation.Seeds = []graph.InvocationSeed{{Label: "start", At: uris[0] + ":0:0", ResolvedURI: uris[0]}}
	r.Seeds = []graph.SeedResult{{Label: "start", Requested: r.Invocation.Target, PreparedTargetIDs: []string{ids[0]}, ReachedNodeIDs: append([]string(nil), ids...), ReachedEdges: r.Edges}}
	relations := []string{}
	for _, e := range r.Edges {
		relations = append(relations, e.RelationID)
	}
	r.Seeds[0].ReachedRelationIDs = relations
	r.Slice = &graph.SliceEvidence{StartMode: "at", SourceURI: uris[0], DownDepth: 1, UpDepth: 1, StartingNodeIDs: []string{ids[0]}, Layers: []graph.SliceLayer{{Depth: 0, NodeIDs: ids[:1]}, {Depth: 1, NodeIDs: ids[1:]}}, FrontierNodeIDs: ids[1:], UpwardStartNodeIDs: ids[1:], OutgoingRelationIDs: relations, TraversalComplete: true}
	r.Diagnostics = []graph.Diagnostic{{Phase: "test", NodeID: ids[1], Message: "attributed"}, {Phase: "server-stderr", Message: "file:///never/read/from/a/message.go"}}
	r.Canonicalize()
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n')
	return root, raw, uris[0]
}
func captured(t *testing.T, root string, raw []byte, seed string, s *sessionruntime.DocumentSupply) Evidence {
	t.Helper()
	out, err := Capture(context.Background(), raw, root, seed, "s", 1, s)
	if err != nil {
		t.Fatal(err)
	}
	var e Evidence
	if err = json.Unmarshal(out, &e); err != nil {
		t.Fatal(err)
	}
	return e
}
func TestGraphCensusFiveNonseedAndOfflineAfterRemoval(t *testing.T) {
	root, raw, seed := fixture(t)
	e := captured(t, root, raw, seed, nil)
	if len(e.Captures) != 6 || !bytes.Equal(e.GraphBytes, raw) || e.SupplyStatus != "NO_NOTIFICATION_OBSERVATION" {
		t.Fatalf("ASSERT_FIVE_NONSEED_EXACT_GRAPH: %+v", e)
	}
	for _, r := range e.Captures {
		if r.Status != "READABLE" || r.Classification != PostTraversal || r.AnalyzedVersion != Unverified {
			t.Fatalf("ASSERT_POST_CAPTURE_UNVERIFIED: %+v", r)
		}
	}
	foundCall, foundDiagnostic, foundSeed, foundNonSource := false, false, false, false
	for _, b := range e.Bindings {
		if strings.Contains(b.Pointer, "/call_sites/") {
			foundCall = true
			if b.URI != seed {
				t.Fatal("ASSERT_CALLSITE_CALLER_OWNS_RANGE")
			}
		}
		if strings.HasPrefix(b.Pointer, "/seeds/") {
			foundSeed = true
		}
		if strings.HasPrefix(b.Pointer, "/diagnostics/") {
			if b.Attribution == "SOURCE" {
				foundDiagnostic = true
			} else {
				foundNonSource = true
				if len(b.ReceiptIDs) != 0 {
					t.Fatal("ASSERT_NON_SOURCE_DIAGNOSTIC")
				}
			}
		}
	}
	if !foundCall || !foundSeed || !foundDiagnostic || !foundNonSource {
		t.Fatal("ASSERT_MANDATORY_BINDING_FAMILIES")
	}
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(e)
	if _, err := ValidateFor(encoded, Family, "v1"); err != nil {
		t.Fatalf("ASSERT_OFFLINE_WITHOUT_CHECKOUT: %v", err)
	}
	if _, err := schema.ValidateFor(encoded, Family, "v1"); err == nil {
		t.Fatal("ASSERT_SHAPE_ONLY_CANNOT_ADMIT")
	}
}
func TestGraphSupplyAndLaterBytesRemainSeparate(t *testing.T) {
	root, raw, seed := fixture(t)
	a := []byte("package p\n// supplied A\n")
	params, _ := json.Marshal(map[string]any{"textDocument": map[string]any{"uri": seed, "languageId": "go", "version": 1, "text": string(a)}})
	s := &sessionruntime.DocumentSupply{Classification: Supplied, SessionID: "s", Generation: 1, URI: seed, DocumentVersion: 1, Method: "textDocument/didOpen", Content: a, Params: params}
	e := captured(t, root, raw, seed, s)
	if e.Supply == nil || !bytes.Equal(e.Supply.Content, a) || bytes.Equal(e.Captures[0].Content, a) {
		t.Fatal("ASSERT_SAME_URI_DISTINCT_OBSERVATIONS")
	}
	for _, b := range e.Bindings {
		if b.URI == seed && len(b.ReceiptIDs) != 2 {
			t.Fatal("ASSERT_TWO_RECEIPT_FOREIGN_KEYS")
		}
	}
}
func TestGraphProvenanceAdversarialOfflineConsistency(t *testing.T) {
	root, raw, seed := fixture(t)
	base := captured(t, root, raw, seed, nil)
	for _, tc := range []struct {
		name   string
		mutate func(*Evidence)
	}{
		{"remove-nonseed-receipt", func(e *Evidence) { e.Captures = e.Captures[:len(e.Captures)-1] }},
		{"remove-binding", func(e *Evidence) { e.Bindings = e.Bindings[1:] }},
		{"fake-census", func(e *Evidence) { e.Bindings = nil; e.Captures = nil }},
		{"changed-graph-bytes", func(e *Evidence) { e.GraphBytes = append(e.GraphBytes, ' ') }},
		{"changed-content", func(e *Evidence) { e.Captures[0].Content = []byte("different") }},
		{"changed-receipt", func(e *Evidence) { e.Captures[0].CanonicalReceipt = []byte("{}") }},
		{"reclassify-post", func(e *Evidence) {
			e.Captures[0].Classification = Supplied
			e.Captures[0] = seal(e.Captures[0])
			bind(e)
		}},
		{"fabricated-supply", func(e *Evidence) {
			r := e.Captures[0]
			r.Classification = Supplied
			r = seal(r)
			e.Supply = &r
			e.SupplyStatus = "OBSERVED_NOTIFICATION"
			bind(e)
		}},
		{"upgrade-version", func(e *Evidence) { e.AnalyzedVersion = "VERIFIED" }},
		{"upgrade-completeness", func(e *Evidence) { e.DependencyCompleteness = "COMPLETE" }},
		{"escape-path", func(e *Evidence) { e.Captures[0].Path = "../outside"; e.Captures[0] = seal(e.Captures[0]); bind(e) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			encoded, _ := json.Marshal(base)
			var e Evidence
			_ = json.Unmarshal(encoded, &e)
			tc.mutate(&e)
			encoded, _ = json.Marshal(e)
			if _, err := ValidateFor(encoded, Family, "v1"); err == nil {
				t.Fatalf("ASSERT_MUTATION_REJECTED: %s", tc.name)
			}
		})
	}
	encoded, _ := json.Marshal(base)
	for _, bad := range [][]byte{bytes.Replace(encoded, []byte(`"schema_version":`), []byte(`"Schema_version":`), 1), append([]byte(`{"schema_version":"x",`), encoded[1:]...), bytes.Replace(encoded, []byte(`"generation":1`), []byte(`"generation":1,"\u0067eneration":1`), 1)} {
		if _, err := ValidateFor(bad, Family, "v1"); err == nil {
			t.Fatal("ASSERT_DUPLICATE_CASE_REJECTED")
		}
	}
}
func TestGraphCensusRejectsURIAmplificationWithoutOmission(t *testing.T) {
	extra := []string{}
	for i := 0; i < 100; i++ {
		extra = append(extra, "file:///outside/"+strings.Repeat("x", 16000)+fmt.Sprintf("/%d.go", i))
	}
	root, raw, seed := fixture(t, extra...)
	if len(raw) > MaxGraphBytes {
		t.Fatal("fixture exceeds graph budget instead of census budget")
	}
	out, err := Capture(context.Background(), raw, root, seed, "s", 1, nil)
	if err == nil || !strings.Contains(err.Error(), "census budget") || out != nil {
		t.Fatalf("ASSERT_CENSUS_AMPLIFICATION_REJECTED: %v", err)
	}
}

func TestGraphCaptureFileBudgetIsExplicit(t *testing.T) {
	extra := []string{}
	for i := 0; i < 64; i++ {
		extra = append(extra, fmt.Sprintf("relative:more%d.go", i))
	}
	root, raw, seed := fixture(t, extra...)
	e := captured(t, root, raw, seed, nil)
	readable, limited := 0, 0
	for _, r := range e.Captures {
		if r.Status == "READABLE" {
			readable++
		}
		if r.Status == "BUDGET_EXCEEDED" {
			limited++
		}
	}
	if readable != 64 || limited != 6 {
		t.Fatalf("ASSERT_FILE_BUDGET_CENSUS: %d %d", readable, limited)
	}
}

func TestGraphCaptureTotalBudgetIsExplicit(t *testing.T) {
	root, raw, seed := fixture(t)
	for i := 0; i < 6; i++ {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("f%d.go", i)), bytes.Repeat([]byte{'x'}, MaxFileBytes), 0600); err != nil {
			t.Fatal(err)
		}
	}
	e := captured(t, root, raw, seed, nil)
	readable, limited := 0, 0
	for _, r := range e.Captures {
		if r.Status == "READABLE" {
			readable++
		} else if r.Status == "BUDGET_EXCEEDED" {
			limited++
			if r.Content != nil {
				t.Fatal("ASSERT_BUDGET_NO_BYTES")
			}
		}
	}
	if readable != 4 || limited != 2 {
		t.Fatalf("ASSERT_TOTAL_BUDGET_EXPLICIT_CENSUS: readable=%d limited=%d", readable, limited)
	}
}

func TestGraphCaptureUnavailableSourcesAndBudgets(t *testing.T) {
	root, raw, seed := fixture(t, "untitled:buffer", "file:///outside/not-readable.go", "file:///bad/../alias.go")
	if err := os.Remove(filepath.Join(root, "f1.go")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "f2.go"), bytes.Repeat([]byte{'x'}, MaxFileBytes+1), 0600); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "private.go")
	if err := os.WriteFile(outside, []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "f3.go")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "f3.go")); err != nil {
		t.Fatal(err)
	}
	e := captured(t, root, raw, seed, nil)
	seen := map[string]bool{}
	for _, r := range e.Captures {
		seen[r.Status] = true
		if r.Status != "READABLE" && r.Content != nil {
			t.Fatal("ASSERT_FAILED_NO_FAKE_BYTES")
		}
	}
	for _, status := range []string{"UNREADABLE", "BYTE_LIMIT_EXCEEDED", "VIRTUAL_URI", "OUTSIDE_SCOPE", "INVALID_URI"} {
		if !seen[status] {
			t.Fatalf("ASSERT_EXPLICIT_SOURCE_FAILURE: %s", status)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out, err := Capture(ctx, raw, root, seed, "s", 1, nil)
	if err != nil || !bytes.Contains(out, []byte(`"status":"CANCELLED"`)) {
		t.Fatalf("ASSERT_CANCELLED_EXPLICIT: %v", err)
	}
}
