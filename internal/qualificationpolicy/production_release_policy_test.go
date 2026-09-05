package qualificationpolicy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestExternalProviderQualifierRejectsRepositorySymlink(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "providers", "ember-glint", "bin", "ember-glint.mjs")
	link := filepath.Join(t.TempDir(), "external-provider")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(filepath.Join(root, "scripts", "qualify-external-provider.sh"), link)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "FAIL ASSERT_EXTERNAL_PROVIDER_REAL_PACKAGE_PATH") {
		t.Fatalf("ASSERT_EXTERNAL_PROVIDER_REAL_TARGET_PATH: err=%v output=%s", err, output)
	}
}

func TestRetainedExternalProviderMCPGraphV4(t *testing.T) {
	const assertion = "ASSERT_RELEASE_RETAINED_EXTERNAL_PROVIDER_MCP_GRAPH_V4"
	raw, err := os.ReadFile(filepath.Join("..", "..", "qualification", "retained", "external-provider", "ember-glint-mcp.json"))
	if err != nil {
		t.Fatalf("%s: %v", assertion, err)
	}
	var evidence struct {
		Outcome                             string          `json:"outcome"`
		Transport                           string          `json:"transport"`
		ProviderPathKind                    string          `json:"provider_path_kind"`
		ProviderIdentity                    string          `json:"provider_identity"`
		RequestedRelation                   string          `json:"requested_relation"`
		ManagedSessionSubstrate             string          `json:"managed_session_substrate"`
		TextEnvelopeEqualsStructuredContent bool            `json:"text_envelope_equals_structured_content"`
		InlineGraphV4SchemaValid            bool            `json:"inline_graph_v4_schema_valid"`
		DeterministicReplay                 bool            `json:"deterministic_replay"`
		GraphV4                             json.RawMessage `json:"graph_v4"`
	}
	if err := json.Unmarshal(raw, &evidence); err != nil || evidence.Outcome != "PASS" || evidence.Transport != "real-lsp-trace-mcp-stdio" || evidence.ProviderPathKind != "absolute-external" || evidence.ProviderIdentity != "ember-glint@1" || evidence.RequestedRelation != "BINDS_ARGUMENT" || evidence.ManagedSessionSubstrate != "managed-fake-lsp" || !evidence.TextEnvelopeEqualsStructuredContent || !evidence.InlineGraphV4SchemaValid || !evidence.DeterministicReplay || !strings.Contains(string(evidence.GraphV4), `"schema_version": "lsp-trace.graph.v4"`) || !strings.Contains(string(evidence.GraphV4), `"kind": "BINDS_ARGUMENT"`) {
		t.Fatalf("%s: exact MCP transport graph-v4 evidence required: evidence=%+v err=%v", assertion, evidence, err)
	}
}

func TestRetainedExternalProviderResponseDigest(t *testing.T) {
	const assertion = "ASSERT_RELEASE_RETAINED_PROVIDER_RESPONSE_DIGEST"
	raw, err := os.ReadFile(filepath.Join("..", "..", "qualification", "retained", "external-provider", "ember-glint.json"))
	if err != nil {
		t.Fatalf("%s: %v", assertion, err)
	}
	var evidence struct {
		Response       []byte `json:"response"`
		ResponseSHA256 string `json:"response_sha256"`
	}
	if err := json.Unmarshal(raw, &evidence); err != nil || len(evidence.Response) == 0 {
		t.Fatalf("%s: retained exact provider response is required: %v", assertion, err)
	}
	digest := sha256.Sum256(evidence.Response)
	if got := hex.EncodeToString(digest[:]); got != evidence.ResponseSHA256 {
		t.Fatalf("%s: got=%s want=%s", assertion, got, evidence.ResponseSHA256)
	}
}

