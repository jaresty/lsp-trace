//go:build !linux && !darwin

package publication

import "os"

func openObservedDirectory(*os.File, string) (*os.File, error) {
	return nil, ErrObservedRootUnsupported
}
