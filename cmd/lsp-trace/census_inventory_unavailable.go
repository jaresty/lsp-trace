//go:build !darwin && !linux

package main

import (
	"context"
	"errors"
	"io"
)

func runCensusApplicationQualifiedCLIPrivate(o censusCLIOptions, stdout, stderr io.Writer) int {
	return writeCensusFailure(stderr, censusStageConfig, o.Machine, errors.New("private Census qualified application unavailable on this platform"))
}

func runCensusInventoryBridgePrivate(o censusCLIOptions, stdout, stderr io.Writer, deps censusRunnerDependencies) int {
	return writeCensusFailure(stderr, censusStageConfig, o.Machine, errors.New("private Census inventory bridge unavailable on this platform"))
}

func runCensusCallerHost(o censusCLIOptions, stdout, stderr io.Writer, deps censusRunnerDependencies) int {
	return writeCensusFailure(stderr, censusStageConfig, o.Machine, errors.New("Census caller host unavailable on this platform"))
}

// Private source-retention family has no implementation on these platforms.
// Ordinary Census parsing, help, defaults and execution remain unchanged.
func runCensusInventoryPrivate(o censusCLIOptions, cfg censusCoreConfig, stdout, stderr io.Writer) int {
	_ = cfg.runner.manager.Shutdown(context.Background())
	return writeCensusFailure(stderr, censusStageConfig, true, errors.New("private Census AST inventory unavailable on this platform"))
}
