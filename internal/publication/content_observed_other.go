//go:build !linux && !darwin

package publication

import "os"

func statObservedContent(*Root, string) (os.FileInfo, error) {
	return nil, ErrContentObservationUnsupported
}
func openObservedContent(*Root, string) (observedFile, error) {
	return nil, ErrContentObservationUnsupported
}