func TestProductionQualificationAndReleasePolicy(t *testing.T) {
	root := filepath.Join("..", "..")
	read := func(assertion, name string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("%s: %v", assertion, err)
		}
		return string(raw)
	}
	contains := func(assertion, body, want string) {
		t.Helper()
		if !strings.Contains(body, want) {
			t.Errorf("%s: missing %q", assertion, want)
		}
	}
	notContains := func(assertion, body, forbidden string) {
		t.Helper()
		if strings.Contains(body, forbidden) {
			t.Errorf("%s: forbidden %q", assertion, forbidden)
		}
	}

	providers := read("ASSERT_EXTERNAL_PROVIDER_INSTALLATION_DOCUMENTED", "docs/PROVIDERS.md")
	contains("ASSERT_EXTERNAL_PROVIDER_INSTALLATION_DOCUMENTED", providers, "independently install")
	contains("ASSERT_EXTERNAL_PROVIDER_ABSOLUTE_PATH_REGISTRATION", providers, "absolute executable path")
	contains("ASSERT_EXTERNAL_PROVIDER_NO_DISCOVERY", providers, "never downloads or implicitly discovers")

	qualifier := read("ASSERT_GENERIC_EXTERNAL_PROVIDER_QUALIFIER_EXISTS", "scripts/qualify-external-provider.sh")
	contains("ASSERT_GENERIC_EXTERNAL_PROVIDER_QUALIFIER_ACCEPTS_PATH", qualifier, "LSP_TRACE_EXTERNAL_PROVIDER_PATH")
	contains("ASSERT_GENERIC_EXTERNAL_PROVIDER_REJECTS_FAKE_TESTDATA", qualifier, "ASSERT_EXTERNAL_PROVIDER_REAL_PACKAGE_PATH")
	contains("ASSERT_GENERIC_EXTERNAL_PROVIDER_LIFECYCLE", qualifier, "TestProductionExternalProviderCompletesManagedLifecycle")
	contains("ASSERT_GENERIC_EXTERNAL_PROVIDER_MCP_TRANSPORT", qualifier, "TestProductionMCPExternalEmberGlintProvider")

	release := read("ASSERT_RELEASE_REQUIRES_RETAINED_EXTERNAL_QUALIFICATION", "scripts/release-check.sh")
	contains("ASSERT_RELEASE_REQUIRES_RETAINED_EXTERNAL_QUALIFICATION", release, "qualification/retained/external-provider/ember-glint.json")
	contains("ASSERT_RELEASE_QUALIFICATION_IS_REAL_EXTERNAL_PATH", release, "ASSERT_RELEASE_REAL_EXTERNAL_PROVIDER_QUALIFICATION")
	contains("ASSERT_RELEASE_REQUIRES_RETAINED_EXTERNAL_MCP_QUALIFICATION", release, "qualification/retained/external-provider/ember-glint-mcp.json")
	contains("ASSERT_RELEASE_ATTESTS_REAL_MCP_TRANSPORT", release, "ASSERT_RELEASE_EXTERNAL_PROVIDER_MCP_TRANSPORT")
	contains("ASSERT_RELEASE_ATTESTS_EXACT_MCP_GRAPH_V4", release, "ASSERT_RELEASE_EXTERNAL_PROVIDER_MCP_GRAPH_V4")
	notContains("ASSERT_RELEASE_DOES_NOT_BUILD_BUNDLED_PROVIDER", release, "go build -trimpath -o \"$release_tmp/lsp-trace-provider-ember-glint\"")

	goreleaser := read("ASSERT_CORE_ARCHIVES_EXCLUDE_PROVIDER_ASSETS", ".goreleaser.yaml")
	notContains("ASSERT_CORE_ARCHIVES_EXCLUDE_PROVIDER_ASSETS", goreleaser, "lsp-trace-provider-ember-glint")
	contains("ASSERT_CORE_ARCHIVES_EXPLICIT_BINARY_IDS", goreleaser, "ids: [lsp-trace, lsp-trace-mcp]")

	releasing := read("ASSERT_RELEASE_POLICY_DOCUMENTED", "docs/RELEASING.md")
	contains("ASSERT_RELEASE_POLICY_DOCUMENTED", releasing, "at least one retained real external-provider production qualification")
	contains("ASSERT_RELEASE_DOES_NOT_REQUIRE_BUNDLED_ANALYZER", releasing, "does not require a bundled analyzer")
	contains("ASSERT_EXTERNAL_QUALIFICATION_E2E_GRAPH_V4", releasing, "MCP request → provider subprocess → observations → adapter → graph-v4")
	contains("ASSERT_EXTERNAL_QUALIFICATION_IDENTITIES_ANCHORS", releasing, "contributor IDs and original anchors")
	contains("ASSERT_EXTERNAL_QUALIFICATION_DETERMINISTIC", releasing, "deterministic replay")
	contains("ASSERT_EXTERNAL_QUALIFICATION_GLINT_BLOCKED_HONEST", releasing, "unsupported Glint outcomes remain `BLOCKED`")
	contains("ASSERT_EXTERNAL_QUALIFICATION_GRAPH_V3_PARITY", releasing, "graph-v3 omission parity")
}
