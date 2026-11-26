//go:build !darwin

package main

import "io"

func runCensusOwnerStart(o censusCLIOptions, out, diagnostic io.Writer) int {
	return writeCensusFailure(diagnostic, censusStageConfig, true, errCensusCallerHostUnavailable)
}
func runCensusOwnerOperation(args []string, input io.Reader, out, diagnostic io.Writer) int {
	return runCensusDurable(args, input, out, diagnostic)
}
