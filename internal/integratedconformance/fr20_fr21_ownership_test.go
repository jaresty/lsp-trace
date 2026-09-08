package integratedconformance

import (
	"strings"
	"testing"
)

// This registration is limited to FR20 acquisition and the internal FR21 core.
// It grants no public hydration operation or provider adapter ownership.
func ownsFR20FR21Path(path string) bool {
	// Match package boundaries, never all internal/, docs/, schema/ or cmd/.
	for _, prefix := range []string{"acquisitionops/", "internal/acquisition/", "internal/hydratedevidence/"} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	switch path {
	case "docs/fr20-public-acquisition.md", "docs/hydrated-evidence.md",
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
