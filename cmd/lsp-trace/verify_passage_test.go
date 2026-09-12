package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/schema"
)

const passageFamily = "passage-verification"

func passageCLIFixture(t *testing.T) (string, []string) {
	t.Helper()
	uri := "file:///w/a.go"
	node := graph.NewNode(graph.Item{Name: "A", Kind: 12, URI: uri, Range: graph.Range{End: graph.Position{Character: 1}}, SelectionRange: graph.Range{End: graph.Position{Character: 1}}})
	g := graph.Result{
		SchemaVersion: graph.SchemaVersionV3,
		Invocation:    graph.Invocation{Server: graph.ServerInvocation{Command: "retained"}, Seeds: []graph.InvocationSeed{{Label: "seed", At: "a.go:1:1", ResolvedURI: uri, LanguageID: "go"}}},
		Nodes:         []graph.Node{node}, Seeds: []graph.SeedResult{{Label: "seed", ReachedNodeIDs: []string{node.ID}}},
		Summary: graph.Summary{NodeCount: 1, Complete: true}, Capabilities: graph.Capabilities{CallHierarchyProvider: true},
	}
	raw, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	var identity struct {
		ExecutionBundleID string `json:"execution_bundle_id"`
	}
	if err := json.Unmarshal(raw, &identity); err != nil {
		t.Fatal(err)
	}
	digest := fmt.Sprintf("sha256:%x", sha256.Sum256(raw))
	path := filepath.Join(t.TempDir(), "artifact.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{"verify", "passage", "--artifact-digest", digest, "--inspection-id", identity.ExecutionBundleID, "--seed", "seed", "--node", node.ID, "--uri", uri, "--range-mode", "EXACT", "--position-encoding", "utf-8", "--start-line", "0", "--start-character", "0", "--end-line", "0", "--end-character", "1", "--passage-digest", "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", path}
	return path, args
}

func TestVerifyPassageDirectArtifactPreservesCoreResultAndOutputSchema(t *testing.T) {
	_, args := passageCLIFixture(t)
	stdout, stderr, code := captureRun(t, args)
	const assertion = "ASSERT_VERIFY_PASSAGE_DIRECT_CORE_AND_SCHEMA"
	t.Logf("ASSERTION: %s; code=%d stderr=%q", assertion, code, stderr)
	if code != 0 || stderr != "" {
		t.Fatalf("%s: code=%d stderr=%q", assertion, code, stderr)
	}
	if _, err := schema.ValidateFor([]byte(stdout), passageFamily, "v1"); err != nil {
		t.Fatalf("%s: %v\n%s", assertion, err, stdout)
	}
	var got struct {
		Results []struct {
			Overall       string `json:"overall"`
			Reacquisition bool   `json:"reacquisition_needed"`
			Checks        struct {
				SelectorCustody  string `json:"selector_custody"`
				PassageBytes     string `json:"passage_bytes"`
				BodyCompleteness string `json:"body_completeness"`
			} `json:"checks"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Results) != 1 || got.Results[0].Overall != "SOURCE_BYTES_UNAVAILABLE" || !got.Results[0].Reacquisition || got.Results[0].Checks.SelectorCustody != "SELECTOR_CUSTODY_NOT_ESTABLISHED" || got.Results[0].Checks.PassageBytes != "SOURCE_BYTES_UNAVAILABLE" || got.Results[0].Checks.BodyCompleteness != "NOT_EVALUATED" {
		t.Fatalf("%s: %+v", assertion, got)
	}
}

func TestVerifyPassageStrictIngressExclusivityAndPrivacy(t *testing.T) {
	_, base := passageCLIFixture(t)
	withFlags := func(extra ...string) []string {
		args := append([]string{}, base[:len(base)-1]...)
		args = append(args, extra...)
		return append(args, base[len(base)-1])
	}
	cases := [][]string{
		withFlags("--publication-root", t.TempDir()),
		withFlags("--private-root", t.TempDir(), "--enable-private-paths"),
		withFlags("--publication-root", t.TempDir(), "--artifact-store", t.TempDir()),
	}
	for i, args := range cases {
		stdout, stderr, code := captureRun(t, args)
		const assertion = "ASSERT_VERIFY_PASSAGE_STRICT_EXCLUSIVITY_PRIVATE_DIAGNOSTICS"
		t.Logf("ASSERTION: %s; case=%d code=%d stderr=%q", assertion, i, code, stderr)
		if code == 0 || stdout != "" || !strings.Contains(stderr, "immutable ingress and exact metadata must be supplied together") || strings.Contains(stderr, "file:///w/a.go") || strings.Contains(stderr, "aaaaaaaa") {
			t.Fatalf("%s: case=%d code=%d stdout=%q stderr=%q", assertion, i, code, stdout, stderr)
		}
	}
}

func TestVerifyPassageContentAndPrivateIngressDoNotClaimSelectorCustody(t *testing.T) {
	path, base := passageCLIFixture(t)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	digest := fmt.Sprintf("sha256:%x", sum)
	generation := fmt.Sprintf("g-%x", sum)
	schemaID := "https://jaresty.github.io/lsp-trace/schemas/lsp-trace.graph.v3.schema.json"
	withIngress := func(input string, flags ...string) []string {
		args := append([]string{}, base[:len(base)-1]...)
		args = append(args, "--artifact-schema-id", schemaID, "--artifact-generation", generation, "--artifact-byte-length", fmt.Sprint(len(raw)))
		args = append(args, flags...)
		return append(args, input)
	}
	store := t.TempDir()
	if err := os.WriteFile(filepath.Join(store, strings.TrimPrefix(digest, "sha256:")), raw, 0600); err != nil {
		t.Fatal(err)
	}
	private := t.TempDir()
	if err := os.Chmod(private, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(private, "artifact.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	cases := [][]string{
		withIngress(digest, "--artifact-store", store),
		withIngress("artifact.json", "--private-root", private, "--enable-private-paths"),
	}
	for i, args := range cases {
		stdout, stderr, code := captureRun(t, args)
		const assertion = "ASSERT_VERIFY_PASSAGE_NONSELECTOR_INGRESS_CUSTODY_CEILING"
		t.Logf("ASSERTION: %s; case=%d code=%d stderr=%q", assertion, i, code, stderr)
		if code != 0 || stderr != "" || !strings.Contains(stdout, `"selector_custody":"SELECTOR_CUSTODY_NOT_ESTABLISHED"`) {
			t.Fatalf("%s: case=%d code=%d stdout=%q stderr=%q", assertion, i, code, stdout, stderr)
		}
	}
}

func TestVerifyPassageRejectsNonRegularAndOversizedDirectFiles(t *testing.T) {
	path, base := passageCLIFixture(t)
	for _, replacement := range []string{t.TempDir(), filepath.Join(t.TempDir(), "oversized.json")} {
		if strings.HasSuffix(replacement, "oversized.json") {
			f, err := os.Create(replacement)
			if err != nil {
				t.Fatal(err)
			}
			if err := f.Truncate((192 << 20) + 1); err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
		}
		args := append([]string{}, base...)
		args[len(args)-1] = replacement
		stdout, stderr, code := captureRun(t, args)
		const assertion = "ASSERT_VERIFY_PASSAGE_BOUNDED_REGULAR_FILE"
		t.Logf("ASSERTION: %s; input=%q code=%d stderr=%q", assertion, filepath.Base(replacement), code, stderr)
		if code == 0 || stdout != "" {
			t.Fatalf("%s: admitted %q (fixture %q)", assertion, replacement, path)
		}
	}
}

func TestVerifyPassageDigestMutationsFailClosed(t *testing.T) {
	_, base := passageCLIFixture(t)
	mutations := []struct{ flag, value, want string }{
		{"--artifact-digest", "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "ARTIFACT_DIGEST_MISMATCH"},
		{"--end-character", "2", "RANGE_MISMATCH"},
	}
	for _, mutation := range mutations {
		args := append([]string{}, base...)
		for i := range args {
			if args[i] == mutation.flag {
				args[i+1] = mutation.value
			}
		}
		stdout, stderr, code := captureRun(t, args)
		const assertion = "ASSERT_VERIFY_PASSAGE_DIGEST_AND_RANGE_MUTATIONS_FAIL_CLOSED"
		t.Logf("ASSERTION: %s; flag=%s code=%d stderr=%q", assertion, mutation.flag, code, stderr)
		if code != 0 || stderr != "" || !strings.Contains(stdout, mutation.want) {
			t.Fatalf("%s: flag=%s code=%d stdout=%q stderr=%q", assertion, mutation.flag, code, stdout, stderr)
		}
	}
}

func TestVerifyPassageSubprocessEmitsParseableJSON(t *testing.T) {
	_, args := passageCLIFixture(t)
	binary := buildStartupSinkCLI(t)
	cmd := exec.Command(binary, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	const assertion = "ASSERT_VERIFY_PASSAGE_PROCESS_JSON_STDOUT"
	t.Logf("ASSERTION: %s; err=%v stderr=%q", assertion, err, stderr.String())
	var output map[string]any
	if err != nil || stderr.Len() != 0 || json.Unmarshal(stdout.Bytes(), &output) != nil || output["schema_version"] != "lsp-trace.passage-verification.v1" {
		t.Fatalf("%s: err=%v stdout=%q stderr=%q", assertion, err, stdout.String(), stderr.String())
	}
}

func TestVerifyPassageUsageAndSchemaSurface(t *testing.T) {
	const assertion = "ASSERT_VERIFY_PASSAGE_HELP_AND_SCHEMA_SURFACE"
	if !strings.Contains(usageText, "lsp-trace verify passage") {
		t.Fatalf("%s: missing global help", assertion)
	}
	stdout, stderr, code := captureRun(t, []string{"schema", "get", "--family", passageFamily, "--version", "v1"})
	t.Logf("ASSERTION: %s; code=%d stderr=%q", assertion, code, stderr)
	if code != 0 || stderr != "" || !strings.Contains(stdout, `"title": "Retained passage verification result"`) {
		t.Fatalf("%s: code=%d stderr=%q stdout=%q", assertion, code, stderr, stdout)
	}
}
