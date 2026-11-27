//go:build !darwin

package main

import (
	"errors"
	"io"
)

// Native normal-folder custody/lifecycle is Darwin-only. Linux's metadata-only
// preparation and durable publication must not imply native acquisition support.
func runCensusFolderAcquisition(o censusCLIOptions, _ censusCoreConfig, diagnostic io.Writer, _ ...io.Writer) int {
	return writeCensusFailure(diagnostic, censusStageConfig, o.Machine, errors.New("normal-folder native lifecycle requires Darwin"))
}
