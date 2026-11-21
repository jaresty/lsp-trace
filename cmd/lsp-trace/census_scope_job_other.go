//go:build !darwin && !linux

package main

import (
	"errors"
	"io"
)

func runCensusScopeJob(_ censusCLIOptions, _ io.Writer, diagnostic io.Writer) int {
	return writeCensusFailure(diagnostic, censusStageConfig, true, errors.New("scope-job candidate unsupported on this platform"))
}
func runCensusScopeJobStatus(_ []string, _ io.Writer, diagnostic io.Writer) int {
	return writeCensusFailure(diagnostic, censusStageConfig, true, errors.New("scope-job candidate unsupported on this platform"))
}
