//go:build !darwin && !linux

package sessionruntime

import (
	"context"
	"io"
	"os"
)

// No weaker fallback: unsupported hosts refuse opt-in before Starter. OFF keeps
// the ordinary discovery/Reader path available and unchanged.
func openPrivateCaptureDirectory(context.Context, string) (*os.File, error) {
	return nil, ErrPrivateDiscovery
}
func privateCaptureOpenFile(context.Context, *os.File, uint8) (io.WriteCloser, error) {
	return nil, ErrPrivateDiscovery
}
