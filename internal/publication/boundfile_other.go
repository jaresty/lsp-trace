//go:build !linux && !darwin

package publication

import (
	"errors"
	"os"
)

var errExactFDUnsupported = errors.New("exact-fd publication primitive unsupported")

func validPublishedMetadata(rootInfo, finalInfo os.FileInfo) bool { return false }

func publishExactFD(root *Root, selector string, raw []byte, verify func([]byte) error, trace BoundFileTrace) (bool, string, string, error) {
	if testHookBoundFileAfterTempBeforeInstall != nil {
		if err := testHookBoundFileAfterTempBeforeInstall(); err != nil {
			emitBoundFileTrace(trace, "HARDLINK", "INJECTED_BEFORE_INSTALL", false)
			return false, DirectorySyncNotAttemptedPostCommit, CloseNotAttempted, errors.Join(errInjectedBeforeInstall, err)
		}
	}
	emitBoundFileTrace(trace, "TEMP", "UNSUPPORTED", false)
	return false, DirectorySyncNotAttemptedPostCommit, CloseNotAttempted, errExactFDUnsupported
}
