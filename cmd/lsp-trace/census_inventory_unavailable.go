//go:build !darwin && !linux

package main

import (
	"context"
	"errors"
	"io"
)

// Private source-retention family has no implementation on these platforms.
// Ordinary Census parsing, help, defaults and execution remain unchanged.
func runCensusInventoryPrivate(o censusCLIOptions, cfg censusCoreConfig, stdout, stderr io.Writer) int {
	_ = cfg.runner.manager.Shutdown(context.Background())
	return writeCensusFailure(stderr, censusStageConfig, true, errors.New("private Census AST inventory unavailable on this platform"))
}
