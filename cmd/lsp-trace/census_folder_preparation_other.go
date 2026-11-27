//go:build !darwin && !linux

package main

import (
	"errors"
	"time"
)

// Funded metadata-only preparation is supported on Darwin and Linux.
// Other platforms must not infer preparation support from acquisition stubs.
type censusFolderPreparation struct{}

func prepareCensusFolder(censusCLIOptions, time.Time) (*censusFolderPreparation, error) {
	return nil, errors.New("normal-folder metadata preparation requires Darwin or Linux")
}
