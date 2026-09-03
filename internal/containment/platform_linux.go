//go:build linux

package containment

func platformProbe() runtimeResult {
	result := unavailable(reasonUnsupportedPlatform, checkPlatformSupport)
	result.platform = "linux"
	return result
}
