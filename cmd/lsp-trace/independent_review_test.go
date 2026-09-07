package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
)

// Independent review regression, made persistent without a /tmp fixture dependency.
func TestIndependentGraphProvenanceOutput(t *testing.T) {
	root := t.TempDir()
	uri := "file://" + filepath.ToSlash(filepath.Join(root, "f.go"))
	if err := os.WriteFile(filepath.Join(root, "f.go"), []byte("package p\n"), 0600); err != nil {
		t.Fatal(err)
	}
	target := graph.Target{URI: uri}
	g := graph.Result{SchemaVersion: graph.SchemaVersionV3,
		Invocation: graph.Invocation{Target: target, Seeds: []graph.InvocationSeed{{Label: "start", At: uri + ":0:0", ResolvedURI: uri}}},
		Seeds:      []graph.SeedResult{{Label: "start", Requested: target}},
		Slice:      &graph.SliceEvidence{StartMode: "at", SourceURI: uri, DownDepth: 1, UpDepth: 1},
	}
	raw, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = graphprovenance.Capture(context.Background(), raw, root, uri, "s", 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "out.json")
	if err := publishGraphProvenance(out, raw); err != nil {
		t.Fatalf("CLI_GRAPH_PROVENANCE_OUTPUT_FAILED: %v", err)
	}
	loaded, stage, err := loadCustodiedGeneration(out)
	if err != nil || !bytes.Equal(loaded, raw) {
		t.Fatalf("selected exact bytes/receipt: %s %v", stage, err)
	}
	if _, err := graphprovenance.ValidateFor(loaded, graphprovenance.Family, "v1"); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if runVerify([]string{"--family", "graph-provenance", "--version", "v1", out}, &stdout, &stderr) != 0 {
		t.Fatal(stderr.String())
	}
	if runVerify([]string{out}, &stdout, &stderr) == 0 {
		t.Fatal("historical verifier admitted envelope")
	}
	for _, args := range [][]string{{"--family", "graph-provenance", out}, {"--family", "graph-provenance", "--version", "v2", out}, {"--family", "arbitrary", out}} {
		if runVerify(args, &stdout, &stderr) == 0 {
			t.Fatal("unsupported verification admitted")
		}
	}
	if err := publishBundle(filepath.Join(t.TempDir(), "legacy.json"), raw); err == nil {
		t.Fatal("historical staged admission weakened")
	}
	before, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if err := publishGraphProvenance(out, raw); err == nil {
		t.Fatal("overwrite admitted")
	}
	after, err := os.ReadFile(out)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("selector replaced")
	}
	entries, err := os.ReadDir(filepath.Dir(out))
	if err != nil || len(entries) != 2 {
		t.Fatalf("failed overwrite leaked generation: %v %d", err, len(entries))
	}
	badPath := filepath.Join(t.TempDir(), "bad.json")
	if err := publishGraphProvenance(badPath, []byte(`{"schema_version":"lsp-trace.graph-provenance.v1"}`)); err == nil {
		t.Fatal("malformed admitted")
	}
	entries, err = os.ReadDir(filepath.Dir(badPath))
	if err != nil || len(entries) != 0 {
		t.Fatal("invalid publication left files")
	}
	selector, err := readGenerationSelector(out)
	if err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(filepath.Dir(out), selector.Generation, generationArtifactName)
	if err := os.WriteFile(artifact, append(raw, ' '), 0600); err != nil {
		t.Fatal(err)
	}
	if runVerify([]string{"--family", "graph-provenance", "--version", "v1", out}, &stdout, &stderr) == 0 {
		t.Fatal("changed exact bytes verified")
	}
	// A coherent byte receipt is necessary but never sufficient semantic admission.
	bad := []byte(`{"schema_version":"lsp-trace.graph-provenance.v1"}`)
	badReceipt, err := receiptBytes(bad)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifact, bad, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(artifact), generationReceiptName), badReceipt, 0600); err != nil {
		t.Fatal(err)
	}
	if runVerify([]string{"--family", "graph-provenance", "--version", "v1", out}, &stdout, &stderr) == 0 {
		t.Fatal("byte receipt bypassed semantic admission")
	}
}
