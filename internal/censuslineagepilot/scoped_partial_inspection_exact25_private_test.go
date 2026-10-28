//go:build darwin || linux

package censuslineagepilot

// Extracted read-only from the 8974-byte historical PlannedBatchRequest object
// sha256:7334ab501da333e03ef5cc92597d0be0318ad4d5f27b58e88250ff978c50ab90.
// Only relative paths, labels, order and zero-based positions are retained here.
// Every row has down/up depth 1/1; topmost_siblings is false. Fresh test URIs
// substitute only the workspace prefix, not production retained scope/provenance.
var partialInspectionExact25 = [...]struct {
	id, path        string
	line, character uint32
}{
	{"root", "cmd/lsp-trace/filter_command.go", 15, 5},
	{"target-000009", "cmd/lsp-trace/inspect_command.go", 26, 5},
	{"target-000010", "cmd/lsp-trace/inspect_command.go", 30, 5},
	{"target-000011", "cmd/lsp-trace/inspect_command.go", 34, 5},
	{"target-000013", "cmd/lsp-trace/inspect_command.go", 40, 5},
	{"target-000017", "cmd/lsp-trace/inspect_command.go", 174, 5},
	{"target-000018", "cmd/lsp-trace/inspect_command.go", 179, 5},
	{"target-000024", "internal/filter/filter.go", 15, 25},
	{"target-000029", "internal/filter/filter.go", 25, 24},
	{"target-000109", "internal/filter/filter.go", 127, 5},
	{"target-000110", "internal/filter/filter.go", 185, 5},
	{"target-000111", "internal/filter/filter.go", 197, 5},
	{"target-000112", "internal/filter/filter.go", 208, 5},
	{"target-000113", "internal/filter/filter.go", 215, 5},
	{"target-000114", "internal/filter/filter.go", 222, 5},
	{"target-000115", "internal/filter/filter.go", 229, 5},
	{"target-000116", "internal/filter/filter.go", 236, 5},
	{"target-000117", "internal/filter/filter.go", 262, 5},
	{"target-000118", "internal/filter/filter.go", 282, 5},
	{"target-000119", "internal/filter/filter.go", 285, 5},
	{"target-000214", "internal/inspection/projection.go", 123, 5},
	{"target-000215", "internal/inspection/projection.go", 181, 5},
	{"target-000216", "internal/inspection/projection.go", 198, 5},
	{"target-000217", "internal/inspection/projection.go", 215, 5},
	{"target-000218", "internal/inspection/projection.go", 298, 5},
}

// Digest of decoded historical CanonicalSeedsV2, not a manually encoded fixture.
// EncodeCanonical with fresh workspace-relative paths must reproduce these bytes.
const partialInspectionExact25SeedSHA = "733e8b43cf7e595790e42c61ad4dc45441b48b0ae82a6b93bddfd11b16a95d87"
