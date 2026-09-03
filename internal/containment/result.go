// Package containment owns the production runtime containment gate.
//
// This package currently authorizes no platform. Its UNAVAILABLE result is an
// internal containment classification, not the external MCP availability value
// CONTAINMENT_UNAVAILABLE. Synthetic/reference observations live in the
// reference subpackage and cannot be converted into this package's runtime state.
package containment

const maxDiagnosticBytes = 96

type Classification string
type Reason string
type CheckID string

// Unavailable is the only constructible production classification. VERIFIED is
// deliberately absent from this exported API until native proof and a sealed
// production construction path exist.
const Unavailable Classification = "UNAVAILABLE"

const (
	reasonUnsupportedPlatform Reason  = "unsupported_platform"
	checkPlatformSupport      CheckID = "platform_support"
)

type runtimeResult struct {
	classification Classification
	platform       string
	reason         Reason
	failedCheck    CheckID
}

func unavailable(reason Reason, check CheckID) runtimeResult {
	return runtimeResult{
		classification: Unavailable,
		reason:         reasonUnsupportedPlatform,
		failedCheck:    checkPlatformSupport,
	}
}

func boundedPlatform(platform string) string {
	if platform == "" || len(platform) > maxDiagnosticBytes {
		return "unknown"
	}
	return platform
}
