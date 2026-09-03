//go:build darwin

package containment

func platformProbe() runtimeResult {
	result := unavailable(reasonUnsupportedPlatform, checkPlatformSupport)
	result.platform = "darwin"
	return result
}
