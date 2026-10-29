//go:build !darwin && !linux

package publication

import "os"

func sourceProtectionIdentityPrivateV2(os.FileInfo) (uint64, uint64, error) {
	return 0, 0, ErrObservedRootUnsupported
}
