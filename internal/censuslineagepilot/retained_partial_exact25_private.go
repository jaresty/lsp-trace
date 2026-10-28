package censuslineagepilot

import (
	"fmt"
	"lsp-trace/internal/acquisitionmanifest"
)

// Immutable historical labels, pathnames and zero-based coordinates, not a
// reconstructed producer identity. The canonical seed digest is independently
// retained by the existing exact-25 producer fixture.
func retainedPartialExact25Private() [25]struct {
	id, path        string
	line, character uint32
} {
	return [25]struct {
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
}
func validateExactPartial25Private(m acquisitionmanifest.Manifest, seeds []byte, files []scopeObservedFile) error {
	if Digest(seeds) != "sha256:733e8b43cf7e595790e42c61ad4dc45441b48b0ae82a6b93bddfd11b16a95d87" || len(m.RequiredTargets) != 24 || m.Expansion.TopmostSiblings {
		return fmt.Errorf("qualification exact canonical 25 seeds")
	}
	targets := append([]acquisitionmanifest.Target{m.Root}, m.RequiredTargets...)
	for i, row := range retainedPartialExact25Private() {
		t := targets[i]
		uri := ""
		for _, file := range files {
			if file.Path == row.path && file.Role == "NOMINATION" {
				uri = file.URI
			}
		}
		if uri == "" || t.ID != row.id || t.Locator.URI != uri || t.Locator.Line == nil || t.Locator.Character == nil || *t.Locator.Line != row.line || *t.Locator.Character != row.character || t.DownDepth == nil || t.UpDepth == nil || *t.DownDepth != 1 || *t.UpDepth != 1 {
			return fmt.Errorf("qualification exact target %d", i)
		}
	}
	return nil
}
