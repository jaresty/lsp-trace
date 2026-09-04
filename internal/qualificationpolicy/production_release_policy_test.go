package qualificationpolicy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

	release := read("ASSERT_RELEASE_REQUIRES_RETAINED_EXTERNAL_QUALIFICATION", "scripts/release-check.sh")
	contains("ASSERT_RELEASE_REQUIRES_RETAINED_EXTERNAL_QUALIFICATION", release, "qualification/retained/external-provider/ember-glint.json")
	contains("ASSERT_RELEASE_QUALIFICATION_IS_REAL_EXTERNAL_PATH", release, "ASSERT_RELEASE_REAL_EXTERNAL_PROVIDER_QUALIFICATION")
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
