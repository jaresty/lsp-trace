package integratedconformance

import (
	"strings"
	"testing"
)

func ownsADR0007ProductionPath(path string) bool {
	for _, prefix := range []string{
		"internal/censuscontinuation/",
		"internal/censusdiagnostic/",
		"internal/censusprogramc/",
		"internal/continuationhost/",
		"internal/describerequest/",
		"internal/describeworker/",
		"internal/provisionalfeaturecatalog/",
		"internal/v5assertions/",
		"internal/v5sourcesnapshotv4/",
		"internal/v5sourcesnapshotv5/",
		"internal/v5sourcesnapshotv6/",
		"cmd/adr0007-source-text-search-v4-private-",
	} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	switch path {
	case "cmd/lsp-trace/census_batch_acquirer_test.go",
		"cmd/lsp-trace/census_catalog.go",
		"cmd/lsp-trace/census_catalog_cli_test.go",
		"cmd/lsp-trace/census_catalog_runner_test.go",
		"cmd/lsp-trace/census_continuation_test.go",
		"cmd/lsp-trace-mcp/bootstrap_continuation_host_test.go",
		"cmd/lsp-trace-mcp/acquisition_diagnostic_bootstrap_test.go",
		"cmd/lsp-trace-mcp/census_acquisition_process_e2e_test.go",
		"cmd/lsp-trace-mcp/census_acquisition_process_retention_test.go",
		"cmd/lsp-trace-mcp/census_handoff_stage_trace_test.go",
		"cmd/lsp-trace-mcp/census_single_constituent_handoff_test.go",
		"cmd/lsp-trace-mcp/census_continuation_binding_test.go",
		"cmd/lsp-trace-mcp/census_continuation_host.go",
		"cmd/lsp-trace-mcp/census_continuation_test.go",
		"internal/mcpcontract/census_continuation_v2.go",
		"internal/mcpcontract/census_continuation_v2_test.go",
		"internal/mcpcontract/testdata/schemas/envelope-census-feature-catalog-result.v2.schema.json",
		"internal/mcpcontract/testdata/schemas/input-census.v2.schema.json",
		"internal/mcpcontract/testdata/schemas/lsp-trace.census-feature-catalog-result.v2.schema.json",
		"internal/schema/census_feature_catalog_test.go",
		"internal/schema/schemas/lsp-trace.census-feature-catalog-result.v2.schema.json",
		"internal/schema/schemas/lsp-trace.graph-v5-source-snapshot.v3.schema.json",
		"internal/schema/schemas/lsp-trace.graph-v5-source-snapshot.v4.schema.json",
		"internal/schema/schemas/lsp-trace.graph-v5-source-snapshot.v5.schema.json",
		"internal/schema/schemas/lsp-trace.graph-v5-source-snapshot.v6.schema.json",
		"schema/schemas/lsp-trace.graph-v5-source-snapshot.v5.schema.json",
		"schema/schemas/lsp-trace.graph-v5-source-snapshot.v6.schema.json",
		"internal/v4assertions/v4_red_test.go":
		return true
	}
	return false
}

func excludesADR0007OwnershipPath(path string) bool {
	return strings.HasPrefix(path, ".pi/evidence/") ||
		strings.Contains(path, "/.pi/evidence/") ||
		strings.HasPrefix(path, "docs/") ||
		path == "censuscontinuation.test"
}

func TestADR0007ProductionOwnershipRegistration(t *testing.T) {
	for _, path := range []string{
		"internal/censuscontinuation/capture.go",
		"internal/censusprogramc/adapter.go",
		"internal/continuationhost/factory.go",
		"internal/describerequest/request.go",
		"internal/describeworker/runner.go",
		"internal/provisionalfeaturecatalog/catalog.go",
		"internal/v5sourcesnapshotv4/v4.go",
		"internal/v5sourcesnapshotv5/v5.go",
		"internal/v5sourcesnapshotv6/v6.go",
		"cmd/lsp-trace/census_batch_acquirer_test.go",
		"cmd/lsp-trace/census_catalog.go",
		"cmd/lsp-trace/census_continuation_test.go",
		"cmd/lsp-trace-mcp/census_continuation_host.go",
		"cmd/lsp-trace-mcp/census_continuation_binding_test.go",
		"internal/mcpcontract/census_continuation_v2.go",
		"internal/mcpcontract/testdata/schemas/input-census.v2.schema.json",
		"internal/schema/schemas/lsp-trace.census-feature-catalog-result.v2.schema.json",
		"internal/schema/schemas/lsp-trace.graph-v5-source-snapshot.v4.schema.json",
		"internal/schema/schemas/lsp-trace.graph-v5-source-snapshot.v5.schema.json",
		"internal/schema/schemas/lsp-trace.graph-v5-source-snapshot.v6.schema.json",
		"schema/schemas/lsp-trace.graph-v5-source-snapshot.v5.schema.json",
		"schema/schemas/lsp-trace.graph-v5-source-snapshot.v6.schema.json",
		"internal/v4assertions/v4_red_test.go",
		"internal/v5assertions/v5_red_test.go",
	} {
		t.Run(path, func(t *testing.T) {
			if !ownsADR0007ProductionPath(path) {
				t.Fatalf("ASSERT_ADR0007_PRODUCTION_REGISTERED: unowned path %q", path)
			}
		})
	}
}

func TestADR0007OwnershipExclusionsRemainUnowned(t *testing.T) {
	for _, path := range []string{
		".pi/evidence/adr0007-v4-red.txt",
		"cmd/lsp-trace-mcp/.pi/evidence/private-runtime.trace",
		"docs/pilot/adr0007/README.md",
		"docs/pilot/adr0007/governance-authorization-proposal.md",
		"censuscontinuation.test",
	} {
		t.Run(path, func(t *testing.T) {
			if ownsADR0007ProductionPath(path) || !excludesADR0007OwnershipPath(path) {
				t.Fatalf("ASSERT_ADR0007_UNOWNED_EXCLUSION: admitted %q", path)
			}
		})
	}
}

func TestADR0007OwnershipRejectsAdjacentPaths(t *testing.T) {
	for _, path := range []string{
		"internal/censuscontinuation-other/capture.go",
		"internal/v5sourcesnapshotv40/v4.go",
		"cmd/lsp-trace/census_unrelated.go",
		"cmd/lsp-trace-mcp/census_unrelated.go",
		"internal/schema/schemas/unrelated.schema.json",
	} {
		t.Run(path, func(t *testing.T) {
			if ownsADR0007ProductionPath(path) {
				t.Fatalf("ASSERT_ADR0007_ADJACENT_REJECTED: admitted %q", path)
			}
		})
	}
}
