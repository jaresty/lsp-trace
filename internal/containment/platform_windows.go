//go:build windows

package containment

func platformProbe() runtimeResult {
	result := unavailable(reasonUnsupportedPlatform, checkPlatformSupport)
	result.platform = "windows"
	return result
}
