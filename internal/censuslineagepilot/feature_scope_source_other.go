//go:build !darwin && !linux

package censuslineagepilot

import (
	"context"
	"errors"
)

func readFeatureScope(context.Context, string, scopeReadOps) ([]scopeObservedFile, scopeFileIdentity, error) {
	return nil, scopeFileIdentity{}, errors.New("feature scope no-follow source transport unsupported")
}
