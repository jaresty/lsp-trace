//go:build !darwin

package main

import (
	"errors"
	"io"
	"time"
)

// Native normal-folder custody/lifecycle is Darwin-only. Linux's durable
// continuation and publication implementation must not imply native support.
type censusFolderPreparation struct{}

func prepareCensusFolder(censusCLIOptions, time.Time) (*censusFolderPreparation, error) {
	return nil, errors.New("normal-folder native preparation/lifecycle requires Darwin; portable native custody is not implemented")
}
func runCensusFolderAcquisition(o censusCLIOptions, _ censusCoreConfig, diagnostic io.Writer, _ ...io.Writer) int {
	return writeCensusFailure(diagnostic, censusStageConfig, o.Machine, errors.New("normal-folder native lifecycle requires Darwin"))
}
