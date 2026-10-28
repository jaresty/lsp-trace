//go:build !darwin && !linux

package graphprovenance

import (
	"context"
	"errors"
	"testing"
)

func TestSourceReaderPrivateV1Unsupported(t *testing.T) {
	root, err := openSourceRootPrivateV1(context.Background(), "/workspace")
	if root != nil || !errors.Is(err, errSourcePrivateUnsupportedV1) {
		t.Fatal("private source root fallback", err)
	}
	raw, err := readSourcePrivateV1(context.Background(), root, "a.go", 10)
	if raw != nil || !errors.Is(err, errSourcePrivateUnsupportedV1) {
		t.Fatal("private source read fallback", err)
	}
}
