package retainedcalls

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
)

func TestEmptyGraphExportHasArrayEndpoints(t *testing.T) {
	for _, complete := range []bool{true, false} {
		t.Run(map[bool]string{true: "empty", false: "partial"}[complete], func(t *testing.T) {
			root := t.TempDir()
			file := filepath.Join(root, "empty.go")
			if err := os.WriteFile(file, []byte("package empty\n"), 0600); err != nil {
				t.Fatal(err)
			}
			uri := (&url.URL{Scheme: "file", Path: file}).String()
			r := graph.Result{SchemaVersion: graph.SchemaVersionV3, Summary: graph.Summary{Complete: complete}, Capabilities: graph.Capabilities{CallHierarchyProvider: true}}
			r.Invocation.Target = graph.Target{URI: uri}
			r.Invocation.Seeds = []graph.InvocationSeed{{Label: "start", At: uri + ":0:0", ResolvedURI: uri}}
			r.Seeds = []graph.SeedResult{{Label: "start", Requested: r.Invocation.Target}}
			r.Slice = &graph.SliceEvidence{StartMode: "at", SourceURI: uri, DownDepth: 1, UpDepth: 1}
			raw, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			input, err := graphprovenance.Capture(context.Background(), raw, root, uri, "session", 1, nil)
			if err != nil {
				t.Fatalf("fixture admission: %v", err)
			}
			if _, err = graphprovenance.ValidateFor(input, graphprovenance.Family, "v1"); err != nil {
				t.Fatalf("fixture validation: %v", err)
			}
			out, err := Export(input)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = ValidateFor(out, Family, "v1"); err != nil {
				t.Fatalf("ASSERT_EMPTY_EXPORT_VALID: %v", err)
			}
			var e Evidence
			if err = json.Unmarshal(out, &e); err != nil {
				t.Fatal(err)
			}
			if e.Tables.Endpoints == nil || len(e.Tables.Endpoints) != 0 || len(e.Tables.Groups) != 0 || len(e.Tables.Occurrences) != 0 || e.Tables.SupportTotal != 0 {
				t.Fatal("ASSERT_EMPTY_TABLES")
			}
			if !bytes.Equal(input, e.InputBytes) {
				t.Fatal("historical input changed")
			}
			projection, err := Reconstruct(e.Tables)
			if err != nil || len(projection.Endpoints) != 0 || len(projection.Edges) != 0 {
				t.Fatalf("empty reconstruction: %v", err)
			}
		})
	}
}
