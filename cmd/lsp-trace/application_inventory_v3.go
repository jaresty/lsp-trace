package main

import (
	"context"
	"errors"
	inventory "lsp-trace/internal/applicationinventory"
)

// Application inventory is intentionally not registered as a command that could
// acquire native evidence implicitly. Embedding callers must explicitly enable
// this wrapper and supply reviewed preparer/HOST/reviewer configuration. The
// production entry is applicationinventory.Run, not a fixture-only executable.
func runApplicationInventoryV3(ctx context.Context, enabled bool, q inventory.Request) (inventory.Report, error) {
	if !enabled {
		return inventory.Report{}, errors.New("application inventory v3 is disabled")
	}
	return inventory.Run(ctx, q)
}
