package publication

import (
	"errors"
	"os"
	"runtime"
)

// SyncDirectory syncs this pinned publication root, not an ambient pathname.
// It is intended for publications directly beneath the root (artifact.json and
// receipt.json). false,nil discloses Windows' unsupported directory-sync policy;
// all supported-platform failures are errors, never downgraded to unavailable.
func (r *Root) SyncDirectory() (bool, error) {
	if r == nil || r.file == nil {
		return false, errors.New("publication root is closed")
	}
	return syncDirectory(r.file, runtime.GOOS)
}

func syncDirectory(dir *os.File, goos string) (bool, error) {
	if goos == "windows" {
		return false, nil
	}
	if err := dir.Sync(); err != nil {
		return false, err
	}
	return true, nil
}
