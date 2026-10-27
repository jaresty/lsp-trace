//go:build !darwin && !linux

package censuslineagepilot

import (
	"context"
	"testing"
)

func TestFeatureScopeUnsupportedTransport(t *testing.T) {
	files, identity, e := readFeatureScope(context.Background(), "unused", scopeReadOps{})
	if e == nil || files != nil || identity != (scopeFileIdentity{}) {
		t.Fatal("unsupported source transport did not fail closed")
	}
}
