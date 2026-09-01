package main

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
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
	data, err := readSelectedArtifact(output)
	if err != nil {
		t.Fatal(err)
	}
	var bundle struct {
		Invocation graph.Invocation `json:"invocation"`
		Identity   struct {
			CallerProvenanceClass      string                 `json:"caller_provenance_class"`
			ResolvedSeeds              []graph.InvocationSeed `json:"resolved_seeds"`
			ResolvedSeedContentsDigest string                 `json:"resolved_seed_contents_digest"`
			AggregateScope             string                 `json:"aggregate_scope"`
		} `json:"identity"`
		ProcessContext graph.ProcessContext `json:"process_context"`
		Seeds          []graph.SeedResult   `json:"seeds"`
		Summary        struct {
			TraversalComplete bool `json:"traversal_complete"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(data, &bundle); err != nil {
		t.Fatal(err)
	}
	inv := bundle.Invocation
	if inv.Limits != (graph.Limits{MaxDepth: 7, MaxNodes: 11, TimeoutMS: 13000}) || inv.RequestTimeoutMS != 17000 || inv.Concurrency != 1 || inv.LanguageID != "go" || !inv.Expansion.TopmostSiblings || !inv.Expansion.DispatchFamily || !inv.Trace.Enabled || inv.Trace.Path != trace || inv.OutputMode != "file" || inv.OutputPath != output || inv.Server.Command != "missing-server" || len(inv.Server.Arguments) != 1 || inv.Server.Arguments[0] != "--stdio" || len(inv.Server.Environment) != 0 {
		t.Fatalf("ASSERT_P2_COMPLETE_EFFECTIVE_INVOCATION: %#v", inv)
	}
	if len(bundle.ProcessContext.Environment) != 1 || bundle.ProcessContext.Environment[0].Name != "TOKEN" || bundle.ProcessContext.Environment[0].Redaction.State != "REDACTED" {
		t.Fatalf("ASSERT_P1_SECRET_SAFE_ENVIRONMENT_IDENTITY: %#v", bundle.ProcessContext)
	}
	if len(inv.Seeds) != 2 || len(bundle.Identity.ResolvedSeeds) != 2 || bundle.Identity.CallerProvenanceClass != "CALLER_ASSERTED" || bundle.Identity.AggregateScope != "RESOLVED_SEED_CONTENTS" || !strings.HasPrefix(bundle.Identity.ResolvedSeedContentsDigest, "sha256:") {
		t.Fatalf("ASSERT_P3_ALL_SEED_IDENTITIES: invocation_seeds=%#v identity=%#v", inv.Seeds, bundle.Identity)
	}
	if len(bundle.Seeds) != 2 || bundle.Seeds[0].Failure == nil || bundle.Seeds[1].Failure == nil || bundle.Seeds[0].Failure.Phase != "spawn" || bundle.Seeds[1].Failure.Phase != "spawn" {
		t.Fatalf("ASSERT_EXACT_RESULT_PER_INVOCATION_SEED_ON_SPAWN_FAILURE: %#v", bundle.Seeds)
	}
	if bundle.Summary.TraversalComplete {
		t.Fatal("ASSERT_SPAWN_FAILURE_CANONICALLY_INCOMPLETE")
	}
}

func TestVerifyRequiresValidDetachedReceipt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bundle.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":"lsp-trace.graph.v3"}`), 0600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := captureRun(t, []string{"verify", path})
	if code == 0 || stdout != "" || !strings.Contains(stderr, "selector") {
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
	selector, err := readGenerationSelector(path)
	if err != nil {
		t.Fatal(err)
	}
	generationDir := filepath.Join(dir, selector.Generation)
	for _, p := range []string{path, generationDir, filepath.Join(generationDir, generationArtifactName), filepath.Join(generationDir, generationReceiptName)} {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatalf("ASSERT_P7_FINAL_PRIVATE_MODE: %s err=%v", p, err)
		}
		if runtime.GOOS != "windows" {
			want := os.FileMode(0600)
			if info.IsDir() {
				want = 0700
			}
			if info.Mode().Perm() != want {
				t.Fatalf("ASSERT_P7_FINAL_PRIVATE_MODE: %s mode=%v want=%v", p, info.Mode().Perm(), want)
			}
		}
	}
	for _, pattern := range []string{".lsp-trace-artifact-*", ".lsp-trace-receipt-*", ".lsp-trace-selector-*"} {
		if residues, err := filepath.Glob(filepath.Join(dir, pattern)); err != nil || len(residues) != 0 {
			t.Fatalf("ASSERT_P7_STAGING_CLEANUP: pattern=%s residues=%v err=%v", pattern, residues, err)
		}
	}
	stdout, stderr, code := captureRun(t, []string{"verify", path})
	if code != 0 || stdout != "verified integrity and custody\n" || stderr != "" {
		t.Fatalf("ASSERT_P6_VERIFY_SUCCESS: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if err := os.WriteFile(filepath.Join(generationDir, generationArtifactName), append(data, ' '), 0600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code = captureRun(t, []string{"verify", path})
	if code == 0 || stdout != "" || !strings.Contains(stderr, "exact-byte integrity mismatch") {
		t.Fatalf("ASSERT_P6_VERIFY_MUTATION: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestVerifyRejectsDetachedReceiptTrailingJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bundle.json")
	data, err := marshalResult(graph.Result{SchemaVersion: graph.SchemaVersionV3}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := publishBundle(path, data); err != nil {
		t.Fatal(err)
	}
	receiptPath, err := selectedGenerationFile(path, generationReceiptName)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(receiptPath, append(receipt, []byte(`{"receipt_version":"second"}`)...), 0600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := captureRun(t, []string{"verify", path})
	if code == 0 || stdout != "" || !strings.Contains(stderr, "trailing JSON content") {
		t.Fatalf("ASSERT_P7_DETACHED_RECEIPT_SINGLE_JSON: code=%d stdout=%q stderr=%q", code, stdout, stderr)
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
	receiptPath, err := selectedGenerationFile(path, generationReceiptName)
	if err != nil {
		t.Fatal(err)
	}
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

func selectedGenerationFile(path, name string) (string, error) {
	selector, err := readGenerationSelector(path)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(path), selector.Generation, name), nil
}

func readSelectedArtifact(path string) ([]byte, error) {
	artifact, err := selectedGenerationFile(path, generationArtifactName)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(artifact)
}

func mapsClone(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func TestV3PublicationSelectsOneCompleteGeneration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bundle.json")
	data, err := marshalResult(graph.Result{SchemaVersion: graph.SchemaVersionV3}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := publishBundle(path, data); err != nil {
		t.Fatal(err)
	}
	selector, err := readGenerationSelector(path)
	if err != nil {
		t.Fatalf("ASSERT_P1_ATOMIC_GENERATION_SELECTOR: %v", err)
	}
	generationDir := filepath.Join(dir, selector.Generation)
	for _, name := range []string{generationArtifactName, generationReceiptName} {
		if info, err := os.Stat(filepath.Join(generationDir, name)); err != nil || !info.Mode().IsRegular() {
			t.Fatalf("ASSERT_P1_COMPLETE_SELECTED_GENERATION: name=%s info=%v err=%v", name, info, err)
		}
	}
	if err := os.Remove(filepath.Join(generationDir, generationReceiptName)); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := captureRun(t, []string{"verify", path})
	if code == 0 || stdout != "" || !strings.Contains(stderr, "incomplete selected generation") {
		t.Fatalf("ASSERT_P5_TRANSITIONAL_GENERATION_REJECTED: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
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

func TestHistoricalSchemaSurvivesSourceFailureEndToEnd(t *testing.T) {
	workspace := t.TempDir()
	stdout, stderr, code := captureRun(t, []string{"incoming", "--workspace", workspace, "--server", "missing-server", "--at", "missing.go:1:1", "--schema", "v2"})
	if code != 2 || !strings.Contains(stdout, `"schema_version":"lsp-trace.graph.v2"`) || strings.Contains(stdout, `"schema_version":"lsp-trace.graph.v3"`) {
		t.Fatalf("ASSERT_P6_END_TO_END_HISTORICAL_SCHEMA: code=%d stdout=%q stderr=%q", code, stdout, stderr)
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
	if err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0600) {
		t.Fatalf("ASSERT_P7_V2_FILE_PRIVATE: info=%v err=%v", info, err)
	}
	if _, err := os.Stat(output + ".receipt.json"); !os.IsNotExist(err) {
		t.Fatalf("ASSERT_P1_V2_NO_V3_SIDECAR: err=%v", err)
	}
}

func TestPublicationPropagatesDirectoryDurabilityFailures(t *testing.T) {
	data, err := marshalResult(graph.Result{SchemaVersion: graph.SchemaVersionV3, Summary: graph.Summary{Complete: true}}, false)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name      string
		failOpen  func(string) bool
		failSync  func(string) bool
		assertion string
	}{
		{
			name:      "generation directory open",
			failOpen:  func(path string) bool { return strings.Contains(filepath.Base(path), ".lsp-trace-generation-") },
			assertion: "ASSERT_DURABILITY_GENERATION_DIRECTORY_OPEN_PROPAGATED",
		},
		{
			name:      "generation directory sync",
			failSync:  func(path string) bool { return strings.Contains(filepath.Base(path), ".lsp-trace-generation-") },
			assertion: "ASSERT_DURABILITY_GENERATION_DIRECTORY_SYNC_PROPAGATED",
		},
		{
			name:      "destination directory open after selector publication",
			failOpen:  func(path string) bool { return filepath.Base(path) == "destination" },
			assertion: "ASSERT_DURABILITY_DESTINATION_DIRECTORY_OPEN_PROPAGATED",
		},
		{
			name:      "destination directory sync after selector publication",
			failSync:  func(path string) bool { return filepath.Base(path) == "destination" },
			assertion: "ASSERT_DURABILITY_DESTINATION_DIRECTORY_SYNC_PROPAGATED",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "destination")
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			originalOpen, originalSync := openPublicationDirectory, syncPublicationDirectory
			t.Cleanup(func() { openPublicationDirectory, syncPublicationDirectory = originalOpen, originalSync })
			openPublicationDirectory = func(path string) (*os.File, error) {
				if tc.failOpen != nil && tc.failOpen(path) {
					return nil, errors.New(tc.assertion)
				}
				return os.Open(path)
			}
			syncPublicationDirectory = func(opened *os.File) error {
				if tc.failSync != nil && tc.failSync(opened.Name()) {
					return errors.New(tc.assertion)
				}
				return opened.Sync()
			}
			err := publishBundle(filepath.Join(dir, "bundle.json"), data)
			if err == nil || !strings.Contains(err.Error(), tc.assertion) {
				t.Fatalf("%s: err=%v", tc.assertion, err)
			}
		})
	}
}

func TestPublicationDisclosesUnavailableDirectorySyncWithoutTreatingItAsFailure(t *testing.T) {
	data, err := marshalResult(graph.Result{SchemaVersion: graph.SchemaVersionV3, Summary: graph.Summary{Complete: true}}, false)
	if err != nil {
		t.Fatal(err)
	}
	originalSync, originalDurability := syncPublicationDirectory, publicationDirectoryDurability
	t.Cleanup(func() {
		syncPublicationDirectory = originalSync
		publicationDirectoryDurability = originalDurability
	})
	syncPublicationDirectory = func(*os.File) error { return errDirectorySyncUnavailable }
	publicationDirectoryDurability = directoryDurabilityUnavailable
	selectorPath := filepath.Join(t.TempDir(), "bundle.json")
	if err := publishBundle(selectorPath, data); err != nil {
		t.Fatalf("ASSERT_PLATFORM_DIRECTORY_SYNC_UNAVAILABLE_NOT_PUBLICATION_FAILURE: %v", err)
	}
	selector, err := readGenerationSelector(selectorPath)
	if err != nil {
		t.Fatal(err)
	}
	receiptData, err := os.ReadFile(filepath.Join(filepath.Dir(selectorPath), selector.Generation, generationReceiptName))
	if err != nil {
		t.Fatal(err)
	}
	var receipt custodyReceipt
	if err := json.Unmarshal(receiptData, &receipt); err != nil || receipt.DirectoryDurability != directoryDurabilityUnavailable {
		t.Fatalf("ASSERT_PLATFORM_DIRECTORY_DURABILITY_DISCLOSED: receipt=%s err=%v", receiptData, err)
	}
}

func TestPublicationFailureRecordPropagatesDirectorySyncFailure(t *testing.T) {
	dir := t.TempDir()
	originalOpen, originalSync := openPublicationDirectory, syncPublicationDirectory
	t.Cleanup(func() { openPublicationDirectory, syncPublicationDirectory = originalOpen, originalSync })
	openPublicationDirectory = os.Open
	syncPublicationDirectory = func(opened *os.File) error {
		if opened.Name() == dir {
			return errors.New("ASSERT_DURABILITY_FAILURE_RECORD_DIRECTORY_SYNC_PROPAGATED")
		}
		return opened.Sync()
	}
	name, err := retainPublicationFailure(filepath.Join(dir, "missing", "bundle.json"), []byte("{}\n"), errors.New("publish failed"))
	if err == nil || !strings.Contains(err.Error(), "ASSERT_DURABILITY_FAILURE_RECORD_DIRECTORY_SYNC_PROPAGATED") || name != "" {
		t.Fatalf("ASSERT_DURABILITY_FAILURE_RECORD_DIRECTORY_SYNC_PROPAGATED: name=%q err=%v", name, err)
	}
}

func TestExactByteCustodyFieldsAndFailureRecordContract(t *testing.T) {
	data := []byte("{\"schema_version\":\"lsp-trace.graph.v3\"}\n")
	receipt, err := receiptBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	var receiptFields map[string]any
	if err := json.Unmarshal(receipt, &receiptFields); err != nil || receiptFields["exact_serialized_bytes_digest"] == nil || receiptFields["digest"] != nil {
		t.Fatalf("ASSERT_DIGEST_ROLE_EXACT_SERIALIZED_BYTES: receipt=%s err=%v", receipt, err)
	}
	if receiptFields["directory_durability"] != publicationDirectoryDurability {
		t.Fatalf("ASSERT_PLATFORM_DIRECTORY_DURABILITY_DISCLOSED: receipt=%s", receipt)
	}
	dir := t.TempDir()
	name, err := retainPublicationFailure(filepath.Join(dir, "missing", "bundle.json"), data, errors.New("publish failed"))
	if err != nil {
		t.Fatal(err)
	}
	failureData, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	var failureFields map[string]any
	decoder := json.NewDecoder(strings.NewReader(string(failureData)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&failureFields); err != nil || failureFields["exact_serialized_bytes_digest"] == nil || failureFields["artifact_digest"] != nil {
		t.Fatalf("ASSERT_DIGEST_ROLE_EXACT_SERIALIZED_BYTES: failure=%s err=%v", failureData, err)
	}
	if failureFields["directory_durability"] != publicationDirectoryDurability {
		t.Fatalf("ASSERT_PLATFORM_DIRECTORY_DURABILITY_DISCLOSED: failure=%s", failureData)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		t.Fatalf("ASSERT_FAILURE_RECORD_PRIVATE_STRICT_JSON: trailing content err=%v data=%q", err, failureData)
	}
	info, err := os.Stat(name)
	if err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0600) {
		t.Fatalf("ASSERT_FAILURE_RECORD_PRIVATE_STRICT_JSON: info=%v err=%v", info, err)
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
	records, err := filepath.Glob(filepath.Join(workspace, ".bundle.json.publication-failure-*.json"))
	if err != nil || len(records) != 1 {
		t.Fatalf("ASSERT_P2_DURABLE_STRUCTURED_PUBLICATION_FAILURE: records=%v err=%v", records, err)
	}
	failureData, err := os.ReadFile(records[0])
	var failure struct {
		Version string `json:"version"`
		Target  string `json:"target"`
		Error   string `json:"error"`
	}
	if err != nil || json.Unmarshal(failureData, &failure) != nil || failure.Version != "lsp-trace.publication-failure.v1" || failure.Target != bad || failure.Error == "" {
		t.Fatalf("ASSERT_P2_DURABLE_STRUCTURED_PUBLICATION_FAILURE: failure=%+v err=%v data=%q", failure, err, failureData)
	}
}
