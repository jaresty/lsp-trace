//go:build !darwin

package managedprocess

import (
	"errors"
	"os"
	"os/exec"
)

func censusLocalDarwinGroupPrivateWindow(_ int, _ *privateProviderWindow) GroupCensusObservation {
	return GroupCensusObservation{Limit: 64, Bounded: true, Truncated: true, Err: ErrPrivateProviderWindow}
}
func localDarwinSupported() bool { return false }
func configureLocalDarwin(*exec.Cmd) error {
	return errors.New("managedprocess: local Darwin supervision unavailable on this platform")
}
func signalLocalDarwinGroup(int, os.Signal) error {
	return errors.New("managedprocess: local Darwin supervision unavailable on this platform")
}
func censusLocalDarwinGroup(_ int, limit int) GroupCensusObservation {
	return GroupCensusObservation{Limit: limit, Bounded: true, Err: errors.New("managedprocess: local Darwin supervision unavailable on this platform")}
}
