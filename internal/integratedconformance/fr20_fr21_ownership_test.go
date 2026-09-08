package integratedconformance

import (
	"strings"
	"testing"
)

// This registration covers FR20, the FR21 core and its bounded public wrapper.
// It grants no provider adapter ownership or wildcard root evidence exception.
func ownsFR20FR21Path(path string) bool {
	// Match package boundaries, never all internal/, docs/, schema/ or cmd/.
	for _, prefix := range []string{"acquisitionops/", "internal/acquisition/", "internal/hydratedevidence/", "internal/hydratedinspection/"} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	switch path {
	case "focused-final-tests.log", "focused-mutation.log", "focused-red.log", "focused-hydration.claim.md", "focused-vet.log",
		"public-hydration-red.log",
		"cmd/lsp-trace/inspect_command.go", "cmd/lsp-trace/inspect_hydrated.go", "cmd/lsp-trace/inspect_hydrated_test.go", "cmd/lsp-trace/inspect_hydrated_unix_test.go", "cmd/lsp-trace-mcp/hydrated_public_test.go", "cmd/lsp-trace-mcp/hydrated_controls_test.go",
		"internal/mcpcontract/inspect_hydrated.go", "internal/operation/inspect_hydrated.go", "internal/operation/inspect_hydrated_test.go",
		"docs/fr20-public-acquisition.md", "docs/hydrated-evidence.md",
		"cmd/lsp-trace/acquisition_v2.go", "cmd/lsp-trace/acquisition_v2_test.go", "cmd/lsp-trace/publication.go",
		"cmd/lsp-trace-mcp/fr20_graph_test.go", "cmd/lsp-trace-mcp/fr20_legacy_test.go", "cmd/lsp-trace-mcp/fr20_native_test.go", "cmd/lsp-trace-mcp/fr20_public_test.go", "cmd/lsp-trace-mcp/fr20_hydration_test.go",
		"cmd/lsp-trace-mcp/testdata/fr20-server/main.go",
		"internal/mcp/acquisition_v2_numbers_test.go", "internal/mcp/acquisition_v2_test.go", "internal/mcp/bounded_analysis.go",
		"internal/operation/verify_v2.go",
		"internal/schema/schemas/lsp-trace.graph-provenance.v2.schema.json":
		return true
	}
	return false
}

func TestFR20FR21OwnershipRegistration(t *testing.T) {
	for _, path := range []string{
		"focused-final-tests.log", "focused-mutation.log", "focused-red.log", "focused-hydration.claim.md", "focused-vet.log",
		"acquisitionops/executor.go", "acquisitionops/executor_test.go",
		"internal/acquisition/coordinator.go",
		"internal/hydratedevidence/admission.go", "internal/hydratedevidence/schema.json",
		"docs/fr20-public-acquisition.md", "docs/hydrated-evidence.md",
		"cmd/lsp-trace/acquisition_v2.go", "cmd/lsp-trace/acquisition_v2_test.go",
		"cmd/lsp-trace-mcp/fr20_hydration_test.go", "cmd/lsp-trace-mcp/fr20_native_test.go",
		"cmd/lsp-trace-mcp/testdata/fr20-server/main.go",
		"internal/schema/schemas/lsp-trace.graph-provenance.v2.schema.json",
	} {
		t.Run(path, func(t *testing.T) {
			if !ownsFR20FR21Path(path) {
				t.Fatalf("ASSERT_FR20_FR21_REGISTERED: unowned path %q", path)
			}
		})
	}
}

func TestFR20FR21OwnershipRejectsUnowned(t *testing.T) {
	for _, path := range []string{
		"unowned.log", "focused-other.log", "unowned.claim.md",
		"internal/unregistered/new.go", "docs/unregistered.md",
		"internal/hydratedevidence-other/schema.json", "acquisitionops-other/executor.go",
		"internal/acquisition-other/acquisition.go", "cmd/lsp-trace-mcp/testdata/unknown-server/main.go",
		"cmd/lsp-trace-mcp/public_hydration.go", "internal/schema/schemas/lsp-trace.hydrated-evidence.v1.schema.json",
		"internal/schema/schemas/unowned.json",
	} {
		t.Run(path, func(t *testing.T) {
			if ownsFR20FR21Path(path) {
				t.Fatalf("ASSERT_UNOWNED_REJECTED: admitted %q", path)
			}
		})
	}
}
