package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"lsp-trace/internal/graph"
)

func TestV3CapturesCompleteEffectiveInvocationAndAllSeedIdentities(t *testing.T) {
	workspace := t.TempDir()
	for _, name := range []string{"a.go", "b.go"} {
		if err := os.WriteFile(filepath.Join(workspace, name), []byte("package main\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	output := filepath.Join(workspace, "bundle.json")
	trace := filepath.Join(workspace, "trace.jsonl")
	args := []string{"incoming", "--workspace", workspace, "--server", "missing-server", "--server-arg", "--stdio", "--server-env", "TOKEN=declared", "--at", "a.go:1:1", "--at", "b.go:1:1", "--language-id", "go", "--max-depth", "7", "--max-nodes", "11", "--timeout", "13s", "--request-timeout", "17s", "--concurrency", "1", "--expand-topmost-siblings", "--expand-dispatch-family", "--trace-lsp", trace, "--output", output, "--provenance-source-revision", "caller-revision"}
	stdout, stderr, code := captureRun(t, args)
	if code == 0 || stdout != "" || !strings.Contains(stderr, "spawn") {
		t.Fatalf("ASSERT_P2_INVOCATION_FIXTURE_EXECUTION: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var bundle struct {
		Invocation graph.Invocation `json:"invocation"`
		Identity   struct {
			CallerProvenanceClass string                 `json:"caller_provenance_class"`
			ResolvedSeeds         []graph.InvocationSeed `json:"resolved_seeds"`
			AggregateFingerprint  string                 `json:"aggregate_fingerprint"`
			AggregateScope        string                 `json:"aggregate_scope"`
		} `json:"identity"`
	}
	if err := json.Unmarshal(data, &bundle); err != nil {
		t.Fatal(err)
	}
	inv := bundle.Invocation
	if inv.Limits != (graph.Limits{MaxDepth: 7, MaxNodes: 11, TimeoutMS: 13000}) || inv.RequestTimeoutMS != 17000 || inv.Concurrency != 1 || inv.LanguageID != "go" || !inv.Expansion.TopmostSiblings || !inv.Expansion.DispatchFamily || !inv.Trace.Enabled || inv.Trace.Path != trace || inv.OutputMode != "file" || inv.OutputPath != output || inv.Server.Command != "missing-server" || len(inv.Server.Arguments) != 1 || inv.Server.Arguments[0] != "--stdio" || inv.Server.Environment["TOKEN"] != "declared" {
		t.Fatalf("ASSERT_P2_COMPLETE_EFFECTIVE_INVOCATION: %#v", inv)
	}
	if len(inv.Seeds) != 2 || len(bundle.Identity.ResolvedSeeds) != 2 || bundle.Identity.CallerProvenanceClass != "CALLER_ASSERTED" || bundle.Identity.AggregateScope != "RESOLVED_SEED_CONTENTS" || !strings.HasPrefix(bundle.Identity.AggregateFingerprint, "sha256:") {
		t.Fatalf("ASSERT_P3_ALL_SEED_IDENTITIES: invocation_seeds=%#v identity=%#v", inv.Seeds, bundle.Identity)
	}
}

func TestVerifyRequiresValidDetachedReceipt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bundle.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":"lsp-trace.graph.v3"}`), 0600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := captureRun(t, []string{"verify", path})
	if code == 0 || stdout != "" || !strings.Contains(stderr, "receipt") {
		t.Errorf("ASSERT_P6_VERIFY_MISSING_SIDECAR: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestPublishAndVerifyRoundTripAndMutation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bundle.json")
	for _, existing := range []string{path, path + ".receipt.json"} {
		if err := os.WriteFile(existing, []byte("permissive old content"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(existing, 0644); err != nil {
			t.Fatal(err)
		}
	}
	data, err := marshalResult(graph.Result{SchemaVersion: graph.SchemaVersionV3, Summary: graph.Summary{Complete: true}}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := publishBundle(path, data); err != nil {
		t.Fatalf("ASSERT_P7_PRIVATE_PUBLICATION: %v", err)
	}
	for _, p := range []string{path, path + ".receipt.json"} {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatalf("ASSERT_P7_FINAL_MODE_0600: %s err=%v", p, err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatalf("ASSERT_P7_FINAL_MODE_0600: %s mode=%v", p, info.Mode().Perm())
		}
	}
	if residues, err := filepath.Glob(filepath.Join(dir, ".lsp-trace-*-*")); err != nil || len(residues) != 0 {
		t.Fatalf("ASSERT_P7_STAGING_CLEANUP: residues=%v err=%v", residues, err)
	}
	stdout, stderr, code := captureRun(t, []string{"verify", path})
	if code != 0 || stdout != "verified integrity and custody\n" || stderr != "" {
		t.Fatalf("ASSERT_P6_VERIFY_SUCCESS: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if err := os.WriteFile(path, append(data, ' '), 0600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code = captureRun(t, []string{"verify", path})
	if code == 0 || stdout != "" || !strings.Contains(stderr, "exact-byte integrity mismatch") {
		t.Fatalf("ASSERT_P6_VERIFY_MUTATION: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestVerifyRejectsInvalidReceiptMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bundle.json")
	data, err := marshalResult(graph.Result{SchemaVersion: graph.SchemaVersionV3}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := publishBundle(path, data); err != nil {
		t.Fatal(err)
	}
	receiptPath := path + ".receipt.json"
	receiptData, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	var receipt map[string]any
	if err := json.Unmarshal(receiptData, &receipt); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		key   string
		value any
	}{
		{"receipt_version", "wrong"},
		{"integrity_claim", "AUTHENTICATED"},
	} {
		mutated := mapsClone(receipt)
		mutated[tc.key] = tc.value
		b, _ := json.Marshal(mutated)
		if err := os.WriteFile(receiptPath, b, 0600); err != nil {
			t.Fatal(err)
		}
		stdout, stderr, code := captureRun(t, []string{"verify", path})
		if code == 0 || stdout != "" || !strings.Contains(stderr, "receipt metadata mismatch") {
			t.Fatalf("ASSERT_P6_STRICT_RECEIPT_METADATA_%s: code=%d stdout=%q stderr=%q", tc.key, code, stdout, stderr)
		}
	}
}

func mapsClone(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func TestConcurrentPublicationUsesUniquePrivateStaging(t *testing.T) {
	dir := t.TempDir()
	data, err := marshalResult(graph.Result{SchemaVersion: graph.SchemaVersionV3}, false)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, name := range []string{"a.json", "b.json"} {
		wg.Add(1)
		go func(path string) { defer wg.Done(); errs <- publishBundle(filepath.Join(dir, path), data) }(name)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("ASSERT_P7_UNIQUE_CONCURRENT_STAGING: %v", err)
		}
	}
}

func TestHistoricalSchemaOutputRemainsAtomicAndPrivateWithoutV3Sidecar(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(workspace, "v2.json")
	stdout, stderr, code := captureRun(t, []string{"incoming", "--workspace", workspace, "--server", "missing-server", "--at", "main.go:1:1", "--schema", "v2", "--output", output})
	if stdout != "" || code != 1 || !strings.Contains(stderr, "spawn") {
		t.Fatalf("ASSERT_P1_V2_FILE_PUBLICATION_STREAMS: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("ASSERT_P1_V2_FILE_PUBLICATION_EXISTS: %v", err)
	}
	if !strings.Contains(string(data), `"schema_version":"lsp-trace.graph.v2"`) {
		t.Fatalf("ASSERT_P1_V2_FILE_PUBLICATION_BYTES: %s", data)
	}
	info, err := os.Stat(output)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("ASSERT_P7_V2_FILE_PRIVATE: info=%v err=%v", info, err)
	}
	if _, err := os.Stat(output + ".receipt.json"); !os.IsNotExist(err) {
		t.Fatalf("ASSERT_P1_V2_NO_V3_SIDECAR: err=%v", err)
	}
}

func TestOutputFailureRetainsCompleteGraph(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(workspace, "missing", "bundle.json")
	stdout, stderr, code := captureRun(t, []string{"incoming", "--workspace", workspace, "--server", "missing-server", "--at", "main.go:1:1", "--output", bad})
	if code == 0 || !json.Valid([]byte(stdout)) || !strings.Contains(stderr, "publish") {
		t.Errorf("ASSERT_P8_PUBLICATION_FAILURE_RETENTION: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}
