//go:build !darwin && !linux

package publication

import (
	"errors"
	"os"
)

func stableFileIdentity(os.FileInfo) (string, error) {
	return "", errors.New("capability-bound activation is unsupported on this platform")
}
