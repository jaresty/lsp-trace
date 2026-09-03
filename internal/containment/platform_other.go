//go:build !darwin && !linux && !windows

package containment

import "runtime"

func platformProbe() runtimeResult {
	result := unavailable(reasonUnsupportedPlatform, checkPlatformSupport)
	result.platform = boundedPlatform(runtime.GOOS)
	return result
}
